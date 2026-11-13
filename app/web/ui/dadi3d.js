// Dadi 3D che rotolano sullo schermo (richiesta dell'utente, 2026-11-12). Il risultato lo decide sempre il server: qui i
// dadi vengono lanciati da un bordo, rimbalzano sul tavolo (gravità, pareti, urti tra dadi) e alla fine si posano con
// la faccia giusta in alto, girati in modo che il numero si legga. Libreria: three.js (web/vendor/three, MIT), caricata
// solo quando serve; senza WebGL, o con «riduci movimento», resta il lancio 2D di dadi.js.
// - Colore: il set di chi tira (tiro.colore «#corpo/#numeri», scelto nel pannello dei dadi).
// - Suono: ticchettii generati al momento a ogni urto (nessun file audio); si spegne dal pannello dei dadi.
// - d20: piccolo effetto grafico secondo la classe del personaggio di chi tira (tiro.classe).
// - Il risultato resta finché non si clicca in un punto qualsiasi (o si preme Esc); si chiude da solo dopo 20 secondi.

export const SET_DADI = [
  { nome: "Sangue di Gaster Bless", c: "#7a1616/#f4e4bc" },
  { nome: "Ossidiana", c: "#1d1d22/#e8c25a" },
  { nome: "Avorio", c: "#efe6d2/#3a2a1a" },
  { nome: "Zaffiro", c: "#1f4fa0/#ffffff" },
  { nome: "Smeraldo", c: "#1d6b45/#f6f0d8" },
  { nome: "Ametista", c: "#5b2a86/#f2e6ff" },
  { nome: "Ambra", c: "#c47a12/#2a1606" },
  { nome: "Nebbia", c: "#8d97a3/#16202a" },
  { nome: "Rubino", c: "#b0122e/#ffe9a8" },
  { nome: "Oro", c: "#c9a227/#3b2a00" },
  { nome: "Argento", c: "#c8ccd2/#1a2a4a" },
  { nome: "Notte", c: "#14213d/#9fd3ff" },
];
const PREDEFINITO = SET_DADI[0].c;

let THREE = null, caricamento = null;
export function caricaTre() {
  if (!caricamento) caricamento = import("/vendor/three/three.module.min.js").then((m) => (THREE = m)).catch(() => { caricamento = null; return null; });
  return caricamento;
}
export const pronto = () => !!THREE;
export function webglOk() {
  try { const c = document.createElement("canvas"); return !!(c.getContext("webgl2") || c.getContext("webgl")); } catch { return false; }
}

// ===== suono =====
const CHIAVE_SUONO = "gasterbless-dadi-suono";
export const suonoAcceso = () => { try { return localStorage.getItem(CHIAVE_SUONO) !== "0"; } catch { return true; } };
export const impostaSuono = (on) => { try { localStorage.setItem(CHIAVE_SUONO, on ? "1" : "0"); } catch { /* niente */ } };
let actx = null;
function audio() {
  if (!suonoAcceso()) return null;
  try {
    actx = actx || new (window.AudioContext || window.webkitAudioContext)();
    if (actx.state === "suspended") actx.resume();
    return actx;
  } catch { return null; }
}
// un colpo di dado sul tavolo: rumore breve filtrato, più forte e più acuto se l'urto è forte
function colpo(forza, legno) {
  const c = audio();
  if (!c || forza < 0.05) return;
  const dur = 0.035 + Math.random() * 0.04;
  const buf = c.createBuffer(1, Math.max(1, Math.floor(c.sampleRate * dur)), c.sampleRate);
  const d = buf.getChannelData(0);
  for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / d.length, 5);
  const src = c.createBufferSource(); src.buffer = buf;
  const bp = c.createBiquadFilter(); bp.type = "bandpass";
  bp.frequency.value = (legno ? 900 : 1700) + Math.random() * 1800 + forza * 900; bp.Q.value = 3 + Math.random() * 5;
  const g = c.createGain(); g.gain.value = Math.min(0.9, 0.12 + forza * 0.6);
  src.connect(bp).connect(g).connect(c.destination);
  src.start(c.currentTime + Math.random() * 0.01);
}

// ===== geometrie =====
// Ogni dado: triangoli con le normali piatte, raggruppati in facce logiche (stessa normale); a ogni faccia un valore.
function geometriaBase(facce) {
  const T = THREE;
  switch (facce) {
    case 4: return new T.TetrahedronGeometry(1.15);
    case 6: return new T.BoxGeometry(1.45, 1.45, 1.45).toNonIndexed();
    case 8: return new T.OctahedronGeometry(1.05);
    case 12: return new T.DodecahedronGeometry(1.05);
    case 20: return new T.IcosahedronGeometry(1.08);
    default: return dieciFacce();
  }
}
// trapezoedro pentagonale (d10): due vertici, due anelli di 5 punti; e = h·(1−cos36°)/(1+cos36°) rende le facce piane
function dieciFacce() {
  const T = THREE, h = 1.05, r = 1.0, e = h * (1 - Math.cos(Math.PI / 5)) / (1 + Math.cos(Math.PI / 5));
  const U = [], L = [];
  for (let k = 0; k < 5; k++) {
    const a = k * 2 * Math.PI / 5, b = a + Math.PI / 5;
    U.push(new T.Vector3(r * Math.cos(a), e, r * Math.sin(a)));
    L.push(new T.Vector3(r * Math.cos(b), -e, r * Math.sin(b)));
  }
  const top = new T.Vector3(0, h, 0), bot = new T.Vector3(0, -h, 0), pos = [];
  const tri = (a, b, c) => pos.push(a.x, a.y, a.z, b.x, b.y, b.z, c.x, c.y, c.z);
  for (let k = 0; k < 5; k++) {
    const u0 = U[k], l0 = L[k], u1 = U[(k + 1) % 5], l1 = L[(k + 1) % 5];
    tri(top, u1, l0); tri(top, l0, u0); // aquilone in alto: top, u0, l0, u1 (verso esterno)
    tri(bot, l0, u1); tri(bot, u1, l1); // aquilone in basso: bot, l0, u1, l1
  }
  const g = new T.BufferGeometry();
  g.setAttribute("position", new T.Float32BufferAttribute(pos, 3));
  g.computeVertexNormals();
  return g;
}

const cacheForme = {};
function forma(facce) {
  const k = facce === 100 ? 10 : facce;
  if (cacheForme[k]) return cacheForme[k];
  const T = THREE;
  let g = geometriaBase(k);
  if (g.index) g = g.toNonIndexed();
  const p = g.getAttribute("position");
  const tris = [];
  for (let i = 0; i < p.count; i += 3) {
    const a = new T.Vector3().fromBufferAttribute(p, i), b = new T.Vector3().fromBufferAttribute(p, i + 1), c = new T.Vector3().fromBufferAttribute(p, i + 2);
    const n = new T.Vector3().subVectors(b, a).cross(new T.Vector3().subVectors(c, a)).normalize();
    if (n.dot(a.clone().add(b).add(c)) < 0) n.negate(); // verso l'esterno
    tris.push({ v: [a, b, c], n });
  }
  // facce logiche: triangoli con la stessa normale
  const fl = [];
  for (const t of tris) {
    let f = fl.find((x) => x.n.dot(t.n) > 0.995);
    if (!f) { f = { n: t.n.clone(), v: [] }; fl.push(f); }
    for (const v of t.v) if (!f.v.some((w) => w.distanceTo(v) < 1e-4)) f.v.push(v.clone());
  }
  for (const f of fl) {
    f.c = f.v.reduce((s, v) => s.add(v), new T.Vector3()).multiplyScalar(1 / f.v.length);
    // vertici in ordine attorno al centro della faccia: così i lati sono le coppie consecutive
    const e1 = f.v[0].clone().sub(f.c).normalize(), e2 = new T.Vector3().crossVectors(f.n, e1);
    f.v.sort((a, b) => Math.atan2(a.clone().sub(f.c).dot(e2), a.clone().sub(f.c).dot(e1)) - Math.atan2(b.clone().sub(f.c).dot(e2), b.clone().sub(f.c).dot(e1)));
    f.ins = Math.min(...f.v.map((v, i) => { const w = f.v[(i + 1) % f.v.length]; return v.clone().add(w).multiplyScalar(0.5).distanceTo(f.c); }));
    // «alto» del numero: verso un vertice (triangoli, pentagoni, aquiloni del d10) o verso un lato (quadrati del d6)
    let verso = f.v[0].clone();
    if (k === 6) { const vic = f.v.slice(1).reduce((m, w) => (w.distanceTo(f.v[0]) < m.distanceTo(f.v[0]) ? w : m)); verso = f.v[0].clone().add(vic).multiplyScalar(0.5); }
    f.su = verso.sub(f.c).projectOnPlane(f.n).normalize();
  }
  // d4: si legge il numero del vertice in alto; i vertici sono i 4 punti distinti
  let vertici = null;
  if (k === 4) {
    vertici = [];
    for (const t of tris) for (const v of t.v) if (!vertici.some((w) => w.distanceTo(v) < 1e-4)) vertici.push(v.clone());
  }
  // valori: facce opposte sommano n+1 (sul d10 si scrive 1…10)
  const n = k;
  const val = new Array(fl.length).fill(0);
  let prossimo = 1;
  for (let i = 0; i < fl.length && prossimo <= n; i++) {
    if (val[i]) continue;
    val[i] = prossimo;
    const o = fl.findIndex((f, j) => !val[j] && f.n.dot(fl[i].n) < -0.995);
    if (o >= 0 && n + 1 - prossimo !== prossimo) val[o] = n + 1 - prossimo;
    prossimo++;
    while (val.includes(prossimo)) prossimo++;
  }
  g.computeBoundingSphere();
  // base: distanza dal centro alla faccia su cui poggia (tutte uguali)
  return (cacheForme[k] = { g, facce: fl, val, vertici, raggio: g.boundingSphere.radius, base: fl[0].c.length() });
}

// numeri: texture disegnate al volo, una per testo e colore
const cacheTex = {};
function texNumero(testo, colore) {
  const k = testo + colore;
  if (cacheTex[k]) return cacheTex[k];
  const cv = document.createElement("canvas"); cv.width = cv.height = 128;
  const x = cv.getContext("2d");
  x.fillStyle = colore; x.textAlign = "center"; x.textBaseline = "middle";
  x.font = "bold " + (testo.length > 2 ? 52 : testo.length > 1 ? 64 : 76) + "px Georgia, 'Times New Roman', serif";
  x.fillText(testo, 64, 68);
  if (testo === "6" || testo === "9") x.fillRect(42, 106, 44, 6); // per distinguere 6 e 9
  const t = new THREE.CanvasTexture(cv);
  t.anisotropy = 4;
  if ("colorSpace" in t) t.colorSpace = THREE.SRGBColorSpace;
  return (cacheTex[k] = t);
}

function testoFaccia(facce, v, decine) {
  if (facce === 100) return decine ? (v === 10 ? "00" : String(v * 10).padStart(2, "0")) : String(v % 10);
  return String(v);
}

// un dado pronto da lanciare: mesh, numeri, e l'orientamento finale per il valore voluto
function costruisciDado(facce, valore, colore, decine) {
  const T = THREE;
  const F = forma(facce);
  const [corpo, numeri] = (colore || PREDEFINITO).split("/");
  const gruppo = new T.Group();
  const mat = new T.MeshStandardMaterial({ color: corpo, roughness: 0.38, metalness: 0.08, flatShading: true });
  const mesh = new T.Mesh(F.g, mat);
  mesh.castShadow = true;
  gruppo.add(mesh);
  const bordi = new T.LineSegments(new T.EdgesGeometry(F.g, 10), new T.LineBasicMaterial({ color: numeri, transparent: true, opacity: 0.25 }));
  gruppo.add(bordi);
  const etichetta = (testo, pos, n, su, dim) => {
    const m = new T.Mesh(new T.PlaneGeometry(dim, dim), new T.MeshBasicMaterial({ map: texNumero(testo, numeri), transparent: true, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -2 }));
    const x = new T.Vector3().crossVectors(su, n).normalize();
    m.quaternion.setFromRotationMatrix(new T.Matrix4().makeBasis(x, su, n));
    m.position.copy(pos).addScaledVector(n, 0.012);
    gruppo.add(m);
  };
  // il valore da mostrare in alto: indice di faccia (o di vertice per il d4)
  const facceN = facce === 100 ? 10 : facce;
  let v = valore;
  if (facce === 100) v = decine ? (Math.floor((valore % 100) / 10) || 10) : (valore % 10 || 10);
  let su0, n0;
  if (facceN === 4) {
    // tre numeri per faccia, vicino ai vertici: in alto c'è il numero del vertice che punta verso l'alto
    F.facce.forEach((f) => {
      f.v.forEach((vv) => {
        const iv = F.vertici.findIndex((w) => w.distanceTo(vv) < 1e-4);
        const dir = vv.clone().sub(f.c);
        etichetta(String(iv + 1), f.c.clone().addScaledVector(dir, 0.55), f.n, dir.clone().normalize(), 0.42);
      });
    });
    const vert = F.vertici[Math.max(0, Math.min(3, v - 1))];
    n0 = vert.clone().normalize();
    // per leggere: una faccia che contiene il vertice, girata verso chi guarda
    const f = F.facce.find((x) => x.v.some((w) => w.distanceTo(vert) < 1e-4));
    su0 = f.n.clone().projectOnPlane(n0).normalize().negate();
  } else {
    const dimN = facceN === 6 ? 0.95 : facceN === 8 ? 0.62 : facceN === 10 ? 0.55 : facceN === 12 ? 0.62 : 0.5;
    F.facce.forEach((f, i) => {
      const testo = testoFaccia(facce, F.val[i], decine);
      etichetta(testo, f.c, f.n, f.su, Math.min(dimN, f.ins * 1.9));
    });
    const i = Math.max(0, F.val.indexOf(v));
    n0 = F.facce[i].n.clone(); su0 = F.facce[i].su.clone();
  }
  // orientamento finale: n0 verso l'alto (+Y), il numero dritto per chi guarda (verso −Z sullo schermo)
  const qa = new T.Quaternion().setFromUnitVectors(n0, new T.Vector3(0, 1, 0));
  const suA = su0.clone().applyQuaternion(qa).projectOnPlane(new T.Vector3(0, 1, 0)).normalize();
  const ang = Math.atan2(suA.x, -suA.z);
  const qb = new T.Quaternion().setFromAxisAngle(new T.Vector3(0, 1, 0), ang);
  const finale = qb.multiply(qa);
  return { gruppo, mat, raggio: F.raggio, base: F.base, finale, facce };
}

// ===== scena (una sola, riusata) =====
let S = null;
function scena() {
  if (S) return S;
  const T = THREE;
  const canvas = document.createElement("canvas");
  canvas.className = "dadi3d-canvas";
  const fx = document.createElement("canvas");
  fx.className = "dadi3d-fx";
  const renderer = new T.WebGLRenderer({ canvas, alpha: true, antialias: true });
  renderer.setPixelRatio(Math.min(2, window.devicePixelRatio || 1));
  renderer.shadowMap.enabled = true;
  renderer.shadowMap.type = T.PCFSoftShadowMap;
  if ("outputColorSpace" in renderer) renderer.outputColorSpace = T.SRGBColorSpace;
  const sc = new T.Scene();
  const cam = new T.PerspectiveCamera(32, 1, 0.1, 100);
  sc.add(new T.HemisphereLight(0xfff6e6, 0x403020, 1.1));
  const sole = new T.DirectionalLight(0xffffff, 1.9);
  sole.position.set(-6, 18, 8);
  sole.castShadow = true;
  sole.shadow.mapSize.set(1024, 1024);
  Object.assign(sole.shadow.camera, { left: -16, right: 16, top: 16, bottom: -16, near: 1, far: 50 });
  sc.add(sole);
  const tavolo = new T.Mesh(new T.PlaneGeometry(80, 80), new T.ShadowMaterial({ opacity: 0.32 }));
  tavolo.rotation.x = -Math.PI / 2;
  tavolo.receiveShadow = true;
  sc.add(tavolo);
  S = { renderer, sc, cam, canvas, fx, ctx: fx.getContext("2d"), dadi: [], W: 6, D: 4 };
  return S;
}

function dimensiona() {
  const T = THREE;
  const w = window.innerWidth, h = window.innerHeight;
  S.renderer.setSize(w, h, false);
  S.canvas.style.width = w + "px"; S.canvas.style.height = h + "px";
  const pr = Math.min(2, window.devicePixelRatio || 1);
  S.fx.width = w * pr; S.fx.height = h * pr; S.fx.style.width = w + "px"; S.fx.style.height = h + "px";
  S.ctx.setTransform(pr, 0, 0, pr, 0, 0);
  S.cam.aspect = w / h;
  // più lontano sugli schermi stretti, così i dadi restano grandi circa uguali
  const alt = 24 * Math.max(1, 0.9 * h / w);
  S.cam.position.set(0, alt, alt * 0.2);
  S.cam.lookAt(0, 0, 0.6);
  S.cam.updateProjectionMatrix();
  S.cam.updateMatrixWorld(); // serve ai raggi qui sotto (altrimenti si aggiorna solo al primo disegno)
  // limiti del tavolo visibile (piano y = 0), con un margine
  const ray = new T.Raycaster(), piano = new T.Plane(new T.Vector3(0, 1, 0), 0), p = new T.Vector3();
  const xs = [], zs = [];
  for (const [x, y] of [[-1, -1], [1, -1], [1, 1], [-1, 1]]) {
    ray.setFromCamera(new T.Vector2(x, y), S.cam);
    if (ray.ray.intersectPlane(piano, p)) { xs.push(p.x); zs.push(p.z); }
  }
  S.W = Math.max(3, Math.min(...xs.map(Math.abs)) - 1.3);
  S.Dn = Math.min(...zs) + 2.4; // lato lontano (in alto sullo schermo), lontano dalla barra in alto
  S.Dv = Math.max(...zs) - 3.2; // lato vicino: lascia spazio al riquadro del totale
}

function svuotaScena() {
  for (const d of S.dadi) {
    S.sc.remove(d.gruppo);
    d.gruppo.traverse((o) => { if (o.material && o.material !== d.mat) o.material.dispose(); if (o.geometry && o.isLineSegments) o.geometry.dispose(); });
    d.mat.dispose();
  }
  S.dadi = [];
  S.ctx.clearRect(0, 0, S.fx.width, S.fx.height);
}

// ===== effetti del d20 per classe =====
// Ogni classe: colori e forma delle particelle. Partono dal d20 quando si ferma.
const EFFETTI = {
  barbaro: { col: ["#ff3b1f", "#ffb347", "#8b0000"], forma: "scintilla", n: 46, anello: "#ff3b1f", scuoti: true },
  bardo: { col: ["#ffd86b", "#ff8fd8", "#9fe3ff"], forma: "nota", n: 16, sale: true },
  chierico: { col: ["#fff3b0", "#ffd700", "#ffffff"], forma: "raggio", n: 14, anello: "#ffe27a" },
  druido: { col: ["#4caf50", "#8bc34a", "#cddc39"], forma: "foglia", n: 22, vortice: true },
  guerriero: { col: ["#ffffff", "#ffd27f", "#c0c0c0"], forma: "scintilla", n: 40 },
  ladro: { col: ["rgba(30,30,40,.55)", "rgba(70,70,90,.5)", "rgba(10,10,15,.6)"], forma: "fumo", n: 18 },
  mago: { col: ["#7fb3ff", "#c3a6ff", "#e0f0ff"], forma: "runa", n: 12, anello: "#7fb3ff", vortice: true },
  monaco: { col: ["#bff7ef", "#ffffff", "#7fded0"], forma: "onda", n: 3 },
  paladino: { col: ["#fff1a8", "#ffd700", "#ffffff"], forma: "raggio", n: 22, colonna: true, anello: "#ffd700" },
  ranger: { col: ["#7cb342", "#a1887f", "#dcedc8"], forma: "freccia", n: 9 },
  stregone: { col: ["#ff4fd8", "#ffd23f", "#3fffd2", "#8f5bff"], forma: "scintilla", n: 50, caos: true },
  warlock: { col: ["#9b30ff", "#5c0099", "#d58bff"], forma: "fiamma", n: 30, sale: true, anello: "#9b30ff" },
  artefice: { col: ["#4dd0e1", "#ffb74d", "#cfd8dc"], forma: "ingranaggio", n: 7, scintille: true },
};
const RUNE = "ᚠᚢᚦᚨᚱᚲᚷᚹᚺᚾᛁᛃᛇᛈᛉᛊᛏᛒᛖᛗᛚᛜᛞᛟ";

function avviaEffetto(classe, x, y, critico) {
  const E = EFFETTI[classe];
  if (!E) return [];
  const parts = [];
  const molt = critico ? 1.6 : critico === null ? 0.4 : 1; // 20 naturale più grande, 1 naturale quasi spento
  for (let i = 0; i < Math.round(E.n * molt); i++) {
    const a = Math.random() * Math.PI * 2, v = (E.forma === "onda" ? 0 : 60 + Math.random() * 180) * molt;
    parts.push({ x, y, vx: Math.cos(a) * v, vy: Math.sin(a) * v - (E.sale ? 90 : 0), a, vita: 0, max: 0.9 + Math.random() * 0.8,
      col: E.col[i % E.col.length], dim: 4 + Math.random() * 8, rot: Math.random() * 6, vr: (Math.random() - 0.5) * 8, i,
      testo: E.forma === "nota" ? "♪♫♬♩"[i % 4] : E.forma === "runa" ? RUNE[(i * 7) % RUNE.length] : "" });
  }
  if (E.scuoti && critico) {
    const app = document.getElementById("app");
    if (app) { app.classList.remove("trema-lieve"); void app.offsetWidth; app.classList.add("trema-lieve"); setTimeout(() => app.classList.remove("trema-lieve"), 500); }
  }
  return [{ E, parts, x, y, t: 0, critico }];
}

function disegnaEffetti(effetti, dt) {
  const c = S.ctx;
  c.clearRect(0, 0, S.fx.width, S.fx.height);
  let vivi = false;
  for (const ef of effetti) {
    ef.t += dt;
    const E = ef.E, t = ef.t;
    if (E.anello && t < 0.9) { // anello che si allarga
      vivi = true;
      c.save(); c.globalAlpha = 1 - t / 0.9; c.strokeStyle = E.anello; c.lineWidth = 6 * (1 - t / 0.9) + 1;
      c.shadowColor = E.anello; c.shadowBlur = 18;
      c.beginPath(); c.arc(ef.x, ef.y, 30 + t * 160, 0, Math.PI * 2); c.stroke(); c.restore();
    }
    if (E.colonna && t < 1.4) { // colonna di luce del paladino
      vivi = true;
      const g = c.createLinearGradient(ef.x, 0, ef.x, ef.y);
      g.addColorStop(0, "rgba(255,230,120,0)"); g.addColorStop(1, "rgba(255,230,120," + (0.55 * (1 - t / 1.4)) + ")");
      c.fillStyle = g; c.fillRect(ef.x - 42, 0, 84, ef.y + 10);
    }
    for (const p of ef.parts) {
      p.vita += dt;
      if (p.vita > p.max) continue;
      vivi = true;
      const k = p.vita / p.max;
      if (E.vortice) { const r = 30 + p.vita * 110, a = p.a + p.vita * 4; p.x = ef.x + Math.cos(a) * r; p.y = ef.y + Math.sin(a) * r * 0.6 - p.vita * 30; }
      else if (E.forma !== "onda") { p.x += p.vx * dt; p.y += p.vy * dt; p.vy += (E.sale ? -40 : 220) * dt; if (E.caos) { p.vx += (Math.random() - 0.5) * 900 * dt; p.vy += (Math.random() - 0.5) * 900 * dt; } }
      p.rot += p.vr * dt;
      c.save(); c.globalAlpha = 1 - k; c.translate(p.x, p.y); c.rotate(p.rot);
      c.fillStyle = p.col; c.strokeStyle = p.col;
      switch (E.forma) {
        case "scintilla": c.shadowColor = p.col; c.shadowBlur = 10; c.fillRect(-p.dim / 2, -1, p.dim, 2.4); break;
        case "nota": case "runa":
          c.shadowColor = p.col; c.shadowBlur = 12; c.font = "bold " + (16 + p.dim * 1.5) + "px serif"; c.textAlign = "center"; c.fillText(p.testo, 0, 0); break;
        case "raggio": c.rotate(p.a - p.rot); c.globalAlpha = (1 - k) * 0.7; c.fillRect(20, -2, 40 + p.vita * 160, 4); break;
        case "foglia": c.beginPath(); c.ellipse(0, 0, p.dim, p.dim / 2.4, 0, 0, Math.PI * 2); c.fill(); break;
        case "fumo": c.globalAlpha = (1 - k) * 0.8; c.beginPath(); c.arc(0, 0, 10 + p.vita * 40, 0, Math.PI * 2); c.fill(); break;
        case "onda": c.globalAlpha = 1 - k; c.lineWidth = 4; c.shadowColor = p.col; c.shadowBlur = 14; c.beginPath(); c.arc(0, 0, 25 + p.vita * (200 + p.i * 60), 0, Math.PI * 2); c.stroke(); break;
        case "freccia": c.rotate(p.a - p.rot); c.lineWidth = 2.5; c.beginPath(); c.moveTo(-14, 0); c.lineTo(14, 0); c.moveTo(14, 0); c.lineTo(8, -5); c.moveTo(14, 0); c.lineTo(8, 5); c.stroke(); break;
        case "fiamma": c.shadowColor = p.col; c.shadowBlur = 16; c.beginPath(); c.ellipse(0, 0, p.dim / 2.2, p.dim, 0, 0, Math.PI * 2); c.fill(); break;
        case "ingranaggio": {
          c.lineWidth = 3; c.shadowColor = p.col; c.shadowBlur = 8; const r = 8 + p.dim;
          c.beginPath(); c.arc(0, 0, r, 0, Math.PI * 2); c.stroke();
          for (let j = 0; j < 8; j++) { c.rotate(Math.PI / 4); c.fillRect(r - 1, -2.5, 6, 5); }
          break;
        }
      }
      c.restore();
    }
  }
  return vivi;
}

// ===== lancio =====
let animazione = 0;
/** Lancia i dadi in 3D dentro «contenitore» (già nella pagina). dadi: [{facce, v, scarto}]. Restituisce i ms prima che
 *  si fermino; alla fine chiama fermi(). */
export function lancia3D(contenitore, dadi, { colore, classe, critico, fallimento }, fermi) {
  scena();
  cancelAnimationFrame(animazione);
  svuotaScena();
  contenitore.prepend(S.canvas, S.fx);
  dimensiona();
  const T = THREE;
  const lista = [];
  dadi.forEach((d) => {
    if (d.facce === 100) { lista.push({ ...d, decine: true }, { ...d, decine: false }); } else lista.push(d);
  });
  const durata = 1.75, settle = 0.5;
  const nd = lista.length;
  lista.forEach((d, i) => {
    const x = costruisciDado(d.facce, d.v, colore, d.decine);
    const g = x.gruppo;
    if (d.scarto) { x.mat.transparent = true; x.mat.opacity = 0.55; }
    // partenza dal bordo vicino (in basso), a destra o a sinistra, verso il centro
    const lato = i % 2 ? 1 : -1;
    const p = new T.Vector3(lato * (S.W * (0.6 + Math.random() * 0.4)), 2.5 + Math.random() * 2, S.Dv + 1.5);
    const meta = new T.Vector3((Math.random() - 0.5) * S.W * 1.2, 0, S.Dn + (S.Dv - S.Dn) * (0.25 + Math.random() * 0.5));
    const v = meta.clone().sub(p).setY(0).normalize().multiplyScalar(11 + Math.random() * 6);
    v.y = 3 + Math.random() * 4;
    g.position.copy(p);
    g.quaternion.setFromEuler(new T.Euler(Math.random() * 6, Math.random() * 6, Math.random() * 6));
    S.sc.add(g);
    S.dadi.push({ ...x, d, p, v, w: new T.Vector3((Math.random() - 0.5) * 22, (Math.random() - 0.5) * 22, (Math.random() - 0.5) * 22),
      ritardo: i * 0.07, t: 0, q0: null, fermo: false });
  });
  let ultimo = performance.now(), totale = 0, effetti = [], fine = false, chiamato = false;
  const passo = (ora) => {
    // tempo reale anche con fotogrammi lenti (telefoni economici): più sotto-passi di fisica per fotogramma
    const reale = Math.min(0.25, Math.max(0, (ora - ultimo) / 1000));
    ultimo = ora;
    const np = Math.max(1, Math.ceil(reale / 0.016));
    for (let k = 0; k < np; k++) fisica(reale / np);
    S.renderer.render(S.sc, S.cam);
    const tuttiFermi = S.dadi.every((a) => a.fermo);
    if (tuttiFermi && !chiamato) { chiamato = true; fermi && fermi(); }
    const vivi = disegnaEffetti(effetti, reale);
    if (!(tuttiFermi && !vivi) || !fine) { if (tuttiFermi && !vivi) fine = true; animazione = requestAnimationFrame(passo); }
  };
  const fisica = (dt) => {
    totale += dt;
    for (const a of S.dadi) {
      if (totale < a.ritardo) { a.gruppo.visible = false; continue; }
      a.gruppo.visible = true;
      a.t += dt;
      const r = a.raggio * 0.82;
      if (a.t < durata - settle) {
        a.v.y -= 32 * dt;
        a.p.addScaledVector(a.v, dt);
        if (a.p.y < r) { // tavolo
          const forza = Math.abs(a.v.y) / 14;
          if (Math.abs(a.v.y) > 1.5) colpo(forza, false);
          a.p.y = r; a.v.y = -a.v.y * 0.42; a.v.x *= 0.82; a.v.z *= 0.82;
          a.w.multiplyScalar(0.78);
          // rotola nella direzione del movimento
          const rot = new T.Vector3(a.v.z, 0, -a.v.x).multiplyScalar(1 / r);
          a.w.lerp(rot, 0.5);
        }
        if (Math.abs(a.p.x) > S.W) { a.p.x = Math.sign(a.p.x) * S.W; a.v.x = -a.v.x * 0.6; colpo(Math.abs(a.v.x) / 20, true); }
        if (a.p.z < S.Dn) { a.p.z = S.Dn; a.v.z = -a.v.z * 0.6; colpo(Math.abs(a.v.z) / 20, true); }
        if (a.p.z > S.Dv + 2) { a.p.z = S.Dv + 2; a.v.z = -Math.abs(a.v.z) * 0.6; }
        const ang = a.w.length() * dt;
        if (ang > 0) a.gruppo.quaternion.premultiply(new T.Quaternion().setFromAxisAngle(a.w.clone().normalize(), ang));
      } else {
        // si posa: dall'orientamento attuale a quello finale, scendendo sul tavolo
        if (!a.q0) { a.q0 = a.gruppo.quaternion.clone(); a.p0 = a.p.clone(); colpo(0.25, false); }
        const k = Math.min(1, (a.t - (durata - settle)) / settle), e = 1 - Math.pow(1 - k, 3);
        a.gruppo.quaternion.slerpQuaternions(a.q0, a.finale, e);
        a.p.lerpVectors(a.p0, new T.Vector3(a.p0.x + a.v.x * 0.04, a.base, a.p0.z + a.v.z * 0.04), e);
        if (k >= 1 && !a.fermo) {
          a.fermo = true;
          if (a.d.facce === 20 && !a.d.scarto && classe) {
            const sp = a.gruppo.position.clone().project(S.cam);
            effetti.push(...avviaEffetto(classe, (sp.x + 1) / 2 * window.innerWidth, (1 - sp.y) / 2 * window.innerHeight, a.d.v === 20 ? true : a.d.v === 1 ? null : false));
          }
        }
      }
      a.gruppo.position.copy(a.p);
    }
    // urti tra dadi: si allontanano
    for (let i = 0; i < S.dadi.length; i++) for (let j = i + 1; j < S.dadi.length; j++) {
      const A = S.dadi[i], B = S.dadi[j];
      if (A.q0 || B.q0 || !A.gruppo.visible || !B.gruppo.visible) continue;
      const dd = B.p.clone().sub(A.p).setY(0), dist = dd.length(), min = (A.raggio + B.raggio) * 0.85;
      if (dist < min && dist > 1e-3) {
        dd.normalize().multiplyScalar((min - dist) / 2);
        A.p.sub(dd); B.p.add(dd);
        const sc_ = A.v.clone(); A.v.x = B.v.x * 0.8; A.v.z = B.v.z * 0.8; B.v.x = sc_.x * 0.8; B.v.z = sc_.z * 0.8;
        colpo(0.3, false);
      }
    }
  };
  animazione = requestAnimationFrame(passo);
  return Math.round((durata + (nd - 1) * 0.07) * 1000) + 60;
}

export function fermaLancio3D() {
  cancelAnimationFrame(animazione);
  if (S) { svuotaScena(); S.renderer.render(S.sc, S.cam); S.canvas.remove(); S.fx.remove(); }
}

export const EFFETTI_CLASSI = Object.keys(EFFETTI);
