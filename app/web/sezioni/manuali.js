// Manuali in PDF: il Master li carica, li consulta e decide con chi condividerli.
// I giocatori vedono solo quelli condivisi con loro; se il Master toglie la condivisione, il
// manuale sparisce subito (il server controlla il permesso a ogni richiesta del file).
import { el, svuota, api, carica, avviso, dialogo, conferma, quando, tempoReale } from "../ui/ui.js";
import { preparaPagine, statoPreparazione, condiviso, pronto } from "../ui/pagine_manuali.js"; // pagine per i telefoni (M2)
import { caricaPdfJs } from "../ui/pdfjs.js";
import { apriImportazione, mostraRiepilogo, SOTTOTIPI_REGOLA } from "../ui/importa_manuali.js"; // «📥 Importa dai manuali» (M3)
import { dimenticaManuale } from "../ui/manuale_scheda.js";

const TIPI = { altro: "Regole e altro", mm: "Manuale dei Mostri", avventura: "Avventura" };
const mb = (n) => (n / 1048576).toLocaleString("it-IT", { maximumFractionDigits: 1 }) + " MB";

export async function mostra(corpo, ctx) {
  const master = ctx.io.utente.ruolo === "master";
  const box = el("div", { class: "largo" });
  svuota(corpo, box);
  let lista = [], giocatori = [], lettore = null, importa = false; // importa: è aperta l'importazione o il suo riepilogo

  async function ricarica() {
    lista = await api("/manuali");
    if (master && !giocatori.length) giocatori = (await api("/utenti").catch(() => [])).filter((u) => u.stato === "approvato" && u.ruolo !== "master");
    if (lettore && !lista.some((m) => m.id === lettore.id)) {
      lettore.chiudi();
      avviso("Il Master ha tolto la condivisione di questo manuale.", "", 5000);
    }
    if (!lettore && !importa) disegna();
  }
  const tornaAllElenco = () => { importa = false; ctx.titolo.textContent = "Manuali"; ricarica().catch(() => {}); };
  function avviaImportazione(opz = {}) { importa = true; svuota(ctx.azioni); ctx.titolo.textContent = "Importa dai manuali"; apriImportazione(box, { ...opz, alla: tornaAllElenco }).then(() => {}); }
  async function ultimoRiepilogo() {
    try {
      const r = await api("/importazione");
      if (!r.voci || !r.voci.length) { avviso("Nessuna importazione fatta finora."); return; }
      importa = true; svuota(ctx.azioni); ctx.titolo.textContent = "Importa dai manuali";
      mostraRiepilogo(box, r, tornaAllElenco);
    } catch (e) { avviso(e.message, "errore"); }
  }
  // «🔁 Sostituisci il PDF»: stesso manuale, file nuovo; poi si propone l'importazione per vedere cosa è cambiato.
  function sostituisci(m) {
    const file = el("input", { type: "file", accept: "application/pdf,.pdf", hidden: true });
    file.addEventListener("change", async () => {
      const f = file.files[0];
      if (!f) return;
      const barra = el("progress", { max: "1", value: "0" });
      const chiudiAttesa = dialogo("Sostituzione di «" + m.nome + "»", el("div", {}, el("p", { testo: "Caricamento del PDF nuovo…" }), barra), { azioni: () => el("div") });
      try {
        await carica("/manuali/" + encodeURIComponent(m.id) + "/file?" + new URLSearchParams({ file: f.name }), f, (x) => { barra.value = x; }, "PUT");
        chiudiAttesa();
        await ricarica();
        if (await conferma("PDF sostituito", "«" + m.nome + "» ora usa il file nuovo (condivisione e impostazioni restano). Vuoi leggerlo subito per vedere cosa cambia nei mostri e nei PNG importati?", "📥 Importa da questo manuale")) {
          avviaImportazione({ soloManuale: m.id });
        }
      } catch (e) { chiudiAttesa(); avviso(e.message, "errore"); }
    });
    document.body.append(file); file.click(); setTimeout(() => file.remove(), 60000);
  }

  // «📝 Testo OCR…» (2026-11-11): file JSON «gasterbless-ocr» con il testo corretto di un PDF scansionato; l'importazione e
  // «Dal manuale…» del bestiario lo usano al posto del testo nascosto del PDF (il PDF non cambia).
  function testoOCR(m) {
    const file = el("input", { type: "file", accept: ".json,application/json", hidden: true });
    file.addEventListener("change", async () => {
      const f = file.files[0];
      if (!f) return;
      const barra = el("progress", { max: "1", value: "0" });
      const chiudiAttesa = dialogo("Testo OCR di «" + m.nome + "»", el("div", {}, el("p", { testo: "Caricamento di «" + f.name + "»…" }), barra), { azioni: () => el("div") });
      try {
        const r = await carica("/manuali/" + encodeURIComponent(m.id) + "/ocr", f, (x) => { barra.value = x; }, "PUT");
        chiudiAttesa();
        dimenticaManuale(m.id);
        await ricarica();
        if (await conferma("Testo OCR caricato", "«" + m.nome + "»: " + ((r && r.pagine) || "?") + " pagine di testo. Da ora l'importazione legge queste righe al posto del testo del PDF. Vuoi rileggere subito il manuale?", "📥 Importa da questo manuale")) {
          avviaImportazione({ soloManuale: m.id });
        }
      } catch (e) { chiudiAttesa(); avviso(e.message, "errore"); }
    });
    document.body.append(file); file.click(); setTimeout(() => file.remove(), 60000);
  }
  async function togliOCR(m) {
    if (!(await conferma("Togliere il testo OCR?", "L'importazione tornerà a leggere il testo nascosto del PDF di «" + m.nome + "». Gli elementi già importati restano.", "Togli"))) return;
    try { await api("/manuali/" + encodeURIComponent(m.id) + "/ocr", { metodo: "DELETE" }); dimenticaManuale(m.id); await ricarica(); } catch (e) { avviso(e.message, "errore"); }
  }

  function disegna() {
    righePag.clear();
    svuota(ctx.azioni,
      master ? el("button", { class: "primario", testo: "📥 Importa dai manuali", title: "Legge i PDF e aggiorna mostri e PNG, incantesimi, oggetti magici e il testo delle regole", on: { click: () => avviaImportazione() } }) : null,
      master ? el("button", { testo: "📋 Ultimo riepilogo", on: { click: ultimoRiepilogo } }) : null,
      master ? el("button", { testo: "📖 Regole dai manuali", title: "Il testo di talenti, condizioni, background, razze e privilegi importato dai manuali (solo tu)", on: { click: regoleDaiManuali } }) : null);
    const gruppi = Object.keys(TIPI).map((t) => [t, lista.filter((m) => m.tipo === t)]).filter(([, l]) => l.length);
    svuota(box,
      master ? caricamento() : null,
      lista.length ? gruppi.map(([t, l]) => el("section", { class: "scheda-ui" }, el("h2", { testo: TIPI[t] }),
        el("div", { class: "manuali-lista" }, l.map(riga))))
        : el("div", { class: "scheda-ui" }, el("h2", { testo: "Nessun manuale" }),
          el("p", { class: "tenue", testo: master ? "Carica i tuoi manuali in PDF: li consulti qui e decidi tu se e con chi condividerli." : "Il Master non ha ancora condiviso manuali con te." })));
  }

  // «📖 Regole dai manuali» (M3.4): ricerca nel testo delle regole importate, solo per il Master.
  async function regoleDaiManuali() {
    let voci = [];
    try { voci = await api("/elementi-manuali?tipo=regola"); } catch (e) { avviso(e.message, "errore"); return; }
    const cerca = el("input", { type: "search", placeholder: "Cerca una regola (es. Allerta, Accecato, Scurovisione)…", "aria-label": "Cerca una regola" });
    const elenco = el("div", { class: "regole-elenco" }), testo = el("div", { class: "regole-testo" });
    const norm = (t) => String(t || "").toLowerCase().normalize("NFKD").replace(/[\u0300-\u036f]/g, "");
    async function apri(v) {
      svuota(testo, el("p", { class: "tenue", testo: "Caricamento…" }));
      try {
        const x = await api("/elementi-manuali?id=" + encodeURIComponent(v.id));
        svuota(testo, el("h3", { testo: x.nome }), el("p", { class: "tenue piccolo", testo: (SOTTOTIPI_REGOLA[v.sottotipo] || "Regola") + " · " + (x.manualeNome || "manuale eliminato") + ", pagina " + x.pagina + " del PDF" }),
          el("p", { class: "regole-corpo", testo: x.testo }));
      } catch (e) { svuota(testo, el("p", { class: "errore", testo: e.message })); }
    }
    function disegnaElenco() {
      const q = norm(cerca.value.trim());
      const L = voci.filter((v) => !q || norm(v.nome).includes(q)).slice(0, 200);
      svuota(elenco, L.length ? L.map((v) => el("button", { class: "piccolo regole-voce", on: { click: () => apri(v) } }, el("b", { testo: v.nome }), " ", el("span", { class: "tenue", testo: SOTTOTIPI_REGOLA[v.sottotipo] || "" })))
        : el("p", { class: "tenue", testo: voci.length ? "Nessuna regola con questo nome." : "Nessuna regola importata: usa «📥 Importa dai manuali» con «Regole» sul Manuale del Giocatore." }));
    }
    cerca.addEventListener("input", disegnaElenco);
    disegnaElenco();
    dialogo("📖 Regole dai manuali", el("div", { class: "regole-dai-manuali" }, el("p", { class: "tenue piccolo", testo: voci.length + " voci importate. Le vedi solo tu; nella scheda le trovi anche aprendo un talento, un privilegio o un tratto." }),
      cerca, el("div", { class: "regole-due" }, elenco, testo)), { largo: true });
  }

  function condivisione(m) {
    if (m.condivisoTutti) return "Condiviso con tutti i giocatori";
    const nomi = (m.utenti || []).map((id) => (giocatori.find((g) => g.id === id) || {}).nome).filter(Boolean);
    return nomi.length ? "Condiviso con " + nomi.join(", ") : "Non condiviso (solo tu)";
  }

  function riga(m) {
    return el("article", { class: "manuale" },
      el("div", { class: "manuale-ico", "aria-hidden": "true", testo: "📕" }),
      el("div", { class: "spazio" },
        el("b", { testo: m.nome }),
        el("div", { class: "tenue", testo: mb(m.dimensione) + (master ? " · caricato " + quando(m.caricato) : "") }),
        master ? el("div", { class: m.condivisoTutti || (m.utenti || []).length ? "condiviso" : "tenue", testo: condivisione(m) }) : null,
        master ? statoPag(m) : null,
        master && m.ocr ? el("div", { class: "piccolo condiviso", testo: "📝 Testo OCR caricato: " + m.ocr.pagine + " pagine (" + quando(m.ocr.caricato) + "): l'importazione legge questo" }) : null),
      el("div", { class: "riga-flex" },
        el("button", { class: "primario", testo: "Apri", on: { click: () => apri(m) } }),
        master ? el("label", { class: "riga interruttore", title: "Condividi con tutti i giocatori" },
          el("input", { type: "checkbox", selezionato: m.condivisoTutti, "aria-label": "Condividi " + m.nome + " con tutti i giocatori",
            on: { change: (e) => salva(m, { condivisoTutti: e.target.checked }) } }), el("span", { testo: "Tutti" })) : null,
        master ? el("button", { testo: "Condivisione…", on: { click: () => scegli(m) } }) : null,
        master ? el("button", { testo: "Modifica", on: { click: () => modifica(m) } }) : null,
        master ? el("button", { testo: "🔁 Sostituisci il PDF", title: "Stesso manuale, file nuovo: condivisione, impostazioni ed elementi importati restano", on: { click: () => sostituisci(m) } }) : null,
        master ? el("button", { testo: m.ocr ? "📝 Testo OCR (sostituisci)" : "📝 Testo OCR…", title: "Carica un file .json «gasterbless-ocr» con il testo corretto di questo PDF scansionato: l'importazione userà quello", on: { click: () => testoOCR(m) } }) : null,
        master && m.ocr ? el("button", { testo: "Togli il testo OCR", on: { click: () => togliOCR(m) } }) : null,
        master ? el("button", { class: "pericolo", testo: "Elimina", on: { click: async () => {
          if (await conferma("Eliminare «" + m.nome + "»?", "Il PDF viene cancellato dal PC del Master e sparisce anche ai giocatori. Mostri, PNG e gli altri elementi importati da questo manuale restano.", "Elimina", true)) {
            try { await api("/manuali/" + m.id, { metodo: "DELETE" }); ricarica(); } catch (e) { avviso(e.message, "errore"); }
          }
        } } }) : null));
  }

  async function salva(m, dati) {
    try { await api("/manuali/" + m.id, { metodo: "PATCH", dati }); avviso("Condivisione aggiornata."); await ricarica(); }
    catch (e) { avviso(e.message, "errore"); ricarica(); }
  }

  function scegli(m) {
    const tutti = el("input", { type: "checkbox", selezionato: m.condivisoTutti });
    const spunte = giocatori.map((g) => ({ g, c: el("input", { type: "checkbox", selezionato: (m.utenti || []).includes(g.id) }) }));
    const aggiorna = () => spunte.forEach((s) => { s.c.disabled = tutti.checked; });
    tutti.addEventListener("change", aggiorna); aggiorna();
    dialogo("Condivisione di «" + m.nome + "»", el("div", {},
      el("label", { class: "riga" }, tutti, el("span", { testo: "Tutti i giocatori (anche quelli che si iscriveranno)" })),
      el("h3", { testo: "Oppure solo questi giocatori" }),
      giocatori.length ? spunte.map((s) => el("label", { class: "riga" }, s.c, el("span", { testo: s.g.nome }))) : el("p", { class: "tenue", testo: "Nessun giocatore approvato." }),
      el("p", { class: "tenue", testo: "Puoi togliere la condivisione in qualsiasi momento: il manuale sparisce subito dai dispositivi dei giocatori." })), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva", on: { click: async () => {
          chiudi();
          await salva(m, { condivisoTutti: tutti.checked, utenti: tutti.checked ? [] : spunte.filter((s) => s.c.checked).map((s) => s.g.id) });
        } } })),
    });
  }

  function modifica(m) {
    const nome = el("input", { maxlength: "80", valore: m.nome });
    const tipo = el("select", {}, Object.entries(TIPI).map(([k, v]) => el("option", { value: k, testo: v, selected: k === m.tipo })));
    dialogo("Modifica manuale", el("div", {}, el("label", {}, "Titolo", nome), el("label", {}, "Tipo", tipo)), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva", on: { click: async () => {
          try { await api("/manuali/" + m.id, { metodo: "PATCH", dati: { nome: nome.value, tipo: tipo.value } }); chiudi(); ricarica(); }
          catch (e) { avviso(e.message, "errore"); }
        } } })),
    });
  }

  function caricamento() {
    const file = el("input", { type: "file", accept: "application/pdf,.pdf", "aria-label": "File PDF del manuale" });
    const nome = el("input", { maxlength: "80", placeholder: "Titolo (se vuoto, il nome del file)" });
    const tipo = el("select", {}, Object.entries(TIPI).map(([k, v]) => el("option", { value: k, testo: v })));
    const barra = el("progress", { max: "1", value: "0", hidden: true });
    const bottone = el("button", { class: "primario", testo: "Carica", on: { click: async () => {
      const f = file.files[0];
      if (!f) { avviso("Scegli un file PDF.", "errore"); return; }
      if (!/\.pdf$/i.test(f.name) && f.type !== "application/pdf") { avviso("Il file deve essere un PDF.", "errore"); return; }
      if (f.size > 500 * 1048576) { avviso("File troppo grande: massimo 500 MB.", "errore"); return; }
      bottone.disabled = true; barra.hidden = false;
      try {
        const q = new URLSearchParams({ nome: nome.value, tipo: tipo.value, file: f.name });
        await carica("/manuali?" + q, f, (x) => { barra.value = x; }, "POST");
        avviso("Manuale caricato. Non è condiviso finché non lo decidi tu.");
        await ricarica();
      } catch (e) { avviso(e.message, "errore"); bottone.disabled = false; barra.hidden = true; }
    } } });
    return el("section", { class: "scheda-ui" }, el("h2", { testo: "Carica un manuale" }),
      el("label", {}, "File PDF (massimo 500 MB)", file),
      el("div", { class: "riga-flex" }, el("label", { class: "spazio" }, "Titolo", nome), el("label", {}, "Tipo", tipo)),
      el("div", { class: "riga-flex" }, bottone, barra),
      el("p", { class: "tenue", testo: "Carica solo manuali che possiedi: restano sul tuo PC e li vedono soltanto i giocatori con cui li condividi." }));
  }

  // Pagine pronte per i telefoni (M2): le prepara il PC del Master in sottofondo (../ui/pagine_manuali.js).
  const righePag = new Map();
  function statoPag(m) {
    const d = el("div", { class: "piccolo manuale-pagine" });
    righePag.set(m.id, { d, m });
    scriviPag(d, m, statoPreparazione());
    return d;
  }
  function scriviPag(d, m, st) {
    const p = m.pagine || {};
    if (!condiviso(m)) { svuota(d, el("span", { class: "tenue", testo: "📱 Le pagine per i telefoni si preparano quando lo condividi." })); return; }
    if (pronto(m)) { svuota(d, el("span", { class: "condiviso", testo: "📱 Pronto per i telefoni: " + p.totale + " pagine leggere." })); return; }
    if (st.attivo && st.manuale === m.id) {
      svuota(d, "📱 Preparazione per i telefoni: " + st.fatte + "/" + st.totale + " ", el("progress", { max: String(st.totale || 1), value: String(st.fatte) }));
      return;
    }
    const sulPC = !!(ctx.io && ctx.io.locale);
    svuota(d, el("span", { class: "tenue", testo: "📱 Pagine per i telefoni: " + (p.pronte || 0) + "/" + (p.totale || "?") + ". " }),
      sulPC ? el("button", { class: "piccolo", testo: "Prepara ora", on: { click: () => preparaPagine() } })
        : el("span", { class: "tenue", testo: "Si preparano da sole quando apri Gaster Bless sul PC." }));
  }
  const suPagine = (e) => {
    for (const { d, m } of righePag.values()) if (d.isConnected) scriviPag(d, m, e.detail);
    if (!e.detail.attivo) ricarica().catch(() => {}); // finito: si rilegge quante pagine sono pronte
  };
  if (master) document.addEventListener("gasterbless:pagine", suPagine);

  function apri(m, opz = {}) {
    lettore = lettorePDF(box, m, ctx, () => { lettore = null; ctx.titolo.textContent = "Manuali"; disegna(); }, opz);
  }

  const via = tempoReale.su("manuali.aggiornati", () => ricarica().catch(() => {}));
  await ricarica();
  const q = new URLSearchParams(location.search), apriId = q.get("apri");
  // ?apri=<manuale>&pagina=<n>: per esempio dalla sezione «PNG e mostri» alla pagina della scheda
  if (apriId) { const m = lista.find((x) => x.id === apriId); if (m) apri(m, { pagina: parseInt(q.get("pagina"), 10) || 0 }); }
  return () => { via(); document.removeEventListener("gasterbless:pagine", suPagine); if (lettore) lettore.chiudi(true); };
}

// ===== Lettore PDF (pdf.js locale) =====

// il lettore PDF sta in ../ui/pdfjs.js (prima era qui: manuale_scheda.js lo importava da questa sezione e il giro di import
// rompeva «PNG e mostri» aperta per prima, 2026-11-11); resta esportato da qui per chi lo cercava in questo modulo
export { caricaPdfJs };

const normalizza = (t) => t.toLocaleLowerCase("it").normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/\s+/g, " ");

// opz.pagina: pagina del PDF da aprire (altrimenti l'ultima letta su questo dispositivo).
// Ai giocatori, se il PC del Master ha preparato le pagine (M2, ../ui/pagine_manuali.js), si sfogliano le immagini leggere
// invece dei pezzi del PDF; «📄 PDF originale» torna al PDF (più nitido ingrandendo). Il Master usa sempre il PDF.
export function lettorePDF(box, m, ctx, alChiudere, opz = {}) {
  const chiave = "gasterbless.manuale." + m.id;
  const puoImmagini = pronto(m) && !(ctx.io && ctx.io.utente && ctx.io.utente.ruolo === "master");
  let immagini = puoImmagini && !opz.pdf, doc = null, totPag = 0, testi = null;
  let pagina = 1, zoom = 1, chiuso = false, rendering = null, ricerca = 0;
  try { pagina = Math.max(1, parseInt(localStorage.getItem(chiave), 10) || 1); } catch { /* memoria non disponibile */ }
  if (opz.pagina) pagina = Math.max(1, opz.pagina);
  const tela = el("canvas", { class: "pdf-pagina", "aria-label": "Pagina del manuale" });
  const immagine = el("img", { class: "pdf-pagina pdf-immagine", alt: "Pagina del manuale", decoding: "async" });
  const area = el("div", { class: "pdf-area", tabindex: "0" }, el("p", { class: "tenue", testo: "Apertura del manuale…" }));
  const numero = el("input", { type: "number", min: "1", class: "pdf-num", "aria-label": "Vai alla pagina", on: { change: () => vai(parseInt(numero.value, 10)) } });
  const totale = el("span", { class: "tenue", testo: "/ —" });
  const cerca = el("input", { type: "search", placeholder: "Cerca nel testo…", "aria-label": "Cerca nel manuale", class: "pdf-cerca" });
  const esito = el("span", { class: "tenue pdf-esito", "aria-live": "polite" });
  const cambia = puoImmagini ? el("button", { class: "pdf-modo", on: { click: () => (immagini ? apriPdf() : apriImmagini()) } }) : null;
  cerca.addEventListener("keydown", (e) => { if (e.key === "Enter") trova(cerca.value); });
  const barra = el("div", { class: "pdf-barra" },
    el("button", { testo: "← Elenco", on: { click: () => chiudi() } }),
    el("b", { class: "pdf-titolo", testo: m.nome }),
    el("span", { class: "riga-flex" },
      el("button", { testo: "◀", "aria-label": "Pagina precedente", on: { click: () => vai(pagina - 1) } }), numero, totale,
      el("button", { testo: "▶", "aria-label": "Pagina successiva", on: { click: () => vai(pagina + 1) } })),
    el("span", { class: "riga-flex" },
      el("button", { testo: "−", "aria-label": "Riduci", on: { click: () => { zoom = Math.max(0.5, zoom / 1.25); disegna(); } } }),
      el("button", { testo: "Adatta", on: { click: () => { zoom = 1; disegna(); } } }),
      el("button", { testo: "+", "aria-label": "Ingrandisci", on: { click: () => { zoom = Math.min(4, zoom * 1.25); disegna(); } } }), cambia),
    el("span", { class: "riga-flex" }, cerca, esito));
  svuota(box, el("div", { class: "pdf-lettore" }, barra, area));
  ctx.titolo.textContent = m.nome;
  const tasti = (e) => {
    if (/INPUT|TEXTAREA|SELECT/.test(document.activeElement.tagName)) return;
    if (e.key === "ArrowRight" || e.key === "PageDown") { vai(pagina + 1); e.preventDefault(); }
    if (e.key === "ArrowLeft" || e.key === "PageUp") { vai(pagina - 1); e.preventDefault(); }
  };
  document.addEventListener("keydown", tasti);
  const ridimensiona = () => disegna();
  window.addEventListener("resize", ridimensiona);

  const urlPagina = (n) => "/api/v1/manuali/" + encodeURIComponent(m.id) + "/pagine/" + n + "?v=" + encodeURIComponent(m.pagine.versione || "");
  function pronti(n) {
    totPag = n; totale.textContent = "/ " + n; numero.max = n;
    pagina = Math.min(pagina, n);
    if (cambia) cambia.textContent = immagini ? "📄 PDF originale" : "⚡ Pagine leggere";
  }
  function apriImmagini() {
    if (doc) { doc.destroy(); doc = null; }
    immagini = true; pronti(m.pagine.totale);
    svuota(area, immagine); disegna();
  }
  async function apriPdf() {
    immagini = false;
    if (cambia) cambia.disabled = true;
    svuota(area, el("p", { class: "tenue", testo: "Apertura del PDF…" }));
    try {
      const lib = await caricaPdfJs();
      // I manuali grandi si leggono a pezzi (richieste Range): si scaricano solo le pagine aperte.
      const d = await lib.getDocument({ url: "/api/v1/manuali/" + encodeURIComponent(m.id) + "/file", isEvalSupported: false,
        disableAutoFetch: true, rangeChunkSize: 1 << 20, standardFontDataUrl: "/vendor/pdfjs/standard_fonts/" }).promise;
      if (chiuso || immagini) { d.destroy(); return; }
      doc = d; pronti(doc.numPages);
      svuota(area, tela); disegna();
    } catch (e) {
      svuota(area, el("p", { class: "errore", testo: "Impossibile aprire il manuale: " + (e.message || e) }));
    } finally { if (cambia) cambia.disabled = false; }
  }
  if (immagini) apriImmagini(); else apriPdf();

  function vai(n) {
    if (!totPag || !Number.isFinite(n)) return;
    pagina = Math.max(1, Math.min(totPag, n));
    try { localStorage.setItem(chiave, String(pagina)); } catch { /* */ }
    disegna(); area.scrollTop = 0;
  }

  async function disegna() {
    if (chiuso || !totPag) return;
    numero.value = pagina;
    const larghezza = Math.max(200, area.clientWidth - 24);
    if (immagini) {
      immagine.style.width = Math.floor(larghezza * zoom) + "px";
      const u = urlPagina(pagina);
      if (immagine.getAttribute("src") !== u) immagine.src = u;
      for (const k of [1, 2]) if (pagina + k <= totPag) new Image().src = urlPagina(pagina + k); // le prossime, già pronte
      return;
    }
    if (!doc) return;
    const n = pagina;
    const p = await doc.getPage(n);
    if (n !== pagina || chiuso || !doc) return;
    const base = p.getViewport({ scale: 1 });
    const scala = (larghezza / base.width) * zoom;
    const dpr = window.devicePixelRatio || 1;
    const vp = p.getViewport({ scale: scala * dpr });
    if (rendering) { try { rendering.cancel(); } catch { /* */ } }
    tela.width = Math.floor(vp.width); tela.height = Math.floor(vp.height);
    tela.style.width = Math.floor(vp.width / dpr) + "px"; tela.style.height = Math.floor(vp.height / dpr) + "px";
    rendering = p.render({ canvasContext: tela.getContext("2d"), viewport: vp });
    try { await rendering.promise; } catch { /* annullato da un nuovo disegno */ }
  }

  // Ricerca: dalla pagina dopo quella aperta, fino in fondo e poi dall'inizio.
  async function trova(q) {
    q = normalizza(q.trim());
    if (!totPag || q.length < 2) { esito.textContent = q ? "Scrivi almeno 2 lettere." : ""; return; }
    const mio = ++ricerca;
    if (immagini && !testi) {
      esito.textContent = "Cerco…";
      try { testi = ((await api("/manuali/" + encodeURIComponent(m.id) + "/testo")).pagine || []).map(normalizza); } catch { testi = []; }
      if (mio !== ricerca || chiuso) return;
    }
    for (let i = 1; i <= totPag; i++) {
      const n = ((pagina - 1 + i) % totPag) + 1;
      let t;
      if (immagini) t = testi[n - 1] || "";
      else {
        if (!doc) return;
        if (i % 10 === 0) esito.textContent = "Cerco… " + i + "/" + totPag;
        t = normalizza((await (await doc.getPage(n)).getTextContent()).items.map((x) => x.str).join(" "));
        if (mio !== ricerca || chiuso) return;
      }
      if (t.includes(q)) { esito.textContent = "Trovato a pagina " + n; vai(n); return; }
    }
    esito.textContent = "Non trovato (nei PDF fatti di sole immagini la ricerca non funziona).";
  }

  function chiudi(silenzioso) {
    if (chiuso) return;
    chiuso = true; ricerca++;
    document.removeEventListener("keydown", tasti);
    window.removeEventListener("resize", ridimensiona);
    if (doc) doc.destroy();
    if (!silenzioso) alChiudere();
  }
  return { id: m.id, chiudi };
}
