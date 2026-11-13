// Sezione "Iniziativa": il tracker del combattimento collegato alle schede.
// Lo stato è sul server; questa pagina mostra la vista che il server prepara per chi la apre
// (i mostri nascosti solo al Master, i PF esatti dei mostri solo a Master e Segretario) e invia i comandi
// sul canale in tempo reale. Comandi e permessi: internal/server/iniziativa.go.
import { el, svuota, api, avviso, dialogo, conferma, tempoReale, quando, dataOra, sessioneScelta, ricordaSessione } from "../ui/ui.js";
import { iconaRitratto } from "../ui/classi.js";
import { pannelloDadi } from "../ui/dadi.js";
import { apriMiniSanita } from "../ui/sanita_mini.js";
import { apriChat, nonLettiDi } from "../ui/chat.js";
import { profiloMostro } from "../regole/attacchi_mostri.js";
import { creaAttacchi } from "../ui/attacco.js";
import { creaEffettiColpi } from "../ui/effetti_colpi.js";
import { apriFinestra } from "../ui/finestra.js";
import { tastoIra, apriEffettoManuale, testoModifiche } from "../ui/effetti.js";

// Condizioni (regole 2014) con una sintesi scritta con parole nostre.
const CONDIZIONI = {
  Accecato: "Non vede: fallisce le prove basate sulla vista; i suoi attacchi hanno svantaggio, quelli contro di lui vantaggio.",
  Affascinato: "Non può attaccare chi lo ha affascinato né prenderlo di mira con effetti dannosi; chi lo affascina ha vantaggio nelle interazioni sociali con lui.",
  Assordato: "Non sente: fallisce le prove basate sull'udito.",
  Spaventato: "Svantaggio a prove e attacchi finché vede la fonte della paura; non può avvicinarsi volontariamente a essa.",
  Afferrato: "Velocità 0. Finisce se chi afferra è incapacitato o se viene allontanato.",
  Incapacitato: "Non può compiere azioni né reazioni.",
  Invisibile: "Non si vede senza magia o sensi speciali; i suoi attacchi hanno vantaggio, quelli contro di lui svantaggio.",
  Paralizzato: "Incapacitato, non si muove né parla; fallisce i TS di Forza e Destrezza; gli attacchi contro di lui hanno vantaggio e, da vicino, sono critici.",
  Pietrificato: "Trasformato in sostanza inanimata: incapacitato, resistenza a tutti i danni, immune a veleno e malattie.",
  Avvelenato: "Svantaggio ai tiri per colpire e alle prove di caratteristica.",
  Prono: "A terra: si muove solo strisciando; svantaggio ai suoi attacchi; gli attacchi vicini contro di lui hanno vantaggio, quelli da lontano svantaggio.",
  Trattenuto: "Velocità 0; svantaggio ai suoi attacchi e ai TS di Destrezza; gli attacchi contro di lui hanno vantaggio.",
  Stordito: "Incapacitato, si muove e parla a fatica; fallisce i TS di Forza e Destrezza; gli attacchi contro di lui hanno vantaggio.",
  "Privo di sensi": "Incapacitato, a terra, ignaro di ciò che lo circonda; fallisce i TS di Forza e Destrezza; gli attacchi vicini sono critici.",
  Concentrato: "Sta mantenendo un incantesimo a concentrazione: se subisce danni fa un TS di Costituzione (CD 10 o metà dei danni).",
  "Indebolimento 1": "Svantaggio alle prove di caratteristica.",
  "Indebolimento 2": "Velocità dimezzata (e gli effetti precedenti).",
  "Indebolimento 3": "Svantaggio a tiri per colpire e tiri salvezza (e gli effetti precedenti).",
  "Indebolimento 4": "PF massimi dimezzati (e gli effetti precedenti).",
  "Indebolimento 5": "Velocità 0 (e gli effetti precedenti).",
  "Indebolimento 6": "Morte.",
};

const suonoTurno = new Audio("/suoni/corvo.mp3");
const musicaAttesa = new Audio("/suoni/attesa.mp3");
musicaAttesa.loop = true;
musicaAttesa.volume = 0.5;

export async function mostra(corpo, { io, azioni }) {
  const me = io.utente;
  const contenitore = el("div", { class: "largo" });
  svuota(corpo, contenitore);

  // ----- Scelta della sessione -----
  let sessioni = [];
  try {
    const gruppi = await api("/gruppi");
    for (const g of gruppi) {
      const l = await api("/gruppi/" + g.id + "/sessioni");
      l.filter((s) => s.stato !== "conclusa" && (me.ruolo === "master" || s.partecipo)).forEach((s) => sessioni.push({ ...s, nomeGruppo: g.nome }));
    }
  } catch (e) { svuota(contenitore, el("p", { class: "errore", testo: e.message })); return; }
  sessioni.sort((a, b) => (a.stato === "in_corso") - (b.stato === "in_corso") || a.inizio - b.inizio).reverse();
  if (!sessioni.length) {
    svuota(contenitore, el("section", { class: "scheda-ui" }, el("h2", { testo: "Nessuna sessione aperta" }),
      el("p", { class: "tenue", testo: me.ruolo === "master"
        ? "Crea o avvia una sessione nella sezione Sessioni: il combattimento si gioca dentro una sessione."
        : "Quando il Master avvierà una sessione a cui partecipi, qui comparirà il tracker del combattimento." })));
    return;
  }
  let sessione = sessioneScelta(sessioni); // ?s=… oppure l'ultima scelta (ui.js)
  const scelta = el("select", { "aria-label": "Sessione", on: { change: () => { sessione = sessioni.find((s) => s.id === scelta.value); history.replaceState(null, "", "/iniziativa?s=" + sessione.id); ricordaSessione(sessione.id); entra(); } } },
    sessioni.map((s) => el("option", { value: s.id, testo: s.nomeGruppo + " · " + s.titolo + (s.stato === "in_corso" ? " (in corso)" : ""), selected: s.id === sessione.id })));
  const musica = el("button", { class: "piccolo", title: "Musica d'attesa", testo: "♪", on: { click: () => {
    if (musicaAttesa.paused) { musicaAttesa.play().catch(() => avviso("Il browser ha bloccato l'audio: tocca di nuovo.", "errore")); musica.classList.add("attivo"); }
    else { musicaAttesa.pause(); musica.classList.remove("attivo"); }
  } } });
  // Musica dello scontro (internal/server/mappa_musica.go): all'inizio il server passa al combattimento e la suona la
  // finestra della mappa o della proiezione. Se nessuna risponde sul canale del browser «gasterbless-musica» (mappa chiusa),
  // la suona questa pagina del Master; se il browser blocca l'audio, «▶ Avvia la musica».
  const canaleMusica = (() => { try { return new BroadcastChannel("gasterbless-musica"); } catch { return null; } })();
  let audioScontro = null;
  const musicaScontro = el("button", { class: "piccolo", hidden: true, title: "Musica dello scontro (la mappa non è aperta: suona da questa pagina)", on: { click: () => {
    if (!audioScontro) return;
    if (audioScontro.paused) audioScontro.play().then(() => { musicaScontro.textContent = "⏸ Musica dello scontro"; }).catch(() => avviso("Il browser ha bloccato l'audio: tocca di nuovo.", "errore"));
    else { audioScontro.pause(); musicaScontro.textContent = "▶ Avvia la musica"; }
  } } });
  const qualcunoSuona = () => new Promise((ok) => {
    if (!canaleMusica) return ok(false);
    const f = (e) => { if (e.data === "suono") { canaleMusica.removeEventListener("message", f); ok(true); } };
    canaleMusica.addEventListener("message", f);
    canaleMusica.postMessage("chi-suona");
    setTimeout(() => { canaleMusica.removeEventListener("message", f); ok(false); }, 600);
  });
  function fermaScontro() { if (audioScontro) { audioScontro.pause(); audioScontro = null; } musicaScontro.hidden = true; }
  async function suonaScontro(m) {
    if (!m || !m.fight || !m.file || !m.playing || (await qualcunoSuona())) return;
    fermaScontro();
    audioScontro = new Audio("/musica/" + m.file.split("/").map(encodeURIComponent).join("/"));
    audioScontro.loop = true;
    audioScontro.volume = 0.6;
    musicaScontro.hidden = false;
    try { await audioScontro.play(); musicaScontro.textContent = "⏸ Musica dello scontro"; }
    catch { musicaScontro.textContent = "▶ Avvia la musica"; }
  }
  if (canaleMusica) canaleMusica.addEventListener("message", (e) => { if (e.data === "suono") fermaScontro(); }); // la mappa ha ripreso a suonare
  svuota(azioni, sessioni.length > 1 ? scelta : el("span", { class: "tenue piccolo", testo: sessione.nomeGruppo + " · " + sessione.titolo }), musica, musicaScontro);

  // ----- Stato e connessione -----
  let vista = null, messaggi = [], nonLetti = 0, conv = "", bozza = "", pannelloD = null; // bozza: messaggio in scrittura (sopravvive agli aggiornamenti) // conv: giocatore scelto dal Master nei messaggi
  const zonaTracker = el("div");
  const zonaLato = el("div");
  // Monte Gelo (2026-11-07): promemoria delle condizioni ambientali se il gruppo è sul tratto Pendici → Passo.
  const zonaGelo = el("div", { class: "gkb-tracker", role: "status", hidden: true });
  svuota(contenitore, zonaGelo, el("div", { class: "griglia-iniz" }, zonaTracker, zonaLato));
  async function gelo() {
    try {
      const g = await api("/mappa/gelo?sessione=" + encodeURIComponent(sessione.id));
      const s = (g.segnalini || [])[0];
      if (!g.attivo || !s) { zonaGelo.hidden = true; return; }
      const R = g.regole;
      svuota(zonaGelo, el("b", { testo: "❄ Monte Gelo · " + String(Math.round(s.quota)).replace(/\B(?=(\d{3})+(?!\d))/g, ".") + " m" + (s.alta ? " (alta quota)" : "") + " · " + R.temperatura + " °C" }),
        el("span", { testo: " — raffiche di vento gelato: svantaggio agli attacchi a distanza e alla Percezione con l'udito, fiamme spente, chi vola atterra o cade · neve: area leggermente oscurata · ghiaccio: entrando sul ghiaccio Acrobazia CD 10 o prono · strapiombo: caduta 1d6 ogni 3 m." }));
      zonaGelo.hidden = false;
    } catch { zonaGelo.hidden = true; }
  }
  gelo();
  const gkTimer = setInterval(gelo, 60000);

  async function entra() {
    try {
      const r = await tempoReale.chiedi("sessione.entra", { sessione: sessione.id });
      vista = r.vista; messaggi = r.messaggi || [];
      nonLetti = messaggi.filter((m) => !m.mio && !m.letto).length;
      disegna();
    } catch (e) { svuota(zonaTracker, el("p", { class: "errore", testo: e.message })); }
  }
  async function comando(tipo, dati = {}) {
    try { return await tempoReale.chiedi(tipo, { sessione: sessione.id, ...dati }); }
    catch (e) { avviso(e.message, "errore"); throw e; }
  }

  const ruolo = () => (vista && vista.ruolo) || "giocatore";
  const master = () => ruolo() === "master";
  const gestisce = () => ruolo() === "master" || ruolo() === "segretario";
  const puoToccare = (c) => c.tipo === "pg" ? (gestisce() || c.mio) : (master() || (ruolo() === "segretario" && !c.nascosto));

  // ----- Disegno -----
  function disegna() {
    disegnaTracker();
    disegnaLato();
  }

  function disegnaTracker() {
    const k = vista.combattimento;
    const barra = el("div", { class: "riga-flex", stile: { marginBottom: "10px" } });
    if (master()) {
      barra.append(...[
        el("button", { class: k ? "piccolo" : "primario", testo: "⚔ Nuovo combattimento", on: { click: async () => {
          if (k && !(await conferma("Nuovo combattimento?", "Lo scontro in corso verrà chiuso.", "Nuovo combattimento", true))) return;
          comando("iniz.nuovo");
        } } }),
        k ? el("button", { class: "piccolo", testo: "＋ Mostri", on: { click: aggiungiMostri } }) : null,
        k ? el("button", { class: "piccolo", testo: "＋ PG", on: { click: aggiungiPG } }) : null,
        // scheda d'azione: una finestra a parte che mostra da sola chi è di turno (attacchi, PF, azioni)
        k ? el("button", { class: "piccolo", testo: "🗗 Scheda del turno", title: "Apre in un'altra finestra la scheda ridotta di chi è di turno: cambia da sola a ogni turno",
          on: { click: () => apriFinestra("/combattente?s=" + encodeURIComponent(sessione.id) + "&segui=1", "turno") } }) : null,
        k && !k.avviato ? el("button", { class: "primario piccolo", testo: "▶ Inizia", title: "Chi non ha l'iniziativa la tira ora", on: { click: async () => { const r = await comando("iniz.avvia").catch(() => null); if (r && r.pareggi) avviso("⚖ " + r.testo, "", 8000); if (r && r.musica) suonaScontro(r.musica); } } }) : null,
        k ? el("button", { class: "piccolo pericolo", testo: "Fine", on: { click: async () => { if (await conferma("Finire il combattimento?", "Il tracker si svuota; i PF dei personaggi restano nelle schede.", "Fine combattimento")) { await comando("iniz.fine").catch(() => {}); fermaScontro(); } } } }) : null].filter(Boolean));
    }
    if (k && k.avviato && gestisce()) {
      barra.append(el("span", { class: "spazio" }),
        el("button", { class: "piccolo", testo: "⏮", title: "Turno precedente", on: { click: () => comando("iniz.turno_precedente") } }),
        el("button", { class: "primario", testo: "Turno successivo ⏭", on: { click: () => comando("iniz.prossimo_turno") } }));
    }
    if (!k) {
      svuota(zonaTracker, el("section", { class: "scheda-ui" }, barra,
        el("h2", { testo: "Nessun combattimento in corso" }),
        el("p", { class: "tenue", testo: master() ? "Con «Nuovo combattimento» entrano subito i personaggi dei partecipanti che hanno scelto il PG; poi aggiungi i mostri." : "Quando il Master inizierà uno scontro lo vedrai qui, in tempo reale." })));
      return;
    }
    const turnoDi = k.turnoDi;
    const intest = el("div", { class: "riga-flex" },
      el("h2", { stile: { margin: 0 }, testo: k.avviato ? "⌛ Round " + k.round + " · Turno " + k.turni : "Preparazione dello scontro" }),
      k.avviato ? el("span", { class: "tenue", testo: "Turno di: " + ((vista.combattenti.find((c) => c.id === turnoDi) || {}).nome || "…") }) : el("span", { class: "tenue", testo: "Impostate le iniziative, poi il Master inizia." }));
    // Incantesimi con tiri salvezza che aspettano la conferma del Master (internal/server/incantesimi_scontro.go)
    const inAttesa = master() && vista.incantesimiInAttesa && vista.incantesimiInAttesa.length ? el("div", { class: "inc-attesa", role: "status" },
      el("b", { testo: "✨ Da confermare: " }), vista.incantesimiInAttesa.map((e) => el("button", { class: "piccolo primario", testo: e.incantesimo + " (" + e.nomeIncantatore + ")",
        on: { click: () => import("../ui/incantesimo_scontro.js").then((x) => x.mostraConferma(e)) } }))) : null;
    svuota(zonaTracker, el("section", { class: "scheda-ui" }, barra, inAttesa, intest,
      el("div", { class: "lista-iniz", role: "list" }, vista.combattenti.map((c) => riga(c, c.id === turnoDi))),
      // Effetti in corso (internal/server/effetti_scontro.go): finiscono da soli; il Master può terminarli
      vista.effetti && vista.effetti.length ? el("div", { class: "iniz-effetti", role: "list", "aria-label": "Effetti in corso" },
        el("b", { testo: "✨ Effetti in corso" }), vista.effetti.map((e) => {
          // il giocatore termina da sé i suoi effetti che non mettono condizioni (ira, Scudo della fede…)
          const suo = (vista.combattenti.find((x) => x.id === e.incantatore) || {}).mio && e.bersagli.every((b) => !b.condizioni.length);
          return el("div", { class: "riga-flex piccolo", role: "listitem" },
            el("span", { testo: e.nome + (e.nomeIncantatore ? " (" + e.nomeIncantatore + ")" : "") + " → " + e.bersagli.map((b) => b.nome + (b.condizioni.length ? ": " + b.condizioni.join(", ").toLowerCase() : "")).join("; ") +
              (e.mod ? " · " + testoModifiche(e.mod) : "") +
              (e.conc ? " · concentrazione" : "") + (e.fineRound ? " · fino al round " + e.fineRound : "") + (e.ts ? " · TS su " + ({ FOR: "Forza", DES: "Destrezza", COS: "Costituzione", INT: "Intelligenza", SAG: "Saggezza", CAR: "Carisma" }[e.ts.car] || e.ts.car) + " CD " + e.ts.cd + " a fine turno" : "") }),
            master() || suo ? el("button", { class: "piccolo", testo: "Termina", title: "Termina l'effetto (toglie le condizioni e le modifiche che dava)", on: { click: async () => { if (await conferma("Terminare " + e.nome + "?", "Le condizioni e le modifiche date da questo effetto vengono tolte.", "Termina")) comando("effetto.termina", { effetto: e.id }).catch(() => {}); } } }) : null);
        })) : null,
      // Pareggi d'iniziativa (internal/server/iniziativa_pareggi.go): il Master tra PG e mostri, i giocatori tra i loro PG
      vista.pareggi && vista.pareggi.length ? pannelloPareggi(vista.pareggi) : null,
      // Effetti che durano dopo gli scontri (internal/server/effetti_persistenti.go): finiscono al riposo lungo, al «Nuovo
      // giorno» (fino a 24 ore), con la concentrazione persa o con «Termina» (Master; il giocatore per quelli del suo PG).
      vista.durevoli && vista.durevoli.length ? el("div", { class: "iniz-effetti iniz-durevoli", role: "list", "aria-label": "Effetti che durano" },
        el("b", { testo: "⏳ Effetti che durano" }), vista.durevoli.map((e) => el("div", { class: "riga-flex piccolo", role: "listitem" },
          el("span", { testo: e.nome + (e.incantatore ? " (" + e.incantatore + ")" : "") + " → " + e.nomePG +
            (e.condizioni && e.condizioni.length ? ": " + e.condizioni.join(", ").toLowerCase() : "") + (e.testo ? " · " + e.testo : "") +
            (e.conc ? " · concentrazione" : "") + (e.giornaliero ? " · fino al riposo lungo o al nuovo giorno" : " · finché non si termina") }),
          e.mio ? el("button", { class: "piccolo", testo: "Termina", on: { click: async () => {
            if (!(await conferma("Terminare " + e.nome + "?", "Le modifiche e le condizioni date a " + e.nomePG + " vengono tolte.", "Termina"))) return;
            try { await api("/effetti-durevoli/" + encodeURIComponent(e.id), { metodo: "DELETE" }); } catch (er) { avviso(er.message, "errore"); }
          } } }) : null))) : null,
      master() && vista.registro && vista.registro.length ? el("details", { stile: { marginTop: "10px" } }, el("summary", { testo: "Registro dello scontro" }),
        el("ul", { class: "tenue piccolo" }, vista.registro.map((t) => el("li", { testo: t })))) : null));
  }

  // Pareggi: chi decide mette in ordine (▲▼) e conferma; gli altri vedono chi sta decidendo. Un avviso alla prima comparsa.
  const pareggiAvvisati = new Set();
  function pannelloPareggi(lista) {
    return el("div", { class: "iniz-pareggi", role: "region", "aria-label": "Pareggi d'iniziativa" }, lista.map((g) => {
      const chiave = g.valore + ":" + g.membri.map((m) => m.id).join(",");
      if (g.decide && !pareggiAvvisati.has(chiave)) {
        pareggiAvvisati.add(chiave);
        avviso("⚖ Pareggio d'iniziativa a " + g.valore + ": " + (g.chi === "master" && master() ? "decidi tu chi agisce prima." : "decidete chi agisce prima."), "", 7000);
      }
      const ordine = g.membri.slice();
      const elenco = el("ol", { class: "iniz-pari-elenco" });
      const disegnaElenco = () => svuota(elenco, ordine.map((m, i) => el("li", {},
        el("span", { testo: m.nome + (m.mio ? " (tu)" : "") }),
        g.decide ? el("span", { class: "riga-flex" },
          el("button", { class: "piccolo", testo: "▲", "aria-label": "Prima " + m.nome, disabled: i === 0 || undefined, on: { click: () => { ordine.splice(i - 1, 0, ordine.splice(i, 1)[0]); disegnaElenco(); } } }),
          el("button", { class: "piccolo", testo: "▼", "aria-label": "Dopo " + m.nome, disabled: i === ordine.length - 1 || undefined, on: { click: () => { ordine.splice(i + 1, 0, ordine.splice(i, 1)[0]); disegnaElenco(); } } })) : null)));
      disegnaElenco();
      return el("div", { class: "iniz-pari" },
        el("b", { testo: "⚖ Pareggio a " + g.valore }),
        el("p", { class: "tenue piccolo", testo: g.chi === "master" ? (master() ? "Tra un personaggio e una creatura: decidi tu l'ordine." : "Il Master decide chi agisce prima.")
          : master() ? "Tra personaggi: lo decidono i giocatori (puoi deciderlo anche tu)." : "Avete lo stesso valore: mettete in ordine chi agisce prima (vale la prima scelta)." }),
        elenco,
        g.decide ? el("div", { class: "riga-flex" },
          el("button", { class: "primario piccolo", testo: "✓ Conferma l'ordine", on: { click: () => comando("iniz.ordine_pari", { ordine: ordine.map((m) => m.id) }).catch(() => {}) } }),
          master() ? el("button", { class: "piccolo", testo: "Inizia lo stesso", title: "Parte senza decidere: a parità vale la Destrezza", on: { click: () => comando("iniz.avvia", { forza: true }).then((r) => { if (r && r.musica) suonaScontro(r.musica); }).catch(() => {}) } }) : null) : null);
    }));
  }

  function riga(c, turno) {
    const pf = c.pf != null ? c.pf : null, max = c.pfMax || 0;
    const perc = pf != null && max ? Math.max(0, Math.min(100, Math.round(pf / max * 100))) : null;
    const tocca = puoToccare(c);
    const iniz = c.iniziativa != null ? el("span", { class: "iniz-num" + (c.naturale === 20 ? " nat20" : c.naturale === 1 ? " nat1" : ""), testo: c.iniziativa,
      title: c.naturale === 20 ? "20 naturale: agisce per primo" : c.naturale === 1 ? "1 naturale: agisce per ultimo" : c.naturale ? "d20: " + c.naturale : null }) :
      (c.mio || master()) ? el("button", { class: "piccolo", testo: "🎲", title: "Tira l'iniziativa", on: { click: () => comando("iniz.iniziativa", { combattente: c.id, tira: true }) } }) :
      el("span", { class: "iniz-num tenue", testo: "–" });
    const corpoRiga = el("div", { class: "iniz-info" },
      el("div", { class: "riga-flex" },
        el("strong", { testo: c.nome }),
        c.mio ? el("span", { class: "etichetta approvato", testo: "tu" }) : null,
        c.giocatore && !c.mio ? el("span", { class: "tenue piccolo", testo: c.giocatore }) : null,
        c.nascosto ? el("span", { class: "etichetta attesa", testo: "nascosto" }) : null,
        c.ca != null ? el("span", { class: "piccolo" + (c.caOrig != null ? " valore-modificato" : ""), testo: "🛡 " + c.ca + (c.caOrig != null ? " (" + c.caOrig + ")" : ""),
          title: c.caOrig != null ? "CA " + c.ca + " con " + (c.effettiCA || []).join(", ") + "; senza: " + c.caOrig : null }) : null,
        c.percezionePassiva ? el("span", { class: "piccolo tenue", title: "Percezione passiva", testo: "👁 " + c.percezionePassiva }) : null),
      pf != null ? el("div", { class: "barra-pf", title: pf + " / " + max + " PF" }, el("span", { stile: { width: perc + "%", background: perc > 50 ? "var(--ok)" : perc > 0 ? "var(--attenzione)" : "var(--errore)" } })) : null,
      el("div", { class: "riga-flex piccolo" },
        pf != null ? el("span", { testo: "❤ " + pf + " / " + max + (c.pfTemp ? "  (+" + c.pfTemp + " temp.)" : "") }) : el("span", { class: "tenue", testo: c.salute || "" }),
        c.condizioni.map((x) => el("button", { class: "cond", title: CONDIZIONI[x] || x, testo: x, on: { click: () => dialogo(x, el("p", { testo: CONDIZIONI[x] || "" })) } })),
        c.tipo === "pg" && pf === 0 && c.tsMorte ? el("span", { class: "etichetta bloccato", testo: "TS morte ✓" + c.tsMorte.s + " ✗" + c.tsMorte.f }) : null),
      c.terrore ? el("div", { class: "iniz-terrore", testo: "😱 Terrore " + c.terrore + " · " + c.terroreNome }) : null,
      economia(c));
    const comandi = el("div", { class: "riga-flex iniz-azioni" });
    if (tocca) {
      const v = el("input", { type: "number", min: "0", max: "999", inputmode: "numeric", "aria-label": "Valore", class: "num-piccolo", placeholder: "0" });
      const val = () => { const n = parseInt(v.value, 10); if (!(n >= 0)) { avviso("Scrivi un numero.", "errore"); return null; } v.value = ""; return n; };
      const pre = c.tipo === "pg" ? "pg." : "mostro.";
      const pfQui = true;
      comandi.append(...[pfQui ? v : null,
        !pfQui ? null : el("button", { class: "piccolo", testo: "− Danno", on: { click: () => { const n = val(); if (n != null) comando(pre + "danno", { combattente: c.id, valore: n }); } } }),
        !pfQui ? null : el("button", { class: "piccolo", testo: "+ Cura", on: { click: () => { const n = val(); if (n != null) comando(pre + "cura", { combattente: c.id, valore: n }); } } }),
        c.tipo === "pg" && pfQui ? el("button", { class: "piccolo", testo: "Temp.", title: "Imposta i PF temporanei", on: { click: () => { const n = val(); if (n != null) comando("pg.temp", { combattente: c.id, valore: n }); } } }) : null,
        el("button", { class: "piccolo", testo: "Condizioni…", on: { click: () => condizioni(c) } }),
        c.tipo === "pg" && pf === 0 && pfQui ? el("button", { class: "piccolo", testo: "TS morte…", on: { click: () => tiriMorte(c) } }) : null].filter(Boolean)); // append() scriverebbe "null"
    }
    // Attacchi automatici (attacchi.go): il Master con chiunque abbia attacchi, il giocatore con il suo PG nel suo turno.
    const k = vista.combattimento;
    if (k && k.avviato && c.attacchi && c.attacchi.length && (master() || (c.mio && turno))) {
      const per = c.attacchiPerAzione || 1, usati = c.attacchiUsati || 0;
      comandi.prepend(el("button", { class: "piccolo primario iniz-attacca", title: "Attacca: il tiro e i danni li calcola l'app" + (per > 1 ? " (" + per + " attacchi per azione)" : ""),
        testo: "⚔ Attacca" + (per > 1 || usati ? " " + Math.min(usati, per) + "/" + per : ""), on: { click: () => apriAttacco(c) } }));
    }
    // Ira del barbaro (../ui/effetti.js): il giocatore per il suo PG, il Master e il Segretario
    const ira = k && k.avviato && c.tipo === "pg" ? tastoIra(sessione.id, vista, c) : null;
    if (ira) comandi.prepend(ira);
    // Incantesimi della creatura (fase F4: ../ui/incantesimi_mostro.js): slot e usi al giorno, poi la scelta dei bersagli
    if (master() && c.tipo === "mostro" && c.incantesimi && c.incantesimi.gruppi && c.incantesimi.gruppi.length) {
      comandi.prepend(el("button", { class: "piccolo iniz-incantesimi", testo: "✨ Incantesimi", title: "Incantesimi di " + c.nome + ": slot e usi al giorno rimasti",
        on: { click: () => import("../ui/incantesimi_mostro.js").then((x) => x.apriIncantesimiMostro(sessione.id, c)) } }));
    }
    if (master()) {
      comandi.append(...[
        el("button", { class: "piccolo", testo: "Iniz.", title: "Imposta l'iniziativa", on: { click: () => impostaIniziativa(c) } }),
        el("button", { class: "piccolo", testo: "✨ Effetto…", title: "Un effetto che cambia CA, resistenze, TS o attacchi (Scudo della fede, Benedizione…), usato da " + c.nome,
          on: { click: () => apriEffettoManuale(sessione.id, vista, c) } }),
        // Dadi reali: il giocatore ha tirato al tavolo un 20 o un 1; il Master lo segna e l'app reagisce
        // come per un tiro nell'app (CRITICO o OOOPS!!! per tutti; con l'1 il tasto "Che sfortuna" al giocatore).
        c.tipo === "pg" ? el("button", { class: "piccolo reale-critico", testo: "CRITICO", title: "20 naturale con un dado reale", on: { click: () => comando("dadi.reale", { personaggio: c.personaggio, nome: "critico" }).catch(() => {}) } }) : null,
        c.tipo === "pg" ? el("button", { class: "piccolo reale-ooops", testo: "OOOPS!!!", title: "1 naturale con un dado reale", on: { click: () => comando("dadi.reale", { personaggio: c.personaggio, nome: "fallimento" }).catch(() => {}) } }) : null,
        c.tipo === "mostro" ? el("button", { class: "piccolo", testo: c.nascosto ? "Mostra" : "Nascondi", on: { click: () => comando("iniz.nascondi", { combattente: c.id, nascosto: !c.nascosto }) } }) : null,
        el("button", { class: "piccolo", testo: c.tipo === "mostro" ? "🗗 Scheda del mostro" : "🗗 Scheda ridotta", title: c.tipo === "mostro" ? "In un'altra finestra: PF, CA, attacchi e incantesimi automatici e la scheda intera, in tempo reale" : "In un'altra finestra: PF, CA, azioni e attacchi, in tempo reale", on: { click: () => apriRidotta(c) } }),
        c.tipo === "pg" && c.personaggio ? el("button", { class: "piccolo", testo: "🗗 Scheda completa", title: "La scheda intera in un'altra finestra", on: { click: () => apriCompleta(c) } }) : null,
        el("button", { class: "piccolo pericolo", testo: "×", title: "Togli dallo scontro", on: { click: async () => { if (await conferma("Togliere " + c.nome + "?", "Esce dallo scontro.", "Togli", true)) comando("iniz.rimuovi", { combattente: c.id }); } } })].filter(Boolean));
    } else if (c.mio) {
      comandi.append(el("button", { class: "piccolo", testo: "Iniz.", title: "Imposta l'iniziativa", on: { click: () => impostaIniziativa(c) } }));
    }
    // Sanità mentale, Terrore e pazzia (dal ramo main-ke8sy4): il Master per tutti i PG, il giocatore per il suo.
    if (c.tipo === "pg" && c.personaggio && (master() || c.mio)) comandi.append(el("button", { class: "piccolo", testo: "🧠 Sanità", title: "Sanità mentale, Terrore e pazzia", on: { click: () => apriMiniSanita(c.personaggio) } }));
    // Master (controllo del 2026-11-09): in vista solo valore, Danno, Cura (e TS contro la morte quando servono);
    // gli altri comandi stanno nel menu «⋯», così le righe sono basse e lo schermo mostra più combattenti.
    if (master()) {
      const sempre = (b) => b.tagName === "INPUT" || /^(− Danno|\+ Cura|TS morte…|⚔ Attacca.*|✨ Incantesimi|😡 Ira|😡 Fine ira)$/.test(b.textContent);
      const altri = [...comandi.children].filter((b) => !sempre(b));
      if (altri.length > 1) {
        const menu = el("details", { class: "iniz-altro" }, el("summary", { class: "pulsante piccolo", title: "Altri comandi", "aria-label": "Altri comandi di " + c.nome, testo: "⋯" }), el("div", { class: "iniz-altro-menu" }, altri));
        menu.addEventListener("click", (e) => { if (e.target.closest(".iniz-altro-menu button")) menu.open = false; });
        comandi.append(menu);
      }
    }
    return el("div", { class: "riga-iniz" + (turno ? " turno" : "") + (c.mio ? " mio" : ""), "data-comb": c.id, role: "listitem", "aria-current": turno ? "true" : null },
      el("div", { class: "riga-flex", stile: { flexWrap: "nowrap", alignItems: "flex-start" } },
        el("span", { class: "freccia", "aria-hidden": "true", testo: turno ? "▶" : "" }), iniz,
        c.haRitratto ? el("img", { class: "ritratto-iniz", alt: "", src: "/api/v1/personaggi/" + encodeURIComponent(c.personaggio) + "/ritratto" }) : (c.tipo === "pg" && iconaRitratto(c.classe, "ritratto-iniz classe")) || el("span", { class: "ritratto-iniz segnaposto", testo: c.tipo === "pg" ? "🛡" : "👹" }),
        corpoRiga),
      comandi.children.length ? comandi : null);
  }

  // Economia del turno: azione, azione bonus, reazione (si azzerano all'inizio del turno del combattente; gli attacchi
  // automatici segnano da soli azione o reazione). Si toccano: PG dal proprietario, Segretario e Master; mostri dal Master.
  function economia(c) {
    const k = vista.combattimento;
    if (!k || !k.avviato || !c.economia) return null;
    const puo = c.tipo === "pg" ? (gestisce() || c.mio) : master();
    const voci = [["a", "Azione"], ["b", "Azione bonus"], ["r", "Reazione"]];
    return el("div", { class: "iniz-eco riga-flex piccolo", role: "group", "aria-label": "Economia del turno di " + c.nome }, voci.map(([x, nome]) => {
      const usata = !!c.economia[x];
      const t = (usata ? "✗ " : "✓ ") + nome;
      return puo ? el("button", { class: "eco" + (usata ? " usata" : ""), "aria-pressed": usata ? "true" : "false", title: usata ? nome + " già usata: tocca per ridarla" : nome + " disponibile: tocca per segnarla come usata", testo: t,
        on: { click: () => comando("iniz.economia", { combattente: c.id, economia: { ...c.economia, [x]: !usata } }).catch(() => {}) } })
        : el("span", { class: "eco" + (usata ? " usata" : ""), testo: t });
    }));
  }

  // Schede in finestre a parte (fase D): ridotta (../sezioni/combattente.js) e completa (la scheda vera, senza menu).
  const apriRidotta = (c) => apriFinestra("/combattente?s=" + encodeURIComponent(sessione.id) + "&c=" + encodeURIComponent(c.id), "ridotta-" + c.id);
  const apriCompleta = (c) => apriFinestra("/schede/" + encodeURIComponent(c.personaggio), "completa-" + c.personaggio, { larghezza: 1100, altezza: 860 });

  // ----- Finestre -----
  function condizioni(c) {
    const spunte = Object.keys(CONDIZIONI).map((n) => ({ n, i: el("input", { type: "checkbox", selezionato: c.condizioni.includes(n) }) }));
    dialogo("Condizioni di " + c.nome, el("div", { class: "griglia-cond" }, spunte.map(({ n, i }) => el("label", { class: "riga", title: CONDIZIONI[n] }, i, el("span", { testo: n })))),
      { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Salva", on: { click: () => { chiudi(); comando((c.tipo === "pg" ? "pg." : "mostro.") + "condizioni", { combattente: c.id, condizioni: spunte.filter((x) => x.i.checked).map((x) => x.n) }); } } })) });
  }
  function tiriMorte(c) {
    const s = el("input", { type: "number", min: "0", max: "3", valore: c.tsMorte.s }), f = el("input", { type: "number", min: "0", max: "3", valore: c.tsMorte.f });
    dialogo("Tiri salvezza contro la morte · " + c.nome, el("div", {}, el("label", {}, "Successi (3 = stabile)", s), el("label", {}, "Fallimenti (3 = morte)", f),
      el("p", { class: "tenue piccolo", testo: "Un 20 naturale fa tornare a 1 PF; un 1 naturale vale due fallimenti. Una cura azzera i tiri." })),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Salva", on: { click: () => { chiudi(); comando("pg.ts_morte", { combattente: c.id, s: +s.value || 0, f: +f.value || 0 }); } } })) });
  }
  function impostaIniziativa(c) {
    const v = el("input", { type: "number", min: "-10", max: "50", valore: c.iniziativa != null ? c.iniziativa : "" });
    // il d20 tirato a mano, se è un 20 o un 1 (20: agisce per primo, 1: per ultimo)
    const nat = el("select", { "aria-label": "Il d20" }, el("option", { value: "", testo: "un altro numero" }), el("option", { value: "20", testo: "20 naturale (primo)" }), el("option", { value: "1", testo: "1 naturale (ultimo)" }));
    if (c.naturale === 20 || c.naturale === 1) nat.value = String(c.naturale);
    dialogo("Iniziativa · " + c.nome, el("div", {}, el("label", {}, "Valore (d20 + bonus" + (c.bonusIniziativa != null ? ", bonus " + (c.bonusIniziativa >= 0 ? "+" : "") + c.bonusIniziativa : "") + ")", v),
      el("label", {}, "Sul d20 è uscito", nat)),
      { azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { testo: "🎲 Tira per me", on: { click: () => { chiudi(); comando("iniz.iniziativa", { combattente: c.id, tira: true }); } } }),
        el("button", { class: "primario", testo: "Salva", on: { click: () => { chiudi(); comando("iniz.iniziativa", { combattente: c.id, valore: parseInt(v.value, 10), ...(nat.value ? { naturale: +nat.value } : {}) }); } } })) });
  }
  function aggiungiMostri() {
    const nome = el("input", { maxlength: "40", placeholder: "Es. Lupo crudele" });
    const quanti = el("input", { type: "number", min: "1", max: "20", valore: "1" });
    const ca = el("input", { type: "number", min: "0", max: "40", valore: "13" });
    const pf = el("input", { type: "number", min: "1", max: "2000", valore: "10" });
    const bonus = el("input", { type: "number", min: "-10", max: "20", valore: "0" });
    const gs = el("input", { type: "number", min: "0", max: "30", step: "0.125", valore: "0", title: "Grado di Sfida: con le ricompense automatiche il gruppo riceve GS × 10 mo quando il mostro scende a 0 PF" });
    const nasc = el("input", { type: "checkbox" });
    const err = el("p", { class: "errore" });
    // Dal bestiario della mappa (voci con scheda SRD): compila CA, PF e bonus d'iniziativa.
    const lista = el("datalist", { id: "lista-bestiario" });
    nome.setAttribute("list", "lista-bestiario");
    // Elenco riempito prima di mostrare la finestra (il bestiario è già in memoria dall'apertura del tracker).
    bestiario().then((v) => { if (!nome.value) lista.replaceChildren(...v.map((x) => el("option", { value: x.nome }))); }).catch(() => {});
    // Attacchi e difese letti dalla scheda del mostro (web/regole/attacchi_mostri.js): servono agli attacchi automatici.
    let profilo = null, schedaIntera = null; // scheda intera della creatura: per la finestra del mostro
    const infoAtt = el("p", { class: "tenue piccolo", hidden: true });
    nome.addEventListener("change", async () => {
      const v = (await bestiario().catch(() => [])).find((x) => x.nome.toLowerCase() === nome.value.trim().toLowerCase());
      profilo = v ? v.profilo : null;
      schedaIntera = v ? v.scheda : null;
      infoAtt.hidden = !profilo;
      if (profilo) infoAtt.textContent = "⚔ Dalla scheda: " + (profilo.attacchi.length ? profilo.attacchi.map((a) => a.nome).join(", ") + (profilo.multi > 1 ? " · Multiattacco: " + profilo.multi + " attacchi" : "") : "nessun attacco con tiro per colpire") +
        (profilo.difese.res.length + profilo.difese.imm.length + profilo.difese.vuln.length ? " · difese ai danni comprese" : "");
      if (!v) return;
      ca.value = v.ca; pf.value = v.pf; bonus.value = v.bonus; gs.value = v.gs;
    });
    dialogo("Aggiungi mostri", el("div", {}, lista, el("label", {}, "Nome (scrivi per cercare nel bestiario)", nome), el("div", { class: "griglia-4" },
      el("label", {}, "Quanti", quanti), el("label", {}, "CA", ca), el("label", {}, "PF", pf), el("label", {}, "Bonus iniziativa", bonus), el("label", {}, "GS (ricompensa)", gs)),
      el("label", { class: "riga" }, nasc, el("span", { testo: "Nascosti ai giocatori (li sveli quando vuoi)" })),
      el("p", { class: "tenue piccolo", testo: "Scegliendo una creatura del bestiario CA, PF e bonus si compilano dalla scheda; puoi cambiarli prima di aggiungere." }), infoAtt, err),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Aggiungi", on: { click: async () => {
        try { await tempoReale.chiedi("iniz.aggiungi_mostri", { sessione: sessione.id, nome: nome.value.trim(), quanti: +quanti.value, ca: +ca.value, pf: +pf.value, bonusIniziativa: +bonus.value, gs: Math.max(0, parseFloat(gs.value) || 0), nascosto: nasc.checked, profilo, scheda: schedaIntera }); chiudi(); }
        catch (e) { err.textContent = e.message; }
      } } })) });
  }
  // Creature del bestiario con scheda: nome, CA, PF medi, bonus d'iniziativa (dalla Destrezza).
  let cacheBestiario = null;
  function bestiario() {
    // bestiario dell'avventura + catalogo dei mostri (mostri.json): CA, PF, bonus d'iniziativa e GS si compilano da soli
    const leggi = (f) => fetch(f, { credentials: "same-origin" }).then((r) => (r.ok ? r.json() : { voci: [] })).catch(() => ({ voci: [] }));
    if (!cacheBestiario) cacheBestiario = Promise.all([leggi("/carta/bestiario.json"), leggi("/carta/mostri.json")]).then(([a, c]) => ({ voci: a.voci.concat(c.voci || []) })).then((j) =>
      j.voci.filter((v) => v.scheda).map((v) => ({ nome: v.nome, ca: parseInt(v.scheda.ca, 10) || 10, pf: parseInt(v.scheda.pf, 10) || 1,
        bonus: Math.floor(((v.scheda.car || [])[1] || 10) / 2) - 5, gs: valoreGS(v.scheda.gs), profilo: profiloMostro(v.scheda), scheda: v.scheda })));
    return cacheBestiario;
  }
  async function aggiungiPG() {
    let pg = [];
    try {
      const s = await api("/sessioni/" + sessione.id);
      const presenti = new Set(vista.combattenti.map((c) => c.personaggio).filter(Boolean));
      const membri = new Set(s.partecipanti.map((p) => p.utente));
      pg = (await api("/personaggi")).filter((p) => p.stato === "attivo" && !presenti.has(p.id) && membri.has(p.proprietario));
    } catch (e) { avviso(e.message, "errore"); return; }
    dialogo("Aggiungi un personaggio", pg.length ? el("div", {}, pg.map((p) => el("button", { class: "pulsante", stile: { width: "100%", justifyContent: "flex-start", marginBottom: "6px" },
      testo: p.nome + " · " + p.nomeGiocatore + (p.riepilogo && p.riepilogo.classe ? " · " + p.riepilogo.classe : ""),
      on: { click: () => { document.querySelectorAll(".velo").forEach((v) => v.remove()); comando("iniz.aggiungi_pg", { personaggio: p.id }); } } }))) :
      el("p", { class: "tenue", testo: "Nessun altro personaggio attivo tra i partecipanti della sessione." }));
  }

  // ----- Attacchi automatici (internal/server/attacchi.go, finestre in ../ui/attacco.js) -----
  const { apriAttacco, avvisoEsito } = creaAttacchi({ sessione: () => sessione.id, vista: () => vista, master });
  // effetti visivi su danni, cure e critici (../ui/effetti_colpi.js)
  const fx = creaEffettiColpi(() => zonaTracker);

  // ----- Pannello laterale: il mio personaggio, messaggi -----
  function disegnaLato() {
    const mio = vista.combattenti.find((c) => c.mio);
    const pannelli = [];
    if (mio && !master()) {
      pannelli.push(el("section", { class: "scheda-ui" }, el("h2", { testo: "Il tuo personaggio" }),
        el("p", {}, el("strong", { testo: mio.nome }), " · ❤ " + mio.pf + " / " + mio.pfMax + (mio.pfTemp ? " (+" + mio.pfTemp + ")" : "")),
        el("div", { class: "riga-flex" },
          el("a", { href: "/schede/" + encodeURIComponent(mio.personaggio), "data-interno": true, class: "pulsante piccolo", testo: "📜 Apri la scheda" }),
          el("button", { class: "piccolo", testo: "🗗 Scheda ridotta", title: "In un'altra finestra: PF, azioni e attacchi, in tempo reale", on: { click: () => apriRidotta(mio) } }),
          el("button", { class: "piccolo", testo: "🗗 Scheda completa", title: "La scheda intera in un'altra finestra", on: { click: () => apriCompleta(mio) } }),
          el("button", { class: "piccolo", testo: "Riposo lungo", on: { click: async () => { if (await conferma("Riposo lungo?", "PF al massimo, PF temporanei e tiri contro la morte azzerati, slot recuperati.", "Riposa")) comando("pg.riposo_lungo", { combattente: mio.id }); } } }))));
    }
    // Messaggi privati. Master (2026-11-11): una finestra per ogni giocatore (web/ui/chat.js), sempre visibili insieme;
    // qui l'elenco dei giocatori della sessione con i non letti. Giocatore: la sua conversazione con il Master.
    if (master()) {
      const giocatori = new Map();
      messaggi.forEach((m) => { const altro = m.mio ? [m.a, m.aNome] : [m.da, m.daNome]; giocatori.set(altro[0], altro[1]); });
      (sessione.partecipanti || []).forEach((p) => giocatori.set(p.utente, p.nome));
      const titoloS = sessione.nomeGruppo + " · " + sessione.titolo;
      pannelli.push(el("section", { class: "scheda-ui" }, el("h2", { testo: "💬 Messaggi privati" }),
        giocatori.size ? el("div", { class: "chat-elenco" }, [...giocatori].map(([id, n]) => {
          const nl = Math.max(nonLettiDi(sessione.id, id), messaggi.filter((m) => m.da === id && !m.letto).length);
          return el("button", { class: "piccolo" + (nl ? " primario" : ""), on: { click: () => apriChat(sessione.id, id, n, titoloS) } }, "💬 " + n, nl ? el("span", { class: "distintivo", testo: String(nl) }) : null);
        })) : el("p", { class: "tenue piccolo", testo: "Nessun giocatore nella sessione." }),
        el("p", { class: "tenue piccolo", testo: "Ogni giocatore ha la sua finestra in basso a destra, visibile in tutte le pagine: le conversazioni restano separate." })));
    } else {
    // Il pannello si ricostruisce a ogni aggiornamento della battaglia: il testo in scrittura, il cursore e
    // la selezione della casella si conservano (prima un danno o un cambio di turno cancellavano la bozza).
    const prima = zonaLato.querySelector("textarea");
    const conFuoco = prima && document.activeElement === prima ? [prima.selectionStart, prima.selectionEnd] : null;
    const scrivi = el("textarea", { maxlength: "1000", rows: "2", placeholder: master() ? "Messaggio privato…" : "Messaggio privato al Master…", stile: { minHeight: "60px" } });
    scrivi.value = bozza;
    scrivi.addEventListener("input", () => { bozza = scrivi.value; });
    let dest = null;
    if (master()) {
      const giocatori = new Map();
      vista.combattenti.forEach(() => {});
      messaggi.forEach((m) => { const altro = m.mio ? [m.a, m.aNome] : [m.da, m.daNome]; giocatori.set(altro[0], altro[1]); });
      (sessione.partecipanti || []).forEach((p) => giocatori.set(p.utente, p.nome));
      if (!conv || !giocatori.has(conv)) conv = [...giocatori.keys()][0] || "";
      dest = el("select", { "aria-label": "A chi", on: { change: () => { conv = dest.value; disegnaLato(); } } },
        [...giocatori].map(([id, n]) => el("option", { value: id, testo: n, selected: id === conv })));
    }
    const visibili = master() ? messaggi.filter((m) => m.a === conv || m.da === conv) : messaggi;
    const elenco = el("div", { class: "messaggi" }, visibili.length ? visibili.map((m) => el("div", { class: "msg" + (m.mio ? " mio" : "") },
      el("div", { class: "tenue piccolo", testo: (m.mio ? "Tu" : m.daNome) + " · " + quando(m.quando) }), el("div", { testo: m.testo }))) :
      el("p", { class: "tenue piccolo", testo: "Nessun messaggio. Solo tu e " + (master() ? "il giocatore" : "il Master") + " li vedete." }));
    const invia = async () => {
      const t = scrivi.value.trim(); if (!t) return;
      try {
        await tempoReale.chiedi("messaggio.invia", { sessione: sessione.id, testo: t, a: conv });
        if (bozza.trim() === t) bozza = "";
        const ora = zonaLato.querySelector("textarea"); // può essere già un'altra casella, se il pannello è stato ridisegnato
        if (ora && ora.value.trim() === t) ora.value = "";
      }
      catch (e) { avviso(e.message, "errore"); }
    };
    scrivi.addEventListener("keydown", (e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); invia(); } });
    pannelli.push(el("section", { class: "scheda-ui" }, el("h2", {}, "💬 Messaggi privati ", nonLetti ? el("span", { class: "distintivo", testo: nonLetti }) : null),
      dest, elenco, scrivi, el("div", { class: "riga-flex", stile: { marginTop: "6px" } }, el("button", { class: "primario piccolo", testo: "Invia", on: { click: invia } }))));
    if (conFuoco) setTimeout(() => { scrivi.focus(); scrivi.setSelectionRange(conFuoco[0], conFuoco[1]); }, 0);
    setTimeout(() => { elenco.scrollTop = elenco.scrollHeight; }, 0);
    // Segna come letti quelli ricevuti e visibili.
    visibili.filter((m) => !m.mio && !m.letto).forEach((m) => { m.letto = true; tempoReale.chiedi("messaggio.letto", { sessione: sessione.id, messaggio: m.id }).catch(() => {}); });
    nonLetti = 0;
    }
    // Dadi: il pannello si crea una volta per sessione e si riusa, così dadi scelti e risultati restano.
    if (!pannelloD || pannelloD.dataset.sessione !== sessione.id) {
      pannelloD = el("details", { class: "scheda-ui iniz-dadi" }, el("summary", { testo: "🎲 Dadi" }), pannelloDadi({ sessione: sessione.id, master: master() }));
      pannelloD.dataset.sessione = sessione.id;
    }
    pannelli.push(pannelloD);
    svuota(zonaLato, pannelli);
  }

  // ----- Eventi in tempo reale -----
  const smetti = [
    tempoReale.su("iniz.stato", (v) => { if (v.sessione !== sessione.id) return; const prima = vista; vista = v; disegnaTracker(); disegnaLato(); fx.vista(prima, v); }),
    tempoReale.su("attacco.esito", (e) => { avvisoEsito(e); if (e && e.sessione === sessione.id) fx.esito(e); }),
    tempoReale.su("iniz.turno", (d) => {
      if (d.sessione !== sessione.id) return;
      avviso("⚔ È il tuo turno, " + d.nome + "!", "", 6000);
      try { suonoTurno.currentTime = 0; suonoTurno.play().catch(() => {}); } catch { /* audio non disponibile */ }
      if (navigator.vibrate) navigator.vibrate([200, 100, 200]);
    }),
    tempoReale.su("messaggio.nuovo", (d) => {
      if (d.sessione !== sessione.id) return;
      messaggi.push(d.messaggio);
      if (!d.messaggio.mio) nonLetti++; // suono, avviso e finestre: web/ui/chat.js (per tutte le pagine)
      if (master()) setTimeout(disegnaLato, 50); else disegnaLato(); // (il Master: dopo che chat.js ha aggiornato i non letti)
    }),
  ];
  const riaperto = () => entra(); // dopo una riconnessione si rientra nella sessione
  tempoReale.stato.addEventListener("aperto", riaperto);
  try { sessione.partecipanti = (await api("/sessioni/" + sessione.id)).partecipanti; } catch { /* non indispensabile */ }
  await entra();
  // Il Master carica subito l'elenco del bestiario: quando apre "＋ Mostri" i suggerimenti sono già pronti
  // (se arrivassero mentre scrive, il browser riaprirebbe l'elenco sotto il campo successivo).
  if (me.ruolo === "master") bestiario().catch(() => {});
  return () => { smetti.forEach((f) => f()); tempoReale.stato.removeEventListener("aperto", riaperto); musicaAttesa.pause(); fermaScontro(); if (canaleMusica) canaleMusica.close(); clearInterval(gkTimer); };
}

/** GS dal testo del bestiario ("1/4 (50 PE)", "5 (1.800 PE)") a numero. */
function valoreGS(t) {
  const m = String(t || "").match(/^\s*(\d+)(?:\s*\/\s*(\d+))?/);
  if (!m) return 0;
  return m[2] ? +m[1] / +m[2] : +m[1];
}
