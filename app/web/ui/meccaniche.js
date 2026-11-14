// Tiri delle famiglie di meccaniche (fase 3c, passo C4c, docs/MECCANICHE.md): per ogni meccanica un piccolo modulo con
// i soli campi che servono; il tiro lo fa il server («dadi.meccanica») e arriva come gli altri tiri (dadi 3D, esito in
// parole). Lo usano la sezione Dadi e i tiri della scheda libera.
import { el, avviso, tempoReale } from "./ui.js";

const CAMPI = {
  "d100": { valore: "Valore dell'abilità", extra: true },
  "riserva-d10": { dadi: "Dadi nella riserva", soglia: 6, difficolta: ["Successi richiesti", 1], opz: [["coppieDi10", "Due 10 valgono due successi in più (critico)"]] },
  "riserva-d6": { dadi: "Dadi nella riserva", soglia: 6, difficolta: ["Successi richiesti", 1], modo: true, opz: [["unoDisastro", "Nessun successo con almeno un 1: disastro"]] },
  "2d6-fasce": {},
  "d10-somma": { valore: "Caratteristica + abilità", difficolta: ["Difficoltà", 15] },
  "passi-esplosivi": { passo: true, difficolta: ["Numero da raggiungere", 4], opz: [["jolly", "Con il dado jolly (d6 esplosivo, vale il migliore)"]] },
  "fate": { valore: "Abilità", difficolta: ["Opposizione", 0] },
  "3d6-sotto": { valore: "Valore (abilità o caratteristica)" },
  "d20-sotto": { valore: "Valore (abilità o caratteristica)", vantaggio: true },
  "narrativi": { narrativi: true },
};
const NARRATIVI = [["abilita", "Abilità (d8)"], ["competenza", "Competenza (d12)"], ["aiuto", "Aiuto (d6)"], ["difficolta", "Difficoltà (d8)"], ["sfida", "Sfida (d12)"], ["ostacolo", "Ostacolo (d6)"]];

export const haMeccanica = (m) => !!CAMPI[m];

/** Quale campo riceve il valore principale (dalla scheda): numero di dadi, dado del tratto o valore. */
export function campoPrincipale(m) {
  const c = CAMPI[m] || {};
  return c.dadi ? "dadi" : c.passo ? "passo" : c.narrativi ? "abilita" : c.valore ? "valore" : "";
}

/**
 * Modulo del tiro. meccanica: id; opz: { valore, sessione, etichetta, personaggio, scheda, segreto (Master),
 * dopo(r) dopo il tiro }.
 */
export function pannelloMeccanica(meccanica, opz = {}) {
  const c = CAMPI[meccanica];
  if (!c) return el("p", { class: "tenue", testo: "Questo gioco non ha un tiro dedicato." });
  const campi = [];
  const num = (k, etichetta, val, min, max) => {
    const i = el("input", { type: "number", name: k, min, max, step: 1, valore: val ?? "", class: "num-piccolo", "aria-label": etichetta });
    campi.push(el("label", { class: "mc-campo" }, el("span", { testo: etichetta }), i));
    return i;
  };
  const princ = campoPrincipale(meccanica);
  const val0 = opz.valore != null ? opz.valore : "";
  const ctl = {};
  if (c.valore) ctl.valore = num("valore", c.valore, princ === "valore" ? val0 : "", 0, 200);
  if (c.dadi) ctl.dadi = num("dadi", c.dadi, princ === "dadi" ? val0 : 3, 0, 40);
  if (c.passo) {
    const s = el("select", { name: "dadi", "aria-label": "Dado del tratto" }, [4, 6, 8, 10, 12].map((f) => el("option", { value: f, testo: "d" + f })));
    s.value = String([4, 6, 8, 10, 12].includes(+val0) ? val0 : 6);
    campi.push(el("label", { class: "mc-campo" }, el("span", { testo: "Dado del tratto" }), s));
    ctl.dadi = s;
  }
  if (c.narrativi) {
    ctl.narr = {};
    for (const [k, n] of NARRATIVI) ctl.narr[k] = num("n-" + k, n, k === "abilita" ? (val0 || 2) : k === "difficolta" ? 1 : 0, 0, 10);
  }
  ctl.mod = meccanica === "narrativi" ? null : num("mod", "Modificatore", 0, -50, 50);
  if (c.soglia) ctl.soglia = num("soglia", "Successo da", c.soglia, 2, meccanica === "riserva-d6" ? 6 : 10);
  if (c.difficolta) ctl.difficolta = num("difficolta", c.difficolta[0], c.difficolta[1], -10, 100);
  if (c.extra) ctl.extra = num("extra", "Dadi bonus (+) o penalità (−)", 0, -2, 2);
  if (c.modo) {
    const s = el("select", { name: "modo", "aria-label": "Come si legge" }, el("option", { value: "successi", testo: "Si contano i successi" }), el("option", { value: "massimo", testo: "Vale il dado più alto (6 pieno, 4-5 parziale)" }));
    campi.push(el("label", { class: "mc-campo mc-largo" }, el("span", { testo: "Come si legge" }), s));
    ctl.modo = s;
    const vedi = () => { [ctl.soglia, ctl.difficolta].forEach((x) => { if (x) x.closest("label").hidden = s.value === "massimo"; }); };
    s.addEventListener("change", vedi); setTimeout(vedi, 0);
  }
  if (c.vantaggio) {
    const s = el("select", { name: "modo", "aria-label": "Vantaggio" }, el("option", { value: "normale", testo: "Un dado" }), el("option", { value: "vantaggio", testo: "Vantaggio (2 dadi, il più basso)" }), el("option", { value: "svantaggio", testo: "Svantaggio (2 dadi, il più alto)" }));
    campi.push(el("label", { class: "mc-campo mc-largo" }, el("span", { testo: "Dadi" }), s));
    ctl.modo = s;
  }
  ctl.opz = (c.opz || []).map(([k, n]) => { const x = el("input", { type: "checkbox", name: k }); campi.push(el("label", { class: "riga mc-largo" }, x, el("span", { testo: n }))); return [k, x]; });
  const etichetta = el("input", { type: "text", maxlength: 40, valore: opz.etichetta || "", placeholder: "Che cosa tiri (facoltativo)", "aria-label": "Che cosa tiri" });
  const segreto = opz.master ? el("input", { type: "checkbox", name: "segreto" }) : null;
  const tasto = el("button", { type: "button", class: "primario", testo: "🎲 Tira", on: { click: async () => {
    const n = (x) => (x && x.value !== "" ? parseInt(x.value, 10) : undefined);
    const dati = { meccanica, sessione: opz.sessione || "", etichetta: etichetta.value.trim(), personaggio: opz.personaggio || "", scheda: !!opz.scheda,
      segreto: !!(segreto && segreto.checked), valore: n(ctl.valore) || 0, dadi: n(ctl.dadi) || 0, mod: n(ctl.mod) || 0,
      soglia: n(ctl.soglia) || 0, extra: n(ctl.extra) || 0, modo: ctl.modo ? ctl.modo.value : "",
      opzioni: Object.fromEntries(ctl.opz.map(([k, x]) => [k, x.checked])) };
    if (ctl.difficolta && ctl.difficolta.value !== "") dati.difficolta = n(ctl.difficolta);
    if (ctl.narr) dati.narrativi = Object.fromEntries(Object.entries(ctl.narr).map(([k, x]) => [k, n(x) || 0]));
    tasto.disabled = true;
    try { const r = await tempoReale.chiedi("dadi.meccanica", dati); if (opz.dopo) opz.dopo(r); }
    catch (e) { avviso(e.message, "errore"); }
    tasto.disabled = false;
  } } });
  return el("div", { class: "mc-pannello", "data-meccanica": meccanica },
    el("div", { class: "mc-campi" }, campi), el("div", { class: "riga-flex mc-azioni" }, etichetta,
      segreto ? el("label", { class: "riga" }, segreto, el("span", { testo: "Segreto" })) : null, tasto));
}
