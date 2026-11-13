// Incantesimi lanciati dalla scheda (richiesta dell'utente, 2026-11-06): quando un giocatore o il Master preme
// "Lancia incantesimo" o "Rituale", la descrizione compare sugli schermi del Master e del gruppo del personaggio.
// Server: internal/server/incantesimi_lancio.go.
import { el, dialogo, tempoReale } from "./ui.js";

let installati = false;
// io: funzione che restituisce l'utente collegato; chi ha lanciato vede già la descrizione nella scheda.
export function installaIncantesimi(io) {
  if (installati) return;
  installati = true;
  tempoReale.su("incantesimo.lanciato", (d) => {
    if (!d || !d.inc) return;
    const u = io && io();
    if (u && d.chiId === u.id) return;
    const i = d.inc;
    const come = i.rituale ? "come rituale" : i.slot ? "con uno slot di " + i.slot + "° livello" : i.livello === 0 ? "(trucchetto)" : "";
    const riga = (etichetta, valore) => valore ? el("div", {}, el("span", { class: "tenue", testo: etichetta + ": " }), el("b", { testo: valore })) : null;
    dialogo("✨ " + d.pg + " lancia " + i.nome, el("div", { class: "incantesimo-lanciato" },
      el("p", { class: "tenue", testo: (i.intestazione || "") + (come ? " · " + come : "") + (d.chi && d.chi !== d.pg ? " · da " + d.chi : "") }),
      el("div", { class: "inc-dati" }, riga("Tempo di lancio", i.tempo), riga("Gittata", i.gittata), riga("Componenti", i.componenti), riga("Durata", i.durata)),
      el("p", { class: "inc-testo", testo: i.descrizione }),
      i.superiori ? el("p", { class: "inc-testo" }, el("b", { testo: i.livello === 0 ? "Potenziamento dei trucchetti. " : "Ai livelli superiori. " }), i.superiori) : null), { largo: true });
  });
}
