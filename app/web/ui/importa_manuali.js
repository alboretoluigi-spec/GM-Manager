// «📥 Importa dai manuali» (fase M3, docs/PIANO_IMPORTA_MANUALI.md, 2026-11-11). Solo il Master, sul PC: legge i PDF
// scelti pagina per pagina (con pdf.js, il testo non va ai giocatori), manda i candidati a Gaster Bless, che li confronta con i
// dati attuali (internal/server/importazione.go), e mostra il riepilogo: per ogni voce stato, provenienza, cosa cambia;
// il Master spunta e conferma. Mostri e PNG (M3.1), incantesimi (M3.2), oggetti magici (M3.3), regole come testo per il Master (M3.4).
import { el, svuota, api, avviso, dialogo } from "./ui.js";
import { apriManuale, schedeNellaPagina, normalizza } from "./manuale_scheda.js";
import { righeDelManuale, leggiIncantesimi } from "./manuale_incantesimi.js";
import { leggiOggetti } from "./manuale_oggetti.js";
import { leggiRegole, vociNote } from "./manuale_regole.js";

const TIPI = [
  { k: "mostro", nome: "Mostri e PNG", pronto: true },
  { k: "incantesimo", nome: "Incantesimi", pronto: true },
  { k: "oggetto", nome: "Oggetti magici", pronto: true },
  { k: "regola", nome: "Regole (talenti, condizioni, background, razze, privilegi: testo solo per te)", pronto: true },
];
export const STATI = {
  nuovo: ["🆕", "Nuovo"], cambiato: ["✏️", "Cambiato"], uguale: ["✔", "Uguale"], a_mano: ["✋", "Modificato a mano da te"],
  non_valido: ["⚠", "Non leggibile"],
};
const FILTRI = [["tutti", "Tutti"], ["nuovo", "Nuovi"], ["cambiato", "Cambiati"], ["a_mano", "Modificati a mano"], ["incerto", "Da controllare"], ["uguale", "Uguali"], ["applicata", "Già confermati"]];

/** Apre la scelta dei manuali; a lettura finita chiama mostraRiepilogo(box, riepilogo). soloManuale: id per «Sostituisci il PDF». */
export async function apriImportazione(box, { soloManuale = "", alla = null } = {}) {
  let manuali = [];
  try { manuali = await api("/manuali"); } catch (e) { avviso(e.message, "errore"); return; }
  if (!manuali.length) { avviso("Carica prima almeno un manuale.", "errore"); return; }
  const spunte = manuali.map((m) => ({ m, c: el("input", { type: "checkbox", selezionato: !soloManuale || m.id === soloManuale }) }));
  const tipi = TIPI.map((t) => ({ t, c: el("input", { type: "checkbox", selezionato: t.pronto, disabled: !t.pronto }) }));
  dialogo("📥 Importa dai manuali", el("div", { class: "imp-scelta" },
    el("p", { class: "tenue", testo: "Gaster Bless legge i PDF qui sul PC, pagina per pagina, e poi ti mostra cosa ha trovato: confermi tu, voce per voce. Il testo dei manuali lo vedi solo tu." }),
    el("h3", { testo: "Da quali manuali" }),
    spunte.map((s) => el("label", { class: "riga" }, s.c, el("span", { testo: s.m.nome }))),
    el("h3", { testo: "Cosa cercare" }),
    tipi.map((x) => el("label", { class: "riga" }, x.c, el("span", { testo: x.t.nome + (x.t.pronto ? "" : " (in arrivo)") })))), {
    azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Inizia la lettura", on: { click: () => {
        const scelti = spunte.filter((s) => s.c.checked).map((s) => s.m);
        const cosa = tipi.filter((x) => x.c.checked).map((x) => x.t.k);
        if (!scelti.length || !cosa.length) { avviso("Scegli almeno un manuale e cosa cercare.", "errore"); return; }
        chiudi();
        leggi(box, scelti, cosa, alla);
      } } })),
  });
}

// Lettura dei PDF con avanzamento; si può fermare (si confronta ciò che è stato letto fino a lì).
async function leggi(box, manuali, cosa, alla) {
  let fermato = false;
  const titolo = el("b", { testo: "Preparazione…" }), riga = el("div", { class: "tenue piccolo" }), barra = el("progress", { max: "1", value: "0" });
  const trovati = el("div", { class: "piccolo" });
  svuota(box, el("section", { class: "scheda-ui imp-lettura" }, el("h2", { testo: "📥 Lettura dei manuali" }), titolo, barra, riga, trovati,
    el("div", { class: "riga-flex" }, el("button", { testo: "⏹ Ferma e mostra ciò che ho trovato", on: { click: () => { fermato = true; } } }))));
  const candidati = [], problemi = [];
  for (const [mi, m] of manuali.entries()) {
    if (fermato) break;
    titolo.textContent = "Manuale " + (mi + 1) + " di " + manuali.length + ": " + m.nome;
    let doc;
    try { doc = await apriManuale(m.id); } catch (e) { problemi.push(m.nome + ": non si apre (" + (e.message || e) + ")"); continue; }
    barra.max = doc.numPages;
    let senzaTesto = 0;
    for (let p = 1; p <= doc.numPages && !fermato; p++) {
      barra.value = p;
      riga.textContent = "Pagina " + p + " di " + doc.numPages;
      try {
        if (cosa.includes("mostro")) {
          for (const x of await schedeNellaPagina(doc, p)) {
            candidati.push({ tipo: "mostro", nome: x.nome, manuale: m.id, pagina: p, dati: x.scheda, testo: x.testo, incerto: x.incerto, motivi: x.motivi, traccia: x.traccia });
          }
        }
        if (p <= 3 && !doc.righeOCR && !(await (await doc.getPage(p)).getTextContent()).items.length) senzaTesto++;
      } catch (e) { problemi.push(m.nome + ", p. " + p + ": " + (e.message || e)); }
      trovati.textContent = "Trovati finora: " + candidati.filter((c) => c.tipo === "mostro").length + " mostri e PNG";
      if (p % 20 === 0) await new Promise((r) => setTimeout(r, 0)); // la pagina resta reattiva
    }
    if (senzaTesto >= 3) problemi.push(m.nome + ": sembra fatto di sole immagini (senza testo), non si può leggere");
    // incantesimi e oggetti magici: il manuale si legge una volta sola come flusso di righe (le descrizioni passano di pagina)
    if ((cosa.includes("incantesimo") || cosa.includes("oggetto") || cosa.includes("regola")) && !fermato && senzaTesto < 3) {
      let righe = [];
      try { righe = await righeDelManuale(doc, (p, n) => { barra.value = p; riga.textContent = "Incantesimi, oggetti e regole: pagina " + p + " di " + n; }, () => fermato); }
      catch (e) { problemi.push(m.nome + " (incantesimi e oggetti): " + (e.message || e)); }
      if (cosa.includes("incantesimo")) {
        try {
          const arch = await archivioIncantesimi();
          for (const x of leggiIncantesimi(righe).incantesimi) {
            candidati.push({ tipo: "incantesimo", nome: x.nome, manuale: m.id, pagina: x.pagina, dati: x.dati, testo: x.testo, incerto: x.incerto, motivi: x.motivi, base: arch.trova(x.nome) });
          }
        } catch (e) { problemi.push(m.nome + " (incantesimi): " + (e.message || e)); }
      }
      if (cosa.includes("oggetto")) {
        try {
          const OM = await motoreOggetti();
          for (const x of leggiOggetti(righe)) {
            // i bonus riconoscibili nel testo diventano modificatori tecnici (valori, non testo): la scheda li calcola
            const sug = OM.suggerisci(x.nome + "\n" + x.testo, { tipo: x.dati.tipo, sint: x.dati.sint, nome: x.nome });
            const dati = { ...x.dati, ...(sug.mod.length ? { mod: sug.mod } : {}), ...(sug.bonus ? { bonus: sug.bonus } : {}) };
            candidati.push({ tipo: "oggetto", nome: x.nome, manuale: m.id, pagina: x.pagina, dati, testo: x.testo, incerto: x.incerto, motivi: x.motivi });
          }
        } catch (e) { problemi.push(m.nome + " (oggetti magici): " + (e.message || e)); }
      }
      if (cosa.includes("regola") && m.tipo !== "mm" && m.tipo !== "avventura") { // le regole stanno nei manuali di regole, non nelle avventure
        try {
          const note = await archivioRegole();
          for (const x of leggiRegole(righe, note)) {
            candidati.push({ tipo: "regola", sottotipo: x.sottotipo, nome: x.nome, manuale: m.id, pagina: x.pagina, dati: { sottotipo: x.sottotipo }, testo: x.testo, incerto: x.incerto, motivi: x.motivi });
          }
        } catch (e) { problemi.push(m.nome + " (regole): " + (e.message || e)); }
      }
    }
    trovati.textContent = "Trovati finora: " + candidati.filter((c) => c.tipo === "mostro").length + " mostri e PNG, " + candidati.filter((c) => c.tipo === "incantesimo").length + " incantesimi, " +
      candidati.filter((c) => c.tipo === "oggetto").length + " oggetti magici, " + candidati.filter((c) => c.tipo === "regola").length + " regole";
  }
  titolo.textContent = "Confronto con i dati di Gaster Bless…";
  try {
    const rie = await api("/importazione/confronta", { metodo: "POST", dati: { candidati } });
    rie.problemi = problemi;
    mostraRiepilogo(box, rie, alla);
  } catch (e) { svuota(box, el("p", { class: "errore", testo: "Confronto non riuscito: " + e.message })); }
}

/** Riepilogo dell'importazione: filtri, voci con spunta, confronto, conferma. */
export function mostraRiepilogo(box, rie, alla = null) {
  const voci = rie.voci || [];
  let filtro = "tutti";
  const scelte = new Set(voci.map((v, i) => [v, i]).filter(([v, i]) => predefinita(v, i, voci)).map(([, i]) => i));
  const elenco = el("div", { class: "imp-elenco" });
  const conferma = el("button", { class: "primario", on: { click: applica } });
  const conta = (f) => voci.filter((v) => passa(v, f)).length;
  const chip = el("div", { class: "riga-flex imp-filtri" });
  function passa(v, f) {
    if (f === "tutti") return true;
    if (f === "incerto") return v.incerto || v.stato === "non_valido";
    if (f === "applicata") return v.applicata;
    return v.stato === f && !v.applicata;
  }
  function disegnaChip() {
    svuota(chip, FILTRI.filter(([f]) => f === "tutti" || conta(f)).map(([f, t]) => el("button", { class: "piccolo" + (f === filtro ? " primario" : ""),
      testo: t + " (" + conta(f) + ")", on: { click: () => { filtro = f; disegna(); } } })));
  }
  function disegna() {
    disegnaChip();
    const mostrate = voci.map((v, i) => [v, i]).filter(([v]) => passa(v, filtro));
    svuota(elenco, mostrate.length ? mostrate.map(([v, i]) => rigaVoce(v, i)) : el("p", { class: "tenue", testo: "Nessuna voce." }));
    conferma.textContent = "✔ Conferma le voci scelte (" + scelte.size + ")";
    conferma.disabled = !scelte.size;
  }
  function rigaVoce(v, i) {
    const [ico, nome] = STATI[v.stato] || ["", v.stato];
    const puo = confermabile(v);
    const sp = el("input", { type: "checkbox", selezionato: scelte.has(i), disabled: !puo, "aria-label": "Conferma " + v.nome,
      on: { change: (e) => { e.target.checked ? scelte.add(i) : scelte.delete(i); conferma.textContent = "✔ Conferma le voci scelte (" + scelte.size + ")"; conferma.disabled = !scelte.size; } } });
    const dettagli = el("div", { class: "imp-dettagli", hidden: true });
    return el("article", { class: "imp-voce imp-" + v.stato + (v.applicata ? " imp-fatta" : "") },
      el("div", { class: "riga-flex" }, sp,
        el("div", { class: "spazio" },
          el("b", { testo: v.nome }), v.nomeVoce && v.nomeVoce.toLowerCase() !== v.nome.toLowerCase() ? el("span", { class: "tenue", testo: " → " + v.nomeVoce }) : null,
          el("div", { class: "piccolo" }, el("span", { class: "imp-stato", testo: (v.applicata ? "✅ Confermato" : ico + " " + nome) }),
            v.incerto ? el("span", { class: "imp-dubbio", testo: " · ⚠ lettura da controllare" }) : null,
            el("span", { class: "tenue", testo: " · " + (v.manualeNome || "manuale") + ", p. " + v.pagina }),
            v.doppio ? el("span", { class: "tenue", testo: " · letto anche in " + v.doppio }) : null),
          v.campi && v.campi.length && v.stato !== "uguale" ? el("div", { class: "piccolo tenue", testo: "Cambia: " + v.campi.join(", ") }) : null,
          (v.motivi || []).length ? el("div", { class: "piccolo imp-dubbio", testo: "Da controllare: " + v.motivi.join(", ") }) : null,
          v.errore ? el("div", { class: "piccolo imp-dubbio", testo: "Non valida: " + v.errore }) : null),
        el("button", { class: "piccolo", testo: "Confronta", on: { click: () => { dettagli.hidden = !dettagli.hidden; if (!dettagli.childNodes.length) svuota(dettagli, confronto(v)); } } })),
      dettagli);
  }
  async function applica() {
    conferma.disabled = true;
    try {
      const r = await api("/importazione/applica", { metodo: "POST", dati: { indici: [...scelte].sort((a, b) => a - b) } });
      avviso("Confermate " + r.applicate + " voci." + (r.saltate ? " (" + r.saltate + " saltate)" : ""));
      const nuovo = await api("/importazione");
      mostraRiepilogo(box, nuovo, alla);
    } catch (e) { avviso(e.message, "errore"); conferma.disabled = false; }
  }
  const problemi = (rie.problemi || []).length ? el("details", { class: "agg-esito agg-ripristinato" }, el("summary", { testo: "⚠ " + rie.problemi.length + " problemi di lettura" }),
    el("ul", {}, rie.problemi.map((p) => el("li", { class: "piccolo", testo: p })))) : null;
  // Motivi delle letture dubbie, contati: dicono subito che cosa non va con l'impaginazione di un manuale.
  const motivi = new Map();
  for (const v of voci) {
    for (const m of v.motivi || []) motivi.set(m, (motivi.get(m) || 0) + 1);
    if (v.errore) motivi.set("non valida: " + v.errore, (motivi.get("non valida: " + v.errore) || 0) + 1);
  }
  const elencoMotivi = motivi.size ? el("p", { class: "piccolo imp-dubbio", testo: "Letture dubbie: " + [...motivi].sort((a, b) => b[1] - a[1]).map(([m, n]) => n + " " + m).join(" · ") }) : null;
  svuota(box, el("section", { class: "scheda-ui imp-riepilogo" },
    el("div", { class: "riga-flex" }, el("h2", { class: "spazio", testo: "📋 Riepilogo dell'importazione" }), alla ? el("button", { testo: "← Manuali", on: { click: alla } }) : null),
    el("p", { class: "tenue", testo: voci.length + " voci lette. Di serie sono spuntate quelle nuove e cambiate; quelle che avevi modificato a mano e quelle da controllare no. Le voci uguali non serve confermarle." }),
    problemi, elencoMotivi, chip,
    el("div", { class: "riga-flex" },
      el("button", { class: "piccolo imp-tutto", testo: "☑ Spunta tutto", title: "Spunta tutte le voci mostrate (con il filtro scelto) che si possono confermare, comprese quelle modificate a mano e da controllare",
        on: { click: () => { voci.forEach((v, i) => { if (passa(v, filtro) && confermabile(v)) scelte.add(i); }); disegna(); } } }),
      el("button", { class: "piccolo", testo: "Spunta nuove e cambiate", on: { click: () => { voci.forEach((v, i) => { if (predefinita(v, i, voci)) scelte.add(i); }); disegna(); } } }),
      el("button", { class: "piccolo", testo: "Togli tutte le spunte", on: { click: () => { scelte.clear(); disegna(); } } }), conferma,
      el("button", { class: "piccolo", testo: "📋 Copia diagnostica", title: "Un testo breve da incollare a chi sistema Gaster Bless: numeri, motivi e qualche esempio di lettura", on: { click: () => diagnostica(voci, motivi) } })),
    elenco));
  disegna();
}

// Diagnostica da incollare in chat (2026-11-11): numeri, motivi e le prime righe lette di 3 voci dubbie e 1 buona.
function diagnostica(voci, motivi) {
  const conta = {};
  for (const v of voci) conta[v.stato + (v.incerto ? "+dubbio" : "")] = (conta[v.stato + (v.incerto ? "+dubbio" : "")] || 0) + 1;
  const esempio = (v) => ["— " + v.nome + " (" + (v.manualeNome || "") + ", p. " + v.pagina + ") stato " + v.stato + (v.motivi && v.motivi.length ? ", " + v.motivi.join(", ") : "") + (v.errore ? ", " + v.errore : ""),
    ...(v.traccia ? ["  traccia: " + v.traccia] : []),
    v.tipo === "mostro" ? "  letto: CA «" + ((v.dati || {}).ca || "") + "» PF «" + ((v.dati || {}).pf || "") + "» car " + JSON.stringify((v.dati || {}).car || []) + " sfida «" + ((v.dati || {}).gs || "") + "»"
      : "  letto (" + v.tipo + "): " + JSON.stringify(v.dati || {}).slice(0, 300),
    ...String(v.testo || "").split("\n").slice(0, 14).map((r) => "  | " + r.slice(0, 110))].join("\n");
  // esempi con motivi diversi (prima uno per motivo, poi gli altri), al massimo 5
  const tutte = voci.filter((v) => v.incerto || v.errore), visti = new Set(), dubbie = [];
  for (const v of tutte) { const m = (v.motivi || []).join("+") || v.errore; if (!visti.has(m)) { visti.add(m); dubbie.push(v); } }
  for (const v of tutte) if (dubbie.length < 5 && !dubbie.includes(v)) dubbie.push(v);
  dubbie.length = Math.min(dubbie.length, 5);
  const buona = voci.find((v) => !v.incerto && !v.errore);
  const testo = ["Diagnostica importazione Gaster Bless", "voci: " + voci.length + " " + JSON.stringify(conta),
    "motivi: " + [...motivi].map(([m, n]) => n + " " + m).join(" · "), ...dubbie.map(esempio), ...(buona ? ["buona:", esempio(buona)] : [])].join("\n");
  const area = el("textarea", { class: "imp-diagnostica", readonly: true, rows: "16" });
  area.value = testo;
  dialogo("📋 Diagnostica", el("div", {}, el("p", { class: "tenue", testo: "Copiato negli appunti (se il browser lo permette). Incollalo in chat: contiene i numeri e poche righe lette dal manuale." }), area), { largo: true });
  setTimeout(() => area.select(), 50);
  if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(testo).catch(() => {});
}

// Si può confermare: non già fatta, leggibile e diversa da ciò che c'è.
const confermabile = (v) => !v.applicata && v.stato !== "non_valido" && v.stato !== "uguale";

// Spuntate di serie: nuove o cambiate, lette bene, non già confermate, solo la prima se la stessa voce compare due volte.
function predefinita(v, i, voci) {
  if (v.applicata || v.incerto || !["nuovo", "cambiato"].includes(v.stato)) return false;
  return !voci.slice(0, i).some((x) => x.tipo === v.tipo && x.chiave === v.chiave);
}

// Confronto affiancato: valori attuali di Gaster Bless e valori letti dal manuale.
function confronto(v) {
  if (v.tipo === "incantesimo") return confrontoIncantesimo(v);
  if (v.tipo === "oggetto") return confrontoOggetto(v);
  if (v.tipo === "regola") return confrontoRegola(v);
  if (v.tipo !== "mostro") return el("p", { class: "tenue", testo: "Confronto non disponibile." });
  const a = v.attuale || {}, n = v.dati || {};
  const brevi = [["ca", "CA"], ["pf", "PF"], ["vel", "Velocità"], ["car", "FOR DES COS INT SAG CAR"], ["ts", "Tiri salvezza"], ["abil", "Abilità"], ["res", "Resistenze"],
    ["imm", "Immunità"], ["immc", "Imm. condizioni"], ["sensi", "Sensi"], ["ling", "Linguaggi"], ["gs", "Sfida"]];
  const val = (x, k) => (Array.isArray(x[k]) ? x[k].join(" ") : x[k] || "—");
  const elenco = (x, k) => (x[k] || []).map((p) => p.n).filter(Boolean).join(", ") || "—";
  return el("table", { class: "tabella imp-confronto" },
    el("thead", {}, el("tr", {}, el("th", { testo: "" }), el("th", { testo: v.attuale ? "Ora in Gaster Bless" : "Ora in Gaster Bless (nessuna scheda)" }), el("th", { testo: "Dal manuale" }))),
    el("tbody", {},
      brevi.map(([k, t]) => el("tr", { class: val(a, k) !== val(n, k) ? "imp-diverso" : "" }, el("th", { testo: t }), el("td", { testo: val(a, k) }), el("td", { testo: val(n, k) }))),
      [["tratti", "Tratti"], ["azioni", "Azioni"], ["reazioni", "Reazioni"], ["legg", "Leggendarie"]].map(([k, t]) => {
        const diverso = JSON.stringify(a[k] || []) !== JSON.stringify(n[k] || []), stessiNomi = elenco(a, k) === elenco(n, k);
          return el("tr", { class: diverso ? "imp-diverso" : "" }, el("th", { testo: t }), el("td", { testo: elenco(a, k) }),
            el("td", { testo: elenco(n, k) + (diverso && stessiNomi ? " (testo diverso)" : "") })); })));
}

// ===== Incantesimi =====
// Archivio della scheda (web/regole/dati.js, script normale con «const SPELLS»): letto una volta, per collegare ogni
// incantesimo letto a quello con lo stesso nome (italiano o inglese; con l'OCR anche un nome simile).
let archivio = null;
export function archivioIncantesimi() {
  if (!archivio) archivio = fetch("/regole/dati.js").then((r) => r.text()).then((t) => {
    const SPELLS = new Function(t + "\n;return SPELLS;")();
    const per = new Map();
    for (const s of SPELLS) {
      const v = { voce: s[1], l: s[2], sc: s[3], t: s[4], r: s[5], cp: s[6], d: s[7], cl: s[8] };
      for (const n of [s[0], s[1]]) if (!per.has(normalizza(n))) per.set(normalizza(n), v);
    }
    const chiavi = [...per.keys()];
    return {
      trova(nome) {
        const k = normalizza(nome);
        if (per.has(k)) return per.get(k);
        const soglia = k.length > 16 ? 3 : k.length > 8 ? 2 : 1;
        let migliore = null, dMin = soglia + 1, pari = false;
        for (const c of chiavi) { const d = distanza(k, c, soglia); if (d < dMin) { migliore = c; dMin = d; pari = false; } else if (d === dMin && per.get(c) !== per.get(migliore)) pari = true; }
        return migliore && !pari ? per.get(migliore) : undefined;
      },
    };
  }).catch((e) => { archivio = null; throw e; });
  return archivio;
}
// Levenshtein con tetto (come distanzaNomi in internal/server/importazione.go)
function distanza(a, b, max) {
  if (Math.abs(a.length - b.length) > max) return max + 1;
  let prec = Array.from({ length: b.length + 1 }, (_, j) => j);
  for (let i = 1; i <= a.length; i++) {
    const cor = [i]; let minimo = i;
    for (let j = 1; j <= b.length; j++) { cor[j] = Math.min(prec[j] + 1, cor[j - 1] + 1, prec[j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1)); minimo = Math.min(minimo, cor[j]); }
    if (minimo > max) return max + 1;
    prec = cor;
  }
  return prec[b.length];
}
const SCUOLE_N = { A: "Abiurazione", C: "Evocazione", D: "Divinazione", E: "Ammaliamento", V: "Invocazione", I: "Illusione", N: "Necromanzia", T: "Trasmutazione" };
const CLASSI_N = { a: "Artefice", b: "Bardo", c: "Chierico", d: "Druido", p: "Paladino", r: "Ranger", s: "Stregone", w: "Warlock", m: "Mago" };
const UNITA = { m: "min", h: "ore", r: "round", g: "giorni" };
export function testoTempo(t) {
  if (!t) return "—";
  const rit = t[0] === "*", x = rit ? t.slice(1) : t;
  const M = { a: "1 azione", ab: "1 azione bonus", re: "1 reazione", "a o 8h": "1 azione o 8 ore" };
  const m = /^(\d+)([mh])$/.exec(x);
  return (M[x] || (m ? m[1] + " " + UNITA[m[2]] : x)) + (rit ? " (rituale)" : "");
}
export function testoDurata(d) {
  if (!d) return "—";
  const F = { I: "Istantanea", P: "Finché non viene dissolto", S: "Speciale", "I o 1h": "Istantanea o 1 ora" };
  if (F[d]) return F[d];
  const m = /^(C?)(\d+)([mhrgd])$/.exec(d);
  return m ? (m[1] ? "Concentrazione, fino a " : "") + m[2] + " " + UNITA[m[3] === "d" ? "g" : m[3]] : d;
}
function confrontoIncantesimo(v) {
  const a = v.attuale || {}, n = v.dati || {};
  const righe = [["Livello", (x) => (x.l === undefined ? "—" : x.l === 0 ? "Trucchetto" : x.l + "° livello")], ["Scuola", (x) => SCUOLE_N[x.sc] || "—"],
    ["Tempo di lancio", (x) => testoTempo(x.t)], ["Gittata", (x) => x.r || "—"], ["Componenti", (x) => (x.cp || "—").split("").join(", ")],
    ["Durata", (x) => testoDurata(x.d)], ["Classi", (x) => (x.cl || "").split("").map((c) => CLASSI_N[c]).filter(Boolean).join(", ") || "—"]];
  return el("div", {},
    el("table", { class: "tabella imp-confronto" },
      el("thead", {}, el("tr", {}, el("th", { testo: "" }), el("th", { testo: v.attuale ? "Ora in Gaster Bless" : "Ora in Gaster Bless (non c'è)" }), el("th", { testo: "Dal manuale" }))),
      el("tbody", {}, righe.map(([t, f]) => el("tr", { class: v.attuale && f(a) !== f(n) ? "imp-diverso" : "" }, el("th", { testo: t }), el("td", { testo: v.attuale ? f(a) : "—" }), el("td", { testo: f(n) }))))),
    v.testo ? el("details", { class: "imp-testo" }, el("summary", { testo: "Testo del manuale (lo vedi solo tu)" }),
      el("p", { class: "piccolo", testo: v.testo }), n.mat ? el("p", { class: "piccolo", testo: "Componenti materiali: " + n.mat }) : null,
      n.hi ? el("p", { class: "piccolo" }, el("b", { testo: n.l === 0 ? "Potenziamento dei trucchetti. " : "Ai livelli superiori. " }), n.hi) : null) : null);
}

// ===== Oggetti magici =====
// Il motore degli oggetti della scheda (web/regole/oggetti.js), per riconoscere i bonus nei testi letti.
let motoreOgg = null;
function motoreOggetti() {
  if (!motoreOgg) motoreOgg = fetch("/regole/oggetti.js").then((r) => r.text()).then((t) => new Function(t + "\n;return OggettiMagici;")()).catch((e) => { motoreOgg = null; throw e; });
  return motoreOgg;
}
const TIPI_OGG = { arma: "Arma", armatura: "Armatura", scudo: "Scudo", anello: "Anello", amuleto: "Amuleto", mantello: "Mantello", cintura: "Cintura", guanti: "Guanti",
  stivali: "Stivali", elmo: "Elmo", bacchetta: "Bacchetta", bastone: "Bastone", meraviglioso: "Oggetto meraviglioso", accessorio: "Complemento / accessorio", altro: "Oggetto personalizzato" };
const RARITA_OGG = { comune: "Comune", non_comune: "Non comune", raro: "Raro", molto_raro: "Molto raro", leggendario: "Leggendario", artefatto: "Artefatto" };
function confrontoOggetto(v) {
  const a = v.attuale || {}, n = v.dati || {};
  const PAR = { CA: "CA", TS: "tiri salvezza", ATT_INC: "attacco con incantesimo", CD_INC: "CD degli incantesimi", INIZIATIVA: "iniziativa", VELOCITA: "velocità", PF_MAX: "PF massimi", RESISTENZE: "resistenza" };
  const bonus = (x) => [...(x.bonus ? ["+" + x.bonus + (x.tipo === "arma" ? " per colpire e ai danni" : " alla CA")] : []),
    ...(Array.isArray(x.mod) ? x.mod : []).map((m) => (m.op === "resist" ? "resistenza: " + m.val : m.op === "sost" ? m.par + " = " + m.val : (PAR[m.par] || m.par) + " +" + m.val))].join(", ") || "—";
  const righe = [["Tipologia", (x) => TIPI_OGG[x.tipo] || "—"], ["Sottotipo", (x) => x.sottotipo || "—"], ["Rarità", (x) => RARITA_OGG[x.rar] || "—"],
    ["Sintonizzazione", (x) => (x.sint ? "Richiesta" + (x.sintDa ? " (" + x.sintDa + ")" : "") : "No")], ["Bonus calcolati dalla scheda", bonus]];
  return el("div", {},
    el("table", { class: "tabella imp-confronto" },
      el("thead", {}, el("tr", {}, el("th", { testo: "" }), el("th", { testo: v.attuale ? "Ora nell'archivio degli oggetti" : "Ora nell'archivio (non c'è)" }), el("th", { testo: "Dal manuale" }))),
      el("tbody", {}, righe.map(([t, f]) => el("tr", { class: v.attuale && f(a).replace(/ \(.*\)$/, "") !== f(n).replace(/ \(.*\)$/, "") ? "imp-diverso" : "" },
        el("th", { testo: t }), el("td", { testo: v.attuale ? f(a) : "—" }), el("td", { testo: f(n) }))))),
    el("p", { class: "piccolo tenue", testo: "Confermando, l'oggetto va nell'archivio (menu Campagna → ✨ Oggetti magici) con questi valori; effetti, modificatori e immagine li aggiungi tu con ✎ Modifica, poi lo consegni con «🎁 Consegna…»." }),
    v.testo ? el("details", { class: "imp-testo" }, el("summary", { testo: "Testo del manuale (lo vedi solo tu)" }), el("p", { class: "piccolo", testo: v.testo })) : null);
}

// ===== Regole =====
// Le voci che la scheda conosce (talenti, condizioni, background, razze, privilegi), dagli elenchi di web/regole/.
let regole = null;
export function archivioRegole() {
  if (!regole) regole = Promise.all(["/regole/dati.js", "/regole/talenti.js"].map((u) => fetch(u).then((r) => r.text())))
    .then(([d, t]) => vociNote(new Function(d + "\n" + t + "\n;return { CLASSES, RACES, BGS, FD_GEN, FD_RACE, FD_BG, FEATS };")()))
    .catch((e) => { regole = null; throw e; });
  return regole;
}
export const SOTTOTIPI_REGOLA = { talento: "Talento", condizione: "Condizione", background: "Background", privilegio_background: "Privilegio di background",
  razza: "Razza", tratto_razziale: "Tratto razziale", privilegio: "Privilegio di classe" };
function confrontoRegola(v) {
  return el("div", {},
    el("p", { class: "piccolo", testo: (SOTTOTIPI_REGOLA[v.sottotipo] || "Regola") + (v.stato === "cambiato" ? " · il testo è diverso da quello importato prima" : "") +
      ". Lo vedi solo tu: nella scheda, aprendo la voce, e in Manuali → «📖 Regole dai manuali». I calcoli della scheda non cambiano." }),
    v.testo ? el("details", { class: "imp-testo", open: true }, el("summary", { testo: "Testo del manuale (lo vedi solo tu)" }), el("p", { class: "piccolo", testo: v.testo })) : null);
}
