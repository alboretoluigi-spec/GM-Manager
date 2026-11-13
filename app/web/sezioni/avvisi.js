// Sezione "Avvisi" (solo Master): assistente per avvisare i partecipanti su WhatsApp.
// Il server prepara il testo personalizzato e il link wa.me; il Master apre WhatsApp, invia, e segna
// "inviato" o "saltato". Nessun invio automatico. Server: internal/server/avvisi.go.
import { el, svuota, api, avviso, dataOra, quando, sessioneScelta, ricordaSessione } from "../ui/ui.js";

const SEGNAPOSTO = ["{nome}", "{sessione}", "{data}", "{ora}", "{luogo}", "{note}", "{gruppo}"];

export async function mostra(corpo, { azioni }) {
  const contenitore = el("div", { class: "largo" });
  svuota(corpo, contenitore);
  let sessioni = [], modelli = [];
  try {
    const gruppi = await api("/gruppi");
    for (const g of gruppi) (await api("/gruppi/" + g.id + "/sessioni")).filter((s) => s.stato !== "conclusa").forEach((s) => sessioni.push({ ...s, nomeGruppo: g.nome }));
    modelli = await api("/modelli-avviso");
  } catch (e) { svuota(contenitore, el("p", { class: "errore", testo: e.message })); return; }
  if (!sessioni.length) {
    svuota(contenitore, el("section", { class: "scheda-ui" }, el("h2", { testo: "Nessuna sessione in programma" }),
      el("p", { class: "tenue", testo: "Crea una sessione nella sezione Sessioni: qui potrai avvisare i partecipanti." })));
    return;
  }
  sessioni.sort((a, b) => a.inizio - b.inizio);
  let sessione = sessioneScelta(sessioni); // ?s=… oppure l'ultima scelta (ui.js)
  const scelta = el("select", { "aria-label": "Sessione", on: { change: () => { sessione = sessioni.find((s) => s.id === scelta.value); history.replaceState(null, "", "/avvisi?s=" + sessione.id); ricordaSessione(sessione.id); disegna(); } } },
    sessioni.map((s) => el("option", { value: s.id, testo: s.nomeGruppo + " · " + s.titolo, selected: s.id === sessione.id })));
  svuota(azioni, scelta);

  const testo = el("textarea", { maxlength: "1000", rows: "4", valore: modelli[0].testo });
  const selModello = el("select", { "aria-label": "Modello", on: { change: () => { testo.value = modelli.find((m) => m.id === selModello.value).testo; anteprima(); } } },
    modelli.map((m) => el("option", { value: m.id, testo: m.nome })));
  const zonaAnteprima = el("div");
  const zonaInvii = el("div");
  let t = 0;
  testo.addEventListener("input", () => { clearTimeout(t); t = setTimeout(anteprima, 400); });

  async function anteprima() {
    try {
      const r = await api("/sessioni/" + sessione.id + "/avvisi/anteprima", { dati: { testo: testo.value } });
      const ok = r.destinatari.filter((d) => !d.motivo), no = r.destinatari.filter((d) => d.motivo);
      svuota(zonaAnteprima,
        ok.length ? el("p", { class: "tenue piccolo", testo: "Anteprima per " + ok[0].nome + ":" }) : null,
        ok.length ? el("blockquote", { class: "anteprima-msg", testo: ok[0].testo }) : null,
        el("p", { testo: "Destinatari: " + (ok.map((d) => d.nome + " (" + d.telefono + ")").join(", ") || "nessuno") }),
        no.length ? el("p", { class: "tenue", testo: "Non riceveranno il messaggio: " + no.map((d) => d.nome + " (" + d.motivo + ")").join(", ") }) : null,
        el("div", { class: "riga-flex" },
          el("button", { class: "primario", testo: "📣 Prepara l'invio", disabled: !ok.length, on: { click: prepara } }),
          el("a", { class: "pulsante", href: r.gruppo, target: "_blank", rel: "noopener", testo: "Invia nel gruppo WhatsApp" }),
          el("button", { testo: "📋 Copia messaggio", on: { click: () => navigator.clipboard.writeText(ok.length ? ok[0].testo : testo.value).then(() => avviso("Messaggio copiato."), () => avviso("Copia non riuscita.", "errore")) } })));
    } catch (e) { svuota(zonaAnteprima, el("p", { class: "errore", testo: e.message })); }
  }

  async function prepara() {
    try { await api("/sessioni/" + sessione.id + "/avvisi", { dati: { testo: testo.value } }); avviso("Pronto: apri WhatsApp per ogni giocatore."); invii(); }
    catch (e) { avviso(e.message, "errore"); }
  }

  async function invii() {
    let l = [];
    try { l = await api("/sessioni/" + sessione.id + "/avvisi"); } catch (e) { avviso(e.message, "errore"); }
    svuota(zonaInvii, l.length ? l.map((a, i) => el("section", { class: "scheda-ui", stile: { marginTop: "12px" } },
      el("h3", { testo: (i === 0 ? "Ultimo avviso" : "Avviso") + " · " + quando(a.quando) }),
      el("table", { class: "tabella adattiva" }, el("tbody", {}, (a.destinatari || []).map((d) => el("tr", {},
        el("td", { "data-etichetta": "Giocatore", testo: d.nome + " " + d.telefono }),
        el("td", { "data-etichetta": "Stato" }, el("span", { class: "etichetta " + (d.stato === "inviato" ? "approvato" : d.stato === "saltato" ? "" : "attesa"), testo: { da_inviare: "da inviare", inviato: "✓ inviato", saltato: "saltato" }[d.stato] })),
        el("td", {}, el("div", { class: "riga-flex" },
          d.link ? el("a", { class: "pulsante primario piccolo", href: d.link, target: "_blank", rel: "noopener", testo: "Apri WhatsApp", on: { click: () => setTimeout(() => chiediEsito(a.id, d), 800) } }) : null,
          d.stato !== "inviato" ? el("button", { class: "piccolo", testo: "Segna inviato", on: { click: () => segna(a.id, d.utente, "inviato") } }) : null,
          d.stato === "da_inviare" ? el("button", { class: "piccolo", testo: "Salta", on: { click: () => segna(a.id, d.utente, "saltato") } }) : null)))))))) : null);
  }
  function chiediEsito(id, d) {
    // Al ritorno da WhatsApp: "Segnato inviato?"
    const b = el("div", { class: "banner" }, "Hai inviato il messaggio a " + d.nome + "? ",
      el("button", { class: "primario piccolo", testo: "Sì, segna inviato", on: { click: () => { b.remove(); segna(id, d.utente, "inviato"); } } }),
      el("button", { class: "piccolo", testo: "No", on: { click: () => b.remove() } }));
    zonaInvii.prepend(b);
  }
  async function segna(id, u, stato) {
    try { await api("/avvisi/" + id + "/destinatari/" + u, { metodo: "PATCH", dati: { stato } }); invii(); } catch (e) { avviso(e.message, "errore"); }
  }

  async function salvaModello() {
    const nome = prompt("Nome del modello:", selModello.selectedOptions[0] ? selModello.selectedOptions[0].textContent : "Nuovo modello");
    if (!nome) return;
    const altri = modelli.filter((m) => m.id !== "predefinito" && m.nome !== nome).map((m) => ({ nome: m.nome, testo: m.testo }));
    try { modelli = await api("/modelli-avviso", { metodo: "PUT", dati: [...altri, { nome, testo: testo.value }] }); avviso("Modello salvato."); disegna(); }
    catch (e) { avviso(e.message, "errore"); }
  }

  function disegna() {
    svuota(contenitore,
      el("section", { class: "scheda-ui" },
        el("h2", { testo: sessione.titolo }),
        el("p", { class: "tenue", testo: sessione.nomeGruppo + " · " + dataOra(sessione.inizio) + (sessione.luogo ? " · " + sessione.luogo : "") }),
        el("div", { class: "riga-flex" }, el("label", {}, "Modello ", selModello), el("button", { class: "piccolo", testo: "Salva come modello…", on: { click: salvaModello } })),
        el("label", {}, "Messaggio", testo),
        el("p", { class: "tenue piccolo", testo: "Segnaposto: " + SEGNAPOSTO.join(" ") + ". Ogni giocatore riceve il messaggio con il suo nome. Chi ha disattivato gli avvisi nel profilo non riceve nulla." }),
        zonaAnteprima),
      zonaInvii);
    anteprima();
    invii();
  }
  disegna();
}
