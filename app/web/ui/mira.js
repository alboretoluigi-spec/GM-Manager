// «[Creatura] prende di mira [Bersaglio]» (richiesta dell'utente, 2026-11-12). Il Master, nel combattimento, sceglie il
// bersaglio di un attacco o di un incantesimo di un mostro o PNG: Gaster Bless lo dice su tutti gli schermi della sessione
// (server: internal/server/mira.go). Chi ha il personaggio preso di mira lo vede in rosso.
import { el, tempoReale } from "./ui.js";

let attesa = 0, ultimo = "";
/** Da chiamare quando il Master sceglie o cambia i bersagli di un mostro (attacco.js, incantesimo_scontro.js). */
export function segnalaMira(sessione, attaccante, bersagli) {
  if (!sessione || !attaccante || attaccante.tipo !== "mostro") return;
  const ids = (bersagli || []).filter(Boolean);
  clearTimeout(attesa);
  if (!ids.length) return;
  const chiave = attaccante.id + ":" + ids.join(",");
  attesa = setTimeout(() => {
    if (chiave === ultimo) return; // stessa scelta di prima: niente doppioni
    ultimo = chiave;
    setTimeout(() => { if (ultimo === chiave) ultimo = ""; }, 8000);
    tempoReale.chiedi("iniz.mira", { sessione, combattente: attaccante.id, bersagli: ids }).catch(() => {});
  }, 350);
}

const elenco = (l) => (l.length < 2 ? l.join("") : l.slice(0, -1).join(", ") + " e " + l[l.length - 1]);

let installato = false;
export function installaMira(io) {
  if (installato) return;
  installato = true;
  tempoReale.su("iniz.mira", (m) => {
    if (!m || !m.chi || !m.bersagli || !m.bersagli.length) return;
    const me = io() && (m.proprieta || []).includes(io().id);
    document.querySelectorAll(".mira-banner").forEach((x) => x.remove());
    const b = el("div", { class: "mira-banner" + (me ? " a-me" : ""), role: "alert" },
      el("span", { class: "mira-icona", "aria-hidden": "true", testo: "🎯" }),
      el("span", {}, el("b", { testo: m.chi }), " prende di mira ", el("b", { testo: elenco(m.bersagli) })));
    document.body.append(b);
    setTimeout(() => { b.classList.add("svanisce"); setTimeout(() => b.remove(), 500); }, 4500);
  });
}
