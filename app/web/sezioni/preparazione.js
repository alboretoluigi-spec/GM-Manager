// Sezione «🧭 Preparazione sessione» (solo Master; fase L5 del Codice della campagna, 2026-11-12). Per un gruppo e la
// sua prossima sessione (o quella in corso): chi viene, dove e quando, dove eravamo rimasti, promemoria degli eventi,
// schede pertinenti del Codice con il motivo, decisioni aperte, bilanciamenti salvati, PNG e mostri a portata di mano,
// note della sessione e la domanda pronta per chiedere all'IA una scaletta. Server: internal/server/preparazione.go.
import { el, svuota, api, avviso } from "../ui/ui.js";
import { ICONE, riferimenti, riferimentiDi, norma } from "../ui/codice_link.js";
import { apriBilanciamento } from "../ui/bilanciamento.js";

// stato per il gruppo, con le parole di ogni tipo di scheda (come in sezioni/lore.js)
const PAROLE = { luogo: ["sentito nominare", "visitato", "distrutto o chiuso"], png: ["sentito nominare", "incontrato", "morto o uscito di scena"],
  fazione: ["sentita nominare", "incontrata", "sciolta o sconfitta"], evento: ["annunciato", "avvenuto", "concluso"], trama: ["avviata", "in corso", "conclusa"],
  oggetto: ["sentito nominare", "trovato", "perso o distrutto"], segreto: ["intuito", "scoperto", "risolto"], cronologia: ["sentita raccontare", "conosciuta", "chiusa"] };
const etStato = (tipo, st) => (PAROLE[tipo] || PAROLE.luogo)[["conosciuto", "incontrato", "concluso"].indexOf(st)] || st;
const ora = (o) => { const h = Math.floor(o || 0) % 24, m = Math.round(((o || 0) % 1) * 60) % 60; return String(h).padStart(2, "0") + ":" + String(m).padStart(2, "0"); };

export async function mostra(corpo, { io, azioni }) {
  if (!io || !io.utente || io.utente.ruolo !== "master") { svuota(corpo, el("p", { class: "errore", testo: "Questa sezione è solo per il Master." })); return null; }
  const qs = new URLSearchParams(location.search);
  let gruppi = await api("/gruppi").catch(() => []);
  gruppi = Array.isArray(gruppi) ? gruppi : [];
  if (!gruppi.length) { svuota(corpo, el("div", { class: "vuoto" }, el("h2", { testo: "Nessun gruppo ancora" }), el("p", { testo: "Crea un gruppo in «Sessioni»." }))); return null; }
  const selGruppo = el("select", { "aria-label": "Gruppo" }, gruppi.map((g) => el("option", { value: g.id, selected: g.id === qs.get("gruppo"), testo: g.nome })));
  const selSessione = el("select", { "aria-label": "Sessione" });
  const pagina = el("div", { class: "prep" });
  const scaletta = el("a", { class: "pulsante piccolo primario", "data-interno": true, testo: "✨ Scaletta con l'IA", title: "Apre l'assistente con la domanda pronta: la invii tu" });
  let rif = null;
  riferimenti().then((r) => { rif = r; disegnaPNG(); });
  let d = null;

  async function carica(sessione) {
    try { d = await api("/preparazione?gruppo=" + encodeURIComponent(selGruppo.value) + (sessione ? "&sessione=" + encodeURIComponent(sessione) : "")); }
    catch (e) { svuota(pagina, el("p", { class: "errore", testo: e.message })); return; }
    svuota(selSessione, el("option", { value: "-", testo: "Nessuna sessione in programma" }),
      d.sessioni.map((x) => el("option", { value: x.id, selected: d.sessione && x.id === d.sessione.id, testo: x.titolo + (x.stato === "in_corso" ? " (in corso)" : "") + " · " + new Date(x.inizio * 1000).toLocaleDateString("it-IT") })));
    scaletta.href = "/assistente?gruppo=" + encodeURIComponent(d.gruppo) + "&domanda=" + encodeURIComponent(d.domandaIA);
    disegna();
  }

  const riquadro = (titolo, ...contenuto) => el("section", { class: "scheda-ui prep-box" }, el("h3", { testo: titolo }), ...contenuto);
  const vuoto = (t) => el("p", { class: "tenue", testo: t });
  const linkScheda = (x) => el("a", { href: "/codice?scheda=" + encodeURIComponent(x.id), "data-interno": true, testo: (ICONE[x.tipo] || "🗝") + " " + x.nome });
  const boxPNG = el("div", { class: "prep-png riga-flex" });

  function disegnaPNG() {
    if (!d) return;
    if (!rif) { svuota(boxPNG, vuoto("Cerco in «PNG e mostri»…")); return; }
    const visti = new Set(), voci = [];
    for (const x of [...d.schede, ...d.promemoria.map((p) => d.schede.find((y) => y.id === p.scheda)).filter(Boolean)]) {
      for (const v of riferimentiDi(x, rif)) if (v.href.startsWith("/bestiario") && !visti.has(v.href)) { visti.add(v.href); voci.push(v); }
    }
    svuota(boxPNG, voci.length ? voci.map((v) => el("a", { class: "pulsante piccolo", href: v.href, "data-interno": true, testo: v.testo }))
      : vuoto("Nessuna voce di «PNG e mostri» legata alle schede di oggi (si collegano per nome o con il campo «Statistiche: «Nome»»)."));
  }

  function disegna() {
    const s = d.sessione;
    // chi viene
    const chi = riquadro("👥 Chi viene" + (s ? "" : " (personaggi attivi del gruppo)"),
      d.presenti.length ? el("ul", { class: "prep-lista" }, d.presenti.map((p) => el("li", {},
        el("b", { testo: p.nome }), " ",
        el("span", { class: "tenue", testo: [p.giocatore, [p.classi, p.livello ? "liv. " + p.livello : ""].filter(Boolean).join(" "), p.pfMax ? "PF " + (p.pf ?? p.pfMax) + "/" + p.pfMax : ""].filter(Boolean).join(" · ") }),
        s && !p.confermato ? el("span", { class: "etichetta tenue", testo: "non ancora confermato" }) : null,
        p.condizioni.length ? el("span", { class: "etichetta", testo: p.condizioni.join(", ") }) : null)))
        : vuoto(s ? "Nessuno ha ancora scelto il personaggio per questa sessione." : "Il gruppo non ha personaggi attivi."));
    // dove e quando
    const loc = d.luogo && rif && rif.luoghi.find((l) => norma(l.nome) === norma(d.luogo));
    const schedaLuogo = d.luogo && d.schede.find((x) => norma(x.nome) === norma(d.luogo));
    const dove = riquadro("🗺 Dove e quando",
      el("p", {}, d.luogo ? ["Il segnalino del gruppo è a ", el("b", { testo: d.luogo }), ". "] : "Il segnalino del gruppo non è vicino a una località della mappa. ",
        "Giorno di gioco ", el("b", { testo: String(d.giorno) }), ", ore ", el("b", { testo: ora(d.ore) }), "."),
      el("div", { class: "riga-flex" },
        loc ? el("a", { class: "pulsante piccolo", href: "/mappa?loc=" + encodeURIComponent(loc.k), "data-interno": true, testo: "🗺 " + d.luogo + " sulla mappa" }) : null,
        schedaLuogo ? el("a", { class: "pulsante piccolo", href: "/codice?scheda=" + encodeURIComponent(schedaLuogo.id), "data-interno": true, testo: "🗝 " + schedaLuogo.nome + " nel Codice" }) : null),
      s && s.luogo ? el("p", { class: "tenue", testo: "Luogo scritto nella sessione: " + s.luogo }) : null);
    // dove eravamo rimasti
    const r = d.riassunto;
    const rimasti = riquadro("📝 Dove eravamo rimasti",
      r ? [el("p", { class: "tenue piccolo", testo: (r.titolo ? "«" + r.titolo + "» · " : "") + r.autore + " · " + new Date(r.quando * 1000).toLocaleDateString("it-IT") }),
        el("div", { class: "prep-testo", testo: r.testo }),
        el("a", { class: "pulsante piccolo", href: "/riassunti", "data-interno": true, testo: "📝 Tutti i riassunti" })]
        : vuoto("Nessun riassunto scritto per questo gruppo."));
    // promemoria
    const promemoria = riquadro("🔔 Promemoria degli eventi",
      d.promemoria.length ? d.promemoria.map((p) => el("div", { class: "prep-prom riga-flex" },
        el("span", {}, el("a", { href: "/codice?scheda=" + encodeURIComponent(p.scheda), "data-interno": true, testo: (ICONE[p.tipo] || "⚡") + " " + p.nome }), el("span", { class: "tenue", testo: " — " + p.motivo })),
        el("button", { class: "piccolo", testo: "✓ Avvenuto", on: { click: () => decidi(p, "avvenuto") } }),
        el("button", { class: "piccolo", testo: "Ignora", on: { click: () => decidi(p, "ignora") } })))
        : vuoto("Nessun evento pronto a scattare per questo gruppo."));
    // schede pertinenti
    const pertinenti = riquadro("🗝 Dal Codice della campagna",
      el("p", { class: "tenue piccolo", testo: "Le stesse schede che riceve l'assistente IA per questo gruppo, con il motivo." }),
      d.schede.length ? el("ul", { class: "prep-lista prep-schede" }, d.schede.map((x) => el("li", {},
        linkScheda(x), " ",
        x.stato ? el("span", { class: "lore-stato " + x.stato, testo: etStato(x.tipo, x.stato) }) : null,
        (x.etichette || []).filter((e) => /^(da decidere|proposta|variante)$/i.test(e)).map((e) => el("span", { class: "etichetta", testo: e })),
        el("div", { class: "tenue piccolo", testo: x.motivi.join(" · ") }),
        x.sintesi ? el("div", { class: "piccolo", testo: x.sintesi }) : null)))
        : vuoto("Nessuna scheda pertinente: posiziona il segnalino del gruppo sulla mappa o segna nel Codice cosa il gruppo conosce."));
    const decisioni = riquadro("❓ Decisioni aperte",
      d.decisioni.length ? d.decisioni.map((x) => el("details", { class: "prep-det" }, el("summary", {}, el("b", { testo: x.nome }), el("span", { class: "tenue piccolo", testo: " — " + x.motivi.join(" · ") })),
        el("div", { class: "prep-testo", testo: x.testo }), linkScheda(x)))
        : vuoto("Nessuna decisione aperta legata alle schede di oggi."));
    const bil = riquadro("⚖ Bilanciamenti del gruppo",
      d.bilanciamenti.length ? d.bilanciamenti.map((x) => el("details", { class: "prep-det" }, el("summary", {}, el("b", { testo: x.nome.replace(/^Bilanciamento – /, "") }), el("span", { class: "tenue piccolo", testo: " — difficoltà " + ((x.campi || {})["Difficoltà"] || "") })),
        el("div", { class: "prep-testo", testo: x.testo }), linkScheda(x)))
        : vuoto("Nessun bilanciamento salvato per questo gruppo."),
      el("button", { class: "piccolo", testo: "⚖ Calcola un bilanciamento", on: { click: () => apriBilanciamento("", { gruppi, gruppo: selGruppo.value }) } }));
    const png = riquadro("📖 PNG e mostri a portata di mano", boxPNG);
    // note della sessione (le stesse di «Sessioni»)
    const note = el("textarea", { class: "testo-lungo", rows: "5", maxlength: "1000", "aria-label": "Note della sessione", valore: s ? s.note : "", disabled: !s });
    const note_ = riquadro("✏ Note della sessione", note,
      el("div", { class: "riga-flex" }, el("button", { class: "piccolo primario", testo: "Salva le note", disabled: !s, on: { click: async () => {
        try { await api("/sessioni/" + encodeURIComponent(s.id), { metodo: "PATCH", dati: { note: note.value } }); s.note = note.value; avviso("Note salvate."); } catch (e) { avviso(e.message, "errore"); }
      } } }), s ? el("span", { class: "tenue piccolo", testo: "Le vedi anche in «Sessioni». Massimo 1000 caratteri." }) : el("span", { class: "tenue piccolo", testo: "Crea una sessione in «Sessioni» per scrivere le note." })));
    svuota(pagina, el("div", { class: "prep-griglia" }, chi, dove, rimasti, promemoria), pertinenti, el("div", { class: "prep-griglia" }, decisioni, bil, png, note_));
    disegnaPNG();
  }

  async function decidi(p, azione) {
    try { await api("/lore/" + encodeURIComponent(p.scheda) + "/promemoria", { metodo: "POST", dati: { gruppo: p.gruppo, azione } }); avviso(azione === "avvenuto" ? "Segnato come avvenuto." : "Promemoria ignorato."); }
    catch (e) { avviso(e.message, "errore"); }
    carica(selSessione.value);
  }

  selGruppo.addEventListener("change", () => carica(""));
  selSessione.addEventListener("change", () => carica(selSessione.value));
  const ricarica = () => carica(selSessione.value);
  document.addEventListener("gasterbless:promemoria", ricarica);
  document.addEventListener("gasterbless:codice-cambiato", ricarica);
  svuota(azioni, scaletta);
  svuota(corpo, el("div", { class: "largo" },
    el("div", { class: "riga-flex prep-scelte" },
      el("label", { class: "riga" }, el("span", { class: "tenue piccolo", testo: "Gruppo" }), selGruppo),
      el("label", { class: "riga" }, el("span", { class: "tenue piccolo", testo: "Sessione" }), selSessione)),
    pagina));
  await carica(qs.get("sessione") || "");
  return () => { svuota(azioni); document.removeEventListener("gasterbless:promemoria", ricarica); document.removeEventListener("gasterbless:codice-cambiato", ricarica); };
}
