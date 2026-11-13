// Sezione «✨ Oggetti magici» del Master (richiesta dell'utente del 2026-11-11: «nell'app non trovo dove il master può
// inserire o creare oggetti magici»). Contiene la pagina dell'archivio (web/scheda/archivio_oggetti.html), che usa lo
// stesso editor della scheda, e le fa da tramite verso il server: solo le richieste dell'archivio degli oggetti
// (/oggetti-magici…) e del testo dei manuali (/elementi-manuali/testo), con la sessione e il token del Master.
import { api, el, svuota } from "../ui/ui.js";

const PERMESSE = [/^\/oggetti-magici$/, /^\/oggetti-magici\/[A-Za-z0-9_.-]{1,40}(\/consegna)?$/, /^\/elementi-manuali\/testo\?tipo=oggetto&voce=[A-Za-z0-9_.%-]{1,120}$/];
const METODI = ["GET", "PUT", "POST", "DELETE"];

function temaPagina() {
  const t = document.documentElement.dataset.tema;
  if (t === "chiaro") return "light";
  if (t === "scuro") return "dark";
  return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export async function mostra(corpo, ctx) {
  if (!ctx.io || !ctx.io.utente || ctx.io.utente.ruolo !== "master") {
    svuota(corpo, el("p", { class: "errore", testo: "Questa sezione è solo per il Master." }));
    return null;
  }
  const frame = el("iframe", { class: "riquadro-scheda", title: "Archivio degli oggetti magici", src: "/scheda/archivio_oggetti.html" });
  const invia = (m) => frame.contentWindow && frame.contentWindow.postMessage(m, location.origin);
  async function messaggio(e) {
    if (e.source !== frame.contentWindow || e.origin !== location.origin) return;
    const m = e.data || {};
    if (m.type === "arch:ready") { invia({ type: "arch:init", theme: temaPagina() }); return; }
    if (m.type !== "arch:request") return;
    const percorso = String(m.percorso || ""), metodo = String(m.metodo || "GET").toUpperCase();
    if (!METODI.includes(metodo) || !PERMESSE.some((re) => re.test(percorso))) {
      invia({ type: "arch:response", id: m.id, ok: false, error: "Richiesta non permessa." });
      return;
    }
    try {
      const data = await api(percorso, { metodo, ...(m.dati !== undefined ? { dati: m.dati } : {}) });
      invia({ type: "arch:response", id: m.id, ok: true, data });
    } catch (er) {
      invia({ type: "arch:response", id: m.id, ok: false, error: er.message || "Errore" });
    }
  }
  addEventListener("message", messaggio);
  // il tema dell'app cambia: la pagina dell'archivio lo segue
  const osserva = new MutationObserver(() => invia({ type: "arch:theme", theme: temaPagina() }));
  osserva.observe(document.documentElement, { attributes: true, attributeFilter: ["data-tema"] });
  svuota(corpo, frame);
  return () => { removeEventListener("message", messaggio); osserva.disconnect(); };
}
