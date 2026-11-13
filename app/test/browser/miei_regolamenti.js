// MY RULEBOOKS, passo L3: importazione (PDF con testo letto nel browser, TXT, RTF, DOCX), file danneggiati, doppioni e
// formati non supportati, categorie, filtri, ricerca nel contenuto, lettura, versioni, sostituzione, eliminazione,
// persistenza dopo il riavvio del programma.
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn, execSync } = require('child_process');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');

const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gb-miei-'));
const file = (n, c) => { const p = path.join(dir, n); fs.writeFileSync(p, c); return p; };
// DOCX minimi creati con Python (zip con word/document.xml)
const docx = (n, testo) => {
  const p = path.join(dir, n);
  execSync(`python3 -c "import zipfile,sys; z=zipfile.ZipFile(sys.argv[1],'w'); z.writestr('[Content_Types].xml','<Types/>'); z.writestr('word/document.xml','<w:document xmlns:w=\\"http://schemas.openxmlformats.org/wordprocessingml/2006/main\\"><w:body><w:p><w:r><w:t>'+sys.argv[2]+'</w:t></w:r></w:p></w:body></w:document>'); z.close()" "${p}" "${testo}"`);
  return p;
};

(async () => {
  const b = await browser();
  const p = await master(b);
  // un PDF vero, con testo, creato dal browser
  const pg = await b.newPage();
  await pg.setContent('<h1>Manuale di prova</h1><p>La magia rituale richiede dieci minuti in più.</p><div style="page-break-before:always"><p>Seconda pagina: il combattimento in mischia.</p></div>');
  const pdf = path.join(dir, 'manuale.pdf');
  await pg.pdf({ path: pdf, format: 'A4' });
  await pg.close();
  const txt = file('regole della casa.txt', 'Capitolo 1\n\nIl riposo breve dura un\'ora intera.\n');
  const rtf = file('antico.rtf', '{\\rtf1\\ansi{\\fonttbl{\\f0 Arial;}}\\f0 Perch\\\'e8 il drago dorme?\\par Fine.}');
  const dx = docx('armi.docx', 'La spada lunga fa danni taglienti.');
  const dx2 = docx('armi v2.docx', 'La spada bastarda fa danni taglienti superiori.');
  const rotto = file('rotto.pdf', '%PDF-1.7 manca la fine');
  const exe = file('programma.exe', 'MZ');

  await p.goto(BASE + '/miei-regolamenti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  ok(await p.getByText('La tua libreria è vuota.').count() === 1, 'libreria vuota all\'inizio');
  ok(await p.locator('.rl-scheda.attiva').textContent() === 'My Rulebooks', 'scheda «My Rulebooks» attiva');

  // categoria
  await p.click('button:has-text("Categorie")');
  await p.fill('input[aria-label="Nome della nuova categoria"]', 'Regole base');
  await p.click('.dialogo button:has-text("Aggiungi")');
  await p.waitForTimeout(500);
  ok(await p.locator('.mr-categorie').getByText('Regole base (0)').count() === 1, 'categoria creata');
  await p.keyboard.press('Escape');

  const importa = async (percorso, campi = {}) => {
    await p.click('button:has-text("Importa un documento")');
    await p.setInputFiles('input[aria-label="File da importare"]', percorso);
    for (const [k, v] of Object.entries(campi)) {
      const c = p.locator(`.dialogo [name="${k}"]`);
      if ((await c.evaluate((e) => e.tagName)) === 'SELECT') await c.selectOption({ label: v }); else await c.fill(v);
    }
    await p.click('.dialogo button:has-text("Importa")');
    await p.waitForTimeout(1500);
    const errore = await p.locator('.dialogo .rl-errore').count() ? (await p.locator('.dialogo .rl-errore').textContent()) : '';
    if (errore) await p.click('.dialogo button:has-text("Annulla")');
    for (let i = 0; i < 40 && await p.locator('.dialogo:has-text("Preparo la ricerca")').count(); i++) await p.waitForTimeout(250);
    await p.waitForTimeout(400);
    return errore;
  };
  ok(await importa(txt, { gioco: 'Gioco di prova', edizione: '1', categoria: 'Regole base' }) === '', 'importato il TXT con gioco, edizione e categoria');
  ok(await importa(pdf, { titolo: 'Manuale di prova', gioco: 'Gioco di prova', tipologia: 'Regolamento base' }) === '', 'importato il PDF');
  ok(await importa(rtf, { gioco: 'Altro gioco' }) === '', 'importato l\'RTF');
  ok(await importa(dx, { gioco: 'Altro gioco' }) === '', 'importato il DOCX');
  ok((await importa(rotto)).includes('danneggiato'), 'PDF danneggiato rifiutato con un messaggio chiaro');
  ok((await importa(exe)).includes('non supportato'), 'formato non supportato rifiutato');
  ok((await importa(txt)).includes('già nella libreria'), 'doppione riconosciuto');
  await p.waitForTimeout(500);
  ok(await p.locator('.mr-doc').count() === 4, 'quattro documenti nella libreria');
  const pdfDoc = p.locator('.mr-doc', { hasText: 'Manuale di prova' });
  ok(await pdfDoc.getByText('testo per la ricerca ✓').count() === 1, 'testo del PDF letto nel browser per la ricerca');
  ok(await p.locator('.mr-doc', { hasText: 'regole della casa' }).getByText('🏷 Regole base').count() === 1, 'categoria mostrata sul documento');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'miei-regolamenti.png') });

  // filtri e ricerca
  await p.selectOption('select[aria-label="Gioco"]', 'Altro gioco');
  await p.waitForTimeout(300);
  ok(await p.locator('.mr-doc').count() === 2, 'filtro per gioco');
  await p.selectOption('select[aria-label="Gioco"]', '');
  await p.fill('input[aria-label="Cerca nei miei documenti"]', 'magia rituale');
  await p.waitForTimeout(1000);
  ok(await p.locator('.mr-trovati li').count() === 1 && (await p.locator('.mr-trovati').textContent()).includes('Manuale di prova'), 'ricerca nel contenuto del PDF');
  await p.fill('input[aria-label="Cerca nei miei documenti"]', 'perche drago');
  await p.waitForTimeout(1000);
  ok(await p.locator('.mr-trovati li').count() === 1, 'ricerca nell\'RTF senza accenti');
  await p.click('.mr-trovati button');
  await p.waitForTimeout(600);
  ok(await p.locator('.dialogo .mr-lettore').getByText('Perchè il drago dorme?').count() === 1, 'il risultato apre il documento nel punto trovato');
  await p.keyboard.press('Escape');
  await p.fill('input[aria-label="Cerca nei miei documenti"]', '');
  await p.waitForTimeout(600);

  // versioni: il PDF diventa la 2ª stampa delle regole della casa
  await pdfDoc.locator('button:has-text("Modifica")').click();
  await p.selectOption('.dialogo [name="versioneDi"]', { label: 'regole della casa (1)' });
  await p.fill('.dialogo [name="versione"]', '2ª stampa');
  await p.click('.dialogo button:has-text("Salva")');
  await p.waitForTimeout(800);
  ok(await p.locator('.mr-doc', { hasText: 'regole della casa' }).locator('.mr-versioni').getByText('Manuale di prova').count() === 1, 'versione mostrata sotto il documento principale');

  // lettura di un documento di testo
  await p.locator('.mr-doc', { hasText: 'regole della casa' }).first().locator('button:has-text("Apri")').first().click();
  await p.waitForTimeout(600);
  ok(await p.locator('.dialogo .mr-lettore').getByText(/riposo breve/).count() === 1, 'lettura del TXT nell\'app');
  await p.keyboard.press('Escape');
  // apertura del PDF in una nuova scheda
  const versionePdf = p.locator('.mr-versioni .mr-doc').first();
  const [nuova, rich] = await Promise.all([p.context().waitForEvent('page'),
    p.context().waitForEvent('response', (r) => r.url().includes('/api/v1/regolamenti/miei/') && r.url().endsWith('/file')),
    versionePdf.locator('.mr-azioni button:has-text("Apri")').click()]);
  ok(rich.status() === 200 && rich.headers()['content-type'] === 'application/pdf', 'il PDF si apre in una nuova scheda (risposta ' + rich.status() + ')');
  await nuova.close().catch(() => {});

  // sostituzione del DOCX
  const docxDoc = p.locator('.mr-doc', { hasText: 'armi' });
  const [scelta] = await Promise.all([p.waitForEvent('filechooser'), docxDoc.locator('button:has-text("Sostituisci il file")').click()]);
  await scelta.setFiles(dx2);
  await p.waitForTimeout(1200);
  await p.fill('input[aria-label="Cerca nei miei documenti"]', 'bastarda');
  await p.waitForTimeout(1000);
  ok(await p.locator('.mr-trovati li').count() === 1, 'dopo la sostituzione la ricerca trova il testo nuovo');
  await p.fill('input[aria-label="Cerca nei miei documenti"]', '');

  // eliminazione
  await p.locator('.mr-doc', { hasText: 'antico' }).locator('button:has-text("Elimina")').click();
  await p.click('.dialogo button.pericolo:has-text("Elimina")');
  await p.waitForTimeout(800);
  ok(await p.locator('.mr-doc').count() === 3, 'documento eliminato');

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
  await p.goto(BASE + '/miei-regolamenti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.waitForSelector('.mr-doc');
  ok(await p.locator('.mr-doc').count() === 3 && await p.locator('.mr-versioni').count() === 1, 'dopo il riavvio: documenti, categorie e versioni intatti');
  ok(p.erroriPagina.length === 0, 'nessun errore JavaScript: ' + p.erroriPagina.join(' | '));
  await b.close();
  fs.rmSync(dir, { recursive: true, force: true });
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
