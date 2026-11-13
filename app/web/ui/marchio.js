// Marchio GASTER BLESS – MASTER SUITE (branding/MARCHIO.md del repository): intestazione completa e logo breve.
import { el } from "./ui.js";

export const NOME = "GASTER BLESS – MASTER SUITE";

// Intestazione completa: accesso, caricamento, schermate d'attesa.
export function intestazione() {
  return el("div", { class: "gb-intestazione" },
    el("h1", { class: "gb-nome", testo: "Gaster Bless" }),
    el("div", { class: "gb-suite", testo: "Master Suite" }),
    el("hr", { class: "gb-filetto" }),
    el("div", { class: "gb-slogan", testo: "Master your world" }),
    el("div", { class: "gb-sottotitolo", testo: "The universal tabletop RPG platform" }));
}

// Logo breve: barra e menu delle schermate interne.
export function logoBreve() {
  return el("div", { class: "gb-logo", role: "img", "aria-label": NOME },
    el("span", { class: "gb-nome", testo: "Gaster Bless" }),
    el("span", { class: "gb-suite", testo: "Master Suite" }));
}
