// «Saluta il gruppo»: il Master apre su tutti gli schermi della sessione l'immagine di saluto su sfondo nero
// con la scritta «Alla prossima sessione...». Ognuno la chiude toccando lo schermo o con Esc.
// Lo stesso saluto compare per 3 secondi al giocatore che preme «Esci» (app.js), poi l'app esce.
import { tempoReale } from "./ui.js";

/** Mostra il saluto. Con durata (ms) si chiude da solo. Restituisce una Promise che si risolve alla chiusura. */
export function mostraSaluto(testo, { durata = 0 } = {}) {
  document.querySelector(".saluto")?.remove();
  let fatto;
  const chiuso = new Promise((ok) => { fatto = ok; });
  const o = document.createElement("div");
  o.className = "saluto";
  o.setAttribute("role", "dialog");
  o.setAttribute("aria-label", testo);
  o.innerHTML = '<img src="/img/saluto.webp" alt=""><p class="saluto-testo"></p><span class="saluto-chiudi">tocca per chiudere</span>';
  o.querySelector(".saluto-testo").textContent = testo;
  let t = 0;
  const chiudi = () => {
    if (o.classList.contains("via")) return;
    clearTimeout(t); o.classList.add("via"); document.removeEventListener("keydown", esc);
    setTimeout(() => { o.remove(); fatto(); }, 600);
  };
  const esc = (e) => { if (e.key === "Escape") chiudi(); };
  o.addEventListener("click", chiudi);
  document.addEventListener("keydown", esc);
  o.__chiudi = chiudi; // anche per Esc premuto dentro la mappa o la scheda (ui.js)
  document.body.append(o);
  try { o.setAttribute("tabindex", "-1"); o.focus({ preventScroll: true }); } catch { /* ignora */ }
  if (durata) t = setTimeout(chiudi, durata);
  return chiuso;
}

export function installaSaluto() {
  tempoReale.su("saluto", (d) => mostraSaluto(d.testo || "Alla prossima sessione..."));
}
