// Sezione "Sessioni": gruppi e sessioni di gioco.
// Il Master crea e modifica gruppi e sessioni e sceglie i partecipanti; il giocatore vede le sessioni
// dei propri gruppi e conferma la propria presenza.
import { el, svuota, api, avviso, dialogo, conferma, dataOra, tempoReale, carica } from "../ui/ui.js";

const GIORNI = ["", "Lunedì", "Martedì", "Mercoledì", "Giovedì", "Venerdì", "Sabato", "Domenica"];
const STATI = { pianificata: "pianificata", in_corso: "in corso", conclusa: "conclusa" };

export async function mostra(corpo, { io, azioni }) {
  const master = io.utente.ruolo === "master";
  const contenitore = el("div", { class: "largo" });
  svuota(corpo, contenitore);
  if (master) svuota(azioni, el("button", { class: "primario piccolo", testo: "＋ Nuovo gruppo", on: { click: () => modificaGruppo(null) } }));

  let utenti = []; // solo per il Master: serve per scegliere i partecipanti

  async function disegna() {
    let gruppi;
    try {
      [gruppi, utenti] = await Promise.all([api("/gruppi"), master ? api("/utenti") : Promise.resolve([])]);
    } catch (e) {
      svuota(contenitore, el("p", { class: "errore", testo: e.message }));
      return;
    }
    if (!gruppi.length) {
      svuota(contenitore, el("section", { class: "scheda-ui" },
        el("h2", { testo: master ? "Nessun gruppo ancora" : "Non fai ancora parte di un gruppo" }),
        el("p", { class: "tenue", testo: master
          ? "Crea il primo gruppo (per esempio «Martedì») con il pulsante «＋ Nuovo gruppo». Poi potrai approvare i giocatori e pianificare le sessioni."
          : "Il Master ti aggiungerà a un gruppo: appena lo farà, qui vedrai le sessioni in programma." })));
      return;
    }
    let promemoria = [];
    if (master) { try { promemoria = await api("/promemoria"); } catch { /* facoltativo */ } }
    const sezioni = await Promise.all(gruppi.map(async (g) => {
      let sessioni = [];
      try { sessioni = await api("/gruppi/" + g.id + "/sessioni"); } catch (e) { avviso(e.message, "errore"); }
      return blocco(g, sessioni);
    }));
    svuota(contenitore, promemoria.map((p) => el("div", { class: "banner" }, "Domani c'è «" + p.titolo + "»: vuoi mandare il promemoria? ",
      el("a", { href: "/avvisi?s=" + encodeURIComponent(p.id), "data-interno": true, class: "pulsante primario piccolo", testo: "Apri assistente" }))), sezioni);
  }

  function blocco(g, sessioni) {
    const future = sessioni.filter((s) => s.stato !== "conclusa").sort((a, b) => a.inizio - b.inizio);
    const passate = sessioni.filter((s) => s.stato === "conclusa");
    return el("section", { class: "scheda-ui", stile: g.colore ? { borderLeft: "6px solid " + g.colore } : {} },
      el("div", { class: "riga-flex" },
        g.immagine ? el("img", { class: "gruppo-immagine", src: "/api/v1/gruppi/" + encodeURIComponent(g.id) + "/immagine?v=" + encodeURIComponent(g.immagine), alt: "Immagine del gruppo " + g.nome, loading: "lazy" }) : null,
        el("h2", { testo: g.nome, stile: { margin: 0 } }),
        g.giorno ? el("span", { class: "etichetta", testo: g.giorno }) : null,
        el("span", { class: "tenue", testo: g.giocatori + (g.giocatori === 1 ? " giocatore" : " giocatori") }),
        g.segretario ? el("span", { class: "etichetta approvato", testo: "Sei Segretario" }) : null,
        el("span", { class: "spazio" }),
        master ? el("button", { class: "piccolo primario", testo: "＋ Sessione", on: { click: () => modificaSessione(g, null) } }) : null,
        master ? el("button", { class: "piccolo", testo: "Modifica gruppo", on: { click: () => modificaGruppo(g) } }) : null),
      future.length ? future.map((s) => rigaSessione(g, s)) : el("p", { class: "tenue", stile: { marginTop: "10px" }, testo: "Nessuna sessione in programma." }),
      passate.length ? el("details", { stile: { marginTop: "10px" } },
        el("summary", { testo: "Sessioni concluse (" + passate.length + ")" }),
        passate.map((s) => rigaSessione(g, s))) : null);
  }

  function rigaSessione(g, s) {
    const confermati = s.partecipanti.filter((p) => p.confermato).length;
    const io_ = s.partecipanti.find((p) => p.utente === io.utente.id);
    return el("div", { class: "sessione", stile: { borderTop: "1px solid var(--bordo)", marginTop: "10px", paddingTop: "10px" } },
      el("div", { class: "riga-flex" },
        el("strong", { testo: s.titolo }),
        el("span", { class: "etichetta" + (s.stato === "in_corso" ? " approvato" : ""), testo: STATI[s.stato] || s.stato })),
      el("div", { class: "tenue", testo: dataOra(s.inizio) + (s.luogo ? " · " + s.luogo : "") }),
      s.note ? el("p", { testo: s.note, stile: { whiteSpace: "pre-wrap", margin: "6px 0" } }) : null,
      el("div", { class: "tenue piccolo", testo: "Partecipanti: " + (s.partecipanti.length
        ? s.partecipanti.map((p) => p.nome + (p.nomePersonaggio ? " (" + p.nomePersonaggio + ")" : "") + (p.confermato ? " ✓" : " ?")).join(" · ")
        : "nessuno") + (s.partecipanti.length ? "  —  confermati " + confermati + " su " + s.partecipanti.length : "") }),
      el("div", { class: "riga-flex", stile: { marginTop: "6px" } },
        !master && io_ && s.stato !== "conclusa" ? el("button", {
          class: "piccolo" + (io_.confermato ? "" : " primario"),
          testo: io_.confermato ? "Non posso più venire" : "✓ Ci sarò",
          on: { click: () => io_.confermato
            ? azione(() => api("/sessioni/" + s.id + "/conferma", { dati: { confermato: false, personaggio: io_.personaggio || "" } }), "Presenza annullata.")
            : confermaConPersonaggio(s, io_) },
        }) : null,
        s.stato !== "conclusa" && (master || io_) ? el("a", { href: "/iniziativa?s=" + encodeURIComponent(s.id), "data-interno": true, class: "pulsante piccolo", testo: "⚔ Iniziativa" }) : null,
        !master && s.stato !== "conclusa" ? el("a", { href: "/api/v1/sessioni/" + encodeURIComponent(s.id) + "/calendario.ics", class: "pulsante piccolo", title: "Aggiungi al calendario", testo: "📅 Calendario" }) : null,
        master && s.stato !== "conclusa" ? el("a", { href: "/preparazione?gruppo=" + encodeURIComponent(g.id) + "&sessione=" + encodeURIComponent(s.id), "data-interno": true, class: "pulsante piccolo", title: "Promemoria, schede del Codice, PNG e note per questa sessione", testo: "🧭 Prepara" }) : null,
        master && s.stato === "pianificata" ? el("a", { href: "/avvisi?s=" + encodeURIComponent(s.id), "data-interno": true, class: "pulsante piccolo", testo: "📣 Avvisa" }) : null,
        master && s.stato === "pianificata" ? el("button", { class: "piccolo primario", testo: "▶ Avvia", on: { click: () => cambiaStato(s, "in_corso", "Sessione avviata.") } }) : null,
        master && s.stato === "in_corso" ? el("button", { class: "piccolo", testo: "■ Concludi", on: { click: () => cambiaStato(s, "conclusa", "Sessione conclusa.") } }) : null,
        master && s.stato === "conclusa" ? el("button", { class: "piccolo", testo: "Riapri", on: { click: () => cambiaStato(s, "pianificata", "Sessione riaperta.") } }) : null,
        master ? el("button", { class: "piccolo", testo: "👋 Saluta il gruppo", title: "Apre su tutti gli schermi l'immagine di saluto: «Alla prossima sessione...»", on: { click: () => saluta(s) } }) : null,
        // Gestione (2026-11-08, «ordine e pulizia»): a destra, più discreta, separata dalle azioni di gioco.
        master ? el("span", { class: "azioni-gestione" },
          s.stato !== "conclusa" ? el("a", { href: "/api/v1/sessioni/" + encodeURIComponent(s.id) + "/calendario.ics", class: "pulsante piccolo", title: "Aggiungi al calendario", testo: "📅 Calendario" }) : null,
          s.stato !== "conclusa" ? el("button", { class: "piccolo" + (s.mqttRemoto ? " primario" : ""), testo: "☁ Remoto: " + (s.mqttRemoto ? "acceso" : "spento"), on: { click: () => giocoRemoto(s) } }) : null,
          el("button", { class: "piccolo", testo: "Partecipanti…", on: { click: () => partecipanti(g, s) } }),
          el("button", { class: "piccolo", testo: "Modifica…", on: { click: () => modificaSessione(g, s) } }),
          el("button", { class: "piccolo pericolo", testo: "Elimina", on: { click: () => eliminaSessione(s) } })) : null));
  }

  async function azione(fn, ok) {
    try { await fn(); if (ok) avviso(ok); await disegna(); } catch (e) { avviso(e.message, "errore"); }
  }

  // Conferma della presenza: si sceglie anche il personaggio (serve al tracker dell'iniziativa).
  async function confermaConPersonaggio(s, io_) {
    let pg = [];
    try { pg = (await api("/personaggi")).filter((p) => p.stato === "attivo"); } catch { /* si conferma senza PG */ }
    const invia = (id) => azione(() => api("/sessioni/" + s.id + "/conferma", { dati: { confermato: true, personaggio: id } }), "Presenza confermata.");
    if (pg.length <= 1) return invia(pg.length ? pg[0].id : io_.personaggio || "");
    const sel = el("select", {}, pg.map((p) => el("option", { value: p.id, testo: p.nome, selected: p.id === io_.personaggio })));
    dialogo("Con quale personaggio?", el("label", {}, "Personaggio per «" + s.titolo + "»", sel),
      { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "✓ Ci sarò", on: { click: () => { chiudi(); invia(sel.value); } } })) });
  }

  // «Saluta il gruppo»: immagine di saluto su sfondo nero con «Alla prossima sessione...» su tutti gli schermi
  // della sessione (partecipanti e Master), saluto.go e web/ui/saluto.js.
  async function saluta(s) {
    try {
      const r = await tempoReale.chiedi("saluto.invia", { sessione: s.id });
      avviso(r.destinatari ? "Saluto inviato al gruppo." : "Saluto mostrato (la sessione non ha ancora partecipanti).");
    } catch (e) { avviso(e.message, "errore"); }
  }

  function cambiaStato(s, stato, ok) {
    azione(() => api("/sessioni/" + s.id, { metodo: "PATCH", dati: { stato } }), ok);
  }

  async function eliminaSessione(s) {
    if (!(await conferma("Eliminare la sessione?", "«" + s.titolo + "» verrà eliminata con l'elenco dei partecipanti. L'operazione non si può annullare.", "Elimina", true))) return;
    azione(() => api("/sessioni/" + s.id, { metodo: "DELETE" }), "Sessione eliminata.");
  }

  function modificaGruppo(g) {
    const nome = el("input", { maxlength: "60", valore: g ? g.nome : "", placeholder: "Es. Martedì sera", required: true });
    const giorno = el("select", {}, GIORNI.map((x) => el("option", { value: x, testo: x || "—", selected: g && g.giorno === x })));
    const colore = el("input", { type: "color", valore: (g && g.colore) || "#8a1c1c", stile: { width: "80px", padding: "2px" } });
    const err = el("p", { class: "errore" });
    // Immagine del gruppo: si sceglie qui e si carica al salvataggio; "Togli" la elimina.
    let togli = false;
    const file = el("input", { type: "file", accept: "image/jpeg,image/png,image/webp,image/gif", "aria-label": "Immagine del gruppo" });
    const anteprima = el("img", { class: "gruppo-immagine grande", alt: "", hidden: !(g && g.immagine) });
    if (g && g.immagine) anteprima.src = "/api/v1/gruppi/" + encodeURIComponent(g.id) + "/immagine?v=" + encodeURIComponent(g.immagine);
    const togliBtn = el("button", { class: "piccolo", testo: "Togli l'immagine", hidden: !(g && g.immagine), on: { click: () => { togli = true; file.value = ""; anteprima.hidden = true; togliBtn.hidden = true; } } });
    file.addEventListener("change", () => {
      const f = file.files[0];
      if (!f) return;
      if (f.size > 10 * 1048576) { err.textContent = "Immagine troppo grande: massimo 10 MB."; file.value = ""; return; }
      togli = false; anteprima.src = URL.createObjectURL(f); anteprima.hidden = false; togliBtn.hidden = false;
    });
    dialogo(g ? "Modifica gruppo" : "Nuovo gruppo", el("div", {},
      el("label", {}, "Nome del gruppo", nome), el("label", {}, "Giorno abituale", giorno), el("label", {}, "Colore", colore),
      el("label", {}, "Immagine del gruppo (facoltativa, compare nell'elenco delle sessioni)", file),
      el("div", { class: "riga-flex" }, anteprima, togliBtn), err),
    { azioni: (chiudi) => el("div", { class: "azioni" },
      g ? el("button", { class: "pericolo", testo: "Archivia gruppo", on: { click: async () => {
        if (!(await conferma("Archiviare il gruppo?", "Il gruppo «" + g.nome + "» sparirà dagli elenchi. I dati restano nel database e nei backup.", "Archivia", true))) return;
        chiudi(); azione(() => api("/gruppi/" + g.id, { metodo: "DELETE" }), "Gruppo archiviato.");
      } } }) : null,
      el("span", { class: "spazio" }),
      el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Salva", on: { click: async () => {
        const dati = { nome: nome.value.trim(), giorno: giorno.value, colore: colore.value };
        try {
          const r = await (g ? api("/gruppi/" + g.id, { metodo: "PATCH", dati }) : api("/gruppi", { dati }));
          const id = g ? g.id : r.id;
          if (file.files[0]) await carica("/gruppi/" + encodeURIComponent(id) + "/immagine", file.files[0], null, "PUT");
          else if (togli && g && g.immagine) await api("/gruppi/" + encodeURIComponent(id) + "/immagine", { metodo: "DELETE" });
          chiudi(); avviso(g ? "Gruppo aggiornato." : "Gruppo creato."); disegna();
        } catch (e) { err.textContent = e.message; }
      } } })) });
  }

  function modificaSessione(g, s) {
    const titolo = el("input", { maxlength: "80", valore: s ? s.titolo : "", placeholder: "Es. La Tana del Lupo" });
    const quando = el("input", { type: "datetime-local", valore: valoreDataLocale(s ? s.inizio : prossimaSerata(g.giorno)) });
    const luogo = el("input", { maxlength: "120", valore: s ? s.luogo : "", placeholder: "Es. Casa del Master" });
    const note = el("textarea", { maxlength: "1000", valore: s ? s.note : "" });
    const err = el("p", { class: "errore" });
    dialogo(s ? "Modifica sessione" : "Nuova sessione · " + g.nome, el("div", {},
      el("label", {}, "Titolo", titolo), el("label", {}, "Data e ora", quando), el("label", {}, "Luogo", luogo),
      el("label", {}, "Note per i giocatori", note),
      s ? null : el("p", { class: "tenue piccolo", testo: "Tutti i giocatori approvati del gruppo vengono aggiunti come partecipanti: potrai togliere chi non c'è." }),
      err),
    { azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Salva", on: { click: async () => {
        const inizio = Math.floor(new Date(quando.value).getTime() / 1000);
        if (!quando.value || !Number.isFinite(inizio)) { err.textContent = "Indica data e ora."; return; }
        const dati = { titolo: titolo.value.trim(), inizio, luogo: luogo.value.trim(), note: note.value };
        try {
          await (s ? api("/sessioni/" + s.id, { metodo: "PATCH", dati }) : api("/gruppi/" + g.id + "/sessioni", { dati }));
          chiudi(); avviso(s ? "Sessione aggiornata." : "Sessione creata."); disegna();
        } catch (e) { err.textContent = e.message; }
      } } })) });
  }

  function partecipanti(g, s) {
    const membri = utenti.filter((u) => u.stato === "approvato" && u.ruolo !== "master" && (u.gruppi || []).includes(g.id));
    const presenti = new Map(s.partecipanti.map((p) => [p.utente, p]));
    const spunte = membri.map((u) => ({ u, c: el("input", { type: "checkbox", selezionato: presenti.has(u.id) }) }));
    dialogo("Partecipanti · " + s.titolo, membri.length ? el("div", {},
      spunte.map(({ u, c }) => el("label", { class: "riga" }, c, el("span", { testo: u.nome }),
        presenti.get(u.id) && presenti.get(u.id).confermato ? el("span", { class: "etichetta approvato", testo: "ha confermato" }) : null)),
      el("p", { class: "tenue piccolo", testo: "Il personaggio con cui partecipa lo sceglie il giocatore quando conferma la presenza." })) :
      el("p", { class: "tenue", testo: "Nel gruppo non ci sono ancora giocatori approvati. Approvali dalla sezione Iscrizioni." }),
    { azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { testo: "Annulla", on: { click: chiudi } }),
      membri.length ? el("button", { class: "primario", testo: "Salva", on: { click: () => {
        const dati = spunte.filter((x) => x.c.checked).map(({ u }) => ({ utente: u.id, personaggio: (presenti.get(u.id) || {}).personaggio || "" }));
        chiudi(); azione(() => api("/sessioni/" + s.id + "/partecipanti", { metodo: "PUT", dati }), "Partecipanti aggiornati.");
      } } }) : null) });
  }

  // Gioco da remoto (server MQTT pubblici): acceso/spento e link personali con QR.
  async function giocoRemoto(s) {
    const zona = el("div");
    let chiudiDialogo = null;
    async function aggiorna() {
      let r;
      try { r = await api("/sessioni/" + s.id + "/remoto"); } catch (e) { svuota(zona, el("p", { class: "errore", testo: e.message })); return; }
      svuota(zona,
        el("p", { testo: "Per chi gioca lontano da casa. Il PC del Master invia ai server MQTT pubblici solo il tracker dell'iniziativa che vedono i giocatori, cifrato: niente mostri nascosti, messaggi, schede o telefoni. Ogni giocatore ha un link personale che può agire solo sul suo PG." }),
        el("p", { class: "tenue piccolo", testo: "Server usati: " + r.server.join(", ") + ". Quando spegni o concludi la sessione, il PC cancella dai server ciò che vi aveva lasciato e butta le chiavi." }),
        el("div", { class: "riga-flex" },
          el("strong", { testo: r.attivo ? "Acceso" + (r.collegato ? " · collegato" : " · collegamento in corso…") : "Spento" }),
          el("button", { class: r.attivo ? "pericolo piccolo" : "primario piccolo", testo: r.attivo ? "Spegni" : "Accendi", on: { click: async () => {
            if (r.attivo && !(await conferma("Spegnere il gioco da remoto?", "I link dei giocatori smetteranno di funzionare. Se lo riaccendi dovrai mandare link nuovi.", "Spegni", true))) return;
            try { await api("/sessioni/" + s.id, { metodo: "PATCH", dati: { mqttRemoto: !r.attivo } }); s.mqttRemoto = !r.attivo; aggiorna(); ridisegna(); }
            catch (e) { avviso(e.message, "errore"); }
          } } })),
        r.attivo ? el("table", { class: "tabella adattiva", stile: { marginTop: "10px" } }, el("tbody", {}, r.giocatori.map((g) => el("tr", {},
          el("td", { "data-etichetta": "Giocatore", testo: g.nome }),
          el("td", { "data-etichetta": "Link", testo: g.link ? "attivo" : "—" }),
          el("td", {}, el("div", { class: "riga-flex" },
            el("button", { class: "piccolo" + (g.link ? "" : " primario"), testo: g.link ? "Nuovo link" : "Crea link", on: { click: () => creaLink(g) } }),
            g.link ? el("button", { class: "piccolo pericolo", testo: "Revoca", on: { click: async () => {
              try { await api("/sessioni/" + s.id + "/remoto/giocatori/" + g.utente, { metodo: "DELETE" }); avviso("Link revocato."); aggiorna(); }
              catch (e) { avviso(e.message, "errore"); }
            } } }) : null)))))) : null,
        r.attivo && !r.giocatori.length ? el("p", { class: "tenue", testo: "Nessun partecipante: aggiungili con «Partecipanti…»." }) : null);
    }
    async function creaLink(g) {
      if (g.link && !(await conferma("Creare un nuovo link?", "Il link che " + g.nome + " ha adesso smetterà di funzionare.", "Nuovo link"))) return;
      let r;
      try { r = await api("/sessioni/" + s.id + "/remoto/giocatori", { dati: { utente: g.utente } }); } catch (e) { avviso(e.message, "errore"); return; }
      aggiorna();
      const campo = el("input", { valore: r.link, readonly: true, "aria-label": "Link personale" });
      dialogo("Link personale di " + g.nome, el("div", {},
        el("p", { testo: "Il modo migliore: " + g.nome + " inquadra il QR quando è a casa tua, così la pagina resta sul telefono e funziona anche da lontano. Altrimenti manda il file." }),
        r.qr ? el("img", { src: r.qr, alt: "QR del link personale", stile: { width: "260px", maxWidth: "100%", background: "#fff", display: "block", margin: "8px auto" } }) : null,
        el("label", {}, "Link", campo),
        el("p", { class: "attenzione piccolo", testo: "È personale: chi lo ha può agire sul PG di " + g.nome + ": mandalo solo a questa persona. Il link si vede solo ora." }),
        el("div", { class: "riga-flex" },
          el("button", { class: "piccolo", testo: "📋 Copia link", on: { click: () => navigator.clipboard.writeText(r.link).then(() => avviso("Link copiato."), () => avviso("Copia non riuscita.", "errore")) } }),
          el("button", { class: "piccolo", testo: "⬇ Scarica come file", on: { click: () => scaricaPagina(r.configurazione, g.nome) } }))), { largo: true });
    }
    chiudiDialogo = dialogo("☁ Gioco da remoto · " + s.titolo, zona, { largo: true });
    await aggiorna();
    return chiudiDialogo;
  }

  // Pagina remota in un solo file HTML (con la configurazione del giocatore dentro), da mandare a chi
  // non può passare da casa. Si apre con il browser del telefono.
  async function scaricaPagina(conf, nome) {
    try {
      const [js, css] = await Promise.all(["/remoto/remoto.js", "/remoto/remoto.css"].map((u) => fetch(u).then((x) => { if (!x.ok) throw new Error(); return x.text(); })));
      const sicuro = (t) => t.replace(/<\/(script|style)/gi, "<\\/$1");
      const html = "<!doctype html><html lang=\"it\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">" +
        "<meta name=\"referrer\" content=\"no-referrer\"><title>Gaster Bless da remoto</title><style>" + sicuro(css) + "</style></head><body><main id=\"app\"></main>" +
        "<script>window.GASTERBLESS_REMOTO=" + JSON.stringify(conf).replace(/</g, "\\u003c") + ";</script><script>" + sicuro(js) + "</script></body></html>";
      const a = el("a", { href: URL.createObjectURL(new Blob([html], { type: "text/html" })), download: "gasterbless-remoto-" + nome.replace(/[^\p{L}\p{N}]+/gu, "-").toLowerCase() + ".html" });
      document.body.append(a); a.click(); a.remove();
      setTimeout(() => URL.revokeObjectURL(a.href), 10000);
    } catch (e) { avviso("Non riesco a preparare il file.", "errore"); }
  }

  // Aggiornamenti in tempo reale: un'altra persona ha cambiato gruppi o sessioni.
  let attesa = 0;
  const ridisegna = () => { clearTimeout(attesa); attesa = setTimeout(disegna, 250); };
  const smetti = [tempoReale.su("sessioni.aggiornate", ridisegna), tempoReale.su("gruppi.aggiornati", ridisegna)];

  await disegna();
  return () => { clearTimeout(attesa); smetti.forEach((f) => f()); };
}

/** Valore per <input type="datetime-local"> nell'ora locale del dispositivo. */
function valoreDataLocale(sec) {
  const d = new Date(sec * 1000);
  const z = (n) => String(n).padStart(2, "0");
  return d.getFullYear() + "-" + z(d.getMonth() + 1) + "-" + z(d.getDate()) + "T" + z(d.getHours()) + ":" + z(d.getMinutes());
}

/** Proposta per una nuova sessione: il prossimo giorno abituale del gruppo (o domani) alle 21:00. */
function prossimaSerata(giorno) {
  const d = new Date();
  d.setHours(21, 0, 0, 0);
  const i = GIORNI.indexOf(giorno); // 1 = lunedì ... 7 = domenica
  if (i > 0) {
    const obiettivo = i % 7; // getDay(): 0 = domenica
    let delta = (obiettivo - d.getDay() + 7) % 7;
    if (delta === 0 && Date.now() > d.getTime()) delta = 7;
    d.setDate(d.getDate() + delta);
  } else {
    d.setDate(d.getDate() + 1);
  }
  return Math.floor(d.getTime() / 1000);
}
