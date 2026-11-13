-- Pacchetti di contenuti (fase 3c, passo C1, 2026-11-13): classi, razze, background, incantesimi, armi, armature,
-- talenti e stili che l'utente importa da un file nel formato aperto «gasterbless-contenuti» (docs/FORMATO_CONTENUTI.md).
-- L'app non include nessun contenuto: questa tabella parte vuota. Si conserva il file originale (per esportarlo di nuovo)
-- e la sua impronta (per riconoscere i doppioni); i pacchetti attivi si uniscono, nell'ordine di importazione, ai dati
-- che la scheda legge da /regole/dati.js e /regole/talenti.js.
-- Rollback: internal/db/rollback/041_contenuti_rollback.sql
CREATE TABLE contenuti_pacchetti (
  id          TEXT PRIMARY KEY,
  nome        TEXT NOT NULL,
  sistema     TEXT NOT NULL,
  autore      TEXT NOT NULL DEFAULT '',
  versione    TEXT NOT NULL DEFAULT '',
  licenza     TEXT NOT NULL DEFAULT '',
  descrizione TEXT NOT NULL DEFAULT '',
  file        TEXT NOT NULL,
  impronta    TEXT NOT NULL UNIQUE,
  conteggi    TEXT NOT NULL DEFAULT '{}',
  attivo      INTEGER NOT NULL DEFAULT 1,
  creato_il   INTEGER NOT NULL
);
