// Menu laterale a scomparsa e «Mappa grande» (richiesta dell'utente, 2026-11-08).
// - ☰ nell'intestazione (PC e tablet): nasconde il menu laterale; con il menu nascosto lo stesso tasto lo fa comparire
//   sopra la pagina (si richiude scegliendo una voce, toccando fuori o con Esc). «📌 Tieni aperto» lo rimette fisso.
//   La scelta resta salvata su questo dispositivo.
// - «⛶ Mappa grande» (sezione Mappa): nasconde menu, intestazione e barra in basso; la mappa occupa tutto lo schermo.
//   Si torna indietro con «✕ Riduci» o con Esc.
import { el, layoutAttuale } from "./ui.js";

const CHIAVE = "gasterbless.menu";
const radice = document.documentElement;

export function menuNascosto() { try { return localStorage.getItem(CHIAVE) === "nascosto"; } catch { return false; } }
function salva(nascosto) { try { nascosto ? localStorage.setItem(CHIAVE, "nascosto") : localStorage.removeItem(CHIAVE); } catch { /* ignora */ } }
export function applicaMenu() { radice.classList.toggle("menu-chiuso", menuNascosto()); }
applicaMenu();

export function chiudiCassetto() {
  radice.classList.remove("menu-aperto");
  document.querySelectorAll(".velo-menu").forEach((v) => v.remove());
}
function apriCassetto() {
  if (radice.classList.contains("menu-aperto")) return chiudiCassetto();
  radice.classList.add("menu-aperto");
  const v = el("div", { class: "velo-menu", "aria-hidden": "true" });
  v.__chiudi = chiudiCassetto; // Esc (ui.js)
  v.addEventListener("click", chiudiCassetto);
  document.body.append(v);
  const prima = document.querySelector(".menu a");
  if (prima) prima.focus();
}

/** Tasto ☰ per l'intestazione (non sul telefono, che ha la barra in basso). */
export function tastoMenu() {
  if (layoutAttuale() === "cellulare") return null;
  return el("button", { class: "piccolo tasto-menu non-stampare", "aria-label": "Mostra o nascondi il menu", title: "Mostra o nascondi il menu laterale", testo: "☰",
    on: { click: () => { if (menuNascosto()) apriCassetto(); else { salva(true); applicaMenu(); } } } });
}
/** Tasto nel menu: quando il menu è a scomparsa lo rimette fisso. */
export function tastoFissa() {
  return el("button", { class: "piccolo tasto-fissa", title: "Rimetti il menu sempre visibile", testo: "📌 Tieni aperto",
    on: { click: () => { salva(false); chiudiCassetto(); applicaMenu(); } } });
}

// ----- Mappa grande -----
export function mappaGrande(attiva) {
  radice.classList.toggle("mappa-grande", attiva);
  document.querySelectorAll(".esci-mappa-grande").forEach((b) => b.remove());
  if (!attiva) return;
  const b = el("button", { class: "esci-mappa-grande non-stampare", title: "Torna alla vista normale (Esc)", testo: "✕ Riduci" });
  b.__chiudi = () => mappaGrande(false); // Esc (ui.js), anche premuto dentro la mappa
  b.addEventListener("click", b.__chiudi);
  document.body.append(b);
}
