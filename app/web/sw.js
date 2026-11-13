// Service worker di Gaster Bless: permette di installare l'app sulla schermata Home e di riaprirla
// anche se la rete di casa ha un attimo di incertezza.
// Regola: i dati (API e canale in tempo reale) non passano MAI dalla cache, solo le pagine e le risorse.
const VERSIONE = "gasterbless-v1"; // v1 (2026-11-13): prima versione di Gaster Bless
const GUSCIO = ["/", "/stile/base.css", "/ui/app.js", "/ui/ui.js", "/manifest.webmanifest", "/img/icona-192.png",
  "/remoto/", "/remoto/remoto.js", "/remoto/remoto.css", // la pagina del gioco da remoto serve anche fuori casa
  "/sezioni/schede.js", "/ui/offline.js", "/ui/campi_scheda.js", "/ui/modifiche_manuali.js", "/ui/riposi.js", "/ui/chat.js", "/ui/viaggio.js", "/ui/iphone.js", "/ui/salvavita.js", "/ui/guscio.js", "/ui/aggiorna.js", "/scheda/"]; // la scheda anche senza connessione (offline.js)
const ATTESA_RETE = 4000; // fuori casa il PC non risponde: dopo 4 secondi si usa la copia salvata

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(VERSIONE).then((c) => c.addAll(GUSCIO)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (e) => {
  e.waitUntil(caches.keys()
    .then((k) => Promise.all(k.filter((n) => n !== VERSIONE).map((n) => caches.delete(n))))
    .then(() => self.clients.claim()));
});

self.addEventListener("fetch", (e) => {
  const r = e.request;
  const u = new URL(r.url);
  if (r.method !== "GET" || u.origin !== location.origin) return;
  if (u.pathname.startsWith("/api/") || u.pathname === "/ws") return; // sempre dal server
  // Prima la rete (così si vede sempre l'ultima versione), la cache solo se il server non risponde.
  const rete = fetch(r).then((risp) => {
    if (risp.status === 200) { // (le risposte parziali dei video non si possono salvare)
      const copia = risp.clone();
      caches.open(VERSIONE).then((c) => c.put(r, copia)).catch(() => {});
    }
    return risp;
  });
  const conLimite = Promise.race([rete, new Promise((_, no) => setTimeout(() => no(new Error("lento")), ATTESA_RETE))]);
  e.respondWith(conLimite.catch(async () => {
    const c = (await caches.match(r)) || (await caches.match(r, { ignoreSearch: true })); // la scheda si apre con «?host=…»
    if (c) return c;
    try { return await rete; } catch (err) { /* il server non risponde e non c'è una copia */ }
    if (r.mode === "navigate") return (await caches.match("/")) || Response.error(); // le sezioni (/mappa, /schede...) usano la pagina principale
    return Response.error();
  }));
});
