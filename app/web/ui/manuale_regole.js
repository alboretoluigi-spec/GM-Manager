// «📥 Importa dai manuali», regole (fase M3.4, docs/PIANO_IMPORTA_MANUALI.md, 2026-11-11). Dai manuali del Master
// (Manuale del Giocatore, supplemento, supplemento; anche scansionati con OCR) prende il testo di talenti, condizioni, background
// e loro privilegi, razze e tratti razziali, privilegi di classe e sottoclasse — solo le voci che la scheda di Gaster Bless
// conosce già per nome. È testo di consultazione per il Master (nel dettaglio della voce nella scheda e in «📖 Regole dai
// manuali»): non cambia nessun calcolo. Le descrizioni brevi della scheda restano le sintesi di Gaster Bless.
// Il PDF si legge come flusso di righe (righeDelManuale, insieme a incantesimi e oggetti).
import { normalizza, mediana } from "./manuale_scheda.js";

export const CONDIZIONI = [["Accecato", "Blinded"], ["Affascinato", "Charmed"], ["Afferrato", "Grappled"], ["Assordato", "Deafened"], ["Avvelenato", "Poisoned"],
  ["Incapacitato", "Incapacitated"], ["Invisibile", "Invisible"], ["Paralizzato", "Paralyzed"], ["Pietrificato", "Petrified"], ["Prono", "Prone"],
  ["Spaventato", "Frightened"], ["Stordito", "Stunned"], ["Trattenuto", "Restrained"], ["Privo di sensi", "Unconscious"], ["Indebolimento", "Exhaustion"]];
// nomi troppo generici o ripetuti in ogni classe e razza: darebbero solo doppioni
const ESCLUSE = new Set(["incantesimi", "lancio di incantesimi", "aumento dei punteggi di caratteristica", "competenze", "equipaggiamento", "linguaggi",
  // nomi uguali alle azioni e ai tratti delle schede dei mostri (prova sull'avventura, 2026-11-11)
  "multiattacco", "morso", "artigli", "mutaforma"]);
// il testo di un'azione o di un tratto di un mostro, non di una regola
const DA_MOSTRO = /^(?:attacco (?:con arma|con incantesimo|a distanza|in mischia))|al tiro per colpire, portata|\b(?:sferra|effettua) (?:due|tre|quattro) attacchi/i;
const pulisci = (n) => String(n || "").replace(/\s*\(.*?\)\s*/g, " ").replace(/✦/g, "").replace(/\s+/g, " ").trim();
const parseF = (s) => String(s || "").split(";").filter(Boolean).flatMap((p) => p.slice(p.indexOf(":") + 1).split("|"));

/** Le voci che la scheda conosce: Map(nome normalizzato → {nome, sottotipo}). R: CLASSES, RACES, BGS, FD_GEN, FD_RACE, FD_BG, FEATS. */
export function vociNote(R) {
  const m = new Map();
  const metti = (letto, sottotipo, nome) => {
    const k = normalizza(pulisci(letto));
    if (k.length >= 3 && !ESCLUSE.has(k) && !m.has(k)) m.set(k, { nome: pulisci(nome || letto), sottotipo });
  };
  for (const [it, en] of CONDIZIONI) { metti(it, "condizione"); metti(en, "condizione", it); }
  for (const f of R.FEATS || []) { metti(f[0], "talento"); metti(f[1], "talento", f[0]); }
  for (const b of Object.values(R.BGS || {})) metti(b.n, "background");
  for (const k of Object.keys(R.FD_BG || {})) metti(k, "privilegio_background");
  for (const r of Object.values(R.RACES || {})) metti(r.n, "razza");
  for (const r of Object.values(R.RACES || {})) for (const t of r.t || []) metti(t, "tratto_razziale");
  for (const k of Object.keys(R.FD_RACE || {})) metti(k, "tratto_razziale");
  for (const c of Object.values(R.CLASSES || {})) {
    for (const n of parseF(c.f)) metti(n, "privilegio");
    for (const s of c.subs || []) for (const n of parseF(s[2])) metti(n, "privilegio");
  }
  for (const k of Object.keys(R.FD_GEN || {})) metti(k, "privilegio");
  return m;
}

// il titolo di una voce: la riga intera («ALLERTA», «Privilegio: Rifugio dei Fedeli») o l'inizio («Scurovisione. Abituato…»)
const PREFISSO = /^(privilegio|feature|privilegio del background)\s*[:.]\s*/i;
function titolo(t, note) {
  const intera = t.replace(PREFISSO, "").replace(/[.:]\s*$/, "").trim();
  if (intera.length <= 60) { const v = note.get(normalizza(intera)); if (v) return { v, resto: "" }; }
  const m = /^([^.:;!?]{5,50})[.:]\s+(\S.*)$/.exec(t.replace(PREFISSO, ""));
  if (m) { const v = note.get(normalizza(m[1])); if (v) return { v, resto: m[2] }; }
  return null;
}

/** Da righe in ordine di lettura ([{testo, h, p}]) alle regole: [{nome, sottotipo, pagina, testo, incerto, motivi}]. */
export function leggiRegole(righe, note) {
  const normale = mediana(righe.map((r) => r.h));
  const inizi = [];
  righe.forEach((r, i) => { const x = titolo(r.testo.trim(), note); if (x) inizi.push({ i, ...x }); });
  return inizi.map((x, n) => {
    const fine = n + 1 < inizi.length ? inizi[n + 1].i : righe.length;
    const testo = x.resto ? [x.resto] : [];
    for (let k = x.i + 1; k < fine && testo.length < 80; k++) {
      const r = righe[k];
      // un titolo grande (capitolo, sezione) chiude la voce
      if (r.h >= normale * 1.4 && r.testo.length <= 60 && !/[.,;:]$/.test(r.testo)) break;
      testo.push(r.testo);
    }
    const unito = testo.reduce((a, b) => (!a ? b : /[-‐]$/.test(a) ? a.slice(0, -1) + b : a + " " + b), "").slice(0, 6000);
    const motivi = unito ? [] : ["testo non letto"];
    return { nome: x.v.nome, sottotipo: x.v.sottotipo, pagina: righe[x.i].p, testo: unito, incerto: motivi.length > 0, motivi };
  }).filter((x) => x.testo && !DA_MOSTRO.test(x.testo.slice(0, 160)));
}
