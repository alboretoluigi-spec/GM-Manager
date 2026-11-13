// Pannello del Master: iscrizioni in attesa, giocatori, inviti con QR, richieste privacy, registro accessi ai dati.
import { el, svuota, api, avviso, dialogo, conferma, quando } from "../ui/ui.js";

const STATI = { in_attesa: "in attesa", approvato: "approvato", rifiutato: "rifiutato", bloccato: "bloccato", sospeso: "sospeso", cancellazione_richiesta: "cancellazione richiesta" };

export async function mostra(corpo) {
  const contenitore = el("div", { class: "largo" });
  svuota(corpo, contenitore);
  async function disegna() {
    const [utenti, gruppi, inviti, richieste] = await Promise.all([api("/utenti"), api("/gruppi"), api("/inviti"), api("/richieste-privacy")]);
    const nomeGruppo = (id) => (gruppi.find((g) => g.id === id) || { nome: "?" }).nome;
    const attesa = utenti.filter((u) => u.stato === "in_attesa");
    const altri = utenti.filter((u) => u.stato !== "in_attesa");
    const aperte = richieste.filter((r) => r.stato === "aperta");

    const selGruppo = (proposto) => el("select", { "aria-label": "Gruppo" },
      gruppi.length ? gruppi.map((g) => el("option", { value: g.id, testo: g.nome, selected: g.id === proposto })) : el("option", { value: "", testo: "Crea prima un gruppo" }));

    const tabAttesa = attesa.length ? el("table", { class: "tabella adattiva" },
      el("thead", {}, el("tr", {}, ["Nome", "Telefono", "Richiesta", "Gruppo", "Azioni"].map((t) => el("th", { testo: t })))),
      el("tbody", {}, attesa.map((u) => {
        const sel = selGruppo(u.gruppoProposto);
        return el("tr", {},
          el("td", { "data-etichetta": "Nome", testo: u.nome }),
          el("td", { "data-etichetta": "Telefono", testo: u.telefono }),
          el("td", { "data-etichetta": "Richiesta", testo: quando(u.creato) }),
          el("td", { "data-etichetta": "Gruppo" }, sel),
          el("td", { "data-etichetta": "Azioni" }, el("div", { class: "riga-flex" },
            el("button", { class: "primario piccolo", testo: "Approva", disabled: !gruppi.length, on: { click: () => azione(() => api("/utenti/" + u.id + "/approva", { dati: { gruppi: [sel.value] } }), u.nome + " approvato.") } }),
            el("button", { class: "piccolo", testo: "Rifiuta", on: { click: () => azione(() => api("/utenti/" + u.id + "/stato", { dati: { stato: "rifiutato" } }), "Richiesta rifiutata.") } }),
            el("button", { class: "piccolo pericolo", testo: "Blocca", on: { click: () => azione(() => api("/utenti/" + u.id + "/stato", { dati: { stato: "bloccato" } }), "Account bloccato.") } }))));
      }))) : el("p", { class: "tenue", testo: "Nessuna richiesta in attesa." });

    const tabGiocatori = altri.length ? el("table", { class: "tabella adattiva" },
      el("thead", {}, el("tr", {}, ["Nome", "Stato", "Gruppi", "Ultimo accesso", ""].map((t) => el("th", { testo: t })))),
      el("tbody", {}, altri.map((u) => el("tr", {},
        el("td", { "data-etichetta": "Nome" }, u.nome, u.inattivo ? el("span", { class: "etichetta attesa", testo: " inattivo ", title: "Nessun accesso da molti mesi: valuta la cancellazione" }) : null),
        el("td", { "data-etichetta": "Stato" }, el("span", { class: "etichetta " + u.stato, testo: STATI[u.stato] || u.stato })),
        el("td", { "data-etichetta": "Gruppi", testo: u.gruppi.map((g) => nomeGruppo(g) + (u.segretario.includes(g) ? " (Segretario)" : "")).join(", ") || "—" }),
        el("td", { "data-etichetta": "Ultimo accesso", testo: quando(u.ultimoAccesso) }),
        el("td", {}, el("button", { class: "piccolo", testo: "Gestisci…", on: { click: () => gestisci(u, gruppi) } })))))) :
      el("p", { class: "tenue", testo: "Nessun giocatore ancora." });

    svuota(contenitore,
      aperte.length ? el("div", { class: "banner" }, "Richieste di cancellazione: ", aperte.map((r) => r.nome).join(", "), ". Apri «Gestisci…» sul giocatore per eseguirle.") : null,
      el("section", { class: "scheda-ui" }, el("h2", {}, "Iscrizioni in attesa ", attesa.length ? el("span", { class: "distintivo", testo: attesa.length }) : null), tabAttesa),
      el("section", { class: "scheda-ui" }, el("h2", { testo: "Giocatori" }), tabGiocatori),
      el("section", { class: "scheda-ui" }, el("h2", { testo: "Inviti" }),
        el("p", { class: "tenue", testo: "Il link d'invito apre la registrazione: il giocatore entra solo dopo la tua approvazione. Puoi stamparne il QR e tenerlo al tavolo." }),
        el("div", { class: "riga-flex" }, el("button", { class: "primario", testo: "＋ Nuovo invito con QR", on: { click: () => nuovoInvito(gruppi, disegna) } })),
        inviti.length ? el("table", { class: "tabella adattiva", stile: { marginTop: "10px" } },
          el("tbody", {}, inviti.map((i) => el("tr", {},
            el("td", { "data-etichetta": "Invito", testo: (i.nota || "Invito") + (i.gruppo ? " · " + nomeGruppo(i.gruppo) : "") }),
            el("td", { "data-etichetta": "Usi", testo: i.usi + (i.usiMax ? " / " + i.usiMax : "") + " usi" }),
            el("td", { "data-etichetta": "Scade", testo: i.revocato ? "revocato" : i.scade ? "scade " + quando(i.scade) : "senza scadenza" }),
            el("td", {}, i.revocato ? null : el("button", { class: "piccolo", testo: "Revoca", on: { click: () => azione(() => api("/inviti/" + i.id, { metodo: "DELETE" }), "Invito revocato.") } })))))) : null),
      el("section", { class: "scheda-ui" }, el("h2", { testo: "Registro degli accessi ai dati personali" }),
        el("button", { testo: "Mostra il registro", on: { click: registro } })));
  }
  async function azione(fn, ok) {
    try { await fn(); if (ok) avviso(ok); await disegna(); } catch (e) { avviso(e.message, "errore"); }
  }

  async function gestisci(u, gruppi) {
    const spunte = gruppi.map((g) => ({ g, iscr: el("input", { type: "checkbox", selezionato: u.gruppi.includes(g.id) }), seg: el("input", { type: "checkbox", selezionato: u.segretario.includes(g.id) }) }));
    const telBox = el("div", { class: "tenue", testo: u.telefono });
    const codBox = el("div");
    dialogo("Giocatore: " + u.nome, el("div", {},
      el("h3", { testo: "Gruppi" }),
      el("table", { class: "tabella" }, el("tbody", {}, spunte.map((s) => el("tr", {},
        el("td", {}, el("label", { class: "riga" }, s.iscr, el("span", { testo: s.g.nome }))),
        el("td", {}, el("label", { class: "riga" }, s.seg, el("span", { testo: "Segretario" }))))))),
      el("button", { class: "primario", testo: "Salva gruppi", on: { click: () => azione(() => api("/utenti/" + u.id + "/gruppi", { metodo: "PUT", dati: {
        gruppi: spunte.filter((s) => s.iscr.checked).map((s) => s.g.id), segretario: spunte.filter((s) => s.iscr.checked && s.seg.checked).map((s) => s.g.id) } }), "Gruppi aggiornati.") } }),
      el("h3", { testo: "Telefono", stile: { marginTop: "14px" } }), telBox,
      el("button", { class: "piccolo", testo: "Mostra numero completo", on: { click: async () => {
        try { telBox.textContent = (await api("/utenti/" + u.id + "/telefono")).telefono; } catch (e) { avviso(e.message, "errore"); }
      } } }),
      el("p", { class: "tenue piccolo", testo: "Ogni visualizzazione del numero completo resta nel registro degli accessi ai dati." }),
      el("h3", { testo: "Password dimenticata", stile: { marginTop: "14px" } }), codBox,
      el("button", { class: "piccolo", testo: "Genera codice di reimpostazione", on: { click: async () => {
        try { const r = await api("/utenti/" + u.id + "/codice-reimpostazione", { dati: {} });
          svuota(codBox, el("div", { class: "codice", testo: r.codice }), el("p", { class: "tenue piccolo", testo: "Dallo a voce al giocatore: vale una volta sola e scade tra 24 ore. Lo userà in «Password dimenticata?»." }));
        } catch (e) { avviso(e.message, "errore"); }
      } } }),
      el("h3", { testo: "Account", stile: { marginTop: "14px" } }),
      el("div", { class: "riga-flex" },
        u.stato === "approvato" ? el("button", { testo: "Sospendi", on: { click: () => azione(() => api("/utenti/" + u.id + "/stato", { dati: { stato: "sospeso" } }), "Account sospeso.") } }) : null,
        u.stato === "sospeso" ? el("button", { testo: "Riattiva", on: { click: () => azione(() => api("/utenti/" + u.id + "/stato", { dati: { stato: "approvato" } }), "Account riattivato.") } }) : null,
        u.stato !== "bloccato" ? el("button", { class: "pericolo", testo: "Blocca", on: { click: () => azione(() => api("/utenti/" + u.id + "/stato", { dati: { stato: "bloccato" } }), "Account bloccato.") } }) : null,
        el("button", { class: "pericolo", testo: "Elimina account…", on: { click: () => elimina(u) } }))), { largo: false });
  }

  async function elimina(u) {
    const scelta = el("select", {}, el("option", { value: "archivia", testo: "Conserva le schede in forma anonima" }), el("option", { value: "elimina", testo: "Elimina anche le schede" }));
    dialogo("Eliminare l'account di " + u.nome + "?", el("div", {},
      el("p", { testo: "Verranno eliminati account, numero di telefono e background. Le schede:" }), scelta,
      el("p", { class: "tenue piccolo", testo: "I backup cifrati che contengono ancora questi dati vengono cancellati automaticamente entro il periodo di conservazione." })),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "pericolo", testo: "Elimina definitivamente", on: { click: async () => {
        if (!(await conferma("Conferma", "L'operazione non si può annullare.", "Elimina", true))) return;
        chiudi(); document.querySelectorAll(".velo").forEach((v) => v.remove());
        azione(() => api("/utenti/" + u.id + "?schede=" + scelta.value, { metodo: "DELETE" }), "Account eliminato.");
      } } })) });
  }

  async function nuovoInvito(gruppi, poi) {
    const g = el("select", {}, el("option", { value: "", testo: "Nessun gruppo proposto" }), gruppi.map((x) => el("option", { value: x.id, testo: x.nome })));
    const nota = el("input", { maxlength: "80", placeholder: "Es. QR del tavolo" });
    const giorni = el("input", { type: "number", min: "0", max: "365", valore: "30" });
    const usi = el("input", { type: "number", min: "0", max: "100", valore: "0" });
    dialogo("Nuovo invito", el("div", {}, el("label", {}, "Gruppo proposto al Master all'approvazione", g), el("label", {}, "Nota", nota),
      el("label", {}, "Valido per giorni (0 = senza scadenza)", giorni), el("label", {}, "Numero massimo di usi (0 = illimitato)", usi)),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Crea", on: { click: async () => {
        try {
          const r = await api("/inviti", { dati: { gruppo: g.value, nota: nota.value, giorni: +giorni.value || 0, usiMax: +usi.value || 0 } });
          chiudi(); await poi(); mostraInvito(r.codice, nota.value);
        } catch (e) { avviso(e.message, "errore"); }
      } } })) });
  }

  async function mostraInvito(codice, nota) {
    let base = location.origin, remoto = "", casa = location.host;
    try {
      const ind = await api("/indirizzi"); if (ind.benvenuto) { base = ind.benvenuto; casa = ind.benvenuto.replace(/^https?:\/\//, ""); }
      // Con la VPN configurata (nome .ts.net in nomi_certificato): link anche per chi è fuori casa.
      // Accesso da Internet (Funnel) se configurato, altrimenti il nome .ts.net della VPN.
      const n = ind.pubblico || (ind.vpn || []).find((u) => /\.ts\.net(:\d+)?$/.test(u)); if (n) remoto = n + "/?invito=" + codice;
    } catch { /* resta l'indirizzo attuale */ }
    // Il QR porta alla pagina di benvenuto (certificato), che poi rimanda alla registrazione con il codice.
    const link = base + "/?invito=" + codice;
    // Messaggio per WhatsApp: WhatsApp rende cliccabile solo la parte numerica di un link con indirizzo e porta
    // (http://192.168.1.202:7601/…), che così non funziona. Indirizzo da scrivere a mano e codice a parte; il link
    // per fuori casa (nome .ts.net) invece WhatsApp lo riconosce intero.
    const messaggio = "Benvenuto in Gaster Bless! 🦇\n\n" +
      (remoto ? "🏠 A casa del Master (sul suo Wi-Fi):\n" : "Collegati al Wi-Fi di casa del Master, poi:\n") +
      "1. apri il browser (Chrome o Safari) e scrivi a mano nella barra degli indirizzi:\n   " + casa + "\n" +
      "   (non toccare l'indirizzo qui: WhatsApp lo apre a metà)\n" +
      "2. tocca «Entra in Gaster Bless», poi «Registrati»\n" +
      "3. nel campo «Codice d'invito» scrivi: " + codice + "\n" +
      (remoto ? "\n🌍 Da fuori casa (quando il Master ha aperto l'accesso da Internet), tocca questo link:\n" + remoto + "\n" : "");
    const copia = (t, ok) => navigator.clipboard.writeText(t).then(() => avviso(ok), () => avviso("Copia non riuscita.", "errore"));
    dialogo("Invito pronto", el("div", { class: "area-stampa" },
      el("h1", { class: "logo", testo: "GASTER BLESS" }), nota ? el("p", { testo: nota, stile: { textAlign: "center" } }) : null,
      el("img", { class: "qr", alt: "Codice QR dell'invito", src: "/api/v1/qr.png?t=" + encodeURIComponent(link) }),
      el("p", { class: "tenue", stile: { textAlign: "center", wordBreak: "break-all" }, testo: link }),
      el("p", { class: "tenue piccolo", testo: "Il codice completo si vede solo ora: conservalo stampando il QR. Il giocatore deve essere collegato al Wi-Fi di casa." }),
      el("div", { class: "non-stampare scheda-ui", stile: { margin: "10px 0" } },
        el("p", {}, el("strong", { testo: "Codice d'invito: " }), el("code", { class: "codice-invito", testo: codice })),
        el("p", { class: "tenue piccolo", testo: "Su WhatsApp un link con l'indirizzo numerico non funziona (diventa cliccabile solo a metà): manda il messaggio pronto qui sotto, con l'indirizzo da scrivere a mano e il codice." }),
        el("div", { class: "riga-flex" },
          el("a", { class: "pulsante primario piccolo", href: "https://wa.me/?text=" + encodeURIComponent(messaggio), target: "_blank", rel: "noopener", testo: "💬 Manda su WhatsApp" }),
          el("button", { class: "piccolo", testo: "📋 Copia il messaggio", on: { click: () => copia(messaggio, "Messaggio copiato: incollalo su WhatsApp.") } }))),
      remoto ? el("div", { class: "non-stampare" }, el("p", { class: "tenue", testo: "Per un giocatore fuori casa (con l'accesso da Internet aperto, oppure con la VPN):" }),
        el("p", { class: "tenue", stile: { textAlign: "center", wordBreak: "break-all" } }, el("code", { testo: remoto })),
        el("button", { class: "piccolo", testo: "📋 Copia il link per fuori casa", on: { click: () => navigator.clipboard.writeText(remoto).then(() => avviso("Link copiato."), () => avviso("Copia non riuscita.", "errore")) } })) : null),
    { azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { testo: "📋 Copia link", on: { click: () => navigator.clipboard.writeText(link).then(() => avviso("Link copiato."), () => avviso("Copia non riuscita.", "errore")) } }),
      el("button", { testo: "🖨 Stampa", on: { click: () => window.print() } }), el("button", { class: "primario", testo: "Fatto", on: { click: chiudi } })) });
  }

  async function registro() {
    const r = await api("/registro-dati");
    const AZ = { telefono_visto: "numero di telefono visto", esportazione_propri_dati: "esportazione dei propri dati", cancellazione_account_archivia: "account cancellato (schede anonime)", cancellazione_account_elimina: "account cancellato (schede eliminate)", background_letto: "background letto dal Master" };
    dialogo("Registro degli accessi ai dati personali", r.length ? el("table", { class: "tabella" }, el("tbody", {}, r.map((x) => el("tr", {},
      el("td", { testo: quando(x.quando) }), el("td", { testo: AZ[x.azione] || x.azione }), el("td", { testo: x.interessato || "—" }))))) :
      el("p", { class: "tenue", testo: "Nessun accesso registrato." }), { largo: true });
  }

  await disegna();
}
