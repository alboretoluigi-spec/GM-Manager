// Effetti che cambiano i valori (internal/server/effetti_valori.go, Parte 2 del piano di combattimento, 2026-11-12).
// - testoModifiche(mod): le modifiche in breve («CA +2, +1d4 ai tiri per colpire»);
// - tastoIra(sessione, vista, c): «😡 Ira» / «😡 Fine ira» per un barbaro (riepilogo della scheda), per il giocatore e il Master;
// - apriEffettoManuale(sessione, vista, c): il Master mette a mano un effetto su uno o più combattenti, scegliendo un
//   potenziamento degli incantesimi (Scudo della fede, Benedizione…) o scrivendo le modifiche;
// - chiedi(sessione, tipo, dati): manda il comando con la conferma del Master quando si va oltre le azioni del turno.
import { el, avviso, dialogo, conferma, tempoReale } from "./ui.js";

const CAR = { FOR: "Forza", DES: "Destrezza", COS: "Costituzione", INT: "Intelligenza", SAG: "Saggezza", CAR: "Carisma" };
export const TIPI = ["acido", "contundente", "freddo", "forza", "fulmine", "fuoco", "necrotico", "perforante", "psichico", "radioso", "tagliente", "tuono", "veleno"];
const sg = (n) => (n >= 0 ? "+" : "−") + Math.abs(n);
const dado = (d) => (String(d).startsWith("-") ? "−" + String(d).slice(1) : "+" + d);
const nuovoId = () => "e" + Date.now().toString(36) + Math.random().toString(36).slice(2, 10);

export function testoModifiche(m) {
  if (!m) return "";
  const p = [];
  if (m.caBase) p.push("CA " + m.caBase + " + Destrezza");
  if (m.caBonus) p.push("CA " + sg(m.caBonus));
  if (m.caMin) p.push("CA almeno " + m.caMin);
  const res = m.res || [];
  if (res.length === TIPI.length) p.push("resistenza a tutti i danni");
  else if (res.length === TIPI.length - 1) p.push("resistenza a tutti i danni tranne " + TIPI.find((t) => !res.includes(t)));
  else if (res.length) p.push("resistenza: " + res.join(", "));
  if (m.imm && m.imm.length) p.push("immunità: " + m.imm.join(", "));
  if (m.immCond && m.immCond.length) p.push("immune a: " + m.immCond.join(", ").toLowerCase());
  const v = m.vantTS || [];
  if (v.length === 6) p.push("vantaggio a tutti i TS");
  else v.forEach((c) => p.push("vantaggio ai TS su " + (CAR[c] || c)));
  if (m.bonusTS) p.push("TS " + sg(m.bonusTS));
  if (m.dadoTS) p.push(dado(m.dadoTS) + " ai TS");
  if (m.dadoAtt) p.push(dado(m.dadoAtt) + " ai tiri per colpire");
  if (m.svantContro) p.push("chi lo attacca ha svantaggio");
  if (m.danniMischia) p.push("+" + m.danniMischia + " ai danni in mischia");
  if (m.pfTempTurno) p.push((m.pfTempTurno === "inc" ? "modificatore da incantatore" : m.pfTempTurno) + " PF temporanei a inizio turno");
  if (m.ritorsione && m.ritorsione.tipo) p.push(m.ritorsione.dadi + " " + m.ritorsione.tipo + " a chi lo colpisce in mischia");
  return p.join(", ");
}

/** Manda un comando dello scontro; se il server chiede conferma (azione già usata…), la chiede e riprova con «forza». */
export async function chiedi(sessione, tipo, dati) {
  let r = await tempoReale.chiedi(tipo, { sessione, azione: nuovoId(), ...dati });
  if (r && r.serveConferma) {
    if (!(await conferma("Procedere lo stesso?", r.serveConferma, "Sì"))) return null;
    r = await tempoReale.chiedi(tipo, { sessione, azione: nuovoId(), forza: true, ...dati });
  }
  return r;
}

export function tastoIra(sessione, vista, c) {
  const k = vista && vista.combattimento;
  if (!c.ira || !k || !(c.mio || vista.ruolo === "master" || vista.ruolo === "segretario")) return null;
  const attiva = (vista.effetti || []).some((e) => e.nome === "Ira" && e.incantatore === c.id);
  return el("button", { class: "piccolo iniz-ira" + (attiva ? " attiva" : ""), testo: attiva ? "😡 Fine ira" : "😡 Ira",
    title: attiva ? "Esce dall'ira" : "Entra in ira (azione bonus, 1 minuto): resistenze, danni in più in mischia, vantaggio ai TS su Forza. Interrompe la concentrazione.",
    on: { click: async (ev) => {
      const b = ev.currentTarget;
      b.disabled = true;
      try { const r = await chiedi(sessione, "pg.ira", { combattente: c.id }); if (r && r.testo) avviso("😡 " + r.testo, "", 5000); }
      catch (e) { avviso(e.message, "errore"); }
      b.disabled = false;
    } } });
}

let potenziamenti = null; // [{nome, mod, scegli, cond, nota, conc, round, tempo}] dagli incantesimi (lettore comune)
async function caricaPotenziamenti() {
  if (potenziamenti) return potenziamenti;
  const [d, e] = await Promise.all([fetch("/regole/dati.js").then((r) => r.text()), fetch("/regole/incantesimi_effetti.js").then((r) => r.text())]);
  const x = new Function(d + "\n" + e + "\n;return { SPELLS, IncantesimiEffetti };")();
  const IE = x.IncantesimiEffetti;
  potenziamenti = x.SPELLS.map((s) => ({ it: s[0], en: s[1], l: s[2], t: s[4], r: s[5], d: s[7], ds: s[10], hi: s[11] || "" }))
    .map((s) => ({ s, r: IE.analizza(s) })).filter((x) => x.r && x.r.valori)
    .map(({ s, r }) => { const t = String(s.t || "").replace(/^\*/, ""); return { nome: s.it, ...IE.effettoValori(r), conc: !!r.conc, round: r.durata && !r.durata.istantanea ? r.durata.round || 0 : 0,
      tempo: t === "re" ? "reazione" : t === "ab" ? "bonus" : /^a\b/.test(t) ? "azione" : "altro" }; })
    .sort((a, b) => a.nome.localeCompare(b.nome, "it"));
  return potenziamenti;
}

/** Finestra comune: descrizione, scelta del tipo (se c'è), bersagli; invio di «effetto.nuovo». */
export function finestraEffetto(sessione, vista, chi, eff, { personalizzato = null } = {}) {
  const err = el("p", { class: "errore" });
  const tipo = eff.scegli && eff.scegli.length ? el("select", {}, eff.scegli.map((t) => el("option", { value: t, testo: t }))) : null;
  const altri = vista.combattenti.slice().sort((x, y) => (y.tipo === chi.tipo) - (x.tipo === chi.tipo));
  const max = eff.soloSe ? 1 : eff.bersagli > 0 ? eff.bersagli : 0;
  const scelte = altri.filter((x) => !eff.soloSe || x.id === chi.id).map((x) => ({ x, i: el("input", { type: "checkbox", value: x.id, selezionato: x.id === chi.id }) }));
  const conta = el("p", { class: "tenue piccolo" });
  const aggiorna = () => {
    const n = scelte.filter((s) => s.i.checked).length;
    conta.textContent = n + (max ? " di " + max : "") + " bersagli scelti";
    scelte.forEach((s) => { s.i.disabled = eff.soloSe || (!!max && !s.i.checked && n >= max); });
  };
  scelte.forEach((s) => s.i.addEventListener("change", aggiorna));
  aggiorna();
  // Eroismo dalla lista del Master: i PF temporanei (modificatore da incantatore) si scrivono qui
  const pfTurno = eff.mod && eff.mod.pfTempTurno === "inc" ? el("input", { type: "number", min: "1", max: "10", valore: "3", "aria-label": "PF temporanei a inizio turno" }) : null;
  const descr = () => [testoModifiche(eff.mod), (eff.cond || []).join(", ").toLowerCase(), eff.tsCar ? "TS su " + (CAR[eff.tsCar] || eff.tsCar) + (eff.tsCd ? " CD " + eff.tsCd : "") + ": chi riesce non è colpito" : ""].filter(Boolean).join(" · ");
  const cdTS = eff.tsCar && !eff.tsCd ? el("input", { type: "number", min: "1", max: "30", valore: "13", "aria-label": "CD del tiro salvezza" }) : null;
  const testa = el("p", { class: "piccolo", testo: descr() || "Scegli le modifiche." });
  dialogo("✨ " + eff.nome + (chi ? " · " + chi.nome : ""), el("div", { class: "att-dialogo" },
    testa, personalizzato,
    tipo ? el("label", {}, eff.nome === "Scudo di fuoco" ? "Resistenza a" : "Resistenza al tipo", tipo) : null,
    pfTurno ? el("label", {}, "PF temporanei a inizio turno (modificatore da incantatore)", pfTurno) : null,
    cdTS ? el("label", {}, "CD del tiro salvezza", cdTS) : null,
    eff.nota ? el("p", { class: "tenue piccolo", testo: "Da ricordare: " + eff.nota }) : null,
    el("p", { class: "tenue piccolo", testo: "Usa " + ({ azione: "l'azione", bonus: "l'azione bonus", reazione: "la reazione" }[eff.tempo] || "nessuna azione del turno") +
      (eff.conc ? " · concentrazione" : "") + (eff.round ? " · " + (eff.round >= 600 ? Math.round(eff.round / 600) + (eff.round >= 1200 ? " ore" : " ora") : eff.round >= 10 ? Math.round(eff.round / 10) + " min" : eff.round + " round") : "") + "." }),
    el("div", { class: "inc-bersagli", role: "group", "aria-label": "Bersagli" }, scelte.map(({ x, i }) => el("label", { class: "riga" }, i,
      el("span", { testo: x.nome + (x.id === chi.id ? " (" + (chi.mio ? "tu" : "chi lo usa") + ")" : "") })))), conta, err),
  { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Lo segno a mano", on: { click: chiudi } }),
    el("button", { class: "primario", testo: "✨ Applica", on: { click: async (ev) => {
      const b = ev.currentTarget;
      if (personalizzato && personalizzato.leggi) Object.assign(eff, personalizzato.leggi());
      const bersagli = scelte.filter((s) => s.i.checked).map((s) => s.x.id);
      if (!bersagli.length) { err.textContent = "Scegli almeno un bersaglio."; return; }
      const mod = { ...(eff.mod || {}) };
      if (tipo) mod.res = [...(mod.res || []), tipo.value];
      // Scudo di fuoco: danni del tipo opposto alla resistenza (caldo: resiste al freddo, brucia col fuoco)
      if (eff.ritorsione && tipo) mod.ritorsione = { dadi: eff.ritorsione.dadi, tipo: tipo.value === "freddo" ? "fuoco" : "freddo" };
      if (mod.pfTempTurno === "inc") mod.pfTempTurno = Math.max(1, Math.min(10, +(pfTurno && pfTurno.value) || 1));
      if (!testoModifiche(mod) && !(eff.cond || []).length) { err.textContent = "L'effetto non cambia nessun valore."; return; }
      b.disabled = true;
      try {
        const ts = eff.tsCar ? { tsCar: eff.tsCar, tsCd: +eff.tsCd || Math.max(1, Math.min(30, +(cdTS && cdTS.value) || 13)) } : {};
        const r = await chiedi(sessione, "effetto.nuovo", { combattente: chi.id, bersagli, valori: { nome: eff.nome, tempo: eff.tempo || "altro", conc: !!eff.conc, round: +eff.round || 0, mod, condizioni: eff.cond || [], ...ts } });
        if (!r) { b.disabled = false; return; }
        chiudi();
        avviso("✨ " + (r.testo || eff.nome), "", 6000);
      } catch (e) { err.textContent = e.message; b.disabled = false; }
    } } })) });
}

/** Master: un effetto a mano usato da «chi» (potenziamento degli incantesimi o modifiche scritte). */
export async function apriEffettoManuale(sessione, vista, chi) {
  let lista = [];
  try { lista = await caricaPotenziamenti(); } catch { /* resta il personalizzato */ }
  const scelta = el("select", { "aria-label": "Effetto" }, el("option", { value: "", testo: "— Personalizzato —" }), lista.map((p, i) => el("option", { value: String(i), testo: p.nome + " (" + (testoModifiche(p.mod) || (p.scegli ? "resistenza a scelta" : (p.cond || []).join(", ").toLowerCase())) + ")" })));
  dialogo("✨ Effetto su " + chi.nome, el("div", { class: "att-dialogo" },
    el("p", { class: "piccolo", testo: "Un effetto che cambia i valori (CA, resistenze, TS, attacchi) finché dura. Scegli un incantesimo o scrivi le modifiche." }),
    el("label", {}, "Effetto", scelta)),
  { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
    el("button", { class: "primario", testo: "Avanti", on: { click: () => {
      chiudi();
      if (scelta.value !== "") { const p = lista[+scelta.value]; finestraEffetto(sessione, vista, chi, { ...p, bersagli: 0 }); return; }
      // personalizzato
      const nome = el("input", { type: "text", valore: "Effetto", maxlength: "60", "aria-label": "Nome" });
      const ca = el("input", { type: "number", min: "-5", max: "10", valore: "0", class: "num-piccolo", "aria-label": "Bonus alla CA" });
      const res = el("select", { "aria-label": "Resistenza" }, el("option", { value: "", testo: "—" }), TIPI.map((t) => el("option", { value: t, testo: t })));
      const vant = el("select", { "aria-label": "Vantaggio ai TS" }, el("option", { value: "", testo: "—" }), Object.entries(CAR).map(([k, n]) => el("option", { value: k, testo: n })));
      const svant = el("input", { type: "checkbox" });
      const durata = el("input", { type: "number", min: "0", max: "14400", valore: "10", class: "num-piccolo", "aria-label": "Durata in round" });
      const conc = el("input", { type: "checkbox" });
      const campi = el("div", { class: "eff-personalizzato" },
        el("label", {}, "Nome", nome), el("label", { class: "riga" }, el("span", { testo: "Bonus alla CA" }), ca), el("label", {}, "Resistenza a", res),
        el("label", {}, "Vantaggio ai TS su", vant), el("label", { class: "riga" }, svant, el("span", { testo: "Chi lo attacca ha svantaggio" })),
        el("label", { class: "riga" }, el("span", { testo: "Durata (round, 0 = finché non lo termini)" }), durata), el("label", { class: "riga" }, conc, el("span", { testo: "Concentrazione di chi lo usa" })));
      campi.leggi = () => {
        const mod = {};
        const n = parseInt(ca.value, 10) || 0;
        if (n) mod.caBonus = Math.max(-5, Math.min(10, n));
        if (res.value) mod.res = [res.value];
        if (vant.value) mod.vantTS = [vant.value];
        if (svant.checked) mod.svantContro = true;
        return { nome: nome.value.trim().slice(0, 60) || "Effetto", mod, round: Math.max(0, Math.min(14400, parseInt(durata.value, 10) || 0)), conc: conc.checked };
      };
      finestraEffetto(sessione, vista, chi, { nome: "Effetto personalizzato", tempo: "altro", mod: {}, round: 10 }, { personalizzato: campi });
    } } })) });
}
