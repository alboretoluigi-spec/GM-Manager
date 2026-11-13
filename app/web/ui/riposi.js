// Riposo lungo e riposo breve dalla barra in alto (richiesta dell'utente, 2026-11-11; server: internal/server/riposi.go).
// Il giocatore sceglie il suo personaggio e i compagni del gruppo con cui riposare; la richiesta va al Master, che approva o
// rifiuta (con gli avvisi: combattimento in corso, riposo lungo meno di 24 ore fa, 0 PF). Il Master fa riposare subito.
// Approvato: passa il tempo di gioco (8 ore / 1 ora); nel riposo breve ognuno sceglie quanti Dadi Vita spendere.
import { el, api, avviso, dialogo, tempoReale } from "./ui.js";
import { lancioAutomatico } from "./dadi.js";

const EFFETTI = {
  lungo: "8 ore di gioco: PF al massimo, tutti gli slot, metà dei Dadi Vita, Indebolimento −1.",
  breve: "1 ora di gioco: ognuno può spendere Dadi Vita per recuperare PF; il warlock recupera gli slot del patto.",
};
const NOME = { lungo: "Riposo lungo", breve: "Riposo breve" };

/** I due tasti della barra in alto. */
export function tastiRiposo() {
  return [
    el("button", { class: "piccolo tasto-riposo con-icona", "data-ico": "🌙", testo: "🌙 Riposo lungo", title: EFFETTI.lungo, on: { click: () => apriRiposo("lungo") } }),
    el("button", { class: "piccolo tasto-riposo con-icona", "data-ico": "☕", testo: "☕ Riposo breve", title: EFFETTI.breve, on: { click: () => apriRiposo("breve") } }),
  ];
}

/** Finestra della richiesta: gruppo, personaggi (i miei e i compagni), avvisi, invio. */
export async function apriRiposo(tipo) {
  let d;
  try { d = await api("/riposi"); } catch (e) { avviso(e.message, "errore"); return; }
  const gruppi = (d.gruppi || []).filter((g) => (g.personaggi || []).length && (d.master || g.personaggi.some((p) => p.mio)));
  if (!gruppi.length) { avviso(d.master ? "Nessun gruppo con personaggi attivi." : "Non hai personaggi attivi in un gruppo.", "errore", 5000); return; }
  let g = gruppi[0];
  const corpo = el("div");
  const disegna = () => {
    const attesa = (d.inAttesa || []).find((r) => r.gruppo === g.id);
    const scelta = gruppi.length > 1 ? el("label", {}, "Gruppo", el("select", { on: { change: (e) => { g = gruppi.find((x) => x.id === e.target.value); disegna(); } } },
      gruppi.map((x) => el("option", { value: x.id, selected: x.id === g.id ? true : null, testo: x.nome })))) : null;
    const caselle = g.personaggi.map((p) => {
      const c = el("input", { type: "checkbox", value: p.id, selezionato: true });
      return el("label", { class: "riga riposo-pg" }, c, el("span", {}, el("b", { testo: p.nome }), " ", el("span", { class: "tenue piccolo", testo: p.mio ? "(tuo)" : p.giocatore ? "(" + p.giocatore + ")" : "(del Master)" })));
    });
    corpo.replaceChildren(...[ // (i null si tolgono: replaceChildren li scriverebbe come testo)
      el("p", { class: "tenue", testo: EFFETTI[tipo] }),
      el("p", { class: "piccolo tenue", testo: "Gruppo " + g.nome + " · giorno " + g.giorno + ", ora di gioco " + oraDi(g.ora) }),
      scelta,
      g.combattimento ? el("p", { class: "errore piccolo", testo: "⚔ C'è un combattimento in corso: il Master lo vedrà nella richiesta." }) : null,
      attesa ? el("p", { class: "piccolo", testo: "⏳ C'è già una richiesta di riposo in attesa per questo gruppo (" + NOME[attesa.tipo].toLowerCase() + " chiesto da " + (attesa.chi || "un giocatore") + ")." }) : null,
      el("div", { class: "riposo-lista", role: "group", "aria-label": "Chi riposa" }, el("p", { class: "piccolo", testo: d.master ? "Chi riposa:" : "Chi riposa con te (togli la spunta ai compagni che non riposano):" }), caselle),
    ].filter(Boolean));
    corpo.dataset.attesa = attesa && !d.master ? attesa.id : "";
    corpo.dataset.mia = attesa && attesa.utente && !d.master ? "1" : "";
  };
  disegna();
  dialogo((tipo === "lungo" ? "🌙 " : "☕ ") + NOME[tipo], corpo, {
    azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: d.master ? "Fai riposare" : "Chiedi al Master", on: { click: async (e) => {
        const pgs = [...corpo.querySelectorAll(".riposo-pg input:checked")].map((x) => x.value);
        if (!pgs.length) { avviso("Scegli almeno un personaggio.", "errore"); return; }
        e.target.disabled = true;
        const invia = (forza) => api("/riposi", { dati: { tipo, gruppo: g.id, personaggi: pgs, forza } });
        try {
          const r = await invia(false);
          chiudi(); avviso((d.master ? "" : "⏳ ") + r.testo, "", 8000);
        } catch (er) {
          if (d.master && er.stato === 409 && /^Da confermare/.test(er.message)) { // avvisi: il Master conferma
            const ok = await chiediConferma(er.message.replace(/^Da confermare: /, ""));
            if (ok) { try { const r = await invia(true); chiudi(); avviso(r.testo, "", 8000); return; } catch (e2) { avviso(e2.message, "errore", 6000); } }
          } else avviso(er.message, "errore", 6000);
          e.target.disabled = false;
        }
      } } })),
  });
}

function chiediConferma(testo) {
  return new Promise((ok) => {
    dialogo("Fare riposare lo stesso?", el("div", {}, el("p", { testo }), el("p", { class: "tenue piccolo", testo: "Puoi far riposare lo stesso: decidi tu." })), {
      azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { testo: "Annulla", on: { click: () => { chiudi(); ok(false); } } }),
        el("button", { class: "primario", testo: "Sì, fai riposare", on: { click: () => { chiudi(); ok(true); } } })),
    });
  });
}

const oraDi = (h) => { const m = Math.round((h || 0) * 60) % 1440; return String(Math.floor(m / 60)).padStart(2, "0") + ":" + String(m % 60).padStart(2, "0") + " (dall'inizio del giorno)"; };

// ----- Master: richiesta da autorizzare -----
const aperte = new Map();
function mostraRichiesta(r) {
  if (!r || r.stato !== "in_attesa" || aperte.has(r.id)) return;
  const avvisi = r.avvisi || [];
  dialogo((r.tipo === "lungo" ? "🌙 " : "☕ ") + NOME[r.tipo] + " chiesto da " + (r.chi || "un giocatore"), el("div", {},
    el("p", {}, "Gruppo ", el("b", { testo: r.nomeGruppo || "" }), ". Riposano: ", el("b", { testo: (r.personaggi || []).map((p) => p.nome).join(", ") }), "."),
    el("p", { class: "tenue piccolo", testo: EFFETTI[r.tipo] }),
    avvisi.length ? el("div", { class: "riposo-avvisi" }, el("p", { class: "piccolo", testo: "⚠ Da sapere prima di autorizzare:" }), el("ul", {}, avvisi.map((a) => el("li", { testo: a })))) : null), {
    azioni: (chiudi) => {
      aperte.set(r.id, chiudi);
      const fine = () => { aperte.delete(r.id); chiudi(); };
      const decidi = async (azione, btn) => {
        btn.disabled = true;
        try { const x = await api("/riposi/" + encodeURIComponent(r.id), { dati: { azione, forza: avvisi.length > 0 } }); avviso(x.testo, "", 8000); fine(); }
        catch (e) { avviso(e.message, "errore", 7000); btn.disabled = false; if (e.stato === 409 && !/^Da confermare/.test(e.message)) fine(); }
      };
      return el("div", { class: "azioni" },
        el("button", { testo: "Più tardi", on: { click: () => { aperte.delete(r.id); chiudi(); } } }),
        el("button", { class: "pericolo", testo: "Non autorizzare", on: { click: (e) => decidi("rifiuta", e.target) } }),
        el("button", { class: "primario", testo: avvisi.length ? "Autorizza lo stesso" : "Autorizza", on: { click: (e) => decidi("approva", e.target) } }));
    },
  });
}

// ----- Riposo breve: Dadi Vita -----
const spesiQui = new Set();
function chiediDadiVita(idRiposo, p) {
  const chiave = idRiposo + ":" + p.id;
  if (spesiQui.has(chiave)) return;
  spesiQui.add(chiave);
  if (p.senzaDati) { avviso("☕ " + p.nome + ": apri la scheda una volta, così Gaster Bless conosce i suoi Dadi Vita.", "", 8000); spesiQui.delete(chiave); return; }
  const campi = (p.dadiVita || []).map((d) => {
    const n = el("input", { type: "number", min: "0", max: String(Math.min(d.n, p.liberi)), value: "0", dataset: { f: String(d.f) }, "aria-label": "Quanti d" + d.f });
    return el("label", { class: "riga" }, "d" + d.f + " (" + d.n + " in tutto)", n);
  });
  dialogo("☕ Dadi Vita di " + p.nome, el("div", {},
    el("p", { testo: "PF " + p.pf + "/" + p.pfMax + " · Dadi Vita liberi: " + p.liberi + " su " + p.totale + " · ogni dado + Costituzione (" + (p.modCos >= 0 ? "+" : "") + p.modCos + ")." }),
    p.liberi ? campi : el("p", { class: "tenue", testo: "Nessun Dado Vita libero: si recuperano con il riposo lungo." })), {
    azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { testo: "Non ne spendo", on: { click: () => spendi({}, chiudi) } }),
      p.liberi ? el("button", { class: "primario", testo: "Tira", on: { click: (e) => {
        const dadi = {};
        campi.forEach((c) => { const i = c.querySelector("input"); const v = Math.max(0, parseInt(i.value, 10) || 0); if (v) dadi[i.dataset.f] = v; });
        e.target.disabled = true; spendi(dadi, chiudi, e.target);
      } } }) : null),
  });
  async function spendi(dadi, chiudi, btn) {
    try { const r = await api("/riposi/" + encodeURIComponent(idRiposo) + "/dadi-vita", { dati: { personaggio: p.id, dadi } }); chiudi();
      // prima i Dadi Vita in 3D (fase D2), poi l'avviso con i PF recuperati
      lancioAutomatico({ dadi: r.dadi, totale: r.tirato, etichetta: "Dadi Vita di " + p.nome, formula: (r.dadi || []).map((g) => g.valori.length + "d" + g.facce).join(" + ") + " + Costituzione" }, () => avviso("☕ " + r.testo, "", 9000)); }
    catch (e) { avviso(e.message, "errore", 6000); if (btn) btn.disabled = false; if (e.stato === 409) chiudi(); }
  }
}
async function dadiInSospeso() {
  try { const d = await api("/riposi"); (d.dadiVita || []).forEach((r) => (r.daSpendere || []).forEach((p) => chiediDadiVita(r.id, p))); } catch { /* niente */ }
}

let installato = false;
export function installaRiposi(io) {
  if (installato) return;
  installato = true;
  const master = () => { const u = io(); return u && u.ruolo === "master"; };
  tempoReale.su("riposo.richiesta", (r) => { if (master()) mostraRichiesta(r); });
  tempoReale.su("riposo.deciso", (d) => { if (d && aperte.has(d.id)) { const c = aperte.get(d.id); aperte.delete(d.id); c(); } });
  tempoReale.su("riposo.info", (d) => { if (d && d.testo) avviso("🌙 " + d.testo, "", 8000); });
  tempoReale.su("riposo.esito", (d) => { if (d && d.testo) avviso(d.testo, "errore", 9000); });
  tempoReale.su("riposo.fatto", (d) => {
    if (!d) return;
    avviso((d.tipo === "lungo" ? "🌙 " : "☕ ") + d.testo, "", 12000);
    if (d.tipo === "breve") setTimeout(dadiInSospeso, 400); // ognuno sceglie i suoi Dadi Vita
  });
  tempoReale.su("riposo.dadi", (d) => { if (master() && d && d.testo) avviso("☕ " + d.testo, "", 7000); });
  tempoReale.su("benvenuto", () => {
    if (master()) api("/riposi").then((d) => (d.inAttesa || []).forEach(mostraRichiesta)).catch(() => {});
    dadiInSospeso();
  });
}
