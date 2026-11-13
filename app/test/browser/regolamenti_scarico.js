// RPG RULEBOOK LIBRARY, passo L2: download REALE dei documenti consentiti dal sito dell'editore, avanzamento, apertura
// del file, persistenza dopo il riavvio del programma. Serve la rete verso www.daggerheart.com e downloads.chaosium.com.
const path = require('path');
const { spawn, execSync } = require('child_process');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');
(async () => {
  const b = await browser();
  const p = await master(b);
  await p.goto(BASE + '/regolamenti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.waitForSelector('[data-doc="dh-srd10"] .rl-scarico button');
  ok(await p.locator('.rl-scarico').count() === 2, 'due documenti con download diretto nel catalogo');
  ok(await p.locator('[data-doc="coc7-qs"] .rl-scarico').count() === 0, 'documento solo con collegamento: nessun pulsante di download');

  for (const doc of ['dh-srd10', 'brp-qs']) {
    const box = p.locator(`[data-doc="${doc}"] .rl-scarico`);
    await box.locator('button').click();
    let vistoAvanzamento = false;
    for (let i = 0; i < 240; i++) {
      if (await box.locator('progress').count()) vistoAvanzamento = true;
      if (await box.getByText('Nella mia libreria').count()) break;
      if (await box.locator('.rl-errore').count()) break;
      await p.waitForTimeout(500);
    }
    const errore = await box.locator('.rl-errore').count() ? await box.locator('.rl-errore').textContent() : '';
    ok(await box.getByText('Nella mia libreria').count() === 1, doc + ': scaricato nella libreria ' + errore);
    ok(vistoAvanzamento, doc + ': avanzamento mostrato durante il download');
    ok(await box.locator('a').getAttribute('href') === '/miei-regolamenti', doc + ': collegamento a My Rulebooks');
    const cat = await (await p.request.get(BASE + '/api/v1/regolamenti/catalogo')).json();
    const r = await p.request.get(BASE + '/api/v1/regolamenti/miei/' + cat.nellaLibreria[doc] + '/file');
    const corpo = await r.body();
    ok(r.status() === 200 && r.headers()['content-type'] === 'application/pdf' && corpo.slice(0, 5).toString() === '%PDF-', doc + ': il file si apre ed è un PDF (' + corpo.length + ' byte)');
  }
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'regolamenti-scaricati.png') });

  // riavvio del programma sugli stessi dati
  execSync(`pkill -f "^${process.env.GB_BIN} " || true`);
  await new Promise((r) => setTimeout(r, 800));
  const srv = spawn(process.env.GB_BIN, ['-config', process.env.GB_CONF, '-prova', process.env.GB_DATI], { stdio: 'ignore', detached: true });
  srv.unref();
  await new Promise((r) => setTimeout(r, 2000));
  await p.goto(BASE + '/', { waitUntil: 'networkidle' });
  if (await p.locator('input[type=password]').count()) { // dopo il riavvio la sessione può essere da rifare
    await p.locator('input').first().fill('Master');
    await p.locator('input[type=password]').fill('PasswordProva123');
    await p.keyboard.press('Enter');
    await p.waitForTimeout(1500);
  }
  await p.goto(BASE + '/regolamenti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.waitForSelector('.rl-gioco');
  ok(await p.getByText('Nella mia libreria').count() === 2, 'dopo il riavvio i due documenti sono ancora nella libreria');
  ok(p.erroriPagina.length === 0, 'nessun errore JavaScript: ' + p.erroriPagina.join(' | '));
  await b.close();
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
