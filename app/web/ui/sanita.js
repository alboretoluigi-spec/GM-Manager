// Schermata Sanità mentale (giocatore e Master) e strumenti del Master per paura, Terrore e pazzia.
// Tutto lo stato è sul server: questa pagina lo mostra e manda i comandi. Il tiro "Soccombi alla paura"
// lo fa solo il server, una volta per evento; giocatore e Master ricevono lo stesso risultato.
import { el, svuota, api, avviso, dialogo, conferma, quando, tempoReale } from "./ui.js";

const CAR = { SAN: "Sanità mentale", SAG: "Saggezza", CAR: "Carisma", COS: "Costituzione" };
const VANT = { normale: "nessuno", vantaggio: "vantaggio", svantaggio: "svantaggio" };
const METODO = { DMG_STANDARD: "Ufficiale DMG 2014 (settima caratteristica)", WISDOM_DERIVED: "Homebrew: 10 + modificatore di Saggezza", MANUAL: "Inserito a mano dal Master" };
const segno = (n) => (n >= 0 ? "+" : "") + n;
let aperta = null; // { pg, ridisegna, chiudi }

/** Apre (o aggiorna) la schermata Sanità mentale di un personaggio. */
export function apriSanita(pg, io) {
  if (aperta && aperta.pg === pg) { aperta.ricarica(); return; }
  if (aperta) aperta.chiudi();
  let v = null, inCorso = false, errore = "";
  const corpo = el("div", { class: "sanita-corpo" }, el("p", { class: "tenue", testo: "Caricamento…" }));
  const chiudiBtn = el("button", { class: "sanita-chiudi", testo: "✕", "aria-label": "Chiudi la schermata Sanità mentale", on: { click: () => chiudi() } });
  const velo = el("div", { class: "sanita-velo", role: "dialog", "aria-modal": "true", "aria-label": "Sanità mentale" }, el("div", { class: "sanita" }, chiudiBtn, corpo));
  document.body.append(velo);
  const esc = (e) => { if (e.key === "Escape" && !document.querySelector(".velo")) chiudi(); };
  document.addEventListener("keydown", esc);
  function chiudi() { velo.remove(); document.removeEventListener("keydown", esc); vie.forEach((f) => f()); if (aperta && aperta.pg === pg) aperta = null; }

  async function ricarica() {
    try { v = await tempoReale.chiedi("sanita.apri", { personaggio: pg }); errore = ""; }
    catch (e) { try { v = await api("/sanita/pg/" + encodeURIComponent(pg)); errore = ""; } catch (e2) { errore = e2.message || e.message; } }
    disegna();
  }
  const vie = [
    tempoReale.su("sanita.stato", (d) => { if (d && d.personaggio === pg) ricaricaSilenziosa(); }),
    tempoReale.su("sanita.config", () => ricaricaSilenziosa()),
  ];
  const riconnesso = () => ricaricaSilenziosa();
  tempoReale.stato.addEventListener("aperto", riconnesso);
  vie.push(() => tempoReale.stato.removeEventListener("aperto", riconnesso));
  // Aggiornamenti dal server: si rilegge senza riannunciare l'apertura e senza cancellare ciò che il Master sta scrivendo.
  async function ricaricaSilenziosa() {
    try { v = await api("/sanita/pg/" + encodeURIComponent(pg)); } catch (e) { errore = e.message; }
    const a = document.activeElement;
    if (velo.contains(a) && (a.tagName === "TEXTAREA" || (a.tagName === "INPUT" && !/checkbox|button/.test(a.type)))) return;
    disegna();
  }

  function disegna() {
    if (errore && !v) { svuota(corpo, el("p", { class: "errore", testo: errore })); return; }
    if (!v) return;
    const c = v.config, ev = v.evento, ult = v.ultimo && v.ultimo.risoluzione ? v.ultimo : null;
    const mio = v.personaggio.mio, master = v.master;
    const img = v.immagine ? el("img", { class: "sanita-img", src: "/api/v1/sanita/immagine", alt: "Illustrazione della paura: un uomo terrorizzato si stringe la testa tra le mani.", decoding: "async" })
      : el("div", { class: "sanita-img mancante", role: "img", "aria-label": "Immagine non caricata" },
        el("p", { testo: "Immagine della schermata non ancora caricata." }),
        el("p", { class: "tenue", testo: master ? "Caricala in Impostazioni → Sanità mentale e Terrore." : "Il Master deve caricarla." }));
    // Pulsante principale: solo il proprietario, solo con un evento attivo.
    let stato;
    if (!c.attiva) stato = el("p", { class: "tenue", testo: "La Sanità mentale non è attiva in questa campagna." });
    else if (!ev) stato = el("p", { class: "sanita-attesa", testo: "Nessun evento di paura attivo: il Master deve prepararne uno prima del tiro." });
    else if (ev.stato === "DRAFT") stato = el("p", { class: "sanita-attesa", testo: "Evento in preparazione (bozza): il giocatore non lo vede ancora." });
    else if (ev.stato === "ROLLING") stato = el("p", { class: "sanita-attesa", "aria-live": "polite", testo: "⏳ Tiro in corso…" });
    else if (ev.stato === "ERROR") stato = el("p", { class: "errore", testo: "La risoluzione non è riuscita: nessun risultato è definitivo. Il Master può riattivare o annullare l'evento." });
    else stato = el("div", { class: "sanita-evento" },
      el("b", { testo: "😱 " + ev.fonte }), ev.descrizione ? el("p", { testo: ev.descrizione }) : null,
      el("p", { class: "tenue", testo: "Tiro salvezza su " + CAR[ev.caratteristica] + " · CD " + ev.cd + (ev.vantaggio !== "normale" ? " · " + ev.vantaggio : "") + (ev.modificatore ? " · modificatore " + segno(ev.modificatore) : "") }));
    const pronto = ev && ev.stato === "READY";
    const bottone = el("button", { class: "primario sanita-soccombi", testo: "Soccombi alla paura", disabled: !(pronto && mio) || inCorso,
      title: !ev ? "Serve un evento preparato dal Master" : !mio ? "Il tiro lo avvia il giocatore" : "", on: { click: () => soccombi(ev) } });
    const terr = el("div", { class: "sanita-scala", role: "meter", "aria-valuemin": "0", "aria-valuemax": "7", "aria-valuenow": String(v.terrore), "aria-label": "Terrore " + v.terrore + ": " + v.livello.nome },
      v.scala.map((l) => el("span", { class: "t" + (l.livello <= v.terrore && l.livello > 0 ? " on" : "") + (l.livello === v.terrore ? " qui" : ""), title: l.livello + " · " + l.nome, testo: String(l.livello) })));
    svuota(corpo,
      el("header", { class: "sanita-testa" }, el("h2", { testo: "Sanità mentale · " + v.personaggio.nome }),
        el("span", { class: "etichetta", testo: METODO[v.metodo] || v.metodo })),
      el("div", { class: "sanita-griglia" },
        el("div", { class: "sanita-sx" }, img, stato, bottone),
        el("div", { class: "sanita-dx" },
          el("div", { class: "sanita-valori" },
            valore("Punteggio", v.punteggio ?? "—"), valore("Modificatore", v.mod != null ? segno(v.mod) : "—"),
            valore("Tiro salvezza", v.bonusTS != null ? segno(v.bonusTS) : "—", (v.competente ? "competente (+" + v.prof + ")" : "non competente") + (v.bonusTemp ? " · bonus temporaneo " + segno(v.bonusTemp) : ""))),
          v.punteggio == null ? el("p", { class: "errore", testo: "Manca il punteggio di Sanità mentale: va scritto nella scheda (o inserito dal Master)." }) : null,
          c.terrore ? el("section", { class: "sanita-sez" }, el("h3", {}, "Terrore ", el("span", { class: "etichetta", testo: "homebrew" })),
            terr, el("p", {}, el("b", { testo: v.terrore + " · " + v.livello.nome })),
            v.effetti.length ? el("ul", { class: "sanita-effetti" }, v.effetti.map((l) => el("li", {}, el("b", { testo: l.livello + " " + l.nome + ": " }), l.effetto,
              l.ufficiale ? el("span", { class: "etichetta", testo: "condizione ufficiale" }) : null))) : el("p", { class: "tenue", testo: "Nessun effetto di Terrore." }),
            tiriEffetti()) : el("p", { class: "tenue", testo: "Scala del Terrore (homebrew) non attiva in questa campagna." }),
          v.pazzie.filter((p) => p.attiva).length ? el("section", { class: "sanita-sez" }, el("h3", { testo: "Pazzie attive" }),
            v.pazzie.filter((p) => p.attiva).map((p) => el("p", {}, el("b", { testo: "Pazzia " + p.tipo + (p.categoria ? " · " + p.categoria : "") + ": " }),
              p.descrizione || (master ? "(testo da scrivere: il catalogo non ha questa voce)" : "il Master ti descriverà l'effetto."),
              el("span", { class: "tenue", testo: " · durata " + p.durata + (p.scadeIl ? " (fino alle " + new Date(p.scadeIl * 1000).toLocaleTimeString("it-IT", { hour: "2-digit", minute: "2-digit" }) + ")" : "") }))))
            : null,
          ult ? risultato(ult) : null)),
      master ? strumentiMaster() : null);
  }

  function valore(t, x, nota) { return el("div", { class: "sanita-valore" }, el("span", { class: "tenue", testo: t }), el("b", { testo: String(x) }), nota ? el("small", { class: "tenue", testo: nota }) : null); }

  // Tiri chiesti dagli effetti dei livelli 4 e 7 (il giocatore li fa per il proprio personaggio).
  function tiriEffetti() {
    if (!v.config.terrore || !(v.personaggio.mio || v.master)) return null;
    const b = [];
    if (v.terrore >= 4 && v.terrore <= 5) b.push(el("button", { testo: "TS di Saggezza CD " + v.config.cdBarcollante + " (inizio del turno)", on: { click: () => tiro({ tipo: "barcollante" }) } }));
    if (v.terrore === 7) b.push(el("button", { testo: "TS di Costituzione CD " + v.config.cdSvenuto + " (fine del turno)", on: { click: () => tiro({ tipo: "svenuto" }) } }));
    return b.length ? el("div", { class: "riga-flex" }, b) : null;
  }
  async function tiro(d) {
    try {
      const r = await api("/sanita/pg/" + encodeURIComponent(pg) + "/tiro", { dati: d });
      if (r.ts) avviso(CAR[r.ts.caratteristica] + ": d20 " + r.ts.scelto + " → " + r.ts.totale + " contro CD " + r.ts.cd + " · " + (r.ts.successo ? "SUPERATO ✔" : "FALLITO ✖"), "", 6000);
      if (r.pazzia) avviso("Pazzia temporanea: " + (r.pazzia.categoria || "") + (r.pazzia.descrizione ? " – " + r.pazzia.descrizione : ""), "", 7000);
      if (r.pazziaErrore) avviso(r.pazziaErrore, "errore", 6000);
      (r.automatiche || []).forEach((x) => avviso("⚙ " + x, "", 8000));
    } catch (e) { avviso(e.message, "errore"); }
  }

  // "Soccombi alla paura": una chiave per tentativo; il server garantisce un solo tiro per evento.
  async function soccombi(ev) {
    if (inCorso) return;
    inCorso = true; disegna();
    const chiave = (crypto.randomUUID && crypto.randomUUID()) || String(Date.now()) + Math.random();
    try { await tempoReale.chiedi("sanita.soccombi", { evento: ev.id, chiave }); }
    catch (e) { avviso(e.message, "errore", 6000); }
    inCorso = false;
    await ricaricaSilenziosa();
  }

  function risultato(e) {
    const r = typeof e.risoluzione === "string" ? JSON.parse(e.risoluzione) : e.risoluzione, t = r.ts;
    const ok = r.esito === "SUPERATO";
    return el("section", { class: "sanita-sez sanita-esito " + (ok ? "ok" : "ko"), "aria-live": "polite" },
      el("h3", { testo: "Ultimo tiro · " + r.fonte }),
      el("p", { class: "sanita-verdetto", testo: (ok ? "✔ SUPERATO" : "✖ FALLITO") }),
      el("p", {}, "d20: ", el("b", { testo: t.d20.join(" e ") + (t.d20.length > 1 ? " (" + t.vantaggio + ": tenuto " + t.scelto + ")" : "") }),
        " · " + CAR[t.caratteristica] + " " + segno(t.modCaratteristica) + (t.competenza ? " · competenza +" + t.competenza : "") + (t.bonus ? " · bonus " + segno(t.bonus) : "") + (t.situazionale ? " · modificatore " + segno(t.situazionale) : "")),
      el("p", {}, "Totale ", el("b", { testo: String(t.totale) }), " contro CD ", el("b", { testo: String(t.cd) })),
      r.terroreAttivo ? el("p", {}, "Terrore: " + r.terrorePrima + " → ", el("b", { testo: "+" + r.terroreAggiunto }), " → ", el("b", { testo: r.terroreDopo + " (" + r.livello.nome + ")" })) : null,
      r.effettoSuccesso ? el("p", { testo: "Effetto del successo: " + r.effettoSuccesso }) : null,
      r.condizioni && r.condizioni.length ? el("p", { testo: "Condizioni applicate: " + r.condizioni.join(", ") }) : null,
      r.resistenze && r.resistenze.length ? el("p", { class: "tenue", testo: "Applicato: " + r.resistenze.join("; ") }) : null,
      // regole automatiche (fallito di 5+, tre fallimenti di seguito, 1 sul d20, soglie 4 e 6 del Terrore)
      r.automatiche && r.automatiche.length ? el("ul", { class: "sanita-auto" }, r.automatiche.map((x) => el("li", { testo: "⚙ " + x }))) : null,
      r.pazzia && r.pazzia.length ? el("div", {}, r.pazzia.map((p) => el("p", { class: "sanita-pazzia" }, "⚠ " + p.motivo + " " + p.verifica,
        v.master ? bottonePazzia(p, e) : null))) : null,
      el("p", { class: "tenue", testo: "Risolto " + quando(r.quando) }));
  }

  function bottonePazzia(p, e) {
    if (p.tipo === "temporanea") return el("button", { class: "piccolo", testo: "Chiedi il TS di Carisma (CD " + e.cd + ")", on: { click: () => tiro({ tipo: "carisma_pazzia", evento: e.id }) } });
    return el("button", { class: "piccolo", testo: "Assegna pazzia " + p.tipo + "…", on: { click: () => formPazzia(p.tipo, e.id, p.motivo) } });
  }

  // ===== Strumenti del Master =====
  function strumentiMaster() {
    const sez = (titolo, ...f) => el("details", { class: "sanita-master" }, el("summary", { testo: titolo }), ...f);
    return el("div", { class: "sanita-strumenti" },
      sez("😱 Evento di paura", formEvento()),
      v.ultimo ? sez("🔁 Ripeti l'ultimo tiro", formRipeti()) : null,
      v.config.terrore ? sez("📈 Terrore", formTerrore()) : null,
      v.config.pazzia ? sez("🌀 Pazzie", formPazzie()) : null,
      sez("🧠 Sanità del personaggio", formPG()),
      sez("📜 Storico", storico()));
  }

  function formEvento() {
    const ev = v.evento, c = v.config;
    if (ev && (ev.stato === "READY" || ev.stato === "DRAFT" || ev.stato === "ERROR")) {
      const cd = el("input", { type: "number", min: "1", max: "40", valore: ev.cd });
      const va = selezione(VANT, ev.vantaggio), mod = el("input", { type: "number", min: "-20", max: "20", valore: ev.modificatore });
      const comp = selezione({ scheda: "come da scheda", si: "concessa", no: "negata" }, ev.competenza);
      const salva = (stato) => fai(api("/sanita/eventi/" + ev.id, { metodo: "PATCH", dati: { cd: +cd.value, vantaggio: va.value, modificatore: +mod.value, competenza: comp.value, ...(stato ? { stato } : {}) } }));
      return el("div", { class: "sanita-form" },
        el("p", {}, el("b", { testo: ev.fonte }), " · intensità " + ev.intensita + " · stato " + ev.stato),
        el("div", { class: "riga-flex" }, el("label", {}, "CD", cd), el("label", {}, "Vantaggio", va), el("label", {}, "Modificatore", mod), el("label", {}, "Competenza", comp)),
        el("div", { class: "riga-flex" },
          el("button", { testo: "Salva le modifiche", on: { click: () => salva() } }),
          ev.stato !== "READY" ? el("button", { class: "primario", testo: "Attiva l'evento", on: { click: () => salva("READY") } }) : null,
          el("button", { class: "pericolo", testo: "Annulla l'evento", on: { click: async () => { if (await conferma("Annullare l'evento?", "Il giocatore non potrà più tirare per questo evento.", "Annulla l'evento", true)) salva("CANCELLED"); } } })));
    }
    const fonte = el("input", { maxlength: "80", placeholder: "Es. L'antagonista alla finestra" }), desc = el("textarea", { rows: "2", maxlength: "1000", placeholder: "Cosa vede o sente il personaggio" });
    const inten = el("select", {}, c.cdIntensita.map((cd, i) => el("option", { value: i, testo: i + " · CD " + cd + " · +" + c.terroreIntensita[i] + " Terrore" + (["", " (minaccia minore)", " (soprannaturale)", " (orrore grave)", " (trauma catastrofico)", " (eccezionale)"][i]), selected: i === 1 })));
    const cd = el("input", { type: "number", min: "1", max: "40", valore: c.cdIntensita[1] }), terr = el("input", { type: "number", min: "0", max: "7", valore: c.terroreIntensita[1] });
    inten.addEventListener("change", () => { cd.value = c.cdIntensita[+inten.value]; terr.value = c.terroreIntensita[+inten.value]; });
    const car = selezione({ SAN: "Sanità mentale", SAG: "Saggezza", CAR: "Carisma" }, "SAN"), va = selezione(VANT, "normale");
    const mod = el("input", { type: "number", min: "-20", max: "20", valore: "0" }), comp = selezione({ scheda: "come da scheda", si: "concessa", no: "negata" }, "scheda");
    const succ = el("input", { maxlength: "300", placeholder: "Facoltativo: cosa succede se supera il tiro" });
    const crea = (bozza) => fai(api("/sanita/eventi", { dati: { personaggi: [pg], fonte: fonte.value, descrizione: desc.value, intensita: +inten.value, cd: +cd.value, terroreFallimento: +terr.value,
      caratteristica: car.value, vantaggio: va.value, modificatore: +mod.value, competenza: comp.value, effettoSuccesso: succ.value, bozza } }), bozza ? "Bozza salvata." : "Evento attivato: il giocatore può tirare.");
    return el("div", { class: "sanita-form" },
      el("label", {}, "Fonte del terrore", fonte), el("label", {}, "Descrizione", desc),
      el("div", { class: "riga-flex" }, el("label", {}, "Intensità", inten), el("label", {}, "CD", cd), el("label", {}, "Terrore se fallisce", terr)),
      el("div", { class: "riga-flex" }, el("label", {}, "Caratteristica", car), el("label", {}, "Vantaggio", va), el("label", {}, "Modificatore", mod), el("label", {}, "Competenza", comp)),
      el("label", {}, "Effetto in caso di successo", succ),
      el("p", { class: "tenue", testo: "Intensità e scala del Terrore sono homebrew. Un nuovo tiro nella stessa scena va chiesto solo per un peggioramento reale della minaccia." }),
      el("div", { class: "riga-flex" }, el("button", { testo: "Salva come bozza", on: { click: () => crea(true) } }), el("button", { class: "primario", testo: "Attiva l'evento", on: { click: () => crea(false) } })));
  }

  function formRipeti() {
    const mot = el("input", { maxlength: "300", placeholder: "Motivo (obbligatorio)" }), rip = el("input", { type: "checkbox" });
    return el("div", { class: "sanita-form" },
      el("p", { class: "tenue", testo: "Crea un nuovo evento con gli stessi parametri; il risultato precedente resta nello storico." }),
      el("label", {}, "Motivo", mot), el("label", { class: "riga" }, rip, el("span", { testo: "Riporta il Terrore al valore di prima del tiro precedente" })),
      el("button", { testo: "Ripeti il tiro", on: { click: () => fai(api("/sanita/eventi/" + v.ultimo.id + "/ripeti", { dati: { motivo: mot.value, ripristinaTerrore: rip.checked } }), "Nuovo tiro pronto.") } }));
  }

  function formTerrore() {
    const val = el("select", {}, v.scala.map((l) => el("option", { value: l.livello, testo: l.livello + " · " + l.nome, selected: l.livello === v.terrore })));
    const mot = el("input", { maxlength: "300", placeholder: "Motivo (obbligatorio)" });
    const rec = (tipo, testo, abilitato = true) => el("button", { class: "piccolo", testo, disabled: !abilitato, on: { click: () => fai(api("/sanita/pg/" + pg + "/terrore", { dati: { tipo } })) } });
    const cdSp = el("input", { type: "number", min: "1", max: "40", valore: "15", class: "corto" });
    return el("div", { class: "sanita-form" },
      el("div", { class: "riga-flex" }, el("label", {}, "Livello", val), el("label", { class: "spazio" }, "Motivo", mot),
        el("button", { testo: "Correggi", on: { click: () => fai(api("/sanita/pg/" + pg + "/terrore", { dati: { tipo: "manuale", valore: +val.value, motivo: mot.value } })) } })),
      el("p", { class: "tenue", testo: "Recupero (homebrew, −1 ciascuno):" }),
      el("div", { class: "riga-flex" }, rec("fonte_scomparsa", "Fonte scomparsa, al sicuro"), rec("riposo_lungo", "Riposo lungo sicuro"), rec("dieci_giorni", "10 giorni senza esposizione"),
        rec("aiuto_alleato", "Aiuto di un alleato (Saggezza CD 15 riuscita)"), rec("risveglio", "Risvegliato (Medicina CD 15) → 6", v.terrore === 7),
        v.config.regolaRimozione ? rec("rimozione_spaventato", "Rimosso Spaventato (−3, opzionale)") : null),
      v.config.regolaSpaventato ? el("div", { class: "riga-flex" }, el("span", { testo: "Spaventato da un effetto soprannaturale (opzionale): TS Sanità mentale CD" }), cdSp,
        el("button", { class: "piccolo", testo: "Tira", on: { click: () => tiro({ tipo: "spaventato_soprannaturale", cd: +cdSp.value }) } })) : null);
  }

  function formPazzia(tipo, evento, motivo) {
    const cat = el("select", {}, el("option", { value: "", testo: "Casuale (d20 categoria, d100 voce)" }), v.categorie.map((c) => el("option", { value: c, testo: c })));
    const desc = el("textarea", { rows: "2", maxlength: "1000", placeholder: "Descrizione (facoltativa se casuale)" });
    const cd = el("input", { type: "number", min: "1", max: "40", placeholder: "come l'evento o 15" }), ov = el("input", { type: "checkbox" });
    dialogo("Pazzia " + tipo, el("div", {},
      motivo ? el("p", { testo: "Motivo: " + motivo }) : null,
      tipo !== "temporanea" ? el("p", { class: "tenue", testo: "Pazzia " + tipo + ": serve la tua conferma esplicita." }) : null,
      el("label", {}, "Categoria", cat), el("label", {}, "Descrizione", desc), el("label", {}, "CD di recupero", cd),
      evento ? el("label", { class: "riga" }, ov, el("span", { testo: "Eccezione del Master: anche se l'evento ha già generato una pazzia" })) : null), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Confermo: assegna", on: { click: async () => {
          await fai(api("/sanita/pg/" + pg + "/pazzie", { dati: { tipo, categoria: cat.value, casuale: !cat.value, descrizione: desc.value, evento: evento || "", cdRecupero: +cd.value || 0, conferma: true, override: ov.checked } }), "Pazzia assegnata.");
          chiudi();
        } } })),
    });
  }

  function formPazzie() {
    const tipi = ["temporanea", "duratura", "indeterminata", ...(v.config.pazziaCronica ? ["cronica"] : [])];
    return el("div", { class: "sanita-form" },
      el("div", { class: "riga-flex" }, tipi.map((t) => el("button", { class: "piccolo", testo: "＋ " + t, on: { click: () => formPazzia(t, "", "") } }))),
      v.pazzie.length ? v.pazzie.map((p) => {
        const d = el("textarea", { rows: "2", maxlength: "1000" }); d.value = p.descrizione;
        return el("div", { class: "sanita-pazzia-riga" + (p.attiva ? "" : " chiusa") },
          el("b", { testo: p.tipo + (p.categoria ? " · " + p.categoria : "") + (p.attiva ? "" : " · chiusa: " + p.motivo) }),
          el("span", { class: "tenue", testo: " durata " + p.durata + " · CD recupero " + p.cdRecupero + " · " + quando(p.creata) }),
          p.attiva ? [d, el("div", { class: "riga-flex" },
            el("button", { class: "piccolo", testo: "Salva testo", on: { click: () => fai(api("/sanita/pazzie/" + p.id, { metodo: "PATCH", dati: { descrizione: d.value } })) } }),
            el("button", { class: "piccolo", testo: "Prova di recupero (10 giorni, homebrew)", on: { click: async () => {
              try { const r = await api("/sanita/pazzie/" + p.id + "/recupero", { dati: {} }); avviso("Saggezza: " + r.ts.totale + " contro CD " + r.cdPrima + (r.ts.successo ? " · CD −" + r.riduzione + " → " + r.cdDopo + (r.guarita ? " · guarita!" : "") : " · nessun progresso"), "", 7000); }
              catch (e) { avviso(e.message, "errore"); }
            } } }),
            el("button", { class: "piccolo pericolo", testo: "Rimuovi (cura)", on: { click: async () => {
              const m = prompt("Motivo della rimozione (per esempio l'incantesimo usato):", ""); if (m === null) return;
              fai(api("/sanita/pazzie/" + p.id, { metodo: "PATCH", dati: { rimuovi: true, motivo: m } }));
            } } }))] : null);
      }) : el("p", { class: "tenue", testo: "Nessuna pazzia." }));
  }

  function formPG() {
    const comp = el("input", { type: "checkbox", selezionato: v.competente }), bon = el("input", { type: "number", min: "-10", max: "10", valore: v.bonusTemp });
    const man = el("input", { type: "number", min: "1", max: "30", valore: v.punteggioManuale ?? "", placeholder: "dalla scheda" });
    const it = el("input", { type: "checkbox", selezionato: v.immuneTerrore }), is = el("input", { type: "checkbox", selezionato: v.immuneSpaventato });
    return el("div", { class: "sanita-form" },
      el("label", { class: "riga" }, comp, el("span", { testo: "Competenza nel TS su Sanità mentale (nelle regole base nessuna classe la dà)" })),
      el("div", { class: "riga-flex" }, el("label", {}, "Bonus temporaneo al TS", bon), el("label", {}, "Punteggio inserito a mano", man)),
      el("label", { class: "riga" }, is, el("span", { testo: "Immune alla condizione Spaventato" + (v.config.regolaResistenza ? " (regola opzionale attiva: vantaggio e 1 Terrore in meno)" : " (la regola opzionale è spenta: nessun effetto sul Terrore)") })),
      el("label", { class: "riga" }, it, el("span", { testo: "Immunità completa al Terrore (regola speciale)" })),
      el("div", { class: "riga-flex" },
        el("button", { class: "primario", testo: "Salva", on: { click: () => fai(api("/sanita/pg/" + pg, { metodo: "PUT", dati: { competente: comp.checked, bonusTemp: +bon.value || 0,
          ...(man.value ? { punteggioManuale: +man.value } : { togliManuale: true }), immuneTerrore: it.checked, immuneSpaventato: is.checked } }), "Salvato.") } })));
  }

  function storico() {
    const nomi = { evento_creato: "Evento creato", evento_modificato: "Evento modificato", evento_ripetuto: "Tiro ripetuto", evento_errore: "Errore di risoluzione", tiro_paura: "Tiro di paura",
      terrore_evento: "Terrore per un evento", terrore_manuale: "Terrore corretto", schermata_aperta: "Schermata aperta dal giocatore", impostazioni_pg: "Impostazioni del personaggio",
      pazzia_assegnata: "Pazzia assegnata", pazzia_modificata: "Pazzia modificata", pazzia_recupero: "Prova di recupero", pazzia_scaduta: "Pazzia scaduta", pazzia_cd_aumentata: "CD di recupero +1",
      tiro_carisma_pazzia: "TS di Carisma (pazzia)", terrore_barcollante: "Barcollante: TS superato", tiro_barcollante: "Barcollante: TS fallito", terrore_svenuto: "Svenuto: si riprende", tiro_svenuto: "Svenuto: TS fallito" };
    const riga = (r) => {
      const d = r.dati || {};
      let x = d.prima != null && d.dopo != null ? d.prima + " → " + d.dopo : "";
      if (d.esito) x = d.esito + " (" + d.ts.totale + " contro CD " + d.ts.cd + "), Terrore " + d.terrorePrima + " → " + d.terroreDopo;
      if (d.motivo) x += (x ? " · " : "") + d.motivo;
      if (d.fonte && !d.esito) x += (x ? " · " : "") + d.fonte;
      return el("li", {}, el("span", { class: "tenue", testo: quando(r.quando) + " · " + (r.chi || "sistema") + " · " }), el("b", { testo: nomi[r.azione] || (r.azione.startsWith("terrore_") ? "Terrore: " + r.azione.slice(8) : r.azione) }), x ? " – " + x : "");
    };
    return v.storico.length ? el("ul", { class: "sanita-storico" }, v.storico.map(riga)) : el("p", { class: "tenue", testo: "Nessuna voce." });
  }

  const selezione = (opz, sel) => el("select", {}, Object.entries(opz).map(([k, t]) => el("option", { value: k, testo: t, selected: k === sel })));
  async function fai(p, ok) { try { await p; if (ok) avviso(ok); await ricaricaSilenziosa(); } catch (e) { avviso(e.message, "errore", 6000); } }

  aperta = { pg, ricarica, chiudi };
  ricarica();
  return aperta;
}

/** Avvisi globali: il Master vede la schermata quando un giocatore la apre; il giocatore è avvisato degli eventi. */
let installata = false;
export function installaSanita(io) {
  if (installata) return;
  installata = true;
  tempoReale.su("sanita.aperta", (d) => { const u = io(); if (u && u.ruolo === "master" && d && d.personaggio) { avviso(d.da + " ha aperto la Sanità mentale di " + d.nome + ".", "", 4000); apriSanita(d.personaggio, u); } });
  tempoReale.su("sanita.evento", (d) => {
    const u = io(); if (!u || u.ruolo === "master" || !d) return;
    if (aperta && aperta.pg === d.personaggio) { aperta.ricarica(); return; }
    dialogo("Evento di paura", el("p", { testo: "Il Master ha preparato un evento di paura: «" + d.fonte + "». Apri la schermata Sanità mentale per il tiro." }), {
      azioni: (chiudi) => el("div", { class: "azioni" }, el("button", { testo: "Più tardi", on: { click: chiudi } }),
        el("button", { class: "primario", testo: "Apri Sanità mentale", on: { click: () => { chiudi(); apriSanita(d.personaggio, u); } } })),
    });
  });
}
