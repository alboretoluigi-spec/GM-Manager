// Modello della scheda libera di una campagna (fase 3c, passo C4b, docs/SCHEDA_LIBERA.md): il Master decide sezioni,
// campi, tiri, punti ferita, difesa e iniziativa, con lo stesso modulo dell'inserimento a mano dei contenuti. Il server
// controlla il modello (formule comprese); si può esportare, importare da file e tornare al modello di base.
import { el, api, avviso, dialogo, conferma } from "../ui/ui.js";
import { modulo } from "./contenuti_editor.js";

const TIPI = [["testo", "Testo breve"], ["area", "Testo lungo"], ["numero", "Numero"], ["risorsa", "Risorsa (attuale / massimo)"], ["spunta", "Sì / no"], ["elenco", "Elenco (una voce per riga)"]];

const CAMPI = [
  { k: "sezioni", t: "sotto", n: "Sezioni della scheda", voce: "una sezione", campi: [
    { k: "titolo", t: "testo", n: "Titolo della sezione", obbl: true },
    { k: "campi", t: "sotto", n: "Campi", voce: "un campo", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "id", t: "id", n: "Identificativo (per le formule: {identificativo})", sep: "_" },
      { k: "tipo", t: "scelta", n: "Tipo", opzioni: TIPI },
      { k: "min", t: "numero", n: "Minimo", min: -9999, max: 9999 }, { k: "max", t: "numero", n: "Massimo", min: -9999, max: 9999 },
      { k: "predefinito", t: "testo", n: "Valore iniziale" }, { k: "aiuto", t: "testo", n: "Spiegazione breve per i giocatori" },
      { k: "tiro", t: "testo", n: "Tiro con un clic (es. 1d20 + {forza})" }] }] },
  { k: "tiri", t: "tabella", n: "Altri tiri (pulsanti in cima alla scheda)", voce: "un tiro", colonne: [["nome", "Nome"], ["formula", "Formula (es. 2d6 + {abilita})"]] },
  { k: "pf", t: "testo", n: "Punti ferita: identificativo del campo (risorsa o numero)" },
  { k: "difesa", t: "testo", n: "Difesa per il tracker: identificativo del campo numerico" },
  { k: "iniziativa", t: "testo", n: "Formula dell'iniziativa (vuota: la decide il Master)" },
];

/** Converte i valori iniziali scritti come testo nel tipo del campo. */
function sistema(m) {
  for (const s of m.sezioni || []) for (const c of s.campi || []) {
    if (c.predefinito === undefined || c.predefinito === "") { delete c.predefinito; continue; }
    if (c.tipo === "numero" || c.tipo === "risorsa") c.predefinito = Number(String(c.predefinito).replace(",", ".")) || 0;
    else if (c.tipo === "spunta") c.predefinito = /^(si|sì|true|1|x)$/i.test(String(c.predefinito));
    else c.predefinito = String(c.predefinito);
  }
  return m;
}

export async function apriModello(c) {
  let r;
  try { r = await api("/campagne/" + encodeURIComponent(c.id) + "/modello-scheda"); } catch (e) { avviso(e.message, "errore"); return; }
  const contenitore = el("div", {});
  const errore = el("pre", { class: "ce-errore ct-pre", role: "alert", hidden: true });
  let m = modulo(CAMPI, r.modello);
  contenitore.append(m.nodo);
  const file = el("input", { type: "file", accept: ".json,application/json", hidden: true, on: { change: async () => {
    const f = file.files[0]; file.value = "";
    if (!f) return;
    try { const x = JSON.parse(await f.text()); m = modulo(CAMPI, x); contenitore.replaceChildren(m.nodo); avviso("Modello caricato dal file: controllalo e salva."); }
    catch { avviso("Il file non è un modello di scheda valido (JSON).", "errore"); }
  } } });
  dialogo("📜 Modello della scheda · " + c.nome, el("div", { class: "ce-editor" },
    el("p", { class: "tenue piccolo", testo: (r.personalizzato ? "Modello personalizzato di questa campagna." : "Modello di base dell'app: adattalo al tuo gioco e salva.") +
      " Le formule usano i dadi d4, d6, d8, d10, d12, d20, d100 e i campi numerici tra graffe, per esempio 2d6 + {forza} - 1." }),
    contenitore, errore, file), { largo: true, azioni: (chiudi) => el("div", { class: "azioni" },
    el("button", { type: "button", testo: "⬇ Esporta", on: { click: () => {
      const b = new Blob([JSON.stringify(sistema(m.leggi()), null, 2)], { type: "application/json" });
      const a = el("a", { href: URL.createObjectURL(b), download: "modello_scheda.json" }); document.body.append(a); a.click(); a.remove();
    } } }),
    el("button", { type: "button", testo: "📥 Importa da file", on: { click: () => file.click() } }),
    el("button", { type: "button", class: "pericolo", testo: "↺ Modello di base", on: { click: async () => {
      if (!(await conferma("Tornare al modello di base?", "Il modello personalizzato di «" + c.nome + "» si cancella. I dati delle schede restano.", "Torna al modello di base", true))) return;
      try { await api("/campagne/" + encodeURIComponent(c.id) + "/modello-scheda", { metodo: "PUT", corpo: "null", tipo: "application/json" }); chiudi(); avviso("Modello di base ripristinato."); }
      catch (e) { avviso(e.message, "errore"); }
    } } }),
    el("button", { type: "button", testo: "Annulla", on: { click: chiudi } }),
    el("button", { type: "button", class: "primario", testo: "💾 Salva il modello", on: { click: async (ev) => {
      const b = ev.currentTarget; b.disabled = true; errore.hidden = true;
      try {
        await api("/campagne/" + encodeURIComponent(c.id) + "/modello-scheda", { metodo: "PUT", corpo: JSON.stringify(sistema(m.leggi())), tipo: "application/json" });
        chiudi(); avviso("Modello della scheda salvato: le schede si aggiornano alla prossima apertura.");
      } catch (e) { errore.textContent = e.message; errore.hidden = false; errore.scrollIntoView({ block: "nearest" }); }
      b.disabled = false;
    } } })) });
}
