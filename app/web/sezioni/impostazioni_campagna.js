// Impostazioni → «📦 File della campagna» (fase M4, docs/PIANO_MEDIA_MANUALI.md, 2026-11-11). Esporta in un file JSON
// leggibile gruppi e sessioni, personaggi con le schede, bestiario, oggetti magici, località e riassunti; lo reimporta
// (anche su un altro PC) aggiungendo senza cancellare, dopo un'anteprima. Server: internal/server/campagna.go.
import { el, svuota, api, avviso } from "../ui/ui.js";

const SEZIONI = [["gruppi", "Gruppi e sessioni"], ["personaggi", "Personaggi e schede"], ["bestiario", "Bestiario (PNG e mostri tuoi, schede e note)"],
  ["oggetti", "Oggetti magici (archivio)"], ["localita", "Testi delle località"], ["riassunti", "Riassunti delle sessioni"]];

export function sezioneCampagna() {
  const spunte = SEZIONI.map(([k, t]) => ({ k, c: el("input", { type: "checkbox", selezionato: true }), t }));
  const esito = el("div", { class: "campagna-esito" });
  const scelte = () => spunte.filter((x) => x.c.checked).map((x) => x.k);

  async function esporta(b) {
    if (!scelte().length) { avviso("Scegli almeno una parte da esportare.", "errore"); return; }
    b.disabled = true;
    try {
      const f = await api("/campagna/esporta?sezioni=" + scelte().join(","));
      const nome = "campagna-gasterbless-" + new Date().toISOString().slice(0, 10) + ".json";
      const a = el("a", { href: URL.createObjectURL(new Blob([JSON.stringify(f, null, 1)], { type: "application/json" })), download: nome });
      document.body.append(a); a.click(); a.remove(); setTimeout(() => URL.revokeObjectURL(a.href), 5000);
      const n = (k) => (f[k] || []).length;
      svuota(esito, el("p", { class: "ok", testo: "✔ File «" + nome + "» pronto: " + n("personaggi") + " personaggi, " + n("gruppi") + " gruppi, " + n("sessioni") + " sessioni, " +
        (n("bestiarioVoci") + n("bestiarioSchede")) + " voci del bestiario, " + n("oggetti") + " oggetti magici, " + n("localita") + " località, " + n("riassunti") + " riassunti." }));
    } catch (e) { avviso(e.message, "errore"); }
    b.disabled = false;
  }

  function tabella(r) {
    return el("table", { class: "tabella campagna-anteprima" },
      el("thead", {}, el("tr", {}, ["", "Nuove", "Uguali", "Diverse", "Aggiornate", "Non valide"].map((t) => el("th", { testo: t })))),
      el("tbody", {}, SEZIONI.filter(([k]) => r.sezioni[k] && scelteImp.includes(k)).map(([k, t]) => {
        const c = r.sezioni[k];
        return el("tr", {}, el("th", { testo: t + (c.senzaGruppo ? " (" + c.senzaGruppo + " senza gruppo)" : "") }),
          [c.nuovi, c.uguali, c.diversi, c.aggiornati, c.nonValidi].map((n) => el("td", { testo: String(n || 0) })));
      })));
  }
  let file = null, scelteImp = [];
  async function anteprima(testo, nomeFile) {
    try { file = JSON.parse(testo); } catch { avviso("Il file non è leggibile (JSON non valido).", "errore"); return; }
    if (!file || file.formato !== "gasterbless-campagna") { avviso("Questo non è un file della campagna di Gaster Bless.", "errore"); return; }
    scelteImp = scelte();
    if (!scelteImp.length) { avviso("Scegli almeno una parte da importare.", "errore"); return; }
    svuota(esito, el("p", { class: "tenue", testo: "Controllo del file…" }));
    let r;
    try { r = await api("/campagna/importa", { metodo: "POST", dati: { file, sezioni: scelteImp, prova: true } }); }
    catch (e) { svuota(esito, el("p", { class: "errore", testo: e.message })); return; }
    const aggiorna = el("input", { type: "checkbox" });
    const diverse = SEZIONI.reduce((n, [k]) => n + ((r.sezioni[k] || {}).diversi || 0), 0);
    const vai = el("button", { class: "primario", testo: "⬇ Importa", on: { click: async () => {
      vai.disabled = true;
      try {
        const fatto = await api("/campagna/importa", { metodo: "POST", dati: { file, sezioni: scelteImp, aggiorna: aggiorna.checked } });
        svuota(esito, el("p", { class: "ok", testo: "✔ Importazione fatta. Non è stato cancellato nulla." }), tabella(fatto));
        file = null;
      } catch (e) { avviso(e.message, "errore"); vai.disabled = false; }
    } } });
    svuota(esito,
      el("p", { testo: "Anteprima di «" + nomeFile + "»" + (file.creato ? " (creato il " + new Date(file.creato * 1000).toLocaleString("it-IT") + ")" : "") + ": non è ancora cambiato nulla." }),
      tabella(r),
      diverse ? el("label", { class: "riga" }, aggiorna, el("span", { testo: " Aggiorna anche le " + diverse + " voci diverse con quelle del file (le schede ricevono una versione nuova: le precedenti restano nella cronologia)" })) : null,
      el("p", { class: "tenue piccolo", testo: "I personaggi nuovi vanno al giocatore con lo stesso nome, se c'è ed è nel gruppo; altrimenti restano da assegnare (Schede → Assegna)." }),
      el("div", { class: "riga-flex" }, el("button", { testo: "Annulla", on: { click: () => { file = null; svuota(esito); } } }), vai));
  }
  function scegliFile() {
    const inp = el("input", { type: "file", accept: ".json,application/json", hidden: true });
    inp.addEventListener("change", () => {
      const f = inp.files[0]; inp.remove();
      if (!f) return;
      if (f.size > 90 * 1048576) { avviso("File troppo grande.", "errore"); return; }
      const rd = new FileReader();
      rd.onload = () => anteprima(String(rd.result || ""), f.name);
      rd.onerror = () => avviso("Non riesco a leggere il file.", "errore");
      rd.readAsText(f);
    });
    document.body.append(inp); inp.click();
  }

  return el("section", { class: "scheda-ui", id: "file-campagna" }, el("h2", { testo: "📦 File della campagna" }),
    el("p", { class: "tenue", testo: "Un unico file leggibile con la tua campagna, da conservare o da portare su un altro PC con Gaster Bless. Non contiene password, telefoni, dati di accesso, testo dei manuali né documenti. Non sostituisce il backup (che è completo e cifrato)." }),
    el("div", { class: "campagna-sezioni" }, spunte.map((x) => el("label", { class: "riga" }, x.c, el("span", { testo: " " + x.t })))),
    el("div", { class: "riga-flex" },
      el("button", { class: "primario", testo: "⬆ Esporta la campagna", on: { click: (e) => esporta(e.currentTarget) } }),
      el("button", { testo: "⬇ Importa la campagna…", on: { click: scegliFile } })),
    el("p", { class: "tenue piccolo", testo: "Importando si aggiunge senza cancellare nulla: prima vedi quante voci sono nuove, uguali o diverse; le diverse si aggiornano solo se lo scegli. Valgono le parti spuntate qui sopra." }),
    esito);
}
