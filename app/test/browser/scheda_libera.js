// Scheda libera (fase 3c, C4b): campagna con la scheda libera; il Master cambia il modello (nuovo campo con un tiro)
// nella pagina Campagne; il giocatore apre la scheda del suo personaggio, la compila, tira i dadi (dal campo e dai
// pulsanti) e ritrova i valori dopo aver ricaricato; Finanze nascoste fuori dalla quinta edizione; telefono.
const path = require('path');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');
const chiama = (p, percorso, opz) => p.evaluate(([percorso, opz]) => import('/ui/ui.js').then((m) => m.api(percorso, opz)), [percorso, opz]);

(async () => {
  const b = await browser();
  const p = await master(b);
  const camp = (await chiama(p, '/campagne', { metodo: 'POST', dati: { nome: 'Notte senza stelle', modulo: 'libera' } })).id;
  await chiama(p, '/gioco', { metodo: 'PUT', dati: { campagna: camp } });
  const gr = (await chiama(p, '/gruppi', { dati: { nome: 'Giovedì' } })).id; // va nella campagna attiva

  // Modello: il Master aggiunge un campo «Astuzia» con un tiro.
  await p.goto(BASE + '/campagne', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  const notte = p.locator('.cp-campagna').filter({ has: p.locator('h3', { hasText: /^Notte senza stelle$/ }) });
  await notte.waitFor();
  await notte.locator('button:has-text("Modello della scheda")').click();
  await p.waitForSelector('.dialogo .ce-editor');
  const valori = p.locator('.dialogo .ce-voce-sotto').filter({ has: p.locator('summary', { hasText: /^Valori$/ }) }).first();
  await valori.locator('summary').first().click();
  await valori.locator('button:has-text("Aggiungi un campo")').click();
  const nuovo = valori.locator('.ce-voce-sotto').last();
  await nuovo.locator('input[name=nome]').fill('Astuzia');
  await nuovo.locator('select[name=tipo]').selectOption('numero');
  await nuovo.locator('input[name=predefinito]').fill('2');
  await nuovo.locator('input[name=tiro]').fill('2d6 + {astuzia}');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'scheda-libera-modello.png') });
  await p.click('.dialogo button:has-text("Salva il modello")');
  await p.waitForTimeout(1000);
  ok(await p.locator('.dialogo .ce-editor').count() === 0, 'modello salvato');
  const mod = await chiama(p, '/campagne/' + camp + '/modello-scheda');
  ok(mod.personalizzato && JSON.stringify(mod.modello).includes('"id":"astuzia"'), 'il campo nuovo è nel modello (identificativo calcolato dal nome)');
  // Un errore nel modello si vede e non si salva.
  await notte.locator('button:has-text("Modello della scheda")').click();
  await p.waitForSelector('.dialogo .ce-editor');
  await p.locator('.dialogo input[name=iniziativa]').fill('1d7 + {mente}');
  await p.click('.dialogo button:has-text("Salva il modello")');
  await p.locator('.dialogo .ce-errore:visible').waitFor({ timeout: 10000 });
  ok(/iniziativa: «1d7»/.test(await p.locator('.dialogo .ce-errore').textContent()), 'errore nella formula spiegato');
  await p.click('.dialogo button:has-text("Annulla")');

  // Giocatore con un personaggio nel gruppo della campagna.
  const cg = await b.newContext({ ignoreHTTPSErrors: true, viewport: { width: 1280, height: 900 } });
  const g = await cg.newPage();
  g.erroriPagina = [];
  g.on('pageerror', (e) => g.erroriPagina.push(e.message));
  await g.goto(BASE + '/registrazione', { waitUntil: 'networkidle' });
  await g.evaluate(() => fetch('/api/v1/registrazione', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ nome: 'Lia',
    telefono: '3330000001', password: 'Nebbia sul lago Varnhold', conferma: 'Nebbia sul lago Varnhold', consenso: true, avvisi: true }) }));
  await chiama(p, '/utenti/' + (await chiama(g, '/io')).utente.id + '/approva', { dati: { gruppi: [gr] } });
  await g.goto(BASE + '/sessioni', { waitUntil: 'networkidle' });
  await chiudiFinestre(g);
  const pg = (await chiama(g, '/personaggi', { dati: { gruppo: gr, nome: 'Arin' } })).id;
  ok(!(await g.locator('nav.menu a').allTextContents()).some((t) => /Finanze/.test(t)), 'scheda libera: niente Finanze (monete della quinta edizione)');
  await g.goto(BASE + '/schede/' + pg, { waitUntil: 'networkidle' });
  await chiudiFinestre(g);
  ok(/scheda_libera/.test(await g.locator('iframe.riquadro-scheda').getAttribute('src')), 'si apre la scheda libera');
  ok(await g.locator('button:has-text("Sanità mentale")').count() === 0, 'niente pulsante della Sanità mentale (non c\'è in questo gioco)');
  const f = g.frameLocator('iframe.riquadro-scheda');
  
  await f.locator('#c-astuzia').waitFor({ timeout: 10000 });
  ok(await f.locator('#c-astuzia').inputValue() === '2' && await f.getByText('Personaggio', { exact: true }).count() === 1, 'scheda disegnata dal modello, con il valore iniziale');
  // Con l'approvazione del Master attiva i dati passano da lui: campi bloccati, punti ferita liberi.
  ok(await f.locator('#c-astuzia').isDisabled() && !(await f.locator('#c-salute').isDisabled()) && await f.locator('.sl-blocco').count() === 1,
    'approvazione del Master: dati bloccati, punti ferita liberi');
  await chiama(p, '/impostazioni', { metodo: 'PATCH', dati: { approvazioneSchede: false } });
  await g.reload({ waitUntil: 'networkidle' });
  await chiudiFinestre(g);
  await f.locator('#c-astuzia:enabled').waitFor({ timeout: 10000 });
  await f.locator('#c-nome').fill('Arin delle Brume');
  await f.locator('#c-concetto').fill('Esploratrice');
  await f.locator('#c-astuzia').fill('4');
  await g.waitForTimeout(2000); // salvataggio automatico
  await f.locator('button[aria-label="Tira Astuzia"]').click();
  await f.locator('#avviso:visible').waitFor({ timeout: 10000 });
  const esito = await f.locator('#avviso').textContent();
  const m = /2d6 \[(\d+), (\d+)\] \+ 4 = (\d+)/.exec(esito);
  ok(m && +m[1] + +m[2] + 4 === +m[3], 'tiro dal campo: 2d6 + astuzia (4) → ' + esito);
  await f.locator('button:has-text("Iniziativa")').click();
  await g.waitForTimeout(1500);
  ok(/Iniziativa: 1d20 \[\d+\]/.test(await f.locator('#avviso').textContent()), 'tiro dell\'iniziativa con la formula del modello');
  await g.screenshot({ path: path.join(__dirname, 'schermate', 'scheda-libera.png') });
  // Dopo aver ricaricato i valori restano; il riepilogo è arrivato al server.
  await g.reload({ waitUntil: 'networkidle' });
  await chiudiFinestre(g);
  await f.locator('#c-astuzia').waitFor({ timeout: 10000 });
  ok(await f.locator('#c-astuzia').inputValue() === '4' && await f.locator('#c-nome').inputValue() === 'Arin delle Brume' && await f.locator('#c-concetto').inputValue() === 'Esploratrice',
    'valori salvati dopo aver ricaricato');
  const elenco = await chiama(g, '/personaggi');
  const io = (Array.isArray(elenco) ? elenco : elenco.lista || elenco.personaggi || []).find((x) => x.id === pg) || {};
  ok(io.riepilogo && io.riepilogo.pfMax === 10 && io.riepilogo.ca === 10 && /^1d20 \+ \d+$/.test(io.riepilogo.iniziativaFormula || ''), 'riepilogo con punti ferita, difesa e formula dell\'iniziativa: ' + JSON.stringify(io.riepilogo));

  // Telefono.
  const tel = await b.newContext({ ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 }, storageState: await cg.storageState() });
  const pt = await tel.newPage();
  await pt.addInitScript(() => { try { localStorage.setItem('gasterbless.dispositivo', 'cellulare'); } catch { /* ignora */ } }); // come sceglie un giocatore al telefono
  await pt.goto(BASE + '/schede/' + pg, { waitUntil: 'networkidle' });
  await chiudiFinestre(pt);
  const ft = pt.frameLocator('iframe.riquadro-scheda');
  await ft.locator('#c-astuzia').waitFor({ timeout: 10000 });
  ok(await ft.locator('body').evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'telefono: la scheda sta nello schermo');
  await pt.screenshot({ path: path.join(__dirname, 'schermate', 'scheda-libera-telefono.png') });
  await tel.close();
  ok(p.erroriPagina.length === 0 && g.erroriPagina.length === 0, 'nessun errore JavaScript: ' + p.erroriPagina.concat(g.erroriPagina).join(' | '));
  await b.close();
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
