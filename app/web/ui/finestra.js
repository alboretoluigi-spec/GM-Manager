// Finestre a parte dell'app (scheda completa o ridotta dal tracker, specifica del combattimento, fase D).
// Si apre una pagina di Gaster Bless senza menu (?finestra=1, web/ui/app.js); se il browser blocca le finestre (popup), avviso e
// apertura nella stessa pagina, come chiede la specifica.
import { avviso } from "./ui.js";

export function apriFinestra(url, nome, { larghezza = 480, altezza = 780 } = {}) {
  const indirizzo = url + (url.includes("?") ? "&" : "?") + "finestra=1";
  let w = null;
  try { w = window.open(indirizzo, "gasterbless-" + nome, "popup=yes,width=" + larghezza + ",height=" + altezza); } catch { w = null; }
  if (!w) {
    avviso("Il browser ha bloccato la finestra: si apre in questa pagina. Per la prossima volta consenti le finestre (popup) per Gaster Bless.", "errore", 8000);
    setTimeout(() => { location.href = indirizzo; }, 1200);
    return null;
  }
  try { w.focus(); } catch { /* ignora */ }
  return w;
}
