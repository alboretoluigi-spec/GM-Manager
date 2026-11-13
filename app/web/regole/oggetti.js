/* ===== OGGETTI MAGICI (quinta edizione, regole 2014) =====
   Modulo puro (nessun accesso alla pagina): modello dati della «Scheda oggetto magico», calcolo dei
   modificatori automatici, slot, sintonizzazione, importazione/esportazione e controllo dei dati.
   Definisce la variabile globale OggettiMagici. Lo usano la scheda (web/scheda/) e i test (test/regole/).

   REGOLA FONDAMENTALE (specifica del Master): i valori base del personaggio non vengono MAI sovrascritti.
   Qui si calcolano solo differenze («effetti»), ognuna legata all'oggetto che la genera; la scheda le somma
   al momento di mostrare i numeri. Se l'oggetto si toglie o non è più attivo, l'effetto sparisce da solo.

   Una «voce» dell'inventario magico del personaggio è: {o: oggetto, eq: equipaggiato, sint: sintonizzato,
   on: attivato, car: cariche rimaste}. L'oggetto (o) ha la forma descritta in nuovoOggetto(). */
const OggettiMagici = (function () {
  'use strict';
  const FORMATO = 'gasterbless-oggetto', VERSIONE_FORMATO = 1;
  const AB = ['FOR', 'DES', 'COS', 'INT', 'SAG', 'CAR'];
  const ABN = { FOR: 'Forza', DES: 'Destrezza', COS: 'Costituzione', INT: 'Intelligenza', SAG: 'Saggezza', CAR: 'Carisma' };
  const MAX_SINTONIE = 3;      // regola di quinta edizione: al massimo tre oggetti sintonizzati
  const MAX_VOCI = 100;        // oggetti magici per personaggio
  const MAX_IMG = 200000;      // caratteri massimi dell'immagine (data URL) di un oggetto

  /* ---------- vocabolari ---------- */
  const TIPI = [['arma', 'Arma'], ['armatura', 'Armatura'], ['scudo', 'Scudo'], ['anello', 'Anello'], ['amuleto', 'Amuleto'], ['mantello', 'Mantello'],
    ['cintura', 'Cintura'], ['guanti', 'Guanti'], ['stivali', 'Stivali'], ['elmo', 'Elmo'], ['bacchetta', 'Bacchetta'], ['bastone', 'Bastone'],
    ['meraviglioso', 'Oggetto meraviglioso'], ['accessorio', 'Complemento / accessorio'], ['altro', 'Oggetto personalizzato']];
  const RARITA = [['comune', 'Comune'], ['non_comune', 'Non comune'], ['raro', 'Raro'], ['molto_raro', 'Molto raro'],
    ['leggendario', 'Leggendario'], ['artefatto', 'Artefatto']];
  /* slot di equipaggiamento: [codice, nome, quanti ne entrano (0 = nessun limite)] */
  const SLOT = [['armatura', 'Armatura', 1], ['scudo', 'Scudo', 1], ['mani', 'Armi (mani)', 2], ['testa', 'Testa', 1], ['collo', 'Collo', 1],
    ['mantello', 'Mantello', 1], ['anello', 'Anelli', 2], ['cintura', 'Cintura', 1], ['guanti', 'Guanti e bracciali', 1],
    ['stivali', 'Stivali', 1], ['altro', 'Altro (nessun limite)', 0]];
  const SLOT_DI_TIPO = { arma: 'mani', armatura: 'armatura', scudo: 'scudo', anello: 'anello', amuleto: 'collo', mantello: 'mantello', cintura: 'cintura', guanti: 'guanti',
    stivali: 'stivali', elmo: 'testa', bacchetta: 'altro', bastone: 'mani', meraviglioso: 'altro', accessorio: 'altro', altro: 'altro' };
  const ATTIVAZIONI = [['passiva', 'Passiva'], ['azione', 'Azione'], ['azione_bonus', 'Azione bonus'], ['reazione', 'Reazione'], ['altro', 'Altro']];
  const OPERAZIONI = [['agg', 'Aggiungi valore'], ['sost', 'Sostituisci valore'], ['min', 'Imposta valore minimo'], ['max', 'Imposta valore massimo'], ['formula', 'Formula'],
    ['vant', 'Applica vantaggio'], ['svant', 'Applica svantaggio'], ['resist', 'Applica resistenza'], ['immun', 'Applica immunità'], ['speciale', 'Effetto speciale']];
  const TIPI_DANNO = ['acido', 'contundente', 'freddo', 'fuoco', 'forza', 'fulmine', 'necrotico', 'perforante', 'veleno', 'psichico', 'radioso', 'tagliente', 'tuono'];
  const CONDIZIONI = [['equip', 'Equipaggiato'], ['sint', 'Sintonizzato'], ['att', 'Attivato'], ['sempre', 'Portato con sé']];
  const DURATE = [['perm', 'Permanente'], ['temp', 'Temporanea (solo se attivato)']];
  const NUM = ['agg', 'sost', 'min', 'max', 'formula'], TESTO = ['resist', 'immun', 'speciale'];
  const SOLO_SENZA = 'senza_armatura';

  const nomiAbilita = () => (typeof SKILLS !== 'undefined' ? SKILLS.map((s) => s[0]) : []);
  /* parametri che un modificatore può toccare: [codice, nome, operazioni ammesse] */
  function parametri() {
    const P = [['CA', 'Classe Armatura', ['agg', 'sost', 'min', 'max', 'formula']], ['INIZIATIVA', 'Iniziativa', ['agg', 'formula', 'vant', 'svant']],
      ['VELOCITA', 'Velocità (metri)', ['agg', 'min', 'max', 'formula']], ['PF_MAX', 'Punti ferita massimi', ['agg', 'formula']]];
    AB.forEach((a) => P.push([a, 'Punteggio di ' + ABN[a], ['agg', 'sost', 'min', 'max']]));
    P.push(['TS', 'Tutti i tiri salvezza', ['agg', 'formula', 'vant', 'svant']]);
    AB.forEach((a) => P.push(['TS_' + a, 'TS di ' + ABN[a], ['agg', 'formula', 'vant', 'svant']]));
    P.push(['PROVE', 'Tutte le prove di abilità', ['agg', 'formula', 'vant', 'svant']]);
    nomiAbilita().forEach((n) => P.push(['ABIL:' + n, 'Abilità: ' + n, ['agg', 'formula', 'vant', 'svant']]));
    P.push(['PERC_PASSIVA', 'Percezione passiva', ['agg', 'formula']]);
    P.push(['ATTACCO', 'Tiri per colpire con armi', ['agg', 'formula', 'vant', 'svant']], ['DANNO', 'Danni con armi', ['agg', 'formula']]);
    P.push(['ATT_INC', 'Attacco con incantesimo', ['agg', 'formula']], ['CD_INC', 'CD degli incantesimi', ['agg', 'formula']]);
    P.push(['COMPETENZE', 'Competenze (nome di un\'abilità o testo libero)', ['speciale']], ['RESISTENZE', 'Resistenze (tipo di danno)', ['resist']],
      ['IMMUNITA', 'Immunità (tipo di danno o condizione)', ['immun']], ['SPECIALE', 'Altro modificatore o effetto speciale (testo)', ['speciale']]);
    return P;
  }
  const parametro = (cod) => parametri().find((p) => p[0] === cod) || null;
  const nomeParametro = (cod) => { const p = parametro(cod); return p ? p[1] : cod; };

  /* ---------- utilità di controllo dei dati ---------- */
  const isObj = (x) => x && typeof x === 'object' && !Array.isArray(x);
  const str = (x, max) => (typeof x === 'string' || typeof x === 'number' ? String(x) : '').replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/g, '').slice(0, max);
  const num = (x, min, max, def) => { const v = typeof x === 'string' ? parseFloat(x.replace(',', '.')) : x; return Number.isFinite(v) ? Math.max(min, Math.min(max, v)) : def; };
  const una = (x, lista, def) => (lista.some((e) => (Array.isArray(e) ? e[0] : e) === x) ? x : def);
  const bool = (x) => x === true || x === 'true' || x === 1;

  function nuovoId() { return 'ogm-' + Date.now().toString(36) + Math.random().toString(36).slice(2, 7); }

  /* Oggetto vuoto con tutti i campi della scheda (sezioni 1-7 della specifica del Master). */
  function nuovoOggetto(tipo) {
    tipo = una(tipo, TIPI, 'altro');
    return { id: nuovoId(), v: 1, nome: '', tipo, sottotipo: '', rar: 'non_comune', sint: false, img: '', peso: 0, valore: 0, slot: SLOT_DI_TIPO[tipo], mani: 1,
      aspetto: '', mater: '', storia: '', testo: '', princ: '', second: '', speciali: '',
      uso: { att: 'passiva', modo: '', durata: '', cariche: 0, ricarica: '', limiti: '' },
      mod: [],
      arma: { cat: 'semplice', dist: 'mischia', base: '', comp: 'si', dado: '1d6', tipo: 'tagliente', bAtt: 0, bDanno: 0, prop: '', vers: '', g1: 0, g2: 0, mun: '', extra: [] },
      armatura: { cat: 'leggera', ca: 11, bonus: 0, des: 'completo', forza: 0, furt: false },
      scudo: { ca: 2, bonus: 0, spec: '' },
      acc: { tipo: '', bonus: 0, car: '', num: '', spec: '' },
      req: '', condiz: '', inter: '' };
  }

  function sanificaMod(m) {
    if (!isObj(m)) return null;
    const par = str(m.par, 60);
    const p = parametro(par);
    if (!p) return null;
    const op = una(m.op, OPERAZIONI, 'agg');
    if (!p[2].includes(op)) return null;
    const o = { par, op, cond: una(m.cond, CONDIZIONI, 'equip'), dur: una(m.dur, DURATE, 'perm'), nota: str(m.nota, 140), solo: m.solo === SOLO_SENZA ? SOLO_SENZA : '',
      gruppo: str(m.gruppo, 40).trim() };
    if (TESTO.includes(op)) { o.val = str(m.val, 100).trim(); if (!o.val) return null; return o; }
    if (op === 'formula') { const f = str(m.val, 60).trim(); if (valutaFormula(f, { M: {}, comp: 0, liv: 0 }) === null) return null; o.val = f; }
    else if (NUM.includes(op)) o.val = num(m.val, -30, 60, 0);
    else o.val = 0;
    return o;
  }

  function sanificaOggetto(x) {
    if (!isObj(x)) return null;
    const b = nuovoOggetto(una(x.tipo, TIPI, 'altro'));
    const o = b;
    o.id = /^[A-Za-z0-9_.-]{1,40}$/.test(String(x.id || '')) ? String(x.id) : nuovoId();
    o.v = Math.round(num(x.v, 1, 9999, 1));
    o.nome = str(x.nome, 80).trim();
    if (!o.nome) return null;
    o.sottotipo = str(x.sottotipo, 80);
    o.rar = una(x.rar, RARITA, 'non_comune');
    o.sint = bool(x.sint);
    o.img = typeof x.img === 'string' && x.img.length <= MAX_IMG && /^data:image\/(jpeg|png|webp);base64,[A-Za-z0-9+/=]+$/.test(x.img) ? x.img : '';
    o.peso = num(x.peso, 0, 100000, 0); o.valore = num(x.valore, 0, 1e9, 0);
    o.slot = una(x.slot, SLOT, SLOT_DI_TIPO[o.tipo]); o.mani = Math.round(num(x.mani, 1, 2, 1));
    ['aspetto', 'mater', 'storia', 'testo', 'princ', 'second', 'speciali', 'req', 'condiz', 'inter'].forEach((k) => { o[k] = str(x[k], 4000); });
    const u = isObj(x.uso) ? x.uso : {};
    o.uso = { att: una(u.att, ATTIVAZIONI, 'passiva'), modo: str(u.modo, 200), durata: str(u.durata, 120), cariche: Math.round(num(u.cariche, 0, 999, 0)), ricarica: str(u.ricarica, 200), limiti: str(u.limiti, 400) };
    o.mod = (Array.isArray(x.mod) ? x.mod : []).slice(0, 40).map(sanificaMod).filter(Boolean);
    const a = isObj(x.arma) ? x.arma : {};
    o.arma = { cat: una(a.cat, ['semplice', 'marziale'], 'semplice'), dist: una(a.dist, ['mischia', 'distanza'], 'mischia'), base: str(a.base, 60), comp: a.comp === 'no' ? 'no' : 'si',
      dado: /^\d{1,2}d\d{1,3}$|^1$|^0$/.test(String(a.dado || '')) ? String(a.dado) : '1d6', tipo: str(a.tipo, 40) || 'tagliente', bAtt: Math.round(num(a.bAtt, -10, 10, 0)), bDanno: Math.round(num(a.bDanno, -10, 10, 0)),
      prop: str(a.prop, 200), vers: /^\d{1,2}d\d{1,3}$/.test(String(a.vers || '')) ? String(a.vers) : '', g1: num(a.g1, 0, 2000, 0), g2: num(a.g2, 0, 5000, 0), mun: str(a.mun, 80),
      /* danni aggiuntivi di altri tipi (2026-11-11): restano separati per resistenze, immunità e vulnerabilità */
      extra: (Array.isArray(a.extra) ? a.extra : []).slice(0, 6).filter((e) => isObj(e) && /^\d{1,2}d\d{1,3}$/.test(String(e.dadi || ''))).map((e) => ({ dadi: String(e.dadi), tipo: una(e.tipo, TIPI_DANNO, 'forza'), mod: Math.round(num(e.mod, -20, 20, 0)) })) };
    const r = isObj(x.armatura) ? x.armatura : {};
    o.armatura = { cat: una(r.cat, ['leggera', 'media', 'pesante'], 'leggera'), ca: Math.round(num(r.ca, 0, 30, 11)), bonus: Math.round(num(r.bonus, 0, 10, 0)),
      des: una(r.des, ['completo', 'max2', 'nessuno'], 'completo'), forza: Math.round(num(r.forza, 0, 30, 0)), furt: bool(r.furt) };
    const s = isObj(x.scudo) ? x.scudo : {};
    o.scudo = { ca: Math.round(num(s.ca, 0, 10, 2)), bonus: Math.round(num(s.bonus, 0, 10, 0)), spec: str(s.spec, 300) };
    const c = isObj(x.acc) ? x.acc : {};
    const pc = parametro(str(c.car, 60));
    o.acc = { tipo: str(c.tipo, 60), bonus: Math.round(num(c.bonus, -10, 10, 0)), car: pc && pc[2].includes('agg') ? pc[0] : '', num: str(c.num, 200), spec: str(c.spec, 400) };
    return o;
  }

  function sanificaVoce(v) {
    if (!isObj(v)) return null;
    const o = sanificaOggetto(v.o);
    if (!o) return null;
    const max = o.uso.cariche;
    const car = v.car === null || v.car === undefined || v.car === '' ? max : Math.round(num(v.car, 0, max, max));
    return { o, eq: bool(v.eq), sint: o.sint && bool(v.sint), on: bool(v.on), car };
  }
  function sanificaVoci(arr) {
    const out = [], visti = new Set(), sint = [];
    (Array.isArray(arr) ? arr : []).slice(0, MAX_VOCI).forEach((x) => { const v = sanificaVoce(x); if (v && !visti.has(v.o.id)) { visti.add(v.o.id); out.push(v); } });
    out.forEach((v) => { if (v.sint) { if (sint.length >= MAX_SINTONIE) v.sint = false; else sint.push(v); } }); // mai più di tre sintonie
    return out;
  }

  /* ---------- formule dei modificatori ---------- */
  /* Somme e differenze di: numeri, caratteristiche (valgono il modificatore), COMP (bonus di competenza), LIV (livello). */
  function valutaFormula(testo, ctx) {
    const t = String(testo == null ? '' : testo).replace(/\s+/g, '').toUpperCase();
    if (!t || !/^[+-]?(\d+|FOR|DES|COS|INT|SAG|CAR|COMP|LIV)([+-](\d+|FOR|DES|COS|INT|SAG|CAR|COMP|LIV))*$/.test(t)) return null;
    let tot = 0;
    const termini = t.match(/[+-]?(\d+|[A-Z]+)/g) || [];
    for (const term of termini) {
      const seg = term[0] === '-' ? -1 : 1, nome = term.replace(/^[+-]/, '');
      let v;
      if (/^\d+$/.test(nome)) v = parseInt(nome, 10);
      else if (nome === 'COMP') v = (ctx && ctx.comp) || 0;
      else if (nome === 'LIV') v = (ctx && ctx.liv) || 0;
      else v = (ctx && ctx.M && ctx.M[nome]) || 0;
      tot += seg * v;
    }
    return tot;
  }

  /* Modificatori in vigore di un oggetto: quelli scritti dal Master più, per complementi e accessori, il «bonus di
     incantamento» sulla «caratteristica modificata» (diventa un'aggiunta, attiva da equipaggiato). */
  function modDi(o) {
    const L = (o.mod || []).slice();
    if (!['arma', 'armatura', 'scudo'].includes(o.tipo) && o.acc && o.acc.bonus && o.acc.car) {
      L.push({ par: o.acc.car, op: 'agg', val: o.acc.bonus, cond: 'equip', dur: 'perm', nota: 'bonus di incantamento', solo: '', gruppo: '', implicito: true });
    }
    return L;
  }

  /* ---------- quando un oggetto (o un suo modificatore) è attivo ---------- */
  const sintOk = (v) => !v.o.sint || !!v.sint;                 // se richiede sintonizzazione, serve
  const equipOk = (v) => !!v.eq && sintOk(v);                  // equipaggiato (e sintonizzato, se serve)
  function modAttivo(v, m) {
    let ok;
    switch (m.cond) {
      case 'sempre': ok = sintOk(v); break;
      case 'att': ok = equipOk(v) && !!v.on; break;
      case 'sint': ok = v.o.sint ? !!v.eq && !!v.sint : equipOk(v); break; // senza sintonizzazione possibile vale come «equipaggiato»
      default: ok = equipOk(v);
    }
    if (ok && m.dur === 'temp') ok = equipOk(v) && !!v.on;
    return ok;
  }

  /* Punteggi di caratteristica. Ordine: sostituzione (vale la più alta), minimo («diventa almeno X»), aggiunte
     (due bonus dello stesso «gruppo» non si sommano: conta il più alto), massimo; poi tetto 30. */
  function punteggi(voci) {
    const r = {};
    AB.forEach((a) => { r[a] = { agg: 0, sost: null, min: null, max: null, da: [], scartati: [] }; });
    const gruppi = {};
    (voci || []).forEach((v) => modDi(v.o).forEach((m) => {
      if (!AB.includes(m.par) || !modAttivo(v, m)) return;
      const x = r[m.par], val = +m.val || 0;
      if (m.op === 'agg') {
        if (m.gruppo) { const k = m.par + '|' + m.gruppo.toLowerCase(); if (!gruppi[k]) gruppi[k] = []; gruppi[k].push({ x, val, da: v.o.nome }); }
        else { x.agg += val; x.da.push([v.o.nome, val]); }
      } else if (m.op === 'sost') { if (x.sost === null || val > x.sost) x.sost = val; x.da.push([v.o.nome, '= ' + val]); }
      else if (m.op === 'min') { if (x.min === null || val > x.min) x.min = val; x.da.push([v.o.nome, 'almeno ' + val]); }
      else if (m.op === 'max') { if (x.max === null || val < x.max) x.max = val; x.da.push([v.o.nome, 'al massimo ' + val]); }
    }));
    Object.values(gruppi).forEach((L) => { const best = L.reduce((a, b) => (b.val > a.val ? b : a)); best.x.agg += best.val; best.x.da.push([best.da, best.val]); L.filter((e) => e !== best).forEach((e) => e.x.scartati.push(e.da)); });
    return r;
  }
  function applicaPunteggio(base, p) {
    let t = base;
    if (p.sost !== null) t = p.sost;
    if (p.min !== null && p.min > t) t = p.min;
    t += p.agg;
    if (p.max !== null && p.max < t) t = p.max;
    return Math.max(1, Math.min(30, t));
  }

  /* Tutti gli altri modificatori. ctx = {M: modificatori finali, comp, liv, senzaArmatura}.
     Risultato: somme per destinazione, basi alternative della CA, minimi e massimi finali, vantaggi/svantaggi e
     le voci di testo (resistenze, immunità, competenze, effetti speciali). Bonus dello stesso «gruppo»: conta il più alto. */
  function calcola(voci, ctx) {
    ctx = ctx || {};
    const R = { add: {}, vant: {}, svant: {}, cand: [], lim: {}, testi: { resist: [], immun: [], comp: [], speciale: [] }, scartati: [] };
    const gruppi = {};
    (voci || []).forEach((v) => modDi(v.o).forEach((m) => {
      if (AB.includes(m.par) || !modAttivo(v, m)) return;
      if (m.solo === SOLO_SENZA && ctx.senzaArmatura === false) return;
      const da = v.o.nome;
      if (m.op === 'vant') (R.vant[m.par] = R.vant[m.par] || []).push(da);
      else if (m.op === 'svant') (R.svant[m.par] = R.svant[m.par] || []).push(da);
      else if (m.op === 'resist') R.testi.resist.push({ t: m.val, da });
      else if (m.op === 'immun') R.testi.immun.push({ t: m.val, da });
      else if (m.op === 'speciale') (m.par === 'COMPETENZE' ? R.testi.comp : R.testi.speciale).push({ t: m.val, da });
      else if (m.op === 'min' || m.op === 'max') {
        const l = (R.lim[m.par] = R.lim[m.par] || { min: null, max: null }), val = +m.val;
        if (m.op === 'min') l.min = l.min === null ? val : Math.max(l.min, val); else l.max = l.max === null ? val : Math.min(l.max, val);
      } else if (m.par === 'CA' && (m.op === 'sost' || m.op === 'formula')) {
        const ca = m.op === 'sost' ? +m.val : valutaFormula(m.val, ctx);
        if (ca !== null) R.cand.push({ ca, da, solo: m.solo === SOLO_SENZA });
      } else {
        const val = m.op === 'formula' ? valutaFormula(m.val, ctx) : +m.val;
        if (val === null || !Number.isFinite(val)) return;
        if (m.gruppo) { const k = m.par + '|' + m.gruppo.toLowerCase(); (gruppi[k] = gruppi[k] || []).push({ par: m.par, v: val, da }); }
        else (R.add[m.par] = R.add[m.par] || []).push({ v: val, da });
      }
    }));
    Object.values(gruppi).forEach((L) => {
      const best = L.reduce((a, b) => (b.v > a.v ? b : a));
      (R.add[best.par] = R.add[best.par] || []).push({ v: best.v, da: best.da });
      L.filter((e) => e !== best).forEach((e) => R.scartati.push(e.da));
    });
    R.dettaglio = (par) => R.add[par] || [];
    R.somma = (...pars) => pars.reduce((s, p) => s + (R.add[p] || []).reduce((a, x) => a + x.v, 0), 0);
    R.fonti = (...pars) => [...new Set(pars.flatMap((p) => (R.add[p] || []).map((x) => x.da)))];
    /* minimo e massimo finali (per CA e velocità): prima il minimo, poi il massimo */
    R.finale = (par, val) => { const l = R.lim[par]; if (!l) return val; let t = val; if (l.min !== null && t < l.min) t = l.min; if (l.max !== null && t > l.max) t = l.max; return t; };
    /* Vantaggio e svantaggio si annullano (regola di quinta edizione): '' (nessuno), 'vantaggio', 'svantaggio' o 'pari'. */
    R.vantaggio = (...pars) => {
      const v = pars.flatMap((p) => R.vant[p] || []), s = pars.flatMap((p) => R.svant[p] || []);
      return v.length && s.length ? 'pari' : v.length ? 'vantaggio' : s.length ? 'svantaggio' : '';
    };
    R.fontiVantaggio = (...pars) => [...new Set(pars.flatMap((p) => [...(R.vant[p] || []), ...(R.svant[p] || [])]))];
    return R;
  }

  /* ---------- armatura, scudo e armi equipaggiati ---------- */
  function armatura(voci) {
    const v = (voci || []).find((x) => x.o.tipo === 'armatura' && equipOk(x));
    if (!v) return null;
    const a = v.o.armatura, cat = { leggera: 'l', media: 'm', pesante: 'p' }[a.cat];
    const ca = a.ca + a.bonus;
    /* stessa forma di ARMORS (dati.js): [nome, categoria, CA, Forza minima, svantaggio a Furtività] + regola della Destrezza */
    return { nome: v.o.nome, ca, cat, forza: a.forza, furt: a.furt, des: a.des, entry: [v.o.nome + (a.bonus ? ' +' + a.bonus : ''), cat, ca, a.forza, a.furt ? 1 : 0, a.des], voce: v };
  }
  function scudo(voci) {
    const v = (voci || []).find((x) => x.o.tipo === 'scudo' && equipOk(x));
    return v ? { nome: v.o.nome + (v.o.scudo.bonus ? ' +' + v.o.scudo.bonus : ''), ca: v.o.scudo.ca + v.o.scudo.bonus, voce: v } : null;
  }
  function armi(voci) { return (voci || []).filter((x) => x.o.tipo === 'arma' && equipOk(x)); }
  /* Bonus alla CA dei soli dati specifici (armatura e scudo) non è incluso nei modificatori: lo somma la scheda. */

  /* ---------- slot e sintonizzazione ---------- */
  const slotInfo = (cod) => SLOT.find((s) => s[0] === cod) || SLOT[SLOT.length - 1];
  const postiOccupati = (o) => (o.slot === 'mani' && o.mani === 2 ? 2 : 1);  // arma o bastone a due mani
  function sintonizzati(voci) { return (voci || []).filter((v) => v.sint); }
  function puoSintonizzare(voci, v) {
    if (!v.o.sint) return { ok: false, motivo: 'Questo oggetto non richiede sintonizzazione.' };
    if (v.sint) return { ok: true };
    if (sintonizzati(voci).length >= MAX_SINTONIE) return { ok: false, motivo: 'Hai già ' + MAX_SINTONIE + ' oggetti sintonizzati: rimuovi prima la sintonia di uno.' };
    return { ok: true };
  }
  function puoEquipaggiare(voci, v) {
    const [cod, nome, cap] = slotInfo(v.o.slot);
    if (!cap) return { ok: true };
    const altri = (voci || []).filter((x) => x !== v && x.eq && x.o.slot === cod);
    const usati = altri.reduce((s, x) => s + postiOccupati(x.o), 0);
    if (usati + postiOccupati(v.o) > cap) return { ok: false, motivo: 'Slot «' + nome + '» pieno (' + altri.map((x) => x.o.nome).join(', ') + '): togli prima un oggetto.' };
    return { ok: true };
  }

  /* Perché un oggetto non sta facendo effetto ('' se è tutto a posto). */
  function motivoInattivo(v) {
    if (!v.eq) return 'Non equipaggiato';
    if (v.o.sint && !v.sint) return 'Serve la sintonizzazione';
    if (modDi(v.o).some((m) => (m.cond === 'att' || m.dur === 'temp') && !v.on) && !modDi(v.o).some((m) => m.cond !== 'att' && m.dur !== 'temp')) return 'Da attivare';
    return '';
  }
  /* Elenco degli effetti attualmente in vigore, oggetto per oggetto (per il riquadro della scheda). */
  function effettiAttivi(voci) {
    const out = [];
    (voci || []).forEach((v) => {
      const righe = modDi(v.o).filter((m) => modAttivo(v, m)).map(descriviMod);
      if (equipOk(v)) {
        if (v.o.tipo === 'armatura') righe.unshift('Armatura: CA ' + (v.o.armatura.ca + v.o.armatura.bonus) + ' di base');
        if (v.o.tipo === 'scudo') righe.unshift('Scudo: +' + (v.o.scudo.ca + v.o.scudo.bonus) + ' alla CA');
        if (v.o.tipo === 'arma' && (v.o.arma.bAtt || v.o.arma.bDanno)) righe.unshift('Arma: ' + sg(v.o.arma.bAtt) + ' per colpire, ' + sg(v.o.arma.bDanno) + ' ai danni');
      }
      if (righe.length) out.push({ nome: v.o.nome, righe });
    });
    return out;
  }

  /* ---------- descrizioni leggibili ---------- */
  const sg = (n) => (n >= 0 ? '+' : '−') + Math.abs(n);
  function descriviMod(m) {
    const nome = nomeParametro(m.par);
    let t;
    if (m.op === 'agg') t = nome + ' ' + sg(m.val);
    else if (m.op === 'sost') t = m.par === 'CA' ? 'CA di base ' + m.val : nome + ': diventa ' + m.val;
    else if (m.op === 'min') t = nome + ': almeno ' + m.val;
    else if (m.op === 'max') t = nome + ': al massimo ' + m.val;
    else if (m.op === 'formula') t = m.par === 'CA' ? 'CA di base = ' + m.val : nome + ' + (' + m.val + ')';
    else if (m.op === 'resist') t = 'Resistenza: ' + m.val;
    else if (m.op === 'immun') t = 'Immunità: ' + m.val;
    else if (m.op === 'speciale') t = (m.par === 'COMPETENZE' ? 'Competenza: ' : 'Effetto: ') + m.val;
    else t = (m.op === 'vant' ? 'Vantaggio: ' : 'Svantaggio: ') + nome;
    const cond = { equip: 'equipaggiato', sint: 'sintonizzato', att: 'attivato', sempre: 'portato con sé' }[m.cond];
    return t + ' (' + cond + (m.dur === 'temp' ? ', temporaneo' : '') + (m.solo ? ', solo senza armatura né scudo' : '') + (m.gruppo ? ', gruppo «' + m.gruppo + '»' : '') + ')';
  }

  /* L'oggetto ha qualcosa che la scheda sa calcolare? (modificatori, bonus di incantamento con la sua destinazione,
     dati di arma, armatura o scudo). Se no, equipaggiarlo non cambia nessun numero (segnalazione del 2026-11-11). */
  function haEffettiCalcolabili(o) {
    if (modDi(o).length) return true;
    if (o.tipo === 'arma' || o.tipo === 'armatura' || o.tipo === 'scudo') return true;
    return false;
  }

  /* Suggerimenti dai testi (2026-11-11): dalle frasi chiare della descrizione o del testo del manuale («bonus di +1 alla CA
     e ai tiri salvezza», «il punteggio di Forza è pari a 19», «resistenza ai danni da fuoco») i modificatori tecnici e i
     bonus di arma, armatura e scudo. Le frasi con condizioni particolari restano testo: decide il Master. */
  const NOMI_CAR = { forza: 'FOR', destrezza: 'DES', costituzione: 'COS', intelligenza: 'INT', saggezza: 'SAG', carisma: 'CAR' };
  function suggerisci(testo, o) {
    const t = String(testo || '').replace(/\s+/g, ' ').replace(/[−–]/g, '-');
    const cond = o && o.sint ? 'sint' : 'equip';
    const out = { mod: [], bonus: 0 };
    const metti = (par, op, val) => { if (!out.mod.some((m) => m.par === par && m.op === op && String(m.val) === String(val))) out.mod.push({ par, op, val, cond, dur: 'perm', nota: '', solo: '', gruppo: '' }); };
    const nomeB = /\+\s?([1-3])\b/.exec((o && o.nome) || '');
    const frase = (re) => { const m = re.exec(t); return m ? +m[1] : 0; };
    const tipo = o ? o.tipo : '';
    if (tipo === 'arma') out.bonus = frase(/\+\s?([1-3])[^.]{0,40}tiri per colpire[^.]{0,40}danni/i) || (nomeB ? +nomeB[1] : 0);
    else if (tipo === 'armatura' || tipo === 'scudo') out.bonus = frase(/\+\s?([1-3])[^.]{0,25}(?:alla )?(?:CA|Classe Armatura)\b/i) || (nomeB ? +nomeB[1] : 0);
    else {
      const ca = frase(/\+\s?([1-5])[^.]{0,25}(?:alla )?(?:CA|Classe Armatura)\b/i);
      if (ca) metti('CA', 'agg', ca);
      const ts = /\+\s?([1-5])([^.]{0,45})tiri salvezza/i.exec(t);
      if (ts && !/\bCD\b|classe di difficolt/i.test(ts[2])) metti('TS', 'agg', +ts[1]);
      const ai = frase(/\+\s?([1-3])[^.]{0,40}tiri per colpire con (?:gli |i propri |i suoi )?incantesimi/i);
      if (ai) metti('ATT_INC', 'agg', ai);
      const cd = frase(/\+\s?([1-3])[^.]{0,40}\bCD\b[^.]{0,30}incantesimi/i);
      if (cd) metti('CD_INC', 'agg', cd);
      const ini = frase(/\+\s?([1-9])[^.]{0,25}(?:all'|alle prove di )?iniziativa/i);
      if (ini) metti('INIZIATIVA', 'agg', ini);
      const vel = frase(/velocit[àa][^.]{0,50}aumenta(?:ta)? di (\d{1,2}) m(?:etri)?\b/i);
      if (vel) metti('VELOCITA', 'agg', vel);
      const pf = frase(/punti ferita massimi[^.]{0,30}aumenta(?:no)? di (\d{1,3})\b/i);
      if (pf) metti('PF_MAX', 'agg', pf);
    }
    let m;
    const reSost = /punteggio di (Forza|Destrezza|Costituzione|Intelligenza|Saggezza|Carisma)[^.]{0,40}?(?:diventa|pari a|è) (\d{1,2})\b/gi;
    while ((m = reSost.exec(t))) metti(NOMI_CAR[m[1].toLowerCase()], 'sost', +m[2]);
    const reAgg = /punteggio di (Forza|Destrezza|Costituzione|Intelligenza|Saggezza|Carisma)[^.]{0,25}?aumenta di (\d)\b/gi;
    while ((m = reAgg.exec(t))) metti(NOMI_CAR[m[1].toLowerCase()], 'agg', +m[2]);
    const reRes = /resistenz[ae] (?:ai|al|contro i) danni (?:da |di )?([a-zà]+)/gi;
    while ((m = reRes.exec(t))) { const d = m[1].toLowerCase().replace(/i$/, 'o').replace(/^fulmin.*/, 'fulmine').replace(/^tuon.*/, 'tuono').replace(/^fuoc.*/, 'fuoco').replace(/^fredd.*/, 'freddo').replace(/^acid.*/, 'acido').replace(/^forz.*/, 'forza'); if (TIPI_DANNO.includes(d)) metti('RESISTENZE', 'resist', d); }
    return out;
  }
  /* Applica i suggerimenti a un oggetto (non tocca ciò che il Master ha già messo). Restituisce le descrizioni aggiunte. */
  function applicaSuggerimenti(o, s) {
    const fatti = [];
    if (s.bonus) {
      if (o.tipo === 'arma' && !o.arma.bAtt && !o.arma.bDanno) { o.arma.bAtt = s.bonus; o.arma.bDanno = s.bonus; fatti.push('Arma: +' + s.bonus + ' per colpire e ai danni'); }
      if (o.tipo === 'armatura' && !o.armatura.bonus) { o.armatura.bonus = s.bonus; fatti.push('Armatura: +' + s.bonus + ' alla CA'); }
      if (o.tipo === 'scudo' && !o.scudo.bonus) { o.scudo.bonus = s.bonus; fatti.push('Scudo: +' + s.bonus + ' alla CA'); }
    }
    s.mod.forEach((m) => {
      if (o.mod.some((x) => x.par === m.par && x.op === m.op)) return;
      const z = sanificaMod(m);
      if (z) { o.mod.push(z); fatti.push(descriviMod(z)); }
    });
    return fatti;
  }

  /* Avvisi per chi compila la scheda (non bloccano il salvataggio). */
  function avvisi(o) {
    const A = [];
    if (!o.nome.trim()) A.push('Manca il nome.');
    if (!['arma', 'armatura', 'scudo'].includes(o.tipo) && o.acc && o.acc.bonus && !o.acc.car) A.push('«Bonus di incantamento» senza «Caratteristica modificata»: la scheda non sa a cosa sommarlo (sezione E).');
    if (!haEffettiCalcolabili(o) && /\+\s?\d/.test([o.princ, o.second, o.speciali, o.acc && o.acc.num].join(' '))) A.push('Nei testi c\'è un bonus, ma nessun modificatore tecnico: la scheda non lo calcola. Usa «✨ Suggerisci dai testi» o «＋ Aggiungi modificatore».');
    if (o.tipo === 'arma' && !o.arma.dado) A.push('Arma senza dado di danno.');
    modDi(o).forEach((m) => {
      if (m.op === 'sost' && m.par !== 'CA' && !AB.includes(m.par)) A.push('«' + descriviMod(m) + '»: la sostituzione vale solo per CA e punteggi di caratteristica.');
      if (m.cond === 'sint' && !o.sint) A.push('«' + descriviMod(m) + '»: l\'oggetto non richiede sintonizzazione, il modificatore vale come «equipaggiato».');
      if (m.dur === 'temp' && m.cond === 'sempre') A.push('«' + descriviMod(m) + '»: un effetto temporaneo richiede che l\'oggetto sia equipaggiato e attivato.');
    });
    if (o.uso.att !== 'passiva' && !o.mod.some((m) => m.cond === 'att' || m.dur === 'temp') && !o.princ.trim()) A.push('L\'attivazione non ha effetti tecnici né descrizione.');
    return A.filter(Boolean);
  }

  /* ---------- importazione ed esportazione (sezione 7: identificativo univoco e versione) ---------- */
  function esporta(o) { return JSON.stringify({ formato: FORMATO, versione: VERSIONE_FORMATO, oggetto: o }, null, 1); }
  function esportaVarie(os) { return JSON.stringify({ formato: FORMATO, versione: VERSIONE_FORMATO, oggetti: os }, null, 1); }
  function importa(testo) {
    const out = { oggetti: [], errori: [] };
    if (typeof testo !== 'string' || !testo.trim()) { out.errori.push('Testo vuoto.'); return out; }
    if (testo.length > 3e6) { out.errori.push('File troppo grande.'); return out; }
    let j;
    try { j = JSON.parse(testo); } catch (e) { out.errori.push('Il testo non è un oggetto magico di Gaster Bless (JSON non valido).'); return out; }
    let lista = [];
    if (isObj(j) && j.formato === FORMATO) lista = Array.isArray(j.oggetti) ? j.oggetti : j.oggetto ? [j.oggetto] : [];
    else if (isObj(j) && j.nome) lista = [j];
    else if (Array.isArray(j)) lista = j;
    else out.errori.push('Formato non riconosciuto.');
    if (isObj(j) && j.formato === FORMATO && j.versione > VERSIONE_FORMATO) out.errori.push('Il file è di una versione più recente dell\'app: aggiorna Gaster Bless.');
    lista.slice(0, 50).forEach((x, i) => { const o = sanificaOggetto(x); if (o) out.oggetti.push(o); else out.errori.push('Oggetto n. ' + (i + 1) + ' non valido (serve almeno il nome).'); });
    return out;
  }
  /* Aggiunge o aggiorna un oggetto nell'inventario magico. Stesso id: vale la versione più alta e si
     conservano equipaggiato, sintonia, attivazione e cariche. Ritorna {voci, esito}. */
  function unisci(voci, nuovo) {
    const lista = (voci || []).slice();
    const i = lista.findIndex((v) => v.o.id === nuovo.id);
    if (i < 0) {
      if (lista.length >= MAX_VOCI) return { voci: lista, esito: 'pieno' };
      lista.push({ o: nuovo, eq: false, sint: false, on: false, car: nuovo.uso.cariche });
      return { voci: lista, esito: 'aggiunto' };
    }
    const vecchio = lista[i];
    if (nuovo.v === vecchio.o.v) return { voci: lista, esito: 'uguale' };
    if (nuovo.v < vecchio.o.v) return { voci: lista, esito: 'vecchio' };
    const car = Math.min(vecchio.car == null ? nuovo.uso.cariche : vecchio.car, nuovo.uso.cariche);
    lista[i] = { o: nuovo, eq: vecchio.eq, sint: nuovo.sint && vecchio.sint, on: vecchio.on, car };
    return { voci: lista, esito: 'aggiornato' };
  }

  return { FORMATO, VERSIONE_FORMATO, MAX_SINTONIE, MAX_VOCI, MAX_IMG, TIPI, RARITA, SLOT, ATTIVAZIONI, OPERAZIONI, CONDIZIONI, DURATE, SOLO_SENZA, AB, ABN,
    TIPI_DANNO, modDi, motivoInattivo, effettiAttivi,
    parametri, parametro, nomeParametro, nuovoId, nuovoOggetto, sanificaOggetto, sanificaVoce, sanificaVoci, sanificaMod, valutaFormula,
    modAttivo, equipOk, sintOk, punteggi, applicaPunteggio, calcola, armatura, scudo, armi, slotInfo, puoSintonizzare, puoEquipaggiare, sintonizzati,
    descriviMod, avvisi, esporta, esportaVarie, importa, unisci, haEffettiCalcolabili, suggerisci, applicaSuggerimenti };
})();
if (typeof window !== 'undefined') window.OggettiMagici = OggettiMagici;
