// Sezione "Impostazioni" (solo Master): accesso esterno, registrazione, secondo fattore, indirizzi e certificato.
import { el, svuota, api, avviso, conferma, quando, dialogo } from "../ui/ui.js";
import { sezioneSanita } from "./impostazioni_sanita.js"; // dal ramo main-ke8sy4
import { sezioneAggiornamento } from "./impostazioni_aggiornamento.js"; // tasto «Aggiorna» (2026-11-11)
import { sezioneMedia } from "./impostazioni_media.js"; // video leggeri per i telefoni (2026-11-11)
import { sezioneCampagna } from "./impostazioni_campagna.js"; // file unico della campagna (2026-11-11)

export async function mostra(corpo) {
  const contenitore = el("div", { class: "largo" });
  svuota(corpo, contenitore);

  async function disegna() {
    let imp, ind;
    try {
      [imp, ind] = await Promise.all([api("/impostazioni"), api("/indirizzi").catch(() => ({ https: [], benvenuto: "" }))]);
    } catch (e) {
      svuota(contenitore, el("p", { class: "errore", testo: e.message }));
      return;
    }
    svuota(contenitore,
      sezioneAggiornamento(imp),
      sezioneMedia(),
      sezioneEsterno(imp),
      sezioneRegistrazione(imp), sezioneApprovazioneSchede(imp),
      sezioneTOTP(imp),
      sezioneIndirizzi(imp, ind),
      sezioneConfigurazione(imp),
      await sezioneBackup(imp),
      sezioneCampagna(),
      sezioneMigrazione(imp),
      await sezionePlugin(),
      sezioneSanita());
  }

  // Schede alternative (plugin) fatte dai giocatori: si installano copiando una cartella, si abilitano qui.
  async function sezionePlugin() {
    let r = { lista: [], cartella: "" };
    try { r = await api("/schede-plugin"); } catch (e) { avviso(e.message, "errore"); }
    const FUNZ = { pf: "PF dal tracker", tiri: "tiri", incantesimi: "incantesimi", background: "background", sanita: "Sanità mentale", tema: "tema", stampa: "stampa" };
    return el("section", { class: "scheda-ui", id: "schede-plugin" },
      el("h2", { testo: "🧩 Schede alternative (plugin)" }),
      el("p", { class: "tenue", testo: "Schede del personaggio fatte da altri, da usare al posto di quella di Gaster Bless per i personaggi che le scelgono. Per installarne una copia la sua cartella in " + r.cartella + " e ricarica questa pagina. Girano isolate: non vedono gli altri giocatori, non possono usare Gaster Bless al posto tuo e non si collegano a Internet." }),
      r.lista.length ? el("table", { class: "tabella" }, el("tbody", {}, r.lista.map((x) => el("tr", {},
        el("td", {}, el("b", { testo: x.nome }), x.versione ? el("small", { class: "tenue", testo: " v" + x.versione }) : null,
          x.autore ? el("div", { class: "tenue piccolo", testo: "di " + x.autore }) : null,
          x.descrizione ? el("div", { class: "tenue piccolo", testo: x.descrizione }) : null),
        el("td", { class: "piccolo" }, x.errore ? el("span", { class: "errore", testo: "⚠ " + x.errore })
          : (x.funzioni.length ? "Supporta: " + x.funzioni.map((f) => FUNZ[f] || f).join(", ") : "Funzioni di base"),
          x.personaggi ? el("div", { class: "tenue", testo: x.personaggi + (x.personaggi > 1 ? " personaggi la usano" : " personaggio la usa") }) : null),
        el("td", {}, el("label", { class: "riga" }, el("input", { type: "checkbox", selezionato: x.abilitato, disabled: !!x.errore && !x.abilitato,
          on: { change: async (e) => {
            try { await api("/schede-plugin/" + encodeURIComponent(x.id), { metodo: "PUT", dati: { abilitato: e.target.checked } }); avviso(e.target.checked ? "Scheda abilitata: i giocatori la possono scegliere." : "Scheda disabilitata."); }
            catch (er) { e.target.checked = !e.target.checked; avviso(er.message, "errore"); }
          } } }), el("span", { testo: "Abilitata" }))))))) : el("p", { class: "tenue", testo: "Nessuna scheda alternativa installata." }),
      el("p", { class: "tenue piccolo", testo: "Per chi le sviluppa: kit con protocollo, esempio e simulatore nella cartella kit-scheda del progetto." }));
  }

  async function sezioneBackup(imp) {
    let b = { backup: [], giorni: 30, ora: 4, cartella: "" };
    try { b = await api("/backup"); } catch (e) { avviso(e.message, "errore"); }
    const mb = (n) => (n / 1048576).toFixed(1).replace(".", ",") + " MB";
    return el("section", { class: "scheda-ui" },
      el("h2", { testo: "Backup e ripristino" }),
      el("p", { class: "tenue", testo: "Ogni giorno alle " + b.ora + ":00 Gaster Bless salva un backup cifrato di account, gruppi, schede, background, mappa e impostazioni. Si tengono gli ultimi " + b.giorni + " giorni. Immagini, video, musica e manuali restano nella cartella GasterBlessDati: copiala tu ogni tanto su un disco esterno." }),
      el("div", { class: "riga-flex" }, el("button", { class: "primario", testo: "💾 Crea backup ora", on: { click: async (e) => {
        e.target.disabled = true;
        try { const r = await api("/backup", { dati: {} }); avviso("Backup creato: " + r.nome); disegna(); } catch (er) { avviso(er.message, "errore"); e.target.disabled = false; }
      } } })),
      b.backup.length ? el("table", { class: "tabella", stile: { marginTop: "10px" } }, el("tbody", {}, b.backup.slice(0, 10).map((x) => el("tr", {},
        el("td", {}, el("code", { testo: x.nome })), el("td", { testo: quando(x.quando) }), el("td", { testo: mb(x.dimensione) })))))
        : el("p", { class: "tenue", testo: "Nessun backup ancora." }),
      b.backup.length > 10 ? el("p", { class: "tenue piccolo", testo: "…e altri " + (b.backup.length - 10) + "." }) : null,
      el("details", { stile: { marginTop: "10px" } }, el("summary", { testo: "Come si ripristina un backup" }),
        el("ol", {},
          el("li", { testo: "Ferma Gaster Bless (icona «Ferma Gaster Bless» o chiudi la finestra nera)." }),
          el("li", {}, "Apri il terminale nella cartella di Gaster Bless e scrivi: ", el("code", { testo: "GasterBless.exe ripristina NOME-DEL-BACKUP.bak" })),
          el("li", { testo: "Riavvia Gaster Bless. Lo stato di prima del ripristino non si perde: viene messo da parte nella cartella dei backup." })),
        el("p", { class: "tenue piccolo" }, "Cartella dei backup: ", el("code", { testo: b.cartella }))),
      el("details", {}, el("summary", { testo: "Codice di ripristino (se il PC si guasta)" }),
        el("p", { testo: "I backup sono cifrati con una chiave che funziona solo su questo PC. Per ripristinarli su un altro PC (o dopo aver reinstallato Windows) serve il codice di ripristino." }),
        imp.locale ? el("button", { testo: "🔑 Mostra il codice di ripristino…", on: { click: () => chiediPassword("Codice di ripristino", "Per sicurezza ripeti la password del Master.", async (pw) => {
          const r = await api("/backup/codice", { dati: { password: pw } });
          dialogo("Codice di ripristino", el("div", {}, el("p", { class: "codice", testo: r.codice }),
            el("p", { testo: "Scrivilo su carta e conservalo lontano dal PC. Non mandarlo per messaggio o e-mail: chi ha il codice e un backup può leggere i dati." }),
            el("p", { class: "tenue piccolo" }, "Per ripristinare (a Gaster Bless fermo): ", el("code", { testo: "GasterBless.exe ripristina NOME.bak -codice " + r.codice }))));
        }) } }) : el("p", { class: "tenue", testo: "Si mostra solo dal PC su cui gira Gaster Bless." })));
  }

  function chiediPassword(titolo, testo, poi) {
    const pw = el("input", { type: "password", autocomplete: "current-password" });
    const err = el("p", { class: "errore" });
    dialogo(titolo, el("div", {}, el("p", { testo }), el("label", {}, "Password del Master", pw), err),
      { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Conferma", on: { click: async () => {
          try { await poi(pw.value); chiudi(); } catch (e) { err.textContent = e.message; }
        } } })) });
  }

  function sezioneMigrazione(imp) {
    const esito = el("pre", { class: "rapporto", hidden: true });
    const mostra = (t) => { esito.textContent = t; esito.hidden = false; };
    return el("section", { class: "scheda-ui" },
      el("h2", { testo: "Importa dalla vecchia Gaster Bless" }),
      el("p", { class: "tenue", testo: "Porta qui gruppi, nebbia e scoperte di ogni gruppo, descrizioni e note delle località, elenco dei media, sessioni giocate, bestiario, manuali e impostazioni della musica dalla cartella GasterBlessDati. La cartella non viene modificata e i file (video, musica, manuali) restano dove sono. Password e link d'invito non si importano: i giocatori si registrano con il loro account." }),
      imp.locale ? el("div", { class: "riga-flex" },
        el("button", { testo: "1. Prova (non scrive nulla)", on: { click: async (e) => {
          e.target.disabled = true;
          try { mostra((await api("/migrazione", { dati: { prova: true } })).rapporto); } catch (er) { mostra(er.message); } finally { e.target.disabled = false; }
        } } }),
        el("button", { class: "primario", testo: "2. Importa…", on: { click: () => chiediPassword("Importare i dati della vecchia Gaster Bless?",
          "Prima viene creato un backup della nuova app. Si può fare una volta sola. Ripeti la password del Master per confermare.", async (pw) => {
            mostra((await api("/migrazione", { dati: { prova: false, password: pw } })).rapporto);
            avviso("Importazione completata.");
          }) } })) : el("p", { class: "tenue", testo: "Si fa solo dal PC su cui gira Gaster Bless." }),
      esito);
  }

  function sezioneEsterno(imp) {
    const vpn = imp.accessoEsterno, inet = imp.accessoPubblico, aperto = vpn || inet;
    const cf = imp.servizioPubblico === "cloudflare", servizio = cf ? "Cloudflare Tunnel" : "Tailscale Funnel";
    const stato = inet && vpn ? "aperto da Internet e dalla VPN" : inet ? "aperto da Internet (" + servizio + ")" : vpn ? "aperto tramite VPN" : "solo rete di casa";
    const apri = (percorso, testo) => el("button", { testo, disabled: !imp.locale || !imp.totpAttivo, title: !imp.totpAttivo ? "Prima attiva il secondo fattore del Master" : !imp.locale ? "Si apre solo dal PC di Gaster Bless" : "",
      on: { click: async () => { try { await api(percorso, { dati: {} }); avviso("Accesso aperto."); disegna(); } catch (e) { avviso(e.message, "errore"); } } } });
    return el("section", { class: "scheda-ui", id: "accesso-esterno" },
      el("h2", { testo: "Accesso da fuori casa" }),
      el("p", {}, "Stato: ", el("span", { class: "etichetta " + (aperto ? "attesa" : "approvato"), testo: stato })),
      aperto ? el("div", {},
        el("p", { class: "tenue", testo: inet
          ? "Gaster Bless è raggiungibile da tutta Internet all'indirizzo https://" + imp.nomePubblico + " (i giocatori non installano nulla; serve il loro account Gaster Bless)." +
            (cf ? " Con Cloudflare va bene anche in casa, su tutti i telefoni (iPhone compresi) senza certificato. Chiudila quando non serve." : " Chiudila quando non giocate.")
          : "Chi è collegato tramite VPN può usare Gaster Bless." }),
        el("button", { class: "pericolo", testo: "⛔ Chiudi subito l'accesso esterno", on: { click: async () => {
          if (!(await conferma("Chiudere l'accesso esterno?", "Chi è collegato da fuori casa (Internet o VPN) verrà scollegato subito. Potrai riaprirlo solo da questo PC.", "Chiudi subito", true))) return;
          try { await api("/accesso-esterno/chiudi", { dati: {} }); avviso("Accesso esterno chiuso."); disegna(); } catch (e) { avviso(e.message, "errore"); }
        } } })) : null,
      !inet ? el("div", {},
        el("h3", { testo: "Da Internet, senza installare nulla (" + servizio + ")" }),
        imp.pubblicoConfigurato
          ? [el("p", { class: "tenue", testo: "Configurato per https://" + imp.nomePubblico + ". Si apre solo da questo PC, con il secondo fattore attivo." }), apri("/accesso-pubblico/apri", "🌐 Apri l'accesso da Internet")]
          : el("p", { class: "tenue", testo: "Non configurato. I giocatori lontani aprirebbero Gaster Bless nel browser, senza installare nulla: segui la guida guide\\internet.md (Tailscale Funnel) o guide\\cloudflare.md (Cloudflare Tunnel, con un tuo dominio)." })) : null,
      !vpn ? el("div", {},
        el("h3", { testo: "Con la VPN (i giocatori installano Tailscale)" }),
        imp.accessoEsternoConfig === "vpn"
          ? [el("p", { class: "tenue", testo: "La VPN è configurata." }), apri("/accesso-esterno/apri", "Riapri l'accesso tramite VPN")]
          : el("p", { class: "tenue", testo: "Non configurata: guida guide\\vpn.md. Per il solo tracker dell'iniziativa c'è anche il gioco da remoto." })) : null);
  }

  function sezioneRegistrazione(imp) {
    const c = el("input", { type: "checkbox", selezionato: imp.registrazioneSenzaInvito, on: { change: async () => {
      try { await api("/impostazioni", { metodo: "PATCH", dati: { registrazioneSenzaInvito: c.checked } }); avviso("Impostazione salvata."); }
      catch (e) { c.checked = !c.checked; avviso(e.message, "errore"); }
    } } });
    return el("section", { class: "scheda-ui" },
      el("h2", { testo: "Registrazione dei giocatori" }),
      el("label", { class: "riga" }, c, el("span", { testo: "Permetti la registrazione anche senza link d'invito" })),
      el("p", { class: "tenue piccolo", testo: "In ogni caso nessuno entra senza la tua approvazione. Togliendo la spunta, ci si può registrare solo con un link o un QR d'invito (sezione Iscrizioni)." }));
  }

  // «Modifica manuale» delle schede (2026-11-11): con la spunta i giocatori cambiano da soli solo i dati di gioco.
  function sezioneApprovazioneSchede(imp) {
    const c = el("input", { type: "checkbox", selezionato: imp.approvazioneSchede, on: { change: async () => {
      try { await api("/impostazioni", { metodo: "PATCH", dati: { approvazioneSchede: c.checked } }); avviso("Impostazione salvata: vale dalla prossima apertura delle schede."); }
      catch (e) { c.checked = !c.checked; avviso(e.message, "errore"); }
    } } });
    return el("section", { class: "scheda-ui" },
      el("h2", { testo: "Modifiche dei giocatori alle schede" }),
      el("label", { class: "riga" }, c, el("span", { testo: "Le modifiche alle schede valgono solo dopo la mia approvazione" })),
      el("p", { class: "tenue piccolo", testo: "Con la spunta i giocatori cambiano da soli solo i dati di gioco (PF, slot, ispirazione, indebolimento, incantesimi preparati, oggetti equipaggiati e cariche; le monete passano già da te). Per tutto il resto accendono «✏ Modifica manuale» nella scheda: quando la spengono ricevi un report e approvi o rifiuti voce per voce. Senza spunta le schede si modificano liberamente, come prima." }));
  }

  function sezioneTOTP(imp) {
    const box = el("div");
    const s = el("section", { class: "scheda-ui" },
      el("h2", { testo: "Secondo fattore (codice dal telefono)" }),
      imp.totpAttivo
        ? el("p", {}, el("span", { class: "etichetta approvato", testo: "attivo" }), " All'accesso del Master viene chiesto anche il codice a 6 cifre dell'app di autenticazione.")
        : el("p", { class: "tenue", testo: "Facoltativo in casa, obbligatorio per aprire l'accesso da fuori casa. Serve un'app di autenticazione sul telefono (per esempio Google Authenticator, Microsoft Authenticator o Aegis)." }),
      box);
    if (imp.totpAttivo) return s;
    if (!imp.locale) {
      box.append(el("p", { class: "tenue", testo: "Si configura solo dal PC su cui gira Gaster Bless." }));
      return s;
    }
    box.append(el("button", { class: "primario", testo: "Configura il secondo fattore", on: { click: async () => {
      let r;
      try { r = await api("/totp/prepara", { dati: {} }); } catch (e) { avviso(e.message, "errore"); return; }
      const codice = el("input", { inputmode: "numeric", autocomplete: "one-time-code", maxlength: "6", placeholder: "123456", stile: { maxWidth: "160px", display: "block" } });
      const err = el("p", { class: "errore" });
      svuota(box,
        el("ol", {},
          el("li", { testo: "Apri l'app di autenticazione sul telefono e aggiungi un account inquadrando questo QR." }),
          el("li", { testo: "Scrivi qui sotto il codice a 6 cifre che compare nell'app." })),
        el("img", { class: "qr", alt: "Codice QR per l'app di autenticazione", src: "/api/v1/qr.png?t=" + encodeURIComponent(r.uri) }),
        el("p", { class: "tenue piccolo" }, "Se non puoi inquadrare il QR, inserisci a mano questa chiave: ", el("code", { testo: r.segreto })),
        el("label", {}, "Codice dall'app", codice), err,
        el("button", { class: "primario", testo: "Attiva", on: { click: async () => {
          try { await api("/totp/attiva", { dati: { codice: codice.value.trim() } }); avviso("Secondo fattore attivato."); disegna(); }
          catch (e) { err.textContent = e.message; }
        } } }));
      codice.focus();
    } } }));
    return s;
  }

  function sezioneIndirizzi(imp, ind) {
    return el("section", { class: "scheda-ui" },
      el("h2", { testo: "Indirizzi e certificato" }),
      el("p", { class: "tenue", testo: "I giocatori aprono Gaster Bless da uno di questi indirizzi, collegati al Wi-Fi di casa:" }),
      ind.https && ind.https.length ? el("ul", {}, ind.https.map((u) => el("li", {}, el("code", { testo: u })))) : el("p", { class: "tenue", testo: "Nessun indirizzo di rete trovato: il PC è collegato al Wi-Fi o al cavo?" }),
      ind.benvenuto ? el("p", {}, "Pagina di benvenuto per i telefoni (porta all'app; certificato facoltativo): ", el("code", { testo: ind.benvenuto })) : null,
      ind.vpn && ind.vpn.length ? [el("p", { class: "tenue", testo: "Da fuori casa, con la VPN (solo quando l'accesso esterno è aperto; vedi guide\\vpn.md). Il nome .ts.net è quello da dare ai giocatori con cui condividi il PC:" }),
        el("ul", {}, ind.vpn.map((u) => el("li", {}, el("code", { testo: u }))))] : null,
      imp.impronta ? el("p", { class: "tenue piccolo" }, "Impronta del certificato di Gaster Bless (per controllarlo sui dispositivi): ", el("code", { testo: imp.impronta, stile: { wordBreak: "break-all" } })) : null);
  }

  function sezioneConfigurazione(imp) {
    return el("section", { class: "scheda-ui" },
      el("h2", { testo: "Configurazione" }),
      el("table", { class: "tabella" }, el("tbody", {},
        riga("Pannello del Master solo da questo PC", imp.pannelloSoloPC ? "sì" : "no"),
        riga("Conservazione dei backup", imp.backupGiorni + " giorni"),
        riga("Account segnalati come inattivi dopo", imp.inattiviMesi + " mesi"),
        riga("Stai usando il PC su cui gira Gaster Bless", imp.locale ? "sì" : "no"))),
      el("p", { class: "tenue piccolo", testo: "Questi valori si cambiano nel file gasterbless.conf, accanto al programma, e valgono dal riavvio successivo." }));
  }

  const riga = (k, v) => el("tr", {}, el("td", { testo: k }), el("td", { testo: v }));

  await disegna();
}
