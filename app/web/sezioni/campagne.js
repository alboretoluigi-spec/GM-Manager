// Campagne (passo L4, docs/RULEBOOK_LIBRARY.md, 13/11/2026): per ogni campagna sistema di gioco ed edizione (testo
// libero), regolamento principale, documenti supplementari e schede personalizzate presi da My Rulebooks (lo stesso
// documento può stare in più campagne), regole aggiuntive del Master, impostazioni libere e gruppi che la giocano.
import { el, svuota, api, avviso, dialogo, conferma, tempoReale } from "../ui/ui.js";

export async function mostra(corpo) {
  const stato = { campagne: [], senza: [], docs: [], giochi: [], moduli: [] };
  const elenco = el("div", { class: "cp-elenco", "aria-live": "polite" });
  const datalist = el("datalist", { id: "cp-sistemi" });
  svuota(corpo, el("div", { class: "largo rl" },
    el("div", { class: "rl-banda" },
      el("div", {}, el("h2", { class: "rl-titolo", testo: "Campagne" }),
        el("p", { class: "rl-sotto", testo: "Sistema di gioco, regolamenti, regole del Master e gruppi di ogni campagna" }))),
    el("div", { class: "mr-comandi" },
      el("button", { class: "primario", type: "button", testo: "＋ Nuova campagna", on: { click: () => finestraCampagna(null) } }),
      el("a", { class: "pulsante", href: "/miei-regolamenti", testo: "📗 My Rulebooks" })),
    elenco, datalist));

  async function ricarica() {
    try {
      const [c, d] = await Promise.all([api("/campagne"), api("/regolamenti/miei")]);
      stato.campagne = c.campagne; stato.senza = c.gruppiSenzaCampagna; stato.docs = d.documenti;
      if (!stato.moduli.length) stato.moduli = (await api("/gioco")).moduli || [];
    } catch (e) { avviso(e.message, "errore"); return; }
    if (!stato.giochi.length) {
      try { stato.giochi = (await api("/regolamenti/catalogo")).giochi.map((g) => g.nome); } catch { /* facoltativo */ }
      svuota(datalist, ["Scheda libera", ...stato.giochi].map((g) => el("option", { value: g })));
    }
    disegna();
  }

  function disegna() {
    svuota(elenco, stato.campagne.map(schedaCampagna),
      stato.senza.length ? el("div", { class: "scheda-ui" }, el("h3", { testo: "Gruppi senza campagna" }),
        stato.senza.map((g) => rigaGruppo(g, ""))) : null);
  }

  const titoloDoc = (id) => (stato.docs.find((d) => d.id === id) || {}).titolo || "(documento eliminato)";

  function rigaGruppo(g, attuale) {
    const s = el("select", { "aria-label": "Campagna del gruppo " + g.nome, on: { change: async () => {
      try { await api("/gruppi/" + encodeURIComponent(g.id) + "/campagna", { metodo: "PUT", dati: { campagna: s.value } }); avviso("«" + g.nome + "» spostato."); ricarica(); }
      catch (e) { avviso(e.message, "errore"); }
    } } }, (attuale ? [] : [el("option", { value: "", testo: "—" })]).concat(stato.campagne.map((c) => el("option", { value: c.id, testo: c.nome }))));
    s.value = attuale;
    return el("div", { class: "cp-gruppo" }, el("span", { testo: "👥 " + g.nome }), s);
  }

  function elencoDocumenti(c, ruolo) {
    const scelti = new Set(ruolo === "scheda" ? c.schede : c.supplementari);
    const docs = stato.docs.filter((d) => d.id !== c.regolamento);
    if (!docs.length) return el("p", { class: "tenue piccolo", testo: "Nessun documento in My Rulebooks." });
    return el("div", { class: "cp-documenti" }, docs.map((d) => {
      const x = el("input", { type: "checkbox", selezionato: scelti.has(d.id), on: { change: async () => {
        try { await api("/campagne/" + encodeURIComponent(c.id) + "/documenti", { metodo: "PUT", dati: { documento: d.id, ruolo, presente: x.checked } }); }
        catch (e) { x.checked = !x.checked; avviso(e.message, "errore"); }
      } } });
      return el("label", { class: "riga" }, x, el("span", { testo: d.titolo + (d.gioco ? " · " + d.gioco : "") }));
    }));
  }

  function editorImpostazioni(c) {
    let voci = Object.entries(c.impostazioni || {});
    const box = el("div", { class: "cp-imp" });
    const riga = (k = "", v = "") => {
      const ck = el("input", { value: k, placeholder: "voce (es. dado d'iniziativa)", "aria-label": "Nome dell'impostazione", maxlength: "60" });
      const cv = el("input", { value: typeof v === "string" ? v : JSON.stringify(v), placeholder: "valore", "aria-label": "Valore dell'impostazione", maxlength: "200" });
      const r = el("div", { class: "riga-flex cp-imp-riga" }, ck, cv, el("button", { class: "piccolo", type: "button", testo: "✕", title: "Togli", on: { click: () => r.remove() } }));
      return r;
    };
    const righe = el("div", {}, voci.map(([k, v]) => riga(k, v)));
    svuota(box, righe, el("div", { class: "riga-flex" },
      el("button", { class: "piccolo", type: "button", testo: "＋ Voce", on: { click: () => righe.append(riga()) } }),
      el("button", { class: "piccolo primario", type: "button", testo: "Salva le impostazioni", on: { click: async () => {
        const o = {};
        righe.querySelectorAll(".cp-imp-riga").forEach((r) => { const [k, v] = r.querySelectorAll("input"); if (k.value.trim()) o[k.value.trim()] = v.value; });
        try { await api("/campagne/" + encodeURIComponent(c.id), { metodo: "PATCH", dati: { impostazioni: o } }); avviso("Impostazioni salvate."); }
        catch (e) { avviso(e.message, "errore"); }
      } } })));
    return box;
  }

  function schedaCampagna(c) {
    const reg = el("select", { "aria-label": "Regolamento principale di " + c.nome, on: { change: async () => {
      try { await api("/campagne/" + encodeURIComponent(c.id), { metodo: "PATCH", dati: { regolamento: reg.value } }); ricarica(); } catch (e) { avviso(e.message, "errore"); }
    } } }, el("option", { value: "", testo: "— nessuno —" }), stato.docs.map((d) => el("option", { value: d.id, testo: d.titolo + (d.edizione ? " (" + d.edizione + ")" : "") })));
    reg.value = c.regolamento || "";
    const regole = el("textarea", { rows: "4", "aria-label": "Regole aggiuntive di " + c.nome, maxlength: "50000", placeholder: "Regole della casa, varianti, chiarimenti…" }, c.regoleMaster || "");
    return el("article", { class: "scheda-ui cp-campagna", "data-campagna": c.id },
      el("div", { class: "rl-doc-testa" }, el("h3", { testo: c.nome }),
        el("span", { class: "etichetta mr-cat cp-modulo", title: "Gioco della campagna: decide sezioni e pulsanti dell'app", testo: "🎮 " + nomeModulo(c.modulo) }),
        c.modulo !== "d20-5e" ? el("button", { class: "piccolo", type: "button", testo: "📜 Modello della scheda…", title: "Campi, tiri, punti ferita e iniziativa della scheda libera di questa campagna",
          on: { click: () => import("./modello_scheda.js").then((m) => m.apriModello(c)) } }) : null,
        c.sistema ? el("span", { class: "etichetta mr-cat", testo: "🎲 " + c.sistema + (c.edizione ? " · " + c.edizione : "") }) : el("span", { class: "etichetta", testo: "sistema non indicato" })),
      c.descrizione ? el("p", { class: "tenue piccolo", testo: c.descrizione }) : null,
      el("div", { class: "cp-griglia" },
        el("section", {}, el("h4", { testo: "Regolamento principale" }), reg),
        el("section", {}, el("h4", { testo: "Gruppi" }), c.gruppi.length ? c.gruppi.map((g) => rigaGruppo(g, c.id)) : el("p", { class: "tenue piccolo", testo: "Nessun gruppo." }))),
      el("details", {}, el("summary", { testo: "Documenti supplementari (" + c.supplementari.length + ")" }), elencoDocumenti(c, "supplementare")),
      el("details", {}, el("summary", { testo: "Schede personalizzate (" + c.schede.length + ")" }), elencoDocumenti(c, "scheda")),
      el("details", {}, el("summary", { testo: "Regole aggiuntive del Master" }), regole,
        el("button", { class: "piccolo primario", type: "button", testo: "Salva le regole", on: { click: async () => {
          try { await api("/campagne/" + encodeURIComponent(c.id), { metodo: "PATCH", dati: { regoleMaster: regole.value } }); avviso("Regole salvate."); } catch (e) { avviso(e.message, "errore"); }
        } } })),
      el("details", {}, el("summary", { testo: "Impostazioni della campagna (" + Object.keys(c.impostazioni || {}).length + ")" }), editorImpostazioni(c)),
      el("div", { class: "mr-azioni" },
        el("button", { class: "piccolo", type: "button", testo: "Modifica", on: { click: () => finestraCampagna(c) } }),
        el("button", { class: "piccolo pericolo", type: "button", testo: "Elimina", on: { click: async () => {
          if (!(await conferma("Eliminare la campagna?", "«" + c.nome + "» sparisce; i documenti restano in My Rulebooks. Si può eliminare solo se non ha gruppi.", "Elimina", true))) return;
          try { await api("/campagne/" + encodeURIComponent(c.id), { metodo: "DELETE" }); avviso("Campagna eliminata."); ricarica(); } catch (e) { avviso(e.message, "errore", 6000); }
        } } })));
  }

  function nomeModulo(id) { return (stato.moduli.find((m) => m.id === id) || {}).nome || id || "—"; }

  function finestraCampagna(c) {
    const campo = (k, etichetta, attr = {}) => el("label", { class: "riga-campo" }, el("span", { testo: etichetta }), el("input", { name: k, value: (c && c[k]) || "", ...attr }));
    const campi = el("div", { class: "mr-campi" },
      campo("nome", "Nome della campagna", { maxlength: "80" }),
      el("label", { class: "riga-campo" }, el("span", { testo: "Gioco (decide sezioni e pulsanti dell'app)" }),
        (() => { const s = el("select", { name: "modulo" }, stato.moduli.map((m) => el("option", { value: m.id,
          testo: m.nome + (m.pronto ? "" : " — per ora con la scheda libera") }))); s.value = (c && c.modulo) || "d20-5e"; return s; })()),
      campo("sistema", "Sistema di gioco", { maxlength: "100", list: "cp-sistemi", placeholder: "dal catalogo o scritto da te" }),
      campo("edizione", "Edizione", { maxlength: "60" }),
      el("label", { class: "riga-campo" }, el("span", { testo: "Descrizione" }), el("textarea", { name: "descrizione", rows: "2", maxlength: "2000" }, (c && c.descrizione) || "")));
    const errore = el("p", { class: "rl-errore piccolo", role: "alert" });
    dialogo(c ? "Modifica «" + c.nome + "»" : "Nuova campagna", el("div", {}, campi, errore), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { type: "button", testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", type: "button", testo: c ? "Salva" : "Crea", on: { click: async () => {
          const dati = {};
          campi.querySelectorAll("[name]").forEach((x) => { dati[x.name] = x.value.trim(); });
          try {
            if (c) await api("/campagne/" + encodeURIComponent(c.id), { metodo: "PATCH", dati });
            else await api("/campagne", { metodo: "POST", dati });
            chiudi(); ricarica();
          } catch (e) { errore.textContent = e.message; }
        } } })),
    });
  }

  const via = tempoReale.su("campagne", () => ricarica());
  await ricarica();
  return () => via();
}
