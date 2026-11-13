/* ===== OGGETTI MAGICI nella scheda: pannello, editor del Master, scheda grafica, importa/esporta =====
   Si carica DOPO scheda.js (usa st, $, esc, toast, render, mag, magE, HOST, IS_MASTER) e PRIMA di avvio.js.
   Il calcolo dei modificatori NON è qui: è in /regole/oggetti.js (OggettiMagici) e in scheda.js (abil, acCalc, ...).
   Qui si legge e si scrive solo st.mag = [{o: oggetto, eq, sint, on, car}]. Ogni testo mostrato passa da esc(). */
const MGO = OggettiMagici;
const mgoVoci = () => { if (!Array.isArray(st.mag)) st.mag = []; return st.mag; };
/* Creare e modificare gli oggetti spetta al Master (nell'app); fuori dall'app, dove non ci sono ruoli, è libero. */
const mgoMaster = () => !HOST.on || IS_MASTER;
const mgoNomeDi = (lista, cod) => (lista.find((e) => e[0] === cod) || [cod, cod])[1];
const mgoTxt = (t) => esc(t).replace(/\n/g, '<br>');
const mgoSg = (n) => (n >= 0 ? '+' : '−') + Math.abs(n);

/* ---------- finestre ---------- */
function mgoChiudi() { const m = $('mgoModal'); if (m) m.remove(); }
function mgoModal(html, bloccato) {
  mgoChiudi();
  const m = document.createElement('div');
  m.className = 'modal on'; m.id = 'mgoModal';
  m.innerHTML = '<div class="mbox mgo" role="dialog" aria-modal="true">' + html + '</div>';
  document.body.appendChild(m);
  if (!bloccato) m.addEventListener('mousedown', (e) => { if (e.target === m) mgoChiudi(); });
  return m.firstChild;
}
document.addEventListener('keydown', (e) => { const m = $('mgoModal'); if (e.key === 'Escape' && m && !m.dataset.edit) mgoChiudi(); });

/* ---------- scheda grafica di un oggetto (aspetto «carta», richiesta dell'utente 2026-11-11) ----------
   Solo grafica: usa i campi che l'editor ha già, nessun dato nuovo e nessun calcolo nuovo.
   Nome → titolo gotico; sottotipo (o tipologia) → nastro; immagine → illustrazione; bonus dell'arma/armatura/scudo/
   accessorio e modificatori tecnici → «Bonus Magico»; per le armi una riga automatica «contro qualsiasi creatura»;
   effetto principale ed effetti secondari → righe con medaglione; capacità speciali → un'altra sezione; descrizione
   narrativa per il giocatore → frase in fondo. Nei testi: una riga = una voce; «Intestazione: resto» mette
   l'intestazione in corsivo; «Nome - danni» mette il nome in rosso; una riga che inizia con «#» apre un nuovo nastro
   («# Vendetta»). L'icona del medaglione si sceglie dalle parole della riga (non morti → teschio, ecc.).
   Tutte le altre informazioni restano nel riquadro «Dettagli» (mgoDettagliTecnici). Ogni testo passa da esc(). */
const MGO_ICO = {
  spade: '<g transform="rotate(-38 32 32)"><path d="M30 4h4l2 6v30h-8V10z"/><path d="M21 40h22v4H21z"/><path d="M30 44h4v10h-4z"/><circle cx="32" cy="57" r="3.5"/></g><g transform="rotate(38 32 32)"><path d="M30 4h4l2 6v30h-8V10z"/><path d="M21 40h22v4H21z"/><path d="M30 44h4v10h-4z"/><circle cx="32" cy="57" r="3.5"/></g>',
  teschio: '<path d="M32 9c-12 0-20 8-20 19 0 6 3 10 6 12v7c0 2 1 3 3 3h3v-5h3v5h4v-5h2v5h4v-5h3v5h3c2 0 3-1 3-3v-7c3-2 6-6 6-12 0-11-8-19-20-19zm-8 17c3 0 6 2 6 5s-3 4-6 4-5-2-5-4 2-5 5-5zm16 0c3 0 5 2 5 5s-2 4-5 4-6-1-6-4 3-5 6-5zm-8 10l3 5h-6z"/>',
  pipistrello: '<path d="M32 22l-3-5-1 6c-6-1-11 0-16-3 1 5 4 8 4 8-3 0-6 0-9-1 3 5 8 7 12 7 1 3 4 5 7 5l6 7 6-7c3 0 6-2 7-5 4 0 9-2 12-7-3 1-6 1-9 1 0 0 3-3 4-8-5 3-10 2-16 3l-1-6z"/>',
  sole: '<circle cx="32" cy="32" r="9"/><path d="M32 6l3 12h-6zM32 58l-3-12h6zM6 32l12-3v6zM58 32l-12 3v-6zM13.6 13.6l10.6 6.4-4.2 4.2zM50.4 50.4l-10.6-6.4 4.2-4.2zM13.6 50.4l6.4-10.6 4.2 4.2zM50.4 13.6l-6.4 10.6-4.2-4.2z"/>',
  fiamma: '<path d="M33 7c2 9-6 12-6 20 0 3 1 5 3 6-5-1-8-6-7-11-6 5-9 11-9 17 0 10 8 17 18 17s18-7 18-17c0-12-10-17-17-32zm-1 32c3 4 7 6 7 11 0 4-3 7-7 7s-7-3-7-7c0-4 4-6 7-11z"/>',
  scudo: '<path d="M32 7l20 7v14c0 14-9 24-20 29C21 52 12 42 12 28V14zm0 6l-14 5v10c0 10 6 18 14 22z"/>',
  fulmine: '<path d="M37 6L15 36h14l-4 22 24-32H35z"/>',
  fiocco: '<path d="M30 6h4v52h-4zM8.5 18.5l2-3.5 45 26-2 3.5zM8.5 45.5l45-26 2 3.5-45 26z"/><path d="M24 10l8 6 8-6 2 3-10 7-10-7zM24 54l8-6 8 6 2-3-10-7-10 7z"/>',
  cuore: '<path d="M32 54C14 41 8 32 8 23c0-7 5-12 12-12 5 0 9 3 12 7 3-4 7-7 12-7 7 0 12 5 12 12 0 9-6 18-24 31z"/>',
  stella: '<path d="M32 6l6 18h19L42 35l6 19-16-11-16 11 6-19L7 24h19z"/>',
};
function mgoIcona(t, def) {
  const s = String(t || '').toLowerCase();
  const R = [[/non[ -]?mort|zombi|schelet|spettr|fantasm|lich/, 'teschio'], [/antagonista|vampir|pipistrell|sangue/, 'pipistrello'],
    [/radios|sole|luce|alba|sacr/, 'sole'], [/fuoco|fiamm|brucia|infuocat/, 'fiamma'], [/fulmin|tuono|elettric/, 'fulmine'],
    [/freddo|ghiacc|gelo|neve/, 'fiocco'], [/guari|cura|vita|pf|punti ferita/, 'cuore'],
    [/\bca\b|classe armatura|protezion|resistenz|immun|difes|scudo/, 'scudo'], [/attacc|colpi|danno|danni|arma|lama|spada/, 'spade']];
  for (const [re, k] of R) if (re.test(s)) return k;
  return def;
}
const mgoMedaglione = (k) => '<span class="mgx-med" aria-hidden="true"><svg viewBox="0 0 64 64">' + (MGO_ICO[k] || MGO_ICO.stella) + '</svg></span>';
const mgoCap = (t) => { t = String(t || ''); return t.charAt(0).toUpperCase() + t.slice(1); };
/* una riga di testo del Master → {testa, forte, resto} (già pronti come HTML sicuro) */
function mgoVoce(riga) {
  let testa = '', corpo = riga;
  const c = riga.indexOf(':');
  if (c > 0 && c < 90 && c < riga.length - 1) { testa = riga.slice(0, c + 1).trim(); corpo = riga.slice(c + 1).trim(); }
  let forte = '', resto = corpo;
  const m = corpo.match(/^(.{1,60}?)\s+[-–—]\s+(.+)$/);
  if (m) { forte = m[1]; resto = '- ' + m[2]; }
  return { testa: esc(testa), forte: esc(forte), resto: esc(resto) };
}
/* testi del Master → sezioni [{titolo, righe:[{ico, html}]}]; «# Titolo» apre una sezione nuova */
function mgoSezioni(testi, titolo, icoDef, sez) {
  let cur = null;
  const apri = (t) => { cur = { titolo: t, righe: [] }; sez.push(cur); };
  testi.forEach((t) => String(t || '').split('\n').forEach((r) => {
    r = r.trim(); if (!r) return;
    if (/^#+\s*/.test(r)) { apri(r.replace(/^#+\s*/, '').trim() || titolo); return; }
    if (!cur) apri(titolo);
    const v = mgoVoce(r);
    cur.righe.push({ ico: mgoIcona(r, icoDef), html: (v.testa ? '<i class="mgx-testa">' + v.testa + '</i>' : '') + '<span>' + (v.forte ? '<b class="mgx-forte">' + v.forte + '</b> ' : '') + v.resto + '</span>' });
  }));
  return sez;
}
function mgoBonusRighe(o) {
  const R = [];
  if (o.tipo === 'arma') {
    const a = o.arma;
    if (a.bAtt || a.bDanno) R.push(mgoSg(a.bAtt) + ' al tiro per colpire, ' + mgoSg(a.bDanno) + ' al Danno');
  } else if (o.tipo === 'armatura') { if (o.armatura.bonus) R.push(mgoSg(o.armatura.bonus) + ' alla Classe Armatura'); }
  else if (o.tipo === 'scudo') { if (o.scudo.bonus) R.push(mgoSg(o.scudo.bonus) + ' alla Classe Armatura (oltre al ' + mgoSg(o.scudo.ca) + ' dello scudo)'); }
  else if (o.acc.bonus && o.acc.car) R.push(mgoSg(o.acc.bonus) + ' a ' + MGO.nomeParametro(o.acc.car));
  MGO.modDi(o).forEach((m) => R.push(MGO.descriviMod(m) + (m.nota ? ' (' + m.nota + ')' : '')));
  return R;
}
/* riga automatica delle armi: «Usata contro qualsiasi creatura: Spada lunga +2 - 1d8 Tagliente + For» */
function mgoRigaArma(o) {
  const a = o.arma, pr = String(a.prop || '').toLowerCase();
  const nome = a.base || o.sottotipo || mgoNomeDi(MGO.TIPI, o.tipo);
  const bonus = a.bAtt && a.bAtt === a.bDanno ? ' ' + mgoSg(a.bAtt) : '';
  const car = a.dist === 'distanza' ? 'Des' : pr.includes('accurata') ? 'For o Des' : 'For';
  const danno = a.dado === '0' ? '' : a.dado + ' ' + mgoCap(a.tipo) + ' + ' + car + (Array.isArray(a.extra) ? a.extra.map((x) => ' + ' + x.dadi + (x.mod ? mgoSg(x.mod) : '') + ' ' + mgoCap(x.tipo)).join('') : '');
  return '<i class="mgx-testa">Usata contro qualsiasi creatura:</i><span><b class="mgx-forte">' + esc(mgoCap(nome) + bonus) + '</b>' + (danno ? ' - ' + esc(danno) : '') + '</span>';
}
function mgoCard(o) {
  const sotto = o.sottotipo || mgoNomeDi(MGO.TIPI, o.tipo);
  const nastro = (t) => '<div class="mgx-nastro"><span>' + esc(t) + '</span></div>';
  const righe = (rr) => rr.map((r) => '<div class="mgx-riga">' + mgoMedaglione(r.ico) + '<div class="mgx-txt">' + r.html + '</div></div>').join('');
  const bonus = mgoBonusRighe(o);
  const sez = [];
  const titoloEff = o.tipo === 'arma' ? 'Effetti di Combattimento' : 'Effetti Magici';
  if (o.tipo === 'arma') sez.push({ titolo: titoloEff, righe: [{ ico: 'spade', html: mgoRigaArma(o) }] });
  mgoSezioni([o.princ, o.second], titoloEff, o.tipo === 'arma' ? 'spade' : 'stella', sez);
  mgoSezioni([o.speciali], 'Capacità Speciali', 'stella', sez);
  /* sezioni con lo stesso titolo consecutive si uniscono (es. la riga automatica dell'arma e gli effetti principali) */
  const unite = [];
  sez.forEach((s) => { const u = unite[unite.length - 1]; if (u && u.titolo === s.titolo) u.righe.push(...s.righe); else unite.push(s); });
  const sole = '<svg class="mgx-sole" viewBox="0 0 120 60" aria-hidden="true">' +
    Array.from({ length: 13 }, (_, i) => { const an = Math.PI * (i / 12), x = 60 - Math.cos(an) * 52, y = 56 - Math.sin(an) * 50; return '<path d="M' + (60 - Math.cos(an - 0.07) * 20).toFixed(1) + ' ' + (56 - Math.sin(an - 0.07) * 20).toFixed(1) + 'L' + x.toFixed(1) + ' ' + y.toFixed(1) + 'L' + (60 - Math.cos(an + 0.07) * 20).toFixed(1) + ' ' + (56 - Math.sin(an + 0.07) * 20).toFixed(1) + 'z"/>'; }).join('') +
    '<path d="M38 56a22 22 0 0 1 44 0z"/></svg>';
  const angolo = (c) => '<svg class="mgx-ang ' + c + '" viewBox="0 0 80 80" aria-hidden="true"><path d="M4 76V20C4 10 10 4 20 4h56" fill="none" stroke-width="3"/><path d="M12 76V26c0-8 6-14 14-14h50" fill="none" stroke-width="1.5"/>' +
    '<path d="M22 22c10-2 18 2 20 10-6-6-14-6-20-2 4-6 2-10 0-8zM22 22c-2 10 2 18 10 20-6-6-6-14-2-20-6 4-10 2-8 0z" stroke="none"/><circle cx="22" cy="22" r="4" stroke="none"/></svg>';
  return '<div class="mgx r-' + o.rar + '">' + angolo('tl') + angolo('tr') + angolo('bl') + angolo('br') +
    '<div class="mgx-cima">' + sole + '</div>' +
    '<div class="mgx-titolo"><h2>' + esc(o.nome) + '</h2></div>' +
    '<div class="mgx-sotto"><span>' + esc(sotto) + '</span></div>' +
    '<div class="mgx-ill">' + (o.img ? '<img alt="" src="' + o.img + '">' : '<div class="mgx-noimg">' + mgoMedaglione(mgoIcona(o.nome + ' ' + o.tipo, o.tipo === 'arma' ? 'spade' : 'stella')) + '</div>') + '</div>' +
    '<div class="mgx-perg">' +
    (bonus.length ? nastro('Bonus Magico') + '<div class="mgx-bonus">' + bonus.map((b) => '<div>' + esc(b) + '</div>').join('') + '</div>' : '') +
    unite.map((s, i) => (i || bonus.length ? '<div class="mgx-div" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M12 1l2.5 8.5L23 12l-8.5 2.5L12 23l-2.5-8.5L1 12l8.5-2.5z"/></svg></div>' : '') + nastro(s.titolo) + '<div class="mgx-righe">' + righe(s.righe) + '</div>').join('') +
    (!bonus.length && !unite.length ? '<div class="mgx-bonus mgx-vuoto">Nessun effetto descritto.</div>' : '') +
    '</div>' +
    (o.testo && o.testo.trim() ? '<div class="mgx-frase">' + mgoTxt(o.testo.trim()) + '</div>' : '') +
    '<div class="mgx-fondo">' + sole + '</div>' +
    '</div>' +
    '<details class="mgo-det"><summary>Dettagli dell\'oggetto</summary>' + mgoDettagliTecnici(o) + '</details>';
}
/* tutte le informazioni della scheda precedente (nulla si perde): rarità, slot, peso, valore, descrizione, uso, requisiti, dati */
function mgoDettagliTecnici(o) {
  const kv = (k, v) => (v === '' || v == null || v === false ? '' : '<div class="mgo-kv"><span>' + esc(k) + '</span><b>' + esc(v) + '</b></div>');
  const sez = (t, corpo) => (corpo ? '<div class="mgo-sec"><h4>' + t + '</h4>' + corpo + '</div>' : '');
  const par = (t) => (t && t.trim() ? '<p>' + mgoTxt(t) + '</p>' : '');
  const slot = MGO.slotInfo(o.slot)[1];
  let spec = '';
  if (o.tipo === 'arma') {
    const a = o.arma;
    spec = kv('Categoria', a.cat === 'marziale' ? 'Marziale' : 'Semplice') + kv('Tipo', a.dist === 'distanza' ? 'A distanza' : 'Corpo a corpo') + kv('Arma base', a.base) +
      kv('Competenza richiesta', a.comp === 'no' ? 'No' : 'Sì') + kv('Danno', a.dado + ' ' + a.tipo + (Array.isArray(a.extra) ? a.extra.map((x) => ' + ' + x.dadi + (x.mod ? mgoSg(x.mod) : '') + ' ' + x.tipo).join('') : '')) + kv('Bonus magico', mgoSg(a.bAtt) + ' per colpire, ' + mgoSg(a.bDanno) + ' ai danni') +
      kv('Proprietà', a.prop) + kv('Danno versatile', a.vers) + kv('Gittata', a.g1 ? a.g1 + ' / ' + a.g2 + ' m' : '') + kv('Munizioni', a.mun);
  } else if (o.tipo === 'armatura') {
    const a = o.armatura;
    spec = kv('Categoria', mgoNomeDi([['leggera', 'Leggera'], ['media', 'Media'], ['pesante', 'Pesante']], a.cat)) + kv('CA base', a.ca) + kv('Bonus magico', a.bonus ? mgoSg(a.bonus) : '') +
      kv('Destrezza', mgoNomeDi([['completo', 'Modificatore completo'], ['max2', 'Massimo +2'], ['nessuno', 'Nessuno']], a.des)) + kv('Forza richiesta', a.forza || '') + kv('Furtività', a.furt ? 'Svantaggio' : '');
  } else if (o.tipo === 'scudo') {
    const a = o.scudo;
    spec = kv('Bonus CA base', mgoSg(a.ca)) + kv('Bonus magico', a.bonus ? mgoSg(a.bonus) : '') + kv('Proprietà speciali', a.spec);
  } else {
    const a = o.acc;
    spec = kv('Tipo', a.tipo) + kv('Bonus di incantamento', a.bonus ? mgoSg(a.bonus) : '') + kv('Caratteristica modificata', a.car ? MGO.nomeParametro(a.car) : '') + kv('Effetto numerico', a.num) + kv('Effetto speciale', a.spec);
  }
  const mods = MGO.modDi(o).map((m) => '<li>' + esc(MGO.descriviMod(m)) + (m.nota ? ' <span class="mut">— ' + esc(m.nota) + '</span>' : '') + '</li>').join('');
  const u = o.uso;
  const uso = kv('Tipo di azione', mgoNomeDi(MGO.ATTIVAZIONI, u.att)) + kv('Modalità di attivazione', u.modo) + kv('Durata', u.durata) + kv('Cariche', u.cariche || '') + kv('Ricarica', u.ricarica) + kv('Limitazioni', u.limiti);
  const req = (o.sint ? '<div class="mgo-kv"><span>Sintonizzazione</span><b>Richiesta</b></div>' : '') + par(o.req) + (o.condiz ? '<h5>Condizioni per attivare i bonus</h5>' + par(o.condiz) : '') + (o.inter ? '<h5>Interazione con altri oggetti</h5>' + par(o.inter) : '');
  /* gli effetti (principale, secondari, speciali) e la descrizione narrativa sono già nella carta qui sopra */
  return '<div class="mgo-card r-' + o.rar + '">' +
    '<div><span class="rar r-' + o.rar + '">' + esc(mgoNomeDi(MGO.RARITA, o.rar)) + '</span>' + (o.sint ? ' <span class="rar">sintonizzazione</span>' : '') +
    '<div class="sm mut">' + esc(mgoNomeDi(MGO.TIPI, o.tipo)) + (o.sottotipo ? ' · ' + esc(o.sottotipo) : '') + ' · slot: ' + esc(slot) + '</div>' +
    '<div class="sm mut">' + (o.peso ? esc(o.peso) + ' kg' : 'peso non indicato') + ' · ' + (o.valore ? esc(o.valore) + ' mo' : 'valore non indicato') + '</div></div>' +
    sez('Descrizione', par(o.aspetto) + (o.mater ? '<h5>Materiali e caratteristiche</h5>' + par(o.mater) : '') + (o.storia ? '<h5>Origine e storia</h5>' + par(o.storia) : '')) +
    sez('Modifiche alla scheda', mods ? '<ul class="mgo-mods">' + mods + '</ul>' : '') + sez('Modalità di utilizzo', uso) + sez('Requisiti e condizioni', req) + sez('Dati specifici', spec) + '</div>';
}
function mgoDettaglio(i) {
  const v = mgoVoci()[i]; if (!v) return;
  const mot = MGO.motivoInattivo(v);
  mgoModal(mgoCard(v.o) + '<div class="sm ' + (mot ? 'warn' : 'ok') + '" style="margin-top:8px">' + (mot ? 'Ora non fa effetto: ' + esc(mot.toLowerCase()) + '.' : 'Equipaggiato: i suoi effetti sono in vigore.') + '</div>' +
    '<div class="mact"><button class="btn" id="mgoX">Chiudi</button></div>').querySelector('#mgoX').onclick = mgoChiudi;
}

/* ---------- pannello nella scheda ---------- */
function mgoRiepilogo(E) {
  let h = '';
  const ef = MGO.effettiAttivi(mag());
  if (ef.length) h += '<div class="mgo-eff"><b>Effetti in vigore</b>' + ef.map((x) => '<div><span class="mgo-n">' + esc(x.nome) + '</span>: ' + x.righe.map(esc).join(' · ') + '</div>').join('') + '</div>';
  const li = (t, arr) => (arr.length ? '<div><b>' + t + ':</b> ' + arr.map((x) => esc(x.t) + ' <span class="mut">(' + esc(x.da) + ')</span>').join(', ') + '</div>' : '');
  const d = li('Resistenze', E.testi.resist) + li('Immunità', E.testi.immun) + li('Competenze', E.testi.comp) + li('Altri effetti', E.testi.speciale);
  if (d) h += '<div class="mgo-eff"><b>Difese e capacità degli oggetti</b>' + d + '</div>';
  const pm = MGO.punteggi(mag());
  const sc = [...new Set([...E.scartati, ...MGO.AB.flatMap((a) => pm[a].scartati)])];
  if (sc.length) h += '<div class="sm warn">Bonus non cumulabili (stesso gruppo): non conta ' + sc.map(esc).join(', ') + '.</div>';
  return h;
}
function renderMag() {
  const box = $('magBox'); if (!box) return;
  const V = mag(), E = magE(), nSint = MGO.sintonizzati(V).length;
  let h = '<div class="mgo-h"><b>✨ OGGETTI MAGICI</b><span class="sm mut">' + nSint + '/' + MGO.MAX_SINTONIE + ' sintonie</span></div>';
  if (!V.length) h += '<div class="sm mut" style="margin:6px 0">Nessun oggetto magico. Importa quello ricevuto dal Master' + (mgoMaster() ? ' oppure creane uno nuovo' : '') + '.</div>';
  V.forEach((v, i) => {
    const o = v.o, mot = MGO.motivoInattivo(v);
    const haAtt = MGO.modDi(o).some((m) => m.cond === 'att' || m.dur === 'temp');
    h += '<div class="mgo-row' + (mot ? '' : ' on') + '" data-i="' + i + '">' +
      (o.img ? '<img class="mgo-th" alt="" src="' + o.img + '">' : '<div class="mgo-th mgo-noimg">✨</div>') +
      '<div class="mgo-main"><div><b class="mgo-name">' + esc(o.nome) + '</b> <span class="rar r-' + o.rar + '">' + esc(mgoNomeDi(MGO.RARITA, o.rar)) + '</span></div>' +
      '<div class="sm mut">' + esc(mgoNomeDi(MGO.TIPI, o.tipo)) + ' · ' + esc(MGO.slotInfo(o.slot)[1]) + (o.sint ? ' · sintonizzazione' : '') + '</div>' +
      (!mot && !MGO.haEffettiCalcolabili(o)
        ? '<div class="sm warn" title="Equipaggiarlo non cambia nessun numero della scheda">⚠ Nessun bonus calcolato: ' +
          (o.acc && o.acc.bonus && !o.acc.car ? 'manca la «Caratteristica modificata» del bonus di incantamento' : 'l\'oggetto non ha modificatori tecnici') +
          (mgoMaster() ? ' (✎ Modifica → sezione D, o «✨ Suggerisci dai testi»)' : ' (chiedi al Master di aggiungerli)') + '</div>'
        : '<div class="sm ' + (mot ? 'warn' : 'ok') + '">' + (mot ? esc(mot) : 'In vigore') + (!mot && haAtt && !v.on ? ' <span class="mut">· altri effetti si accendono con «Attivato»</span>' : '') + '</div>') +
      '<div class="mgo-ctl"><label class="tog"><input type="checkbox" data-mg="eq"' + (v.eq ? ' checked' : '') + '> Equipaggiato</label>' +
      (o.sint ? '<label class="tog"><input type="checkbox" data-mg="sint"' + (v.sint ? ' checked' : '') + '> Sintonizzato</label>' : '') +
      (haAtt ? '<label class="tog"><input type="checkbox" data-mg="on"' + (v.on ? ' checked' : '') + '> Attivato</label>' : '') +
      (o.uso.cariche ? '<span class="mgo-car" title="' + esc(o.uso.ricarica || 'Cariche') + '"><button class="linkbtn" data-mgc="-1" aria-label="Meno una carica">−</button> ' + (v.car == null ? o.uso.cariche : v.car) + '/' + o.uso.cariche + ' cariche <button class="linkbtn" data-mgc="1" aria-label="Più una carica">+</button></span>' : '') +
      '</div></div><div class="mgo-btns">' +
      '<button class="linkbtn" data-mgv="card" title="Scheda dell\'oggetto">📄</button>' +
      (mgoMaster() ? '<button class="linkbtn" data-mgv="edit" title="Modifica (Master)">✎</button><button class="linkbtn" data-mgv="dup" title="Duplica come variante (Master)">⧉</button>' : '') +
      '<button class="linkbtn" data-mgv="exp" title="Esporta in un file">⬆</button>' +
      '<button class="linkbtn" data-mgv="del" title="Rimuovi dall\'inventario">🗑</button></div></div>';
  });
  h += mgoRiepilogo(E);
  h += '<div class="mgo-act">' + (mgoMaster() ? '<button class="linkbtn" id="mgoNew">＋ Nuovo oggetto</button>' : '') + '<button class="linkbtn" id="mgoImp">⬇ Importa oggetto</button></div>';
  box.innerHTML = h;
  box.onchange = mgoCambia; box.onclick = mgoClic;
}
function mgoCambia(e) {
  const el = e.target.closest('[data-mg]'); if (!el) return;
  const i = +el.closest('.mgo-row').dataset.i, V = mgoVoci(), v = V[i]; if (!v) return;
  const k = el.dataset.mg;
  if (k === 'eq') {
    if (el.checked) { const r = MGO.puoEquipaggiare(V, v); if (!r.ok) { el.checked = false; toast(r.motivo, 5500); return; } }
    v.eq = el.checked;
    if (v.eq && v.o.sint && !v.sint) toast('Equipaggiato, ma i suoi effetti restano spenti finché non ti sintonizzi.', 4500);
  } else if (k === 'sint') {
    if (el.checked) { const r = MGO.puoSintonizzare(V, v); if (!r.ok) { el.checked = false; toast(r.motivo, 5500); return; } }
    v.sint = el.checked;
  } else if (k === 'on') v.on = el.checked;
  render();
}
function mgoClic(e) {
  const c = e.target.closest('[data-mgc]');
  if (c) { const V = mgoVoci(), v = V[+c.closest('.mgo-row').dataset.i]; if (!v) return; const max = v.o.uso.cariche, cur = v.car == null ? max : v.car; v.car = Math.max(0, Math.min(max, cur + (+c.dataset.mgc))); render(); return; }
  const b = e.target.closest('[data-mgv]');
  if (b) {
    const i = +b.closest('.mgo-row').dataset.i, V = mgoVoci(), v = V[i]; if (!v) return;
    const a = b.dataset.mgv;
    if (a === 'card') mgoDettaglio(i);
    else if (a === 'edit') mgoEditor(i);
    else if (a === 'dup') {
      const o = JSON.parse(JSON.stringify(v.o)); o.id = MGO.nuovoId(); o.v = 1; o.nome = (o.nome + ' (variante)').slice(0, 80);
      const r = MGO.unisci(V, MGO.sanificaOggetto(o)); if (r.esito === 'pieno') { toast('Hai raggiunto il massimo di ' + MGO.MAX_VOCI + ' oggetti magici.', 4500); return; }
      st.mag = r.voci; render(); toast('Variante creata: ' + o.nome + '. Modificala con ✎.', 4500);
    } else if (a === 'exp') mgoEsporta(v.o);
    else if (a === 'del') {
      if (b.dataset.sicuro) { V.splice(i, 1); render(); toast('Oggetto rimosso dall\'inventario.'); return; }
      b.dataset.sicuro = '1'; b.textContent = 'Sicuro?'; setTimeout(() => { if (b.isConnected) { delete b.dataset.sicuro; b.textContent = '🗑'; } }, 3500);
    }
    return;
  }
  if (e.target.id === 'mgoNew') mgoEditor(null);
  if (e.target.id === 'mgoImp') mgoImporta();
}

/* ---------- esportazione ---------- */
function mgoEsporta(o) {
  const testo = MGO.esporta(o);
  const b = mgoModal('<h2>Esporta «' + esc(o.nome) + '»</h2><p>Passa il file (o il testo qui sotto) al giocatore: lo importa dalla sua scheda con «⬇ Importa oggetto». Contiene la scheda completa, i modificatori e l\'immagine.</p>' +
    '<textarea class="txt" id="mgoTx" readonly style="min-height:190px;font-family:monospace;font-size:11px"></textarea>' +
    '<div class="mact"><button class="btn" id="mgoX">Chiudi</button><button class="btn" id="mgoCopia">Copia</button><button class="btn pri" id="mgoScarica">Scarica il file</button></div>');
  b.querySelector('#mgoTx').value = testo;
  b.querySelector('#mgoX').onclick = mgoChiudi;
  b.querySelector('#mgoScarica').onclick = () => {
    const slug = o.nome.toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 40) || 'oggetto';
    const a = document.createElement('a'); a.href = URL.createObjectURL(new Blob([testo], { type: 'application/json' })); a.download = 'oggetto-' + slug + '.json';
    document.body.appendChild(a); a.click(); a.remove(); setTimeout(() => URL.revokeObjectURL(a.href), 4000);
  };
  b.querySelector('#mgoCopia').onclick = () => {
    const ta = b.querySelector('#mgoTx'); ta.select();
    const manuale = () => toast('Seleziona il testo e copialo a mano.', 3500);
    if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(testo).then(() => toast('Copiato negli appunti.'), manuale);
    else { let ok = false; try { ok = document.execCommand('copy'); } catch (er) { /* ok resta false */ } ok ? toast('Copiato negli appunti.') : manuale(); }
  };
}

/* ---------- importazione ---------- */
function mgoImporta() {
  const b = mgoModal('<h2>Importa un oggetto magico</h2><p>Scegli il file .json ricevuto dal Master oppure incolla il testo. Il file viene controllato: se non è valido non viene importato nulla.</p>' +
    '<input type="file" id="mgoFile" accept=".json,application/json,text/plain"><textarea class="txt" id="mgoPaste" placeholder="…oppure incolla qui il testo dell\'oggetto" style="min-height:120px;margin-top:8px"></textarea>' +
    '<div id="mgoRes" class="sm" style="margin:8px 0"></div><div class="mact"><button class="btn" id="mgoX">Chiudi</button><button class="btn pri" id="mgoGo">Importa</button></div>');
  b.querySelector('#mgoX').onclick = mgoChiudi;
  b.querySelector('#mgoFile').onchange = (e) => {
    const f = e.target.files[0]; if (!f) return;
    if (f.size > 3e6) { b.querySelector('#mgoRes').innerHTML = '<span class="warn">File troppo grande.</span>'; return; }
    const r = new FileReader(); r.onload = () => { b.querySelector('#mgoPaste').value = String(r.result || ''); }; r.readAsText(f);
  };
  b.querySelector('#mgoGo').onclick = () => {
    const out = MGO.importa(b.querySelector('#mgoPaste').value), righe = [];
    out.errori.forEach((er) => righe.push('<div class="warn">✖ ' + esc(er) + '</div>'));
    let V = mgoVoci(), toccati = 0;
    out.oggetti.forEach((o) => {
      const r = MGO.unisci(V, o);
      const msg = { aggiunto: '✔ Aggiunto', aggiornato: '↻ Aggiornato alla versione ' + o.v, uguale: '= Già presente (stessa versione), non duplicato', vecchio: '✖ Hai già una versione più recente, non importato', pieno: '✖ Inventario magico pieno' }[r.esito];
      righe.push('<div class="' + (r.esito === 'aggiunto' || r.esito === 'aggiornato' ? 'ok' : 'warn') + '">' + msg + ': ' + esc(o.nome) + '</div>');
      if (r.esito === 'aggiunto' || r.esito === 'aggiornato') { V = r.voci; toccati++; }
    });
    if (toccati) { st.mag = V; render(); }
    b.querySelector('#mgoRes').innerHTML = righe.join('') || '<div class="warn">Niente da importare.</div>';
  };
}

/* ---------- editor del Master ---------- */
function mgoGruppo(cod) {
  if (['CA', 'INIZIATIVA', 'VELOCITA', 'PF_MAX', 'PERC_PASSIVA'].includes(cod)) return 'Statistiche';
  if (MGO.AB.includes(cod)) return 'Punteggi di caratteristica';
  if (cod === 'TS' || cod.startsWith('TS_')) return 'Tiri salvezza';
  if (cod === 'PROVE' || cod.startsWith('ABIL:')) return 'Prove e abilità';
  if (['ATTACCO', 'DANNO', 'ATT_INC', 'CD_INC'].includes(cod)) return 'Combattimento e incantesimi';
  return 'Competenze, difese e altro';
}
/* idx: posizione nell'inventario della scheda (null = oggetto nuovo). est (archivio del Master, web/scheda/archivio_oggetti.js):
   {oggetto: oggetto da modificare (o nulla per uno nuovo), salva(o, esporta): Promise<boolean>} — l'oggetto non sta in una scheda. */
function mgoEditor(idx, est) {
  const esistente = est ? (est.oggetto ? { o: est.oggetto } : null) : (idx != null ? mgoVoci()[idx] : null);
  const w = esistente ? JSON.parse(JSON.stringify(esistente.o)) : MGO.nuovoOggetto('anello');
  let anteprima = false;
  const b = mgoModal('<div id="mgoEd"></div>', true);
  $('mgoModal').dataset.edit = '1';
  const ed = b.querySelector('#mgoEd');
  const get = (p) => p.split('.').reduce((o, k) => (o == null ? o : o[k]), w);
  const set = (p, v) => { const k = p.split('.'); const last = k.pop(); k.reduce((o, x) => o[x], w)[last] = v; };
  const f = {
    in: (l, p, ex) => '<label class="mgo-f"><span>' + l + '</span><input data-p="' + p + '" value="' + esc(get(p)) + '" ' + (ex || '') + '></label>',
    num: (l, p, mn, mx, st2) => '<label class="mgo-f"><span>' + l + '</span><input type="number" data-n="1" data-p="' + p + '" value="' + esc(get(p)) + '" min="' + mn + '" max="' + mx + '" step="' + (st2 || 1) + '"></label>',
    sel: (l, p, opts) => '<label class="mgo-f"><span>' + l + '</span><select data-p="' + p + '">' + opts.map(([v, n]) => '<option value="' + esc(v) + '"' + (String(v) === String(get(p)) ? ' selected' : '') + '>' + esc(n) + '</option>').join('') + '</select></label>',
    ta: (l, p, h) => '<label class="mgo-f wide"><span>' + l + '</span><textarea class="txt" data-p="' + p + '" style="min-height:' + (h || 54) + 'px">' + esc(get(p)) + '</textarea></label>',
    ck: (l, p) => '<label class="tog"><input type="checkbox" data-b="1" data-p="' + p + '"' + (get(p) ? ' checked' : '') + '> ' + l + '</label>',
  };
  const valoreMod = (m, i) => {
    const d = 'data-mi="' + i + '" data-mk="val"';
    if (MGO.OPERAZIONI.find((x) => x[0] === m.op) && ['agg', 'sost', 'min', 'max'].includes(m.op)) return '<input type="number" ' + d + ' value="' + esc(m.val) + '" min="-30" max="60" step="1" aria-label="Valore">';
    if (m.op === 'formula') return '<input ' + d + ' value="' + esc(m.val) + '" placeholder="es. 13+DES" aria-label="Formula">';
    if (m.op === 'resist' || m.op === 'immun') return '<input ' + d + ' list="mgoDanni" value="' + esc(m.val) + '" placeholder="tipo di danno" aria-label="Tipo di danno">';
    if (m.op === 'speciale') return '<input ' + d + ' value="' + esc(m.val) + '" placeholder="testo" aria-label="Testo">';
    return '<span class="mut">—</span>';
  };
  const modRow = (m, i) => {
    const P = MGO.parametri(), gruppi = [...new Set(P.map((p) => mgoGruppo(p[0])))];
    const par = MGO.parametro(m.par);
    const ops = MGO.OPERAZIONI.filter((o) => par && par[2].includes(o[0]));
    return '<div class="mgo-mod"><div class="mgo-mr">' +
      '<select data-mi="' + i + '" data-mk="par" aria-label="Destinazione">' + gruppi.map((g) => '<optgroup label="' + esc(g) + '">' + P.filter((p) => mgoGruppo(p[0]) === g).map((p) => '<option value="' + esc(p[0]) + '"' + (p[0] === m.par ? ' selected' : '') + '>' + esc(p[1]) + '</option>').join('') + '</optgroup>').join('') + '</select>' +
      '<select data-mi="' + i + '" data-mk="op" aria-label="Operazione">' + ops.map((o) => '<option value="' + o[0] + '"' + (o[0] === m.op ? ' selected' : '') + '>' + o[1] + '</option>').join('') + '</select>' +
      valoreMod(m, i) + '<button class="linkbtn" data-mdel="' + i + '" title="Rimuovi il modificatore" aria-label="Rimuovi il modificatore">✖</button></div>' +
      '<div class="mgo-mr sm"><select data-mi="' + i + '" data-mk="cond" aria-label="Condizione">' + MGO.CONDIZIONI.map((c) => '<option value="' + c[0] + '"' + (c[0] === m.cond ? ' selected' : '') + '>Attivo se: ' + c[1].toLowerCase() + '</option>').join('') + '</select>' +
      '<select data-mi="' + i + '" data-mk="dur" aria-label="Durata">' + MGO.DURATE.map((c) => '<option value="' + c[0] + '"' + (c[0] === m.dur ? ' selected' : '') + '>' + c[1] + '</option>').join('') + '</select>' +
      '<label class="tog"><input type="checkbox" data-mi="' + i + '" data-mk="solo"' + (m.solo ? ' checked' : '') + '> solo senza armatura né scudo</label></div>' +
      '<div class="mgo-mr sm"><input data-mi="' + i + '" data-mk="gruppo" value="' + esc(m.gruppo || '') + '" placeholder="gruppo non cumulabile (facoltativo)" maxlength="40"><input data-mi="' + i + '" data-mk="nota" value="' + esc(m.nota || '') + '" placeholder="condizioni particolari (nota)" maxlength="140"></div></div>';
  };
  const speciale = () => {
    if (w.tipo === 'arma') return '<h3>E. Dati dell\'arma</h3><div class="mgo-g">' +
      f.sel('Categoria', 'arma.cat', [['semplice', 'Semplice'], ['marziale', 'Marziale']]) + f.sel('Tipo', 'arma.dist', [['mischia', 'Corpo a corpo'], ['distanza', 'A distanza']]) +
      f.sel('Arma base (compila i campi)', 'arma.base', [['', '(arma personalizzata)'], ...WEAPONS.map((x) => [x[0], x[0]])]) + f.sel('Competenza richiesta', 'arma.comp', [['si', 'Sì'], ['no', 'No']]) +
      f.in('Dado di danno', 'arma.dado', 'placeholder="1d8" maxlength="6"') + f.in('Tipo di danno', 'arma.tipo', 'maxlength="40"') +
      f.num('Bonus magico all\'attacco', 'arma.bAtt', -10, 10) + f.num('Bonus magico ai danni', 'arma.bDanno', -10, 10) + f.in('Proprietà', 'arma.prop', 'maxlength="200" placeholder="Accurata, Versatile…"') +
      f.in('Danno versatile', 'arma.vers', 'placeholder="1d10" maxlength="6"') + f.num('Gittata normale (m)', 'arma.g1', 0, 2000, 0.5) + f.num('Gittata lunga (m)', 'arma.g2', 0, 5000, 0.5) + f.in('Munizioni', 'arma.mun', 'maxlength="80"') + '</div>' +
      '<h3 class="mgo-sub">Danni aggiuntivi (altri tipi)</h3>' + (w.arma.extra || []).map((x, j) => '<div class="mgo-mr"><input data-ax="' + j + '" data-axk="dadi" value="' + esc(x.dadi) + '" maxlength="6" placeholder="1d6" aria-label="Dadi">' +
        '<select data-ax="' + j + '" data-axk="tipo" aria-label="Tipo di danno">' + MGO.TIPI_DANNO.map((d) => '<option' + (d === x.tipo ? ' selected' : '') + '>' + d + '</option>').join('') + '</select>' +
        '<input type="number" data-ax="' + j + '" data-axk="mod" value="' + esc(x.mod || '') + '" placeholder="+0" min="-20" max="20" aria-label="Modificatore">' +
        '<button class="linkbtn" data-axdel="' + j + '" aria-label="Togli questo danno">✖</button></div>').join('') +
      ((w.arma.extra || []).length < 6 ? '<button class="linkbtn" id="mgoAddEx">＋ Aggiungi danno di un altro tipo (es. 1d6 fuoco)</button>' : '');
    if (w.tipo === 'armatura') return '<h3>E. Dati dell\'armatura</h3><div class="mgo-g">' +
      f.sel('Categoria', 'armatura.cat', [['leggera', 'Leggera'], ['media', 'Media'], ['pesante', 'Pesante']]) + f.num('CA base', 'armatura.ca', 0, 30) + f.num('Bonus magico alla CA', 'armatura.bonus', 0, 10) +
      f.sel('Modificatore di Destrezza', 'armatura.des', [['completo', 'Completo'], ['max2', 'Massimo +2'], ['nessuno', 'Nessuno']]) + f.num('Forza minima richiesta', 'armatura.forza', 0, 30) + '<div class="mgo-f">' + f.ck('Svantaggio alle prove di Furtività', 'armatura.furt') + '</div></div>';
    if (w.tipo === 'scudo') return '<h3>E. Dati dello scudo</h3><div class="mgo-g">' + f.num('Bonus CA base', 'scudo.ca', 0, 10) + f.num('Bonus magico alla CA', 'scudo.bonus', 0, 10) + f.ta('Proprietà speciali', 'scudo.spec') + '</div>';
    return '<h3>E. Complemento / accessorio</h3><p class="mut sm">Il bonus di incantamento sulla caratteristica scelta viene applicato davvero alla scheda (da equipaggiato). Per tutto il resto usa i modificatori della sezione D.</p><div class="mgo-g">' +
      f.in('Tipologia', 'acc.tipo', 'maxlength="60" placeholder="es. Anello, Cintura…"') + f.num('Bonus di incantamento', 'acc.bonus', -10, 10) +
      f.sel('Caratteristica modificata', 'acc.car', [['', '(nessuna)'], ...MGO.parametri().filter((p) => p[2].includes('agg')).map((p) => [p[0], p[1]])]) +
      f.in('Effetto numerico (descrizione)', 'acc.num', 'maxlength="200"') + f.ta('Effetto speciale', 'acc.spec') + '</div>';
  };
  const avvisiHtml = () => {
    const senzaNome = !String(w.nome || '').trim();
    const o = MGO.sanificaOggetto(Object.assign({}, w, { nome: senzaNome ? '—' : w.nome }));
    const A = o ? MGO.avvisi(o) : [];
    if (senzaNome) A.unshift('Manca il nome.');
    return A.length ? '<div class="warn">' + A.map((x) => '⚠ ' + esc(x)).join('<br>') + '</div>' : '';
  };
  const ridisegna = () => {
    const sy = b.scrollTop;
    const corpo = '<h2>' + (esistente ? 'Modifica oggetto' : 'Nuovo oggetto magico') + '</h2>' +
      '<div class="sm mut">ID univoco: <code>' + esc(w.id) + '</code> · versione ' + esc(esistente ? esistente.o.v + 1 : w.v) + (esistente ? ' (salvando sale di uno: chi lo ha già lo aggiorna importandolo)' : '') + '</div>' +
      '<h3>A. Identificazione</h3><div class="mgo-g">' +
      f.in('Nome', 'nome', 'maxlength="80"') + f.sel('Tipologia', 'tipo', MGO.TIPI) + f.in('Sottotipo', 'sottotipo', 'maxlength="80" placeholder="es. spada lunga, anello, bacchetta…"') + f.sel('Rarità', 'rar', MGO.RARITA) +
      f.num('Peso (kg)', 'peso', 0, 100000, 0.1) + f.num('Valore commerciale (mo)', 'valore', 0, 1000000000) + f.sel('Slot di equipaggiamento', 'slot', MGO.SLOT.map((s) => [s[0], s[1]])) +
      (w.slot === 'mani' ? f.sel('Mani occupate', 'mani', [[1, 'Una mano'], [2, 'Due mani']]) : '') + '<div class="mgo-f">' + f.ck('Richiede sintonizzazione', 'sint') + '</div>' +
      '<div class="mgo-f wide"><span>Immagine</span><div class="mgo-imgrow">' + (w.img ? '<img class="mgo-th" alt="" src="' + w.img + '"><button class="linkbtn" id="mgoImgDel">rimuovi</button>' : '') + '<input type="file" id="mgoImg" accept="image/png,image/jpeg,image/webp"></div></div></div>' +
      '<h3>B. Descrizione</h3><div class="mgo-g">' + f.ta('Aspetto estetico', 'aspetto') + f.ta('Materiali e caratteristiche', 'mater') + f.ta('Origine e storia', 'storia') + f.ta('Descrizione narrativa per il giocatore', 'testo', 70) + '</div>' +
      '<h3>C. Proprietà magiche</h3><p class="mut sm">Come appaiono nella carta dell\'oggetto: una riga per ogni effetto. «Usata contro i non morti: Spada lunga +2 - 1d8 Tagliente + 2d8 Radiosi» → la parte prima dei due punti in corsivo, il nome prima del trattino in rosso. Una riga che inizia con <b>#</b> apre un nuovo nastro (es. «# Vendetta»). L\'icona si sceglie dalle parole (non morti, l’antagonista, fuoco, radioso…). La «Descrizione narrativa per il giocatore» diventa la frase in fondo; il sottotipo, il nastro sotto il titolo.</p><div class="mgo-g">' + f.ta('Effetto principale', 'princ') + f.ta('Effetti secondari', 'second') + f.ta('Capacità speciali', 'speciali') +
      f.sel('Tipo di azione richiesta', 'uso.att', MGO.ATTIVAZIONI) + f.in('Modalità di attivazione', 'uso.modo', 'maxlength="200" placeholder="parola di comando, tocco…"') + f.in('Durata', 'uso.durata', 'maxlength="120"') +
      f.num('Numero di cariche (0 = nessuna)', 'uso.cariche', 0, 999) + f.in('Modalità di ricarica', 'uso.ricarica', 'maxlength="200" placeholder="es. 1d6+1 cariche all\'alba"') + f.in('Limitazioni', 'uso.limiti', 'maxlength="400"') +
      f.ta('Requisiti per usare l\'oggetto', 'req') + f.ta('Condizioni per attivare i bonus', 'condiz') + f.ta('Interazione con altri oggetti', 'inter') + '</div>' +
      '<h3>D. Modificatori tecnici</h3><p class="mut sm">Cosa cambia nella scheda quando l\'oggetto è in vigore. Il valore originale del personaggio non viene mai toccato. Formule ammesse: somme di numeri, FOR DES COS INT SAG CAR (il modificatore), COMP (competenza) e LIV (livello).</p>' +
      '<datalist id="mgoDanni">' + MGO.TIPI_DANNO.map((d) => '<option value="' + d + '">').join('') + '</datalist>' +
      (w.mod.length ? w.mod.map(modRow).join('') : '<div class="sm mut">Nessun modificatore: l\'oggetto avrà solo descrizione' + (w.tipo === 'arma' || w.tipo === 'armatura' || w.tipo === 'scudo' ? ' e i dati della sezione E' : '') + '.</div>') +
      '<button class="linkbtn" id="mgoAddMod">＋ Aggiungi modificatore</button> <button class="linkbtn" id="mgoSugg" title="Legge i testi dell\'oggetto' + (est && est.testoManuale ? ' e il testo del manuale' : '') + ' e aggiunge i bonus riconosciuti">✨ Suggerisci dai testi</button>' + speciale() +
      '<div id="mgoAvv" style="margin-top:8px">' + avvisiHtml() + '</div>';
    ed.innerHTML = '<div id="mgoForm"' + (anteprima ? ' style="display:none"' : '') + '>' + corpo + '</div><div id="mgoPrev"' + (anteprima ? '' : ' style="display:none"') + '>' + (anteprima ? mgoCard(MGO.sanificaOggetto(w) || Object.assign(MGO.nuovoOggetto(w.tipo), { nome: '(senza nome)' })) : '') + '</div>' +
      '<div class="mact"><button class="btn" id="mgoAnn">Annulla</button><button class="btn" id="mgoPrevBtn">' + (anteprima ? '← Torna alla modifica' : '👁 Anteprima') + '</button><button class="btn" id="mgoSalvaExp">Salva ed esporta</button><button class="btn pri" id="mgoSalva">Salva</button></div>';
    b.scrollTop = sy;
  };
  const salva = (esporta) => {
    for (let i = 0; i < w.mod.length; i++) if (!MGO.sanificaMod(w.mod[i])) { toast('Il modificatore n. ' + (i + 1) + ' è incompleto o non valido: correggilo o toglilo.', 5500); return; }
    const o = MGO.sanificaOggetto(w);
    if (!o) { toast('Serve almeno il nome dell\'oggetto.', 4000); return; }
    if (est) { // archivio del Master: salva il server
      if (esistente) { o.id = esistente.o.id; o.v = esistente.o.v + 1; }
      Promise.resolve(est.salva(o)).then((ok) => { if (ok === false) return; mgoChiudi(); toast('Oggetto salvato: ' + o.nome + '.'); if (esporta) mgoEsporta(o); });
      return;
    }
    const V = mgoVoci();
    if (esistente && V[idx] === esistente) {
      o.id = esistente.o.id; o.v = esistente.o.v + 1;
      V[idx] = { o, eq: esistente.eq, sint: o.sint && esistente.sint, on: esistente.on, car: Math.min(esistente.car == null ? o.uso.cariche : esistente.car, o.uso.cariche) };
    } else {
      const r = MGO.unisci(V, o);
      if (r.esito === 'pieno') { toast('Hai raggiunto il massimo di ' + MGO.MAX_VOCI + ' oggetti magici.', 4500); return; }
      st.mag = r.voci;
    }
    render(); mgoChiudi(); toast('Oggetto salvato: ' + o.nome + '.');
    if (esporta) mgoEsporta(o);
  };
  /* campi semplici: si aggiorna w; alcuni cambiano la forma del modulo e lo ridisegnano */
  ed.addEventListener('input', (e) => { const t = e.target; if (t.dataset.p && t.type !== 'checkbox' && t.tagName !== 'SELECT' && t.type !== 'file') { set(t.dataset.p, t.dataset.n ? (parseFloat(t.value) || 0) : t.value); const a = ed.querySelector('#mgoAvv'); if (a) a.innerHTML = avvisiHtml(); } else if (t.dataset.mi && t.tagName === 'INPUT' && t.type !== 'checkbox') { const m = w.mod[+t.dataset.mi]; if (m) m[t.dataset.mk] = t.type === 'number' ? (parseFloat(t.value) || 0) : t.value; } });
  ed.addEventListener('change', (e) => {
    const t = e.target;
    if (t.dataset.p && (t.type === 'checkbox' || t.tagName === 'SELECT')) {
      const ant = get(t.dataset.p); set(t.dataset.p, t.type === 'checkbox' ? t.checked : (['mani'].includes(t.dataset.p) ? +t.value : t.value));
      if (t.dataset.p === 'tipo') { if (w.slot === MGO.nuovoOggetto(ant).slot) w.slot = MGO.nuovoOggetto(w.tipo).slot; if (w.tipo === 'bastone' || w.tipo === 'arma') w.mani = w.mani || 1; }
      if (t.dataset.p === 'arma.base') { const x = WEAPONS.find((k) => k[0] === w.arma.base); if (x) { w.arma.cat = x[1] === 'g' ? 'marziale' : 'semplice'; w.arma.dist = x[2] === 'd' ? 'distanza' : 'mischia'; w.arma.dado = x[3]; w.arma.tipo = x[4]; w.arma.prop = x[5]; } }
      if (['tipo', 'slot', 'sint', 'arma.base'].includes(t.dataset.p)) ridisegna(); else { const a = ed.querySelector('#mgoAvv'); if (a) a.innerHTML = avvisiHtml(); }
    } else if (t.dataset.mi) {
      const m = w.mod[+t.dataset.mi]; if (!m) return; const k = t.dataset.mk;
      if (t.type === 'checkbox') m.solo = t.checked ? MGO.SOLO_SENZA : ''; else if (k === 'par' || k === 'op' || k === 'cond' || k === 'dur') m[k] = t.value;
      if (k === 'par') { const p = MGO.parametro(m.par); if (!p[2].includes(m.op)) m.op = p[2][0]; }
      if (k === 'par' || k === 'op') m.val = ['agg', 'sost', 'min', 'max'].includes(m.op) ? (typeof m.val === 'number' ? m.val : 1) : m.op === 'vant' || m.op === 'svant' ? 0 : '';
      if (k === 'par' || k === 'op') ridisegna(); else { const a = ed.querySelector('#mgoAvv'); if (a) a.innerHTML = avvisiHtml(); }
    } else if (t.dataset.ax != null) { // danni aggiuntivi dell'arma (2026-11-11)
      const x = (w.arma.extra || [])[+t.dataset.ax]; if (!x) return;
      x[t.dataset.axk] = t.dataset.axk === 'mod' ? (parseInt(t.value) || 0) : t.value;
    } else if (t.id === 'mgoImg') mgoImmagine(t.files[0], (url) => { w.img = url; ridisegna(); });
  });
  ed.addEventListener('click', (e) => {
    const t = e.target;
    if (t.id === 'mgoAddEx') { if (!Array.isArray(w.arma.extra)) w.arma.extra = []; w.arma.extra.push({ dadi: '1d6', tipo: 'fuoco', mod: 0 }); ridisegna(); return; }
    if (t.dataset.axdel != null) { w.arma.extra.splice(+t.dataset.axdel, 1); ridisegna(); return; }
    if (t.id === 'mgoAddMod') { w.mod.push({ par: 'CA', op: 'agg', val: 1, cond: 'equip', dur: 'perm', nota: '', solo: '', gruppo: '' }); ridisegna(); }
    else if (t.id === 'mgoSugg') { // bonus riconosciuti nei testi (2026-11-11): «+1 alla CA», «Forza pari a 19», «resistenza ai danni da fuoco»…
      const testi = [w.nome, w.princ, w.second, w.speciali, w.acc && w.acc.num, w.testo, est && est.testoManuale].filter(Boolean).join('\n');
      const fatti = MGO.applicaSuggerimenti(w, MGO.suggerisci(testi, w));
      // bonus di incantamento senza destinazione: se nei testi c'è la CA, è la CA
      if (w.acc && w.acc.bonus && !w.acc.car && !['arma', 'armatura', 'scudo'].includes(w.tipo) && /\b(CA|C\.A\.?|Classe Armatura)\b/i.test(testi)) { w.acc.car = 'CA'; fatti.push('Bonus di incantamento sulla Classe Armatura'); }
      ridisegna();
      toast(fatti.length ? 'Aggiunti: ' + fatti.join('; ') + '. Controlla e salva.' : 'Nei testi non ho trovato bonus chiari: aggiungili con «＋ Aggiungi modificatore».', 6000);
    }
    else if (t.dataset.mdel != null && t.dataset.mdel !== '') { w.mod.splice(+t.dataset.mdel, 1); ridisegna(); }
    else if (t.id === 'mgoImgDel') { w.img = ''; ridisegna(); }
    else if (t.id === 'mgoAnn') mgoChiudi();
    else if (t.id === 'mgoPrevBtn') { anteprima = !anteprima; ridisegna(); }
    else if (t.id === 'mgoSalva') salva(false);
    else if (t.id === 'mgoSalvaExp') salva(true);
  });
  ridisegna();
}
/* ridimensiona l'immagine scelta: lato massimo 320 px, WebP o JPEG, entro il limite dei dati */
function mgoImmagine(file, fatto) {
  if (!file) return;
  if (!/^image\/(png|jpeg|webp)$/.test(file.type)) { toast('Usa un\'immagine PNG, JPEG o WebP.', 4000); return; }
  const r = new FileReader();
  r.onerror = () => toast('Non riesco a leggere l\'immagine.', 4000);
  r.onload = () => {
    const im = new Image();
    im.onerror = () => toast('Immagine non valida.', 4000);
    im.onload = () => {
      for (const [lato, q] of [[720, 0.8], [560, 0.75], [420, 0.72], [320, 0.7], [240, 0.6], [160, 0.6], [96, 0.5]]) {
        const sc = Math.min(1, lato / Math.max(im.width, im.height)), c = document.createElement('canvas');
        c.width = Math.max(1, Math.round(im.width * sc)); c.height = Math.max(1, Math.round(im.height * sc));
        c.getContext('2d').drawImage(im, 0, 0, c.width, c.height);
        let url = c.toDataURL('image/webp', q); if (!/^data:image\/webp/.test(url)) url = c.toDataURL('image/jpeg', q);
        if (url.length <= MGO.MAX_IMG) { fatto(url); return; }
      }
      toast('Immagine troppo pesante anche ridotta: scegline una più semplice.', 5000);
    };
    im.src = String(r.result);
  };
  r.readAsDataURL(file);
}
