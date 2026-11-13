// Icona di Gaster Bless sulla schermata Home (richiesta dell'utente, 2026-11-07).
// Al primo accesso da un dispositivo l'app invita a creare l'icona: le volte dopo basta toccarla (l'accesso resta
// salvato sul dispositivo). Dove il browser lo permette c'è il pulsante «Installa»; altrimenti le istruzioni giuste
// per quel telefono, tablet o PC. L'invito si riapre da «📲 Icona» nel menu.
import { el, dialogo, avviso } from "./ui.js";

const CHIAVE = "gasterbless.icona";
let richiesta = null; // evento beforeinstallprompt (Chrome, Edge, Samsung Internet)

/** Da chiamare all'avvio: il browser annuncia presto che l'app si può installare. */
export function preparaInstallazione() {
  window.addEventListener("beforeinstallprompt", (e) => { e.preventDefault(); richiesta = e; });
  window.addEventListener("appinstalled", () => {
    richiesta = null;
    ricorda("installata");
    avviso("Fatto: l'icona di Gaster Bless è sulla schermata Home. La prossima volta basta toccarla.", "", 7000);
  });
}

/** Gaster Bless è aperta dall'icona (come app), non da una scheda del browser. */
export const daIcona = () => matchMedia("(display-mode: standalone)").matches || matchMedia("(display-mode: fullscreen)").matches ||
  matchMedia("(display-mode: minimal-ui)").matches || navigator.standalone === true;

function ricorda(v) { try { localStorage.setItem(CHIAVE, v); } catch { /* memoria non disponibile */ } }
function giàVisto() { try { return !!localStorage.getItem(CHIAVE); } catch { return true; } }

function piattaforma() {
  const ua = navigator.userAgent;
  const ios = /iPhone|iPad|iPod/.test(ua) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
  return {
    ios, android: /Android/.test(ua),
    safari: ios && /Safari/.test(ua) && !/CriOS|FxiOS|EdgiOS|OPiOS|GSA/.test(ua),
    samsung: /SamsungBrowser/.test(ua), firefox: /Firefox|FxiOS/.test(ua),
    macSafari: !ios && /Macintosh/.test(ua) && /Safari/.test(ua) && !/Chrome|Chromium|Edg|Firefox/.test(ua),
  };
}

// Istruzioni passo passo quando il browser non offre il pulsante «Installa».
function istruzioni() {
  const p = piattaforma();
  if (p.ios && p.safari) return ["Tocca il pulsante Condividi (il quadrato con la freccia verso l'alto).", "Scorri e tocca «Aggiungi alla schermata Home».", "Tocca «Aggiungi»."];
  if (p.ios) return ["Su iPhone e iPad l'icona si crea da Safari: apri questo stesso indirizzo in Safari.", "Poi tocca Condividi → «Aggiungi alla schermata Home» → «Aggiungi».", "(Nelle versioni recenti funziona anche dal menu Condividi del tuo browser.)"];
  if (p.samsung) return ["Tocca il menu ≡ in basso a destra.", "Tocca «Aggiungi pagina a» → «Schermata Home».", "Conferma con «Aggiungi»."];
  if (p.android && p.firefox) return ["Tocca il menu ⋮.", "Tocca «Installa» oppure «Aggiungi alla schermata Home».", "Conferma."];
  if (p.android) return ["Tocca il menu ⋮ in alto a destra.", "Tocca «Installa app» oppure «Aggiungi a schermata Home».", "Conferma con «Installa» o «Aggiungi»."];
  if (p.macSafari) return ["Nel menu File (o Condividi) scegli «Aggiungi al Dock».", "Conferma: Gaster Bless compare nel Dock."];
  if (p.firefox) return ["Firefox sul PC non crea icone di app: aggiungi Gaster Bless ai preferiti,", "oppure apri questo indirizzo con Chrome o Edge e installala da lì."];
  return ["Nella barra dell'indirizzo cerca l'icona «Installa» (uno schermo con una freccia),", "oppure apri il menu ⋮ (Chrome) o … (Edge) → «Trasmetti, salva e condividi» / «App» → «Installa Gaster Bless».", "Conferma con «Installa»."];
}

// Quale indirizzo ricorderà l'icona, e cosa vuol dire per il giocatore.
function notaIndirizzo(io) {
  const ind = (io && io.indirizzi) || {};
  const qui = location.origin;
  const pubblico = ind.pubblico || "";
  if (pubblico && qui === pubblico.replace(/\/$/, "")) {
    return el("p", {}, el("b", { testo: "Icona per giocare ovunque. " }),
      "Usa l'indirizzo Internet di Gaster Bless: funziona da casa tua, fuori e anche a casa del Master, quando lui ha aperto l'accesso da Internet (di solito poco prima della partita).");
  }
  return el("div", {},
    el("p", {}, el("b", { testo: "Icona per giocare a casa del Master. " }),
      "Usa l'indirizzo di casa: funziona quando sei collegato al suo Wi-Fi."),
    pubblico ? el("p", {}, "Giochi anche da lontano? Apri ", el("a", { href: pubblico + "/", testo: pubblico.replace("https://", "") }),
      " e crea l'icona da lì: quella funziona sia a casa del Master sia fuori.") : null);
}

/** Apre l'invito. auto = al primo accesso (una volta per dispositivo). */
export function invitoInstallazione(io, auto = false) {
  if (daIcona()) { if (!auto) avviso("Stai già usando Gaster Bless dall'icona: non devi fare altro."); return; }
  if (auto && giàVisto()) return;
  ricorda("visto");
  const p = piattaforma();
  const passi = el("ol", { class: "icona-passi" }, istruzioni().map((x) => el("li", { testo: x })));
  const corpo = el("div", { class: "icona-invito" },
    el("div", { class: "icona-testa" },
      el("img", { src: "/img/icona-192.png", alt: "", class: "icona-anteprima", width: "64", height: "64" }),
      el("p", { testo: "Metti l'icona di Gaster Bless sulla schermata Home: le prossime volte basta toccarla, senza cercare l'indirizzo e senza rifare l'accesso." })),
    richiesta ? null : el("div", {}, el("h3", { testo: "Come fare su questo dispositivo" }), passi),
    notaIndirizzo(io),
    el("p", { class: "tenue piccolo", testo: "Il tuo accesso resta salvato su questo dispositivo per 30 giorni dall'ultima volta che usi Gaster Bless; se passa più tempo ti chiede di nuovo numero di telefono e password. Quando il telefono propone di salvare la password, accetta." }),
    p.ios ? el("p", { class: "tenue piccolo", testo: "Su iPhone e iPad la prima volta che apri l'icona inserisci di nuovo numero di telefono e password: l'icona ha una memoria separata da Safari. Poi resta salvato." }) : null);
  dialogo("📲 Gaster Bless sulla schermata Home", corpo, {
    azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { testo: auto ? "Più tardi" : "Chiudi", on: { click: chiudi } }),
      richiesta ? el("button", { class: "primario", testo: "📲 Installa Gaster Bless", on: { click: async () => {
        const r = richiesta; richiesta = null;
        try { await r.prompt(); const scelta = await r.userChoice; if (scelta && scelta.outcome === "accepted") chiudi(); }
        catch { avviso("Il browser non ha completato l'installazione: segui le istruzioni del menu.", "errore"); }
      } } }) : el("button", { class: "primario", testo: "Ho capito", on: { click: chiudi } })),
  });
}

/** Il pulsante «📲 Icona» si mostra solo se Gaster Bless non è già aperta dall'icona. */
export const mostraPulsanteIcona = () => !daIcona();
