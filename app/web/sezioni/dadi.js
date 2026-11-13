// Sezione Dadi: il simulatore a tutta pagina. Se c'è uno scontro in corso a cui partecipi, i tiri
// li vedono tutti i partecipanti (con gli effetti CRITICO e OOOPS); altrimenti solo tu.
import { el, svuota } from "../ui/ui.js";
import { pannelloDadi, editorSfortuna } from "../ui/dadi.js";

export async function mostra(corpo, ctx) {
  const sessione = new URLSearchParams(location.search).get("sessione") || "";
  svuota(corpo, el("div", { class: "largo", stile: { maxWidth: "820px" } },
    el("section", { class: "scheda-ui" }, pannelloDadi({ sessione, master: ctx.io.utente.ruolo === "master" })),
    ctx.io.utente.ruolo === "master" ? editorSfortuna() : null));
}
