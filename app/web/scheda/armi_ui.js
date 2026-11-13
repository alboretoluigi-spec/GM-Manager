/* ===== ARMI E INVENTARIO (Gaster Bless, 2026-11-11, Fase A del sistema di combattimento: docs/PIANO_COMBATTIMENTO.md) =====
   Si carica DOPO scheda.js e oggetti_ui.js. Due parti:
   1) «⚙» su una riga d'attacco: finestra con tutti i campi dell'arma (nome, magica, tipologia, proprietà, portata,
      gittata, competenza, caratteristica, bonus magici e aggiuntivi a colpire e ai danni, dado e tipo di danno, danni
      aggiuntivi di altri tipi) e anteprima calcolata con lo stesso calcolo della tabella (calcolaWp in scheda.js).
      Si salvano solo i campi diversi dal catalogo: cambiando l'arma i dati si aggiornano, le personalizzazioni restano.
   2) Elenco degli oggetti (st.inv = [{id, n: nome, q: quantità, eq: equipaggiato, k: arma|armatura|scudo|altro, ref:
      nome nel catalogo}]) con la casella «equipaggiato»: armatura → CA (st.armor), scudo → CA (st.shield), arma → riga
      d'attacco collegata (st.wp[].inv), visibile solo se equipaggiata. Le note di testo (st.eq) restano come sono
      (acquisti, strumenti, professioni le usano): ogni riga si può trasformare in oggetto.
   Ogni testo mostrato passa da esc(). Gli oggetti magici restano nel loro riquadro (oggetti_ui.js). */
(function () {
  'use strict';
  const INV_MAX = 200;
  const nuovoIdInv = () => 'inv-' + Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
  const invVoci = () => { if (!Array.isArray(st.inv)) st.inv = []; st.inv = st.inv.filter((x) => x && typeof x === 'object' && String(x.n || '').trim()); return st.inv; };
  const norm = (t) => String(t || '').toLowerCase().replace(/\s+/g, ' ').trim();

  /* che cos'è un oggetto, dal nome: arma o armatura del catalogo, scudo, altro */
  function tipoOggetto(n) {
    const k = norm(n);
    const w = WEAPONS.find((x) => norm(x[0]) === k);
    if (w) return ['arma', w[0]];
    const a = ARMORS.find((x) => norm(x[0]) === k || norm('armatura ' + x[0]) === k);
    if (a) return ['armatura', a[0]];
    if (/^scudo\b/.test(k)) return ['scudo', 'Scudo'];
    return ['altro', ''];
  }
  /* «2 × Pugnale», «Pugnale (2)», «Pugnale x2», «- Pugnale» → {n, q} */
  function leggiRiga(r) {
    let t = String(r || '').replace(/^[\s\-–•*·]+/, '').trim(), q = 1, m;
    if ((m = t.match(/^(\d{1,3})\s*(?:×|x|\*)?\s+(.+)$/i))) { q = +m[1]; t = m[2]; }
    else if ((m = t.match(/^(.+?)\s*(?:\((\d{1,3})\)|[×x]\s*(\d{1,3}))$/i))) { t = m[1]; q = +(m[2] || m[3]); }
    return { n: t.slice(0, 80).trim(), q: Math.max(1, Math.min(999, q || 1)) };
  }

  /* ---------- finestre ---------- */
  function armChiudi() { const m = $('armModal'); if (m) m.remove(); }
  function armModal(html) {
    armChiudi();
    const m = document.createElement('div');
    m.className = 'modal on'; m.id = 'armModal';
    m.innerHTML = '<div class="mbox mgo arm" role="dialog" aria-modal="true">' + html + '</div>';
    document.body.appendChild(m);
    m.addEventListener('mousedown', (e) => { if (e.target === m) armChiudi(); });
    return m.firstChild;
  }
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && $('armModal')) armChiudi(); });
  function scegli(titolo, testo, bottoni) { // bottoni: [[etichetta, classe, fn]]
    const b = armModal('<h2>' + esc(titolo) + '</h2><p>' + esc(testo) + '</p><div class="mact">' + bottoni.map((x, i) => '<button class="btn ' + (x[1] || '') + '" data-sc="' + i + '">' + esc(x[0]) + '</button>').join('') + '</div>');
    b.addEventListener('click', (e) => { const t = e.target.closest('[data-sc]'); if (!t) return; armChiudi(); const f = bottoni[+t.dataset.sc][2]; if (f) f(); });
  }

  /* ---------- 1) configurazione di una riga d'attacco ---------- */
  function predefiniti(w) { // valori del catalogo per l'arma base (per non salvare ciò che è uguale)
    const B = WEAPONS.find((x) => x[0] === w);
    if (!B) return null;
    const g = gittataDa(B[5]);
    return { cat: B[1], dist: B[2], dado: B[3], tipo: B[4], prop: B[5], port: B[2] === 'm' ? (B[6].includes('r') ? '3 m' : '1,5 m') : '', g1: g ? g[0] : '', g2: g ? g[1] : '' };
  }
  function apriConfig(i) {
    const orig = st.wp[i]; if (!orig) return;
    const E0 = wpEff(orig);
    const w = { w: orig.w || '', nome: orig.nome || '', mag: E0.mag, cat: E0.W ? E0.W[1] : 's', dist: E0.W ? E0.W[2] : 'm', prop: E0.W ? E0.W[5] : '', port: E0.port,
      g1: E0.g ? E0.g[0] : '', g2: E0.g ? E0.g[1] : '', comp: E0.comp, a: E0.a, bA: E0.bA, bD: E0.bD, mA: E0.mA, mD: E0.mD,
      dado: E0.W ? E0.W[3] : '1d6', tipo: E0.W ? E0.W[4] : 'contundente', extra: E0.extra.map((x) => ({ ...x })), inv: orig.inv, arg: E0.arg };
    const opzArmi = '<option value="">(arma personalizzata)</option>' + WEAPONS.map((x) => '<option value="' + esc(x[0]) + '">' + esc(x[0]) + '</option>').join('');
    const opz = (l, v) => l.map(([k, n]) => '<option value="' + esc(k) + '"' + (String(k) === String(v) ? ' selected' : '') + '>' + esc(n) + '</option>').join('');
    const tipiOpz = (v) => TIPI_DANNO_IT.map((t) => '<option' + (t === v ? ' selected' : '') + '>' + t + '</option>').join('');
    const campo = (l, html, wide) => '<label class="mgo-f' + (wide ? ' wide' : '') + '"><span>' + l + '</span>' + html + '</label>';
    const num = (k, mn, mx) => '<input type="number" data-k="' + k + '" value="' + esc(w[k]) + '" min="' + mn + '" max="' + mx + '" step="1">';
    const b = armModal('<div id="armCfg"></div>');
    const corpo = b.querySelector('#armCfg');
    const disegna = () => {
      const r = calcolaWp(salvabile());
      corpo.innerHTML = '<h2>⚙ Attacco: ' + esc(w.nome || w.w || 'nuovo') + '</h2>' +
        '<h3>Identificazione</h3><div class="mgo-g">' +
        campo('Nome dell\'arma', '<input data-k="nome" value="' + esc(w.nome) + '" maxlength="60" placeholder="' + esc(w.w || 'es. Lama del lupo') + '">') +
        campo('Arma base (dati dal catalogo)', '<select data-k="w">' + opzArmi + '</select>') +
        '<div class="mgo-f"><span>Arma magica</span><label class="tog"><input type="checkbox" data-k="mag"' + (w.mag ? ' checked' : '') + '> sì</label></div>' +
        '<div class="mgo-f"><span>Arma argentata</span><label class="tog" title="Supera le difese «da attacchi non magici non argentati» (lupi mannari…)"><input type="checkbox" data-k="arg"' + (w.arg ? ' checked' : '') + '> sì</label></div>' +
        campo('Tipologia', '<select data-k="cat">' + opz([['s', 'Semplice'], ['g', 'Da guerra']], w.cat) + '</select>') +
        campo('Uso', '<select data-k="dist">' + opz([['m', 'Da mischia'], ['d', 'A distanza']], w.dist) + '</select>') +
        campo('Proprietà', '<input data-k="prop" value="' + esc(w.prop) + '" maxlength="200" placeholder="Accurata, leggera, versatile (1d10), lancio (6/18 m)…">', true) +
        campo('Portata', '<input data-k="port" value="' + esc(w.port) + '" maxlength="20" placeholder="1,5 m">') +
        campo('Gittata normale (m)', '<input data-k="g1" value="' + esc(w.g1) + '" maxlength="8" inputmode="decimal">') +
        campo('Gittata massima (m)', '<input data-k="g2" value="' + esc(w.g2) + '" maxlength="8" inputmode="decimal">') + '</div>' +
        '<h3>Tiro per colpire</h3><div class="mgo-g">' +
        campo('Competenza', '<select data-k="comp">' + opz([['auto', 'Automatica (dalle competenze)'], ['si', 'Sempre'], ['no', 'Mai']], w.comp) + '</select>') +
        campo('Caratteristica', '<select data-k="a">' + opz([['auto', 'Automatica (Accurata: la migliore)'], ...AB.map((a) => [a, ABN[a]])], w.a) + '</select>') +
        campo('Bonus magico a colpire', num('bA', -10, 10)) + campo('Bonus aggiuntivo a colpire', num('mA', -20, 20)) + '</div>' +
        '<h3>Danni</h3><div class="mgo-g">' +
        campo('Dado di danno base', '<input data-k="dado" value="' + esc(w.dado) + '" maxlength="6" placeholder="1d8">') +
        campo('Tipo di danno base', '<select data-k="tipo">' + tipiOpz(w.tipo) + '</select>') +
        campo('Bonus magico ai danni', num('bD', -10, 10)) + campo('Bonus aggiuntivo ai danni', num('mD', -20, 20)) + '</div>' +
        '<h4 class="arm-sub">Danni aggiuntivi (altri tipi: restano separati per resistenze e immunità)</h4>' +
        (w.extra.length ? w.extra.map((x, j) => '<div class="mgo-mr arm-ex"><input data-x="' + j + '" data-xk="dadi" value="' + esc(x.dadi) + '" maxlength="6" placeholder="1d6" aria-label="Dadi">' +
          '<select data-x="' + j + '" data-xk="tipo" aria-label="Tipo">' + tipiOpz(x.tipo) + '</select><input type="number" data-x="' + j + '" data-xk="mod" value="' + esc(x.mod || '') + '" placeholder="+0" aria-label="Modificatore" min="-20" max="20">' +
          '<button class="linkbtn" data-xdel="' + j + '" title="Togli" aria-label="Togli questo danno">✖</button></div>').join('') : '<div class="sm mut">Nessuno.</div>') +
        (w.extra.length < 6 ? '<button class="linkbtn" id="armAddEx">＋ Aggiungi danno di un altro tipo</button>' : '') +
        '<div class="arm-prev">' + anteprima(r) + '</div>' +
        '<div class="mact"><button class="btn pericolo" id="armDel">Elimina attacco</button><button class="btn" id="armAnn">Annulla</button><button class="btn pri" id="armOk">Salva</button></div>';
      corpo.querySelector('[data-k="w"]').value = w.w;
    };
    function anteprima(r) {
      return r ? '<div><b>Tiro per colpire: d20 ' + r.atk + '</b> <span class="mut">(' + r.partiAtt + ')</span></div><div><b>Danni: ' + r.dmg + '</b></div><div class="sm">' + r.parti + '</div>' +
        '<div class="sm mut">Danno medio: ' + Math.round(r.profilo.danni.reduce((t, d) => t + medio(d.dadi) + d.mod, 0) * 10) / 10 + (r.profilo.danni.length > 1 ? ' (somma dei tipi, prima di resistenze e immunità)' : '') + '</div>'
        : '<span class="warn">Scegli un\'arma base oppure scrivi nome e dado di danno.</span>';
    }
    const soloAnteprima = () => { const p = corpo.querySelector('.arm-prev'); if (p) p.innerHTML = anteprima(calcolaWp(salvabile())); };
    const medio = (d) => { const m = String(d).match(/^(\d+)d(\d+)$/); return m ? +m[1] * (+m[2] + 1) / 2 : +d || 0; };
    /* la riga com'è da salvare: solo ciò che differisce dal catalogo */
    function salvabile() {
      const P = predefiniti(w.w), o = { w: w.w, a: w.a || 'auto', m: 0 };
      if (w.inv) o.inv = w.inv;
      if (w.nome && w.nome !== w.w) o.nome = w.nome.slice(0, 60);
      const diverso = (k, v) => !P || String(P[k]) !== String(v);
      ['cat', 'dist', 'prop', 'port', 'dado', 'tipo'].forEach((k) => { if (diverso(k, w[k]) && String(w[k]).trim() !== '') o[k] = String(w[k]).trim(); });
      if (!P && w.prop === '') o.prop = '';
      if (w.g1 && w.g2 && (diverso('g1', w.g1) || diverso('g2', w.g2))) { o.g1 = String(w.g1); o.g2 = String(w.g2); }
      if (w.comp !== 'auto') o.comp = w.comp;
      const bA = numOr(w.bA, 0), bD = numOr(w.bD, 0);
      if (bA === bD) o.m = bA; else { o.bA = bA; o.bD = bD; }
      if (numOr(w.mA, 0)) o.mA = numOr(w.mA, 0);
      if (numOr(w.mD, 0)) o.mD = numOr(w.mD, 0);
      if (w.mag && !(bA > 0 || bD > 0)) o.mag = true;
      if (w.arg) o.arg = true; // arma d'argento
      const ex = w.extra.filter((x) => DADI_OK(x.dadi) && x.dadi !== '0').map((x) => ({ dadi: x.dadi, tipo: x.tipo, mod: numOr(x.mod, 0) }));
      if (ex.length) o.extra = ex;
      return o;
    }
    corpo.addEventListener('input', (e) => {
      const t = e.target;
      if (t.dataset.k && t.type !== 'checkbox' && t.tagName !== 'SELECT') { w[t.dataset.k] = t.value; soloAnteprima(); }
      else if (t.dataset.x != null && t.tagName !== 'SELECT') { w.extra[+t.dataset.x][t.dataset.xk] = t.value; soloAnteprima(); }
    });
    corpo.addEventListener('change', (e) => {
      const t = e.target;
      if (t.dataset.k === 'w') { // nuova arma base: i campi presi dal catalogo si aggiornano
        const P = predefiniti(t.value);
        w.w = t.value;
        if (P) Object.assign(w, { cat: P.cat, dist: P.dist, dado: P.dado, tipo: P.tipo, prop: P.prop, port: P.port, g1: P.g1, g2: P.g2 });
        disegna(); return;
      }
      if (t.dataset.k) { w[t.dataset.k] = t.type === 'checkbox' ? t.checked : t.value; if (t.tagName === 'SELECT' || t.type === 'checkbox') disegna(); return; }
      if (t.dataset.x != null && t.tagName === 'SELECT') { w.extra[+t.dataset.x][t.dataset.xk] = t.value; disegna(); }
    });
    corpo.addEventListener('click', (e) => {
      const t = e.target;
      if (t.id === 'armAddEx') { w.extra.push({ dadi: '1d6', tipo: 'fuoco', mod: 0 }); disegna(); return; }
      if (t.dataset.xdel != null) { w.extra.splice(+t.dataset.xdel, 1); disegna(); return; }
      if (t.id === 'armAnn') { armChiudi(); return; }
      if (t.id === 'armOk') {
        const o = salvabile();
        if (!wpEff(o).W) { toast('Scegli un\'arma base oppure scrivi nome e dado di danno (es. 1d8).', 4500); return; }
        st.wp[i] = o; armChiudi(); render(); toast('Attacco salvato.'); return;
      }
      if (t.id === 'armDel') {
        scegli('Eliminare l\'attacco?', 'La riga sparisce dalla sezione Attacchi.' + (orig.inv ? ' L\'oggetto resta nell\'inventario; equipaggiandolo di nuovo l\'attacco si ricrea con i dati del catalogo.' : ''),
          [['Annulla', '', null], ['Elimina', 'pericolo', () => { st.wp.splice(i, 1); render(); }]]);
      }
    });
    disegna();
  }

  /* ---------- 2) inventario ---------- */
  function rigaArmaCollegata(id) { return st.wp.findIndex((x) => x && x.inv === id); }
  function collegaArma(it) {
    if (rigaArmaCollegata(it.id) >= 0) return;
    const vuota = st.wp.findIndex((x) => x && !x.w && !x.nome && !x.inv && !(+x.m) && !x.extra);
    const riga = { w: it.ref, a: 'auto', m: 0, inv: it.id };
    if (vuota >= 0) st.wp[vuota] = riga; else st.wp.push(riga);
  }
  function equipaggia(it, on) {
    it.eq = !!on;
    if (it.k === 'armatura') {
      if (on) { invVoci().forEach((x) => { if (x !== it && x.k === 'armatura') x.eq = false; }); st.armor = it.ref; }
      else if (st.armor === it.ref) st.armor = '';
    } else if (it.k === 'scudo') {
      st.shield = invVoci().some((x) => x.k === 'scudo' && x.eq);
    } else if (it.k === 'arma' && on) collegaArma(it);
    if (typeof buildCfg === 'function' && (it.k === 'armatura' || it.k === 'scudo')) buildCfg();
    render();
  }
  function aggiungi(nome, q) {
    const V = invVoci();
    if (V.length >= INV_MAX) { toast('Inventario pieno (' + INV_MAX + ' oggetti).', 4000); return null; }
    const [k, ref] = tipoOggetto(nome);
    const it = { id: nuovoIdInv(), n: String(nome).slice(0, 80).trim(), q: Math.max(1, Math.min(999, +q || 1)), eq: false, k, ref };
    V.push(it);
    if (k === 'arma') { it.eq = true; collegaArma(it); } // un'arma inserita genera subito la sua voce negli Attacchi
    return it;
  }
  function rimuovi(i) {
    const V = invVoci(), it = V[i]; if (!it) return;
    const fine = (togliAttacco) => {
      const r = rigaArmaCollegata(it.id);
      if (r >= 0) { if (togliAttacco) st.wp.splice(r, 1); else delete st.wp[r].inv; }
      V.splice(i, 1);
      if (it.k === 'armatura' && st.armor === it.ref && !V.some((x) => x.k === 'armatura' && x.ref === it.ref && x.eq)) st.armor = '';
      if (it.k === 'scudo') st.shield = V.some((x) => x.k === 'scudo' && x.eq);
      if (typeof buildCfg === 'function') buildCfg();
      render();
    };
    if (it.k === 'arma' && rigaArmaCollegata(it.id) >= 0) {
      scegli('Rimuovere «' + it.n + '»?', 'L\'arma ha una configurazione nella sezione Attacchi. Vuoi eliminare anche quella?',
        [['Annulla', '', null], ['No, tieni l\'attacco', '', () => fine(false)], ['Sì, elimina anche l\'attacco', 'pericolo', () => fine(true)]]);
    } else fine(false);
  }
  /* armatura e scudo scelti dal menu della configurazione: l'elenco si allinea (nessun doppio bonus: la CA viene solo da st.armor/st.shield) */
  function allinea() {
    const V = invVoci();
    let arm = false;
    V.forEach((x) => { if (x.k === 'armatura') { x.eq = !arm && !!st.armor && x.ref === st.armor; if (x.eq) arm = true; } });
    let sc = false;
    V.forEach((x) => { if (x.k === 'scudo') { x.eq = !sc && !!st.shield; if (x.eq) sc = true; } });
  }
  const ICONE = { arma: '⚔', armatura: '🥋', scudo: '🛡', altro: '•' };
  function renderInv() {
    const box = $('invBox'); if (!box) return;
    allinea();
    const V = invVoci(), nEq = V.filter((x) => x.eq).length;
    const lista = [...WEAPONS.map((x) => x[0]), ...ARMORS.map((x) => x[0]), 'Scudo', ...(typeof EQC === 'function' ? EQC().flatMap((k) => k.voci.map((v) => v[0])) : [])];
    let h = '<div class="inv-h"><b>🎒 OGGETTI</b><span class="sm mut">' + nEq + ' equipaggiat' + (nEq === 1 ? 'o' : 'i') + '</span></div>';
    if (!V.length) h += '<div class="sm mut" style="margin:4px 0">Nessun oggetto nell\'elenco. Aggiungine uno qui sotto o trasforma una riga delle note.</div>';
    h += V.map((it, i) => '<div class="inv-row' + (it.eq ? ' on' : '') + '" data-ii="' + i + '">' +
      '<button class="dot click inv-eq' + (it.eq ? ' on' : '') + '" data-inveq="' + i + '" aria-pressed="' + it.eq + '" title="' + (it.eq ? 'Equipaggiato: tocca per togliere' : 'Non equipaggiato: tocca per equipaggiare') + '" aria-label="Equipaggiato: ' + esc(it.n) + '"></button>' +
      '<span class="inv-k" title="' + esc(it.k) + '">' + ICONE[it.k] + '</span>' +
      '<input class="inv-n" data-invn="' + i + '" value="' + esc(it.n) + '" maxlength="80" aria-label="Nome dell\'oggetto">' +
      '<input class="inv-q" type="number" min="1" max="999" data-invq="' + i + '" value="' + esc(it.q) + '" aria-label="Quantità">' +
      (it.k === 'arma' ? '<button class="linkbtn solo-modifica" data-invcfg="' + i + '" title="Configura l\'attacco di quest\'arma" aria-label="Configura l\'attacco">⚙</button>' : '') +
      '<button class="linkbtn solo-modifica" data-invdel="' + i + '" title="Rimuovi" aria-label="Rimuovi ' + esc(it.n) + '">🗑</button></div>').join('');
    h += '<div class="inv-add solo-modifica"><input id="invNuovo" list="invLista" maxlength="80" placeholder="Aggiungi un oggetto (es. Spada lunga, Cotta di maglia, Scudo, Corda)…" aria-label="Nuovo oggetto">' +
      '<input id="invNuovoQ" type="number" min="1" max="999" value="1" aria-label="Quantità"><button class="linkbtn" id="invAgg">＋ Aggiungi</button></div>' +
      '<datalist id="invLista">' + [...new Set(lista)].map((n) => '<option value="' + esc(n) + '">').join('') + '</datalist>' +
      '<div class="inv-azioni solo-modifica"><button class="linkbtn" id="invDaNote">↪ Trasforma una riga delle note in oggetto</button></div>' +
      '<div class="sm mut">Equipaggiare un\'armatura o uno scudo cambia la CA; un\'arma equipaggiata compare negli Attacchi. Le note qui sotto restano come sono.</div>';
    box.innerHTML = h;
  }
  function daNote() {
    const righe = String(st.eq || '').split('\n').map((r, i) => [r, i]).filter(([r]) => r.trim() && !/^\s*—.*—\s*$/.test(r));
    if (!righe.length) { toast('Le note sono vuote.', 3000); return; }
    const b = armModal('<h2>↪ Riga delle note → oggetto</h2><p class="sm mut">Tocca una riga: diventa un oggetto dell\'elenco (con la quantità, se c\'è) e sparisce dalle note.</p>' +
      '<div class="inv-righe">' + righe.map(([r, i]) => '<button class="btn inv-riga" data-rn="' + i + '">' + esc(r.trim()) + '</button>').join('') + '</div>' +
      '<div class="mact"><button class="btn" id="armAnn">Chiudi</button></div>');
    b.addEventListener('click', (e) => {
      if (e.target.id === 'armAnn') { armChiudi(); return; }
      const t = e.target.closest('[data-rn]'); if (!t) return;
      const l = String(st.eq || '').split('\n'), idx = +t.dataset.rn, x = leggiRiga(l[idx]);
      if (!x.n) return;
      const it = aggiungi(x.n, x.q); if (!it) return;
      l.splice(idx, 1); st.eq = l.join('\n'); $('tEq').value = st.eq;
      armChiudi(); render(); toast('«' + it.n + '» aggiunto agli oggetti.');
    });
  }

  /* ---------- eventi ---------- */
  document.addEventListener('click', (e) => {
    const t = e.target;
    if (t.id === 'wpAdd') { st.wp.push({ w: '', a: 'auto', m: 0 }); render(); apriConfig(st.wp.length - 1); return; }
    if (t.dataset && t.dataset.wcfg != null) { apriConfig(+t.dataset.wcfg); return; }
    if (t.dataset && t.dataset.inveq != null) { const it = invVoci()[+t.dataset.inveq]; if (it) equipaggia(it, !it.eq); return; }
    if (t.dataset && t.dataset.invdel != null) { rimuovi(+t.dataset.invdel); return; }
    if (t.dataset && t.dataset.invcfg != null) { const it = invVoci()[+t.dataset.invcfg]; if (!it) return; if (!it.eq) equipaggia(it, true); const r = rigaArmaCollegata(it.id); if (r >= 0) apriConfig(r); return; }
    if (t.id === 'invAgg') {
      const n = $('invNuovo').value.trim(); if (!n) { $('invNuovo').focus(); return; }
      const it = aggiungi(n, $('invNuovoQ').value); if (!it) return;
      render(); toast('Aggiunto: ' + it.n + (it.k === 'arma' ? ' (equipaggiata, negli Attacchi)' : ''));
      return;
    }
    if (t.id === 'invDaNote') { daNote(); return; }
  });
  document.addEventListener('keydown', (e) => { if (e.key === 'Enter' && e.target && e.target.id === 'invNuovo') { e.preventDefault(); $('invAgg').click(); } });
  document.addEventListener('change', (e) => {
    const t = e.target;
    if (t.dataset && t.dataset.invn != null) { const it = invVoci()[+t.dataset.invn]; if (!it) return; const v = t.value.trim(); if (!v) { t.value = it.n; return; }
      it.n = v.slice(0, 80); if (it.k === 'altro') { const [k, ref] = tipoOggetto(v); if (k !== 'altro') { it.k = k; it.ref = ref; } } render(); return; }
    if (t.dataset && t.dataset.invq != null) { const it = invVoci()[+t.dataset.invq]; if (!it) return; it.q = Math.max(1, Math.min(999, parseInt(t.value) || 1)); save(); }
  });
  /* il pannello si ridisegna con la scheda */
  const _render = render;
  render = function () { _render(); try { renderInv(); } catch (er) { /* l'inventario non deve fermare la scheda */ } };
  window.ArmiInventario = { tipoOggetto, leggiRiga, aggiungi, equipaggia, rimuovi };
})();
