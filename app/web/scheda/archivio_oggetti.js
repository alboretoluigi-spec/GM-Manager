/* ===== ARCHIVIO DEGLI OGGETTI MAGICI DEL MASTER (richiesta dell'utente del 2026-11-11) =====
   Pagina web/scheda/archivio_oggetti.html, aperta dentro Gaster Bless dalla sezione «✨ Oggetti magici»
   (web/sezioni/oggetti_magici.js). Qui il Master crea, modifica, duplica, importa ed esporta gli oggetti magici senza
   aprire nessuna scheda, e li consegna ai personaggi. Gli oggetti letti dai manuali («📥 Importa dai manuali», M3.3)
   arrivano qui con manuale e pagina; il loro testo lo vede solo il Master («📖 Testo»).
   L'editor e la carta dell'oggetto sono quelli della scheda (oggetti_ui.js, caricato DOPO questo file): qui si
   definiscono i pochi nomi che si aspetta ($, esc, toast, st, render, HOST, IS_MASTER). Le richieste al server
   passano dalla pagina che contiene questa (messaggi arch:request / arch:response), che le limita all'archivio.
   Ogni testo mostrato passa da esc(). */

/* ---------- nomi che oggetti_ui.js si aspetta dalla scheda ---------- */
const $ = (id) => document.getElementById(id);
const esc = (s) => String(s == null ? '' : s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
function toast(t, ms) { const e = $('toast'); e.textContent = t; e.classList.add('on'); clearTimeout(toast.t); toast.t = setTimeout(() => e.classList.remove('on'), ms || 2200); }
let st = { mag: [] }; // l'archivio non ha un inventario: l'editor salva con est.salva
const render = () => {};
const HOST = { on: false, info: { role: 'master' } };
const IS_MASTER = true;
const mag = () => [];

/* ---------- la pagina che ci contiene (Gaster Bless) ---------- */
const OSPITE = (() => {
  const on = window.parent !== window, origin = location.origin, pend = {};
  let n = 0, pronto;
  const ready = new Promise((r) => { pronto = r; });
  const tema = (t) => { if (t === 'light' || t === 'dark') document.documentElement.dataset.theme = t; };
  addEventListener('message', (e) => {
    if (!on || e.source !== window.parent || e.origin !== origin) return;
    const m = e.data || {};
    if (m.type === 'arch:init') { tema(m.theme); pronto(true); }
    else if (m.type === 'arch:theme') tema(m.theme);
    else if (m.type === 'arch:response') { const p = pend[m.id]; if (p) { delete pend[m.id]; m.ok ? p.res(m.data) : p.rej(new Error(m.error || 'Errore')); } }
  });
  if (on) window.parent.postMessage({ type: 'arch:ready' }, origin);
  setTimeout(() => pronto(false), 8000);
  return {
    on, ready,
    req(metodo, percorso, dati) {
      return new Promise((res, rej) => {
        if (!on) { rej(new Error('Apri questa pagina da Gaster Bless.')); return; }
        const id = 'a' + (++n); pend[id] = { res, rej };
        window.parent.postMessage({ type: 'arch:request', id, metodo, percorso, dati }, origin);
        setTimeout(() => { if (pend[id]) { delete pend[id]; rej(new Error('Gaster Bless non risponde: riprova.')); } }, 90000);
      });
    },
  };
})();

/* ---------- stato ---------- */
const ARC = { oggetti: [], personaggi: [], q: '', rar: '', tipo: '', orig: '' };
const percorso = (id, resto) => '/oggetti-magici/' + encodeURIComponent(id) + (resto || '');
const nomeDi = (lista, cod) => (lista.find((e) => e[0] === cod) || [cod, cod])[1];
const confronto = (a, b) => a.o.nome.localeCompare(b.o.nome, 'it', { sensitivity: 'base' });

async function arcCarica() {
  const r = await OSPITE.req('GET', '/oggetti-magici');
  ARC.oggetti = (r.oggetti || []).map((x) => Object.assign({}, x, { o: MGO.sanificaOggetto(Object.assign({}, x.oggetto, { img: '' })) })).filter((x) => x.o).sort(confronto);
  ARC.personaggi = r.personaggi || [];
}
/** l'oggetto completo (con l'immagine), dal server */
async function arcCompleto(id) {
  const r = await OSPITE.req('GET', percorso(id));
  const o = MGO.sanificaOggetto(r.oggetto);
  if (!o) throw new Error('Oggetto non leggibile.');
  return o;
}
async function arcAggiorna() {
  try { await arcCarica(); arcDisegnaLista(); } catch (e) { toast(e.message, 6000); }
}

/* ---------- disegno ---------- */
function arcDisegna() {
  const sel = (id, tutti, lista, val) => '<select id="' + id + '">' + '<option value="">' + tutti + '</option>' + lista.map(([v, n]) => '<option value="' + esc(v) + '"' + (v === val ? ' selected' : '') + '>' + esc(n) + '</option>').join('') + '</select>';
  $('arch').innerHTML =
    '<div class="arch-testa"><div><b class="arch-tit">✨ Oggetti magici</b><div class="sm mut">Il tuo archivio: crea gli oggetti qui e consegnali alle schede dei personaggi con «🎁 Consegna…». ' +
    'Quelli letti dai manuali arrivano con «📥 Importa dai manuali» (sezione Manuali).</div></div>' +
    '<div class="arch-az"><button class="btn pri" id="arcNuovo">＋ Nuovo oggetto</button><button class="btn" id="arcImporta">⬇ Importa da file</button><button class="btn" id="arcEsporta">⬆ Esporta tutti</button></div></div>' +
    '<div class="arch-filtri"><input id="arcQ" type="search" placeholder="Cerca per nome…" value="' + esc(ARC.q) + '" aria-label="Cerca per nome">' +
    sel('arcRar', 'Tutte le rarità', MGO.RARITA, ARC.rar) + sel('arcTipo', 'Tutte le tipologie', MGO.TIPI, ARC.tipo) +
    sel('arcOrig', 'Tutte le origini', [['manuale', '📥 Dai manuali'], ['pacchetto', '📦 Dai pacchetti'], ['mano', '✎ Creati da te']], ARC.orig) + '</div>' +
    '<div id="arcLista"></div>';
  arcDisegnaLista();
}
function arcFiltrati() {
  const q = ARC.q.trim().toLowerCase();
  return ARC.oggetti.filter((x) => (!q || (x.o.nome + ' ' + x.o.sottotipo).toLowerCase().includes(q)) && (!ARC.rar || x.o.rar === ARC.rar) && (!ARC.tipo || x.o.tipo === ARC.tipo) &&
    (!ARC.orig || (x.pacchetto ? 'pacchetto' : x.manualeNome ? 'manuale' : 'mano') === ARC.orig));
}
function arcRiga(x) {
  const o = x.o;
  const vecchi = x.possessori.filter((p) => p.v < o.v);
  const chi = x.possessori.length
    ? '🎒 Ce l\'hanno: ' + x.possessori.map((p) => esc(p.nome) + (p.v < o.v ? ' <span class="warn">(versione ' + esc(p.v) + ')</span>' : '')).join(', ')
    : '<span class="mut">Non l\'ha ancora nessuno.</span>';
  return '<div class="mgo-row arch-riga" data-id="' + esc(x.id) + '">' +
    (x.immagine ? '<img class="mgo-th" alt="" loading="lazy" src="/api/v1' + esc(percorso(x.id, '/immagine')) + '?v=' + esc(o.v) + '">' : '<div class="mgo-th mgo-noimg">✨</div>') +
    '<div class="mgo-main"><div><b class="mgo-name">' + esc(o.nome) + '</b> <span class="rar r-' + esc(o.rar) + '">' + esc(nomeDi(MGO.RARITA, o.rar)) + '</span> <span class="sm mut">v' + esc(o.v) + '</span></div>' +
    '<div class="sm mut">' + esc(nomeDi(MGO.TIPI, o.tipo)) + (o.sottotipo ? ' (' + esc(o.sottotipo) + ')' : '') +
    (MGO.slotInfo(o.slot)[1] !== nomeDi(MGO.TIPI, o.tipo) ? ' · slot: ' + esc(MGO.slotInfo(o.slot)[1]) : '') + (o.sint ? ' · richiede sintonizzazione' : '') +
    (MGO.haEffettiCalcolabili(o) ? '' : ' · <span class="warn" title="Equipaggiato non cambia nessun numero della scheda: ✎ Modifica → sezione D o «✨ Suggerisci dai testi»">⚠ nessun bonus calcolato</span>') + '</div>' +
    (x.manualeNome ? '<div class="sm mut">📥 ' + esc(x.manualeNome) + (x.pagina ? ', pagina ' + esc(x.pagina) + ' del PDF' : '') + '</div>' : '') +
    (x.pacchetto ? '<div class="sm mut arch-pacchetto" title="Modificandolo, la copia modificata entra nel tuo archivio e vale quella">📦 Dal pacchetto «' + esc(x.pacchetto) + '»</div>' : '') +
    '<div class="sm arch-chi">' + chi + (vecchi.length ? ' <button class="linkbtn" data-a="agg">↻ Aggiorna nelle schede</button>' : '') + '</div></div>' +
    '<div class="arch-btns"><button class="linkbtn" data-a="card">📄 Scheda</button><button class="linkbtn" data-a="edit">✎ Modifica</button>' +
    '<button class="linkbtn" data-a="give">🎁 Consegna…</button><button class="linkbtn" data-a="dup" title="Duplica come variante">⧉ Duplica</button>' +
    '<button class="linkbtn" data-a="exp" title="Esporta in un file">⬆ Esporta</button>' + (x.manualeNome ? '<button class="linkbtn" data-a="txt">📖 Testo</button>' : '') +
    (x.pacchetto ? '' : '<button class="linkbtn" data-a="del" title="Togli dall\'archivio (le schede lo tengono)">🗑</button>') + '</div></div>';
}
function arcDisegnaLista() {
  const box = $('arcLista'); if (!box) return;
  const L = arcFiltrati();
  if (!ARC.oggetti.length) {
    box.innerHTML = '<div class="arch-vuoto">Nessun oggetto ancora. Crea il primo con «＋ Nuovo oggetto», importa un file .json, leggi i tuoi manuali con «📥 Importa dai manuali» oppure importa un pacchetto di contenuti.</div>';
    return;
  }
  box.innerHTML = '<div class="sm mut arch-conta">' + (L.length === ARC.oggetti.length ? L.length + ' oggetti' : L.length + ' di ' + ARC.oggetti.length + ' oggetti') + '</div>' +
    (L.length ? L.map(arcRiga).join('') : '<div class="arch-vuoto">Nessun oggetto con questi filtri.</div>');
}

/* ---------- salvataggio e consegna ---------- */
async function arcSalva(o) {
  const r = await OSPITE.req('PUT', percorso(o.id), { oggetto: o });
  await arcCarica(); arcDisegnaLista();
  return r;
}
/** dall'editor: dopo il salvataggio propone di aggiornare le schede che hanno una versione precedente */
async function arcSalvaDaEditor(o) {
  let r;
  try { r = await arcSalva(o); } catch (e) { toast(e.message, 6000); return false; }
  const vecchi = (r.possessori || []).filter((p) => p.v < o.v);
  if (vecchi.length) setTimeout(() => arcProponiAggiornamento(o, vecchi), 0); // dopo che l'editor si è chiuso
  return true;
}
function arcProponiAggiornamento(o, vecchi) {
  const b = mgoModal('<h2>Aggiornare anche le schede?</h2><p>«' + esc(o.nome) + '» ora è alla versione ' + esc(o.v) + '. Ce l\'hanno con una versione precedente: <b>' +
    vecchi.map((p) => esc(p.nome)).join(', ') + '</b>.</p><p class="sm mut">Si conservano equipaggiato, sintonia e cariche (senza superare il nuovo massimo).</p>' +
    '<div id="arcEsiti" class="sm"></div><div class="mact"><button class="btn" id="arcNo">Non ora</button><button class="btn pri" id="arcSi">↻ Aggiorna nelle loro schede</button></div>');
  b.querySelector('#arcNo').onclick = mgoChiudi;
  b.querySelector('#arcSi').onclick = () => arcConsegna(o.id, vecchi.map((p) => p.id), b);
}
const ESITI = {
  aggiunto: ['ok', '✔ Consegnato a %'], aggiornato: ['ok', '↻ Aggiornato nella scheda di %'], uguale: ['mut', '= % ce l\'ha già (stessa versione)'],
  vecchio: ['warn', '✖ % ha una versione più recente: non toccato'], pieno: ['warn', '✖ L\'inventario magico di % è pieno'],
  scheda_alternativa: ['warn', '✖ % usa una scheda alternativa'], non_trovato: ['warn', '✖ Personaggio non trovato'],
};
async function arcConsegna(id, pgs, b) {
  const esiti = b.querySelector('#arcEsiti'), bott = b.querySelectorAll('.mact .pri');
  bott.forEach((x) => { x.disabled = true; });
  try {
    const r = await OSPITE.req('POST', percorso(id, '/consegna'), { personaggi: pgs });
    esiti.innerHTML = (r.risultati || []).map((x) => { const [c, t] = ESITI[x.esito] || ['warn', '? %']; return '<div class="' + c + '">' + esc(t).replace('%', '<b>' + esc(x.nome) + '</b>') + '</div>'; }).join('');
    b.querySelector('.mact .btn').textContent = 'Chiudi';
    await arcCarica(); arcDisegnaLista();
  } catch (e) { esiti.innerHTML = '<div class="warn">✖ ' + esc(e.message) + '</div>'; bott.forEach((x) => { x.disabled = false; }); }
}
function arcApriConsegna(x) {
  const o = x.o, ha = new Map(x.possessori.map((p) => [p.id, p.v]));
  if (!ARC.personaggi.length) { toast('Non ci sono ancora schede di Gaster Bless a cui consegnarlo.', 5000); return; }
  const righe = ARC.personaggi.map((p) => {
    const v = ha.get(p.id);
    const nota = v == null ? '' : v < o.v ? ' <span class="warn">· ha la versione ' + esc(v) + ': si aggiorna</span>' : ' <span class="mut">· ce l\'ha già</span>';
    return '<label class="arch-pg"><input type="checkbox" data-pg="' + esc(p.id) + '"' + (v != null && v >= o.v ? ' disabled' : '') + '> <b>' + esc(p.nome) + '</b>' +
      (p.giocatore ? ' <span class="mut">— ' + esc(p.giocatore) + '</span>' : '') + (p.stato && p.stato !== 'attivo' ? ' <span class="mut">(' + esc(p.stato) + ')</span>' : '') + nota + '</label>';
  }).join('');
  const b = mgoModal('<h2>🎁 Consegna «' + esc(o.nome) + '»</h2><p class="sm mut">L\'oggetto entra nel riquadro ✨ OGGETTI MAGICI della scheda (non equipaggiato): il giocatore lo vede subito, se ha la scheda aperta.</p>' +
    '<div class="arch-pgs">' + righe + '</div><div id="arcEsiti" class="sm" style="margin-top:8px"></div>' +
    '<div class="mact"><button class="btn" id="arcAnn">Annulla</button><button class="btn pri" id="arcDai">🎁 Consegna</button></div>');
  b.querySelector('#arcAnn').onclick = mgoChiudi;
  b.querySelector('#arcDai').onclick = () => {
    const pgs = [...b.querySelectorAll('[data-pg]:checked')].map((c) => c.dataset.pg);
    if (!pgs.length) { toast('Scegli almeno un personaggio.'); return; }
    arcConsegna(o.id, pgs, b);
  };
}

/* ---------- importa ed esporta ---------- */
function arcScarica(nome, testo) {
  const a = document.createElement('a'); a.href = URL.createObjectURL(new Blob([testo], { type: 'application/json' })); a.download = nome;
  document.body.appendChild(a); a.click(); a.remove(); setTimeout(() => URL.revokeObjectURL(a.href), 4000);
}
async function arcEsportaTutti() {
  if (!ARC.oggetti.length) { toast('L\'archivio è vuoto.'); return; }
  toast('Preparo il file…', 4000);
  try {
    const tutti = [];
    for (const x of ARC.oggetti) tutti.push(await arcCompleto(x.id));
    arcScarica('oggetti-magici-gasterbless.json', MGO.esportaVarie(tutti));
    toast('File pronto: ' + tutti.length + ' oggetti.', 3500);
  } catch (e) { toast(e.message, 6000); }
}
function arcImporta() {
  const b = mgoModal('<h2>Importa oggetti magici</h2><p>Scegli un file .json di oggetti di Gaster Bless (anche esportato da una scheda) oppure incolla il testo. Stesso oggetto: vale la versione più alta.</p>' +
    '<input type="file" id="arcFile" accept=".json,application/json,text/plain"><textarea class="txt" id="arcTesto" placeholder="…oppure incolla qui il testo" style="min-height:120px;margin-top:8px"></textarea>' +
    '<div id="arcEsiti" class="sm" style="margin:8px 0"></div><div class="mact"><button class="btn" id="arcX">Chiudi</button><button class="btn pri" id="arcVai">Importa</button></div>');
  b.querySelector('#arcX').onclick = mgoChiudi;
  b.querySelector('#arcFile').onchange = (e) => {
    const f = e.target.files[0]; if (!f) return;
    if (f.size > 3e6) { b.querySelector('#arcEsiti').innerHTML = '<span class="warn">File troppo grande.</span>'; return; }
    const r = new FileReader(); r.onload = () => { b.querySelector('#arcTesto').value = String(r.result || ''); }; r.readAsText(f);
  };
  b.querySelector('#arcVai').onclick = async () => {
    const out = MGO.importa(b.querySelector('#arcTesto').value), righe = out.errori.map((er) => '<div class="warn">✖ ' + esc(er) + '</div>');
    for (const o of out.oggetti) {
      const c = ARC.oggetti.find((x) => x.id === o.id);
      if (c && o.v < c.o.v) { righe.push('<div class="warn">✖ Nell\'archivio c\'è una versione più recente: ' + esc(o.nome) + '</div>'); continue; }
      if (c && o.v === c.o.v) { righe.push('<div class="mut">= Già presente (stessa versione): ' + esc(o.nome) + '</div>'); continue; }
      try { await OSPITE.req('PUT', percorso(o.id), { oggetto: o }); righe.push('<div class="ok">' + (c ? '↻ Aggiornato alla versione ' + esc(o.v) : '✔ Aggiunto') + ': ' + esc(o.nome) + '</div>'); }
      catch (e) { righe.push('<div class="warn">✖ ' + esc(o.nome) + ': ' + esc(e.message) + '</div>'); }
    }
    b.querySelector('#arcEsiti').innerHTML = righe.join('') || '<div class="warn">Niente da importare.</div>';
    await arcAggiorna();
  };
}

/* ---------- azioni sulle righe ---------- */
async function arcAzione(a, x, bottone) {
  try {
    if (a === 'card') {
      const o = await arcCompleto(x.id);
      const m = mgoModal(mgoCard(o) + '<div class="mact" style="margin-top:10px"><button class="btn" id="arcX">Chiudi</button></div>'); // la carta ha già «Dettagli dell'oggetto»
      m.querySelector('#arcX').onclick = mgoChiudi;
    } else if (a === 'edit') {
      const o = await arcCompleto(x.id);
      let testoManuale = ''; // per «✨ Suggerisci dai testi»: il testo del manuale lo legge solo l'editor del Master
      if (x.manualeNome) try { testoManuale = (await OSPITE.req('GET', '/elementi-manuali/testo?tipo=oggetto&voce=' + encodeURIComponent(x.id))).testo || ''; } catch { /* testo non disponibile */ }
      mgoEditor(null, { oggetto: o, salva: arcSalvaDaEditor, testoManuale });
    } else if (a === 'give') arcApriConsegna(x);
    else if (a === 'agg') {
      const vecchi = x.possessori.filter((p) => p.v < x.o.v);
      arcProponiAggiornamento(x.o, vecchi);
    } else if (a === 'dup') {
      const o = await arcCompleto(x.id);
      o.id = MGO.nuovoId(); o.v = 1; o.nome = (o.nome + ' (variante)').slice(0, 80);
      await arcSalva(o); toast('Variante creata: ' + o.nome + '. Modificala con ✎.', 4500);
    } else if (a === 'exp') mgoEsporta(await arcCompleto(x.id));
    else if (a === 'txt') {
      const t = await OSPITE.req('GET', '/elementi-manuali/testo?tipo=oggetto&voce=' + encodeURIComponent(x.id));
      const m = mgoModal('<h2>📖 ' + esc(t.nome || x.o.nome) + '</h2><div class="sm mut">📥 ' + esc(t.manualeNome) + (t.pagina ? ', pagina ' + esc(t.pagina) + ' del PDF' : '') +
        ' · lo vedi solo tu: i giocatori vedono solo ciò che scrivi nell\'oggetto.</div><div class="sdman arch-testo">' + (esc(t.testo) || '<span class="mut">(testo non letto)</span>').replace(/\n/g, '<br>') + '</div>' +
        '<div class="mact"><button class="btn" id="arcX">Chiudi</button></div>');
      m.querySelector('#arcX').onclick = mgoChiudi;
    } else if (a === 'del') {
      if (!bottone.dataset.sicuro) {
        bottone.dataset.sicuro = '1'; bottone.textContent = 'Sicuro? Clic di nuovo';
        setTimeout(() => { if (bottone.isConnected) { delete bottone.dataset.sicuro; bottone.textContent = '🗑'; } }, 3500);
        return;
      }
      await OSPITE.req('DELETE', percorso(x.id));
      toast('Tolto dall\'archivio. Le schede che lo hanno lo tengono.', 4500);
      await arcAggiorna();
    }
  } catch (e) { toast(e.message, 6000); }
}

/* ---------- avvio ---------- */
async function arcAvvia() {
  const box = $('arch');
  if (!OSPITE.on) { box.innerHTML = '<div class="warn">Questa pagina si apre da Gaster Bless: sezione «✨ Oggetti magici» (solo il Master).</div>'; return; }
  await OSPITE.ready;
  try { await arcCarica(); } catch (e) { box.innerHTML = '<div class="warn">Non riesco a leggere l\'archivio: ' + esc(e.message) + '</div>'; return; }
  arcDisegna();
  box.addEventListener('input', (e) => { if (e.target.id === 'arcQ') { ARC.q = e.target.value; arcDisegnaLista(); } });
  box.addEventListener('change', (e) => {
    const k = { arcRar: 'rar', arcTipo: 'tipo', arcOrig: 'orig' }[e.target.id];
    if (k) { ARC[k] = e.target.value; arcDisegnaLista(); }
  });
  box.addEventListener('click', (e) => {
    const t = e.target;
    if (t.id === 'arcNuovo') { mgoEditor(null, { salva: arcSalvaDaEditor }); return; }
    if (t.id === 'arcImporta') { arcImporta(); return; }
    if (t.id === 'arcEsporta') { arcEsportaTutti(); return; }
    const b = t.closest('[data-a]'), riga = t.closest('.arch-riga');
    if (!b || !riga) return;
    const x = ARC.oggetti.find((y) => y.id === riga.dataset.id);
    if (x) arcAzione(b.dataset.a, x, b);
  });
}
document.addEventListener('DOMContentLoaded', arcAvvia);
