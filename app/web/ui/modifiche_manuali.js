// «Modifica manuale» della scheda con approvazione del Master (richiesta dell'utente, 2026-11-11).
// Regole (le stesse del server, internal/server/modifiche_manuali.go): con l'approvazione attiva il giocatore salva da
// solo solo i campi di gioco; gli altri li cambia con «✏ Modifica manuale» e, spegnendo il tasto, li manda al Master.
// Qui: le regole per la pagina della scheda, il report del Master (approva/rifiuta voce per voce) e l'esito al giocatore.
import { el, api, avviso, dialogo, tempoReale } from "./ui.js";
import { nomeCampo, mostraValore, stabile } from "./campi_scheda.js";

const IN_GIOCO = new Set(["hpCur", "hpTmp", "ds", "slotU", "insp", "exh"]);
const LIBERI = new Set(["v", "eqAuto", "san", "spp", "coins", "armor", "shield", "resRip"]); // resRip: Resilienza immonda, si cambia dopo ogni riposo

export function campoDiGioco(k) {
  if (k === "f.hdUsati") return true; // Dadi Vita usati: si spendono giocando (riposi)
  const t = k.split(".")[0];
  return IN_GIOCO.has(t) || LIBERI.has(t);
}

// a, b: testo JSON stabile del campo «mag» (campi_scheda.js). Stessi oggetti, cambia solo lo stato.
export function magSoloStato(a, b) {
  let la, lb;
  try { la = a ? JSON.parse(a) : []; lb = b ? JSON.parse(b) : []; } catch { return false; }
  if (!Array.isArray(la) || !Array.isArray(lb) || la.length !== lb.length) return false;
  const dati = (v) => { const c = { ...(v || {}) }; delete c.eq; delete c.sint; delete c.on; delete c.car; return stabile(c); };
  return la.every((v, i) => dati(v) === dati(lb[i]));
}

// Elenco degli oggetti della scheda (2026-11-11): stessi oggetti, cambia solo «equipaggiato».
export function invSoloStato(a, b) {
  let la, lb;
  try { la = a ? JSON.parse(a) : []; lb = b ? JSON.parse(b) : []; } catch { return false; }
  if (!Array.isArray(la) || !Array.isArray(lb) || la.length !== lb.length) return false;
  const dati = (v) => { const c = { ...(v || {}) }; delete c.eq; return stabile(c); };
  return la.every((v, i) => dati(v) === dati(lb[i]));
}

// Divide le differenze tra il nuovo stato e quello salvato (entrambi appiattiti): di gioco o da approvare.
// Valori «vuoti» (elenco o oggetto vuoto, testo vuoto, null): un campo vuoto e un campo assente sono la stessa cosa. La
// scheda crea all'apertura alcuni campi vuoti (per esempio gli elenchi degli incantesimi spl.0 … spl.9): non sono
// modifiche e non devono finire nel report del Master (2026-11-12).
const VUOTI = new Set(["[]", "{}", '""', "null"]);
const vuoto = (v) => v === undefined || VUOTI.has(v);

export function separa(nuovo, salvato) {
  const gioco = { modifiche: {}, rimossi: [] }, manuali = { modifiche: {}, rimossi: [] };
  for (const [k, v] of Object.entries(nuovo)) {
    if (salvato[k] === v || (vuoto(v) && vuoto(salvato[k]))) continue;
    const g = campoDiGioco(k) || (k === "mag" && magSoloStato(salvato[k], v)) || (k === "inv" && invSoloStato(salvato[k], v));
    (g ? gioco : manuali).modifiche[k] = JSON.parse(v);
  }
  for (const k of Object.keys(salvato)) {
    if (k in nuovo || vuoto(salvato[k])) continue;
    (campoDiGioco(k) ? gioco : manuali).rimossi.push(k);
  }
  return { gioco, manuali };
}
export const quante = (b) => Object.keys(b.modifiche).length + b.rimossi.length;

// Valore leggibile: per gli oggetti magici i nomi, per il resto il valore breve di sempre.
function valore(campo, v, rimosso) {
  if (rimosso) return "(tolto)";
  if (campo === "mag") {
    const l = Array.isArray(v) ? v : [];
    return l.length ? l.map((x) => (x && x.o && x.o.nome) || "?").join(", ") : "(nessuno)";
  }
  return mostraValore(v);
}

// Tabella prima/dopo. Con «scelte» (Map campo → "si" | "no" | "") aggiunge i pulsanti Approva/Rifiuta.
export function tabellaVoci(voci, scelte, aggiorna) {
  const riga = (v) => {
    const cella = el("td", { class: "mm-scelta" });
    if (scelte) {
      const btn = (val, testo, cls, titolo) => el("button", { class: "piccolo " + cls + (scelte.get(v.campo) === val ? " attivo" : ""), testo, title: titolo, "aria-pressed": String(scelte.get(v.campo) === val),
        on: { click: () => { scelte.set(v.campo, scelte.get(v.campo) === val ? "" : val); aggiorna(); } } });
      cella.append(btn("si", "✓ Approva", "mm-si", "La modifica entra nella scheda"), btn("no", "✗ Rifiuta", "mm-no", "La scheda resta com'è per questo campo"));
    }
    return el("tr", { class: scelte ? "mm-" + (scelte.get(v.campo) || "attesa") : "" },
      el("td", { testo: nomeCampo(v.campo) }),
      el("td", { class: "tenue", testo: valore(v.campo, v.prima, false) }),
      el("td", {}, el("b", { testo: valore(v.campo, v.dopo, v.rimosso) })),
      scelte ? cella : null);
  };
  return el("table", { class: "tabella mm-tabella" },
    el("thead", {}, el("tr", {}, el("th", { testo: "Campo" }), el("th", { testo: "Valore valido" }), el("th", { testo: "Richiesto" }), scelte ? el("th", { testo: "Decisione" }) : null)),
    el("tbody", {}, (voci || []).map(riga)));
}

// ----- Master: report di una richiesta -----
const aperti = new Map(); // id della richiesta → funzione che chiude la finestra

export function mostraRichiesta(r) {
  if (!r || r.stato !== "in_attesa") return;
  if (aperti.has(r.id)) { aperti.get(r.id)(); aperti.delete(r.id); } // richiesta aggiornata: si riapre con le voci nuove
  const voci = r.voci || [];
  const scelte = new Map(voci.map((v) => [v.campo, ""]));
  const contenitore = el("div");
  const conto = el("p", { class: "tenue piccolo" });
  const disegna = () => {
    contenitore.replaceChildren(tabellaVoci(voci, scelte, disegna));
    const si = [...scelte.values()].filter((x) => x === "si").length, no = [...scelte.values()].filter((x) => x === "no").length;
    conto.textContent = si + " da approvare, " + no + " da rifiutare, " + (voci.length - si - no) + " senza decisione (restano in attesa).";
  };
  disegna();
  dialogo("📝 Modifiche alla scheda di " + (r.nome || "un personaggio"), el("div", {},
    el("p", {}, el("b", { testo: r.giocatore || "Il giocatore" }), " ha usato «Modifica manuale» e chiede " + voci.length + (voci.length === 1 ? " modifica" : " modifiche") +
      ". Finché non decidi vale la scheda di prima."),
    contenitore, conto,
    el("p", { class: "tenue piccolo", testo: "Le voci approvate diventano una nuova versione della scheda (nello Storico c'è sempre quella di prima)." })), {
    largo: true,
    azioni: (chiudi) => {
      aperti.set(r.id, chiudi);
      const fine = () => { aperti.delete(r.id); chiudi(); };
      const invia = async (approvati, rifiutati) => {
        if (!approvati.length && !rifiutati.length) { avviso("Scegli Approva o Rifiuta per almeno una voce.", "errore", 4000); return; }
        try {
          const x = await api("/richieste-schede/" + encodeURIComponent(r.id), { dati: { approvati, rifiutati } });
          avviso("📝 " + x.testo, "", 7000);
          fine();
          if (x.richiesta && x.richiesta.stato === "in_attesa") setTimeout(() => mostraRichiesta(x.richiesta), 200);
        } catch (e) { avviso(e.message, "errore", 6000); if (e.stato === 409) fine(); }
      };
      const lista = (val) => [...scelte].filter(([, s]) => s === val).map(([k]) => k);
      return el("div", { class: "azioni" },
        el("button", { testo: "Più tardi", title: "La ritrovi aprendo la scheda o al prossimo accesso", on: { click: () => { aperti.delete(r.id); chiudi(); } } }),
        el("button", { class: "pericolo", testo: "Rifiuta tutto", on: { click: () => invia([], voci.map((v) => v.campo)) } }),
        el("button", { testo: "Approva tutto", on: { click: () => invia(voci.map((v) => v.campo), []) } }),
        el("button", { class: "primario", testo: "Conferma le scelte", on: { click: () => invia(lista("si"), lista("no")) } }));
    },
  });
}

export async function richiesteAperte() {
  try { (await api("/richieste-schede")).lista.filter((r) => r.stato === "in_attesa").forEach(mostraRichiesta); } catch { /* nessuna */ }
}

let installato = false;
export function installaModificheManuali(io) {
  if (installato) return;
  installato = true;
  const master = () => { const u = io(); return u && u.ruolo === "master"; };
  tempoReale.su("scheda.richiesta", (r) => { if (master()) mostraRichiesta(r); });
  tempoReale.su("benvenuto", () => { if (master()) richiesteAperte(); });
  const chiudi = (d) => { if (d && aperti.has(d.id)) { const c = aperti.get(d.id); aperti.delete(d.id); c(); } };
  tempoReale.su("scheda.richiesta.decisa", (d) => { if (master() && d && d.stato !== "in_attesa") chiudi(d); });
  tempoReale.su("scheda.richiesta.ritirata", (d) => { if (master()) { chiudi(d); if (d && d.id) avviso("📝 Una richiesta di modifiche è stata ritirata dal giocatore.", "", 5000); } });
  tempoReale.su("scheda.richiesta.esito", (d) => { if (d && d.testo) avviso("📝 " + d.testo, "", 9000); });
}
