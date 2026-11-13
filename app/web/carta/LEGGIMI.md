# Mappa di Gaster Bless nella nuova app

> Nota (13/11/2026): in GASTER BLESS – MASTER SUITE i dati di gioco sono stati tolti (nessun contenuto incluso) e i
> riferimenti alle vecchie cartelle di origine non valgono più: questa descrizione resta come guida alla struttura.

Copia della pagina originale (`02_mappa_originale/estratto_da_exe/web/index.html`, che resta in sola
lettura), con stile e script separati (`carta.css`, `carta.js`) perché valga la Content-Security-Policy.
Catalogo dei mostri: `mostri.json`, generato da `strumenti/converti_mostri.py` (solo per il Master).
Dati e immagini copiati: `locs.json`, `bestiario.json`, `img/gasterbless.webp`, `img/mask.png`, `img/lm/`.
Si apre dalla sezione Mappa (`web/sezioni/mappa.js`) come `/carta/?sessione=<id>`.

## Modifiche rispetto all'originale
| Dove | Modifica |
|---|---|
| `api()`, caricamenti, eventi | Le chiamate `/api/x` vanno a `/api/v1/mappa/x?sessione=<id>` con il token anti-CSRF; gli errori del nuovo server (`errore`) vengono mostrati. |
| `boot()` | Niente password né link d'invito: il ruolo arriva dall'account (Master o giocatore). |
| `showSessions()` | Gruppi e sessioni si gestiscono nella sezione Sessioni dell'app. |
| `openTracker()` | Il tracker è la sezione Iniziativa dell'app. |
| Proiezione | La finestra si apre su `/carta/?sessione=<id>#proj`. Proiezione pulita (2026-11-12): `proiettore.js` (nuovo) disegna mappa, immagini e video con WebGL (ingrandimento bicubico, nitidezza adattiva, luminosità/ombre/contrasto/colori/calore), regolazioni nel pannello «🎛 Regolazioni del proiettore» della finestra «📺 Proiezione» e schermata di prova; la finestra si apre sul proiettore (permesso «gestione delle finestre» chiesto dalla mappa), non si riapre se è già aperta, Invio la avvia; nella finestra di proiezione niente avvisi, puntatore nascosto, video dei canti senza barra, schermo sempre acceso, apertura diretta sul secondo schermo se il browser ha il permesso. Corretto l'id doppio `pjs` (i segni delle località non comparivano sulla mappa proiettata). |
| PDF.js | Una sola copia, in `/vendor/pdfjs/`. |
| `carta.css` | Nascosti "Invita giocatori", "Esci", la scelta della sessione e la password Master (sono nell'app). |
| Service worker | Quello dell'app. |
| Bestiario, manuali, musica, canti | Collegati al nuovo server (`mappa_bestiario.go`, `mappa_musica.go`). |
| Segnalino del gruppo | Nuovo: `drawParty()`, `partyDrag()`, `partyMenu()` e pulsante "📍 Posiziona il gruppo". Si trascina (Master e giocatori); tasto destro o tocco lungo: dividi il gruppo, riunisci, personalizza (nome, colore, simbolo: solo Master), togli. Server: `internal/server/mappa_segnalino.go` (`POST /api/v1/mappa/party`). |
| Correzioni | B5 (indice della galleria), B9 (voce del bestiario eliminata), B7 (`saveFog()` manda la base, il server unisce). |
| Immagine della mappa | Il Master carica `img/gasterbless.webp`; il giocatore l'immagine coperta dalla nebbia (`bgImg()`, `GET /api/v1/mappa/immagine`). `gasterbless.webp` e `mask.png` sono serviti solo al Master. Canti con video: `songChoose()`, `songVideo()`. |
| Prestazioni | P1 (`snap()` e `astar()` chiedono al server, `POST /api/v1/mappa/strada`; niente più `img/mask.png` nella pagina), P2 (`fogOn()` usa il timbro `FST`), P3 (`refreshOnce()` non ridisegna se lo stato è uguale, la nebbia solo se cambiata). |
| Terreno (2026-11-07) | `terreno.json`: zone di pianura, collina, montagna e acqua lette sulla mappa (il resto è foresta). `pezzi()` divide ogni tratto fuori strada per terreno; `legH()` usa metà velocità sul terreno difficile; `avvisoTerreno()` scrive in cima al pannello perché, quanto e su quale tappa; il tracciato fuori strada è arancio (difficile) o azzurro (pianura). |

Il server corrispondente è `internal/server/mappa.go` (formato delle risposte uguale all'originale).
