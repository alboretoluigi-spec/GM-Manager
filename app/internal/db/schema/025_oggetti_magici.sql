-- Archivio degli oggetti magici del Master (richiesta dell'utente del 2026-11-11: «nell'app non trovo dove il master può
-- inserire o creare oggetti magici»). Ogni riga è un oggetto completo nel formato della scheda (web/regole/oggetti.js,
-- OggettiMagici): il Master lo crea, lo modifica e lo consegna alle schede dei personaggi. Gli oggetti importati dai
-- manuali (fase M3.3) ricordano manuale e pagina. Eliminare un oggetto dall'archivio non lo toglie dalle schede.
-- Rollback: internal/db/rollback/025_oggetti_magici_rollback.sql
CREATE TABLE oggetti_magici (
  id            TEXT PRIMARY KEY,           -- lo stesso id dell'oggetto (campo «id» di dati)
  nome          TEXT NOT NULL,
  chiave        TEXT NOT NULL DEFAULT '',   -- nome normalizzato (per l'importazione dai manuali)
  dati          TEXT NOT NULL,              -- l'oggetto completo (JSON), immagine compresa
  manuale_nome  TEXT NOT NULL DEFAULT '',
  pagina        INTEGER NOT NULL DEFAULT 0,
  creato_il     INTEGER NOT NULL,
  aggiornato_il INTEGER NOT NULL
);
CREATE INDEX oggetti_magici_chiave ON oggetti_magici(chiave);
