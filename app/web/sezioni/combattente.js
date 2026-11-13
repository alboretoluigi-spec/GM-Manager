// Scheda ridotta di un combattente, in una finestra a parte aperta dal tracker (specifica del combattimento, fase D).
// Indirizzo: /combattente?s=<sessione>&c=<combattente>&finestra=1, oppure &segui=1 per seguire chi è di turno (la «scheda
// d'azione» del Master, che cambia da sola a ogni turno). Dati e permessi sono quelli del tracker: la pagina entra nella
// sessione sul canale in tempo reale e mostra la vista che il server prepara per chi la apre (i giocatori non vedono
// attacchi, PF esatti e CA dei mostri). Gli attacchi usano le stesse finestre del tracker (../ui/attacco.js).
import { el, svuota, avviso, tempoReale } from "../ui/ui.js";
import { creaAttacchi, sg, testoDanni } from "../ui/attacco.js";
import { apriFinestra } from "../ui/finestra.js";
import { tastoIra, testoModifiche } from "../ui/effetti.js";
import { statistiche } from "../ui/statistiche.js";
import { creaEffettiColpi } from "../ui/effetti_colpi.js";

export async function mostra(corpo, { titolo, azioni }) {
  const q = new URLSearchParams(location.search);
  const sid = q.get("s") || "", segui = q.has("segui");
  let cid = q.get("c") || "";
  let vista = null;
  const zona = el("div", { class: "comb-ridotta" });
  svuota(corpo, zona);
  const master = () => !!vista && vista.ruolo === "master";
  const gestisce = () => !!vista && (vista.ruolo === "master" || vista.ruolo === "segretario");
  const A = creaAttacchi({ sessione: () => sid, vista: () => vista, master });
  // effetti visivi su danni, cure e critici (../ui/effetti_colpi.js) sul riquadro di CA e PF
  const fx = creaEffettiColpi(() => zona);
  const comando = async (tipo, dati = {}) => {
    try { return await tempoReale.chiedi(tipo, { sessione: sid, ...dati }); }
    catch (e) { avviso(e.message, "errore"); throw e; }
  };

  async function entra() {
    try { vista = (await tempoReale.chiedi("sessione.entra", { sessione: sid })).vista; disegna(); }
    catch (e) { svuota(zona, el("p", { class: "errore", testo: e.message })); }
  }

  function disegna() {
    const k = vista.combattimento;
    if (segui && k && k.turnoDi) cid = k.turnoDi;
    const c = k && vista.combattenti.find((x) => x.id === cid);
    if (!k || !c) {
      titolo.textContent = "Scheda di combattimento";
      svuota(azioni);
      svuota(zona, el("p", { class: "tenue", testo: !k ? "Nessun combattimento in corso in questa sessione." : segui ? "In attesa che inizi il turno di qualcuno." : "Questo combattente non è più nello scontro." }));
      return;
    }
    titolo.textContent = (segui ? "▶ " : "") + c.nome;
    document.title = c.nome + " · Gaster Bless";
    const turno = k.turnoDi === c.id;
    const puoPG = c.tipo === "pg" && (gestisce() || c.mio), puo = c.tipo === "pg" ? puoPG : master() || (vista.ruolo === "segretario" && !c.nascosto);
    const puoAttaccare = k.avviato && c.attacchi && c.attacchi.length && (master() || (c.mio && turno));
    svuota(azioni, c.personaggio && (c.mio || master()) ? el("button", { class: "piccolo", testo: "📜 Scheda completa", title: "La scheda intera in un'altra finestra",
      on: { click: () => apriFinestra("/schede/" + encodeURIComponent(c.personaggio), "completa-" + c.personaggio, { larghezza: 1100, altezza: 860 }) } }) : null);

    const pf = c.pf != null ? c.pf : null, max = c.pfMax || 0;
    const perc = pf != null && max ? Math.max(0, Math.min(100, Math.round(pf / max * 100))) : null;
    const v = el("input", { type: "number", min: "0", max: "999", inputmode: "numeric", "aria-label": "Valore", class: "num-piccolo", placeholder: "0" });
    const val = () => { const n = parseInt(v.value, 10); if (!(n >= 0)) { avviso("Scrivi un numero.", "errore"); return null; } v.value = ""; return n; };
    const pre = c.tipo === "pg" ? "pg." : "mostro.";
    const per = c.attacchiPerAzione || 1, usati = Math.min(c.attacchiUsati || 0, per);

    svuota(zona,
      el("p", { class: "cr-turno" + (turno ? " tuo" : ""), testo: !k.avviato ? "Preparazione dello scontro" :
        "Round " + k.round + " · " + (turno ? (c.mio ? "▶ È il tuo turno!" : "▶ È il turno di " + c.nome) : "Turno di " + ((vista.combattenti.find((x) => x.id === k.turnoDi) || {}).nome || "…")) }),
      el("div", { class: "cr-stat", "data-comb": c.id },
        c.ca != null ? el("div", { class: "cr-box" + (c.caOrig != null ? " valore-modificato" : ""), title: c.caOrig != null ? "Con " + (c.effettiCA || []).join(", ") + "; senza: " + c.caOrig : null },
          el("small", { testo: "CA" }), el("b", { testo: String(c.ca) }), c.caOrig != null ? el("small", { testo: " (" + c.caOrig + ")" }) : null) : null,
        pf != null ? el("div", { class: "cr-box cr-pf" }, el("small", { testo: "PF" }), el("b", { testo: pf + " / " + max + (c.pfTemp ? " (+" + c.pfTemp + ")" : "") }),
          el("div", { class: "barra-pf" }, el("span", { stile: { width: perc + "%", background: perc > 50 ? "var(--ok)" : perc > 0 ? "var(--attenzione)" : "var(--errore)" } })))
          : el("div", { class: "cr-box" }, el("small", { testo: "Stato" }), el("b", { testo: c.salute || "—" })),
        c.iniziativa != null ? el("div", { class: "cr-box" }, el("small", { testo: "Iniziativa" }), el("b", { testo: String(c.iniziativa) })) : null),
      c.condizioni.length ? el("p", { class: "piccolo", testo: "Condizioni: " + c.condizioni.join(", ") }) : null,
      // effetti in corso su questo combattente (../ui/effetti.js) e tasto dell'ira
      (() => { const l = (vista.effetti || []).filter((e) => e.mod && e.bersagli.some((b) => b.id === c.id));
        return l.length ? el("p", { class: "piccolo valore-modificato", testo: "✨ " + l.map((e) => e.nome + " (" + testoModifiche(e.mod) + ")").join(" · ") }) : null; })(),
      k.avviato && c.tipo === "pg" ? tastoIra(sid, vista, c) : null,
      c.tipo === "pg" && pf === 0 && c.tsMorte ? el("p", { class: "etichetta bloccato", testo: "TS contro la morte ✓" + c.tsMorte.s + " ✗" + c.tsMorte.f }) : null,
      c.economia && k.avviato ? el("div", { class: "iniz-eco riga-flex piccolo", role: "group", "aria-label": "Azione, azione bonus, reazione" }, [["a", "Azione"], ["b", "Azione bonus"], ["r", "Reazione"]].map(([x, nome]) => {
        const usata = !!c.economia[x], t = (usata ? "✗ " : "✓ ") + nome;
        const tocca = c.tipo === "pg" ? puoPG : master();
        return tocca ? el("button", { class: "eco" + (usata ? " usata" : ""), "aria-pressed": usata ? "true" : "false", testo: t,
          on: { click: () => comando("iniz.economia", { combattente: c.id, economia: { ...c.economia, [x]: !usata } }).catch(() => {}) } }) : el("span", { class: "eco" + (usata ? " usata" : ""), testo: t });
      })) : null,
      c.attacchi && c.attacchi.length ? el("section", { class: "cr-attacchi" },
        el("h2", { testo: "⚔ Attacchi" + (per > 1 ? " · " + usati + "/" + per + " in questo turno" : "") }),
        el("ul", {}, c.attacchi.map((a, i) => el("li", {},
          el("div", {}, el("b", { testo: a.nome }), el("span", { class: "tenue piccolo", testo: " " + sg(a.bonus) + " · " + testoDanni(a) + (a.portata ? " · " + a.portata : "") })),
          puoAttaccare ? el("button", { class: "primario piccolo", testo: "⚔", title: "Attacca con " + a.nome, "aria-label": "Attacca con " + a.nome, on: { click: () => A.apriAttacco(c, undefined, i) } }) : null))),
        !puoAttaccare && k.avviato && c.mio ? el("p", { class: "tenue piccolo", testo: "Gli attacchi si tirano nel tuo turno." }) : null)
        : c.tipo === "pg" && (c.mio || master()) ? el("p", { class: "tenue piccolo", testo: "La scheda non ha ancora mandato gli attacchi: apri la scheda completa una volta." }) : null,
      master() && c.tipo === "mostro" && c.incantesimi && c.incantesimi.gruppi && c.incantesimi.gruppi.length ? el("section", { class: "cr-attacchi" },
        el("h2", { testo: "✨ Incantesimi" }),
        el("p", { class: "piccolo tenue", testo: c.incantesimi.gruppi.map((g) => g.etichetta + (g.usi ? " " + Math.max(0, g.usi - ((c.incantesimi.usati || {})[c.incantesimi.gruppi.indexOf(g)] || 0)) + "/" + g.usi : "")).join(" · ") }),
        el("button", { class: "primario piccolo", testo: "✨ Lancia un incantesimo", on: { click: () => import("../ui/incantesimi_mostro.js").then((x) => x.apriIncantesimiMostro(sid, c)) } })) : null,
      puo ? el("div", { class: "riga-flex cr-pf-comandi" }, v,
        el("button", { class: "piccolo", testo: "− Danno", on: { click: () => { const n = val(); if (n != null) comando(pre + "danno", { combattente: c.id, valore: n }).catch(() => {}); } } }),
        el("button", { class: "piccolo", testo: "+ Cura", on: { click: () => { const n = val(); if (n != null) comando(pre + "cura", { combattente: c.id, valore: n }).catch(() => {}); } } })) : null,
      // Scheda intera della creatura (dal bestiario, solo il Master): tratti, azioni, sensi, azioni leggendarie (2026-11-12)
      master() && c.tipo === "mostro" ? schedaIntera(c) : null);
  }

  // la scheda intera si chiede una volta per mostro e resta in memoria mentre la finestra è aperta
  const schede = new Map();
  function schedaIntera(c) {
    const box = el("section", { class: "cr-scheda-intera" });
    const metti = (sc) => svuota(box, el("details", { open: true }, el("summary", {}, el("h2", { testo: "📖 Scheda della creatura" })),
      sc ? statistiche(sc) : el("p", { class: "tenue piccolo", testo: "Nessuna scheda: il mostro è stato aggiunto a mano, non dal bestiario." })));
    if (schede.has(c.id)) metti(schede.get(c.id));
    else fetch("/api/v1/sessioni/" + encodeURIComponent(sid) + "/combattenti/" + encodeURIComponent(c.id) + "/scheda", { credentials: "same-origin" })
      .then((r) => (r.ok ? r.json() : { scheda: null })).then((j) => { schede.set(c.id, j.scheda || null); metti(j.scheda || null); })
      .catch(() => metti(null));
    return box;
  }

  const smetti = [
    tempoReale.su("iniz.stato", (v) => { if (v.sessione !== sid) return; const prima = vista; vista = v; disegna(); fx.vista(prima, v); }),
    tempoReale.su("attacco.esito", (e) => { A.avvisoEsito(e); if (e && e.sessione === sid) fx.esito(e); }),
    tempoReale.su("iniz.turno", (d) => {
      if (d.sessione !== sid) return;
      avviso("⚔ È il tuo turno, " + d.nome + "!", "", 6000);
      if (navigator.vibrate) navigator.vibrate([200, 100, 200]);
    }),
  ];
  const riaperto = () => entra();
  tempoReale.stato.addEventListener("aperto", riaperto);
  if (!sid) { svuota(zona, el("p", { class: "errore", testo: "Manca la sessione." })); return () => smetti.forEach((f) => f()); }
  await entra();
  return () => { smetti.forEach((f) => f()); tempoReale.stato.removeEventListener("aperto", riaperto); };
}
