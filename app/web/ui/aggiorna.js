// Aggiornamento automatico delle pagine (richiesta dell'utente, 2026-11-08: «Gaster Bless si aggiorni automaticamente ogni
// 5 minuti»). Ogni 5 minuti, e quando il canale in tempo reale si riapre (per esempio dopo lo standby del PC), la pagina
// chiede al server la sua versione: se è cambiata (programma aggiornato o server ripartito) si ricarica da sola.
// Non interrompe chi sta lavorando: aspetta che nessun campo sia in scrittura, nessuna finestra sia aperta e nessun
// video sia in riproduzione, e che siano passati 20 secondi dall'ultimo tasto o tocco.
import { api, avviso, tempoReale } from "./ui.js";

export const OGNI = 5 * 60 * 1000;
let versione = "", ultimaAzione = Date.now(), inAttesa = 0, controllo = 0;
["keydown", "pointerdown", "input"].forEach((t) => document.addEventListener(t, () => { ultimaAzione = Date.now(); }, true));

export function occupato() {
  const a = document.activeElement;
  if (a && (/^(INPUT|TEXTAREA|SELECT)$/.test(a.tagName) || a.isContentEditable)) return true;
  if (a && a.tagName === "IFRAME") { // dentro la scheda o la mappa: si guarda l'elemento attivo del riquadro
    try { const b = a.contentDocument && a.contentDocument.activeElement; if (b && (/^(INPUT|TEXTAREA|SELECT)$/.test(b.tagName) || b.isContentEditable)) return true; } catch { /* altra origine */ }
  }
  if (document.querySelector(".velo, .viaggio-avviso, .benvenuto, .pdf-sovrapposto")) return true;
  if ([...document.querySelectorAll("video, audio")].some((v) => !v.paused && !v.ended)) return true;
  return Date.now() - ultimaAzione < 20000;
}

function ricaricaQuandoLibero() {
  clearTimeout(inAttesa);
  if (occupato()) { inAttesa = setTimeout(ricaricaQuandoLibero, 15000); return; }
  avviso("🔄 Gaster Bless è stata aggiornata: ricarico la pagina…", "", 2500);
  setTimeout(() => location.reload(), 1500);
}

export async function controllaVersione() {
  if (!versione || document.hidden) return;
  let io;
  try { io = await api("/io"); } catch { return; } // server non raggiungibile: ci pensa il salvavita / la modalità offline
  if (io.versione && io.versione !== versione) ricaricaQuandoLibero();
}

/** Da chiamare all'avvio con la versione ricevuta da /io. */
export function avviaAggiornamento(v, ogni = OGNI) {
  if (!v || versione) { if (v) versione = versione || v; return; }
  versione = v;
  clearInterval(controllo);
  controllo = setInterval(controllaVersione, ogni);
  let ultimo = Date.now();
  document.addEventListener("visibilitychange", () => { if (!document.hidden && Date.now() - ultimo > ogni) { ultimo = Date.now(); controllaVersione(); } });
  tempoReale.stato.addEventListener("aperto", () => controllaVersione()); // riconnessione (server ripartito, standby)
}
