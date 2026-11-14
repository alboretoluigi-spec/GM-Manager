// Sezione «💰 Finanze». Master: modifiche da autorizzare, monete di tutti i personaggi (dare o togliere), registro dei
// movimenti, ricompense automatiche. Giocatore: le monete dei suoi personaggi, le sue richieste, «Trasferisci», registro.
import { el, svuota, api, avviso, quando, tempoReale, dialogo } from "../ui/ui.js";
import { apriTrasferimento, decidiRichiesta, moStr } from "../ui/finanze.js";

export async function mostra(corpo, { io }) {
  // Giochi diversi dalla quinta edizione: le valute del modello della scheda (fase 3c, C4d)
  if (io.gioco && io.gioco.modulo && io.gioco.modulo.id !== "d20-5e") return mostraValute(corpo, io);
  const master = io.utente.ruolo === "master";
  const box = el("div", { class: "largo" });
  svuota(corpo, box);

  async function disegna() {
    let f;
    try { f = await api("/finanze"); } catch (e) { svuota(box, el("p", { class: "errore", testo: e.message })); return; }
    const testoMonete = (m) => ["MP", "MO", "ME", "MA", "MR"].filter((k) => m[k]).map((k) => m[k] + " " + k.toLowerCase()).join(", ") || "nessuna moneta";
    svuota(box,
      f.richieste.length || master ? el("section", { class: "scheda-ui" }, el("h2", { testo: master ? "Modifiche da autorizzare" : "Richieste in attesa del Master" }),
        f.richieste.length ? f.richieste.map((r) => el("div", { class: "fin-richiesta" },
          el("p", {}, el("b", { testo: r.nome }), el("span", { class: "tenue", testo: " · " + r.da + " · " + quando(r.quando) })),
          el("p", { testo: r.prima + " → " + r.dopo + " (" + r.differenza + ")" }),
          master ? el("div", { class: "riga-flex" },
            el("button", { class: "pericolo piccolo", testo: "Rifiuta", on: { click: async () => { await decidiRichiesta(r.id, false); disegna(); } } }),
            el("button", { class: "primario piccolo", testo: "Autorizza", on: { click: async () => { await decidiRichiesta(r.id, true); disegna(); } } })) : null))
          : el("p", { class: "tenue", testo: "Nessuna richiesta." })) : null,
      el("section", { class: "scheda-ui" }, el("h2", { testo: master ? "Monete dei personaggi" : "Le tue monete" }),
        f.personaggi.length ? el("div", { class: "griglia" }, f.personaggi.map((p) => el("div", { class: "fin-pg" },
          el("p", {}, el("b", { testo: p.nome }), p.giocatore && master ? el("span", { class: "tenue", testo: " · " + p.giocatore }) : null),
          el("p", { class: "fin-tot", testo: moStr(p.rame) }), testoMonete(p.monete) !== moStr(p.rame) ? el("p", { class: "tenue piccolo", testo: testoMonete(p.monete) }) : null,
          el("div", { class: "riga-flex" },
            el("button", { class: "piccolo", testo: "💸 Trasferisci", on: { click: () => apriTrasferimento(p.id, p.nome) } }),
            master ? daiTogli(p) : null))))
          : el("p", { class: "tenue", testo: "Nessun personaggio attivo." }),
        el("p", { class: "tenue piccolo", testo: master
          ? "Le tue modifiche valgono subito. Quelle scritte a mano dai giocatori nella scheda arrivano qui da autorizzare. Acquisti dall'elenco, professioni e trasferimenti scalano le monete da soli."
          : "Se cambi a mano le monete nella scheda, il Master deve autorizzare la modifica. Acquisti dall'elenco, professioni e trasferimenti le scalano da soli." })),
      master ? el("section", { class: "scheda-ui" }, el("h2", { testo: "Ricompense automatiche" }),
        el("label", { class: "riga" }, el("input", { type: "checkbox", selezionato: f.ricompenseAuto, on: { change: async (e) => {
          try { await api("/finanze/impostazioni", { metodo: "PUT", dati: { ricompenseAuto: e.target.checked } }); avviso("Salvato."); } catch (er) { avviso(er.message, "errore"); }
        } } }), el("span", { testo: "Quando un mostro del tracker scende a 0 PF, il gruppo riceve GS × 10 mo, divise tra i personaggi presenti nello scontro." })),
        el("p", { class: "tenue piccolo", testo: "Il GS si scrive quando aggiungi il mostro al tracker (dal bestiario si compila da solo). Ogni mostro paga una volta sola." })) : null,
      el("section", { class: "scheda-ui" }, el("h2", { testo: "Movimenti" }),
        f.registro.length ? el("ul", { class: "fin-registro" }, f.registro.map((r) => el("li", {},
          el("span", { class: "tenue", testo: quando(r.quando) + " · " }), el("b", { class: r.importo > 0 ? "fin-piu" : r.importo < 0 ? "fin-meno" : "", testo: (r.importo > 0 ? "+" : r.importo < 0 ? "−" : "") + (r.importo ? moStr(Math.abs(r.importo)) : "") + " " }),
          r.nome + ": " + r.testo)))
          : el("p", { class: "tenue", testo: "Nessun movimento." })));
  }

  function daiTogli(p) {
    const v = el("input", { type: "number", step: "0.01", placeholder: "± mo", class: "corto", "aria-label": "Monete da dare (+) o togliere (−) a " + p.nome });
    const m = el("input", { maxlength: "120", placeholder: "Motivo", class: "corto", "aria-label": "Motivo" });
    return el("span", { class: "riga-flex" }, v, m, el("button", { class: "piccolo", testo: "Applica", on: { click: async () => {
      const rame = Math.round((parseFloat(String(v.value).replace(",", ".")) || 0) * 100);
      if (!rame) return;
      try { await api("/finanze/" + encodeURIComponent(p.id) + "/master", { dati: { rame, motivo: m.value } }); disegna(); } catch (e) { avviso(e.message, "errore"); }
    } } }));
  }

  await disegna();
  const vie = [tempoReale.su("finanze.movimento", () => disegna()), tempoReale.su("finanze.richiesta", () => disegna()), tempoReale.su("finanze.decisione", () => disegna())];
  return () => vie.forEach((f) => f());
}

// ===== Valute del gioco (fase 3c, C4d, internal/server/valute.go) =====
// Il denaro sta nei campi «valute» del modello della scheda libera. Master: dà o toglie; tutti: trasferire ad altri
// personaggi con la stessa valuta; registro dei movimenti.
async function mostraValute(corpo, io) {
  const master = io.utente.ruolo === "master";
  const box = el("div", { class: "largo" });
  svuota(corpo, box);
  async function disegna() {
    let f;
    try { f = await api("/valute"); } catch (e) { svuota(box, el("p", { class: "errore", testo: e.message })); return; }
    svuota(box,
      el("section", { class: "scheda-ui" }, el("h2", { testo: master ? "Denaro dei personaggi" : "Il tuo denaro" }),
        f.personaggi.length ? el("div", { class: "griglia" }, f.personaggi.map((p) => el("div", { class: "fin-pg", "data-pg": p.id },
          el("p", {}, el("b", { testo: p.nome }), p.giocatore && master ? el("span", { class: "tenue", testo: " · " + p.giocatore }) : null),
          p.valute.map((v) => el("p", { class: "fin-tot", "data-valuta": v.id, testo: v.valore + " " + v.nome })),
          el("div", { class: "riga-flex" },
            el("button", { class: "piccolo", testo: "💸 Trasferisci", on: { click: () => trasferisci(p) } }),
            master ? daiTogli(p) : null))))
          : el("p", { class: "tenue", testo: "Nessun personaggio con una valuta. Le valute si scelgono nel modello della scheda della campagna (Campagne → «Modello della scheda»)." }),
        el("p", { class: "tenue piccolo", testo: "Il denaro è un campo della scheda: si cambia anche dalla scheda (con l'approvazione del Master, se è attiva)." })),
      el("section", { class: "scheda-ui" }, el("h2", { testo: "Movimenti" }),
        f.registro.length ? el("ul", { class: "fin-registro" }, f.registro.map((r) => el("li", {},
          el("span", { class: "tenue", testo: quando(r.quando) + " · " }), el("span", { testo: r.nome + ": " + r.testo }))))
          : el("p", { class: "tenue", testo: "Nessun movimento." })));
  }
  function sceltaValuta(p) {
    const s = el("select", { "aria-label": "Valuta" }, p.valute.map((v) => el("option", { value: v.id, testo: v.nome })));
    if (p.valute.length === 1) s.hidden = true;
    return s;
  }
  function daiTogli(p) {
    const v = el("input", { type: "number", step: "1", placeholder: "±", class: "corto", "aria-label": "Importo da dare (+) o togliere (−) a " + p.nome });
    const val = sceltaValuta(p);
    const m = el("input", { maxlength: "120", placeholder: "Motivo", class: "corto", "aria-label": "Motivo" });
    return el("span", { class: "riga-flex" }, v, val, m, el("button", { class: "piccolo", testo: "Applica", on: { click: async () => {
      const importo = parseInt(v.value, 10) || 0;
      if (!importo) return;
      try { await api("/valute/" + encodeURIComponent(p.id) + "/master", { dati: { valuta: val.value, importo, motivo: m.value } }); disegna(); } catch (e) { avviso(e.message, "errore"); }
    } } }));
  }
  async function trasferisci(p) {
    let dest = [];
    try { dest = await api("/finanze/" + encodeURIComponent(p.id) + "/destinatari"); } catch (e) { avviso(e.message, "errore"); return; }
    const a = el("select", { "aria-label": "A chi" }, dest.map((d) => el("option", { value: d.id, testo: d.nome + (d.giocatore ? " (" + d.giocatore + ")" : "") })));
    const val = sceltaValuta(p);
    const q = el("input", { type: "number", min: 1, step: 1, valore: 1, class: "corto", "aria-label": "Quanto" });
    dialogo("💸 Trasferisci da " + p.nome, el("div", { class: "riga-flex" }, a, q, val), { azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { type: "button", testo: "Annulla", on: { click: chiudi } }),
      el("button", { type: "button", class: "primario", testo: "Trasferisci", on: { click: async () => {
        try { const r = await api("/valute/" + encodeURIComponent(p.id) + "/trasferisci", { dati: { a: a.value, valuta: val.value, importo: parseInt(q.value, 10) || 0 } }); chiudi(); avviso(r.testo); disegna(); }
        catch (e) { avviso(e.message, "errore"); }
      } } })) });
  }
  await disegna();
  const via = tempoReale.su("finanze.movimento", () => disegna());
  return () => via();
}

