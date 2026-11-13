// Sezione "Mappa": la mappa di Gaster Bless (web/carta/, la pagina originale adattata) per la sessione scelta.
// La pagina parla con il server della mappa (internal/server/mappa.go): ogni giocatore riceve solo ciò che
// è svelato al suo gruppo.
import { el, svuota, api, sessioneScelta, ricordaSessione } from "../ui/ui.js";
import { mappaGrande } from "../ui/guscio.js";

export async function mostra(corpo, { io, azioni }) {
  const me = io.utente;
  let sessioni = [];
  try {
    for (const g of await api("/gruppi")) {
      const aperte = (await api("/gruppi/" + g.id + "/sessioni")).filter((s) => s.stato !== "conclusa" && (me.ruolo === "master" || s.partecipo));
      aperte.forEach((s) => sessioni.push({ ...s, nomeGruppo: g.nome }));
      // Il Master può preparare la mappa di un gruppo anche senza una sessione aperta; il giocatore la consulta
      // (solo ciò che è svelato, senza modificare nulla) tra una sessione e l'altra (2026-11-08).
      if (!aperte.length) sessioni.push({ id: "g:" + g.id, titolo: me.ruolo === "master" ? "preparazione (nessuna sessione aperta)" : "consultazione (nessuna sessione aperta)", stato: "", inizio: Infinity, nomeGruppo: g.nome });
    }
  } catch (e) { svuota(corpo, el("div", { class: "largo" }, el("p", { class: "errore", testo: e.message }))); return; }
  if (!sessioni.length) {
    svuota(corpo, el("div", { class: "largo" }, el("section", { class: "scheda-ui" }, el("h2", { testo: "Nessuna sessione aperta" }),
      el("p", { class: "tenue", testo: me.ruolo === "master"
        ? "La mappa si apre dentro una sessione: creane una nella sezione Sessioni. Nebbia e scoperte sono salvate per ogni gruppo."
        : "Non fai ancora parte di un gruppo: quando il Master ti aggiungerà, qui vedrai la mappa del tuo gruppo." }))));
    return;
  }
  // In corso prima, poi la più vicina.
  sessioni.sort((a, b) => (b.stato === "in_corso") - (a.stato === "in_corso") || a.inizio - b.inizio);
  let sessione = sessioneScelta(sessioni); // ?s=… oppure l'ultima scelta (ui.js)
  const frame = el("iframe", { class: "riquadro-scheda riquadro-mappa", title: "Mappa di Gaster Bless", allow: "fullscreen; window-management; autoplay" });
  // ?loc=<chiave>: apre subito la scheda di quella località (dal Codice della campagna, 2026-11-12)
  const loc = new URLSearchParams(location.search).get("loc");
  const apri = () => { frame.src = "/carta/?sessione=" + encodeURIComponent(sessione.id) + (loc ? "&loc=" + encodeURIComponent(loc) : ""); };
  const scelta = el("select", { "aria-label": "Sessione", on: { change: () => {
    sessione = sessioni.find((s) => s.id === scelta.value);
    history.replaceState(null, "", "/mappa?s=" + sessione.id); ricordaSessione(sessione.id);
    apri();
  } } }, sessioni.map((s) => el("option", { value: s.id, testo: s.nomeGruppo + " · " + s.titolo + (s.stato === "in_corso" ? " (in corso)" : ""), selected: s.id === sessione.id })));
  const grande = el("button", { class: "piccolo con-icona tasto-mappa-grande", "data-ico": "⛶", "aria-label": "Mappa grande", title: "Nasconde menu e intestazione: la mappa occupa tutto lo schermo (Esc per tornare)", testo: "⛶ Mappa grande",
    on: { click: () => mappaGrande(true) } });
  svuota(azioni, sessioni.length > 1 ? scelta : el("span", { class: "tenue piccolo sessione-mappa", testo: sessione.nomeGruppo + " · " + sessione.titolo }), grande);
  azioni.classList.add("compatte"); // sul telefono restano nella riga del titolo (base.css)
  // sul PC: barra in alto più bassa in questa pagina, così la mappa ha più spazio (richiesta dell'utente, 2026-11-12)
  const barra = azioni.closest(".intestazione");
  if (barra) barra.classList.add("intestazione-mappa");
  svuota(corpo, frame);
  apri();
  return () => { mappaGrande(false); azioni.classList.remove("compatte"); if (barra) barra.classList.remove("intestazione-mappa"); };
}
