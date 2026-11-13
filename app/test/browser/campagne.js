// Campagne (passo L4): Campagna principale con i gruppi, nuova campagna con sistema, regolamento principale, documento
// supplementare, regole del Master, impostazioni, spostamento del gruppo, eliminazione, collegamento da My Rulebooks,
// persistenza dopo il riavvio del programma.
const path = require('path');
const { spawn, execSync } = require('child_process');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');
const chiama = (p, percorso, opz) => p.evaluate(([percorso, opz]) => import('/ui/ui.js').then((m) => m.api(percorso, opz)), [percorso, opz]);
(async () => {
  const b = await browser();
  const p = await master(b);
  await chiama(p, '/gruppi', { dati: { nome: 'Martedì' } });
  const reg = await chiama(p, '/regolamenti/miei?file=regole%20base.txt', { corpo: 'Il regolamento base del gioco di prova.' });
  const sup = await chiama(p, '/regolamenti/miei?file=viaggi.txt', { corpo: 'Regole aggiuntive per i viaggi.' });
  await p.goto(BASE + '/campagne', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.waitForSelector('.cp-campagna');
  const principale = p.locator('[data-campagna="principale"]');
  ok((await principale.locator('h3').textContent()) === 'Campagna principale' && await principale.getByText('👥 Martedì').count() === 1, 'Campagna principale con il gruppo esistente');

  await p.click('button:has-text("Nuova campagna")');
  await p.fill('.dialogo [name="nome"]', 'Le terre del nord');
  await p.fill('.dialogo [name="sistema"]', 'Gioco di prova');
  await p.fill('.dialogo [name="edizione"]', '2');
  await p.click('.dialogo button:has-text("Crea")');
  const nord = p.locator('.cp-campagna').filter({ has: p.locator('h3', { hasText: /^Le terre del nord$/ }) });
  await nord.waitFor({ timeout: 10000 });
  await p.waitForTimeout(300);
  ok(await nord.getByText('🎲 Gioco di prova · 2').count() === 1, 'nuova campagna con sistema ed edizione');

  await nord.locator('select[aria-label^="Regolamento principale"]').selectOption(reg.id);
  await p.waitForTimeout(800);
  await nord.locator('summary:has-text("Documenti supplementari")').click();
  await nord.locator('details:has(summary:has-text("Documenti supplementari")) .cp-documenti label', { hasText: 'viaggi' }).locator('input').check();
  await p.waitForTimeout(500);
  await nord.locator('summary:has-text("Regole aggiuntive")').click();
  await nord.locator('textarea').fill('Il riposo lungo richiede un luogo sicuro.');
  await nord.locator('button:has-text("Salva le regole")').click();
  await p.waitForTimeout(500);
  await nord.locator('summary:has-text("Impostazioni")').click();
  await nord.locator('button:has-text("＋ Voce")').click();
  await nord.locator('input[aria-label="Nome dell\'impostazione"]').last().fill('Dado di iniziativa');
  await nord.locator('input[aria-label="Valore dell\'impostazione"]').last().fill('d20');
  await nord.locator('button:has-text("Salva le impostazioni")').click();
  await p.waitForTimeout(500);
  await principale.locator('select[aria-label="Campagna del gruppo Martedì"]').selectOption({ label: 'Le terre del nord' });
  await p.waitForTimeout(1000);
  ok(await nord.getByText('👥 Martedì').count() === 1, 'gruppo spostato nella nuova campagna');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'campagne.png'), fullPage: true });

  // eliminazione della Campagna principale, ormai senza gruppi
  await p.locator('[data-campagna="principale"] button:has-text("Elimina")').click();
  await p.click('.dialogo button.pericolo:has-text("Elimina")');
  await p.waitForTimeout(800);
  ok(await p.locator('.cp-campagna').count() === 1, 'campagna senza gruppi eliminata');
  // l'ultima rimasta non si elimina
  await p.locator('.cp-campagna button:has-text("Elimina")').click();
  await p.click('.dialogo button.pericolo:has-text("Elimina")');
  await p.waitForTimeout(800);
  ok(await p.locator('.cp-campagna').count() === 1, 'la campagna con gruppi non si elimina');

  // My Rulebooks: campagne del documento e collegamento come scheda
  await p.goto(BASE + '/miei-regolamenti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  const docSup = p.locator('.mr-doc', { hasText: 'viaggi' });
  ok(await docSup.getByText('🏰 Le terre del nord').count() === 1, 'My Rulebooks mostra la campagna del documento');
  const docReg = p.locator('.mr-doc', { hasText: 'regole base' });
  ok(await docReg.getByText('🏰 Le terre del nord').count() === 1, 'anche il regolamento principale mostra la campagna');
  await docReg.locator('button:has-text("Campagne…")').click();
  await p.locator('.dialogo label', { hasText: 'scheda personalizzata' }).locator('input').check();
  await p.waitForTimeout(500);
  await p.click('.dialogo button:has-text("Fatto")');

  // riavvio del programma
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
  await p.goto(BASE + '/campagne', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.waitForSelector('.cp-campagna');
  const c = p.locator('.cp-campagna');
  ok(await c.locator('select[aria-label^="Regolamento principale"]').inputValue() === reg.id, 'dopo il riavvio: regolamento principale');
  ok((await c.locator('textarea').inputValue()) === 'Il riposo lungo richiede un luogo sicuro.', 'dopo il riavvio: regole del Master');
  ok(await c.getByText('Impostazioni della campagna (1)').count() === 1 && await c.getByText('Schede personalizzate (1)').count() === 1 &&
     await c.getByText('Documenti supplementari (1)').count() === 1 && await c.getByText('👥 Martedì').count() === 1, 'dopo il riavvio: impostazioni, documenti e gruppo');
  ok(p.erroriPagina.length === 0, 'nessun errore JavaScript: ' + p.erroriPagina.join(' | '));
  await b.close();
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
