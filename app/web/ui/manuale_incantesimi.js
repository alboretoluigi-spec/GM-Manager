// «📥 Importa dai manuali», incantesimi (fase M3.2, docs/PIANO_IMPORTA_MANUALI.md, 2026-11-11). Legge dai PDF del Master
// (Manuale del Giocatore, supplemento, supplemento; anche scansionati con OCR) gli incantesimi e gli elenchi per classe.
// Il PDF si legge come un unico flusso di righe (colonna sinistra, poi destra, pagina dopo pagina), perché le descrizioni
// passano da una colonna o da una pagina all'altra. Un incantesimo inizia con il nome seguito dalla riga
// «Scuola di N° livello (rituale)» o «Trucchetto di scuola» (anche in inglese), poi Tempo di lancio, Gittata, Componenti,
// Durata e la descrizione, con l'eventuale «Ai livelli superiori».
// I valori vanno nelle schede dei giocatori (con i codici di web/regole/dati.js); il testo resta al Master.
import { righePagina, componiRighe, mediana, senzaMargini, correggiOCR, nomeLeggibile, normalizza, I_ } from "./manuale_scheda.js";

export const SCUOLE = { abiurazione: "A", evocazione: "C", divinazione: "D", ammaliamento: "E", invocazione: "V", illusione: "I", necromanzia: "N", trasmutazione: "T",
  abjuration: "A", conjuration: "C", divination: "D", enchantment: "E", evocation: "V", illusion: "I", necromancy: "N", transmutation: "T" };
// Nomi delle scuole con le «i» che l'OCR storpia («lnvocazione», «llusione»): si riconoscono e si riportano al codice.
const SCUOLE_RE = Object.entries(SCUOLE).map(([n, c]) => [new RegExp("^" + n.replace(/i/g, I_) + "$", "i"), c]);
const scuolaDa = (w) => (SCUOLE_RE.find(([re]) => re.test(w)) || [])[1];
const NOMI_SCUOLE = Object.keys(SCUOLE).join("|");
const PAROLA = "([A-Za-zÀ-ÿ1|!]{6,})";
// OCR: «1°» può uscire come «1o», «l°», «1 °», «1º»
const LIVELLO_IT = new RegExp("^" + PAROLA + "\\s+d[i1l]\\s+([0-9lI])\\s*[°ºo*]?\\s*l[i1l]vello\\s*(\\(\\s*r[i1l]tuale\\s*\\))?\\s*$", "i");
const TRUCCHETTO_IT = new RegExp("^trucchetto\\s+d[i1l]\\s+" + PAROLA + "\\s*$", "i");
const LIVELLO_EN = new RegExp("^([1-9])(st|nd|rd|th)-level\\s+(" + NOMI_SCUOLE + ")\\s*(\\(ritual\\))?\\s*$", "i");
const TRUCCHETTO_EN = new RegExp("^(" + NOMI_SCUOLE + ")\\s+cantrip\\s*$", "i");
const et = (re) => new RegExp("^(" + re.replace(/%/g, I_) + ")\\s*[:.;]?\\s*", "i");
const CAMPI = [["t", et("tempo d% lanc%o|casting time")], ["r", et("g%ttata|range|portata")], ["cp", et("component%|components")], ["d", et("durata|duration")]];
const SUPERIORI = new RegExp("^(a%\\s+l%vell%\\s+super%or%|at higher levels|potenziamento de%\\s+trucchett%)\\s*[.:]\\s*".replace(/%/g, I_), "i");
export const CLASSI = { artefice: "a", bardo: "b", chierico: "c", druido: "d", paladino: "p", ranger: "r", stregone: "s", warlock: "w", mago: "m",
  artificer: "a", bard: "b", cleric: "c", druid: "d", paladin: "p", sorcerer: "s", wizard: "m" };
const ELENCO_CLASSE = new RegExp("^(?:incantesimi\\s+d(?:a|el|ell'|ello)\\s*|incantesimi\\s+(?:del|dell')\\s*|)(" + Object.keys(CLASSI).join("|") + ")(?:\\s+spells)?\\s*$", "i");
const ELENCO_TITOLO = new RegExp("^(?:incantesimi\\s+d(?:a|el|ell'|ello)\\s*(" + Object.keys(CLASSI).join("|") + ")|(" + Object.keys(CLASSI).join("|") + ")\\s+spells)\\s*$", "i");

/** Riga «scuola e livello»: { l, sc, rit } oppure null. */
export function rigaLivello(t) {
  t = String(t).replace(/\s+/g, " ").trim();
  let m = LIVELLO_IT.exec(t);
  if (m && scuolaDa(m[1])) return { l: +m[2].replace(/[lI]/, "1"), sc: scuolaDa(m[1]), rit: !!m[3] };
  if ((m = TRUCCHETTO_IT.exec(t)) && scuolaDa(m[1])) return { l: 0, sc: scuolaDa(m[1]), rit: false };
  if ((m = LIVELLO_EN.exec(t))) return { l: +m[1], sc: SCUOLE[m[3].toLowerCase()], rit: !!m[4] };
  if ((m = TRUCCHETTO_EN.exec(t))) return { l: 0, sc: SCUOLE[m[1].toLowerCase()], rit: false };
  return null;
}

// ----- Valori nei codici dell'archivio della scheda (web/regole/dati.js) -----
// numeri storpiati dall'OCR solo dove sono numeri: «l minuto» → «1 minuto», «1O minuti» → «10 minuti»
const n2 = (s) => String(s).replace(/(^|\s)[lI|](?=\s)/g, "$11").replace(/(\d)[oO](?=\s|$)/g, "$10").replace(/(^|\s)[lI|](\d)/g, "$11$2");
/** «1 azione» → «a», «1 azione bonus» → «ab», «1 reazione, che …» → «re», «10 minuti» → «10m», «1 ora» → «1h»; rituale → «*…». */
export function codiceTempo(t, rit) {
  const x = normalizza(n2(t));
  let c;
  if (/^1 azione bonus|^1 bonus action/.test(x)) c = "ab";
  else if (/^1 reazione|^1 reaction/.test(x)) c = "re";
  else if (/^1 azione o 8 ore/.test(x)) c = "a o 8h";
  else if (/^1 azione|^1 action/.test(x)) c = "a";
  else { const m = /^(\d+) (minut|ore|ora|hour|minute)/.exec(x); c = m ? m[1] + (/^min/.test(m[2]) ? "m" : "h") : t.trim().slice(0, 40); }
  return (rit ? "*" : "") + c;
}
/** «Istantanea» → «I», «Concentrazione, fino a 1 minuto» → «C1m», «8 ore» → «8h», «Finché non viene dissolto» → «P». */
export function codiceDurata(t) {
  const x = normalizza(n2(t));
  if (/^[il1]stantane|^instantaneous/.test(x)) return /o 1 ora|or 1 hour/.test(x) ? "I o 1h" : "I";
  if (/^f[il1]nche non v[il1]ene d[il1]ssolt|^until dispelled/.test(x)) return "P";
  if (/^speciale|^special/.test(x)) return "S";
  const conc = /^concentrazione|^concentration/.test(x);
  const m = /(\d+) (round|minut|ora|ore|giorn|hour|minute|day)/.exec(x);
  if (!m) return t.trim().slice(0, 40);
  const u = /^round/.test(m[2]) ? "r" : /^minut/.test(m[2]) ? "m" : /^(ora|ore|hour)/.test(m[2]) ? "h" : "g";
  return (conc ? "C" : "") + m[1] + u;
}
/** «V, S, M (un pizzico di sale)» → «VSM». */
export function codiceComponenti(t) {
  const x = String(t).replace(/\(.*$/, "").toUpperCase();
  return ["V", "S", "M"].filter((k) => new RegExp("\\b" + k + "\\b").test(x)).join("");
}
/** Gittata come nell'archivio: «36 metri» → «36 m», «Sé (raggio di 3 metri)» → «Sé (raggio di 3 m)». */
export function testoGittata(t) {
  return correggiOCR(String(t)).replace(/\s+/g, " ").replace(/\bmetri\b|\bmetro\b/gi, "m").replace(/^se\b/i, "Sé").trim().slice(0, 60);
}

// ----- Lettura -----
/** Righe di un manuale in ordine di lettura, senza intestazioni e numeri di pagina: [{testo, h, p, colonna}]. */
export async function righeDelManuale(doc, onp, fermato) {
  const out = [];
  for (let p = 1; p <= doc.numPages; p++) {
    if (fermato && fermato()) break;
    const pag = senzaMargini(await righePagina(doc, p));
    const metà = pag.larghezza / 2;
    for (const sx of [true, false]) {
      for (const r of componiRighe(pag.pezzi.filter((q) => (sx ? q.x < metà - 4 : q.x >= metà - 4)))) out.push({ testo: r.testo, h: r.h, p, font: r.font, x: r.x });
    }
    if (onp) onp(p, doc.numPages);
    if (p % 20 === 0) await new Promise((r) => setTimeout(r, 0));
  }
  return out;
}

/**
 * Legge gli incantesimi di un manuale. onp(p, totale) segnala l'avanzamento.
 * Restituisce { incantesimi: [{ nome, pagina, dati: {l, sc, t, r, cp, d, cl, hi}, testo, incerto, motivi }], classi: {nome normalizzato: "bcm"} }.
 */
export async function incantesimiDelManuale(doc, onp, fermato) {
  const righe = await righeDelManuale(doc, onp, fermato);
  return leggiIncantesimi(righe);
}

/** La parte senza PDF (provata da sola): da righe in ordine di lettura a incantesimi ed elenchi per classe. */
export function leggiIncantesimi(righe) {
  const normale = mediana(righe.map((r) => r.h));
  const classi = {};
  // 1) elenchi per classe: «Incantesimi da Bardo» poi nomi, uno per riga, fino al prossimo elenco o a una riga lunga
  let classe = "", vuote = 0;
  for (const r of righe) {
    const t = r.testo.trim();
    const m = ELENCO_TITOLO.exec(t);
    if (m) { classe = CLASSI[(m[1] || m[2]).toLowerCase()]; vuote = 0; continue; }
    if (!classe) continue;
    if (/^(trucchetti|cantrips)\b|^\d\s*[°º]?\s*livello\b|^\d(st|nd|rd|th) level\b/i.test(t)) continue; // «Trucchetti (livello 0)», «1° livello»
    // l'elenco finisce a un titolo grande, a un'etichetta o alla riga del livello di un incantesimo
    if (r.h >= normale * 1.4 || CAMPI.some(([, re]) => re.test(t)) || rigaLivello(t)) { classe = ""; continue; }
    if (t.length > 45 || /[.:;,]$/.test(t) || !/^[A-ZÀ-Ý]/.test(t)) { if (++vuote > 2) classe = ""; continue; }
    const k = normalizza(t.replace(/\s*\((rituale|ritual|R|C)\)\s*$/i, ""));
    if (k.length >= 3) { classi[k] = (classi[k] || "") + (classi[k] && classi[k].includes(classe) ? "" : classe); vuote = 0; }
  }
  // 2) incantesimi: nome + riga del livello
  const inizi = [];
  for (let i = 1; i < righe.length; i++) {
    const lv = rigaLivello(righe[i].testo);
    if (!lv) continue;
    const t = righe[i - 1].testo.trim();
    if (t.length < 3 || t.length > 50 || /[.,;:]$/.test(t)) continue;
    inizi.push({ i: i - 1, lv });
  }
  const out = [];
  inizi.forEach((x, n) => {
    const fine = n + 1 < inizi.length ? inizi[n + 1].i : righe.length;
    const titolo = righe[x.i];
    const dati = { l: x.lv.l, sc: x.lv.sc, t: "", r: "", cp: "", d: "", cl: "", hi: "" };
    const testo = [], superiori = [];
    let campo = null, inSup = false, letti = 0;
    for (let k = x.i + 2; k < fine && letti < 120; k++) {
      const r = righe[k], t = r.testo;
      // un titolo grande che non è un incantesimo (capitolo, sezione): la descrizione finisce
      if (r.h >= normale * 1.5 && t.length <= 50 && !/[.,;:]$/.test(t) && !CAMPI.some(([, re]) => re.test(t))) break;
      if (ELENCO_TITOLO.test(t.trim())) break;
      letti++;
      const c = CAMPI.find(([, re]) => re.test(t));
      if (c && !dati[c[0]]) { campo = c[0]; dati[campo] = t.replace(c[1], "").trim(); continue; }
      if (campo && campo !== "fine" && !testo.length) {
        // la riga continua il campo precedente (per esempio componenti materiali su due righe) finché non arriva la Durata
        if (campo !== "d" || /^(fino|concentr|o |or )/i.test(t)) { dati[campo] += " " + t; continue; }
        campo = "fine";
      }
      const sup = SUPERIORI.exec(t);
      if (sup) { inSup = true; superiori.push(t.replace(sup[0], "")); continue; }
      (inSup ? superiori : testo).push(t);
    }
    const unisci = (l) => l.reduce((a, b) => (!a ? b : /[-‐]$/.test(a) ? a.slice(0, -1) + b : a + " " + b), "");
    const motivi = [];
    for (const [k, nome] of [["t", "tempo di lancio"], ["r", "gittata"], ["cp", "componenti"], ["d", "durata"]]) if (!dati[k]) motivi.push(nome + " non letto");
    if (!testo.length) motivi.push("descrizione non letta");
    const nome = nomeLeggibile(titolo.testo);
    const dav = { l: dati.l, sc: dati.sc, t: codiceTempo(dati.t, x.lv.rit), r: testoGittata(dati.r), cp: codiceComponenti(dati.cp), d: codiceDurata(dati.d),
      cl: "", hi: unisci(superiori).slice(0, 1500), mat: (/\((.*)\)/.exec(dati.cp) || [])[1] || "" };
    out.push({ nome, pagina: titolo.p, dati: dav, testo: unisci(testo).slice(0, 12000), incerto: motivi.length > 0, motivi });
  });
  // le classi lette dopo (elenchi in fondo al capitolo) valgono anche per gli incantesimi letti prima; con l'OCR il nome
  // nell'elenco e quello del titolo possono differire di una o due lettere
  const chiaviClassi = Object.keys(classi);
  const classiDi = (nome) => {
    const k = normalizza(nome);
    if (classi[k]) return classi[k];
    const max = k.length > 10 ? 2 : 1;
    const vicine = chiaviClassi.filter((c) => vicino(k, c, max));
    return vicine.length === 1 ? classi[vicine[0]] : "";
  };
  for (const x of out) if (!x.dati.cl) x.dati.cl = classiDi(x.nome);
  return { incantesimi: out, classi };
}

// due nomi che differiscono al massimo di «max» lettere (Levenshtein con tetto)
function vicino(a, b, max) {
  if (Math.abs(a.length - b.length) > max) return false;
  let prec = Array.from({ length: b.length + 1 }, (_, j) => j);
  for (let i = 1; i <= a.length; i++) {
    const cor = [i]; let minimo = i;
    for (let j = 1; j <= b.length; j++) { cor[j] = Math.min(prec[j] + 1, cor[j - 1] + 1, prec[j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1)); minimo = Math.min(minimo, cor[j]); }
    if (minimo > max) return false;
    prec = cor;
  }
  return prec[b.length] <= max;
}
