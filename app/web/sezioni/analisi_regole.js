// Analisi dei regolamenti (passo L5, docs/RULEBOOK_LIBRARY.md, 13/11/2026): le regole individuate nei documenti di My
// Rulebooks (analizzatore per parole chiave, IA facoltativa in futuro) o scritte dal Master. Ogni regola resta una
// PROPOSTA finché il Master non la verifica e la approva: solo le approvate potranno essere usate dal motore.
import { el, svuota, api, avviso, dialogo, conferma, tempoReale } from "../ui/ui.js";
import { schedeLibreria } from "../ui/libreria_schede.js";

const STATI = { proposta: "Da verificare", approvata: "Approvata", rifiutata: "Rifiutata" };

export async function mostra(corpo) {
  const filtri = { stato: "proposta", categoria: "", documento: "" };
  const stato = { proposte: [], categorie: [], analizzatori: [], conteggi: {}, docs: [], campagne: [] };
  const elenco = el("div", { class: "an-elenco", "aria-live": "polite" });
  const sel = (k, etichetta) => {
    const s = el("select", { "aria-label": etichetta, on: { change: () => { filtri[k] = s.value; ricarica(); } } });
    return { s, box: el("label", { class: "rl-filtro" }, el("span", { testo: etichetta }), s) };
  };
  const fStato = sel("stato", "Stato"), fCat = sel("categoria", "Categoria"), fDoc = sel("documento", "Documento");
  const conti = el("p", { class: "tenue piccolo" });
  svuota(corpo, el("div", { class: "largo rl" },
    schedeLibreria("analisi"),
    el("div", { class: "rl-banda" },
      el("div", {}, el("h2", { class: "rl-titolo", testo: "Regole da approvare" }),
        el("p", { class: "rl-sotto", testo: "Regole trovate nei tuoi documenti: nulla viene usato finché non lo approvi" }))),
    el("div", { class: "mr-comandi" },
      el("button", { class: "primario", type: "button", testo: "🔎 Analizza un documento", on: { click: finestraAnalisi } }),
      el("button", { type: "button", testo: "＋ Regola scritta da me", on: { click: () => finestraProposta(null) } })),
    el("div", { class: "scheda-ui rl-filtri" }, fStato.box, fCat.box, fDoc.box), conti, elenco));

  async function ricarica() {
    const q = new URLSearchParams(Object.entries(filtri).filter(([, v]) => v));
    try {
      const [r, d, c] = await Promise.all([api("/regolamenti/analisi?" + q.toString()), api("/regolamenti/miei"), api("/campagne")]);
      Object.assign(stato, { proposte: r.proposte, categorie: r.categorie, analizzatori: r.analizzatori, conteggi: r.conteggi || {}, docs: d.documenti, campagne: c.campagne });
    } catch (e) { avviso(e.message, "errore"); return; }
    const riempi = (s, valori, val) => { svuota(s, valori.map(([v, t]) => el("option", { value: v, testo: t }))); s.value = val; };
    riempi(fStato.s, [["", "Tutte"], ...Object.entries(STATI)], filtri.stato);
    riempi(fCat.s, [["", "Tutte"], ...stato.categorie.map((x) => [x.ID, x.Nome])], filtri.categoria);
    riempi(fDoc.s, [["", "Tutti"], ...stato.docs.map((x) => [x.id, x.titolo])], filtri.documento);
    const n = stato.conteggi;
    conti.textContent = "Da verificare: " + (n.proposta || 0) + " · approvate: " + (n.approvata || 0) + " · rifiutate: " + (n.rifiutata || 0);
    disegna();
  }

  const nomeCat = (id) => (stato.categorie.find((c) => c.ID === id) || {}).Nome || id;

  function disegna() {
    if (!stato.proposte.length) {
      svuota(elenco, el("p", { class: "tenue", testo: "Nessuna regola con questi filtri. Usa «Analizza un documento» o scrivine una." }));
      return;
    }
    const gruppi = new Map();
    for (const p of stato.proposte) { if (!gruppi.has(p.categoria)) gruppi.set(p.categoria, []); gruppi.get(p.categoria).push(p); }
    svuota(elenco, stato.categorie.filter((c) => gruppi.has(c.ID)).map((c) => el("section", { class: "mr-gruppo" },
      el("h3", { class: "mr-gioco", testo: c.Nome + " (" + gruppi.get(c.ID).length + ")" }), gruppi.get(c.ID).map(schedaProposta))));
  }

  function fonte(p) {
    const testo = (p.documentoTitolo || "senza documento") + (p.pagina ? " · p. " + p.pagina : "") + (p.dove ? " · " + p.dove : "");
    if (!p.documento) return el("span", { class: "tenue piccolo", testo: "Fonte: " + testo });
    const d = stato.docs.find((x) => x.id === p.documento);
    const url = "/api/v1/regolamenti/miei/" + encodeURIComponent(p.documento) + "/file" + (d && d.formato === "pdf" && p.pagina ? "#page=" + p.pagina : "");
    return el("span", { class: "piccolo" }, "Fonte: ", el("a", { href: url, target: "_blank", rel: "noopener", testo: testo + " ↗" }));
  }

  function schedaProposta(p) {
    const decidi = (st) => async () => {
      try { await api("/regolamenti/analisi/" + encodeURIComponent(p.id), { metodo: "PATCH", dati: { stato: st } }); ricarica(); } catch (e) { avviso(e.message, "errore"); }
    };
    return el("article", { class: "scheda-ui mr-doc an-proposta an-" + p.stato, "data-proposta": p.id },
      el("div", { class: "rl-doc-testa" }, el("strong", { testo: p.nome }),
        el("span", { class: "etichetta an-stato", testo: STATI[p.stato] || p.stato }),
        el("span", { class: "etichetta", testo: p.origine === "manuale" ? "scritta dal Master" : p.origine === "ia" ? "proposta dall'IA" : "trovata per parole chiave" })),
      fonte(p),
      p.testo ? el("p", { class: "an-testo", testo: p.testo }) : null,
      p.estratto ? el("details", { class: "piccolo" }, el("summary", { testo: "Testo trovato nel documento" }), el("p", { class: "tenue", testo: p.estratto })) : null,
      p.nota ? el("p", { class: "piccolo tenue", testo: "Nota: " + p.nota }) : null,
      el("div", { class: "mr-azioni" },
        p.stato !== "approvata" ? el("button", { class: "piccolo primario", type: "button", testo: "✓ Approva", on: { click: decidi("approvata") } }) : null,
        p.stato !== "rifiutata" ? el("button", { class: "piccolo", type: "button", testo: "✕ Rifiuta", on: { click: decidi("rifiutata") } }) : null,
        p.stato !== "proposta" ? el("button", { class: "piccolo", type: "button", testo: "↺ Rimetti da verificare", on: { click: decidi("proposta") } }) : null,
        el("button", { class: "piccolo", type: "button", testo: "Modifica", on: { click: () => finestraProposta(p) } }),
        el("button", { class: "piccolo pericolo", type: "button", testo: "Elimina", on: { click: async () => {
          if (!(await conferma("Eliminare la regola?", "«" + p.nome + "» sparisce dall'elenco.", "Elimina", true))) return;
          try { await api("/regolamenti/analisi/" + encodeURIComponent(p.id), { metodo: "DELETE" }); ricarica(); } catch (e) { avviso(e.message, "errore"); }
        } } })));
  }

  function finestraAnalisi() {
    const doc = el("select", { "aria-label": "Documento da analizzare" }, stato.docs.map((d) => el("option", { value: d.id, testo: d.titolo + (d.testo ? "" : " (senza testo)") })));
    const an = el("select", { "aria-label": "Analizzatore" }, stato.analizzatori.map((a) => el("option", { value: a.id, disabled: !a.disponibile, testo: a.nome + (a.disponibile ? "" : " — non disponibile") })));
    const camp = el("select", { "aria-label": "Campagna" }, el("option", { value: "", testo: "— nessuna in particolare —" }), stato.campagne.map((c) => el("option", { value: c.id, testo: c.nome })));
    const errore = el("p", { class: "rl-errore piccolo", role: "alert" });
    const nota = stato.analizzatori.filter((a) => !a.disponibile).map((a) => el("p", { class: "tenue piccolo", testo: a.nome + ": " + a.motivo }));
    dialogo("Analizza un documento", el("div", { class: "mr-campi" },
      el("label", { class: "riga-campo" }, el("span", { testo: "Documento" }), doc),
      el("label", { class: "riga-campo" }, el("span", { testo: "Analizzatore" }), an),
      el("label", { class: "riga-campo" }, el("span", { testo: "Campagna (facoltativa)" }), camp), nota, errore), {
      azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { type: "button", testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", type: "button", testo: "Analizza", on: { click: async () => {
          try {
            const r = await api("/regolamenti/analisi/avvia", { metodo: "POST", dati: { documento: doc.value, analizzatore: an.value, campagna: camp.value } });
            chiudi(); avviso(r.nuove + " regole trovate da verificare" + (r.giaPresenti ? " (" + r.giaPresenti + " già presenti)" : "") + ".", "", 5000);
            Object.assign(filtri, { stato: "proposta", documento: doc.value, categoria: "" }); ricarica();
          } catch (e) { errore.textContent = e.message; }
        } } })),
    });
  }

  function finestraProposta(p) {
    const v = (k, pred = "") => (p && p[k] !== undefined && p[k] !== null ? p[k] : pred);
    const nome = el("input", { name: "nome", value: v("nome"), maxlength: "200" });
    const cat = el("select", { name: "categoria" }, stato.categorie.map((c) => el("option", { value: c.ID, testo: c.Nome })));
    cat.value = v("categoria", "speciali");
    const doc = el("select", { name: "documento" }, el("option", { value: "", testo: "— nessun documento —" }), stato.docs.map((d) => el("option", { value: d.id, testo: d.titolo })));
    doc.value = v("documento", "");
    if (p) doc.disabled = true;
    const pag = el("input", { name: "pagina", type: "number", min: "0", value: String(v("pagina", 0) || "") });
    const testo = el("textarea", { name: "testo", rows: "5", maxlength: "20000", placeholder: "La regola scritta con parole tue" }, v("testo"));
    const nota = el("input", { name: "nota", value: v("nota"), maxlength: "2000" });
    const errore = el("p", { class: "rl-errore piccolo", role: "alert" });
    dialogo(p ? "Modifica la regola" : "Nuova regola", el("div", { class: "mr-campi" },
      el("label", { class: "riga-campo" }, el("span", { testo: "Nome" }), nome),
      el("label", { class: "riga-campo" }, el("span", { testo: "Categoria" }), cat),
      el("label", { class: "riga-campo" }, el("span", { testo: "Documento di origine" }), doc),
      el("label", { class: "riga-campo" }, el("span", { testo: "Pagina" }), pag),
      el("label", { class: "riga-campo an-largo" }, el("span", { testo: "Regola" }), testo),
      el("label", { class: "riga-campo an-largo" }, el("span", { testo: "Nota" }), nota), errore), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { type: "button", testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", type: "button", testo: "Salva", on: { click: async () => {
          const dati = { nome: nome.value, categoria: cat.value, testo: testo.value, nota: nota.value, pagina: Number(pag.value || 0) };
          try {
            if (p) await api("/regolamenti/analisi/" + encodeURIComponent(p.id), { metodo: "PATCH", dati });
            else await api("/regolamenti/analisi", { metodo: "POST", dati: { ...dati, documento: doc.value } });
            chiudi(); ricarica();
          } catch (e) { errore.textContent = e.message; }
        } } })),
    });
  }

  const via = tempoReale.su("regolamenti.analisi", () => ricarica());
  await ricarica();
  return () => via();
}
