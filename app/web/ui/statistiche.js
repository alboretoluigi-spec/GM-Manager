// Statistiche di una creatura (blocco della scheda: CA, PF, caratteristiche, difese, tratti, azioni, azioni leggendarie),
// stesso aspetto della Mappa. Usata dal bestiario (../sezioni/bestiario.js) e dalla finestra del mostro nel tracker
// (../sezioni/combattente.js, 2026-11-12).
import { el } from "./ui.js";

export const CAR = ["FOR", "DES", "COS", "INT", "SAG", "CAR"];
export const segno = (n) => (n >= 0 ? "+" : "") + n;
export const modCar = (v) => Math.floor((v - 10) / 2);

export function statistiche(s) {
  const riga = (k, v) => v ? el("p", {}, el("b", { testo: k + " " }), v) : null;
  const blocco = (t, l) => l && l.length ? [el("h4", { testo: t }), l.map((x) => el("p", {}, x.n ? el("b", {}, el("i", { testo: x.n + ". " })) : null, x.d))] : null;
  return el("div", { class: "stb" },
    el("p", { class: "it", testo: [s.taglia, s.tipo].filter(Boolean).join(" ") + (s.all ? ", " + s.all : "") }), el("hr"),
    riga("Classe Armatura", s.ca), riga("Punti ferita", s.pf), riga("Velocità", s.vel), el("hr"),
    el("div", { class: "car" }, CAR.map((k, i) => el("div", {}, el("b", { testo: k }), (s.car && s.car[i] || 10) + " (" + segno(modCar(s.car && s.car[i] || 10)) + ")"))), el("hr"),
    riga("Tiri salvezza", s.ts), riga("Abilità", s.abil), riga("Vulnerabilità ai danni", s.vuln), riga("Resistenze ai danni", s.res), riga("Immunità ai danni", s.imm),
    riga("Immunità alle condizioni", s.immc), riga("Sensi", s.sensi), riga("Linguaggi", s.ling), riga("Sfida", s.gs), el("hr"),
    s.nota ? el("p", { class: "tenue piccolo", testo: /^Importata da/.test(s.nota) ? "📥 " + s.nota + " (con «Importa dai manuali»)." // scheda completa dal manuale
      : "📕 " + s.nota + ". Il testo si legge dal tuo manuale con «Leggi dal manuale» o «Apri a pag.»." }) : null,
    (s.tratti || []).map((x) => el("p", {}, x.n ? el("b", {}, el("i", { testo: x.n + ". " })) : null, x.d)),
    blocco("Azioni", s.azioni), blocco("Reazioni", s.reazioni), blocco("Azioni leggendarie", s.legg));
}
