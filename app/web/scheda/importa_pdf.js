/* Scheda da PDF compilabile (richiesta dell'utente, 2026-11-09).
   Il giocatore carica in «Importa» la scheda 5e in PDF compilata al computer o sul telefono (la scheda ufficiale in
   inglese o una delle italiane più diffuse). Gaster Bless legge i campi del modulo, riconosce nome, classe e livello, razza,
   background, allineamento, caratteristiche, competenze, monete, equipaggiamento e testi, e prepara la sua scheda:
   classe, razza e background sono quelli delle regole di Gaster Bless, quindi tutto il resto (bonus, tiri, incantesimi,
   privilegi) si calcola da solo. Prima di applicare mostra cosa ha riconosciuto. Legge anche la scheda italiana
   ufficiale NON compilabile, se è stata riempita al computer o sul telefono (vedi «campiDaPosizione»). Una scheda
   scritta a penna e fotografata non ha testo da leggere: va ricopiata a mano.
   Usa le variabili globali di scheda.js e regole/dati.js (st, blank, boot, CLASSES, RACES, BGS, ALIGN, SKILLS...). */
(function () {
  'use strict';
  const norm = (s) => String(s || '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]/g, '');
  const testoCampo = (v) => (v == null ? '' : String(v)).trim();

  // Nomi inglesi → chiavi di Gaster Bless
  const CLASSI_EN = { artificer: 'artefice', barbarian: 'barbaro', bard: 'bardo', cleric: 'chierico', druid: 'druido', fighter: 'guerriero',
    monk: 'monaco', paladin: 'paladino', ranger: 'ranger', rogue: 'ladro', sorcerer: 'stregone', warlock: 'warlock', wizard: 'mago' };
  const RAZZE_EN = { dragonborn: 'dragonide', dwarf: 'nano', elf: 'elfo', gnome: 'gnomo', halfelf: 'mezzelfo', halforc: 'mezzorco', halfling: 'halfling',
    human: 'umano', tiefling: 'tiefling', aasimar: 'aasimar', goliath: 'goliath', tabaxi: 'tabaxi', firbolg: 'firbolg', genasi: 'genasi', kenku: 'kenku', lizardfolk: 'lucertoloide', triton: 'tritone' };
  const BG_EN = { acolyte: 'accolito', charlatan: 'ciarlatano', criminal: 'criminale', entertainer: 'intrattenitore', folkhero: 'eroe', guildartisan: 'artigiano',
    hermit: 'eremita', noble: 'nobile', outlander: 'forestiero', sage: 'sapiente', sailor: 'marinaio', soldier: 'soldato', urchin: 'monello', hauntedone: 'perseguitato' };
  const ALL_EN = { lawfulgood: 'Legale Buono', neutralgood: 'Neutrale Buono', chaoticgood: 'Caotico Buono', lawfulneutral: 'Legale Neutrale', neutral: 'Neutrale',
    trueneutral: 'Neutrale', chaoticneutral: 'Caotico Neutrale', lawfulevil: 'Legale Malvagio', neutralevil: 'Neutrale Malvagio', chaoticevil: 'Caotico Malvagio',
    lg: 'Legale Buono', ng: 'Neutrale Buono', cg: 'Caotico Buono', ln: 'Legale Neutrale', n: 'Neutrale', cn: 'Caotico Neutrale', le: 'Legale Malvagio', ne: 'Neutrale Malvagio', ce: 'Caotico Malvagio' };
  const SK_EN = { acrobatics: 'Acrobazia', animalhandling: 'Addestrare Animali', arcana: 'Arcano', athletics: 'Atletica', stealth: 'Furtività', investigation: 'Indagare',
    deception: 'Inganno', intimidation: 'Intimidire', performance: 'Intrattenere', insight: 'Intuizione', medicine: 'Medicina', nature: 'Natura', perception: 'Percezione',
    persuasion: 'Persuasione', sleightofhand: 'Rapidità di Mano', religion: 'Religione', survival: 'Sopravvivenza', history: 'Storia' };
  // Scheda ufficiale inglese: caselle delle competenze nelle abilità (in ordine alfabetico inglese).
  const SK_CASELLE = { checkbox23: 'acrobatics', checkbox24: 'animalhandling', checkbox25: 'arcana', checkbox26: 'athletics', checkbox27: 'deception', checkbox28: 'history',
    checkbox29: 'insight', checkbox30: 'intimidation', checkbox31: 'investigation', checkbox32: 'medicine', checkbox33: 'nature', checkbox34: 'perception',
    checkbox35: 'performance', checkbox36: 'persuasion', checkbox37: 'religion', checkbox38: 'sleightofhand', checkbox39: 'stealth', checkbox40: 'survival' };

  // Campo del PDF (nome normalizzato) → cosa diventa nella scheda di Gaster Bless
  const CAMPI = {
    charactername: 'nome', nomepersonaggio: 'nome', nomedelpersonaggio: 'nome', personaggio: 'nome', charactername2: 'nome',
    playername: 'giocatore', nomegiocatore: 'giocatore', giocatore: 'giocatore',
    classlevel: 'classeLivello', classeelivello: 'classeLivello', classelivello: 'classeLivello', classe: 'classe', livello: 'livello', level: 'livello',
    race: 'razza', razza: 'razza', background: 'bg', retroterra: 'bg', alignment: 'allineamento', allineamento: 'allineamento',
    xp: 'pe', experiencepoints: 'pe', puntiesperienza: 'pe', pe: 'pe',
    str: 'FOR', forza: 'FOR', for: 'FOR', dex: 'DES', destrezza: 'DES', des: 'DES', con: 'COS', costituzione: 'COS', cos: 'COS',
    int: 'INT', intelligenza: 'INT', wis: 'SAG', saggezza: 'SAG', sag: 'SAG', cha: 'CAR', carisma: 'CAR', car: 'CAR',
    hpcurrent: 'pfAttuali', puntiferitaattuali: 'pfAttuali', pfattuali: 'pfAttuali',
    cp: 'MR', mr: 'MR', sp: 'MA', ma: 'MA', ep: 'ME', me: 'ME', gp: 'MO', mo: 'MO', pp: 'MP', mp: 'MP',
    equipment: 'eq', equipaggiamento: 'eq',
    personalitytraits: 'tratti', trattidellapersonalita: 'tratti', tratticaratteriali: 'tratti', ideals: 'ideali', ideali: 'ideali',
    bonds: 'legami', legami: 'legami', flaws: 'difetti', difetti: 'difetti',
    featuresandtraits: 'privNote', privilegietratti: 'privNote', feattraits: 'trattiAgg', trattieprivilegiaggiuntivi: 'trattiAgg',
    proficiencieslang: 'altreComp', altrecompetenzeelinguaggi: 'altreComp', altrecompetenze: 'altreComp',
    age: 'eta', eta: 'eta', height: 'altezza', altezza: 'altezza', weight: 'peso', peso: 'peso', eyes: 'occhi', occhi: 'occhi',
    skin: 'carnagione', carnagione: 'carnagione', hair: 'capelli', capelli: 'capelli',
    backstory: 'storia', storiadelpersonaggio: 'storia', storia: 'storia', allies: 'alleati', alleatieorganizzazioni: 'alleati', factionname: 'orgNome',
    treasure: 'tesoro', tesoro: 'tesoro', inspiration: 'ispirazione', ispirazione: 'ispirazione',
  };
  const ETICHETTE = { nome: 'Nome', giocatore: 'Giocatore', classeLivello: 'Classe e livello', classe: 'Classe', livello: 'Livello', razza: 'Razza', bg: 'Background',
    allineamento: 'Allineamento', pe: 'Punti esperienza', FOR: 'Forza', DES: 'Destrezza', COS: 'Costituzione', INT: 'Intelligenza', SAG: 'Saggezza', CAR: 'Carisma',
    pfAttuali: 'PF attuali', MR: 'Monete di rame', MA: "Monete d'argento", ME: 'Monete di electrum', MO: "Monete d'oro", MP: 'Monete di platino', eq: 'Equipaggiamento',
    tratti: 'Tratti caratteriali', ideali: 'Ideali', legami: 'Legami', difetti: 'Difetti', privNote: 'Privilegi e tratti', trattiAgg: 'Tratti aggiuntivi',
    altreComp: 'Altre competenze e linguaggi', eta: 'Età', altezza: 'Altezza', peso: 'Peso', occhi: 'Occhi', carnagione: 'Carnagione', capelli: 'Capelli',
    storia: 'Storia', alleati: 'Alleati', orgNome: 'Organizzazione', tesoro: 'Tesoro', ispirazione: 'Ispirazione' };

  function trova(elenco, inglese, testo) {
    const t = norm(testo);
    if (!t) return '';
    for (const [k, v] of Object.entries(elenco)) if (norm(k) === t || norm(v.n) === t) return k;
    for (const [en, k] of Object.entries(inglese)) if (t.includes(en) && elenco[k]) return k;
    for (const [k, v] of Object.entries(elenco)) if (t.includes(norm(v.n)) || norm(v.n).includes(t)) return k;
    return '';
  }


  // ===== Scheda italiana ufficiale NON compilabile, riempita al computer o sul telefono (2026-11-09) =====
  // Programmi come Anteprima, Acrobat («Compila e firma»), Xodo o le app del telefono scrivono il testo sopra la pagina
  // (come testo della pagina o come annotazione «testo libero»). Gaster Bless riconosce la scheda dalle sue scritte stampate,
  // le scarta e assegna il resto alle caselle in base alla posizione. Coordinate in punti, dall'alto della pagina.
  // Scritte stampate della scheda vuota: [pagina, x, y della linea di base, testo].
  const MODELLO_IT = [[1,264,90,"RAZZA"],[1,264,63,"CLASSE & LIVELLO"],[1,473,63,"NOME GIOCATORE"],[1,62,90,"NOME PERSONAGGIO"],[1,377,63,"BACKGROUND"],[1,473,90,"PUNTI ESPERIENZA"],[1,377,90,"ALLINEAMENTO"],[1,176,773,"TM & © 2014 l'editore. È permesso fotocopiare questa scheda per uso personale."],[1,223,200,"Massimo dei Punti Ferita"],[1,123,207,"Forza"],[1,123,220,"Destrezza"],[1,123,234,"Costituzione"],[1,123,247,"Intelligenza"],[1,123,261,"Saggezza"],[1,123,274,"Carisma"],[1,79,595,"SAGGEZZA (PERCEZIONE) PASSIVA"],[1,270,756,"EQUIPAGGIAMENTO"],[1,60,756,"ALTRE COMPETENZE & LINGUAGGI"],[1,262,569,"ATTACCHI & INCANTESIMI"],[1,460,757,"PRIVILEGI & TRATTI"],[1,123,322,"Acrobazia (Des)"],[1,123,336,"Addestrare Animali (Sag)"],[1,123,349,"Arcano (Int)"],[1,123,363,"Atletica (For)"],[1,123,376,"Furtività (Des)"],[1,123,390,"Indagare (Int)"],[1,123,403,"Inganno (Car)"],[1,123,417,"Intimidire (Car)"],[1,123,430,"Intrattenere (Car)"],[1,123,444,"Intuizione (Sag)"],[1,123,457,"Medicina (Sag)"],[1,123,471,"Natura (Int)"],[1,123,484,"Percezione (Sag)"],[1,123,498,"Persuasione (Car)"],[1,123,511,"Rapidità di Mano (Des)"],[1,123,525,"Religione (Int)"],[1,123,538,"Sopravvivenza (Sag)"],[1,123,552,"Storia (Int)"],[1,316,352,"TS CONTRO MORTE"],[1,243,352,"DADI VITA"],[1,217,382,"NOME"],[1,284,382,"BONUS ATT."],[1,320,382,"DANNI/TIPO"],[1,223,321,"Totale"],[1,312,322,"SUCCESSI"],[1,307,337,"FALLIMENTI"],[1,480,243,"IDEALI"],[1,478,299,"LEGAMI"],[1,478,353,"DIFETTI"],[1,461,188,"TRATTI CARATTERIALI"],[1,237,171,"CA"],[1,266,244,"PUNTI FERITA ATTUALI"],[1,259,296,"PUNTI FERITA TEMPORANEI"],[1,281,174,"INIZIATIVA"],[1,341,174,"VELOCITÀ"],[1,124,173,"BONUS DI COMPETENZA"],[1,40,145,"FORZA"],[1,34,216,"DESTREZZA"],[1,28,288,"COSTITUZIONE"],[1,29,360,"INTELLIGENZA"],[1,35,431,"SAGGEZZA"],[1,37,503,"CARISMA"],[1,122,290,"TIRI SALVEZZA"],[1,140,135,"ISPIRAZIONE"],[1,132,569,"ABILITÀ"],[1,206,603,"MR"],[1,206,655,"ME"],[1,206,706,"MP"],[1,206,681,"MO"],[1,206,629,"MA"],[2,59,91,"NOME PERSONAGGIO"],[2,173,770,"TM & © 2014 l'editore. È permesso fotocopiare questa scheda per uso personale."],[2,256,90,"OCCHI"],[2,256,64,"ETÀ"],[2,466,64,"PESO"],[2,369,64,"ALTEZZA"],[2,466,90,"CAPELLI"],[2,369,90,"CARNAGIONE"],[2,412,141,"NOME"],[2,379,754,"TESORO"],[2,97,754,"STORIA"],[2,94,350,"ASPETTO"],[2,344,579,"TRATTI & PRIVILEGI AGGIUNTIVI"],[2,349,350,"ALLEATI & ORGANIZZAZIONI"],[2,468,292,"SIMBOLO"],[3,175,773,"TM & © 2014 l'editore. È permesso fotocopiare questa scheda per uso personale."],[3,62,92,"CLASSE DA"],[3,62,100,"INCANTATORE"],[3,13,466,"INCANTESIMI CONOSCIUTI"],[3,84,347,"NOME INCANTESIMO"],[3,23,347,"P"],[3,23,345,"R"],[3,24,343,"E"],[3,25,342,"P"],[3,27,341,"A"],[3,29,341,"R"],[3,31,342,"A"],[3,32,344,"T"],[3,33,346,"I"],[3,21,292,"LIVELLO"],[3,21,298,"INC.MO"],[3,129,302,"SLOT SPESI"],[3,44,302,"SLOT TOTALI"],[3,30,321,"1"],[3,30,154,"0"],[3,30,550,"2"],[3,219,154,"3"],[3,406,154,"6"],[3,406,323,"7"],[3,406,493,"8"],[3,406,634,"9"],[3,219,379,"4"],[3,219,606,"5"],[3,283,86,"CARATTERISTICA"],[3,280,93,"DA INCANTATORE"],[3,381,86,"CD TIRO SALVEZZA"],[3,389,93,"INCANTESIMI"],[3,482,86,"BONUS DI ATTACCO"],[3,492,93,"INCANTESIMI"],[3,100,152,"TRUCCHETTI"]];
  // Caselle: [pagina, x0, x1, y0, y1, dove]. y = centro della riga scritta.
  const CASELLE_IT = [
    [1, 28, 258, 38, 86, 'nomepersonaggio'],
    [1, 262, 373, 34, 60, 'classeelivello'], [1, 375, 471, 34, 60, 'background'], [1, 472, 580, 34, 60, 'nomegiocatore'],
    [1, 262, 373, 62, 86, 'razza'], [1, 375, 471, 62, 86, 'allineamento'], [1, 472, 580, 62, 86, 'puntiesperienza'],
    [1, 84, 120, 116, 146, 'ispirazione'],
    [1, 20, 88, 146, 202, 'forza'], [1, 20, 88, 218, 274, 'destrezza'], [1, 20, 88, 289, 345, 'costituzione'],
    [1, 20, 88, 361, 417, 'intelligenza'], [1, 20, 88, 433, 489, 'saggezza'], [1, 20, 88, 504, 560, 'carisma'],
    [1, 216, 382, 202, 240, 'pfattuali'],
    [1, 402, 578, 126, 184, 'tratticaratteriali'], [1, 402, 578, 192, 239, 'ideali'], [1, 402, 578, 248, 295, 'legami'], [1, 402, 578, 304, 350, 'difetti'],
    [1, 212, 264, 589, 612, 'mr'], [1, 212, 264, 615, 638, 'ma'], [1, 212, 264, 641, 664, 'me'], [1, 212, 264, 667, 690, 'mo'], [1, 212, 264, 693, 716, 'mp'],
    [1, 265, 390, 580, 752, 'equipaggiamento'], [1, 20, 200, 612, 752, 'altrecompetenzeelinguaggi'], [1, 398, 578, 372, 752, 'privilegietratti'],
    [2, 28, 252, 38, 88, 'nomepersonaggio'],
    [2, 252, 365, 34, 60, 'eta'], [2, 366, 462, 34, 60, 'altezza'], [2, 463, 580, 34, 60, 'peso'],
    [2, 252, 365, 62, 86, 'occhi'], [2, 366, 462, 62, 86, 'carnagione'], [2, 463, 580, 62, 86, 'capelli'],
    [2, 405, 556, 132, 162, 'factionname'], [2, 205, 578, 112, 346, 'alleati'],
    [2, 205, 578, 360, 574, 'trattieprivilegiaggiuntivi'], [2, 16, 199, 368, 750, 'storia'], [2, 205, 578, 588, 750, 'tesoro'],
  ];
  // Abilità: il segno della competenza (x, ●, ✓…) va nel cerchietto a sinistra del nome, alla stessa altezza.
  const ABIL_IT = ['acrobatics', 'animalhandling', 'arcana', 'athletics', 'stealth', 'investigation', 'deception', 'intimidation', 'performance',
    'insight', 'medicine', 'nature', 'perception', 'persuasion', 'sleightofhand', 'religion', 'survival', 'history'];
  const CASELLA_SK = Object.fromEntries(Object.entries(SK_CASELLE).map(([k, v]) => [v, k]));

  async function scritteDellaPagina(pg) {
    const h = pg.view[3], out = [];
    for (const it of (await pg.getTextContent()).items) {
      if (!it.str || !it.str.trim()) continue;
      const alt = Math.abs(it.transform[3]) || it.height || 8;
      out.push({ t: it.str, x: it.transform[4], y: h - it.transform[5], yc: h - it.transform[5] - alt * 0.35, alt });
    }
    // Testo libero aggiunto come annotazione (Anteprima, Acrobat, app del telefono)
    for (const a of await pg.getAnnotations()) {
      if (!/^(FreeText|Text)$/.test(a.subtype || '') || !a.rect) continue;
      const testo = (Array.isArray(a.textContent) && a.textContent.length ? a.textContent.join('\n') : (a.contentsObj && a.contentsObj.str) || a.contents || '').trim();
      if (!testo) continue;
      const righe = testo.split(/\r?\n/), alt = Math.max(6, Math.min(14, (a.rect[3] - a.rect[1]) / righe.length));
      righe.forEach((r, i) => { if (r.trim()) out.push({ t: r, x: a.rect[0] + 1, y: h - a.rect[3] + alt * (i + 0.8), yc: h - a.rect[3] + alt * (i + 0.5), alt, ann: true }); });
    }
    return out;
  }

  async function campiDaPosizione(doc) {
    if (doc.numPages < 2) return null;
    const pagine = [];
    for (let p = 1; p <= Math.min(3, doc.numPages); p++) pagine.push(await scritteDellaPagina(await doc.getPage(p)));
    const stampata = (n, it) => !it.ann && MODELLO_IT.some(([p, x, y, t]) => p === n && Math.abs(x - it.x) < 4 && Math.abs(y - it.y) < 4 && norm(t) === norm(it.t));
    const stampate = pagine[0].filter((it) => stampata(1, it)).length;
    if (stampate < 25) return null; // non è la scheda italiana
    const out = {};
    pagine.forEach((scritte, i) => {
      const n = i + 1;
      const libere = scritte.filter((it) => !stampata(n, it));
      for (const [p, x0, x1, y0, y1, dove] of CASELLE_IT) {
        if (p !== n || out[dove] != null) continue;
        const dentro = libere.filter((it) => !it.preso && it.x >= x0 - 2 && it.x <= x1 && it.yc >= y0 && it.yc <= y1);
        if (!dentro.length) continue;
        dentro.forEach((it) => { it.preso = true; });
        dentro.sort((a, b) => Math.abs(a.y - b.y) > Math.min(a.alt, b.alt) * 0.5 ? a.y - b.y : a.x - b.x);
        let testo = '', prec = null;
        for (const it of dentro) {
          if (prec) testo += Math.abs(prec.y - it.y) > Math.min(prec.alt, it.alt) * 0.5 ? '\n' : (testo.endsWith(' ') || it.t.startsWith(' ') ? '' : ' ');
          testo += it.t; prec = it;
        }
        testo = testo.replace(/[ \t]+/g, ' ').trim();
        if (/^(forza|destrezza|costituzione|intelligenza|saggezza|carisma)$/.test(dove)) {
          // nella casella ci possono essere il modificatore («+2») e il punteggio: vale il punteggio
          const numeri = (testo.match(/[+\-−]?\d+/g) || []).filter((v) => /^\d/.test(v)).map(Number).filter((v) => v >= 1 && v <= 30);
          if (numeri.length) out[dove] = String(Math.max(...numeri));
        } else if (dove === 'ispirazione') out[dove] = true;
        else if (!/^(nomepersonaggio|classeelivello|background|nomegiocatore|razza|allineamento|puntiesperienza|eta|altezza|peso|occhi|carnagione|capelli|factionname|mr|ma|me|mo|mp)$/.test(dove)) out[dove] = testo;
        else out[dove] = testo.replace(/\n/g, ' ');
      }
      if (n === 1) {
        // segni di competenza nelle abilità
        const nomi = MODELLO_IT.filter(([p, x, y, t]) => p === 1 && x === 123 && y > 300 && y < 560);
        for (const it of libere) {
          if (it.x < 84 || it.x > 104) continue;
          const riga = nomi.find(([, , y]) => Math.abs(y - it.y) < 5.5);
          if (riga) out[CASELLA_SK[ABIL_IT[nomi.indexOf(riga)]]] = true;
        }
      }
    });
    return out;
  }

  async function campiDelPDF(file) {
    await loadScript(PDFJS + 'pdf.min.js');
    const lib = window.pdfjsLib;
    if (lib.GlobalWorkerOptions && !lib.GlobalWorkerOptions.workerSrc) lib.GlobalWorkerOptions.workerSrc = PDFJS + 'pdf.worker.min.js';
    const doc = await lib.getDocument({ data: new Uint8Array(await file.arrayBuffer()), isEvalSupported: false }).promise;
    const out = {};
    let obj = null;
    try { obj = await doc.getFieldObjects(); } catch (e) { obj = null; }
    if (obj) {
      for (const [nome, arr] of Object.entries(obj)) {
        const x = (arr || []).find((a) => a && a.value != null && a.value !== '' && a.value !== 'Off') || (arr || [])[0];
        if (!x) continue;
        out[nome] = x.type === 'checkbox' ? (x.value && x.value !== 'Off') : x.value;
      }
    } else {
      for (let p = 1; p <= doc.numPages; p++) {
        for (const a of await (await doc.getPage(p)).getAnnotations()) if (a.fieldName) out[a.fieldName] = a.checkBox ? (a.fieldValue && a.fieldValue !== 'Off') : a.fieldValue;
      }
    }
    if (!Object.keys(out).length) return (await campiDaPosizione(doc)) || out;
    return out;
  }

  /** Dal PDF a una scheda di Gaster Bless: {scheda, riconosciuti: [[etichetta, valore]], ignorati: n} */
  async function schedaDaPDF(file) {
    const campi = await campiDelPDF(file);
    const nomi = Object.keys(campi);
    if (!nomi.length) throw new Error('In questo PDF non trovo dati da leggere: va bene una scheda compilabile, oppure la scheda italiana ufficiale compilata al computer o sul telefono. Se è una scansione o una foto di una scheda scritta a penna, ricopia i dati a mano nella scheda.');
    const s = blank();
    const visti = {};
    const sk = new Set();
    let ignorati = 0;
    for (const [nome, v] of Object.entries(campi)) {
      const n = norm(nome);
      if (SK_CASELLE[n]) { if (v === true) sk.add(SK_EN[SK_CASELLE[n]]); continue; }
      const dove = CAMPI[n] || CAMPI[n.replace(/\d+$/, '')];
      const val = typeof v === 'boolean' ? v : testoCampo(v);
      if (!dove || val === '' || val === false) { if (val !== '' && val !== false) ignorati++; continue; }
      if (visti[dove] == null) visti[dove] = val;
    }
    // Abilità scritte per nome (schede italiane: campo o casella con il nome dell'abilità)
    for (const [nome, v] of Object.entries(campi)) {
      if (v !== true) continue;
      const n = norm(nome);
      for (const [en, it] of Object.entries(SK_EN)) if (n.includes(en) || n.includes(norm(it))) sk.add(it);
    }
    const riconosciuti = [];
    const metti = (et, v) => riconosciuti.push([et, String(v)]);
    // Classe e livello
    let clsTesto = visti.classe || '', lvl = parseInt(visti.livello, 10);
    if (visti.classeLivello) { clsTesto = visti.classeLivello.replace(/\d+/g, ' '); const m = visti.classeLivello.match(/\d+/); if (m) lvl = +m[0]; }
    const cls = trova(CLASSES, CLASSI_EN, clsTesto);
    if (cls) { s.cls = cls; metti('Classe', CLASSES[cls].n); }
    if (lvl >= 1 && lvl <= 20) { s.lvl = lvl; metti('Livello', lvl); }
    const race = trova(RACES, RAZZE_EN, visti.razza);
    if (race) { s.race = race; metti('Razza', RACES[race].n); }
    const bg = trova(BGS, BG_EN, visti.bg);
    if (bg) { s.bg = bg; metti('Background', BGS[bg].n); }
    if (visti.allineamento) {
      const a = norm(visti.allineamento);
      const al = ALIGN.find((x) => norm(x) === a) || ALL_EN[a] || '';
      if (al) { s.al = al; metti('Allineamento', al); }
    }
    // Caratteristiche: nella scheda in PDF sono i totali; in Gaster Bless la base è senza i bonus della razza.
    const bonusRazza = {};
    if (race && RACES[race].asi) for (const [k, n] of RACES[race].asi) bonusRazza[k] = (bonusRazza[k] || 0) + n;
    for (const k of AB) {
      const tot = parseInt(visti[k], 10);
      if (tot >= 1 && tot <= 30) { s.base[k] = Math.max(1, tot - (bonusRazza[k] || 0)); metti(ABN[k], tot + (bonusRazza[k] ? ' (base ' + s.base[k] + ' + ' + bonusRazza[k] + ' razziale)' : '')); }
    }
    // Abilità con competenza (quelle date da classe, razza e background Gaster Bless le segna già da sé)
    for (const n of sk) s.sk[n] = true;
    if (sk.size) metti('Competenze nelle abilità', [...sk].join(', '));
    // Testi, monete, equipaggiamento
    for (const k of ['nome', 'giocatore', 'pe', 'tratti', 'ideali', 'legami', 'difetti', 'privNote', 'trattiAgg', 'altreComp', 'eta', 'altezza', 'peso', 'occhi', 'carnagione', 'capelli', 'storia', 'alleati', 'orgNome', 'tesoro']) {
      if (visti[k] != null && visti[k] !== '') { s.f[k] = String(visti[k]); metti(ETICHETTE[k], String(visti[k]).length > 50 ? String(visti[k]).slice(0, 50) + '…' : visti[k]); }
    }
    for (const k of ['MR', 'MA', 'ME', 'MO', 'MP']) if (visti[k] != null && visti[k] !== '') { s.coins[k] = String(parseInt(visti[k], 10) || 0); metti(ETICHETTE[k], s.coins[k]); }
    if (visti.eq) { s.eq = String(visti.eq); metti('Equipaggiamento', s.eq.length > 50 ? s.eq.slice(0, 50) + '…' : s.eq); }
    if (visti.pfAttuali) { s.hpCur = String(parseInt(visti.pfAttuali, 10) || ''); if (s.hpCur) metti('PF attuali', s.hpCur); }
    if (visti.ispirazione === true || /^(si|yes|x|1)$/i.test(String(visti.ispirazione || ''))) { s.insp = true; metti('Ispirazione', 'sì'); }
    return { scheda: s, riconosciuti, ignorati, campi: nomi.length };
  }

  // Nella finestra «Importa»: si può caricare anche un PDF compilabile.
  const fIn = document.getElementById('fJson');
  if (fIn) {
    fIn.setAttribute('accept', '.json,application/json,.pdf,application/pdf');
    const lab = document.getElementById('mIOf');
    if (lab && lab.firstChild && lab.firstChild.nodeType === 3) lab.firstChild.textContent = 'Carica file .json o scheda in PDF';
    const vecchio = fIn.onchange; // i .json li legge scheda.js
    fIn.onchange = async (e) => {
      const f = e.target.files[0];
      if (!f || !/\.pdf$/i.test(f.name) && f.type !== 'application/pdf') { if (vecchio) vecchio.call(fIn, e); return; }
      $('mIOtx').value = '';
      $('mIOp').textContent = 'Lettura della scheda in PDF…';
      try {
        const r = await schedaDaPDF(f);
        $('mIOtx').value = JSON.stringify(r.scheda);
        $('mIOtx').style.display = 'none'; // il testo tecnico della scheda non serve a chi importa un PDF
        $('mIOp').innerHTML = '<b>Dalla scheda in PDF ho riconosciuto:</b><br>' +
          r.riconosciuti.map(([k, v]) => '· ' + esc(k) + ': <b>' + esc(v) + '</b>').join('<br>') +
          (r.ignorati ? '<br><small>' + r.ignorati + ' campi compilati non hanno un posto nella scheda di Gaster Bless (per esempio bonus già calcolati: Gaster Bless li ricalcola da sé).</small>' : '') +
          '<br><small>Premi «Importa» per sostituire la scheda attuale con questa; poi controlla le scelte del personaggio (sottoclasse, abilità, incantesimi).</small>';
      } catch (er) { $('mIOp').textContent = er.message || 'Impossibile leggere il PDF.'; }
    };
  }
  const bImp = document.getElementById('bImp');
  if (bImp) { const vecchioImp = bImp.onclick; bImp.onclick = (e) => { $('mIOtx').style.display = ''; if (vecchioImp) vecchioImp.call(bImp, e); }; }
  window.gasterblessSchedaDaPDF = schedaDaPDF; // per le prove automatiche
})();
