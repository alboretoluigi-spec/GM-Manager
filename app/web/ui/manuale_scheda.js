// Lettura delle schede delle creature dai manuali PDF del Master (Manuale dei Mostri, avventura…).
// Tutto avviene nel browser del Master, sul PC: il PDF non lascia il PC e il testo non va ai giocatori.
// Due funzioni:
//   trovaPagina(doc, nomi)    → la pagina in cui il nome compare come titolo (testo più grande della pagina);
//   leggiScheda(doc, p, nomi) → una bozza di scheda (stesso formato di web/carta/bestiario.json) da
//                               controllare e correggere: leggere un PDF non è mai perfetto.
// Riconosce le etichette italiane e inglesi (Classe Armatura / Armor Class, Punti Ferita / Hit Points…).
import { caricaPdfJs } from "./pdfjs.js"; // modulo a sé: niente giro di import con la sezione Manuali

const documenti = new Map();
/** Apre un manuale (a pezzi, con richieste parziali) e lo tiene aperto per le ricerche successive. */
export async function apriManuale(id) {
  if (!documenti.has(id)) {
    const lib = await caricaPdfJs();
    documenti.set(id, Promise.all([
      lib.getDocument({ url: "/api/v1/manuali/" + encodeURIComponent(id) + "/file", isEvalSupported: false,
        disableAutoFetch: true, rangeChunkSize: 1 << 20, standardFontDataUrl: "/vendor/pdfjs/standard_fonts/" }).promise,
      testoOCR(id),
    ]).then(([doc, ocr]) => { doc.righeOCR = ocr; return doc; }).catch((e) => { documenti.delete(id); throw e; }));
  }
  return documenti.get(id);
}
/** Dopo aver caricato o tolto il testo OCR (o sostituito il PDF) il manuale si riapre da capo. */
export function dimenticaManuale(id) { documenti.delete(id); }

// Testo OCR caricato dal Master accanto al PDF (2026-11-11, internal/server/manuali_ocr.go): Map(pagina → [[x0, y0, x1, y1,
// testo], …]) in punti PDF con l'origine in basso a sinistra, come le coordinate di pdf.js; null se non c'è.
function testoOCR(id) {
  return fetch("/api/v1/manuali/" + encodeURIComponent(id) + "/ocr", { credentials: "same-origin" })
    .then((r) => (r.ok && r.status !== 204 ? r.json() : null)) // 204: il manuale non ha un testo OCR
    .then((f) => (f && Array.isArray(f.pagine) ? new Map(f.pagine.map((p) => [p.p, Array.isArray(p.l) ? p.l : []])) : null))
    .catch(() => null);
}

export const normalizza = (t) => String(t || "").toLowerCase().normalize("NFKD").replace(/[̀-ͯ]/g, "").replace(/[^a-z0-9]+/g, " ").trim();

// Righe di una pagina: pezzi di testo raggruppati per altezza, con posizione, grandezza e carattere.
export async function righePagina(doc, p) {
  const pagina = await doc.getPage(p);
  const vista = pagina.getViewport({ scale: 1 });
  if (doc.righeOCR) { // c'è il testo OCR corretto: si leggono le sue righe (una riga = un pezzo, h = altezza del riquadro)
    const pezzi = (doc.righeOCR.get(p) || []).filter((l) => l && typeof l[4] === "string" && l[4].trim())
      .map((l) => ({ x: +l[0], y: +l[1], h: Math.max(4, (+l[3]) - (+l[1])), w: (+l[2]) - (+l[0]), s: l[4], f: "" }));
    return { pezzi, larghezza: vista.width, altezza: vista.height };
  }
  const tc = await pagina.getTextContent();
  const pezzi = [];
  for (const it of tc.items) {
    if (!it.str || !it.str.trim()) continue;
    const t = it.transform, h = Math.hypot(t[2], t[3]) || Math.abs(t[3]) || 10;
    pezzi.push({ x: t[4], y: t[5], h, w: it.width || 0, s: it.str, f: it.fontName || "" });
  }
  return { pezzi, larghezza: vista.width, altezza: vista.height };
}

// metà: se indicata, una riga non attraversa mai lo spazio tra le due colonne della pagina.
export function componiRighe(pezzi, metà = 0) {
  const ord = [...pezzi].sort((a, b) => b.y - a.y || a.x - b.x);
  const righe = [];
  const lato = (q) => (metà && q.x >= metà - 4 ? 1 : 0);
  for (const q of ord) {
    const r = righe.find((x) => x.lato === lato(q) && Math.abs(x.y - q.y) < Math.max(2, Math.min(x.h, q.h) * 0.5));
    if (r) { r.parti.push(q); r.h = Math.max(r.h, q.h); } else righe.push({ y: q.y, h: q.h, lato: lato(q), parti: [q] });
  }
  for (const r of righe) {
    r.parti.sort((a, b) => a.x - b.x);
    let txt = "", fine = null;
    for (const q of r.parti) { if (fine !== null && q.x - fine > q.h * 0.2 && !txt.endsWith(" ")) txt += " "; txt += q.s; fine = q.x + q.w; }
    r.testo = correggiOCR(txt.replace(/\s+/g, " ").trim());
    r.x = r.parti[0].x;
    r.font = r.parti[0].f;
    r.primo = r.parti[0].s.trim();
  }
  return righe.sort((a, b) => b.y - a.y);
}

export const mediana = (v) => { const a = [...v].sort((x, y) => x - y); return a[a.length >> 1] || 10; };

/** Cerca la pagina in cui un nome compare come titolo. onp(p, N) segnala l'avanzamento. */
export async function trovaPagina(doc, nomi, onp, da = 1, a = 0) {
  const voluti = new Set(nomi.map(normalizza).filter((x) => x.length > 2));
  const N = doc.numPages, fine = a || N;
  let migliore = null, caratteri = 0;
  for (let p = da; p <= fine; p++) {
    const { pezzi, larghezza } = await righePagina(doc, p);
    caratteri += pezzi.reduce((n, q) => n + q.s.length, 0);
    const med = mediana(pezzi.map((q) => q.h));
    for (const r of componiRighe(pezzi, larghezza / 2)) {
      if (!voluti.has(normalizza(r.testo))) continue;
      const punti = r.h / med;
      if (!migliore || punti > migliore.punti + 0.05) migliore = { p, punti };
    }
    if (onp && (p % 5 === 0 || p === fine)) onp(p, fine);
  }
  return { pagina: migliore ? migliore.p : 0, senzaTesto: caratteri < (fine - da + 1) * 15 };
}

// Etichette delle righe della scheda (italiano e inglese).
// Etichette delle righe della scheda (italiano e inglese). Le «i» possono uscire dall'OCR come «l», «1» o «ì» («Sflda»,
// «Punt1 Ferita»): nei punti dove capita si accettano tutte.
export const I_ = "[iíìl1|!]";
const etichetta_ = (re) => new RegExp("^(" + re.replace(/%/g, I_) + ")(?=[\\s\\-—–:]|$)[\\s\\-—–:]*", "i"); // «%» = una «i» (anche storpiata)
const ETICHETTE = [
  ["ca", etichetta_("c[l1|i]asse armatura|armor class")],
  ["pf", etichetta_("punt% fer%ta|hit points")],
  ["vel", etichetta_("veloc%t[aà]|speed")],
  ["ts", etichetta_("t%r% salvezza|saving throws")],
  ["abil", etichetta_("ab%l%t[aà]|skills")],
  ["vuln", etichetta_("vulnerab%l%t[aà] a% dann%|damage vulnerabilities")],
  ["res", etichetta_("res%stenz[ae] a% dann%|damage resistances")],
  ["imm", etichetta_("%mmun%t[aà] a% dann%|damage immunities")],
  ["immc", etichetta_("%mmun%t[aà] alle cond%z%on%|condition immunities")],
  ["sensi", etichetta_("sens%|senses")],
  ["ling", etichetta_("[l1|]%nguagg%|lingue|languages")],
  ["gs", etichetta_("sf%da|grado d% sf%da|challenge")],
];
const SEZIONI = [
  ["legg", /^(azioni leggendarie|legendary actions)$/i],
  ["reazioni", /^(reazioni|reactions)$/i],
  ["azioni", /^(azioni|actions)$/i],
];
const TAGLIE = /\b(minuscol[ao]|piccol[ao]|medi[ao]|grande|enorme|mastodontic[ao]|tiny|small|medium|large|huge|gargantuan)\b/i;
// «12 (+1)», «3 (–4)»: il segno meno nei PDF può essere uno di molti trattini (o mancare lo spazio).
const NUM_CAR = /(\d{1,2})\s*\(\s*[+\-−–—‒﹣－]?\s*\d{1,2}\s*\)/g;
// Intestazione delle caratteristiche, anche in maiuscoletto («For Des Cos…») o con spazi strani.
// Anche spezzata o storpiata dall'OCR: «FO R DES…», «FOR DES COS INT» + «SAG CAR», «FOR» + «DES COS INT SAG CAR», «SAC» per «SAG».
const RIGA_CAR = { test: (t) => {
  const c = String(t).replace(/\s+/g, "").toUpperCase().replace(/0/g, "O");
  if (c.length > 26) return false;
  const m = c.match(/^(FOR|STR)?(DES|DEX)?(COS|CON)?(INT|LNT)?(SAG|SAC|WIS)?(CAR|CHA)?$/);
  return !!m && m.slice(1).filter(Boolean).length >= 2 || /^(FOR|STR)(DES|DEX)(COS|CON)/.test(c) || c === "FOR" || c === "STR";
} };
const RIGA_TAGLIA = /\b(minuscol[ao]|piccol[ao]|medi[ao]|grande|enorme|mastodontic[ao]|tiny|small|medium|large|huge|gargantuan)\b.*,/i;
// Riga «Tipo Taglia, allineamento» che apre una scheda (non una frase che contiene «Grande,»): corta e con la taglia
// tra le prime parole (italiano: «Mostruosità Grande, …»; inglese: «Large monstrosity, …»).
const APRE_SCHEDA = /^\(?[A-ZÀ-Ýa-zà-ÿ]+(?:\s[a-zà-ÿ]+)?\)?\s(minuscol[ao]|piccol[ao]|medi[ao]|grande|enorme|mastodontic[ao])\b[^.]{0,60},|^(tiny|small|medium|large|huge|gargantuan)\s[^.]{0,60},/i;
const tagliaInTesta = (t) => t.length <= 80 && (APRE_SCHEDA.test(t) || /^[A-ZÀ-Ý(][^,]{0,30}\b(minuscol[ao]|piccol[ao]|medi[ao]|grande|enorme|mastodontic[ao])\b[^,]{0,25},/i.test(t));

// Un nome di creatura: corto, con delle lettere, senza punteggiatura alla fine (non la fine di una frase del racconto).
export const possibileTitolo = (t) => t.length >= 3 && t.length <= 60 && /[A-Za-zÀ-ÿ]{3}/.test(t) && !/[.,;:]$/.test(t) && !/\.\s/.test(t) && /^[A-ZÀ-Ý(«"']/.test(t);

/** Valori delle caratteristiche in una riga, anche con gli errori dell'OCR: «1S (+2)», «B (-l)», «1 4 (+2)», «{+3}», «(t4)». */
export function valoriCaratteristiche(t) {
  const cifre = (v) => v.replace(/[lI|]/g, "1").replace(/[oO]/g, "0").replace(/S/g, "5").replace(/B/g, "8").replace(/Z/g, "2").replace(/g/g, "9");
  const x = String(t).replace(/[{\[]/g, "(").replace(/[}\]]/g, ")")
    .replace(/\(\s*[tT](?=\s*[0-9lIOoSZ])/g, "(+") // «(t4)»: il «+» letto come «t»
    .replace(/(^|[\s)])([0-9lIOo|SBZg])\s([0-9lIOo|SBZg])(?=\s*\()/g, (m, a, x, y) => a + x + y) // «1 4 (+2)», «l O(+0)»
    .replace(/(^|[\s)])([0-9lIOo|SBZg]{1,2})(?=\s*\()/g, (m, a, v) => a + cifre(v))
    .replace(/\(\s*([+\-−–—‒﹣－]?)\s*([0-9lIOoSZ]{1,2})\s*\)/g, (m, s, n) => "(" + s + cifre(n) + ")");
  return [...x.matchAll(NUM_CAR)].map((m) => +m[1]).filter((v) => v >= 1 && v <= 30);
}
// Riga fatta (quasi) solo di valori di caratteristica, anche senza l'intestazione «FOR DES COS…» saltata dall'OCR.
const rigaDiValori = (t) => valoriCaratteristiche(t).length >= 3 && String(t).replace(/[0-9()+\-−–—‒\s{}\[\]lIOoSBZgt|.,]/g, "").length <= 2;

/** Errori tipici dei PDF scansionati con OCR sui numeri: «l8dlO» → «18d10», «(+O)» → «(+0)», «lnt» → «Int». */
export function correggiOCR(t) {
  const cifre = (x) => x.replace(/[lI|]/g, "1").replace(/[oO]/g, "0");
  return String(t)
    .replace(/\b([0-9lIO]{1,2}(?:\s[0-9lIO](?=\s?d))?)\s?d\s?([0-9lIO]{1,2}(?:\s[0-9IO](?=[\s+)\-,.;]|$))?)(?![a-zà-ÿ])/g, // dadi, anche «lOdl O»
      (m, a, b) => (/\d/.test(a + b) || /^[lI]/.test(a) ? cifre(a.replace(/\s/g, "")) + "d" + cifre(b.replace(/\s/g, "")) : m))
    .replace(/(passiva\s+)([0-9lIO])\s?([0-9lIO])?(?![A-Za-zà-ÿ])/g, (m, p, a, b) => p + cifre(a + (b || ""))) // «passiva l O»
    .replace(/^((?:grado di )?sfida|challenge)(\s+)([0-9lIO]{1,2})(\s?\/\s?(\d+))?(?=\s*\()/i, (m, e, sp, n, f, d) => e + sp + cifre(n) + (d ? "/" + d : "")) // «Sfida l», «1/ 4»
    .replace(/([a-zà-ÿ])\/l/g, "$1ll") // «a/lineamento», «bu/lywug»
    .replace(/\(\s*([+\-−–—‒])\s*([0-9lIOo]{1,2})\s*\)/g, (m, s, n) => "(" + s + cifre(n) + ")") // modificatori
    .replace(/([+])\s?([0-9lIO])\s?([0-9lIO])?(?=[\s,;.)]|$)/g, (m, s, a, b, pos, tutto) => // «+l O» → «+10» (solo se c'è una lettera)
      (/[lIO]/.test(a + (b || "")) && !(b && /^\s*[+\-−]\s?\d/.test(tutto.slice(pos + m.length))) ? s + cifre(a + (b || "")) : m)) // ma «+2 O +3» è «+2 o +3»
    .replace(/\(\s*([0-9lIO][0-9lIO.,\s]{0,8})\s*(PE|XP)\s*\)/g, (m, n, u) => "(" + n.replace(/[lI]/g, "1").replace(/O/g, "0").replace(/\s+/g, "") + " " + u + ")") // «(l .800 PE)»
    .replace(/\blnt\b/g, "Int").replace(/\bcos\b(?=\s+INT)/g, "COS")
    .replace(/\s+([,;:])(?=\s|$)/g, "$1").replace(/\s+\.(?=\s|$)/g, ".") // «9 m , volare», «bersaglio .»
    .replace(/([a-zà-ÿ])\+(\d)/g, "$1 +$2"); // «Percezione+10»
}

/**
 * Legge la scheda che inizia alla pagina p con uno dei nomi indicati.
 * Restituisce { scheda, testo, pagine, trovata } — testo è il testo grezzo letto, utile per correggere.
 */
// Intestazioni e numeri di pagina (fasce in alto e in basso): non fanno parte delle schede e, in cima alla colonna dove una
// scheda continua, sembravano il titolo di un'altra creatura (2026-11-11).
export const senzaMargini = (pag) => ({ ...pag, pezzi: pag.pezzi.filter((q) => q.y < pag.altezza - 30 && q.y > 30) });

// includiTitolo: la riga trovata fa già parte della scheda (è la riga del tipo o della CA, quando il nome è altrove).
export async function leggiScheda(doc, p, nomi, vicino = null, includiTitolo = false) {
  const voluti = new Set(nomi.map(normalizza).filter((x) => x.length > 2));
  const pag = senzaMargini(await righePagina(doc, p));
  const metà = pag.larghezza / 2;
  const tutte = componiRighe(pag.pezzi, metà);
  // vicino {x, y}: il titolo esatto trovato da schedeNellaPagina (lo stesso nome può comparire anche come intestazione grande)
  const titolo = tutte.filter((r) => voluti.has(normalizza(r.testo)))
    .sort((a, b) => (vicino ? Math.abs(a.y - vicino.y) + Math.abs(a.x - vicino.x) - Math.abs(b.y - vicino.y) - Math.abs(b.x - vicino.x) : b.h - a.h))[0];
  if (!titolo) return { scheda: null, testo: "", pagine: [p], trovata: false };
  // Colonna del titolo (le pagine dei manuali sono di solito su due colonne).
  const sinistra = titolo.x < metà * 0.9;
  const inCol = (q, sx) => (sx ? q.x < metà - 4 : q.x >= metà - 4);
  let righe = componiRighe(pag.pezzi.filter((q) => inCol(q, sinistra) && q.y <= titolo.y + 1));
  const seguito = righe.length - (includiTitolo ? 0 : 1); // indice (in «resto», senza il titolo) della prima riga della colonna in cui la scheda continua
  const pagine = [p];
  if (sinistra) righe = righe.concat(componiRighe(pag.pezzi.filter((q) => inCol(q, false))));
  else if (p < doc.numPages) { // la scheda può continuare nella colonna sinistra della pagina dopo
    const dopo = senzaMargini(await righePagina(doc, p + 1));
    righe = righe.concat(componiRighe(dopo.pezzi.filter((q) => q.x < dopo.larghezza / 2 - 4)));
    pagine.push(p + 1);
  }
  // Ci si ferma al titolo di un'altra creatura: una riga più grande del testo normale (almeno quanto il titolo), oppure
  // una riga seguita da «Taglia tipo, allineamento». Non basta «grande come il titolo»: in molti manuali il nome della
  // creatura è in grassetto ma della stessa grandezza del testo, e la scheda si fermava subito (2026-11-11).
  const corpo = [], resto = includiTitolo ? righe : righe.slice(1), normale = mediana(resto.map((r) => r.h));
  let fermata = "";
  for (const [k, r] of resto.entries()) {
    const speciale = ETICHETTE.some(([, re]) => re.test(r.testo)) || SEZIONI.some(([, re]) => re.test(r.testo)) || RIGA_CAR.test(r.testo);
    if (r.testo.replace(/[^A-Za-zÀ-ÿ0-9]/g, "").length <= 2) continue; // briciole dell'OCR («o», «·»)
    const altroTitolo = !speciale && possibileTitolo(r.testo) && !tagliaInTesta(r.testo) && normalizza(r.testo) !== normalizza(titolo.testo) &&
      !/[.,;:]$/.test(r.testo) && ((r.h >= titolo.h * 0.92 && r.h > normale * 1.12) || (k > 0 && resto[k + 1] && tagliaInTesta(resto[k + 1].testo) && /[A-ZÀ-Ý]/.test(r.testo.charAt(0))));
    // in cima alla colonna dove la scheda continua: se sotto ci sono valori della scheda è un'intestazione, si salta
    const seguono = resto[k + 1] && (ETICHETTE.some(([, re]) => re.test(resto[k + 1].testo)) || /\d{1,2}\s*\(\s*[+\-−–—‒]?\s*\d/.test(resto[k + 1].testo));
    if (altroTitolo && k >= seguito && k <= seguito + 2 && seguono) continue;
    if (altroTitolo) { fermata = "fermata a «" + r.testo.slice(0, 40) + "» (sembrava un'altra creatura)"; break; }
    corpo.push(r);
  }
  // Traccia per la diagnostica: dov'era il titolo, perché si è fermata, come inizia la colonna in cui la scheda continua.
  const inizioSeguito = resto.slice(seguito, seguito + 3).map((x) => x.testo.slice(0, 50));
  const traccia = "titolo nella colonna " + (sinistra ? "sinistra" : "destra") + " (y " + Math.round(titolo.y) + " di " + Math.round(pag.altezza) + ", largh. " + Math.round(pag.larghezza) +
    "); " + (fermata || "letta fino in fondo") + "; continua in " + (sinistra ? "colonna destra" : "pagina dopo, colonna sinistra") + ": " + (inizioSeguito.length ? inizioSeguito.join(" / ") : "(vuota)");
  return { scheda: analizza(corpo), testo: corpo.map((r) => r.testo).join("\n"), pagine, trovata: true, traccia };
}

// «📥 Importa dai manuali» (2026-11-11): tutte le schede di creatura di una pagina, senza conoscerne il nome.
// Si cercano le righe «Classe Armatura»; il titolo è la riga più grande poco sopra, nella stessa colonna (o in fondo alla
// colonna sinistra, se la scheda inizia in cima a quella destra). Restituisce [{ nome, pagina, scheda, testo, incerto }].
export async function schedeNellaPagina(doc, p) {
  const pag = await righePagina(doc, p);
  // «Classe» e «Armatura» possono essere pezzi di testo separati: si guarda il testo della pagina tutto unito
  if (!/c[l1|i]asse\s*armatura|armor\s*class/i.test(pag.pezzi.map((q) => q.s).join(""))) return [];
  const metà = pag.larghezza / 2;
  const colonna = (sx) => componiRighe(pag.pezzi.filter((q) => (sx ? q.x < metà - 4 : q.x >= metà - 4)));
  const col = [colonna(true), colonna(false)];
  const etichetta = (t) => ETICHETTE.some(([, re]) => re.test(t)) || SEZIONI.some(([, re]) => re.test(t)) || RIGA_CAR.test(t);
  const trovate = [], visti = new Set();
  for (let c = 0; c < 2; c++) {
    col[c].forEach((r, i) => {
      if (!/^(c[l1|i]asse armatura|armor class)\s*[:\-]?\s*\d/i.test(r.testo)) return; // «CLASSE ARMATURA» da sola è un titolo delle regole
      let titolo = null, rigaTipo = null;
      // 1) struttura fissa della scheda: titolo, poi «Taglia tipo, allineamento» (anche su due righe), poi la CA
      for (let k = i - 1; k >= Math.max(0, i - 3); k--) {
        const t = col[c][k];
        if (etichetta(t.testo)) break;
        if (t.testo.replace(/[^A-Za-zÀ-ÿ0-9]/g, "").length <= 2) continue; // briciole dell'OCR tra il tipo e la CA
        const seguente = k + 1 < i ? col[c][k + 1].testo : "";
        if (tagliaInTesta(t.testo) || (TAGLIE.test(t.testo) && t.testo.length < 60 && seguente.includes(",") && seguente.length < 50)) {
          rigaTipo = t;
          // il titolo è la riga sopra (saltando le briciole dell'OCR); se la riga del tipo apre la colonna destra, è in fondo
          // alla colonna sinistra
          const briciola = (q) => q.testo.replace(/[^A-Za-zÀ-ÿ0-9]/g, "").length <= 2;
          let x = null;
          for (let j = k - 1; j >= 0 && !x; j--) if (!briciola(col[c][j])) x = col[c][j];
          if (!x && c === 1) x = col[0].filter((q) => !briciola(q) && !etichetta(q.testo)).slice(-1)[0] || null;
          if (x && possibileTitolo(x.testo) && !etichetta(x.testo) && !tagliaInTesta(x.testo)) titolo = x;
          break;
        }
      }
      // 2) la prima riga più grande della CA salendo (sopra c'è solo «taglia tipo, allineamento»), entro 4 righe;
      // non la più grande in assoluto, che può essere l'intestazione della pagina («LUPI» sopra la scheda «LUPO»)
      if (!titolo) for (let k = i - 1; k >= Math.max(0, i - 4); k--) {
        const t = col[c][k];
        if (etichetta(t.testo) || t.testo.length > 60) break;
        if (tagliaInTesta(t.testo) || !possibileTitolo(t.testo)) continue; // la riga del tipo (o una frase) non è mai il titolo
        if (t.h > r.h * 1.08 || (/bold|black|heavy|semibold|demi/i.test(t.font || "") && t.h >= r.h * 0.95 && !RIGA_TAGLIA.test(t.testo))) { titolo = t; break; }
      }
      if (!titolo && c === 1 && i <= 2) { // scheda iniziata in fondo alla colonna sinistra
        // solo l'ultima riga vera della colonna sinistra (più su c'è il racconto, e un'intestazione lì non è il titolo)
        const ultima = col[0].filter((q) => q.testo.replace(/[^A-Za-zÀ-ÿ0-9]/g, "").length > 2).slice(-1)[0];
        titolo = ultima && !etichetta(ultima.testo) && possibileTitolo(ultima.testo) && ultima.h > r.h * 1.08 ? ultima : null;
      }
      if (!titolo && i >= 2 && !etichetta(col[c][i - 2].testo) && possibileTitolo(col[c][i - 2].testo)) titolo = { ...col[c][i - 2], dubbio: true }; // titolo, poi «taglia tipo»
      if (!titolo) { // nome non trovato sopra la scheda (titolo saltato dall'OCR): l'intestazione grande della pagina
        const normale = mediana(col[0].concat(col[1]).map((q) => q.h));
        // l'intestazione che precede la scheda nell'ordine di lettura (colonna sinistra dall'alto, poi destra); se non c'è, la più grande
        const ordine = col[0].concat(col[1]), dove = ordine.indexOf(r);
        const grandi = ordine.map((q, n) => [q, n]).filter(([q]) => q.h >= normale * 1.4 && possibileTitolo(q.testo) && !etichetta(q.testo));
        const prima = grandi.filter(([, n]) => n < dove).slice(-1)[0];
        const grande = prima ? prima[0] : (grandi.sort((a, b) => b[0].h - a[0].h)[0] || [null])[0];
        // la scheda si legge dalla riga del tipo (o dalla CA), compresa: l'intestazione sta sopra il racconto
        const da = rigaTipo || r;
        if (grande) titolo = { x: da.x, y: da.y, h: da.h, testo: grande.testo, cerca: da.testo, includi: true, dubbio: true };
      }
      if (!titolo) return;
      const k = normalizza(titolo.testo) + (titolo.includi ? "@" + Math.round(titolo.y) : "");
      if (k.length < 3 || visti.has(k)) return;
      visti.add(k);
      trovate.push(titolo);
    });
  }
  const out = [];
  for (const t of trovate) {
    const letta = await leggiScheda(doc, p, [t.cerca || t.testo], { x: t.x, y: t.y }, !!t.includi);
    if (!letta.trovata || !letta.scheda) continue;
    const sc = letta.scheda;
    // perché la lettura è dubbia: si mostra nel riepilogo (e nella diagnostica da copiare)
    const motivi = [];
    if (t.dubbio) motivi.push("nome indovinato");
    if (!sc.ca) motivi.push("Classe Armatura non letta");
    if (!sc.pf || !/\d/.test(sc.pf)) motivi.push("Punti Ferita non letti");
    if (sc._mancanti) motivi.push("caratteristiche incomplete (" + (6 - sc._mancanti) + " su 6)");
    else if (sc.car.every((v) => v === 10)) motivi.push("caratteristiche non lette");
    if (!sc.gs) motivi.push("Sfida non letta");
    if (!sc.azioni.length && !sc.tratti.length) motivi.push("né tratti né azioni");
    out.push({ nome: nomeLeggibile(t.testo), pagina: p, scheda: sc, testo: letta.testo, incerto: motivi.length > 0, motivi, traccia: motivi.length ? letta.traccia : "" });
  }
  return out;
}

// «LUPO MANNARO» → «Lupo mannaro»; i nomi già scritti con le minuscole restano come sono.
export function nomeLeggibile(t) {
  t = String(t || "").replace(/\s+/g, " ").trim();
  if (/[A-ZÀ-Ý]/.test(t) && !/[a-zà-ÿ]/.test(t.replace(/l/g, ""))) t = t.replace(/l/g, "I"); // tutto maiuscolo tranne le «l» dell'OCR: «ANELLO Dl PROTEZIONE»
  t = t
    .replace(/([A-Za-zÀ-ÿ])([01]+)(?=[A-Za-zÀ-ÿ])/g, (m, a, d) => a + d.replace(/1/g, "i").replace(/0/g, "o")) // OCR: «Cucc10Lo»
    .replace(/\b[A-ZÀ-Ýl]{3,}\b/g, (w) => (/[A-ZÀ-Ý]/.test(w) && /l/.test(w) ? w.replace(/l/g, "I") : w)); // «lNCANTATO»: «l» in una parola maiuscola è una «I»
  // tutto maiuscolo, o maiuscole sparse dall'OCR («DRAGO Rosso CuccioLo»): solo l'iniziale maiuscola
  if (t && /[A-ZÀ-Ý]/.test(t) && (t === t.toUpperCase() || /[a-zà-ÿ][A-ZÀ-Ý]|\b[A-ZÀ-Ý]{3,}\b.*[a-zà-ÿ]/.test(t))) t = t.charAt(0) + t.slice(1).toLowerCase();
  return t.replace(/ (i{2,3}|iv|vi{0,3}|ix)$/, (m) => m.toUpperCase()); // numeri romani in fondo: «Pidlwick II»
}

// Trasforma le righe in una scheda. Le righe che non riconosce vanno avanti nel campo precedente.
export function analizza(righe) {
  const sc = { taglia: "", tipo: "", all: "", ca: "", pf: "", vel: "", car: [10, 10, 10, 10, 10, 10], ts: "", abil: "", vuln: "", res: "", imm: "", immc: "",
    sensi: "", ling: "", gs: "", tratti: [], azioni: [], reazioni: [], legg: [] };
  if (!righe.length) return sc;
  // Carattere del testo normale (il più usato): il nome di un tratto è di solito in grassetto/corsivo.
  const conta = new Map();
  for (const r of righe) for (const q of r.parti || []) conta.set(q.f, (conta.get(q.f) || 0) + q.s.length);
  const fontCorpo = [...conta.entries()].sort((a, b) => b[1] - a[1])[0]?.[0] || "";
  const unisci = (a, b) => (!a ? b : /[-‐]$/.test(a) ? a.slice(0, -1) + b : a + " " + b);

  // 1) taglia, tipo e allineamento: la prima riga
  let i = 0;
  const prima = righe[0].testo;
  if (!ETICHETTE.some(([, re]) => re.test(prima)) && !RIGA_CAR.test(prima)) {
    const virgola = prima.lastIndexOf(",");
    const tipo = virgola > 0 ? prima.slice(0, virgola) : prima;
    sc.all = virgola > 0 ? prima.slice(virgola + 1).trim() : "";
    const t = tipo.match(TAGLIE);
    sc.taglia = t ? t[0].charAt(0).toUpperCase() + t[0].slice(1).toLowerCase() : "";
    sc.tipo = (t ? tipo.replace(t[0], "") : tipo).replace(/\s+/g, " ").trim().replace(/^\((.*)\)$/, "$1");
    sc.tipo = sc.tipo.charAt(0).toLowerCase() + sc.tipo.slice(1);
    i = 1;
  }
  // 2) righe con etichetta e caratteristiche, fino alla Sfida
  let campo = null, inCar = false, numeri = [], righeCar = 0;
  const posizioni = [];
  for (; i < righe.length; i++) {
    const t = righe[i].testo;
    if (RIGA_CAR.test(t.replace(/\s+/g, " ")) && t.length < 60) {
      // quali caratteristiche ci sono nell'intestazione (l'OCR a volte ne perde qualcuna): servono a mettere i valori al posto giusto
      if (!numeri.length) for (const m of t.replace(/\s+/g, "").toUpperCase().replace(/0/g, "O").matchAll(/(FOR|STR)|(DES|DEX)|(COS|CON)|(INT|LNT)|(SAG|SAC|WIS)|(CAR|CHA)/g)) posizioni.push(m.slice(1).findIndex(Boolean));
      inCar = true; righeCar = 0; campo = null; continue;
    }
    // valori senza l'intestazione (saltata dall'OCR): una riga di soli valori prima dei tratti
    if (!inCar && numeri.length < 6 && campo !== "gs" && rigaDiValori(t)) { inCar = true; righeCar = 0; }
    if (inCar && (numeri.length >= 6 || ETICHETTE.some(([, re]) => re.test(t)) || SEZIONI.some(([, re]) => re.test(t)) || ++righeCar > 3)) inCar = false; // non si inghiottono le righe dopo
    if (inCar && numeri.length < 6) {
      // valori anche con gli errori dell'OCR («lO(+0)», «1S (+2)», «1 4 (+2)»…), su una o più righe
      numeri.push(...valoriCaratteristiche(t));
      // senza i modificatori tra parentesi: sei numeri da 1 a 30 sulla riga
      if (!numeri.length) { const n = (t.match(/\b\d{1,2}\b/g) || []).map(Number).filter((v) => v >= 1 && v <= 30); if (n.length >= 6) numeri.push(...n.slice(0, 6)); }
      if (numeri.length >= 6) { inCar = false; sc.car = numeri.slice(0, 6).map((v) => Math.max(1, Math.min(30, v))); }
      continue;
    }
    const et = ETICHETTE.find(([, re]) => re.test(t));
    if (et) {
      campo = et[0];
      sc[campo] = t.replace(et[1], "").trim();
      continue;
    }
    if (SEZIONI.some(([, re]) => re.test(t))) break;
    if (campo === "gs") break; // dopo la Sfida iniziano i tratti
    if (campo) { sc[campo] = unisci(sc[campo], t); continue; }
  }
  // Intestazione e valori incompleti (l'OCR ha perso un pezzo): i valori letti vanno al loro posto, gli altri restano 10 e
  // la scheda è segnata «caratteristiche incomplete».
  if (numeri.length >= 3 && numeri.length < 6 && posizioni.length === numeri.length && new Set(posizioni).size === posizioni.length) {
    const car = [10, 10, 10, 10, 10, 10];
    posizioni.forEach((pos, n) => { car[pos] = Math.max(1, Math.min(30, numeri[n])); });
    sc.car = car;
    Object.defineProperty(sc, "_mancanti", { value: 6 - numeri.length, enumerable: false }); // non va nel JSON
    numeri = [...numeri, ...Array(6 - numeri.length).fill(0)]; // non cercarli altrove
  }
  // Se mancano i numeri sulle righe dopo l'intestazione, si cercano in tutto il blocco.
  if (numeri.length < 6) {
    const tutti = valoriCaratteristiche(righe.map((r) => r.testo).join(" "));
    if (tutti.length >= 6) sc.car = tutti.slice(0, 6).map((v) => Math.max(1, Math.min(30, v)));
  }
  // 3) tratti, azioni, reazioni, azioni leggendarie: ogni voce inizia con un nome seguito da un punto
  let elenco = sc.tratti, voce = null;
  let precedente = "";
  const nuovaVoce = (r) => {
    const t = r.testo;
    const evidenziato = r.font && r.font !== fontCorpo; // nome del tratto in grassetto o corsivo
    // una riga che continua una frase (la precedente non finisce con un punto) non apre una voce nuova
    if (voce && !evidenziato && !/[.!?)»”:]$/.test(precedente)) return null;
    const m = t.match(/^([A-ZÀ-ÝÈÉ][^.:]{0,70}?(?:\([^)]{0,40}\))?)\.\s*(.*)$/);
    if (!m) return null;
    const parole = m[1].split(/\s+/).length;
    if (evidenziato || parole <= 6) return { n: m[1].trim(), d: m[2].trim() };
    return null;
  };
  for (; i < righe.length; i++) {
    const r = righe[i], t = r.testo;
    const sez = SEZIONI.find(([, re]) => re.test(t));
    if (sez) { elenco = sc[sez[0]]; voce = null; precedente = ""; continue; }
    if (ETICHETTE.some(([k, re]) => k === "gs" && re.test(t))) { sc.gs = t.replace(ETICHETTE[11][1], "").trim(); continue; }
    const nv = nuovaVoce(r);
    precedente = t;
    if (nv) { voce = nv; elenco.push(voce); continue; }
    if (voce) voce.d = unisci(voce.d, t);
    else if (elenco === sc.legg || elenco === sc.reazioni || elenco === sc.azioni) { voce = { n: "", d: t }; elenco.push(voce); }
  }
  for (const l of [sc.tratti, sc.azioni, sc.reazioni, sc.legg]) for (const v of l) { v.n = v.n.slice(0, 120); v.d = v.d.slice(0, 5000); }
  for (const k of ["taglia", "tipo", "all", "ca", "pf", "vel", "gs"]) sc[k] = sc[k].slice(0, 200);
  for (const k of ["ts", "abil", "vuln", "res", "imm", "immc", "sensi", "ling"]) sc[k] = sc[k].slice(0, 600);
  return sc;
}
