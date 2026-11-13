// Mini scheda "Sanità mentale": un popup con solo ciò che riguarda la Sanità mentale del personaggio,
// separato dalla scheda (funziona con qualsiasi scheda, anche con un plugin).
// Mostra: valore (10 + modificatore di Saggezza, o quello scritto dal Master), tiro salvezza su Sanità
// mentale (il risultato lo vedono il giocatore e il Master), livello di Terrore, follia (pazzie attive)
// e condizioni attive legate a paura e follia. Gli strumenti del Master restano nella schermata completa.
import { el, svuota, api, avviso, quando, tempoReale } from "./ui.js";
import { apriSanita } from "./sanita.js";

const segno = (n) => (n >= 0 ? "+" : "") + n;
const VANT = { normale: "nessuno", vantaggio: "vantaggio", svantaggio: "svantaggio" };
let aperta = null; // { pg, ricarica, chiudi, mostraTiro }

/** Apre la mini scheda Sanità mentale di un personaggio (o la aggiorna, se è già aperta). */
export function apriMiniSanita(pg) {
  if (aperta && aperta.pg === pg) { aperta.ricarica(); return aperta; }
  if (aperta) aperta.chiudi();
  let v = null, errore = "", ultimoTiro = null, inCorso = false;
  const corpo = el("div", { class: "sanita-mini-corpo" }, el("p", { class: "tenue", testo: "Caricamento…" }));
  const chiudiBtn = el("button", { class: "sanita-chiudi", testo: "✕", "aria-label": "Chiudi la Sanità mentale", on: { click: () => chiudi() } });
  const finestra = el("div", { class: "sanita sanita-mini", role: "dialog", "aria-modal": "true", "aria-label": "Sanità mentale" }, chiudiBtn, corpo);
  const velo = el("div", { class: "sanita-velo", on: { click: (e) => { if (e.target === velo) chiudi(); } } }, finestra);
  document.body.append(velo);
  const esc = (e) => { if (e.key === "Escape" && !document.querySelector(".velo")) chiudi(); };
  document.addEventListener("keydown", esc);
  const vie = [
    tempoReale.su("sanita.stato", (d) => { if (d && d.personaggio === pg) ricarica(); }),
    tempoReale.su("sanita.config", () => ricarica()),
  ];
  function chiudi() { velo.remove(); document.removeEventListener("keydown", esc); vie.forEach((f) => f()); if (aperta && aperta.pg === pg) aperta = null; }

  async function ricarica() {
    try { v = await api("/sanita/pg/" + encodeURIComponent(pg)); errore = ""; } catch (e) { errore = e.message; }
    // non si ridisegna mentre si scrive la CD
    const a = document.activeElement;
    if (v && finestra.contains(a) && a.tagName === "INPUT" && a.type === "number") return;
    disegna();
  }

  function disegna() {
    if (errore && !v) { svuota(corpo, el("p", { class: "errore", testo: errore })); return; }
    if (!v) return;
    const c = v.config, mio = v.personaggio.mio, master = v.master;
    if (!ultimoTiro) {
      const s = (v.storico || []).find((x) => x.azione === "tiro_sanita" && x.dati && x.dati.ts);
      if (s) ultimoTiro = { ts: s.dati.ts, da: s.chi, quando: s.quando };
    }
    const pazzie = v.pazzie.filter((p) => p.attiva);
    const condizioni = [...v.condizioni.map((x) => ({ t: x, nota: "dal Terrore o dalla paura" })),
      ...pazzie.map((p) => ({ t: "Pazzia " + p.tipo + (p.categoria ? " · " + p.categoria : ""), nota: "durata " + p.durata }))];
    const origine = v.metodo === "MANUAL" ? "valore scritto dal Master" : v.mod != null ? "10 + modificatore di Saggezza" : "";
    svuota(corpo,
      el("header", { class: "sanita-testa" }, el("h2", { testo: "🧠 Sanità mentale · " + v.personaggio.nome })),
      !c.attiva ? el("p", { class: "tenue", testo: "La Sanità mentale non è attiva in questa campagna." }) : null,
      el("div", { class: "sanita-valori" },
        valore("Sanità mentale", v.punteggio ?? "—", origine, "sanita-mini-valore"),
        valore("Modificatore", v.mod != null ? segno(v.mod) : "—"),
        valore("Tiro salvezza", v.bonusTS != null ? segno(v.bonusTS) : "—",
          (v.competente ? "competente (+" + v.prof + ")" : "senza competenza") + (v.bonusTemp ? " · bonus " + segno(v.bonusTemp) : ""))),
      v.punteggio == null ? el("p", { class: "errore", testo: "Manca la Saggezza: apri una volta la scheda del personaggio (pulsante Schede), poi torna qui." }) : null,
      v.evento && v.evento.stato === "READY" ? el("div", { class: "sanita-evento" },
        el("b", { testo: "😱 Evento di paura: " + v.evento.fonte }),
        el("p", { class: "tenue", testo: mio ? "Il Master ti chiede il tiro: aprilo dalla schermata della paura." : "In attesa del tiro del giocatore." }),
        el("button", { class: "primario", testo: mio ? "Soccombi alla paura…" : "Apri la schermata della paura", on: { click: () => { chiudi(); apriSanita(pg, {}); } } })) : null,
      sezioneTiro(mio || master, c.attiva && v.punteggio != null),
      el("section", { class: "sanita-sez" }, el("h3", {}, "😱 Terrore ", el("span", { class: "etichetta", testo: "homebrew" })),
        c.terrore ? [
          el("div", { class: "sanita-scala", role: "meter", "aria-valuemin": "0", "aria-valuemax": "7", "aria-valuenow": String(v.terrore), "aria-label": "Terrore " + v.terrore + ": " + v.livello.nome },
            v.scala.map((l) => el("span", { class: "t" + (l.livello <= v.terrore && l.livello > 0 ? " on" : "") + (l.livello === v.terrore ? " qui" : ""), title: l.livello + " · " + l.nome, testo: String(l.livello) }))),
          el("p", { class: "sanita-mini-livello" }, el("b", { testo: "Livello " + v.terrore + " · " + v.livello.nome })),
          v.effetti.length ? el("ul", { class: "sanita-effetti" }, v.effetti.map((l) => el("li", {}, el("b", { testo: l.livello + " " + l.nome + ": " }), l.effetto)))
            : el("p", { class: "tenue", testo: "Nessun effetto di Terrore." }),
          tiriTerrore(mio || master)]
          : el("p", { class: "tenue", testo: "La scala del Terrore non è attiva in questa campagna." })),
      el("section", { class: "sanita-sez" }, el("h3", { testo: "🌀 Follia" }),
        !c.pazzia ? el("p", { class: "tenue", testo: "La gestione delle pazzie non è attiva in questa campagna." })
          : pazzie.length ? pazzie.map((p) => el("p", { class: "sanita-mini-pazzia" },
            el("b", { testo: "Pazzia " + p.tipo + (p.categoria ? " · " + p.categoria : "") + ": " }),
            p.descrizione || (master ? "(testo da scrivere)" : "il Master ti descriverà l'effetto."),
            el("span", { class: "tenue", testo: " · durata " + p.durata + (p.scadeIl ? " (fino alle " + new Date(p.scadeIl * 1000).toLocaleTimeString("it-IT", { hour: "2-digit", minute: "2-digit" }) + ")" : "") })))
            : el("p", { class: "tenue", testo: "Nessuna pazzia attiva." })),
      el("section", { class: "sanita-sez" }, el("h3", { testo: "⚠ Condizioni attive (paura e follia)" }),
        condizioni.length ? el("ul", { class: "sanita-mini-condizioni" }, condizioni.map((x) => el("li", {}, el("b", { testo: x.t }), el("span", { class: "tenue", testo: " · " + x.nota }))))
          : el("p", { class: "tenue", testo: "Nessuna condizione legata a paura o follia." })),
      master ? el("div", { class: "riga-flex sanita-mini-piede" },
        el("button", { testo: "🛠 Strumenti del Master…", title: "Eventi di paura, Terrore, pazzie, competenza e storico", on: { click: () => { chiudi(); apriSanita(pg, {}); } } })) : null);
  }

  function valore(t, x, nota, classe) {
    return el("div", { class: "sanita-valore" + (classe ? " " + classe : "") }, el("span", { class: "tenue", testo: t }), el("b", { testo: String(x) }), nota ? el("small", { class: "tenue", testo: nota }) : null);
  }

  // Tiro salvezza su Sanità mentale: CD e vantaggio facoltativi (li dice il Master).
  function sezioneTiro(puo, possibile) {
    const cd = el("input", { type: "number", min: "1", max: "40", placeholder: "—", class: "corto", "aria-label": "CD (facoltativa)" });
    const va = el("select", { "aria-label": "Vantaggio" }, Object.entries(VANT).map(([k, t]) => el("option", { value: k, testo: t })));
    const tira = el("button", { class: "primario", testo: "🎲 Tiro salvezza su Sanità mentale", disabled: !possibile || inCorso, on: { click: async () => {
      if (inCorso) return;
      inCorso = true; tira.disabled = true;
      try {
        const r = await api("/sanita/pg/" + encodeURIComponent(pg) + "/tiro", { dati: { tipo: "sanita", cd: +cd.value || 0, vantaggio: va.value } });
        ultimoTiro = { ts: r.ts, da: r.da, quando: Math.floor(Date.now() / 1000), automatiche: r.automatiche };
      } catch (e) { avviso(e.message, "errore"); }
      inCorso = false;
      await ricarica();
    } } });
    return el("section", { class: "sanita-sez" }, el("h3", { testo: "🎲 Tiro salvezza su Sanità mentale" }),
      puo ? el("div", { class: "riga-flex" }, el("label", {}, "CD ", cd), el("label", {}, "Vantaggio ", va), tira) : null,
      el("p", { class: "tenue piccolo", testo: "d20 + modificatore di Sanità mentale (+ competenza, se il Master la concede). Il risultato lo vedono il giocatore e il Master." }),
      ultimoTiro ? esitoTiro(ultimoTiro) : null);
  }

  function esitoTiro({ ts: t, da, quando: q, automatiche }) {
    const conCD = t.cd > 0;
    return el("div", { class: "sanita-esito sanita-mini-esito " + (conCD ? (t.successo ? "ok" : "ko") : ""), "aria-live": "polite" },
      el("p", {}, "Ultimo tiro: ", el("b", { class: "sanita-mini-totale", testo: String(t.totale) }),
        conCD ? [" contro CD ", el("b", { testo: String(t.cd) }), " · ", el("b", { class: "sanita-verdetto-piccolo", testo: t.successo ? "SUPERATO ✔" : "FALLITO ✖" })] : null),
      el("p", { class: "tenue piccolo", testo: "d20 " + t.d20.join(" e ") + (t.d20.length > 1 ? " (" + t.vantaggio + ": tenuto " + t.scelto + ")" : "") +
        " · Sanità " + segno(t.modCaratteristica) + (t.competenza ? " · competenza +" + t.competenza : "") + (t.bonus ? " · bonus " + segno(t.bonus) : "") +
        (da ? " · tirato da " + da : "") + (q ? " · " + quando(q) : "") }),
      automatiche && automatiche.length ? el("ul", { class: "sanita-auto" }, automatiche.map((x) => el("li", { testo: "⚙ " + x }))) : null);
  }

  // Tiri chiesti dai livelli di Terrore 4–5 (Barcollante) e 7 (Svenuto).
  function tiriTerrore(puo) {
    if (!puo) return null;
    const b = [];
    if (v.terrore >= 4 && v.terrore <= 5) b.push(el("button", { class: "piccolo", testo: "TS di Saggezza CD " + v.config.cdBarcollante + " (inizio del turno)", on: { click: () => tiroTerrore("barcollante") } }));
    if (v.terrore === 7) b.push(el("button", { class: "piccolo", testo: "TS di Costituzione CD " + v.config.cdSvenuto + " (fine del turno)", on: { click: () => tiroTerrore("svenuto") } }));
    return b.length ? el("div", { class: "riga-flex" }, b) : null;
  }
  async function tiroTerrore(tipo) {
    try { await api("/sanita/pg/" + encodeURIComponent(pg) + "/tiro", { dati: { tipo } }); } catch (e) { avviso(e.message, "errore"); }
  }

  // Un tiro fatto da un altro (il Master per il giocatore o viceversa) arriva in tempo reale.
  function mostraTiro(d) {
    if (d.esito && d.esito.tipo === "sanita" && d.esito.ts) { ultimoTiro = { ts: d.esito.ts, da: d.da, quando: Math.floor(Date.now() / 1000), automatiche: d.esito.automatiche }; disegna(); }
  }

  aperta = { pg, ricarica, chiudi, mostraTiro };
  ricarica();
  return aperta;
}

/** Avvisi del tiro salvezza su Sanità mentale: arrivano solo al Master e al giocatore del personaggio. */
let installata = false;
export function installaMiniSanita() {
  if (installata) return;
  installata = true;
  tempoReale.su("sanita.tiro", (d) => {
    if (!d || !d.esito || d.esito.tipo !== "sanita" || !d.esito.ts) return;
    if (aperta && aperta.pg === d.personaggio) { aperta.mostraTiro(d); return; }
    const t = d.esito.ts;
    avviso("🧠 " + (d.nome || "") + " · TS Sanità mentale: " + t.totale + (t.cd > 0 ? " contro CD " + t.cd + (t.successo ? " · SUPERATO" : " · FALLITO") : "") + (d.da ? " (tirato da " + d.da + ")" : ""), "", 7000);
    (d.esito.automatiche || []).forEach((x) => avviso("⚙ " + (d.nome || "") + ": " + x, "", 8000));
  });
}
