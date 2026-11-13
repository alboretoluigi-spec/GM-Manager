// Simulatore di dadi (pannello riusabile) ed effetti dei tiri durante gli scontri.
// I dadi li tira il server: durante uno scontro il risultato arriva a tutti i partecipanti, e un
// 20 naturale sul d20 mostra "CRITICO" su tutti gli schermi, un 1 fa tremare lo schermo con "OOOPS".
import { el, svuota, api, avviso, tempoReale } from "./ui.js";
import { SET_DADI, caricaTre, pronto, webglOk, lancia3D, fermaLancio3D, suonoAcceso, impostaSuono } from "./dadi3d.js";

// colore del mio set di dadi («#corpo/#numeri», dal server: GET /io, PUT /io/dadi); vale anche per i tiri che il
// server non decora (scheda esterna)
let mioColore = "";
async function leggiMioColore() { try { mioColore = (await api("/io")).coloreDadi || ""; } catch { /* resta il predefinito */ } return mioColore; }

const FACCE = [4, 6, 8, 10, 12, 20, 100];

/** Pannello dei dadi. sessione: facoltativa (senza, il server usa lo scontro in corso, se c'è). */
export function pannelloDadi({ sessione = "", master = false } = {}) {
  let pool = {}; // facce → quantità
  const formula = el("div", { class: "dadi-formula", "aria-live": "polite" });
  const mod = el("input", { type: "number", min: "-50", max: "50", valore: "0", class: "dadi-mod", "aria-label": "Modificatore" });
  const etich = el("input", { maxlength: "40", placeholder: "Es. Attacco con la spada (facoltativo)", "aria-label": "Descrizione del tiro" });
  const modo = el("select", { "aria-label": "Vantaggio o svantaggio" },
    el("option", { value: "normale", testo: "Normale" }), el("option", { value: "vantaggio", testo: "Vantaggio" }), el("option", { value: "svantaggio", testo: "Svantaggio" }));
  const segreto = master ? el("input", { type: "checkbox" }) : null;
  const risultati = el("div", { class: "dadi-risultati", role: "log" });

  const aggiornaFormula = () => {
    const parti = FACCE.filter((f) => pool[f]).map((f) => pool[f] + "d" + f);
    const m = parseInt(mod.value, 10) || 0;
    formula.textContent = parti.length ? parti.join(" + ") + (m ? (m > 0 ? " +" : " ") + m : "") : "Scegli i dadi";
  };
  mod.addEventListener("input", aggiornaFormula);

  async function tira(dadi) {
    const lista = dadi || FACCE.filter((f) => pool[f]).map((f) => ({ n: pool[f], facce: f }));
    if (!lista.length) { avviso("Scegli almeno un dado."); return; }
    try {
      const r = await tempoReale.chiedi("dadi.tira", { sessione, dadi: lista, mod: parseInt(mod.value, 10) || 0, modo: modo.value,
        etichetta: etich.value, segreto: !!(segreto && segreto.checked) });
      risultati.prepend(rigaRisultato(r, true));
      while (risultati.children.length > 12) risultati.lastChild.remove();
    } catch (e) { avviso(e.message, "errore"); }
  }

  // scelta del numero di dadi dello stesso tipo (richiesta dell'utente, 2026-11-12): un clic sul tipo apre i numeri
  const scelta = el("div", { class: "dadi-quanti", hidden: true, role: "group" });
  const conta = {};
  const segna = () => FACCE.forEach((f) => { conta[f].textContent = pool[f] ? "×" + pool[f] : ""; conta[f].parentElement.classList.toggle("scelto", !!pool[f]); });
  function chiediQuanti(f) {
    const imposta = (n) => { if (n > 0) pool[f] = Math.min(20, n); else delete pool[f]; segna(); aggiornaFormula(); scelta.hidden = true; };
    const altro = el("input", { type: "number", min: "1", max: "20", valore: String(pool[f] || ""), "aria-label": "Numero di d" + f + " (fino a 20)", placeholder: "…" });
    altro.addEventListener("keydown", (e) => { if (e.key === "Enter") imposta(parseInt(altro.value, 10) || 0); });
    svuota(scelta, el("span", { class: "dadi-quanti-titolo", testo: "Quanti d" + f + "?" }),
      [1, 2, 3, 4, 5, 6, 8, 10].map((n) => el("button", { class: "piccolo" + (pool[f] === n ? " primario" : ""), testo: String(n), "aria-label": n + " d" + f, on: { click: () => imposta(n) } })),
      altro, el("button", { class: "piccolo", testo: "OK", on: { click: () => imposta(parseInt(altro.value, 10) || 0) } }),
      pool[f] ? el("button", { class: "piccolo", testo: "Togli", on: { click: () => imposta(0) } }) : null,
      el("button", { class: "piccolo", testo: "✕", "aria-label": "Chiudi la scelta", on: { click: () => { scelta.hidden = true; } } }));
    scelta.hidden = false;
    scelta.querySelector("button").focus();
  }
  const bottoni = el("div", { class: "dadi-bottoni" }, FACCE.map((f) => el("button", { class: "dado", "aria-label": "d" + f + ": scegli il numero di dadi",
    on: { click: () => chiediQuanti(f) } }, "d" + f, (conta[f] = el("span", { class: "dado-conta" })))));
  aggiornaFormula();
  segna();
  // il mio set di dadi e il suono
  const selSet = el("select", { "aria-label": "Colore dei miei dadi" }, SET_DADI.map((x) => el("option", { value: x.c, testo: x.nome })), el("option", { value: "su-misura", testo: "Su misura…" }));
  const cCorpo = el("input", { type: "color", "aria-label": "Colore dei dadi", valore: "#7a1616" });
  const cNum = el("input", { type: "color", "aria-label": "Colore dei numeri", valore: "#f4e4bc" });
  const campione = el("span", { class: "dadi-campione", "aria-hidden": "true", testo: "20" });
  const suMisura = el("span", { class: "riga", hidden: true }, cCorpo, cNum);
  const mostraSet = (c) => {
    const [a, b] = (c || SET_DADI[0].c).split("/");
    campione.style.background = a; campione.style.color = b;
    const noto = SET_DADI.some((x) => x.c === c);
    selSet.value = noto || !c ? (c || SET_DADI[0].c) : "su-misura";
    suMisura.hidden = noto || !c;
    cCorpo.value = a; cNum.value = b;
  };
  const salvaSet = async (c) => {
    mostraSet(c);
    try { await api("/io/dadi", { metodo: "PUT", dati: { colore: c } }); mioColore = c; } catch (e) { avviso(e.message, "errore"); }
  };
  selSet.addEventListener("change", () => { if (selSet.value === "su-misura") { suMisura.hidden = false; return; } salvaSet(selSet.value); });
  const daMisura = () => salvaSet(cCorpo.value.toLowerCase() + "/" + cNum.value.toLowerCase());
  cCorpo.addEventListener("change", daMisura); cNum.addEventListener("change", daMisura);
  leggiMioColore().then(mostraSet);
  const suono = el("input", { type: "checkbox", checked: suonoAcceso() });
  suono.addEventListener("change", () => impostaSuono(suono.checked));
  const auto = el("input", { type: "checkbox", checked: dadiAutomatici() });
  auto.addEventListener("change", () => impostaDadiAutomatici(auto.checked));
  return el("div", { class: "dadi" },
    bottoni, scelta,
    el("div", { class: "riga-flex dadi-riga" }, formula,
      el("label", { class: "riga" }, "Mod.", mod), modo,
      el("button", { testo: "Azzera", on: { click: () => { pool = {}; mod.value = "0"; aggiornaFormula(); } } })),
    el("div", { class: "riga-flex dadi-riga" }, etich,
      segreto ? el("label", { class: "riga", title: "Il risultato lo vedi solo tu" }, segreto, el("span", { testo: "Segreto" })) : null,
      el("button", { class: "primario dadi-tira", testo: "🎲 Tira", on: { click: () => tira() } }),
      el("button", { testo: "d20 veloce", title: "Tira subito 1d20 con modificatore e vantaggio scelti", on: { click: () => tira([{ n: 1, facce: 20 }]) } })),
    el("div", { class: "riga-flex dadi-riga dadi-aspetto" },
      el("label", { class: "riga" }, campione, el("span", { testo: "I miei dadi" }), selSet), suMisura,
      el("label", { class: "riga", title: "Il rumore dei dadi che rotolano, su questo dispositivo" }, suono, el("span", { testo: "🔊 Suono" })),
      el("label", { class: "riga", title: "Attacchi, danni, tiri salvezza degli incantesimi, cure e Dadi Vita: prima dell'esito si vedono i dadi che rotolano (su questo dispositivo)" },
        auto, el("span", { testo: "🎲 Dadi 3D anche nei tiri automatici" }))),
    el("p", { class: "tenue dadi-nota", testo: sessione || master ? "Durante uno scontro il tiro lo vedono tutti i partecipanti; fuori dallo scontro vedi anche i tiri dei giocatori." : "Durante uno scontro il tiro lo vedono tutti i partecipanti; fuori dallo scontro tu e il Master." }),
    risultati);
}

/** Editor della tabella della sfortuna (solo Master): un effetto per riga, al massimo 100. */
export function editorSfortuna() {
  const area = el("textarea", { rows: "12", "aria-label": "Effetti della tabella della sfortuna, uno per riga", class: "sfortuna-area" });
  const stato = el("p", { class: "tenue" });
  const carica = async () => {
    const t = await api("/sfortuna");
    area.value = t.effetti.join("\n");
    stato.textContent = (t.personalizzata ? "Tabella personalizzata: " : "Tabella predefinita di Gaster Bless: ") + t.effetti.length + " effetti. Il d100 sceglie la riga in proporzione (con 100 righe, una per numero).";
  };
  const salva = async (righe) => {
    try { await api("/sfortuna", { metodo: "PUT", dati: { effetti: righe } }); avviso("Tabella salvata."); await carica(); }
    catch (e) { avviso(e.message, "errore"); }
  };
  const box = el("details", { class: "scheda-ui" }, el("summary", { testo: "🃏 Tabella della sfortuna (solo Master)" }),
    el("p", { class: "tenue", testo: "I giocatori non vedono mai l'elenco: vedono solo l'effetto uscito. Scrivi un effetto per riga (massimo 100)." }),
    area, stato,
    el("div", { class: "riga-flex" },
      el("button", { class: "primario", testo: "Salva la tabella", on: { click: () => salva(area.value.split("\n")) } }),
      el("button", { testo: "Torna alla tabella predefinita", on: { click: () => salva([]) } })));
  box.addEventListener("toggle", () => { if (box.open && !area.value) carica().catch((e) => avviso(e.message, "errore")); });
  return box;
}

/** Riga con il dettaglio di un tiro (dadi tenuti, scartati, totale). */
export function rigaRisultato(r, mio) {
  const dadi = r.dadi.map((g) => el("span", { class: "dadi-gruppo" }, g.n + "d" + g.facce + ": ",
    g.valori.map((v, i) => el("span", { class: "dadi-v" + (g.facce === 20 && v === 20 ? " venti" : g.facce === 20 && v === 1 ? " uno" : "") },
      String(v), g.scarto && g.scarto[i] != null ? el("s", { class: "tenue", testo: " " + g.scarto[i] }) : null))));
  return el("div", { class: "dadi-ris" + (r.critico ? " critico" : r.fallimento ? " fallimento" : "") },
    el("b", { class: "dadi-tot", testo: String(r.totale) }),
    el("div", {},
      el("div", { testo: (mio ? "" : r.chi + ": ") + (r.etichetta ? r.etichetta + " · " : "") + r.formula + (r.segreto ? " · segreto" : r.scontro ? " · visto da tutti" : "") }),
      el("div", { class: "tenue" }, dadi, r.mod ? " " + (r.mod > 0 ? "+" : "") + r.mod : "")));
}

/** Effetti dei tiri in scontro, attivi in tutta l'app (si installano una volta all'avvio). */
let installati = false;
export function installaEffettiDadi(io) {
  if (installati) return;
  installati = true;
  // dadi 3D: la libreria si carica appena l'app è ferma, così il primo tiro è già pronto
  if (webglOk() && !riduci()) setTimeout(() => { caricaTre(); leggiMioColore(); }, 2500);
  tempoReale.su("dadi.risultato", (r) => {
    if (!r) return;
    const mio = io() && r.chiId === io().id;
    // il Master vede anche i tiri dei giocatori fuori dallo scontro (2026-11-12)
    if (!mio && (r.scontro || r.scheda || (io() && io().ruolo === "master"))) avviso("🎲 " + r.chi + ": " + (r.etichetta ? r.etichetta + " · " : "") + r.formula + " = " + r.totale, "", 5000);
    // Lancio grafico al centro dello schermo per ogni tiro che questo utente vede (richiesta dell'utente,
    // 2026-11-06); gli effetti CRITICO/OOOPS e il tasto della sfortuna arrivano quando i dadi si fermano.
    const fermi = mostraLancio(r, mio);
    if (r.scontro && !r.segreto) {
      if (r.critico) setTimeout(() => effetto("critico", r.chi), fermi);
      else if (r.fallimento) setTimeout(() => effetto("ooops", r.chi), fermi);
    }
    // Solo chi ha fatto 1 (giocatore, in scontro) riceve il tasto "Che sfortuna".
    if (mio && r.sfortuna && r.id) setTimeout(() => tastoSfortuna(r.id), fermi + 700);
  });
  tempoReale.su("dadi.sfortuna", (d) => { if (d && d.effetto) cartaSfortuna(d, io() && d.chiId === io().id); });
}

// Riquadro con il tasto: resta finché il giocatore non lo preme (o lo chiude).
function tastoSfortuna(tiro) {
  document.querySelectorAll(".sfortuna-tasto").forEach((x) => x.remove());
  const chiudi = () => box.remove();
  const tasto = el("button", { class: "primario sfortuna-bottone", testo: "Che sfortuna", on: { click: async () => {
    tasto.disabled = true;
    try { await tempoReale.chiedi("dadi.sfortuna", { tiro }); chiudi(); }
    catch (e) { avviso(e.message, "errore"); chiudi(); }
  } } });
  const box = el("div", { class: "sfortuna sfortuna-tasto", role: "dialog", "aria-label": "Che sfortuna" },
    el("div", { class: "sfortuna-titolo", testo: "OOOPS!!!" }),
    el("p", { testo: "Hai fatto 1 sul d20. Scopri cosa ti succede:" }),
    tasto,
    el("button", { class: "piccolo sfortuna-chiudi", testo: "✕", "aria-label": "Chiudi senza tirare", on: { click: chiudi } }));
  document.body.append(box);
  tasto.focus();
}

// L'effetto estratto: lo vedono tutti i partecipanti; si chiude a mano (va letto con calma).
function cartaSfortuna(d, mio) {
  document.querySelectorAll(".sfortuna-effetto").forEach((x) => x.remove());
  const box = el("div", { class: "sfortuna sfortuna-effetto", role: "alertdialog", "aria-label": "Effetto della sfortuna" },
    el("div", { class: "sfortuna-titolo", testo: "Che sfortuna!" }),
    el("div", { class: "tenue", testo: mio ? "Ti succede questo:" : d.chi + " ha tirato sulla tabella della sfortuna:" }),
    el("p", { class: "sfortuna-testo", testo: d.effetto }),
    el("button", { class: "primario", testo: "Ho capito", on: { click: () => box.remove() } }));
  document.body.append(box);
}

function effetto(tipo, chi) {
  document.querySelectorAll(".effetto-dadi").forEach((x) => x.remove());
  const testo = tipo === "critico" ? "CRITICO" : "OOOPS!!!";
  const o = el("div", { class: "effetto-dadi " + tipo, role: "alert", "aria-label": testo + " di " + chi },
    el("span", { class: "effetto-testo", testo }), el("span", { class: "effetto-chi", testo: chi }));
  document.body.append(o);
  if (tipo === "ooops") {
    const app = document.getElementById("app");
    app.classList.remove("trema"); void app.offsetWidth; app.classList.add("trema");
    setTimeout(() => app.classList.remove("trema"), 2000);
    if (navigator.vibrate) navigator.vibrate([120, 60, 120, 60, 200]);
  }
  setTimeout(() => o.remove(), 2600);
}

/* ===== Simulatore grafico dei dadi (richiesta dell'utente, 2026-11-06) =====
   I dadi (forme diverse per d4…d100) entrano rotolando dai bordi, i numeri scorrono e si fermano sul valore
   tirato dal server; poi compare il totale in grande, che resta per alcuni secondi. Non blocca i clic. */
const FORME = {
  4: "50,6 95,88 5,88",
  6: "12,12 88,12 88,88 12,88",
  8: "50,4 94,50 50,96 6,50",
  10: "50,4 92,40 50,96 8,40",
  12: "50,4 94,36 77,92 23,92 6,36",
  20: "50,3 92,27 92,73 50,97 8,73 8,27",
  100: "50,4 92,40 50,96 8,40",
};
const COLORI = { 4: "#2f7d5b", 6: "#2f5fa8", 8: "#7d3fa8", 10: "#a8562f", 12: "#a83f6b", 20: "#8a1c1c", 100: "#5a6470" };
const riduci = () => matchMedia("(prefers-reduced-motion: reduce)").matches;
let lancioT = 0;

function svgDado(facce, testo) {
  const ns = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(ns, "svg");
  svg.setAttribute("viewBox", "0 0 100 100");
  svg.setAttribute("class", "dado-svg");
  const p = document.createElementNS(ns, "polygon");
  p.setAttribute("points", FORME[facce] || FORME[6]);
  p.setAttribute("fill", COLORI[facce] || "#444");
  p.setAttribute("stroke", "#f4e4bc");
  p.setAttribute("stroke-width", "3");
  p.setAttribute("stroke-linejoin", "round");
  const t = document.createElementNS(ns, "text");
  t.setAttribute("x", "50"); t.setAttribute("y", facce === 4 ? "70" : "58");
  t.setAttribute("text-anchor", "middle"); t.setAttribute("dominant-baseline", "middle");
  t.setAttribute("class", "dado-num");
  t.textContent = testo;
  svg.append(p, t);
  return { svg, t };
}

// chiusura del riquadro del lancio: clic in un punto qualsiasi, Esc, o dopo un po' (richiesta dell'utente, 2026-11-12)
let chiudiLancio = null;
let dopoChiusura = null; // tiri automatici (fase D2): l'esito si apre quando il riquadro dei dadi si chiude
function chiudibile(box, dopo) {
  const chiudi = () => {
    clearTimeout(lancioT);
    document.removeEventListener("keydown", esc);
    if (box.classList.contains("lancio3d")) fermaLancio3D();
    box.classList.add("svanisce"); setTimeout(() => box.remove(), 300);
    if (chiudiLancio === chiudi) chiudiLancio = null;
    const f = dopoChiusura; dopoChiusura = null;
    if (f) f();
  };
  const esc = (e) => { if (e.key === "Escape") chiudi(); };
  box.addEventListener("click", chiudi);
  document.addEventListener("keydown", esc);
  lancioT = setTimeout(chiudi, dopo);
  chiudiLancio = chiudi;
}

/* ===== Dadi 3D nei tiri automatici (fase D2, 2026-11-12, scelta dell'utente) =====
   Attacchi, danni, tiri salvezza degli incantesimi, cure e Dadi Vita li tira il server; prima dell'esito la pagina di chi
   ha confermato mostra i dadi che rotolano e si fermano sui valori tirati. Si spegne nel pannello dei dadi (per
   dispositivo); con «riduci movimento» i dadi compaiono fermi. */
const CHIAVE_AUTO = "gasterbless-dadi-automatici";
export const dadiAutomatici = () => { try { return localStorage.getItem(CHIAVE_AUTO) !== "0"; } catch { return true; } };
export const impostaDadiAutomatici = (on) => { try { localStorage.setItem(CHIAVE_AUTO, on ? "1" : "0"); } catch { /* niente */ } };
export function chiudiLancioDadi() { if (chiudiLancio) chiudiLancio(); }

/** Dadi di un tiro automatico. dadi: [{facce, valori, scarto?}]; dopo(): quando il riquadro si chiude (da solo poco dopo
 *  che i dadi si fermano, con un clic o con Esc). Senza dadi o con l'interruttore spento chiama subito dopo(). */
export function lancioAutomatico({ dadi, totale, etichetta, formula, critico = false, fallimento = false, modo = "", colore = "", classe = "", chi = "", mio = true }, dopo = () => {}) {
  const gruppi = (dadi || []).filter((g) => g && g.facce && g.valori && g.valori.length)
    .map((g) => ({ n: g.valori.length, facce: g.facce, valori: g.valori, scarto: g.scarto }));
  if (!dadiAutomatici() || !gruppi.length) { dopo(); return false; }
  let fatto = false;
  const una = () => { if (!fatto) { fatto = true; dopo(); } };
  // colore e classe di chi tira (esito del server, dadi_aspetto.go): gli altri vedono i suoi dadi (2026-11-13)
  const ms = mostraLancio({ dadi: gruppi, totale, etichetta, formula, critico, fallimento, modo, colore, classe, chi }, mio);
  dopoChiusura = una;
  setTimeout(() => { if (dopoChiusura === una) chiudiLancioDadi(); else una(); }, ms + 1600);
  return true;
}

/** Mostra il lancio; restituisce dopo quanti millisecondi i dadi sono fermi. */
export function mostraLancio(r, mio) {
  if (!r || !r.dadi) return 0;
  if (chiudiLancio) chiudiLancio();
  document.querySelectorAll(".lancio-dadi").forEach((x) => x.remove());
  clearTimeout(lancioT);
  if (pronto() && !riduci() && webglOk()) return lancio3D(r, mio);
  if (!riduci() && webglOk()) caricaTre(); // il prossimo sarà in 3D
  const statico = riduci();
  const durata = statico ? 0 : 1300;
  const dadi = [];
  r.dadi.forEach((g) => (g.valori || []).forEach((v, i) => {
    const sc = g.scarto && g.scarto[i] != null ? g.scarto[i] : null;
    dadi.push({ facce: g.facce, v, scarto: false });
    if (sc != null) dadi.push({ facce: g.facce, v: sc, scarto: true });
  }));
  const piano = el("div", { class: "lancio-piano" });
  const vis = dadi.slice(0, 12);
  vis.forEach((d, i) => {
    const { svg, t } = svgDado(d.facce, statico ? String(d.v) : "?");
    const box = el("div", { class: "lancio-dado" + (d.scarto ? " scartato" : "") + (d.facce === 20 && d.v === 20 && !d.scarto ? " venti" : "") + (d.facce === 20 && d.v === 1 && !d.scarto ? " uno" : "") },
      svg, el("span", { class: "lancio-tipo", testo: "d" + d.facce }));
    if (!statico) {
      const da = (i % 2 ? 1 : -1) * (180 + Math.random() * 160), alto = -120 - Math.random() * 140;
      box.animate([
        { transform: `translate(${da}px, ${alto}px) rotate(${(Math.random() * 2 - 1) * 720}deg) scale(.6)`, opacity: 0 },
        { offset: .55, transform: `translate(${da * -0.15}px, 14px) rotate(${(Math.random() * 2 - 1) * 90}deg) scale(1.08)`, opacity: 1 },
        { offset: .8, transform: "translate(0, -8px) rotate(-6deg) scale(1)" },
        { transform: "translate(0,0) rotate(0) scale(1)", opacity: 1 },
      ], { duration: durata, easing: "cubic-bezier(.2,.8,.3,1)", fill: "both", delay: i * 60 });
      const fine = performance.now() + durata + i * 60;
      const giro = () => { if (performance.now() >= fine) { t.textContent = String(d.v); return; } t.textContent = String(1 + Math.floor(Math.random() * d.facce)); setTimeout(giro, 70); };
      giro();
    }
    piano.append(box);
  });
  const extra = dadi.length > vis.length ? el("div", { class: "lancio-extra", testo: "+" + (dadi.length - vis.length) + " dadi" }) : null;
  const tot = el("div", { class: "lancio-totale" + (r.critico ? " critico" : r.fallimento ? " fallimento" : ""), testo: String(r.totale) });
  const det = el("div", { class: "lancio-dettaglio", testo: r.formula });
  const nota = r.critico ? "20 naturale!" : r.fallimento ? "1 naturale" : r.modo && r.modo !== "normale" ? r.modo : "";
  const box = el("div", { class: "lancio-dadi", role: "status", "aria-live": "polite" },
    el("div", { class: "lancio-chi", testo: (mio ? "Il tuo tiro" : r.chi) + (r.etichetta ? " · " + r.etichetta : "") + (r.segreto ? " · segreto" : "") }),
    piano, extra, el("div", { class: "lancio-esito" }, tot, el("div", {}, det, nota ? el("div", { class: "lancio-nota", testo: nota }) : null)));
  document.body.append(box);
  const fermi = durata + Math.min(vis.length, 12) * 60;
  if (!statico) { tot.style.opacity = "0"; setTimeout(() => { tot.style.opacity = ""; tot.classList.add("appare"); }, fermi); }
  box.title = "Clic per chiudere";
  chiudibile(box, fermi + 20000);
  return fermi;
}

// lancio in 3D: dadi su tutto lo schermo, il totale compare quando si fermano
function lancio3D(r, mio) {
  const dadi = [];
  r.dadi.forEach((g) => (g.valori || []).forEach((v, i) => {
    dadi.push({ facce: g.facce, v, scarto: false });
    if (g.scarto && g.scarto[i] != null) dadi.push({ facce: g.facce, v: g.scarto[i], scarto: true });
  }));
  const vis = dadi.slice(0, 10);
  const tot = el("div", { class: "lancio-totale" + (r.critico ? " critico" : r.fallimento ? " fallimento" : ""), testo: String(r.totale) });
  const nota = r.critico ? "20 naturale!" : r.fallimento ? "1 naturale" : r.modo && r.modo !== "normale" ? r.modo : "";
  const esito = el("div", { class: "lancio3d-esito", hidden: true },
    el("div", { class: "lancio-chi", testo: (mio ? "Il tuo tiro" : r.chi) + (r.etichetta ? " · " + r.etichetta : "") + (r.segreto ? " · segreto" : "") }),
    el("div", { class: "lancio-esito" }, tot, el("div", {}, el("div", { class: "lancio-dettaglio", testo: r.formula }),
      nota ? el("div", { class: "lancio-nota", testo: nota }) : null,
      dadi.length > vis.length ? el("div", { class: "lancio-extra", testo: "+" + (dadi.length - vis.length) + " dadi non mostrati" }) : null)),
    el("div", { class: "lancio3d-chiudi tenue", testo: "Clic per chiudere" }));
  const box = el("div", { class: "lancio-dadi lancio3d", role: "status", "aria-live": "polite", "aria-label": "Tiro di " + (mio ? "te" : r.chi) + ": " + r.formula + " = " + r.totale }, esito);
  document.body.append(box);
  const fermi = lancia3D(box, vis, { colore: r.colore || (mio ? mioColore : ""), classe: r.classe || "" }, () => {
    esito.hidden = false; tot.classList.add("appare");
  });
  chiudibile(box, fermi + 20000);
  return fermi;
}
