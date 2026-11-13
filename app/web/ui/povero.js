// Monete insufficienti (richiesta dell'utente, 2026-11-07): per 2 secondi, su sfondo nero, l'immagine scelta dal Master
// e la scritta. Si attiva da sola per ogni risposta del server con il codice "monete_insufficienti" (ui.js → api).
let attivo = false;
export function mostraPovero() {
  if (attivo) return;
  attivo = true;
  const velo = document.createElement("div");
  velo.className = "povero";
  velo.setAttribute("role", "alert");
  const img = document.createElement("img");
  img.src = "/img/povero.webp";
  img.alt = "";
  const t = document.createElement("p");
  t.textContent = "Sei povero come la merda. Trovati un lavoro!";
  velo.append(img, t);
  document.body.append(velo);
  try { velo.setAttribute("tabindex", "-1"); velo.focus({ preventScroll: true }); } catch { /* ignora */ } // Esc lo chiude (ui.js)
  setTimeout(() => { velo.remove(); attivo = false; }, 2000);
}
export function installaPovero() {
  document.addEventListener("gasterbless:povero", mostraPovero);
}
