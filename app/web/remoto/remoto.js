// Tracker dell'iniziativa per il gioco da remoto (pagina autonoma, nessuna libreria).
// Riceve dal server MQTT lo stato cifrato con la chiave personale del giocatore e invia le azioni
// sul proprio PG, cifrate e autenticate con la stessa chiave. La chiave arriva nella parte del link
// dopo "#", che il browser non invia mai a nessun server; poi resta solo su questo telefono.
// Il server di Gaster Bless ricontrolla ogni azione con gli stessi permessi dell'app.
// Server: internal/server/ponte_mqtt.go. Funziona anche salvata come file (window.GASTERBLESS_REMOTO).
(function () {
  "use strict";
  const CONDIZIONI = ["Accecato", "Affascinato", "Assordato", "Spaventato", "Afferrato", "Incapacitato", "Invisibile",
    "Paralizzato", "Pietrificato", "Avvelenato", "Prono", "Trattenuto", "Stordito", "Privo di sensi", "Concentrato",
    "Indebolimento 1", "Indebolimento 2", "Indebolimento 3", "Indebolimento 4", "Indebolimento 5", "Indebolimento 6"];
  const MEMORIA = "gasterbless-remoto";
  const app = document.getElementById("app");
  const testo = new TextEncoder(), lettore = new TextDecoder();

  // ===== Piccoli aiuti per il DOM (solo textContent: niente HTML costruito da stringhe) =====
  function el(tag, attr, ...figli) {
    const e = document.createElement(tag);
    for (const [k, v] of Object.entries(attr || {})) {
      if (v === null || v === undefined || v === false) continue;
      if (k === "testo") e.textContent = v;
      else if (k === "classe") e.className = v;
      else if (k.startsWith("on")) e.addEventListener(k.slice(2), v);
      else e.setAttribute(k, v === true ? "" : v);
    }
    for (const f of figli.flat()) if (f !== null && f !== undefined && f !== false) e.append(f);
    return e;
  }
  function mostra(...figli) { app.replaceChildren(...figli.flat().filter(Boolean)); }

  // ===== Configurazione: dal link (#...), dal file salvato o da questo telefono =====
  function b64url(s) {
    s = s.replace(/-/g, "+").replace(/_/g, "/");
    while (s.length % 4) s += "=";
    return Uint8Array.from(atob(s), (c) => c.charCodeAt(0));
  }
  function leggiMemoria() { try { return JSON.parse(localStorage.getItem(MEMORIA) || "null"); } catch (e) { return null; } }
  function scriviMemoria(v) { try { v ? localStorage.setItem(MEMORIA, JSON.stringify(v)) : localStorage.removeItem(MEMORIA); } catch (e) { /* memoria non disponibile */ } }
  function configurazione() {
    if (location.hash.length > 1) {
      try {
        const c = JSON.parse(lettore.decode(b64url(location.hash.slice(1))));
        scriviMemoria(c);
        history.replaceState(null, "", location.pathname); // la chiave non resta nella barra degli indirizzi
        return c;
      } catch (e) { /* link rovinato: si prova con ciò che c'è già */ }
    }
    return window.GASTERBLESS_REMOTO || leggiMemoria();
  }
  const conf = configurazione();
  const valida = conf && Array.isArray(conf.b) && /^[0-9a-f]{32}$/.test(conf.t || "") && /^[0-9a-f]{16}$/.test(conf.u || "") && typeof conf.k === "string";
  if (!valida) {
    mostra(el("h1", { testo: "GASTER BLESS" }), el("div", { classe: "scheda" },
      el("h2", { testo: "Serve il tuo link personale" }),
      el("p", { testo: "Apri il link (o inquadra il QR) che ti ha dato il Master per il gioco da remoto. Basta farlo una volta: poi questa pagina si ricorda di te." })));
    return;
  }
  if (!window.crypto || !crypto.subtle) {
    mostra(el("h1", { testo: "GASTER BLESS" }), el("p", { classe: "errore", testo: "Questo browser non permette la cifratura necessaria. Apri la pagina con Chrome, Safari, Firefox o Edge aggiornati." }));
    return;
  }
  if ("serviceWorker" in navigator && location.protocol === "https:") navigator.serviceWorker.register("/sw.js").catch(() => {});

  const base = "gasterbless/" + conf.t + "/";
  const argStato = base + "s/" + conf.u, argEsito = base + "e/" + conf.u, argAzioni = base + "a";
  const chiave = crypto.subtle.importKey("raw", b64url(conf.k), "AES-GCM", false, ["encrypt", "decrypt"]);

  async function apri(dati, contesto) {
    const k = await chiave;
    const chiaro = await crypto.subtle.decrypt({ name: "AES-GCM", iv: dati.slice(0, 12), additionalData: testo.encode(contesto + "|" + conf.t + "|" + conf.u) }, k, dati.slice(12));
    return JSON.parse(lettore.decode(chiaro));
  }
  async function sigilla(obj, contesto) {
    const k = await chiave;
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const c = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv, additionalData: testo.encode(contesto + "|" + conf.t + "|" + conf.u) }, k, testo.encode(JSON.stringify(obj))));
    const tutto = new Uint8Array(12 + c.length);
    tutto.set(iv); tutto.set(c, 12);
    let s = "";
    for (const b of tutto) s += String.fromCharCode(b);
    return btoa(s);
  }

  // ===== Client MQTT 3.1.1 minimo sopra WebSocket (QoS 0) =====
  function stringaMQTT(s) { const b = testo.encode(s); return [b.length >> 8, b.length & 255, ...b]; }
  function lunghezza(n) { const o = []; do { let d = n % 128; n = Math.floor(n / 128); if (n > 0) d |= 128; o.push(d); } while (n > 0); return o; }
  function pacchetto(h, corpo) { return new Uint8Array([h, ...lunghezza(corpo.length), ...corpo]); }

  const rete = { ws: null, ok: false, server: "", indice: 0, attesa: 2000, ping: 0, id: 1, timer: 0 };
  function collega() {
    const url = conf.b[rete.indice % conf.b.length];
    let ws;
    try { ws = new WebSocket(url, "mqtt"); } catch (e) { return riprova(); }
    rete.ws = ws;
    ws.binaryType = "arraybuffer";
    let buf = new Uint8Array(0), connesso = false;
    const scadenza = setTimeout(() => { if (!connesso) ws.close(); }, 10000);
    ws.onopen = () => {
      const id = "gasterbless-r-" + Math.random().toString(16).slice(2, 10);
      ws.send(pacchetto(0x10, [...stringaMQTT("MQTT"), 4, 0x02, 0, 60, ...stringaMQTT(id)]));
    };
    ws.onmessage = (ev) => {
      const nuovo = new Uint8Array(ev.data), t = new Uint8Array(buf.length + nuovo.length);
      t.set(buf); t.set(nuovo, buf.length); buf = t;
      for (;;) {
        if (buf.length < 2) break;
        let n = 0, molt = 1, i = 1, fine = false;
        while (i < buf.length && i <= 4) { const d = buf[i++]; n += (d & 127) * molt; molt *= 128; if (!(d & 128)) { fine = true; break; } }
        if (!fine || buf.length < i + n) break;
        const tipo = buf[0] >> 4, flag = buf[0] & 15, corpo = buf.slice(i, i + n);
        buf = buf.slice(i + n);
        if (tipo === 2) { // CONNACK
          if (corpo[1] !== 0) { ws.close(); return; }
          connesso = true; clearTimeout(scadenza);
          rete.ok = true; rete.attesa = 2000; rete.server = new URL(url).hostname;
          for (const a of [argStato, argEsito]) ws.send(pacchetto(0x82, [0, (rete.id = rete.id % 65535 + 1), ...stringaMQTT(a), 0]));
          rete.ping = setInterval(() => { try { ws.send(new Uint8Array([0xc0, 0])); } catch (e) { /* si richiude da sola */ } }, 30000);
          disegna();
        } else if (tipo === 3) { // PUBLISH
          const lt = (corpo[0] << 8) | corpo[1];
          const arg = lettore.decode(corpo.slice(2, 2 + lt));
          let dati = corpo.slice(2 + lt);
          if ((flag >> 1) & 3) dati = dati.slice(2);
          ricevi(arg, dati);
        }
      }
    };
    ws.onclose = () => {
      clearTimeout(scadenza); clearInterval(rete.ping);
      if (rete.ws !== ws) return;
      rete.ok = false;
      if (!connesso) rete.indice++; // il server non risponde: si prova il prossimo
      disegna();
      riprova();
    };
    ws.onerror = () => { try { ws.close(); } catch (e) { /* già chiusa */ } };
  }
  function riprova() {
    rete.timer = setTimeout(() => { rete.timer = 0; collega(); }, rete.attesa);
    rete.attesa = Math.min(rete.attesa * 2, 30000);
  }
  function pubblica(arg, testoMsg) {
    if (!rete.ok) return false;
    rete.ws.send(pacchetto(0x30, [...stringaMQTT(arg), ...testo.encode(testoMsg)]));
    return true;
  }
  // Tornando sulla pagina (telefono sbloccato) ci si ricollega subito se la connessione è caduta.
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible" && rete.timer) { clearTimeout(rete.timer); rete.timer = 0; rete.attesa = 2000; collega(); }
  });

  // ===== Stato e azioni =====
  let stato = null, chiuso = false, avviso = null, eraMioTurno = false;
  const inAttesa = new Map(); // azione → timer
  // Attacchi (2026-11-12): scelte che restano tra un aggiornamento e l'altro, ultimo esito, azioni mandate da qui
  const scelta = { attacco: null, bersaglio: null, modo: "normale" };
  let ultimoEsito = null;
  const mieAzioni = new Set();
  const nuovaAzione = () => { const id = "r" + Date.now().toString(36) + Math.random().toString(36).slice(2, 10); mieAzioni.add(id); return id; };
  const sg = (n) => (n >= 0 ? "+" : "−") + Math.abs(n);
  // Incantesimi automatici (fase 9b): scelte che restano tra gli aggiornamenti e ultimo lancio (per i raggi successivi)
  const sceltaInc = { nome: null, slot: null, bersaglio: null, modo: "normale", raggi: 1, bersagli: new Set() };
  let ultimoLancio = null;
  const NOMI_CAR = { FOR: "Forza", DES: "Destrezza", COS: "Costituzione", INT: "Intelligenza", SAG: "Saggezza", CAR: "Carisma" };

  async function ricevi(arg, dati) {
    if (arg === argStato) {
      if (!dati.length) { stato = null; chiuso = true; disegna(); return; }
      try { stato = await apri(dati, "stato"); chiuso = false; disegna(); } catch (e) { /* non è per noi */ }
    } else if (arg === argEsito && dati.length) {
      try {
        const e = await apri(dati, "esito");
        if (e.evento) { evento(e.evento, e.dati); return; }
        if (!inAttesa.has(e.n)) return;
        clearTimeout(inAttesa.get(e.n)); inAttesa.delete(e.n);
        avviso = e.ok ? null : { testo: e.errore || "Operazione non riuscita.", errore: true };
        if (e.ok && e.risultato && e.risultato.iniziativa !== undefined) avviso = { testo: "Iniziativa: " + e.risultato.iniziativa };
        if (e.ok && e.risultato && e.risultato.d20) ultimoEsito = e.risultato; // esito del proprio attacco
        if (e.ok && e.risultato && e.risultato.testo) avviso = { testo: e.risultato.testo };
        if (e.ok && e.risultato && e.risultato.inAttesa) avviso = { testo: "✨ Tiri salvezza fatti: il Master li controlla e applica l'effetto." };
        if (e.ok && e.risultato && e.risultato.righe && e.risultato.applicato) avviso = { testo: "✨ " + e.risultato.incantesimo + ": " + e.risultato.righe.map((r) => r.nome + (r.cura ? " +" + r.cura + " PF" : "")).join("; ") };
        disegna();
      } catch (err) { /* non è per noi */ }
    }
  }

  // Esiti degli scontri inoltrati dal PC (attacchi e incantesimi degli altri, colpi senza nome dei mostri nascosti).
  function evento(tipo, e) {
    if (!e || (e.azione && mieAzioni.has(e.azione))) return;
    let t = "";
    if (tipo === "attacco.esito") {
      const tipi = [...new Set((e.danni || []).map((d) => d.tipo).filter(Boolean))].join(" e ");
      const danni = e.danni && e.danni.length ? ": " + e.totaleDanni + " danni" + (tipi ? " di tipo " + tipi : "") : "";
      if (e.anonimo) t = e.risultato === "mancato" ? "👁 Qualcosa che non vedi attacca " + e.nomeBersaglio + " e lo manca." : "👁 Qualcosa che non vedi colpisce " + e.nomeBersaglio + danni + "!";
      else if (e.risultato === "mancato") t = "⚔ " + e.nomeAttaccante + " manca " + e.nomeBersaglio + (e.attacco ? " (" + e.attacco + ")" : "");
      else if (e.risultato === "critico" && !danni) t = "💥 COLPO CRITICO di " + e.nomeAttaccante + " su " + e.nomeBersaglio + "!";
      else t = "⚔ " + e.nomeAttaccante + " colpisce " + e.nomeBersaglio + danni + (e.modoCritico ? " (critico)" : "");
      if (e.sconfitto) t += " · " + e.nomeBersaglio + " è a terra!";
      if (e.concentrazione) t += " · TS di Costituzione CD " + e.concentrazione + " per la concentrazione";
    } else if (tipo === "incantesimo.esito" && e.righe && e.righe.length) {
      const r = e.righe.map((x) => x.nome + (e.tipo === "ts" ? (x.riuscito ? " riesce" : " fallisce") : "") + (x.totaleDanni ? ", " + x.totaleDanni + " danni" : "") + (x.cura ? ", +" + x.cura + " PF" : "") + (x.condizioni && x.condizioni.length ? ", " + x.condizioni.join(", ").toLowerCase() : "")).join("; ");
      t = e.anonimo ? "👁 Qualcosa che non vedi lancia un incantesimo: " + r : "✨ " + e.incantesimo + " (" + e.nomeIncantatore + "): " + r;
    }
    if (!t) return;
    avviso = { testo: t, errore: !!e.anonimo };
    if (navigator.vibrate && e.nomeBersaglio && (stato && (stato.vista.combattenti || []).some((c) => c.mio && c.nome === e.nomeBersaglio))) navigator.vibrate(120);
    disegna();
  }

  async function azione(tipo, dati) {
    const n = Math.random().toString(36).slice(2) + Date.now().toString(36);
    const c = await sigilla({ tipo, dati, ts: Date.now(), n }, "azione");
    if (!pubblica(argAzioni, JSON.stringify({ u: conf.u, c }))) {
      avviso = { testo: "Non sei collegato: riprova tra poco.", errore: true }; disegna(); return;
    }
    inAttesa.set(n, setTimeout(() => {
      inAttesa.delete(n);
      avviso = { testo: "Il PC del Master non risponde: forse Gaster Bless è spento o senza Internet.", errore: true };
      disegna();
    }, 12000));
    avviso = { testo: "Inviato…" };
    disegna();
  }

  // ===== Disegno =====
  function ora(ms) { return new Date(ms).toLocaleTimeString("it-IT", { hour: "2-digit", minute: "2-digit" }); }

  function riga(c, turnoDi) {
    const pf = c.pf !== undefined && c.pf !== null
      ? el("span", { classe: "pf" + (c.pfMax && c.pf * 4 <= c.pfMax ? " basso" : ""), testo: c.pf + (c.pfMax ? "/" + c.pfMax : "") + (c.pfTemp ? " +" + c.pfTemp : "") })
      : el("span", { classe: "tenue piccolo", testo: c.salute || "" });
    return el("li", { classe: (c.id === turnoDi ? "turno " : "") + (c.mio ? "mio" : "") },
      el("span", { classe: "ini", testo: c.iniziativa === null || c.iniziativa === undefined ? "–" : String(c.iniziativa) }),
      el("div", {},
        el("div", { classe: "nome", testo: c.nome + (c.mio ? " (tu)" : "") }),
        c.giocatore && !c.mio ? el("div", { classe: "dettagli", testo: c.giocatore }) : null,
        (c.condizioni || []).length ? el("div", {}, c.condizioni.map((x) => el("span", { classe: "cond", testo: x }))) : null),
      pf);
  }

  // Attacchi automatici (come «⚔ Attacca» nell'app): il d20 e i danni li tira il PC del Master.
  function attacchi(c, lista, comb) {
    if (!comb.avviato || !c.attacchi || !c.attacchi.length) return null;
    const altri = lista.filter((x) => x.id !== c.id);
    const inPiedi = (x) => x.pf !== undefined && x.pf !== null ? x.pf > 0 : x.salute !== "a terra";
    altri.sort((x, y) => (y.tipo !== c.tipo) - (x.tipo !== c.tipo) || inPiedi(y) - inPiedi(x));
    if (!altri.length) return null;
    if (scelta.attacco === null || scelta.attacco >= c.attacchi.length) scelta.attacco = 0;
    if (!altri.some((x) => x.id === scelta.bersaglio)) scelta.bersaglio = altri[0].id;
    const testoDanni = (a) => (a.danni || []).map((d) => (d.dadi || "") + (d.mod ? sg(d.mod) : "") + (d.tipo ? " " + d.tipo : "")).join(" + ");
    const selA = el("select", { "aria-label": "Attacco", onchange: (ev) => { scelta.attacco = +ev.target.value; } },
      c.attacchi.map((a, i) => el("option", { value: i, selected: i === scelta.attacco, testo: a.nome + " · " + sg(a.bonus) + " · " + testoDanni(a) })));
    const selB = el("select", { "aria-label": "Bersaglio", onchange: (ev) => { scelta.bersaglio = ev.target.value; } },
      altri.map((x) => el("option", { value: x.id, selected: x.id === scelta.bersaglio, testo: x.nome + (x.pf !== undefined && x.pf !== null ? " · " + x.pf + " PF" : x.salute ? " · " + x.salute : "") })));
    const selM = el("select", { "aria-label": "Tiro", onchange: (ev) => { scelta.modo = ev.target.value; } },
      ["normale", "vantaggio", "svantaggio"].map((m) => el("option", { value: m, selected: m === scelta.modo, testo: m[0].toUpperCase() + m.slice(1) })));
    const per = c.attacchiPerAzione || 1, usati = Math.min(c.attacchiUsati || 0, per);
    return el("details", { classe: "attacca", open: true }, el("summary", { testo: "⚔ Attacca (" + usati + " di " + per + " in questo turno)" }),
      el("div", { classe: "riga" }, selA), el("div", { classe: "riga" }, selB, selM),
      el("button", { classe: "primario", testo: "🎲 Tira l'attacco", onclick: () => {
        azione("attacco.tira", { combattente: c.id, bersaglio: scelta.bersaglio, attacco: scelta.attacco, modo: scelta.modo, azione: nuovaAzione() });
      } }),
      el("p", { classe: "tenue piccolo", testo: "Puoi attaccare nel tuo turno (o con la reazione se il Master lo permette). Resistenze, immunità e condizioni le calcola il PC." }),
      esitoAttacco());
  }

  function esitoAttacco() {
    const e = ultimoEsito;
    if (!e) return null;
    const tiro = (e.d20.length > 1 ? "d20: " + e.d20.join(" e ") + " (" + e.modo + ")" : "d20: " + e.naturale) + " " + sg(e.bonus) + " = " + e.totale;
    const titolo = { mancato: "MANCATO", colpito: "COLPITO", critico: "💥 COLPO CRITICO!" }[e.risultato];
    const righe = [el("p", { testo: tiro }), el("p", { classe: "esito " + e.risultato, testo: titolo })];
    if (e.danni && e.danni.length) {
      righe.push(el("ul", {}, e.danni.map((d) => el("li", { testo: d.espr + (d.dadi && d.dadi.length ? " [" + d.dadi.join(", ") + "]" : "") + " = " + d.tirato + (d.tipo ? " " + d.tipo : "") + (d.nota ? " → " + d.finale + " (" + d.nota + ")" : "") }))),
        el("p", { testo: "Danni: " + e.totaleDanni + (e.sconfitto ? " · " + e.nomeBersaglio + " è a terra!" : "") }));
    }
    if (e.risultato === "critico" && e.gettone) {
      const scegli = (modo) => { azione("attacco.critico", { gettone: e.gettone, modo, azione: nuovaAzione() }); };
      righe.push(el("p", { classe: "piccolo", testo: "Come si calcolano i danni del critico?" }),
        el("div", { classe: "riga" }, el("button", { classe: "primario", testo: "🎲🎲 Tira due volte i dadi", onclick: () => scegli("dadi") }),
          el("button", { testo: "✖2 Raddoppia il risultato", onclick: () => scegli("raddoppia") })));
    } else if (e.attacchiRimasti > 0 && ultimoLancio && e.attacco === "✨ " + ultimoLancio.inc.nome) {
      righe.push(el("button", { classe: "primario", testo: "✨ Raggio successivo (" + (e.attacchiRimasti === 1 ? "ne resta 1" : "ne restano " + e.attacchiRimasti) + ")",
        onclick: () => azione("attacco.tira", { combattente: ultimoLancio.combattente, bersaglio: sceltaInc.bersaglio || e.bersaglio, modo: sceltaInc.modo, incantesimo: ultimoLancio.inc, azione: nuovaAzione() }) }));
    } else if (e.attacchiRimasti > 0) righe.push(el("p", { classe: "tenue piccolo", testo: "Puoi attaccare ancora: " + (e.attacchiRimasti === 1 ? "ne resta 1." : "ne restano " + e.attacchiRimasti + ".") }));
    return el("div", { classe: "esito-attacco", role: "status" }, el("b", { testo: "⚔ " + e.nomeAttaccante + " → " + e.nomeBersaglio + " · " + e.attacco }), righe);
  }

  // Incantesimi automatici (fase 9b): quelli che la scheda sa risolvere da sola, già calcolati per ogni slot. Lo slot lo
  // toglie il PC (solo se è libero); attacchi con incantesimo come le armi, tiri salvezza confermati dal Master.
  function incantesimi(c, lista, comb) {
    const elenco = c.incantesimiAuto || [];
    if (!comb.avviato || !elenco.length) return null;
    if (!elenco.some((x) => x.nome === sceltaInc.nome)) sceltaInc.nome = elenco[0].nome;
    const inc = elenco.find((x) => x.nome === sceltaInc.nome);
    const slotDi = (lv) => (c.slot || []).find((x) => x.livello === lv);
    const libero = (lv) => lv === 0 || (slotDi(lv) && slotDi(lv).usati < slotDi(lv).tot);
    if (!inc.livelli.some((p) => p.slot === sceltaInc.slot && libero(p.slot))) sceltaInc.slot = (inc.livelli.find((p) => libero(p.slot)) || inc.livelli[0]).slot;
    const prof = inc.livelli.find((p) => p.slot === sceltaInc.slot) || inc.livelli[0];
    const selI = el("select", { "aria-label": "Incantesimo", onchange: (ev) => { sceltaInc.nome = ev.target.value; sceltaInc.slot = null; sceltaInc.bersagli.clear(); disegna(); } },
      elenco.map((x) => el("option", { value: x.nome, selected: x.nome === inc.nome, testo: x.nome + (x.livello ? " (" + x.livello + "°)" : " (trucchetto)") })));
    const selS = inc.livello === 0 ? null : el("select", { "aria-label": "Slot", onchange: (ev) => { sceltaInc.slot = +ev.target.value; disegna(); } },
      inc.livelli.map((p) => { const x = slotDi(p.slot); return el("option", { value: p.slot, selected: p.slot === prof.slot, disabled: !libero(p.slot),
        testo: "Slot di " + p.slot + "° livello" + (x ? " · " + (x.tot - x.usati) + "/" + x.tot + " liberi" : "") }); }));
    const tipoTesto = prof.tipo === "attacco" ? "Attacco con incantesimo " + sg(prof.bonus) : prof.tipo === "ts" ? "TS su " + (NOMI_CAR[prof.car] || prof.car) + " CD " + prof.cd + (prof.seRiesce === "meta" ? " (metà se riesce)" : "")
      : prof.tipo === "cura" ? "Cura" : "PF temporanei";
    const tempo = { azione: "azione", bonus: "azione bonus", reazione: "reazione" }[prof.tempo] || "nessuna azione del turno";
    const righe = [el("p", { classe: "piccolo", testo: tipoTesto + (prof.riassunto ? " · " + prof.riassunto : "") + " · usa: " + tempo + (prof.conc ? " · concentrazione" : "") })];
    const alleati = prof.tipo === "cura" || prof.tipo === "pftemp";
    const altri = lista.slice().sort((x, y) => alleati ? (y.tipo === c.tipo) - (x.tipo === c.tipo) : (y.tipo !== c.tipo) - (x.tipo !== c.tipo));
    const etichetta = (x) => x.nome + (x.id === c.id ? " (tu)" : "") + (x.pf !== undefined && x.pf !== null ? " · " + x.pf + " PF" : x.salute ? " · " + x.salute : "");
    const id = nuovaAzione;
    if (prof.tipo === "attacco") {
      const bers = altri.filter((x) => x.id !== c.id);
      if (!bers.some((x) => x.id === sceltaInc.bersaglio)) sceltaInc.bersaglio = bers.length ? bers[0].id : null;
      righe.push(el("div", { classe: "riga" },
        el("select", { "aria-label": "Bersaglio dell'incantesimo", onchange: (ev) => { sceltaInc.bersaglio = ev.target.value; } }, bers.map((x) => el("option", { value: x.id, selected: x.id === sceltaInc.bersaglio, testo: etichetta(x) }))),
        el("select", { "aria-label": "Tiro dell'incantesimo", onchange: (ev) => { sceltaInc.modo = ev.target.value; } }, ["normale", "vantaggio", "svantaggio"].map((m) => el("option", { value: m, selected: m === sceltaInc.modo, testo: m[0].toUpperCase() + m.slice(1) }))),
        el("label", {}, "Raggi ", el("input", { type: "number", min: "1", max: "10", value: String(sceltaInc.raggi), "aria-label": "Raggi", onchange: (ev) => { sceltaInc.raggi = Math.max(1, Math.min(10, parseInt(ev.target.value, 10) || 1)); } }))));
    } else {
      const max = prof.bersagli || 0;
      for (const b of [...sceltaInc.bersagli]) if (!altri.some((x) => x.id === b)) sceltaInc.bersagli.delete(b);
      righe.push(el("div", { classe: "griglia-cond", role: "group", "aria-label": "Bersagli" }, altri.map((x) => el("label", {},
        el("input", { type: "checkbox", checked: sceltaInc.bersagli.has(x.id), disabled: !!(max && !sceltaInc.bersagli.has(x.id) && sceltaInc.bersagli.size >= max),
          onchange: (ev) => { ev.target.checked ? sceltaInc.bersagli.add(x.id) : sceltaInc.bersagli.delete(x.id); disegna(); } }), el("span", { testo: etichetta(x) })))),
        el("p", { classe: "tenue piccolo", testo: sceltaInc.bersagli.size + (max ? " di " + max : "") + " bersagli scelti" + (prof.tipo === "ts" ? ". I tiri salvezza li tira il PC; il Master li controlla e applica." : ".") }));
    }
    const lancia = () => {
      if (!libero(prof.slot)) { avviso = { testo: "Nessuno slot libero di quel livello.", errore: true }; disegna(); return; }
      const profilo = Object.assign({}, prof, { nome: inc.nome, livello: inc.livello });
      delete profilo.bersagli; delete profilo.riassunto;
      if (prof.tipo === "attacco") {
        if (!sceltaInc.bersaglio) return;
        profilo.raggi = sceltaInc.raggi; profilo.lancio = id();
        ultimoLancio = { inc: profilo, combattente: c.id };
        azione("attacco.tira", { combattente: c.id, bersaglio: sceltaInc.bersaglio, modo: sceltaInc.modo, incantesimo: profilo, usaSlot: true, azione: id() });
      } else {
        if (!sceltaInc.bersagli.size) { avviso = { testo: "Scegli almeno un bersaglio.", errore: true }; disegna(); return; }
        azione(prof.tipo === "ts" ? "incantesimo.ts" : "incantesimo.cura", { combattente: c.id, incantesimo: profilo, bersagli: [...sceltaInc.bersagli], usaSlot: true, azione: id() });
        sceltaInc.bersagli.clear();
      }
    };
    return el("details", { classe: "attacca", open: true }, el("summary", { testo: "✨ Incantesimi" }),
      el("div", { classe: "riga" }, selI, selS), righe,
      el("button", { classe: "primario", testo: prof.tipo === "ts" ? "🎲 Tira i TS" : prof.tipo === "attacco" ? "🎲 Lancia e tira" : "✨ Lancia", onclick: lancia }),
      (c.slot || []).length ? el("p", { classe: "tenue piccolo", testo: "Slot liberi: " + c.slot.map((x) => x.livello + "° " + (x.tot - x.usati) + "/" + x.tot).join(" · ") + ". Lo slot si toglie solo se il lancio riesce." }) : null);
  }

  // Azioni del turno (azione, azione bonus, reazione) e ira del barbaro.
  function turno(c, v, comb) {
    if (!comb.avviato || !c.economia) return null;
    const eco = c.economia;
    const voci = [["a", "Azione"], ["b", "Azione bonus"], ["r", "Reazione"]].map(([k, nome]) => el("button", {
      classe: "eco" + (eco[k] ? " usata" : ""), "aria-pressed": eco[k] ? "true" : "false", testo: (eco[k] ? "✗ " : "✓ ") + nome,
      title: eco[k] ? "Già usata: tocca per liberarla" : "Disponibile: tocca per segnarla come usata",
      onclick: () => azione("iniz.economia", { combattente: c.id, economia: { a: !!eco.a, b: !!eco.b, r: !!eco.r, [k]: !eco[k] } }) }));
    const iraAttiva = (v.effetti || []).some((e) => e.nome === "Ira" && e.incantatore === c.id);
    return el("div", { classe: "riga" }, voci,
      c.ira ? el("button", { classe: "ira" + (iraAttiva ? " attiva" : ""), testo: iraAttiva ? "😡 Fine ira" : "😡 Ira", onclick: () => azione("pg.ira", { combattente: c.id, azione: nuovaAzione() }) }) : null);
  }

  function schedaMia(c) {
    const numero = el("input", { type: "number", min: "0", max: "999", inputmode: "numeric", "aria-label": "Quantità" });
    const val = () => { const v = parseInt(numero.value, 10); if (!(v >= 0 && v <= 999)) { avviso = { testo: "Scrivi un numero.", errore: true }; disegna(); return null; } return v; };
    const ini = el("input", { type: "number", min: "-10", max: "50", inputmode: "numeric", "aria-label": "Iniziativa" });
    const spunte = CONDIZIONI.map((n) => ({ n, i: el("input", { type: "checkbox", checked: (c.condizioni || []).includes(n) }) }));
    const morte = c.tsMorte || { s: 0, f: 0 };
    const ts = (s, f) => azione("pg.ts_morte", { personaggio: c.personaggio, s: Math.max(0, Math.min(3, s)), f: Math.max(0, Math.min(3, f)) });
    return el("section", { classe: "scheda" },
      el("h2", { testo: c.nome }),
      el("p", {}, "PF ", el("b", { testo: String(c.pf) }), c.pfMax ? " su " + c.pfMax : "", c.pfTemp ? " · temporanei " + c.pfTemp : "", c.ca ? " · CA " + c.ca : ""),
      stato && stato.vista.combattimento ? turno(c, stato.vista, stato.vista.combattimento) : null,
      stato && stato.vista.combattimento ? attacchi(c, stato.vista.combattenti || [], stato.vista.combattimento) : null,
      stato && stato.vista.combattimento ? incantesimi(c, stato.vista.combattenti || [], stato.vista.combattimento) : null,
      el("div", { classe: "riga" }, numero,
        el("button", { classe: "primario", testo: "Danno", onclick: () => { const v = val(); if (v !== null) azione("pg.danno", { personaggio: c.personaggio, valore: v }); } }),
        el("button", { testo: "Cura", onclick: () => { const v = val(); if (v !== null) azione("pg.cura", { personaggio: c.personaggio, valore: v }); } }),
        el("button", { testo: "PF temporanei", onclick: () => { const v = val(); if (v !== null) azione("pg.temp", { personaggio: c.personaggio, valore: v }); } })),
      el("div", { classe: "riga" }, ini,
        el("button", { testo: "Imposta iniziativa", onclick: () => { const v = parseInt(ini.value, 10); if (v >= -10 && v <= 50) azione("iniz.iniziativa", { personaggio: c.personaggio, valore: v }); } }),
        el("button", { testo: "🎲 Tira", onclick: () => azione("iniz.iniziativa", { personaggio: c.personaggio, tira: true }) })),
      c.pf === 0 ? el("div", { classe: "riga" },
        el("span", { testo: "Tiri contro la morte: ✔ " + morte.s + " · ✖ " + morte.f }),
        el("button", { testo: "+ ✔", onclick: () => ts(morte.s + 1, morte.f) }),
        el("button", { testo: "+ ✖", onclick: () => ts(morte.s, morte.f + 1) }),
        el("button", { testo: "Azzera", onclick: () => ts(0, 0) })) : null,
      el("details", {}, el("summary", { testo: "Condizioni" }),
        el("div", { classe: "griglia-cond" }, spunte.map(({ n, i }) => el("label", {}, i, el("span", { testo: n })))),
        el("button", { testo: "Salva le condizioni", onclick: () => azione("pg.condizioni", { personaggio: c.personaggio, condizioni: spunte.filter((x) => x.i.checked).map((x) => x.n) }) })));
  }

  function disegna() {
    // Non ridisegnare mentre si sta scrivendo un numero: si perderebbe quanto digitato.
    const a = document.activeElement;
    if (a && a.tagName === "INPUT" && a.type === "number" && a.value !== "" && app.contains(a)) { setTimeout(disegna, 1500); return; }
    if (a && a.tagName === "SELECT" && app.contains(a)) { setTimeout(disegna, 1500); return; } // un menu aperto
    const testata = el("div", { classe: "testata" },
      el("div", {}, el("h1", { testo: (stato && stato.titolo) || conf.s || "Gaster Bless" }), el("div", { classe: "tenue piccolo", testo: "Giochi come " + (conf.n || "giocatore") })),
      el("span", { classe: "stato-rete " + (rete.ok ? "ok" : "no"), testo: rete.ok ? "● collegato" : "○ collegamento…" }));
    const msg = avviso ? el("div", { classe: "avviso" + (avviso.errore ? " errore" : ""), role: "status", testo: avviso.testo }) : null;
    if (chiuso) {
      return mostra(testata, el("div", { classe: "scheda" },
        el("h2", { testo: "Gioco da remoto chiuso" }),
        el("p", { testo: "Il Master ha chiuso il gioco da remoto per questa sessione, oppure ti ha dato un link nuovo. Chiedigli il link aggiornato." }),
        el("button", { testo: "Dimentica questo link", onclick: () => { scriviMemoria(null); location.reload(); } })));
    }
    if (!stato) {
      return mostra(testata, msg, el("div", { classe: "scheda" },
        el("p", { testo: rete.ok ? "In attesa del tavolo del Master…" : "Mi collego al server…" }),
        el("p", { classe: "tenue piccolo", testo: "Se resta così a lungo, il PC del Master potrebbe essere spento o senza Internet." })));
    }
    const v = stato.vista || {}, comb = v.combattimento;
    if (!comb) {
      return mostra(testata, msg, el("div", { classe: "scheda" },
        el("h2", { testo: "Nessun combattimento in corso" }),
        el("p", { classe: "tenue", testo: "Lascia aperta questa pagina: quando il Master inizia un combattimento comparirà qui da sola." })),
        el("p", { classe: "tenue piccolo", testo: "Aggiornato alle " + ora(stato.ts) }));
    }
    const lista = v.combattenti || [];
    const diTurno = lista.find((c) => c.id === comb.turnoDi);
    const miei = lista.filter((c) => c.mio && c.tipo === "pg");
    const mioTurno = !!(diTurno && diTurno.mio);
    if (mioTurno && !eraMioTurno && navigator.vibrate && (!navigator.userActivation || navigator.userActivation.hasBeenActive)) navigator.vibrate([200, 100, 200]);
    eraMioTurno = mioTurno;
    mostra(testata, msg,
      el("div", { classe: "scheda" },
        el("h2", { testo: comb.avviato ? "Round " + comb.round + (diTurno ? " · tocca a " + diTurno.nome : "") : "Si tira l'iniziativa" }),
        diTurno && diTurno.mio ? el("p", { classe: "avviso", testo: "⚔ È il tuo turno!" }) : null,
        el("ol", { classe: "ordine" }, lista.map((c) => riga(c, comb.turnoDi)))),
      miei.map(schedaMia),
      el("p", { classe: "tenue piccolo", testo: "Aggiornato alle " + ora(stato.ts) + (rete.server ? " · tramite " + rete.server : "") }));
  }

  disegna();
  collega();
})();
