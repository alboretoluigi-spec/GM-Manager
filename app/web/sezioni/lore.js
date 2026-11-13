// Sezione «🗝 Codice della campagna» (solo Master; richiesta dell'utente del 2026-11-12, fase L1). La lore della campagna
// in schede collegate (luoghi, PNG, fazioni, eventi, trame, oggetti, segreti, cronologia), con il testo del Master e quello
// per i giocatori e, per ogni gruppo, a che punto è. Importazione dal Markdown scritto con ChatGPT (con anteprima),
// esportazione nello stesso formato, testo da dare a ChatGPT. Server: internal/server/lore.go.
import { el, svuota, api, avviso, dialogo, conferma } from "../ui/ui.js";
import { trova, nomiDi, norma, riferimenti, riferimentiDi, dimenticaCodice, dimenticaRiferimenti } from "../ui/codice_link.js";
import { apriBilanciamento } from "../ui/bilanciamento.js";

const TIPI = {
  luogo: { t: "Luoghi", u: "Luogo", i: "🏰", stati: ["sentito nominare", "visitato", "distrutto o chiuso"] },
  png: { t: "PNG", u: "PNG", i: "🧑", stati: ["sentito nominare", "incontrato", "morto o uscito di scena"] },
  fazione: { t: "Fazioni", u: "Fazione", i: "⚜", stati: ["sentita nominare", "incontrata", "sciolta o sconfitta"] },
  evento: { t: "Eventi", u: "Evento", i: "⚡", stati: ["annunciato", "avvenuto", "concluso"] },
  trama: { t: "Trame", u: "Trama", i: "🧵", stati: ["avviata", "in corso", "conclusa"] },
  oggetto: { t: "Oggetti", u: "Oggetto", i: "🗡", stati: ["sentito nominare", "trovato", "perso o distrutto"] },
  segreto: { t: "Segreti", u: "Segreto", i: "🔒", stati: ["intuito", "scoperto", "risolto"] },
  cronologia: { t: "Cronologia", u: "Cronologia", i: "📅", stati: ["sentita raccontare", "conosciuta", "chiusa"] },
};
const STATI = ["conosciuto", "incontrato", "concluso"];
const etStato = (tipo, stato) => (TIPI[tipo] || TIPI.luogo).stati[STATI.indexOf(stato)] || "sconosciuto";

export const TESTO_CHATGPT = `Sei il mio archivista per una campagna di quinta edizione (l'avventura, con le mie modifiche). Trasforma il materiale che ti do in schede per il «Codice della campagna» della mia app Gaster Bless, rispettando ESATTAMENTE questo formato, perché l'app lo importa da sola.

FORMATO DI OGNI SCHEDA
# Tipo: Nome
Chiave: valore
Chiave: valore
Collegamenti: Nome scheda; Nome scheda
Etichette: stato, fonte, altre parole chiave

## Per il Master
Tutto ciò che sa il Master: segreti, intenzioni, verità nascoste, come condurre la scena.

## Per i giocatori
Solo ciò che i personaggi possono vedere, sentire o sapere. Niente segreti.

REGOLE
1. Tipo: uno tra Luogo, PNG, Fazione, Evento, Trama, Oggetto, Segreto, Cronologia. Una scheda per ogni elemento; un luogo dentro un altro (una taverna in un villaggio) ha la sua scheda.
2. Righe «Chiave: valore» subito sotto il titolo, poi UNA RIGA VUOTA. Chiavi utili: Luogo: Ruolo: Stato: (vivo, morto, non morto, distrutto…) Regione: Atmosfera: Quando: Dove: Proprietario: Fonte: (da quale documento viene). Una sola riga ciascuna. Per gli eventi, «Innesco:» dice cosa li fa scattare; l'app li ricorda da sola se è scritto in una di queste forme: «quando il gruppo arriva a <nome di una località della mappa>», «alla prossima sessione», «dal giorno <numero>».
3. Collegamenti: i nomi ESATTI di altre schede, separati da punto e virgola. Usa sempre lo stesso nome per la stessa cosa (stesse maiuscole e accenti). Se una persona o un luogo ha più nomi (un nome vero e uno usato in pubblico, un titolo), fai UNA sola scheda con il nome principale, scrivi gli altri nella riga «Alias:» e nei collegamenti usa solo il nome principale.
4. Etichette, separate da virgole, sempre con:
   - lo STATO: «approvato» (fatto stabilito), «proposta» (idea non ancora approvata) oppure «da decidere» (non stabilito);
   - la FONTE: «canone» (dal modulo ufficiale), «romanzi» (dai romanzi di Corvonero) oppure «campagna» (decisione nostra); se cambia un fatto del modulo aggiungi «variante» e la riga «Variante di: <nome della scheda canonica>».
5. Non inventare: se un dettaglio non è stabilito, non trasformarlo in un fatto. Raccogli i punti aperti di un luogo o di una trama in una scheda «# Segreto: Decisioni aperte – <nome>» con l'etichetta «da decidere».
6. Le proposte restano proposte: sezione o scheda con l'etichetta «proposta».
7. Frasi d'esempio di un PNG: in «## Per il Master», sotto «Frasi d'esempio:», una per riga tra caporali.
8. Statistiche di un PNG o di un mostro: in una sezione «## Statistiche», nel formato del blocco statistiche (CA, PF, Velocità, le sei caratteristiche, TS, Resistenze, Immunità alle condizioni, Tratti, Azioni). Ogni attacco così: «Nome. Attacco con arma da mischia: +7 al tiro per colpire, portata 1,5 m, un bersaglio. Colpito: 8 (1d8 + 4) danni taglienti più 4 (1d8) danni necrotici.» Multiattacco così: «Effettua tre attacchi: due con la spada lunga e uno con il morso.»
9. Niente testo copiato dai manuali ufficiali: riassumi con parole tue.
10. Se ti do il file esportato dall'app, rimandami solo le schede nuove o cambiate, con lo stesso identico titolo.
11. Rispondi solo con le schede, tutte in un unico blocco di codice, senza introduzioni né commenti.

ESEMPIO
# PNG: Aldric Borinovich
Ruolo: ex erede del borgomastro, ora progenie vampirica
Luogo: Chiesa del Villaggio di Nebbiavalle
Stato: non morto
Collegamenti: Villaggio di Nebbiavalle; Elara Voss; Simbolo Sacro del Corvo
Etichette: approvato, campagna, variante
Variante di: Aldric Borinovich (modulo)

## Per il Master
Vive nascosto nella chiesa in rovina e rifiuta la propria natura. La memoria è frammentata: a volte non riconosce chi ha davanti.
Frasi d'esempio:
«Andatevene. Non c'è nulla qui per voi.»

## Per i giocatori
Una figura sfregiata che vi osserva dall'ombra della chiesa e vi chiede di andarvene.`;

export async function mostra(corpo, { io, azioni }) {
  if (!io || !io.utente || io.utente.ruolo !== "master") { svuota(corpo, el("p", { class: "errore", testo: "Questa sezione è solo per il Master." })); return null; }
  let dati = { schede: [], gruppi: [] };
  const filtro = { testo: "", tipo: "", gruppo: "" };
  const aperte = new Set();
  const lista = el("div", { class: "lore-lista" });

  async function ricarica() {
    dimenticaCodice();
    try { dati = await api("/lore"); } catch (e) { svuota(lista, el("p", { class: "errore", testo: e.message })); return; }
    if (!dati.gruppi.some((g) => g.id === filtro.gruppo)) filtro.gruppo = dati.gruppi.length ? dati.gruppi[0].id : "";
    disegna();
  }
  const perNome = (n) => trova(dati.schede, n); // anche per alias («Madre Maren» → Lysenne Aldaris)
  let rif = { voci: [], pg: [], oggetti: [], luoghi: [] }; // ciò che c'è altrove in Gaster Bless (codice_link.js)
  let modelliBil = []; // avventure con il bilanciamento (bilanciamento.js): il pulsante compare sulle loro schede
  api("/bilanciamento").then((l) => { modelliBil = l || []; }).catch(() => {});
  const modelloDi = (x) => modelliBil.find((m) => (m.schede || []).some((n) => nomiDi(x).some((y) => norma(y) === norma(n))));
  const bilanciamento = (modello) => apriBilanciamento(modello, { gruppi: dati.gruppi, gruppo: filtro.gruppo });
  riferimenti().then((r) => { rif = r; lista.querySelectorAll(".lore-altrove").forEach((d) => d.dispatchEvent(new Event("riempi"))); });

  function corrisponde(x) {
    if (filtro.tipo && x.tipo !== filtro.tipo) return false;
    const q = filtro.testo.trim().toLowerCase();
    if (!q) return true;
    return [x.nome, x.testoMaster, x.testoGiocatori, ...(x.etichette || []), ...(x.collegamenti || []), ...Object.values(x.campi || {})].some((v) => String(v || "").toLowerCase().includes(q));
  }

  function scheda(x) {
    const T = TIPI[x.tipo];
    const st = filtro.gruppo && x.stati && x.stati[filtro.gruppo];
    const det = el("details", { class: "scheda-ui lore-scheda", "data-lore": x.id, open: aperte.has(x.id) },
      el("summary", {}, el("span", { class: "lore-ico", testo: T.i }), el("b", { testo: x.nome }),
        (x.etichette || []).map((e) => el("span", { class: "etichetta", testo: e })),
        filtro.gruppo ? el("span", { class: "lore-stato" + (st ? " " + st.stato : ""), testo: st ? etStato(x.tipo, st.stato) : "sconosciuto" }) : null));
    det.addEventListener("toggle", () => { if (det.open) { aperte.add(x.id); if (!det.dataset.pieno) riempi(); } else aperte.delete(x.id); });
    const riempi = () => {
      det.dataset.pieno = "1";
      const campi = Object.entries(x.campi || {});
      det.append(...[
        campi.length ? el("dl", { class: "lore-campi" }, campi.map(([k, v]) => [el("dt", { testo: k }), el("dd", { testo: v })])) : null,
        (x.collegamenti || []).length ? el("p", { class: "lore-coll" }, "Collegamenti: ", x.collegamenti.map((n) => {
          const y = perNome(n);
          if (y) return el("button", { class: "piccolo", testo: TIPI[y.tipo].i + " " + y.nome + (norma(y.nome) !== norma(n) ? " (" + n + ")" : ""), on: { click: () => vaiA(y) } });
          const pg = rif.pg.find((p) => norma(p.nome) === norma(n)); // un personaggio dei giocatori: il suo background
          return pg ? el("a", { class: "pulsante piccolo", href: "/background?pg=" + encodeURIComponent(pg.id), "data-interno": true, testo: "📔 " + pg.nome, title: "Personaggio di un giocatore: apri il background" })
            : el("span", { class: "etichetta tenue", title: "Nessuna scheda con questo nome", testo: n });
        })) : null,
        innesco(x),
        citataDa(x),
        altrove(x),
        x.testoMaster ? el("div", { class: "lore-testo" }, el("h4", { testo: "🔒 Per il Master" }), el("div", { class: "lore-corpo", testo: x.testoMaster })) : null,
        x.testoGiocatori ? el("div", { class: "lore-testo giocatori" }, el("h4", { testo: "👥 Per i giocatori" }), el("div", { class: "lore-corpo", testo: x.testoGiocatori })) : null,
        dati.gruppi.length ? el("div", { class: "lore-gruppi" }, el("h4", { testo: "A che punto sono i gruppi" }), dati.gruppi.map((g) => statoGruppo(x, g))) : null,
        el("div", { class: "riga-flex" },
          modelloDi(x) ? el("button", { class: "piccolo", testo: "⚖ Bilanciamento", title: "Calcola il bilanciamento di «" + modelloDi(x).nome + "» per un gruppo", on: { click: () => bilanciamento(modelloDi(x).id) } }) : null,
          el("button", { class: "piccolo", testo: "✏ Modifica", on: { click: () => editor(x) } }),
          el("button", { class: "piccolo pericolo", testo: "Elimina", on: { click: async () => {
            if (!(await conferma("Eliminare «" + x.nome + "»?", "La scheda e lo stato dei gruppi spariscono dal Codice della campagna.", "Elimina", true))) return;
            try { await api("/lore/" + encodeURIComponent(x.id), { metodo: "DELETE" }); avviso("Scheda eliminata."); ricarica(); } catch (e) { avviso(e.message, "errore"); }
          } } }))].filter(Boolean)); // DOM append: i null diventerebbero la scritta «null»
    };
    if (det.open) riempi();
    return det;
  }

  // «Innesco»: come lo ha capito Gaster Bless (lore_promemoria.go) o, se non automatico, che resta da controllare a mano
  function innesco(x) {
    const testo = Object.entries(x.campi || {}).find(([k]) => /^innesco$/i.test(k.trim()));
    if (!testo) return null;
    return x.innesco ? el("p", { class: "lore-innesco ok", testo: "⏰ Gaster Bless te lo ricorderà " + x.innesco.testo + ", per ogni gruppo." })
      : el("p", { class: "lore-innesco", testo: "⏰ Innesco non automatico: lo controlli tu. Gaster Bless capisce l'arrivo in una località della mappa («quando il gruppo arriva a Borgofosco»), l'inizio della sessione («alla prossima sessione») e il giorno di gioco («dal giorno 5»)." });
  }

  // Promemoria aperti: eventi scattati per un gruppo, da segnare avvenuti o da ignorare
  const riquadroPromemoria = el("section", { class: "scheda-ui lore-promemoria" });
  function disegnaPromemoria() {
    const l = dati.promemoria || [];
    if (!l.length) { svuota(riquadroPromemoria); riquadroPromemoria.hidden = true; return; }
    riquadroPromemoria.hidden = false;
    const azione = async (p, a) => {
      try { await api("/lore/" + encodeURIComponent(p.scheda) + "/promemoria", { metodo: "POST", dati: { gruppo: p.gruppo, azione: a } }); ricarica(); }
      catch (e) { avviso(e.message, "errore"); }
    };
    svuota(riquadroPromemoria, el("h2", { testo: "🔔 Promemoria (" + l.length + ")" }),
      l.map((p) => el("div", { class: "riga-flex lore-prom" },
        el("span", { class: "etichetta", testo: p.nomeGruppo }),
        el("b", { testo: (TIPI[p.tipo] || {}).i + " " + p.nome }),
        el("span", { class: "tenue piccolo", testo: p.motivo }),
        el("span", { class: "spazio" }),
        el("button", { class: "piccolo", testo: "Apri", on: { click: () => { const y = dati.schede.find((x) => x.id === p.scheda); if (y) vaiA(y); } } }),
        el("button", { class: "piccolo primario", testo: "✓ Avvenuto", title: "Per questo gruppo la scheda diventa «incontrato/avvenuto»", on: { click: () => azione(p, "avvenuto") } }),
        el("button", { class: "piccolo", testo: "Ignora", title: "Chiude il promemoria senza cambiare lo stato", on: { click: () => azione(p, "ignora") } }))));
  }

  // le schede che collegano questa (anche con un suo alias)
  function citataDa(x) {
    const nomi = nomiDi(x).map(norma);
    const l = dati.schede.filter((y) => y.id !== x.id && (y.collegamenti || []).some((c) => nomi.includes(norma(c))));
    return l.length ? el("p", { class: "lore-coll lore-citata" }, "Citata da: ", l.map((y) => el("button", { class: "piccolo", testo: TIPI[y.tipo].i + " " + y.nome, on: { click: () => vaiA(y) } }))) : null;
  }
  // «Altrove in Gaster Bless»: PNG e mostri, background, oggetti magici, mappa (si riempie quando i riferimenti sono pronti)
  function altrove(x) {
    const box = el("p", { class: "lore-coll lore-altrove" });
    const riempi = () => {
      const r = riferimentiDi(x, rif);
      svuota(box, r.length ? ["Altrove in Gaster Bless: ", r.map((v) => el("a", { class: "pulsante piccolo", href: v.href, "data-interno": true, testo: v.testo }))] : null);
    };
    box.addEventListener("riempi", riempi);
    riempi();
    return box;
  }

  function statoGruppo(x, g) {
    const s = (x.stati || {})[g.id] || {};
    const sel = el("select", { "aria-label": "Stato per " + g.nome },
      el("option", { value: "", testo: "sconosciuto", selected: !s.stato }),
      STATI.map((v) => el("option", { value: v, selected: s.stato === v, testo: etStato(x.tipo, v) })));
    const nota = el("input", { type: "text", maxlength: "500", placeholder: "nota (facoltativa)", valore: s.nota || "", "aria-label": "Nota per " + g.nome });
    const salva = async () => {
      try { await api("/lore/" + encodeURIComponent(x.id) + "/stato", { metodo: "PUT", dati: { gruppo: g.id, stato: sel.value, nota: nota.value } }); avviso("Salvato per " + g.nome + "."); ricarica(); }
      catch (e) { avviso(e.message, "errore"); }
    };
    sel.addEventListener("change", salva);
    nota.addEventListener("change", salva);
    return el("div", { class: "riga-flex lore-gruppo" }, el("span", { class: "lore-gnome", testo: g.nome }), sel, nota);
  }

  function vaiA(y) {
    filtro.testo = ""; filtro.tipo = ""; aperte.add(y.id);
    disegna();
    const d = lista.querySelector('[data-lore="' + CSS.escape(y.id) + '"]');
    if (d) { d.scrollIntoView({ behavior: "smooth", block: "start" }); d.classList.add("lore-evidenza"); setTimeout(() => d.classList.remove("lore-evidenza"), 1600); }
  }

  function disegna() {
    disegnaPromemoria();
    const visibili = dati.schede.filter(corrisponde);
    if (!dati.schede.length) {
      svuota(lista, el("section", { class: "scheda-ui" }, el("h2", { testo: "Il Codice della campagna è vuoto" }),
        el("p", { testo: "Qui raccogli la lore della tua campagna: luoghi, PNG, fazioni, eventi, trame, oggetti, segreti e cronologia, ognuno con ciò che sai tu e ciò che possono sapere i giocatori, e a che punto è ogni gruppo." }),
        el("p", { testo: "Il modo più veloce: «🤖 Testo per ChatGPT», copia il testo in ChatGPT insieme al tuo materiale, poi incolla la risposta in «📥 Importa»." })));
      return;
    }
    svuota(lista, !visibili.length ? el("p", { class: "tenue", testo: "Nessuna scheda corrisponde alla ricerca." }) : null,
      Object.keys(TIPI).map((k) => {
        const l = visibili.filter((x) => x.tipo === k);
        return l.length ? el("section", { class: "lore-gruppo-tipo" }, el("h2", { testo: TIPI[k].i + " " + TIPI[k].t + " (" + l.length + ")" }), l.map(scheda)) : null;
      }));
  }

  function editor(x) {
    const nuova = !x;
    x = x || { tipo: filtro.tipo || "png", nome: "", testoMaster: "", testoGiocatori: "", campi: {}, collegamenti: [], etichette: [] };
    const tipo = el("select", { "aria-label": "Tipo" }, Object.entries(TIPI).map(([k, T]) => el("option", { value: k, selected: k === x.tipo, testo: T.i + " " + T.u })));
    const nome = el("input", { type: "text", maxlength: "100", valore: x.nome, "aria-label": "Nome" });
    const campi = el("textarea", { rows: "3", "aria-label": "Dati brevi", placeholder: "Luogo: Borgofosco\nRuolo: borgomastro" });
    campi.value = Object.entries(x.campi || {}).map(([k, v]) => k + ": " + v).join("\n");
    const coll = el("input", { type: "text", valore: (x.collegamenti || []).join("; "), "aria-label": "Collegamenti", placeholder: "Nomi di altre schede, separati da ;" });
    const et = el("input", { type: "text", valore: (x.etichette || []).join(", "), "aria-label": "Etichette", placeholder: "alleato, nobile" });
    const tm = el("textarea", { rows: "8", "aria-label": "Per il Master" }); tm.value = x.testoMaster || "";
    const tg = el("textarea", { rows: "5", "aria-label": "Per i giocatori" }); tg.value = x.testoGiocatori || "";
    const err = el("p", { class: "errore" });
    dialogo(nuova ? "Nuova scheda" : "Modifica «" + x.nome + "»", el("div", { class: "lore-editor" },
      el("label", {}, "Tipo", tipo), el("label", {}, "Nome", nome),
      el("label", {}, "Dati brevi (una riga «Chiave: valore» per ognuno)", campi),
      el("label", {}, "Collegamenti (nomi di altre schede, separati da ;)", coll), el("label", {}, "Etichette (separate da virgole)", et),
      el("label", {}, "🔒 Per il Master (segreti compresi)", tm), el("label", {}, "👥 Per i giocatori", tg), err),
    { largo: true, azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Salva", on: { click: async (ev) => {
        const b = ev.currentTarget; b.disabled = true;
        const c = {};
        campi.value.split("\n").forEach((r) => { const i = r.indexOf(":"); if (i > 0) c[r.slice(0, i).trim()] = r.slice(i + 1).trim(); });
        const d = { tipo: tipo.value, nome: nome.value, campi: c, testoMaster: tm.value, testoGiocatori: tg.value,
          collegamenti: coll.value.split(";").map((v) => v.trim()).filter(Boolean), etichette: et.value.split(",").map((v) => v.trim()).filter(Boolean) };
        try {
          const r = await api("/lore/" + (nuova ? "nuova" : encodeURIComponent(x.id)), { metodo: "PUT", dati: d });
          aperte.add(r.id); chiudi(); avviso("Scheda salvata."); ricarica();
        } catch (e) { err.textContent = e.message; b.disabled = false; }
      } } })) });
    nome.focus();
  }

  function importa() {
    const testo = el("textarea", { rows: "12", "aria-label": "Testo da importare", placeholder: "# PNG: Nome\nLuogo: …\n\n## Per il Master\n…\n\n## Per i giocatori\n…" });
    const file = el("input", { type: "file", accept: ".md,.txt,text/plain,text/markdown", "aria-label": "File da importare" });
    file.addEventListener("change", async () => { const f = file.files[0]; if (f) testo.value = await f.text(); });
    const aggiorna = el("input", { type: "checkbox" });
    const esito = el("div", { class: "lore-anteprima", role: "status" });
    const nomi = (l) => l.map((v) => (TIPI[v.tipo] || {}).u + ": " + v.nome).join(", ");
    let fatta = false;
    const anteprima = async () => {
      const r = await api("/lore/importa", { metodo: "POST", dati: { testo: testo.value } });
      r.errori = r.errori || []; r.nuove = r.nuove || []; r.diverse = r.diverse || [];
      svuota(esito, el("p", {}, el("b", { testo: r.nuove.length + " nuove" }), ", " + r.uguali + " già uguali, ", el("b", { testo: r.diverse.length + " diverse" }), " da quelle che hai."),
        r.nuove.length ? el("p", { class: "piccolo", testo: "Nuove: " + nomi(r.nuove) }) : null,
        r.diverse.length ? el("p", { class: "piccolo", testo: "Diverse: " + nomi(r.diverse) }) : null,
        r.errori.length ? el("details", {}, el("summary", { testo: "⚠ " + r.errori.length + " righe non importate" }), el("ul", { class: "piccolo" }, r.errori.map((e) => el("li", { testo: (e.riga ? "riga " + e.riga + ": " : "") + e.testo })))) : null);
      fatta = true;
      return r;
    };
    dialogo("📥 Importa nel Codice della campagna", el("div", { class: "lore-editor" },
      el("p", { class: "tenue piccolo", testo: "Incolla qui le schede scritte con ChatGPT (formato di «🤖 Testo per ChatGPT») oppure scegli un file .md o .txt. Prima vedi cosa cambia; poi «Importa» aggiunge le schede nuove senza cancellare nulla." }),
      file, testo, el("label", { class: "riga" }, aggiorna, el("span", { testo: "Aggiorna anche le schede diverse (il testo importato sostituisce il tuo)" })), esito),
    { largo: true, azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Chiudi", on: { click: chiudi } }),
      el("button", { testo: "👁 Anteprima", on: { click: async () => { try { await anteprima(); } catch (e) { avviso(e.message, "errore"); } } } }),
      el("button", { class: "primario", testo: "Importa", on: { click: async (ev) => {
        const b = ev.currentTarget; b.disabled = true;
        try {
          if (!fatta) await anteprima();
          const r = await api("/lore/importa", { metodo: "POST", dati: { testo: testo.value, applica: true, aggiorna: aggiorna.checked } });
          chiudi(); avviso("Importate " + r.nuove.length + " schede nuove" + (aggiorna.checked && r.diverse.length ? " e aggiornate " + r.diverse.length : "") + ".", "", 6000); ricarica();
        } catch (e) { avviso(e.message, "errore"); b.disabled = false; }
      } } })) });
  }

  function testoChatGPT() {
    const t = el("textarea", { rows: "16", readonly: true, "aria-label": "Testo per ChatGPT" }); t.value = TESTO_CHATGPT;
    dialogo("🤖 Testo per ChatGPT", el("div", { class: "lore-editor" },
      el("p", { class: "tenue piccolo", testo: "Copialo in ChatGPT prima del tuo materiale (o chiedigli di riscrivere ciò che avete già fatto). Poi incolla la risposta in «📥 Importa». Per continuare una campagna già importata, dai a ChatGPT anche il file di «📤 Esporta»." }), t),
    { largo: true, azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Chiudi", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "📋 Copia", on: { click: async () => {
        try { await navigator.clipboard.writeText(TESTO_CHATGPT); avviso("Copiato."); } catch { t.select(); document.execCommand("copy"); avviso("Copiato."); }
      } } })) });
  }

  const cerca = el("input", { type: "search", placeholder: "Cerca nomi, testi, etichette…", "aria-label": "Cerca nel Codice" });
  cerca.addEventListener("input", () => { filtro.testo = cerca.value; disegna(); });
  const tipoSel = el("select", { "aria-label": "Tipo di scheda" }, el("option", { value: "", testo: "Tutti i tipi" }), Object.entries(TIPI).map(([k, T]) => el("option", { value: k, testo: T.i + " " + T.t })));
  tipoSel.addEventListener("change", () => { filtro.tipo = tipoSel.value; disegna(); });
  const gruppoSel = el("select", { "aria-label": "Gruppo" });
  gruppoSel.addEventListener("change", () => { filtro.gruppo = gruppoSel.value; disegna(); });

  svuota(azioni, el("button", { class: "primario piccolo", testo: "＋ Nuova scheda", on: { click: () => editor(null) } }),
    el("button", { class: "piccolo", testo: "📥 Importa", on: { click: importa } }),
    el("a", { class: "pulsante piccolo", href: "/api/v1/lore/esporta", download: "Codice_della_campagna.md", testo: "📤 Esporta" }),
    el("button", { class: "piccolo", testo: "🤖 Testo per ChatGPT", on: { click: testoChatGPT } }),
    el("button", { class: "piccolo", testo: "⚖ Bilanciamento", title: "Bilancia un'avventura per un gruppo, dai dati delle schede", on: { click: () => bilanciamento("") } }));
  svuota(corpo, el("div", { class: "largo" },
    riquadroPromemoria,
    el("div", { class: "riga-flex lore-filtri" }, cerca, tipoSel, el("label", { class: "riga" }, el("span", { class: "tenue piccolo", testo: "Stato del gruppo" }), gruppoSel)),
    lista));
  // un promemoria arriva mentre la pagina è aperta: si ricarica
  const nuovoPromemoria = () => ricarica();
  document.addEventListener("gasterbless:promemoria", nuovoPromemoria);
  document.addEventListener("gasterbless:codice-cambiato", nuovoPromemoria); // bilanciamento salvato nel Codice
  await ricarica();
  // apertura diretta: /codice?scheda=<id> (dai collegamenti di PNG e mostri, mappa, background)
  const diretta = new URLSearchParams(location.search).get("scheda");
  const sd = diretta && dati.schede.find((x) => x.id === diretta);
  if (sd) vaiA(sd);
  svuota(gruppoSel, dati.gruppi.map((g) => el("option", { value: g.id, selected: g.id === filtro.gruppo, testo: g.nome })));
  return () => { svuota(azioni); document.removeEventListener("gasterbless:promemoria", nuovoPromemoria); document.removeEventListener("gasterbless:codice-cambiato", nuovoPromemoria); };
}
