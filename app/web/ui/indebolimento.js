// Indebolimento (richiesta dell'utente, 2026-11-07): quando un personaggio del giocatore prende un livello di
// indebolimento (dalla scheda, dal tracker o assegnato dal Master), lo schermo diventa nero per 1 secondo con
// l'immagine «stanco» (senza sfondo) e il livello raggiunto. Il messaggio arriva dal server (indebolimento.go).
import { tempoReale, fuocoInPrimoPiano } from "./ui.js";

const IMMAGINE = "/img/stanco.svg"; // si può sostituire con un PNG trasparente
let attivo = false;

export function mostraIndebolimento(d = {}) {
  if (attivo) return;
  attivo = true;
  const velo = document.createElement("div");
  velo.className = "indebolimento";
  velo.setAttribute("role", "alert");
  const img = document.createElement("img");
  img.src = IMMAGINE;
  img.alt = "Stanco";
  const t = document.createElement("p");
  t.textContent = (d.nome ? d.nome + ": " : "") + "indebolimento " + (d.livello || "");
  velo.append(img, t);
  document.body.append(velo);
  fuocoInPrimoPiano(velo); // Esc lo chiude (ui.js)
  const chiudi = () => { velo.remove(); attivo = false; };
  velo.addEventListener("click", chiudi);
  setTimeout(chiudi, 1000);
}

export function installaIndebolimento() {
  tempoReale.su("indebolimento", mostraIndebolimento);
}
