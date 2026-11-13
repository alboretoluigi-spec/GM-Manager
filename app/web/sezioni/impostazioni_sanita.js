// Impostazioni della campagna per Sanità mentale, Terrore e pazzia (solo Master): regole ufficiali DMG 2014
// e regole homebrew separate e attivabili, immagine della schermata, catalogo dei testi delle pazzie.
import { el, api, carica, avviso } from "../ui/ui.js";

const U = (t) => el("span", { class: "etichetta approvato", testo: t });

export function sezioneSanita() {
  const box = el("section", { class: "scheda-ui" }, el("h2", { testo: "Sanità mentale e Terrore" }), el("p", { class: "tenue", testo: "Caricamento…" }));
  disegna(box).catch((e) => box.append(el("p", { class: "errore", testo: e.message })));
  return box;
}

async function disegna(box) {
  const c = await api("/sanita/config");
  const spunta = (k, testo, tipo) => el("label", { class: "riga" }, el("input", { type: "checkbox", selezionato: c[k], on: { change: (e) => { c[k] = e.target.checked; } } }),
    el("span", {}, testo, " ", tipo === "u" ? U("ufficiale DMG 2014") : el("span", { class: "etichetta attesa", testo: "homebrew" })));
  const tabelle = el("select", { on: { change: (e) => { c.tabelle = e.target.value; } } },
    el("option", { value: "homebrew", testo: "Categorie homebrew (d20 categoria, d100 voce)", selected: c.tabelle === "homebrew" }),
    el("option", { value: "dmg", testo: "Tabelle ufficiali DMG (testi da inserire)", selected: c.tabelle === "dmg" }));
  const num = (v, f, min, max) => el("input", { type: "number", min: String(min), max: String(max), valore: v, class: "corto", on: { change: (e) => f(+e.target.value) } });
  const intens = el("table", { class: "tabella-semplice" }, el("tr", {}, el("th", { testo: "Intensità" }), el("th", { testo: "CD" }), el("th", { testo: "Terrore se fallisce" })),
    [0, 1, 2, 3, 4, 5].map((i) => el("tr", {}, el("td", { testo: String(i) }), el("td", {}, num(c.cdIntensita[i], (x) => { c.cdIntensita[i] = x; }, 1, 40)),
      el("td", {}, num(c.terroreIntensita[i], (x) => { c.terroreIntensita[i] = x; }, 0, 7)))));
  const salva = el("button", { class: "primario", testo: "Salva le impostazioni", on: { click: async () => {
    try { await api("/sanita/config", { metodo: "PUT", dati: c }); avviso("Impostazioni salvate: la Sanità mentale si aggiorna da sola."); }
    catch (e) { avviso(e.message, "errore"); }
  } } });
  box.replaceChildren(el("h2", { testo: "Sanità mentale e Terrore" }),
    el("p", { class: "tenue", testo: "Le regole ufficiali vengono dalla Guida del Dungeon Master 2014 (capitoli 8 e 9); quelle homebrew sono quelle della tua specifica e si attivano a parte." }),
    spunta("attiva", "Sanità mentale con il suo tiro salvezza (funzione «🧠 Sanità mentale» dell'app, separata dalla scheda)", "u"),
    el("p", { class: "tenue", testo: "Valore della Sanità mentale: 10 + modificatore di Saggezza (la Saggezza arriva da qualsiasi scheda, anche alternativa). Per un singolo personaggio puoi scriverlo a mano negli strumenti del Master." }),
    spunta("competenzaPredefinita", "Competenza nel TS su Sanità mentale per i personaggi senza impostazioni (nelle regole base nessuna classe la dà)", "u"),
    spunta("fallbackSaggezza", "Creature senza Sanità mentale: usare la Saggezza", "u"),
    spunta("terrore", "Scala del Terrore 0–7 con effetti, soglie e condizioni", "h"),
    spunta("pazzia", "Gestione delle pazzie (temporanea, duratura, indeterminata)", "u"),
    el("label", {}, "Tabelle di pazzia", tabelle),
    el("details", {}, el("summary", { testo: "Regole opzionali e valori (homebrew)" }),
      spunta("regolaResistenza", "Immuni a Spaventato: vantaggio al TS e 1 Terrore in meno", "h"),
      spunta("regolaSpaventato", "Spaventato da un effetto soprannaturale: TS su Sanità mentale, se fallito +3 Terrore", "h"),
      spunta("regolaRimozione", "Togliere la condizione Spaventato toglie 3 Terrore", "h"),
      spunta("aumentaCdPrecedente", "Nuova pazzia: +1 alla CD di recupero di quelle già attive", "h"),
      spunta("pazziaCronica", "Pazzia cronica (mai assegnata in automatico)", "h"),
      intens,
      el("div", { class: "riga-flex" }, el("label", {}, "CD del TS di Saggezza a Terrore 4", num(c.cdBarcollante, (x) => { c.cdBarcollante = x; }, 1, 40)),
        el("label", {}, "CD del TS di Costituzione a Terrore 7", num(c.cdSvenuto, (x) => { c.cdSvenuto = x; }, 1, 40)))),
    salva, immagine(), catalogo());
}

// Immagine della schermata Sanità mentale (mostrata a giocatore e Master).
function immagine() {
  const anteprima = el("img", { class: "sanita-anteprima", alt: "Immagine attuale della schermata Sanità mentale", hidden: true });
  const prova = () => { anteprima.hidden = false; anteprima.src = "/api/v1/sanita/immagine?t=" + Date.now(); anteprima.onerror = () => { anteprima.hidden = true; stato.textContent = "Nessuna immagine caricata."; }; anteprima.onload = () => { stato.textContent = "Immagine caricata."; }; };
  const stato = el("p", { class: "tenue" });
  const file = el("input", { type: "file", accept: "image/jpeg,image/png,image/webp,image/gif", on: { change: async () => {
    const f = file.files[0]; if (!f) return;
    try { await carica("/sanita/immagine", f, null, "PUT"); avviso("Immagine caricata."); prova(); } catch (e) { avviso(e.message, "errore"); }
    file.value = "";
  } } });
  const det = el("details", {}, el("summary", { testo: "Immagine della schermata" }),
    el("p", { class: "tenue", testo: "Carica qui l'illustrazione maschile realizzata per la schermata Sanità mentale (JPG, PNG o WebP, fino a 20 MB). Finché non c'è, la schermata mostra un avviso al suo posto." }),
    el("label", {}, "Scegli l'immagine", file), stato, anteprima,
    el("button", { class: "piccolo pericolo", testo: "Togli l'immagine", on: { click: async () => { await api("/sanita/immagine", { metodo: "DELETE" }); prova(); } } }));
  det.addEventListener("toggle", () => { if (det.open) prova(); }, { once: true });
  return det;
}

// Catalogo dei testi delle pazzie: 10 voci (d100 a gruppi di 10) per categoria o tabella.
function catalogo() {
  const det = el("details", {}, el("summary", { testo: "Catalogo dei testi delle pazzie" }));
  det.addEventListener("toggle", async () => {
    if (!det.open || det.dataset.pronto) return; det.dataset.pronto = "1";
    const cat = await api("/sanita/catalogo");
    const scelte = [];
    for (const [t, cats] of Object.entries(cat)) for (const c of Object.keys(cats)) scelte.push([t, c]);
    const sel = el("select", {}, scelte.map(([t, c], i) => el("option", { value: i, testo: (t === "homebrew" ? "Homebrew · " : "DMG · ") + c })));
    const voci = Array.from({ length: 10 }, (_, i) => el("textarea", { rows: "2", maxlength: "1000", "aria-label": "Voce " + (i * 10 + 1) + "–" + (i === 9 ? "00" : i * 10 + 10) }));
    const carica2 = () => { const [t, c] = scelte[+sel.value]; voci.forEach((x, i) => { x.value = cat[t][c][i] || ""; }); };
    sel.addEventListener("change", carica2); carica2();
    det.append(el("p", { class: "tenue", testo: "Scrivi qui gli effetti (con parole tue o testi che hai il diritto di usare). L'app non contiene testi delle tabelle: se una voce estratta è vuota, la pazzia viene creata e ti viene chiesto di descriverla." }),
      el("label", {}, "Tabella", sel),
      el("div", { class: "catalogo-voci" }, voci.map((x, i) => el("label", {}, "d100 " + String(i * 10 + 1).padStart(2, "0") + "–" + (i === 9 ? "00" : String(i * 10 + 10)), x))),
      el("button", { class: "primario", testo: "Salva questa tabella", on: { click: async () => {
        const [t, c] = scelte[+sel.value];
        try { await api("/sanita/catalogo", { metodo: "PUT", dati: { tabella: t, categoria: c, voci: voci.map((x) => x.value) } }); cat[t][c] = voci.map((x) => x.value); avviso("Tabella salvata."); }
        catch (e) { avviso(e.message, "errore"); }
      } } }));
  });
  return det;
}
