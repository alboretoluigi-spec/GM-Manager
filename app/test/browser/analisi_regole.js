// Analisi dei regolamenti (passo L5): analisi dal pulsante di My Rulebooks, proposte da verificare, approvazione,
// rifiuto, modifica, regola scritta a mano, IA non disponibile, solo le approvate arrivano alla campagna, riavvio.
const path = require('path');
const { spawn, execSync } = require('child_process');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');
const chiama = (p, percorso, opz) => p.evaluate(([percorso, opz]) => import('/ui/ui.js').then((m) => m.api(percorso, opz)), [percorso, opz]);
const testo = 'Capitolo 1\n\nCreazione del personaggio\n\nScegli un concetto, poi distribuisci i punti tra le caratteristiche.\n\n' +
  'Iniziativa\n\nAll\'inizio dello scontro ognuno tira un dado e aggiunge la sua prontezza.\n\nConditions\n\nBlinded: the creature cannot see.\n\n' +
  'Armi e armature\n\nLe armi pesanti richiedono due mani.\n\nIncantesimi\n\nOgni incantesimo costa punti di energia.\n';
(async () => {
  const b = await browser();
  const p = await master(b);
  const doc = await chiama(p, '/regolamenti/miei?file=regole%20del%20gioco.txt', { corpo: testo });
  const camp = await chiama(p, '/campagne', { dati: { nome: 'Le terre del nord' } });
  await chiama(p, '/campagne/' + camp.id + '/documenti', { metodo: 'PUT', dati: { documento: doc.id, ruolo: 'supplementare', presente: true } });

  await p.goto(BASE + '/miei-regolamenti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.locator('.mr-doc', { hasText: 'regole del gioco' }).locator('button:has-text("Analizza")').click();
  await p.waitForTimeout(1000);
  ok(await p.getByText(/regole trovate da verificare/).count() > 0, 'analisi avviata da My Rulebooks');

  await p.goto(BASE + '/analisi-regole', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.waitForSelector('.an-proposta');
  ok(await p.locator('.rl-scheda.attiva').textContent() === 'Regole da approvare', 'terza scheda della libreria attiva');
  ok(await p.locator('.an-proposta').count() >= 5, 'proposte da verificare: ' + await p.locator('.an-proposta').count());
  ok(await p.locator('.an-proposta .an-stato', { hasText: 'Approvata' }).count() === 0, 'nessuna regola approvata da sola');
  const card = (nome) => p.locator('.an-proposta').filter({ has: p.locator('strong', { hasText: new RegExp('^' + nome + '$') }) });
  ok(await card('Iniziativa').locator('a').getAttribute('target') === '_blank', 'fonte collegata al documento');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'analisi-regole.png') });

  // IA non disponibile
  await p.click('button:has-text("Analizza un documento")');
  ok(await p.locator('.dialogo select[aria-label="Analizzatore"] option[disabled]').count() === 1, 'analizzatore con l\'IA predisposto ma non disponibile');
  await p.click('.dialogo button:has-text("Annulla")');

  // modifica, approvazione, rifiuto
  await card('Iniziativa').locator('button:has-text("Modifica")').click();
  await p.fill('.dialogo textarea[name="testo"]', 'Ognuno tira 1d10 + prontezza; si agisce dal più alto.');
  await p.click('.dialogo button:has-text("Salva")');
  await p.waitForTimeout(600);
  ok(await card('Iniziativa').getByText('Ognuno tira 1d10 + prontezza').count() === 1, 'regola corretta dal Master');
  await card('Iniziativa').locator('button:has-text("Approva")').click();
  await p.waitForTimeout(600);
  ok(await card('Iniziativa').count() === 0, 'approvata: esce dalle regole da verificare');
  await card('Conditions').locator('button:has-text("Rifiuta")').click();
  await p.waitForTimeout(600);
  // regola scritta a mano
  await p.click('button:has-text("Regola scritta da me")');
  await p.fill('.dialogo input[name="nome"]', 'Riposo in zona pericolosa');
  await p.selectOption('.dialogo select[name="categoria"]', 'speciali');
  await p.selectOption('.dialogo select[name="documento"]', doc.id);
  await p.fill('.dialogo textarea[name="testo"]', 'Nessun recupero completo dei punti ferita.');
  await p.click('.dialogo button:has-text("Salva")');
  await p.waitForTimeout(600);
  ok(await card('Riposo in zona pericolosa').count() === 1, 'regola scritta a mano, da verificare');
  await card('Riposo in zona pericolosa').locator('button:has-text("Approva")').click();
  await p.waitForTimeout(600);
  await p.selectOption('select[aria-label="Stato"]', '');
  await p.waitForTimeout(600);
  ok(await card('Iniziativa').locator('.an-stato').textContent() === 'Approvata' && await card('Conditions').locator('.an-stato').textContent() === 'Rifiutata', 'stati visibili con il filtro «Tutte»');

  let reg = await chiama(p, '/campagne/' + camp.id + '/regole');
  ok(reg.regole.length === 2, 'alla campagna arrivano solo le 2 regole approvate');

  // riavvio
  execSync(`pkill -f "^${process.env.GB_BIN} " || true`);
  await new Promise((r) => setTimeout(r, 800));
  spawn(process.env.GB_BIN, ['-config', process.env.GB_CONF, '-prova', process.env.GB_DATI], { stdio: 'ignore', detached: true }).unref();
  await new Promise((r) => setTimeout(r, 2000));
  await p.goto(BASE + '/', { waitUntil: 'networkidle' });
  if (await p.locator('input[type=password]').count()) {
    await p.locator('input').first().fill('Master');
    await p.locator('input[type=password]').fill('PasswordProva123');
    await p.keyboard.press('Enter');
    await p.waitForTimeout(1500);
  }
  await p.goto(BASE + '/analisi-regole', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  reg = await chiama(p, '/campagne/' + camp.id + '/regole');
  ok(reg.regole.length === 2, 'dopo il riavvio le regole approvate ci sono ancora');
  ok(p.erroriPagina.length === 0, 'nessun errore JavaScript: ' + p.erroriPagina.join(' | '));
  await b.close();
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
