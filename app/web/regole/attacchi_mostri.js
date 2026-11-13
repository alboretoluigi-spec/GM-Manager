// Profilo d'attacco di un mostro letto dalla sua scheda (bestiario della mappa, catalogo mostri.json, mostri importati
// dai manuali): attacchi con bonus e danni per tipo, numero di attacchi del Multiattacco, resistenze, immunità e
// vulnerabilità ai danni. Serve agli attacchi automatici del tracker (internal/server/attacchi.go, che ricontrolla tutto).
//
// Riconosce i due modi in cui sono scritte le schede:
//   lungo:   «Attacco con arma da mischia: +6 al tiro per colpire, portata 1,5 m, un bersaglio. Colpito: 11 (2d6 + 4) danni
//             perforanti più 3 (1d6) danni da fuoco.»
//   breve:   «Mischia +5, portata 1,5 m: 7 (1d10 + 2) contundenti più 3 (1d6) da freddo.»
// Le azioni che non sono attacchi con tiro per colpire (soffi, incantesimi con tiro salvezza…) restano fuori: il Master le
// gestisce a mano come prima.

const TIPI = [
  ["contundente", /^contundent[ei]$/], ["perforante", /^perforant[ei]$/], ["tagliente", /^taglient[ei]$/],
  ["fuoco", /^fuoco$/], ["freddo", /^freddo$/], ["fulmine", /^fulmin[ei]$|^elettric/], ["tuono", /^tuon[oi]$/],
  ["acido", /^acid[oi]$/], ["veleno", /^velen[oi]$/], ["necrotico", /^necrotic[oi]$/], ["radioso", /^radios[oi]$/],
  ["psichico", /^psichic[oi]$/], ["forza", /^forza$/],
];

/** «Taglienti», «danni da fuoco» → tipo del motore («tagliente», «fuoco»); sconosciuto: "". */
export function tipoDanno(t) {
  const p = String(t || "").toLowerCase().replace(/^danni\s+/, "").replace(/^da\s+/, "").trim();
  const x = TIPI.find(([, re]) => re.test(p));
  return x ? x[0] : "";
}

const NUMERI = { un: 1, uno: 1, una: 1, due: 2, tre: 3, quattro: 4, cinque: 5, sei: 6, sette: 7, otto: 8 };
const segno = (s) => (s === "-" || s === "−" ? -1 : 1);

/** Danni dal testo dopo «Colpito:» (o dopo i due punti nella forma breve). */
export function leggiDanni(t) {
  let s = String(t || "")
    .replace(/\(\d+\s*\([^)]*\)[^)]*\)/g, "") // alternativa tra parentesi: «(6 (1d8 + 2) contundenti a due mani)»
    .split(/[.;]\s|[.;]$/)[0]
    .split(/,?\s+o\s+\d/)[0]; // «…, o 5 (1d6 + 2) danni perforanti se impugnata a due mani»
  const danni = [];
  // «7 (1d10 + 2) contundenti», «1 danno perforante», «11 (1d6 + 3 più 1d8) perforanti», «15 (1d12 + 4 + 1d8) taglienti»
  const re = /(\d+)\s*(?:\(\s*(\d{1,2})d(\d{1,3})(?:\s*([+−-])\s*(\d+))?(?:\s*(?:più|\+)\s*(\d{1,2})d(\d{1,3}))?\s*\))?\s+(?:danni|danno)?\s*(?:da\s+)?([a-zàèéìòù]+)/gi;
  let m;
  while ((m = re.exec(s)) && danni.length < 6) {
    const tipo = tipoDanno(m[8]);
    if (!tipo && !/^(danni|danno)$/i.test(m[8])) { if (danni.length) break; continue; }
    if (m[2]) danni.push({ dadi: m[2] + "d" + m[3], mod: m[4] ? segno(m[4]) * +m[5] : 0, tipo });
    else danni.push({ dadi: "", mod: +m[1], tipo });
    if (m[6]) danni.push({ dadi: m[6] + "d" + m[7], mod: 0, tipo });
    // dopo un componente si continua solo con «più …» / «e …» / «, …»
    const resto = s.slice(re.lastIndex);
    if (!/^\s*(?:,\s*)?(?:più|e|piu|plus)\s+\d/i.test(resto)) break;
  }
  return danni;
}

/** Un'azione → profilo d'attacco, oppure null se non è un attacco con tiro per colpire. */
export function attaccoDaAzione(az, armiMagiche) {
  const nome = String(az.n || "").trim(), d = String(az.d || "").replace(/\s+/g, " ").trim();
  let bonus, dopo, portata = "", magica = !!armiMagiche;
  let m = d.match(/^Attacco con (arma|incantesimo)[^:]*:\s*([+−-]\s*\d+) al tiro per colpire(.*)$/i);
  if (m) {
    bonus = segno(m[2][0]) * parseInt(m[2].replace(/[^\d]/g, ""), 10);
    if (/incantesimo/i.test(m[1])) magica = true;
    // dopo «Colpito:»; nelle schede senza la parola, il primo numero dopo un punto («…un bersaglio. 7 (1d6 + 4) perforanti»)
    const c = m[3].match(/Colpito:\s*(.*)$/i) || m[3].match(/^.*?[.:]\s*(\d.*)$/);
    dopo = c ? c[1] : "";
    const p = m[3].match(/(portata [\d,]+ m|gittata [\d,/]+ m)/i);
    if (p) portata = p[1];
  } else {
    m = d.match(/^(Mischia o distanza|Mischia|Distanza)\s+([+−-]\s*\d+)([^:]*):\s*(.*)$/i);
    if (!m) return null;
    bonus = segno(m[2][0]) * parseInt(m[2].replace(/[^\d]/g, ""), 10);
    dopo = m[4];
    const p = m[3].match(/(portata [\d,]+ m|gittata [\d,/]+ m)/i);
    if (p) portata = p[1];
  }
  if (!nome || !(bonus >= -10 && bonus <= 30)) return null;
  // anche senza danni (ragnatela, presa…): il tiro per colpire serve lo stesso, l'effetto lo applica il Master
  return { nome: nome.slice(0, 60), bonus, portata, magica, danni: leggiDanni(dopo) };
}

/** Sequenza del Multiattacco (2026-11-12): «uno con il morso e due con gli artigli», «1 con il morso e 2 con gli artigli»,
 *  «tre attacchi con il tentacolo» → [{attacco: indice, volte}], collegata agli attacchi letti dalla scheda. null se il
 *  testo offre una scelta («con la stoccata o con gli aculei») o se un nome non corrisponde a nessun attacco. */
export function sequenzaMultiattacco(t, attacchi) {
  // le frasi sulle sostituzioni («Può sostituire un attacco con la spada con un colpo senz'armi») non sono la sequenza
  const s = String(t || "").toLowerCase().split(/(?<=\.)\s+/).filter((f) => !/sostitui|al posto d|invece d/.test(f)).join(" ");
  if (/\bo con\b|\boppure\b/.test(s) || !attacchi || !attacchi.length) return null;
  const radice = (w) => normalizzaNome(w).slice(0, 5);
  const trova = (parola) => {
    const r = radice(parola);
    if (r.length < 3) return -1;
    return attacchi.findIndex((a) => normalizzaNome(a.nome).split(/\s+/).some((x) => x.slice(0, 5) === r || x.startsWith(r) || r.startsWith(x.slice(0, 4)) && x.length >= 4));
  };
  const passi = [];
  const re = /\b(\d|uno|una|un|due|tre|quattro|cinque)\s+(?:attacchi\s+|attacco\s+)?con\s+(?:il|lo|la|i|gli|le|l'|la sua|il suo|i suoi|le sue)?\s*([a-zàèéìòù]+)/g;
  for (const m of s.matchAll(re)) {
    const volte = /^\d$/.test(m[1]) ? +m[1] : (NUMERI[m[1]] || 1);
    const i = trova(m[2]);
    if (i < 0) return null;
    const g = passi.find((x) => x.attacco === i);
    if (g) g.volte += volte; else passi.push({ attacco: i, volte });
  }
  if (!passi.length) { // «Tre attacchi: morso e due artigli», «zoccoli, artigli e coda seghettata»
    const m = s.match(/attacchi\s*:\s*([^.;]+)/);
    if (m) {
      for (const voce of m[1].split(/,\s*|\s+e\s+/)) {
        const w = voce.trim().split(/\s+/);
        if (!w[0]) continue;
        let volte = 1;
        if (/^\d$/.test(w[0]) || NUMERI[w[0]] || ["uno", "una", "un"].includes(w[0])) { volte = /^\d$/.test(w[0]) ? +w[0] : (NUMERI[w[0]] || 1); w.shift(); }
        if (["con", "il", "lo", "la", "i", "gli", "le"].includes(w[0])) w.shift();
        const i = w[0] ? trova(w[0]) : -1;
        if (i < 0) return null;
        const g = passi.find((x) => x.attacco === i);
        if (g) g.volte += volte; else passi.push({ attacco: i, volte });
      }
    }
  }
  if (!passi.length) return null;
  if (passi.reduce((a, b) => a + b.volte, 0) > 10) return null;
  return passi;
}
const normalizzaNome = (t) => String(t || "").toLowerCase().normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/[^a-z\s]/g, " ").trim();

/** Attacchi per azione dal testo del Multiattacco («Effettua tre attacchi», «1 con il morso e 1 con gli artigli»). */
export function attacchiMultiattacco(t) {
  const s = String(t || "").toLowerCase();
  const n = s.match(/\b(\d)\s+(?:attacchi|colpi)\b/); // «2 attacchi in mischia», «2 attacchi con l'artiglio»
  if (n) return +n[1];
  const cifre = [...s.matchAll(/\b(\d)\s+con\b/g)].map((x) => +x[1]); // «1 con il morso e 1 con gli artigli»
  if (cifre.length) return Math.min(10, cifre.reduce((a, b) => a + b, 0));
  const p = s.match(/\b(due|tre|quattro|cinque|sei|sette|otto)\s+(?:attacchi|volte|colpi)/);
  return p ? NUMERI[p[1]] : 0;
}

/** «freddo; contundenti, perforanti e taglienti da attacchi non magici» → [{tipo:"freddo"}, {tipo:"contundente", nonMagici:true}, …] */
export function leggiDifese(t) {
  const out = [], visti = new Set();
  for (const parte of String(t || "").split(/;/)) {
    const nonMagici = /non magic/i.test(parte);
    const nonArgentate = nonMagici && /argentat/i.test(parte); // «da attacchi non magici non argentati» (lupi mannari…)
    for (const w of parte.toLowerCase().split(/[^a-zàèéìòù]+/)) {
      const tipo = tipoDanno(w);
      if (tipo && !visti.has(tipo)) { visti.add(tipo); out.push(nonArgentate ? { tipo, nonMagici, nonArgentate } : nonMagici ? { tipo, nonMagici } : { tipo }); }
    }
  }
  return out;
}

const CAR = ["FOR", "DES", "COS", "INT", "SAG", "CAR"];
const ABBR = { for: "FOR", des: "DES", cos: "COS", int: "INT", sag: "SAG", car: "CAR" };
const CONDIZIONI = ["Accecato", "Affascinato", "Assordato", "Spaventato", "Afferrato", "Incapacitato", "Invisibile", "Paralizzato",
  "Pietrificato", "Avvelenato", "Prono", "Trattenuto", "Stordito", "Privo di sensi"];

/** Bonus ai tiri salvezza: dai punteggi delle caratteristiche, sostituiti da quelli scritti nella scheda («Des +5, Sag +4»). */
export function tiriSalvezza(scheda) {
  const ts = {};
  const car = Array.isArray(scheda.car) ? scheda.car : [];
  CAR.forEach((k, i) => { if (+car[i] >= 1 && +car[i] <= 30) ts[k] = Math.floor((+car[i] - 10) / 2); });
  for (const m of String(scheda.ts || "").matchAll(/\b(For|Des|Cos|Int|Sag|Car)[a-zà]*\s*([+−-])\s*(\d+)/gi)) ts[ABBR[m[1].toLowerCase()]] = segno(m[2]) * +m[3];
  return ts;
}

/** Immunità alle condizioni («avvelenato, spaventato») → nomi del tracker. */
export function immunitaCondizioni(t) {
  const s = String(t || "").toLowerCase();
  return CONDIZIONI.filter((c) => new RegExp("\\b" + c.toLowerCase().slice(0, -1) + "[oaie]?\\b").test(s));
}

const NOMI_CAR = { forza: "FOR", destrezza: "DES", costituzione: "COS", intelligenza: "INT", saggezza: "SAG", carisma: "CAR" };

/** Incantesimi della creatura (tratti «Incantesimi», «Incantesimi innati»): CD, bonus d'attacco, modificatore, livello da
 *  incantatore e gruppi («Trucchetti», «3° (3)» = tre slot di 3°, «1° (a volontà)», «A volontà», «3/giorno»). */
export function incantesimiMostro(tratti, car) {
  const gruppi = [];
  let cd = 0, att = 0, mod = 0, liv = 0;
  for (const t of tratti || []) {
    if (!/^incantesimi/i.test(String(t.n || "").trim())) continue;
    const d = String(t.d || "");
    // CD, attacco e caratteristica valgono per i gruppi di QUESTO tratto (un mago drow ha gli innati con CD 12 e quelli da
    // mago con CD 14 e +6): si salvano in ogni gruppo; i valori della creatura sono quelli del primo tratto che li ha.
    const m = /CD (?:del tiro salvezza degli incantesimi )?(\d+)(?:,\s*([+−-]\s*\d+) (?:a colpire|al tiro per colpire))?/i.exec(d);
    const c = /\b(Forza|Destrezza|Costituzione|Intelligenza|Saggezza|Carisma)\b/i.exec(d);
    const l = /(?:di|del) (\d+)° livello/i.exec(d);
    const tCd = m ? +m[1] : 0, tAtt = m && m[2] ? segno(m[2].trim()[0]) * +m[2].replace(/[^\d]/g, "") : 0;
    let tMod = 0;
    if (c) { const i = CAR.indexOf(NOMI_CAR[c[1].toLowerCase()]); if (i >= 0 && +car[i]) tMod = Math.floor((+car[i] - 10) / 2); }
    if (tCd && !cd) cd = tCd;
    if (tAtt && !att) att = tAtt;
    if (tMod && !mod) mod = tMod;
    if (l && !liv) liv = +l[1];
    const re = /(Trucchetti|A volontà|(\d)\/giorno(?: ciascuno)?|(\d)° \((\d) slot|(\d)° \((\d)\)|(\d)° \(a volontà\)):\s*([^\n]*?)(?=\s*(?:\n|\.\s+(?:Trucchetti|A volontà|\d\/giorno|\d° \()|\.?\s*\*|\.?$))/gi;
    let g;
    while ((g = re.exec(d)) && gruppi.length < 12) {
      const nomi = g[8].replace(/\s*\([^)]*\)?/g, "").split(/,\s*/).map((x) => x.replace(/\*/g, "").trim()).filter((x) => x && x.length <= 60).slice(0, 20);
      if (!nomi.length) continue;
      let etichetta, livello, usi;
      if (/^trucchetti/i.test(g[1])) { etichetta = "Trucchetti"; livello = 0; usi = 0; }
      else if (/^a volontà/i.test(g[1])) { etichetta = "A volontà"; livello = -1; usi = 0; }
      else if (g[2]) { etichetta = g[2] + "/giorno"; livello = -1; usi = +g[2]; }
      else if (g[3]) { etichetta = g[3] + "°"; livello = +g[3]; usi = +g[4]; }
      else if (g[5]) { etichetta = g[5] + "°"; livello = +g[5]; usi = +g[6]; }
      else { etichetta = g[7] + "° (a volontà)"; livello = +g[7]; usi = 0; }
      const x = { etichetta, livello, usi, nomi };
      if (tCd) x.cd = tCd;
      if (tAtt) x.att = tAtt;
      if (tMod) x.mod = tMod;
      gruppi.push(x);
    }
  }
  return gruppi.length ? { cd, att, mod, liv, gruppi } : null;
}

/** Profilo completo per il tracker (null se non c'è niente di utile). */
export function profiloMostro(scheda) {
  if (!scheda || typeof scheda !== "object") return null;
  const tratti = Array.isArray(scheda.tratti) ? scheda.tratti : [];
  const armiMagiche = tratti.some((x) => /armi magiche/i.test(x.n || "") || /attacchi con (?:arma|armi)[^.]*sono magici/i.test(x.d || ""));
  const azioni = Array.isArray(scheda.azioni) ? scheda.azioni : [];
  const attacchi = [];
  let multi = 0, testoMulti = "";
  for (const az of azioni) {
    if (/^multiattacco/i.test(String(az.n || "").trim())) { multi = attacchiMultiattacco(az.d); testoMulti = az.d; continue; }
    const a = attaccoDaAzione(az, armiMagiche);
    if (a && attacchi.length < 30) attacchi.push(a);
  }
  const difese = { res: leggiDifese(scheda.res), imm: leggiDifese(scheda.imm), vuln: leggiDifese(scheda.vuln) };
  const ts = tiriSalvezza(scheda);
  if (!attacchi.length && !difese.res.length && !difese.imm.length && !difese.vuln.length && !Object.keys(ts).length) return null;
  const p = { attacchi, multi: multi > 1 ? multi : 0, difese, ts };
  // sequenza del Multiattacco (un morso e due artigli…): Gaster Bless propone l'attacco successivo e tiene il conto per tipo
  const seq = multi > 1 ? sequenzaMultiattacco(testoMulti, attacchi) : null;
  if (seq && seq.reduce((a, b) => a + b.volte, 0) === multi) p.sequenza = seq;
  // contro gli incantesimi (internal/server/incantesimi_scontro.go)
  if (tratti.some((x) => /resistenza alla magia/i.test(x.n || ""))) p.resMagia = true;
  const leg = tratti.map((x) => /resistenza leggendaria\s*\((\d)\s*\/\s*giorno\)/i.exec(x.n || "")).find(Boolean);
  if (leg) p.leggendaria = +leg[1];
  const imm = immunitaCondizioni(scheda.immc);
  if (imm.length) p.immCond = imm;
  const inc = incantesimiMostro(tratti, scheda.car || []);
  if (inc) p.incantesimi = inc;
  return p;
}
