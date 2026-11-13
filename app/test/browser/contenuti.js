// Contenuti di gioco (fase 3c, C1): pagina vuota senza pacchetti, file sbagliato spiegato, anteprima e importazione del
// modello, contenuti nella scheda del personaggio (classe con privilegi, razza con sottorazze, background, incantesimi),
// disattivazione, telefono, persistenza dopo il riavvio, eliminazione.
const path = require('path');
const fs = require('fs');
const os = require('os');
const { spawn, execSync } = require('child_process');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');
const MODELLO = path.join(__dirname, '..', '..', 'web', 'modelli', 'modello_pacchetto.json');

async function opzioni(p, sel) { return p.locator(sel + ' option').allTextContents(); }

(async () => {
  const b = await browser();
  const p = await master(b);
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gb-contenuti-'));
  const sbagliato = path.join(tmp, 'sbagliato.json');
  fs.writeFileSync(sbagliato, JSON.stringify({ formato: 'gasterbless-contenuti', versioneFormato: 1, sistema: 'd20-5e', nome: 'Prova',
    classi: [{ id: 'x', nome: 'Errata', dadi: 8 }] }));

  // Scheda senza pacchetti: nessuna classe.
  await p.goto(BASE + '/scheda/', { waitUntil: 'networkidle' });
  ok(!(await opzioni(p, '#sCls')).some((t) => /Custode/.test(t)), 'scheda senza pacchetti: nessuna classe del modello');

  await p.goto(BASE + '/contenuti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.waitForSelector('.ct-vuoto');
  ok(await p.getByText('Nessun pacchetto: la scheda').count() === 1, 'pagina vuota con la spiegazione');
  const r = await p.request.get(BASE + '/modelli/modello_pacchetto.json');
  ok(r.ok() && (await r.json()).formato === 'gasterbless-contenuti', 'il modello si scarica');

  // File con un errore: spiegato con il punto preciso, niente importato.
  await p.setInputFiles('input[type=file]', sbagliato);
  await p.waitForSelector('.dialogo .ct-pre');
  const errore = await p.locator('.dialogo .ct-pre').textContent();
  ok(/classi n\. 1 \(«Errata»\): campo sconosciuto "dadi"/.test(errore), 'errore spiegato: ' + errore.split('\n').pop());
  await p.click('.dialogo button:has-text("Chiudi")');

  // Modello: anteprima e importazione.
  await p.setInputFiles('input[type=file]', MODELLO);
  await p.waitForSelector('.dialogo .ct-anteprima');
  const ant = await p.locator('.dialogo .ct-anteprima').textContent();
  ok(/1 classi · 1 razze · 1 background · 2 incantesimi · 1 armi · 1 armature · 1 talenti · 1 stili di combattimento · 1 strumenti · 1 categorie di equipaggiamento · 1 mostri · 1 oggetti magici · 1 professioni/.test(ant), 'anteprima con i conteggi');
  await p.click('.dialogo button:has-text("Importa")');
  const scheda = p.locator('.ct-pacchetto').filter({ hasText: 'Modello di pacchetto' });
  await scheda.waitFor({ timeout: 10000 });
  ok(await scheda.locator('input[type=checkbox]').isChecked(), 'pacchetto importato e attivo');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'contenuti.png'), fullPage: true });
  // lo stesso file di nuovo: doppione
  await p.setInputFiles('input[type=file]', MODELLO);
  await p.waitForSelector('.dialogo .ct-anteprima');
  ok(await p.locator('.dialogo').getByText('già stato importato').count() === 1 && await p.locator('.dialogo button:has-text("Importa")').count() === 0, 'doppione riconosciuto prima di importare');
  await p.click('.dialogo button:has-text("Annulla")');

  // Nella scheda del personaggio.
  await p.goto(BASE + '/scheda/', { waitUntil: 'networkidle' });
  ok((await opzioni(p, '#sCls')).some((t) => /Custode delle Lanterne/.test(t)), 'scheda: la classe del pacchetto');
  ok((await opzioni(p, '#sRace')).some((t) => /Gente del Muschio/.test(t)) && (await opzioni(p, '#sBg')).some((t) => /Cartografo/.test(t)), 'scheda: razza e background del pacchetto');
  await p.selectOption('#sCls', 'custode');
  await p.selectOption('#sRace', 'muschiati');
  await p.waitForTimeout(500);
  ok((await opzioni(p, '#sSr')).some((t) => /Muschiati della Valle/.test(t)), 'scheda: sottorazze');
  const testo = await p.locator('body').innerText();
  ok(/Luce Guida/.test(testo) && /Vigilanza/.test(testo), 'scheda: privilegi di 1° livello della classe');
  const inc = await p.evaluate(() => SP.filter((s) => s.cl.includes('l')).map((s) => s.it));
  ok(inc.includes('Scintilla del Custode') && inc.includes('Velo di Nebbia Chiara'), 'scheda: incantesimi nella lista del Custode');
  ok(await p.evaluate(() => CLS_CODE.custode === 'l' && CODE_N.l === 'Custode' && !!featInfo('Passo Leggero')), 'scheda: lista della classe e talento');
  ok(await p.evaluate(() => SUB_SPELLS['Ordine del Faro']['3'][0] === 'Bright Mist Veil' && RACE_SPELLS['muschiati/valle'].ab === 'SAG' &&
    STRUMENTI.some((s) => s.id === 'vetraio') && EQUIPAGGIAMENTO.some((c) => c.voci.some((v) => v[0] === 'Lanterna schermata')) &&
    EQUIPAGGIAMENTO.some((c) => c.id === 'armi' && c.voci[0][1] === '2 ma')), 'scheda: incantesimi concessi, strumenti, equipaggiamento con i prezzi');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'contenuti-scheda.png') });

  // Disattivato: la scheda non lo vede più.
  await p.goto(BASE + '/contenuti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.locator('.ct-pacchetto input[type=checkbox]').uncheck();
  await p.waitForSelector('.ct-pacchetto.ct-spento');
  await p.goto(BASE + '/scheda/', { waitUntil: 'networkidle' });
  ok(!(await opzioni(p, '#sCls')).some((t) => /Custode/.test(t)), 'pacchetto disattivato: sparisce dalla scheda');
  await p.goto(BASE + '/contenuti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.locator('.ct-pacchetto input[type=checkbox]').check();
  await p.waitForTimeout(600);

  // Telefono: niente scorrimento orizzontale.
  const tel = await b.newContext({ ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 }, storageState: await p.context().storageState() });
  const pt = await tel.newPage();
  await pt.goto(BASE + '/contenuti', { waitUntil: 'networkidle' });
  await chiudiFinestre(pt);
  await pt.waitForSelector('.ct-pacchetto');
  ok(await pt.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1), 'telefono: nessuno scorrimento orizzontale');
  await pt.screenshot({ path: path.join(__dirname, 'schermate', 'contenuti-telefono.png'), fullPage: true });
  await tel.close();

  // Riavvio del programma.
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
  await p.goto(BASE + '/scheda/', { waitUntil: 'networkidle' });
  ok((await opzioni(p, '#sCls')).some((t) => /Custode delle Lanterne/.test(t)), 'dopo il riavvio: la classe è ancora nella scheda');

  // Eliminazione.
  await p.goto(BASE + '/contenuti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.locator('.ct-pacchetto button:has-text("Elimina")').click();
  await p.click('.dialogo button.pericolo:has-text("Elimina")');
  await p.waitForSelector('.ct-vuoto');
  ok(await p.locator('.ct-pacchetto').count() === 0, 'pacchetto eliminato');
  ok(p.erroriPagina.length === 0, 'nessun errore JavaScript: ' + p.erroriPagina.join(' | '));
  fs.rmSync(tmp, { recursive: true, force: true });
  await b.close();
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
