// Nomi leggibili dei campi della scheda e valori brevi (storico, conflitti, avvisi di controllo).
export const NOMI = { mag: "Oggetti magici", lvl: "Livello", cls: "Classe", sub: "Sottoclasse", race: "Razza", sr: "Sottorazza", bg: "Background", al: "Allineamento",
  "f.nome": "Nome", "f.pe": "Punti esperienza", "f.tesoro": "Tesoro", "f.altreComp": "Altre competenze", tl: "Strumenti", "f.giocatore": "Giocatore", armor: "Armatura", shield: "Scudo", eq: "Equipaggiamento", img: "Ritratto", style: "Stile di combattimento",
  base: "Caratteristiche", sk: "Abilità", ex: "Maestrie", asi: "Aumenti di caratteristica", hp: "Punti ferita per livello", sp: "Incantesimi",
  spp: "Incantesimi preparati", wp: "Armi", coins: "Monete", rA: "Aumenti razziali", eqAuto: "Equipaggiamento iniziale", spl: "Incantesimi", sym: "Simbolo", anc: "Antenato", sAnc: "Antenato", lin: "Lignaggio", rFeat: "Talento razziale", rFeatA: "Talento razziale", tOrig: "Origini personalizzate", tOpt: "Privilegi opzionali", style2: "Secondo stile di combattimento", mc: "Multiclasse", hpCur: "PF attuali", hpTmp: "PF temporanei", ds: "Tiri contro la morte", slotU: "Slot usati", insp: "Ispirazione" };
export function nomeCampo(k) {
  if (NOMI[k]) return NOMI[k];
  const [a, b] = k.split(".");
  if (a === "base") return "Caratteristica " + b;
  if (a === "f") return "Campo «" + b + "»";
  return (NOMI[a] || a) + (b ? " · " + b : "");
}
export function mostraValore(v) {
  if (v === null || v === undefined || v === "") return "(vuoto)";
  if (typeof v === "string") return v.startsWith("data:image") ? "(immagine)" : v.length > 60 ? v.slice(0, 60) + "…" : v;
  const s = JSON.stringify(v);
  return s.length > 60 ? s.slice(0, 60) + "…" : s;
}

// Come sul server (personaggi.go): chiavi principali e, per gli oggetti, un livello più in profondità; valori in JSON
// stabile (chiavi ordinate), così due valori uguali hanno lo stesso testo.
export function stabile(v) {
  if (Array.isArray(v)) return "[" + v.map(stabile).join(",") + "]";
  if (v && typeof v === "object") return "{" + Object.keys(v).sort().map((k) => JSON.stringify(k) + ":" + stabile(v[k])).join(",") + "}";
  return JSON.stringify(v === undefined ? null : v);
}

/** Come sul server: chiavi principali e, per gli oggetti, un livello più in profondità. */
export function appiattisci(dati) {
  const out = {};
  for (const [k, v] of Object.entries(dati || {})) {
    if (v && typeof v === "object" && !Array.isArray(v) && Object.keys(v).length) {
      for (const [k2, v2] of Object.entries(v)) out[k + "." + k2] = stabile(v2);
    } else out[k] = stabile(v);
  }
  return out;
}

export function ricomponi(p) {
  const out = {};
  for (const [k, v] of Object.entries(p)) {
    const i = k.indexOf(".");
    if (i > 0) { const a = k.slice(0, i); if (!out[a] || typeof out[a] !== "object") out[a] = {}; out[a][k.slice(i + 1)] = JSON.parse(v); }
    else out[k] = JSON.parse(v);
  }
  return out;
}

