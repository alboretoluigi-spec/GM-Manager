# GASTER BLESS – MASTER SUITE: piano, decisioni e inventario

Prima stesura il 13/11/2026 (nel repository di Barovia, `docs/gm_desktop/01_DECISIONI_E_INVENTARIO.md`, con il nome di
lavoro «GM Desktop»); da qui in avanti il documento di riferimento è questo.

Le indicazioni legali sono un orientamento tecnico, **non una consulenza legale**: prima della vendita serve il parere di
un avvocato esperto di proprietà intellettuale.

## 1. Decisioni del Master
| Data | Decisione |
|---|---|
| 13/11/2026 | Seconda app commerciale, separata da Barovia, con tutte le sue funzioni rese generiche |
| 13/11/2026 | Vendita come **app sul PC del Master con licenza** (nessun abbonamento online) |
| 13/11/2026 | **Barovia resta** la copia personale del Master e non viene modificata da questo progetto |
| 13/11/2026 | Nome: **GM Manager** (prima, come nome di lavoro, «GM Desktop»); repository privato `alboretoluigi-spec/GM-Manager` |
| 13/11/2026 | Icone delle classi, video di benvenuto, immagini di saluto, del «povero» e del Monte Ghakis: **tolti per ora** (si sostituiscono con materiale nuovo) |
| 13/11/2026 | Sistemi: **Windows e macOS** |
| 13/11/2026 | Licenza: **1 PC per licenza, a vita** |
| 13/11/2026 | **Niente gioco da remoto** nella prima versione (MQTT, Cloudflare, VPN restano fuori) |
| 13/11/2026 | **App generica per diversi giochi di ruolo. Nessun contenuto di gioco incluso né scaricato dall'app** (mostri, manuali, ambientazioni, incantesimi, classi, oggetti, nemmeno dall'SRD): l'utente li **importa da file che si procura** o li **inserisce a mano** |
| 13/11/2026 | **Interfaccia adattiva**: all'accesso il Master sceglie **la campagna** (il gioco è quello della campagna); l'app mostra solo pulsanti, sezioni e materiale di quel gioco, **nascondendo del tutto** il resto; i **giocatori seguono** la scelta del Master; nella prima versione quinta edizione, scheda libera **e le altre famiglie** di meccaniche. Analisi e sotto-passi C4a–C4e: `docs/INTERFACCIA_ADATTIVA.md` |
| 13/11/2026 | **Sistema di gioco a scelta fra i più noti**: per ogni campagna il Master sceglie il sistema; l'app ne conosce le meccaniche (dadi, tiri, iniziativa, salute, condizioni, struttura della scheda), sempre senza contenuti né testi dei manuali. Elenco e priorità: proposta al §3.3bis, da confermare |
| 13/11/2026 | **Nome ufficiale: GASTER BLESS – MASTER SUITE**, slogan **MASTER YOUR WORLD**, sottotitolo **THE UNIVERSAL TABLETOP RPG PLATFORM**; stile dell'immagine di riferimento (oro antico su fondo scuro). Cambiano solo nome, logo testuale e intestazione; funzioni, struttura e contenuti restano. Dettagli: `branding/MARCHIO.md` |

### Nota sul nome
Il 13/11/2026 il nome ufficiale è diventato **GASTER BLESS – MASTER SUITE** (prima «GM Manager», nome di lavoro, e prima
ancora «GM Desktop»). Va verificato con l'avvocato nelle banche dati dei marchi (EUIPO, UIBM, USPTO): «Gaster» è anche il
nome di un personaggio noto di un videogioco (W. D. Gaster di *Undertale*). Per «GM Manager» una ricerca sul web aveva
trovato solo nomi vicini (Game Master, RPG Manager).

## 2. Come nasce il codice
1. Il repository è nato **vuoto** e privato.
2. Il codice si copia da Barovia (`04_nuova_app`) **senza la sua storia Git**.
3. Ogni parte entra in Gaster Bless **solo dopo** la pulizia dell'inventario qui sotto: nessun commit deve mai contenere
   materiale da togliere (la storia Git lo conserverebbe).
4. Barovia continua nel suo repository; una correzione utile a entrambe si riporta a mano.

## 3. Inventario: cosa resta, cosa si toglie, cosa si sostituisce

Legenda: **RESTA** (si copia, al massimo con il nome cambiato) · **GENERICO** (resta la funzione, va tolto il contenuto
di Barovia) · **TOGLIERE** · **SOSTITUIRE** (serve materiale nuovo con licenza chiara) · **VERIFICARE** (origine o
licenza da accertare).

### 3.1 Codice e librerie
| Elemento | Esito | Note |
|---|---|---|
| Server Go (`cmd/`, `internal/`, 172 file nel solo `internal/server`) | RESTA | Modulo da rinominare (`barovia` → `gmdesktop`); «Barovia» compare in circa 200 file |
| Dipendenze Go (SQLite modernc, websocket, x/crypto, SDK Anthropic, qr) | RESTA | Licenze permissive (BSD, ISC, MIT); serve l'elenco delle licenze nell'app |
| `web/vendor`: pdf.js, mammoth, DOMPurify, html2canvas, jsPDF, three.js | RESTA | Apache 2.0, BSD-2, MPL/Apache, MIT; file di licenza già presenti |
| Font Cinzel, Pirata One, Source Sans 3 | RESTA | Licenza OFL, file di licenza presenti |
| ffmpeg | RESTA (esterno) | Non è incluso: l'app lo cerca sul PC. Se un giorno si includesse, va rivista la licenza (LGPL/GPL) |

### 3.2 Regole e dati di gioco (rivisto il 13/11: app generica, nessun contenuto incluso)
| Elemento | Esito | Note |
|---|---|---|
| Motore delle regole di quinta edizione (`web/regole/motore.js`: livelli, multiclasse, slot, CD) | GENERICO | Resta come **primo modulo di sistema**; i calcoli sono codice nostro. Lo stesso schema servirà per altri sistemi |
| Dati di classi, sottoclassi, razze, talenti, incantesimi (`web/regole/dati.js`) | TOGLIERE | Nessun dato incluso, **nemmeno dell'SRD** |
| Mostri (`web/carta/mostri.json`, 892 voci, anche le 245 dell'SRD) e bestiario della mappa | TOGLIERE | Il bestiario parte vuoto |
| Manuali, testi OCR, ambientazioni, avventure | TOGLIERE | Nessuno incluso |
| Importazione (file JSON, CSV, Markdown, PDF del Master) e inserimento a mano | GENERICO | Diventano il modo **unico** per avere contenuti: formati documentati e aperti, così l'utente può prepararli o scaricarli da fonti che sceglie lui |
| Script di conversione e traduzione (`strumenti/*.py`, `strumenti/dati/`) | TOGLIERE | Strumenti di lavoro di Barovia |
| Effetti automatici di incantesimi, oggetti magici, attacchi | GENERICO | Restano come meccaniche; funzionano sui dati importati dall'utente |

### 3.3 Più sistemi di gioco
| Elemento | Esito | Note |
|---|---|---|
| Scheda del personaggio | GENERICO | Scheda per sistema di gioco; oltre alla quinta edizione, una scheda libera (campi definiti dall'utente) per gli altri giochi |
| Tracker dell'iniziativa, dadi, condizioni | GENERICO | Regole di iniziativa e dadi scelte dal sistema (es. d20, pool di d6, d100) |
| Sanità mentale, terrore, indebolimento | GENERICO | Moduli facoltativi, attivabili per campagna |

### 3.3bis Sistemi di gioco (proposta del 13/11/2026, da confermare)
**Come funziona.** Un «modulo di sistema» descrive solo le meccaniche: tipo di dadi e di tiro (d20 + modificatore,
percentuale d100, riserve di d6 o d10, dadi esplosivi, dadi Fate, 2d6 + modificatore), caratteristiche e abilità come
*campi* (senza descrizioni dei manuali), iniziativa, salute e condizioni, avanzamento, campi della scheda. I contenuti
(classi, incantesimi, mostri…) restano all'utente. Il Master può anche creare o importare un modulo suo.

**Famiglie di meccaniche** (un motore per famiglia copre più giochi):
| Famiglia | Esempi di giochi |
|---|---|
| d20 + modificatore contro una difficoltà | quinta edizione 2014 e 2024, Pathfinder 1ª e 2ª edizione, Starfinder, giochi OSR |
| Percentuale d100 sotto l'abilità | Il Richiamo di Cthulhu 7ª edizione, Warhammer Fantasy Roleplay 4ª, RuneQuest, Pendragon |
| Riserva di d10 con successi | Vampiri (V5) e Mondo di Tenebra |
| Riserva di d6 con successi | Shadowrun 6ª, Year Zero Engine (Alien, Vaesen), Blades in the Dark |
| 2d6 + modificatore a fasce | Powered by the Apocalypse (Dungeon World, Monster of the Week), Traveller |
| d10 + caratteristica + abilità | Cyberpunk RED |
| Dadi a passi ed esplosivi | Savage Worlds |
| Dadi Fate (dF) | Fate Core, Fate Accelerato |
| 3d6 sotto il valore | GURPS |
| d20 sotto il valore | Dragonbane, Mörk Borg |
| Dadi narrativi con simboli | Genesys / Star Wars (FFG) |
| Giochi italiani | Sine Requie (tarocchi), Lex Arcana |
| Scheda libera | qualsiasi altro gioco: campi e tiri definiti dal Master |

**Nomi dei giochi.** I nomi sono marchi dei rispettivi editori: nell'app si potrebbero usare solo come «compatibile
con…», se e come l'avvocato lo conferma (alcuni editori hanno licenze di compatibilità: Paizo ORC, Chaosium, Free
League, Paradox/Dark Pack…). Fino ad allora i moduli hanno nomi descrittivi («d20 fantasy 5ª edizione»,
«percentuale d100 orrore»…).

### 3.4 Mappa
| Elemento | Esito | Note |
|---|---|---|
| Funzioni della mappa (nebbia, percorsi, terreni, viaggio, segnalino, proiezione) | GENERICO | Restano tutte, su una mappa caricata dal Master |
| `barovia.webp`, `mask.png`, `locs.json`, `terreno.json`, 9 immagini dei luoghi (`img/lm/`) | TOGLIERE | Mappa e luoghi di Barovia, provenienti dalla vecchia app, di origine non nostra |
| Mappa di esempio | SOSTITUIRE | Una mappa dimostrativa originale, fatta o comprata con licenza commerciale |
| Regole del Monte Ghakis (`ghakis.go`, `web/ui/ghakis.js`, immagini) | GENERICO | Diventano «regole di zona» configurabili (freddo, caduta, tempo atmosferico) |

### 3.5 Immagini, video e suoni
| Elemento | Esito | Note |
|---|---|---|
| Icone delle classi (`web/img/classi/`, 26 SVG) | VERIFICARE | Fornite dall'utente da archivi zip: origine e licenza sconosciute. Se non c'è licenza commerciale, ridisegnarle |
| `benvenuto.mp4` («Bentornato in BAROVIA»), `saluto.webp`, `povero.webp`, immagini del Ghakis | VERIFICARE | Materiale del Master: va accertato chi li ha creati e con quali diritti |
| Suoni `corvo.mp3`, `attesa.mp3` | SOSTITUIRE | Estratti dal vecchio tracker, origine sconosciuta; sostituirli con suoni CC0 o comprati |
| `internal/server/dati/barovia.jpg` | TOGLIERE | Immagine di Barovia |
| Icone dell'app (`icona-*.png`) | SOSTITUIRE | Icone nuove con il marchio Gaster Bless (`branding/MARCHIO.md`) |

### 3.6 Contenuti della campagna e funzioni legate a Curse of Strahd
| Elemento | Esito | Note |
|---|---|---|
| Codice della campagna (funzione) | GENERICO | Diventa un compendio con più campagne, importazione ed esportazione in pacchetti |
| `docs/lore/*` (Orfanotrofio, Strahd, Villaggio, Ismark) | TOGLIERE | Campagna del Master |
| Master Campaign Bible (`prompt_assistente.md`) | SOSTITUIRE | Istruzioni neutre per un assistente del Master, più modelli d'esempio scritti da noi |
| `prompt_strahd.md`, `assistente_strahd.go` («Parla con Strahd») | GENERICO | «Parla con un PNG»: il Master sceglie il personaggio e le sue istruzioni |
| `prompt_canone.md` e identità «canone» (biblioteca dei romanzi e del manuale) | GENERICO | La ricerca nei testi caricati dal Master resta; nessun testo di terzi incluso, nessun riferimento a Curse of Strahd |
| Tarocchi di Madame Eva (`tarocchi.go`, catalogo di 54 carte, scene) | GENERICO | Diventa «mazzo di carte» generico: immagini, nomi e significati caricati dal Master. Il catalogo Tarokka si toglie |
| Estrazione dei testi dal manuale (`tarocchi_ocr.go`, `manuali_ocr.go`) | TOGLIERE | Legata al manuale di Curse of Strahd; nel prodotto non si estraggono testi da manuali di terzi |
| Bilanciamento (`bilanciamento.go`) | RESTA | Motore generico della potenza del gruppo |
| Modello «Orfanotrofio di Vallaki» (`bilanciamento_orfanotrofio.go`) | TOGLIERE | Diventa il formato per «modelli di scenario» da caricare; si può fornire un esempio originale |
| PNG in scena con le raccolte «Campagna ufficiale» e «CoS Reloaded» | GENERICO | Raccolte con nomi scelti dal Master |
| Manuali in PDF del Master (lettura, pagine, modifiche) | RESTA | Il Master carica i propri file; l'app non ne distribuisce |

### 3.7 Integrazioni, rete e installazione
| Elemento | Esito | Note |
|---|---|---|
| Arcana (`arcana*.go`, `kit-scheda/`, `guide/arcana.md`) | TOLTO | Accordo con un'altra app, non trasferibile. Il 13/11 (decisione del Master: «Non includere Arcana in GasterBless») tolti anche i resti: pulsante e finestra «scheda esterna» nelle Schede, etichette in Iniziativa e nella scheda di combattimento, impostazione `origini_arcana`, migrazioni 009 e 011 (ora vuote), stile; parole aggiunte a `docs/parole_vietate.txt` |
| Cloudflare, VPN, MQTT da remoto | VERIFICARE | La funzione «gioco da remoto» va riprogettata per un cliente qualsiasi (proposta nella fase 4) |
| Certificato HTTPS da accettare su ogni dispositivo | SOSTITUIRE | Ostacolo principale per un cliente; soluzione da scegliere nella fase 4 |
| Installazione e aggiornamenti con PowerShell (`strumenti/*.ps1`, `portatile/`) | SOSTITUIRE | Installatore firmato e aggiornamento automatico verificato |
| Criteri del browser per il tutto schermo (comandi PowerShell) | GENERICO | Da semplificare o spiegare nell'installatore |
| Guide (`guide/`, PDF di Barovia) | SOSTITUIRE | Da riscrivere per Gaster Bless, in italiano e inglese |

### 3.8 Da aggiungere (non esiste in Barovia)
- Attivazione della licenza (chiave, verifica offline, più PC per licenza da decidere).
- Installatore firmato per Windows; macOS da decidere.
- Aggiornamento automatico con pacchetti firmati.
- Lingue: italiano e inglese.
- Primo avvio guidato (creazione del Master, primo gruppo, prima mappa).
- Esportazione e importazione delle campagne; backup.
- Pagina delle licenze e delle attribuzioni (SRD 5.1, librerie, font).
- Informativa sulla privacy e condizioni d'uso; consenso dei genitori per i minori.
- Chiave dell'IA del cliente o modello locale (Ollama), con l'avviso su quali dati vanno al fornitore.
- Segnalazione degli errori, solo con il consenso.

## 4. Cosa serve fuori dal codice
| Voce | Chi | Costo indicativo (da verificare) |
|---|---|---|
| Parere sulla proprietà intellettuale e sulle condizioni d'uso | Avvocato | da qualche centinaio a circa 2.000 € |
| Partita IVA, IVA sulle vendite digitali nell'UE | Commercialista | secondo il professionista |
| Piattaforma di vendita che fa da rivenditore e gestisce l'IVA | Paddle, Lemon Squeezy o simili | circa il 5% di ogni vendita |
| Certificato per firmare l'installatore | Fornitore di certificati | da circa 10 € al mese a qualche centinaio di euro l'anno |
| Dominio | Registrar | circa 15 € l'anno |
| Marchio UE (una classe), se si decide di registrarlo | EUIPO | circa 850 € |
| Logo, mappa d'esempio, suoni | Grafico o acquisto con licenza | variabile |
| Master che provano la versione di prova | Il Master | — |

## 5. Fasi del progetto
| Fase | Contenuto | Stato |
|---|---|---|
| 1 | Decisioni (nome, modello di vendita, Barovia separata) | **fatta** (13/11) |
| 2 | Inventario dei contenuti e delle licenze (§3) | **fatta** (13/11) |
| 3a | Repository e struttura di base: istruzioni, stato del lavoro, piano, regole sui contenuti | **fatta** (13/11) |
| 3b | Copia ripulita del codice: server, pagine, prove; nome cambiato (`barovia` → `gasterbless`) e marchio applicato a tutte le schermate (`branding/MARCHIO.md`); nessun contenuto di Barovia | **fatta** (13/11): vedi §5.1 |
| 3c | Nessun dato di gioco incluso: bestiario, regole e manuali partono vuoti; importazione documentata (formati aperti) e inserimento a mano; motore della quinta edizione come primo «modulo di sistema» | **in corso** (13/11): C1, C2 e C3 fatti, vedi §5.2 |
| 3d | Compilazione e prove automatiche verdi (le 19 rosse del §5.1, nuove prove nel browser); elenco delle licenze nell'app | **prove verdi** (13/11): mondo di prova inventato in `app/internal/server/testdata`; manca l'elenco delle licenze nell'app |
| 4 | Funzioni generiche (mappa, compendio, «Parla con un PNG», mazzo di carte, regole di zona, modelli di scenario) e rete per un cliente qualsiasi | — |
| 5 | Installatore firmato, licenza, aggiornamenti, lingue (italiano e inglese) | — |
| 6 | Versione di prova privata, poi il lancio | — |

### 5.1 Esito della fase 3b (13/11/2026)
- Il programma è in `app/` (modulo Go `gasterbless`, eseguibile `cmd/gasterbless`). Compila, `go vet` pulito.
- **Tolti**: tarocchi della veggente, regole del Monte (Gelo/Ghakis), integrazione Arcana, modello di bilanciamento
  dell'orfanotrofio, mappa, luoghi e terreni di Barovia, guide di Barovia, kit della scheda esterna, prove nel browser
  e prove delle regole (dipendevano dai dati), script Python di conversione, icone delle classi, suoni e immagini di
  origine incerta.
- **Svuotati** (struttura e nomi restano, contenuto vuoto): classi, sottoclassi, razze, background, incantesimi, talenti,
  strumenti, equipaggiamento, difese per razza/classe, professioni, bestiario (`mostri.json`, `bestiario.json`),
  luoghi (`locs.json`) e terreni (`terreno.json`) della mappa. La scheda si apre ma senza classi né razze: si
  riempirà con l'importazione (fase 3c).
- **Resi generici**: «Parla con Strahd» → **«Parla con l'antagonista»** (istruzioni predefinite neutre, modificabili dal
  Master); istruzioni dell'assistente neutre (non più la campagna di Barovia); «Archivista del canone» → **«Archivista
  delle fonti»**. Nei testi e nelle prove i nomi di luoghi e personaggi sono stati sostituiti con nomi inventati.
- **Rifatti con materiale nostro**: icone dell'app «GB», video di benvenuto e immagine di saluto con il marchio,
  immagine «borsa vuota», simbolo di stanchezza, suoni del turno e della musica d'attesa (sintesi), immagine
  provvisoria della mappa, guide PDF provvisorie con copertina del marchio.
- **Marchio applicato**: titoli delle pagine, manifest, icone, intestazione completa (accesso, caricamento, mappa,
  proiettore), logo breve nel menu e nella barra della mappa (`app/web/stile/marchio.css`, `app/web/ui/marchio.js`).
- **Controllo automatico** delle parole vietate: `app/internal/controlli/parole_test.go` con l'elenco
  `docs/parole_vietate.txt` (passa).
- Provato nel browser: accesso, creazione del Master, menu, mappa, scheda, sessioni, impostazioni senza errori.
- **Prove del server ancora rosse: 19** (tutte usano dati che non ci sono più): TestBestiarioImmaginiMostrateAiGiocatori, TestCodiceNellAssistente, TestImportazioneManuali, TestImportazioneNomeDallIntestazione, TestManualiSoloMaster, TestMappaConsultazione, TestMappaProiezioneSvelaAiGiocatori, TestMappaVisibilitaPerGiocatore, TestMappeLocaliSoloSeSvelate, TestMigrazioneDaGasterBlessDati, TestMusicaECanti, TestPreparazioneSessione, TestProfessioni, TestPromemoriaDelCodice, TestRaccoltaPNG, TestSchedePlugin, TestSezionePNGeMostri, TestStradeSulServer, TestViaggi.
  Si sistemano nella fase 3d con dati di prova creati dentro le prove stesse, oppure si riscrivono quando mappa,
  bestiario e professioni diventano generici (fasi 3c e 4).
- Da fare ancora: `web/regole/difese.js` contiene due regole legate a sottoclassi con il nome (3c); gioco da remoto
  (MQTT, Cloudflare, VPN) ancora nel codice ma spento (si toglie nella fase 4, decisione «niente gioco da remoto»);
  script PowerShell di `app/strumenti/` ereditati (si rifanno nella fase 5).

### 5.2 Piano della fase 3c (13/11/2026)
| Passo | Contenuto | Stato |
|---|---|---|
| **C1** | **Pacchetti di contenuti**: formato aperto e documentato «gasterbless-contenuti» (`docs/FORMATO_CONTENUTI.md`) per classi, sottoclassi, razze, sottorazze, background, incantesimi, armi, armature, talenti, stili; controllo del file con errori precisi; anteprima, importazione, attivazione, esportazione, eliminazione; i pacchetti attivi arrivano alla scheda della quinta edizione; modello con contenuti inventati | **fatto** (13/11) |
| **C2** | **Inserimento a mano**: pacchetti personali (anche vuoti, migrazione 042); moduli nella pagina «Contenuti di gioco» per creare, modificare, copiare ed eliminare ogni tipo di voce (classe con privilegi e sottoclassi, razza con tratti e sottorazze, background, incantesimo, arma, armatura, talento, stile) e i dati del pacchetto; anche sui pacchetti importati, senza perdere i campi non mostrati; stesso controllo dell'importazione | **fatto** (13/11) |
| **C3** | **Altri contenuti nei pacchetti**: incantesimi concessi da sottoclassi, razze e terreni; strumenti; prezzi e categorie dell'equipaggiamento; mostri (catalogo del bestiario, solo al Master); oggetti magici (nell'archivio, «dal pacchetto»); professioni (catalogo del server). I cataloghi del server si rileggono quando cambiano i pacchetti. Editor per strumenti, equipaggiamento, mostri e incantesimi concessi | **fatto** (13/11) |
| C4 | **in corso: C4a, C4b (13/11), C4c e C4d (14/11) fatti**. **Modulo di sistema e interfaccia adattiva** (richiesta del Master del 13/11, sotto): il Master sceglie il gioco quando entra nell'app; l'interfaccia mostra solo le sezioni e i pulsanti di quel gioco e nasconde il resto; scheda, dadi e iniziativa seguono il gioco; la quinta edizione è il primo modulo; **scheda libera** (campi e tiri definiti dal Master) per tutti gli altri giochi | da fare |
| C5 | **Pulizia**: regole legate a sottoclassi con il nome in `web/regole/difese.js`, magie innate scritte nel codice della scheda (diventano dati dei pacchetti); **nomi di manuali di editori ancora visibili nell'app** («Manuale del Giocatore», «Manuale dei Mostri», «Guida del Dungeon Master», «La maledizione dell'antagonista»: in `web/scheda/index.html`, `web/ui/professioni.js`, `web/sezioni/manuali.js`, `bestiario.js`, `impostazioni_sanita.js`, `web/carta/carta.js`, commenti di `web/ui/manuale_*.js`, `internal/server/professioni.go`) da togliere e aggiungere a `docs/parole_vietate.txt`; guida aggiornata, prove | da fare |

**Richiesta del Master (13/11/2026) per C4**: «Gaster Bless cambia l'interfaccia adattandola di volta in volta al gdr
scelto dal Master. Nasconde pulsanti e materiale non presenti nel gioco scelto e abilita pulsanti e funzioni riguardanti
esclusivamente il gioco scelto. Il Master seleziona il GDR quando fa accesso all'app.»

I moduli per le altre famiglie di meccaniche (§3.3bis) vengono dopo C4, nell'ordine che sceglie il Master (domanda
aperta n. 1 del §6).

### Metodo per la fase 3b
La copia si prepara **fuori dal repository** (cartella di lavoro temporanea), si ripulisce, si compila e si prova; solo
quando è pulita entra con un commit. Un controllo automatico (da scrivere nella 3b) cerca nel repository le parole
vietate (Barovia, Strahd, Ravenloft, Vallaki, Tarokka, Madame Eva, Curse of Strahd, Dungeons & Dragons…) e fa fallire
le prove se ne trova.

## 6. Domande aperte
Le quattro domande del 13/11 hanno avuto risposta (vedi §1). Restano:
1. **Sistemi di gioco nella prima versione** (13/11: il Master vuole poterli scegliere fra i più noti): quali famiglie
   del §3.3bis entrano nella prima versione e in che ordine.
5. **Nomi dei giochi nell'app**: verifica con l'avvocato delle licenze di compatibilità dei singoli editori.
2. Accesso dei giocatori da Internet (fuori casa) nella prima versione, oppure solo dalla rete di casa?
3. Lingua di partenza: italiano e inglese insieme, oppure prima l'italiano?
4. Prezzo e piattaforma di vendita (dopo il parere del commercialista).
