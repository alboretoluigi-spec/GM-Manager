// Libreria interna comune a tutte le sezioni (sostituisce h(), esc(), ls() duplicati negli originali).
// Regola d'oro: i dati degli utenti entrano nella pagina SOLO come testo (textContent), mai come HTML.

/** Crea un elemento. attr: {class, testo, on: {click: fn}, ...attributi}. figli: nodi o stringhe. */
export function el(tag, attr = {}, ...figli) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attr || {})) {
    if (v === undefined || v === null || v === false) continue;
    if (k === "class") e.className = v;
    else if (k === "testo") e.textContent = v;
    else if (k === "on") for (const [ev, fn] of Object.entries(v)) e.addEventListener(ev, fn);
    else if (k === "dataset") Object.assign(e.dataset, v);
    else if (k === "stile") Object.assign(e.style, v);
    else if (k === "valore") e.value = v;
    else if (k === "selezionato") e.checked = !!v;
    else e.setAttribute(k, v === true ? "" : String(v));
  }
  aggiungi(e, figli);
  return e;
}

function aggiungi(e, figli) {
  for (const f of figli.flat(Infinity)) {
    if (f === null || f === undefined || f === false) continue;
    e.append(f instanceof Node ? f : document.createTextNode(String(f)));
  }
}

/** Sostituisce il contenuto di un elemento. */
export function svuota(e, ...figli) { e.replaceChildren(); aggiungi(e, figli); return e; }

export const $ = (s, radice = document) => radice.querySelector(s);
export const $$ = (s, radice = document) => [...radice.querySelectorAll(s)];

// ----- Comunicazione con il server -----
let csrf = "";
export function impostaCSRF(t) { csrf = t || ""; }
export function leggiCSRF() { return csrf; } // per le richieste lette a pezzi (assistente IA)

/** Chiama le API. Restituisce il JSON; in caso di errore lancia Error con il messaggio del server. */
export async function api(percorso, { metodo, dati, corpo, tipo } = {}) {
  const m = metodo || (dati !== undefined || corpo !== undefined ? "POST" : "GET");
  const o = { method: m, credentials: "same-origin", headers: {} };
  if (m !== "GET") o.headers["X-CSRF"] = csrf;
  if (dati !== undefined) { o.headers["Content-Type"] = "application/json"; o.body = JSON.stringify(dati); }
  else if (corpo !== undefined) { o.headers["Content-Type"] = tipo || "application/octet-stream"; o.body = corpo; }
  // Limite di tempo (2026-11-08): con il server bloccato o il PC in standby la richiesta non resta appesa per sempre.
  const ctl = typeof AbortController === "function" ? new AbortController() : null;
  const limite = ctl ? setTimeout(() => ctl.abort(), m === "GET" ? 20000 : 90000) : 0;
  if (ctl) o.signal = ctl.signal;
  let r;
  try { r = await fetch("/api/v1" + percorso, o); }
  catch { clearTimeout(limite); throw Object.assign(new Error("Gaster Bless non risponde: controlla che il server sia acceso e di essere sul Wi-Fi di casa."), { stato: 0 }); }
  let j = null;
  try { j = await r.json(); } catch { /* risposta senza JSON */ }
  clearTimeout(limite);
  if (!r.ok) {
    const e = new Error((j && j.errore) || "Errore " + r.status);
    e.stato = r.status;
    e.dati = j;
    if (r.status === 401) document.dispatchEvent(new CustomEvent("gasterbless:sessione-scaduta"));
    if (j && j.codice === "monete_insufficienti") document.dispatchEvent(new CustomEvent("gasterbless:povero")); // immagine «povero»
    throw e;
  }
  return j;
}

/** Caricamento di un file con barra di avanzamento. */
export function carica(percorso, file, avanzamento, metodo = "PUT", nomeFile = "") {
  return new Promise((ok, ko) => {
    const x = new XMLHttpRequest();
    x.open(metodo, "/api/v1" + percorso);
    x.setRequestHeader("X-CSRF", csrf);
    if (nomeFile) x.setRequestHeader("X-Nome-File", encodeURIComponent(nomeFile)); // nome originale (solo da mostrare)
    x.setRequestHeader("Content-Type", file.type || "application/octet-stream");
    x.upload.onprogress = (e) => { if (e.lengthComputable && avanzamento) avanzamento(e.loaded / e.total); };
    x.onload = () => {
      let j = null; try { j = JSON.parse(x.responseText); } catch { /* vuota */ }
      x.status < 300 ? ok(j) : ko(new Error((j && j.errore) || "Caricamento non riuscito."));
    };
    x.onerror = () => ko(new Error("Connessione interrotta durante il caricamento."));
    x.send(file);
  });
}

// ----- Canale in tempo reale (WebSocket) con riconnessione automatica -----
export const tempoReale = (() => {
  let ws = null, tentativi = 0, timer = 0, aperto = false, nRich = 0;
  const ascoltatori = new Map(), attese = new Map(), coda = [];
  const stato = new EventTarget();
  function connetti() {
    clearTimeout(timer);
    const sock = ws = new WebSocket((location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/ws");
    ws.onopen = () => { tentativi = 0; aperto = true; stato.dispatchEvent(new Event("aperto")); while (coda.length) ws.send(coda.shift()); };
    ws.onmessage = (e) => {
      let m; try { m = JSON.parse(e.data); } catch { return; }
      if ((m.tipo === "risposta" || m.tipo === "errore") && m.id && attese.has(m.id)) {
        const [ok, ko] = attese.get(m.id); attese.delete(m.id);
        m.tipo === "risposta" ? ok(m.dati) : ko(new Error(m.dati));
        return;
      }
      if (m.tipo === "errore") avviso(String(m.dati), "errore");
      (ascoltatori.get(m.tipo) || []).forEach((f) => f(m.dati));
      (ascoltatori.get("*") || []).forEach((f) => f(m));
    };
    ws.onclose = () => {
      const eraAperto = aperto; aperto = false;
      if (eraAperto) stato.dispatchEvent(new Event("chiuso"));
      for (const [, [, ko]] of attese) ko(new Error("Connessione interrotta."));
      attese.clear();
      if (sock._fermo || ws !== sock) return; // chiuso apposta (ferma) o già sostituito: niente riconnessione
      timer = setTimeout(connetti, Math.min(15000, 1000 * 2 ** tentativi++));
    };
  }
  return {
    avvia() { if (!ws || ws.readyState > 1) connetti(); },
    ferma() { clearTimeout(timer); if (ws) { ws._fermo = true; ws.close(); } ws = null; },
    /** Ascolta un tipo di messaggio. Restituisce la funzione per smettere. */
    su(tipo, fn) {
      if (!ascoltatori.has(tipo)) ascoltatori.set(tipo, []);
      ascoltatori.get(tipo).push(fn);
      return () => ascoltatori.set(tipo, (ascoltatori.get(tipo) || []).filter((x) => x !== fn));
    },
    /** Invia un messaggio e attende la risposta del server. */
    chiedi(tipo, dati) {
      const id = "r" + (++nRich);
      const msg = JSON.stringify({ tipo, dati, id });
      return new Promise((ok, ko) => {
        attese.set(id, [ok, ko]);
        if (aperto) ws.send(msg); else coda.push(msg);
        setTimeout(() => { if (attese.has(id)) { attese.delete(id); ko(new Error("Il server non ha risposto.")); } }, 15000);
      });
    },
    get aperto() { return aperto; },
    stato,
  };
})();

// ----- Avvisi e finestre -----
export function avviso(testo, tipo = "", ms = 3500) {
  const box = $("#avvisi");
  // Lo stesso avviso già in vista non si ripete; al massimo 2 avvisi insieme (i più vecchi lasciano il posto),
  // così sul telefono non coprono la pagina (2026-11-09).
  for (const x of box.children) if (x.textContent === testo) { x.remove(); }
  while (box.children.length >= 2) box.firstElementChild.remove();
  const a = el("div", { class: "avviso " + tipo, testo, role: tipo === "errore" ? "alert" : "status" });
  box.append(a);
  setTimeout(() => a.remove(), ms);
}

/** Apre una finestra di dialogo. contenuto: nodo o funzione(chiudi) che restituisce un nodo. */
export function dialogo(titolo, contenuto, { largo = false, azioni = null } = {}) {
  let velo;
  const chiudi = () => { velo.remove(); document.removeEventListener("keydown", esc); };
  const esc = (e) => { if (e.key === "Escape" && velo === [...document.querySelectorAll(".velo")].pop()) chiudi(); }; // solo il dialogo in cima
  const corpo = typeof contenuto === "function" ? contenuto(chiudi) : contenuto;
  const d = el("div", { class: "dialogo" + (largo ? " largo-d" : ""), role: "dialog", "aria-modal": "true", "aria-label": titolo },
    el("h2", { testo: titolo }), corpo,
    azioni === null ? el("div", { class: "azioni" }, el("button", { testo: "Chiudi", on: { click: chiudi } })) : azioni(chiudi));
  velo = el("div", { class: "velo", on: { click: (e) => { if (e.target === velo) chiudi(); } } }, d);
  velo.__chiudi = chiudi;
  document.body.append(velo);
  document.addEventListener("keydown", esc);
  const primo = d.querySelector("input,select,textarea,button.primario");
  if (primo) setTimeout(() => primo.focus(), 30);
  return chiudi;
}

/** Chiude tutte le finestre di dialogo aperte (cambiando pagina con un collegamento interno: 2026-11-12). */
export function chiudiDialoghi() {
  document.querySelectorAll(".velo").forEach((v) => (typeof v.__chiudi === "function" ? v.__chiudi() : v.remove()));
}

/** Chiede conferma. Restituisce una Promise<boolean>. */
export function conferma(titolo, testo, etichetta = "Conferma", pericolo = false) {
  return new Promise((ok) => {
    dialogo(titolo, el("p", { testo }), {
      azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { testo: "Annulla", on: { click: () => { chiudi(); ok(false); } } }),
        el("button", { class: pericolo ? "pericolo" : "primario", testo: etichetta, on: { click: () => { chiudi(); ok(true); } } })),
    });
  });
}

/** Pulsante che chiede un secondo clic per confermare (azioni distruttive veloci). */
export function pulsanteDoppio(etichetta, conferma, azione, attr = {}) {
  let armato = false, t = 0;
  const b = el("button", { ...attr, testo: etichetta, on: { click: () => {
    if (!armato) { armato = true; b.textContent = conferma; t = setTimeout(() => { armato = false; b.textContent = etichetta; }, 4000); return; }
    clearTimeout(t); armato = false; b.textContent = etichetta; azione();
  } } });
  return b;
}

// ----- Tema chiaro/scuro -----
export function applicaTema(t) {
  if (t === "chiaro" || t === "scuro") document.documentElement.dataset.tema = t;
  else delete document.documentElement.dataset.tema;
  try { localStorage.setItem("gasterbless.tema", t || ""); } catch { /* memoria non disponibile */ }
}
export function temaSalvato() { try { return localStorage.getItem("gasterbless.tema") || ""; } catch { return ""; } }

// ----- Aspetto: dispositivo (layout) e palette di colori -----
const leggi = (k) => { try { return localStorage.getItem(k) || ""; } catch { return ""; } };
const scrivi = (k, v) => { try { localStorage.setItem(k, v); } catch { /* memoria non disponibile */ } };

/** I tre layout. "auto" (solo per chi non ha scelto, o per le prove) segue la larghezza della finestra. */
export const DISPOSITIVI = [
  { id: "pc", t: "PC", i: "🖥", d: "Menu laterale completo, pagine larghe, più cose a colpo d'occhio." },
  { id: "tablet", t: "Tablet", i: "📱", d: "Menu a icone sottile, testo e pulsanti più grandi da toccare." },
  { id: "cellulare", t: "Cellulare", i: "📞", d: "Barra in basso, una colonna sola, pulsanti grandi." },
];
export function dispositivoSalvato() { return leggi("gasterbless.dispositivo"); }
export function applicaDispositivo(d, salva = true) {
  const r = document.documentElement;
  if (DISPOSITIVI.some((x) => x.id === d)) r.dataset.dispositivo = d; else delete r.dataset.dispositivo;
  if (salva) scrivi("gasterbless.dispositivo", d || "");
}
/** Il dispositivo che sembra più probabile, da proporre come prima scelta. */
export function dispositivoSuggerito() {
  const tocco = matchMedia("(pointer: coarse)").matches;
  const lato = Math.min(screen.width || innerWidth, screen.height || innerHeight);
  if (tocco && lato < 600) return "cellulare";
  if (tocco) return "tablet";
  return innerWidth <= 600 ? "cellulare" : "pc";
}
/** Layout effettivo: quello scelto, o in automatico secondo la larghezza della finestra. */
export function layoutAttuale() {
  const d = document.documentElement.dataset.dispositivo;
  if (d) return d;
  return innerWidth <= 800 ? "cellulare" : "pc";
}

/** Palette di colori. Ognuna è chiara o scura: la scheda del personaggio segue di conseguenza. */
export const PALETTE = [
  { id: "", t: "Automatica", tema: "", c: ["#f1e9d6", "#15120e", "#8a1c1c"], d: "Pergamena di giorno, Notte di Gaster Bless se il sistema è scuro." },
  { id: "pergamena", t: "Pergamena", tema: "chiaro", c: ["#f1e9d6", "#e6dbc0", "#8a1c1c"] },
  { id: "notte", t: "Notte di Gaster Bless", tema: "scuro", c: ["#15120e", "#221c15", "#b3202e"] },
  { id: "sangue", t: "Sangue", tema: "scuro", c: ["#140608", "#250c10", "#d4283a"] },
  { id: "nebbia", t: "Nebbia", tema: "chiaro", c: ["#e4e7ea", "#d3d8dd", "#4b5f7a"] },
  { id: "foresta", t: "Foresta di Selvascura", tema: "scuro", c: ["#0e1410", "#18221a", "#6f9b4e"] },
  { id: "contrasto", t: "Alto contrasto", tema: "scuro", c: ["#000000", "#111111", "#ffd400"] },
];
export function paletteSalvata() { return leggi("gasterbless.palette"); }
export function applicaPalette(id) {
  const p = PALETTE.find((x) => x.id === id) || PALETTE[0];
  const r = document.documentElement;
  if (p.id) r.dataset.palette = p.id; else delete r.dataset.palette;
  scrivi("gasterbless.palette", p.id);
  applicaTema(p.tema);
}

// ----- Date in italiano -----
const fmtData = new Intl.DateTimeFormat("it-IT", { weekday: "long", day: "numeric", month: "long", year: "numeric" });
const fmtOra = new Intl.DateTimeFormat("it-IT", { hour: "2-digit", minute: "2-digit" });
const fmtBreve = new Intl.DateTimeFormat("it-IT", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
/** "sabato 14 novembre 2026" */
export const dataLunga = (sec) => fmtData.format(new Date(sec * 1000));
/** "21:00" */
export const ora = (sec) => fmtOra.format(new Date(sec * 1000));
/** "sabato 14 novembre 2026, ore 21:00" */
export const dataOra = (sec) => dataLunga(sec) + ", ore " + ora(sec);
export const quando = (sec) => (sec ? fmtBreve.format(new Date(sec * 1000)) : "mai");

// ----- Esc chiude sempre ciò che è in primo piano (richiesta dell'utente, 2026-11-08) -----
// Vale per avvisi, immagini e video a tutto schermo, effetti dei dadi; le finestre di dialogo (.velo) e la Sanità
// mentale hanno già la loro gestione. Se il tasto arriva dentro la Mappa o la scheda (riquadri), la pagina interna chiude
// le sue finestre e, se non ne ha, lo inoltra qui con il messaggio «gasterbless:esc».
const IN_PRIMO_PIANO = [".benvenuto", ".saluto", ".povero", ".indebolimento", ".gelo-schermo", ".viaggio-avviso",
  ".effetto-dadi", ".lancio-dadi", ".sfortuna", ".pdf-sovrapposto", ".velo-menu", ".esci-mappa-grande", ".documento-mostrato"];
const SI_TOCCA = new Set(["benvenuto", "saluto", "indebolimento", "gelo-schermo", "viaggio-avviso"]); // si chiudono con un tocco (e puliscono da sé)

/** Chiude l'elemento a tutto schermo più in alto. Restituisce true se ha chiuso qualcosa. */
export function chiudiInCima() {
  if (document.querySelector(".velo")) return false; // c'è una finestra di dialogo: la chiude la sua gestione di Esc
  const tutti = [...document.querySelectorAll(IN_PRIMO_PIANO.join(","))].filter((x) => x.isConnected && !x.classList.contains("via"));
  if (!tutti.length) return false;
  const z = (x) => parseInt(getComputedStyle(x).zIndex, 10) || 0;
  const cima = tutti.reduce((a, b) => (z(b) >= z(a) ? b : a));
  if (typeof cima.__chiudi === "function") { cima.__chiudi(); return true; } // benvenuto, saluto: chiusura con dissolvenza
  const cls = [...cima.classList].find((c) => SI_TOCCA.has(c));
  if (cls) cima.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  if (cima.isConnected) cima.remove();
  return true;
}
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && chiudiInCima()) { e.preventDefault(); e.stopImmediatePropagation(); }
}, true);
window.addEventListener("message", (e) => {
  if ((e.origin === location.origin || e.origin === "null") && e.data && e.data.type === "gasterbless:esc") chiudiInCima();
});
/** Mette il fuoco su un elemento a tutto schermo appena comparso: così Esc arriva qui anche se prima si era toccata la
 *  mappa o la scheda (riquadri). */
export function fuocoInPrimoPiano(el) {
  try { el.setAttribute("tabindex", "-1"); el.focus({ preventScroll: true }); } catch { /* ignora */ }
}

// Sessione scelta (2026-11-08, bug segnalato dal Master): passando da una sezione all'altra l'indirizzo perdeva «?s=…» e
// Mappa, Iniziativa e Avvisi tornavano alla prima sessione dell'elenco (anche di un altro gruppo). Ora si ricorda l'ultima
// sessione scelta (per questa scheda del browser) e si usa quando l'indirizzo non ne indica una.
const CHIAVE_SESSIONE = "gasterbless.sessione";
export function ricordaSessione(id) { try { if (id) sessionStorage.setItem(CHIAVE_SESSIONE, id); } catch { /* ignora */ } }
export function sessioneScelta(sessioni) {
  const richiesta = new URLSearchParams(location.search).get("s");
  let ricordata = null;
  try { ricordata = sessionStorage.getItem(CHIAVE_SESSIONE); } catch { /* ignora */ }
  const s = sessioni.find((x) => x.id === richiesta) || sessioni.find((x) => x.id === ricordata) || sessioni[0];
  ricordaSessione(s && s.id);
  return s;
}
