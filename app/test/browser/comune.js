// Funzioni comuni alle prove nel browser.
const { chromium } = require('playwright');
const BASE = 'https://127.0.0.1:7600';
let errori = 0;
function ok(cond, msg) { if (cond) console.log('ok', msg); else { console.log('FALLITO', msg); errori++; } }
async function browser() { return chromium.launch(); }
// Crea l'account del Master al primo avvio (oppure entra) e restituisce la pagina.
async function master(b, vp = { width: 1280, height: 900 }) {
  const c = await b.newContext({ ignoreHTTPSErrors: true, viewport: vp });
  const p = await c.newPage();
  p.erroriPagina = [];
  p.on('pageerror', (e) => p.erroriPagina.push(e.message));
  await p.goto(BASE + '/', { waitUntil: 'networkidle' });
  const campi = await p.locator('input').all();
  await campi[0].fill('Master'); await campi[1].fill('PasswordProva123'); await campi[2].fill('PasswordProva123');
  await p.getByRole('button', { name: /Crea l.account/ }).click();
  await p.waitForTimeout(2000);
  await chiudiFinestre(p);
  return p;
}
// Chiude le finestre iniziali (scelta del dispositivo, icona sulla schermata Home).
async function chiudiFinestre(p) {
  for (let i = 0; i < 4; i++) {
    if (!(await p.locator('.velo').count())) return;
    const tardi = p.getByRole('button', { name: /Più tardi|Ho capito|Chiudi/ });
    if (await tardi.count()) await tardi.first().click().catch(() => {}); else await p.keyboard.press('Escape');
    await p.waitForTimeout(400);
  }
}
function fine() { if (errori) { console.log('FALLITO: ' + errori + ' controlli'); process.exit(1); } }
module.exports = { BASE, ok, browser, master, chiudiFinestre, fine };
