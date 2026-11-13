# Stato del lavoro: GASTER BLESS – MASTER SUITE

## Fase attuale
**Fase 3a fatta (13/11/2026)**: repository privato `alboretoluigi-spec/GM-Manager` creato dal Master, struttura di base
(istruzioni per Claude, stato del lavoro, piano, regole sui contenuti e licenze). Nessun codice ancora: la copia ripulita
da Barovia è la fase 3b.

## Fase attuale (aggiornamento)
**13/11/2026 sera: fase 3b fatta.** Il programma ripulito è in `app/`: compila, il controllo delle parole vietate passa, il
marchio è su tutte le schermate, provato nel browser senza errori. **19 prove del server sono ancora rosse** perché
usano dati che non ci sono più (elenco in `docs/PIANO.md` §5.1): da sistemare nella fase 3d. La scheda del personaggio
si apre ma senza classi, razze e incantesimi: arriveranno con l'importazione (fase 3c).

**13/11/2026: marchio deciso.** Nome ufficiale **GASTER BLESS – MASTER SUITE**, slogan **MASTER YOUR WORLD**, sottotitolo
**THE UNIVERSAL TABLETOP RPG PLATFORM**, stile dell'immagine del Master (oro `#d8b978` su `#191923`). Pronti in `branding/`:
`marchio.css`, `intestazione.html` (anteprima), `logo.svg`, `MARCHIO.md`, carattere Source Sans 3. **App generica per diversi giochi di ruolo, senza
contenuti di gioco inclusi** (mostri, manuali, ambientazioni li importa o inserisce l'utente). Il marchio si applica a
tutte le schermate nella fase 3b, insieme alla copia del codice (oggi il repository non ha ancora schermate).

## Decisioni prese
- **13/11/2026, sistemi di gioco**: il Master vuole poter scegliere il sistema fra i più noti. Proposta di moduli per
  famiglia di meccaniche in `docs/PIANO.md` §3.3bis; da confermare elenco e priorità della prima versione.
- **13/11/2026, app generica**: utilizzabile con diversi giochi di ruolo; **nessun contenuto di gioco** incluso né scaricato
  dall'app (nemmeno l'SRD): mostri, manuali e ambientazioni li importa da file o li inserisce a mano l'utente. Il motore della
  quinta edizione resta come primo «modulo di sistema». Aggiornati piano (§3.2, §3.3, fase 3c), licenze e istruzioni.
- **13/11/2026, marchio**: nome ufficiale «GASTER BLESS – MASTER SUITE» (breve «Gaster Bless»), slogan e sottotitolo
  come sopra; cambiano solo nome, logo testuale e intestazione, il resto dell'app resta com'è. Il repository resta
  `GM-Manager` (il Master può rinominarlo su GitHub). Nomi tecnici proposti: modulo `gasterbless`, eseguibile `GasterBless`.
- **13/11/2026**: app commerciale separata da Barovia, venduta con licenza sul PC del Master; Barovia resta la copia
  personale del Master e non cambia.
- **13/11/2026**: nome **GM Manager** (prima «GM Desktop», nome di lavoro); repository privato `GM-Manager` (creato come
  `gm-manager`, poi rinominato dal Master).
- **13/11/2026, risposte del Master**: icone delle classi, video di benvenuto e immagini (saluto, «povero», Monte Ghakis)
  tolti per ora; Windows **e macOS**; **1 PC per licenza, a vita**; **niente gioco da remoto** nella prima versione.
- **13/11/2026**: il codice entra solo già ripulito, senza la storia Git di Barovia; regole in `CLAUDE.md` e
  `docs/LICENZE_E_CONTENUTI.md`.

## Problemi aperti
- **Logo (13/11)**: il Master ha mandato due proposte fatte con Turbologo; le immagini nella chat hanno la filigrana
  «turbologo» e la scritta «GASTERL BLESS» (con una L di troppo). Serve il file acquistato, senza filigrana, con la
  scritta corretta, e la licenza d'uso commerciale; le immagini della chat non arrivano come file nella sessione.
- **Aggiorna**: si ferma finché ci sono prove rosse (19, elenco in `docs/PIANO.md` §5.1).
- Da decidere: quali famiglie di sistemi di gioco nella prima versione e in che ordine (`docs/PIANO.md` §3.3bis);
  nomi dei giochi nell'app solo dopo il parere dell'avvocato sulle licenze di compatibilità.
- Domande ancora aperte (`docs/PIANO.md` §6): accesso da Internet nella prima versione, prezzo e piattaforma di vendita,
  lingua di partenza.
- Verifica del nome «Gaster Bless» con l'avvocato (banche dati dei marchi; «Gaster» è anche un personaggio di *Undertale*)
  prima di registrare dominio e marchio.

## Prossimo passo
000. **App di base installabile (richiesta del Master del 13/11)**: script sistemati (porta 7700/7701, servizio e cartelle
   `GasterBless`, percorsi `C:\Progetti\GM-Manager\app`, niente passaggi di Barovia), pacchetto completo
   (`app/strumenti/crea-pacchetto.ps1`), icona «GB», guida `docs/INSTALLAZIONE.md`. **Da fare subito**: rimettere in verde
   le 19 prove rosse, altrimenti il tasto «Aggiorna» si ferma ai test. Logo nuovo: in attesa del file senza filigrana
   (vedi Problemi aperti). Il passo L4 (campagne) della libreria è in pausa.
00. **RPG RULEBOOK LIBRARY / MY RULEBOOKS** (`docs/RULEBOOK_LIBRARY.md`): decisioni prese (§5bis), **L1, L2 e L3 fatti**;
   prossimo **L4 – campagne** (entità campagna con sistema, edizione, regolamento, documenti, regole e impostazioni; gruppi
   esistenti nella «Campagna principale»), poi L5 analisi, L6 rapporto. Prove nel browser: `cd app && bash test/browser/esegui.sh "regolamenti regolamenti_scarico miei_regolamenti"` (la seconda
   scarica dai siti veri: nella sessione cloud serve `SSL_CERT_FILE=/root/.ccr/ca-bundle.crt`).
0. **Fase 3c** (dopo la conferma del Master): importazione dei contenuti da parte dell'utente (formati aperti e documentati
   per classi, razze, incantesimi, talenti, mostri, equipaggiamento, professioni; inserimento a mano), motore della quinta
   edizione come primo «modulo di sistema», scheda libera per gli altri giochi; poi **3d**: le 19 prove rosse e nuove
   prove nel browser. Comandi: `cd app && go build ./... && go test ./...` (oggi `internal/server` fallisce per le 19).
1. **Fase 3b** (dopo la conferma del Master): copia ripulita del codice da Barovia (`04_nuova_app`) in una cartella di
   lavoro fuori dal repository; tolti i contenuti dell'inventario; nome cambiato (`barovia` → `gasterbless`) e marchio applicato
   a tutte le schermate secondo `branding/MARCHIO.md`; controllo
   automatico delle parole vietate; compilazione e prove. Solo quando è pulita, commit nel repository.

## Registro sessioni
| Data | Chi | Cosa |
|---|---|---|
| 2026-11-13 | Claude Code (cloud) | App di base installabile: porta 7700/7701 (config, conf d'esempio, prove), script `installa.ps1` (riscritto senza i passaggi di Barovia), `aggiorna.ps1`, `aggiorna_auto.ps1`, `disinstalla.ps1`, `.bat`, `crea-pacchetto.ps1` (pacchetto zip completo), `LEGGIMI_INSTALLAZIONE.txt`, icona `gasterbless.ico` «GB»; pagina di aggiornamento con i percorsi nuovi; `docs/INSTALLAZIONE.md`. Compilazione per Windows e macOS verificata. Logo nuovo in attesa del file valido. |
| 2026-11-13 | Claude Code (cloud) | **L3 fatto**: MY RULEBOOKS completo (importazione con controlli, RTF, ricerca nel contenuto, categorie, versioni, sostituzione, eliminazione, lettura), pagina e schede tra le due sezioni; prove Go e nel browser 24/24; nessuna regressione. |
| 2026-11-13 | Claude Code (cloud) | **L2 fatto**: migrazione 038, download con ripresa e controlli, elenco e file di MY RULEBOOKS, pulsante con avanzamento; prove Go e nel browser con download veri (10/10); nessuna regressione. Rinominate in modo neutro le raccolte dei PNG («CoS Reloaded» → «Raccolta alternativa»), parole vietate aggiunte. Creato l'ambiente cloud personale «Gaster Bless (GM-Manager e Barovia)» con i due siti degli editori. |
| 2026-11-13 | Claude Code (cloud) | Decisioni del Master sulla libreria: nomi ufficiali solo nel catalogo (CLAUDE.md aggiornato, catalogo escluso dal controllo delle parole), entità campagna, ordine L1→L6. **L1 fatto**: catalogo di 20 giochi con fonti verificate, API con filtri, pagina RPG Rulebook Library, prove Go e nel browser (13/13). |
| 2026-11-13 | Claude Code (cloud) | Richiesta «Libreria universale dei regolamenti»: analisi dell'architettura e piano tecnico (passi L1-L6, componenti, dipendenze, conflitti, decisioni) in `docs/RULEBOOK_LIBRARY.md`. Nessun codice scritto. |
| 2026-11-13 | Claude Code (cloud) | Richiesta del Master: sistema di gioco a scelta fra i più noti. Scritta la proposta (moduli di sistema per famiglia di meccaniche, elenco dei giochi, nodo dei marchi) in `docs/PIANO.md` §3.3bis; in attesa della scelta delle priorità. |
| 2026-11-13 | Claude Code (cloud) | **Fase 3b**: copia di Barovia (`04_nuova_app`, senza storia Git) ripulita in una cartella di lavoro e portata in `app/`. Tolte le funzioni legate alla campagna, svuotati i dati di gioco, nomi cambiati (`gasterbless`, «Parla con l'antagonista», «Archivista delle fonti»), istruzioni dell'IA neutre, media e icone rifatti, marchio applicato. Nuovi `app/internal/controlli/parole_test.go`, `docs/parole_vietate.txt`, `app/web/stile/marchio.css`, `app/web/ui/marchio.js`, `app/README.md`. Corretto `branding/` (carattere del logo SVG, intestazione con div). Prove: build e vet puliti, parole vietate ok, 19 prove del server rosse (elenco in `docs/PIANO.md` §5.1), pagine principali provate nel browser. |
| 2026-11-13 | Claude Code (cloud) | Decisione del Master: app generica per diversi giochi di ruolo, nessun contenuto di gioco incluso (importazione o inserimento a mano da parte dell'utente). Aggiornati `CLAUDE.md`, `README.md`, `docs/PIANO.md` (§1, §3.2, §3.3, fase 3c, §6), `docs/LICENZE_E_CONTENUTI.md`. |
| 2026-11-13 | Claude Code (cloud) | Marchio: nome GASTER BLESS – MASTER SUITE, slogan e sottotitolo dall'immagine del Master (colori ricavati dall'immagine). Creati `branding/MARCHIO.md`, `marchio.css`, `intestazione.html`, `logo.svg`, `font/` (Source Sans 3, OFL); aggiornati `CLAUDE.md`, `README.md`, `docs/PIANO.md`, `docs/LICENZE_E_CONTENUTI.md`. Anteprima controllata su PC e telefono. |
| 2026-11-13 | Claude Code (cloud) | Registrate le risposte del Master alle quattro domande (immagini tolte, anche macOS, 1 PC a vita, niente gioco da remoto) in `docs/PIANO.md`, `docs/LICENZE_E_CONTENUTI.md` e qui. |
| 2026-11-13 | Claude Code (cloud) | Fase 3a: repository collegato alla sessione; creati `CLAUDE.md`, `README.md`, `.gitignore`, `STATO_LAVORO.md`, `docs/PIANO.md` (dall'inventario di Barovia, con il nome GM Manager e le fasi 3a-3d), `docs/LICENZE_E_CONTENUTI.md` (regole, attribuzione SRD 5.1, componenti di terzi). |

## File creati o modificati nell'ultima sessione
`app/` (nuovo: tutto il programma), `docs/parole_vietate.txt` (nuovo), `docs/PIANO.md`, `docs/LICENZE_E_CONTENUTI.md`,
`branding/MARCHIO.md`, `branding/intestazione.html`, `branding/logo.svg`, `.gitignore`, `STATO_LAVORO.md`.
