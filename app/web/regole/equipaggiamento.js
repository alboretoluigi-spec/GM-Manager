/* Equipaggiamento acquistabile: nomi e prezzi (vuoto: lo riempie il Master con l'importazione o a mano).
   Usato dalla scheda: clic su «EQUIPAGGIAMENTO» → elenco completo → la scelta si aggiunge a ciò che il personaggio
   possiede (richiesta dell'utente, 2026-11-06). Armi, armature e strumenti riusano i dati di dati.js e strumenti.js
   (WEAPONS, ARMORS, STRUMENTI): qui ci sono solo i prezzi e gli oggetti che lì mancano.
   Voce: [nome, costo, nota]. Monete: mo = oro, ma = argento, mr = rame. */
(function () {
  // prezzi dai pacchetti di contenuti (PREZZI in dati.js)
  const PREZZI_ARMI = typeof PREZZI !== 'undefined' ? PREZZI : {};
  const PREZZI_ARMATURE = PREZZI_ARMI;
  const W = typeof WEAPONS !== 'undefined' ? WEAPONS : [];
  const A = typeof ARMORS !== 'undefined' ? ARMORS : [];
  const S = typeof STRUMENTI !== 'undefined' ? STRUMENTI : [];
  const CAT_ARMA = { s: 'semplice', g: 'da guerra' }, TIPO_ARMA = { m: 'mischia', d: 'distanza' };
  const TIPO_ARM = { l: 'leggera', m: 'media', p: 'pesante' };

  const armi = W.map((w) => [w[0], PREZZI_ARMI[w[0]] || '—', `${CAT_ARMA[w[1]]} da ${TIPO_ARMA[w[2]]} · ${w[3]} ${w[4]}${w[5] && w[5] !== '—' ? ' · ' + w[5] : ''}`]);
  const armature = A.map((a) => [a[0], PREZZI_ARMATURE[a[0]] || '—', `${TIPO_ARM[a[1]]} · CA ${a[2]}${a[1] === 'l' ? ' + Des' : a[1] === 'm' ? ' + Des (max 2)' : ''}${a[3] ? ' · For ' + a[3] : ''}${a[4] ? ' · svantaggio a Furtività' : ''}`]);
  const strumenti = S.map((t) => [t.n, t.costo || '—', { artigiano: 'strumenti da artigiano', musica: 'strumento musicale', gioco: 'set da gioco', kit: 'kit', veicolo: 'veicolo' }[t.cat] || '']);

  // armi, armature e strumenti dei pacchetti, poi le altre categorie dei pacchetti (EQUIP_CAT in dati.js)
  const CATALOGO = [];
  if (armi.length) CATALOGO.push({ id: 'armi', n: 'Armi', voci: armi });
  if (armature.length) CATALOGO.push({ id: 'armature', n: 'Armature', voci: armature });
  if (strumenti.length) CATALOGO.push({ id: 'strumenti', n: 'Strumenti', voci: strumenti });
  (typeof EQUIP_CAT !== 'undefined' ? EQUIP_CAT : []).forEach((c) => {
    const g = CATALOGO.find((x) => x.id === c.id);
    if (g) g.voci = g.voci.concat(c.voci); else CATALOGO.push({ id: c.id, n: c.n, voci: c.voci.slice() });
  });
  if (typeof window !== 'undefined') window.EQUIPAGGIAMENTO = CATALOGO;
})();
