// «📥 Importa dai manuali», oggetti magici (fase M3.3, docs/PIANO_IMPORTA_MANUALI.md, 2026-11-11). Legge dalla Guida del
// Dungeon Master (e da supplemento, supplemento, La maledizione dell'antagonista; anche scansionati con OCR) gli oggetti magici: il nome,
// poi la riga «Oggetto meraviglioso, raro (richiede sintonia)» / «Arma (qualsiasi spada), rara» / «Anello, molto raro»
// (anche in inglese: «Wondrous item, rare (requires attunement)»), poi la descrizione. I valori (tipologia, sottotipo,
// rarità, sintonizzazione) vanno nell'archivio degli oggetti magici del Master; il testo resta a lui.
// Il PDF si legge come flusso di righe (righeDelManuale, come per gli incantesimi).
import { nomeLeggibile, mediana, I_ } from "./manuale_scheda.js";

const r_ = (s) => s.replace(/%/g, I_);
// la parola della tipologia all'inizio della riga, eventuale parentesi, poi una virgola e il resto (con la rarità)
const RIGA_TIPO = new RegExp("^(" + r_("oggetto\\s+meravigl%oso|arma|armatura|anello|poz%one|pergamena|bacchetta|bastone|verga|wondrous\\s+item|weapon|armou?r|ring|potion|scroll|wand|staff|rod") +
  ")\\s*(?:\\(([^)]*)\\)?)?\\s*[,.;]\\s*(.+)$", "i");
const TIPO_DI = [[/^oggetto|^wondrous/i, "meraviglioso"], [/^arma\b|^weapon/i, "arma"], [/^armatura|^armou?r/i, "armatura"], [/^anello|^ring/i, "anello"],
  [/^poz|^potion/i, "altro", "pozione"], [/^pergamena|^scroll/i, "altro", "pergamena"], [/^bacchetta|^wand/i, "bacchetta"], [/^bastone|^staff/i, "bastone"],
  [/^verga|^rod/i, "altro", "verga"]];
// gli oggetti meravigliosi indossati: lo slot giusto dal nome
const DAL_NOME = [[/\bstival|\bcalzar|\bpantofol|\bboots?\b|\bslippers?\b/i, "stivali"], [/\bmantell|\bcappa\b|\bmanto\b|\bcloak|\bcape\b/i, "mantello"],
  [/\bamulet|\bmedaglion|\bcollana|\bciondol|\bpendent|\btalisman|\bnecklace|\bperiapt|\bmedallion/i, "amuleto"], [/\bcintur|\bbelt\b/i, "cintura"],
  [/\bguant|\bbracciali|\bbracers?\b|\bgauntlets?\b|\bgloves?\b/i, "guanti"], [/\belmo\b|\bcappello|\bcorona\b|\bdiadema|\bhelm\b|\bhat\b|\bcirclet/i, "elmo"]];
// rarità: vale la prima nominata («non comune (+1), rara (+2) o molto rara (+3)» → non comune, con l'avviso)
const RARITA = [["molto_raro", r_("molto\\s+rar[oa]|very\\s+rare")], ["non_comune", r_("non\\s+comun[ei]|uncommon")], ["leggendario", r_("leggendar%[oa]|legendary")],
  ["artefatto", r_("artefatt[oi]|artifact")], ["raro", "\\brar[oa]\\b|\\brare\\b"], ["comune", "\\bcomun[ei]\\b|\\bcommon\\b"],
  ["varia", r_("rar%t[àa]\\s+var%[ae]|rarity\\s+varies|\\bvar%a\\b")]].map(([k, re]) => [k, new RegExp(re, "gi")]);
const CONTINUA = new RegExp("^\\(\\s*[+\\-−]\\s?\\d\\s*\\)|^(?:o|or|e)\\s+(?:molto\\s+|very\\s+)?(?:rar|comun|leggendar|uncommon|rare|legendary)|^\\(\\s*(?:" + r_("r%ch%ede|requires") + ")", "i");
const SINTONIA = new RegExp(r_("\\(\\s*(r%ch%ede\\s+(?:la\\s+)?s%nton%[az]+a?|requires\\s+attunement)([^)]*)\\)?"), "i");

/** Riconosce la riga della tipologia; restituisce i valori o null. */
export function rigaTipo(t, nome) {
  t = String(t || "").replace(/\s+/g, " ").trim();
  if (t.length > 170) return null;
  const m = RIGA_TIPO.exec(t);
  if (!m) return null;
  // prima le espressioni lunghe («molto rara», «non comune»), poi si cancellano, così «rara» e «comune» non contano due volte
  const trovate = [];
  let resto = m[3];
  for (const [k, re] of RARITA) resto = resto.replace(re, (x, pos) => { trovate.push([pos, k]); return " ".repeat(x.length); });
  if (!trovate.length) return null; // senza rarità non è la riga di un oggetto
  trovate.sort((a, b) => a[0] - b[0]);
  const diverse = [...new Set(trovate.map(([, k]) => k))];
  const motivi = [];
  let rar = diverse[0];
  if (rar === "varia") { rar = "non_comune"; motivi.push("rarità variabile: scegli tu"); }
  else if (diverse.length > 1) motivi.push("rarità diverse secondo la versione: c'è la prima");
  const [, tipo0, sotto0] = TIPO_DI.find(([re]) => re.test(m[1])) || [null, "meraviglioso"];
  let tipo = tipo0, sottotipo = sotto0 || String(m[2] || "").trim();
  if (tipo === "armatura" && /scudo|shield/i.test(sottotipo)) { tipo = "scudo"; sottotipo = ""; }
  if (tipo === "meraviglioso") { const d = DAL_NOME.find(([re]) => re.test(nome || "")); if (d) tipo = d[1]; }
  const s = SINTONIA.exec(t);
  return { dati: { tipo, sottotipo: sottotipo.slice(0, 80), rar, sint: !!s, sintDa: s ? s[2].trim().replace(/\)$/, "").slice(0, 120) : "" }, motivi };
}

/** Da righe in ordine di lettura ([{testo, h, p}]) agli oggetti magici: [{nome, pagina, dati, testo, incerto, motivi}]. */
export function leggiOggetti(righe) {
  const normale = mediana(righe.map((r) => r.h));
  const inizi = [];
  for (let i = 1; i < righe.length; i++) {
    let t = righe[i].testo.trim(), usate = 1;
    // la riga della tipologia può andare a capo dentro la parentesi
    while ((t.match(/\(/g) || []).length > (t.match(/\)/g) || []).length && usate < 3 && i + usate < righe.length) t += " " + righe[i + usate++].testo.trim();
    if (!RIGA_TIPO.test(t)) continue;
    // …o continua nella riga dopo con le altre rarità: «(+2) o molto rara (+3)»
    while (usate < 4 && i + usate < righe.length && CONTINUA.test(righe[i + usate].testo.trim())) t += " " + righe[i + usate++].testo.trim();
    const titolo = righe[i - 1].testo.trim();
    if (titolo.length < 3 || titolo.length > 60 || /[.,;:]$/.test(titolo) || !/[A-Za-zÀ-ÿ]{3}/.test(titolo) || RIGA_TIPO.test(titolo)) continue;
    const letta = rigaTipo(t, titolo);
    if (letta) inizi.push({ i: i - 1, usate, letta });
  }
  return inizi.map((x, n) => {
    const fine = n + 1 < inizi.length ? inizi[n + 1].i : righe.length;
    const testo = [];
    for (let k = x.i + 1 + x.usate; k < fine && testo.length < 150; k++) {
      const r = righe[k];
      // un titolo grande che non è un oggetto (capitolo, tabella): la descrizione finisce
      if (r.h >= normale * 1.5 && r.testo.length <= 50 && !/[.,;:]$/.test(r.testo)) break;
      testo.push(r.testo);
    }
    const motivi = [...x.letta.motivi];
    if (!testo.length) motivi.push("descrizione non letta");
    const unito = testo.reduce((a, b) => (!a ? b : /[-‐]$/.test(a) ? a.slice(0, -1) + b : a + " " + b), "");
    return { nome: nomeLeggibile(righe[x.i].testo), pagina: righe[x.i].p, dati: x.letta.dati, testo: unito.slice(0, 12000), incerto: motivi.length > 0, motivi };
  });
}
