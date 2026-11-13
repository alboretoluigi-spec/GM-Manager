// Inserimento a mano dei contenuti (fase 3c, passo C2, docs/FORMATO_CONTENUTI.md). Il Master modifica le voci di un
// pacchetto con dei moduli, senza scrivere il file: ogni modulo è descritto da uno «schema» di campi; i campi che il
// modulo non mostra restano com'erano nella voce. Ogni salvataggio manda al server il pacchetto intero, che passa lo
// stesso controllo dell'importazione: se c'è un errore il pacchetto resta com'era e l'errore compare nel modulo.
import { el, svuota, api, avviso, dialogo, conferma } from "../ui/ui.js";

const CAR = [["FOR", "Forza"], ["DES", "Destrezza"], ["COS", "Costituzione"], ["INT", "Intelligenza"], ["SAG", "Saggezza"], ["CAR", "Carisma"]];
const ABILITA = ["Acrobazia", "Addestrare Animali", "Arcano", "Atletica", "Furtività", "Indagare", "Inganno", "Intimidire", "Intrattenere",
  "Intuizione", "Medicina", "Natura", "Percezione", "Persuasione", "Rapidità di Mano", "Religione", "Sopravvivenza", "Storia"].map((a) => [a, a]);
const ARMATURE = [["l", "Leggere"], ["m", "Medie"], ["p", "Pesanti"], ["sc", "Scudi"]];
const ARMI = [["s", "Semplici"], ["g", "Da guerra"]];
const SCUOLE = ["abiurazione", "ammaliamento", "divinazione", "evocazione", "illusione", "invocazione", "necromanzia", "trasmutazione"].map((s) => [s, s[0].toUpperCase() + s.slice(1)]);
const LETTERA_SCUOLA = { A: "abiurazione", C: "evocazione", D: "divinazione", E: "ammaliamento", V: "invocazione", I: "illusione", N: "necromanzia", T: "trasmutazione" };

// ----- Schemi delle voci -----
// Tipi di campo: testo, area, numero, scelta, multi (spunte → elenco), elenco (separato da virgole), numeri, righe
// (una per riga), si (sì/no), car (bonus alle caratteristiche), privilegi, tratti, gruppo (oggetto), sotto (elenco di
// voci annidate), coppie (codice → nome).
function incantatore(liste, sotto) {
  return { k: "incantatore", t: "gruppo", n: sotto ? "Incantatrice (come le sottoclassi che lanciano incantesimi)" : "Lancia incantesimi", campi: sotto
    ? [{ k: "tipo", t: "scelta", n: "Progressione", opzioni: [["third", "Un terzo (sottoclasse)"]] }, { k: "caratteristica", t: "scelta", n: "Caratteristica", opzioni: CAR }]
    : [{ k: "tipo", t: "scelta", n: "Progressione degli slot", opzioni: [["full", "Completa"], ["half", "Metà"], ["art", "Metà, per eccesso"], ["pact", "Magia del patto"]] },
      { k: "caratteristica", t: "scelta", n: "Caratteristica", opzioni: CAR },
      { k: "preparazione", t: "scelta", n: "Incantesimi", opzioni: [["full", "Preparati: livello + modificatore"], ["half", "Preparati: metà livello + modificatore"], ["known", "Conosciuti (tabella)"]] },
      { k: "lista", t: "scelta", n: "Lista di incantesimi", opzioni: [["", "—"], ...liste] },
      { k: "conosciuti", t: "numeri", n: "Incantesimi conosciuti per livello (20 numeri, solo per «Conosciuti»)" },
      { k: "trucchetti", t: "numeri", n: "Trucchetti: al 1° livello e i due livelli in cui se ne aggiunge uno (3 numeri)" }] };
}

function incRazza() {
  return { k: "incantesimiRazziali", t: "gruppo", n: "Incantesimi concessi dalla razza", campi: [
    { k: "caratteristica", t: "scelta", n: "Caratteristica", opzioni: CAR }, { k: "livelli", t: "livelli", n: "Per livello del personaggio" }] };
}

export function schemi(liste) {
  return {
    classi: { nome: "Classe", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "id", t: "id", n: "Identificativo" }, { k: "fonte", t: "testo", n: "Fonte (codice)" },
      { k: "dadoVita", t: "numero", n: "Dado vita (facce)", min: 4, max: 20, obbl: true, predef: 8 },
      { k: "tiriSalvezza", t: "multi", n: "Tiri salvezza con competenza", opzioni: CAR },
      { k: "armature", t: "multi", n: "Competenza nelle armature", opzioni: ARMATURE }, { k: "armi", t: "multi", n: "Competenza nelle armi", opzioni: ARMI },
      { k: "armiSpecifiche", t: "elenco", n: "Altre armi con competenza (separate da virgole)" },
      { k: "testoArmature", t: "testo", n: "Armature (testo nella scheda)" }, { k: "testoArmi", t: "testo", n: "Armi (testo nella scheda)" },
      { k: "strumenti", t: "testo", n: "Strumenti" },
      { k: "abilitaDaScegliere", t: "numero", n: "Abilità da scegliere", min: 0, max: 18 }, { k: "abilita", t: "multi", n: "Fra le abilità", opzioni: ABILITA },
      { k: "lingue", t: "elenco", n: "Lingue (separate da virgole)" },
      { k: "livelliAumento", t: "numeri", n: "Livelli con l'aumento delle caratteristiche (vuoto: 4, 8, 12, 16, 19)" },
      { k: "stiliDiCombattimento", t: "scelta", n: "Stile di combattimento", numerico: true, opzioni: [["0", "Nessuno"], ["1", "Dal 1° livello"], ["2", "Dal 2° livello"]] },
      incantatore(liste, false),
      { k: "nomeSottoclasse", t: "testo", n: "Nome della scelta di sottoclasse (es. «Ordine»)" },
      { k: "livelloSottoclasse", t: "numero", n: "Livello della sottoclasse", min: 1, max: 20, predef: 3 },
      { k: "privilegi", t: "privilegi", n: "Privilegi" },
      { k: "equipaggiamento", t: "righe", n: "Equipaggiamento iniziale (una riga per voce)" },
      { k: "sottoclassi", t: "sotto", n: "Sottoclassi", voce: "sottoclasse", campi: [
        { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "fonte", t: "testo", n: "Fonte (codice)" },
        { k: "privilegi", t: "privilegi", n: "Privilegi", inOrdine: true },
        { k: "abilitaFisse", t: "multi", n: "Abilità con competenza", opzioni: ABILITA },
        { k: "armature", t: "multi", n: "Altre armature con competenza", opzioni: ARMATURE }, { k: "armi", t: "multi", n: "Altre armi con competenza", opzioni: ARMI },
        { k: "attaccoExtra", t: "si", n: "Attacco extra" }, incantatore(liste, true),
        { k: "incantesimiSempre", t: "gruppo", n: "Incantesimi concessi dalla sottoclasse", campi: [
          { k: "modo", t: "scelta", n: "Come li ottiene", opzioni: [["preparati", "Sempre preparati"], ["conosciuti", "Conosciuti"]] },
          { k: "livelli", t: "livelli", n: "Per livello di classe (non contano nei limiti)" },
          { k: "trucchetti", t: "livelli", n: "Trucchetti concessi" }] }] }] },
    razze: { nome: "Razza", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "id", t: "id", n: "Identificativo" }, { k: "fonte", t: "testo", n: "Fonte (codice)" },
      { k: "caratteristiche", t: "car", n: "Bonus alle caratteristiche" },
      { k: "velocita", t: "numero", n: "Velocità (metri)", min: 0, max: 60, passo: 1.5 },
      { k: "taglia", t: "scelta", n: "Taglia", opzioni: [["", "—"], ["Minuscola", "Minuscola"], ["Piccola", "Piccola"], ["Media", "Media"], ["Grande", "Grande"]] },
      { k: "scurovisione", t: "numero", n: "Scurovisione (metri)", min: 0, max: 120, passo: 1.5 },
      { k: "lingue", t: "elenco", n: "Lingue (separate da virgole)" }, { k: "lingueExtra", t: "numero", n: "Lingue da scegliere", min: 0, max: 10 },
      { k: "abilitaFisse", t: "multi", n: "Abilità con competenza", opzioni: ABILITA }, { k: "abilitaDaScegliere", t: "numero", n: "Abilità da scegliere", min: 0, max: 18 },
      { k: "tratti", t: "tratti", n: "Tratti" }, { k: "talento", t: "si", n: "Un talento al 1° livello" },
      { k: "armaturaNaturale", t: "numero", n: "Armatura naturale (CA base + Destrezza)", min: 0, max: 30 },
      incRazza(), { k: "trucchettoAScelta", t: "area", n: "Trucchetto a scelta (spiegazione per il giocatore)" },
      { k: "sottorazze", t: "sotto", n: "Sottorazze", voce: "sottorazza", campi: [
        { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "id", t: "id", n: "Identificativo" },
        { k: "caratteristiche", t: "car", n: "Bonus alle caratteristiche" },
        { k: "velocita", t: "numero", n: "Velocità (metri)", min: 0, max: 60, passo: 1.5 },
        { k: "scurovisione", t: "numero", n: "Scurovisione (metri)", min: 0, max: 120, passo: 1.5 },
        { k: "abilitaFisse", t: "multi", n: "Abilità con competenza", opzioni: ABILITA }, { k: "tratti", t: "tratti", n: "Tratti" },
        incRazza(), { k: "trucchettoAScelta", t: "area", n: "Trucchetto a scelta (spiegazione per il giocatore)" }] }] },
    background: { nome: "Background", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "id", t: "id", n: "Identificativo" },
      { k: "abilita", t: "multi", n: "Abilità con competenza", opzioni: ABILITA }, { k: "abilitaDaScegliere", t: "numero", n: "Abilità da scegliere", min: 0, max: 18 },
      { k: "lingueExtra", t: "numero", n: "Lingue da scegliere", min: 0, max: 10 }, { k: "strumenti", t: "testo", n: "Strumenti" },
      { k: "privilegio", t: "testo", n: "Privilegio" }, { k: "descrizionePrivilegio", t: "area", n: "Descrizione del privilegio" },
      { k: "equipaggiamento", t: "area", n: "Equipaggiamento" }, { k: "monete", t: "numero", n: "Monete d'oro iniziali", min: 0, max: 100000 }] },
    incantesimi: { nome: "Incantesimo", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "nomeInglese", t: "testo", n: "Altro nome (facoltativo)" },
      { k: "livello", t: "numero", n: "Livello (0 = trucchetto)", min: 0, max: 9, obbl: true, predef: 0 },
      { k: "scuola", t: "scelta", n: "Scuola", opzioni: SCUOLE, leggi: (v) => LETTERA_SCUOLA[v] || v },
      { k: "tempo", t: "testo", n: "Tempo di lancio", obbl: true, suggerimenti: ["azione", "azione bonus", "reazione", "1m", "10m", "1h"], predef: "azione" },
      { k: "rituale", t: "si", n: "Rituale" },
      { k: "gittata", t: "testo", n: "Gittata" }, { k: "componenti", t: "testo", n: "Componenti", suggerimenti: ["V", "V, S", "V, S, M", "S"] },
      { k: "durata", t: "testo", n: "Durata", suggerimenti: ["istantanea", "1r", "1m", "10m", "1h", "8h", "24h", "permanente", "speciale"] },
      { k: "concentrazione", t: "si", n: "Concentrazione" },
      { k: "classi", t: "multi", n: "Liste di incantesimi", opzioni: liste, stringa: true },
      { k: "fonte", t: "testo", n: "Fonte (codice)" }, { k: "descrizione", t: "area", n: "Descrizione" }, { k: "livelliSuperiori", t: "area", n: "Ai livelli superiori" }] },
    armi: { nome: "Arma", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true },
      { k: "categoria", t: "scelta", n: "Categoria", opzioni: [["semplice", "Semplice"], ["guerra", "Da guerra"]] },
      { k: "tipo", t: "scelta", n: "Tipo", opzioni: [["mischia", "Da mischia"], ["distanza", "A distanza"]] },
      { k: "danno", t: "testo", n: "Danno (es. 1d8)", obbl: true }, { k: "tipoDanno", t: "testo", n: "Tipo di danno", suggerimenti: ["contundente", "perforante", "tagliente"] },
      { k: "proprieta", t: "testo", n: "Proprietà (testo nella scheda)" }, { k: "costo", t: "testo", n: "Costo (es. 10 mo)" },
      { k: "accurata", t: "si", n: "Accurata" }, { k: "leggera", t: "si", n: "Leggera" }, { k: "pesante", t: "si", n: "Pesante" }, { k: "dueMani", t: "si", n: "A due mani" },
      { k: "lancio", t: "si", n: "Da lancio" }, { k: "versatile", t: "si", n: "Versatile" }, { k: "portata", t: "si", n: "Portata" }] },
    armature: { nome: "Armatura", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true },
      { k: "tipo", t: "scelta", n: "Tipo", opzioni: [["leggera", "Leggera"], ["media", "Media"], ["pesante", "Pesante"]] },
      { k: "ca", t: "numero", n: "Classe Armatura base", min: 1, max: 30, obbl: true, predef: 11 },
      { k: "forza", t: "numero", n: "Forza minima (0 = nessuna)", min: 0, max: 30 }, { k: "svantaggioFurtivita", t: "si", n: "Svantaggio su Furtività" },
      { k: "costo", t: "testo", n: "Costo (es. 50 mo)" }] },
    talenti: { nome: "Talento", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "nomeInglese", t: "testo", n: "Altro nome (facoltativo)" }, { k: "fonte", t: "testo", n: "Fonte (codice)" },
      { k: "caratteristiche", t: "carTalento", n: "+1 a una caratteristica" }, { k: "descrizione", t: "area", n: "Descrizione" },
      { k: "effetti", t: "gruppo", n: "Effetti calcolati dalla scheda", sempre: true, campi: [
        { k: "init", t: "numero", n: "Bonus all'iniziativa", min: -10, max: 20 }, { k: "speed", t: "numero", n: "Velocità in più (metri)", min: 0, max: 30, passo: 1.5 },
        { k: "hpLvl", t: "numero", n: "Punti ferita per livello", min: 0, max: 10 }, { k: "pass", t: "numero", n: "Percezione passiva", min: -10, max: 20 },
        { k: "saveAb", t: "si", n: "Competenza nel tiro salvezza della caratteristica scelta", uno: true },
        { k: "armor", t: "multi", n: "Competenza nelle armature", opzioni: ARMATURE },
        { k: "medDex3", t: "si", n: "Destrezza fino a +3 con l'armatura media", uno: true },
        { k: "unarmored", t: "si", n: "CA 13 + Destrezza senza armatura", uno: true }] }] },
    stili: { nome: "Stile di combattimento", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "descrizione", t: "area", n: "Descrizione" },
      { k: "opzionale", t: "si", n: "Stile della regola facoltativa" }] },
    strumenti: { nome: "Strumento", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "id", t: "id", n: "Identificativo" },
      { k: "categoria", t: "scelta", n: "Categoria", opzioni: [["artigiano", "Strumenti da artigiano"], ["kit", "Kit"], ["gioco", "Set da gioco"], ["musica", "Strumento musicale"], ["veicolo", "Veicolo"]] },
      { k: "costo", t: "testo", n: "Costo (es. 25 mo)" }, { k: "alias", t: "elenco", n: "Altri nomi da riconoscere (separati da virgole)" },
      { k: "abilita", t: "multi", n: "Abilità collegate (vantaggio con entrambe le competenze)", opzioni: ABILITA },
      { k: "uso", t: "area", n: "Come si usa" }, { k: "puo", t: "righe", n: "Cosa si può fare (una riga per voce)" }, { k: "nonPuo", t: "righe", n: "Cosa non si può fare (una riga per voce)" },
      { k: "operazioni", t: "sotto", n: "Operazioni con un tiro", voce: "un'operazione", campi: [
        { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "caratteristica", t: "scelta", n: "Caratteristica", opzioni: CAR },
        { k: "cd", t: "numero", n: "CD indicativa (vuota: la decide il Master)", min: 1, max: 40 },
        { k: "abilita", t: "scelta", n: "Abilità che dà vantaggio", opzioni: [["", "—"], ...ABILITA] }] },
      { k: "fabbricazione", t: "sotto", n: "Lavori di fabbricazione", voce: "un lavoro", campi: [
        { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "materiali", t: "numero", n: "Materiali (mo)", min: 0, max: 1000000, passo: 0.01 },
        { k: "tempo", t: "testo", n: "Tempo" }, { k: "prodotto", t: "testo", n: "Prodotto" }, { k: "generico", t: "si", n: "Oggetto a scelta con la regola generale" }] }] },
    equipaggiamento: { nome: "Categoria dell'equipaggiamento", campi: [
      { k: "categoria", t: "testo", n: "Categoria (es. Equipaggiamento d'avventura)", obbl: true },
      { k: "voci", t: "tabella", n: "Oggetti acquistabili", voce: "un oggetto", colonne: [["nome", "Nome"], ["costo", "Costo"], ["nota", "Nota"]] }] },
    mostri: { nome: "Mostro", campi: [
      { k: "nome", t: "testo", n: "Nome", obbl: true }, { k: "nomeInglese", t: "testo", n: "Altro nome (facoltativo)" },
      { k: "fonte", t: "testo", n: "Fonte" }, { k: "note", t: "area", n: "Note per il Master" },
      { k: "scheda", t: "gruppo", n: "Scheda della creatura", sempre: true, campi: [
        { k: "taglia", t: "testo", n: "Taglia", suggerimenti: ["Minuscola", "Piccola", "Media", "Grande", "Enorme", "Mastodontica"] },
        { k: "tipo", t: "testo", n: "Tipo (es. bestia, umanoide)" }, { k: "all", t: "testo", n: "Allineamento" },
        { k: "ca", t: "testo", n: "Classe Armatura (es. 13 (armatura naturale))" }, { k: "pf", t: "testo", n: "Punti ferita (es. 22 (4d8 + 4))" },
        { k: "vel", t: "testo", n: "Velocità (es. 9 m)" }, { k: "car", t: "sei", n: "Caratteristiche" },
        { k: "ts", t: "testo", n: "Tiri salvezza" }, { k: "abil", t: "testo", n: "Abilità" },
        { k: "vuln", t: "testo", n: "Vulnerabilità" }, { k: "res", t: "testo", n: "Resistenze" }, { k: "imm", t: "testo", n: "Immunità ai danni" },
        { k: "immc", t: "testo", n: "Immunità alle condizioni" }, { k: "sensi", t: "testo", n: "Sensi" }, { k: "ling", t: "testo", n: "Linguaggi" },
        { k: "gs", t: "testo", n: "Grado di sfida (es. 1/2 (100 PE))" },
        { k: "tratti", t: "tabella", n: "Tratti", voce: "un tratto", colonne: [["n", "Nome"], ["d", "Descrizione", true]] },
        { k: "azioni", t: "tabella", n: "Azioni", voce: "un'azione", colonne: [["n", "Nome"], ["d", "Descrizione", true]] },
        { k: "reazioni", t: "tabella", n: "Reazioni", voce: "una reazione", colonne: [["n", "Nome"], ["d", "Descrizione", true]] },
        { k: "legg", t: "tabella", n: "Azioni leggendarie", voce: "un'azione", colonne: [["n", "Nome"], ["d", "Descrizione", true]] }] }] },
  };
}

export const INTESTAZIONE = [
  { k: "nome", t: "testo", n: "Nome del pacchetto", obbl: true }, { k: "autore", t: "testo", n: "Autore" }, { k: "versione", t: "testo", n: "Versione" },
  { k: "licenza", t: "testo", n: "Licenza" }, { k: "descrizione", t: "area", n: "Descrizione" },
  { k: "fonti", t: "coppie", n: "Fonti (codice → nome esteso)", chiave: "Codice", valore: "Nome" },
  { k: "listeIncantesimi", t: "coppie", n: "Liste di incantesimi nuove (una lettera minuscola → nome)", chiave: "Lettera", valore: "Nome", max: 1 },
];

// ----- Modulo generico -----
const vuoto = (v) => v === undefined || v === null || v === "" || (Array.isArray(v) && !v.length) || (typeof v === "object" && !Array.isArray(v) && !Object.keys(v).length);
const idDa = (s) => String(s || "").toLowerCase().normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 40);
let progressivo = 0;

function etichetta(c, controllo, aiuto) {
  const id = "ce-" + (++progressivo);
  controllo.id = id;
  return el("div", { class: "ce-campo" + (c.t === "area" ? " ce-largo" : "") }, el("label", { for: id, testo: c.n + (c.obbl ? " *" : "") }), controllo, aiuto || null);
}

/** Disegna un modulo per l'oggetto o (non modificato); restituisce { nodo, leggi() } con leggi() → oggetto nuovo. */
export function modulo(campi, o = {}) {
  const lettori = [];
  const nodi = campi.map((c) => {
    const v = o[c.k];
    switch (c.t) {
      case "testo": case "id": {
        const i = el("input", { type: "text", name: c.k, valore: v ?? c.predef ?? "", autocomplete: "off", list: c.suggerimenti ? "ce-dl-" + c.k : null });
        if (c.t === "id") { i.placeholder = "calcolato dal nome"; i.pattern = "[a-z0-9][a-z0-9_\\-]{0,39}"; }
        lettori.push((r) => { const x = i.value.trim(); if (c.t === "id") r[c.k] = x || idDa(r.nome); else r[c.k] = x; });
        return etichetta(c, i, c.suggerimenti ? el("datalist", { id: "ce-dl-" + c.k }, c.suggerimenti.map((s) => el("option", { value: s }))) : null);
      }
      case "area": {
        const i = el("textarea", { name: c.k, rows: 3 }); i.value = v ?? "";
        lettori.push((r) => { r[c.k] = i.value.trim(); });
        return etichetta(c, i);
      }
      case "numero": {
        const i = el("input", { type: "number", name: c.k, min: c.min, max: c.max, step: c.passo || 1, valore: v ?? c.predef ?? "" });
        lettori.push((r) => { r[c.k] = i.value === "" ? undefined : Number(i.value); });
        return etichetta(c, i);
      }
      case "scelta": {
        const s = el("select", { name: c.k }, c.opzioni.map(([val, n]) => el("option", { value: val, testo: n })));
        const att = c.leggi ? c.leggi(v) : v;
        s.value = att === undefined || att === null ? (c.opzioni[0] || [""])[0] : String(att);
        lettori.push((r) => { r[c.k] = c.numerico ? (Number(s.value) || undefined) : s.value; });
        return etichetta(c, s);
      }
      case "multi": {
        const scelti = new Set(Array.isArray(v) ? v : typeof v === "string" ? [...v] : []);
        const spunte = c.opzioni.map(([val, n]) => ({ val, x: el("input", { type: "checkbox", selezionato: scelti.has(val), "aria-label": c.n + ": " + n }) , n }));
        lettori.push((r) => { const l = spunte.filter((s) => s.x.checked).map((s) => s.val); r[c.k] = c.stringa ? l.join("") : l; });
        return el("fieldset", { class: "ce-campo ce-largo ce-multi" }, el("legend", { testo: c.n }),
          spunte.map((s) => el("label", { class: "riga" }, s.x, el("span", { testo: s.n }))));
      }
      case "elenco": case "numeri": {
        const i = el("input", { type: "text", name: c.k, valore: Array.isArray(v) ? v.join(", ") : "" });
        lettori.push((r) => { const l = i.value.split(",").map((x) => x.trim()).filter(Boolean); r[c.k] = c.t === "numeri" ? l.map(Number) : l; });
        return etichetta(c, i);
      }
      case "righe": {
        const i = el("textarea", { name: c.k, rows: 3 }); i.value = Array.isArray(v) ? v.join("\n") : (v || "");
        lettori.push((r) => { r[c.k] = i.value.split("\n").map((x) => x.trim()).filter(Boolean); });
        return etichetta(c, i);
      }
      case "si": {
        const x = el("input", { type: "checkbox", name: c.k, selezionato: !!v });
        lettori.push((r) => { r[c.k] = x.checked ? (c.uno ? 1 : true) : undefined; });
        return el("label", { class: "ce-campo riga ce-si" }, x, el("span", { testo: c.n }));
      }
      case "car": {
        const m = v || {};
        const ii = CAR.map(([k, n]) => ({ k, i: el("input", { type: "number", min: -10, max: 10, step: 1, valore: m[k] ?? "", "aria-label": c.n + ": " + n, placeholder: "0" }), n }));
        lettori.push((r) => { const out = {}; ii.forEach(({ k, i }) => { if (i.value !== "" && Number(i.value) !== 0) out[k] = Number(i.value); }); r[c.k] = out; });
        return el("fieldset", { class: "ce-campo ce-largo ce-car" }, el("legend", { testo: c.n }),
          ii.map(({ k, i }) => el("label", {}, el("span", { testo: k }), i)));
      }
      case "carTalento": {
        const modo = v === "*" ? "*" : Array.isArray(v) ? "alcune" : "";
        const s = el("select", { name: c.k }, el("option", { value: "", testo: "Nessuno" }), el("option", { value: "*", testo: "Qualsiasi" }), el("option", { value: "alcune", testo: "A scelta fra…" }));
        s.value = modo;
        const scelti = new Set(Array.isArray(v) ? v : []);
        const spunte = CAR.map(([k, n]) => ({ k, x: el("input", { type: "checkbox", selezionato: scelti.has(k), "aria-label": n }) }));
        const box = el("div", { class: "ce-multi-riga" }, spunte.map(({ k, x }) => el("label", { class: "riga" }, x, el("span", { testo: k }))));
        const vedi = () => { box.hidden = s.value !== "alcune"; };
        s.addEventListener("change", vedi); vedi();
        lettori.push((r) => { r[c.k] = s.value === "*" ? "*" : s.value === "alcune" ? spunte.filter((y) => y.x.checked).map((y) => y.k) : null; });
        return el("div", { class: "ce-campo ce-largo" }, etichetta(c, s), box);
      }
      case "privilegi": case "tratti": return righeNomi(c, o, lettori);
      case "tabella": { // elenco di oggetti con le stesse colonne (voci dell'equipaggiamento, tratti e azioni dei mostri)
        const righe = el("div", { class: "ce-righe" });
        const aggiungiRiga = (x = {}) => {
          const celle = c.colonne.map(([k, n, area]) => {
            const i = area ? el("textarea", { rows: 2, "aria-label": n, placeholder: n }) : el("input", { type: "text", "aria-label": n, placeholder: n });
            i.value = x[k] ?? ""; return [k, i];
          });
          const r = el("div", { class: "ce-riga" }, celle.map(([, i]) => i), el("button", { type: "button", class: "ce-x", "aria-label": "Togli", testo: "✕", on: { click: () => r.remove() } }));
          r.__leggi = () => Object.fromEntries(celle.map(([k, i]) => [k, i.value.trim()]));
          righe.append(r);
        };
        (Array.isArray(v) ? v : []).forEach(aggiungiRiga);
        lettori.push((r) => { r[c.k] = [...righe.children].map((x) => x.__leggi()).filter((x) => Object.values(x).some(Boolean)); });
        return el("fieldset", { class: "ce-campo ce-largo" }, el("legend", { testo: c.n }), righe,
          el("button", { type: "button", testo: "＋ Aggiungi " + (c.voce || "una riga"), on: { click: () => aggiungiRiga() } }));
      }
      case "livelli": { // {"3": ["A", "B"]}: livello e nomi separati da virgole
        const righe = el("div", { class: "ce-righe" });
        const aggiungiRiga = (liv = "", nomi = "") => {
          const l = el("input", { type: "number", min: 1, max: 20, valore: liv, "aria-label": "Livello", class: "ce-liv" });
          const n = el("input", { type: "text", valore: nomi, "aria-label": "Incantesimi (separati da virgole)", placeholder: "Incantesimi, separati da virgole" });
          const r = el("div", { class: "ce-riga" }, l, n, el("button", { type: "button", class: "ce-x", "aria-label": "Togli", testo: "✕", on: { click: () => r.remove() } }));
          r.__leggi = () => [l.value, n.value.split(",").map((x) => x.trim()).filter(Boolean)];
          righe.append(r);
        };
        Object.entries(v || {}).sort((a, b) => a[0] - b[0]).forEach(([liv, nomi]) => aggiungiRiga(liv, (nomi || []).join(", ")));
        lettori.push((r) => { const out = {}; [...righe.children].forEach((x) => { const [l, n] = x.__leggi(); if (l && n.length) out[l] = (out[l] || []).concat(n); }); r[c.k] = out; });
        return el("fieldset", { class: "ce-campo ce-largo" }, el("legend", { testo: c.n }), righe,
          el("button", { type: "button", testo: "＋ Aggiungi un livello", on: { click: () => aggiungiRiga() } }));
      }
      case "sei": { // sei caratteristiche in ordine (schede dei mostri)
        const val = Array.isArray(v) && v.length === 6 ? v : [10, 10, 10, 10, 10, 10];
        const ii = CAR.map(([k, n], i) => ({ i: el("input", { type: "number", min: 1, max: 30, valore: val[i], "aria-label": c.n + ": " + n }), k }));
        lettori.push((r) => { r[c.k] = ii.map(({ i }) => Number(i.value) || 10); });
        return el("fieldset", { class: "ce-campo ce-largo ce-car" }, el("legend", { testo: c.n }), ii.map(({ i, k }) => el("label", {}, el("span", { testo: k }), i)));
      }
      case "coppie": {
        const righe = el("div", { class: "ce-righe" });
        const aggiungiRiga = (k = "", val = "") => {
          const a = el("input", { type: "text", valore: k, "aria-label": c.chiave, placeholder: c.chiave, maxlength: c.max || 40, class: "ce-corto" });
          const b = el("input", { type: "text", valore: val, "aria-label": c.valore, placeholder: c.valore });
          const r = el("div", { class: "ce-riga" }, a, b, el("button", { type: "button", class: "ce-x", "aria-label": "Togli", testo: "✕", on: { click: () => r.remove() } }));
          r.__leggi = () => [a.value.trim(), b.value.trim()];
          righe.append(r);
        };
        Object.entries(v || {}).forEach(([k, val]) => aggiungiRiga(k, val));
        lettori.push((r) => { const out = {}; [...righe.children].forEach((x) => { const [k, val] = x.__leggi(); if (k) out[k] = val; }); r[c.k] = out; });
        return el("fieldset", { class: "ce-campo ce-largo" }, el("legend", { testo: c.n }), righe,
          el("button", { type: "button", testo: "＋ Aggiungi", on: { click: () => aggiungiRiga() } }));
      }
      case "gruppo": {
        const sotto = modulo(c.campi, v || {});
        const attivo = el("input", { type: "checkbox", selezionato: c.sempre || !vuoto(v), "aria-label": c.n });
        const corpo = el("div", { class: "ce-gruppo" }, sotto.nodo);
        const vedi = () => { corpo.hidden = !attivo.checked; };
        attivo.addEventListener("change", vedi); vedi();
        lettori.push((r) => { r[c.k] = attivo.checked ? sotto.leggi() : undefined; });
        return el("fieldset", { class: "ce-campo ce-largo" }, c.sempre ? el("legend", { testo: c.n }) : el("legend", {}, el("label", { class: "riga" }, attivo, el("span", { testo: c.n }))), corpo);
      }
      case "sotto": {
        const elenco = el("div", { class: "ce-sotto" });
        const aggiungiVoce = (x = {}, aperta = false) => {
          const m = modulo(c.campi, x);
          const titolo = el("summary", { testo: x.nome || "(nuova voce)" });
          const d = el("details", { class: "ce-voce-sotto", open: aperta }, titolo, m.nodo,
            el("button", { type: "button", class: "pericolo", testo: "🗑 Togli", on: { click: () => d.remove() } }));
          d.__leggi = m.leggi;
          elenco.append(d);
        };
        (Array.isArray(v) ? v : []).forEach((x) => aggiungiVoce(x));
        lettori.push((r) => { r[c.k] = [...elenco.children].map((d) => d.__leggi()); });
        return el("fieldset", { class: "ce-campo ce-largo" }, el("legend", { testo: c.n }), elenco,
          el("button", { type: "button", testo: "＋ Aggiungi " + c.voce, on: { click: () => aggiungiVoce({}, true) } }));
      }
    }
    return null;
  });
  return {
    nodo: el("div", { class: "ce-modulo" }, nodi),
    leggi() {
      const r = JSON.parse(JSON.stringify(o)); // i campi non mostrati restano com'erano
      lettori.forEach((f) => f(r));
      for (const k of Object.keys(r)) if (vuoto(r[k]) && !(campi.find((c) => c.k === k) || {}).obbl) delete r[k];
      return r;
    },
  };
}

// Privilegi (livello, nome, descrizione) e tratti (nome, descrizione): scrivono due campi del formato insieme.
function righeNomi(c, o, lettori) {
  const priv = c.t === "privilegi";
  const righe = el("div", { class: "ce-righe" });
  const aggiungiRiga = (liv = 1, nome = "", descr = "") => {
    const l = priv ? el("input", { type: "number", min: 1, max: 20, valore: liv, "aria-label": "Livello", class: "ce-liv" }) : null;
    const n = el("input", { type: "text", valore: nome, "aria-label": priv ? "Nome del privilegio" : "Nome del tratto", placeholder: "Nome" });
    const d = el("textarea", { rows: 2, "aria-label": "Descrizione", placeholder: "Descrizione (sintesi con parole tue)" }); d.value = descr;
    const r = el("div", { class: "ce-riga ce-riga-nome" }, l, n, d, el("button", { type: "button", class: "ce-x", "aria-label": "Togli", testo: "✕", on: { click: () => r.remove() } }));
    r.__leggi = () => ({ liv: l ? Number(l.value) || 1 : 0, nome: n.value.trim(), descr: d.value.trim() });
    righe.append(r);
  };
  if (priv) {
    const p = o.privilegi || {};
    const elenco = Object.keys(p).sort((a, b) => a - b).flatMap((liv) => (p[liv] || []).map((nome) => ({ liv: Number(liv), nome })));
    elenco.forEach((x, i) => aggiungiRiga(x.liv, x.nome, c.inOrdine ? (o.descrizioni || [])[i] || "" : (o.descrizioni || {})[x.nome] || ""));
  } else (o.tratti || []).forEach((nome) => aggiungiRiga(0, nome, (o.descrizioni || {})[nome] || ""));
  lettori.push((r) => {
    const v = [...righe.children].map((x) => x.__leggi()).filter((x) => x.nome);
    if (priv) {
      v.sort((a, b) => a.liv - b.liv); // stabile: dentro lo stesso livello resta l'ordine scritto
      const m = {};
      v.forEach((x) => (m[x.liv] = m[x.liv] || []).push(x.nome));
      r.privilegi = m;
      if (c.inOrdine) r.descrizioni = v.some((x) => x.descr) ? v.map((x) => x.descr) : undefined;
      else { const d = {}; v.forEach((x) => { if (x.descr) d[x.nome] = x.descr; }); r.descrizioni = d; }
    } else {
      r.tratti = v.map((x) => x.nome);
      const d = {}; v.forEach((x) => { if (x.descr) d[x.nome] = x.descr; }); r.descrizioni = d;
    }
  });
  return el("fieldset", { class: "ce-campo ce-largo" }, el("legend", { testo: c.n }),
    el("p", { class: "tenue piccolo", testo: priv ? "Un privilegio per riga, con il livello in cui si ottiene." : "Un tratto per riga." }), righe,
    el("button", { type: "button", testo: priv ? "＋ Aggiungi un privilegio" : "＋ Aggiungi un tratto", on: { click: () => aggiungiRiga() } }));
}

// ----- Finestra delle voci di un pacchetto -----
const TIPI = [["classi", "Classi"], ["razze", "Razze"], ["background", "Background"], ["incantesimi", "Incantesimi"], ["armi", "Armi"],
  ["armature", "Armature"], ["talenti", "Talenti"], ["stili", "Stili"], ["strumenti", "Strumenti"], ["equipaggiamento", "Equipaggiamento"], ["mostri", "Mostri"]];

async function leggiFile(id) {
  const r = await fetch("/api/v1/contenuti/" + encodeURIComponent(id) + "/file", { credentials: "same-origin" });
  if (!r.ok) throw new Error("Non riesco a leggere il pacchetto.");
  return r.json();
}

/** Apre l'elenco delle voci del pacchetto p (vista della pagina); liste: {lettera: nome} di tutti i pacchetti attivi. */
export async function apriEditor(p, listeNote, dopo) {
  let pac;
  try { pac = await leggiFile(p.id); } catch (e) { avviso(e.message, "errore"); return; }
  let tipo = "classi";
  const liste = () => Object.entries({ ...listeNote, ...(pac.listeIncantesimi || {}) }).sort((a, b) => a[1].localeCompare(b[1], "it"));
  const schede = el("div", { class: "ce-schede", role: "tablist" });
  const corpo = el("div", { class: "ce-corpo" });

  async function salva(nuovo) {
    const r = await api("/contenuti/" + encodeURIComponent(p.id), { metodo: "PUT", corpo: JSON.stringify(nuovo, null, 2), tipo: "application/json" });
    pac = nuovo;
    if (r.avvisi && r.avvisi.length) avviso("Salvato. Da controllare: " + r.avvisi.join(" · "), "errore");
    if (dopo) dopo();
    return r;
  }

  function disegna() {
    svuota(schede, TIPI.map(([k, n]) => el("button", { type: "button", role: "tab", "aria-selected": String(k === tipo), class: k === tipo ? "attiva" : "",
      testo: n + " (" + (pac[k] || []).length + ")", on: { click: () => { tipo = k; disegna(); } } })),
      el("button", { type: "button", role: "tab", "aria-selected": String(tipo === "intestazione"), class: tipo === "intestazione" ? "attiva" : "",
        testo: "⚙ Dati del pacchetto", on: { click: () => { tipo = "intestazione"; disegna(); } } }));
    if (tipo === "intestazione") { svuota(corpo, el("button", { type: "button", class: "primario", testo: "✏️ Modifica i dati del pacchetto", on: { click: () => moduloIntestazione() } }),
      el("p", { class: "piccolo", testo: [pac.nome, pac.autore && "di " + pac.autore, pac.versione && "versione " + pac.versione].filter(Boolean).join(" · ") }),
      el("p", { class: "tenue piccolo", testo: "Liste di incantesimi del pacchetto: " + (Object.entries(pac.listeIncantesimi || {}).map(([k, v]) => k + " = " + v).join(", ") || "nessuna") }));
      return; }
    const voci = pac[tipo] || [];
    const sch = schemi(liste())[tipo];
    const solaLettura = el("p", { class: "tenue piccolo ce-altro", testo: altriContenuti(pac) });
    svuota(corpo,
      el("button", { type: "button", class: "primario", testo: "＋ Nuova voce: " + sch.nome.toLowerCase(), on: { click: () => moduloVoce(-1) } }),
      voci.length ? el("ul", { class: "ce-voci" }, voci.map((v, i) => el("li", { class: "ce-voce" },
        el("span", { class: "ce-voce-nome", testo: v.nome || v.id || "(senza nome)" }),
        el("span", { class: "tenue piccolo", testo: descrizioneBreve(tipo, v) }),
        el("span", { class: "ce-voce-azioni" },
          el("button", { type: "button", testo: "✏️ Modifica", "aria-label": "Modifica " + (v.nome || ""), on: { click: () => moduloVoce(i) } }),
          el("button", { type: "button", testo: "⧉ Copia", "aria-label": "Copia " + (v.nome || ""), on: { click: () => moduloVoce(i, true) } }),
          el("button", { type: "button", class: "pericolo", testo: "🗑", "aria-label": "Elimina " + (v.nome || ""), on: { click: async () => {
            if (!(await conferma("Eliminare la voce?", "«" + (v.nome || v.id) + "» sparisce dal pacchetto e dalle schede.", "Elimina", true))) return;
            const nuovo = JSON.parse(JSON.stringify(pac)); nuovo[tipo].splice(i, 1);
            try { await salva(nuovo); avviso("Voce eliminata."); disegna(); } catch (e) { avviso(e.message, "errore"); }
          } } }))))) : el("p", { class: "tenue", testo: "Nessuna voce di questo tipo nel pacchetto." }), solaLettura.textContent ? solaLettura : null);
  }

  function finestraModulo(titolo, m, applica) {
    const errore = el("pre", { class: "ce-errore ct-pre", role: "alert", hidden: true });
    dialogo(titolo, el("div", {}, m.nodo, errore), { largo: true, azioni: (chiudi) => el("div", { class: "azioni" },
      el("button", { type: "button", testo: "Annulla", on: { click: chiudi } }),
      el("button", { type: "button", class: "primario", testo: "💾 Salva", on: { click: async (ev) => {
        const b = ev.currentTarget; b.disabled = true; errore.hidden = true;
        const nuovo = applica(m.leggi());
        try { await salva(nuovo); chiudi(); avviso("Salvato."); disegna(); }
        catch (e) { errore.textContent = e.message; errore.hidden = false; errore.scrollIntoView({ block: "nearest" }); }
        b.disabled = false;
      } } })) });
  }

  function moduloVoce(i, copia = false) {
    const sch = schemi(liste())[tipo];
    let v = i >= 0 ? JSON.parse(JSON.stringify(pac[tipo][i])) : {};
    if (copia) { v.nome = (v.nome || "") + " (copia)"; delete v.id; }
    const m = modulo(sch.campi, v);
    const t = tipo;
    finestraModulo(i >= 0 && !copia ? sch.nome + ": «" + (v.nome || v.id) + "»" : "Nuova voce: " + sch.nome.toLowerCase(), m, (letta) => {
      const nuovo = JSON.parse(JSON.stringify(pac));
      nuovo[t] = nuovo[t] || [];
      if (i >= 0 && !copia) nuovo[t][i] = letta; else nuovo[t].push(letta);
      return nuovo;
    });
  }

  function moduloIntestazione() {
    const m = modulo(INTESTAZIONE, pac);
    finestraModulo("Dati del pacchetto", m, (letta) => letta);
  }

  disegna();
  dialogo("Voci di «" + p.nome + "»", el("div", { class: "ce-editor" },
    p.personale ? null : el("p", { class: "tenue piccolo", testo: "Stai modificando la copia del pacchetto importato: il file originale sul tuo PC non cambia. «Esporta» salva la versione modificata." }),
    schede, corpo), { largo: true });
}

// Oggetti magici e professioni si inseriscono con il file del pacchetto (o, per gli oggetti, nell'archivio «Oggetti magici»).
function altriContenuti(pac) {
  const o = (pac.oggettiMagici || []).length, p = pac.professioni ? Object.keys(pac.professioni.professioni || {}).length : 0;
  if (!o && !p) return "";
  return "Il pacchetto contiene anche " + [o && o + " oggetti magici (si vedono e si consegnano nell'archivio «Oggetti magici»)", p && p + " professioni"].filter(Boolean).join(" e ") +
    ": si modificano nel file del pacchetto.";
}

function descrizioneBreve(tipo, v) {
  switch (tipo) {
    case "classi": return "d" + (v.dadoVita || "?") + ((v.sottoclassi || []).length ? " · " + v.sottoclassi.length + " sottoclassi" : "");
    case "razze": return (v.sottorazze || []).length ? v.sottorazze.length + " sottorazze" : "";
    case "incantesimi": return v.livello === 0 ? "trucchetto" : v.livello !== undefined ? v.livello + "° livello" : "";
    case "armi": return [v.danno, v.tipoDanno].filter(Boolean).join(" ");
    case "armature": return v.ca ? "CA " + v.ca : "";
    case "equipaggiamento": return (v.voci || []).length + " oggetti";
    case "mostri": return v.scheda && v.scheda.gs ? "GS " + v.scheda.gs : "";
    default: return "";
  }
}
