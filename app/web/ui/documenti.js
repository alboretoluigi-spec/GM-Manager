// Notifica dei documenti ricevuti (2026-11-07): «[personaggio] ti ha consegnato un documento», con «Apri».
// Arriva in tempo reale; i documenti ricevuti mentre il giocatore era scollegato si annunciano al collegamento.
import { el, api, dialogo, tempoReale, fuocoInPrimoPiano } from "./ui.js";

let installato = false;
const mostrati = new Set();

function notifica(d) {
  if (!d || mostrati.has(d.id)) return;
  mostrati.add(d.id);
  const visto = () => api("/documenti/" + encodeURIComponent(d.id) + "/visto", { dati: {} }).catch(() => {});
  dialogo("📜 " + d.testo, el("div", { class: "consegna-doc" },
    el("p", {}, el("b", { testo: d.da }), " ti ha consegnato un documento: ", el("b", { testo: "«" + d.nome + "»" }), "."),
    el("p", { class: "tenue piccolo", testo: "Lo trovi sempre in 📁 Documenti." })), {
    azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { testo: "Più tardi", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Apri", on: { click: async () => {
        chiudi(); visto();
        const { apriDocumento } = await import("../sezioni/documenti.js");
        apriDocumento({ id: d.id, tipo: d.tipo });
      } } })),
  });
}

// «Mostra» (2026-11-09): un compagno (o il Master) fa comparire un suo documento a tutto schermo. Immagini e PDF si
// vedono subito; i documenti Word si leggono come testo. Si chiude con un tocco fuori, con «Chiudi» o con Esc.
async function mostrato(d) {
  if (!d || !d.id) return;
  document.querySelectorAll(".documento-mostrato").forEach((x) => x.remove());
  const url = "/api/v1/documenti/" + encodeURIComponent(d.id) + "/file";
  const corpo = el("div", { class: "dm-corpo" });
  const velo = el("div", { class: "documento-mostrato", role: "dialog", "aria-label": "Documento mostrato" },
    el("div", { class: "dm-testa" }, el("span", {}, el("b", { testo: d.da }), " ti mostra ", el("b", { testo: "«" + d.nome + "»" })),
      el("button", { class: "piccolo", testo: "✕ Chiudi" })),
    corpo);
  const chiudi = () => velo.remove();
  velo.__chiudi = chiudi; // Esc (ui.js)
  velo.querySelector(".dm-testa button").addEventListener("click", chiudi);
  velo.addEventListener("click", (e) => { if (e.target === velo) chiudi(); });
  document.body.append(velo);
  fuocoInPrimoPiano(velo);
  if (/^(png|jpg|gif|webp)$/.test(d.tipo)) corpo.append(el("img", { src: url, alt: d.nome }));
  else if (d.tipo === "pdf") corpo.append(el("iframe", { src: url, title: d.nome }));
  else {
    try {
      const { estraiTesto, mostraTesto } = await import("./testo_formattato.js");
      const b = await (await fetch(url, { credentials: "same-origin" })).blob();
      corpo.append(el("div", { class: "dm-testo" }, mostraTesto(await estraiTesto(new File([b], "documento." + d.tipo)))));
    } catch (e) { corpo.append(el("p", { class: "errore", testo: "Impossibile aprire il documento: " + e.message })); }
  }
}

export function installaDocumenti(io) {
  if (installato) return;
  installato = true;
  tempoReale.su("documento.consegnato", notifica);
  tempoReale.su("documento.mostrato", mostrato);
  tempoReale.su("benvenuto", async () => { // documenti ricevuti mentre si era scollegati
    const u = io(); if (!u) return;
    try {
      const l = (await api("/documenti")).lista.filter((d) => d.nuovo && d.proprietario === u.id);
      l.forEach((d) => notifica({ id: d.id, nome: d.nome, tipo: d.tipo, da: d.daPersonaggio || "Qualcuno", testo: (d.daPersonaggio || "Qualcuno") + " ti ha consegnato un documento" }));
    } catch { /* niente */ }
  });
}
