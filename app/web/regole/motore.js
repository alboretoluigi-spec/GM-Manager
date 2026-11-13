/* Motore delle regole quinta edizione (regole 2014: PHB, supplemento, supplemento) — Gaster Bless.
 *
 * Modulo unico per la scheda e, più avanti, per il tracker dell'iniziativa. Contiene i calcoli che
 * dipendono da classi e livelli, scritti per il MULTICLASSE. Si appoggia ai dati di regole/dati.js
 * (CLASSES, FULL_SLOTS, KNOWN, cantripsKnown...), che devono essere caricati prima.
 *
 * Forma della scheda (compatibile con le schede già salvate):
 *   st.cls, st.sub, st.lvl  → classe PRINCIPALE (quella del 1° livello) e i suoi livelli
 *   st.mc = [{cls, sub, lvl, hp:{2:{m:'man',v:5}}}]  → classi aggiunte con il multiclasse
 *   (gli hp della classe principale restano in st.hp, per livello di classe)
 *
 * Le sintesi delle regole sono scritte con parole nostre, mai con il testo dei manuali.
 * Script "classico": definisce la variabile globale Regole5e (usabile da pagine normali e da moduli).
 */
(function (globale) {
  "use strict";

  // ----- Multiclasse: requisiti e competenze ottenute entrando in una nuova classe (PHB cap. 6) -----
  // Requisiti: punteggio minimo 13 nelle caratteristiche indicate. "o" = basta una delle due.
  var REQUISITI = {
    artefice: { tutte: ["INT"] }, barbaro: { tutte: ["FOR"] }, bardo: { tutte: ["CAR"] },
    chierico: { tutte: ["SAG"] }, druido: { tutte: ["SAG"] }, guerriero: { una: ["FOR", "DES"] },
    ladro: { tutte: ["DES"] }, mago: { tutte: ["INT"] }, monaco: { tutte: ["DES", "SAG"] },
    paladino: { tutte: ["FOR", "CAR"] }, ranger: { tutte: ["DES", "SAG"] }, stregone: { tutte: ["CAR"] },
    warlock: { tutte: ["CAR"] },
  };
  // Competenze ridotte. armorP: l/m/p/sc; weapP: s (semplici) g (da guerra); weapN: armi singole.
  var COMPETENZE_MC = {
    artefice: { armorP: ["l", "m", "sc"], weapP: [], weapN: [], skP: 0, testo: "armature leggere e medie, scudi, arnesi da scasso, strumenti da inventore" },
    barbaro: { armorP: ["sc"], weapP: ["s", "g"], weapN: [], skP: 0, testo: "scudi, armi semplici e da guerra" },
    bardo: { armorP: ["l"], weapP: [], weapN: [], skP: 1, testo: "armature leggere, un'abilità a scelta, uno strumento musicale" },
    chierico: { armorP: ["l", "m", "sc"], weapP: [], weapN: [], skP: 0, testo: "armature leggere e medie, scudi" },
    druido: { armorP: ["l", "m", "sc"], weapP: [], weapN: [], skP: 0, testo: "armature leggere e medie, scudi (non di metallo)" },
    guerriero: { armorP: ["l", "m", "sc"], weapP: ["s", "g"], weapN: [], skP: 0, testo: "armature leggere e medie, scudi, armi semplici e da guerra" },
    ladro: { armorP: ["l"], weapP: [], weapN: [], skP: 1, testo: "armature leggere, un'abilità della lista del ladro, arnesi da scasso" },
    mago: { armorP: [], weapP: [], weapN: [], skP: 0, testo: "nessuna" },
    monaco: { armorP: [], weapP: ["s"], weapN: ["Spada corta"], skP: 0, testo: "armi semplici, spade corte" },
    paladino: { armorP: ["l", "m", "sc"], weapP: ["s", "g"], weapN: [], skP: 0, testo: "armature leggere e medie, scudi, armi semplici e da guerra" },
    ranger: { armorP: ["l", "m", "sc"], weapP: ["s", "g"], weapN: [], skP: 1, testo: "armature leggere e medie, scudi, armi semplici e da guerra, un'abilità della lista del ranger" },
    stregone: { armorP: [], weapP: [], weapN: [], skP: 0, testo: "nessuna" },
    warlock: { armorP: ["l"], weapP: ["s"], weapN: [], skP: 0, testo: "armature leggere, armi semplici" },
  };

  // ----- Classi del personaggio -----

  function sottoclasse(c, nome) {
    if (!c || !nome) return null;
    var s = c.subs.find(function (x) { return x[0] === nome; });
    return s ? { n: s[0], src: s[1], f: s[2], x: s[3] || {} } : null;
  }

  /** Elenco delle classi: la principale per prima, poi quelle aggiunte (solo quelle valide). */
  function classi(st) {
    var out = [];
    var c = CLASSES[st.cls];
    if (c) out.push(voce(st.cls, c, st.sub, +st.lvl || 1, true, -1, st.hp || {}));
    (Array.isArray(st.mc) ? st.mc : []).forEach(function (m, i) {
      var d = m && CLASSES[m.cls];
      if (!d || m.cls === st.cls || out.some(function (o) { return o.id === m.cls; })) return;
      var l = Math.max(1, Math.min(20, +m.lvl || 1));
      out.push(voce(m.cls, d, m.sub, l, false, i, m.hp || {}));
    });
    return out;
  }
  function voce(id, c, sub, lvl, primaria, idx, hp) {
    var s = sottoclasse(c, sub);
    var on = !!(s && lvl >= c.subL);
    return { id: id, c: c, lvl: lvl, sub: s, subOn: on, x: on ? s.x : {}, primaria: primaria, idx: idx, hp: hp };
  }

  function livelloTotale(st) {
    var t = classi(st).reduce(function (a, k) { return a + k.lvl; }, 0);
    return Math.max(1, Math.min(20, t || +st.lvl || 1));
  }
  /** Livelli nella classe indicata (0 se il personaggio non ha quella classe). */
  function livelloClasse(st, id) {
    var k = classi(st).find(function (x) { return x.id === id; });
    return k ? k.lvl : 0;
  }
  /** Bonus di competenza: dipende dal livello del PERSONAGGIO, non della classe. */
  function competenza(st) { return Math.ceil(livelloTotale(st) / 4) + 1; }

  /** Classe e livello in forma leggibile: "Chierico 3 / Mago 2". */
  function descrizioneClassi(st) {
    return classi(st).map(function (k) { return k.c.n + " " + k.lvl; }).join(" / ");
  }

  // ----- Requisiti del multiclasse -----

  /** Avvisi sui requisiti (T = punteggi totali delle caratteristiche). Vuoto se tutto in regola. */
  function avvisiMulticlasse(st, T) {
    var k = classi(st);
    if (k.length < 2) return [];
    var out = [];
    k.forEach(function (x) {
      var r = REQUISITI[x.id];
      if (!r) return;
      var ok = r.una ? r.una.some(function (a) { return (T[a] || 0) >= 13; }) : r.tutte.every(function (a) { return (T[a] || 0) >= 13; });
      if (!ok) out.push(x.c.n + ": servono almeno 13 in " + (r.una ? r.una.join(" o ") : r.tutte.join(" e ")) + " per il multiclasse.");
    });
    if (livelloTotale(st) < k.reduce(function (a, x) { return a + x.lvl; }, 0)) out.push("Il livello totale non può superare 20.");
    return out;
  }

  /** Competenze di armature e armi date dalle classi aggiunte (ridotte) e dalle sottoclassi. */
  function competenzeClassi(st) {
    var armorP = [], weapP = [], weapN = [];
    classi(st).forEach(function (k) {
      var b = k.primaria ? { armorP: k.c.armorP, weapP: k.c.weapP, weapN: k.c.weapN } : COMPETENZE_MC[k.id] || {};
      armorP = armorP.concat(b.armorP || [], k.x.armorP || []);
      weapP = weapP.concat(b.weapP || [], k.x.weapP || []);
      weapN = weapN.concat(b.weapN || [], k.x.weapN || []);
    });
    return { armorP: armorP, weapP: weapP, weapN: weapN };
  }

  // ----- Punti ferita -----

  /** PF dai dadi vita: massimo al 1° livello della classe principale, poi media o tiro per ogni livello. */
  function puntiFerita(st, modCos) {
    var tot = 0, man = 0, med = 0, primo = true, dadi = [];
    classi(st).forEach(function (k) {
      var hd = k.c.hd, media = hd / 2 + 1;
      dadi.push(k.lvl + "d" + hd);
      for (var l = 1; l <= k.lvl; l++) {
        var v;
        if (primo && k.primaria && l === 1) { v = hd; primo = false; }
        else {
          var h = k.hp[l];
          if (h && h.m === "man" && h.v >= 1 && h.v <= hd) { v = +h.v; man++; } else { v = media; med++; }
        }
        tot += Math.max(1, v + modCos);
      }
    });
    var metodo = man && med ? "misto" : man ? "manuale" : livelloTotale(st) === 1 ? "massimo al 1°" : "media";
    return { max: tot, dadi: dadi.join(" + "), metodo: metodo };
  }

  // ----- Incantesimi -----

  // Tipo di incantatore di una classe (o della sua sottoclasse: Cavaliere Mistico, Mistificatore Arcano).
  function tipoIncantatore(k) {
    if (k.c.cast) return { tipo: k.c.cast, ab: k.c.sab, prep: k.c.prep };
    if (k.subOn && k.x.cast) return { tipo: k.x.cast, ab: k.x.sab, prep: null };
    return null;
  }

  // Livello da incantatore per la tabella del multiclasse.
  function livelloIncantatoreMC(tipo, L) {
    if (tipo === "full") return L;
    if (tipo === "half") return Math.floor(L / 2);
    if (tipo === "art") return Math.ceil(L / 2);
    if (tipo === "third") return Math.floor(L / 3);
    return 0;
  }
  // Livello da incantatore di una classe SINGOLA (arrotondamenti diversi, come nella scheda originale).
  function livelloIncantatoreSingolo(tipo, L) {
    if (tipo === "full") return L;
    if (tipo === "half") return L >= 2 ? Math.ceil(L / 2) : 0;
    if (tipo === "art") return Math.ceil(L / 2);
    if (tipo === "third") return L >= 3 ? Math.ceil(L / 3) : 0;
    return 0;
  }

  /**
   * Incantesimi del personaggio.
   * M = modificatori delle caratteristiche, p = bonus di competenza.
   * Restituisce le classi incantatrici (ognuna con CD, attacco, conosciuti/preparati, trucchetti),
   * gli slot condivisi e la Magia del Patto (separata).
   */
  function incantatore(st, M, p) {
    var per = [];
    classi(st).forEach(function (k) {
      var t = tipoIncantatore(k);
      if (!t) return;
      var L = k.lvl, m = M[t.ab] || 0, known = null, kLabel = "Incantesimi conosciuti";
      if (t.prep === "full") { known = Math.max(1, m + L); kLabel = "Incantesimi preparati"; }
      else if (t.prep === "half") { known = L < 2 && k.id !== "artefice" ? 0 : Math.max(1, m + Math.floor(L / 2)); kLabel = "Incantesimi preparati"; }
      else if (t.tipo === "third") known = KNOWN.third[L - 1];
      else if (KNOWN[k.id]) known = KNOWN[k.id][L - 1];
      var attivo = t.tipo === "pact" || t.tipo === "full" || t.tipo === "art" || livelloIncantatoreSingolo(t.tipo, L) > 0;
      per.push({ id: k.id, nome: k.c.n, sub: k.sub ? k.sub.n : "", tipo: t.tipo, ab: t.ab, m: m, cd: 8 + p + m, atk: p + m,
        known: known, kLabel: kLabel, trucchetti: cantripsKnown(k.id, L, k.subOn ? k.sub : null), attivo: attivo, lvl: L,
        codice: k.c.cast ? CODICI[k.id] : t.tipo === "third" ? "m" : null });
    });
    var slotClassi = per.filter(function (x) { return x.tipo !== "pact" && x.attivo; });
    var livello = 0;
    if (slotClassi.length === 1) livello = livelloIncantatoreSingolo(slotClassi[0].tipo, slotClassi[0].lvl);
    else slotClassi.forEach(function (x) { livello += livelloIncantatoreMC(x.tipo, x.lvl); });
    livello = Math.min(20, livello);
    var w = per.find(function (x) { return x.tipo === "pact"; });
    var patto = w ? { n: w.lvl === 1 ? 1 : w.lvl <= 10 ? 2 : w.lvl <= 16 ? 3 : 4, lv: Math.min(5, Math.ceil(w.lvl / 2)),
      arcanum: [11, 13, 15, 17].filter(function (l) { return w.lvl >= l; }).map(function (l, i) { return 6 + i; }) } : null;
    return { classi: per, slots: (FULL_SLOTS[livello] || []).slice(), livello: livello, patto: patto,
      multiclasse: slotClassi.length > 1 };
  }
  var CODICI = { artefice: "a", bardo: "b", chierico: "c", druido: "d", paladino: "p", ranger: "r", stregone: "s", warlock: "w", mago: "m" };

  /** Livello massimo di incantesimo lanciabile (slot condivisi, patto e arcanum mistico). */
  function livelloMassimo(inc) {
    var m = 0;
    inc.slots.forEach(function (n, i) { if (n) m = i + 1; });
    if (inc.patto) m = Math.max(m, inc.patto.lv, Math.max.apply(null, [0].concat(inc.patto.arcanum)));
    return m;
  }

  // ----- Aumenti di caratteristica -----

  /**
   * Livelli in cui si ottiene un aumento di caratteristica o un talento, per ogni classe (dipendono dal
   * livello DI CLASSE). Chiave: il numero del livello per la classe principale (compatibile con le
   * schede salvate), "classe:livello" per le classi aggiunte.
   */
  function aumenti(st) {
    var out = [];
    classi(st).forEach(function (k) {
      (k.c.asi || ASI_STD).filter(function (l) { return l <= k.lvl; }).forEach(function (l) {
        out.push({ chiave: k.primaria ? String(l) : k.id + ":" + l, classe: k.c.n, livello: l, primaria: k.primaria });
      });
    });
    return out;
  }

  globale.Regole5e = {
    REQUISITI: REQUISITI, COMPETENZE_MC: COMPETENZE_MC,
    classi: classi, sottoclasse: sottoclasse, livelloTotale: livelloTotale, livelloClasse: livelloClasse,
    competenza: competenza, descrizioneClassi: descrizioneClassi, avvisiMulticlasse: avvisiMulticlasse,
    competenzeClassi: competenzeClassi, puntiFerita: puntiFerita, incantatore: incantatore,
    livelloMassimo: livelloMassimo, aumenti: aumenti, livelloIncantatoreMC: livelloIncantatoreMC,
  };
})(typeof globalThis !== "undefined" ? globalThis : this);
