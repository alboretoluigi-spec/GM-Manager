// Sezione "Schede": elenco dei personaggi e scheda aperta.
// La scheda 5e (web/scheda/) gira in un riquadro e dialoga con questa pagina tramite postMessage
// (protocollo in 01_scheda/INTEGRAZIONE.md). Questa pagina la salva sul server inviando solo i
// campi cambiati e gestisce versioni, conflitti, PF dal tracker e background privato.
import { el, svuota, api, avviso, dialogo, conferma, quando, tempoReale, applicaTema } from "../ui/ui.js";
import { iconaRitratto, iconePiccole } from "../ui/classi.js";
import { apriMiniSanita } from "../ui/sanita_mini.js";
import { nomeCampo, mostraValore, appiattisci, ricomponi } from "../ui/campi_scheda.js";
import { copia, salvaCopia, copieDi } from "../ui/offline.js";
import { separa, quante, tabellaVoci, mostraRichiesta } from "../ui/modifiche_manuali.js";
export { appiattisci, ricomponi }; // usati anche da altre pagine // la Sanità mentale è una funzione separata dalla scheda

const STATI = { attivo: "attivo", ritirato: "ritirato", morto: "morto", archiviato: "archiviato", anonimo: "anonimo" };

export async function mostra(corpo, ctx) {
  const id = location.pathname.split("/")[2] || "";
  return id ? scheda(corpo, ctx, decodeURIComponent(id)) : elenco(corpo, ctx);
}

// ===================== Elenco =====================

async function elenco(corpo, { io, azioni, vai }) {
  const master = io.utente.ruolo === "master";
  const contenitore = el("div", { class: "largo" });
  svuota(corpo, contenitore);
  svuota(azioni, el("button", { class: "primario piccolo", testo: "＋ Nuovo personaggio", on: { click: nuovo } }));

  let giocatori = [], plugin = [];
  async function disegna() {
    plugin = (await api("/schede-plugin").catch(() => ({ lista: [] }))).lista.filter((x) => x.abilitato);
    let pg;
    try {
      [pg, giocatori] = await Promise.all([api("/personaggi"), master ? api("/utenti") : Promise.resolve([])]);
    } catch (e) {
      // Senza rete: le schede di cui il dispositivo ha una copia (offline.js).
      const copie = e.stato === 0 ? copieDi(io.utente.id) : [];
      if (!copie.length) { svuota(contenitore, el("p", { class: "errore", testo: e.stato === 0 && io.offline ? "Su questo dispositivo non c'è ancora nessuna copia delle tue schede: apri la scheda almeno una volta con la connessione." : e.message })); return; }
      pg = copie.map((c) => ({ ...c.personaggio, haRitratto: false, offline: true, pendente: c.pendente }));
    }
    if (!pg.length) {
      svuota(contenitore, el("section", { class: "scheda-ui" }, el("h2", { testo: "Nessun personaggio" }),
        el("p", { class: "tenue", testo: master
          ? "I giocatori creano i loro personaggi da qui. Puoi anche crearne uno per conto di un giocatore con «＋ Nuovo personaggio»."
          : "Crea il tuo personaggio con «＋ Nuovo personaggio»: la scheda viene salvata sul PC del Master e la ritrovi da qualsiasi dispositivo." })));
      return;
    }
    const gruppiPer = master ? raggruppa(pg, (p) => p.nomeGiocatore || "Senza giocatore (anonimo)") : [["", pg]];
    svuota(contenitore, gruppiPer.map(([titolo, l]) => el("section", { class: "scheda-ui" },
      titolo ? el("h2", { testo: titolo }) : null,
      el("div", { class: "griglia" }, l.map(carta)))));
  }

  function carta(p) {
    const r = p.riepilogo || {};
    // Con il multiclasse "classe" è già "Chierico 3 / Mago 2": il livello si aggiunge solo per una classe sola.
    const cl = r.classe && (r.classe.includes("/") ? r.classe : r.classe + (r.livello ? " " + r.livello : ""));
    const descr = [r.razza, cl, r.sottoclasse].filter(Boolean).join(" · ");
    return el("a", { href: "/schede/" + encodeURIComponent(p.id), "data-interno": true, class: "carta-pg" + (p.stato !== "attivo" ? " spento" : "") },
      p.haRitratto ? el("img", { src: "/api/v1/personaggi/" + encodeURIComponent(p.id) + "/ritratto?v=" + p.versione, alt: "" }) :
        iconaRitratto(r.classe, "ritratto-classe") || el("div", { class: "segnaposto-ritratto", testo: (p.nome || "?").slice(0, 1).toUpperCase() }),
      el("div", {},
        el("strong", { testo: p.nome }),
        el("div", { class: "tenue piccolo" }, iconePiccole(r.classe), descr || "Scheda da completare"),
        el("div", { class: "piccolo riga-flex", stile: { marginTop: "4px" } },
          r.ca ? el("span", { testo: "CA " + r.ca }) : null,
          r.pfMax ? el("span", { testo: "PF " + r.pfMax }) : null,
          p.stato !== "attivo" ? el("span", { class: "etichetta", testo: STATI[p.stato] || p.stato }) : null,
          p.pendente ? el("span", { class: "etichetta", testo: "📴 da inviare", title: "Modifiche fatte senza connessione, ancora su questo dispositivo" }) : null,
          p.haBackground ? el("span", { class: "etichetta", testo: "background", title: "Ha caricato il background privato" }) : null,
          p.scheda ? el("span", { class: "etichetta", testo: "🧩 " + ((plugin.find((x) => x.id === p.scheda) || {}).nome || p.scheda), title: "Usa una scheda alternativa" }) : null)));
  }

  function nuovo() {
    const nome = el("input", { maxlength: "60", placeholder: "Es. Arin" });
    const chi = master ? el("select", {}, el("option", { value: "", testo: "Me stesso (PNG del Master)" }),
      giocatori.filter((g) => g.stato === "approvato").map((g) => el("option", { value: g.id, testo: g.nome }))) : null;
    const err = el("p", { class: "errore" });
    // Schede alternative (plugin) abilitate dal Master: si sceglie qui, una volta per personaggio.
    const tipo = plugin.length ? el("select", { "aria-label": "Scheda" }, el("option", { value: "", testo: "Scheda di Gaster Bless (consigliata)" }),
      plugin.map((x) => el("option", { value: x.id, testo: "🧩 " + x.nome + (x.autore ? " (di " + x.autore + ")" : "") }))) : null;
    dialogo("Nuovo personaggio", el("div", {}, el("label", {}, "Nome del personaggio", nome),
      chi ? el("label", {}, "Giocatore", chi) : null,
      tipo ? el("label", {}, "Scheda", tipo) : null,
      tipo ? el("p", { class: "tenue piccolo", testo: "Le schede alternative sono fatte da altri giocatori. La scelta vale per questo personaggio: le due schede non si scambiano i dati." }) : null,
      el("p", { class: "tenue piccolo", testo: "Classe, razza e il resto si scelgono nella scheda. Una scheda esportata in passato si carica con il pulsante «Importa» della scheda." }), err),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Crea", on: { click: async () => {
        try {
          const r = await api("/personaggi", { dati: { nome: nome.value.trim(), proprietario: chi ? chi.value : "", scheda: tipo ? tipo.value : "" } });
          chiudi(); vai("/schede/" + encodeURIComponent(r.id));
        } catch (e) { err.textContent = e.message; }
      } } })) });
  }

  const via1 = tempoReale.su("scheda.aggiornata", () => disegna());
  const via2 = tempoReale.su("scheda.eliminata", () => disegna());
  const smetti = () => { via1(); via2(); };
  await disegna();
  return smetti;
}

function raggruppa(l, chiave) {
  const m = new Map();
  for (const x of l) { const k = chiave(x); if (!m.has(k)) m.set(k, []); m.get(k).push(x); }
  return [...m.entries()];
}

// ===================== Scheda aperta =====================

async function scheda(corpo, { io, titolo, azioni, vai }) {
  const io_ = io.utente;
  let info;
  let senzaRete = false;   // scheda aperta dalla copia del dispositivo (offline.js)
  const mia = (p) => p && p.proprietario === io_.id; // si tengono copie solo delle proprie schede
  const locale = copia(io_.id, id_());
  try { info = await api("/personaggi/" + encodeURIComponent(id_())); } catch (e) {
    if (e.stato === 0 && locale) { info = { personaggio: locale.personaggio, dati: locale.dati, modificabile: locale.modificabile }; senzaRete = true; }
    else {
      svuota(corpo, el("div", { class: "largo" }, el("p", { class: "errore", testo: e.message }), el("a", { href: "/schede", "data-interno": true, testo: "← Torna alle schede" })));
      return;
    }
  }
  const pg = info.personaggio;
  let base = pg.versione;               // versione da cui parte la pagina
  let salvato = appiattisci(info.dati); // ultimo stato confermato dal server
  let modificabile = info.modificabile;
  let ultimoStato = null, invio = null, attesa = 0, inConflitto = false;
  // Modifiche fatte senza rete: data della prima mancata consegna; dopo 60 secondi (o se riprese dalla copia del
  // dispositivo) partono con il segno «offline» e il Master ne sarà avvisato all'accesso in sessione.
  let offlineDal = 0, ripresaDaCopia = false;
  if (locale && locale.pendente && locale.stato && !info.personaggio.scheda) {
    salvato = locale.salvato || salvato; base = locale.base || base; offlineDal = locale.offlineDal || Date.now(); ripresaDaCopia = true;
    info = { ...info, dati: locale.stato };
  }
  const aggiornaCopia = (extra = {}) => {
    if (!mia(pg) || pg.scheda) return; // niente copie di schede altrui né delle schede alternative
    salvaCopia(io_.id, pg.id, { personaggio: { ...pg, versione: base }, dati: ricomponi(salvato), modificabile, salvato, base,
      pendente: false, stato: null, offlineDal: 0, ...extra });
  };
  if (!senzaRete && !ripresaDaCopia) aggiornaCopia();

  // ----- Modifica manuale (2026-11-11, ../ui/modifiche_manuali.js) -----
  // Con l'approvazione del Master attiva il giocatore salva da solo solo i campi di gioco; gli altri li cambia con
  // «✏ Modifica manuale»: restano una bozza su questo dispositivo e, spegnendo il tasto, vanno al Master.
  let approvazione = false, richiesta = null;
  if (!senzaRete && modificabile) {
    try { const x = await api("/personaggi/" + encodeURIComponent(id_()) + "/richiesta-modifiche"); approvazione = !!x.approvazione && mia(pg); richiesta = x.richiesta; } catch { /* come prima */ }
  }
  const chiaveBozza = "gasterbless-manuale:" + io_.id + ":" + pg.id;
  const leggiBozza = () => { try { const b = JSON.parse(localStorage.getItem(chiaveBozza) || "null"); return b && b.modifiche ? b : null; } catch { return null; } };
  const scriviBozza = (b) => { try { if (b) localStorage.setItem(chiaveBozza, JSON.stringify(b)); else localStorage.removeItem(chiaveBozza); } catch { /* spazio pieno: la bozza resta solo in questa pagina */ } };
  let manuale = approvazione && !!leggiBozza();
  if (manuale) { // bozza rimasta da prima (pagina chiusa con il tasto acceso): si riapplica sulla scheda valida
    const b = leggiBozza(), p = { ...appiattisci(info.dati) };
    for (const [k, v] of Object.entries(b.modifiche)) p[k] = JSON.stringify(v);
    for (const k of b.rimossi || []) delete p[k];
    info = { ...info, dati: ricomponi(p) };
  }
  if (!approvazione) scriviBozza(null);
  let riepilogoInviato = false; // il riepilogo si manda almeno una volta per apertura, anche senza modifiche

  // Scheda alternativa (plugin): gira isolata, con origine opaca; se non è più abilitata non si apre.
  const elencoPlugin = (await api("/schede-plugin").catch(() => ({ lista: [] }))).lista;
  const plugin = pg.scheda ? elencoPlugin.find((x) => x.id === pg.scheda && x.abilitato) : null;
  const pluginAbilitati = elencoPlugin.filter((x) => x.abilitato);

  titolo.textContent = pg.nome;
  const statoSalv = el("span", { class: "tenue piccolo", "aria-live": "polite" });
  const aggiornaStato = (t) => { statoSalv.textContent = t; };
  aggiornaStato(modificabile ? "Salvata sul PC del Master ✓ · versione " + base : "Solo consultazione (" + (STATI[pg.stato] || pg.stato) + ")");
  const btnManuale = approvazione ? el("button", { class: "piccolo", on: { click: () => (manuale ? spegniManuale() : accendiManuale()) } }) : null;
  const fascia = el("div", { class: "mm-fascia", hidden: true, "aria-live": "polite" });
  svuota(azioni, statoSalv, btnManuale,
    // solo nei giochi che hanno la Sanità mentale (interfaccia adattiva, C4a)
    !(io.gioco && !io.gioco.sezioni.includes("sanita")) && el("button", { class: "piccolo", testo: "🧠 Sanità mentale", title: "Valore, tiro salvezza, Terrore, follia e condizioni", on: { click: () => apriMiniSanita(pg.id) } }),
    el("button", { class: "piccolo", testo: "Storico", on: { click: storico } }),
    el("button", { class: "piccolo", testo: "Stato…", on: { click: cambiaStato } }),
    io_.ruolo === "master" ? el("button", { class: "piccolo", testo: "👤 Assegna…", title: "Assegna la scheda a un giocatore", on: { click: assegna } }) : null,
    io_.ruolo === "master" ? el("button", { class: "piccolo pericolo", testo: "🗑 Elimina…", title: "Elimina definitivamente la scheda", on: { click: elimina } }) : null,
    io_.ruolo === "master" ? el("button", { class: "piccolo", testo: "🧩 Tipo di scheda…", on: { click: cambiaTipo } }) : null,
    el("a", { href: "/schede", "data-interno": true, class: "pulsante piccolo", testo: "← Schede" }));

  if (pg.scheda && !plugin) {
    svuota(corpo, el("div", { class: "largo" }, el("section", { class: "scheda-ui" },
      el("h2", { testo: "🧩 Scheda alternativa non disponibile" }),
      el("p", { testo: "Questo personaggio usa la scheda alternativa «" + pg.scheda + "», che ora non è abilitata o non è più installata." }),
      el("p", { class: "tenue", testo: io_.ruolo === "master" ? "Abilitala in Impostazioni → Schede alternative, oppure cambia il tipo di scheda con «Tipo di scheda…» (i dati salvati restano)." : "Chiedi al Master di abilitarla." }))));
    return;
  }
  const libera = !plugin && !!(info.gioco && info.gioco.modulo && info.gioco.modulo !== "d20-5e");
  const frame = plugin
    ? el("iframe", { class: "riquadro-scheda", title: "Scheda di " + pg.nome + " (" + plugin.nome + ")", src: plugin.url, sandbox: "allow-scripts allow-modals allow-forms allow-downloads" })
    : el("iframe", { class: "riquadro-scheda", title: "Scheda di " + pg.nome,
      // gioco diverso dalla quinta edizione: la scheda libera con il modello della campagna (fase 3c, C4b)
      src: (libera ? "/scheda_libera/" : "/scheda/") + "?host=" + encodeURIComponent(location.origin) });
  svuota(corpo, fascia, frame);
  disegnaManuale();
  // Il plugin ha un'origine opaca ("null"): i messaggi si accettano solo dal suo riquadro e gli si rispondono
  // solo al suo riquadro.
  const invia = (m) => frame.contentWindow && frame.contentWindow.postMessage(m, plugin ? "*" : location.origin);
  // Incantesimi importati dai manuali del Master (2026-11-11, «📥 Importa dai manuali»): solo i valori, per la scheda di Gaster Bless.
  const incantesimiImportati = plugin ? Promise.resolve(null) : api("/regole/incantesimi").catch(() => null);

  function temaScheda() {
    const t = document.documentElement.dataset.tema;
    if (t === "chiaro") return "light";
    if (t === "scuro") return "dark";
    return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  let ultimiEffetti = "null"; // effetti in corso già mandati alla scheda
  async function messaggio(e) {
    if (e.source !== frame.contentWindow || e.origin !== (plugin ? "null" : location.origin)) return;
    const m = e.data || {};
    switch (m.type) {
      case "scheda:ready": {
        const incantesimi = await incantesimiImportati;
        invia({ type: "scheda:init", role: io_.ruolo === "master" ? "master" : "player", playerId: pg.id, playerName: io_.nome, theme: temaScheda(), state: info.dati,
          ...(incantesimi && incantesimi.length ? { incantesimi } : {}) });
        if (approvazione && !plugin) invia({ type: "scheda:blocco", bloccata: !manuale });
        if (info.effetti) { ultimiEffetti = JSON.stringify(info.effetti); invia({ type: "scheda:effetti", effetti: info.effetti }); } // CA cambiata da effetti in corso
        if (ripresaDaCopia && !senzaRete) { // modifiche rimaste sul dispositivo: partono ora
          ultimoStato = { state: info.dati, summary: null };
          aggiornaStato("📶 Invio delle modifiche fatte senza connessione…");
          clearTimeout(attesa); attesa = setTimeout(salva, 300);
        }
        // La scheda completa da sé i campi mancanti con i valori predefiniti: non è una modifica,
        // quindi non deve creare una versione solo perché qualcuno l'ha aperta. Si legge lo stato
        // appena caricato (la scheda è servita dallo stesso server, quindi è raggiungibile) e si
        // considerano "già salvati" i campi aggiunti.
        if (!plugin) setTimeout(() => {
          try {
            const iniziale = appiattisci(frame.contentWindow.SchedaAPI.getState());
            const predefiniti = appiattisci(frame.contentWindow.blank());
            for (const [k, v] of Object.entries(iniziale)) {
              if (k in salvato || Object.keys(salvato).some((x) => x.startsWith(k + "."))) continue;
              if (predefiniti[k] === v) salvato[k] = v; // solo i valori predefiniti, mai una scelta dell'utente
            }
          } catch { /* scheda non raggiungibile: al primo salvataggio si salveranno anche i predefiniti */ }
        }, 50);
        break;
      }
      case "scheda:tiroLibero": { // scheda libera: formula già con i valori (es. «2d6 + 3»), tirata dal server
        const t = dadiDaFormula(String(m.formula || ""));
        if (t.errore) { invia({ type: "scheda:tiroErrore", errore: t.errore }); break; }
        tempoReale.chiedi("dadi.tira", { dadi: t.dadi, mod: t.mod, modo: "normale", etichetta: String(m.etichetta || "").slice(0, 40), scheda: true, personaggio: pg.id })
          .then((r) => invia({ type: "scheda:tiroRisultato", r })).catch((e) => invia({ type: "scheda:tiroErrore", errore: e.message }));
        break;
      }
      case "scheda:tiroMeccanica": { // scheda libera: tiro con la meccanica del gioco della campagna (C4c)
        const mod = info.gioco && info.gioco.modulo;
        import("../ui/meccaniche.js").then(({ pannelloMeccanica, haMeccanica }) => {
          if (!haMeccanica(mod)) { invia({ type: "scheda:tiroErrore", errore: "Questo gioco non ha un tiro dedicato: nel modello usa una formula." }); return; }
          let chiudi = null;
          chiudi = dialogo("🎲 " + (String(m.etichetta || "") || "Tiro"), pannelloMeccanica(mod, { valore: parseInt(m.valore, 10) || 0, etichetta: String(m.etichetta || "").slice(0, 40),
            personaggio: pg.id, scheda: true, master: io_.ruolo === "master", dopo: (r) => { if (chiudi) chiudi(); invia({ type: "scheda:tiroRisultato", r }); } }));
        });
        break;
      }
      case "scheda:tiro": // clic su un tiro salvezza, un'abilità o una prova: si chiede come tirare (sceltaTiro)
        sceltaTiro(m);
        break;
      case "scheda:incantesimo": // "Lancia incantesimo" o "Rituale": la descrizione va sugli schermi del tavolo
        // Con un effetto automatico e lo scontro in corso si apre la scelta dei bersagli (../ui/incantesimo_scontro.js).
        if (m.inc && typeof m.inc === "object") tempoReale.chiedi("incantesimo.lancia", { personaggio: pg.id, inc: m.inc }).then((r) => {
          if (r && r.scontro && m.effetto && typeof m.effetto === "object") import("../ui/incantesimo_scontro.js").then((x) => x.apriIncantesimo(r.scontro, m.effetto));
        }).catch((e) => avviso(e.message, "errore"));
        break;
      case "scheda:trasferisci": // «💸 Trasferisci» nella scheda
        import("../ui/finanze.js").then((x) => x.apriTrasferimento(pg.id, pg.nome));
        break;
      case "scheda:acquista": { // oggetto scelto dall'elenco: il server scala le monete (gratis se lo assegna il Master)
        const q = Math.max(1, Math.min(999, parseInt(m.quantita, 10) || 1));
        api("/finanze/" + encodeURIComponent(pg.id) + "/acquista", { dati: { nome: String(m.nome || "").slice(0, 100), prezzo: String(m.prezzo || ""), quantita: q } })
          .then((r) => avviso(r.testo, "", 5000)).catch((e) => avviso(e.message, "errore", 6000));
        break;
      }
      case "scheda:professione": // pulsante «⚒ Professione» della scheda: il popup di Gaster Bless
        if (typeof m.id === "string") import("../ui/professioni.js").then((x) => x.apriProfessione(pg.id, m.id));
        break;
      case "scheda:sanita": // una scheda (anche un plugin) chiede di aprire la Sanità mentale: la mini scheda dell'app
        apriMiniSanita(pg.id);
        break;
      case "scheda:state":
        // Scheda libera all'apertura (C4b): i valori iniziali del modello non sono modifiche del giocatore (come i
        // predefiniti della scheda 5e letti con blank()): si considerano già salvati.
        if (m.iniziale && libera) for (const [k, v] of Object.entries(appiattisci(m.state || {}))) {
          if (!(k in salvato) && !Object.keys(salvato).some((x) => x.startsWith(k + ".") || k.startsWith(x + "."))) salvato[k] = v;
        }
        ultimoStato = m;
        if (!modificabile || inConflitto) return;
        clearTimeout(attesa);
        aggiornaStato("Modifiche da salvare…");
        attesa = setTimeout(salva, 1200);
        break;
      case "scheda:theme":
        applicaTema(m.theme === "light" ? "chiaro" : "scuro");
        break;
      case "scheda:request": {
        let r;
        try { r = { ok: true, data: await richiestaBackground(m.action, m.payload || {}) }; }
        catch (er) { r = { ok: false, error: er.message }; avviso(er.message, "errore"); }
        invia({ type: "scheda:response", id: m.id, ...r });
        break;
      }
    }
  }
  window.addEventListener("message", messaggio);

  // ----- Salvataggio: solo i campi cambiati, con la versione di partenza -----
  async function salva(forza = [], escludi = []) {
    if (!ultimoStato) return;
    if (invio) { attesa = setTimeout(salva, 400); return; } // un salvataggio alla volta
    const stato = ultimoStato.state;
    const nuovo = appiattisci(stato);
    let modifiche = {}, rimossi = [];
    for (const [k, v] of Object.entries(nuovo)) if (!escludi.includes(k) && salvato[k] !== v) modifiche[k] = JSON.parse(v);
    for (const k of Object.keys(salvato)) if (!(k in nuovo) && !escludi.includes(k)) rimossi.push(k);
    if (approvazione) { // solo i campi di gioco partono; gli altri sono la bozza (tasto acceso) o tornano indietro
      const { gioco, manuali } = separa(nuovo, salvato);
      for (const k of automatici) { delete manuali.modifiche[k]; manuali.rimossi = manuali.rimossi.filter((x) => x !== k); }
      modifiche = Object.fromEntries(Object.entries(gioco.modifiche).filter(([k]) => !escludi.includes(k)));
      rimossi = gioco.rimossi.filter((k) => !escludi.includes(k));
      if (quante(manuali)) {
        if (manuale) scriviBozza(manuali);
        else riportaIndietro([...Object.keys(manuali.modifiche), ...manuali.rimossi]);
      } else if (manuale) scriviBozza({ modifiche: {}, rimossi: [] });
      if (!Object.keys(modifiche).length && !rimossi.length && quante(manuali) && manuale) {
        aggiornaStato("✏ Modifica manuale: " + quante(manuali) + (quante(manuali) === 1 ? " campo" : " campi") + " da mandare al Master");
        return;
      }
    }
    if (!Object.keys(modifiche).length && !rimossi.length) {
      aggiornaStato("Salvata sul PC del Master ✓ · versione " + base);
      // Nessuna modifica: alla prima apertura si manda comunque il riepilogo (caratteristiche per la Sanità
      // mentale, CA e PF per il tracker), così anche un personaggio nuovo o salvato tempo fa è aggiornato.
      if (!riepilogoInviato && ultimoStato.summary) {
        riepilogoInviato = true;
        fetchSalva({ base, modifiche: {}, rimossi: [], forza: [], riepilogo: senzaRitratto(ultimoStato.summary) }).catch(() => { riepilogoInviato = false; });
      }
      return;
    }
    if (senzaRete) { // aperta dalla copia del dispositivo: si salva solo qui, il server si prova al riavvio dell'app
      tieniSulDispositivo(stato);
      return;
    }
    aggiornaStato("Salvataggio…");
    riepilogoInviato = true;
    const offline = ripresaDaCopia || (offlineDal > 0 && Date.now() - offlineDal > 60000);
    invio = fetchSalva({ base, modifiche, rimossi, forza, riepilogo: senzaRitratto(ultimoStato.summary), offline });
    try {
      const r = await invio;
      if (r.conflitti) { invio = null; return risolviConflitti(r, nuovo); }
      if (r.ignorati && r.ignorati.length) riportaIndietro(r.ignorati); // il server applica le stesse regole
      base = r.versione;
      salvato = { ...salvato };
      for (const k of Object.keys(modifiche)) salvato[k] = nuovo[k];
      for (const k of rimossi) delete salvato[k];
      aggiornaStato("Salvata sul PC del Master ✓ · versione " + base);
      if (offline || offlineDal) {
        const nuovoOra = appiattisci(ultimoStato.state);
        if (!Object.keys(nuovoOra).some((k) => nuovoOra[k] !== salvato[k])) { offlineDal = 0; ripresaDaCopia = false; }
        if (offline) avviso("📶 Modifiche fatte senza connessione inviate al PC del Master.", "", 5000);
      }
      aggiornaCopia(offlineDal ? { pendente: true, stato: ultimoStato.state, offlineDal } : {});
      if (r.monetePendenti) { // le monete scritte a mano aspettano il Master: la scheda torna ai valori attuali
        avviso("💰 Modifica delle monete inviata al Master: vale quando la autorizza.", "", 6000);
        try {
          const dati = (await api("/personaggi/" + encodeURIComponent(pg.id))).dati;
          salvato = appiattisci(dati);
          if (ultimoStato) ultimoStato = { ...ultimoStato, state: dati }; // niente modifiche in sospeso: gli aggiornamenti dal server arrivano
          invia({ type: "scheda:load", state: dati });
        } catch { /* al prossimo aggiornamento */ }
      }
      if (modifiche["f.nome"] !== undefined && stato.f && stato.f.nome) titolo.textContent = stato.f.nome;
    } catch (e) {
      if (e.stato === 0 && mia(pg) && !pg.scheda) { // senza rete: le modifiche restano sul dispositivo e si riprova
        tieniSulDispositivo(stato);
        attesa = setTimeout(salva, 5000);
        return;
      }
      aggiornaStato("⚠ Non salvata: " + e.message);
      if (e.stato === 403) { modificabile = false; avviso(e.message, "errore", 6000); }
      else attesa = setTimeout(salva, 5000); // riprova (per esempio dopo un'interruzione del Wi-Fi)
    } finally { invio = null; }
  }

  // Campi non di gioco cambiati con il tasto spento: la scheda torna ai valori validi (i dati di gioco restano).
  let ultimoAvvisoBlocco = 0, ultimoRitorno = { chiave: "", quando: 0 };
  const automatici = new Set(); // campi che la scheda sistema da sé a ogni apertura: non si salvano e non si riportano indietro
  function riportaIndietro(campi) {
    if (!ultimoStato) return;
    const chiave = [...campi].sort().join("|");
    if (chiave === ultimoRitorno.chiave && Date.now() - ultimoRitorno.quando < 10000) { campi.forEach((k) => automatici.add(k)); return; }
    ultimoRitorno = { chiave, quando: Date.now() };
    const p = appiattisci(ultimoStato.state);
    for (const k of campi) { if (k in salvato) p[k] = salvato[k]; else delete p[k]; }
    const dati = ricomponi(p);
    ultimoStato = { ...ultimoStato, state: dati };
    invia({ type: "scheda:load", state: dati });
    if (Date.now() - ultimoAvvisoBlocco > 4000) {
      ultimoAvvisoBlocco = Date.now();
      avviso("🔒 Per cambiare " + (campi.length === 1 ? "«" + nomeCampo(campi[0]) + "»" : "questi dati") + " accendi «✏ Modifica manuale»: le modifiche andranno al Master.", "", 6000);
    }
  }

  function statoAttuale() { // lo stato più recente della scheda (la scheda di Gaster Bless si legge subito, i plugin dall'ultimo messaggio)
    if (!plugin) { try { return frame.contentWindow.SchedaAPI.getState(); } catch { /* sotto */ } }
    return ultimoStato ? ultimoStato.state : ricomponi(salvato);
  }

  function disegnaManuale() {
    if (btnManuale) {
      btnManuale.textContent = manuale ? "✏ Modifica manuale: ACCESA" : "✏ Modifica manuale";
      btnManuale.classList.toggle("primario", manuale);
      btnManuale.setAttribute("aria-pressed", String(manuale));
      btnManuale.title = manuale ? "Spegni per mandare le modifiche al Master" : "Accendi per cambiare qualsiasi dato della scheda (vale dopo l'approvazione del Master)";
    }
    const parti = [];
    if (manuale) {
      parti.push(el("div", { class: "mm-riga mm-accesa" },
        el("span", { testo: "✏ Modifica manuale accesa: puoi cambiare qualsiasi dato. Quando hai finito spegni il tasto: le modifiche vanno al Master e valgono dopo la sua approvazione." }),
        el("button", { class: "piccolo", testo: "Annulla le modifiche", on: { click: annullaManuale } }),
        el("button", { class: "piccolo primario", testo: "Fine: manda al Master", on: { click: spegniManuale } })));
    }
    if (richiesta && richiesta.stato === "in_attesa") {
      const n = (richiesta.voci || []).length;
      const det = el("details", {}, el("summary", { testo: "⏳ " + n + (n === 1 ? " modifica in attesa" : " modifiche in attesa") + " dell'approvazione del Master" + (io_.ruolo === "master" ? " (chieste da " + (richiesta.giocatore || "giocatore") + ")" : "") }),
        tabellaVoci(richiesta.voci));
      parti.push(el("div", { class: "mm-riga mm-attesa" }, det,
        io_.ruolo === "master" ? el("button", { class: "piccolo primario", testo: "Apri il report", on: { click: () => mostraRichiesta(richiesta) } })
          : el("button", { class: "piccolo", testo: "Ritira la richiesta", on: { click: ritira } })));
    }
    fascia.replaceChildren(...parti);
    fascia.hidden = !parti.length;
  }

  function accendiManuale() {
    manuale = true;
    scriviBozza({ modifiche: {}, rimossi: [] });
    invia({ type: "scheda:blocco", bloccata: false });
    disegnaManuale();
    avviso("✏ Modifica manuale accesa: cambia quello che vuoi, poi spegni il tasto per mandare tutto al Master.", "", 6000);
  }

  async function ricaricaValida() {
    const fresco = await api("/personaggi/" + encodeURIComponent(pg.id));
    base = fresco.personaggio.versione; salvato = appiattisci(fresco.dati);
    if (ultimoStato) ultimoStato = { ...ultimoStato, state: fresco.dati };
    invia({ type: "scheda:load", state: fresco.dati });
    aggiornaStato("Salvata sul PC del Master ✓ · versione " + base);
  }

  function chiudiManuale() {
    manuale = false;
    scriviBozza(null);
    invia({ type: "scheda:blocco", bloccata: true });
    disegnaManuale();
  }

  async function spegniManuale() {
    const { manuali } = separa(appiattisci(statoAttuale()), salvato);
    for (const k of automatici) { delete manuali.modifiche[k]; manuali.rimossi = manuali.rimossi.filter((x) => x !== k); }
    if (!quante(manuali)) { chiudiManuale(); avviso("Modifica manuale spenta: nessun dato cambiato."); return; }
    const voci = [...Object.entries(manuali.modifiche).map(([campo, dopo]) => ({ campo, prima: salvato[campo] === undefined ? null : JSON.parse(salvato[campo]), dopo })),
      ...manuali.rimossi.map((campo) => ({ campo, prima: JSON.parse(salvato[campo]), dopo: null, rimosso: true }))].sort((a, b) => a.campo.localeCompare(b.campo));
    dialogo("Mandare le modifiche al Master?", el("div", {},
      el("p", { testo: "Queste " + voci.length + (voci.length === 1 ? " modifica" : " modifiche") + " arriveranno al Master con un report. Finché non le approva vale la scheda di prima." }),
      tabellaVoci(voci)), {
      largo: true,
      azioni: (chiudi) => el("div", { class: "azioni" },
        el("button", { testo: "Continua a modificare", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Manda al Master", on: { click: async (e) => {
          e.target.disabled = true;
          try {
            if (invio) await invio.catch(() => {});
            const r = await api("/personaggi/" + encodeURIComponent(pg.id) + "/richiesta-modifiche", { dati: manuali });
            chiudi();
            richiesta = r.richiesta;
            chiudiManuale();
            await ricaricaValida().catch(() => {});
            avviso("📝 " + r.testo, "", 7000);
          } catch (er) { e.target.disabled = false; avviso((er.stato === 0 ? "Senza connessione: " : "") + er.message + " La modifica manuale resta accesa.", "errore", 7000); }
        } } })),
    });
  }

  async function annullaManuale() {
    if (!(await conferma("Annullare le modifiche?", "La scheda torna com'è ora sul PC del Master; quello che hai cambiato con la modifica manuale si perde.", "Annulla le modifiche", true))) return;
    chiudiManuale();
    await ricaricaValida().catch((e) => avviso(e.message, "errore"));
  }

  async function ritira() {
    if (!(await conferma("Ritirare la richiesta?", "Le modifiche in attesa non arriveranno nella scheda. Puoi rifarle con «✏ Modifica manuale».", "Ritira"))) return;
    try { const r = await api("/personaggi/" + encodeURIComponent(pg.id) + "/richiesta-modifiche", { metodo: "DELETE" }); richiesta = null; disegnaManuale(); avviso(r.testo); }
    catch (e) { avviso(e.message, "errore"); }
  }

  async function aggiornaRichiesta() {
    try { const x = await api("/personaggi/" + encodeURIComponent(pg.id) + "/richiesta-modifiche"); richiesta = x.richiesta; disegnaManuale(); } catch { /* al prossimo giro */ }
  }

  function tieniSulDispositivo(stato) {
    if (!offlineDal) offlineDal = Date.now();
    aggiornaCopia({ pendente: true, stato, offlineDal });
    aggiornaStato("📴 Senza connessione: modifiche salvate su questo dispositivo, partiranno appena Gaster Bless risponde");
  }

  async function fetchSalva(dati) {
    try { return await api("/personaggi/" + encodeURIComponent(pg.id) + "/scheda", { metodo: "PATCH", dati }); }
    catch (e) {
      if (e.stato === 409 && e.dati && e.dati.conflitti) return e.dati;
      throw e;
    }
  }

  // ----- Conflitti: stesso campo cambiato da un'altra persona -----
  function risolviConflitti(r, nuovo) {
    inConflitto = true;
    aggiornaStato("⚠ Modifiche in conflitto");
    const scelte = r.conflitti.map((c) => ({ c, mio: el("input", { type: "radio", name: "c-" + c.campo, selezionato: true }), suo: el("input", { type: "radio", name: "c-" + c.campo }) }));
    const chi = r.conflitti[0].autore || "Un'altra persona";
    dialogo("Modifiche in conflitto", el("div", {},
      el("p", { testo: chi + " ha cambiato gli stessi campi mentre li modificavi tu. Per ognuno scegli quale valore tenere: l'altro resta comunque nello storico." }),
      el("table", { class: "tabella" }, el("tbody", {}, scelte.map(({ c, mio, suo }) => el("tr", {},
        el("td", { testo: nomeCampo(c.campo) }),
        el("td", {}, el("label", { class: "riga" }, mio, el("span", { testo: "Il tuo: " + mostraValore(c.tuo) }))),
        el("td", {}, el("label", { class: "riga" }, suo, el("span", { testo: "Il suo: " + mostraValore(c.attuale) })))))))),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { class: "primario", testo: "Conferma le scelte", on: { click: async () => {
      chiudi();
      const forza = scelte.filter((s) => s.mio.checked).map((s) => s.c.campo);
      const loro = scelte.filter((s) => s.suo.checked).map((s) => s.c.campo);
      base = r.versione;
      // Ricarica dal server ciò che è cambiato, poi reinvia le proprie modifiche (tranne i campi lasciati all'altro).
      try {
        const fresco = await api("/personaggi/" + encodeURIComponent(pg.id));
        const server = appiattisci(fresco.dati);
        salvato = server; base = fresco.personaggio.versione;
        const unito = { ...server };
        for (const [k, v] of Object.entries(nuovo)) if (!loro.includes(k)) unito[k] = v;
        for (const k of Object.keys(server)) if (!(k in nuovo) && !loro.includes(k)) delete unito[k];
        ultimoStato = { state: ricomponi(unito), summary: ultimoStato.summary };
        inConflitto = false;
        await salva(forza);
        invia({ type: "scheda:load", state: ultimoStato.state });
      } catch (e) { inConflitto = false; avviso(e.message, "errore"); }
    } } })) });
  }

  // ----- Aggiornamenti dagli altri (Master, altro dispositivo, tracker) -----
  const smetti = [
    tempoReale.su("scheda.eliminata", (d) => {
      if (!d || d.personaggio !== pg.id || eliminataQui) return;
      modificabile = false; clearTimeout(attesa); // niente più salvataggi di una scheda che non esiste più
      avviso("Il Master ha eliminato la scheda di " + pg.nome + ".", "", 6000); vai("/schede");
    }),
    tempoReale.su("scheda.aggiornata", async (d) => {
      if (d.personaggio !== pg.id) return;
      if (manuale) { // la bozza resta: si tengono i campi cambiati dal giocatore, il resto viene dalla scheda nuova
        if (d.autoreId === io_.id && !d.server) return;
        if (d.versione && d.versione <= base) return;
        try {
          const fresco = await api("/personaggi/" + encodeURIComponent(pg.id));
          const vecchio = salvato, mio = appiattisci(statoAttuale()), nuovo = appiattisci(fresco.dati), unito = { ...nuovo };
          for (const k of new Set([...Object.keys(mio), ...Object.keys(vecchio)])) {
            if (mio[k] === vecchio[k]) continue;
            if (k in mio) unito[k] = mio[k]; else delete unito[k];
          }
          base = fresco.personaggio.versione; salvato = nuovo;
          const dati = ricomponi(unito);
          if (ultimoStato) ultimoStato = { ...ultimoStato, state: dati };
          invia({ type: "scheda:load", state: dati });
          avviso((d.autore || "Il Master") + " ha aggiornato la scheda: la tua modifica manuale resta in corso.", "", 6000);
        } catch { /* al prossimo aggiornamento */ }
        return;
      }
      if (d.server) { // monete, acquisti, professioni: modifiche del server, si applicano anche se l'autore siamo noi
        if (invio) await invio.catch(() => {});
        if (d.versione && d.versione <= base) return;
        const pref = (k) => (d.campi || []).some((c) => k === c || k.startsWith(c + "."));
        if (ultimoStato) { // altre modifiche in sospeso: le unirà il prossimo salvataggio
          const ora = appiattisci(ultimoStato.state);
          // (i campi mai salvati, cioè i valori predefiniti della scheda, non contano)
          if (Object.keys(ora).some((k) => !pref(k) && k in salvato && ora[k] !== salvato[k])) return;
        }
      } else {
        if (d.autoreId === io_.id && (invio || d.versione <= base)) return; // è il nostro salvataggio
        if (d.versione && d.versione <= base) return;
        if (ultimoStato) { // modifiche in sospeso: le unirà il prossimo salvataggio
          const ora = appiattisci(ultimoStato.state);
          if (Object.keys(ora).some((k) => ora[k] !== salvato[k])) return;
        }
      }
      try {
        const fresco = await api("/personaggi/" + encodeURIComponent(pg.id));
        base = fresco.personaggio.versione; salvato = appiattisci(fresco.dati); modificabile = fresco.modificabile;
        if (ultimoStato) ultimoStato = { ...ultimoStato, state: fresco.dati };
        invia({ type: "scheda:load", state: fresco.dati });
        aggiornaStato((modificabile ? "Aggiornata da " + d.autore + " ✓ · versione " : "Solo consultazione · versione ") + base);
        if (d.autoreId !== io_.id && !d.server) avviso((d.autore || "Qualcuno") + " ha aggiornato la scheda.");
      } catch { /* ignora: al prossimo salvataggio si unisce */ }
    }),
    tempoReale.su("scheda.richiesta.esito", (d) => { if (d && d.personaggio === pg.id) aggiornaRichiesta(); }),
    tempoReale.su("scheda.richiesta", (d) => { if (d && d.personaggio === pg.id && io_.ruolo === "master") { richiesta = d; disegnaManuale(); } }),
    tempoReale.su("scheda.richiesta.decisa", (d) => { if (d && d.personaggio === pg.id && io_.ruolo === "master") { richiesta = d.stato === "in_attesa" ? d : null; disegnaManuale(); } }),
    tempoReale.su("scheda.richiesta.ritirata", (d) => { if (d && d.personaggio === pg.id && io_.ruolo === "master") { richiesta = null; disegnaManuale(); } }),
    tempoReale.su("pg.stato", (d) => {
      if (d.personaggio !== pg.id) return;
      // effetti in corso nello scontro (internal/server/effetti_valori.go): la scheda mostra la CA cambiata con l'originale
      const eff = JSON.stringify(d.effetti || null);
      if (eff !== ultimiEffetti) { ultimiEffetti = eff; invia({ type: "scheda:effetti", effetti: d.effetti || null }); }
      // Indebolimento: livello dalle condizioni (tracker, Master) → riquadro della scheda
      const exh = Math.max(0, ...(d.condizioni || []).map((c) => /^Indebolimento (\d)$/.test(c) ? +RegExp.$1 : 0));
      if (salvato.exh !== JSON.stringify(exh)) {
        salvato.exh = JSON.stringify(exh);
        if (ultimoStato && ultimoStato.state) ultimoStato = { ...ultimoStato, state: { ...ultimoStato.state, exh } }; // un salvataggio in coda non lo riporta indietro
        invia({ type: "scheda:setExh", livello: exh });
      }
      const cur = d.pf === null || d.pf === undefined ? "" : String(d.pf);
      const temp = d.pfTemp ? String(d.pfTemp) : "";
      if (salvato.hpCur === JSON.stringify(cur) && salvato.hpTmp === JSON.stringify(temp)) return;
      salvato.hpCur = JSON.stringify(cur); salvato.hpTmp = JSON.stringify(temp);
      invia({ type: "scheda:setHp", cur, temp });
    }),
  ];

  // ----- Background privato (richieste della scheda) -----
  async function richiestaBackground(azione, p) {
    // Una scheda alternativa può chiedere solo il background del personaggio aperto.
    const pid = encodeURIComponent(plugin ? pg.id : (p.playerId || pg.id));
    switch (azione) {
      case "bg.get": {
        const r = await api("/personaggi/" + pid + "/background/info");
        if (!r.background) return { meta: null };
        const meta = metaScheda(r.background);
        if (p.metaOnly) return { meta };
        const f = await fetch("/api/v1/personaggi/" + pid + "/background", { credentials: "same-origin" });
        if (!f.ok) throw new Error("Background non leggibile.");
        return { meta, data: base64(await f.arrayBuffer()) };
      }
      case "bg.put": {
        const bin = Uint8Array.from(atob(p.data), (c) => c.charCodeAt(0));
        const r = await fetch("/api/v1/personaggi/" + pid + "/background", { method: "PUT", credentials: "same-origin",
          headers: { "X-CSRF": io.csrf, "X-Nome-File": encodeURIComponent((p.meta && p.meta.name) || "background"), "Content-Type": "application/octet-stream" }, body: bin });
        if (!r.ok) { let m = "Caricamento non riuscito."; try { m = (await r.json()).errore || m; } catch { /* vuota */ } throw new Error(m); }
        return true;
      }
      case "bg.delete":
        await api("/personaggi/" + pid + "/background", { metodo: "DELETE" });
        return true;
      case "bg.list":
        if (io_.ruolo !== "master" || plugin) throw new Error("Non consentito.");
        return (await api("/background")).map((b) => ({ id: b.personaggio, ...metaScheda(b) }));
    }
    throw new Error("Richiesta sconosciuta.");
  }

  // ----- Tipo di scheda (solo Master) -----
  // Il Master elimina la scheda (irreversibile: resta solo nei backup). Server: personaggi_assegna.go.
  function elimina() {
    const conferma = el("input", { maxlength: "60", placeholder: pg.nome, autocomplete: "off" });
    const err = el("p", { class: "errore" });
    dialogo("Eliminare " + pg.nome + "?", el("div", {},
      el("p", { testo: "La scheda, le sue versioni, lo stato in gioco, la Sanità mentale e il background privato vengono cancellati; il personaggio esce anche dagli scontri. Non si può annullare: resta solo nei backup." }),
      el("label", {}, "Per confermare scrivi il nome del personaggio: «" + pg.nome + "»", conferma), err),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "pericolo", testo: "Elimina definitivamente", on: { click: async () => {
        if (conferma.value.trim() !== pg.nome) { err.textContent = "Il nome non corrisponde."; return; }
        try {
          await api("/personaggi/" + encodeURIComponent(pg.id), { metodo: "DELETE", dati: { conferma: conferma.value.trim() } });
          chiudi(); eliminataQui = true; modificabile = false; clearTimeout(attesa); avviso(pg.nome + " è stato eliminato."); vai("/schede");
        } catch (e) { err.textContent = e.message; }
      } } })) });
  }
  let eliminataQui = false;

  // Il Master assegna la scheda a un giocatore (o la riprende per sé). Server: personaggi_assegna.go.
  async function assegna() {
    let utenti = [], gruppi = [];
    try { [utenti, gruppi] = await Promise.all([api("/utenti"), api("/gruppi")]); } catch (e) { avviso(e.message, "errore"); return; }
    const gioc = utenti.filter((g) => g.stato === "approvato");
    const nomeGruppo = (id) => (gruppi.find((g) => g.id === id) || {}).nome || id;
    const chi = el("select", { "aria-label": "Giocatore" }, el("option", { value: "", testo: "Me stesso (PNG del Master)" }),
      gioc.map((g) => el("option", { value: g.id, testo: g.nome, selected: g.id === pg.proprietario })));
    const gruppo = el("select", { "aria-label": "Gruppo" });
    const aggiornaGruppi = () => {
      const g = gioc.find((x) => x.id === chi.value);
      const ammessi = g ? (g.gruppi || []) : gruppi.map((x) => x.id);
      svuota(gruppo, g ? null : el("option", { value: "", testo: "— nessun gruppo —" }),
        ammessi.map((id) => el("option", { value: id, testo: nomeGruppo(id), selected: id === pg.gruppo })));
    };
    chi.addEventListener("change", aggiornaGruppi); aggiornaGruppi();
    const err = el("p", { class: "errore" });
    dialogo("Assegna " + pg.nome, el("div", {}, el("label", {}, "Giocatore", chi), el("label", {}, "Gruppo", gruppo),
      el("p", { class: "tenue piccolo", testo: "Il giocatore scelto vedrà e modificherà la scheda; tu continui a vederla e modificarla. Chi la aveva prima non la vede più." }), err),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Assegna", on: { click: async () => {
        try {
          await api("/personaggi/" + encodeURIComponent(pg.id) + "/assegna", { dati: { proprietario: chi.value, gruppo: gruppo.value } });
          chiudi(); pg.proprietario = chi.value; pg.gruppo = gruppo.value;
          avviso(chi.value ? "Scheda assegnata a " + chi.selectedOptions[0].textContent + "." : "Scheda ripresa dal Master.");
        } catch (e) { err.textContent = e.message; }
      } } })) });
  }

  function cambiaTipo() {
    const sel = el("select", { "aria-label": "Tipo di scheda" }, el("option", { value: "", testo: "Scheda di Gaster Bless", selected: !pg.scheda }),
      pluginAbilitati.map((x) => el("option", { value: x.id, testo: "🧩 " + x.nome, selected: x.id === pg.scheda })));
    dialogo("Tipo di scheda", el("div", {}, el("label", {}, "Questo personaggio si apre con", sel),
      el("p", { class: "tenue piccolo", testo: "I dati salvati restano quelli di prima: una scheda diversa potrebbe non saperli leggere e ripartire vuota. Le versioni precedenti restano nello storico." })),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Cambia", on: { click: async () => {
        try { await api("/personaggi/" + encodeURIComponent(pg.id) + "/tipo-scheda", { metodo: "PUT", dati: { scheda: sel.value } }); chiudi(); location.reload(); }
        catch (e) { avviso(e.message, "errore"); }
      } } })) });
  }

  // ----- Storico e stato del personaggio -----
  async function storico() {
    let v;
    try { v = await api("/personaggi/" + encodeURIComponent(pg.id) + "/versioni"); } catch (e) { avviso(e.message, "errore"); return; }
    dialogo("Storico di " + titolo.textContent, el("div", {},
      el("p", { class: "tenue piccolo", testo: "Le modifiche fatte di seguito dalla stessa persona sono raggruppate. Ripristinare crea una nuova versione: non si perde nulla." }),
      el("table", { class: "tabella adattiva" }, el("tbody", {}, v.map((x) => el("tr", {},
        el("td", { "data-etichetta": "Versione", testo: "v" + x.numero + (x.numero === base ? " (attuale)" : "") }),
        el("td", { "data-etichetta": "Quando", testo: quando(x.quando) }),
        el("td", { "data-etichetta": "Chi", testo: x.mia ? "tu" : x.autore || "—" }),
        el("td", { "data-etichetta": "Cosa", testo: x.nota || riassumiCampi(x.campi) }),
        el("td", {}, modificabile && x.numero !== base ? el("button", { class: "piccolo", testo: "Ripristina", on: { click: async () => {
          if (!(await conferma("Ripristinare la versione " + x.numero + "?", "La scheda tornerà com'era in quella versione. La versione attuale resta nello storico.", "Ripristina"))) return;
          try {
            const rr = await api("/personaggi/" + encodeURIComponent(pg.id) + "/ripristina/" + x.numero, { dati: {} });
            document.querySelectorAll(".velo").forEach((e) => e.remove());
            if (rr && "richiesta" in rr) { richiesta = rr.richiesta; disegnaManuale(); avviso("📝 " + rr.testo, "", 7000); return; } // approvazione attiva
            const fresco = await api("/personaggi/" + encodeURIComponent(pg.id));
            base = fresco.personaggio.versione; salvato = appiattisci(fresco.dati);
            invia({ type: "scheda:load", state: fresco.dati });
            aggiornaStato("Ripristinata ✓ · versione " + base); avviso("Versione ripristinata.");
          } catch (e) { avviso(e.message, "errore"); }
        } } }) : null)))))), { largo: true });
  }

  function cambiaStato() {
    // Il giocatore può ritirare o segnare morto il personaggio; riattivarlo o archiviarlo spetta al Master.
    const master = io_.ruolo === "master";
    const scelte = master ? ["attivo", "ritirato", "morto", "archiviato"] : pg.stato === "attivo" ? ["attivo", "ritirato", "morto"] : [pg.stato];
    const sel = el("select", { disabled: !master && pg.stato !== "attivo" }, scelte
      .map((s) => el("option", { value: s, testo: STATI[s] || s, selected: s === pg.stato })));
    dialogo("Stato del personaggio", el("div", {}, el("label", {}, "Stato", sel),
      el("p", { class: "tenue piccolo", testo: master
        ? "Un personaggio ritirato o morto resta consultabile ma non si modifica più. Riattivarlo o archiviarlo spetta a te."
        : "Un personaggio ritirato o morto resta consultabile ma non si modifica più. Per riattivarlo chiedi al Master." })),
    { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
      el("button", { class: "primario", testo: "Salva", on: { click: async () => {
        try {
          await api("/personaggi/" + encodeURIComponent(pg.id) + "/stato", { dati: { stato: sel.value } });
          chiudi(); pg.stato = sel.value; modificabile = sel.value === "attivo";
          aggiornaStato(modificabile ? "Salvata sul PC del Master ✓ · versione " + base : "Solo consultazione (" + STATI[sel.value] + ")");
          avviso("Stato aggiornato.");
        } catch (e) { avviso(e.message, "errore"); }
      } } })) });
  }

  // Tiro dalla scheda (richiesta dell'utente, 2026-11-06): prima si sceglie «Tiro automatico» (normale, vantaggio o
  // svantaggio: tira il server con il modificatore della scheda) o «Tiro manuale» (il giocatore tira il suo d20 vero e
  // scrive il numero; l'app aggiunge il modificatore). In entrambi i casi lo vedono giocatore e Master (dadi.go).
  function sceltaTiro(m) {
    const mod = Math.max(-50, Math.min(50, parseInt(m.mod, 10) || 0));
    const modoScheda = m.modo === "vantaggio" || m.modo === "svantaggio" ? m.modo : "normale"; // per esempio le prove con gli strumenti
    const etichetta = String(m.etichetta || "").slice(0, 40);
    const segno = (n) => (n < 0 ? "−" + -n : "+" + n);
    const nome = etichetta.split(" · ").pop();
    const fatto = (r) => invia({ type: "scheda:tiroRisultato", r });
    const MODI = { normale: "Normale (1 dado)", vantaggio: "Con vantaggio (2 dadi, tieni il maggiore)", svantaggio: "Con svantaggio (2 dadi, tieni il minore)" };
    dialogo("🎲 " + nome + " · d20 " + segno(mod), (chiudi) => {
      const zona = el("div", { class: "tiro-scelta" });
      const indietro = () => el("button", { class: "piccolo", testo: "← Indietro", on: { click: passo1 } });
      function passo1() {
        svuota(zona, el("p", { testo: "Come vuoi tirare?" }),
          el("div", { class: "tiro-scelte" },
            el("button", { class: "primario", testo: "🎲 Tiro automatico", on: { click: passo2 } }),
            el("button", { testo: "✋ Tiro manuale (dado reale)", on: { click: passo3 } })));
        zona.querySelector("button").focus();
      }
      function passo2() {
        svuota(zona, el("p", { testo: "Tiro automatico: d20 " + segno(mod) + ". Come tiri?" }),
          el("div", { class: "tiro-scelte" }, Object.entries(MODI).map(([k, t]) => el("button", { class: k === modoScheda ? "primario" : "", testo: t, on: { click: () => {
            chiudi();
            tempoReale.chiedi("dadi.tira", { dadi: [{ n: 1, facce: 20 }], mod, modo: k, etichetta, scheda: true, personaggio: pg.id }).then(fatto).catch((e) => avviso(e.message, "errore"));
          } } }))), indietro());
        zona.querySelector("button.primario").focus();
      }
      function passo3() {
        const n = el("input", { type: "number", min: "1", max: "20", inputmode: "numeric", placeholder: "1–20" });
        const modo = el("select", {}, Object.entries(MODI).map(([k, t]) => el("option", { value: k, testo: t, selected: k === modoScheda })));
        const registra = () => {
          const v = parseInt(n.value, 10);
          if (!(v >= 1 && v <= 20)) { avviso("Scrivi il numero uscito sul d20 (da 1 a 20).", "errore"); n.focus(); return; }
          chiudi();
          tempoReale.chiedi("dadi.manuale", { d20: v, mod, modo: modo.value, etichetta, personaggio: pg.id }).then(fatto).catch((e) => avviso(e.message, "errore"));
        };
        n.addEventListener("keydown", (e) => { if (e.key === "Enter") registra(); });
        svuota(zona, el("p", { testo: "Tira il tuo d20 e scrivi il numero uscito. Con vantaggio o svantaggio tira due d20 e scrivi quello che tieni." }),
          el("label", {}, "Numero uscito sul d20", n), el("label", {}, "Come hai tirato", modo),
          el("p", { class: "tenue", testo: "Il modificatore della scheda (" + segno(mod) + ") lo aggiunge l'app." }),
          el("div", { class: "riga-flex" }, indietro(), el("span", { class: "spazio" }), el("button", { class: "primario", testo: "Registra il tiro", on: { click: registra } })));
        setTimeout(() => n.focus(), 30);
      }
      passo1();
      return zona;
    }, { azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } })) });
  }

  return () => {
    clearTimeout(attesa);
    if (ultimoStato && modificabile && !inConflitto) salva(); // salva le ultime modifiche uscendo
    window.removeEventListener("message", messaggio);
    smetti.forEach((f) => f());
  };
}

const id_ = () => decodeURIComponent(location.pathname.split("/")[2] || "");

// ===================== Utilità =====================

/** Serializzazione con chiavi in ordine: due oggetti uguali danno la stessa stringa. */
// Le caratteristiche (abilities) restano: servono al server per la Sanità mentale (10 + mod Saggezza) e i TS del Terrore.
// La scheda non contiene più nulla sulla Sanità mentale: è la funzione separata "Sanità mentale" dell'app.
function senzaRitratto(s) { if (!s) return {}; const c = { ...s }; delete c.portrait; delete c.mods; delete c.deathSaves; return c; }

function metaScheda(b) {
  return { name: b.nome, type: b.tipo === "pdf" ? "application/pdf" : "docx", size: b.dimensione, at: b.quando * 1000,
    char: b.nomePersonaggio, player: b.nomeGiocatore, cls: b.classe };
}

function base64(buf) {
  let s = ""; const u = new Uint8Array(buf);
  for (let i = 0; i < u.length; i += 32768) s += String.fromCharCode.apply(null, u.subarray(i, i + 32768));
  return btoa(s);
}

function riassumiCampi(c) {
  if (!c || !c.length) return "—";
  if (c[0] === "*") return "ripristino";
  const n = [...new Set(c.map((k) => nomeCampo(k.split(".")[0] === "f" ? k : k.split(".")[0])))];
  return n.slice(0, 4).join(", ") + (n.length > 4 ? " e altro" : "");
}

/** Formula della scheda libera («2d6 + 1d4 - 2») → gruppi di dadi e modificatore per il server (dadi.tira). */
export function dadiDaFormula(f) {
  const s = f.toLowerCase().replace(/\s+/g, "").replace(/−/g, "-");
  if (!s) return { errore: "Formula vuota." };
  const per = new Map();
  let mod = 0, dadi = 0;
  for (const t of (/^[+-]/.test(s) ? s : "+" + s).match(/[+-][^+-]+/g) || []) {
    const segno = t[0] === "-" ? -1 : 1, x = t.slice(1);
    const d = /^(\d{0,2})d(\d{1,3})$/.exec(x);
    if (d) {
      const n = d[1] ? +d[1] : 1, facce = +d[2];
      if (![4, 6, 8, 10, 12, 20, 100].includes(facce) || n < 1 || n > 20) return { errore: "«" + x + "»: dadi ammessi da 1 a 20, d4, d6, d8, d10, d12, d20 o d100." };
      if (segno < 0) return { errore: "«-" + x + "»: i dadi da sottrarre non sono ancora previsti." };
      per.set(facce, (per.get(facce) || 0) + n); dadi += n;
    } else if (/^\d{1,3}$/.test(x)) mod += segno * +x;
    else return { errore: "«" + x + "» non è un dado (es. 2d6) né un numero." };
  }
  if (!dadi) return { errore: "La formula non ha dadi." };
  if (dadi > 50 || mod < -50 || mod > 50) return { errore: "Al massimo 50 dadi e un modificatore tra −50 e +50." };
  const gruppi = []; // il server accetta al massimo 20 dadi per gruppo e 7 gruppi
  for (const [facce, n] of per) for (let r = n; r > 0; r -= 20) gruppi.push({ n: Math.min(r, 20), facce });
  if (gruppi.length > 7) return { errore: "Troppi tipi di dadi nella stessa formula." };
  return { dadi: gruppi, mod };
}
