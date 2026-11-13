-- RPG RULEBOOK LIBRARY e MY RULEBOOKS (docs/RULEBOOK_LIBRARY.md, 2026-11-13): documenti del Master, scaricati dalle fonti
-- ufficiali del catalogo (origine «download») o importati da lui (origine «importato»). Il file sta in
-- Dati\regolamenti\file\<id>.<formato>; «impronta» (SHA-256 del contenuto) riconosce i doppioni. «versione_di» collega
-- più versioni dello stesso regolamento al documento principale. Le categorie le crea il Master.
-- Rollback: internal/db/rollback/038_regolamenti_rollback.sql
CREATE TABLE regolamenti_categorie (
  id        TEXT PRIMARY KEY,
  nome      TEXT NOT NULL UNIQUE,
  creato_il INTEGER NOT NULL
);
CREATE TABLE regolamenti_documenti (
  id            TEXT PRIMARY KEY,
  titolo        TEXT NOT NULL,
  gioco         TEXT NOT NULL DEFAULT '',
  edizione      TEXT NOT NULL DEFAULT '',
  tipologia     TEXT NOT NULL DEFAULT 'altro',
  lingua        TEXT NOT NULL DEFAULT '',
  editore       TEXT NOT NULL DEFAULT '',
  formato       TEXT NOT NULL,
  file          TEXT NOT NULL,
  dimensione    INTEGER NOT NULL,
  impronta      TEXT NOT NULL UNIQUE,
  origine       TEXT NOT NULL CHECK (origine IN ('download', 'importato')),
  catalogo_id   TEXT NOT NULL DEFAULT '',
  fonte_url     TEXT NOT NULL DEFAULT '',
  licenza       TEXT NOT NULL DEFAULT '',
  categoria_id  TEXT REFERENCES regolamenti_categorie(id) ON DELETE SET NULL,
  versione_di   TEXT REFERENCES regolamenti_documenti(id) ON DELETE SET NULL,
  versione      TEXT NOT NULL DEFAULT '',
  note          TEXT NOT NULL DEFAULT '',
  creato_il     INTEGER NOT NULL,
  aggiornato_il INTEGER NOT NULL
);
CREATE INDEX regolamenti_documenti_gioco ON regolamenti_documenti(gioco, edizione);
CREATE INDEX regolamenti_documenti_catalogo ON regolamenti_documenti(catalogo_id);
