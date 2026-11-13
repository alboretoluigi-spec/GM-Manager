/* ===== LANCIO DEGLI INCANTESIMI DALLA SCHEDA (Gaster Bless, 2026-11-11, Parte 2 del piano: fase F1) =====
   Si carica DOPO scheda.js, oggetti_ui.js e armi_ui.js. Estende la finestra dell'incantesimo esistente (openInfo,
   lanciaIncantesimo in scheda.js) senza un secondo sistema:
   - blocco «Effetto» letto da /regole/incantesimi_effetti.js: tiro per colpire, TS (con la CD del personaggio), danni per
     tipo, cure, PF temporanei, condizioni, area/bersagli, Concentrazione Sì/No, Rituale Sì/No; «da risolvere con il
     Master» quando l'effetto non si legge con certezza;
   - «Rituale» solo se l'incantesimo è rituale E la classe lo permette (mago: dal libro anche non preparato; chierico,
     druido, artefice: preparato; bardo: conosciuto; talento Celebrante Rituale; warlock solo col Patto del Tomo, che la
     scheda non registra);
   - «Lancia incantesimo»: finestra della scelta dello slot (livelli disponibili e utilizzabili, effetto a ogni livello,
     tratto razziale senza slot); lo slot si toglie solo alla conferma; un secondo clic non lancia due volte. */
(function () {
  'use strict';
  const IE = IncantesimiEffetti;
  const RIT_PREPARATI = ['chierico', 'druido', 'artefice'];

  function voceElenco(s) { // l'incantesimo nella scheda: {k, p: preparato, o: classe} oppure null
    return Object.values(st.spl || {}).flat().find((x) => x && x.k === s.en) || null;
  }
  function voceAuto(s) { try { return autoSpells().find((x) => x.k === s.en) || null; } catch (e) { return null; } }

  /* Regole del lancio rituale (quinta edizione 2014): {ok, motivo} */
  function ritualeConsentito(s) {
    if (!s || String(s.t || '')[0] !== '*') return { ok: false, motivo: 'Questo incantesimo non è un rituale.' };
    const classi = KL().map((k) => k.id), v = voceElenco(s), a = voceAuto(s);
    if (typeof hasFeat === 'function' && hasFeat('Celebrante Rituale')) return { ok: true, motivo: 'talento Celebrante Rituale' };
    if (classi.includes('mago') && v) return { ok: true, motivo: 'dal libro degli incantesimi, anche se non preparato' };
    if (classi.includes('bardo') && (v || a)) return { ok: true, motivo: 'incantesimo conosciuto' };
    if (classi.some((c) => RIT_PREPARATI.includes(c))) {
      if ((v && v.p) || (a && a.modo === 'prep')) return { ok: true, motivo: 'incantesimo preparato' };
      return { ok: false, motivo: 'Per lanciarlo come rituale devi averlo preparato.' };
    }
    if (classi.includes('warlock')) return { ok: false, motivo: 'Il warlock lancia rituali solo con il Patto del Tomo (Libro delle ombre antiche), che la scheda non registra: chiedi al Master.' };
    return { ok: false, motivo: 'La tua classe non può lanciare incantesimi come rituali.' };
  }

  const modInc = () => { const cs = caster(); if (!cs) return null; const M = abil().M; return M[cs.ab]; };

  /* blocco «Effetto» nella finestra */
  function bloccoEffetto(s) {
    const r = IE.analizza(s), cs = caster(), mi = modInc(), L = LT();
    const righe = [];
    const riga = (k, v) => righe.push('<div><span class="lbl">' + k + '</span><b>' + v + '</b></div>');
    if (r.attacco) riga('Tiro per colpire', 'Attacco ' + (r.attacco.tipo === 'mischia' ? 'in mischia' : 'a distanza') + ' con incantesimo' + (cs && cs.active ? ': d20 ' + sg(cs.atk) : ''));
    if (r.ts) riga('Tiro salvezza', esc(r.ts.nome) + (cs && cs.active ? ', CD ' + cs.dc : '') + (r.ts.seRiesce === 'meta' ? ' · metà danni se riesce' : ' · nessun effetto se riesce') + (r.ts.ripetuto ? ' · si ripete a fine turno' : ''));
    const D = IE.danniAlLivello(r, s.l, L);
    if (D.length) riga('Danni', D.map((d) => esc(d.dadi + (d.mod === 'inc' ? (mi != null ? sg(mi) : ' + mod.') : d.mod ? sg(d.mod) : '') + ' ' + d.tipo)).join(' + '));
    if (r.cura) riga('Cura', esc(r.cura.dadi) + (r.cura.modInc ? (mi != null ? ' ' + sg(mi) : ' + mod.') : '') + ' PF (mai oltre il massimo)');
    if (r.pfTemp) riga('PF temporanei', esc(r.pfTemp.dadi) + (r.pfTemp.modInc && mi != null ? ' ' + sg(mi) : '') + ' (non si sommano ad altri)');
    if (r.ca) riga('Classe Armatura', r.ca.base ? 'CA base ' + r.ca.base + ' + Destrezza (non si somma a un\'armatura)' : '+' + r.ca.bonus);
    if (r.valori) riga('Finché dura', esc(IE.testoValori(r.valori)) + (r.valori.soloSe ? ' (su di te)' : '') + (r.valori.nota ? '<br><span class="sm mut">' + esc(r.valori.nota) + '</span>' : ''));
    if (r.condizioni.length) riga('Condizioni', r.condizioni.map(esc).join(', '));
    if (r.area) riga('Area', esc(r.area.forma + ' di ' + r.area.misura));
    if (r.bersagli) riga('Bersagli', r.bersagli + (r.superiori.bersagliPerSlot ? ' (+1 per ogni slot superiore)' : ''));
    riga('Concentrazione', r.conc ? 'Sì' : 'No');
    riga('Rituale', r.rituale ? 'Sì' : 'No');
    const rit = r.rituale ? ritualeConsentito(s) : null;
    const nota = r.automatico ? '<div class="sm ok">Effetto riconosciuto: nel tracker sarà applicato in automatico (con il controllo del Master).</div>'
      : '<div class="sm warn">Effetto da risolvere con il Master: la descrizione non si traduce in un\'azione automatica sicura.</div>';
    return '<div class="sdeff"><div class="sdeff-t">Effetto · ' + r.categorie.map(esc).join(', ') + '</div><div class="sdg">' + righe.join('') + '</div>' + nota +
      (rit && !rit.ok && $('sdCast').style.display !== 'none' ? '<div class="sm mut">Lancio come rituale non disponibile: ' + esc(rit.motivo) + '</div>' : '') + '</div>';
  }

  const _openInfo = openInfo;
  openInfo = function (s, ctx) {
    _openInfo(s, ctx);
    try {
      $('sdD').insertAdjacentHTML('beforeend', bloccoEffetto(s));
      if ($('sdRit').style.display !== 'none') {
        const rit = ritualeConsentito(s);
        if (!rit.ok) $('sdRit').style.display = 'none';
        else $('sdRit').title = 'Lancio come rituale: ' + rit.motivo + ' · +10 minuti, nessuno slot';
      }
      $('sdCast').disabled = false; $('sdRit').disabled = false;
    } catch (e) { /* il blocco dell'effetto non deve impedire di leggere l'incantesimo */ }
  };

  /* ---------- effetto per lo scontro (internal/server/incantesimi_scontro.go) ----------
     Valori già calcolati al livello dello slot: bonus d'attacco e CD della scheda, dadi dei danni e delle cure con il
     modificatore da incantatore. Li usa l'app (web/ui/incantesimo_scontro.js) per la scelta dei bersagli; il server li
     ricontrolla. null se l'effetto non si risolve in automatico. */
  window.effettoScontro = function (s, slot) {
    const r = IE.analizza(s); if (!r || !r.automatico) return null;
    const cs = caster(), mi = modInc() || 0, L = LT(), lv = slot || s.l;
    const t = String(s.t || '').replace(/^\*/, '');
    const tempo = t === 're' ? 'reazione' : t === 'ab' ? 'bonus' : /^a\b/.test(t) ? 'azione' : 'altro';
    const fix = (d) => ({ dadi: d.dadi, mod: d.mod === 'inc' ? mi : (+d.mod || 0), tipo: d.tipo });
    const e = { nome: s.it, livello: s.l, slot: lv, tempo, conc: !!r.conc, condizioni: r.condizioni || [], bersagli: IE.bersagliAlLivello(r, lv) || 0,
      round: r.durata && !r.durata.istantanea ? r.durata.round || 0 : 0, ripeti: !!(r.ts && r.ts.ripetuto),
      riassunto: IE.riassunto(r, lv, L, mi) };
    if (r.attacco) Object.assign(e, { tipo: 'attacco', mischia: r.attacco.tipo === 'mischia', bonus: cs ? cs.atk : 0, danni: IE.danniAlLivello(r, lv, L).map(fix) });
    else if (r.ts && (r.danni.length || r.condizioni.length)) Object.assign(e, { tipo: 'ts', cd: cs ? cs.dc : 10, car: r.ts.car, seRiesce: r.ts.seRiesce, danni: IE.danniAlLivello(r, lv, L).map(fix) });
    else if (r.cura) { const c = IE.curaAlLivello(r, lv); e.tipo = 'cura'; e.cura = { dadi: c.dadi, mod: c.modInc ? mi : 0 }; }
    else if (r.pfTemp) { const p = IE.pfTempAlLivello(r, lv); e.tipo = 'pftemp'; e.cura = { dadi: p.dadi, mod: p.modInc ? mi : 0 }; }
    else if (r.valori) { // potenziamenti (Scudo della fede, Benedizione…): effetto che cambia i valori
      Object.assign(e, IE.effettoValori(r));
      if (e.mod && e.mod.pfTempTurno === 'inc') e.mod = Object.assign({}, e.mod, { pfTempTurno: Math.max(1, mi) }); // Eroismo
      if (e.tsCar) e.tsCd = cs ? cs.dc : 10; // Anatema: TS dei bersagli con la CD della scheda
    }
    else return null;
    return e;
  };

  /* ---------- incantesimi automatici nel riepilogo (internal/server/incantesimi_remoto.go, 2026-11-12) ----------
     Per lanciarli senza la scheda aperta (gioco da remoto): gli incantesimi del PG che si risolvono da soli (attacco, TS,
     cura, PF temporanei), già calcolati a ogni livello di slot che ha, e gli slot totali per livello. Incantatori che
     preparano: solo quelli preparati (e i trucchetti); gli altri: tutti quelli dell'elenco; più quelli di sottoclasse e razza. */
  const TIPI_AUTO = ['attacco', 'ts', 'cura', 'pftemp'];
  let cacheAuto = { chiave: '', valore: {} };
  window.incantesimiAutomatici = function () {
    const cs = caster(); if (!cs || !cs.active) return {};
    const slots = {};
    for (let lv = 1; lv <= 9; lv++) { const t = slotTot(cs, lv); if (t > 0) slots[lv] = t; }
    const prep = cs.kLabel === 'Incantesimi preparati';
    const chiavi = [];
    Object.entries(st.spl || {}).forEach(([l, arr]) => (arr || []).forEach((x) => { if (x && x.k && (+l === 0 || !prep || x.p) && !chiavi.includes(x.k)) chiavi.push(x.k); }));
    try { autoSpells().forEach((x) => { if (!chiavi.includes(x.k)) chiavi.push(x.k); }); } catch (e) { /* senza sottoclasse */ }
    const chiave = JSON.stringify([chiavi, slots, cs.atk, cs.dc, LT(), modInc()]);
    if (cacheAuto.chiave === chiave) return cacheAuto.valore; // il riepilogo si ricalcola spesso: si rifà solo se cambia
    const out = [];
    for (const k of chiavi) {
      const sp = SPI[k]; if (!sp) continue;
      const livelli = [];
      for (const lv of sp.l === 0 ? [0] : Object.keys(slots).map(Number).filter((x) => x >= sp.l)) {
        let e = null; try { e = window.effettoScontro(sp, lv); } catch (er) { e = null; }
        if (!e || !TIPI_AUTO.includes(e.tipo)) continue;
        livelli.push({ slot: lv, tempo: e.tempo, conc: !!e.conc, tipo: e.tipo, bonus: e.bonus, cd: e.cd, car: e.car, seRiesce: e.seRiesce, danni: e.danni || [],
          condizioni: e.condizioni || [], cura: e.cura, mischia: !!e.mischia, round: e.round || 0, ripeti: !!e.ripeti, bersagli: e.bersagli || 0, riassunto: e.riassunto || '' });
      }
      if (livelli.length) out.push({ nome: sp.it, livello: sp.l, livelli });
      if (out.length >= 40) break;
    }
    cacheAuto = { chiave, valore: { spellsAuto: out, slots } };
    return cacheAuto.valore;
  };

  /* ---------- finestra della scelta dello slot ---------- */
  function chiudi() { const m = $('incModal'); if (m) m.remove(); }
  function finestra(html) {
    chiudi();
    const m = document.createElement('div');
    m.className = 'modal on'; m.id = 'incModal';
    m.innerHTML = '<div class="mbox inc" role="dialog" aria-modal="true">' + html + '</div>';
    document.body.appendChild(m);
    m.addEventListener('mousedown', (e) => { if (e.target === m) chiudi(); });
    return m.firstChild;
  }
  /* ogni finestra lancia una volta sola (il pulsante si disattiva al primo clic): niente doppio clic */
  function lancia(rituale, scelta) { chiudi(); lanciaIncantesimo(rituale, scelta); }
  function sceltaSlot() {
    const s = infoSp, cs = caster(); if (!s) return;
    if (s.l === 0) { lancia(false, null); return; } // trucchetto: nessuno slot
    const r = IE.analizza(s), mi = modInc(), L = LT(), a = voceAuto(s);
    const livelli = [];
    for (let lv = s.l; lv <= 9; lv++) { const tot = slotTot(cs, lv); if (tot > 0) livelli.push({ lv, tot, liberi: Math.max(0, tot - (st.slotU[lv] || 0)) }); }
    const primo = livelli.find((x) => x.liberi > 0);
    const senza = a && a.modo === 'razza';
    if (!primo && !senza) { toast('Nessuno slot libero di ' + s.l + '° livello o superiore: recupera gli slot con un riposo.', 4500); return; }
    const sup = r.superiori && r.superiori.testo;
    const b = finestra('<h2>✨ ' + esc(s.it) + '</h2><p class="sm mut">Livello base: ' + s.l + '° · scegli lo slot da usare' + (sup ? '. Ai livelli superiori: ' + esc(sup) : ' (questo incantesimo non migliora ai livelli superiori)') + '</p>' +
      '<div class="inc-slot">' + livelli.map((x) => '<label class="inc-lv' + (x.liberi ? '' : ' esaurito') + '"><input type="radio" name="incLv" value="' + x.lv + '"' + (x.liberi ? '' : ' disabled') + (primo && x.lv === primo.lv ? ' checked' : '') + '>' +
        '<b>' + x.lv + '° livello</b><span class="inc-n">' + x.liberi + '/' + x.tot + ' liberi</span><span class="inc-e">' + (x.liberi ? esc(IE.riassunto(r, x.lv, L, mi) || (x.lv > s.l ? 'stesso effetto' : '')) : 'esaurito') + '</span></label>').join('') +
        (senza ? '<label class="inc-lv"><input type="radio" name="incLv" value="0"' + (!primo ? ' checked' : '') + '><b>Senza slot</b><span class="inc-n">tratto razziale</span><span class="inc-e">di solito una volta per riposo lungo: ricordalo tu</span></label>' : '') + '</div>' +
      '<p class="sm mut">Lo slot si toglie solo quando confermi.</p><div class="mact"><button class="btn" id="incAnn">Annulla</button><button class="btn pri" id="incOk">Lancia</button></div>');
    let fatto = false;
    b.addEventListener('click', (e) => {
      if (e.target.id === 'incAnn') { chiudi(); return; }
      if (e.target.id !== 'incOk' || fatto) return;
      const v = (b.querySelector('input[name="incLv"]:checked') || {}).value;
      if (v == null) { toast('Scegli uno slot.', 3000); return; }
      fatto = true; e.target.disabled = true;
      if (+v === 0) lancia(false, { senzaSlot: 'tratto razziale', effetto: IE.riassunto(r, s.l, L, mi) });
      else lancia(false, { slot: +v, effetto: IE.riassunto(r, +v, L, mi) });
    });
  }
  function confermaRituale() {
    const s = infoSp; if (!s) return;
    const rit = ritualeConsentito(s);
    if (!rit.ok) { toast(rit.motivo, 5000); return; }
    const b = finestra('<h2>📖 ' + esc(s.it) + ' come rituale</h2><p>Nessuno slot consumato. Il tempo di lancio aumenta di <b>10 minuti</b> (' + esc(fmtCast(s.t)) + ' + 10 minuti).</p><p class="sm mut">Consentito: ' + esc(rit.motivo) + '.</p>' +
      '<div class="mact"><button class="btn" id="incAnn">Annulla</button><button class="btn pri" id="incOk">Lancia come rituale</button></div>');
    let fatto = false;
    b.addEventListener('click', (e) => {
      if (e.target.id === 'incAnn') { chiudi(); return; }
      if (e.target.id === 'incOk' && !fatto) { fatto = true; e.target.disabled = true; lancia(true, null); }
    });
  }
  $('sdCast').onclick = sceltaSlot;
  $('sdRit').onclick = confermaRituale;
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && $('incModal')) chiudi(); });
  window.IncantesimiScheda = { ritualeConsentito, bloccoEffetto };
})();
