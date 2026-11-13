// Biblioteca dell'assistente IA (2026-11-13, internal/server/biblioteca.go): i testi che le due identità usano per
// rispondere. Scaffale «canone» (Archivista: manuale dell'avventura, romanzi, compendi, Tomo interattivo) e scaffale
// «campagna» (Cronista: documenti della campagna del Master). Formati: EPUB, DOCX, TXT/MD, HTML, PDF (letto qui nel
// browser con pdf.js), testo OCR di Gaster Bless (.json), oppure un manuale già caricato in Gaster Bless.
// Qui anche le fonti sotto le risposte: i riferimenti [C1] / [H1] nel testo diventano pulsanti che aprono il brano.
import { el, svuota, api, avviso, conferma, dialogo } from "./ui.js";

const SCAFFALI = {
  canone: { t: "📜 Canone", d: "Testi ufficiali per l'Archivista del canone: manuale dell'avventura, romanzi, compendi, Tomo interattivo." },
  campagna: { t: "🏰 Campagna", d: "Documenti della tua campagna homebrew per il Cronista: appunti, cronache, ambientazione, varianti." },
};
const TIPO_PREDEF = { canone: "romanzo", campagna: "appunti" };
const numero = (n) => Number(n || 0).toLocaleString("it-IT");

/** Legge il testo di ogni pagina di un PDF (file scelto o indirizzo di un manuale). avanza(fatte, totale). */
export async function paginePDF(sorgente, avanza = () => {}) {
  const { caricaPdfJs } = await import("./pdfjs.js");
  const lib = await caricaPdfJs();
  const opz = { isEvalSupported: false, standardFontDataUrl: "/vendor/pdfjs/standard_fonts/" };
  if (typeof sorgente === "string") Object.assign(opz, { url: sorgente, disableAutoFetch: true, rangeChunkSize: 1 << 20 });
  else opz.data = new Uint8Array(await sorgente.arrayBuffer());
  const doc = await lib.getDocument(opz).promise;
  try {
    const testi = [];
    for (let n = 1; n <= doc.numPages; n++) {
      const pg = await doc.getPage(n);
      const t = await pg.getTextContent();
      // a capo dove pdf.js lo segnala: i titoli dei capitoli restano riconoscibili
      testi.push(t.items.map((x) => x.str + (x.hasEOL ? "\n" : " ")).join("").replace(/[ \t]+/g, " ").trim());
      pg.cleanup();
      avanza(n, doc.numPages);
    }
    return testi;
  } finally { doc.destroy(); }
}

/** Il dialogo «📚 Biblioteca». dopo(): richiamata quando l'elenco cambia. */
export async function apriBiblioteca({ scaffale = "canone", dopo = () => {} } = {}) {
  let dati = { fonti: [], tipi: {} }, manuali = [];
  const elenco = el("div", { class: "bib-elenco" });
  const stato = el("p", { class: "tenue piccolo", "aria-live": "polite" });
  const scelta = el("select", { "aria-label": "Scaffale" }, Object.entries(SCAFFALI).map(([k, v]) => el("option", { value: k, testo: v.t })));
  scelta.value = scaffale;
  const spiega = el("p", { class: "tenue piccolo" });

  async function ricarica() {
    [dati, manuali] = await Promise.all([api("/biblioteca"), api("/manuali").catch(() => [])]);
    disegna();
    dopo();
  }
  function disegna() {
    spiega.textContent = SCAFFALI[scelta.value].d;
    const lista = dati.fonti.filter((f) => f.dominio === scelta.value);
    svuota(elenco, lista.length ? lista.map(voce) : el("p", { class: "tenue", testo: "Nessun testo in questo scaffale." }));
    svuota(tipo, Object.entries(dati.tipi).map(([k, v]) => el("option", { value: k, testo: v })));
    tipo.value = TIPO_PREDEF[scelta.value];
    svuota(selManuale, el("option", { value: "", testo: "— manuale già caricato —" }), (Array.isArray(manuali) ? manuali : []).map((m) => el("option", { value: m.id, testo: m.nome })));
  }
  function voce(f) {
    const attiva = el("input", { type: "checkbox", selezionato: !!f.attiva, "aria-label": "Usa questo testo" });
    attiva.addEventListener("change", async () => {
      try { await api("/biblioteca/" + f.id, { metodo: "PATCH", dati: { attiva: attiva.checked } }); f.attiva = attiva.checked; dopo(); }
      catch (e) { avviso(e.message, "errore"); attiva.checked = !attiva.checked; }
    });
    const sposta = el("button", { class: "piccolo", testo: f.dominio === "canone" ? "→ 🏰" : "→ 📜", title: "Sposta nell'altro scaffale", on: { click: async () => {
      try { await api("/biblioteca/" + f.id, { metodo: "PATCH", dati: { dominio: f.dominio === "canone" ? "campagna" : "canone" } }); await ricarica(); } catch (e) { avviso(e.message, "errore"); }
    } } });
    const togli = el("button", { class: "piccolo pericolo", testo: "🗑", "aria-label": "Togli dalla biblioteca", on: { click: async () => {
      if (!(await conferma("Togliere il testo?", "«" + f.titolo + "» esce dalla biblioteca (il file originale e i manuali non cambiano).", "Togli", true))) return;
      try { await api("/biblioteca/" + f.id, { metodo: "DELETE" }); await ricarica(); } catch (e) { avviso(e.message, "errore"); }
    } } });
    return el("div", { class: "bib-voce" + (f.attiva ? "" : " spenta") },
      el("label", { class: "riga" }, attiva, el("span", {}, el("b", { testo: f.titolo }), el("span", { class: "tenue piccolo",
        testo: " · " + (dati.tipi[f.tipo] || f.tipo) + " · " + (f.capitoli ? numero(f.parti) + (f.manuale || /pagin/.test(f.file || "") ? " pagine o parti" : " capitoli o parti") : "senza capitoli") +
          " · " + numero(f.pezzi) + " brani · " + numero(f.caratteri) + " caratteri" + (f.manuale ? " · dal manuale di Gaster Bless" : "") }))),
      el("span", { class: "spazio" }), sposta, togli);
  }

  // ---- aggiungi ----
  const file = el("input", { type: "file", accept: ".epub,.pdf,.docx,.txt,.md,.html,.htm,.json", "aria-label": "File da aggiungere" });
  const titolo = el("input", { maxlength: "140", placeholder: "Titolo (se vuoto, dal file)", "aria-label": "Titolo" });
  const tipo = el("select", { "aria-label": "Tipo di testo" });
  const selManuale = el("select", { "aria-label": "Manuale già caricato" });
  const barra = el("progress", { max: "1", value: "0", hidden: true });
  const aggiungi = el("button", { class: "primario", testo: "Aggiungi" });
  const lavora = async (fn) => {
    aggiungi.disabled = true; barra.hidden = false; barra.removeAttribute("value");
    try { await fn(); await ricarica(); file.value = ""; titolo.value = ""; selManuale.value = ""; }
    catch (e) { avviso(e.message, "errore"); stato.textContent = ""; }
    finally { aggiungi.disabled = false; barra.hidden = true; }
  };
  const datiQ = (extra = {}) => new URLSearchParams({ titolo: titolo.value.trim(), tipo: tipo.value, dominio: scelta.value, ...extra });
  const mandaPagine = async (pagine, extra) => api("/biblioteca?" + datiQ({ formato: "pagine", ...extra }), { metodo: "POST", dati: { pagine } });
  const esito = (f) => { stato.textContent = "«" + f.titolo + "»: " + numero(f.pezzi) + " brani pronti" + (f.capitoli ? "." : " (capitoli non riconosciuti: le citazioni indicheranno la posizione nel testo)."); };
  aggiungi.addEventListener("click", () => {
    const f = file.files[0], m = selManuale.value;
    if (!f && !m) { avviso("Scegli un file o un manuale già caricato.", "errore"); return; }
    lavora(async () => {
      if (m) {
        stato.textContent = "Preparo il testo del manuale…";
        try { esito(await api("/biblioteca/manuale", { dati: { manuale: m, titolo: titolo.value.trim(), tipo: tipo.value, dominio: scelta.value } })); }
        catch (e) {
          if (!/non ha ancora il testo/i.test(e.message)) throw e;
          const pagine = await paginePDF("/api/v1/manuali/" + encodeURIComponent(m) + "/file", (n, t) => { barra.max = t; barra.value = n; stato.textContent = "Leggo il PDF del manuale: pagina " + n + " di " + t + "…"; });
          const nome = (manuali.find((x) => x.id === m) || {}).nome || "";
          if (!titolo.value.trim()) titolo.value = nome;
          esito(await mandaPagine(pagine, { manuale: m, file: nome + ".pdf" }));
        }
        return;
      }
      if (f.size > 96 * 1048576 && !/\.pdf$/i.test(f.name)) throw new Error("File troppo grande: massimo 96 MB.");
      if (/\.pdf$/i.test(f.name) || f.type === "application/pdf") {
        const pagine = await paginePDF(f, (n, t) => { barra.max = t; barra.value = n; stato.textContent = "Leggo il PDF: pagina " + n + " di " + t + "…"; });
        if (pagine.join("").replace(/\s/g, "").length < 200) throw new Error("Il PDF non contiene testo leggibile: probabilmente è scansionato. Serve il testo OCR (.json) del PDF.");
        esito(await mandaPagine(pagine, { file: f.name }));
        return;
      }
      stato.textContent = "Carico «" + f.name + "»…";
      esito(await api("/biblioteca?" + datiQ({ file: f.name }), { metodo: "POST", corpo: f, tipo: "application/octet-stream" }));
    });
  });

  // ---- cerca ----
  const cerca = el("input", { type: "search", placeholder: "Prova una ricerca: es. «Chi è il padre di Elara?»", "aria-label": "Cerca nei testi" });
  const risultati = el("div", { class: "bib-risultati" });
  const vaiCerca = async () => {
    const q = cerca.value.trim();
    if (!q) { svuota(risultati); return; }
    try {
      const l = await api("/biblioteca/cerca?" + new URLSearchParams({ q, dominio: scelta.value }));
      svuota(risultati, l.length ? l.map((p) => el("details", { class: "bib-brano" },
        el("summary", {}, el("b", { testo: p.titolo }), " — " + p.dove),
        el("p", { class: "bib-testo", testo: p.testo }), apriManuale(p))) : el("p", { class: "tenue", testo: "Nessun brano trovato in questo scaffale." }));
    } catch (e) { avviso(e.message, "errore"); }
  };
  cerca.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); vaiCerca(); } });
  scelta.addEventListener("change", () => { disegna(); svuota(risultati); });

  dialogo("📚 Biblioteca dell'assistente", el("div", { class: "bib" },
    el("div", { class: "riga-flex" }, el("label", {}, "Scaffale", scelta), spiega),
    elenco,
    el("h3", { testo: "Aggiungi un testo" }),
    el("div", { class: "riga-flex" }, el("label", { class: "spazio" }, "File (EPUB, PDF, DOCX, TXT, MD, HTML o testo OCR .json)", file), el("label", {}, "oppure", selManuale)),
    el("div", { class: "riga-flex" }, el("label", { class: "spazio" }, "Titolo", titolo), el("label", {}, "Tipo", tipo), aggiungi),
    barra, stato,
    el("p", { class: "tenue piccolo", testo: "Usa solo copie dei testi che possiedi. Restano sul PC, nella cartella dei dati di Gaster Bless. A ogni domanda l'IA riceve solo i brani pertinenti: con un servizio online quei brani gli vengono inviati, con Ollama non escono di casa." }),
    el("h3", { testo: "🔎 Cerca nei testi" }),
    el("div", { class: "riga-flex" }, cerca, el("button", { testo: "Cerca", on: { click: vaiCerca } })),
    risultati), { largo: true });
  try { await ricarica(); } catch (e) { svuota(elenco, el("p", { class: "errore", testo: e.message })); }
}

function apriManuale(p) {
  return p.manuale && p.pagina ? el("a", { class: "pulsante piccolo", href: "/manuali?apri=" + encodeURIComponent(p.manuale) + "&pagina=" + p.pagina, "data-interno": true,
    testo: "📖 Apri a pag. " + p.pagina + " del PDF", on: { click: () => document.querySelectorAll(".velo").forEach((v) => v.__chiudi && v.__chiudi()) } }) : null;
}

/** Mostra un brano citato (fonte di una risposta). */
export async function mostraBrano(f) {
  let p;
  try { p = await api("/biblioteca/" + encodeURIComponent(f.fonte) + "/brano/" + f.pezzo + (f.fino ? "?fino=" + f.fino : "")); }
  catch (e) { avviso(e.message, "errore"); return; }
  dialogo((f.rif ? "[" + f.rif + "] " : "") + p.titolo, el("div", {},
    el("p", { class: "tenue", testo: p.dove + " · " + (p.dominio === "canone" ? "canone (Archivista)" : "campagna (Cronista)") }),
    el("p", { class: "bib-testo", testo: p.testo }), apriManuale(p)), { largo: true });
}

/** Le fonti sotto una risposta e i riferimenti [C1] / [H1] del testo resi cliccabili. */
export function fontiRisposta(corpo, fonti) {
  if (!Array.isArray(fonti) || !fonti.length) return null;
  const per = Object.fromEntries(fonti.map((f) => [f.rif, f]));
  const cammina = document.createTreeWalker(corpo, NodeFilter.SHOW_TEXT);
  const nodi = [];
  while (cammina.nextNode()) if (/\[[CH]\d+\]/.test(cammina.currentNode.nodeValue)) nodi.push(cammina.currentNode);
  for (const n of nodi) {
    const pezzi = n.nodeValue.split(/(\[[CH]\d+\])/);
    const frammento = document.createDocumentFragment();
    for (const x of pezzi) {
      const m = x.match(/^\[([CH]\d+)\]$/);
      if (m && per[m[1]]) frammento.append(el("button", { class: "ia-rif " + (m[1][0] === "C" ? "canone" : "campagna"), testo: m[1], title: per[m[1]].titolo + " — " + per[m[1]].dove,
        on: { click: () => mostraBrano(per[m[1]]) } }));
      else frammento.append(x);
    }
    n.replaceWith(frammento);
  }
  return el("details", { class: "ia-fonti" }, el("summary", { testo: "📚 Brani dati all'IA (" + fonti.length + ")" }),
    el("ul", {}, fonti.map((f) => el("li", {}, el("button", { class: "ia-rif " + (f.dominio === "canone" ? "canone" : "campagna"), testo: f.rif, on: { click: () => mostraBrano(f) } }),
      " " + f.titolo + " — " + f.dove))));
}
