-- Viaggi del segnalino del gruppo (2026-11-08): ogni spostamento oltre 1/4 di miglio è un viaggio con percorso, miglia,
-- ore e terreno; il gruppo ha un tempo di gioco (ore di viaggio, ore di marcia del giorno, giorno).
-- Rollback: internal/db/rollback/016_viaggi_rollback.sql
CREATE TABLE viaggi (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  gruppo_id   TEXT NOT NULL REFERENCES gruppi(id) ON DELETE CASCADE,
  sessione_id TEXT,
  utente_id   TEXT,
  da_x        REAL NOT NULL,
  da_y        REAL NOT NULL,
  a_x         REAL NOT NULL,
  a_y         REAL NOT NULL,
  arrivo      TEXT NOT NULL DEFAULT '',   -- chiave della località raggiunta ('' = nessuna)
  miglia      REAL NOT NULL,
  ore         REAL NOT NULL,              -- tempo di viaggio
  fatica      REAL NOT NULL,              -- ore ai fini della marcia forzata (l'alta quota conta doppio)
  giorno      INTEGER NOT NULL,           -- giorno di gioco alla fine del viaggio
  dettagli    TEXT NOT NULL DEFAULT '{}', -- {"pezzi":[...],"eventi":[...]}
  creato_il   INTEGER NOT NULL
);
CREATE INDEX viaggi_gruppo ON viaggi(gruppo_id, id);

CREATE TABLE tempo_gruppo (
  gruppo_id     TEXT PRIMARY KEY REFERENCES gruppi(id) ON DELETE CASCADE,
  ore_gioco     REAL NOT NULL DEFAULT 0,  -- ore di viaggio accumulate (ogni 24 ore passa un giorno di gioco)
  ore_marcia    REAL NOT NULL DEFAULT 0,  -- ore di marcia del giorno in corso (oltre 8 = marcia forzata)
  giorno        INTEGER NOT NULL DEFAULT 1,
  modificato_il INTEGER NOT NULL DEFAULT 0
);
