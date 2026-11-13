// Sezione «Assistente» del Master (2026-11-09): chat con un'IA che conosce la situazione del gruppo scelto. Dal 2026-11-13
// due identità separate: 🏰 Cronista della campagna (lore homebrew) e 📜 Archivista del canone (testi ufficiali della
// «📚 Biblioteca»), che si consultano a vicenda in riquadri separati; più «🧛 Parla con l'antagonista».
// Dove gira l'IA lo sceglie il Master in «⚙ Impostazioni IA»: sul PC (Ollama), servizi online gratuiti (Gemini, Groq,
// OpenRouter, Mistral, Cerebras, NVIDIA NIM) o Claude. Il server aggiunge da sé il contesto di gioco (internal/server/assistente.go): «👁 Cosa sa
// l'IA» mostra esattamente il testo inviato. Le risposte arrivano a pezzi e si vedono mentre l'IA scrive.
// Dopo ogni riassunto nuovo l'IA apre da sola una conversazione «💡 Dopo «…»: idee per l'antagonista» (segnata 🆕), a cui il
// Master può rispondere per ragionarci insieme.
import { el, svuota, api, avviso, conferma, dialogo, leggiCSRF, tempoReale } from "../ui/ui.js";
import { mostraTesto } from "../ui/testo_formattato.js";
import { apriBiblioteca, fontiRisposta } from "../ui/biblioteca.js";

// Le due identità (2026-11-13, internal/server/identita_ia.go) e l'antagonista. Ogni conversazione resta della sua identità.
const IDENT = {
  assistente: { i: "🏰", nome: "Cronista della campagna", pensa: "Il Cronista sta pensando…", ph: "Chiedi al Cronista della campagna…",
    titolo: "Risponde dalla lore della tua campagna homebrew: Campaign Bible, Codice della campagna, documenti della campagna e situazione del gruppo." },
  canone: { i: "📜", nome: "Archivista del canone", pensa: "L'Archivista consulta i testi…", ph: "Chiedi all'Archivista del canone…",
    titolo: "Risponde solo dai testi ufficiali della biblioteca (manuale dell'avventura, romanzi, compendi, Tomo interattivo), con i riferimenti." },
  antagonista: { i: "🧛", nome: "l'antagonista", pensa: "l'antagonista riflette…", ph: "Parla all'antagonista…", titolo: "" },
};
const altra = (modo) => (modo === "canone" ? "assistente" : "canone");

// Pulsanti rapidi: riempiono la casella con una richiesta già scritta, che il Master può completare prima di inviare.
const RAPIDI = [
  { i: "🧙", t: "PNG", testo: "Inventa un PNG adatto al luogo in cui si trova il gruppo, da usare adesso. Dettagli: " },
  { i: "🏚", t: "Luogo", testo: "Descrivi il luogo in cui si trova il gruppo: testo da leggere ad alta voce, cosa c'è da scoprire e un possibile incontro." },
  { i: "🧛", t: "l'antagonista e background", testo: "Per ogni personaggio proponi due agganci personali che l'antagonista può usare (da storia, legami, ideali e difetti), indicando il dettaglio da cui nasce ciascuno. Poi suggerisci una mossa dell'antagonista per la prossima sessione." },
  { i: "⚔", t: "Combattimento", testo: "Suggerisci le tattiche dei mostri per questo round e una frase breve e cupa per descrivere ogni loro attacco." },
  { i: "📝", t: "Riassunto", testo: "Scrivi la bozza del riassunto della sessione: un titolo, poi un racconto in terza persona al passato, breve, nello stile di Gaster Bless. Ecco i miei appunti:\n- " },
];

// Pulsanti rapidi dell'Archivista del canone
const RAPIDI_CANONE = [
  { i: "🧛", t: "l'antagonista", testo: "Che cosa dicono i testi sull'antagonista riguardo a: " },
  { i: "📍", t: "Luogo", testo: "Che cosa dice il manuale dell'avventura su questo luogo (descrizione, abitanti, segreti, tesori): " },
  { i: "👤", t: "PNG", testo: "Chi è, secondo i testi ufficiali, e che cosa vuole: " },
  { i: "📖", t: "Capitolo", testo: "Riassumi questo capitolo, con i fatti principali e i personaggi: " },
  { i: "⚖", t: "Confronta", testo: "Su questo punto, che cosa dicono il manuale dell'avventura e i romanzi? Dove non sono d'accordo? " },
];

export async function mostra(corpo, { azioni }) {
  const box = el("div", { class: "largo assistente-sez" });
  svuota(corpo, box);
  let imp = null, gruppi = [], conv = null; // conv: {id, titolo, messaggi}
  let ctx = null; // contesto del gruppo scelto: sessione e personaggi (per i pulsanti «salva in Gaster Bless»)
  let inCorso = null; // AbortController della risposta in arrivo
  let ritorno = null; // «↔ Chiedi all'altra identità»: la conversazione da cui si è partiti {id, modo}
  const selGruppo = el("select", { "aria-label": "Gruppo di cui l'IA conosce la situazione" });
  const elenco = el("div", { class: "ia-elenco" });
  const filo = el("div", { class: "ia-filo", "aria-live": "polite" });
  const domanda = el("textarea", { class: "ia-domanda", rows: "3", placeholder: "Chiedi all'assistente… (Invio per inviare, Maiusc+Invio per andare a capo)", "aria-label": "Domanda all'assistente" });
  const invia = el("button", { class: "primario", testo: "Invia" });
  const stato = el("p", { class: "tenue piccolo ia-stato" });
  const rapidi = el("div", { class: "ia-rapidi" });
  const disegnaRapidi = (modo) => svuota(rapidi, modo === "antagonista" ? null : (modo === "canone" ? RAPIDI_CANONE : RAPIDI).map((r) => el("button", { class: "piccolo", testo: r.i + " " + r.t, title: r.testo,
    on: { click: () => { domanda.value = r.testo; domanda.focus(); domanda.setSelectionRange(r.testo.length, r.testo.length); } } })));
  const qs = new URLSearchParams(location.search);
  const dalLink = { gruppo: qs.get("gruppo") || "", domanda: qs.get("domanda") || "" };
  if (dalLink.domanda) domanda.value = dalLink.domanda.slice(0, 4000);

  async function carica() {
    [imp, gruppi] = await Promise.all([api("/assistente/impostazioni"), api("/gruppi").catch(() => [])]);
    gruppi = Array.isArray(gruppi) ? gruppi : [];
    const g0 = selGruppo.value || dalLink.gruppo; // /assistente?gruppo=…&domanda=… (dal bilanciamento: domanda pronta, non inviata)
    ctx = await api("/assistente/contesto" + (g0 ? "?gruppo=" + encodeURIComponent(g0) : "")).catch(() => null);
    svuota(selGruppo, gruppi.map((g) => el("option", { value: g.id, testo: g.nome })));
    if (ctx && ctx.gruppo) selGruppo.value = ctx.gruppo;
    disegnaStato();
    await caricaElenco();
  }

  function disegnaStato() {
    const c = imp && imp.config;
    if (!c || !c.fornitore) { stato.textContent = "IA non ancora scelta: apri «⚙ Impostazioni IA»."; return; }
    const f = imp.fornitori[c.fornitore];
    stato.textContent = f.nome + (c.modello ? " · " + c.modello : "") + (f.online ? " · online: i dati di gioco del gruppo vengono inviati al servizio" : " · sul PC: nulla esce di casa");
  }

  async function caricaElenco() {
    const l = await api("/assistente/conversazioni").catch(() => []);
    // Gruppi separati: solo le conversazioni del gruppo scelto in alto.
    const lista = (Array.isArray(l) ? l : []).filter((c) => !c.gruppo || !selGruppo.value || c.gruppo === selGruppo.value);
    svuota(elenco,
      el("div", { class: "riga-flex ia-nuove" },
        el("button", { class: "piccolo", testo: "＋ 🏰 Cronista della campagna", title: IDENT.assistente.titolo, on: { click: () => nuova("assistente") } }),
        el("button", { class: "piccolo ia-canone-btn", testo: "＋ 📜 Archivista del canone", title: IDENT.canone.titolo, on: { click: () => nuova("canone") } }),
        el("button", { class: "piccolo ia-antagonista-btn", testo: "🧛 Parla con l'antagonista", title: "L'IA risponde nei panni dell'antagonista (istruzioni in «⚙ Impostazioni IA»)",
          on: { click: () => nuova("antagonista") } })),
      lista.length ? null : el("p", { class: "tenue piccolo", testo: "Le conversazioni di questo gruppo restano salvate sul PC. Ogni gruppo ha le sue: l'IA non mescola le informazioni." }),
      lista.map((c) => el("div", { class: "ia-voce" + (conv && conv.id === c.id ? " attiva" : "") },
        el("button", { class: "ia-apri" + (c.nuova ? " nuova" : "") + (c.modo === "antagonista" ? " antagonista" : c.modo === "canone" ? " canone" : ""), testo: (c.nuova ? "🆕 " : "") + (IDENT[c.modo] || IDENT.assistente).i + " " + (c.titolo || "Conversazione"),
          title: (IDENT[c.modo] || IDENT.assistente).nome + (c.nomeGruppo ? " · " + c.nomeGruppo : "") + " · " + new Date(c.modificato * 1000).toLocaleString("it-IT"),
          on: { click: () => apri(c.id) } }),
        el("button", { class: "piccolo pericolo", testo: "🗑", title: "Elimina", "aria-label": "Elimina la conversazione", on: { click: async () => {
          if (!(await conferma("Eliminare la conversazione?", "«" + (c.titolo || "Conversazione") + "» sparisce dal PC.", "Elimina", true))) return;
          try { await api("/assistente/conversazioni/" + c.id, { metodo: "DELETE" }); if (conv && conv.id === c.id) { conv = null; disegnaFilo(); } await caricaElenco(); } catch (e) { avviso(e.message, "errore"); }
        } } }))));
  }

  function nuova(modo) { conv = { id: "", titolo: "", messaggi: [], modo }; ritorno = null; disegnaFilo(); caricaElenco(); domanda.focus(); }

  async function apri(id, tieniRitorno) {
    try { conv = await api("/assistente/conversazioni/" + id); if (conv.gruppo) selGruppo.value = conv.gruppo; if (!tieniRitorno) ritorno = null; disegnaFilo(); caricaElenco(); }
    catch (e) { avviso(e.message, "errore"); }
  }

  // «↔ Chiedi all'altra identità»: la stessa domanda all'altra identità, in una conversazione sua; poi «↩ Riporta»
  // mette la risposta nella casella della conversazione di partenza, dichiarata come tale, da mandare o correggere.
  function chiediAllAltra(i) {
    const da = conv.modo || "assistente", verso = altra(da);
    const dom = [...conv.messaggi.slice(0, i)].reverse().find((x) => x.ruolo === "master");
    if (!dom) return;
    ritorno = { id: conv.id, modo: da };
    conv = { id: "", titolo: "", messaggi: [], modo: verso };
    disegnaFilo();
    domanda.value = (da === "canone" ? "L'Archivista del canone" : "Il Cronista della campagna") + " ti chiede: " + dom.testo;
    chiedi();
  }
  async function riporta(m) {
    const r = ritorno, chi = conv.modo === "canone" ? "L'Archivista del canone" : "Il Cronista della campagna";
    const cosa = conv.modo === "canone" ? "È il canone, non la campagna: tienilo separato." : "È la campagna del Master, non il canone: tienilo separato.";
    if (r.id) await apri(r.id); else nuova(r.modo);
    domanda.value = chi + " risponde:\n«" + m.testo.trim() + "»\n\n" + cosa;
    domanda.focus();
  }

  function bolla(m) {
    if (m.ruolo === "master") return el("div", { class: "ia-msg ia-master" }, el("p", { testo: m.testo }));
    if (antagonista()) { // nei panni dell'antagonista: solo la risposta, senza i pulsanti per salvare PNG e riassunti
      return el("div", { class: "ia-msg ia-risposta ia-antagonista" }, el("p", { class: "ia-chi", testo: "🧛 l'antagonista" }), mostraTesto(m.testo),
        el("div", { class: "riga-flex ia-piede" }, el("button", { class: "piccolo", testo: "📋 Copia", on: { click: async () => {
          try { await navigator.clipboard.writeText(m.testo); avviso("Copiato."); } catch { avviso("Copia non riuscita: seleziona il testo a mano.", "errore"); }
        } } }), el("button", { class: "piccolo", testo: "📌 Negli appunti di un personaggio", on: { click: () => salvaAppunti(m.testo) } }),
          m.fornitore ? el("span", { class: "tenue piccolo", testo: m.fornitore }) : null));
    }
    const copia = el("button", { class: "piccolo", testo: "📋 Copia", on: { click: async () => {
      try { await navigator.clipboard.writeText(m.testo); avviso("Copiato."); } catch { avviso("Copia non riuscita: seleziona il testo a mano.", "errore"); }
    } } });
    const modo = (conv && conv.modo) || "assistente", canone = modo === "canone";
    const i = conv ? conv.messaggi.indexOf(m) : -1;
    const corpo = mostraTesto(m.testo);
    const fonti = fontiRisposta(corpo, m.fonti);
    return el("div", { class: "ia-msg ia-risposta" + (canone ? " ia-canone" : "") }, el("p", { class: "ia-chi", testo: IDENT[modo].i + " " + IDENT[modo].nome }), corpo, fonti,
      el("div", { class: "riga-flex ia-piede" }, copia,
        i > 0 ? el("button", { class: "piccolo", testo: "↔ Chiedi " + (canone ? "al Cronista" : "all'Archivista"), title: "La stessa domanda all'" + (canone ? "altra identità: il Cronista della campagna" : "altra identità: l'Archivista del canone"),
          on: { click: () => chiediAllAltra(i) } }) : null,
        ritorno && i === conv.messaggi.length - 1 ? el("button", { class: "piccolo primario", testo: "↩ Riporta " + (ritorno.modo === "canone" ? "all'Archivista" : "al Cronista"), on: { click: () => riporta(m) } }) : null,
        el("button", { class: "piccolo", testo: "🧙 Salva come PNG", on: { click: () => salvaPNG(m.testo) } }),
        canone ? null : el("button", { class: "piccolo", testo: "📝 Pubblica come riassunto", on: { click: () => salvaRiassunto(m.testo) } }),
        el("button", { class: "piccolo", testo: "📌 Negli appunti di un personaggio", on: { click: () => salvaAppunti(m.testo) } }),
        m.fornitore ? el("span", { class: "tenue piccolo", testo: m.fornitore }) : null));
  }

  // ===== Fase IA-2: le risposte diventano dati di Gaster Bless, sempre con un dialogo da controllare prima =====
  // Titolo proposto: il primo titolo («# …») o la prima riga in grassetto; il resto è il corpo.
  function titoloECorpo(t) {
    const righe = t.split("\n");
    const i = righe.findIndex((r) => r.trim());
    if (i < 0) return { titolo: "", corpo: t };
    const r = righe[i].trim();
    const m = r.match(/^#{1,6}\s+(.+)$/) || r.match(/^\*\*([^*]{2,80})\*\*:?$/);
    if (!m) return { titolo: "", corpo: t.trim() };
    return { titolo: m[1].replace(/\*\*/g, "").trim(), corpo: righe.slice(i + 1).join("\n").trim() };
  }
  const nomeDa = (titolo) => titolo.split(/[,:–—(]| - /)[0].trim().slice(0, 80);

  function salvaPNG(t) {
    const { titolo, corpo } = titoloECorpo(t);
    const nome = el("input", { maxlength: "80", valore: nomeDa(titolo), "aria-label": "Nome del PNG" });
    const ruolo = el("input", { maxlength: "120", valore: titolo && nomeDa(titolo) !== titolo ? titolo.slice(nomeDa(titolo).length).replace(/^[\s,:–—(-]+|\)$/g, "").slice(0, 120) : "", placeholder: "es. locandiera di Borgofosco", "aria-label": "Ruolo del PNG" });
    const note = el("textarea", { class: "testo-lungo", rows: "10", "aria-label": "Note del PNG" });
    note.value = corpo;
    const err = el("p", { class: "errore" });
    dialogo("🧙 Salva come PNG", el("div", { class: "ia-imp" },
      el("p", { class: "tenue", testo: "Il PNG va nella sezione «PNG e mostri» (solo tu lo vedi), con il testo dell'assistente nelle note. Controlla e correggi prima di salvare." }),
      el("label", {}, "Nome", nome), el("label", {}, "Ruolo", ruolo), el("label", {}, "Note", note), err), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva il PNG", on: { click: async (e) => {
          err.textContent = ""; e.target.disabled = true;
          try {
            const v = await api("/bestiario/voci", { dati: { nome: nome.value.trim(), cat: "png", ruolo: ruolo.value.trim() } });
            if (note.value.trim()) await api("/bestiario/voci/" + encodeURIComponent(v.id), { metodo: "PATCH", dati: { note: note.value } });
            chiudi(); avviso("PNG salvato in «PNG e mostri»: " + nome.value.trim() + ".");
          } catch (er) { err.textContent = er.message; e.target.disabled = false; }
        } } })),
    });
  }

  function salvaRiassunto(t) {
    if (!ctx || !ctx.gruppo) { avviso("Scegli prima un gruppo.", "errore"); return; }
    const { titolo, corpo } = titoloECorpo(t);
    const tit = el("input", { maxlength: "120", valore: titolo, "aria-label": "Titolo del riassunto" });
    const testo = el("textarea", { class: "testo-lungo", rows: "12", "aria-label": "Testo del riassunto" });
    testo.value = corpo;
    const ses = ctx.sessione && ctx.sessione.id;
    const legata = el("input", { type: "checkbox", selezionato: !!ses });
    const err = el("p", { class: "errore" });
    dialogo("📝 Pubblica come riassunto", el("div", { class: "ia-imp" },
      el("p", { class: "tenue", testo: "Il riassunto va nella sezione Riassunti del gruppo " + ctx.nomeGruppo + ": i giocatori lo vedono appena lo pubblichi. Controllalo bene (niente segreti!). La firma sarà il tuo nome; puoi cambiarla dopo con «✒ Firma»." }),
      el("label", {}, "Titolo", tit), el("label", {}, "Testo", testo),
      ses ? el("label", { class: "riga" }, legata, el("span", { testo: "Collegato alla sessione «" + ctx.sessione.titolo + "»" })) : null, err), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Pubblica", on: { click: async (e) => {
          err.textContent = ""; e.target.disabled = true;
          try {
            await api("/riassunti", { dati: { gruppo: ctx.gruppo, sessione: ses && legata.checked ? ses : "", titolo: tit.value.trim(), testo: testo.value } });
            chiudi(); avviso("Riassunto pubblicato per il gruppo " + ctx.nomeGruppo + ".");
          } catch (er) { err.textContent = er.message; e.target.disabled = false; }
        } } })),
    });
  }

  async function salvaAppunti(t) {
    const pgs = (ctx && ctx.personaggi) || [];
    if (!pgs.length) { avviso("Nel gruppo scelto non ci sono personaggi.", "errore"); return; }
    const sel = el("select", { "aria-label": "Personaggio" }, pgs.map((p) => el("option", { value: p.id, testo: p.nome })));
    // Proposta: il personaggio citato per primo nella risposta.
    const citato = pgs.map((p) => ({ p, i: t.indexOf(p.nome) })).filter((x) => x.i >= 0).sort((a, b) => a.i - b.i)[0];
    if (citato) sel.value = citato.p.id;
    const prima = el("div", { class: "tenue piccolo ia-prima" });
    const nuovo = el("textarea", { class: "testo-lungo", rows: "10", "aria-label": "Appunti da aggiungere" });
    nuovo.value = t.trim();
    const err = el("p", { class: "errore" });
    let attuali = "";
    const leggi = async () => {
      try { attuali = (await api("/personaggi/" + encodeURIComponent(sel.value) + "/note-master")).testo || ""; } catch { attuali = ""; }
      prima.textContent = attuali ? "Appunti già presenti: " + attuali.length.toLocaleString("it-IT") + " caratteri; il nuovo testo si aggiunge in fondo." : "Nessun appunto ancora su questo personaggio.";
    };
    sel.addEventListener("change", leggi);
    dialogo("📌 Appunti del Master", el("div", { class: "ia-imp" },
      el("p", { class: "tenue", testo: "Gli appunti li vedi solo tu: nella sezione Background, sotto «🧛 Scheda per il Master». L'assistente li legge nelle domande successive." }),
      el("label", {}, "Personaggio", sel), prima, el("label", {}, "Da aggiungere (puoi tagliare quello che non serve)", nuovo), err), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Aggiungi agli appunti", on: { click: async (e) => {
          err.textContent = ""; e.target.disabled = true;
          const data = new Date().toLocaleDateString("it-IT");
          const testo = (attuali.trim() ? attuali.trim() + "\n\n" : "") + "## Dall'assistente, " + data + "\n\n" + nuovo.value.trim();
          try {
            await api("/personaggi/" + encodeURIComponent(sel.value) + "/note-master", { metodo: "PUT", dati: { testo } });
            chiudi(); avviso("Appunti salvati su " + sel.selectedOptions[0].textContent + ".");
            ctx = await api("/assistente/contesto?gruppo=" + encodeURIComponent(selGruppo.value)).catch(() => ctx);
          } catch (er) { err.textContent = er.message; e.target.disabled = false; }
        } } })),
    });
    await leggi();
  }

  // «🧛 Parla con l'antagonista»: conversazioni in cui l'IA interpreta l'antagonista (assistente_antagonista.go)
  const antagonista = () => !!(conv && conv.modo === "antagonista");
  function disegnaFilo() {
    const msgs = conv ? conv.messaggi : [];
    const modo = (conv && conv.modo) || "assistente";
    filo.classList.toggle("antagonista", antagonista());
    filo.classList.toggle("canone", modo === "canone");
    disegnaRapidi(modo);
    domanda.placeholder = IDENT[modo].ph + " (Invio per inviare, Maiusc+Invio per andare a capo)";
    if (!msgs.length && antagonista()) {
      svuota(filo, el("div", { class: "ia-vuoto" },
        el("p", { class: "ia-antagonista-titolo", testo: "🧛 il Conte Varn ti ascolta." }),
        el("p", { testo: "l'antagonista sa di essere l'antagonista della tua campagna e che tu sei il Master: segue i forestieri del gruppo scelto in alto e ti consiglia, in prima persona, cosa farebbe o non farebbe, e perché." }),
        el("p", { class: "tenue piccolo", testo: "Chiedigli per esempio «Cosa faresti adesso con il gruppo?» o «Ti piace questa idea per Borgofosco?». Per una scena con un personaggio scrivi «[Arin dice:] …». Le sue istruzioni le cambi in «⚙ Impostazioni IA» → «🧛 Istruzioni dell'antagonista»." })));
      return;
    }
    if (!msgs.length && modo === "canone") {
      svuota(filo, el("div", { class: "ia-vuoto" },
        el("p", { class: "ia-canone-titolo", testo: "📜 L'Archivista del canone" }),
        el("p", { testo: "Risponde solo dai testi ufficiali che hai messo nella «📚 Biblioteca» (manuale dell'avventura, romanzi, compendi, Tomo interattivo) e dalle schede del Codice segnate canone, romanzi o tomo. Cita libro e capitolo o pagina; se nei testi non c'è, lo dice." }),
        el("p", { class: "tenue piccolo", testo: "Non conosce la tua campagna: quando serve riceve un breve consulto del Cronista, tenuto separato. Non dipende dal gruppo scelto." }),
        el("button", { class: "piccolo", testo: "📚 Apri la biblioteca", on: { click: () => apriBiblioteca({ scaffale: "canone" }) } })));
      return;
    }
    svuota(filo, msgs.length ? msgs.map(bolla) : el("div", { class: "ia-vuoto" },
      el("p", { class: "ia-canone-titolo", testo: "🏰 Il Cronista della campagna" }),
      el("p", { testo: "Chiedi quello che ti serve: il Cronista conosce la lore della tua campagna (Campaign Bible e Codice) e già sessione, luogo, personaggi e combattimento del gruppo scelto." }),
      el("p", { class: "tenue piccolo", testo: "Del canone sa solo ciò che gli riferisce l'Archivista, sempre dichiarato. Le sue risposte sono proposte: nulla cambia in Gaster Bless senza un tuo clic." })));
    filo.scrollTop = filo.scrollHeight;
  }

  async function chiedi() {
    const testo = domanda.value.trim();
    if (!testo || inCorso) return;
    if (!imp.config.fornitore) { impostazioni(); return; }
    const mio = { ruolo: "master", testo };
    if (!conv) conv = { id: "", titolo: "", messaggi: [], modo: "assistente" };
    conv.messaggi.push(mio);
    disegnaFilo();
    domanda.value = "";
    const risposta = { ruolo: "ia", testo: "" };
    const corpoR = el("div", { class: "testo-gasterbless" }, el("p", { class: "tenue", testo: IDENT[conv.modo || "assistente"].pensa }));
    const fermati = el("button", { class: "piccolo", testo: "■ Ferma", on: { click: () => inCorso && inCorso.abort() } });
    const b = el("div", { class: "ia-msg ia-risposta in-arrivo" }, corpoR, el("div", { class: "riga-flex ia-piede" }, fermati));
    filo.append(b);
    filo.scrollTop = filo.scrollHeight;
    invia.disabled = true;
    inCorso = new AbortController();
    let fine = null, errore = "";
    try {
      const r = await fetch("/api/v1/assistente/chiedi", { method: "POST", credentials: "same-origin", signal: inCorso.signal,
        headers: { "Content-Type": "application/json", "X-CSRF": leggiCSRF() },
        body: JSON.stringify({ conversazione: conv.id, gruppo: selGruppo.value, testo, modo: conv.modo || "assistente" }) });
      if (!r.ok) { const j = await r.json().catch(() => ({})); throw new Error(j.errore || "Errore " + r.status); }
      const lettore = r.body.getReader(), dec = new TextDecoder();
      let resto = "", ultimo = 0;
      for (;;) {
        const { value, done } = await lettore.read();
        if (done) break;
        resto += dec.decode(value, { stream: true });
        const righe = resto.split("\n"); resto = righe.pop();
        for (const riga of righe) {
          if (!riga.trim()) continue;
          const j = JSON.parse(riga);
          if (j.t) risposta.testo += j.t;
          if (j.errore) errore = j.errore;
          if (j.fine) fine = j;
        }
        if (Date.now() - ultimo > 120) { ultimo = Date.now(); svuota(corpoR, ...mostraTesto(risposta.testo || "…").childNodes); filo.scrollTop = filo.scrollHeight; }
      }
    } catch (e) {
      errore = e.name === "AbortError" ? "" : e.message;
    } finally { inCorso = null; invia.disabled = false; }
    if (errore && !risposta.testo) {
      conv.messaggi.pop(); // la domanda non è stata salvata: torna nella casella
      domanda.value = testo;
      b.replaceWith(el("div", { class: "ia-msg ia-errore" }, el("p", { class: "errore", testo: errore }),
        /impostazioni ia|chiave|modello|ollama/i.test(errore) ? el("button", { class: "piccolo", testo: "⚙ Impostazioni IA", on: { click: impostazioni } }) : null));
      if (!conv.messaggi.length) conv = null;
      return;
    }
    if (fine) {
      risposta.fornitore = fine.fornitore;
      if (Array.isArray(fine.fonti) && fine.fonti.length) risposta.fonti = fine.fonti;
      const nuova = !conv.id;
      conv.id = fine.conversazione;
      conv.messaggi.push(risposta);
      b.replaceWith(bolla(risposta));
      if (nuova) caricaElenco();
    } else {
      // interrotta prima della fine (pulsante «Ferma» o connessione caduta): resta visibile ma non salvata
      risposta.testo += "\n\n(Interrotta: questa risposta non è stata salvata.)";
      b.replaceWith(bolla(risposta));
    }
  }

  async function cosaSa() {
    let c;
    try { c = await api("/assistente/contesto?gruppo=" + encodeURIComponent(selGruppo.value || "")); } catch (e) { avviso(e.message, "errore"); return; }
    dialogo("Cosa sa l'IA", el("div", {},
      el("p", { class: "tenue", testo: "Questo è il testo che Gaster Bless aggiunge a ogni domanda" + (c.online ? ", inviato a " + c.nomeFornitore + "." : ".") +
        " Contiene solo dati di gioco: mai nomi degli account, telefoni o password." }),
      el("h3", { testo: "🏰 Cronista della campagna" }),
      el("details", {}, el("summary", { testo: "📜 Istruzioni del Cronista (Campaign Bible)" }), el("pre", { class: "ia-contesto", testo: c.istruzioni })),
      c.istruzioniAntagonista ? el("details", {}, el("summary", { testo: "🧛 Istruzioni dell'antagonista («Parla con l'antagonista»)" }), el("pre", { class: "ia-contesto", testo: c.istruzioniAntagonista })) : null,
      el("h4", { testo: "Situazione del gruppo " + (c.nomeGruppo || "") }),
      el("pre", { class: "ia-contesto", testo: c.contesto }),
      // Codice della campagna (fase L4): le schede pertinenti; con una domanda si aggiungono quelle che nomina
      c.codice ? el("details", {}, el("summary", { testo: "🗝 Codice della campagna (schede pertinenti al gruppo; a ogni domanda si aggiungono quelle che nomini)" }),
        el("pre", { class: "ia-contesto", testo: c.codice })) : null,
      el("h3", { testo: "📜 Archivista del canone" }),
      el("details", {}, el("summary", { testo: "Istruzioni dell'Archivista" }), el("pre", { class: "ia-contesto", testo: c.istruzioniCanone || "" })),
      c.codiceCanone ? el("details", {}, el("summary", { testo: "🗝 Schede del canone nel Codice (etichette canone, romanzi, tomo interattivo)" }), el("pre", { class: "ia-contesto", testo: c.codiceCanone })) : null,
      el("p", { testo: "📚 Biblioteca: " + ((c.biblioteca || []).length ? (c.biblioteca || []).map((f) => (f.dominio === "canone" ? "📜 " : "🏰 ") + f.titolo + (f.attiva ? "" : " (spento)")).join(" · ") : "vuota") + "." }),
      el("p", { class: "tenue piccolo", testo: "A ogni domanda Gaster Bless aggiunge i brani pertinenti dei testi della sua identità e, " + (c.scambio ? "se servono, un breve consulto dell'altra identità in un riquadro separato." : "con lo scambio spento, niente dell'altra identità.") +
        " Sotto ogni risposta, «📚 Brani dati all'IA» mostra esattamente quali." })), { largo: true });
  }

  function impostazioni() {
    const c = imp.config, F = imp.fornitori;
    const ordine = ["ollama", "gemini", "groq", "openrouter", "mistral", "cerebras", "nvidia", "claude", "altro"];
    const sel = el("select", { "aria-label": "Servizio di IA" }, el("option", { value: "", testo: "— scegli —" }),
      ordine.map((k) => el("option", { value: k, testo: F[k].nome })));
    sel.value = c.fornitore || "";
    const nota = el("p", { class: "tenue piccolo" });
    const indirizzo = el("input", { "aria-label": "Indirizzo del servizio", placeholder: "https://…" });
    const modello = el("input", { "aria-label": "Modello", list: "ia-modelli", placeholder: "nome del modello" });
    const modelli = el("datalist", { id: "ia-modelli" });
    const chiave = el("input", { type: "password", autocomplete: "off", "aria-label": "Chiave del servizio" });
    const rigaChiave = el("label", {}, "Chiave del servizio", chiave);
    const cancella = el("button", { class: "piccolo", testo: "Cancella la chiave salvata" });
    let cancellaChiave = false;
    cancella.addEventListener("click", () => { cancellaChiave = true; chiave.value = ""; chiave.placeholder = "verrà cancellata al salvataggio"; cancella.hidden = true; });
    const err = el("p", { class: "errore" });
    const scambio = el("input", { type: "checkbox", selezionato: imp.scambio !== false, "aria-label": "Scambio tra le identità" });
    const approfondita = el("input", { type: "checkbox", selezionato: !!imp.ricercaApprofondita, "aria-label": "Ricerca approfondita nei testi" });
    const elencoMod = el("button", { class: "piccolo", testo: "Elenca modelli" });
    const aggiorna = (cambio) => {
      const k = sel.value, f = F[k];
      indirizzo.value = cambio ? (f ? f.indirizzo : "") : (c.indirizzo || (f ? f.indirizzo : ""));
      modello.value = cambio ? (f ? f.modello : "") : (c.modello || (f ? f.modello : ""));
      indirizzo.closest("label").hidden = !f || k === "claude";
      rigaChiave.hidden = !f || (!f.chiave && k !== "altro");
      const salvata = imp.chiaviSalvate[k];
      chiave.value = ""; cancellaChiave = false;
      chiave.placeholder = salvata ? "chiave salvata (lascia vuoto per tenerla)" : k === "ollama" ? "non serve" : "incolla qui la chiave";
      cancella.hidden = !salvata;
      nota.textContent = f ? f.gratis + (f.online ? " I dati di gioco del gruppo (non quelli personali) vengono inviati al servizio." : "") : "";
      svuota(modelli);
    };
    sel.addEventListener("change", () => aggiorna(true));
    elencoMod.addEventListener("click", async () => {
      err.textContent = "";
      elencoMod.disabled = true;
      try {
        await salva(false);
        const r = await api("/assistente/modelli");
        svuota(modelli, r.modelli.map((m) => el("option", { value: m })));
        avviso(r.modelli.length ? r.modelli.length + " modelli disponibili: scegli dalla casella «Modello»." : "Nessun modello: per Ollama scaricane uno (comando nella guida).");
        modello.focus();
      } catch (e) { err.textContent = e.message; }
      elencoMod.disabled = false;
    });
    async function salva(chiudiDopo, chiudi) {
      const dati = { fornitore: sel.value, indirizzo: indirizzo.value.trim(), modello: modello.value.trim(), scambio: scambio.checked, ricercaApprofondita: approfondita.checked };
      if (chiave.value.trim()) dati.chiave = chiave.value.trim(); else if (cancellaChiave) dati.chiave = "";
      imp = await api("/assistente/impostazioni", { metodo: "PUT", dati });
      if (dati.chiave !== undefined) { chiave.value = ""; cancellaChiave = false; }
      Object.assign(c, imp.config);
      disegnaStato();
      if (chiudiDopo) { chiudi(); avviso("Impostazioni dell'IA salvate."); }
    }
    dialogo("⚙ Impostazioni IA", el("div", { class: "ia-imp" },
      el("p", { class: "tenue", testo: "Scegli dove gira l'assistente. Le istruzioni passo passo per ogni servizio sono nella Guida del Master (capitolo «Assistente IA»)." }),
      el("label", {}, "Servizio", sel), nota,
      el("label", {}, "Indirizzo", indirizzo),
      el("div", { class: "riga-flex" }, el("label", { stile: { flex: "1" } }, "Modello", modello, modelli), elencoMod),
      rigaChiave, cancella,
      el("p", { class: "tenue piccolo", testo: "La chiave resta sul PC, cifrata, e non viene mai mostrata alle pagine." }),
      el("div", { class: "riga-flex" }, el("button", { class: "piccolo", testo: "🏰 Istruzioni del Cronista…", on: { click: istruzioniAssistente } }),
        el("span", { class: "tenue piccolo", testo: "la «Campaign Bible»: ruolo dell'assistente e canone della campagna" })),
      el("div", { class: "riga-flex" }, el("button", { class: "piccolo", testo: "🧛 Istruzioni dell'antagonista…", on: { click: istruzioniAntagonista } }),
        el("span", { class: "tenue piccolo", testo: "il «system prompt» usato in «Parla con l'antagonista»" })),
      el("div", { class: "riga-flex" }, el("button", { class: "piccolo", testo: "📜 Istruzioni dell'Archivista…", on: { click: istruzioniCanone } }),
        el("span", { class: "tenue piccolo", testo: "come risponde l'Archivista del canone" })),
      el("h3", { testo: "Le due identità" }),
      el("label", { class: "riga" }, scambio, el("span", { testo: "Scambio tra le identità: il Cronista riceve un breve consulto dell'Archivista e viceversa, solo quando serve e sempre in un riquadro separato" })),
      el("label", { class: "riga" }, approfondita, el("span", { testo: "Ricerca approfondita nei testi: prima di cercare, l'IA scrive nomi e sinonimi da cercare (una richiesta in più; consigliata con i servizi online, più lenta con Ollama)" })),
      err), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva", on: { click: async () => { err.textContent = ""; try { await salva(true, chiudi); } catch (e) { err.textContent = e.message; } } } })),
    });
    aggiorna(false);
  }

  // I due system prompt (Assistente e l'antagonista): li scrive il Master; vuoto o «Ripristina» = testo originale.
  const istruzioniAntagonista = () => editorIstruzioni("/assistente/antagonista", "🧛 Istruzioni dell'antagonista", "Istruzioni dell'antagonista",
    "Questo testo dice all'IA chi è l'antagonista e come si comporta. Si usa solo nelle conversazioni «🧛 Parla con l'antagonista»: l'Assistente del Master non cambia. Gaster Bless aggiunge in fondo ciò che l'antagonista «osserva» della campagna: la situazione del gruppo scelto.");
  const istruzioniCanone = () => editorIstruzioni("/assistente/canone", "📜 Istruzioni dell'Archivista", "Istruzioni dell'Archivista",
    "Come risponde l'Archivista del canone: solo dai testi della biblioteca e dalle schede del canone, con i riferimenti, senza mescolare la campagna. Gaster Bless aggiunge in fondo i brani pertinenti, le schede del canone e, se attivo, il consulto del Cronista.");
  const istruzioniAssistente = () => editorIstruzioni("/assistente/istruzioni", "🏰 Istruzioni del Cronista", "Istruzioni dell'Assistente",
    "La «Master Campaign Bible»: come lavora l'assistente e il canone della campagna (PNG, trame dei personaggi, questioni aperte). Vale per la chat dell'Assistente, la «Scheda per il Master» del Background e le idee dopo i riassunti; non per «Parla con l'antagonista». Quando la campagna va avanti, aggiornala qui (per esempio le questioni aperte che diventano canone). Gaster Bless aggiunge in fondo la regola dei gruppi separati e la situazione del gruppo scelto.");
  async function editorIstruzioni(percorso, titolo, etichetta, spiegazione) {
    let p;
    try { p = await api(percorso); } catch (e) { avviso(e.message, "errore"); return; }
    const testo = el("textarea", { class: "ia-prompt", rows: "18", "aria-label": etichetta, spellcheck: "false" });
    testo.value = p.testo;
    const info = el("p", { class: "tenue piccolo" });
    const conta = () => { info.textContent = testo.value.length.toLocaleString("it-IT") + " caratteri" + (testo.value.trim() === p.predefinito.trim() ? " · testo originale" : " · modificato da te"); };
    testo.addEventListener("input", conta); conta();
    dialogo(titolo, el("div", {},
      el("p", { class: "tenue", testo: spiegazione }),
      testo, info), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { testo: "Ripristina il testo originale", on: { click: () => { testo.value = p.predefinito; conta(); } } }),
        el("span", { class: "spazio" }),
        el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva", on: { click: async () => {
          try { p = await api(percorso, { metodo: "PUT", dati: { testo: testo.value } }); chiudi(); avviso(p.personalizzato ? etichetta + " salvate." : etichetta + ": testo originale."); }
          catch (e) { avviso(e.message, "errore"); }
        } } })),
    });
  }

  domanda.addEventListener("keydown", (e) => { if (e.key === "Enter" && !e.shiftKey && !e.isComposing) { e.preventDefault(); chiedi(); } });
  invia.addEventListener("click", chiedi);
  selGruppo.addEventListener("change", async () => {
    if (conv && conv.id) { conv = null; disegnaFilo(); }
    caricaElenco();
    ctx = await api("/assistente/contesto?gruppo=" + encodeURIComponent(selGruppo.value)).catch(() => ctx);
  });

  // Sul telefono i pulsanti restano nella riga del titolo, come icone (stessa regola della Mappa, base.css).
  azioni.classList.add("compatte");
  svuota(azioni,
    el("label", { class: "ia-gruppo" }, el("span", { class: "tenue piccolo", testo: "Gruppo" }), selGruppo),
    el("button", { class: "con-icona", "data-ico": "📚", "aria-label": "Biblioteca", testo: "📚 Biblioteca", on: { click: () => apriBiblioteca({ scaffale: conv && conv.modo === "assistente" ? "campagna" : "canone" }) } }),
    el("button", { class: "con-icona", "data-ico": "👁", "aria-label": "Cosa sa l'IA", testo: "👁 Cosa sa l'IA", on: { click: cosaSa } }),
    el("button", { class: "con-icona", "data-ico": "⚙", "aria-label": "Impostazioni IA", testo: "⚙ Impostazioni IA", on: { click: () => impostazioni() } }));
  svuota(box,
    el("div", { class: "ia-griglia" },
      el("aside", { class: "scheda-ui ia-lato" }, el("h2", { testo: "Conversazioni" }), elenco),
      el("section", { class: "scheda-ui ia-chat" },
        filo,
        rapidi,
        el("div", { class: "ia-scrivi" }, domanda, invia),
        stato)));
  disegnaFilo();
  try {
    await carica();
    if (!imp.config.fornitore) impostazioni();
  } catch (e) { svuota(box, el("p", { class: "errore", testo: e.message })); }
  // Proposte scritte dall'IA dopo un riassunto nuovo (agganci_ia.go): compaiono subito nell'elenco.
  const via = tempoReale.su("assistente.nuova", () => caricaElenco());
  return () => { via(); if (inCorso) inCorso.abort(); azioni.classList.remove("compatte"); };
}
