// «🎭 PNG in scena» (richiesta dell'utente, 2026-11-12). Il Master, mentre interpreta un PNG, lo sceglie da un elenco e
// con un clic la sua immagine compare sugli schermi dei giocatori del gruppo, su quello del Master e sul proiettore
// (finestra di proiezione della Mappa): senza riquadro né sfondo, Gaster Bless resta visibile dietro; alta quasi tutto lo
// schermo, larga poco più di metà, su un lato, ingrandita anche se il file è piccolo. Le immagini PNG/WebP con lo sfondo
// trasparente si vedono «ritagliate»; le JPG (che lo sfondo ce l'hanno) hanno i bordi sfumati. Un altro clic, o «Togli
// dalla scena», la toglie. Server: internal/server/scena_png.go.
import { el, svuota, api, avviso, dialogo, tempoReale } from "./ui.js";
import { riferimenti, norma } from "./codice_link.js";

const CHIAVE = "gasterbless-scena";
const pref = () => { try { return JSON.parse(localStorage.getItem(CHIAVE)) || {}; } catch { return {}; } };
const salvaPref = (p) => { try { localStorage.setItem(CHIAVE, JSON.stringify(p)); } catch { /* niente */ } };

// ===== sugli schermi =====
// Più immagini insieme (richiesta dell'utente, 2026-11-13): ogni gruppo ha un elenco (al massimo 6), ognuna su un lato
// (sinistra, centro, destra). Le immagini stanno in fila in basso: quelle a sinistra partono dal bordo sinistro, quelle a
// destra dal bordo destro, quelle al centro in mezzo. Con una sola immagine il riquadro è largo poco più di metà schermo;
// con più immagini ognuna ha la sua parte di larghezza (senza sovrapporsi) e un po' meno altezza.
let mioRuolo = "";
const inScena = {}; // gruppo → elenco dei PNG in scena (per il Master: tutti i gruppi)
let ultimoGruppo = ""; // il Master vede sul suo schermo il gruppo cambiato per ultimo
const nascoste = new Set(); // immagini nascoste con ✕ su questo schermo (voce:quando: si rivedono se rimesse)

/* misure dei riquadri in pixel: [{sx, w, h}] nello stesso ordine dell'elenco */
export function disposizione(l, W, H) {
  const n = l.length;
  if (!n) return [];
  const stretto = W < 700; // telefono: l'immagine sola quasi a tutta larghezza
  const w = Math.min(W * (stretto ? 0.94 : 0.58), (W * 0.98) / n);
  const h = H * (stretto ? 0.82 : 0.94) * (n <= 2 ? 1 : Math.max(0.74, 1 - 0.07 * (n - 2)));
  const gruppi = { sinistra: [], centro: [], destra: [] };
  l.forEach((sc, i) => gruppi[sc.lato === "sinistra" || sc.lato === "centro" ? sc.lato : "destra"].push(i));
  const out = new Array(n);
  gruppi.sinistra.forEach((i, k) => { out[i] = { sx: k * w, w, h }; });
  gruppi.destra.forEach((i, k) => { out[i] = { sx: W - (gruppi.destra.length - k) * w, w, h }; });
  const fineSx = gruppi.sinistra.length * w, inizioDx = W - gruppi.destra.length * w, cw = gruppi.centro.length * w;
  const c0 = Math.min(Math.max((W - cw) / 2, fineSx), inizioDx - cw);
  gruppi.centro.forEach((i, k) => { out[i] = { sx: c0 + k * w, w, h }; });
  return out;
}

/* Grande e ben visibile (richiesta dell'utente, 2026-11-12): ogni immagine si ingrandisce fino a riempire il suo riquadro,
   anche se il file è piccolo. Il riquadro dell'immagine coincide con l'immagine, così la sfumatura delle JPG resta sui bordi. */
function adatta() {
  const boxes = [...document.querySelectorAll(".scena-png")];
  const pos = disposizione(boxes.map((b) => ({ lato: b.dataset.lato })), window.innerWidth, window.innerHeight);
  boxes.forEach((box, i) => {
    const p = pos[i];
    Object.assign(box.style, { left: Math.round(p.sx) + "px", right: "auto", width: Math.round(p.w) + "px", height: Math.round(p.h) + "px" });
    const img = box.querySelector("img");
    if (!img || !img.naturalWidth) return;
    const k = Math.min(p.w / img.naturalWidth, p.h / img.naturalHeight);
    img.style.width = Math.round(img.naturalWidth * k) + "px";
    img.style.height = Math.round(img.naturalHeight * k) + "px";
    img.classList.add("pronta");
  });
}
window.addEventListener("resize", adatta);

function disegnaScene(l) {
  const vis = (l || []).filter((sc) => !nascoste.has(sc.voce + ":" + sc.quando));
  const chiavi = vis.map((sc) => sc.voce + ":" + sc.quando + ":" + sc.lato + ":" + sc.nome);
  // le immagini già presenti restano dove sono (niente nuova entrata), le altre si tolgono o si aggiungono al loro posto:
  // l'ordine nel documento è quello dell'elenco (serve ad adatta())
  document.querySelectorAll(".scena-png").forEach((x) => { if (!chiavi.includes(x.dataset.k)) x.remove(); });
  const presenti = new Map([...document.querySelectorAll(".scena-png")].map((x) => [x.dataset.k, x]));
  let dopo = null;
  vis.forEach((sc, i) => {
    let box = presenti.get(chiavi[i]);
    if (!box) {
      const lato = sc.lato === "sinistra" || sc.lato === "centro" ? sc.lato : "destra";
      const jpg = /\.jpe?g$/i.test(sc.file || "");
      const img = el("img", { src: sc.url, alt: sc.nome || "PNG", class: jpg ? "sfuma" : "" });
      img.addEventListener("load", adatta);
      box = el("div", { class: "scena-png " + lato + (mioRuolo === "master" ? " per-master" : ""), role: "img", "aria-label": sc.nome || "PNG in scena", "data-k": chiavi[i], "data-lato": lato },
        img, sc.nome ? el("div", { class: "scena-nome", testo: sc.nome }) : null,
        el("button", { class: "scena-chiudi piccolo", testo: "✕", "aria-label": "Nascondi " + (sc.nome || "l'immagine") + " su questo schermo", title: "Nascondi su questo schermo",
          on: { click: () => { nascoste.add(sc.voce + ":" + sc.quando); box.remove(); adatta(); } } }));
      if (dopo) dopo.after(box);
      else { const primo = document.querySelector(".scena-png"); if (primo) primo.before(box); else document.body.append(box); }
    }
    dopo = box;
  });
  adatta();
}

function chipMaster() {
  document.querySelectorAll(".scena-chip").forEach((x) => x.remove());
  const l = Object.values(inScena).flat();
  if (!l.length) return;
  document.body.append(el("div", { class: "scena-chip" }, l.map((sc) => el("span", { class: "riga" },
    el("img", { src: sc.url, alt: "" }), el("span", { testo: "🎭 In scena: " + (sc.nome || "PNG") }),
    el("button", { class: "piccolo", testo: "✕", "aria-label": "Togli " + (sc.nome || "il PNG") + " dalla scena", title: "Togli dalla scena", on: { click: () => togli(sc.gruppo, sc.voce) } })))));
}

/** Toglie dalla scena un PNG del gruppo (voce) o, senza voce, tutti. */
export async function togli(gruppo, voce) {
  try { await api("/scena?gruppo=" + encodeURIComponent(gruppo) + (voce ? "&voce=" + encodeURIComponent(voce) : ""), { metodo: "DELETE" }); } catch (e) { avviso(e.message, "errore"); }
}

// elenco di un messaggio: «scene» (dal 13/11) o la sola «scena» (server di prima)
const elencoDi = (m) => (Array.isArray(m.scene) ? m.scene : m.attiva && m.scena ? [m.scena] : []);

let installato = false;
export function installaScena(io) {
  if (installato) return;
  installato = true;
  const ruolo = () => (io() && io().ruolo) || "";
  tempoReale.su("scena.png", (m) => {
    if (!m) return;
    const l = elencoDi(m);
    if (ruolo() === "master") {
      mioRuolo = "master";
      if (l.length) inScena[m.gruppo] = l; else delete inScena[m.gruppo];
      ultimoGruppo = m.gruppo;
      chipMaster();
      disegnaScene(inScena[ultimoGruppo] || []);
      document.dispatchEvent(new CustomEvent("gasterbless:scena"));
      return;
    }
    disegnaScene(l);
  });
  // chi apre Gaster Bless dopo vede i PNG già in scena
  setTimeout(async () => {
    mioRuolo = ruolo();
    const u = io();
    if (!mioRuolo || !u || (mioRuolo !== "master" && u.stato !== "approvato")) return; // in attesa di approvazione: niente
    const l = await api("/scena").catch(() => []);
    if (!Array.isArray(l) || !l.length) return;
    if (mioRuolo === "master") {
      l.forEach((sc) => { (inScena[sc.gruppo] = inScena[sc.gruppo] || []).push(sc); });
      ultimoGruppo = l.reduce((a, b) => ((b.quando || 0) >= (a.quando || 0) ? b : a)).gruppo;
      chipMaster(); disegnaScene(inScena[ultimoGruppo]);
      document.dispatchEvent(new CustomEvent("gasterbless:scena"));
    } else {
      const g = l[l.length - 1].gruppo; // un giocatore vede il suo gruppo
      disegnaScene(l.filter((sc) => sc.gruppo === g));
    }
  }, 1200);
}

// ===== il Master sceglie =====
// Raccolte dell'elenco (richiesta dell'utente, 2026-11-13): i PNG si dividono in «Raccolta principale» e «Raccolta alternativa»
// (campo «raccolta» della voce in «PNG e mostri», internal/server/bestiario.go); i mostri restano a parte.
const RACCOLTE = [
  { k: "ufficiale", t: "👤 PNG Raccolta principale", vuota: "Nessun PNG della campagna ufficiale con un'immagine." },
  { k: "reloaded", t: "👤 PNG Raccolta alternativa", vuota: "Nessun PNG della Raccolta alternativa con un'immagine: sposta qui i PNG con il pulsante ⇄ dalla campagna ufficiale, o caricane l'immagine in «PNG e mostri»." },
  { k: "mostri", t: "🐺 Mostri", vuota: "Nessun mostro con un'immagine: caricala in «PNG e mostri» (🖼 Immagini)." },
];
export const raccoltaDi = (v) => (v.cat !== "png" ? "mostri" : v.raccolta === "reloaded" ? "reloaded" : "ufficiale");

export async function apriScena() {
  let gruppi = await api("/gruppi").catch(() => []);
  gruppi = Array.isArray(gruppi) ? gruppi : [];
  if (!gruppi.length) { avviso("Crea prima un gruppo.", "errore"); return; }
  const [best, rif] = await Promise.all([api("/bestiario").catch(() => null), riferimenti()]);
  const imm = (best && best.entries) || {};
  const voci = [];
  const visti = new Set();
  for (const v of rif.voci) if (imm[v.id] && imm[v.id].img && !visti.has(v.id)) { visti.add(v.id); voci.push({ ...v, img: imm[v.id].img, raccolta: imm[v.id].raccolta || "" }); }
  voci.sort((a, b) => (a.cat === "png" ? 0 : 1) - (b.cat === "png" ? 0 : 1) || (b.mia - a.mia) || a.nome.localeCompare(b.nome, "it"));
  const p = pref();
  const selGruppo = el("select", { "aria-label": "Gruppo" }, gruppi.map((g) => el("option", { value: g.id, selected: g.id === p.gruppo, testo: g.nome })));
  const lati = ["sinistra", "centro", "destra"].map((l) => ({ l, i: el("input", { type: "radio", name: "scena-lato", value: l, selezionato: (p.lato || "destra") === l }) }));
  // più immagini insieme (2026-11-13): un clic aggiunge; «una alla volta»: un clic prende il posto delle altre
  const insieme = el("input", { type: "checkbox", selezionato: p.insieme !== false, "aria-label": "Più immagini insieme" });
  const fila = el("div", { class: "scena-fila", "aria-live": "polite" });
  const conNome = el("input", { type: "checkbox", checked: p.nome !== false });
  const cerca = el("input", { type: "search", placeholder: "Cerca un PNG o un mostro…", "aria-label": "Cerca nell'elenco" });
  const lista = el("div", { class: "scena-lista" });
  let scheda = RACCOLTE.some((r) => r.k === p.raccolta) ? p.raccolta : "ufficiale";
  const schede = el("div", { class: "scena-schede", role: "tablist", "aria-label": "Raccolta" });
  const ricorda = () => salvaPref({ gruppo: selGruppo.value, lato: lati.find((x) => x.i.checked).l, nome: conNome.checked, raccolta: scheda, insieme: insieme.checked });
  const metti = async (g, voce, nome, lato, aggiungi) => {
    try { await api("/scena", { metodo: "POST", dati: { gruppo: g, voce, nome, lato, aggiungi } }); } catch (e) { avviso(e.message, "errore"); }
  };
  const ICONA_LATO = { sinistra: "◧", centro: "▣", destra: "◨" };
  // le immagini in scena nel gruppo scelto, da spostare (◧ ▣ ◨) o togliere una per una
  const disegnaFila = () => {
    const g = selGruppo.value, l = inScena[g] || [];
    svuota(fila, l.length ? [el("span", { class: "tenue piccolo", testo: "In scena (" + l.length + "/6):" }), ...l.map((sc) => el("span", { class: "scena-in" },
      el("img", { src: sc.url, alt: "" }), el("span", { testo: sc.nome || "senza nome" }),
      ["sinistra", "centro", "destra"].map((lt) => el("button", { class: "piccolo" + (sc.lato === lt ? " attivo" : ""), testo: ICONA_LATO[lt], "aria-pressed": sc.lato === lt ? "true" : "false",
        "aria-label": (sc.nome || "PNG") + ": " + (lt === "centro" ? "al centro" : "a " + lt), title: lt === "centro" ? "Al centro" : "A " + lt,
        on: { click: () => { if (sc.lato !== lt) metti(g, sc.voce, sc.nome, lt, true); } } })),
      el("button", { class: "piccolo", testo: "✕", "aria-label": "Togli " + (sc.nome || "il PNG") + " dalla scena", title: "Togli dalla scena", on: { click: () => togli(g, sc.voce) } })))]
      : el("span", { class: "tenue piccolo", testo: "Nessuna immagine in scena per questo gruppo." }));
  };
  const sposta = async (v) => {
    const verso = raccoltaDi(v) === "reloaded" ? "ufficiale" : "reloaded";
    try { await api("/bestiario/voci/" + encodeURIComponent(v.id) + "/raccolta", { metodo: "PUT", dati: { raccolta: verso } }); v.raccolta = verso; disegna(); avviso("«" + v.nome + "» ora è in " + (verso === "reloaded" ? "Raccolta alternativa." : "Raccolta principale.")); }
    catch (e) { avviso(e.message, "errore"); }
  };

  const disegna = () => {
    const q = norma(cerca.value);
    const g = selGruppo.value, ora = inScena[g] || [];
    const presente = (id) => ora.some((sc) => sc.voce === id);
    disegnaFila();
    const trovate = voci.filter((v) => !q || norma(v.nome).includes(q));
    // con una ricerca senza risultati qui ma con risultati in un'altra raccolta, si passa a quella
    if (q && !trovate.some((v) => raccoltaDi(v) === scheda) && trovate.length) scheda = raccoltaDi(trovate[0]);
    svuota(schede, RACCOLTE.map((r) => {
      const n = trovate.filter((v) => raccoltaDi(v) === r.k).length;
      return el("button", { class: "piccolo scena-scheda" + (r.k === scheda ? " attivo" : ""), role: "tab", "aria-selected": r.k === scheda ? "true" : "false", testo: r.t + " (" + n + ")",
        on: { click: () => { scheda = r.k; ricorda(); disegna(); } } });
    }));
    const l = trovate.filter((v) => raccoltaDi(v) === scheda);
    const vuota = RACCOLTE.find((r) => r.k === scheda).vuota;
    svuota(lista, l.length ? l.map((v) => el("div", { class: "scena-cella" }, el("button", { class: "scena-voce" + (presente(v.id) ? " attiva" : ""), title: presente(v.id) ? "In scena: clic per toglierlo" : insieme.checked ? "Aggiungi alla scena" : "Mostra ai giocatori",
      "aria-pressed": presente(v.id) ? "true" : "false",
      on: { click: async () => {
        ricorda();
        if (presente(v.id)) { await togli(g, v.id); return; }
        await metti(g, v.id, conNome.checked ? v.nome : "", lati.find((x) => x.i.checked).l, insieme.checked);
      } } },
      el("img", { src: "/media/" + v.img, alt: "", loading: "lazy" }), el("span", { testo: (v.cat === "png" ? "👤 " : "🐺 ") + v.nome })),
      v.cat === "png" ? el("button", { class: "piccolo scena-sposta", testo: "⇄", title: raccoltaDi(v) === "reloaded" ? "Sposta in Raccolta principale" : "Sposta in Raccolta alternativa",
        "aria-label": (raccoltaDi(v) === "reloaded" ? "Sposta in Raccolta principale: " : "Sposta in Raccolta alternativa: ") + v.nome, on: { click: () => sposta(v) } }) : null))
      : el("p", { class: "tenue", testo: q ? "Nessuna voce con questo nome." : vuota }));
  };
  cerca.addEventListener("input", disegna);
  selGruppo.addEventListener("change", () => { ricorda(); disegna(); });
  insieme.addEventListener("change", () => { ricorda(); disegna(); });
  const aggiorna = () => disegna();
  document.addEventListener("gasterbless:scena", aggiorna);
  dialogo("🎭 PNG in scena", el("div", { class: "scena-dialogo" },
    el("p", { class: "tenue piccolo", testo: "Un clic su un nome e la sua immagine compare grande sugli schermi dei giocatori del gruppo, sul tuo e sul proiettore (se la finestra di proiezione è aperta), senza sfondo; un altro clic la toglie. Con «Più immagini insieme» ogni clic ne aggiunge una (fino a 6), sul lato scelto: si mettono in fila senza coprirsi. Per l'effetto «ritagliato» usa immagini PNG con lo sfondo trasparente (con Paint di Windows 11: «Rimuovi sfondo»); le JPG si vedono con i bordi sfumati." }),
    el("div", { class: "riga-flex" },
      el("label", { class: "riga" }, el("span", { class: "tenue piccolo", testo: "Gruppo" }), selGruppo),
      el("span", { class: "riga", role: "radiogroup", "aria-label": "Lato dello schermo" }, lati.map(({ l, i }) => el("label", { class: "riga" }, i, el("span", { testo: l === "sinistra" ? "◧ A sinistra" : l === "centro" ? "▣ Al centro" : "◨ A destra" })))),
      el("label", { class: "riga" }, conNome, el("span", { testo: "Con il nome" })),
      el("label", { class: "riga" }, insieme, el("span", { testo: "➕ Più immagini insieme" }))),
    fila, schede, cerca, lista), {
    largo: true,
    azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { testo: "Togli tutte", on: { click: () => togli(selGruppo.value) } }),
      el("button", { class: "primario", testo: "Chiudi", on: { click: () => { document.removeEventListener("gasterbless:scena", aggiorna); chiudi(); } } })),
  });
  disegna();
}
