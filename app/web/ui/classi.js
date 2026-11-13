// Icone delle classi (web/img/classi/, fornite dall'utente il 2026-11-06).
// Dal testo della classe del riepilogo ("Chierico", "Chierico 3 / Mago 2") si ricavano le icone, in ordine.
import { el } from "./ui.js";

const ID = { artefice: "artefice", barbaro: "barbaro", bardo: "bardo", chierico: "chierico", druido: "druido", guerriero: "guerriero",
  ladro: "ladro", mago: "mago", monaco: "monaco", paladino: "paladino", ranger: "ranger", stregone: "stregone", warlock: "warlock" };

/** Chiavi delle classi presenti nel testo, nell'ordine (la prima è la classe principale). */
export function classiDa(testo) {
  return String(testo || "").split("/").map((p) => ID[p.trim().split(/\s+/)[0].toLowerCase()]).filter(Boolean);
}

/** Icona grande (con cornice) della classe principale, da usare al posto del ritratto; null se sconosciuta. */
export function iconaRitratto(testo, classe = "") {
  const c = classiDa(testo)[0];
  return c ? el("img", { class: classe, src: "/img/classi/" + c + ".svg", alt: "", title: testo }) : null;
}

/** Icone piccole (trasparenti) di tutte le classi, per il multiclasse. */
export function iconePiccole(testo) {
  const l = classiDa(testo);
  return l.length ? el("span", { class: "icone-classi" }, l.map((c) => el("img", { src: "/img/classi/" + c + "_t.svg", alt: c, title: c[0].toUpperCase() + c.slice(1) }))) : null;
}
