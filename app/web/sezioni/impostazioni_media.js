// Impostazioni → «🎬 Video leggeri per i telefoni» (fase M1, docs/PIANO_MEDIA_MANUALI.md, 2026-11-11).
// Con ffmpeg sul PC Gaster Bless prepara da sola, in sottofondo, la versione 720p dei video per i telefoni dei giocatori e le
// copie leggere (internal/server/media_preparazione.go). Qui si vede a che punto è e si può farla ripartire.
import { el, svuota, api, avviso } from "../ui/ui.js";

export const tempi = { aggiorna: 5000 };

const COMANDI = [
  "New-Item -ItemType Directory -Force C:\\Progetti\\Strumenti",
  "curl.exe -L -o C:\\Progetti\\Strumenti\\ffmpeg.zip https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip",
  "Expand-Archive C:\\Progetti\\Strumenti\\ffmpeg.zip -DestinationPath C:\\Progetti\\Strumenti\\ffmpeg -Force",
].join("\n");

export function sezioneMedia() {
  const corpo = el("div", {}, el("p", { class: "tenue", testo: "Caricamento…" }));
  const box = el("section", { class: "scheda-ui agg", id: "media-leggeri" }, el("h2", { testo: "🎬 Video leggeri per i telefoni" }),
    el("p", { class: "tenue", testo: "Al tavolo i telefoni ricevono una versione dei video più leggera (720p), il tuo PC e il proiettore quella di qualità. Gaster Bless la prepara da sola, in sottofondo, un video alla volta: gli originali non si toccano. Sul telefono il video parte dopo qualche secondo di caricamento, così non va a scatti." }),
    corpo);
  let timer = 0;

  async function carica() {
    clearTimeout(timer);
    if (!box.isConnected && timer) return; // pagina cambiata
    let s;
    try { s = await api("/media/preparazione"); } catch (e) { svuota(corpo, el("p", { class: "errore", testo: e.message })); return; }
    disegna(s);
    if (s.attivo) timer = setTimeout(carica, tempi.aggiorna);
  }

  function disegna(s) {
    if (!s.ffmpeg) {
      svuota(corpo, el("div", { class: "agg-esito agg-fallito" },
        el("b", { testo: "ffmpeg non è installato. " }),
        "Si installa una volta sola (gratuito). Clic destro su Start → «Terminale (amministratore)» e incolla, uno alla volta:",
        el("pre", { class: "agg-pre", testo: COMANDI }),
        "Poi premi «Prepara ora».",
        el("div", { class: "riga-flex" }, el("button", { class: "primario media-prepara", testo: "▶ Prepara ora", on: { click: prepara } }))));
      return;
    }
    const stato = s.attivo
      ? el("p", { class: "agg-attesa media-stato", testo: "⏳ In preparazione: " + s.fatte + " di " + s.totale + (s.corrente ? " — " + s.corrente : "") })
      : el("p", { class: "media-stato", testo: s.totale && s.fatte >= s.totale || s.pronte ? "✅ Tutto pronto: " + (s.pronte + s.fatte) + " versioni leggere." : "Nessun video o immagine da preparare." });
    svuota(corpo,
      el("p", { class: "piccolo tenue", testo: "ffmpeg: " + s.ffmpeg }),
      stato,
      s.errori && s.errori.length ? el("details", { class: "agg-esito agg-fallito" }, el("summary", { testo: "⚠ " + (s.errori.length === 1 ? "1 file non preparato (resta l'originale)" : s.errori.length + " file non preparati (restano gli originali)") }),
        el("ul", {}, s.errori.map((e) => el("li", { class: "piccolo", testo: e })))) : null,
      el("div", { class: "riga-flex" }, el("button", { class: "media-prepara", testo: "▶ Prepara ora", disabled: s.attivo, on: { click: prepara } }),
        el("span", { class: "tenue piccolo", testo: "Riparte da sola dopo ogni caricamento, dopo «Cerca nelle cartelle» e ogni 30 minuti." })));
  }

  async function prepara() {
    try { await api("/media/preparazione", { metodo: "POST", dati: {} }); avviso("Preparazione avviata: continua in sottofondo."); setTimeout(carica, 1500); }
    catch (e) { avviso(e.message, "errore", 8000); }
  }

  carica();
  return box;
}
