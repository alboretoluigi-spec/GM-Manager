// RPG RULEBOOK LIBRARY, passo L1: catalogo, ricerca, filtri, fonti ufficiali.
const path = require('path');
const { BASE, ok, browser, master, chiudiFinestre, fine } = require('./comune');
(async () => {
  const b = await browser();
  const p = await master(b);
  await p.goto(BASE + '/regolamenti', { waitUntil: 'networkidle' });
  await p.waitForSelector('.rl-gioco');
  await chiudiFinestre(p);
  const giochi = () => p.locator('.rl-gioco').count();
  ok(await giochi() === 20, 'catalogo: 20 giochi');
  ok((await p.locator('.rl-titolo').textContent()).toLowerCase().includes('rulebook library'), 'titolo della sezione');
  ok(await p.locator('nav.menu').getByText('RPG Rulebook Library').count() === 1, 'voce nel menu del Master');
  await p.screenshot({ path: path.join(__dirname, 'schermate', 'regolamenti-catalogo.png'), fullPage: false });

  await p.fill('input[aria-label="Nome del gioco"]', 'cthulhu');
  await p.waitForTimeout(700);
  ok(await giochi() === 1 && (await p.locator('.rl-gioco h3').textContent()) === 'Call of Cthulhu', 'ricerca per nome');
  await p.fill('input[aria-label="Nome del gioco"]', '');
  await p.waitForTimeout(700);
  ok(await giochi() === 20, 'ricerca svuotata');

  await p.selectOption('select[aria-label="Editore"]', 'Chaosium');
  await p.waitForTimeout(500);
  ok(await giochi() === 3, 'filtro editore: 3 giochi Chaosium');
  await p.selectOption('select[aria-label="Editore"]', '');
  await p.selectOption('select[aria-label="Tipologia di documento"]', 'srd');
  await p.waitForTimeout(500);
  const tip = await p.locator('.rl-doc > p.tenue').allTextContents();
  ok(tip.length > 0 && tip.every((t) => t.startsWith('SRD')), 'filtro tipologia: solo SRD');
  await p.click('button:has-text("Azzera i filtri")');
  await p.waitForTimeout(500);
  await p.check('.rl-gratis input');
  await p.waitForTimeout(500);
  const nomiGratis = await p.locator('.rl-gioco h3').allTextContents();
  ok(!nomiGratis.includes('The One Ring') && nomiGratis.length > 5, 'filtro gratuiti: esclusi i giochi senza documenti gratuiti');
  await p.click('button:has-text("Azzera i filtri")');
  await p.waitForTimeout(500);
  await p.selectOption('select[aria-label="Genere"]', 'horror');
  await p.waitForTimeout(500);
  ok((await p.locator('.rl-gioco h3').allTextContents()).includes('Call of Cthulhu'), 'filtro genere');
  await p.click('button:has-text("Azzera i filtri")');
  await p.waitForTimeout(500);

  // fonti ufficiali: si aprono in una nuova scheda, solo https, senza referrer
  const fonti = p.locator('a.rl-fonte');
  const n = await fonti.count();
  let buone = 0;
  for (let i = 0; i < n; i++) {
    const a = fonti.nth(i);
    if ((await a.getAttribute('href')).startsWith('https://') && await a.getAttribute('target') === '_blank' && (await a.getAttribute('rel')).includes('noopener')) buone++;
  }
  ok(n > 20 && buone === n, 'collegamenti alle fonti ufficiali: ' + n + ' in https, nuova scheda, noopener');
  // la nuova scheda chiede la pagina dell'editore (qui la rete esterna può non rispondere: conta la richiesta)
  const [nuova, richiesta] = await Promise.all([p.context().waitForEvent('page'),
    p.context().waitForEvent('request', (r) => r.url().startsWith('https://www.daggerheart.com/')),
    p.locator('[data-doc="dh-srd10"] a.rl-fonte').click()]);
  ok(!!richiesta, 'clic su «Apri la fonte ufficiale» apre la pagina dell\'editore in una nuova scheda');
  await nuova.close();

  // telefono: la pagina si adatta
  const t = await p.context().newPage();
  await t.setViewportSize({ width: 390, height: 844 });
  await t.goto(BASE + '/regolamenti', { waitUntil: 'networkidle' });
  await t.waitForSelector('.rl-gioco');
  await chiudiFinestre(t);
  const largo = await t.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1);
  ok(largo, 'telefono: nessuno scorrimento orizzontale');
  await t.screenshot({ path: path.join(__dirname, 'schermate', 'regolamenti-telefono.png') });
  ok(p.erroriPagina.length === 0, 'nessun errore JavaScript nella pagina: ' + p.erroriPagina.join(' | '));
  await b.close();
  fine();
})().catch((e) => { console.log('FALLITO', e.message); process.exit(1); });
