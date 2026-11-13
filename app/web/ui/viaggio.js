// Viaggi del segnalino del gruppo (2026-11-08).
// Quando il segnalino viene spostato di più di 1/4 di miglio, il server registra il viaggio e manda «viaggio.avviso»
// al gruppo e al Master. Se ci sono eventi (arrivo a una località, terreno difficile, Monte Gelo, marcia forzata,
// nuovo giorno) compare l'avviso a tutto schermo; altrimenti solo un avviso breve con strada fatta e tempo passato.
// L'avviso si chiude con un tocco, con il pulsante o con Esc.
import { tempoReale, avviso, fuocoInPrimoPiano } from "./ui.js";

const mi1 = (x) => (Math.round(x * 10) / 10).toFixed(1).replace(".", ",");
export function durata(h) { const m = Math.round(h * 60); return Math.floor(m / 60) + " h " + String(m % 60).padStart(2, "0") + " min"; }

export function mostraViaggio(d = {}) {
  const r = d.riepilogo || {}, ev = d.eventi || [];
  const sintesi = mi1(r.miglia || 0) + " miglia in " + durata(r.ore || 0) + " · giorno " + (r.giorno || 1);
  if (!ev.length) { avviso("🧭 Viaggio: " + sintesi, "", 5000); return; }
  const mostra = () => {
    const velo = document.createElement("div");
    velo.className = "viaggio-avviso";
    velo.setAttribute("role", "alertdialog");
    velo.setAttribute("aria-label", "Avviso di viaggio");
    const bx = document.createElement("div");
    bx.className = "viaggio-box";
    const h = document.createElement("h2");
    h.textContent = "🧭 Viaggio del gruppo";
    const s = document.createElement("p");
    s.className = "viaggio-sintesi"; s.textContent = sintesi;
    bx.append(h, s);
    for (const e of ev) {
      const sez = document.createElement("section");
      sez.className = "viaggio-evento " + e.tipo;
      const t = document.createElement("h3"); t.textContent = (e.icona ? e.icona + " " : "") + e.titolo;
      sez.append(t);
      if (e.testo) { const p = document.createElement("p"); p.textContent = e.testo; sez.append(p); }
      bx.append(sez);
    }
    const ok = document.createElement("button");
    ok.className = "primario"; ok.textContent = "Ho capito";
    const nota = document.createElement("p");
    nota.className = "tenue piccolo"; nota.textContent = "Tocca lo schermo o premi Esc per chiudere.";
    bx.append(ok, nota);
    velo.append(bx);
    document.body.append(velo);
    const chiudi = () => velo.remove();
    velo.__chiudi = chiudi; // Esc (ui.js)
    velo.addEventListener("click", chiudi);
    fuocoInPrimoPiano(velo);
  };
  // L'immagine dell'arrivo al Monte Gelo (3 secondi) viene prima.
  if (document.querySelector(".gelo-schermo")) setTimeout(mostra, 3100); else mostra();
}

export function installaViaggi() {
  tempoReale.su("viaggio.avviso", mostraViaggio);
}
