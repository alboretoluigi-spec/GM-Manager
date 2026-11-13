// Effetti visivi su danni, cure e critici nel tracker (fase 4 delle richieste in sospeso, 2026-11-12).
//   const fx = creaEffettiColpi(() => contenitoreDelleRighe);
//   fx.esito(e)              ← ogni «attacco.esito» (anche i propri): «Mancato», 💥 CRITICO per tutti, colore del danno
//   fx.vista(prima, dopo)    ← dopo aver ridisegnato le righe: −N rosso / +N verde / ☠ a terra, la riga trema o si illumina
// Le righe portano data-comb="<id>". I numeri volano in uno strato fisso sopra la pagina (non spariscono quando il tracker si
// ridisegna); le classi delle righe si rimettono a ogni ridisegno con il tempo già passato, così l'animazione non riparte.
// I numeri vengono dai PF che cambiano (attacchi, incantesimi, danni e cure a mano); per i mostri di cui un giocatore non
// vede i PF, dal totale dell'esito dell'attacco. Con «riduci movimento» del sistema: niente scosse, solo colori e numeri.
// nessuna dipendenza: le funzioni pure si provano anche senza browser (test/regole/effetti_colpi_test.mjs)
function el(tag, attr, ...figli) {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(attr || {})) if (k === "class") n.className = v; else if (k === "testo") n.textContent = v; else n.setAttribute(k, v);
  n.append(...figli);
  return n;
}

const DURATA = { trema: 650, cura: 900, terra: 1400, mancato: 700, critico: 1200 };
const VOLO = 1700; // ms in cui un numero resta sullo schermo
// colore del numero secondo il tipo di danno principale dell'attacco
const COLORI_TIPO = ["acido", "freddo", "fuoco", "forza", "fulmine", "necrotico", "psichico", "radiante", "tuono", "veleno"];

export function tipoPrincipale(e) {
  if (!e || !e.danni || !e.danni.length) return "";
  const p = [...e.danni].sort((a, b) => (b.finale || 0) - (a.finale || 0))[0];
  return COLORI_TIPO.includes(p.tipo) ? p.tipo : "";
}

// variazione dei PF tra due viste: [{id, prima, dopo, delta, max, terra}] solo per chi ha i PF visibili in entrambe
export function confrontaPF(prima, dopo) {
  if (!prima || !dopo || !prima.combattimento || !dopo.combattimento || prima.combattimento.id !== dopo.combattimento.id) return [];
  const vecchi = new Map((prima.combattenti || []).map((c) => [c.id, c]));
  const out = [];
  for (const c of dopo.combattenti || []) {
    const v = vecchi.get(c.id);
    if (!v) continue;
    if (c.pf != null && v.pf != null) {
      const a = v.pf + (v.pfTemp || 0), b = c.pf + (c.pfTemp || 0);
      if (a !== b) out.push({ id: c.id, delta: b - a, max: c.pfMax || 0, terra: v.pf > 0 && c.pf <= 0, rialzato: v.pf <= 0 && c.pf > 0 });
    } else if (c.salute && v.salute && c.salute !== v.salute) {
      const ordine = ["a terra", "gravemente ferito", "ferito", "illeso"];
      const d = ordine.indexOf(c.salute) - ordine.indexOf(v.salute);
      out.push({ id: c.id, delta: d < 0 ? -1 : 1, senzaNumero: true, terra: c.salute === "a terra", rialzato: v.salute === "a terra" });
    }
  }
  return out;
}

export function creaEffettiColpi(contenitore) {
  const attivi = new Map(); // id → {classe, inizio, durata}
  const recenti = new Map(); // id bersaglio → {tipo, t, numeri: [elementi], danniDaEsito}
  const critici = new Map(); // attaccante|bersaglio → istante dell'ultimo 💥 (il critico arriva in due esiti)
  let strato = null;

  const riduci = () => { try { return matchMedia("(prefers-reduced-motion: reduce)").matches; } catch { return false; } };
  const rigaDi = (id) => { const c = contenitore(); return c ? c.querySelector('[data-comb="' + CSS.escape(id) + '"]') : null; };

  function segna(id, classe) {
    attivi.set(id, { classe, inizio: performance.now(), durata: DURATA[classe] || 800 });
    applica(id);
  }
  function applica(id) {
    const a = attivi.get(id), r = rigaDi(id);
    if (!a || !r) return;
    const passato = performance.now() - a.inizio;
    if (passato >= a.durata) { attivi.delete(id); return; }
    r.classList.add("fx-" + a.classe);
    r.style.animationDelay = "-" + Math.round(passato) + "ms";
    setTimeout(() => { r.classList.remove("fx-" + a.classe); r.style.animationDelay = ""; }, a.durata - passato);
  }

  function vola(id, testo, classe) {
    const r = rigaDi(id);
    if (!r) return null;
    if (!strato || !strato.isConnected) { strato = el("div", { class: "fx-strato", "aria-hidden": "true" }); document.body.append(strato); }
    const box = r.getBoundingClientRect();
    // più numeri sulla stessa riga: al massimo tre posizioni, un po' sfalsate (non escono dalla riga)
    const gia = [...strato.children].filter((x) => x.dataset.comb === id).length % 3;
    const n = el("div", { class: "fx-numero " + classe, testo });
    n.dataset.comb = id;
    n.style.left = Math.round(Math.min(box.left + box.width * 0.62 + gia * 34, window.innerWidth - 140)) + "px";
    n.style.top = Math.round(box.top + box.height / 2 - 18 - gia * 14) + "px";
    strato.append(n);
    setTimeout(() => n.remove(), VOLO);
    return n;
  }

  function overlayCritico(e) {
    const k = (e.attaccante || e.nomeAttaccante) + "|" + e.bersaglio;
    if (Date.now() - (critici.get(k) || 0) < 90000) return;
    critici.set(k, Date.now());
    document.querySelectorAll(".fx-critico-schermo").forEach((x) => x.remove());
    const o = el("div", { class: "effetto-dadi critico fx-critico-schermo", role: "alert", "aria-label": "Colpo critico di " + e.nomeAttaccante + " su " + e.nomeBersaglio },
      el("span", { class: "effetto-testo", testo: "CRITICO" }), el("span", { class: "effetto-chi", testo: e.nomeAttaccante + " → " + e.nomeBersaglio }));
    document.body.append(o);
    if (navigator.vibrate) try { navigator.vibrate([60, 40, 160]); } catch { /* niente vibrazione */ }
    setTimeout(() => o.remove(), 2600);
  }

  function esito(e) {
    if (!e || !e.bersaglio) return;
    const tipo = tipoPrincipale(e);
    const rec = { tipo, t: Date.now(), numeri: [] };
    recenti.set(e.bersaglio, rec);
    if (e.risultato === "mancato") {
      vola(e.bersaglio, "Mancato", "fx-mancato");
      if (!riduci()) segna(e.bersaglio, "mancato");
      return;
    }
    if (e.risultato === "critico") {
      overlayCritico(e);
      if (!riduci()) segna(e.bersaglio, "critico");
    }
    // un giocatore non vede i PF del mostro: il numero lo dà l'esito
    if (e.danni && e.danni.length && e.pfPrima == null && e.pfDopo == null) {
      rec.danniDaEsito = true;
      const n = vola(e.bersaglio, e.totaleDanni > 0 ? "−" + e.totaleDanni : "0", "fx-danno" + (tipo ? " fx-" + tipo : "") + (e.risultato === "critico" ? " fx-forte" : ""));
      if (n) rec.numeri.push(n);
      if (e.totaleDanni > 0 && !riduci()) segna(e.bersaglio, "trema");
    }
  }

  function vista(prima, dopo) {
    // classi ancora in corso sulle righe appena ridisegnate
    for (const id of [...attivi.keys()]) applica(id);
    for (const x of confrontaPF(prima, dopo)) {
      const rec = recenti.get(x.id);
      const fresco = rec && Date.now() - rec.t < 4000;
      if (x.delta < 0) {
        if (!(fresco && rec.danniDaEsito)) {
          const forte = x.max && -x.delta >= x.max / 4;
          if (!x.senzaNumero) vola(x.id, "−" + -x.delta, "fx-danno" + (fresco && rec.tipo ? " fx-" + rec.tipo : "") + (forte ? " fx-forte" : ""));
          if (!riduci()) segna(x.id, "trema");
        }
      } else if (!x.senzaNumero) {
        vola(x.id, "+" + x.delta, "fx-cura");
        segna(x.id, "cura");
      }
      if (x.terra) { vola(x.id, "☠ a terra", "fx-terra"); segna(x.id, "terra"); }
      else if (x.rialzato) vola(x.id, "✚ in piedi", "fx-cura");
    }
  }

  return { esito, vista };
}
