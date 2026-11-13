/* Scheda libera (fase 3c, passo C4b, docs/SCHEDA_LIBERA.md). Pagina incorporata dall'app (sezione Schede) per i
   personaggi delle campagne con un gioco diverso dalla quinta edizione. Il modello (sezioni, campi, tiri, punti ferita,
   difesa, iniziativa) lo decide il Master per la campagna; qui si disegna e si compila. Protocollo con l'app: lo stesso
   della scheda della quinta edizione (scheda:ready → scheda:init; scheda:state con lo stato e il riepilogo; scheda:blocco;
   scheda:setHp; scheda:tiroLibero → scheda:tiroRisultato). I punti ferita attuali stanno in «hpCur», come nell'altra
   scheda: così il tracker dell'iniziativa li aggiorna. */
(function () {
  'use strict';
  const qs = new URLSearchParams(location.search);
  const ORIGINE = qs.get('host') || location.origin;
  const radice = document.getElementById('sl');
  let st = {}, modello = null, gioco = '', bloccata = false, pid = '';

  const invia = (m) => { if (window.parent !== window) window.parent.postMessage(m, ORIGINE); };
  const el = (tag, attr, ...figli) => {
    const e = document.createElement(tag);
    for (const [k, v] of Object.entries(attr || {})) {
      if (v === undefined || v === null || v === false) continue;
      if (k === 'testo') e.textContent = v; else if (k === 'on') for (const [ev, f] of Object.entries(v)) e.addEventListener(ev, f);
      else if (k === 'valore') e.value = v; else e.setAttribute(k, v === true ? '' : v);
    }
    for (const f of figli.flat(Infinity)) if (f !== null && f !== undefined && f !== false) e.append(f instanceof Node ? f : String(f));
    return e;
  };
  let tAvviso = 0;
  function avviso(t, ms = 6000) { const a = document.getElementById('avviso'); a.textContent = t; a.hidden = false; clearTimeout(tAvviso); tAvviso = setTimeout(() => { a.hidden = true; }, ms); }

  const campi = () => modello.sezioni.flatMap((s) => s.campi);
  const campo = (id) => campi().find((c) => c.id === id);
  st.campi = st.campi || {};
  function valoreNumerico(id) {
    const c = campo(id), v = (st.campi || {})[id];
    if (!c) return 0;
    if (c.tipo === 'risorsa') return id === modello.pf ? (parseInt(v && v.max, 10) || 0) : (parseInt(v && v.v, 10) || 0);
    return parseInt(v, 10) || 0;
  }
  /** La formula con i valori dei campi al posto di {id}. */
  const sostituisci = (f) => String(f || '').replace(/\{([a-z][a-z0-9_]{0,29})\}/g, (_, id) => String(valoreNumerico(id)));

  function predefiniti() {
    st.campi = st.campi || {};
    for (const c of campi()) {
      if (st.campi[c.id] !== undefined) continue;
      if (c.tipo === 'risorsa') st.campi[c.id] = { v: c.predefinito ?? 0, max: c.predefinito ?? 0 };
      else if (c.tipo === 'spunta') st.campi[c.id] = !!c.predefinito;
      else if (c.tipo === 'elenco') st.campi[c.id] = [];
      else if (c.predefinito !== undefined) st.campi[c.id] = c.predefinito;
    }
    if (modello.pf && (st.hpCur === undefined || st.hpCur === '')) {
      const m = valoreNumerico(modello.pf);
      if (m) st.hpCur = String(m);
    }
  }

  // Riepilogo per elenchi e tracker, con i nomi di campo della scheda della quinta edizione (personaggi.go).
  function riepilogo() {
    const r = { initiative: 0 };
    if (modello.pf) r.hpMax = valoreNumerico(modello.pf);
    if (modello.difesa) r.ac = valoreNumerico(modello.difesa);
    if (modello.iniziativa) r.initiativeFormula = sostituisci(modello.iniziativa);
    const primo = campi().find((c) => c.tipo === 'testo');
    if (primo && st.campi[primo.id]) r.class = String(st.campi[primo.id]).slice(0, 40);
    return r;
  }
  let tCambio = 0;
  function cambiato() { clearTimeout(tCambio); tCambio = setTimeout(() => invia({ type: 'scheda:state', state: st, summary: riepilogo() }), 150); }

  function tira(nome, formula) {
    invia({ type: 'scheda:tiroLibero', etichetta: String(nome).slice(0, 40), formula: sostituisci(formula) });
  }

  function controlloCampo(c) {
    const v = st.campi[c.id];
    const id = 'c-' + c.id;
    const dis = bloccata;
    const aggiorna = (x) => { st.campi[c.id] = x; cambiato(); };
    let ctl;
    switch (c.tipo) {
      case 'area':
        ctl = el('textarea', { id, disabled: dis, on: { input: (e) => aggiorna(e.target.value) } }); ctl.value = v || ''; break;
      case 'numero':
        ctl = el('input', { id, type: 'number', min: c.min, max: c.max, valore: v ?? '', disabled: dis, on: { input: (e) => aggiorna(e.target.value === '' ? '' : Number(e.target.value)) } }); break;
      case 'spunta':
        ctl = el('input', { id, type: 'checkbox', disabled: dis, on: { change: (e) => aggiorna(e.target.checked) } }); ctl.checked = !!v; break;
      case 'elenco':
        ctl = el('textarea', { id, disabled: dis, 'aria-label': c.nome + ' (una voce per riga)', on: { input: (e) => aggiorna(e.target.value.split('\n')) } });
        ctl.value = (Array.isArray(v) ? v : []).join('\n'); break;
      case 'risorsa': {
        const x = v && typeof v === 'object' ? v : { v: 0, max: 0 };
        const ePF = c.id === modello.pf;
        const att = el('input', { id, type: 'number', 'aria-label': c.nome + ': attuale', valore: ePF ? (st.hpCur ?? '') : (x.v ?? ''), disabled: dis && !ePF,
          on: { input: (e) => { if (ePF) { st.hpCur = e.target.value; cambiato(); } else aggiorna({ ...x, v: Number(e.target.value) || 0 }); } } });
        const max = el('input', { type: 'number', 'aria-label': c.nome + ': massimo', valore: x.max ?? '', disabled: dis,
          on: { input: (e) => { x.max = Number(e.target.value) || 0; aggiorna({ ...x }); } } });
        ctl = el('div', { class: 'sl-riga sl-risorsa' }, att, el('span', { testo: '/' }), max);
        break;
      }
      default:
        ctl = el('input', { id, type: 'text', valore: v || '', disabled: dis, on: { input: (e) => aggiorna(e.target.value) } });
    }
    return el('div', { class: 'sl-campo', 'data-campo': c.id },
      el('label', { for: id, testo: c.nome }),
      c.tiro ? el('div', { class: 'sl-riga' }, ctl, el('button', { type: 'button', class: 'sl-dado', title: 'Tira: ' + c.tiro, 'aria-label': 'Tira ' + c.nome,
        testo: '🎲', on: { click: () => tira(c.nome, c.tiro) } })) : ctl,
      c.aiuto ? el('div', { class: 'sl-aiuto', testo: c.aiuto }) : null);
  }

  function disegna() {
    radice.replaceChildren(...[
      el('div', { class: 'sl-testa' },
        el('div', { class: 'sl-campo sl-nome' }, el('label', { for: 'c-nome', testo: 'Nome del personaggio' }),
          el('input', { id: 'c-nome', type: 'text', valore: (st.f && st.f.nome) || '', disabled: bloccata, on: { input: (e) => { st.f = st.f || {}; st.f.nome = e.target.value; cambiato(); } } })),
        el('div', { class: 'sl-gioco', testo: '📜 ' + (gioco || 'Scheda libera') })),
      bloccata ? el('p', { class: 'sl-blocco', testo: 'Le modifiche ai dati della scheda passano dal Master: accendi «Modifica manuale» per proporle. Punti ferita e tiri restano liberi.' }) : null,
      (modello.tiri || []).length || modello.iniziativa ? el('section', { class: 'sl-sezione' }, el('h2', { testo: 'Tiri' }),
        el('div', { class: 'sl-tiri' },
          (modello.tiri || []).map((t) => el('button', { type: 'button', class: 'sl-dado', title: t.formula, on: { click: () => tira(t.nome, t.formula) } }, '🎲 ' + t.nome)),
          modello.iniziativa ? el('button', { type: 'button', class: 'sl-dado', title: modello.iniziativa, on: { click: () => tira('Iniziativa', modello.iniziativa) } }, '⚡ Iniziativa') : null)) : null,
      el('div', { class: 'sl-sezioni' }, modello.sezioni.map((s) => el('section', { class: 'sl-sezione' }, el('h2', { testo: s.titolo }), s.campi.map(controlloCampo))))].filter(Boolean));
  }

  async function carica() {
    try {
      const r = await fetch('/api/v1/personaggi/' + encodeURIComponent(pid) + '/modello-scheda', { credentials: 'same-origin' });
      if (!r.ok) throw new Error('Modello della scheda non disponibile.');
      const j = await r.json();
      modello = j.modello; gioco = j.nomeModulo || '';
    } catch (e) { radice.replaceChildren(el('p', { class: 'sl-blocco', testo: e.message })); return; }
    predefiniti();
    disegna();
    invia({ type: 'scheda:state', state: st, summary: riepilogo() }); // il riepilogo arriva al server anche senza modifiche
  }

  window.addEventListener('message', (e) => {
    if (e.source !== window.parent || e.origin !== ORIGINE) return;
    const m = e.data || {};
    switch (m.type) {
      case 'scheda:init':
        st = m.state && typeof m.state === 'object' ? JSON.parse(JSON.stringify(m.state)) : {};
        pid = m.playerId || '';
        if (m.theme) document.documentElement.dataset.tema = m.theme === 'light' ? 'chiaro' : 'scuro';
        carica();
        break;
      case 'scheda:theme':
        document.documentElement.dataset.tema = m.theme === 'light' ? 'chiaro' : 'scuro';
        break;
      case 'scheda:blocco':
        bloccata = !!m.bloccata; if (modello) disegna();
        break;
      case 'scheda:setHp':
        if (m.cur != null) { st.hpCur = String(m.cur); if (modello) disegna(); }
        break;
      case 'scheda:load':
        if (m.state) { st = m.state; if (modello) { predefiniti(); disegna(); } }
        break;
      case 'scheda:tiroRisultato': {
        const r = m.r || {};
        const dadi = (r.dadi || []).map((d) => d.n + 'd' + d.facce + ' [' + (d.valori || []).join(', ') + ']').join(' + ');
        avviso('🎲 ' + (r.etichetta || '') + ': ' + dadi + (r.mod ? (r.mod > 0 ? ' + ' : ' − ') + Math.abs(r.mod) : '') + ' = ' + r.totale);
        break;
      }
      case 'scheda:tiroErrore':
        avviso('⚠ ' + (m.errore || 'Tiro non riuscito.'));
        break;
      case 'scheda:print':
        window.print();
        break;
    }
  });
  // Compatibilità con l'app (sezione Schede): lo stato corrente e una scheda vuota.
  window.SchedaAPI = { getState: () => JSON.parse(JSON.stringify(st)) };
  window.blank = () => ({});
  invia({ type: 'scheda:ready' });
})();
