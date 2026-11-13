# RPG RULEBOOK LIBRARY e MY RULEBOOKS: analisi e piano tecnico

Richiesta del Master del 13/11/2026 («Implementazione della libreria universale dei regolamenti»). Questo documento è
la parte «prima presenta» della richiesta: analisi, piano, componenti, dipendenze, decisioni (§5) e avanzamento (§7).

## 1. Analisi dell'architettura esistente (cartella `app/`)

| Aspetto | Com'è oggi |
|---|---|
| Linguaggi | Go 1.27 (server), JavaScript «vanilla» a moduli ES (pagine), HTML/CSS; nessun framework e nessun passaggio di build per le pagine |
| Server | un solo eseguibile (`cmd/gasterbless`), `net/http` con rotte `/api/v1/...` (`api()` restituisce JSON; `richiedi(r, "master")` per i permessi), tempo reale con WebSocket (`Hub.Pubblica`, `AlMaster`, `AUtente`) |
| Database | SQLite (modernc, senza CGO) in `Dati\`, 37 migrazioni in `internal/db/schema` con il loro rollback; dati sensibili cifrati (`internal/cripto`) |
| File | cartella dati dell'installazione: `documenti/`, `biblioteca/`, `manuali` (PDF), media; scrittura atomica (`scriviAtomico`); file riconosciuti dal contenuto (`tipoDocumento`), HTML pulito con DOMPurify |
| Pagine | `web/` incorporata nell'eseguibile (`web/embed.go`), service worker, menu a gruppi in `web/ui/app.js`, sezioni in `web/sezioni/`, componenti in `web/ui/` |
| Account | Master e giocatori; il Master è uno per installazione. «Campagna» oggi = l'installazione: ci sono **gruppi** e **sessioni**, non un'entità campagna |
| Marchio | `web/stile/marchio.css`, `web/ui/marchio.js` (fase 3b) |

### Moduli già esistenti da riusare (non si riscrivono)
| Modulo | Cosa offre | Uso nel nuovo modulo |
|---|---|---|
| `biblioteca.go`, `biblioteca_testi.go` | lettura di EPUB, PDF (testo dal browser), DOCX, TXT, MD, HTML, testo OCR; divisione in capitoli/pagine e brani; ricerca per parole con indice | estrazione del testo e **ricerca nel contenuto** dei documenti di MY RULEBOOKS; base per l'analisi futura |
| `manuali.go`, `manuali_pagine.go`, `manuali_ocr.go`, pdf.js in `web/vendor` | caricamento e visualizzazione dei PDF, pagine per i telefoni, testo OCR | **visualizzazione** dei PDF di MY RULEBOOKS |
| `documenti.go` | caricamento, riconoscimento del tipo dal contenuto, sostituzione, rinomina, eliminazione | modello per i controlli di sicurezza sui file importati |
| `importazione.go`, tabelle `elementi_manuali`, `importazioni` | candidati letti dai manuali con pagina di provenienza, confronto e **approvazione del Master** | modello per le **proposte di regole** da approvare (analisi intelligente) |
| `assistente.go` (fornitori IA facoltativi) | collegamento a Ollama, servizi gratuiti, Claude | punto di aggancio, facoltativo, per l'analisi con IA (fase futura) |

### Conflitti e dipendenze individuati
1. **Nomi dei giochi.** La richiesta elenca i giochi con il loro nome commerciale (per esempio «Dungeons & Dragons 5e»).
   Le regole attuali (`CLAUDE.md`, prova `internal/controlli/parole_test.go`) vietano nel prodotto alcuni di questi
   marchi. Serve una decisione (§5, punto 1).
2. **Nessun contenuto incluso** (decisione del 13/11): resta valida. Il catalogo contiene solo *riferimenti* (titolo,
   editore, licenza, indirizzo ufficiale), mai i documenti. Il download parte solo su richiesta del Master.
3. **Campagna.** Oggi non esiste un'entità «campagna» (solo gruppi). Per associare sistema, regolamento e documenti a
   più campagne serve una tabella nuova, collegata ai gruppi senza cambiare i dati esistenti (§3, L4).
4. **«Manuali» e «Biblioteca» esistono già**, con file propri. MY RULEBOOKS non li sostituisce: li riusa (un documento
   può essere «usato anche dall'assistente» o «aperto come manuale») senza copiare due volte lo stesso file.
5. **Rete.** Il server oggi chiama l'esterno solo per l'IA. I download aggiungono chiamate verso siti di editori: solo
   HTTPS, solo verso il dominio registrato nella voce del catalogo, solo su richiesta del Master.

## 2. Principi
- La libreria documentale è **indipendente dal motore** di gioco: nessuna tabella o rotta della libreria conosce
  attributi, classi o meccaniche di un sistema.
- Il catalogo è un **file di dati** separato dal codice (aggiornabile senza ricompilare), con la data dell'ultima
  verifica di ogni voce. Ogni voce di documento gratuito viene **verificata sulla fonte ufficiale** prima di entrare;
  se un gioco non ha materiale gratuito verificato, il catalogo mostra solo il collegamento al sito dell'editore.
- Download diretto **solo** se la voce è segnata «download consentito» dopo la verifica di licenza e condizioni; mai
  per pagine che richiedono accesso, pagamento o restrizioni geografiche: lì solo il collegamento (modalità B).
- Ogni regola estratta da un documento resta una **proposta** finché il Master non la approva.
- Nessuna dipendenza nuova obbligatoria; l'IA resta facoltativa.

## 3. Piano di implementazione (incrementale, ogni passo provato prima del successivo)

| Passo | Contenuto | Componenti |
|---|---|---|
| **L1 – Catalogo** | file `catalogo_regolamenti.json` (incorporato, con possibile aggiornamento locale), giochi con genere, editore, edizioni, sito ufficiale; documenti con tutti i campi richiesti (titolo, gioco ed edizione, editore, lingua, tipologia, formato, URL ufficiale, licenza, condizioni, data di verifica, download diretto sì/no). API `GET /api/v1/regolamenti/catalogo` con filtri (nome, genere, editore, lingua, tipologia, gratuito, edizione). Pagina **RPG RULEBOOK LIBRARY** con filtri e «Apri la fonte ufficiale» | nuovi: `regolamenti_catalogo.go`, `catalogo_regolamenti.json`, `web/sezioni/regolamenti.js`; modificati: `server.go` (una riga di rotte), `web/ui/app.js` (voce di menu), `web/stile/` |
| **L2 – Download** | il Master preme «Scarica nella mia libreria» solo sulle voci consentite: il server scarica in un file temporaneo (HTTPS, dominio della voce, limite di dimensione, tempo massimo, ripresa con `Range` dopo un'interruzione, nuovo tentativo), controlla il tipo dal contenuto e l'impronta, poi salva in MY RULEBOOKS con licenza e fonte. Avanzamento in tempo reale | nuovo: `regolamenti_scarica.go` |
| **L3 – MY RULEBOOKS** | archivio personale: caricamento di PDF, DOCX, TXT, RTF, EPUB, MD, HTML (RTF: lettore minimo nuovo); controllo del contenuto, file corrotti e formati non supportati rifiutati con un messaggio chiaro, **doppioni** riconosciuti dall'impronta SHA-256; gioco, edizione, tipologia, lingua, **categorie personalizzate**, **versioni** dello stesso regolamento, sostituzione ed eliminazione; ricerca per titolo e contenuto (riuso dell'indice della biblioteca); visualizzazione (pdf.js per i PDF, testo impaginato per gli altri) | nuove tabelle (migrazione 038): `regolamenti_documenti`, `regolamenti_categorie`; nuovi: `regolamenti_miei.go`, `regolamenti_rtf.go`, `web/sezioni/miei_regolamenti.js`; riuso di `biblioteca_testi.go` |
| **L4 – Campagne** | entità **campagna**: sistema di gioco, edizione, regolamento principale, documenti supplementari, schede personalizzate, regole aggiuntive del Master, impostazioni (JSON libero). Un documento può stare in più campagne. I gruppi esistenti vanno in una «Campagna principale» creata dalla migrazione (nessun dato perso) | migrazione 039: `campagne`, `campagna_documenti`, colonna facoltativa `gruppi.campagna_id`; nuovo `campagne.go`; pagina di impostazione della campagna |
| **L5 – Analisi (architettura)** | interfaccia Go `analizzatoreRegole` (nessuna IA obbligatoria): produce **proposte** nelle 13 categorie richieste (attributi, abilità, classi/archetipi, origini, equipaggiamento, armi e armature, magie, combattimento, iniziativa, condizioni, progressione, creazione del personaggio, regole speciali) con documento e pagina di provenienza; il Master le vede, corregge, **approva** o rifiuta. Prima versione: inserimento manuale delle proposte e segnaposto per l'analizzatore IA | migrazione 040: `regole_proposte`; nuovi: `regolamenti_analisi.go`, pannello di revisione |
| **L6 – Prove e rapporto** | prove Go delle API (catalogo e filtri; download da un server finto: riuscito, interrotto e ripreso, tipo sbagliato, troppo grande, dominio non consentito; importazioni valide, corrotte, doppie, non supportate; associazione alle campagne; **persistenza** riavviando il server sullo stesso database) e prove nel browser con Playwright (catalogo, filtri, fonte ufficiale, download, importazione, visualizzazione, campagne, errori) più il controllo che le funzioni esistenti non cambino | `*_test.go` nuovi, `test/browser/` nuovo |

Ogni passo si chiude con un commit; il rapporto finale elenca file creati e modificati, prove eseguite con il loro
esito, cosa manca e i limiti tecnici e legali. Nessuna funzione si dichiara funzionante senza una prova eseguita.

## 4. Dipendenze
Nessuna nuova libreria: HTTP, ZIP (DOCX/EPUB), SHA-256 e SQLite ci sono già; pdf.js è già incluso; il lettore RTF
minimo si scrive nel progetto. La ricerca delle fonti ufficiali per il catalogo si fa una volta, a mano, durante L1
(con la data di verifica): l'app non «cerca» documenti da sola su Internet.

## 5. Decisioni del Master necessarie prima di iniziare
1. **Nomi dei giochi nel catalogo.** (a) Mostrare i nomi ufficiali dei giochi ed editori *solo nel catalogo* (file di
   dati a parte, escluso dal controllo delle parole vietate), come riferimento, con la dicitura «marchi dei rispettivi
   proprietari, nessuna affiliazione o approvazione»; da far verificare all'avvocato prima della vendita. (b) Usare solo
   nomi descrittivi («d20 fantasy 5ª edizione»). La richiesta va nella direzione (a), che però cambia la regola attuale
   di `CLAUDE.md`.
2. **Campagne.** Introdurre l'entità «campagna» (§3, L4), con i gruppi esistenti spostati in una «Campagna principale».
3. **Ordine dei passi**: L1 → L6 come sopra, oppure prima MY RULEBOOKS (L3) e poi catalogo e download.

## 5bis. Decisioni prese (13/11/2026)
1. **Nomi ufficiali** dei giochi ed editori **solo nel catalogo**, con l'avviso sui marchi; `CLAUDE.md` aggiornato; il file
   del catalogo è escluso dal controllo delle parole vietate. Da verificare con l'avvocato prima della vendita.
2. **Entità campagna**: sì, con i gruppi esistenti in una «Campagna principale».
3. **Ordine**: L1 → L6.

## 6. Limiti noti fin d'ora
- Molti giochi dell'elenco non hanno materiale gratuito scaricabile liberamente: per questi il catalogo darà solo il
  collegamento ufficiale. Alcuni editori pubblicano i documenti gratuiti solo dopo l'accesso a un negozio online
  (account): lì l'app non scarica, apre la pagina.
- Licenze e indirizzi cambiano: ogni voce ha la data di verifica; serve un controllo periodico (manuale, o un
  controllo automatico che segnala i collegamenti non più raggiungibili senza scaricare nulla).
- La ricerca nel contenuto funziona sui documenti con testo; i PDF scansionati richiedono il testo OCR.
- Le 19 prove del server ancora rosse dalla fase 3b (`docs/PIANO.md` §5.1) restano un lavoro separato (fase 3d).

## 7. Avanzamento
### L1 – Catalogo: fatto (13/11/2026)
- `app/internal/server/catalogo_regolamenti.json`: 20 giochi, 16 documenti. Fonti cercate il 13/11/2026: per ogni
  documento la data di verifica (vuota = indirizzo da verificare). Download diretto consentito solo per 2 PDF gratuiti
  ospitati dall'editore senza accesso (SRD di Daggerheart, Quickstart di Basic Roleplaying). Nessun documento gratuito
  verificato per: Warhammer Fantasy Roleplay, Werewolf: The Apocalypse, RuneQuest, The One Ring, Fabula Ultima,
  Starfinder, Traveller, Alien RPG (solo il sito dell'editore, da verificare).
- `app/internal/server/regolamenti_catalogo.go`: lettura e controllo del catalogo (id unici, tipologie, solo https, date,
  download diretto solo per documenti gratuiti e verificati); copia locale aggiornabile in `Dati\regolamenti\catalogo.json`
  (se non valida si ignora); `GET /api/v1/regolamenti/catalogo` con i filtri gioco, genere, editore, lingua, tipologia,
  edizione, gratuito; solo il Master.
- `app/web/sezioni/regolamenti.js`, `app/web/stile/regolamenti.css`, voce «RPG Rulebook Library» nel menu (Campagna).
- Prove: `TestCatalogoRegolamenti` (Go) verde; `bash test/browser/esegui.sh regolamenti` 13/13 (catalogo, ricerca,
  filtri, fonti ufficiali in nuova scheda, telefono senza scorrimento orizzontale, nessun errore JavaScript). Nuova
  cartella delle prove nel browser: `app/test/browser/` (`esegui.sh`, `comune.js`).

### L2 – Download: fatto (13/11/2026)
- Migrazione **038** (`regolamenti_documenti`, `regolamenti_categorie`, con rollback): la tabella di MY RULEBOOKS, già con
  i campi per categorie, versioni e note (servono al passo L3).
- `app/internal/server/regolamenti_scarica.go`: `POST /api/v1/regolamenti/scarica` (solo voci con download diretto),
  `GET /api/v1/regolamenti/scaricamenti`; download in sottofondo con avanzamento in tempo reale (`regolamenti.scarico`),
  solo https, rinvii solo verso lo stesso dominio, massimo 200 MB e 10 minuti, ripresa con `Range` e fino a 3 tentativi,
  pagine con accesso (401/402/403) e documenti spariti (404/410) segnalati senza riprovare, tipo controllato dal
  contenuto (`%PDF-`, ZIP), doppioni riconosciuti dall'impronta SHA-256, file parziale in `Dati\regolamenti\parziali`
  e definitivo in `Dati\regolamenti\file`.
- `app/internal/server/regolamenti_miei.go`: `GET /api/v1/regolamenti/miei` (elenco) e `GET .../miei/{id}/file`
  (consultazione o salvataggio, con `nosniff` e `sandbox`). Il catalogo indica i documenti già nella libreria.
- Pagina: pulsante «⬇ Scarica nella mia libreria», barra di avanzamento, «✓ Nella mia libreria» con «Apri», errore
  con «Riprova».
- Prove: `TestScaricaRegolamenti` (editore finto: riuscito, già presente, doppione, interruzione a metà e ripresa,
  pagina HTML al posto del PDF, accesso richiesto, rinvio verso un altro sito, file troppo grande, voce senza download
  diretto, voce inesistente, giocatore rifiutato, riavvio con dati e file intatti) verde.
  Nel browser `regolamenti_scarico` **10/10 con i download veri** dai siti degli editori (SRD di Daggerheart
  1.553.971 byte, Quickstart di Basic Roleplaying 10.312.464 byte), avanzamento visto, file aperti come PDF, documenti
  presenti dopo il riavvio del programma. Per questa prova nella sessione cloud servono i due siti nella rete
  dell'ambiente (ambiente «Gaster Bless (GM-Manager e Barovia)») e `SSL_CERT_FILE=/root/.ccr/ca-bundle.crt`.
- Nessuna regressione: le prove rosse del server restano le stesse 19 della fase 3b.

### L3 – MY RULEBOOKS: fatto (13/11/2026)
- `app/internal/server/regolamenti_miei.go` (ampliato): importazione (`POST /api/v1/regolamenti/miei?file=…`) di PDF,
  DOCX, EPUB, RTF, TXT, MD, HTML fino a 200 MB, con controlli sul contenuto (PDF con intestazione e fine, DOCX/EPUB come
  ZIP con le parti necessarie, testo senza byte binari, RTF ben formato): file vuoti, danneggiati, binari e formati non
  supportati rifiutati con un messaggio chiaro, doppioni riconosciuti dall'impronta (409); modifica dei dati (gioco,
  edizione, tipologia, lingua, editore, categoria, versione, note), **versioni** dello stesso regolamento (un livello:
  principale e sue versioni), sostituzione del file (`PUT …/file`), eliminazione (le versioni restano), lettura del
  testo a blocchi (`GET …/testo`), testo dei PDF letto nel browser con pdf.js e mandato al server (`POST …/testo`),
  **ricerca** nel titolo e nel contenuto senza maiuscole e accenti (`GET …/miei/cerca`), **categorie** personali
  (creare, rinominare, eliminare: i documenti restano).
- `app/internal/server/regolamenti_rtf.go` (nuovo): lettore RTF minimo senza librerie esterne.
- Pagine: `app/web/sezioni/miei_regolamenti.js` (nuova, voce di menu «My Rulebooks»), `app/web/ui/libreria_schede.js`
  (schede tra le due sezioni), stile in `app/web/stile/regolamenti.css`. I PDF si aprono nel lettore del browser; gli
  altri formati nel lettore dell'app, anche nel punto trovato dalla ricerca.
- Prove: `TestMieiRegolamenti` e `TestLettoreRTF` (Go) verdi; nel browser `miei_regolamenti` **24/24** (importazione di
  PDF vero, TXT, RTF, DOCX; PDF danneggiato, formato non supportato e doppione rifiutati; categoria; filtri; ricerca nel
  PDF e nell'RTF senza accenti; apertura nel punto trovato; versione sotto il principale; lettura del TXT; PDF aperto
  in una nuova scheda; sostituzione del DOCX con ricerca aggiornata; eliminazione; tutto intatto dopo il riavvio; nessun
  errore JavaScript). Nessuna regressione (le stesse 19 prove rosse della fase 3b).
- Manca ancora (passo L4): l'associazione dei documenti alle **campagne**.
