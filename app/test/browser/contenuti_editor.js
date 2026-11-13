// Inserimento a mano dei contenuti (fase 3c, C2): pacchetto personale, nuova classe con privilegi e sottoclasse,
// incantesimo con un errore spiegato e poi corretto, razza con tratti e sottorazza, copia ed eliminazione di una voce,
// dati del pacchetto, modifica di un pacchetto importato senza perdere i campi che il modulo non mostra, scheda del
// personaggio aggiornata, telefono.
const path = require('path');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');
const MODELLO = path.join(__dirname, '..', '..', 'web', 'modelli', 'modello_pacchetto.json');
const chiama = (p, percorso, opz) => p.evaluate(([percorso, opz]) => import('/ui/ui.js').then((m) => m.api(percorso, opz)), [percorso, opz]);
const file = (p, id) => p.evaluate((id) => fetch('/api/v1/contenuti/' + id + '/file').then((r) => r.json()), id);
const ultimo = (p) => p.locator('.dialogo').last();

async function campo(d, etichetta) { return d.locator('.ce-campo', { has: d.page().locator(`label:text-is("${etichetta}")`) }).first().locator('input,select,textarea').first(); }
async function salva(p) {
  const n = await p.locator('.dialogo').count();
  await ultimo(p).locator('button:has-text("Salva")').click();
  // finito quando il modulo si chiude o mostra l'errore
  await p.waitForFunction((n) => document.querySelectorAll('.dialogo').length < n || [...document.querySelectorAll('.ce-errore')].some((e) => !e.hidden), n, { timeout: 10000 });
  await p.waitForTimeout(300);
}

(async () => {
  const b = await browser();
  const p = await master(b);
  await p.goto(BASE + '/contenuti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.waitForSelector('.ct-vuoto');

  // Pacchetto personale: si crea e si apre l'editor.
  await p.click('button:has-text("Nuovo pacchetto personale")');
  await p.fill('.dialogo input[name=nome]', 'Le mie aggiunte');
  await p.click('.dialogo button:has-text("Crea")');
  await p.waitForSelector('.dialogo .ce-editor');
  ok(await p.locator('.ct-pacchetto', { hasText: 'Le mie aggiunte' }).count() === 1, 'pacchetto personale creato');

  // Nuova classe.
  await p.click('.dialogo button:has-text("Nuova voce: classe")');
  let d = ultimo(p);
  await (await campo(d, 'Nome *')).fill('Guardiano del Ponte');
  await (await campo(d, 'Dado vita (facce) *')).fill('10');
  await d.locator('input[aria-label="Tiri salvezza con competenza: Forza"]').check();
  await d.locator('input[aria-label="Tiri salvezza con competenza: Costituzione"]').check();
  await (await campo(d, 'Abilità da scegliere')).fill('2');
  await d.locator('input[aria-label="Fra le abilità: Atletica"]').check();
  await d.locator('input[aria-label="Fra le abilità: Percezione"]').check();
  await d.locator('button:has-text("Aggiungi un privilegio")').first().click();
  await d.locator('input[aria-label="Nome del privilegio"]').first().fill('Passo Saldo');
  await d.locator('textarea[aria-label="Descrizione"]').first().fill('Non puoi essere spinto indietro contro la tua volontà.');
  await d.locator('button:has-text("Aggiungi sottoclasse")').click();
  const sc = d.locator('.ce-voce-sotto').last();
  await (await campo(sc, 'Nome *')).fill('Via del Fiume');
  await sc.locator('button:has-text("Aggiungi un privilegio")').click();
  await sc.locator('input[aria-label="Livello"]').fill('3');
  await sc.locator('input[aria-label="Nome del privilegio"]').fill('Corrente');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'contenuti-editor-classe.png') });
  await salva(p);
  ok(await p.locator('.dialogo .ce-voce', { hasText: 'Guardiano del Ponte' }).count() === 1, 'classe salvata e nell\'elenco');

  // Incantesimo con un errore spiegato, poi corretto.
  await p.click('.dialogo button:has-text("Incantesimi (0)")');
  await p.click('.dialogo button:has-text("Nuova voce: incantesimo")');
  d = ultimo(p);
  await (await campo(d, 'Nome *')).fill('Lampo di Brina');
  await (await campo(d, 'Livello (0 = trucchetto) *')).fill('12');
  await (await campo(d, 'Scuola')).selectOption('invocazione');
  await d.locator('input[aria-label="Liste di incantesimi: Mago"]').check();
  await salva(p);
  if (!(await p.locator('.dialogo .ce-errore:visible').count())) {
    console.log('DIAG dialoghi', await p.locator('.dialogo h2').allTextContents(), JSON.stringify((await file(p, (await chiama(p, '/contenuti')).pacchetti[0].id)).incantesimi));
    await p.screenshot({ path: path.join(__dirname, 'schermate', 'debug-errore.png') });
  }
  await p.locator('.dialogo .ce-errore:visible').waitFor({ timeout: 10000 });
  const err = ultimo(p).locator('.ce-errore');
  ok(await err.isVisible() && /«livello» deve essere tra 0 e 9/.test(await err.textContent()), 'errore spiegato nel modulo: ' + (await err.textContent()).split('\n').pop());
  await (await campo(ultimo(p), 'Livello (0 = trucchetto) *')).fill('1');
  await salva(p);
  ok(await p.locator('.dialogo .ce-voce', { hasText: 'Lampo di Brina' }).count() === 1, 'incantesimo corretto e salvato');

  // Razza con tratto e sottorazza.
  await p.click('.dialogo button:has-text("Razze (0)")');
  await p.click('.dialogo button:has-text("Nuova voce: razza")');
  d = ultimo(p);
  await (await campo(d, 'Nome *')).fill('Gente dei Pontili');
  await d.locator('input[aria-label="Bonus alle caratteristiche: Destrezza"]').first().fill('2');
  await d.locator('button:has-text("Aggiungi un tratto")').first().click();
  await d.locator('input[aria-label="Nome del tratto"]').first().fill('Piede Marino');
  await d.locator('button:has-text("Aggiungi sottorazza")').click();
  await (await campo(d.locator('.ce-voce-sotto').last(), 'Nome *')).fill('Pontili del Nord');
  await salva(p);
  ok(await p.locator('.dialogo .ce-voce', { hasText: 'Gente dei Pontili' }).count() === 1, 'razza salvata');

  // Copia ed eliminazione.
  await p.click('.dialogo button[aria-label="Copia Gente dei Pontili"]');
  await salva(p);
  ok(await p.locator('.dialogo .ce-voce', { hasText: 'Gente dei Pontili (copia)' }).count() === 1, 'voce copiata (con un identificativo nuovo)');
  await p.click('.dialogo button[aria-label="Elimina Gente dei Pontili (copia)"]');
  await p.click('.dialogo button.pericolo:has-text("Elimina")');
  await p.waitForTimeout(700);
  ok(await p.locator('.dialogo .ce-voce', { hasText: '(copia)' }).count() === 0, 'voce eliminata');

  // Dati del pacchetto.
  await p.click('.dialogo button:has-text("Dati del pacchetto")');
  await p.click('.dialogo button:has-text("Modifica i dati del pacchetto")');
  await (await campo(ultimo(p), 'Autore')).fill('Il Master');
  await salva(p);
  await p.keyboard.press('Escape');
  await p.waitForTimeout(600);
  const pers = p.locator('.ct-pacchetto', { hasText: 'Le mie aggiunte' });
  ok(/di Il Master/.test(await pers.textContent()) && /1 classi · 1 razze · 1 incantesimi/.test(await pers.textContent()), 'scheda del pacchetto aggiornata');
  const lista = await chiama(p, '/contenuti');
  const idPers = lista.pacchetti[0].id;
  const f1 = await file(p, idPers);
  const cl = f1.classi[0];
  ok(cl.id === 'guardiano-del-ponte' && cl.dadoVita === 10 && cl.tiriSalvezza.join() === 'FOR,COS' && cl.privilegi['1'][0] === 'Passo Saldo' &&
     cl.descrizioni['Passo Saldo'] && cl.sottoclassi[0].privilegi['3'][0] === 'Corrente', 'file del pacchetto nel formato aperto');

  // Pacchetto importato: modifica senza perdere i campi che il modulo non mostra.
  await p.setInputFiles('input[type=file]', MODELLO);
  await p.click('.dialogo button:has-text("Importa")');
  const mod = p.locator('.ct-pacchetto', { hasText: 'Modello di pacchetto' });
  await mod.waitFor();
  await mod.locator('button:has-text("Voci")').click();
  await p.waitForSelector('.dialogo .ce-editor');
  ok(await p.locator('.dialogo').getByText('copia del pacchetto importato').count() === 1, 'avviso sulla copia del pacchetto importato');
  await p.click('.dialogo button[aria-label="Modifica Custode delle Lanterne"]');
  await (await campo(ultimo(p), 'Dado vita (facce) *')).fill('10');
  await salva(p);
  await p.keyboard.press('Escape');
  const idMod = (await chiama(p, '/contenuti')).pacchetti.find((x) => !x.personale).id;
  const cu = (await file(p, idMod)).classi[0];
  ok(cu.dadoVita === 10 && cu.incantatore.lista === 'l' && cu.descrizioni['Luce Guida'] && cu.sottoclassi[0].abilitaFisse[0] === 'Percezione' &&
     cu.sottoclassi[0].descrizioni.length === 2 && cu.equipaggiamento.length === 3, 'campi conservati dopo la modifica');

  // Nella scheda del personaggio.
  await p.goto(BASE + '/scheda/', { waitUntil: 'networkidle' });
  const opz = await p.locator('#sCls option').allTextContents();
  ok(opz.some((t) => /Guardiano del Ponte/.test(t)) && opz.some((t) => /Custode delle Lanterne/.test(t)), 'scheda: classi dei due pacchetti');
  await p.selectOption('#sCls', 'guardiano-del-ponte');
  await p.waitForTimeout(400);
  ok(/Passo Saldo/.test(await p.locator('body').innerText()), 'scheda: privilegio inserito a mano');

  // Telefono: l'editor sta nello schermo.
  const tel = await b.newContext({ ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 }, storageState: await p.context().storageState() });
  const pt = await tel.newPage();
  await pt.goto(BASE + '/contenuti', { waitUntil: 'networkidle' });
  await chiudiFinestre(pt);
  await pt.locator('.ct-pacchetto', { hasText: 'Le mie aggiunte' }).locator('button:has-text("Voci")').click();
  await pt.click('.dialogo button[aria-label="Modifica Guardiano del Ponte"]');
  await pt.waitForTimeout(500);
  ok(await pt.evaluate(() => [...document.querySelectorAll('.dialogo')].every((x) => x.scrollWidth <= x.clientWidth + 1) && document.documentElement.scrollWidth <= innerWidth + 1), 'telefono: il modulo sta nello schermo');
  await pt.screenshot({ path: path.join(__dirname, 'schermate', 'contenuti-editor-telefono.png') });
  await tel.close();
  ok(p.erroriPagina.length === 0, 'nessun errore JavaScript: ' + p.erroriPagina.join(' | '));
  await b.close();
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
