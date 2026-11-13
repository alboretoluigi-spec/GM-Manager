// Riassunti delle sessioni: ogni giocatore scrive o carica il suo testo; tutti i membri del gruppo lo leggono
// con la firma "Scriba: <nome>". Solo lo scriba lo modifica; lo scriba o il Master lo eliminano.
import { el, svuota, api, avviso, conferma, tempoReale, dialogo } from "../ui/ui.js";
import { estraiTesto, mostraTesto, ACCETTA_TESTI } from "../ui/testo_formattato.js";

const MAX = 50000;
const data = (s) => new Date(s * 1000).toLocaleDateString("it-IT", { day: "numeric", month: "long", year: "numeric" });

export async function mostra(corpo, ctx) {
  const box = el("div", { class: "largo riassunti" });
  const editor = el("div", {}), lista = el("div", {});
  svuota(corpo, box);
  svuota(box, editor, lista);
  const u = ctx.io.utente;
  let gruppi = [], tutti = [], filtro = "";
  try { filtro = sessionStorage.getItem("gasterbless.riassuntiGruppo") || ""; } catch { /* */ }

  async function ricarica() {
    if (!gruppi.length) gruppi = await api("/gruppi").catch(() => []);
    tutti = (await api("/riassunti")).lista;
    disegna();
  }

  function disegna() {
    if (filtro && !gruppi.find((g) => g.id === filtro)) filtro = "";
    const vis = tutti.filter((r) => !filtro || r.gruppo === filtro);
    svuota(lista,
      gruppi.length > 1 ? el("div", { class: "riga-flex" }, [{ id: "", nome: "Tutti i gruppi" }, ...gruppi].map((g) => el("button", { class: "piccolo" + (filtro === g.id ? " attivo" : ""), testo: g.nome,
        on: { click: () => { filtro = g.id; try { sessionStorage.setItem("gasterbless.riassuntiGruppo", filtro); } catch { /* */ } disegna(); } } }))) : null,
      vis.length ? vis.map((r) => el("article", { class: "scheda-ui riassunto", "data-id": r.id },
        el("h2", { testo: r.titolo || "Riassunto" }),
        el("p", { class: "tenue riassunto-meta", testo: [gruppi.length > 1 ? r.gruppoNome : "", r.sessioneTitolo, data(r.creato) + (r.modificato > r.creato + 60 ? " (modificato il " + data(r.modificato) + ")" : "")].filter(Boolean).join(" · ") }),
        r.ottimo ? el("p", { class: "riassunto-ottimo", testo: "⭐ Ottimo lavoro (segnato dal Master)" }) : null,
        el("div", { class: "riassunto-testo" }, mostraTesto(r.testo)),
        el("p", { class: "riassunto-firma", testo: "Scriba: " + r.scriba }),
        r.mio || u.ruolo === "master" ? el("div", { class: "riga-flex" },
          r.mio ? el("button", { class: "piccolo", testo: "✏ Modifica", on: { click: () => apriEditor(r) } }) : null,
          u.ruolo === "master" ? el("button", { class: "piccolo" + (r.ottimo ? " attivo" : ""), testo: r.ottimo ? "⭐ Ottimo lavoro ✓" : "⭐ Ottimo lavoro",
            title: r.ottimo ? "Togli il riconoscimento (l'ispirazione già data resta nella scheda)" : "Segnalo come ottimo: il personaggio che lo firma riceve l'ispirazione",
            on: { click: () => ottimo(r) } }) : null,
          u.ruolo === "master" ? el("button", { class: "piccolo", testo: "✒ Firma", title: "Cambia il nome con cui è firmato", on: { click: () => firma(r) } }) : null,
          el("button", { class: "piccolo pericolo", testo: "🗑 Elimina", on: { click: async () => {
            if (!(await conferma("Eliminare questo riassunto?", "«" + (r.titolo || "Riassunto") + "» di " + r.scriba + " sparisce per tutti.", "Elimina", true))) return;
            try { await api("/riassunti/" + r.id, { metodo: "DELETE" }); await ricarica(); } catch (e) { avviso(e.message, "errore"); }
          } } })) : null))
        : el("section", { class: "scheda-ui" }, el("p", { class: "tenue", testo: gruppi.length ? "Nessun riassunto. Premi «＋ Nuovo riassunto» per scrivere il primo: lo leggeranno tutti i giocatori del gruppo, con la tua firma." : "Non fai ancora parte di un gruppo." })));
  }

  async function ottimo(r) {
    try {
      const x = await api("/riassunti/" + r.id + "/ottimo", { dati: { ottimo: !r.ottimo } });
      avviso(x.ottimo ? "⭐ Segnato come Ottimo lavoro" + (x.ispirazione ? ": " + x.personaggio + " riceve l'ispirazione." : ".") : "Riconoscimento tolto.");
      await ricarica();
    } catch (e) { avviso(e.message, "errore"); }
  }
  function firma(r) {
    const campo = el("input", { maxlength: "80", valore: r.scriba });
    const err = el("p", { class: "errore" });
    dialogo("Firma del riassunto", el("div", {}, el("label", {}, "Firmato da", campo), err), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva", on: { click: async () => {
          try { await api("/riassunti/" + r.id + "/firma", { metodo: "PATCH", dati: { firma: campo.value } }); chiudi(); await ricarica(); } catch (e) { err.textContent = e.message; }
        } } })) });
  }

  async function apriEditor(r) {
    if (!gruppi.length) { avviso("Non fai ancora parte di un gruppo.", "errore"); return; }
    const gSel = el("select", { "aria-label": "Gruppo", disabled: !!r }, gruppi.map((g) => el("option", { value: g.id, testo: g.nome, selected: r ? r.gruppo === g.id : filtro === g.id })));
    const sSel = el("select", { "aria-label": "Sessione" });
    // Firma: il personaggio (di default quello attivo più recente del gruppo); il Master firma con il suo nome.
    const pgSel = el("select", { "aria-label": "Firmato da" });
    const pgs = u.ruolo === "master" ? [] : (await api("/personaggi").catch(() => [])).filter((p) => p.stato === "attivo" && p.proprietario === u.id);
    const disegnaPg = () => {
      const g = gSel.value;
      const l = pgs.slice().sort((a, b) => ((b.gruppo === g) - (a.gruppo === g)) || (b.modificato || 0) - (a.modificato || 0));
      svuota(pgSel, l.map((p) => el("option", { value: p.id, testo: p.nome, selected: r ? r.personaggio === p.id : false })), el("option", { value: "", testo: u.nome + " (il tuo nome)", selected: r ? !r.personaggio : !l.length }));
    };
    disegnaPg(); gSel.addEventListener("change", disegnaPg);
    const titolo = el("input", { maxlength: "120", placeholder: "Titolo (facoltativo), es. Sessione 3: l'arrivo a Borgofosco", valore: r ? r.titolo : "" });
    const testo = el("textarea", { placeholder: "Scrivi qui il riassunto, oppure caricalo da un file di testo o Word.", "aria-label": "Testo del riassunto" });
    testo.value = r ? r.testo : "";
    const conta = el("span", { class: "tenue" });
    const aggiornaConta = () => { conta.textContent = testo.value.length.toLocaleString("it-IT") + " / " + MAX.toLocaleString("it-IT") + " caratteri"; conta.classList.toggle("errore", testo.value.length > MAX); };
    testo.addEventListener("input", aggiornaConta); aggiornaConta();
    const file = el("input", { type: "file", accept: ACCETTA_TESTI, hidden: true });
    file.addEventListener("change", async () => {
      const f = file.files[0]; file.value = ""; if (!f) return;
      try {
        const t = await estraiTesto(f);
        if (testo.value.trim() && !(await conferma("Sostituire il testo?", "Il testo scritto finora viene sostituito da quello del file.", "Sostituisci"))) return;
        testo.value = t; if (!titolo.value) titolo.value = f.name.replace(/\.[^.]+$/, "").slice(0, 120); aggiornaConta();
      } catch (e) { avviso(e.message, "errore"); }
    });
    // Sessione giusta per un riassunto nuovo: quella in corso, altrimenti l'ultima già iniziata.
    const giusta = (l) => {
      const ora = Date.now() / 1000;
      const inCorso = l.find((s) => s.stato === "in_corso");
      if (inCorso) return inCorso.id;
      const passate = l.filter((s) => s.inizio <= ora).sort((a, b) => b.inizio - a.inizio);
      return passate.length ? passate[0].id : "";
    };
    async function sessioni() {
      const l = await api("/gruppi/" + gSel.value + "/sessioni").catch(() => []);
      svuota(sSel, el("option", { value: "", testo: "— nessuna sessione —" }),
        l.sort((a, b) => b.inizio - a.inizio).map((s) => el("option", { value: s.id, testo: s.titolo + " · " + data(s.inizio), selected: r ? r.sessione === s.id : s.id === giusta(l) })));
    }
    gSel.addEventListener("change", sessioni);
    await sessioni();
    const chiudi = () => svuota(editor);
    svuota(editor, el("section", { class: "scheda-ui riassunti-editor" },
      el("h2", { testo: r ? "✏ Modifica il riassunto" : "＋ Nuovo riassunto" }),
      el("div", { class: "riga-flex" }, gruppi.length > 1 ? el("label", { class: "riga" }, "Gruppo", gSel) : null, el("label", { class: "riga" }, "Sessione", sSel),
        u.ruolo !== "master" ? el("label", { class: "riga" }, "Firmato da", pgSel) : null),
      el("label", {}, "Titolo", titolo),
      el("label", {}, "Testo", testo),
      el("div", { class: "riga-flex" }, file,
        el("button", { testo: "📄 Carica da file (testo, Word, PDF)", on: { click: () => file.click() } }), conta, el("span", { class: "spazio" }),
        el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Pubblica", on: { click: async (e) => {
          if (testo.value.length > MAX) { avviso("Testo troppo lungo: massimo " + MAX + " caratteri.", "errore"); return; }
          e.target.disabled = true;
          const dati = { gruppo: gSel.value, sessione: sSel.value, titolo: titolo.value, testo: testo.value, personaggio: u.ruolo === "master" ? "" : pgSel.value };
          try {
            if (r) await api("/riassunti/" + r.id, { metodo: "PUT", dati }); else await api("/riassunti", { dati });
            avviso(r ? "Riassunto aggiornato." : "Riassunto pubblicato: lo leggono tutti i giocatori del gruppo."); chiudi(); await ricarica();
          } catch (er) { avviso(er.message, "errore"); e.target.disabled = false; }
        } } })),
      el("p", { class: "tenue", testo: "Lo leggeranno tutti i giocatori del gruppo e il Master, firmato con il nome scelto. Titoli (# Titolo), elenchi (- voce) e paragrafi del file restano: Gaster Bless li impagina nel suo stile. Solo tu potrai modificarlo." })));
    titolo.focus();
    editor.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  svuota(ctx.azioni, el("button", { class: "primario", testo: "＋ Nuovo riassunto", on: { click: () => apriEditor(null) } }));
  const via = tempoReale.su("riassunti.aggiornati", () => ricarica().catch(() => {}));
  try { await ricarica(); } catch (e) { svuota(lista, el("p", { class: "errore", testo: e.message })); }
  return () => via();
}
