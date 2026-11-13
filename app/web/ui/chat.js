// Messaggi privati in finestre separate (richiesta dell'utente, 2026-11-11).
// Il Master ha una finestra per ogni giocatore che gli scrive (per sessione), affiancate in basso a destra e sempre visibili in
// qualunque pagina dell'app: vede tutte le conversazioni insieme, ognuna con la sua cronologia, i non letti e la sua casella
// di risposta. Nessun messaggio di giocatori diversi finisce nella stessa finestra o nello stesso avviso.
// Quando un giocatore scrive al Master: suono di messaggio (generato dal browser, diverso dal corvo del turno), finestra
// aperta in primo piano, avviso col nome, titolo della scheda del browser che lampeggia e, se la pagina è in secondo piano
// e il Master l'ha permessa, notifica di sistema. Il giocatore riceve suono e avviso quando il Master risponde.
// Server: messaggi privati del tracker (internal/server/iniziativa.go: messaggio.invia, messaggio.letto, messaggio.nuovo,
// GET /api/v1/sessioni/{id}/messaggi), invariati.
import { el, api, avviso, tempoReale, quando } from "./ui.js";

const conv = new Map(); // "sessione|utente" → {sessione, utente, nome, titolo, messaggi, caricata, nonLetti, ridotta, nodo, bozza}
let dock = null, io = () => null;
const master = () => { const u = io(); return u && u.ruolo === "master"; };
const chiave = (s, u) => s + "|" + u;

/* ---------- suono e segnali ---------- */
let ctx = null;
export function suonoMessaggio() { // due note brevi, a volume moderato
  try {
    ctx = ctx || new (window.AudioContext || window.webkitAudioContext)();
    if (ctx.state === "suspended") ctx.resume();
    [[880, 0], [1320, 0.13]].forEach(([f, t]) => {
      const o = ctx.createOscillator(), g = ctx.createGain();
      o.type = "sine"; o.frequency.value = f;
      g.gain.setValueAtTime(0.0001, ctx.currentTime + t);
      g.gain.exponentialRampToValueAtTime(0.25, ctx.currentTime + t + 0.02);
      g.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + t + 0.35);
      o.connect(g).connect(ctx.destination);
      o.start(ctx.currentTime + t); o.stop(ctx.currentTime + t + 0.4);
    });
  } catch { /* audio non disponibile */ }
  if (navigator.vibrate) navigator.vibrate([120, 60, 120]);
}
let titoloOrig = "", lampo = 0;
function lampeggiaTitolo(n) {
  if (!titoloOrig) titoloOrig = document.title;
  clearInterval(lampo);
  if (!n) { document.title = titoloOrig; return; }
  let on = false;
  lampo = setInterval(() => { on = !on; document.title = on ? "💬 (" + n + ") messaggi" : titoloOrig; }, 1000);
}
const totaleNonLetti = () => [...conv.values()].reduce((t, c) => t + c.nonLetti, 0);
function notificaSistema(nome, testo) {
  try {
    if (document.visibilityState === "visible" || !("Notification" in window) || Notification.permission !== "granted") return;
    const n = new Notification("💬 " + nome, { body: testo.slice(0, 140), tag: "gasterbless-" + nome });
    n.onclick = () => { window.focus(); n.close(); };
  } catch { /* notifiche non disponibili */ }
}
document.addEventListener("visibilitychange", () => { if (document.visibilityState === "visible") segnaLettiVisibili(); });

/* ---------- finestre ---------- */
function contenitore() {
  if (!dock || !document.body.contains(dock)) {
    dock = el("div", { class: "chat-dock non-stampare", role: "region", "aria-label": "Conversazioni private" });
    document.body.append(dock);
  }
  return dock;
}
function disegna(c) {
  if (!c.nodo) {
    c.nodo = el("section", { class: "chat-fin", "aria-label": "Conversazione con " + c.nome });
    contenitore().append(c.nodo);
  }
  const lista = el("div", { class: "chat-lista", "aria-live": "polite" }, c.messaggi.length ? c.messaggi.map((m) =>
    el("div", { class: "msg" + (m.mio ? " mio" : "") }, el("div", { class: "tenue piccolo", testo: (m.mio ? "Tu" : m.daNome) + " · " + quando(m.quando) }), el("div", { testo: m.testo })))
    : el("p", { class: "tenue piccolo", testo: c.caricata ? "Nessun messaggio: solo voi due li vedete." : "Caricamento…" }));
  const scrivi = el("textarea", { maxlength: "1000", rows: "2", placeholder: master() ? "Rispondi a " + c.nome + "…" : "Messaggio al Master…", "aria-label": "Messaggio a " + c.nome });
  scrivi.value = c.bozza || "";
  scrivi.addEventListener("input", () => { c.bozza = scrivi.value; });
  const invia = async (btn) => {
    const t = scrivi.value.trim(); if (!t) return;
    if (btn) btn.disabled = true;
    try { await tempoReale.chiedi("messaggio.invia", { sessione: c.sessione, testo: t, a: c.utente }); c.bozza = ""; scrivi.value = ""; }
    catch (e) { avviso(e.message, "errore"); }
    finally { if (btn) btn.disabled = false; }
  };
  scrivi.addEventListener("keydown", (e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); invia(); } });
  const conFuoco = c.nodo.contains(document.activeElement) && document.activeElement.tagName === "TEXTAREA";
  const arrivo = c.nodo.classList.contains("arrivo");
  c.nodo.className = "chat-fin" + (c.ridotta ? " ridotta" : "") + (c.nonLetti ? " nuovi" : "") + (arrivo ? " arrivo" : "");
  c.nodo.replaceChildren(
    el("header", { class: "chat-testa", on: { click: (e) => { if (e.target.closest("button")) return; c.ridotta = !c.ridotta; disegna(c); if (!c.ridotta) segnaLetti(c); } } },
      el("b", { testo: "💬 " + c.nome }), c.titolo ? el("span", { class: "tenue piccolo", testo: c.titolo }) : null,
      c.nonLetti ? el("span", { class: "distintivo", testo: String(c.nonLetti) }) : null, el("span", { class: "spazio" }),
      el("button", { class: "piccolo", title: c.ridotta ? "Apri" : "Riduci", "aria-label": c.ridotta ? "Apri la conversazione" : "Riduci la conversazione", testo: c.ridotta ? "▴" : "▾", on: { click: () => { c.ridotta = !c.ridotta; disegna(c); if (!c.ridotta) segnaLetti(c); } } }),
      el("button", { class: "piccolo", title: "Chiudi (la conversazione resta: si riapre al prossimo messaggio)", "aria-label": "Chiudi la conversazione con " + c.nome, testo: "✕", on: { click: () => { c.nodo.remove(); c.nodo = null; } } })),
    c.ridotta ? null : lista,
    c.ridotta ? null : el("div", { class: "chat-scrivi" }, scrivi, el("button", { class: "primario piccolo", testo: "Invia", on: { click: (e) => invia(e.target) } })));
  if (!c.ridotta) { lista.scrollTop = lista.scrollHeight; if (conFuoco) scrivi.focus(); }
}
async function carica(c) {
  if (c.caricata) return;
  try {
    const l = await api("/sessioni/" + encodeURIComponent(c.sessione) + "/messaggi");
    const nuovi = new Set(c.messaggi.map((m) => m.id));
    const storia = (l || []).filter((m) => (m.mio ? m.a : m.da) === c.utente && !nuovi.has(m.id));
    c.messaggi = [...storia, ...c.messaggi].sort((a, b) => a.quando - b.quando);
    c.nonLetti = c.messaggi.filter((m) => !m.mio && !m.letto).length;
  } catch { /* si riprova al prossimo messaggio */ }
  c.caricata = true;
}
function segnaLetti(c) {
  if (c.ridotta || !c.nodo || document.visibilityState !== "visible") return;
  c.messaggi.filter((m) => !m.mio && !m.letto).forEach((m) => { m.letto = true; tempoReale.chiedi("messaggio.letto", { sessione: c.sessione, messaggio: m.id }).catch(() => {}); });
  if (c.nonLetti) { c.nonLetti = 0; disegna(c); }
  lampeggiaTitolo(totaleNonLetti());
}
function segnaLettiVisibili() { conv.forEach((c) => segnaLetti(c)); }

/** Apre (o porta in primo piano) la finestra della conversazione con un giocatore in una sessione. */
export async function apriChat(sessione, utente, nome, titolo) {
  const k = chiave(sessione, utente);
  let c = conv.get(k);
  if (!c) { c = { sessione, utente, nome, titolo: titolo || "", messaggi: [], caricata: false, nonLetti: 0, ridotta: false, nodo: null, bozza: "" }; conv.set(k, c); }
  c.ridotta = false;
  disegna(c);
  await carica(c);
  disegna(c);
  segnaLetti(c);
}
/** Non letti per giocatore in una sessione (per l'elenco del tracker). */
export const nonLettiDi = (sessione, utente) => (conv.get(chiave(sessione, utente)) || { nonLetti: 0 }).nonLetti;

async function arrivato(d) {
  const m = d && d.messaggio; if (!m) return;
  const altro = m.mio ? m.a : m.da, nomeAltro = m.mio ? m.aNome : m.daNome;
  if (!master()) { // giocatore: la sua chat col Master resta nel tracker; qui suono e avviso quando il Master risponde
    if (!m.mio) { suonoMessaggio(); avviso("💬 " + m.daNome + ": " + m.testo.slice(0, 80), "", 6000); }
    return;
  }
  const k = chiave(d.sessione, altro);
  let c = conv.get(k);
  if (!c) { c = { sessione: d.sessione, utente: altro, nome: nomeAltro, titolo: d.titolo || "", messaggi: [], caricata: false, nonLetti: 0, ridotta: false, nodo: null, bozza: "" }; conv.set(k, c); }
  if (!c.messaggi.some((x) => x.id === m.id)) { c.messaggi.push(m); if (!m.mio) c.nonLetti++; }
  if (!m.mio) {
    c.ridotta = false;
    suonoMessaggio();
    notificaSistema(nomeAltro, m.testo);
    lampeggiaTitolo(totaleNonLetti());
  }
  disegna(c);
  if (!m.mio && c.nodo) { // notifica a video: la finestra di QUEL giocatore lampeggia (niente avvisi generici, che si accavallano)
    c.nodo.classList.remove("arrivo"); void c.nodo.offsetWidth; c.nodo.classList.add("arrivo");
    setTimeout(() => c.nodo && c.nodo.classList.remove("arrivo"), 3200);
  }
  if (!c.caricata) { await carica(c); disegna(c); }
  segnaLetti(c);
}

/* Messaggi non letti arrivati mentre il Master era scollegato: si aprono le finestre delle sessioni non concluse. */
async function nonLettiAlRientro() {
  try {
    const gruppi = await api("/gruppi");
    for (const g of gruppi) {
      const l = await api("/gruppi/" + g.id + "/sessioni");
      for (const s of l.filter((x) => x.stato !== "conclusa")) {
        const msg = await api("/sessioni/" + s.id + "/messaggi").catch(() => []);
        const per = new Map();
        (msg || []).filter((m) => !m.mio && !m.letto).forEach((m) => per.set(m.da, m.daNome));
        for (const [u, n] of per) { await apriChat(s.id, u, n, g.nome + " · " + s.titolo); }
        if (per.size) { suonoMessaggio(); lampeggiaTitolo(totaleNonLetti()); }
      }
    }
  } catch { /* niente */ }
}

/** Notifiche di sistema: le chiede il Master con un clic (i browser non le permettono senza un gesto). */
export function chiediNotifiche() {
  try { if ("Notification" in window && Notification.permission === "default") Notification.requestPermission(); } catch { /* niente */ }
}

let installato = false;
export function installaChat(funzioneIo) {
  if (installato) return;
  installato = true;
  io = funzioneIo;
  tempoReale.su("messaggio.nuovo", arrivato);
  tempoReale.su("benvenuto", () => { if (master()) nonLettiAlRientro(); });
  document.addEventListener("pointerdown", function primo() { if (master()) chiediNotifiche(); document.removeEventListener("pointerdown", primo); }, { once: true });
}
