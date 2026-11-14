// Sezione Dadi: il simulatore a tutta pagina. Se c'è uno scontro in corso a cui partecipi, i tiri
// li vedono tutti i partecipanti (con gli effetti CRITICO e OOOPS); altrimenti solo tu.
import { el, svuota } from "../ui/ui.js";
import { pannelloDadi, editorSfortuna } from "../ui/dadi.js";
import { pannelloMeccanica, haMeccanica } from "../ui/meccaniche.js";

export async function mostra(corpo, ctx) {
  const sessione = new URLSearchParams(location.search).get("sessione") || "";
  // Tiro della famiglia del gioco della campagna attiva (interfaccia adattiva, C4c), sopra i dadi liberi.
  const g = ctx.io.gioco, tiro = g && g.modulo && g.modulo.tiro;
  const master = ctx.io.utente.ruolo === "master";
  svuota(corpo, el("div", { class: "largo", stile: { maxWidth: "820px" } },
    tiro && haMeccanica(tiro) ? el("section", { class: "scheda-ui mc-sezione" }, el("h2", { testo: "🎲 Tiro: " + g.modulo.nome }),
      el("p", { class: "tenue piccolo", testo: g.modulo.meccanica }), pannelloMeccanica(tiro, { sessione, master })) : null,
    el("section", { class: "scheda-ui" }, tiro ? el("h2", { testo: "Dadi liberi" }) : null, pannelloDadi({ sessione, master })),
    master && !tiro ? editorSfortuna() : null)); // la tabella della sfortuna è del d20 (quinta edizione e scheda libera)
}
