# Rapporto finale: RPG RULEBOOK LIBRARY, MY RULEBOOKS, campagne e analisi dei regolamenti

GASTER BLESS – MASTER SUITE · passi L1–L6 della richiesta del Master del 13/11/2026 («Implementazione della libreria
universale dei regolamenti»). Analisi e piano: `docs/RULEBOOK_LIBRARY.md` (§1–§6); avanzamento passo per passo: §7.
Tutto il codice è nel repository `GM-Manager`, cartella `app/`.

## 1. Funzionalità implementate
| Area | Cosa fa | Dove si usa |
|---|---|---|
| **RPG Rulebook Library** (L1) | Catalogo di **21 giochi** (i 20 richiesti più Farsight), con editore, generi, edizioni, sito; per ogni documento gratuito titolo, edizione, editore, lingua, tipologia, formato, URL della fonte ufficiale, licenza, condizioni d'uso, data dell'ultima verifica, disponibilità del download diretto. Filtri per nome, genere, editore, lingua, tipologia, gratuità, edizione. Il catalogo è un file di dati aggiornabile senza ricompilare | Menu Campagna → «RPG Rulebook Library» |
| **Download** (L2) | Modalità **A** (download diretto) solo per i documenti consentiti: https, solo dal dominio dell'editore, massimo 200 MB e 10 minuti, ripresa dopo un'interruzione, tipo controllato dal contenuto, doppioni riconosciuti, avanzamento in tempo reale. Modalità **B** (collegamento alla fonte ufficiale) per tutti gli altri | Pulsante «⬇ Scarica nella mia libreria» o «Apri la fonte ufficiale» |
| **My Rulebooks** (L3) | Modalità **C** (importazione personale): PDF, DOCX, EPUB, RTF, TXT, MD, HTML fino a 200 MB, con rifiuto di file vuoti, danneggiati, binari, non supportati e doppioni; gioco, edizione, tipologia, lingua, editore, note; categorie personali; versioni dello stesso regolamento; ricerca nel titolo e nel contenuto (senza maiuscole e accenti; per i PDF il testo si legge nel browser); lettura nell'app o nel lettore PDF; sostituzione, download ed eliminazione | Menu Campagna → «My Rulebooks» |
| **Campagne** (L4) | Per ogni campagna: sistema di gioco ed edizione (testo libero), regolamento principale, documenti supplementari, schede personalizzate, regole aggiuntive del Master, impostazioni libere, gruppi. Lo stesso documento in più campagne. I gruppi esistenti sono nella «Campagna principale» | Menu Campagna → «Campagne»; «Campagne…» su ogni documento |
| **Analisi dei regolamenti** (L5) | 13 categorie; analizzatore per parole chiave (senza IA) con fonte, pagina ed estratto; analizzatore con l'IA predisposto ma non attivo; regole scritte dal Master; ogni regola resta **proposta** finché il Master non la approva; solo le approvate escono per la campagna (`GET /api/v1/campagne/{id}/regole`) | Menu Campagna → «Regole da approvare»; «🔎 Analizza» su ogni documento |

Il motore del gioco non dipende da nessun regolamento: nessuna tabella o rotta della libreria conosce attributi, classi
o meccaniche di un sistema. Il marchio (fondo scuro, oro antico) è usato in tutte le pagine nuove.

## 2. File creati
**Server (Go, `app/internal/server/`)**: `regolamenti_catalogo.go`, `catalogo_regolamenti.json`, `regolamenti_scarica.go`,
`regolamenti_miei.go`, `regolamenti_rtf.go`, `campagne.go`, `regolamenti_analisi.go`.
**Prove Go**: `regolamenti_catalogo_test.go`, `regolamenti_scarica_test.go`, `regolamenti_miei_test.go`,
`campagne_test.go`, `regolamenti_analisi_test.go`; mondo di prova inventato in `testdata/` (luoghi, strade, terreni,
bestiario, professioni, scheda d'esempio).
**Database (`app/internal/db/`)**: migrazioni `038_regolamenti.sql`, `039_campagne.sql`, `040_regole_proposte.sql` e i
tre rollback.
**Pagine (`app/web/`)**: `sezioni/regolamenti.js`, `sezioni/miei_regolamenti.js`, `sezioni/campagne.js`,
`sezioni/analisi_regole.js`, `ui/libreria_schede.js`, `stile/regolamenti.css`.
**Prove nel browser (`app/test/browser/`)**: `esegui.sh`, `comune.js`, `regolamenti.js`, `regolamenti_scarico.js`,
`miei_regolamenti.js`, `campagne.js`, `analisi_regole.js`.
**Documenti**: `docs/RULEBOOK_LIBRARY.md`, `docs/INSTALLAZIONE.md`, questo rapporto.

## 3. File modificati
`app/internal/server/server.go` (registrazione delle rotte), `app/web/ui/app.js` (voci di menu), `app/web/index.html`
(foglio di stile), `app/internal/controlli/parole_test.go` e `docs/parole_vietate.txt` (eccezione per il catalogo,
nuove parole vietate), `CLAUDE.md` (regola sui nomi dei giochi nel catalogo), `docs/PIANO.md`,
`docs/LICENZE_E_CONTENUTI.md`, `STATO_LAVORO.md`. Altri file toccati nello stesso periodo per l'app installabile e le
prove (porta 7700, script di Windows, mondo di prova): elencati in `STATO_LAVORO.md`.

## 4. Prove eseguite e risultati (13/11/2026)
| Prova | Esito |
|---|---|
| `go vet ./...` e `go test ./...` (tutte le prove del programma, comprese quelle di catalogo, download, My Rulebooks, RTF, campagne, analisi; parole vietate) | **verdi** |
| Nel browser `regolamenti`: catalogo, ricerca, filtri, fonti ufficiali in nuova scheda, telefono | **13/13** |
| Nel browser `regolamenti_scarico`: download **veri** dai siti degli editori (SRD di Daggerheart 1,5 MB, Quickstart di Basic Roleplaying 10 MB), avanzamento, file aperti come PDF, riavvio | **12/12** |
| Nel browser `miei_regolamenti`: importazione di PDF, TXT, RTF, DOCX; file danneggiato, formato non supportato e doppione rifiutati; categorie; filtri; ricerca nel contenuto; versioni; sostituzione; eliminazione; riavvio | **24/24** |
| Nel browser `campagne`: Campagna principale, nuova campagna, regolamento, documenti, regole, impostazioni, spostamento dei gruppi, eliminazione, collegamento da My Rulebooks, riavvio | **11/11** |
| Nel browser `analisi_regole`: analisi, nessuna regola approvata da sola, modifica, approvazione, rifiuto, regola scritta a mano, IA non disponibile, solo le approvate alla campagna, riavvio | **13/13** |

Errori di rete e download interrotti: provati con un editore finto nelle prove Go (interruzione a metà con ripresa,
pagina HTML al posto del PDF, accesso richiesto, rinvio verso un altro sito, file troppo grande).
Compatibilità con le funzioni esistenti: tutte le prove del programma restano verdi.
**Non provato qui**: l'installazione e il tasto «Aggiorna» su Windows (in questo ambiente non c'è Windows); vanno provati
sul PC del Master con `docs/INSTALLAZIONE.md`.

## 5. Funzionalità ancora da completare
1. **Analizzatore con l'IA** (oggi predisposto e non attivo): collegarlo all'assistente già configurato, sempre con
   l'approvazione del Master.
2. **Uso delle regole approvate nel motore** (iniziativa, condizioni, tiri) e moduli per famiglie di sistemi di gioco
   (`docs/PIANO.md` §3.3bis).
3. **My Rulebooks e assistente IA**: oggi i documenti di My Rulebooks non entrano da soli nella Biblioteca
   dell'assistente; manca il collegamento.
4. Testo dei **PDF scansionati** (OCR) per la ricerca.
5. **Controllo periodico del catalogo** (collegamenti che non rispondono più) e verifica delle voci «da verificare»,
   comprese quelle di Farsight.
6. Interfaccia in **inglese**; installatore per **macOS**.

## 6. Limitazioni tecniche e legali
- **Nomi dei giochi**: nel catalogo compaiono i nomi ufficiali, con l'avviso «marchi dei rispettivi proprietari, nessuna
  affiliazione». Va verificato con un avvocato prima della vendita, insieme al nome «Gaster Bless».
- **Materiale gratuito**: molti giochi non ne hanno di scaricabile liberamente; solo 2 documenti hanno il download diretto.
  Gli altri richiedono un account sul negozio dell'editore o non hanno un PDF diretto verificato: l'app apre la pagina e
  non aggira accessi, pagamenti o protezioni. Licenze e indirizzi possono cambiare: ogni voce ha la sua data di verifica.
- **Documenti personali**: il Master importa solo ciò che ha il diritto di usare; i file restano sul suo PC
  (`C:\ProgramData\GasterBless\regolamenti`) e non vengono distribuiti.
- **Analisi**: l'analizzatore di base riconosce le sezioni dai titoli, non interpreta le regole.
- **Ricerca**: è per parole, non per significato; i PDF senza testo si trovano solo per titolo.
