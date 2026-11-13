// Attacchi automatici (internal/server/attacchi.go): finestra dell'attacco, finestra dell'esito con la scelta del critico,
// avvisi per gli attacchi degli altri. La usano il tracker (web/sezioni/iniziativa.js) e la scheda ridotta in finestra
// (web/sezioni/combattente.js).
//   const A = creaAttacchi({ sessione: () => id, vista: () => vista, master: () => bool });
//   A.apriAttacco(combattente, idBersaglio?, indiceAttacco?); tempoReale.su("attacco.esito", A.avvisoEsito)
import { el, svuota, avviso, dialogo, conferma, tempoReale } from "./ui.js";
import { segnalaMira } from "./mira.js";
import { lancioAutomatico } from "./dadi.js";

const suono = new Audio("/suoni/corvo.mp3");

export const sg = (n) => (n >= 0 ? "+" : "−") + Math.abs(n);
export const testoDanni = (a) => a.danni.length ? a.danni.map((d) => (d.dadi || "") + (d.mod ? (d.dadi ? sg(d.mod) : String(d.mod)) : "") + (d.tipo ? " " + d.tipo : "")).join(" + ") : "nessun danno (effetto a mano)";

/* Dadi D2 (2026-11-12): nella conferma il numero dei dadi dei danni si può cambiare (un dado in più per un'arma a due mani, un
   privilegio…); poi i dadi 3D mostrano i valori tirati dal server prima dell'esito. */
export function editorDadi(danni, titolo = "Dadi dei danni") {
  const righe = [];
  const voci = (danni || []).map((d, i) => {
    const m = /^(\d+)d(\d+)$/.exec(d.dadi || "");
    if (!m) return null;
    const n = el("input", { type: "number", min: "1", max: "20", valore: m[1], class: "num-piccolo", "aria-label": "Numero di d" + m[2] + (d.tipo ? " " + d.tipo : "") });
    righe.push({ i, n, orig: +m[1], facce: +m[2] });
    return el("label", { class: "riga dadi-conf" }, n, el("span", { testo: "d" + m[2] + (d.mod ? " " + sg(d.mod) : "") + (d.tipo ? " " + d.tipo : "") }));
  }).filter(Boolean);
  const quanti = (r) => Math.max(1, Math.min(20, parseInt(r.n.value, 10) || r.orig));
  const riga = (i) => righe.find((x) => x.i === i);
  return {
    el: voci.length ? el("div", { class: "dadi-conferma", role: "group", "aria-label": titolo }, el("span", { class: "tenue piccolo", testo: titolo + " (puoi cambiare il numero):" }), voci) : null,
    cambiato: () => righe.some((r) => quanti(r) !== r.orig),
    conteggi: () => (danni || []).map((_, i) => (riga(i) ? quanti(riga(i)) : 0)),
    danni: () => (danni || []).map((d, i) => (riga(i) ? { ...d, dadi: quanti(riga(i)) + "d" + riga(i).facce } : d)),
  };
}

/** Dadi di un esito d'attacco per i dadi 3D: il d20 tenuto (l'altro scartato) e i dadi dei danni. */
export function dadiDiEsito(e, soloDanni = false) {
  const g = [];
  if (!soloDanni && e.d20 && e.d20.length) {
    const altri = e.d20.slice();
    const i = altri.indexOf(e.naturale);
    if (i >= 0) altri.splice(i, 1);
    g.push({ facce: 20, valori: [e.naturale], scarto: altri.length ? altri : undefined });
  }
  (e.danni || []).forEach((d) => { const m = /d(\d+)/.exec(d.espr || ""); if (m && d.dadi && d.dadi.length) g.push({ facce: +m[1], valori: d.dadi }); });
  return g;
}

/** Dadi 3D di un esito d'attacco, poi dopo() (la finestra dell'esito). */
export function lanciaEsito(e, dopo, soloDanni = false, mio = true) {
  if (!e || e.anonimo) { dopo(); return; }
  const esitoTxt = { mancato: "mancato", colpito: "colpito", critico: "colpo critico" }[e.risultato] || "";
  lancioAutomatico({ dadi: dadiDiEsito(e, soloDanni), totale: soloDanni ? e.totaleDanni : e.totale, etichetta: e.nomeAttaccante + " → " + e.nomeBersaglio + " · " + e.attacco,
    formula: soloDanni ? "danni del critico: " + e.totaleDanni : "d20 " + sg(e.bonus) + " = " + e.totale + (e.ca != null ? " contro CA " + e.ca : "") + " · " + esitoTxt +
      (e.danni && e.danni.length ? " · danni " + e.totaleDanni : ""),
    critico: !soloDanni && e.naturale === 20, fallimento: !soloDanni && e.naturale === 1, modo: e.modo,
    colore: e.colore || "", classe: soloDanni ? "" : e.classe || "", chi: e.nomeAttaccante, mio }, dopo);
}

// Contenuto dell'esito (finestra di chi attacca e avvisi per gli altri).
export function corpoEsito(e) {
  const tiro = e.d20.length > 1 ? "d20: " + e.d20.join(" e ") + " (" + e.modo + ": vale " + e.naturale + ")" : "d20: " + e.naturale;
  const righe = [el("p", { class: "att-tiro", testo: tiro + " " + sg(e.bonus) + " = " + e.totale + (e.ca != null ? " contro CA " + e.ca : "") }),
    e.fonti && e.fonti.length ? el("p", { class: "tenue piccolo att-fonti", testo: e.fonti.join(" · ") }) : null,
    el("p", { class: "att-esito att-" + e.risultato, testo: { mancato: "MANCATO", colpito: "COLPITO", critico: "💥 COLPO CRITICO!" }[e.risultato] })];
  if (e.danni && e.danni.length) {
    righe.push(el("ul", { class: "att-danni" }, e.danni.map((d) => el("li", {},
      el("span", { testo: d.espr + (d.dadi && d.dadi.length ? " [" + d.dadi.join(", ") + "]" : "") + " = " + d.tirato + (d.tipo ? " " + d.tipo : "") }),
      d.nota ? el("span", { class: "att-nota", testo: " → " + d.finale + " (" + d.nota + ")" }) : null))),
    el("p", { class: "att-totale", testo: "Danni: " + e.totaleDanni + (e.modoCritico ? " (critico, " + e.modoCritico + ")" : "") }));
    if (e.pfPrima != null && e.pfDopo != null) righe.push(el("p", { testo: e.nomeBersaglio + ": " + e.pfPrima + " → " + e.pfDopo + " PF" + (e.sconfitto ? " · a terra!" : "") }));
    else if (e.sconfitto) righe.push(el("p", { testo: e.nomeBersaglio + " è a terra!" }));
  }
  if (e.ritorsione && e.ritorsione.length) e.ritorsione.forEach((t) => righe.push(el("p", { class: "att-nota", testo: "🔥 " + t })));
  if (e.concentrazione) righe.push(el("p", { class: "att-nota", testo: "🧠 " + e.nomeBersaglio + " è concentrato: TS di Costituzione CD " + e.concentrazione + " per mantenere l'incantesimo." }));
  if (e.dadiCambiati) righe.push(el("p", { class: "tenue piccolo", testo: "🎲 Dadi dei danni scelti: " + e.dadiCambiati }));
  return righe;
}

export function creaAttacchi(ctx) {
  // Le azioni mandate da questa pagina: il loro esito arriva anche dal canale, ma qui si vede già nella finestra.
  const mieAzioni = new Set();
  const nuovaAzione = () => { const id = "a" + Date.now().toString(36) + Math.random().toString(36).slice(2, 10); mieAzioni.add(id); return id; };

  function apriAttacco(c, bersaglio, indice) {
    // Multiattacco con attacchi diversi (un morso e due artigli): si propone il prossimo attacco della sequenza
    const seq = c.multiattacco || [], fatti = (c.economia && c.economia.seq) || [];
    const prossimo = seq.find((x) => (fatti[x.attacco] || 0) < x.volte);
    if (indice === undefined && prossimo) indice = prossimo.attacco;
    const atk = el("select", { "aria-label": "Attacco" }, c.attacchi.map((a, i) => el("option", { value: i, selected: i === indice, testo: a.nome + " · " + sg(a.bonus) + " · " + testoDanni(a) })));
    const testoSeq = seq.length ? "Multiattacco: " + seq.map((x) => (c.attacchi[x.attacco] || {}).nome + " " + Math.min(fatti[x.attacco] || 0, x.volte) + "/" + x.volte).join(" · ") : "";
    // bersagli: prima gli avversari (PG contro mostri e viceversa) ancora in piedi
    const altri = ctx.vista().combattenti.filter((x) => x.id !== c.id);
    const inPiedi = (x) => x.pf != null ? x.pf > 0 : x.salute !== "a terra";
    altri.sort((x, y) => (y.tipo !== c.tipo) - (x.tipo !== c.tipo) || inPiedi(y) - inPiedi(x));
    const ber = el("select", { "aria-label": "Bersaglio" }, altri.map((x) => el("option", { value: x.id, selected: x.id === bersaglio,
      testo: x.nome + (x.ca != null ? " · CA " + x.ca : "") + (x.pf != null ? " · " + x.pf + " PF" : x.salute ? " · " + x.salute : "") + (x.nascosto ? " (nascosto)" : "") })));
    const modi = ["normale", "vantaggio", "svantaggio"].map((m) => ({ m, i: el("input", { type: "radio", name: "modo-att", value: m, selezionato: m === "normale" }) }));
    const extra = el("input", { type: "number", min: "-10", max: "10", valore: "0", class: "num-piccolo", title: "Per esempio Benedizione: tira il d4 e scrivi il risultato" });
    const err = el("p", { class: "errore" });
    // dadi dei danni dell'attacco scelto, con il numero modificabile (fase D2)
    const zonaDadi = el("div");
    let ed = null;
    const editor = () => { const a = c.attacchi[+atk.value]; ed = editorDadi(a ? a.danni : []); svuota(zonaDadi, ed.el); };
    atk.addEventListener("change", editor);
    editor();
    const per = c.attacchiPerAzione || 1, usati = Math.min(c.attacchiUsati || 0, per);
    if (!altri.length) { avviso("Non c'è nessun bersaglio nello scontro.", "errore"); return; }
    // turno di un mostro o PNG: «X prende di mira Y» sugli schermi, appena il Master sceglie il bersaglio
    if (ctx.master()) { segnalaMira(ctx.sessione(), c, [ber.value]); ber.addEventListener("change", () => segnalaMira(ctx.sessione(), c, [ber.value])); }
    dialogo("⚔ Attacco di " + c.nome, el("div", { class: "att-dialogo" },
      el("label", {}, "Attacco", atk), testoSeq ? el("p", { class: "piccolo att-multi", testo: "🐾 " + testoSeq }) : null, el("label", {}, "Bersaglio", ber),
      el("div", { class: "riga-flex att-modi", role: "radiogroup", "aria-label": "Tiro" }, modi.map(({ m, i }) => el("label", { class: "riga" }, i, el("span", { testo: m[0].toUpperCase() + m.slice(1) })))),
      el("label", { class: "riga" }, el("span", { testo: "Bonus extra al tiro per colpire" }), extra), zonaDadi,
      el("p", { class: "tenue piccolo", testo: "Attacchi in questo turno: " + usati + " di " + per + "." + (ctx.master() ? " Fuori dal turno del combattente l'attacco usa la sua reazione (attacco di opportunità)." : "") + " Il d20 e i danni li tira il server; resistenze e immunità del bersaglio e le condizioni (prono, paralizzato, avvelenato…) sono comprese." }), err),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "🎲 Tira l'attacco", on: { click: async (ev) => {
        const b = ev.currentTarget; b.disabled = true;
        const dati = { sessione: ctx.sessione(), combattente: c.id, bersaglio: ber.value, attacco: +atk.value, modo: modi.find((x) => x.i.checked).m, extra: parseInt(extra.value, 10) || 0, azione: nuovaAzione() };
        if (ed && ed.cambiato()) dati.dadiDanni = ed.conteggi();
        try {
          let r = await tempoReale.chiedi("attacco.tira", dati);
          if (r && r.serveConferma) {
            if (!(await conferma("Attacco in più?", r.serveConferma, "Attacca lo stesso"))) { b.disabled = false; return; }
            r = await tempoReale.chiedi("attacco.tira", { ...dati, forza: true });
          }
          chiudi(); lanciaEsito(r, () => mostraEsito(r, c)); // prima i dadi 3D, poi l'esito
        } catch (e) { err.textContent = e.message; b.disabled = false; }
      } } })) });
  }



  function mostraEsito(e, c) {
    if (!e || !e.d20) return;
    const zona = el("div", { class: "att-risultato" });
    const attesa = e.risultato === "critico" && e.gettone;
    let chiudiD = null;
    const disegnaE = (x) => {
      svuota(zona, corpoEsito(x));
      if (x.risultato === "critico" && x.gettone) {
        const scegli = async (modo, b) => {
          b.disabled = true;
          try { const r = await tempoReale.chiedi("attacco.critico", { sessione: ctx.sessione(), gettone: x.gettone, modo, azione: nuovaAzione() }); lanciaEsito(r, () => disegnaE(r), true); }
          catch (err) { avviso(err.message, "errore"); b.disabled = false; }
        };
        zona.append(el("p", { class: "piccolo", testo: "Come si calcolano i danni del critico?" }),
          el("div", { class: "riga-flex" },
            el("button", { class: "primario", testo: "🎲🎲 Tira due volte i dadi", title: "Regola del manuale: tutti i dadi dei danni si tirano due volte, il modificatore una volta", on: { click: (ev) => scegli("dadi", ev.currentTarget) } }),
            el("button", { testo: "✖2 Raddoppia il risultato", title: "Variante: si tirano i dadi una volta e il risultato si raddoppia (il modificatore no)", on: { click: (ev) => scegli("raddoppia", ev.currentTarget) } })));
      } else if (x.attacchiRimasti > 0 && c) {
        zona.append(el("div", { class: "riga-flex" }, el("button", { class: "primario", testo: "⚔ Attacco successivo (" + (x.attacchiRimasti === 1 ? "ne resta 1" : "ne restano " + x.attacchiRimasti) + ")", on: { click: () => {
          if (chiudiD) chiudiD();
          const ora = ctx.vista().combattenti.find((y) => y.id === c.id) || c;
          apriAttacco(ora, x.bersaglio);
        } } })));
      }
    };
    disegnaE(e);
    chiudiD = dialogo("⚔ " + e.nomeAttaccante + " → " + e.nomeBersaglio + " · " + e.attacco, zona);
    if (attesa) suonaCritico();
  }
  function suonaCritico() { try { suono.currentTime = 0; suono.play().catch(() => {}); } catch { /* audio non disponibile */ } }

  // Esito degli attacchi degli altri: un avviso per tutti i presenti (chi ha attaccato ha già la finestra).
  function avvisoEsito(e) {
    if (e.sessione !== ctx.sessione() || mieAzioni.has(e.azione)) return;
    // un mostro nascosto ha attaccato il tuo PG (internal/server/attacchi.go, anonimo): non sai chi né con cosa
    if (e.anonimo) {
      const tipi = [...new Set((e.danni || []).map((d) => d.tipo).filter(Boolean))].join(" e ");
      avviso(e.risultato === "mancato" ? "👁 Qualcosa che non vedi attacca " + e.nomeBersaglio + " e lo manca."
        : "👁 Qualcosa che non vedi colpisce " + e.nomeBersaglio + (e.risultato === "critico" ? " in pieno" : "") + (e.danni && e.danni.length ? ": " + e.totaleDanni + " danni" + (tipi ? " di tipo " + tipi : "") : "") + "!" +
          (e.sconfitto ? " " + e.nomeBersaglio + " è a terra!" : "") + (e.concentrazione ? " TS Cos CD " + e.concentrazione + " per la concentrazione." : ""), "errore", 9000);
      return;
    }
    const testo = e.risultato === "mancato" ? "⚔ " + e.nomeAttaccante + " manca " + e.nomeBersaglio + " (" + e.attacco + ")"
      : e.risultato === "critico" && !(e.danni && e.danni.length) ? "💥 COLPO CRITICO di " + e.nomeAttaccante + " su " + e.nomeBersaglio + "!"
      : "⚔ " + e.nomeAttaccante + " colpisce " + e.nomeBersaglio + (e.danni && e.danni.length ? ": " + e.totaleDanni + " danni" + (e.modoCritico ? " (critico)" : "") : "") + (e.sconfitto ? " · a terra!" : "") + (e.concentrazione ? " · " + e.nomeBersaglio + ": TS Cos CD " + e.concentrazione + " per la concentrazione" : "");
    // gli altri vedono i dadi 3D di chi ha attaccato, con il suo set, poi l'avviso (2026-11-13)
    lanciaEsito(e, () => avviso(testo, "", 6000), !!e.modoCritico, false);
  }

  return { apriAttacco, mostraEsito, avvisoEsito };
}
