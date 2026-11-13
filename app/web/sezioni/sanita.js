// Sezione "Sanità mentale": una funzione dell'app separata dalla scheda. Ogni personaggio ha il suo
// pulsante; il clic apre la mini scheda (valore, tiro salvezza, Terrore, follia, condizioni attive).
// Il giocatore vede i suoi personaggi, il Master tutti, divisi per giocatore.
import { el, svuota, api, tempoReale } from "../ui/ui.js";
import { apriMiniSanita } from "../ui/sanita_mini.js";

const segno = (n) => (n >= 0 ? "+" : "") + n;

export async function mostra(corpo, { io }) {
  const master = io.utente.ruolo === "master";
  const contenitore = el("div", { class: "largo" });
  svuota(corpo, contenitore);

  async function disegna() {
    let pg;
    try { pg = (await api("/personaggi")).filter((p) => p.stato === "attivo"); }
    catch (e) { svuota(contenitore, el("p", { class: "errore", testo: e.message })); return; }
    if (!pg.length) {
      svuota(contenitore, el("section", { class: "scheda-ui" }, el("h2", { testo: "Nessun personaggio" }),
        el("p", { class: "tenue", testo: "La Sanità mentale si apre per ogni personaggio attivo: crea prima un personaggio nella sezione Schede." })));
      return;
    }
    const stati = await Promise.all(pg.map((p) => api("/sanita/pg/" + encodeURIComponent(p.id)).catch(() => null)));
    const gruppi = new Map();
    pg.forEach((p, i) => {
      const k = master ? (p.nomeGiocatore || "Senza giocatore") : "";
      if (!gruppi.has(k)) gruppi.set(k, []);
      gruppi.get(k).push(carta(p, stati[i]));
    });
    svuota(contenitore,
      el("section", { class: "scheda-ui" }, el("h2", { testo: "🧠 Sanità mentale" }),
        el("p", { class: "tenue", testo: "Valore = 10 + modificatore di Saggezza (preso dalla scheda, di qualsiasi tipo). Tocca un personaggio per aprire la sua mini scheda: tiro salvezza, Terrore, follia e condizioni attive." })),
      [...gruppi.entries()].map(([titolo, carte]) => el("section", { class: "scheda-ui" }, titolo ? el("h2", { testo: titolo }) : null, el("div", { class: "griglia" }, carte))));
  }

  function carta(p, s) {
    const righe = [];
    if (s && s.punteggio != null) righe.push("Sanità " + s.punteggio + " (" + segno(s.mod) + ") · TS " + segno(s.bonusTS));
    else if (s) righe.push("Sanità: apri una volta la scheda del personaggio");
    if (s && s.config.terrore) righe.push("Terrore " + s.terrore + " · " + s.livello.nome);
    const pazzie = s ? s.pazzie.filter((x) => x.attiva).length : 0;
    if (pazzie) righe.push(pazzie === 1 ? "1 pazzia attiva" : pazzie + " pazzie attive");
    return el("button", { class: "carta-pg carta-sanita", "aria-label": "Sanità mentale di " + p.nome, on: { click: () => apriMiniSanita(p.id) } },
      el("div", { class: "segnaposto-ritratto", testo: "🧠" }),
      el("div", {}, el("strong", { testo: p.nome }), righe.map((r) => el("div", { class: "tenue piccolo", testo: r }))));
  }

  await disegna();
  const vie = [tempoReale.su("sanita.stato", () => disegna()), tempoReale.su("sanita.config", () => disegna())];
  return () => vie.forEach((f) => f());
}
