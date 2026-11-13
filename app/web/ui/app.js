// Avvio dell'app: decide cosa mostrare (accesso, attesa, app) e gestisce la navigazione tra le sezioni.
import { intestazione, logoBreve } from "./marchio.js";
import { el, svuota, api, impostaCSRF, tempoReale, avviso, applicaTema, temaSalvato, dialogo,
  DISPOSITIVI, dispositivoSalvato, applicaDispositivo, dispositivoSuggerito, layoutAttuale, PALETTE, paletteSalvata, applicaPalette, chiudiDialoghi } from "./ui.js";
import { installaEffettiDadi } from "./dadi.js";
import { installaMira } from "./mira.js";
import { installaScena, apriScena } from "./scena.js";
import { tastoMenu, tastoFissa, chiudiCassetto } from "./guscio.js";
import { avviaAggiornamento } from "./aggiorna.js";
import { installaSanita } from "./sanita.js";
import { installaMiniSanita } from "./sanita_mini.js";
import { installaIncantesimi } from "./incantesimi.js";
import { installaSaluto, mostraSaluto } from "./saluto.js";
import { preparaInstallazione, invitoInstallazione, mostraPulsanteIcona } from "./installa.js";
import { installaProfessioni } from "./professioni.js";
import { installaFinanze } from "./finanze.js";
import { installaControlli } from "./controlli.js";
import { installaModificheManuali } from "./modifiche_manuali.js";
import { tastiRiposo, installaRiposi } from "./riposi.js";
import { installaChat } from "./chat.js";
import { installaDocumenti } from "./documenti.js";
import { installaIndebolimento } from "./indebolimento.js";
import { installaViaggi } from "./viaggio.js";
import { controllaCertificatoIOS } from "./iphone.js";
import { salvaIo, ioSalvato, sincronizza } from "./offline.js";
import { installaPovero } from "./povero.js";
import { mostraBenvenuto, benvenutoGiaVisto } from "./benvenuto.js";

// Aspetto scelto su questo dispositivo: layout (PC, tablet, cellulare) e palette di colori.
applicaDispositivo(dispositivoSalvato(), false);
preparaInstallazione(); // il browser annuncia presto se Gaster Bless si può installare come app
if (paletteSalvata()) applicaPalette(paletteSalvata()); else applicaTema(temaSalvato());

const radice = document.getElementById("app");
export const stato = { io: null };

// Sezioni dell'app: percorso, etichetta, icona, chi le vede, modulo da caricare.
const SEZIONI = [
  { p: "/sessioni", t: "Sessioni", i: "📅", chi: "tutti", m: "sessioni", g: "gioco" },
  { p: "/documenti", t: "Documenti", i: "📁", chi: "tutti", m: "documenti", g: "personaggi" }, // PDF, Word e immagini (2026-11-07)
  { p: "/mappa", t: "Mappa", i: "🗺", chi: "tutti", m: "mappa", g: "gioco" },
  { p: "/iniziativa", t: "Iniziativa", i: "⚔", chi: "tutti", m: "iniziativa", g: "gioco" },
  { p: "/schede", t: "Schede", i: "📜", chi: "tutti", m: "schede", g: "personaggi" },
  { p: "/background", t: "Background", i: "📔", chi: "tutti", m: "background", g: "personaggi" }, // storie dei personaggi (2026-11-09)
  { p: "/sanita", t: "Sanità mentale", i: "🧠", chi: "tutti", m: "sanita", g: "personaggi" },
  { p: "/dadi", t: "Dadi", i: "🎲", chi: "tutti", m: "dadi", g: "gioco" },
  { p: "/riassunti", t: "Riassunti", i: "📝", chi: "tutti", m: "riassunti", g: "campagna" },
  { p: "/manuali", t: "Manuali", i: "📚", chi: "tutti", m: "manuali", g: "campagna" },
  { p: "/iscrizioni", t: "Iscrizioni", i: "👥", chi: "master", m: "iscrizioni", g: "campagna" },
  { p: "/avvisi", t: "Avvisi", i: "📣", chi: "master", m: "avvisi", g: "campagna" },
  { p: "/preparazione", t: "Preparazione sessione", i: "🧭", chi: "master", m: "preparazione", g: "campagna" }, // fase L5 del Codice (2026-11-12)
  { p: "/assistente", t: "Assistente IA", i: "🔮", chi: "master", m: "assistente", g: "campagna" }, // IA del Master (2026-11-09)
  { p: "/codice", t: "Codice della campagna", i: "🗝", chi: "master", m: "lore", g: "campagna" }, // lore del Master (2026-11-12)
  { p: "/bestiario", t: "PNG e mostri", i: "📖", chi: "master", m: "bestiario", g: "campagna" },
  { p: "/oggetti-magici", t: "Oggetti magici", i: "✨", chi: "master", m: "oggetti_magici", g: "campagna" }, // archivio del Master (2026-11-11)
  { p: "/strumenti", t: "Strumenti e kit", i: "🧰", chi: "master", m: "strumenti", g: "campagna" },
  { p: "/finanze", t: "Finanze", i: "💰", chi: "tutti", m: "finanze", g: "personaggi" },
  { p: "/professioni", t: "Professioni", i: "⚒", chi: "professioni", m: "professioni", g: "personaggi" }, // giocatori: solo con competenza e strumento
  { p: "/impostazioni", t: "Impostazioni", i: "⚙", chi: "master", m: "impostazioni", g: "account" },
  { p: "/profilo", t: "Profilo", i: "👤", chi: "tutti", m: "profilo", g: "account" },
  { p: "/privacy", t: "Privacy", i: "🔒", chi: "pubblica", m: "privacy", g: "account" },
  // Scheda ridotta di un combattente in una finestra a parte (dal tracker, 2026-11-11): non compare nei menu.
  { p: "/combattente", t: "Scheda di combattimento", i: "⚔", chi: "tutti", m: "combattente", g: "gioco", nascosta: true },
];
// Sul telefono la barra in basso ha 4 voci; le altre stanno in "Altro".
const BARRA = ["/mappa", "/iniziativa", "/schede", "/altro"];

let pulizia = null;

export function vai(percorso, sostituisci = false) {
  if (location.pathname + location.search !== percorso) {
    sostituisci ? history.replaceState(null, "", percorso) : history.pushState(null, "", percorso);
  }
  mostra();
}
window.addEventListener("popstate", () => mostra());
document.addEventListener("click", (e) => {
  const a = e.target.closest("a[data-interno]");
  if (!a || e.ctrlKey || e.metaKey || e.shiftKey) return;
  e.preventDefault();
  chiudiDialoghi(); // un collegamento dentro una finestra (per esempio dal PNG al Codice): la finestra si chiude
  vai(a.getAttribute("href"));
});
// Dalla mappa (iframe) al Codice della campagna: «gasterbless:vai» con un percorso interno ammesso (2026-11-12)
addEventListener("message", (e) => {
  if (e.origin !== location.origin || !e.data || e.data.type !== "gasterbless:vai") return;
  const p = String(e.data.percorso || "");
  if (/^\/(codice|bestiario|background)(\?[A-Za-z0-9=&%._-]*)?$/.test(p)) { chiudiDialoghi(); vai(p); }
});
document.addEventListener("gasterbless:sessione-scaduta", () => { if (stato.io && stato.io.collegato) avvio(); });

export async function avvio() {
  try {
    stato.io = await api("/io");
  } catch (e) {
    // Senza rete, con un accesso già fatto su questo dispositivo: si lavora sulle copie delle schede (offline.js).
    const c = e.stato === 0 ? ioSalvato() : null;
    if (c) return avvioOffline(c);
    svuota(radice, el("div", { class: "caricamento" }, intestazione(), el("p", { testo: e.message })));
    return;
  }
  impostaCSRF(stato.io.csrf);
  salvaIo(stato.io);
  avviaAggiornamento(stato.io.versione, window.__gasterblessOgni || undefined); // ogni 5 minuti: versione nuova → ricarica (aggiorna.js)
  if (stato.io.collegato && stato.io.utente && stato.io.utente.ruolo !== "master" && stato.io.utente.stato === "approvato") {
    sincronizza(stato.io.utente.id).then((n) => { if (n) avviso("📶 Di nuovo collegato: " + (n === 1 ? "una scheda modificata" : n + " schede modificate") + " senza connessione è stata inviata al Master.", "", 7000); });
  }
  if (pcDelMaster()) setTimeout(() => import("./pagine_manuali.js").then((m) => m.preparaPagine()).catch(() => {}), 20000); // pagine dei manuali per i telefoni
  if (stato.io.collegato && stato.io.utente.stato === "approvato") {
    tempoReale.avvia();
    import("./incantesimo_scontro.js").then((m) => m.ascolta(stato.io)).catch(() => {}); // incantesimi nello scontro: conferme ed esiti
    controllaCertificatoIOS(stato.io); // iPhone senza certificato: il canale in tempo reale non si apre
  } else tempoReale.ferma();
  mostra();
}

// Modalità senza connessione: solo le schede (copie sul dispositivo); ogni 10 secondi si riprova il server e, appena
// risponde, si ricarica l'app (le modifiche rimaste partono da sole con sincronizza).
let riprova = 0;
function avvioOffline(c) {
  stato.io = { ...c, offline: true };
  tempoReale.ferma();
  if (!location.pathname.startsWith("/schede")) history.replaceState(null, "", "/schede");
  clearInterval(riprova);
  riprova = setInterval(async () => {
    try { await api("/io"); clearInterval(riprova); location.reload(); } catch { /* ancora senza rete */ }
  }, 10000);
  mostra();
}

async function mostra() {
  if (pulizia) { try { pulizia(); } catch { /* ignora */ } pulizia = null; }
  const io = stato.io;
  const p = location.pathname;
  if (p === "/privacy") return sezione("privacy", io && io.collegato);
  if (!io.collegato) {
    const mod = await import("../sezioni/accesso.js");
    return mod.mostra(radice, io);
  }
  const u = io.utente;
  if (u.stato !== "approvato") {
    if (p === "/profilo") return sezione("profilo", true);
    const mod = await import("../sezioni/accesso.js");
    return mod.attesa(radice, io);
  }
  // Finestra a parte aperta dal tracker («?finestra=1»: scheda completa o ridotta): senza menu, senza benvenuto né domande.
  const finestra = new URLSearchParams(location.search).has("finestra");
  if (finestra && !io.offline) {
    const s = SEZIONI.find((x) => p === x.p || p.startsWith(x.p + "/"));
    if (s && visibile(s, u)) return sezione(s.m, "finestra");
  }
  // Giocatore che entra in Gaster Bless: video «Bentornato in GASTER BLESS» (una volta per apertura dell'app; il Master no).
  if (u.ruolo !== "master" && !benvenutoGiaVisto()) mostraBenvenuto();
  // Primo accesso da questo dispositivo: prima la scelta del dispositivo, poi l'invito a mettere l'icona sulla
  // schermata Home (una volta per dispositivo; poi resta il pulsante «📲 Icona» nel menu).
  if (!dispositivoSalvato()) setTimeout(() => aspetto(true, () => setTimeout(() => invitoInstallazione(stato.io, true), 300)), 0);
  else setTimeout(() => { if (!document.querySelector(".velo")) invitoInstallazione(stato.io, true); }, 1200);
  if (io.offline && !p.startsWith("/schede")) return vai("/schede", true); // senza rete: solo le schede
  if (p === "/" || p === "/altro" && layoutAttuale() !== "cellulare") return vai("/sessioni", true);
  if (p === "/altro") return altro();
  const s = SEZIONI.find((x) => p === x.p || p.startsWith(x.p + "/"));
  if (!s || !visibile(s, u)) return vai("/sessioni", true);
  return sezione(s.m, true);
}

/** Pulsante «Scarica Guida»: la guida in PDF più aggiornata, del Master per il Master e per i giocatori per tutti gli altri. */
function tastoGuida() {
  const master = !!(stato.io && stato.io.utente && stato.io.utente.ruolo === "master");
  return el("a", { href: "/api/v1/guida", download: master ? "GasterBless_Guida_Master.pdf" : "GasterBless_Guida_Giocatori.pdf",
    class: "pulsante piccolo tasto-guida con-icona", "data-ico": "📘", testo: "📘 Scarica Guida",
    title: master ? "Scarica la guida del Master (PDF, la versione più aggiornata)" : "Scarica la guida per i giocatori (PDF, la versione più aggiornata)" });
}

/** Disegna il guscio (menu + intestazione) e carica il modulo della sezione. */
async function sezione(modulo, conGuscio) {
  const s = SEZIONI.find((x) => x.m === modulo);
  chiudiCassetto(); // menu a scomparsa: si richiude scegliendo una voce
  const corpo = el("main", { class: "corpo", id: "corpo" });
  const titolo = el("h1", { testo: s ? s.t : "" });
  const azioni = el("div", { class: "riga-flex", id: "azioni-intestazione" });
  if (conGuscio) {
    svuota(radice, el("div", { class: "guscio" }, menu(), el("div", { class: "contenuto" },
      el("header", { class: "intestazione non-stampare" }, tastoMenu(), titolo, el("span", { class: "spazio" }), azioni,
        !(stato.io && stato.io.offline) ? tastiRiposo() : null, // riposo lungo e breve (2026-11-11)
        modulo !== "documenti" && !(stato.io && stato.io.offline) ? el("a", { href: "/documenti", "data-interno": true, class: "pulsante piccolo tasto-documenti con-icona", "data-ico": "📁", testo: "📁 Documenti", title: "I tuoi documenti (PDF, Word, immagini)" }) : null,
        // «🎭 PNG in scena» (2026-11-12, solo Master): un clic su un PNG e la sua immagine va sugli schermi dei giocatori
        stato.io && stato.io.utente && stato.io.utente.ruolo === "master" ? el("button", { class: "piccolo tasto-scena con-icona", "data-ico": "🎭", testo: "🎭 PNG in scena",
          title: "Mostra ai giocatori l'immagine del PNG che stai interpretando", on: { click: () => apriScena() } }) : null,
        // «Scarica Guida» (2026-11-12): il server sceglie la guida in base a chi la chiede (internal/server/guida.go)
        !(stato.io && stato.io.offline) ? tastoGuida() : null),
      corpo)), barraBasso());
  } else if (conGuscio === "finestra") { // finestra a parte: solo titolo, azioni della sezione e contenuto
    document.documentElement.dataset.finestra = "1";
    svuota(radice, el("div", { class: "guscio-finestra" }, el("header", { class: "intestazione" }, titolo, el("span", { class: "spazio" }), azioni), corpo));
  } else {
    svuota(radice, corpo);
  }
  if (stato.io && stato.io.offline) {
    corpo.before(el("div", { class: "fascia-offline", role: "status", testo: "📴 Senza connessione con il PC del Master: puoi consultare e modificare le tue schede. Le modifiche restano su questo dispositivo e partono appena Gaster Bless risponde; il Master ne sarà avvisato." }));
  }
  try {
    const mod = await import("../sezioni/" + modulo + ".js");
    pulizia = (await mod.mostra(corpo, { io: stato.io, titolo, azioni, vai })) || null;
  } catch (e) {
    console.error(e);
    svuota(corpo, el("div", { class: "largo" }, el("p", { class: "errore", testo: "Impossibile aprire la sezione: " + e.message })));
  }
}

function menu() {
  const u = stato.io.utente;
  const voci = SEZIONI.filter((s) => !s.nascosta && (s.chi === "pubblica" || visibile(s, u)));
  return el("nav", { class: "menu non-stampare", "aria-label": "Menu principale" },
    logoBreve(),
    tastoFissa(),
    GRUPPI_MENU.map((g) => gruppoMenu(g, voci.filter((s) => s.g === g.id))),
    el("div", { class: "piede" },
      el("div", { class: "tenue", testo: u.nome + (u.ruolo === "master" ? " · Master" : "") }),
      el("div", { class: "riga-flex" },
        el("button", { class: "piccolo", testo: "🎨 Aspetto", title: "Dispositivo e colori", on: { click: () => aspetto() } }),
        mostraPulsanteIcona() ? el("button", { class: "piccolo", testo: "📲 Icona", title: "Gaster Bless sulla schermata Home", on: { click: () => invitoInstallazione(stato.io) } }) : null,
        el("button", { class: "piccolo", testo: "Esci", on: { click: esci } }))));
}

// Menu più corto (richiesta dell'utente, 2026-11-08): voci raggruppate; «Gioco» sempre visibile, gli altri gruppi si
// aprono e chiudono con un tocco (la scelta resta salvata sul dispositivo; il gruppo della pagina aperta è sempre aperto).
const GRUPPI_MENU = [
  { id: "gioco", t: "Gioco", fisso: true },
  { id: "personaggi", t: "Personaggi", aperto: true },
  { id: "campagna", t: "Campagna" },
  { id: "account", t: "Account" },
];
function statoGruppi() { try { return JSON.parse(localStorage.getItem("gasterbless.menu.gruppi") || "{}"); } catch { return {}; } }
function vocemenu(s) {
  return el("a", { href: s.p, "data-interno": true, class: location.pathname.startsWith(s.p) ? "attivo" : "" },
    el("span", { "aria-hidden": "true", testo: s.i }), s.t,
    s.m === "iscrizioni" && stato.io.iscrizioniInAttesa ? el("span", { class: "distintivo", testo: stato.io.iscrizioniInAttesa }) : null);
}
function gruppoMenu(g, voci) {
  if (!voci.length) return null;
  if (g.fisso) return el("div", { class: "gruppo-menu" }, voci.map(vocemenu));
  const qui = voci.some((s) => location.pathname.startsWith(s.p));
  const salvato = statoGruppi()[g.id];
  const avvisi = g.id === "campagna" && stato.io.iscrizioniInAttesa;
  const d = el("details", { class: "gruppo-menu", open: qui || (salvato === undefined ? !!g.aperto : salvato) },
    el("summary", {}, g.t, avvisi && !qui ? el("span", { class: "distintivo", testo: stato.io.iscrizioniInAttesa }) : null), voci.map(vocemenu));
  d.addEventListener("toggle", () => { const st = statoGruppi(); st[g.id] = d.open; try { localStorage.setItem("gasterbless.menu.gruppi", JSON.stringify(st)); } catch { /* ignora */ } });
  return d;
}

function barraBasso() {
  return el("nav", { class: "barra-basso non-stampare", "aria-label": "Navigazione" },
    BARRA.map((p) => {
      const s = SEZIONI.find((x) => x.p === p) || { t: "Altro", i: "☰" };
      return el("a", { href: p, "data-interno": true, class: location.pathname.startsWith(p) ? "attivo" : "" },
        el("span", { class: "ico", "aria-hidden": "true", testo: s.i }), s.t);
    }));
}

/** Chi vede una sezione: «professioni» il Master sempre, il giocatore solo se un suo personaggio ne ha una. */
function visibile(s, u) {
  if (s.chi === "tutti" || s.chi === "pubblica") return true;
  if (u.ruolo === "master") return true;
  return s.chi === "professioni" && (stato.io.professioni || 0) > 0;
}

function altro() {
  const u = stato.io.utente;
  const corpo = el("div", { class: "largo" },
    SEZIONI.filter((s) => !s.nascosta && !BARRA.includes(s.p) && (s.chi === "pubblica" || visibile(s, u))).map((s) =>
      el("a", { href: s.p, "data-interno": true, class: "pulsante", stile: { width: "100%", justifyContent: "flex-start", marginBottom: "8px" } }, s.i + "  " + s.t)),
    el("div", { class: "riga-flex" }, el("button", { testo: "🎨 Aspetto", on: { click: () => aspetto() } }),
      mostraPulsanteIcona() ? el("button", { testo: "📲 Icona sulla schermata Home", on: { click: () => invitoInstallazione(stato.io) } }) : null,
      el("button", { testo: "Esci", on: { click: esci } })));
  svuota(radice, el("div", { class: "guscio" }, el("div", { class: "contenuto" },
    el("header", { class: "intestazione" }, el("h1", { testo: "Altro" })), corpo)), barraBasso());
}

/** Finestra "Aspetto": su quale dispositivo si usa l'app (tre layout) e con quale palette di colori.
 *  Al primo accesso da un dispositivo si apre da sola con la scelta consigliata già applicata. */
let aspettoAperto = false;
function aspetto(primaVolta = false, poi = null) {
  if (aspettoAperto) return;
  aspettoAperto = true;
  const consigliato = dispositivoSuggerito();
  if (primaVolta) applicaDispositivo(consigliato); // chi chiude senza scegliere tiene quella consigliata
  let disegnaDisp, disegnaPal;
  const dispBox = el("div", { class: "aspetto-disp", role: "radiogroup", "aria-label": "Dispositivo" });
  const palBox = el("div", { class: "aspetto-pal", role: "radiogroup", "aria-label": "Palette di colori" });
  disegnaDisp = () => {
    const att = document.documentElement.dataset.dispositivo || "";
    svuota(dispBox, DISPOSITIVI.map((d) => el("button", {
      type: "button", role: "radio", "aria-checked": String(att === d.id), class: "aspetto-voce" + (att === d.id ? " scelto" : ""), "data-disp": d.id,
      on: { click: () => { applicaDispositivo(d.id); disegnaDisp(); mostra(); } } },
      el("span", { class: "aspetto-ico", "aria-hidden": "true", testo: d.i }),
      el("strong", { testo: d.t + (d.id === consigliato ? " (consigliato)" : "") }),
      el("span", { class: "tenue", testo: d.d }))));
  };
  disegnaPal = () => {
    const att = document.documentElement.dataset.palette || "";
    svuota(palBox, PALETTE.map((p) => el("button", {
      type: "button", role: "radio", "aria-checked": String(att === p.id), class: "aspetto-voce pal" + (att === p.id ? " scelto" : ""), "data-pal": p.id || "auto",
      title: p.d || "", on: { click: () => { applicaPalette(p.id); disegnaPal(); } } },
      el("span", { class: "campioni", "aria-hidden": "true" }, p.c.map((c) => el("span", { stile: { background: c } }))),
      el("strong", { testo: p.t }))));
  };
  disegnaDisp(); disegnaPal();
  const chiudiTutto = dialogo(primaVolta ? "Benvenuto! Su quale dispositivo usi Gaster Bless?" : "🎨 Aspetto", el("div", {},
    primaVolta ? el("p", { testo: "Scegli il dispositivo che hai in mano: l'app si sistema per lui. Puoi cambiarlo quando vuoi da “🎨 Aspetto” nel menu." }) : null,
    el("h3", { testo: "Dispositivo" }), dispBox,
    el("h3", { testo: "Colori", stile: { marginTop: "14px" } }), palBox,
    el("p", { class: "tenue", testo: "Le scelte valgono solo per questo dispositivo." })), {
    largo: true,
    azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { class: "primario", testo: "Fatto", on: { click: chiudi } })),
  });
  // il dialogo si può chiudere anche con Esc o cliccando fuori: ci accorgiamo quando sparisce
  const oss = new MutationObserver(() => { if (!document.querySelector(".aspetto-disp")) { aspettoAperto = false; oss.disconnect(); if (poi) poi(); } });
  oss.observe(document.body, { childList: true });
  return chiudiTutto;
}

async function esci() {
  // Il giocatore che esce vede il saluto per 3 secondi (richiesta dell'utente, 2026-11-06); il Master no.
  const u = stato.io && stato.io.utente;
  if (u && u.ruolo !== "master") await mostraSaluto("Alla prossima sessione...", { durata: 3000 });
  tempoReale.ferma(); // prima di uscire: altrimenti il canale prova a ricollegarsi senza più l'accesso
  try { await api("/uscita", { dati: {} }); } catch { /* già uscito */ }
  history.replaceState(null, "", "/");
  avvio();
}

// Avvisi in tempo reale per il Master (nuove iscrizioni, dispositivi nuovi, richieste privacy).
tempoReale.su("iscrizioni.nuova", (d) => { avviso("Nuova richiesta di iscrizione: " + d.nome); if (stato.io) stato.io.iscrizioniInAttesa = (stato.io.iscrizioniInAttesa || 0) + 1; });
tempoReale.su("accesso.nuovo_dispositivo", (d) => avviso("Accesso da un dispositivo nuovo: " + d.nome + " (" + d.dispositivo + ")", "", 6000));
// Il Master crea o avvia lo scontro: i giocatori della sessione vanno da soli sul tracker; quando proietta sui loro
// dispositivi vanno sulla Mappa, dove compare la proiezione (saluto.go, chiamaGiocatori). Richiesta dell'utente.
function chiamata(pagina, testo) {
  return (d) => {
    const u = stato.io && stato.io.utente;
    if (!u || u.ruolo === "master" || !d || !d.sessione) return;
    // già sulla pagina giusta con questa sessione (dall'indirizzo, o da ciò che la pagina mostra davvero)?
    const mostrata = pagina === "/mappa" ? new URLSearchParams((document.querySelector("iframe.riquadro-mappa")?.src || "").split("?")[1] || "").get("sessione")
      : document.querySelector("#corpo [data-sessione]")?.dataset.sessione;
    const qui = location.pathname.startsWith(pagina) && (new URLSearchParams(location.search).get("s") === d.sessione || mostrata === d.sessione);
    if (!qui) { document.querySelectorAll(".velo").forEach((v) => v.remove()); vai(pagina + "?s=" + encodeURIComponent(d.sessione)); }
    if (testo) avviso(typeof testo === "function" ? testo(d) : testo, "", 4000);
  };
}
tempoReale.su("iniz.chiamata", chiamata("/iniziativa", (d) => d.fase === "inizio" ? "⚔ Il combattimento è iniziato!" : "⚔ Si combatte! Tira l'iniziativa."));
tempoReale.su("mappa.chiamata", chiamata("/mappa", "📺 Il Master sta proiettando sulla mappa."));
tempoReale.su("arcana.avviso", (d) => avviso(d.testo, "", 6000));
tempoReale.su("riassunto.ottimo", (d) => avviso(d.testo, "", 8000)); // «Ottimo lavoro» del Master sul tuo riassunto // scheda esterna: collegamenti, sessioni, incantesimi, note (arcana.go)
tempoReale.su("assistente.nuova", (d) => { if (location.pathname !== "/assistente") avviso("🧛 L'assistente ha nuove idee per l'antagonista (" + d.gruppo + "): apri «Assistente IA».", "", 9000); }); // dopo un riassunto nuovo (agganci_ia.go)
// Il Master sul PC di Gaster Bless: prepara le pagine dei manuali per i telefoni (pagine_manuali.js, 2026-11-11).
function pcDelMaster() { return !!(stato.io && stato.io.collegato && stato.io.utente && stato.io.utente.ruolo === "master" && stato.io.locale); }
tempoReale.su("manuali.aggiornati", () => { if (pcDelMaster()) import("./pagine_manuali.js").then((m) => m.preparaPagine()).catch(() => {}); }); // condivisione cambiata
tempoReale.su("sistema.aggiornamento", (d) => avviso("🔄 " + d.testo, "", 15000)); // il Master ha premuto «Aggiorna» (aggiornamento.go)
tempoReale.su("privacy.richiesta", (d) => avviso(d.nome + " ha chiesto la cancellazione dell'account.", "", 6000));

// Effetti dei tiri (CRITICO, OOOPS, sfortuna) su qualunque pagina dell'app (da web/ui/dadi.js).
installaEffettiDadi(() => stato.io && stato.io.utente);
installaMira(() => stato.io && stato.io.utente); // «X prende di mira Y» nel combattimento (2026-11-12)
installaScena(() => stato.io && stato.io.utente); // «🎭 PNG in scena» sugli schermi (2026-11-12)
// Sanità mentale: il Master vede la schermata quando un giocatore la apre; i giocatori sono avvisati degli eventi.
installaSanita(() => stato.io && stato.io.utente);
installaMiniSanita(); // risultato del tiro salvezza su Sanità mentale al Master e al giocatore
installaProfessioni(); // esiti di raccolta e fabbricazione al Master e al giocatore
installaFinanze(() => stato.io && stato.io.utente); // richieste da autorizzare (Master), decisioni e movimenti
installaPovero(); // monete insufficienti: immagine per 2 secondi
installaViaggi(); // viaggi del segnalino del gruppo: avviso a tutto schermo con gli eventi
// Codice della campagna (fase L3): un evento con «Innesco» è scattato per un gruppo → avviso al Master in ogni pagina
tempoReale.su("lore.promemoria", (d) => {
  if (!d || !stato.io || !stato.io.utente || stato.io.utente.ruolo !== "master") return;
  avviso("🔔 " + (d.nomeGruppo || "Gruppo") + ": «" + d.nome + "» — " + d.motivo + ". Lo trovi in Codice della campagna → Promemoria.", "", 15000);
  document.dispatchEvent(new CustomEvent("gasterbless:promemoria", { detail: d }));
});
installaIndebolimento(); // un livello di indebolimento in più: schermo nero e immagine «stanco» per 1 secondo
installaControlli(() => stato.io && stato.io.utente);
installaModificheManuali(() => stato.io && stato.io.utente); // «Modifica manuale» delle schede (2026-11-11)
installaRiposi(() => stato.io && stato.io.utente); // riposo lungo e breve con autorizzazione del Master (2026-11-11)
installaChat(() => stato.io && stato.io.utente); // messaggi privati: una finestra per giocatore, suono e avviso (2026-11-11)
installaDocumenti(() => stato.io && stato.io.utente); // documenti consegnati: notifica al giocatore // controllo delle schede all'accesso in sessione (avvisi al Master)
// Incantesimi lanciati dalla scheda: la descrizione compare sugli schermi del tavolo.
installaIncantesimi(() => stato.io && stato.io.utente);
// «Saluta il gruppo»: a fine serata il Master apre l'immagine di saluto su tutti gli schermi della sessione.
installaSaluto();

if ("serviceWorker" in navigator && window.isSecureContext) navigator.serviceWorker.register("/sw.js").catch(() => {});
avvio();
