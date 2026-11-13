// Segnaposto per le sezioni non ancora realizzate (vedi l'ordine dei lavori in docs/FASE_2_PROGETTO.md, sezione 13).
import { el, svuota } from "../ui/ui.js";

export function segnaposto(corpo, titolo, descrizione) {
  svuota(corpo, el("div", { class: "largo" }, el("section", { class: "scheda-ui" },
    el("h2", { testo: titolo }),
    el("p", { testo: descrizione }),
    el("p", { class: "tenue", testo: "Questa parte è in preparazione: arriverà nei prossimi passi della Fase 3." }))));
}
