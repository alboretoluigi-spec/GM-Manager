// Sezione «Documenti» (2026-11-07): PDF, documenti Word e immagini.
// Giocatore: i suoi documenti e la sottocartella «Backup» (copie dei documenti consegnati); può caricare, aprire,
// rinominare, sostituire, eliminare e consegnare un documento a un altro giocatore.
// Master: la sua cartella e le cartelle di tutti i giocatori; può modificare, eliminare e condividere qualsiasi file.
import { el, svuota, api, avviso, dialogo, conferma, quando, carica, tempoReale } from "../ui/ui.js";

const ICONE = { pdf: "📕", docx: "📘", doc: "📘", png: "🖼", jpg: "🖼", gif: "🖼", webp: "🖼" };
const ACCETTA = ".pdf,.doc,.docx,.png,.jpg,.jpeg,.gif,.webp,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,image/*";

function dimensione(n) { return n < 1024 * 1024 ? Math.max(1, Math.round(n / 1024)) + " KB" : (n / 1024 / 1024).toFixed(1).replace(".", ",") + " MB"; }
const url = (d, scarica) => "/api/v1/documenti/" + encodeURIComponent(d.id) + "/file" + (scarica ? "?scarica=1" : "");

/** Apre un documento: PDF e immagini in una nuova scheda del browser, Word scaricato. */
export function apriDocumento(d) {
  if (d.nuovo) api("/documenti/" + encodeURIComponent(d.id) + "/visto", { dati: {} }).catch(() => {});
  window.open(url(d, d.tipo === "doc" || d.tipo === "docx"), "_blank", "noopener");
}

export async function mostra(corpo, { io, azioni }) {
  const master = io.utente.ruolo === "master";
  const contenitore = el("div", { class: "largo" });
  svuota(corpo, contenitore);
  const scelta = el("input", { type: "file", accept: ACCETTA, multiple: true, stile: { display: "none" } });
  let cartellaCarica = ""; // Master: cartella del giocatore in cui caricare ("" = la sua)
  const avanz = el("span", { class: "tenue piccolo" });
  svuota(azioni, avanz, scelta, el("button", { class: "primario piccolo", testo: "＋ Carica documento", on: { click: () => { cartellaCarica = ""; scelta.click(); } } }));
  scelta.addEventListener("change", async () => {
    const file = [...scelta.files]; scelta.value = "";
    for (const f of file) {
      try {
        await carica("/documenti" + (cartellaCarica ? "?per=" + encodeURIComponent(cartellaCarica) : ""), f, (p) => { avanz.textContent = f.name + " " + Math.round(p * 100) + "%"; }, "PUT", f.name);
        avviso("📁 Caricato: " + f.name, "", 3000);
      } catch (e) { avviso(f.name + ": " + e.message, "errore", 7000); }
    }
    avanz.textContent = "";
    disegna();
  });

  let dati = { lista: [], giocatori: [] };
  const aperte = new Map(); // cartelle aperte o chiuse a mano: restano così quando la pagina si aggiorna
  const cartella = (chiave, predefinita, ...figli) => {
    const d = el("details", { class: "scheda-ui", open: aperte.has(chiave) ? aperte.get(chiave) : predefinita }, ...figli);
    d.addEventListener("toggle", () => aperte.set(chiave, d.open));
    return d;
  };
  async function disegna() {
    try { dati = await api("/documenti"); } catch (e) { svuota(contenitore, el("p", { class: "errore", testo: e.message })); return; }
    if (!master) {
      const miei = dati.lista.filter((d) => d.cartella === "");
      const bk = dati.lista.filter((d) => d.cartella === "backup");
      svuota(contenitore,
        el("section", { class: "scheda-ui" }, el("h2", { testo: "📁 I miei documenti" }),
          el("p", { class: "tenue piccolo", testo: "PDF, documenti Word e immagini. Li vedete solo tu e il Master. Con «Consegna» passi un documento a un altro giocatore: ne tieni una copia nel Backup." }),
          elenco(miei, "Nessun documento: caricane uno con «＋ Carica documento».")),
        cartella("backup", bk.length > 0 && bk.length < 6,
          el("summary", {}, el("strong", { testo: "🗄 Backup" }), el("span", { class: "tenue piccolo", testo: " · copie dei documenti consegnati (" + bk.length + ")" })),
          elenco(bk, "Nessun documento consegnato.")));
      return;
    }
    // Master: la sua cartella, poi una cartella per giocatore (anche vuota).
    const per = new Map();
    for (const d of dati.lista) { if (!per.has(d.proprietario)) per.set(d.proprietario, []); per.get(d.proprietario).push(d); }
    const mio = per.get(io.utente.id) || [];
    svuota(contenitore,
      el("section", { class: "scheda-ui" }, el("h2", { testo: "📁 I tuoi documenti" }),
        el("p", { class: "tenue piccolo", testo: "Con «Condividi» consegni una copia a uno o più giocatori: la ricevono con una notifica." }),
        elenco(mio, "Nessun documento.")),
      (dati.giocatori || []).map((g) => {
        const l = per.get(g.id) || [];
        const miei = l.filter((d) => d.cartella === ""), bk = l.filter((d) => d.cartella === "backup");
        const c = cartella(g.id, false,
          el("summary", {}, el("strong", { testo: "📂 " + g.nome }), el("span", { class: "tenue piccolo", testo: " · " + (miei.length === 1 ? "1 documento" : miei.length + " documenti") + (bk.length ? ", " + bk.length + " nel Backup" : "") })),
          el("div", { class: "riga-flex", stile: { margin: "6px 0" } }, el("button", { class: "piccolo", testo: "＋ Carica in questa cartella", on: { click: () => { cartellaCarica = g.id; scelta.click(); } } })),
          elenco(miei, "Nessun documento."),
          bk.length ? el("div", {}, el("h3", { testo: "🗄 Backup" }), elenco(bk, "")) : null);
        c.classList.add("cartella-doc"); c.dataset.giocatore = g.nome;
        return c;
      }));
  }

  function elenco(l, vuoto) {
    if (!l.length) return vuoto ? el("p", { class: "tenue", testo: vuoto }) : null;
    return el("ul", { class: "elenco-doc" }, l.map((d) => el("li", { class: "doc" + (d.nuovo ? " nuovo" : ""), "data-doc": d.nome },
      el("span", { class: "doc-ico", "aria-hidden": "true", testo: ICONE[d.tipo] || "📄" }),
      el("div", { class: "doc-info" },
        el("a", { href: url(d), target: "_blank", rel: "noopener", testo: d.nome, on: { click: (e) => { e.preventDefault(); apriDocumento(d); d.nuovo = false; } } }),
        el("div", { class: "tenue piccolo", testo: [d.tipo.toUpperCase(), dimensione(d.dimensione), quando(d.modificato),
          d.daPersonaggio ? "consegnato da " + d.daPersonaggio : "", d.aUtente ? "consegnato a " + d.aUtente : ""].filter(Boolean).join(" · ") }),
        d.nuovo ? el("span", { class: "etichetta approvato", testo: "nuovo" }) : null),
      el("div", { class: "doc-azioni riga-flex" },
        el("a", { class: "pulsante piccolo", href: url(d, true), testo: "⬇", title: "Scarica", download: "" }),
        el("button", { class: "piccolo", testo: "👁 Mostra…", title: "Fallo comparire sugli schermi di altri giocatori (lo guardano, non lo ricevono)", on: { click: () => mostraAltri(d) } }),
        d.cartella === "" || master ? el("button", { class: "piccolo", testo: master ? "Condividi…" : "Consegna…", on: { click: () => consegna(d) } }) : null,
        el("button", { class: "piccolo", testo: "Rinomina", on: { click: () => rinomina(d) } }),
        el("button", { class: "piccolo", testo: "Sostituisci", title: "Carica una nuova versione del file", on: { click: () => sostituisci(d) } }),
        el("button", { class: "piccolo pericolo", testo: "Elimina", on: { click: () => elimina(d) } })))));
  }

  function rinomina(d) {
    const nome = el("input", { value: d.nome, maxlength: "100" });
    const err = el("p", { class: "errore" });
    dialogo("Rinomina il documento", el("div", {}, el("label", {}, "Nome", nome), err), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva", on: { click: async () => {
          try { await api("/documenti/" + encodeURIComponent(d.id), { metodo: "PATCH", dati: { nome: nome.value.trim() } }); chiudi(); disegna(); }
          catch (e) { err.textContent = e.message; }
        } } })) });
  }

  function sostituisci(d) {
    const f = el("input", { type: "file", accept: ACCETTA });
    f.addEventListener("change", async () => {
      if (!f.files[0]) return;
      try { await carica("/documenti/" + encodeURIComponent(d.id) + "/file", f.files[0], null, "PUT", f.files[0].name); avviso("📁 Documento sostituito.", "", 3000); disegna(); }
      catch (e) { avviso(e.message, "errore", 7000); }
    });
    f.click();
  }

  async function elimina(d) {
    if (!(await conferma("Eliminare il documento?", "«" + d.nome + "» sarà eliminato definitivamente.", "Elimina", true))) return;
    try { await api("/documenti/" + encodeURIComponent(d.id), { metodo: "DELETE" }); disegna(); } catch (e) { avviso(e.message, "errore"); }
  }

  // «Mostra» (2026-11-09): il documento compare a tutto schermo sugli schermi scelti; nessuno ne riceve una copia.
  async function mostraAltri(d) {
    let info;
    try { info = await api("/documenti/destinatari"); } catch (e) { avviso(e.message, "errore"); return; }
    const altri = info.giocatori.filter((g) => g.id !== d.proprietario);
    if (!altri.length) { avviso("Nessun altro giocatore a cui mostrarlo.", "errore"); return; }
    const tutti = el("input", { type: "checkbox" });
    const scelte = altri.map((g) => ({ g, c: el("input", { type: "checkbox", value: g.id }) }));
    const pg = !master && info.personaggi.length > 1 ? el("select", { "aria-label": "Lo mostra il personaggio" }, info.personaggi.map((p) => el("option", { value: p.id, testo: p.nome }))) : null;
    const err = el("p", { class: "errore" });
    dialogo("👁 Mostra «" + d.nome + "»", el("div", {},
      el("p", { class: "tenue", testo: "Compare a tutto schermo sui loro dispositivi: lo guardano, ma non ne ricevono una copia." }),
      el("label", { class: "riga" }, tutti, el("span", { testo: master ? "Tutti i giocatori" : "Tutti i compagni dei miei gruppi" })),
      el("div", {}, scelte.map(({ g, c }) => el("label", { class: "riga" }, c, el("span", { testo: g.nome })))),
      pg ? el("label", {}, "Lo mostra il personaggio", pg) : null, err), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Mostra", on: { click: async () => {
          const a = tutti.checked ? ["tutti"] : scelte.filter((x) => x.c.checked).map((x) => x.g.id);
          if (!a.length) { err.textContent = "Scegli a chi mostrarlo."; return; }
          try {
            const r = await api("/documenti/" + encodeURIComponent(d.id) + "/mostra", { dati: { a, personaggio: pg ? pg.value : (!master && info.personaggi[0] ? info.personaggi[0].id : "") } });
            chiudi(); avviso("👁 Mostrato a " + (r.mostrato === 1 ? "1 giocatore." : r.mostrato + " giocatori."), "", 4000);
          } catch (e) { err.textContent = e.message; }
        } } })) });
  }

  async function consegna(d) {
    let info;
    try { info = await api("/documenti/destinatari"); } catch (e) { avviso(e.message, "errore"); return; }
    const altri = info.giocatori.filter((g) => g.id !== d.proprietario);
    if (!altri.length) { avviso(master ? "Nessun giocatore approvato." : "Nessun altro giocatore nei tuoi gruppi.", "errore"); return; }
    const err = el("p", { class: "errore" });
    let scelta, pg = null;
    if (master) {
      scelta = altri.map((g) => ({ g, c: el("input", { type: "checkbox", value: g.id }) }));
    } else {
      scelta = el("select", { "aria-label": "A chi" }, altri.map((g) => el("option", { value: g.id, testo: g.nome })));
      if (info.personaggi.length > 1) pg = el("select", { "aria-label": "Chi lo consegna" }, info.personaggi.map((p) => el("option", { value: p.id, testo: p.nome })));
    }
    dialogo(master ? "Condividi «" + d.nome + "»" : "Consegna «" + d.nome + "»", el("div", {},
      master ? el("div", {}, el("p", { testo: "Ogni giocatore scelto riceve una copia con la notifica «Il Master ti ha consegnato un documento»." }),
        scelta.map(({ g, c }) => el("label", { class: "riga" }, c, el("span", { testo: g.nome }))))
        : el("div", {}, el("label", {}, "A chi", scelta), pg ? el("label", {}, "Lo consegna il personaggio", pg) : null,
          el("p", { class: "tenue piccolo", testo: "Il giocatore riceve il documento con la notifica «" + (pg ? "[personaggio]" : (info.personaggi[0] ? info.personaggi[0].nome : io.utente.nome)) +
            " ti ha consegnato un documento». Tu ne conservi una copia nel Backup." })),
      err), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: master ? "Condividi" : "Consegna", on: { click: async () => {
          const a = master ? scelta.filter((x) => x.c.checked).map((x) => x.g.id) : [scelta.value];
          if (!a.length) { err.textContent = "Scegli almeno un giocatore."; return; }
          try {
            const r = await api("/documenti/" + encodeURIComponent(d.id) + "/consegna", { dati: { a, personaggio: pg ? pg.value : (info.personaggi[0] ? info.personaggi[0].id : "") } });
            chiudi(); avviso("📁 Documento consegnato" + (r.consegnati > 1 ? " a " + r.consegnati + " giocatori." : "."), "", 4000); disegna();
          } catch (e) { err.textContent = e.message; }
        } } })) });
  }

  const via = tempoReale.su("documenti.aggiornati", () => disegna());
  await disegna();
  return () => via();
}
