// Incantesimi nello scontro (internal/server/incantesimi_scontro.go; Parte 2 del piano di combattimento).
// - apriIncantesimo(scontro, effetto): dopo «Lancia incantesimo» nella scheda, se il personaggio è in uno scontro e
//   l'effetto si risolve in automatico (web/scheda/incantesimi_ui.js, effettoScontro): scelta dei bersagli e lancio.
//   Attacco con incantesimo: tiro come le armi (anche più raggi, uno alla volta); tiro salvezza: i TS li tira il server e
//   li conferma il Master; cure e PF temporanei: subito.
// - mostraConferma(e): il riepilogo riservato al Master (tiri salvezza dei bersagli, danni tirati, condizioni), con la
//   possibilità di cambiare ogni esito (Resistenza leggendaria…) e «Applica».
// - ascolta(io): avvisi in tutta l'app (riepiloghi per il Master, esiti per tutti, senza doppioni).
import { el, svuota, api, avviso, dialogo, conferma, tempoReale } from "./ui.js";
import { segnalaMira } from "./mira.js";
import { corpoEsito, sg, testoDanni, editorDadi, lanciaEsito } from "./attacco.js";
import { lancioAutomatico } from "./dadi.js";
import { finestraEffetto } from "./effetti.js";

const NOMI_CAR = { FOR: "Forza", DES: "Destrezza", COS: "Costituzione", INT: "Intelligenza", SAG: "Saggezza", CAR: "Carisma" };
const nuovoId = () => "a" + Date.now().toString(36) + Math.random().toString(36).slice(2, 10);
const visti = new Set(); // esiti già mostrati (arrivano dal canale della sessione e, a chi ha lanciato, anche direttamente)
const mieAzioni = new Set(); // azioni mandate da questa pagina
let utente = null;

let inAscolto = false;
export function ascolta(io) {
  utente = io && io.utente;
  if (inAscolto) return; // avvio() può ripartire (sessione scaduta): gli ascoltatori una volta sola
  inAscolto = true;
  tempoReale.su("incantesimo.conferma", (e) => { if (utente && utente.ruolo === "master") mostraConferma(e); });
  tempoReale.su("incantesimo.esito", (e) => mostraEsitoIncantesimo(e));
}

const testoTiro = (parti) => (parti || []).map((d) => d.espr + (d.dadi && d.dadi.length ? " [" + d.dadi.join(", ") + "]" : "") + " = " + d.tirato + (d.tipo ? " " + d.tipo : "")).join(" + ");

export async function apriIncantesimo(scontro, eff) {
  if (!eff || !eff.tipo || !scontro) return;
  const sid = scontro.sessione;
  let vista;
  try { vista = await api("/sessioni/" + encodeURIComponent(sid) + "/combattimento"); } catch (e) { avviso(e.message, "errore"); return; }
  const c = vista.combattimento && vista.combattenti.find((x) => x.id === scontro.combattente);
  if (!c) return;
  // potenziamenti (Scudo della fede, Benedizione, Armatura magica…): effetto che cambia i valori, anche prima dell'inizio
  if (eff.tipo === "effetto") { finestraEffetto(sid, vista, c, eff); return; }
  if (!vista.combattimento.avviato) return; // scontro non ancora iniziato: l'effetto resta da applicare a mano
  const lancio = nuovoId();
  const inc = { nome: eff.nome, livello: eff.livello, slot: eff.slot, tempo: eff.tempo, conc: !!eff.conc, tipo: eff.tipo, bonus: eff.bonus, cd: eff.cd, car: eff.car,
    seRiesce: eff.seRiesce, danni: eff.danni || [], condizioni: eff.condizioni || [], cura: eff.cura, raggi: 1, lancio, mischia: !!eff.mischia,
    round: +eff.round || 0, ripeti: !!eff.ripeti };
  const altri = vista.combattenti.slice();
  const inPiedi = (x) => (x.pf != null ? x.pf > 0 : x.salute !== "a terra");
  const alleati = eff.tipo === "cura" || eff.tipo === "pftemp";
  // avversari prima (o alleati prima, per le cure)
  altri.sort((x, y) => alleati ? (y.tipo === c.tipo) - (x.tipo === c.tipo) : ((y.tipo !== c.tipo) - (x.tipo !== c.tipo) || inPiedi(y) - inPiedi(x)));
  const etichetta = (x) => x.nome + (x.id === c.id ? " (tu)" : "") + (x.ca != null && eff.tipo === "attacco" ? " · CA " + x.ca : "") + (x.pf != null ? " · " + x.pf + "/" + x.pfMax + " PF" : x.salute ? " · " + x.salute : "");
  const err = el("p", { class: "errore" });
  const descr = eff.tipo === "attacco" ? "Attacco con incantesimo " + sg(eff.bonus) + " · " + testoDanni({ danni: eff.danni || [] })
    : eff.tipo === "ts" ? "TS su " + (NOMI_CAR[eff.car] || eff.car) + " CD " + eff.cd + (eff.danni && eff.danni.length ? " · " + testoDanni({ danni: eff.danni }) + (eff.seRiesce === "meta" ? " (metà se riesce)" : " (niente se riesce)") : "") +
      (eff.condizioni && eff.condizioni.length ? " · " + eff.condizioni.join(", ") : "")
      : (eff.tipo === "cura" ? "Cura " : "PF temporanei ") + (eff.cura ? eff.cura.dadi + (eff.cura.mod ? sg(eff.cura.mod) : "") : "");
  const testa = [el("p", { class: "piccolo", testo: descr }), eff.riassunto ? el("p", { class: "tenue piccolo", testo: "Al livello scelto: " + eff.riassunto }) : null,
    el("p", { class: "tenue piccolo", testo: "Usa " + ({ azione: "l'azione", bonus: "l'azione bonus", reazione: "la reazione" }[eff.tempo] || "nessuna azione del turno") + (eff.conc ? " · concentrazione" : "") + "." })];
  // dadi dei danni (o della cura) con il numero modificabile prima del tiro (fase D2): cambiano l'incantesimo mandato
  const edDanni = editorDadi(eff.danni || []);
  const cura = eff.cura && /^(\d+)d(\d+)$/.exec(eff.cura.dadi || "");
  const edCura = cura ? editorDadi([{ dadi: eff.cura.dadi, mod: eff.cura.mod }], eff.tipo === "cura" ? "Dadi della cura" : "Dadi dei PF temporanei") : null;
  const applicaDadi = () => {
    if (edDanni.cambiato()) inc.danni = edDanni.danni();
    if (edCura && edCura.cambiato()) inc.cura = { ...eff.cura, dadi: edCura.danni()[0].dadi };
  };
  // dadi 3D di un esito di incantesimo (TS dei bersagli e danni, oppure la cura), poi dopo()
  const dadiIncantesimo = (r, dopo) => {
    const g = [];
    if (r.righe && r.tipo === "ts") { const d20 = r.righe.flatMap((x) => x.d20 || []); if (d20.length) g.push({ facce: 20, valori: d20 }); }
    (r.tiro || []).forEach((d) => { const m = /d(\d+)/.exec(d.espr || ""); if (m && d.dadi && d.dadi.length) g.push({ facce: +m[1], valori: d.dadi }); });
    const tot = (r.tiro || []).reduce((a, d) => a + (d.tirato || 0), 0);
    lancioAutomatico({ dadi: g, totale: tot || (r.righe && r.righe[0] ? r.righe[0].totale : 0), etichetta: (r.nomeIncantatore || c.nome) + " · " + (r.incantesimo || eff.nome),
      formula: r.tipo === "ts" ? "TS su " + (NOMI_CAR[r.car] || r.car) + " CD " + r.cd + (tot ? " · danni " + tot : "") : (r.tipo === "cura" ? "cura " : "PF temporanei ") + tot,
      colore: r.colore || "" }, dopo);
  };
  const invia = async (tipo, dati, b) => {
    b.disabled = true;
    const id = nuovoId(); mieAzioni.add(id);
    try {
      let r = await tempoReale.chiedi(tipo, { sessione: sid, combattente: c.id, incantesimo: inc, azione: id, ...dati });
      if (r && r.serveConferma) {
        if (!(await conferma("Lanciare lo stesso?", r.serveConferma, "Lancia"))) { b.disabled = false; return null; }
        const id2 = nuovoId(); mieAzioni.add(id2);
        r = await tempoReale.chiedi(tipo, { sessione: sid, combattente: c.id, incantesimo: inc, azione: id2, forza: true, ...dati });
      }
      return r;
    } catch (e) { err.textContent = e.message; b.disabled = false; return null; }
  };

  if (eff.tipo === "attacco") { // un raggio alla volta, ognuno con il suo bersaglio
    const raggi = el("input", { type: "number", min: "1", max: "10", valore: "1", class: "num-piccolo", title: "Raggi o dardi dell'incantesimo (per esempio 3 per Raggio rovente)" });
    const raggio = (n, bersaglio) => {
      const ber = el("select", { "aria-label": "Bersaglio" }, altri.filter((x) => x.id !== c.id).map((x) => el("option", { value: x.id, selected: x.id === bersaglio, testo: etichetta(x) })));
      segnalaMira(sid, c, [ber.value]); // mostri: «X prende di mira Y» (mira.js)
      ber.addEventListener("change", () => segnalaMira(sid, c, [ber.value]));
      const modi = ["normale", "vantaggio", "svantaggio"].map((m) => ({ m, i: el("input", { type: "radio", name: "modo-inc", value: m, selezionato: m === "normale" }) }));
      const err2 = el("p", { class: "errore" });
      dialogo("✨ " + eff.nome + (n > 1 ? " · raggio " + n : ""), el("div", { class: "att-dialogo" }, ...testa, el("label", {}, "Bersaglio", ber),
        n === 1 ? el("label", { class: "riga" }, el("span", { testo: "Raggi (tiri per colpire)" }), raggi) : null, n === 1 ? edDanni.el : null,
        el("div", { class: "riga-flex att-modi", role: "radiogroup", "aria-label": "Tiro" }, modi.map(({ m, i }) => el("label", { class: "riga" }, i, el("span", { testo: m[0].toUpperCase() + m.slice(1) })))), err2, err),
      { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: n === 1 ? "Non tirare" : "Basta così", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "🎲 Tira", on: { click: async (ev) => {
          if (n === 1) { inc.raggi = Math.max(1, Math.min(10, parseInt(raggi.value, 10) || 1)); applicaDadi(); }
          const r = await invia("attacco.tira", { bersaglio: ber.value, modo: modi.find((x) => x.i.checked).m }, ev.currentTarget);
          if (!r || !r.d20) { if (r) chiudi(); return; }
          chiudi(); lanciaEsito(r, () => esitoRaggio(r, n, ber.value)); // prima i dadi 3D
        } } })) });
    };
    const esitoRaggio = (e, n, bersaglio) => {
      const zona = el("div", { class: "att-risultato" });
      let chiudiD = null;
      const disegna = (x) => {
        svuota(zona, corpoEsito(x));
        if (x.risultato === "critico" && x.gettone) {
          const scegli = async (modo, b) => {
            b.disabled = true;
            try { const r = await tempoReale.chiedi("attacco.critico", { sessione: sid, gettone: x.gettone, modo, azione: nuovoId() }); lanciaEsito(r, () => disegna(r), true); }
            catch (er) { avviso(er.message, "errore"); b.disabled = false; }
          };
          zona.append(el("p", { class: "piccolo", testo: "Come si calcolano i danni del critico?" }), el("div", { class: "riga-flex" },
            el("button", { class: "primario", testo: "🎲🎲 Tira due volte i dadi", on: { click: (ev) => scegli("dadi", ev.currentTarget) } }),
            el("button", { testo: "✖2 Raddoppia il risultato", on: { click: (ev) => scegli("raddoppia", ev.currentTarget) } })));
        } else if (x.attacchiRimasti > 0) {
          zona.append(el("button", { class: "primario", testo: "✨ Raggio successivo (" + (x.attacchiRimasti === 1 ? "ne resta 1" : "ne restano " + x.attacchiRimasti) + ")",
            on: { click: () => { if (chiudiD) chiudiD(); raggio(n + 1, bersaglio); } } }));
        }
      };
      disegna(e);
      chiudiD = dialogo("✨ " + e.nomeAttaccante + " → " + e.nomeBersaglio + " · " + eff.nome, zona);
    };
    raggio(1, (altri.find((x) => x.id !== c.id && x.tipo !== c.tipo && inPiedi(x)) || {}).id);
    return;
  }

  // tiro salvezza, cura, PF temporanei: più bersagli
  const max = eff.bersagli > 0 ? eff.bersagli : 0;
  const scelte = altri.map((x) => ({ x, i: el("input", { type: "checkbox", value: x.id }) }));
  const conta = el("p", { class: "tenue piccolo" });
  const aggiorna = () => {
    const n = scelte.filter((s) => s.i.checked).length;
    conta.textContent = n + (max ? " di " + max : "") + " bersagli scelti" + (eff.tipo === "ts" ? ". I tiri salvezza li tira Gaster Bless; il Master li controlla e poi applica l'effetto." : ".");
    scelte.forEach((s) => { s.i.disabled = !!max && !s.i.checked && n >= max; });
  };
  scelte.forEach((s) => s.i.addEventListener("change", aggiorna));
  if (eff.tipo === "ts") scelte.forEach((s) => s.i.addEventListener("change", () => segnalaMira(sid, c, scelte.filter((y) => y.i.checked).map((y) => y.x.id))));
  aggiorna();
  dialogo("✨ " + eff.nome, el("div", { class: "att-dialogo" }, ...testa,
    el("div", { class: "inc-bersagli", role: "group", "aria-label": "Bersagli" }, scelte.map(({ x, i }) => el("label", { class: "riga" }, i, el("span", { testo: etichetta(x) })))), conta,
    edDanni.el, edCura ? edCura.el : null, err),
  { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Lo applico a mano", on: { click: chiudi } }),
    el("button", { class: "primario", testo: eff.tipo === "ts" ? "🎲 Tira i TS" : "✨ Applica", on: { click: async (ev) => {
      const bersagli = scelte.filter((s) => s.i.checked).map((s) => s.x.id);
      if (!bersagli.length) { err.textContent = "Scegli almeno un bersaglio."; return; }
      applicaDadi();
      const r = await invia(eff.tipo === "ts" ? "incantesimo.ts" : "incantesimo.cura", { bersagli }, ev.currentTarget);
      if (!r) return;
      chiudi();
      if (r.inAttesa) avviso("✨ " + eff.nome + ": tiri salvezza fatti. Il Master li controlla e applica l'effetto.", "", 7000);
      else if (r.gettone) dadiIncantesimo(r, () => mostraConferma(r)); // il Master che lancia: dadi 3D, poi il riepilogo
      else dadiIncantesimo(r, () => mostraEsitoIncantesimo(r));
    } } })) });
}

// Riepilogo riservato al Master: un aperto per gettone.
const confermeAperte = new Set();
export function mostraConferma(e) {
  if (!e || !e.gettone || confermeAperte.has(e.gettone)) return;
  confermeAperte.add(e.gettone);
  const righe = e.righe.map((r) => ({ r, ok: el("input", { type: "checkbox", selezionato: r.riuscito, "aria-label": "TS riuscito per " + r.nome }) }));
  const conCond = el("input", { type: "checkbox", selezionato: true });
  const err = el("p", { class: "errore" });
  const danni = e.tiro && e.tiro.length ? el("p", {}, el("b", { testo: "Danni tirati: " }), testoTiro(e.tiro) + (e.tiro.length > 1 ? " = " + e.tiro.reduce((a, d) => a + d.tirato, 0) : "") +
    (e.seRiesce === "meta" ? " (metà a chi riesce)" : " (niente a chi riesce)")) : null;
  const condizioni = e.condizioni || [];
  dialogo("✨ " + e.incantesimo + " · " + e.nomeIncantatore + " (solo tu)", el("div", { class: "inc-conferma" },
    el("p", { class: "piccolo", testo: "TS su " + (NOMI_CAR[e.car] || e.car) + " contro CD " + e.cd + ". Togli o metti la spunta per cambiare un esito (per esempio con la Resistenza leggendaria), poi «Applica»." }),
    danni, condizioni.length ? el("p", { class: "piccolo", testo: "A chi fallisce: " + condizioni.join(", ") }) : null,
    el("table", { class: "inc-ts" }, el("thead", {}, el("tr", {}, ["Bersaglio", "d20", "Bonus", "Totale", "Riesce"].map((t) => el("th", { testo: t })))),
      el("tbody", {}, righe.map(({ r, ok }) => el("tr", {},
        el("td", {}, el("b", { testo: r.nome }), r.resMagia ? el("div", { class: "tenue piccolo", testo: "Vantaggio contro gli incantesimi" }) : null,
          r.leggendaria ? el("div", { class: "tenue piccolo", testo: "Resistenza leggendaria (" + r.leggendaria + "/giorno): può riuscire comunque" }) : null,
          r.immuneA && r.immuneA.length ? el("div", { class: "tenue piccolo", testo: "Immune a: " + r.immuneA.join(", ") }) : null,
          r.nota ? el("div", { class: "tenue piccolo", testo: r.nota }) : null),
        el("td", { testo: (r.d20 || []).join(" e ") || "—" }), el("td", { testo: sg(r.bonus) }), el("td", { testo: r.d20 ? String(r.totale) : "—" }), el("td", {}, ok))))),
    condizioni.length ? el("label", { class: "riga" }, conCond, el("span", { testo: "Applica le condizioni a chi fallisce (" + condizioni.join(", ") + ")" })) : null, err),
  { azioni: (ch) => el("div", { class: "azioni" }, el("button", { testo: "Più tardi", title: "Resta in attesa: lo ritrovi nel tracker", on: { click: () => { confermeAperte.delete(e.gettone); ch(); } } }),
    el("button", { class: "primario", testo: "✨ Applica", on: { click: async (ev) => {
      ev.currentTarget.disabled = true;
      const riusciti = {}; righe.forEach(({ r, ok }) => { riusciti[r.bersaglio] = ok.checked; });
      const id = nuovoId(); mieAzioni.add(id);
      try {
        const x = await tempoReale.chiedi("incantesimo.applica", { sessione: e.sessione, gettone: e.gettone, riusciti, applicaCondizioni: conCond.checked, azione: id });
        confermeAperte.delete(e.gettone); ch(); mostraEsitoIncantesimo(x, true);
      } catch (er) { err.textContent = er.message; ev.currentTarget.disabled = false; confermeAperte.delete(e.gettone); }
    } } })) });
}

// Esito applicato: finestra per chi ha lanciato e per il Master che applica, avviso per gli altri.
export function mostraEsitoIncantesimo(e, finestra = false) {
  if (!e || !e.righe || !e.applicato) return;
  if (e.id && visti.has(e.id)) return;
  if (e.id) visti.add(e.id);
  const riga = (r) => {
    const p = [r.nome];
    if (e.tipo === "ts") p.push(r.riuscito ? "riesce" : "fallisce");
    if (e.tipo === "cura") p.push("+" + r.cura + " PF");
    if (e.tipo === "pftemp") p.push(r.cura + " PF temporanei");
    if (r.totaleDanni) p.push(r.totaleDanni + " danni");
    if (r.pfPrima != null && r.pfDopo != null && e.tipo !== "pftemp") p.push(r.pfPrima + " → " + r.pfDopo + " PF");
    if (r.condizioni && r.condizioni.length) p.push(r.condizioni.join(", "));
    if (r.sconfitto) p.push("a terra!");
    if (r.concentrazione) p.push("concentrazione: TS Cos CD " + r.concentrazione);
    return p.join(" · ");
  };
  if (e.anonimo) { // incantatore nascosto: al giocatore solo la riga del suo PG, senza chi e che cosa
    const tipi = [...new Set((e.tiro || []).map((t) => t.tipo).filter(Boolean))].join(" e ");
    avviso("👁 Qualcosa che non vedi lancia un incantesimo su " + e.righe.map(riga).join("; ") + (tipi ? " (danni di tipo " + tipi + ")" : ""), "errore", 9000);
    return;
  }
  if (!finestra && !(utente && e.autore === utente.id)) { // gli altri: i dadi 3D di chi ha lanciato (2026-11-13), poi un avviso
    const g = [];
    if (e.tipo === "ts") { const d20 = e.righe.flatMap((x) => x.d20 || []); if (d20.length) g.push({ facce: 20, valori: d20 }); }
    (e.tiro || []).forEach((d) => { const m = /d(\d+)/.exec(d.espr || ""); if (m && d.dadi && d.dadi.length) g.push({ facce: +m[1], valori: d.dadi }); });
    const tot = (e.tiro || []).reduce((a, d) => a + (d.tirato || 0), 0);
    lancioAutomatico({ dadi: g, totale: tot || (e.righe[0] ? e.righe[0].totale || 0 : 0), etichetta: e.nomeIncantatore + " · " + e.incantesimo,
      formula: e.tipo === "ts" ? "TS su " + (NOMI_CAR[e.car] || e.car) + " CD " + e.cd + (tot ? " · danni " + tot : "") : (e.tipo === "cura" ? "cura " : "PF temporanei ") + tot,
      colore: e.colore || "", chi: e.nomeIncantatore, mio: false },
    () => avviso("✨ " + e.incantesimo + " (" + e.nomeIncantatore + "): " + e.righe.map(riga).join("; "), "", 8000));
    return;
  }
  dialogo("✨ " + e.incantesimo + " · " + e.nomeIncantatore, el("div", { class: "att-risultato" },
    e.tiro && e.tiro.length ? el("p", { class: "att-tiro", testo: (e.tipo === "ts" ? "Danni: " : "Tiro: ") + testoTiro(e.tiro) }) : null,
    el("ul", { class: "att-danni" }, e.righe.map((r) => el("li", { testo: riga(r) })))));
}
