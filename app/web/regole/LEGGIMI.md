# Regole quinta edizione (modulo unico)

> Nota (13/11/2026): in GASTER BLESS – MASTER SUITE i dati di gioco sono stati tolti (nessun contenuto incluso) e i
> riferimenti alle vecchie cartelle di origine non valgono più: questa descrizione resta come guida alla struttura.
> I dati arrivano dai **pacchetti di contenuti** dell'utente (`docs/FORMATO_CONTENUTI.md`): il server aggiunge in fondo a
> `dati.js` e `talenti.js` le voci dei pacchetti attivi (`internal/server/contenuti.go`).

| File | Contenuto |
|---|---|
| `dati.js` | Dati delle regole: classi, sottoclassi, razze, background, armi, armature, tabelle degli slot, ~470 incantesimi e le spiegazioni dei privilegi (sintesi in italiano). Copia di `01_scheda/sorgenti/data.js`, `spells1-4.js`, `featdesc.js`. |
| `motore.js` | Motore delle regole: livelli (del personaggio e di ogni classe), bonus di competenza, **multiclasse** (requisiti, competenze ridotte, punti ferita classe per classe, slot condivisi secondo la tabella del multiclasse, Magia del Patto separata, CD/attacco e incantesimi per ogni classe), aumenti di caratteristica per livello di classe. Definisce la variabile globale `Regole5e`. |
| `incantesimi_effetti.js` | **Effetti degli incantesimi** (modulo puro, `IncantesimiEffetti`): legge dalla descrizione attacco, TS, danni per tipo, cure, PF temporanei, CA, condizioni, area, bersagli, durata, livelli superiori; «automatico» falso quando non è certo (decide il Master). Prova: `node test/regole/incantesimi_effetti_test.js` |
| `oggetti.js` | **Oggetti magici** (modulo puro, `OggettiMagici`): modello dati della «Scheda oggetto magico», modificatori tecnici (aggiungi, sostituisci, minimo, massimo, formula, vantaggio/svantaggio, resistenza, immunità, effetto speciale), condizioni (equipaggiato, sintonizzato, attivato, portato con sé), slot, al massimo 3 sintonie, gruppi non cumulabili, importazione/esportazione JSON con id univoco e versione, controllo di ogni campo. Non scrive mai nei valori base del personaggio: calcola solo differenze. Prove: `test/regole/oggetti_test.js`. |

Si caricano come script normali, prima `dati.js` e poi `motore.js`: li usa la scheda (`web/scheda/`) e li
userà il tracker dell'iniziativa.

## Forma della scheda per il multiclasse
- `cls`, `sub`, `lvl`, `hp`: classe **principale** (quella del 1° livello), come nelle schede già salvate.
- `mc`: `[{cls, sub, lvl, hp}]` classi aggiunte. `hp` per livello di quella classe: `{1:{m:'avg'}, 2:{m:'man', v:5}}`.
- Aumenti di caratteristica: chiave `"4"` per la classe principale, `"mago:4"` per le classi aggiunte.
- Incantesimi: `spl[livello] = [{k, p, o}]`; `o` = classe per cui è stato preso, oppure `"extra"` per i
  bonus (trucchetti razziali, talenti, incantesimi sempre preparati della sottoclasse) che non contano nei limiti.

## Verifiche
- `node test/regole/motore_test.js`: 18 personaggi di prova con i valori delle regole.
- `node test/regole/confronto_scheda_originale.js`: confronta, in un browser, la scheda originale
  (`01_scheda/index.html`) con quella nuova su tutte le classi, sottoclassi e livelli 1-20 (2.280 casi):
  per una sola classe i numeri devono essere identici. Serve Gaster Bless in modalità di prova sulla porta 7600
  e Playwright (presenti nelle sessioni cloud).
