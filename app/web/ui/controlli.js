// Controllo delle schede all'accesso in sessione (2026-11-07): il server avvisa il Master solo se la scheda di un
// giocatore ha campi importanti cambiati fuori sessione o senza rete. Il Master vede il prima/dopo e sceglie
// «Va bene» (le modifiche restano) o «Ripristina» (quei campi tornano come all'ultimo controllo).
import { el, api, avviso, dialogo, tempoReale } from "./ui.js";
import { nomeCampo, mostraValore } from "./campi_scheda.js";

let installato = false;
const aperti = new Map(); // id dell'avviso → funzione che chiude la finestra

export async function decidiControllo(id, azione) {
  try {
    const r = await api("/controlli-schede/" + encodeURIComponent(id), { dati: { azione } });
    avviso("📜 " + r.testo, "", 6000);
    return true;
  } catch (e) { avviso(e.message, "errore", 6000); return e.stato === 409; } // già deciso: si chiude comunque
}

export function tabellaDifferenze(c) {
  return el("table", { class: "tabella" },
    el("thead", {}, el("tr", {}, el("th", { testo: "Campo" }), el("th", { testo: "Prima" }), el("th", { testo: "Ora" }), el("th", { testo: "" }))),
    el("tbody", {}, (c.differenze || []).map((d) => el("tr", {},
      el("td", { testo: nomeCampo(d.campo) }),
      el("td", { testo: mostraValore(d.prima) }),
      el("td", {}, el("b", { testo: mostraValore(d.dopo) })),
      el("td", {}, el("span", { class: "etichetta", testo: d.offline ? "offline" : "fuori sessione",
        title: d.offline ? "Modificato sul telefono senza connessione e inviato dopo" : "Modificato quando nessuna sessione era in corso" }))))));
}

function mostraAvviso(c) {
  if (!c || aperti.has(c.id)) return;
  const quante = (c.differenze || []).length;
  dialogo("📜 Controllo della scheda di " + (c.nome || "un personaggio"), el("div", {},
    el("p", {}, el("b", { testo: c.giocatore || "Il giocatore" }),
      " è entrato in sessione" + (c.sessione ? " («" + c.sessione + "»)" : "") + " e la scheda ha " + quante +
      (quante === 1 ? " campo cambiato " : " campi cambiati ") + (c.offline ? "anche senza connessione" : "fuori sessione") + " dall'ultimo controllo."),
    tabellaDifferenze(c),
    el("p", { class: "tenue piccolo", testo: "«Ripristina» riporta solo questi campi ai valori di prima; le altre modifiche restano. Tutte le versioni restano nello storico della scheda." })), {
    azioni: (chiudi) => {
      aperti.set(c.id, chiudi);
      const fine = () => { aperti.delete(c.id); chiudi(); };
      return el("div", { class: "azioni" },
        el("button", { testo: "Più tardi", title: "Lo ritrovi aprendo la scheda o all'avvio dell'app", on: { click: () => { aperti.delete(c.id); chiudi(); } } }),
        el("button", { class: "pericolo", testo: "Ripristina", on: { click: async () => { if (await decidiControllo(c.id, "ripristina")) fine(); } } }),
        el("button", { class: "primario", testo: "Va bene", on: { click: async () => { if (await decidiControllo(c.id, "accetta")) fine(); } } }));
    },
  });
}

export function installaControlli(io) {
  if (installato) return;
  installato = true;
  const master = () => { const u = io(); return u && u.ruolo === "master"; };
  tempoReale.su("scheda.controllo", (c) => { if (master()) mostraAvviso(c); });
  // A ogni collegamento (anche dopo un'interruzione): gli avvisi arrivati mentre il canale era chiuso.
  tempoReale.su("benvenuto", () => { if (master()) controlliAperti(); });
  tempoReale.su("scheda.controllo.deciso", (d) => { // deciso da un'altra finestra del Master
    if (d && aperti.has(d.id)) { const ch = aperti.get(d.id); aperti.delete(d.id); ch(); }
  });
  tempoReale.su("scheda.controllo.esito", (d) => { if (d && d.testo) avviso("📜 " + d.testo, "", 8000); });
}

// Gli avvisi rimasti aperti (per esempio arrivati mentre il Master era scollegato).
export async function controlliAperti() {
  try {
    const l = (await api("/controlli-schede")).lista.filter((c) => c.stato === "aperto");
    l.forEach(mostraAvviso);
  } catch { /* nessun avviso */ }
}
