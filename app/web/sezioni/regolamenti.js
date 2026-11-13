// RPG RULEBOOK LIBRARY (passo L1, docs/RULEBOOK_LIBRARY.md, 13/11/2026): catalogo dei giochi e dei documenti gratuiti
// pubblicati dagli editori, con filtri. Solo riferimenti alle fonti ufficiali: nessun documento è incluso nell'app.
import { el, svuota, api, avviso } from "../ui/ui.js";

const LINGUE = { it: "Italiano", en: "Inglese", de: "Tedesco", fr: "Francese", es: "Spagnolo" };
const FORMATI = { pdf: "PDF", web: "Pagina web", zip: "Archivio ZIP", epub: "EPUB", docx: "DOCX" };

export async function mostra(corpo) {
  const filtri = { gioco: "", genere: "", editore: "", lingua: "", tipologia: "", edizione: "", gratuito: false };
  const elenco = el("div", { class: "rl-elenco", "aria-live": "polite" });
  const conta = el("p", { class: "tenue piccolo rl-conta" });
  const avvisoMarchi = el("p", { class: "tenue piccolo rl-avviso" });
  const scelte = {};
  const menu = (chiave, etichetta) => {
    const s = el("select", { "aria-label": etichetta, on: { change: () => { filtri[chiave] = s.value; carica(); } } });
    scelte[chiave] = { s, etichetta };
    return el("label", { class: "rl-filtro" }, el("span", { testo: etichetta }), s);
  };
  let attesa;
  const nome = el("input", { type: "search", placeholder: "Nome del gioco…", "aria-label": "Nome del gioco",
    on: { input: () => { clearTimeout(attesa); attesa = setTimeout(() => { filtri.gioco = nome.value.trim(); carica(); }, 250); } } });
  const gratis = el("input", { type: "checkbox", on: { change: () => { filtri.gratuito = gratis.checked; carica(); } } });
  const azzera = el("button", { class: "piccolo", type: "button", testo: "Azzera i filtri", on: { click: () => {
    Object.assign(filtri, { gioco: "", genere: "", editore: "", lingua: "", tipologia: "", edizione: "", gratuito: false });
    nome.value = ""; gratis.checked = false; for (const k in scelte) scelte[k].s.value = ""; carica();
  } } });

  svuota(corpo, el("div", { class: "largo rl" },
    el("div", { class: "rl-banda" },
      el("div", {}, el("h2", { class: "rl-titolo", testo: "RPG Rulebook Library" }),
        el("p", { class: "rl-sotto", testo: "Catalogo dei giochi di ruolo e dei documenti gratuiti pubblicati dagli editori" }))),
    el("div", { class: "scheda-ui rl-filtri" },
      el("label", { class: "rl-filtro rl-nome" }, el("span", { testo: "Gioco" }), nome),
      menu("genere", "Genere"), menu("editore", "Editore"), menu("lingua", "Lingua"),
      menu("tipologia", "Tipologia di documento"), menu("edizione", "Edizione"),
      el("label", { class: "riga rl-gratis" }, gratis, el("span", { testo: "Solo documenti gratuiti" })),
      azzera),
    conta, elenco, avvisoMarchi));

  let primo = true;
  async function carica() {
    const q = new URLSearchParams();
    for (const [k, v] of Object.entries(filtri)) if (v) q.set(k, v === true ? "1" : v);
    let r;
    try { r = await api("/regolamenti/catalogo?" + q.toString()); } catch (e) { avviso(e.message, "errore"); return; }
    if (primo) { // i menu si riempiono una volta, con tutte le scelte del catalogo
      primo = false;
      const etichette = { lingue: (v) => LINGUE[v] || v, tipologie: (v) => r.tipologie[v] || v };
      const mappa = { genere: "generi", editore: "editori", lingua: "lingue", tipologia: "tipologie", edizione: "edizioni" };
      for (const [k, lista] of Object.entries(mappa)) {
        const f = etichette[lista] || ((v) => v);
        svuota(scelte[k].s, el("option", { value: "", testo: "Tutti" }), (r.filtri[lista] || []).map((v) => el("option", { value: v, testo: f(v) })));
      }
      avvisoMarchi.textContent = r.avviso + " Catalogo aggiornato al " + data(r.aggiornatoIl) + ".";
    }
    conta.textContent = r.giochi.length + " giochi su " + r.totale;
    svuota(elenco, r.giochi.length ? r.giochi.map((g) => schedaGioco(g, r.tipologie)) :
      el("p", { class: "tenue", testo: "Nessun gioco corrisponde ai filtri." }));
  }
  await carica();
}

function data(d) {
  if (!d) return "";
  const [a, m, g] = d.split("-");
  return g + "/" + m + "/" + a;
}

function collegamento(url, testo) {
  return el("a", { class: "rl-fonte", href: url, target: "_blank", rel: "noopener noreferrer", testo });
}

function schedaGioco(g, tipologie) {
  const verificato = g.sitoVerificatoIl ? "verificato il " + data(g.sitoVerificatoIl) : "indirizzo da verificare";
  return el("article", { class: "scheda-ui rl-gioco", "data-gioco": g.id },
    el("header", { class: "rl-testa" },
      el("h3", { testo: g.nome }),
      el("p", { class: "tenue piccolo", testo: g.editore + " · " + (g.edizioni || []).join(", ") }),
      el("div", { class: "rl-etichette" }, (g.generi || []).map((x) => el("span", { class: "etichetta", testo: x })))),
    el("p", { class: "piccolo" }, collegamento(g.sito, "Sito ufficiale ↗"), el("span", { class: "tenue", testo: " · " + verificato })),
    g.documenti.length ? el("ul", { class: "rl-documenti" }, g.documenti.map((d) => voceDocumento(d, g, tipologie))) :
      el("p", { class: "tenue piccolo", testo: "Nessun documento gratuito verificato nel catalogo: consulta il sito dell'editore." }));
}

function voceDocumento(d, g, tipologie) {
  const dati = [tipologie[d.tipologia] || d.tipologia, LINGUE[d.lingua] || d.lingua, FORMATI[d.formato] || d.formato, d.edizione]
    .filter(Boolean).join(" · ");
  return el("li", { class: "rl-doc", "data-doc": d.id },
    el("div", { class: "rl-doc-testa" },
      el("strong", { testo: d.titolo }),
      d.gratuito ? el("span", { class: "etichetta rl-gratis-et", testo: "Gratuito" }) : null,
      d.downloadDiretto ? el("span", { class: "etichetta rl-diretto", testo: "Download diretto consentito" }) : null),
    el("p", { class: "tenue piccolo", testo: dati + (d.editore ? " · " + d.editore : "") }),
    el("p", { class: "piccolo", testo: "Licenza: " + d.licenza }),
    el("details", { class: "piccolo" }, el("summary", { testo: "Condizioni d'uso" }), el("p", { testo: d.condizioni })),
    el("p", { class: "piccolo rl-azioni" },
      collegamento(d.url, "Apri la fonte ufficiale ↗"),
      el("span", { class: "tenue", testo: d.verificatoIl ? " · verificato il " + data(d.verificatoIl) : " · indirizzo da verificare" })));
}
