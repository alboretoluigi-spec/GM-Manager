// Collegamenti del «Codice della campagna» con il resto di Gaster Bless (fase L2, 2026-11-12). Solo per il Master.
// - nomi di una scheda: il nome e gli alias (campo «Alias: Madre Maren, Maren»);
// - schedeDi(nome, {luogo}): le schede che si chiamano così (o hanno quell'alias), più, con «luogo», quelle che hanno quel
//   luogo nei campi Luogo/Dove/Regione o tra i collegamenti (per la mappa: PNG ed eventi del posto);
// - riferimenti(): ciò che c'è altrove per nome — voci di «PNG e mostri», personaggi dei giocatori, oggetti dell'archivio,
//   località della mappa — con i percorsi da aprire;
// - linkCodice(scheda): un collegamento interno a /codice?scheda=…
import { el, api } from "./ui.js";

export const ICONE = { luogo: "🏰", png: "🧑", fazione: "⚜", evento: "⚡", trama: "🧵", oggetto: "🗡", segreto: "🔒", cronologia: "📅" };

// confronto dei nomi senza maiuscole, accenti, virgolette e spazi doppi
export function norma(s) {
  return String(s || "").normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase().replace(/[«»"“”'’`]/g, "").replace(/\s+/g, " ").trim();
}
const dividi = (v) => String(v || "").split(/[;,]/).map((x) => x.trim()).filter(Boolean);

export function aliasDi(x) {
  const out = [];
  for (const [k, v] of Object.entries((x && x.campi) || {})) if (/^(alias|altri nomi|detto|detta|conosciut[oa] come)$/i.test(k.trim())) out.push(...dividi(v));
  return out;
}
export const nomiDi = (x) => [x.nome, ...aliasDi(x)];

let cache = null, quando = 0;
export async function caricaCodice(forza) {
  if (!forza && cache && Date.now() - quando < 20000) return cache;
  cache = await api("/lore"); quando = Date.now();
  return cache;
}
export function dimenticaCodice() { cache = null; }

// trova(schede, nome): la scheda con quel nome o alias
export function trova(schede, nome) {
  const n = norma(nome);
  return (schede || []).find((x) => nomiDi(x).some((y) => norma(y) === n)) || null;
}

export function schedeDi(schede, nome, { luogo = false } = {}) {
  const n = norma(nome);
  if (!n) return [];
  return (schede || []).filter((x) => {
    if (nomiDi(x).some((y) => norma(y) === n)) return true;
    if (!luogo) return false;
    const campi = Object.entries(x.campi || {}).filter(([k]) => /^(luogo|dove|regione)$/i.test(k.trim())).map(([, v]) => norma(v));
    return campi.some((v) => v === n || v.split(/[;,]/).map((y) => y.trim()).includes(n)) || (x.collegamenti || []).some((c) => norma(c) === n);
  });
}

export function linkCodice(x, testo) {
  return el("a", { href: "/codice?scheda=" + encodeURIComponent(x.id), "data-interno": true, class: "pulsante piccolo",
    testo: testo || (ICONE[x.tipo] || "🗝") + " " + x.nome, title: "Apri nel Codice della campagna" });
}

// ===== riferimenti fuori dal Codice =====
let rif = null;
export async function riferimenti() {
  if (rif) return rif;
  rif = (async () => {
    const json = (u) => fetch(u).then((r) => (r.ok ? r.json() : null)).catch(() => null);
    const [best, cat, mie, pgs, ogg, locs] = await Promise.all([json("/carta/bestiario.json"), json("/carta/mostri.json"),
      api("/bestiario").catch(() => null), api("/personaggi").catch(() => []), api("/oggetti-magici").catch(() => null), json("/carta/locs.json")]);
    const voci = [...((best && best.voci) || []), ...((cat && cat.voci) || []), ...(((mie && mie.extras) || []).map((x) => ({ id: x.id, nome: x.nome, cat: x.cat, mia: true })))];
    return {
      voci: voci.map((v) => ({ id: v.id, nome: v.nome, cat: v.cat, mia: !!v.mia })),
      pg: (pgs || []).map((p) => ({ id: p.id, nome: p.nome })),
      oggetti: ((ogg && ogg.oggetti) || []).map((o) => ({ id: o.id, nome: (o.oggetto && o.oggetto.nome) || "" })).filter((o) => o.nome),
      luoghi: (locs || []).map((l) => ({ k: l.k, nome: l.n })),
    };
  })().catch(() => ({ voci: [], pg: [], oggetti: [], luoghi: [] }));
  return rif;
}
export function dimenticaRiferimenti() { rif = null; }

// campo «Statistiche: «Aldric (vampiro)» in PNG e mostri»: se ci sono nomi tra «», valgono solo quelli (uguali);
// altrimenti il testo intero (contiene il nome)
export function statisticheDi(x) {
  const out = [];
  for (const [k, v] of Object.entries((x && x.campi) || {})) {
    if (!/^(statistiche|scheda|png e mostri)$/i.test(k.trim())) continue;
    const q = [...String(v).matchAll(/«([^»]+)»/g)].map((m) => norma(m[1]));
    out.push(q.length ? { esatti: q } : { testo: norma(v) });
  }
  return out;
}
export const statisticheCitano = (x, nome) => {
  const n = norma(nome);
  return statisticheDi(x).some((s) => (s.esatti ? s.esatti.includes(n) : n.length > 3 && s.testo.includes(n)));
};

// riferimenti di una scheda del Codice (per nome, alias o campo «Statistiche: «Nome»»)
export function riferimentiDi(x, r) {
  const nomi = nomiDi(x).map(norma);
  const out = [];
  for (const v of r.voci) {
    const n = norma(v.nome);
    if (nomi.includes(n) || statisticheCitano(x, v.nome)) out.push({ testo: (v.cat === "png" ? "👤 " : "🐺 ") + v.nome + " (PNG e mostri)", href: "/bestiario?voce=" + encodeURIComponent(v.id), mia: v.mia });
  }
  out.sort((a, b) => b.mia - a.mia);
  for (const p of r.pg) if (nomi.includes(norma(p.nome))) out.push({ testo: "📔 " + p.nome + " (background)", href: "/background?pg=" + encodeURIComponent(p.id) });
  for (const o of r.oggetti) if (nomi.includes(norma(o.nome))) out.push({ testo: "✨ " + o.nome + " (oggetti magici)", href: "/oggetti-magici" });
  if (x.tipo === "luogo") for (const l of r.luoghi) if (nomi.includes(norma(l.nome))) out.push({ testo: "🗺 " + l.nome + " sulla mappa", href: "/mappa?loc=" + encodeURIComponent(l.k) });
  return out;
}
