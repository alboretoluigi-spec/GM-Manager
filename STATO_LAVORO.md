# Stato del lavoro: GASTER BLESS – MASTER SUITE

## Fase attuale
**Fase 3a fatta (13/11/2026)**: repository privato `alboretoluigi-spec/GM-Manager` creato dal Master, struttura di base
(istruzioni per Claude, stato del lavoro, piano, regole sui contenuti e licenze). Nessun codice ancora: la copia ripulita
da Barovia è la fase 3b.

## Fase attuale (aggiornamento)
**13/11/2026: marchio deciso.** Nome ufficiale **GASTER BLESS – MASTER SUITE**, slogan **MASTER YOUR WORLD**, sottotitolo
**THE UNIVERSAL TABLETOP RPG PLATFORM**, stile dell'immagine del Master (oro `#d8b978` su `#191923`). Pronti in `branding/`:
`marchio.css`, `intestazione.html` (anteprima), `logo.svg`, `MARCHIO.md`, carattere Source Sans 3. **App generica per diversi giochi di ruolo, senza
contenuti di gioco inclusi** (mostri, manuali, ambientazioni li importa o inserisce l'utente). Il marchio si applica a
tutte le schermate nella fase 3b, insieme alla copia del codice (oggi il repository non ha ancora schermate).

## Decisioni prese
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
- Da decidere: quali sistemi di gioco nella prima versione (quinta edizione più scheda libera, oppure altri completi).
- Domande ancora aperte (`docs/PIANO.md` §6): accesso da Internet nella prima versione, prezzo e piattaforma di vendita,
  lingua di partenza.
- Verifica del nome «Gaster Bless» con l'avvocato (banche dati dei marchi; «Gaster» è anche un personaggio di *Undertale*)
  prima di registrare dominio e marchio.

## Prossimo passo
1. **Fase 3b** (dopo la conferma del Master): copia ripulita del codice da Barovia (`04_nuova_app`) in una cartella di
   lavoro fuori dal repository; tolti i contenuti dell'inventario; nome cambiato (`barovia` → `gasterbless`) e marchio applicato
   a tutte le schermate secondo `branding/MARCHIO.md`; controllo
   automatico delle parole vietate; compilazione e prove. Solo quando è pulita, commit nel repository.

## Registro sessioni
| Data | Chi | Cosa |
|---|---|---|
| 2026-11-13 | Claude Code (cloud) | Decisione del Master: app generica per diversi giochi di ruolo, nessun contenuto di gioco incluso (importazione o inserimento a mano da parte dell'utente). Aggiornati `CLAUDE.md`, `README.md`, `docs/PIANO.md` (§1, §3.2, §3.3, fase 3c, §6), `docs/LICENZE_E_CONTENUTI.md`. |
| 2026-11-13 | Claude Code (cloud) | Marchio: nome GASTER BLESS – MASTER SUITE, slogan e sottotitolo dall'immagine del Master (colori ricavati dall'immagine). Creati `branding/MARCHIO.md`, `marchio.css`, `intestazione.html`, `logo.svg`, `font/` (Source Sans 3, OFL); aggiornati `CLAUDE.md`, `README.md`, `docs/PIANO.md`, `docs/LICENZE_E_CONTENUTI.md`. Anteprima controllata su PC e telefono. |
| 2026-11-13 | Claude Code (cloud) | Registrate le risposte del Master alle quattro domande (immagini tolte, anche macOS, 1 PC a vita, niente gioco da remoto) in `docs/PIANO.md`, `docs/LICENZE_E_CONTENUTI.md` e qui. |
| 2026-11-13 | Claude Code (cloud) | Fase 3a: repository collegato alla sessione; creati `CLAUDE.md`, `README.md`, `.gitignore`, `STATO_LAVORO.md`, `docs/PIANO.md` (dall'inventario di Barovia, con il nome GM Manager e le fasi 3a-3d), `docs/LICENZE_E_CONTENUTI.md` (regole, attribuzione SRD 5.1, componenti di terzi). |

## File creati o modificati nell'ultima sessione
`branding/MARCHIO.md`, `branding/marchio.css`, `branding/intestazione.html`, `branding/logo.svg`,
`branding/font/source-sans-3.woff2`, `branding/font/LICENZA-source-sans-3.txt` (nuovi); `CLAUDE.md`, `README.md`,
`docs/PIANO.md`, `docs/LICENZE_E_CONTENUTI.md`, `STATO_LAVORO.md`.
