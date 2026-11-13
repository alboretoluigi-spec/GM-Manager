// Scheda d'esempio per Gaster Bless: mostra il protocollo minimo (vedi ../PROTOCOLLO.md).
// Gira in una sandbox: niente localStorage, niente rete, niente cookie. I dati arrivano e partono solo
// con postMessage verso la pagina che la contiene (window.parent).
(function () {
  'use strict';
  var CAR = ['FOR', 'DES', 'COS', 'INT', 'SAG', 'CAR'];
  // Lo stato della scheda: un oggetto JSON qualsiasi. Alcune chiavi hanno un significato per Gaster Bless:
  //   f.nome  il nome del personaggio (lo scrive Gaster Bless alla creazione)
  //   hpCur, hpTmp  PF attuali e temporanei (condivisi con il tracker dell'iniziativa)
  var stato = { f: { nome: '' }, classe: '', livello: 1, razza: '', ca: 10, pfMax: 10, comp: 2,
    car: { FOR: 10, DES: 10, COS: 10, INT: 10, SAG: 10, CAR: 10 }, hpCur: 10, hpTmp: 0 };
  var ruolo = 'player', attesa = 0;
  var $ = function (id) { return document.getElementById(id); };
  var mod = function (v) { return Math.floor((v - 10) / 2); };
  var segno = function (n) { return (n >= 0 ? '+' : '') + n; };

  // Messaggi verso Gaster Bless: sempre a window.parent, con destinazione "*" (la pagina è isolata e
  // Gaster Bless controlla da quale riquadro arrivano).
  function invia(m) { window.parent.postMessage(m, '*'); }

  // Riepilogo per Gaster Bless: tracker, elenco dei personaggi, Sanità mentale dalla Saggezza (chiavi in inglese, come nella
  // scheda di Gaster Bless). Solo numeri e testi brevi.
  function riepilogo() {
    var c = stato.car;
    return {
      name: stato.f.nome, class: stato.classe, level: +stato.livello || 1, race: stato.razza,
      ac: +stato.ca || 10, hpMax: +stato.pfMax || 1, hpCur: +stato.hpCur || 0, hpTemp: +stato.hpTmp || 0,
      initiative: mod(c.DES), dex: c.DES, passivePerception: 10 + mod(c.SAG), proficiency: +stato.comp || 2,
      abilities: { FOR: c.FOR, DES: c.DES, COS: c.COS, INT: c.INT, SAG: c.SAG, CAR: c.CAR }
    };
  }

  // Ogni modifica: si manda tutto lo stato (Gaster Bless salva solo i campi cambiati), con un piccolo ritardo.
  function modificato() {
    clearTimeout(attesa);
    attesa = setTimeout(function () { invia({ type: 'scheda:state', state: stato, summary: riepilogo() }); }, 500);
  }

  function disegna() {
    $('titolo').textContent = stato.f.nome || 'Scheda d\'esempio';
    ['nome', 'classe', 'livello', 'razza', 'ca', 'pfMax', 'comp'].forEach(function (k) {
      var v = k === 'nome' ? stato.f.nome : stato[k];
      if (document.activeElement !== $(k)) $(k).value = v == null ? '' : v;
    });
    if (document.activeElement !== $('hpCur')) $('hpCur').value = stato.hpCur;
    if (document.activeElement !== $('hpTmp')) $('hpTmp').value = stato.hpTmp;
    var box = $('car');
    if (!box.children.length) {
      CAR.forEach(function (k) {
        var d = document.createElement('div'); d.className = 'car';
        d.innerHTML = '<b>' + k + '</b><br><input type="number" min="1" max="30" data-car="' + k + '"><br><small data-mod="' + k + '"></small><br>' +
          '<button type="button" data-tiro="' + k + '">🎲 Prova</button>';
        box.appendChild(d);
      });
    }
    CAR.forEach(function (k) {
      var inp = box.querySelector('[data-car="' + k + '"]');
      if (document.activeElement !== inp) inp.value = stato.car[k];
      box.querySelector('[data-mod="' + k + '"]').textContent = segno(mod(stato.car[k]));
    });
  }

  // Dati in entrata dalla scheda salvata: si tengono solo i valori del tipo giusto.
  function carica(s) {
    if (!s || typeof s !== 'object') return;
    if (s.f && typeof s.f.nome === 'string') stato.f.nome = s.f.nome;
    ['classe', 'razza'].forEach(function (k) { if (typeof s[k] === 'string') stato[k] = s[k].slice(0, 60); });
    ['livello', 'ca', 'pfMax', 'comp'].forEach(function (k) { if (typeof s[k] === 'number') stato[k] = s[k]; });
    // hpCur e hpTmp possono arrivare come testo (li scrive anche il tracker di Gaster Bless)
    ['hpCur', 'hpTmp'].forEach(function (k) { if (s[k] !== undefined && s[k] !== '' && isFinite(+s[k])) stato[k] = +s[k]; });
    if (s.car && typeof s.car === 'object') CAR.forEach(function (k) { if (typeof s.car[k] === 'number') stato.car[k] = s.car[k]; });
    disegna();
  }

  document.addEventListener('input', function (e) {
    var t = e.target;
    if (t.dataset.campo && t.dataset.campo !== 'nome') stato[t.dataset.campo] = t.type === 'number' ? (+t.value || 0) : t.value;
    else if (t.dataset.car) stato.car[t.dataset.car] = +t.value || 10;
    else if (t.id === 'hpCur' || t.id === 'hpTmp') stato[t.id] = Math.max(0, +t.value || 0);
    else return;
    if (t.dataset.car) disegna();
    modificato();
  });
  document.addEventListener('click', function (e) {
    var b = e.target.closest('[data-tiro]');
    if (!b) return;
    var k = b.dataset.tiro;
    // Il tiro lo fa Gaster Bless (lo vedono giocatore e Master); la risposta arriva con scheda:tiroRisultato.
    invia({ type: 'scheda:tiro', mod: mod(stato.car[k]), etichetta: 'Prova di ' + k });
  });

  // Messaggi da Gaster Bless: si accettano solo dalla pagina che contiene la scheda.
  window.addEventListener('message', function (e) {
    if (e.source !== window.parent) return;
    var m = e.data || {};
    switch (m.type) {
      case 'scheda:init':
        ruolo = m.role === 'master' ? 'master' : 'player';
        document.body.classList.toggle('scuro', m.theme === 'dark');
        carica(m.state);
        $('stato').textContent = 'Collegata a Gaster Bless' + (ruolo === 'master' ? ' (Master)' : '') + ' · i dati si salvano da soli';
        break;
      case 'scheda:load': carica(m.state); break;
      case 'scheda:setHp': // PF dal tracker: arrivano come testo ("15"; "" = non indicato) o come numero
        if (m.cur !== undefined && m.cur !== '' && isFinite(+m.cur)) stato.hpCur = +m.cur;
        if (m.temp !== undefined) stato.hpTmp = m.temp === '' ? 0 : (+m.temp || 0);
        disegna();
        break;
      case 'scheda:theme': document.body.classList.toggle('scuro', m.theme === 'dark'); break;
      case 'scheda:tiroRisultato': {
        var r = m.r || {};
        var d = r.dadi && r.dadi[0] && r.dadi[0].valori ? r.dadi[0].valori[0] : '?';
        $('ultimoTiro').textContent = '🎲 ' + (r.etichetta || 'Tiro') + ': d20 (' + d + ') ' + segno(r.mod || 0) + ' = ' + r.totale;
        break;
      }
    }
  });

  disegna();
  invia({ type: 'scheda:ready', version: '1.0' });
})();
