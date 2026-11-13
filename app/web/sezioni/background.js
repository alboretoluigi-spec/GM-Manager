// Sezione «Background» (2026-11-09).
// Il giocatore carica la storia del suo personaggio (testo, Word o PDF) o la scrive: Gaster Bless la impagina nel suo stile.
// La vedono il giocatore e il Master; il giocatore può condividerla con alcuni giocatori o con tutto il gruppo.
// Il Master può caricarla al posto del giocatore (resta del giocatore) e per ogni background vede la «Scheda per il
// Master»: sintesi, agganci per l'antagonista e opportunità dalle ultime sessioni, scritti dall'assistente IA (agganci_ia.go).
import { el, svuota, api, avviso, conferma, dialogo, tempoReale } from "../ui/ui.js";
import { estraiTesto, mostraTesto, ACCETTA_TESTI } from "../ui/testo_formattato.js";
import { caricaCodice, nomiDi, norma, linkCodice } from "../ui/codice_link.js";

const MAX = 60000;

export async function mostra(corpo, { io, azioni }) {
  const master = io.utente.ruolo === "master";
  const box = el("div", { class: "largo background-sez" });
  const editor = el("div", {}), lista = el("div", {});
  svuota(corpo, box);
  svuota(box, editor, lista);
  let dati = { lista: [], personaggi: [], noteSenzaBackground: [] };
  // Riquadri aperti dal Master: restano aperti quando la pagina si aggiorna da sola (per esempio l'IA che finisce).
  const aperti = new Set();
  const ricorda = (k) => ({ toggle: (e) => { if (e.target.open) aperti.add(k); else aperti.delete(k); } });

  async function ricarica() {
    dati = await api("/background-testi");
    disegna();
  }

  function disegna() {
    const miei = dati.lista.filter((b) => b.modificabile), altri = dati.lista.filter((b) => !b.modificabile);
    svuota(lista,
      !dati.lista.length ? el("section", { class: "scheda-ui" }, el("h2", { testo: "Nessun background" }),
        el("p", { class: "tenue", testo: master
          ? "Quando un giocatore carica la storia del suo personaggio la trovi qui, con la scheda riassuntiva per te. Puoi anche caricarla tu al suo posto con «＋ Carica background»."
          : "Carica la storia del tuo personaggio con «＋ Carica background»: un file di testo, Word o PDF, oppure scrivila qui. La vedete solo tu e il Master, finché non decidi di condividerla." })) : null,
      miei.map(scheda),
      altri.length ? el("h2", { class: "titolo-sezione", testo: "Condivisi con te" }) : null,
      altri.map(scheda),
      // Appunti del Master su personaggi senza background (2026-11-09, assistente IA): solo il Master.
      (dati.noteSenzaBackground || []).length ? el("h2", { class: "titolo-sezione", testo: "Appunti del Master su altri personaggi" }) : null,
      (dati.noteSenzaBackground || []).map((n) => el("article", { class: "scheda-ui background-card", "data-pg": n.personaggio },
        el("h2", { testo: n.nome }), appunti(n.personaggio, n.nome, n.testo, true))));
  }

  // Appunti del Master su un personaggio (solo Master): per esempio gli agganci salvati dall'assistente IA.
  function appunti(pg, nome, testo, apriti) {
    return el("details", { class: "appunti-master", open: aperti.has("a" + pg) || apriti || undefined, on: ricorda("a" + pg) },
      el("summary", {}, el("strong", { testo: "📌 Appunti del Master" }), el("span", { class: "tenue piccolo", testo: " · solo tu" + (testo ? "" : " · vuoti") })),
      testo ? mostraTesto(testo) : el("p", { class: "tenue", testo: "Nessun appunto. Puoi scriverli qui o salvarli dall'Assistente IA." }),
      el("button", { class: "piccolo", testo: "✏ Modifica appunti", on: { click: () => modificaAppunti(pg, nome, testo) } }));
  }

  function modificaAppunti(pg, nome, testo) {
    const t = el("textarea", { class: "testo-lungo", rows: "14", "aria-label": "Appunti del Master" });
    t.value = testo || "";
    const err = el("p", { class: "errore" });
    dialogo("📌 Appunti su " + nome, el("div", {}, el("p", { class: "tenue", testo: "Li vedi solo tu; l'Assistente IA li legge. Svuota il testo per cancellarli." }), t, err), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva", on: { click: async () => {
          try { await api("/personaggi/" + encodeURIComponent(pg) + "/note-master", { metodo: "PUT", dati: { testo: t.value } }); chiudi(); await ricarica(); }
          catch (e) { err.textContent = e.message; }
        } } })) });
  }

  // Codice della campagna: schede che si chiamano come il personaggio o che lo collegano (solo Master)
  function codicePG(nome) {
    const box = el("p", { class: "riga-flex background-codice" });
    caricaCodice().then((c) => {
      const n = norma(nome);
      const l = (c.schede || []).filter((x) => nomiDi(x).some((y) => norma(y) === n) || (x.collegamenti || []).some((y) => norma(y) === n));
      if (l.length) svuota(box, el("span", { class: "tenue piccolo", testo: "🗝 Nel Codice:" }), l.map((x) => linkCodice(x)));
    }).catch(() => {});
    return box;
  }

  function scheda(b) {
    const cond = b.condiviso || [];
    const statoCond = !b.modificabile ? "" : cond.includes("tutti") ? "condiviso con tutto il gruppo" : cond.length ? "condiviso con " + cond.length + (cond.length === 1 ? " giocatore" : " giocatori") : "visibile solo al giocatore e al Master";
    const testo = mostraTesto(b.testo);
    const lungo = b.testo.length > 1800;
    if (lungo) testo.classList.add("chiuso");
    return el("article", { class: "scheda-ui background-card", "data-pg": b.personaggio },
      el("h2", { testo: b.titolo || "La storia di " + b.nome }),
      el("p", { class: "tenue riassunto-meta", testo: [b.nome, b.giocatore && b.giocatore !== io.utente.nome ? "giocatore: " + b.giocatore : "", statoCond, b.origine ? "dal file " + b.origine : ""].filter(Boolean).join(" · ") }),
      testo,
      lungo ? el("button", { class: "piccolo", testo: "Leggi tutto ▾", on: { click: (e) => { const c = testo.classList.toggle("chiuso"); e.target.textContent = c ? "Leggi tutto ▾" : "Riduci ▴"; } } }) : null,
      master ? codicePG(b.nome) : null, // schede del Codice della campagna su questo personaggio (fase L2)
      b.agganciIA ? schedaMaster(b) : null,
      master ? appunti(b.personaggio, b.nome, b.noteMaster || "", false) : null,
      b.modificabile ? el("div", { class: "riga-flex", stile: { marginTop: "10px" } },
        el("button", { class: "piccolo", testo: "👥 Condividi…", on: { click: () => condividi(b) } }),
        el("button", { class: "piccolo", testo: "✏ Modifica", on: { click: () => apriEditor(b) } }),
        el("button", { class: "piccolo pericolo", testo: "🗑 Elimina", on: { click: async () => {
          if (!(await conferma("Eliminare il background?", "La storia di " + b.nome + " sparisce per tutti.", "Elimina", true))) return;
          try { await api("/personaggi/" + encodeURIComponent(b.personaggio) + "/background-testo", { metodo: "DELETE" }); await ricarica(); } catch (e) { avviso(e.message, "errore"); }
        } } })) : null);
  }

  // Scheda per il Master (2026-11-09): sintesi, agganci per l'antagonista e opportunità dalle ultime sessioni, scritti
  // dall'assistente IA leggendo background, scheda e riassunti; si aggiornano da soli (internal/server/agganci_ia.go).
  function schedaMaster(b) {
    const a = b.agganciIA;
    const quando = a.generato ? new Date(a.generato * 1000).toLocaleString("it-IT", { dateStyle: "short", timeStyle: "short" }) : "";
    const stato = a.inCorso ? "⏳ l'IA sta scrivendo" + (a.testo ? " la versione aggiornata" : "") + "…"
      : a.errore ? "⚠ " + a.errore
      : !a.iaScelta ? "" : a.testo && !a.aggiornati ? "da aggiornare" : "";
    const corpo = a.testo ? mostraTesto(a.testo)
      : !a.iaScelta ? el("p", { class: "tenue", testo: "Gli agganci per l'antagonista li scrive l'Assistente IA, leggendo la storia, la scheda e i riassunti delle sessioni. Scegli l'IA in «Assistente IA» → «⚙ Impostazioni IA»." })
      : el("p", { class: "tenue", testo: a.inCorso ? "L'IA sta leggendo la storia, la scheda e i riassunti: tra poco qui trovi sintesi, agganci e opportunità." : "Ancora nessun aggancio: premi «🔄 Rigenera»." });
    const rigenera = el("button", { class: "piccolo", testo: "🔄 Rigenera", disabled: !a.iaScelta || a.inCorso || undefined,
      title: "Chiede all'IA nuovi agganci (anche se i dati non sono cambiati)", on: { click: async (e) => {
        e.target.disabled = true;
        try { await api("/personaggi/" + encodeURIComponent(b.personaggio) + "/agganci-ia", { metodo: "POST", dati: {} }); avviso("L'IA sta scrivendo nuovi agganci per " + b.nome + "."); await ricarica(); }
        catch (er) { avviso(er.message, "errore"); e.target.disabled = false; }
      } } });
    const negliAppunti = a.testo ? el("button", { class: "piccolo", testo: "📌 Copia negli appunti", title: "Aggiunge questi agganci agli appunti del Master (restano anche quando l'IA li riscrive)", on: { click: async () => {
      try {
        const n = await api("/personaggi/" + encodeURIComponent(b.personaggio) + "/note-master");
        const t = (n.testo ? n.testo.trim() + "\n\n" : "") + "## Agganci dell'IA, " + new Date().toLocaleDateString("it-IT") + "\n\n" + a.testo.trim();
        await api("/personaggi/" + encodeURIComponent(b.personaggio) + "/note-master", { metodo: "PUT", dati: { testo: t } });
        avviso("Copiati negli appunti di " + b.nome + "."); await ricarica();
      } catch (er) { avviso(er.message, "errore"); }
    } } }) : null;
    return el("details", { class: "analisi-master", open: aperti.has("m" + b.personaggio) || (a.inCorso && !a.testo) || undefined, on: ricorda("m" + b.personaggio) },
      el("summary", {}, el("strong", { testo: "🧛 Scheda per il Master" }), el("span", { class: "tenue piccolo", testo: " · solo tu" + (stato ? " · " + stato : "") })),
      corpo,
      el("div", { class: "riga-flex", stile: { marginTop: "8px", alignItems: "center" } }, rigenera, negliAppunti,
        a.testo ? el("span", { class: "tenue piccolo", testo: "Scritti da " + a.fornitore + (quando ? ", " + quando : "") + ". Si aggiornano da soli quando cambiano la storia, la scheda o i riassunti." }) : null));
  }

  async function condividi(b) {
    let info;
    try { info = await api("/documenti/destinatari"); } catch (e) { avviso(e.message, "errore"); return; }
    const altri = (info.giocatori || []).filter((g) => g.nome !== b.giocatore);
    const cond = b.condiviso || [];
    const tutti = el("input", { type: "checkbox", selezionato: cond.includes("tutti") });
    const scelte = altri.map((g) => ({ g, c: el("input", { type: "checkbox", value: g.id, selezionato: cond.includes(g.id) }) }));
    const err = el("p", { class: "errore" });
    dialogo("Con chi condividere la storia di " + b.nome + "?", el("div", {},
      el("p", { class: "tenue", testo: "Il Master la vede sempre. Chi la riceve può solo leggerla." }),
      el("label", { class: "riga" }, tutti, el("span", { testo: "Tutto il gruppo del personaggio" })),
      scelte.length ? el("div", {}, el("p", { class: "tenue piccolo", testo: "Oppure solo alcuni giocatori:" }), scelte.map(({ g, c }) => el("label", { class: "riga" }, c, el("span", { testo: g.nome })))) : null,
      err), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva", on: { click: async () => {
          const c = tutti.checked ? ["tutti"] : scelte.filter((x) => x.c.checked).map((x) => x.g.id);
          try {
            await api("/personaggi/" + encodeURIComponent(b.personaggio) + "/background-testo", { metodo: "PATCH", dati: { condiviso: c } });
            chiudi(); avviso(c.length ? "Background condiviso." : "Ora lo vedete solo tu e il Master."); await ricarica();
          } catch (e) { err.textContent = e.message; }
        } } })) });
  }

  async function apriEditor(b) {
    const pgs = dati.personaggi;
    if (!b && !pgs.length) { avviso(master ? "Nessun personaggio attivo." : "Prima crea il tuo personaggio nella sezione Schede.", "errore"); return; }
    const pgSel = el("select", { "aria-label": "Personaggio", disabled: !!b },
      (b ? [{ id: b.personaggio, nome: b.nome, giocatore: b.giocatore }] : pgs).map((p) => el("option", { value: p.id, testo: p.nome + (master && p.giocatore ? " (" + p.giocatore + ")" : master ? " (PNG)" : "") })));
    const titolo = el("input", { maxlength: "120", placeholder: "Titolo (facoltativo), es. La storia di Arin", valore: b ? b.titolo : "" });
    const testo = el("textarea", { class: "testo-lungo", placeholder: "Scrivi qui la storia, oppure caricala da un file. Titoli: # Titolo · elenchi: - voce · paragrafi separati da una riga vuota." });
    testo.value = b ? b.testo : "";
    let origine = b ? b.origine : "";
    const anteprima = el("div", { class: "anteprima-testo" });
    const conta = el("span", { class: "tenue" });
    const aggiorna = () => { conta.textContent = testo.value.length.toLocaleString("it-IT") + " / " + MAX.toLocaleString("it-IT") + " caratteri"; conta.classList.toggle("errore", testo.value.length > MAX); svuota(anteprima, mostraTesto(testo.value)); };
    testo.addEventListener("input", aggiorna); aggiorna();
    const file = el("input", { type: "file", accept: ACCETTA_TESTI, hidden: true });
    file.addEventListener("change", async () => {
      const f = file.files[0]; file.value = ""; if (!f) return;
      try {
        conta.textContent = "Lettura di " + f.name + "…";
        const t = await estraiTesto(f);
        if (!t.trim()) throw new Error("Nel file non c'è testo da leggere (un PDF fatto di sole immagini non si può leggere).");
        if (testo.value.trim() && !(await conferma("Sostituire il testo?", "Il testo scritto finora viene sostituito da quello del file.", "Sostituisci"))) { aggiorna(); return; }
        testo.value = t.slice(0, MAX); origine = f.name; if (!titolo.value) titolo.value = f.name.replace(/\.[^.]+$/, "").slice(0, 120); aggiorna();
      } catch (e) { avviso(e.message, "errore"); aggiorna(); }
    });
    const chiudi = () => svuota(editor);
    svuota(editor, el("section", { class: "scheda-ui" },
      el("h2", { testo: b ? "✏ Modifica il background" : "＋ Carica background" }),
      el("label", {}, master ? "Personaggio (lo carichi al posto del giocatore: resterà suo)" : "Personaggio", pgSel),
      el("label", {}, "Titolo", titolo),
      el("div", { class: "editor-due" }, el("label", {}, "Testo", testo), el("div", {}, el("span", { class: "tenue piccolo", testo: "Anteprima" }), anteprima)),
      el("div", { class: "riga-flex" }, file,
        el("button", { testo: "📄 Carica da file (testo, Word, PDF)", on: { click: () => file.click() } }), conta, el("span", { class: "spazio" }),
        el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Pubblica", on: { click: async (e) => {
          if (testo.value.length > MAX) { avviso("Testo troppo lungo: massimo " + MAX + " caratteri.", "errore"); return; }
          e.target.disabled = true;
          try {
            await api("/personaggi/" + encodeURIComponent(pgSel.value) + "/background-testo", { metodo: "PUT", dati: { titolo: titolo.value, testo: testo.value, origine } });
            avviso("Background pubblicato: lo vedete tu e il Master" + (master ? " (e il giocatore)." : ".")); chiudi(); await ricarica();
          } catch (er) { avviso(er.message, "errore"); e.target.disabled = false; }
        } } })),
      el("p", { class: "tenue piccolo", testo: "Da un PDF fatto di sole immagini (una scansione) il testo non si può leggere: in quel caso scrivilo o incollalo qui." })));
    editor.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  svuota(azioni, el("button", { class: "primario", testo: "＋ Carica background", on: { click: () => apriEditor(null) } }));
  const via = tempoReale.su("background.aggiornato", () => ricarica().catch(() => {}));
  try { await ricarica(); } catch (e) { svuota(lista, el("p", { class: "errore", testo: e.message })); }
  // apertura diretta: /background?pg=<id> (dal Codice della campagna)
  const diretta = new URLSearchParams(location.search).get("pg");
  const art = diretta && lista.querySelector('[data-pg="' + CSS.escape(diretta) + '"]');
  if (art) { art.scrollIntoView({ behavior: "smooth", block: "start" }); art.classList.add("lore-evidenza"); setTimeout(() => art.classList.remove("lore-evidenza"), 1800); }
  return () => via();
}
