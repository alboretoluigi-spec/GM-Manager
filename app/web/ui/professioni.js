// Popup «Professione» (richiesta dell'utente, 2026-11-07): le sottofunzioni Raccolta, Estrazione, Fabbricazione,
// Inventario e Regole di una professione per un personaggio. I tiri e l'inventario li gestisce il server
// (/api/v1/professioni/...); monete e oggetti prodotti finiscono nella scheda. Catalogo: /regole/professioni.json.
import { el, svuota, api, avviso, dialogo, quando, tempoReale } from "./ui.js";

let catalogo = null;
export async function caricaCatalogo() {
  if (!catalogo) {
    const j = await (await fetch("/regole/professioni.json")).json();
    for (const a of j.artigiani || []) if (!j.professioni[a.id]) j.professioni[a.id] = { n: a.n, icona: "🔨", generico: true, formule: [], fonte: "PHB p. 187 (fabbricazione), XGtE (attività tra le avventure)" };
    catalogo = j;
  }
  return catalogo;
}

const segno = (n) => (n >= 0 ? "+" : "") + n;
const mo = (rame) => (rame / 100).toLocaleString("it-IT", { maximumFractionDigits: 2 }) + " mo";
let aperto = null; // { pg, prof, ricarica }

/** Apre il popup della professione `prof` per il personaggio `pg`. */
export async function apriProfessione(pg, prof, scheda = "raccolta") {
  const cat = await caricaCatalogo();
  const def = cat.professioni[prof];
  if (!def) { avviso("Professione sconosciuta.", "errore"); return; }
  let v = null, esito = null;
  if (!def.raccolta && !def.estrazione && scheda === "raccolta") scheda = "fabbricazione";
  const corpo = el("div", { class: "prof" }, el("p", { class: "tenue", testo: "Caricamento…" }));
  const chiudi = dialogo((def.icona || "⚒") + " " + def.n, corpo, { largo: true });
  const vie = [
    tempoReale.su("professione.esito", (d) => { if (d && d.personaggio === pg && d.professione === prof) { esito = d; ricarica(); } }),
    tempoReale.su("professione.giorno", () => ricarica()),
  ];
  const oss = new MutationObserver(() => { if (!document.body.contains(corpo)) { vie.forEach((f) => f()); oss.disconnect(); if (aperto && aperto.corpo === corpo) aperto = null; } });
  oss.observe(document.body, { childList: true, subtree: true });
  aperto = { pg, prof, corpo, ricarica };

  async function ricarica() {
    try { v = await api("/professioni/" + encodeURIComponent(pg) + "/" + encodeURIComponent(prof)); }
    catch (e) { svuota(corpo, el("p", { class: "errore", testo: e.message })); return; }
    disegna();
  }
  const quanti = (tipo, voce) => v.inventario.filter((x) => x.tipo === tipo && x.voce === voce).reduce((n, x) => n + x.quantita, 0);
  const nota = (f) => v.formule.find((x) => x.id === f.id)?.conosciuta;
  async function azione(percorso, dati) {
    try { esito = await api("/professioni/" + encodeURIComponent(pg) + "/" + encodeURIComponent(prof) + percorso, { metodo: dati === undefined ? "POST" : "POST", dati: dati || {} }); await ricarica(); }
    catch (e) { avviso(e.message, "errore", 6000); }
  }

  function disegna() {
    const schede = [
      def.raccolta ? ["raccolta", "🌿 Raccolta"] : null,
      def.estrazione ? ["estrazione", "🧪 Estrazione"] : null,
      ["fabbricazione", "⚒ Fabbricazione"],
      ["inventario", "🎒 Inventario"],
      ["regole", "📜 Regole"],
    ].filter(Boolean);
    svuota(corpo,
      el("p", { class: "prof-testa" }, el("b", { testo: v.personaggio.nome }),
        el("span", { class: "tenue", testo: " · livello " + (v.livello || "?") + " · competenza " + segno(v.bc) + " · Int " + segno(v.mod.INT) + " · Sag " + segno(v.mod.SAG) + " · monete " + mo(v.monete) }),
        v.master ? el("span", { class: "etichetta", testo: v.attiva ? "Master" : "Master · il personaggio non ha questa professione" }) : null),
      el("div", { class: "prof-schede riga-flex", role: "tablist" }, schede.map(([k, t]) => el("button", {
        class: "piccolo" + (k === scheda ? " primario" : ""), role: "tab", "aria-selected": String(k === scheda), testo: t,
        on: { click: () => { scheda = k; disegna(); } } }))),
      esito ? riquadroEsito(esito) : null,
      { raccolta, estrazione, fabbricazione, inventario, regole }[scheda]());
  }

  function riquadroEsito(e) {
    const t = e.tiro;
    return el("div", { class: "prof-esito " + (t ? (t.riuscito ? "ok" : "ko") : ""), "aria-live": "polite" },
      t ? el("p", {}, el("b", { testo: "🎲 " + t.etichetta + ": " }), "d20 " + t.d20.join(" e ") + (t.d20.length > 1 ? " (tenuto " + t.tenuto + ")" : "") + " " + segno(t.mod) + " = ",
        el("b", { testo: String(t.totale) }), " contro CD " + t.cd + " · ", el("b", { testo: t.riuscito ? "RIUSCITO ✔" : "FALLITO ✖" })) : null,
      el("p", { testo: e.testo }),
      el("button", { class: "piccolo", testo: "OK", on: { click: () => { esito = null; disegna(); } } }));
  }

  // ----- Raccolta (foraggiamento) -----
  function raccolta() {
    const ingr = def.ingredienti || [];
    const sel = el("select", { "aria-label": "Ingrediente" }, ingr.map((i) => el("option", { value: i.id, testo: i.n + " (CD " + i.cd + ")" })));
    const modo = el("select", { "aria-label": "Modalità" },
      el("option", { value: "viaggio", testo: "In viaggio (1 al giorno · fatte oggi: " + v.giorno.viaggio + ")" }),
      el("option", { value: "dedicata", testo: "Ricerca dedicata, 1 ora (4 al giorno · fatte oggi: " + v.giorno.dedicate + ")" }));
    const disp = el("select", { "aria-label": "Disponibilità" }, cat.disponibilita.map((d) => el("option", { value: d.cd, testo: d.n + " · CD " + d.cd })));
    const abil = el("select", { "aria-label": "Abilità" });
    const aiuto = el("input", { type: "checkbox" });
    const info = el("p", { class: "tenue piccolo" });
    const aggiorna = () => {
      const i = ingr.find((x) => x.id === sel.value); if (!i) return;
      disp.value = String(i.cd);
      svuota(abil, i.abil.map((a) => {
        const car = a === "Natura" ? "INT" : "SAG";
        const b = v.mod[car] + ((v.abilita || []).includes(a) ? v.bc : 0);
        return el("option", { value: a, testo: (a === "Natura" ? "Intelligenza (Natura) " : "Saggezza (Sopravvivenza) ") + segno(b) });
      }));
      info.textContent = "Dove: " + i.dove + ". In magazzino: " + quanti("ingrediente", i.id) + " lotti.";
    };
    sel.addEventListener("change", aggiorna);
    aggiorna();
    if (!ingr.length) return el("p", { class: "tenue", testo: "Questa professione non ha una raccolta nelle regole." });
    return el("section", { class: "prof-sez" },
      el("div", { class: "prof-griglia" }, el("label", {}, "Ingrediente", sel), el("label", {}, "Modalità", modo), el("label", {}, "Disponibilità", disp), el("label", {}, "Prova", abil)),
      info, el("label", { class: "riga" }, aiuto, el("span", { testo: "Un compagno aiuta (azione Aiutare): vantaggio" })),
      el("button", { class: "primario", testo: "🌿 Raccogli (tira)", on: { click: () => azione("/raccolta", { ingrediente: sel.value, modo: modo.value, cd: +disp.value, abilita: abil.value, aiuto: aiuto.checked }) } }),
      def.cibo ? el("p", { class: "tenue piccolo", testo: "Ricerca di cibo (DMG p. 111): se riesci trovi 1d6 + mod. Sag libbre di cibo e altrettanti galloni d'acqua, scritti nell'equipaggiamento." }) : null,
      el("ul", { class: "prof-regole" }, cat.regoleRaccolta.map((x) => el("li", { testo: x }))));
  }

  // ----- Estrazione delle secrezioni -----
  function estrazione() {
    const n = el("input", { type: "number", min: "1", max: "20", valore: "1", class: "corto", "aria-label": "Quante fiale" });
    const aiuto = el("input", { type: "checkbox" });
    return el("section", { class: "prof-sez" },
      el("p", {}, "Fiale vuote: ", el("b", { testo: String(quanti("fiala", "vuota")) }), " ", n,
        el("button", { class: "piccolo", testo: "Compra fiale (1 mo l'una)", on: { click: () => azione("/fiale", { n: +n.value }) } })),
      el("label", { class: "riga" }, aiuto, el("span", { testo: "Un compagno aiuta: vantaggio" })),
      (def.secrezioni || []).map((s) => el("div", { class: "prof-voce" },
        el("b", { testo: s.n }), el("span", { class: "tenue", testo: " · Intelligenza (Natura) CD " + s.cd + " · " + segno(v.mod.INT + v.bc) + " · in magazzino: " + quanti("secrezione", s.id) }),
        el("div", { class: "riga-flex" },
          s.tana ? el("button", { class: "piccolo", testo: quanti("tana", s.id) ? "✓ Tana trovata" : "Cerca la tana (Sopravvivenza CD " + s.tana + ", 1 ora)", disabled: quanti("tana", s.id) > 0,
            on: { click: () => azione("/tana", { secrezione: s.id, aiuto: aiuto.checked }) } }) : null,
          el("button", { class: "primario piccolo", testo: "🧪 Estrai (1d6 minuti)", on: { click: () => azione("/estrazione", { secrezione: s.id, aiuto: aiuto.checked }) } })))),
      el("ul", { class: "prof-regole" }, cat.regoleEstrazione.map((x) => el("li", { testo: x }))));
  }

  // ----- Fabbricazione -----
  function fabbricazione() {
    const form = (def.formule || []).map((f) => {
        const conosciuta = nota(f);
        const racc = (f.raccoglibili || []).map((id) => [id, quanti("ingrediente", id)]);
        const tuttiRaccolti = racc.length > 0 && racc.every(([, q]) => q > 0) && f.ridotto != null;
        const costo = tuttiRaccolti ? f.ridotto : f.pieno;
        const secr = (f.secrezioni || []).map((id) => [id, quanti("secrezione", id)]);
        const opz = (f.opzionali || []).map((o) => { const q = quanti("speciale", o.id) + quanti("secrezione", o.id); return [o, q, el("input", { type: "checkbox", disabled: !q })]; });
        const nomeIngr = (id) => (def.ingredienti || []).concat(def.secrezioni || [], def.speciali || []).find((x) => x.id === id)?.n || id;
        const dosi = f.dosi === "BC" ? v.bc + " (= competenza)" : String(f.dosi) + (f.bonus5 ? " (2 se superi la CD di 5)" : "");
        return el("div", { class: "prof-voce" + (conosciuta ? "" : " spento") },
          el("div", { class: "riga-flex" }, el("b", { testo: f.n }),
            f.proposta ? el("span", { class: "etichetta attesa", title: "Regola della casa proposta: il costo ridotto con gli ingredienti raccolti va confermato dal Master", testo: "proposta" }) : null,
            !conosciuta ? el("span", { class: "etichetta", testo: f.talento ? "serve il talento Avvelenatore: la sblocca il Master" : "da sbloccare (Master)" }) : null,
            v.master ? el("button", { class: "piccolo", testo: conosciuta ? "Togli la formula" : "Sblocca", on: { click: async () => {
              try { await api("/professioni/" + encodeURIComponent(pg) + "/" + encodeURIComponent(prof) + "/formule", { metodo: "PUT", dati: { formula: f.id, conosciuta: !conosciuta } }); await ricarica(); }
              catch (e) { avviso(e.message, "errore"); }
            } } }) : null),
          el("p", { class: "tenue piccolo", testo: "Tempo " + f.tempo + " · " + (f.cd ? "prova di Int con il kit CD " + f.cd + " (" + segno(v.mod.INT + v.bc) + ")" : "nessuna prova") + " · costo " +
            (f.ridotto != null ? f.pieno.toLocaleString("it-IT") + " mo pieno / " + f.ridotto.toLocaleString("it-IT") + " mo con i raccolti" : f.pieno.toLocaleString("it-IT") + " mo") + " · dosi " + dosi }),
          racc.length ? el("p", { class: "piccolo" }, "Raccolti: ", racc.map(([id, q]) => el("span", { class: "etichetta" + (q ? " approvato" : ""), testo: (q ? "✓ " : "✗ ") + nomeIngr(id) + (q ? " (" + q + ")" : "") }))) : null,
          secr.length ? el("p", { class: "piccolo" }, "Secrezione (non si compra): ", secr.map(([id, q]) => el("span", { class: "etichetta" + (q ? " approvato" : " rifiutato"), testo: (q ? "✓ " : "✗ ") + nomeIngr(id) }))) : null,
          opz.length ? opz.map(([o, q, cb]) => el("label", { class: "riga piccolo" }, cb, el("span", { testo: "Facoltativo: " + nomeIngr(o.id) + " → " + o.effetto + (q ? "" : " (non ce l'hai)") }))) : null,
          f.ts || f.effetto ? el("p", { class: "piccolo", testo: (f.ts ? "TS " + f.ts + ". " : "") + (f.effetto || "") + (f.immuni ? " Nessun effetto su non morti e costrutti." : "") }) : null,
          el("button", { class: "primario piccolo", disabled: !conosciuta && !v.master || secr.some(([, q]) => !q), testo: "⚒ Prepara (" + costo.toLocaleString("it-IT") + " mo)", on: { click: () =>
            azione("/fabbrica", { formula: f.id, opzionali: opz.filter(([, , cb]) => cb.checked).map(([o]) => o.id) }) } }));
      });
    const generico = def.generico || prof === "alchimista" ? (() => {
      const ogg = el("input", { maxlength: "80", placeholder: "Oggetto (es. Spada lunga)" });
      const prezzo = el("input", { type: "number", min: "0.01", step: "0.01", placeholder: "Prezzo in mo", class: "corto" });
      return el("div", { class: "prof-voce" }, el("b", { testo: "Oggetto del mestiere (regola generale)" }),
        el("p", { class: "tenue piccolo", testo: "Materiali a metà del prezzo; 5 mo di lavoro al giorno (Manuale del Giocatore) o 50 mo a settimana (supplemento). L'oggetto va nell'equipaggiamento." }),
        el("div", { class: "riga-flex" }, ogg, prezzo, el("button", { class: "primario piccolo", testo: "⚒ Fabbrica", on: { click: () =>
          azione("/fabbrica", { formula: "generico", oggetto: ogg.value, prezzo: parseFloat(String(prezzo.value).replace(",", ".")) || 0 }) } })));
    })() : null;
    return el("section", { class: "prof-sez" }, form, generico,
      def.formule && def.formule.some((f) => f.cd) ? el("ul", { class: "prof-regole" }, cat.regoleCreazione.map((x) => el("li", { testo: x }))) : null);
  }

  // ----- Inventario -----
  function inventario() {
    const nomi = { ingrediente: "Ingredienti raccolti", secrezione: "Secrezioni", speciale: "Ingredienti speciali", fiala: "Fiale", dose: "Dosi pronte", tana: "Tane trovate (oggi)" };
    const gruppi = Object.keys(nomi).map((k) => [k, v.inventario.filter((x) => x.tipo === k)]).filter(([, l]) => l.length);
    return el("section", { class: "prof-sez" },
      gruppi.length ? gruppi.map(([k, l]) => [el("h3", { testo: nomi[k] }), el("ul", { class: "prof-inv" }, l.map((x) => el("li", {},
        el("b", { testo: x.quantita + " × " + x.nome }), x.variante ? el("span", { class: "tenue", testo: " (con " + x.variante + ")" }) : null,
        x.giorni != null ? el("span", { class: "tenue", testo: " · " + x.giorni + " giorni rimasti" }) : null,
        k === "dose" ? el("button", { class: "piccolo", testo: "🗡 Applica una dose", on: { click: () => azione("/applica", { formula: x.voce, variante: x.variante || "" }) } }) : null)))])
        : el("p", { class: "tenue", testo: "Inventario vuoto." }),
      def.dosi ? el("ul", { class: "prof-regole" }, cat.regoleUso.map((x) => el("li", { testo: x }))) : null,
      v.master ? strumentiMaster() : null,
      v.registro.length ? [el("h3", { testo: "Registro" }), el("ul", { class: "prof-registro" }, v.registro.map((r) => el("li", {},
        el("span", { class: "tenue", testo: quando(r.quando) + " · " + r.chi + " · " }), r.testo)))] : null);
  }

  function strumentiMaster() {
    const voci = [...(def.ingredienti || []).map((x) => ["ingrediente", x]), ...(def.secrezioni || []).map((x) => ["secrezione", x]),
      ...(def.speciali || []).map((x) => ["speciale", x]), ["fiala", { id: "vuota", n: "Fiala vuota" }], ...(def.formule || []).map((x) => ["dose", x])];
    const sel = el("select", { "aria-label": "Voce" }, voci.map(([t, x], i) => el("option", { value: i, testo: { ingrediente: "Ingrediente", secrezione: "Secrezione", speciale: "Speciale", fiala: "Fiala", dose: "Dose" }[t] + ": " + x.n })));
    const q = el("input", { type: "number", min: "-50", max: "50", valore: "1", class: "corto", "aria-label": "Quantità" });
    return el("details", { class: "prof-master" }, el("summary", { testo: "🛠 Strumenti del Master" }),
      el("p", { class: "tenue piccolo", testo: "Aggiungi o togli voci (per esempio ectoplasma o terra vampirica, «a discrezione del DM»). Quantità negativa per togliere." }),
      el("div", { class: "riga-flex" }, sel, q, el("button", { class: "piccolo", testo: "Applica", on: { click: async () => {
        const [tipo, x] = voci[+sel.value];
        try { await api("/professioni/" + encodeURIComponent(pg) + "/" + encodeURIComponent(prof) + "/inventario", { dati: { tipo, voce: x.id, quantita: +q.value } }); await ricarica(); }
        catch (e) { avviso(e.message, "errore"); }
      } } })),
      el("button", { class: "piccolo", testo: "⏭ Passa un giorno di gioco (per tutti)", on: { click: passaGiorno } }));
  }

  function regole() {
    return el("section", { class: "prof-sez" },
      el("p", { class: "tenue", testo: "Fonte: " + (def.fonte || "") }),
      def.raccolta ? [el("h3", { testo: "Raccolta" }), el("ul", { class: "prof-regole" }, cat.regoleRaccolta.map((x) => el("li", { testo: x })))] : null,
      def.estrazione ? [el("h3", { testo: "Estrazione" }), el("ul", { class: "prof-regole" }, cat.regoleEstrazione.map((x) => el("li", { testo: x })))] : null,
      (def.formule || []).some((f) => f.cd) ? [el("h3", { testo: "Creazione" }), el("ul", { class: "prof-regole" }, cat.regoleCreazione.map((x) => el("li", { testo: x })))] : null,
      def.dosi ? [el("h3", { testo: "Uso dei veleni" }), el("ul", { class: "prof-regole" }, cat.regoleUso.map((x) => el("li", { testo: x })))] : null,
      def.generico ? el("p", { testo: "Fabbricazione con la regola generale: materiali a metà del prezzo; ogni giorno di lavoro vale 5 mo (Manuale del Giocatore) oppure 50 mo a settimana (supplemento)." }) : null);
  }

  await ricarica();
  return chiudi;
}

/** Il Master fa passare un giorno di gioco: gli ingredienti invecchiano, i contatori del giorno si azzerano. */
export async function passaGiorno() {
  try { const r = await api("/professioni-giorno", { dati: {} }); avviso("Un giorno di gioco è passato" + (r.scaduti ? ": " + r.scaduti + " voci scadute." : ".")); }
  catch (e) { avviso(e.message, "errore"); }
}

/** Avvisi in tutta l'app: il Master (e il giocatore del personaggio) vede gli esiti, se il popup non è aperto. */
let installato = false;
export function installaProfessioni() {
  if (installato) return;
  installato = true;
  tempoReale.su("professione.esito", (d) => {
    if (!d || (aperto && aperto.pg === d.personaggio && aperto.prof === d.professione && document.body.contains(aperto.corpo))) return;
    const t = d.tiro;
    avviso("⚒ " + d.nome + " · " + d.nomeProfessione + ": " + d.testo + (t ? " (" + t.totale + " contro CD " + t.cd + ")" : ""), "", 8000);
  });
}
