// Informativa sulla privacy: breve, comprensibile, sempre raggiungibile (anche senza accesso).
// Nota: è un testo informativo preparato per il Master, non un parere legale.
import { el, svuota, api } from "../ui/ui.js";

export async function mostra(corpo) {
  let info = { versione: "", conservazioneMesi: 12, backupGiorni: 30 };
  try { info = await api("/privacy"); } catch { /* si mostra comunque il testo */ }
  const p = (t) => el("p", { testo: t });
  const voce = (titolo, ...testi) => el("section", {}, el("h3", { testo: titolo }), ...testi.map(p));
  svuota(corpo, el("div", { class: "largo", stile: { maxWidth: "760px" } }, el("article", { class: "scheda-ui" },
    el("h2", { testo: "Informativa sulla privacy di Gaster Bless" }),
    el("p", { class: "tenue", testo: "Versione " + info.versione + ". Gaster Bless è un'app privata per giocare a gioco di ruolo al tavolo del Master." }),
    voce("Chi conserva i tuoi dati",
      "I dati sono conservati dal Master della campagna, sul suo computer di casa. Non vengono inviati a servizi esterni, non sono venduti e non sono usati per pubblicità."),
    voce("Quali dati",
      "Il nome che scegli, il tuo numero di telefono, la password (salvata solo in forma cifrata e irreversibile: nessuno, nemmeno il Master, può leggerla), le schede dei tuoi personaggi e il background che carichi.",
      "Non chiediamo email, indirizzi o altri dati."),
    voce("Perché",
      "Per farti giocare (accesso, gruppi, schede, mappa e iniziativa) e, se lo vuoi, per avvisarti delle sessioni su WhatsApp. Gli avvisi li invia il Master a mano dal suo WhatsApp: puoi disattivarli dal tuo profilo."),
    voce("Come sono protetti",
      "Il numero di telefono è cifrato nel database e nell'app compare mascherato (per esempio +39 ••• ••• 4567). Solo il Master può vederlo per intero, e ogni volta resta traccia nel registro degli accessi. Il collegamento all'app è cifrato (HTTPS).",
      "Gli altri giocatori non vedono il tuo numero, le tue schede né il tuo background."),
    voce("Per quanto tempo",
      "Finché hai un account. Se non entri per " + info.conservazioneMesi + " mesi, il Master riceve un avviso e può cancellare l'account. I backup sono cifrati e vengono cancellati dopo " + info.backupGiorni + " giorni: un dato eliminato sparisce anche dai backup entro quel periodo."),
    voce("I tuoi diritti",
      "Dal tuo profilo puoi scaricare in qualsiasi momento tutti i tuoi dati, modificare nome e numero, disattivare gli avvisi e chiedere la cancellazione dell'account. La richiesta arriva al Master, che la conferma. Con la cancellazione vengono eliminati account, numero e background; le schede vengono eliminate o conservate in forma anonima, secondo la scelta del Master.",
      "Per qualsiasi domanda puoi rivolgerti direttamente al Master."),
  )));
}
