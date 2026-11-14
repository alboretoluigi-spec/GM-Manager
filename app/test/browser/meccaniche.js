// Tiri delle famiglie (fase 3c, C4c): nella sezione Dadi il pannello cambia con il gioco della campagna attiva (d100,
// riserva di d6 con il dado più alto, dadi narrativi) e l'esito compare in parole; nella scheda libera un campo con la
// meccanica del gioco apre il modulo del tiro con il valore già scritto e l'esito torna nella scheda.
const path = require('path');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');
const chiama = (p, percorso, opz) => p.evaluate(([percorso, opz]) => import('/ui/ui.js').then((m) => m.api(percorso, opz)), [percorso, opz]);

async function tiroDadi(p, attesa) {
  await p.goto(BASE + '/dadi', { waitUntil: 'networkidle' });
  await chiudiFinestre(p);
  await p.locator('.mc-pannello').waitFor({ timeout: 10000 });
  if (attesa) await attesa();
  await p.locator('.mc-pannello button:has-text("Tira")').click();
  const nota = p.locator('.lancio-dadi .lancio-nota');
  await nota.waitFor({ state: 'attached', timeout: 15000 });
  await p.waitForTimeout(2500); // i dadi si fermano
  const t = await nota.textContent();
  await p.keyboard.press('Escape');
  return t;
}

(async () => {
  const b = await browser();
  const p = await master(b);
  const camp = (await chiama(p, '/campagne', { metodo: 'POST', dati: { nome: 'Ombre sul porto', modulo: 'd100' } })).id;
  await chiama(p, '/gioco', { metodo: 'PUT', dati: { campagna: camp } });
  await p.reload({ waitUntil: 'networkidle' });

  // d100 sotto il valore
  let esito = await tiroDadi(p, async () => {
    ok(await p.getByText('Tabella della sfortuna').count() === 0, 'niente tabella della sfortuna (è del d20)');
    ok(await p.locator('.mc-sezione h2').textContent() === '🎲 Tiro: Percentuale d100' && await p.locator('.mc-pannello input[name=valore]').count() === 1 &&
       await p.locator('.mc-pannello input[name=extra]').count() === 1, 'Dadi: pannello del d100 con valore e dadi bonus');
    await p.fill('.mc-pannello input[name=valore]', '55');
    await p.fill('.mc-azioni input[type=text]', 'Furtività');
  });
  ok(/^(Successo critico|Successo estremo|Successo difficile|Successo|Fallimento|Disastro)$/.test(esito), 'esito del d100 in parole: ' + esito);
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'meccaniche-d100.png') });

  // riserva di d6, dado più alto
  await chiama(p, '/campagne/' + camp, { metodo: 'PATCH', dati: { modulo: 'riserva-d6' } });
  await p.waitForTimeout(800);
  esito = await tiroDadi(p, async () => {
    await p.selectOption('.mc-pannello select[name=modo]', 'massimo');
    await p.fill('.mc-pannello input[name=dadi]', '3');
    ok(await p.locator('.mc-pannello input[name=soglia]').isHidden(), 'riserva: con il dado più alto la soglia non serve');
  });
  ok(/^(Critico|Successo pieno|Successo parziale|Fallimento)$/.test(esito), 'esito della riserva di d6: ' + esito);

  // dadi narrativi
  await chiama(p, '/campagne/' + camp, { metodo: 'PATCH', dati: { modulo: 'narrativi' } });
  await p.waitForTimeout(800);
  esito = await tiroDadi(p, async () => {
    ok(await p.locator('.mc-pannello input[name=n-competenza]').count() === 1 && await p.locator('.mc-pannello input[name=mod]').count() === 0, 'dadi narrativi: sei tipi di dado, niente modificatore');
  });
  ok(/^(Successo \(\d+ netti\)|Fallimento)/.test(esito), 'esito dei dadi narrativi: ' + esito);
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'meccaniche-narrativi.png') });

  // Scheda libera con un campo che tira con la meccanica (d100).
  await chiama(p, '/campagne/' + camp, { metodo: 'PATCH', dati: { modulo: 'd100' } });
  await chiama(p, '/campagne/' + camp + '/modello-scheda', { metodo: 'PUT', corpo: JSON.stringify({ sezioni: [{ titolo: 'Abilità', campi: [
    { id: 'furtivita', nome: 'Furtività', tipo: 'numero', predefinito: 45, meccanica: true }, { id: 'vita', nome: 'Vita', tipo: 'risorsa', predefinito: 11 }] }],
    tiri: [{ nome: 'Agguato', valore: '{furtivita} + 10' }], pf: 'vita' }), tipo: 'application/json' });
  const gr = (await chiama(p, '/gruppi', { dati: { nome: 'Giovedì' } })).id;
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
  await g.goto(BASE + '/schede/' + pg, { waitUntil: 'networkidle' });
  await chiudiFinestre(g);
  const f = g.frameLocator('iframe.riquadro-scheda');
  await f.locator('#c-furtivita').waitFor({ timeout: 10000 });
  await f.locator('button[aria-label="Tira Furtività"]').click();
  await g.locator('.dialogo .mc-pannello').waitFor({ timeout: 10000 });
  ok(await g.locator('.dialogo .mc-pannello input[name=valore]').inputValue() === '45', 'dalla scheda: modulo del d100 con il valore del campo');
  await g.locator('.dialogo .mc-pannello button:has-text("Tira")').click();
  await f.locator('#avviso:visible').waitFor({ timeout: 10000 });
  const t = await f.locator('#avviso').textContent();
  ok(/Furtività: d100 ≤ 45 → \d+ · (Successo|Fallimento|Disastro)/.test(t), 'esito nella scheda: ' + t);
  await g.keyboard.press('Escape');
  await g.waitForTimeout(500);
  await f.locator('button:has-text("Agguato")').click();
  await g.locator('.dialogo .mc-pannello').waitFor({ timeout: 10000 });
  ok(await g.locator('.dialogo .mc-pannello input[name=valore]').inputValue() === '55', 'tiro del modello con un valore calcolato (45 + 10)');
  ok(!(await g.locator('body').innerText()).includes('Per cambiare questi dati'), 'aprire la scheda non conta come una modifica dei dati');
  await g.screenshot({ path: path.join(__dirname, 'schermate', 'meccaniche-scheda.png') });
  ok(p.erroriPagina.length === 0 && g.erroriPagina.length === 0, 'nessun errore JavaScript: ' + p.erroriPagina.concat(g.erroriPagina).join(' | '));
  await b.close();
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
