// MY RULEBOOKS (passo L3, docs/RULEBOOK_LIBRARY.md, 13/11/2026): archivio personale dei documenti del Master, scaricati
// dalla RPG RULEBOOK LIBRARY o importati da lui. Importazione con controlli sul server, gioco/edizione/tipologia/lingua,
// categorie personali, versioni dello stesso regolamento, ricerca nel titolo e nel contenuto, lettura, sostituzione,
// eliminazione. Il testo dei PDF per la ricerca si legge qui nel browser (pdf.js) e si manda al server.
import { el, svuota, api, avviso, carica, dialogo, conferma, tempoReale } from "../ui/ui.js";
import { schedeLibreria } from "../ui/libreria_schede.js";

const LINGUE = { "": "—", it: "Italiano", en: "Inglese", de: "Tedesco", fr: "Francese", es: "Spagnolo" };
const FORMATI_OK = ".pdf,.docx,.epub,.rtf,.txt,.md,.markdown,.html,.htm";
const mb = (n) => (n < 1048576 ? Math.max(1, Math.round(n / 1024)) + " KB" : (n / 1048576).toFixed(1).replace(".", ",") + " MB");

export async function mostra(corpo) {
  const stato = { docs: [], tipologie: {}, categorie: [], campagne: [], giochiCatalogo: [], filtri: { gioco: "", edizione: "", categoria: "", tipologia: "" } };
  const elenco = el("div", { class: "mr-elenco", "aria-live": "polite" });
  const risultati = el("div", { class: "mr-risultati" });
  const conta = el("p", { class: "tenue piccolo" });
  const menuFiltri = {};
  const filtro = (k, etichetta) => {
    const s = el("select", { "aria-label": etichetta, on: { change: () => { stato.filtri[k] = s.value; disegna(); } } });
    menuFiltri[k] = s;
    return el("label", { class: "rl-filtro" }, el("span", { testo: etichetta }), s);
  };
  let attesa;
  const cerca = el("input", { type: "search", placeholder: "Cerca nel titolo e nel contenuto…", "aria-label": "Cerca nei miei documenti",
    on: { input: () => { clearTimeout(attesa); attesa = setTimeout(cercaOra, 350); } } });
  const datalist = el("datalist", { id: "mr-giochi" });

  svuota(corpo, el("div", { class: "largo rl" },
    schedeLibreria("miei"),
    el("div", { class: "rl-banda" },
      el("div", {}, el("h2", { class: "rl-titolo", testo: "My Rulebooks" }),
        el("p", { class: "rl-sotto", testo: "I tuoi regolamenti e documenti: importati da te o scaricati dalla RPG Rulebook Library" }))),
    el("div", { class: "mr-comandi" },
      el("button", { class: "primario", type: "button", testo: "＋ Importa un documento", on: { click: () => finestraDocumento(null) } }),
      el("button", { type: "button", testo: "🏷 Categorie", on: { click: finestraCategorie } })),
    el("div", { class: "scheda-ui rl-filtri" },
      el("label", { class: "rl-filtro rl-nome" }, el("span", { testo: "Ricerca" }), cerca),
      filtro("gioco", "Gioco"), filtro("edizione", "Edizione"), filtro("categoria", "Categoria"), filtro("tipologia", "Tipologia")),
    risultati, conta, elenco, datalist));

  async function ricarica() {
    try {
      const [d, c, cp] = await Promise.all([api("/regolamenti/miei"), api("/regolamenti/categorie"), api("/campagne")]);
      stato.docs = d.documenti; stato.tipologie = d.tipologie; stato.categorie = c.categorie; stato.campagne = cp.campagne;
    } catch (e) { avviso(e.message, "errore"); return; }
    if (!stato.giochiCatalogo.length) {
      try { stato.giochiCatalogo = (await api("/regolamenti/catalogo")).giochi.map((g) => g.nome); } catch { /* facoltativo */ }
    }
    const giochi = [...new Set([...stato.giochiCatalogo, ...stato.docs.map((d) => d.gioco).filter(Boolean)])].sort();
    svuota(datalist, giochi.map((g) => el("option", { value: g })));
    const opzioni = (k, valori, etichetta = (v) => v) => {
      const s = menuFiltri[k], v = stato.filtri[k];
      svuota(s, el("option", { value: "", testo: "Tutti" }), valori.map((x) => el("option", { value: x, testo: etichetta(x) })));
      s.value = valori.includes(v) ? v : "";
      stato.filtri[k] = s.value;
    };
    opzioni("gioco", [...new Set(stato.docs.map((d) => d.gioco || "(senza gioco)"))].sort());
    opzioni("edizione", [...new Set(stato.docs.map((d) => d.edizione).filter(Boolean))].sort());
    opzioni("categoria", stato.categorie.map((c) => c.id), (id) => (stato.categorie.find((c) => c.id === id) || {}).nome || id);
    opzioni("tipologia", Object.keys(stato.tipologie), (k) => stato.tipologie[k]);
    disegna();
  }

  function disegna() {
    const f = stato.filtri;
    const passa = (d) => (!f.gioco || (d.gioco || "(senza gioco)") === f.gioco) && (!f.edizione || d.edizione === f.edizione) &&
      (!f.categoria || d.categoria === f.categoria) && (!f.tipologia || d.tipologia === f.tipologia);
    const visibili = stato.docs.filter(passa);
    conta.textContent = visibili.length + " documenti su " + stato.docs.length;
    if (!stato.docs.length) {
      svuota(elenco, el("div", { class: "scheda-ui" }, el("p", { testo: "La tua libreria è vuota." }),
        el("p", { class: "tenue", testo: "Importa i tuoi documenti con «＋ Importa un documento», oppure scarica quelli gratuiti dalla RPG Rulebook Library." })));
      return;
    }
    // versioni sotto il documento principale; raggruppate per gioco
    const perId = new Map(stato.docs.map((d) => [d.id, d]));
    const principali = visibili.filter((d) => !d.versioneDi || !perId.has(d.versioneDi) || !passa(perId.get(d.versioneDi)));
    const gruppi = new Map();
    for (const d of principali) {
      const g = d.gioco || "(senza gioco)";
      if (!gruppi.has(g)) gruppi.set(g, []);
      gruppi.get(g).push(d);
    }
    svuota(elenco, [...gruppi.keys()].sort().map((g) => el("section", { class: "mr-gruppo" },
      el("h3", { class: "mr-gioco", testo: g }),
      gruppi.get(g).map((d) => schedaDoc(d, visibili.filter((v) => v.versioneDi === d.id))))));
  }

  function schedaDoc(d, versioni) {
    const cat = stato.categorie.find((c) => c.id === d.categoria);
    return el("article", { class: "scheda-ui mr-doc", "data-mio": d.id },
      el("div", { class: "rl-doc-testa" }, el("strong", { testo: d.titolo }),
        d.versione ? el("span", { class: "etichetta", testo: d.versione }) : null,
        cat ? el("span", { class: "etichetta mr-cat", testo: "🏷 " + cat.nome }) : null,
        el("span", { class: "etichetta", testo: d.origine === "download" ? "Scaricato" : "Importato" })),
      el("p", { class: "tenue piccolo", testo: [stato.tipologie[d.tipologia] || d.tipologia, d.edizione && "ed. " + d.edizione, d.lingua ? LINGUE[d.lingua] || d.lingua : "",
        d.formato.toUpperCase(), mb(d.dimensione), d.testo ? "testo per la ricerca ✓" : "senza testo per la ricerca"].filter(Boolean).join(" · ") }),
      d.licenza ? el("p", { class: "piccolo tenue", testo: "Licenza: " + d.licenza + (d.fonteUrl ? " · fonte: " + new URL(d.fonteUrl).hostname : "") }) : null,
      d.note ? el("p", { class: "piccolo", testo: d.note }) : null,
      (d.campagne || []).length ? el("div", { class: "mr-campagne" }, d.campagne.map((id) => el("span", { class: "etichetta", testo: "🏰 " + ((stato.campagne.find((c) => c.id === id) || {}).nome || id) }))) : null,
      el("div", { class: "mr-azioni" },
        el("button", { class: "piccolo primario", type: "button", testo: "Apri", on: { click: () => apri(d) } }),
        el("a", { class: "pulsante piccolo", href: "/api/v1/regolamenti/miei/" + encodeURIComponent(d.id) + "/file?scarica=1", testo: "Scarica" }),
        el("button", { class: "piccolo", type: "button", testo: "Modifica", on: { click: () => finestraDocumento(d) } }),
        el("button", { class: "piccolo", type: "button", testo: "Campagne…", on: { click: () => finestraCampagne(d) } }),
        el("button", { class: "piccolo", type: "button", testo: "Sostituisci il file", on: { click: () => sostituisci(d) } }),
        d.formato === "pdf" && !d.testo ? el("button", { class: "piccolo", type: "button", testo: "Prepara la ricerca", on: { click: () => leggiPDF(d.id, "/api/v1/regolamenti/miei/" + encodeURIComponent(d.id) + "/file") } }) : null,
        el("button", { class: "piccolo pericolo", type: "button", testo: "Elimina", on: { click: () => elimina(d) } })),
      versioni.length ? el("div", { class: "mr-versioni" }, el("p", { class: "piccolo tenue", testo: "Altre versioni:" }), versioni.map((v) => schedaDoc(v, []))) : null);
  }

  function apri(d) {
    const url = "/api/v1/regolamenti/miei/" + encodeURIComponent(d.id) + "/file";
    if (d.formato === "pdf") { window.open(url, "_blank", "noopener"); return; }
    lettore(d.id, d.titolo, 0);
  }

  async function lettore(id, titolo, parte) {
    let t;
    try { t = await api("/regolamenti/miei/" + encodeURIComponent(id) + "/testo?da=" + Math.max(0, parte - 2) + "&n=60"); } catch (e) { avviso(e.message, "errore"); return; }
    const corpoTesto = el("div", { class: "mr-lettore" }, t.parti.map((p, i) => el("section", { class: "mr-parte" + (t.da + i === parte ? " evidenziata" : ""), "data-parte": String(t.da + i) },
      p.e ? el("h4", { testo: p.e }) : null, el("p", { testo: p.t }))));
    const piu = t.da + t.parti.length < t.totale ? el("p", { class: "tenue piccolo", testo: "Mostrate le parti " + (t.da + 1) + "–" + (t.da + t.parti.length) + " di " + t.totale + "." }) : null;
    if (!t.totale) corpoTesto.append(el("p", { class: "tenue", testo: "Nessun testo disponibile per questo documento." }));
    dialogo(titolo, el("div", {}, corpoTesto, piu), { largo: true });
    setTimeout(() => corpoTesto.querySelector(".evidenziata")?.scrollIntoView({ block: "center" }), 60);
  }

  async function cercaOra() {
    const q = cerca.value.trim();
    if (!q) { svuota(risultati); return; }
    let r;
    try { r = await api("/regolamenti/miei/cerca?q=" + encodeURIComponent(q)); } catch (e) { svuota(risultati, el("p", { class: "rl-errore piccolo", testo: e.message })); return; }
    svuota(risultati, el("div", { class: "scheda-ui" },
      el("h3", { testo: "Risultati per «" + q + "»" }),
      r.risultati.length ? el("ul", { class: "mr-trovati" }, r.risultati.map((x) => el("li", {},
        el("button", { class: "collegamento", type: "button", testo: x.titolo + (x.dove ? " — " + x.dove : x.nelTitolo ? " (titolo)" : ""), on: { click: () => {
          const d = stato.docs.find((v) => v.id === x.doc);
          if (d && d.formato === "pdf" && x.pagina) window.open("/api/v1/regolamenti/miei/" + encodeURIComponent(x.doc) + "/file#page=" + x.pagina, "_blank", "noopener");
          else if (x.parte >= 0) lettore(x.doc, x.titolo, x.parte); else if (d) apri(d);
        } } }),
        x.estratto ? el("p", { class: "piccolo tenue", testo: x.estratto }) : null))) :
        el("p", { class: "tenue", testo: "Nessun risultato. I PDF senza «testo per la ricerca» si cercano solo per titolo." })));
  }

  // legge il testo delle pagine di un PDF nel browser e lo manda al server (per la ricerca)
  async function leggiPDF(id, sorgente) {
    const barra = el("progress", { max: "1", value: "0" });
    const chiudi = dialogo("Preparo la ricerca nel PDF", el("div", {}, el("p", { testo: "Leggo il testo delle pagine…" }), barra), { azioni: () => el("div") });
    try {
      const { paginePDF } = await import("../ui/biblioteca.js");
      const pagine = await paginePDF(sorgente, (n, t) => { barra.max = t; barra.value = n; });
      if (!pagine.some((p) => p.trim())) throw new Error("Il PDF non contiene testo (forse è scansionato): si cerca solo per titolo.");
      const r = await api("/regolamenti/miei/" + encodeURIComponent(id) + "/testo", { metodo: "POST", dati: { pagine } });
      avviso("Ricerca pronta: " + r.parti + " brani.");
    } catch (e) { avviso(e.message, "errore", 6000); }
    chiudi();
    ricarica();
  }

  function campiDocumento(d) {
    const v = (k, pred = "") => (d && d[k]) || pred;
    const testo = (k, etichetta, attr = {}) => el("label", { class: "riga-campo" }, el("span", { testo: etichetta }), el("input", { name: k, value: v(k), ...attr }));
    const scelta = (k, etichetta, opzioni) => {
      const s = el("select", { name: k }, opzioni.map(([val, t]) => el("option", { value: val, testo: t })));
      s.value = v(k, k === "tipologia" ? "personale" : "");
      return el("label", { class: "riga-campo" }, el("span", { testo: etichetta }), s);
    };
    const principali = stato.docs.filter((x) => !x.versioneDi && (!d || x.id !== d.id));
    return el("div", { class: "mr-campi" },
      testo("titolo", "Titolo", { maxlength: "200", placeholder: d ? "" : "dal nome del file" }),
      testo("gioco", "Gioco", { maxlength: "100", list: "mr-giochi" }),
      testo("edizione", "Edizione", { maxlength: "60" }),
      scelta("tipologia", "Tipologia", Object.entries(stato.tipologie)),
      scelta("lingua", "Lingua", Object.entries(LINGUE)),
      testo("editore", "Editore", { maxlength: "100" }),
      scelta("categoria", "Categoria", [["", "Nessuna"], ...stato.categorie.map((c) => [c.id, c.nome])]),
      scelta("versioneDi", "Versione di", [["", "— (documento principale)"], ...principali.map((x) => [x.id, x.titolo + (x.edizione ? " (" + x.edizione + ")" : "")])]),
      testo("versione", "Nome della versione", { maxlength: "60", placeholder: "es. 2ª stampa, errata 2025" }),
      el("label", { class: "riga-campo" }, el("span", { testo: "Note" }), el("textarea", { name: "note", rows: "2", maxlength: "4000" }, v("note"))));
  }

  function leggiCampi(box) {
    const o = {};
    box.querySelectorAll("[name]").forEach((x) => { o[x.name] = x.value.trim(); });
    return o;
  }

  function finestraDocumento(d) {
    const campi = campiDocumento(d);
    const file = d ? null : el("input", { type: "file", accept: FORMATI_OK, "aria-label": "File da importare" });
    const barra = el("progress", { max: "1", value: "0", hidden: true });
    const errore = el("p", { class: "rl-errore piccolo", role: "alert" });
    dialogo(d ? "Modifica «" + d.titolo + "»" : "Importa un documento", el("div", {},
      d ? null : el("p", { class: "tenue piccolo", testo: "PDF, DOCX, EPUB, RTF, TXT, MD o HTML, fino a 200 MB. Importa solo documenti che hai il diritto di usare." }),
      file, campi, barra, errore), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { type: "button", testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", type: "button", testo: d ? "Salva" : "Importa", on: { click: async (ev) => {
          errore.textContent = "";
          const dati = leggiCampi(campi);
          ev.target.disabled = true;
          try {
            if (d) {
              await api("/regolamenti/miei/" + encodeURIComponent(d.id), { metodo: "PATCH", dati });
            } else {
              const f = file.files[0];
              if (!f) throw new Error("Scegli un file.");
              if (f.size > 200 * 1048576) throw new Error("File troppo grande: massimo 200 MB.");
              const q = new URLSearchParams({ file: f.name });
              for (const [k, val] of Object.entries(dati)) if (val) q.set(k, val);
              barra.hidden = false;
              const nuovo = await carica("/regolamenti/miei?" + q.toString(), f, (x) => { barra.value = x; }, "POST", f.name);
              avviso("Importato: «" + nuovo.titolo + "».");
              chiudi();
              if (nuovo.formato === "pdf") await leggiPDF(nuovo.id, f);
              ricarica();
              return;
            }
            chiudi(); ricarica();
          } catch (e) { errore.textContent = e.message; ev.target.disabled = false; barra.hidden = true; }
        } } })),
    });
  }

  function sostituisci(d) {
    const file = el("input", { type: "file", accept: FORMATI_OK, hidden: true, on: { change: async () => {
      const f = file.files[0];
      if (!f) return;
      try {
        const nuovo = await carica("/regolamenti/miei/" + encodeURIComponent(d.id) + "/file?file=" + encodeURIComponent(f.name), f, null, "PUT", f.name);
        avviso("File sostituito: «" + nuovo.titolo + "».");
        if (nuovo.formato === "pdf") await leggiPDF(nuovo.id, f);
        ricarica();
      } catch (e) { avviso(e.message, "errore", 6000); }
    } } });
    document.body.append(file);
    file.click();
    setTimeout(() => file.remove(), 60000);
  }

  async function elimina(d) {
    if (!(await conferma("Eliminare il documento?", "«" + d.titolo + "» e il suo file spariscono dalla libreria. Le sue versioni restano.", "Elimina", true))) return;
    try { await api("/regolamenti/miei/" + encodeURIComponent(d.id), { metodo: "DELETE" }); avviso("Documento eliminato."); ricarica(); }
    catch (e) { avviso(e.message, "errore"); }
  }

  // In quali campagne usare il documento (documento supplementare o scheda personalizzata).
  function finestraCampagne(d) {
    const righe = stato.campagne.map((c) => {
      const usa = (ruolo, attivo) => {
        const x = el("input", { type: "checkbox", selezionato: attivo, on: { change: async () => {
          try { await api("/campagne/" + encodeURIComponent(c.id) + "/documenti", { metodo: "PUT", dati: { documento: d.id, ruolo, presente: x.checked } }); }
          catch (e) { x.checked = !x.checked; avviso(e.message, "errore"); }
        } } });
        return el("label", { class: "riga" }, x, el("span", { testo: ruolo === "scheda" ? "scheda personalizzata" : "documento supplementare" }));
      };
      return el("div", { class: "scheda-ui", "data-campagna": c.id }, el("strong", { testo: c.nome }),
        c.regolamento === d.id ? el("p", { class: "piccolo tenue", testo: "È il regolamento principale di questa campagna." }) : null,
        usa("supplementare", c.supplementari.includes(d.id)), usa("scheda", c.schede.includes(d.id)));
    });
    dialogo("Campagne di «" + d.titolo + "»", el("div", {}, righe.length ? righe : el("p", { testo: "Nessuna campagna." }),
      el("p", { class: "tenue piccolo", testo: "Lo stesso documento può stare in più campagne. Il regolamento principale si sceglie nella pagina Campagne." })),
      { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { class: "primario", type: "button", testo: "Fatto", on: { click: () => { chiudi(); ricarica(); } } })) });
  }

  function finestraCategorie() {
    const lista = el("ul", { class: "mr-categorie" });
    const nome = el("input", { maxlength: "60", placeholder: "Nuova categoria", "aria-label": "Nome della nuova categoria" });
    const ridisegna = () => svuota(lista, stato.categorie.length ? stato.categorie.map((c) => el("li", {},
      el("span", { testo: c.nome + " (" + c.documenti + ")" }),
      el("button", { class: "piccolo", type: "button", testo: "Rinomina", on: { click: async () => {
        const n = prompt("Nuovo nome della categoria", c.nome);
        if (!n) return;
        try { await api("/regolamenti/categorie/" + c.id, { metodo: "PATCH", dati: { nome: n } }); await ricarica(); ridisegna(); } catch (e) { avviso(e.message, "errore"); }
      } } }),
      el("button", { class: "piccolo pericolo", type: "button", testo: "Elimina", on: { click: async () => {
        try { await api("/regolamenti/categorie/" + c.id, { metodo: "DELETE" }); await ricarica(); ridisegna(); } catch (e) { avviso(e.message, "errore"); }
      } } }))) : el("li", { class: "tenue", testo: "Nessuna categoria." }));
    ridisegna();
    dialogo("Categorie", el("div", {}, lista, el("div", { class: "riga-flex" }, nome,
      el("button", { class: "primario", type: "button", testo: "Aggiungi", on: { click: async () => {
        try { await api("/regolamenti/categorie", { metodo: "POST", dati: { nome: nome.value } }); nome.value = ""; await ricarica(); ridisegna(); }
        catch (e) { avviso(e.message, "errore"); }
      } } })), el("p", { class: "tenue piccolo", testo: "Eliminando una categoria i documenti restano, senza categoria." })));
  }

  const via = tempoReale.su("regolamenti.miei", () => ricarica());
  const via2 = tempoReale.su("campagne", () => ricarica());
  await ricarica();
  return () => { via(); via2(); };
}
