// Pagine dei manuali pronte per i telefoni (fase M2, docs/PIANO_MEDIA_MANUALI.md, 2026-11-11).
// Gira SOLO sul PC del Master (lo avvia app.js): con lo stesso pdf.js del lettore trasforma una volta ogni pagina dei
// manuali condivisi in un'immagine WebP larga 1400 px e ne estrae il testo per la ricerca; le manda a Gaster Bless
// (internal/server/manuali_pagine.go). I telefoni dei giocatori poi sfogliano le immagini invece dei pezzi del PDF.
// Riprende da dove si era fermato (il server sa quali pagine mancano) e si rifà da sola se il PDF cambia.
import { api } from "./ui.js";

export const LARGHEZZA = 1400, QUALITA = 0.75;
const stato = { attivo: false, manuale: "", nome: "", fatte: 0, totale: 0, errore: "" };
let inCorso = null, ancora = false;

export function statoPreparazione() { return { ...stato }; }
const avvisa = () => document.dispatchEvent(new CustomEvent("gasterbless:pagine", { detail: statoPreparazione() }));
const pausa = (ms) => new Promise((r) => setTimeout(r, ms));

/** Avvia (o riaccoda) la preparazione di tutti i manuali condivisi che non sono pronti. */
export function preparaPagine() {
  if (inCorso) { ancora = true; return inCorso; }
  inCorso = giro().finally(() => { inCorso = null; if (ancora) { ancora = false; preparaPagine(); } });
  return inCorso;
}

export const condiviso = (m) => !!(m.condivisoTutti || (m.utenti && m.utenti.length));
export const pronto = (m) => !!(m.pagine && m.pagine.totale && m.pagine.pronte >= m.pagine.totale && m.pagine.testo);

async function giro() {
  let lista = [];
  try { lista = await api("/manuali"); } catch { return; }
  stato.errore = "";
  for (const m of lista) {
    if (!condiviso(m) || pronto(m)) continue;
    try { await preparaManuale(m); } catch (e) { stato.errore = m.nome + ": " + (e.message || e); avvisa(); }
  }
  Object.assign(stato, { attivo: false, manuale: "", nome: "" });
  avvisa();
}

async function preparaManuale(m) {
  const { caricaPdfJs } = await import("./pdfjs.js");
  const lib = await caricaPdfJs();
  const doc = await lib.getDocument({ url: "/api/v1/manuali/" + encodeURIComponent(m.id) + "/file", isEvalSupported: false,
    disableAutoFetch: true, rangeChunkSize: 1 << 20, standardFontDataUrl: "/vendor/pdfjs/standard_fonts/" }).promise;
  try {
    const tot = doc.numPages;
    const st = await api("/manuali/" + m.id + "/pagine");
    const mancanti = st.totale === tot ? (st.mancanti || []) : Array.from({ length: tot }, (_, i) => i + 1);
    Object.assign(stato, { attivo: true, manuale: m.id, nome: m.nome, totale: tot, fatte: tot - mancanti.length });
    avvisa();
    const tela = document.createElement("canvas");
    for (const n of mancanti) {
      const pg = await doc.getPage(n);
      const vp = pg.getViewport({ scale: LARGHEZZA / pg.getViewport({ scale: 1 }).width });
      tela.width = Math.round(vp.width); tela.height = Math.round(vp.height);
      const c = tela.getContext("2d");
      c.fillStyle = "#fff"; c.fillRect(0, 0, tela.width, tela.height); // fondo bianco: le pagine trasparenti restano leggibili
      await pg.render({ canvasContext: c, viewport: vp }).promise;
      const blob = await new Promise((ok) => tela.toBlob(ok, "image/webp", QUALITA));
      if (!blob || blob.type !== "image/webp") throw new Error("questo browser non crea immagini WebP: apri Gaster Bless con Chrome o Edge sul PC");
      await api(`/manuali/${encodeURIComponent(m.id)}/pagine/${n}?totale=${tot}&larghezza=${LARGHEZZA}`, { metodo: "PUT", corpo: blob, tipo: "image/webp" });
      pg.cleanup();
      stato.fatte++; avvisa();
      await pausa(20); // il PC resta reattivo
    }
    if (!(await api("/manuali/" + m.id + "/pagine")).testo) {
      const testi = [];
      for (let n = 1; n <= tot; n++) {
        const t = await (await doc.getPage(n)).getTextContent();
        testi.push(t.items.map((x) => x.str).join(" ").replace(/\s+/g, " ").trim());
      }
      await api("/manuali/" + encodeURIComponent(m.id) + "/testo", { metodo: "PUT", dati: { pagine: testi } });
    }
  } finally { doc.destroy(); }
}
