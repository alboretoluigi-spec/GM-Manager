// Schede che collegano le due sezioni della libreria dei regolamenti (docs/RULEBOOK_LIBRARY.md).
import { el } from "./ui.js";

export function schedeLibreria(attiva) {
  const voce = (href, testo, id) => el("a", { href, class: "rl-scheda" + (attiva === id ? " attiva" : ""), "aria-current": attiva === id ? "page" : undefined, testo });
  return el("nav", { class: "rl-schede", "aria-label": "Libreria dei regolamenti" },
    voce("/regolamenti", "RPG Rulebook Library", "catalogo"), voce("/miei-regolamenti", "My Rulebooks", "miei"),
    voce("/analisi-regole", "Regole da approvare", "analisi"));
}
