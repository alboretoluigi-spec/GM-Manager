// Sezione «Professioni» (richiesta dell'utente, 2026-11-07).
// Giocatore: le professioni dei suoi personaggi che hanno competenza nello strumento e lo strumento
// nell'equipaggiamento. Master: tutte le professioni, sempre attive, con i nomi dei personaggi che le possiedono.
import { el, svuota, api, tempoReale } from "../ui/ui.js";
import { apriProfessione, caricaCatalogo, passaGiorno } from "../ui/professioni.js";

export async function mostra(corpo, { io, azioni }) {
  const master = io.utente.ruolo === "master";
  const box = el("div", { class: "largo" });
  svuota(corpo, box);
  if (master) svuota(azioni, el("button", { class: "piccolo", testo: "⏭ Passa un giorno di gioco", title: "Gli ingredienti invecchiano (durano 7 giorni) e i contatori del giorno si azzerano", on: { click: passaGiorno } }));

  async function disegna() {
    let cat, dati;
    try { [cat, dati] = await Promise.all([caricaCatalogo(), api("/professioni")]); }
    catch (e) { svuota(box, el("p", { class: "errore", testo: e.message })); return; }
    const pgs = dati.personaggi;
    const bottone = (pg, id) => el("button", { class: "prof-btn", on: { click: () => apriProfessione(pg.id, id) } },
      el("span", { class: "prof-ico", "aria-hidden": "true", testo: cat.professioni[id].icona || "⚒" }), el("span", { testo: cat.professioni[id].n }));
    if (!master) {
      svuota(box, pgs.length ? pgs.map((pg) => el("section", { class: "scheda-ui" },
        el("h2", { testo: pg.nome }),
        el("div", { class: "riga-flex" }, pg.professioni.map((id) => bottone(pg, id))),
        el("p", { class: "tenue piccolo", testo: "Compare una professione per ogni strumento in cui il personaggio ha competenza e che ha nell'equipaggiamento." })))
        : el("section", { class: "scheda-ui" }, el("h2", { testo: "Nessuna professione" }),
          el("p", { class: "tenue", testo: "Servono la competenza in uno strumento (per esempio il kit da erborista o da avvelenatore) e lo strumento nell'equipaggiamento della scheda." })));
      return;
    }
    // Master: tutte le professioni, con chi le possiede; si aprono per qualsiasi personaggio
    const scelta = (id) => {
      const sel = el("select", { "aria-label": "Personaggio" }, pgs.map((p) => el("option", { value: p.id, testo: p.nome + (p.professioni.includes(id) ? " ✓" : "") })));
      return pgs.length ? el("div", { class: "riga-flex" }, sel, el("button", { class: "piccolo", testo: "Apri per…", on: { click: () => apriProfessione(sel.value, id) } })) : null;
    };
    svuota(box,
      el("section", { class: "scheda-ui" }, el("h2", { testo: "⚒ Professioni" }),
        el("p", { class: "tenue", testo: "Tutte le professioni sono attive per te. Accanto a ognuna, i personaggi che hanno competenza e strumento: tocca un nome per aprire la professione per lui; «Apri per…» la apre per qualsiasi personaggio." })),
      el("div", { class: "griglia prof-griglia-master" }, Object.entries(cat.professioni).map(([id, d]) => {
        const chi = pgs.filter((p) => p.professioni.includes(id));
        return el("section", { class: "scheda-ui prof-carta" },
          el("h3", {}, el("span", { "aria-hidden": "true", testo: (d.icona || "⚒") + " " }), d.n),
          el("p", { class: "tenue piccolo", testo: [d.raccolta ? "raccolta" : "", d.estrazione ? "estrazione" : "", "fabbricazione"].filter(Boolean).join(" · ") }),
          chi.length ? el("div", { class: "riga-flex" }, chi.map((p) => el("button", { class: "piccolo primario", testo: p.nome + (p.giocatore ? " (" + p.giocatore + ")" : ""), on: { click: () => apriProfessione(p.id, id) } })))
            : el("p", { class: "tenue piccolo", testo: "Nessun personaggio la possiede." }),
          scelta(id));
      })));
  }
  await disegna();
  const vie = [tempoReale.su("scheda.aggiornata", () => disegna())];
  return () => vie.forEach((f) => f());
}
