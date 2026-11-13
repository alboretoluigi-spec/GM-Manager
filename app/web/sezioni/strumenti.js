// Sezione «Strumenti e kit» (solo Master): la guida di ogni strumento e professione, sempre consultabile.
// I giocatori vedono la guida nella scheda, solo per gli strumenti che possiedono. Dati: /regole/strumenti.js.
import { el, svuota } from "../ui/ui.js";

let pronto = null;
function carica() {
  if (!pronto) pronto = new Promise((ok, ko) => {
    if (window.STRUMENTI) return ok();
    const s = document.createElement("script");
    s.src = "/regole/strumenti.js";
    s.onload = () => ok(); s.onerror = () => { pronto = null; ko(new Error("Impossibile caricare la guida agli strumenti.")); };
    document.head.append(s);
  });
  return pronto;
}

const CAT = { kit: "Kit", artigiano: "Strumenti da artigiano", gioco: "Giochi", musica: "Strumenti musicali" };

export async function mostra(corpo, { io }) {
  if (io.utente.ruolo !== "master") { svuota(corpo, el("p", { class: "errore", testo: "Solo il Master." })); return; }
  svuota(corpo, el("p", { class: "tenue", testo: "Caricamento…" }));
  try { await carica(); } catch (e) { svuota(corpo, el("p", { class: "errore", testo: e.message })); return; }
  const lista = window.STRUMENTI;
  const guida = (T) => el("details", { class: "scheda-ui strum" },
    el("summary", {}, el("b", { testo: T.n }), el("span", { class: "tenue piccolo", testo: " · " + T.costo })),
    el("p", {}, el("b", { testo: "Come si usa. " }), T.uso),
    el("p", {}, el("b", { testo: "Cosa si può fare" })), el("ul", {}, T.puo.map((x) => el("li", { testo: x }))),
    el("p", {}, el("b", { testo: "Cosa non si può fare" })), el("ul", {}, T.nonpuo.map((x) => el("li", { testo: x }))),
    T.ops.length ? [el("p", {}, el("b", { testo: "Prove (CD indicative)" })),
      el("ul", {}, T.ops.map((o) => el("li", { testo: o.n + " · " + o.car + " · " + (o.cd ? "CD " + o.cd : "CD decisa dal Master") + (o.ab ? " · vantaggio con " + o.ab : "") })))] : null,
    (T.fab || []).length ? [el("p", {}, el("b", { testo: "Fabbricazione" })),
      el("ul", {}, T.fab.map((f) => el("li", { testo: f.generico ? f.n + ": materiali a metà del prezzo; " + f.tempo : f.n + ": " + f.mo.toLocaleString("it-IT") + " mo di materiali, " + f.tempo })))] : null,
    el("p", { class: "tenue piccolo", testo: "Abilità collegate: " + (T.abil || []).join(", ") }));
  const gruppi = Object.keys(CAT).map((k) => [k, lista.filter((T) => T.cat === k)]).filter(([, l]) => l.length);
  svuota(corpo, el("div", { class: "largo" },
    el("section", { class: "scheda-ui" }, el("h2", { testo: "🧰 Strumenti e kit" }),
      el("p", { testo: window.STRUMENTI_REGOLE }),
      el("p", { class: "tenue piccolo", testo: "Nella scheda i giocatori vedono la guida solo degli strumenti che hanno nell'equipaggiamento, e possono usarli (prove e fabbricazione) solo con la competenza. Testi scritti con parole nostre dalle regole 2014 (Manuale del Giocatore, supplemento)." })),
    gruppi.map(([k, l]) => [el("h2", { class: "strum-cat", testo: CAT[k] }), l.map(guida)])));
}
