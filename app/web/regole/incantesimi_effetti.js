/* ===== LETTORE DEGLI EFFETTI DEGLI INCANTESIMI (quinta edizione 2014) — Gaster Bless, 2026-11-11, Parte 2 del piano di combattimento =====
   Modulo puro (nessun accesso alla pagina): definisce la variabile globale IncantesimiEffetti. Lo usano la scheda
   (finestra dell'incantesimo, scelta dello slot) e, nelle fasi successive, il tracker e le schede dei mostri.
   Legge i dati di SPELLS (web/regole/dati.js) nella forma della scheda: {it, en, l, sc, t, r, cp, d, cl, src, ds, hi}.
   Le descrizioni sono riassunti scritti con parole nostre in forma regolare («TS su Destrezza o 8d6 danni da fuoco (metà
   se riesce)», «Attacco a distanza con incantesimo: 1d10 danni da fuoco», livelli superiori «+1d6 per ogni slot sopra
   il 3°»): qui si riconosce SOLO ciò che è scritto. Quando l'effetto non si legge con certezza, «automatico» è falso e
   l'effetto lo decide il Master con la descrizione (regola della specifica: nessuna risoluzione inventata). */
const IncantesimiEffetti = (function () {
  'use strict';
  const CAR = { Forza: 'FOR', Destrezza: 'DES', Costituzione: 'COS', Intelligenza: 'INT', Saggezza: 'SAG', Carisma: 'CAR' };
  const TIPI = [['acido', 'acido'], ['contundent', 'contundente'], ['freddo', 'freddo'], ['fuoco', 'fuoco'], ['forza', 'forza'], ['fulmin', 'fulmine'],
    ['necrotic', 'necrotico'], ['perforant', 'perforante'], ['veleno', 'veleno'], ['psichic', 'psichico'], ['radios', 'radioso'], ['taglient', 'tagliente'], ['tuono', 'tuono']];
  const RE_TIPO = TIPI.map((x) => x[0]).join('|');
  const tipoDa = (t) => { t = String(t || '').toLowerCase(); const x = TIPI.find(([k]) => t.startsWith(k)); return x ? x[1] : ''; };
  /* condizioni ufficiali (nomi come nel tracker: internal/server/personaggi.go, condizioniAmmesse) */
  const CONDIZIONI = [[/\baccecat[oaie]\b/i, 'Accecato'], [/\baffascinat[oaie]\b/i, 'Affascinato'], [/\bassordat[oaie]\b/i, 'Assordato'],
    [/\bspaventat[oaie]\b/i, 'Spaventato'], [/\bafferrat[oaie]\b/i, 'Afferrato'], [/\bincapacitat[oaie]\b/i, 'Incapacitato'],
    [/\binvisibil[ei]\b/i, 'Invisibile'], [/\bparalizzat[oaie]\b/i, 'Paralizzato'], [/\bpietrificat[oaie]\b/i, 'Pietrificato'],
    [/\bavvelenat[oaie]\b/i, 'Avvelenato'], [/\bpron[oaie]\b/i, 'Prono'], [/\btrattenut[oaie]\b/i, 'Trattenuto'],
    [/\bstordit[oaie]\b/i, 'Stordito'], [/\bpriv[oaie] di sensi\b/i, 'Privo di sensi']];
  /* Effetti che cambiano i valori (Parte 2, F2, 2026-11-12): i potenziamenti più usati, per nome inglese, riassunti con
     parole nostre. mod: modifiche come nel server (internal/server/effetti_valori.go); scegli: il tipo di danno della
     resistenza si sceglie al lancio; cond: condizioni sui bersagli finché dura; nota: ciò che resta al Master. */
  const SEI = ['FOR', 'DES', 'COS', 'INT', 'SAG', 'CAR'];
  const VALORI = {
    'Mage Armor': { mod: { caBase: 13 }, nota: 'vale solo senza armatura' },
    'Shield of Faith': { mod: { caBonus: 2 } },
    'Shield': { mod: { caBonus: 5 }, nota: 'niente danni da dardo incantato' },
    'Haste': { mod: { caBonus: 2, vantTS: ['DES'] }, nota: 'velocità doppia e un\'azione in più; quando finisce perde un turno' },
    'Barkskin': { mod: { caMin: 16 } },
    'Stoneskin': { mod: { res: ['contundente (non magici)', 'perforante (non magici)', 'tagliente (non magici)'] } },
    'Protection from Energy': { scegli: ['acido', 'freddo', 'fuoco', 'fulmine', 'tuono'] },
    'Blur': { mod: { svantContro: true }, nota: 'non vale contro chi ha vista cieca o visione del vero' },
    'Bless': { mod: { dadoAtt: '1d4', dadoTS: '1d4' } },
    'Heroism': { mod: { immCond: ['Spaventato'], pfTempTurno: 'inc' } },
    'Bane': { mod: { dadoAtt: '-1d4', dadoTS: '-1d4' }, tsCar: 'CAR' },
    'Warding Bond': { mod: { caBonus: 1, bonusTS: 1, res: TIPI.map((x) => x[1]) }, nota: 'chi lo lancia subisce gli stessi danni: a mano' },
    'Holy Aura': { mod: { vantTS: SEI, svantContro: true } },
    'Foresight': { mod: { vantTS: SEI, svantContro: true }, nota: 'vantaggio anche ai tiri per colpire: sceglilo quando attacca' },
    'Fire Shield': { scegli: ['freddo', 'fuoco'], ritorsione: { dadi: '2d8', opposta: true }, nota: 'scudo caldo: resistenza al freddo e 2d8 da fuoco a chi lo colpisce in mischia; scudo freddo: resistenza al fuoco e 2d8 da freddo' },
    'Greater Invisibility': { cond: ['Invisibile'] },
    'Invisibility': { cond: ['Invisibile'], nota: 'finisce se attacca o lancia un incantesimo: toglila a mano' },
  };
  const NUMERI = { una: 1, uno: 1, un: 1, due: 2, tre: 3, quattro: 4, cinque: 5, sei: 6, sette: 7, otto: 8, nove: 9, dieci: 10, dodici: 12 };
  const num = (x) => (/^\d+$/.test(String(x)) ? +x : NUMERI[String(x).toLowerCase()] || 0);
  const UNITA = { r: ['round', 1], m: ['minuti', 10], h: ['ore', 600], g: ['giorni', 14400], d: ['giorni', 14400] };

  function durata(code) {
    const c = String(code || '');
    const m = /^(C)?(\d+)([rmhgd])$/.exec(c);
    if (m) return { conc: !!m[1], quanti: +m[2], unita: UNITA[m[3]][0], round: +m[2] * UNITA[m[3]][1], istantanea: false };
    return { conc: c[0] === 'C', quanti: 0, unita: '', round: 0, istantanea: c === 'I', testo: c };
  }

  /* danni: «8d6 danni da fuoco», «1d10 danni da fuoco», «2d8 + 1d6 danni…», «4d4 danni da acido» */
  function danniDa(testo) {
    const out = [];
    const re = new RegExp('(\\d{1,2}d\\d{1,3})(?:\\s*\\+\\s*(\\d+|mod\\. da incantatore))?\\s+danni\\s+(?:da\\s+|di\\s+)?(' + RE_TIPO + ')[a-z]*', 'gi');
    let m;
    while ((m = re.exec(testo))) out.push({ dadi: m[1], mod: m[2] === 'mod. da incantatore' ? 'inc' : m[2] ? +m[2] : 0, tipo: tipoDa(m[3]) });
    return out;
  }

  /* livelli superiori: «+1d6 per ogni slot sopra il 3°», «Un umanoide aggiuntivo per ogni slot sopra il 2°», trucchetti */
  function superiori(hi, l) {
    const h = String(hi || ''), out = { testo: h };
    let m;
    if (l === 0 && (m = /(\d+d\d+)\s+al 5°,\s*(\d+d\d+)\s+all'11°,\s*(\d+d\d+)\s+al 17°/.exec(h))) out.trucchetto = [[5, m[1]], [11, m[2]], [17, m[3]]];
    if ((m = /^\+(\d+d\d+)(?:[^.]*?)per ogni slot sopra il (\d)°/i.exec(h))) out.dadiPerSlot = m[1];
    if ((m = /^\+(\d+)\s+(?:PF|punti ferita) temporanei/i.exec(h))) out.pfTempPerSlot = +m[1];
    if (/^(?:Un|Una|Uno)\s+[^.]{0,30}?aggiuntiv[oa] per ogni slot sopra/i.test(h)) out.bersagliPerSlot = 1;
    out.leggibile = !!(out.trucchetto || out.dadiPerSlot || out.pfTempPerSlot || out.bersagliPerSlot);
    return out;
  }

  function analizza(s) {
    if (!s) return null;
    const ds = String(s.ds || ''), low = ds.toLowerCase();
    const r = { nome: s.it, livello: s.l, rituale: String(s.t || '')[0] === '*', durata: durata(s.d), categorie: [], automatico: false };
    r.conc = r.durata.conc;
    let m;
    if ((m = /attacco\s+(in mischia|a distanza)?\s*con incantesimo/i.exec(ds))) r.attacco = { tipo: m[1] && m[1].toLowerCase() === 'in mischia' ? 'mischia' : 'distanza' };
    if ((m = /\bTS su (Forza|Destrezza|Costituzione|Intelligenza|Saggezza|Carisma)\b/.exec(ds))) {
      r.ts = { car: CAR[m[1]], nome: m[1], seRiesce: /metà se riesce|dimezzat/i.test(ds) ? 'meta' : 'nulla', ripetuto: /ripete il TS alla fine di ogni/i.test(ds) };
    }
    r.danni = danniDa(ds);
    if ((m = /recupera(?:no)?\s+(\d{1,2}d\d{1,3}|\d+)(\s*\+\s*mod\. da incantatore)?\s+punti ferita/i.exec(ds))) r.cura = { dadi: m[1], modInc: !!m[2] };
    if ((m = /(\d{1,2}d\d{1,3}|\d+)(\s*\+\s*mod\. da incantatore)?\s+(?:punti ferita|PF) temporanei/i.exec(ds))) r.pfTemp = { dadi: m[1], modInc: !!m[2] };
    r.condizioni = [...new Set(CONDIZIONI.filter(([re]) => re.test(ds)).map(([, n]) => n))];
    if ((m = /\b(sfera|cono|linea|cubo|cilindro|emanazione)\s+(?:di|da|lunga)\s+(\d+(?:,\d+)?)\s*m/i.exec(ds))) r.area = { forma: m[1].toLowerCase(), misura: m[2] + ' m' };
    else if ((m = /\bentro (\d+(?:,\d+)?) m (?:da te|dal punto|attorno)/i.exec(ds))) r.area = { forma: 'raggio', misura: m[1] + ' m' };
    if ((m = /\bfino a (una|due|tre|quattro|cinque|sei|sette|otto|nove|dieci|dodici|\d+) creature\b/i.exec(ds))) r.bersagli = num(m[1]);
    else if (/^(?:Una creatura|Un umanoide|Un bersaglio|Una creatura che tocchi)\b/i.test(ds)) r.bersagli = 1;
    if ((m = /CA base (\d+)\s*\+\s*(?:il\s+)?modificatore di Destrezza/i.exec(ds))) r.ca = { base: +m[1], des: true };
    else if ((m = /\+(\d+) (?:alla|a) CA\b/i.exec(ds))) r.ca = { bonus: +m[1] };
    /* effetto che cambia i valori: dalla tabella, altrimenti dalla CA letta nel testo */
    const val = VALORI[s.en];
    if (val || r.ca) {
      r.valori = { mod: Object.assign({}, r.ca ? (r.ca.base ? { caBase: r.ca.base } : { caBonus: r.ca.bonus }) : {}, val && val.mod), scegli: (val && val.scegli) || null,
        cond: (val && val.cond) || [], nota: (val && val.nota) || '', soloSe: String(s.r || '').trim() === 'Sé',
        tsCar: (val && val.tsCar) || '', ritorsione: (val && val.ritorsione) || null };
      if (!r.valori.tsCar) delete r.valori.tsCar;
      if (!r.valori.ritorsione) delete r.valori.ritorsione;
    }
    r.superiori = superiori(s.hi, s.l);

    /* categorie (anche più di una) */
    const cat = (c, ok) => { if (ok && !r.categorie.includes(c)) r.categorie.push(c); };
    cat('attacco', !!r.attacco);
    cat('area', !!r.area || (r.bersagli || 0) > 1);
    cat('cura', !!r.cura);
    cat('difensivo', !!r.ca || !!r.pfTemp || /\bresistenza ai danni\b|\bresistenza a\b/.test(low));
    cat('potenziamento', /aggiung[oe]n?o? \d+d\d+ ai tiri|vantaggio (?:alle|ai)|velocità (?:aumenta|raddoppia)|\+\d+ ai tiri/.test(low));
    cat('indebolimento', /sottra(?:e|ggono) \d+d\d+|svantaggio (?:alle|ai)|velocità (?:dimezzat|ridott)/.test(low));
    cat('condizioni', r.condizioni.length > 0);
    cat('controllo', r.condizioni.some((c) => ['Paralizzato', 'Trattenuto', 'Affascinato', 'Stordito', 'Incapacitato', 'Prono', 'Spaventato'].includes(c)));
    cat('evocazione', /\bevoc|\brichiam|\bcompare (?:un|una)|\bcrea (?:un|una) (?:creatura|servitore)/.test(low));
    cat('movimento', /teletrasport|\bvola\b|velocità di volare|\bsalt/.test(low));
    cat('persistente', !r.durata.istantanea && r.durata.round > 1);
    const automatizzabile = !!r.attacco || (!!r.ts && (r.danni.length > 0 || r.condizioni.length > 0)) || !!r.cura || !!r.pfTemp || !!r.ca || !!r.valori;
    r.automatico = automatizzabile;
    cat('speciale', !automatizzabile || r.categorie.length === 0);
    return r;
  }

  /* dadi «NdM» → {n, f} */
  const dado = (d) => { const m = /^(\d+)d(\d+)$/.exec(String(d)); return m ? { n: +m[1], f: +m[2] } : null; };
  const aggiungiDadi = (base, extra, volte) => { const a = dado(base), b = dado(extra); if (!a || !b || volte <= 0) return base; return a.f === b.f ? (a.n + b.n * volte) + 'd' + a.f : base; };

  /* danni al livello di slot scelto (e al livello del personaggio, per i trucchetti) */
  function danniAlLivello(r, slot, livelloPG) {
    if (!r) return [];
    const l = r.livello, out = r.danni.map((d) => ({ ...d }));
    if (!out.length) return out;
    const sup = r.superiori || {};
    if (l === 0 && sup.trucchetto) { let dd = out[0].dadi; sup.trucchetto.forEach(([lv, d]) => { if ((livelloPG || 1) >= lv) dd = d; }); out[0].dadi = dd; }
    else if (sup.dadiPerSlot && slot > l) {
      const nuovo = aggiungiDadi(out[0].dadi, sup.dadiPerSlot, slot - l);
      if (nuovo !== out[0].dadi) out[0].dadi = nuovo; else out.push({ dadi: (dado(sup.dadiPerSlot).n * (slot - l)) + 'd' + dado(sup.dadiPerSlot).f, mod: 0, tipo: out[0].tipo });
    }
    return out;
  }
  function curaAlLivello(r, slot) {
    if (!r || !r.cura) return null;
    const c = { ...r.cura }, sup = r.superiori || {};
    if (sup.dadiPerSlot && slot > r.livello) c.dadi = aggiungiDadi(c.dadi, sup.dadiPerSlot, slot - r.livello);
    return c;
  }
  function pfTempAlLivello(r, slot) {
    if (!r || !r.pfTemp) return null;
    const p = { ...r.pfTemp }, sup = r.superiori || {};
    if (sup.pfTempPerSlot && slot > r.livello && /^\d+$/.test(p.dadi)) p.dadi = String(+p.dadi + sup.pfTempPerSlot * (slot - r.livello));
    else if (sup.dadiPerSlot && slot > r.livello) p.dadi = aggiungiDadi(p.dadi, sup.dadiPerSlot, slot - r.livello);
    return p;
  }
  const bersagliAlLivello = (r, slot) => (r && r.bersagli ? r.bersagli + (r.superiori && r.superiori.bersagliPerSlot && slot > r.livello ? slot - r.livello : 0) : null);

  /* testo breve dell'effetto a un livello (finestra della scelta dello slot) */
  function riassunto(r, slot, livelloPG, modInc) {
    if (!r) return '';
    const p = [];
    const fmt = (d) => d.dadi + (d.mod === 'inc' ? (modInc != null ? (modInc >= 0 ? '+' : '') + modInc : ' + mod.') : d.mod ? '+' + d.mod : '') + ' ' + d.tipo;
    const D = danniAlLivello(r, slot, livelloPG);
    if (D.length) p.push(D.map(fmt).join(' + '));
    const c = curaAlLivello(r, slot); if (c) p.push('cura ' + c.dadi + (c.modInc ? (modInc != null ? (modInc >= 0 ? '+' : '') + modInc : ' + mod.') : '') + ' PF');
    const t = pfTempAlLivello(r, slot); if (t) p.push(t.dadi + (t.modInc ? (modInc != null ? (modInc >= 0 ? '+' : '') + modInc : ' + mod.') : '') + ' PF temporanei');
    const b = bersagliAlLivello(r, slot); if (b && (b > 1 || (r.superiori && r.superiori.bersagliPerSlot))) p.push(b + (b === 1 ? ' bersaglio' : ' bersagli'));
    return p.join(' · ');
  }

  /** Le modifiche di un potenziamento in breve (come testoModifiche di web/ui/effetti.js), per la finestra della scheda. */
  function testoValori(v) {
    if (!v) return '';
    const m = v.mod || {}, p = [], nomeCar = { FOR: 'Forza', DES: 'Destrezza', COS: 'Costituzione', INT: 'Intelligenza', SAG: 'Saggezza', CAR: 'Carisma' };
    const segno = (d) => (String(d).startsWith('-') ? '−' + String(d).slice(1) : '+' + d);
    if (m.caBase) p.push('CA ' + m.caBase + ' + Destrezza');
    if (m.caBonus) p.push('CA ' + (m.caBonus > 0 ? '+' : '−') + Math.abs(m.caBonus));
    if (m.caMin) p.push('CA almeno ' + m.caMin);
    if (m.res && m.res.length) p.push(m.res.length >= TIPI.length ? 'resistenza a tutti i danni' : 'resistenza: ' + m.res.join(', '));
    if (v.scegli) p.push('resistenza a un tipo a scelta (' + v.scegli.join(', ') + ')');
    if (m.immCond && m.immCond.length) p.push('immune a: ' + m.immCond.join(', ').toLowerCase());
    if (m.vantTS && m.vantTS.length) p.push(m.vantTS.length === 6 ? 'vantaggio a tutti i TS' : 'vantaggio ai TS su ' + m.vantTS.map((c) => nomeCar[c]).join(', '));
    if (m.bonusTS) p.push('TS ' + (m.bonusTS > 0 ? '+' : '−') + Math.abs(m.bonusTS));
    if (m.dadoTS) p.push(segno(m.dadoTS) + ' ai TS');
    if (m.dadoAtt) p.push(segno(m.dadoAtt) + ' ai tiri per colpire');
    if (m.svantContro) p.push('chi lo attacca ha svantaggio');
    if (v.cond && v.cond.length) p.push(v.cond.join(', ').toLowerCase());
    return p.join(', ');
  }

  /** Effetto per lo scontro di un potenziamento (tipo «effetto»): da aggiungere ai dati comuni del lancio. */
  function effettoValori(r) {
    if (!r || !r.valori) return null;
    const e = { tipo: 'effetto', mod: r.valori.mod, scegli: r.valori.scegli, cond: r.valori.cond, nota: r.valori.nota, soloSe: r.valori.soloSe };
    if (r.valori.tsCar) e.tsCar = r.valori.tsCar; // Anatema: TS dei bersagli
    if (r.valori.ritorsione) e.ritorsione = r.valori.ritorsione; // Scudo di fuoco
    return e;
  }

  return { analizza, durata, danniAlLivello, curaAlLivello, pfTempAlLivello, bersagliAlLivello, riassunto, effettoValori, testoValori, VALORI, CONDIZIONI: CONDIZIONI.map((x) => x[1]) };
})();
if (typeof module !== 'undefined') module.exports = IncantesimiEffetti;
