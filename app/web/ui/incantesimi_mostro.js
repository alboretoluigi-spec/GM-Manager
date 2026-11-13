// Incantesimi dei mostri (Parte 2 del piano di combattimento, fase F4). Il Master apre «✨ Incantesimi» di una creatura
// (tracker o scheda ridotta): gruppi letti dalla sua scheda (web/regole/attacchi_mostri.js, incantesimiMostro) con gli
// slot e gli usi al giorno rimasti; sceglie un incantesimo, Gaster Bless spende lo slot («mostro.incantesimo») e, se l'effetto si
// risolve in automatico, apre la scelta dei bersagli come per i personaggi (incantesimo_scontro.js) con la CD e il bonus
// d'attacco della creatura. I nomi dei bestiari (traduzioni diverse) si collegano all'elenco della scheda anche con i sinonimi.
import { el, avviso, dialogo, conferma, tempoReale } from "./ui.js";
import { apriIncantesimo } from "./incantesimo_scontro.js";
import { sg, testoDanni } from "./attacco.js";

// nome nei bestiari → nome inglese nell'elenco della scheda (web/regole/dati.js)
const SINONIMI = {
  "camuffare se stessi": "Disguise Self", "camuffarsi": "Disguise Self", "amici": "Friends", "salto": "Jump", "spruzzo acido": "Acid Splash",
  "anti-individuazione": "Nondetection", "protezione dalle lame": "Blade Ward", "avvizzire": "Blight", "scagliare maledizione": "Bestow Curse",
  "debilitazione mentale": "Feeblemind", "raggio di malattia": "Ray of Sickness", "scherno feroce": "Vicious Mockery", "passare senza tracce": "Pass without Trace",
  "infliggi ferite": "Inflict Wounds", "assassino fantasmatico": "Phantasmal Killer", "produrre fiamma": "Produce Flame", "sfera cromatica": "Chromatic Orb",
  "controllare acqua": "Control Water", "falsa vita": "False Life", "evocare elementali": "Conjure Elemental", "forma eterea": "Etherealness",
  "risparmiare i morenti": "Spare the Dying", "deflagrazione occulta": "Eldritch Blast", "plasmare la terra": "Mold Earth", "scolpire la pietra": "Stone Shape",
  "scolpire pietra": "Stone Shape", "smuovere la terra": "Move Earth", "serratura arcana": "Arcane Lock", "vuoto mentale": "Mind Blank",
  "movimento del ragno": "Spider Climb", "occhio malefico": "Eyebite", "occhi malevoli": "Eyebite", "piaga di insetti": "Insect Plague",
  "piaga degli insetti": "Insect Plague", "inganno": "Mislead", "sviare": "Mislead", "plasmare l'acqua": "Shape Water", "maremoto": "Tidal Wave",
  "stretta terrena di maximilian": "Maximilian's Earthen Grasp", "terra eruttiva": "Erupting Earth", "localizzare creature": "Locate Creature",
  "camminare tra gli alberi": "Tree Stride", "preveggenza": "Foresight", "scossa tellurica": "Earth Tremor", "trova cavalcatura": "Find Steed",
  "coltello di ghiaccio": "Ice Knife", "punizione ustionante": "Searing Smite", "punizione barcollante": "Staggering Smite", "fuoco fatato": "Faerie Fire",
  "individuazione delle malattie e dei veleni": "Detect Poison and Disease", "evoca animali": "Conjure Animals", "localizzare oggetti": "Locate Object",
  "schema ipnotico": "Hypnotic Pattern", "resurrezione pura": "True Resurrection", "legame di interdizione": "Warding Bond", "tocco del vampiro": "Vampiric Touch",
  "pelle di corteccia": "Barkskin", "senso animale": "Beast Sense", "riscaldare metallo": "Heat Metal",
};

let dati = null;
function caricaDati() {
  if (!dati) dati = Promise.all([fetch("/regole/dati.js").then((r) => r.text()), fetch("/regole/incantesimi_effetti.js").then((r) => r.text())]).then(([d, e]) => {
    const x = new Function(d + "\n" + e + "\n;return { SPELLS, IncantesimiEffetti };")();
    const per = new Map(), perEn = new Map();
    for (const s of x.SPELLS) {
      const v = { it: s[0], en: s[1], l: s[2], sc: s[3], t: s[4], r: s[5], cp: s[6], d: s[7], cl: s[8], src: s[9], ds: s[10], hi: s[11] || "" };
      per.set(s[0].toLowerCase(), v); perEn.set(s[1].toLowerCase(), v);
    }
    return { IE: x.IncantesimiEffetti, trova: (n) => { const k = String(n).toLowerCase().trim(); return per.get(k) || (SINONIMI[k] && perEn.get(SINONIMI[k].toLowerCase())) || perEn.get(k) || null; } };
  }).catch((e) => { dati = null; throw e; });
  return dati;
}

/** Effetto per lo scontro con i valori della creatura (come effettoScontro della scheda). */
function effettoMostro(IE, s, inc, livello) {
  const r = IE.analizza(s);
  if (!r || !r.automatico) return null;
  const lv = livello > 0 ? Math.max(livello, s.l) : s.l, mi = inc.mod || 0, L = inc.liv || 1;
  const t = String(s.t || "").replace(/^\*/, "");
  const tempo = t === "re" ? "reazione" : t === "ab" ? "bonus" : /^a\b/.test(t) ? "azione" : "altro";
  const fix = (d) => ({ dadi: d.dadi, mod: d.mod === "inc" ? mi : (+d.mod || 0), tipo: d.tipo });
  const e = { nome: s.it, livello: s.l, slot: lv, tempo, conc: !!r.conc, condizioni: r.condizioni || [], bersagli: IE.bersagliAlLivello(r, lv) || 0,
    round: r.durata && !r.durata.istantanea ? r.durata.round || 0 : 0, ripeti: !!(r.ts && r.ts.ripetuto), riassunto: IE.riassunto(r, lv, L, mi) };
  if (r.attacco) Object.assign(e, { tipo: "attacco", mischia: r.attacco.tipo === "mischia", bonus: inc.att || 0, danni: IE.danniAlLivello(r, lv, L).map(fix) });
  else if (r.ts && (r.danni.length || r.condizioni.length)) Object.assign(e, { tipo: "ts", cd: inc.cd || 10, car: r.ts.car, seRiesce: r.ts.seRiesce, danni: IE.danniAlLivello(r, lv, L).map(fix) });
  else if (r.cura) { const c = IE.curaAlLivello(r, lv); e.tipo = "cura"; e.cura = { dadi: c.dadi, mod: c.modInc ? mi : 0 }; }
  else if (r.pfTemp) { const p = IE.pfTempAlLivello(r, lv); e.tipo = "pftemp"; e.cura = { dadi: p.dadi, mod: p.modInc ? mi : 0 }; }
  else if (r.valori) Object.assign(e, IE.effettoValori(r)); // potenziamenti: effetto che cambia i valori
  else return null;
  return e;
}

export async function apriIncantesimiMostro(sessione, c) {
  const inc = c && c.incantesimi;
  if (!inc) return;
  let D;
  try { D = await caricaDati(); } catch (e) { avviso("Elenco degli incantesimi non disponibile: " + e.message, "errore"); return; }
  const usati = inc.usati || {};
  const chiudi = dialogo("✨ Incantesimi di " + c.nome, el("div", { class: "inc-mostro" },
    el("p", { class: "piccolo", testo: [inc.cd ? "CD " + inc.cd : "", inc.att ? sg(inc.att) + " a colpire" : "", inc.liv ? "incantatore di " + inc.liv + "° livello" : ""].filter(Boolean).join(" · ") }),
    inc.gruppi.map((g, i) => {
      const rimasti = g.usi ? Math.max(0, g.usi - (usati[i] || 0)) : null;
      return el("div", { class: "inc-gruppo" },
        el("b", { testo: g.etichetta + (g.usi ? " · " + rimasti + "/" + g.usi + (g.livello < 0 ? " oggi" : " slot") : g.livello === 0 ? "" : " · a volontà") +
          (g.cd && g.cd !== inc.cd ? " · CD " + g.cd : "") }),
        el("div", { class: "riga-flex" }, g.nomi.map((n) => {
          const s = D.trova(n);
          return el("button", { class: "piccolo" + (s ? "" : " tenue"), testo: n, title: s ? s.it + " · " + s.ds.slice(0, 200) : "Non è nell'elenco della scheda: si risolve a mano",
            on: { click: () => { chiudi(); scegli(i, g, n, s); } } });
        })));
    })));

  async function scegli(i, g, nome, s) {
    // CD, attacco e modificatore del tratto del gruppo (innati e da classe possono essere diversi), altrimenti della creatura
    const valori = { ...inc, cd: g.cd || inc.cd, att: g.att || inc.att, mod: g.mod || inc.mod };
    const eff = s ? effettoMostro(D.IE, s, valori, g.livello) : null;
    const spende = g.usi > 0;
    const corpo = el("div", {},
      s ? el("p", { class: "piccolo", testo: s.ds }) : el("p", { class: "tenue", testo: "«" + nome + "» non è nell'elenco degli incantesimi della scheda: l'effetto si risolve a mano." }),
      eff ? el("p", { class: "piccolo", testo: "Effetto: " + (eff.tipo === "attacco" ? "attacco con incantesimo " + sg(eff.bonus) + " · " + testoDanni({ danni: eff.danni || [] })
        : eff.tipo === "ts" ? "TS CD " + eff.cd + (eff.danni && eff.danni.length ? " · " + testoDanni({ danni: eff.danni }) : "") + (eff.condizioni.length ? " · " + eff.condizioni.join(", ") : "")
          : (eff.tipo === "cura" ? "cura " : "PF temporanei ") + eff.cura.dadi + (eff.cura.mod ? sg(eff.cura.mod) : "")) }) : s ? el("p", { class: "tenue piccolo", testo: "Effetto da applicare a mano (non si traduce in un'azione automatica sicura)." }) : null);
    dialogo("✨ " + (s ? s.it : nome) + " · " + c.nome, corpo, { azioni: (ch) => el("div", { class: "azioni" }, el("button", { testo: "Annulla", on: { click: ch } }),
      el("button", { class: "primario", testo: spende ? "Lancia (usa " + (g.livello < 0 ? "un uso" : "uno slot di " + g.etichetta) + ")" : "Lancia", on: { click: async (ev) => {
        ev.currentTarget.disabled = true;
        try {
          if (spende) {
            let r = await tempoReale.chiedi("mostro.incantesimo", { sessione, combattente: c.id, gruppo: i });
            if (r && r.serveConferma) {
              if (!(await conferma("Lanciare lo stesso?", r.serveConferma, "Lancia"))) { ch(); return; }
              r = await tempoReale.chiedi("mostro.incantesimo", { sessione, combattente: c.id, gruppo: i, forza: true });
            }
          }
          ch();
          if (eff) apriIncantesimo({ sessione, combattente: c.id }, eff);
          else avviso("✨ " + (s ? s.it : nome) + ": " + (spende ? "slot usato. " : "") + "Applica l'effetto a mano.", "", 6000);
        } catch (e) { avviso(e.message, "errore"); ev.currentTarget.disabled = false; }
      } } })) });
  }
}
