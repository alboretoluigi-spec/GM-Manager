/* ===== DIFESE DEI PERSONAGGI (quinta edizione, regole 2014) — Gaster Bless, 2026-11-12 =====
   Modulo puro (nessun accesso alla pagina): definisce la variabile globale DifesePG. Lo usa la scheda per mostrare al
   giocatore le sue difese e per mandarle al server nel riepilogo (motore degli attacchi e degli incantesimi del tracker).
   Contiene solo le difese SEMPRE ATTIVE che vengono da razza, sottorazza, classe, sottoclasse e talenti (manuali 2014: PHB,
   supplemento, supplemento, Volo, Mordenkainen, il cacciatore). Quelle temporanee (ira del barbaro, forma stellare, Affinità
   elementale) sono effetti dello scontro, non stanno qui. Nomi dei privilegi in italiano, riassunti con parole nostre.
   Risultato di calcola():
     res, imm   resistenze e immunità ai danni [{t: tipo, da: fonte, nm: true se vale solo contro attacchi non magici}]
     immCond    immunità alle condizioni [{c: condizione come nel tracker, da}]
     vant       vantaggio ai tiri salvezza contro una condizione [{c, da}]
     magia      vantaggio ai tiri salvezza contro gli incantesimi: {car: [caratteristiche], da: [fonti]}
     resInc     resistenza ai danni degli incantesimi: [fonti]
     note       ciò che il motore non applica da solo (testo per il giocatore e il Master) */
const DifesePG = (function () {
  'use strict';
  const TUTTE = ['FOR', 'DES', 'COS', 'INT', 'SAG', 'CAR'], MENTE = ['INT', 'SAG', 'CAR'];
  const TIPI = ['acido', 'contundente', 'freddo', 'forza', 'fulmine', 'fuoco', 'necrotico', 'perforante', 'psichico', 'radioso', 'tagliente', 'tuono', 'veleno'];
  const FISICI = ['contundente', 'perforante', 'tagliente'];

  /* razze: chiave della razza o «razza/sottorazza» (chiavi di RACES in web/regole/dati.js) */
  const RAZZE = {};

  /* classi e sottoclassi: c = chiave della classe, sub = nome della sottoclasse (come in CLASSES), liv = livello nella classe.
     scelta: la resistenza dipende da una scelta del giocatore (vedi SCELTE). armPesante: vale solo con l'armatura pesante. */
  const CLASSI = [];

  const TALENTI = {};

  /* scelte del giocatore che decidono il tipo di danno. dal: livello della classe da cui la scelta compare nella scheda. */
  const SCELTE = {};

  /** Scelte da mostrare nella scheda per queste classi: [{k, etichetta, opzioni: [[valore, testo]]}]. */
  function scelte(classi) {
    return Object.entries(SCELTE).filter(([, s]) => (classi || []).some((k) => k.id === s.c && k.sub === s.sub && k.lvl >= s.dal))
      .map(([k, s]) => ({ k, etichetta: s.etichetta, opzioni: s.opzioni.map((o) => [o[0], o[1]]) }));
  }

  /** ctx: {razza, sottorazza, draconico (tipo di danno della discendenza), classi: [{id, lvl, sub}], talenti: [nomi],
   *  scelte: {amb, genio, resRip}, armPesante (indossa un'armatura pesante)}. */
  function calcola(ctx) {
    ctx = ctx || {};
    const R = { res: [], imm: [], immCond: [], vant: [], magia: { car: [], da: [] }, resInc: [], note: [] };
    const metti = (lista, chiave, valore, da, nm) => {
      const x = lista.find((y) => y[chiave] === valore && !!y.nm === !!nm);
      if (x) { if (!x.da.split(', ').includes(da)) x.da += ', ' + da; return; }
      const o = { [chiave]: valore, da };
      if (nm) o.nm = nm; // true: solo contro attacchi non magici; 'argento': non magici e non argentati
      lista.push(o);
    };
    const applica = (e, prefisso) => {
      const da = (prefisso ? prefisso + ': ' : '') + e.da;
      (e.res || []).forEach((t) => metti(R.res, 't', t, da, e.nm));
      (e.imm || []).forEach((t) => metti(R.imm, 't', t, da));
      (e.immCond || []).forEach((c) => metti(R.immCond, 'c', c, da));
      (e.vant || []).forEach((c) => metti(R.vant, 'c', c, da));
      if (e.magia) { e.magia.forEach((c) => { if (!R.magia.car.includes(c)) R.magia.car.push(c); }); R.magia.da.push(da); }
      if (e.resInc) R.resInc.push(da);
      if (e.nota) R.note.push(e.da + ': ' + e.nota);
    };
    const razza = String(ctx.razza || ''), sotto = String(ctx.sottorazza || '');
    (RAZZE[razza] || []).forEach((e) => applica(e));
    if (sotto) (RAZZE[razza + '/' + sotto] || []).forEach((e) => applica(e));
    if (razza === 'dragonide' && TIPI.includes(ctx.draconico)) applica({ res: [ctx.draconico], da: 'Resistenza ai danni (discendenza draconica)' });
    const sc = ctx.scelte || {};
    CLASSI.forEach((e) => {
      const k = (ctx.classi || []).find((x) => x.id === e.c);
      if (!k || k.lvl < e.liv || (e.sub && k.sub !== e.sub)) return;
      if (e.armPesante && !ctx.armPesante) return;
      if (e.scelta) {
        const o = SCELTE[e.scelta].opzioni.find((x) => x[0] === sc[e.scelta]);
        if (!o) { R.note.push(e.da + ': scegli il tipo nelle «Scelte del personaggio»'); return; }
        applica({ ...e, res: [o[2]] });
        return;
      }
      applica(e);
    });
    (ctx.talenti || []).forEach((n) => (TALENTI[String(n).trim().toLowerCase()] || []).forEach((e) => applica(e)));
    // un'immunità rende inutile la resistenza allo stesso tipo
    R.res = R.res.filter((x) => !R.imm.some((y) => y.t === x.t));
    return R;
  }

  /** Ira del barbaro (effetto dello scontro, internal/server/effetti_valori.go): resistenza ai danni contundenti, perforanti e
   *  taglienti (Totem dell'orso: a tutto tranne lo psichico), vantaggio ai TS su Forza, danni in più in mischia per livello
   *  da barbaro (+2, +3 dal 9°, +4 dal 16°); Berserker dal 6°: immune ad affascinato e spaventato. null se non è un barbaro. */
  function ira(ctx) {
    const k = ((ctx && ctx.classi) || []).find((x) => x.id === 'barbaro');
    if (!k) return null;
    const orso = k.sub === 'Cammino del Combattente Totemico' && k.lvl >= 3 && ctx.scelte && ctx.scelte.totem === 'orso';
    const out = { res: orso ? TIPI.filter((t) => t !== 'psichico') : FISICI.slice(), vantTS: ['FOR'], danniMischia: k.lvl >= 16 ? 4 : k.lvl >= 9 ? 3 : 2 };
    if (k.sub === 'Cammino del Berserker' && k.lvl >= 6) out.immCond = ['Affascinato', 'Spaventato'];
    return out;
  }

  /** Le difese nella forma del riepilogo per il server: tipi, con « (non magici)» quando valgono solo contro gli attacchi
   *  non magici. Oggetti magici: {res: [tipi], imm: [tipi]} già calcolati dalla scheda. */
  function perRiepilogo(R, oggetti) {
    const sufNm = (x) => (x.nm === 'argento' ? ' (non magici e non argentati)' : x.nm ? ' (non magici)' : '');
    const tipi = (l) => l.map((x) => x.t + sufNm(x));
    const unisci = (a, b) => [...new Set([...(a || []), ...(b || [])].map((x) => String(x).toLowerCase().trim()).filter(Boolean))];
    const o = oggetti || {};
    const imm = unisci(tipi(R.imm), o.imm);
    const res = unisci(tipi(R.res), o.res).filter((t) => !imm.includes(t));
    const out = { res, imm };
    if (R.resInc.length) out.incantesimi = true;
    return out;
  }

  /** L'ira in una riga, per il riquadro «Difese» della scheda. */
  function testoIra(m) {
    if (!m) return '';
    return (m.res.length >= 12 ? 'resistenza a tutti i danni tranne psichico' : 'resistenza a contundente, perforante e tagliente') +
      ', vantaggio ai TS su Forza, +' + m.danniMischia + ' ai danni in mischia' + (m.immCond ? ', immune ad affascinato e spaventato' : '');
  }

  /** Testo breve per la scheda: una riga per gruppo (ira: le modifiche dell'ira, se è un barbaro). */
  function righe(R, oggetti, ira) {
    const o = oggetti || {}, out = [];
    const elenco = (l, k, f = (v) => v) => l.map((x) => f(x[k]) + (x.nm === 'argento' ? ' (non magici e non argentati)' : x.nm ? ' (non magici)' : '') + ' — ' + x.da);
    const min = (v) => v.toLowerCase();
    const res = [...elenco(R.res, 't'), ...(o.res || []).map((t) => t + ' — oggetto magico')];
    const imm = [...elenco(R.imm, 't'), ...(o.imm || []).map((t) => t + ' — oggetto magico')];
    if (res.length) out.push(['Resistenze', res]);
    if (imm.length) out.push(['Immunità ai danni', imm]);
    if (R.immCond.length) out.push(['Immune alle condizioni', elenco(R.immCond, 'c', min)]);
    if (R.vant.length) out.push(['Vantaggio ai TS contro', elenco(R.vant, 'c', min)]);
    if (R.magia.car.length) out.push(['Vantaggio ai TS contro gli incantesimi', [(R.magia.car.length === 6 ? 'tutti i TS' : 'TS su ' + R.magia.car.join(', ')) + ' — ' + R.magia.da.join(', ')]]);
    if (R.resInc.length) out.push(['Resistenza ai danni degli incantesimi', R.resInc]);
    if (ira) out.push(['Ira (tasto «😡 Ira» nel tracker, azione bonus)', [testoIra(ira)]]);
    if (R.note.length) out.push(['Da ricordare', R.note]);
    return out;
  }

  return { calcola, scelte, perRiepilogo, righe, ira, testoIra, TIPI, _dati: { RAZZE, CLASSI, SCELTE, TALENTI } };
})();
if (typeof module !== 'undefined') module.exports = DifesePG;
