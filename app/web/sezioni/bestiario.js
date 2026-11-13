// Sezione «PNG e mostri» (solo Master): tutti i PNG dell'avventura e i mostri citati, con le schede.
// - Le voci di base sono in /carta/bestiario.json (schede SRD 5.1, CC-BY 4.0); il server salva ciò che
//   aggiunge il Master: immagine, note, scheda compilata, voci nuove (stessi dati del 📖 della Mappa).
// - Un PNG nuovo nasce da un modello ufficiale (Nobile, Guardia, Cavaliere…): la scheda si copia e si
//   può modificare.
// - La scheda di un mostro si può leggere dal manuale PDF caricato (web/ui/manuale_scheda.js): l'app
//   trova la pagina, compila un modulo e il Master controlla prima di salvare.
import { el, svuota, api, avviso, dialogo, conferma, carica } from "../ui/ui.js";
import { apriManuale, trovaPagina, leggiScheda, normalizza } from "../ui/manuale_scheda.js";
import { statistiche, CAR, segno, modCar } from "../ui/statistiche.js";
import { caricaCodice, nomiDi as nomiScheda, norma, linkCodice, statisticheCitano } from "../ui/codice_link.js";

// Modelli ufficiali per i PNG (appendice B del Manuale dei Mostri, presenti nell'SRD 5.1).
const MODELLI = ["mm-commoner", "mm-noble", "mm-guard", "mm-acolyte", "mm-priest", "mm-cultist", "mm-cult-fanatic", "mm-bandit", "mm-bandit-captain",
  "mm-thug", "mm-scout", "mm-spy", "mm-tribal-warrior", "mm-berserker", "mm-veteran", "mm-knight", "mm-gladiator", "mm-druid", "mm-mage", "mm-assassin", "mm-archmage"];
const FILTRI = [["tutti", "Tutti"], ["avventura", "Dell'avventura"], ["png", "PNG"], ["mostro", "Mostri"], ["catalogo", "Catalogo (tutti i mostri)"], ["miei", "Creati da te"], ["img", "Con immagine"], ["senza", "Senza scheda"]];
const MAX_CARTE = 240; // oltre, si chiede di affinare la ricerca (il catalogo ha quasi mille creature)

export async function mostra(corpo, { io, azioni }) {
  if (io.utente.ruolo !== "master") { svuota(corpo, el("p", { class: "errore", testo: "Solo il Master." })); return; }
  let base = [], dati = { entries: {}, extras: [], manuals: [] }, filtro = "tutti", ricerca = "";
  const elenco = el("div", { class: "griglia best-griglia" });
  const conteggio = el("span", { class: "tenue piccolo" });
  const cerca = el("input", { type: "search", placeholder: "Cerca per nome, ruolo, tipo o luogo…", "aria-label": "Cerca PNG e mostri", class: "best-cerca",
    on: { input: (e) => { ricerca = e.target.value; disegna(); } } });
  const chip = el("div", { class: "riga-flex best-filtri", role: "group", "aria-label": "Filtri" },
    FILTRI.map(([k, t]) => el("button", { class: "piccolo" + (k === filtro ? " primario" : ""), "aria-pressed": String(k === filtro), dataset: { f: k }, testo: t,
      on: { click: (e) => { filtro = k; chip.querySelectorAll("button").forEach((b) => { const on = b.dataset.f === k; b.classList.toggle("primario", on); b.setAttribute("aria-pressed", String(on)); }); disegna(); } } })));
  svuota(azioni,
    el("button", { class: "primario piccolo", testo: "＋ Nuovo PNG", on: { click: () => nuovoPNG() } }),
    el("button", { class: "piccolo", testo: "＋ Nuovo mostro", on: { click: () => nuovoMostro() } }),
    el("a", { href: "/manuali", "data-interno": true, class: "pulsante piccolo", testo: "📕 Manuali" }));
  svuota(corpo, el("div", { class: "largo" },
    el("section", { class: "scheda-ui" },
      el("div", { class: "riga-flex" }, cerca, conteggio), chip,
      el("p", { class: "tenue piccolo", testo: "Tutti i PNG dell'avventura e i mostri citati, più quelli che crei tu. Tocca una voce per la scheda, l'immagine e le note. Le schede di base vengono dal System Reference Document 5.1 (CC-BY 4.0)." })),
    elenco));

  async function ricarica() {
    if (!base.length) {
      // bestiario dell'avventura + catalogo dei mostri (mostri.json, convertito da strumenti/converti_mostri.py)
      const [a, c] = await Promise.all([fetch("/carta/bestiario.json").then((r) => r.json()), fetch("/carta/mostri.json").then((r) => (r.ok ? r.json() : { voci: [] })).catch(() => ({ voci: [] }))]);
      base = a.voci.concat(c.voci || []);
    }
    dati = await api("/bestiario");
    disegna();
  }

  // ----- Dati -----
  const ov = (id) => dati.entries[id] || {};
  const perId = (id) => tutte().find((e) => e.id === id);
  function tutte() {
    return base.map((e) => ({ ...e, mia: false })).concat(dati.extras.map((x) => ({ id: x.id, nome: x.nome, cat: x.cat, ruolo: x.ruolo, base: x.base || "", fonte: "Personale", pag: [], mia: true })));
  }
  // Scheda da mostrare: quella compilata dal Master, poi quella di base, poi quella del modello.
  function schedaDi(e) {
    if (ov(e.id).sheet) return { s: ov(e.id).sheet, da: "compilata" };
    if (e.scheda) return { s: e.scheda, da: "base" };
    if (e.base) { const b = base.find((x) => x.id === e.base); const s = ov(e.base).sheet || (b && b.scheda); if (s) return { s, da: "modello", modello: b }; }
    return null;
  }
  function sottotitolo(e) {
    const sc = schedaDi(e);
    if (e.cat === "png") return [e.profilo, e.ruolo].filter(Boolean).join(" · ") || (e.mia ? "PNG creato da te" : "PNG");
    if (sc) return "GS " + String(sc.s.gs || "?").split(" ")[0] + " · " + (sc.s.tipo || "");
    return e.fonte === "App. D" ? "Scheda nell'appendice D dell'avventura" : "Scheda da compilare";
  }

  function disegna() {
    const q = normalizza(ricerca);
    const lista = tutte().filter((e) => {
      if (filtro === "png" && e.cat !== "png") return false;
      if (filtro === "mostro" && e.cat === "png") return false;
      if (filtro === "miei" && !e.mia) return false;
      if (filtro === "img" && !ov(e.id).img) return false;
      if (filtro === "senza" && schedaDi(e)) return false;
      if (filtro === "avventura" && (e.fonte === "Codex" || e.mia)) return false;
      if (filtro === "catalogo" && e.fonte !== "Codex") return false;
      return !q || normalizza([e.nome, e.en, e.ruolo, e.profilo, e.note, sottotitolo(e)].join(" ")).includes(q);
    }).sort((a, b) => a.nome.localeCompare(b.nome, "it"));
    conteggio.textContent = lista.length + (lista.length === 1 ? " voce" : " voci");
    svuota(elenco, lista.length ? [lista.slice(0, MAX_CARTE).map(carta),
      lista.length > MAX_CARTE ? el("p", { class: "tenue", testo: "Mostrate le prime " + MAX_CARTE + " voci su " + lista.length + ": scrivi qualcosa nella ricerca per trovare le altre." }) : null]
      : el("p", { class: "tenue", testo: "Nessuna voce trovata." }));
  }

  function carta(e) {
    const img = ov(e.id).img;
    return el("button", { class: "carta-pg best-carta", "aria-label": e.nome + (e.cat === "png" ? ", PNG" : ", mostro"), on: { click: () => apriVoce(e.id) } },
      img ? el("img", { src: "/media/" + img, alt: "", loading: "lazy" }) : el("div", { class: "segnaposto-ritratto", testo: e.cat === "png" ? "👤" : "🐺" }),
      el("div", {},
        el("strong", { testo: e.nome }),
        el("div", { class: "tenue piccolo", testo: sottotitolo(e) }),
        el("div", { class: "riga-flex piccolo best-etichette" },
          el("span", { class: "etichetta", testo: e.cat === "png" ? "PNG" : "Mostro" }),
          e.mia ? el("span", { class: "etichetta approvato", testo: "tuo" }) : null,
          ov(e.id).sheet ? el("span", { class: "etichetta", testo: "scheda compilata" }) : null,
          e.fonte === "App. D" ? el("span", { class: "etichetta", testo: "App. D" }) : null,
          e.fonte === "Codex" ? el("span", { class: "etichetta", title: e.origine || "", testo: e.srd ? "catalogo · SRD" : "catalogo" }) : null)));
  }

  // ----- Scheda di una voce -----
  function apriVoce(id) {
    dialogo((perId(id) || {}).nome || "Voce", (chiudi) => {
      const box = el("div", { class: "best-voce" });
      const ridisegna = () => {
        const e = perId(id);
        if (!e) { chiudi(); return; }
        const o = ov(e.id), sc = schedaDi(e), img = o.img;
        const file = el("input", { type: "file", accept: "image/*", hidden: true, on: { change: async (ev) => {
          const f = ev.target.files[0]; if (!f) return;
          try { await carica("/bestiario/voci/" + encodeURIComponent(e.id) + "/immagine", f, null, "PUT"); await ricarica(); ridisegna(); avviso("Immagine salvata."); }
          catch (er) { avviso(er.message, "errore"); }
        } } });
        const note = el("textarea", { rows: "3", maxlength: "30000", placeholder: "Tattiche, segreti, voce, dove si trova…" }); note.value = o.notes || "";
        const pagina = paginaNota(e);
        svuota(box,
          el("header", { class: "best-testa" },
            el("span", { class: "etichetta", testo: e.cat === "png" ? "PNG" : "Mostro" }),
            e.en ? el("span", { class: "tenue piccolo", testo: e.en }) : null),
          el("div", { class: "best-voce-griglia" },
            el("div", { class: "best-img" },
              img ? el("img", { src: "/media/" + img, alt: "Immagine di " + e.nome }) : el("div", { class: "segnaposto-ritratto grande", testo: e.cat === "png" ? "👤" : "🐺" }),
              el("div", { class: "riga-flex" }, file,
                el("button", { class: "piccolo", testo: img ? "🖼 Cambia immagine" : "🖼 Carica immagine", on: { click: () => file.click() } }),
                img ? el("button", { class: "piccolo", testo: "Togli", on: { click: async () => { await api("/bestiario/voci/" + encodeURIComponent(e.id) + "/immagine", { metodo: "DELETE" }); await ricarica(); ridisegna(); } } }) : null),
              e.ruolo || e.profilo || e.note || e.origine ? el("p", { class: "tenue piccolo", testo: [e.ruolo, e.profilo, e.note, e.origine, e.ambiente ? "ambiente: " + e.ambiente : ""].filter(Boolean).join(" · ") }) : null,
              e.mia ? datiPersonali(e) : null),
            el("div", {},
              sc ? el("div", { class: "best-stat" },
                sc.da === "modello" ? el("p", { class: "tenue piccolo", testo: "Scheda del modello «" + sc.modello.nome + "»: modificala per farla sua." }) : null,
                sc.da === "compilata" && !e.mia ? el("p", { class: "tenue piccolo", testo: "Scheda compilata da te (sostituisce quella di base)." }) : null,
                sc.da === "compilata" && e.mia && e.base ? el("p", { class: "tenue piccolo", testo: "Scheda presa dal modello «" + ((base.find((x) => x.id === e.base) || {}).nome || e.base) + "»: puoi modificarla." }) : null,
                statistiche(sc.s))
                : el("p", { class: "tenue", testo: e.fonte === "App. D" ? "La scheda è nell'appendice D dell'avventura: leggila dal manuale o compilala." : "Nessuna scheda ancora: leggila dal manuale caricato o compilala a mano." }),
              el("div", { class: "riga-flex best-comandi" },
                el("button", { class: "primario piccolo", testo: "✏ " + (sc ? "Modifica la scheda" : "Compila la scheda"), on: { click: () => editor(e, sc ? sc.s : null) } }),
                el("button", { class: "piccolo", testo: "📕 Leggi dal manuale", title: "Trova la scheda nel manuale PDF e compila il modulo", on: { click: () => dalManuale(e) } }),
                pagina ? el("a", { class: "pulsante piccolo", href: "/manuali?apri=" + encodeURIComponent(pagina.m.id) + "&pagina=" + pagina.p, "data-interno": true, testo: "📖 Apri a pag. " + pagina.p + " del PDF", on: { click: () => chiudi() } }) : null,
                o.sheet && !e.mia ? el("button", { class: "piccolo", testo: "↺ Scheda di base", on: { click: async () => {
                  if (!await conferma("Tornare alla scheda di base?", "La scheda compilata da te per «" + e.nome + "» viene tolta.", "Torna alla base", true)) return;
                  await api("/bestiario/voci/" + encodeURIComponent(e.id) + "/scheda", { metodo: "PUT", dati: { scheda: null } }); await ricarica(); ridisegna();
                } } }) : null))),
          el("section", { class: "best-note" }, el("h3", { testo: "Note del Master" }), note,
            el("div", { class: "riga-flex" },
              el("button", { class: "piccolo", testo: "Salva le note", on: { click: async () => {
                try { await api("/bestiario/voci/" + encodeURIComponent(e.id), { metodo: "PATCH", dati: { note: note.value } }); await ricarica(); avviso("Note salvate."); } catch (er) { avviso(er.message, "errore"); }
              } } }),
              e.mia ? el("button", { class: "piccolo pericolo", testo: "🗑 Elimina la voce", on: { click: async () => {
                if (!await conferma("Eliminare «" + e.nome + "»?", "Si perdono scheda, immagine e note di questa voce.", "Elimina", true)) return;
                await api("/bestiario/voci/" + encodeURIComponent(e.id), { metodo: "DELETE" }); await ricarica(); chiudi();
              } } }) : null)),
          codiceDi(e));
      };
      voceAperta = ridisegna;
      ridisegna();
      return box;
    }, { largo: true });
  }
  let voceAperta = null;

  // Schede del Codice della campagna su questa voce (stesso nome o alias, oppure «Statistiche: «nome»»; fase L2)
  function codiceDi(e) {
    const box = el("section", { class: "best-codice" });
    caricaCodice().then((c) => {
      const n = norma(e.nome);
      const l = (c.schede || []).filter((x) => nomiScheda(x).some((y) => norma(y) === n) || statisticheCitano(x, e.nome));
      if (l.length) svuota(box, el("h3", { testo: "🗝 Nel Codice della campagna" }), el("div", { class: "riga-flex" }, l.map((x) => linkCodice(x))));
    }).catch(() => {});
    return box;
  }
  const aggiornaVoce = () => { if (voceAperta && document.body.contains(document.querySelector(".best-voce"))) voceAperta(); };

  function datiPersonali(e) {
    const nome = el("input", { maxlength: "80", valore: e.nome, "aria-label": "Nome" });
    const ruolo = el("input", { maxlength: "120", valore: e.ruolo || "", placeholder: "Ruolo", "aria-label": "Ruolo" });
    return el("div", { class: "sanita-form" }, el("label", {}, "Nome", nome), el("label", {}, "Ruolo", ruolo),
      el("button", { class: "piccolo", testo: "Salva nome e ruolo", on: { click: async () => {
        try { await api("/bestiario/voci/" + encodeURIComponent(e.id), { metodo: "PATCH", dati: { nome: nome.value, ruolo: ruolo.value } }); await ricarica(); aggiornaVoce(); }
        catch (er) { avviso(er.message, "errore"); }
      } } }));
  }

  // Pagina del PDF già nota: indice del manuale (ricerca automatica) per la voce o per la creatura di base.
  function paginaNota(e) {
    for (const id of [e.id, e.cat === "png" && e.fonte === "MM" ? e.base : ""].filter(Boolean)) {
      for (const m of ordineManuali(e)) if (m.index && m.index[id]) return { m, p: m.index[id] };
    }
    return null;
  }
  function ordineManuali(e) {
    const pref = e.cat === "png" || e.fonte === "App. D" ? ["avventura", "mm", "altro"] : ["mm", "avventura", "altro"];
    return [...dati.manuals].sort((a, b) => pref.indexOf(a.tipo) - pref.indexOf(b.tipo));
  }
  // il nome scritto dal Master (come nel manuale) viene per primo
  const nomiDi = (e, extra) => [extra, e.nome.replace(/\s*\(.*\)\s*$/, ""), e.en].filter(Boolean);

  // Statistiche: ../ui/statistiche.js (le usa anche la finestra del mostro nel tracker)

  // ----- Modulo della scheda -----
  // salva(scheda) decide dove salvare: voce esistente (PUT) o voce nuova (POST con la scheda).
  function editor(e, s, opz = {}) {
    s = s || { car: [10, 10, 10, 10, 10, 10], tratti: [], azioni: [], reazioni: [], legg: [] };
    const campo = (k, t, lungo) => { const i = el(lungo ? "textarea" : "input", { maxlength: lungo ? "600" : "200", rows: lungo ? "2" : null, "data-k": k }); i.value = s[k] || ""; return el("label", {}, t, i); };
    const testoElenco = (l) => (l || []).map((x) => (x.n ? x.n + ". " : "") + x.d).join("\n\n");
    const area = (k, t) => { const a = el("textarea", { rows: "5", "data-l": k, placeholder: "Nome. Descrizione\n\n(una voce per paragrafo, separate da una riga vuota)" }); a.value = testoElenco(s[k]); return el("label", {}, t, a); };
    const car = CAR.map((k, i) => el("label", { class: "best-car" }, k, el("input", { type: "number", min: "1", max: "30", valore: (s.car && s.car[i]) || 10, "data-c": i })));
    const form = el("div", { class: "sanita-form best-form" },
      opz.nota ? el("p", { class: "best-nota", testo: opz.nota }) : null,
      el("div", { class: "riga-flex" }, campo("taglia", "Taglia"), campo("tipo", "Tipo"), campo("all", "Allineamento")),
      el("div", { class: "riga-flex" }, campo("ca", "Classe Armatura"), campo("pf", "Punti ferita"), campo("vel", "Velocità")),
      el("div", { class: "riga-flex best-cars" }, car),
      el("div", { class: "riga-flex" }, campo("ts", "Tiri salvezza"), campo("abil", "Abilità")),
      el("div", { class: "riga-flex" }, campo("vuln", "Vulnerabilità ai danni"), campo("res", "Resistenze ai danni"), campo("imm", "Immunità ai danni")),
      el("div", { class: "riga-flex" }, campo("immc", "Immunità alle condizioni"), campo("sensi", "Sensi"), campo("ling", "Linguaggi"), campo("gs", "Sfida (GS)")),
      area("tratti", "Tratti"), area("azioni", "Azioni"), area("reazioni", "Reazioni"), area("legg", "Azioni leggendarie"),
      opz.testo ? el("details", {}, el("summary", { testo: "Testo letto dal manuale (per controllare)" }), el("pre", { class: "best-grezzo", testo: opz.testo })) : null);
    const leggi = () => {
      const o = { car: [] };
      form.querySelectorAll("[data-k]").forEach((i) => { o[i.dataset.k] = i.value.replace(/\s*\n\s*/g, " ").trim(); });
      form.querySelectorAll("[data-c]").forEach((i) => { o.car[+i.dataset.c] = Math.max(1, Math.min(30, parseInt(i.value, 10) || 10)); });
      form.querySelectorAll("[data-l]").forEach((a) => {
        o[a.dataset.l] = a.value.split(/\n\s*\n/).map((p) => p.replace(/\s*\n\s*/g, " ").trim()).filter(Boolean).map((p) => {
          const m = p.match(/^([^.]{1,120}?)\.\s+(.*)$/); return m ? { n: m[1].trim(), d: m[2].trim() } : { n: "", d: p };
        });
      });
      return o;
    };
    dialogo((opz.titolo || "Scheda") + " · " + e.nome, form, { largo: true, azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Salva la scheda", on: { click: async () => {
        try {
          const sc = leggi();
          if (opz.salva) await opz.salva(sc);
          else await api("/bestiario/voci/" + encodeURIComponent(e.id) + "/scheda", { metodo: "PUT", dati: { scheda: sc } });
          await ricarica(); chiudi(); aggiornaVoce(); avviso("Scheda salvata.");
        } catch (er) { avviso(er.message, "errore", 6000); }
      } } })) });
  }

  // ----- Lettura dal manuale -----
  async function dalManuale(e, opz = {}) {
    const manuali = ordineManuali(e);
    if (!manuali.length) {
      dialogo("Nessun manuale", el("p", { testo: "Carica prima il manuale in PDF (Manuale dei Mostri o l'avventura) nella sezione 📕 Manuali, poi torna qui." }));
      return;
    }
    const stato = el("p", { "aria-live": "polite", testo: "Cerco la scheda…" });
    const altroNome = el("input", { maxlength: "80", placeholder: "Nome come è scritto nel manuale", valore: opz.nome || "" });
    const scelta = el("select", {}, manuali.map((m) => el("option", { value: m.id, testo: m.nome })));
    const pag = el("input", { type: "number", min: "1", max: "5000", placeholder: "trovala da solo", class: "corto" });
    let chiudiDlg = null, inCorso = false;
    const vai = async () => {
      if (inCorso) return; inCorso = true;
      const m = manuali.find((x) => x.id === scelta.value), nomi = nomiDi(e, altroNome.value.trim());
      try {
        const doc = await apriManuale(m.id);
        let p = parseInt(pag.value, 10) || (m.index && m.index[e.id]) || 0;
        if (!p) {
          const r = await trovaPagina(doc, nomi, (x, n) => { stato.textContent = "Cerco «" + nomi[0] + "» nel manuale: pagina " + x + " di " + n + "…"; });
          if (r.senzaTesto) { stato.textContent = "⚠ Questo PDF è una scansione senza testo: non posso leggerlo. Compila la scheda a mano."; inCorso = false; return; }
          p = r.pagina;
          if (!p) { stato.textContent = "Non ho trovato «" + nomi[0] + "» come titolo in questo manuale. Prova con il nome esatto del manuale, un altro manuale o indica la pagina."; inCorso = false; return; }
        }
        stato.textContent = "Leggo la scheda a pagina " + p + "…";
        const r = await leggiScheda(doc, p, nomi);
        if (!r.trovata) { stato.textContent = "A pagina " + p + " non c'è un titolo «" + nomi[0] + "». Controlla il nome o la pagina."; inCorso = false; return; }
        if (e.id) api("/bestiario/manuali/" + encodeURIComponent(m.id) + "/indice", { dati: { voce: e.id, pagina: p } }).catch(() => {}).then(ricarica); // pagina ricordata
        chiudiDlg();
        editor(e, r.scheda, { titolo: "Dal manuale", testo: r.testo, salva: opz.salva,
          nota: "Letta da «" + m.nome + "», pagina " + r.pagine.join("-") + " del PDF. Controlla i valori prima di salvare: la lettura automatica può sbagliare (righe spezzate, colonne)." });
      } catch (er) { stato.textContent = "Lettura non riuscita: " + (er.message || er); }
      inCorso = false;
    };
    chiudiDlg = dialogo("📕 Leggi dal manuale · " + e.nome, el("div", { class: "sanita-form" },
      el("label", {}, "Manuale", scelta), el("label", {}, "Nome nel manuale (facoltativo)", altroNome), el("label", {}, "Pagina del PDF (facoltativa)", pag),
      el("p", { class: "tenue piccolo", testo: "L'app cerca il nome come titolo nelle pagine del manuale (sul tuo PC: può richiedere un minuto) e compila un modulo da controllare." }), stato), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Chiudi", on: { click: chiudi } }), el("button", { class: "primario", testo: "🔎 Cerca e leggi", on: { click: vai } })) });
    if (opz.subito) vai();
  }

  // ----- Nuovi PNG e mostri -----
  function nuovoPNG() {
    const nome = el("input", { maxlength: "80", placeholder: "Es. Conte Dragomir" });
    const ruolo = el("input", { maxlength: "120", placeholder: "Es. nobile decaduto di Borgofosco" });
    // Scheda di partenza: modelli per PNG, PNG dell'avventura e mostri (avventura e catalogo), tutte le schede di base
    // (richiesta dell'utente, 2026-11-08). Una ricerca filtra l'elenco, diviso in gruppi.
    const conScheda = base.filter((b) => b.scheda);
    const visti = new Set();
    const gruppi = [
      ["Modelli per PNG (Manuale dei Mostri, appendice B)", MODELLI.map((id) => conScheda.find((x) => x.id === id)).filter(Boolean)],
      ["PNG dell'avventura", conScheda.filter((b) => b.cat === "png" && !MODELLI.includes(b.id))],
      ["Mostri", conScheda.filter((b) => b.cat !== "png" && !MODELLI.includes(b.id)).sort((x, y) => x.nome.localeCompare(y.nome, "it"))],
    ].map(([t, l]) => [t, l.filter((b) => !visti.has(b.id) && visti.add(b.id))]);
    const gs = (b) => (b.scheda && b.scheda.gs ? " · GS " + String(b.scheda.gs).split(" ")[0] : "");
    const modello = el("select", { size: "8", "aria-label": "Scheda di partenza", class: "best-modelli" });
    const cercaModello = el("input", { type: "search", placeholder: "Cerca una scheda: Guardia, Lupo crudele, l'antagonista…", "aria-label": "Cerca una scheda di partenza" });
    const quante = el("span", { class: "tenue piccolo" });
    const riempi = () => {
      const q = normalizza(cercaModello.value.trim()), scelto = modello.value || "mm-commoner";
      let n = 0;
      svuota(modello, el("option", { value: "", testo: "Nessuno (scheda da compilare)" }),
        gruppi.map(([t, l]) => {
          const v = l.filter((b) => !q || normalizza(b.nome + " " + (b.en || "")).includes(q));
          n += v.length;
          return v.length ? el("optgroup", { label: t + " (" + v.length + ")" }, v.map((b) => el("option", { value: b.id, testo: b.nome + gs(b) + (b.en && b.en !== b.nome ? " (" + b.en + ")" : "") }))) : null;
        }));
      modello.value = [...modello.options].some((o) => o.value === scelto) ? scelto : (modello.options[1] ? modello.options[1].value : "");
      quante.textContent = n + " schede";
      mostraModello();
    };
    cercaModello.addEventListener("input", riempi);
    const anteprima = el("div", { class: "best-anteprima" });
    const mostraModello = () => { const b = base.find((x) => x.id === modello.value); svuota(anteprima, b && b.scheda ? statistiche(b.scheda) : null); };
    modello.addEventListener("change", mostraModello);
    const img = el("input", { type: "file", accept: "image/*" });
    dialogo("＋ Nuovo PNG", el("div", { class: "sanita-form" },
      el("label", {}, "Nome", nome), el("label", {}, "Ruolo", ruolo),
      el("label", {}, "Scheda di partenza (PNG o mostro)", cercaModello), el("div", { class: "riga-flex" }, quante), modello, anteprima,
      el("label", {}, "Immagine (facoltativa)", img),
      el("p", { class: "tenue piccolo", testo: "La scheda scelta (di un PNG o di un mostro) viene copiata nel nuovo PNG: dopo puoi cambiarla come vuoi." })), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Crea il PNG", on: { click: async () => {
          try {
            const r = await api("/bestiario/voci", { dati: { nome: nome.value, cat: "png", ruolo: ruolo.value, base: modello.value } });
            if (img.files[0]) await carica("/bestiario/voci/" + encodeURIComponent(r.id) + "/immagine", img.files[0], null, "PUT");
            await ricarica(); chiudi(); apriVoce(r.id);
          } catch (er) { avviso(er.message, "errore", 6000); }
        } } })) });
    riempi();
  }

  function nuovoMostro() {
    const nome = el("input", { maxlength: "80", placeholder: "Nome come nel manuale, es. Lupo crudele" });
    const ruolo = el("input", { maxlength: "120", placeholder: "Facoltativo: dove compare, chi è" });
    const crea = async (dalPdf) => {
      try {
        if (!nome.value.trim()) { avviso("Scrivi il nome.", "errore"); return; }
        if (dalPdf) {
          // La voce si crea solo quando il Master salva la scheda letta.
          const finta = { id: "", nome: nome.value.trim(), cat: "mostro", ruolo: ruolo.value, fonte: "Personale", pag: [] };
          dalManuale(finta, { subito: true, salva: async (sc) => { const r = await api("/bestiario/voci", { dati: { nome: finta.nome, cat: "mostro", ruolo: ruolo.value, scheda: sc } }); setTimeout(() => apriVoce(r.id), 50); } });
          return;
        }
        const r = await api("/bestiario/voci", { dati: { nome: nome.value, cat: "mostro", ruolo: ruolo.value } });
        await ricarica(); apriVoce(r.id);
      } catch (er) { avviso(er.message, "errore", 6000); }
    };
    dialogo("＋ Nuovo mostro", el("div", { class: "sanita-form" }, el("label", {}, "Nome", nome), el("label", {}, "Ruolo", ruolo),
      el("p", { class: "tenue piccolo", testo: "«Leggi dal manuale» cerca il nome nel PDF caricato e compila la scheda da controllare. Altrimenti crei la voce e compili la scheda a mano." })), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { testo: "Crea vuoto", on: { click: () => { chiudi(); crea(false); } } }),
        el("button", { class: "primario", testo: "📕 Leggi dal manuale", on: { click: () => { chiudi(); crea(true); } } })) });
  }

  await ricarica();
  // apertura diretta: /bestiario?voce=<id> (dal Codice della campagna)
  const diretta = new URLSearchParams(location.search).get("voce");
  if (diretta && perId(diretta)) apriVoce(diretta);
}
