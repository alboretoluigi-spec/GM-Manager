// Interfaccia adattiva (fase 3c, C4a): dopo l'accesso il Master sceglie la campagna; con la quinta edizione ci sono le
// sue sezioni e i riposi; con la scheda libera spariscono per il Master e per il giocatore (in tempo reale); una pagina
// nascosta aperta dall'indirizzo porta alle Sessioni; il gioco si sceglie anche nella pagina Campagne; nuovo accesso.
const path = require('path');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');
const chiama = (p, percorso, opz) => p.evaluate(([percorso, opz]) => import('/ui/ui.js').then((m) => m.api(percorso, opz)), [percorso, opz]);
const voci = (p) => p.locator('nav.menu a').allTextContents();

(async () => {
  const b = await browser();
  // Primo avvio: la scelta compare da sola.
  const c = await b.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1280, height: 900 } });
  const p = await c.newPage();
  p.erroriPagina = [];
  p.on('pageerror', (e) => p.erroriPagina.push(e.message));
  await p.goto(BASE + '/', { waitUntil: 'networkidle' });
  const campi = await p.locator('input').all();
  await campi[0].fill('Master'); await campi[1].fill('PasswordProva123'); await campi[2].fill('PasswordProva123');
  await p.getByRole('button', { name: /Crea l.account/ }).click();
  await p.waitForSelector('.dialogo:has-text("Che cosa giochiamo oggi?")', { timeout: 10000 });
  ok(await p.locator('.scelta-voce', { hasText: 'Campagna principale' }).count() === 1, 'dopo l\'accesso: «Che cosa giochiamo oggi?» con le campagne');
  await p.locator('.scelta-voce', { hasText: 'Campagna principale' }).click();
  await p.waitForTimeout(800);
  await chiudiFinestre(p);
  let v = await voci(p);
  ok(v.some((t) => /Contenuti di gioco/.test(t)) && v.some((t) => /Oggetti magici/.test(t)) && await p.locator('.tasto-riposo').count() === 2,
    'quinta edizione: sezioni e riposi della quinta edizione');
  ok(/Campagna principale/.test(await p.locator('.gioco-attivo').textContent()) && /Quinta edizione/.test(await p.locator('.gioco-attivo').textContent()), 'campagna e gioco sotto il logo');

  // Giocatore approvato in un gruppo.
  const gr = (await chiama(p, '/gruppi', { dati: { nome: 'Martedì' } })).id;
  const cg = await b.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1280, height: 900 } });
  const g = await cg.newPage();
  g.erroriPagina = [];
  g.on('pageerror', (e) => g.erroriPagina.push(e.message));
  await g.goto(BASE + '/registrazione', { waitUntil: 'networkidle' });
  await g.evaluate(() => fetch('/api/v1/registrazione', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ nome: 'Lia',
    telefono: '3330000001', password: 'Nebbia sul lago Varnhold', conferma: 'Nebbia sul lago Varnhold', consenso: true, avvisi: true }) }));
  const idLia = (await chiama(g, '/io')).utente.id;
  await chiama(p, '/utenti/' + idLia + '/approva', { dati: { gruppi: [gr] } });
  await g.goto(BASE + '/sessioni', { waitUntil: 'networkidle' });
  await chiudiFinestre(g);
  await g.waitForSelector('nav.menu');
  ok((await voci(g)).some((t) => /Sanità mentale/.test(t)) && await g.locator('.tasto-riposo').count() === 2, 'giocatore: quinta edizione con sanità mentale e riposi');

  // Il Master crea una campagna con la scheda libera e la sceglie: cambia tutto, anche per il giocatore.
  await chiama(p, '/campagne', { metodo: 'POST', dati: { nome: 'Notte senza stelle', modulo: 'libera' } });
  await p.click('.gioco-attivo button:has-text("Cambia campagna")');
  await p.locator('.scelta-voce', { hasText: 'Notte senza stelle' }).click();
  await p.waitForTimeout(1200);
  v = await voci(p);
  ok(!v.some((t) => /Contenuti di gioco|Oggetti magici|Strumenti e kit|Sanità mentale/.test(t)) && v.some((t) => /Mappa/.test(t)) && await p.locator('.tasto-riposo').count() === 0,
    'scheda libera: niente sezioni né riposi della quinta edizione');
  ok(/Notte senza stelle/.test(await p.locator('.gioco-attivo').textContent()) && /Scheda libera/.test(await p.locator('.gioco-attivo').textContent()), 'sotto il logo la nuova campagna');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'gioco-libera.png') });
  await g.waitForTimeout(800);
  ok(!(await voci(g)).some((t) => /Sanità mentale/.test(t)) && await g.locator('.tasto-riposo').count() === 0, 'giocatore aggiornato in tempo reale');
  ok(/«Notte senza stelle»/.test(await g.locator('body').innerText()), 'il giocatore riceve l\'avviso del gioco');

  // Pagina nascosta aperta dall'indirizzo.
  await p.goto(BASE + '/contenuti', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  ok(new URL(p.url()).pathname === '/sessioni', 'sezione nascosta: si torna alle Sessioni');

  // Pagina Campagne: il gioco della campagna si vede e si cambia.
  await p.goto(BASE + '/campagne', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  const notte = p.locator('.cp-campagna').filter({ has: p.locator('h3', { hasText: /^Notte senza stelle$/ }) });
  await notte.waitFor();
  ok(/Scheda libera/.test(await notte.locator('.cp-modulo').textContent()), 'pagina Campagne: gioco della campagna');
  await notte.locator('button:has-text("Modifica")').first().click();
  ok(await p.locator('.dialogo select[name=modulo] option').count() >= 10, 'pagina Campagne: elenco dei giochi');
  await p.selectOption('.dialogo select[name=modulo]', 'd100');
  await p.click('.dialogo button:has-text("Salva")');
  await p.waitForTimeout(1200);
  ok((await voci(p)).some((t) => /Sanità mentale/.test(t)) && /Percentuale d100/.test(await p.locator('.gioco-attivo').textContent()), 'gioco cambiato: con d100 torna la sanità mentale');

  // Nuovo accesso: la scelta compare di nuovo.
  await p.locator('nav.menu button:has-text("Esci")').click();
  await p.waitForTimeout(800);
  await p.getByText('Sono il Master').click();
  await p.waitForTimeout(300);
  await p.locator('input:visible').first().fill('Master');
  await p.locator('input[type=password]:visible').fill('PasswordProva123');
  await p.keyboard.press('Enter');
  
  await p.waitForSelector('.dialogo:has-text("Che cosa giochiamo oggi?")', { timeout: 10000 });
  ok(await p.locator('.scelta-voce.scelta-attuale', { hasText: 'Notte senza stelle' }).count() === 1, 'nuovo accesso: scelta con la campagna attuale evidenziata');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'gioco-scelta.png') });
  ok(p.erroriPagina.length === 0 && g.erroriPagina.length === 0, 'nessun errore JavaScript: ' + p.erroriPagina.concat(g.erroriPagina).join(' | '));
  await b.close();
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
