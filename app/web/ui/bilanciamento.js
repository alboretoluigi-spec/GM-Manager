// Finestra «⚖ Bilanciamento» (solo Master; richiesta dell'utente del 2026-11-12). Si apre dal Codice della campagna.
// Si sceglie gruppo e sessione: i personaggi presenti vengono dalle loro schede (si possono togliere gli assenti), si
// sceglie la difficoltà (o quella consigliata) e Gaster Bless calcola la scheda A–E con il perché di ogni numero.
// «Salva nel Codice» la mette tra le schede (una per gruppo, aggiornata se si ricalcola); dopo il salvataggio si può
// chiedere all'IA il racconto (nomi e carattere dei bambini, indizi) senza cambiare i numeri.
// Server: internal/server/bilanciamento.go e bilanciamento_orfanotrofio.go.
import { el, svuota, api, avviso, dialogo } from "./ui.js";

const DIFF = ["Consigliata", "Media", "Difficile", "Molto difficile"];

export async function apriBilanciamento(modello, { gruppi = [], gruppo = "" } = {}) {
  let modelli = [];
  try { modelli = await api("/bilanciamento"); } catch (e) { avviso(e.message, "errore"); return; }
  const m = modelli.find((x) => x.id === modello) || modelli[0];
  if (!m) { avviso("Nessun modello di bilanciamento disponibile.", "errore"); return; }
  if (!gruppi.length) gruppi = await api("/gruppi").catch(() => []);
  if (!gruppi.length) { avviso("Crea prima un gruppo.", "errore"); return; }

  const selModello = el("select", { "aria-label": "Avventura" }, modelli.map((x) => el("option", { value: x.id, selected: x.id === m.id, testo: x.nome })));
  const selGruppo = el("select", { "aria-label": "Gruppo" }, gruppi.map((g) => el("option", { value: g.id, selected: g.id === gruppo, testo: g.nome })));
  const selSessione = el("select", { "aria-label": "Sessione" });
  const selDiff = el("select", { "aria-label": "Difficoltà" }, DIFF.map((d, i) => el("option", { value: String(i), testo: d })));
  const boxPG = el("div", { class: "bil-pg riga-flex" });
  const esito = el("div", { class: "bil-esito", "aria-live": "polite" });
  const salva = el("button", { class: "primario", testo: "💾 Salva nel Codice", disabled: true });
  const ia = el("a", { class: "pulsante", hidden: true, "data-interno": true, testo: "✨ Chiedi all'IA il racconto" });
  let ultimo = null;

  const richiesta = () => ({ gruppo: selGruppo.value, sessione: selSessione.value || "-", obiettivo: +selDiff.value,
    pg: [...boxPG.querySelectorAll("input:checked")].map((x) => x.value) });

  async function caricaGruppo(sessione) {
    let d;
    try { d = await api("/bilanciamento/gruppo?gruppo=" + encodeURIComponent(selGruppo.value) + (sessione ? "&sessione=" + encodeURIComponent(sessione) : "")); }
    catch (e) { svuota(boxPG, el("p", { class: "errore", testo: e.message })); return; }
    svuota(selSessione, el("option", { value: "", testo: "Nessuna sessione: personaggi attivi del gruppo" }),
      d.sessioni.map((x) => el("option", { value: x.id, selected: x.id === d.sessione, testo: x.titolo + (x.stato === "in_corso" ? " (in corso)" : "") + " · " + new Date(x.inizio * 1000).toLocaleDateString("it-IT") })));
    svuota(boxPG, d.pg.length ? d.pg.map((p) => el("label", { class: "riga bil-pg-voce" },
      el("input", { type: "checkbox", value: p.id, checked: p.presente, on: { change: calcola } }),
      el("span", { testo: p.nome + (p.classi || p.livello ? " (" + [p.classi, p.livello ? "liv. " + p.livello : ""].filter(Boolean).join(", ") + ")" : "") })))
      : el("p", { class: "tenue", testo: "Il gruppo non ha personaggi attivi." }));
    await calcola();
  }

  async function calcola() {
    ultimo = null; salva.disabled = true; ia.hidden = true;
    const r = richiesta();
    if (!r.pg.length) { svuota(esito, el("p", { class: "tenue", testo: "Spunta almeno un personaggio presente." })); return; }
    svuota(esito, el("p", { class: "tenue", testo: "Calcolo…" }));
    try { ultimo = await api("/bilanciamento/" + selModello.value, { metodo: "POST", dati: r }); }
    catch (e) { svuota(esito, el("p", { class: "errore", testo: e.message })); return; }
    salva.disabled = false;
    if (+selDiff.value === 0) selDiff.options[0].textContent = "Consigliata (" + DIFF[ultimo.consigliato] + ")";
    svuota(esito,
      el("p", { class: "bil-sintesi" }, "Difficoltà complessiva: ", el("b", { testo: ultimo.complessiva }),
        " · usata per i calcoli: " + DIFF[ultimo.obiettivo] + (ultimo.obiettivo !== ultimo.consigliato ? " (consigliata: " + DIFF[ultimo.consigliato] + ")" : " (consigliata)")),
      ultimo.sezioni.map((s, i) => el("details", { class: "scheda-ui bil-sezione", open: i === 0 || i === 4 },
        el("summary", {}, el("b", { testo: s.titolo })),
        el("dl", { class: "bil-righe" }, s.righe.map((x) => [
          el("dt", { testo: x.voce }),
          el("dd", {}, el("span", { testo: x.valore }), x.perche ? el("small", { class: "tenue bil-perche", testo: "Perché: " + x.perche }) : null),
        ])))));
  }

  salva.addEventListener("click", async () => {
    salva.disabled = true;
    try {
      const s = await api("/bilanciamento/" + selModello.value + "/salva", { metodo: "POST", dati: richiesta() });
      avviso("Salvata nel Codice: «" + s.nome + "».");
      document.dispatchEvent(new CustomEvent("gasterbless:codice-cambiato"));
      ia.href = "/assistente?gruppo=" + encodeURIComponent(selGruppo.value) + "&domanda=" + encodeURIComponent(s.domandaIA);
      ia.hidden = false;
    } catch (e) { avviso(e.message, "errore"); }
    salva.disabled = false;
  });

  selModello.addEventListener("change", calcola);
  selGruppo.addEventListener("change", () => caricaGruppo(""));
  selSessione.addEventListener("change", () => caricaGruppo(selSessione.value || "-"));
  selDiff.addEventListener("change", calcola);

  dialogo("⚖ Bilanciamento", el("div", { class: "bil" },
    el("p", { class: "tenue piccolo", testo: "I dati vengono dalle schede dei personaggi: togli chi è assente. Ogni numero ha il suo perché; il racconto (nomi, carattere, indizi) lo puoi chiedere all'IA dopo aver salvato." }),
    el("div", { class: "riga-flex bil-scelte" },
      el("label", { class: "riga" }, el("span", { class: "tenue piccolo", testo: "Avventura" }), selModello),
      el("label", { class: "riga" }, el("span", { class: "tenue piccolo", testo: "Gruppo" }), selGruppo),
      el("label", { class: "riga" }, el("span", { class: "tenue piccolo", testo: "Sessione" }), selSessione),
      el("label", { class: "riga" }, el("span", { class: "tenue piccolo", testo: "Difficoltà" }), selDiff)),
    boxPG, esito), {
    largo: true,
    azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Chiudi", on: { click: chiudi } }), ia, salva),
  });
  await caricaGruppo("");
}
