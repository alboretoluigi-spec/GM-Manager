// Profilo: nome, telefono, avvisi WhatsApp, password, esportazione e cancellazione dei dati.
import { el, svuota, api, avviso, conferma } from "../ui/ui.js";
import { avvio } from "../ui/app.js";

export async function mostra(corpo, ctx) {
  const p = await api("/profilo");
  const master = ctx.io.utente.ruolo === "master";
  const nome = el("input", { valore: p.nome, maxlength: "40" });
  const tel = el("input", { type: "tel", valore: p.telefono, maxlength: "24" });
  const avvisi = el("input", { type: "checkbox", selezionato: p.avvisi });
  const salva = el("button", { class: "primario", testo: "Salva", on: { click: async () => {
    try {
      const d = { nome: nome.value, avvisi: avvisi.checked };
      if (!master) d.telefono = tel.value;
      await api("/profilo", { metodo: "PATCH", dati: d });
      avviso("Profilo salvato.");
    } catch (e) { avviso(e.message, "errore"); }
  } } });

  const att = el("input", { type: "password", autocomplete: "current-password" });
  const nuova = el("input", { type: "password", autocomplete: "new-password" });
  const conf = el("input", { type: "password", autocomplete: "new-password" });
  const cambia = el("button", { testo: "Cambia password", on: { click: async () => {
    try { await api("/password/cambia", { dati: { attuale: att.value, nuova: nuova.value, conferma: conf.value } }); avviso("Password cambiata. Le altre sessioni sono state chiuse."); att.value = nuova.value = conf.value = ""; }
    catch (e) { avviso(e.message, "errore"); }
  } } });

  const scarica = el("a", { class: "pulsante", href: "/api/v1/profilo/esporta", download: "gasterbless-miei-dati.json", testo: "⬇ Scarica i miei dati" });
  const cancella = el("button", { class: "pericolo", testo: p.cancellazioneRichiesta ? "Cancellazione già richiesta" : "Chiedi la cancellazione dell'account", disabled: p.cancellazioneRichiesta,
    on: { click: async () => {
      if (!(await conferma("Cancellare l'account?", "La richiesta arriva al Master, che la conferma. Verranno eliminati account, numero e background; le schede saranno eliminate o rese anonime.", "Invia la richiesta", true))) return;
      try { await api("/profilo/cancellazione", { dati: {} }); avviso("Richiesta inviata al Master."); avvio(); } catch (e) { avviso(e.message, "errore"); }
    } } });

  svuota(corpo, el("div", { class: "largo", stile: { maxWidth: "720px" } },
    el("section", { class: "scheda-ui" }, el("h2", { testo: "I tuoi dati" }),
      el("label", {}, "Nome", nome),
      master ? null : el("label", {}, "Numero di telefono (formato internazionale)", tel),
      master ? null : el("label", { class: "riga" }, avvisi, el("span", { testo: "Ricevi avvisi WhatsApp delle sessioni" })),
      salva),
    el("section", { class: "scheda-ui" }, el("h2", { testo: "Password" }),
      el("label", {}, "Password attuale", att), el("label", {}, "Nuova password (almeno 10 caratteri)", nuova), el("label", {}, "Ripeti la nuova password", conf), cambia),
    master ? null : el("section", { class: "scheda-ui" }, el("h2", { testo: "I tuoi diritti" }),
      el("p", { class: "tenue", testo: "Puoi scaricare tutti i tuoi dati (account, consensi, gruppi e schede) in un file JSON, oppure chiedere la cancellazione." }),
      el("div", { class: "riga-flex" }, scarica, cancella),
      el("p", { class: "tenue", stile: { marginTop: "10px" } }, el("a", { href: "/privacy", "data-interno": true, testo: "Leggi l'informativa sulla privacy" })))));
}
