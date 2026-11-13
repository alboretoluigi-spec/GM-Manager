// Ingresso: primo avvio (account del Master), accesso, registrazione, reimpostazione password, attesa.
import { el, svuota, api, avviso } from "../ui/ui.js";
import { intestazione } from "../ui/marchio.js";
import { avvio } from "../ui/app.js";

// Interfaccia adattiva (fase 3c, C4a): dopo ogni accesso il Master sceglie la campagna da giocare (web/ui/app.js).
function appenaEntrato() { try { sessionStorage.setItem("gasterbless.sceltaCampagna", "1"); } catch { /* ignora */ } }

const PREFISSI = [["+39", "🇮🇹 +39"], ["+41", "🇨🇭 +41"], ["+33", "🇫🇷 +33"], ["+49", "🇩🇪 +49"], ["+34", "🇪🇸 +34"],
  ["+43", "🇦🇹 +43"], ["+44", "🇬🇧 +44"], ["+378", "🇸🇲 +378"], ["+356", "🇲🇹 +356"], ["", "Altro"]];

function pagina(...figli) {
  return el("div", { class: "ingresso" }, el("div", { class: "centrato" }, intestazione(), ...figli));
}

function campoTelefono() {
  const pref = el("select", { "aria-label": "Prefisso internazionale" }, PREFISSI.map(([v, t]) => el("option", { value: v, testo: t })));
  const num = el("input", { type: "tel", inputmode: "tel", autocomplete: "tel-national", placeholder: "333 1234567", required: true });
  pref.addEventListener("change", () => { num.placeholder = pref.value ? "333 1234567" : "+1 555 0100"; });
  const nodo = el("label", {}, "Numero di telefono", el("div", { class: "campo-tel" }, pref, num));
  return { nodo, valore: () => { const n = num.value.trim(); return n.startsWith("+") || n.startsWith("00") || !pref.value ? n : pref.value + " " + n; }, num };
}

/** Requisiti della password, mostrati e spuntati mentre si scrive. */
function campoPassword(etichetta = "Password") {
  const pw = el("input", { type: "password", autocomplete: "new-password", required: true, minlength: "10" });
  const conf = el("input", { type: "password", autocomplete: "new-password", required: true });
  const r1 = el("li", { testo: "almeno 10 caratteri" }), r2 = el("li", { testo: "almeno 4 caratteri diversi" }), r3 = el("li", { testo: "le due password coincidono" });
  const agg = () => {
    r1.classList.toggle("ok", [...pw.value].length >= 10);
    r2.classList.toggle("ok", new Set([...pw.value.toLowerCase()]).size >= 4);
    r3.classList.toggle("ok", pw.value !== "" && pw.value === conf.value);
  };
  pw.addEventListener("input", agg); conf.addEventListener("input", agg);
  return {
    nodo: el("div", {}, el("label", {}, etichetta, pw), el("label", {}, "Ripeti la password", conf), el("ul", { class: "requisiti" }, r1, r2, r3)),
    pw, conf,
  };
}

function inviaModulo(modulo, bottone, errore, fn) {
  modulo.addEventListener("submit", async (e) => {
    e.preventDefault();
    errore.textContent = "";
    bottone.disabled = true;
    try { await fn(); } catch (er) { errore.textContent = er.message; } finally { bottone.disabled = false; }
  });
}

export function mostra(radice, io) {
  const invito = new URLSearchParams(location.search).get("invito") || "";
  if (io.primoAvvio) return primoAvvio(radice);
  if (io.serveMaster) return svuota(radice, pagina(el("div", { class: "scheda-ui" },
    el("h2", { testo: "Gaster Bless non è ancora pronta" }), el("p", { testo: "Il Master deve completare la configurazione dal PC su cui gira Gaster Bless." }))));
  if (invito || location.pathname === "/registrazione") return registrazione(radice, invito);
  if (location.pathname === "/reimposta") return reimposta(radice);
  return accesso(radice);
}

function primoAvvio(radice) {
  const nome = el("input", { autocomplete: "username", required: true, maxlength: "60" });
  const pw = campoPassword("Password del Master");
  const err = el("p", { class: "errore" });
  const b = el("button", { class: "primario", type: "submit", testo: "Crea l'account del Master" });
  const f = el("form", { class: "scheda-ui" }, el("h2", { testo: "Benvenuto, Master" }),
    el("p", { class: "tenue", testo: "È il primo avvio. Crea il tuo account: con questo nome e questa password gestirai gruppi, iscrizioni e partite. Il nome serve per accedere." }),
    el("label", {}, "Il tuo nome", nome), pw.nodo, err, b);
  inviaModulo(f, b, err, async () => {
    if (pw.pw.value !== pw.conf.value) throw new Error("Le due password non coincidono.");
    await api("/primo-avvio", { dati: { nome: nome.value, password: pw.pw.value } });
    appenaEntrato();
    avvio();
  });
  svuota(radice, pagina(f));
}

function accesso(radice) {
  let master = false;
  const tel = campoTelefono();
  const nome = el("input", { autocomplete: "username", maxlength: "60" });
  const campoNome = el("label", { hidden: true }, "Nome del Master", nome);
  const pw = el("input", { type: "password", autocomplete: "current-password", required: true });
  const err = el("p", { class: "errore" });
  const b = el("button", { class: "primario", type: "submit", testo: "Entra in Gaster Bless" });
  const cambia = el("button", { type: "button", class: "piccolo", testo: "Sono il Master", on: { click: () => {
    master = !master; tel.nodo.hidden = master; campoNome.hidden = !master;
    tel.num.required = !master; cambia.textContent = master ? "Sono un giocatore" : "Sono il Master";
  } } });
  const f = el("form", { class: "scheda-ui" },
    el("p", { class: "tenue", testo: "Entra con il numero di telefono e la password del tuo account." }),
    tel.nodo, campoNome, el("label", {}, "Password", pw), err, b,
    el("div", { class: "riga-flex", stile: { marginTop: "12px" } },
      el("a", { href: "/registrazione", testo: "Crea un account" }), el("span", { class: "spazio" }),
      el("a", { href: "/reimposta", testo: "Password dimenticata?" })),
    el("div", { class: "riga-flex", stile: { marginTop: "10px" } }, cambia, el("span", { class: "spazio" }), el("a", { href: "/privacy", testo: "Privacy" })));
  inviaModulo(f, b, err, async () => {
    const r = await api("/accesso", { dati: master ? { nome: nome.value, password: pw.value } : { telefono: tel.valore(), password: pw.value } });
    if (r.serveTOTP) return secondoFattore(radice);
    history.replaceState(null, "", "/");
    appenaEntrato();
    avvio();
  });
  svuota(radice, pagina(f));
}

function secondoFattore(radice) {
  const c = el("input", { inputmode: "numeric", autocomplete: "one-time-code", maxlength: "7", required: true, placeholder: "123456" });
  const err = el("p", { class: "errore" });
  const b = el("button", { class: "primario", type: "submit", testo: "Conferma" });
  const f = el("form", { class: "scheda-ui" }, el("h2", { testo: "Secondo fattore" }),
    el("p", { class: "tenue", testo: "Apri l'app di autenticazione sul telefono e scrivi il codice di 6 cifre di Gaster Bless." }),
    el("label", {}, "Codice", c), err, b);
  inviaModulo(f, b, err, async () => { await api("/accesso/totp", { dati: { codice: c.value } }); history.replaceState(null, "", "/"); appenaEntrato(); avvio(); });
  svuota(radice, pagina(f));
}

async function registrazione(radice, invito) {
  let gruppo = "";
  if (invito) {
    try { gruppo = (await api("/invito/" + encodeURIComponent(invito))).gruppo; }
    catch (e) { svuota(radice, pagina(el("div", { class: "scheda-ui" }, el("p", { class: "errore", testo: e.message }), el("a", { href: "/", testo: "Torna all'accesso" })))); return; }
  }
  const nome = el("input", { autocomplete: "nickname", required: true, maxlength: "40", minlength: "2" });
  // Codice d'invito scritto a mano: WhatsApp rende cliccabile solo la parte numerica dei link con indirizzo e porta
  // (http://192.168.x.x:7701/?invito=…), quindi spesso il codice non arriva con il link.
  const codice = invito ? null : el("input", { autocomplete: "off", autocapitalize: "characters", spellcheck: "false", maxlength: "40", placeholder: "per esempio U89S6U3B2DFD" });
  const tel = campoTelefono();
  const pw = campoPassword();
  const consenso = el("input", { type: "checkbox", required: true });
  const avvisi = el("input", { type: "checkbox", selezionato: true });
  const err = el("p", { class: "errore" });
  const b = el("button", { class: "primario", type: "submit", testo: "Registrati" });
  const f = el("form", { class: "scheda-ui" },
    el("h2", { testo: "Crea il tuo account" }),
    gruppo ? el("p", { class: "tenue", testo: "Invito per il gruppo «" + gruppo + "»." }) : null,
    codice ? el("label", {}, "Codice d'invito (se il Master te ne ha mandato uno)", codice) : null,
    el("label", {}, "Nome (quello che vedranno gli altri al tavolo)", nome), tel.nodo, pw.nodo,
    el("label", { class: "riga" }, consenso, el("span", {}, "Ho letto e accetto l'", el("a", { href: "/privacy", target: "_blank", testo: "informativa sulla privacy" }), ".")),
    el("label", { class: "riga" }, avvisi, el("span", { testo: "Voglio ricevere gli avvisi delle sessioni su WhatsApp (puoi cambiarlo quando vuoi)." })),
    err, b, el("p", { class: "tenue", stile: { marginTop: "10px" } }, "Hai già un account? ", el("a", { href: "/", testo: "Entra" })));
  inviaModulo(f, b, err, async () => {
    if (pw.pw.value !== pw.conf.value) throw new Error("Le due password non coincidono.");
    if (!consenso.checked) throw new Error("Per registrarti devi accettare l'informativa.");
    await api("/registrazione", { dati: { nome: nome.value, telefono: tel.valore(), password: pw.pw.value, conferma: pw.conf.value,
      consenso: consenso.checked, avvisi: avvisi.checked, invito: invito || (codice && codice.value.trim()) || "" } });
    history.replaceState(null, "", "/");
    avvio();
  });
  svuota(radice, pagina(f));
}

function reimposta(radice) {
  const tel = campoTelefono();
  const codice = el("input", { required: true, autocomplete: "one-time-code", placeholder: "ABCDE-FGHJK", maxlength: "14" });
  const pw = campoPassword("Nuova password");
  const err = el("p", { class: "errore" });
  const b = el("button", { class: "primario", type: "submit", testo: "Imposta la nuova password" });
  const f = el("form", { class: "scheda-ui" }, el("h2", { testo: "Nuova password" }),
    el("p", { class: "tenue", testo: "Chiedi al Master un codice di reimpostazione: vale una sola volta e scade dopo 24 ore." }),
    tel.nodo, el("label", {}, "Codice del Master", codice), pw.nodo, err, b,
    el("p", { stile: { marginTop: "10px" } }, el("a", { href: "/", testo: "Torna all'accesso" })));
  inviaModulo(f, b, err, async () => {
    await api("/password/reimposta", { dati: { telefono: tel.valore(), codice: codice.value, password: pw.pw.value, conferma: pw.conf.value } });
    avviso("Password cambiata: ora puoi entrare.");
    history.replaceState(null, "", "/");
    avvio();
  });
  svuota(radice, pagina(f));
}

/** Pagina per gli account non ancora approvati (o sospesi): nessun accesso alle funzioni di gioco. */
export function attesa(radice, io) {
  const u = io.utente;
  const testi = {
    in_attesa: ["🕯 Richiesta inviata", "Il Master deve approvare il tuo account. Appena lo farà potrai entrare: ricarica la pagina o torna più tardi."],
    sospeso: ["Account sospeso", "Il Master ha sospeso temporaneamente il tuo account."],
    cancellazione_richiesta: ["Cancellazione richiesta", "Il Master sta gestendo la tua richiesta di cancellazione."],
  };
  const [t, d] = testi[u.stato] || ["Account non attivo", "Rivolgiti al Master."];
  const esci = async () => { try { await api("/uscita", { dati: {} }); } catch { /* già uscito */ } history.replaceState(null, "", "/"); avvio(); };
  svuota(radice, pagina(el("div", { class: "scheda-ui" }, el("h2", { testo: t }), el("p", { testo: "Ciao " + u.nome + "." }), el("p", { class: "tenue", testo: d }),
    el("div", { class: "riga-flex" },
      el("button", { testo: "Aggiorna", on: { click: avvio } }),
      el("a", { class: "pulsante", href: "/profilo", "data-interno": true, testo: "Profilo" }),
      el("a", { class: "pulsante", href: "/privacy", "data-interno": true, testo: "Privacy" }),
      el("button", { testo: "Esci", on: { click: esci } })))));
}
