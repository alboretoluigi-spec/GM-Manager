// iPhone e iPad (2026-11-08): Safari apre le pagine di Gaster Bless anche senza il «certificato di casa» (toccando
// «Visita il sito»), ma non apre il canale in tempo reale (WebSocket) né salva l'app offline. Se dopo qualche secondo
// il canale non si è mai aperto, l'app spiega cosa fare e porta alla pagina del certificato (porta 7601, in HTTP).
import { tempoReale, el, dialogo } from "./ui.js";

export const suIOS = () => /iPhone|iPad|iPod/.test(navigator.userAgent) || (/Macintosh/.test(navigator.userAgent) && navigator.maxTouchPoints > 1);

let controllato = false;
export function controllaCertificatoIOS(io, attesa = 9000) {
  if (controllato || !suIOS() || !io || !io.collegato || io.daFuori || io.pubblica || !io.portaCertificato) return;
  controllato = true;
  let aperto = tempoReale.aperto;
  tempoReale.stato.addEventListener("aperto", () => { aperto = true; });
  setTimeout(() => {
    if (aperto) return;
    const url = "http://" + location.hostname + ":" + io.portaCertificato + "/";
    const pubblico = io.accessoPubblico && io.indirizzi && io.indirizzi.pubblico ? io.indirizzi.pubblico + location.pathname : "";
    dialogo("📱 Su iPhone serve il certificato di Gaster Bless", el("div", {},
      el("p", { testo: "Gaster Bless si è aperta, ma Safari blocca gli aggiornamenti in tempo reale (avvisi, combattimento, Mappa) finché il «certificato di casa» non è installato sul telefono. Si fa una volta sola, in 2 minuti." }),
      el("p", {}, el("a", { class: "pulsante primario", href: url, testo: "Installa il certificato" })),
      el("p", { class: "tenue piccolo", testo: "Si apre la pagina con i passi da seguire (in Safari). Alla fine tocca «Controlla ed entra in Gaster Bless»." }),
      pubblico ? el("div", {},
        el("p", { testo: "Oppure, senza installare nulla, usa l'indirizzo Internet di Gaster Bless (funziona anche qui al tavolo):" }),
        el("p", {}, el("a", { class: "pulsante", href: pubblico, testo: "Apri " + pubblico.replace(/^https:\/\//, "") })),
        el("p", { class: "tenue piccolo", testo: "La prima volta rientra con numero e password; poi puoi mettere l'icona sulla schermata Home da lì." })) : null));
  }, attesa);
}
