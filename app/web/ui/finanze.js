// Finanze (richiesta dell'utente, 2026-11-07): trasferimento di monete tra personaggi, autorizzazione del Master
// per le modifiche manuali, avvisi dei movimenti. Server: internal/server/finanze.go.
import { el, api, avviso, dialogo, tempoReale } from "./ui.js";

export const moStr = (rame) => (rame / 100).toLocaleString("it-IT", { maximumFractionDigits: 2 }) + " mo";

/** Finestra «💸 Trasferisci»: una parte o tutte le monete del personaggio a un altro personaggio. */
export async function apriTrasferimento(pg, nome = "") {
  let dest = [], fin = null;
  try { [dest, fin] = await Promise.all([api("/finanze/" + encodeURIComponent(pg) + "/destinatari"), api("/finanze")]); }
  catch (e) { avviso(e.message, "errore"); return; }
  const mio = (fin.personaggi || []).find((p) => p.id === pg);
  const a = el("select", { "aria-label": "A chi" }, dest.map((d) => el("option", { value: d.id, testo: d.nome + (d.giocatore ? " (" + d.giocatore + ")" : "") })));
  const imp = el("input", { type: "number", min: "0.01", step: "0.01", placeholder: "in mo, es. 12,5", "aria-label": "Importo in monete d'oro" });
  const tutto = el("input", { type: "checkbox" });
  tutto.addEventListener("change", () => { imp.disabled = tutto.checked; });
  dialogo("💸 Trasferisci monete" + (nome || (mio && mio.nome) ? " · " + (nome || mio.nome) : ""), el("div", { class: "sanita-form" },
    mio ? el("p", {}, "Hai: ", el("b", { testo: moStr(mio.rame) })) : null,
    dest.length ? el("label", {}, "A chi", a) : el("p", { class: "tenue", testo: "Non ci sono altri personaggi attivi." }),
    el("label", {}, "Quanto (monete d'oro; anche decimali: 0,1 mo = 1 ma)", imp),
    el("label", { class: "riga" }, tutto, el("span", { testo: "Tutte le monete" }))), {
    azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "💸 Trasferisci", disabled: !dest.length, on: { click: async () => {
        const rame = Math.round((parseFloat(String(imp.value).replace(",", ".")) || 0) * 100);
        if (!tutto.checked && rame < 1) { avviso("Scrivi quanto trasferire.", "errore"); return; }
        try { const r = await api("/finanze/" + encodeURIComponent(pg) + "/trasferisci", { dati: tutto.checked ? { a: a.value, tutto: true } : { a: a.value, rame } }); avviso(r.testo); chiudi(); }
        catch (e) { avviso(e.message, "errore", 6000); }
      } } })),
  });
}

/** Il Master autorizza o rifiuta una modifica manuale delle monete. */
export async function decidiRichiesta(id, autorizza) {
  try { const r = await api("/finanze-richieste/" + encodeURIComponent(id), { dati: { autorizza } }); avviso(r.testo); return true; }
  catch (e) { avviso(e.message, "errore"); return false; }
}

/** Avvisi in tutta l'app: richieste al Master (con i pulsanti), decisioni e movimenti ai giocatori. */
let installato = false;
export function installaFinanze(io) {
  if (installato) return;
  installato = true;
  tempoReale.su("finanze.richiesta", (d) => {
    const u = io(); if (!u || u.ruolo !== "master" || !d) return;
    dialogo("💰 Modifica delle monete da autorizzare", el("div", {},
      el("p", {}, el("b", { testo: d.da }), " vuole cambiare le monete di ", el("b", { testo: d.nome }), "."),
      el("p", {}, "Prima: ", el("b", { testo: d.prima })), el("p", {}, "Dopo: ", el("b", { testo: d.dopo }), " (differenza " + d.differenza + ")")), {
      azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { testo: "Più tardi", title: "La trovi nella pagina Finanze", on: { click: chiudi } }),
        el("button", { class: "pericolo", testo: "Rifiuta", on: { click: async () => { if (await decidiRichiesta(d.id, false)) chiudi(); } } }),
        el("button", { class: "primario", testo: "Autorizza", on: { click: async () => { if (await decidiRichiesta(d.id, true)) chiudi(); } } })),
    });
  });
  tempoReale.su("finanze.decisione", (d) => { if (d) avviso("💰 " + d.testo, d.autorizzata ? "" : "errore", 7000); });
  tempoReale.su("finanze.movimento", (d) => { if (d && d.testo && !/autorizzata|rifiutata/.test(d.testo)) avviso("💰 " + d.testo, "", 6000); });
}
