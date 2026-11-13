# GM Manager: piano, decisioni e inventario

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

### Nota sul nome
Una ricerca sul web (13/11/2026) non ha trovato un'app chiamata esattamente «GM Manager», ma il settore ha nomi vicini:
«Game Master» su Steam, «Game Master 5th Edition» per iPad, «RPG Manager» (estensione di Obsidian). Il nome è
**descrittivo**: facile da capire, difficile da proteggere come marchio. Prima di registrare dominio e marchio serve una
ricerca nelle banche dati EUIPO e UIBM (e USPTO per gli Stati Uniti), con l'avvocato.

## 2. Come nasce il codice
1. Il repository è nato **vuoto** e privato.
2. Il codice si copia da Barovia (`04_nuova_app`) **senza la sua storia Git**.
3. Ogni parte entra in GM Manager **solo dopo** la pulizia dell'inventario qui sotto: nessun commit deve mai contenere
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

### 3.2 Regole di quinta edizione
| Elemento | Esito | Note |
|---|---|---|
| Motore delle regole (`web/regole/motore.js`: livelli, multiclasse, slot, CD) | RESTA | Le meccaniche si possono usare; il codice è nostro |
| Dati di classi, sottoclassi, razze, talenti (`web/regole/dati.js`) | GENERICO | Oggi includono contenuti di PHB fuori dall'SRD, Xanathar (circa 120 voci), Tasha (circa 50), Volo, Mordenkainen, Van Richten. Nella versione in vendita: **solo l'SRD 5.1** (CC-BY-4.0, regole 2014) con l'attribuzione |
| Circa 470 incantesimi | GENERICO | Restano quelli dell'SRD 5.1; gli altri si tolgono |
| Contenuti oltre l'SRD | SOSTITUIRE | Funzione «importa contenuti» per il Master (file suoi, non distribuiti da noi) |
| Effetti automatici degli incantesimi, oggetti magici, attacchi | RESTA | È codice nostro; i dati seguono la regola dell'SRD |

### 3.3 Mostri
| Elemento | Esito | Note |
|---|---|---|
| `web/carta/mostri.json`: 892 creature | GENERICO | **245 dall'SRD restano**; le **647 non SRD** (anche solo valori e nomi dei tratti, con il riferimento al manuale) si tolgono |
| `web/carta/bestiario.json` (bestiario della mappa) | VERIFICARE | Dichiara materiale SRD adattato; vanno controllate le voci di Curse of Strahd |
| Script di conversione e traduzione (`strumenti/*.py`, `strumenti/dati/`) | TOGLIERE | Strumenti di lavoro, non vanno nel prodotto |

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
| Icone dell'app (`icona-*.png`) | SOSTITUIRE | Nuovo logo di GM Manager |

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
| Arcana (`arcana*.go`, `kit-scheda/`, `guide/arcana.md`) | TOGLIERE | Accordo con un'altra app, non trasferibile |
| Cloudflare, VPN, MQTT da remoto | VERIFICARE | La funzione «gioco da remoto» va riprogettata per un cliente qualsiasi (proposta nella fase 4) |
| Certificato HTTPS da accettare su ogni dispositivo | SOSTITUIRE | Ostacolo principale per un cliente; soluzione da scegliere nella fase 4 |
| Installazione e aggiornamenti con PowerShell (`strumenti/*.ps1`, `portatile/`) | SOSTITUIRE | Installatore firmato e aggiornamento automatico verificato |
| Criteri del browser per il tutto schermo (comandi PowerShell) | GENERICO | Da semplificare o spiegare nell'installatore |
| Guide (`guide/`, PDF di Barovia) | SOSTITUIRE | Da riscrivere per GM Manager, in italiano e inglese |

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
| 3b | Copia ripulita del codice: server, pagine, prove; nome cambiato (`barovia` → `gmmanager`); nessun contenuto di Barovia | da fare |
| 3c | Dati di gioco solo SRD 5.1 (classi, sottoclassi, razze, incantesimi, mostri) con l'attribuzione | da fare |
| 3d | Compilazione e prove automatiche verdi; elenco delle licenze nell'app | da fare |
| 4 | Funzioni generiche (mappa, compendio, «Parla con un PNG», mazzo di carte, regole di zona, modelli di scenario) e rete per un cliente qualsiasi | — |
| 5 | Installatore firmato, licenza, aggiornamenti, lingue (italiano e inglese) | — |
| 6 | Versione di prova privata, poi il lancio | — |

### Metodo per la fase 3b
La copia si prepara **fuori dal repository** (cartella di lavoro temporanea), si ripulisce, si compila e si prova; solo
quando è pulita entra con un commit. Un controllo automatico (da scrivere nella 3b) cerca nel repository le parole
vietate (Barovia, Strahd, Ravenloft, Vallaki, Tarokka, Madame Eva, Curse of Strahd, Dungeons & Dragons…) e fa fallire
le prove se ne trova.

## 6. Domande aperte
Le quattro domande del 13/11 hanno avuto risposta (vedi §1). Restano:
1. Accesso dei giocatori da fuori casa (Internet) nella prima versione: solo rete di casa, oppure anche da Internet?
2. Prezzo e piattaforma di vendita (dopo il parere del commercialista).
3. Lingua di partenza: italiano e inglese insieme, oppure prima l'italiano.
