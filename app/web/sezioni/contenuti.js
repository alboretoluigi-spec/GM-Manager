// Contenuti di gioco (fase 3c, passo C1, docs/FORMATO_CONTENUTI.md): l'app non include classi, razze, incantesimi o
// altro materiale; il Master importa pacchetti nel formato aperto «gasterbless-contenuti» (un file .json che prepara
// lui o che si procura), li controlla prima di importarli, li attiva o li disattiva, li esporta e li elimina. I pacchetti
// attivi arrivano alla scheda del personaggio di tutti i giocatori.
import { el, svuota, api, avviso, dialogo, conferma, tempoReale } from "../ui/ui.js";

const TIPI = [["classi", "classi"], ["razze", "razze"], ["background", "background"], ["incantesimi", "incantesimi"], ["armi", "armi"],
  ["armature", "armature"], ["talenti", "talenti"], ["stili", "stili di combattimento"]];

function conteggi(c) {
  const p = TIPI.filter(([k]) => c && c[k]).map(([k, n]) => c[k] + " " + n);
  return p.length ? p.join(" · ") : "nessuna voce";
}

export async function mostra(corpo) {
  const stato = { pacchetti: [], sovrascritti: [] };
  const elenco = el("div", { class: "ct-elenco", "aria-live": "polite" });
  const scegli = el("input", { type: "file", accept: ".json,application/json", hidden: true, "aria-label": "File del pacchetto", on: { change: () => {
    const f = scegli.files[0]; scegli.value = ""; if (f) anteprima(f);
  } } });
  svuota(corpo, el("div", { class: "largo rl" },
    el("div", { class: "rl-banda" },
      el("div", {}, el("h2", { class: "rl-titolo", testo: "Contenuti di gioco" }),
        el("p", { class: "rl-sotto", testo: "Classi, razze, background, incantesimi, armi, armature e talenti che importi tu" }))),
    el("div", { class: "scheda-ui ct-info" },
      el("p", { testo: "Gaster Bless non contiene nessun contenuto di gioco. Importa qui i pacchetti che prepari o che ti procuri, " +
        "nel formato aperto «gasterbless-contenuti» (un file .json). I pacchetti attivi compaiono nella scheda del personaggio di tutti i giocatori." }),
      el("p", { class: "tenue piccolo", testo: "Importa solo materiale che hai il diritto di usare. Per ora i pacchetti valgono per il sistema «d20-5e» (quinta edizione)." })),
    el("div", { class: "mr-comandi" },
      el("button", { class: "primario", type: "button", testo: "📥 Importa un pacchetto…", on: { click: () => scegli.click() } }),
      el("a", { class: "pulsante", href: "/modelli/modello_pacchetto.json", download: "modello_pacchetto.json", testo: "⬇ Scarica il modello" }),
      scegli),
    elenco));

  async function ricarica() {
    try { const r = await api("/contenuti"); stato.pacchetti = r.pacchetti; stato.sovrascritti = r.sovrascritti || []; }
    catch (e) { avviso(e.message, "errore"); return; }
    disegna();
  }

  function disegna() {
    if (!stato.pacchetti.length) {
      svuota(elenco, el("p", { class: "tenue ct-vuoto", testo: "Nessun pacchetto importato: la scheda del personaggio non ha classi, razze né incantesimi. Comincia dal modello." }));
      return;
    }
    svuota(elenco,
      stato.sovrascritti.length ? el("div", { class: "scheda-ui ct-avviso", role: "status" },
        el("strong", { testo: "Voci presenti in più pacchetti attivi" }),
        el("p", { class: "piccolo", testo: "Vale quella del pacchetto importato per ultimo: " + stato.sovrascritti.join(", ") + "." })) : null,
      stato.pacchetti.map(schedaPacchetto));
  }

  function schedaPacchetto(p) {
    const att = el("input", { type: "checkbox", selezionato: p.attivo, "aria-label": "Attivo: " + p.nome, on: { change: async () => {
      try { await api("/contenuti/" + encodeURIComponent(p.id), { metodo: "PATCH", dati: { attivo: att.checked } });
        avviso(att.checked ? "«" + p.nome + "» attivato." : "«" + p.nome + "» disattivato: non compare più nelle schede."); ricarica(); }
      catch (e) { att.checked = !att.checked; avviso(e.message, "errore"); }
    } } });
    const dett = [p.versione && "versione " + p.versione, p.autore && "di " + p.autore, p.sistema].filter(Boolean).join(" · ");
    return el("div", { class: "scheda-ui ct-pacchetto" + (p.attivo ? "" : " ct-spento"), "data-id": p.id },
      el("div", { class: "ct-testa" },
        el("h3", { testo: "📦 " + p.nome }),
        el("label", { class: "riga ct-attivo" }, att, el("span", { testo: "Attivo" }))),
      dett ? el("p", { class: "tenue piccolo", testo: dett }) : null,
      el("p", { class: "ct-conteggi", testo: conteggi(p.conteggi) }),
      p.licenza ? el("p", { class: "piccolo", testo: "Licenza: " + p.licenza }) : null,
      p.descrizione ? el("p", { class: "piccolo", testo: p.descrizione }) : null,
      el("div", { class: "mr-comandi" },
        el("a", { class: "pulsante", href: "/api/v1/contenuti/" + encodeURIComponent(p.id) + "/file", testo: "⬇ Esporta" }),
        el("button", { type: "button", class: "pericolo", testo: "🗑 Elimina", on: { click: async () => {
          if (!(await conferma("Eliminare il pacchetto?", "«" + p.nome + "»: le sue voci spariscono dalle schede. I personaggi già creati restano, ma la scheda non riconoscerà più classi, razze o incantesimi del pacchetto.", "Elimina", true))) return;
          try { await api("/contenuti/" + encodeURIComponent(p.id), { metodo: "DELETE" }); avviso("Pacchetto eliminato."); ricarica(); }
          catch (e) { avviso(e.message, "errore"); }
        } } })));
  }

  async function anteprima(file) {
    let testo;
    try { testo = await file.text(); } catch { avviso("Non riesco a leggere il file.", "errore"); return; }
    let r;
    try { r = await api("/contenuti/verifica", { corpo: testo, tipo: "application/json" }); }
    catch (e) {
      dialogo("Il pacchetto non si può importare", el("div", { class: "ct-errori" },
        el("p", { testo: "«" + file.name + "»" }), el("pre", { class: "ct-pre", role: "alert", testo: e.message }),
        el("p", { class: "tenue piccolo", testo: "Correggi il file e riprova. Il formato è descritto nella guida «Formato dei pacchetti di contenuti»; il modello mostra tutti i campi." })),
        { largo: true });
      return;
    }
    const corpoD = el("div", { class: "ct-anteprima" },
      el("p", {}, el("strong", { testo: r.nome }), r.versione ? " · versione " + r.versione : "", r.autore ? " · di " + r.autore : ""),
      el("p", { testo: "Contiene: " + conteggi(r.conteggi) + "." }),
      r.licenza ? el("p", { class: "piccolo", testo: "Licenza: " + r.licenza }) : null,
      r.avvisi && r.avvisi.length ? el("div", { class: "ct-avviso" }, el("strong", { testo: "Da controllare (" + r.avvisi.length + ")" }),
        el("ul", {}, r.avvisi.map((a) => el("li", { class: "piccolo", testo: a })))) : null,
      r.doppione ? el("p", { class: "errore", role: "alert", testo: "Questo pacchetto è già stato importato: «" + r.doppione + "»." }) : null);
    dialogo("Importare questo pacchetto?", corpoD, { azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { type: "button", testo: "Annulla", on: { click: chiudi } }),
      r.doppione ? null : el("button", { type: "button", class: "primario", testo: "📥 Importa", on: { click: async (ev) => {
        ev.currentTarget.disabled = true;
        try { await api("/contenuti", { corpo: testo, tipo: "application/json" }); chiudi(); avviso("«" + r.nome + "» importato e attivo."); ricarica(); }
        catch (e) { avviso(e.message, "errore"); chiudi(); }
      } } })) });
  }

  const via = tempoReale.su("contenuti", () => ricarica());
  await ricarica();
  return () => via();
}
