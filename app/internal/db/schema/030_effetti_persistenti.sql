-- Effetti fuori dallo scontro (decisione dell'utente n. 2 della Parte 2 del piano di combattimento; 2026-11-12).
-- Gli effetti lunghi ancora in corso alla fine di un combattimento passano ai personaggi bersaglio
-- (internal/server/effetti_persistenti.go): valgono fuori dallo scontro e negli scontri successivi; quelli fino a 24 ore
-- finiscono al riposo lungo o al «Nuovo giorno»; quelli a concentrazione quando l'incantatore non è più concentrato.
-- Rollback: internal/db/rollback/030_effetti_persistenti_rollback.sql
CREATE TABLE effetti_persistenti (
  id             TEXT PRIMARY KEY,
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  nome           TEXT NOT NULL,
  incantatore    TEXT NOT NULL DEFAULT '',   -- nome di chi lo ha lanciato
  incantatore_pg TEXT,                        -- personaggio di chi lo ha lanciato, se è un PG (concentrazione)
  conc           INTEGER NOT NULL DEFAULT 0,
  giornaliero    INTEGER NOT NULL DEFAULT 0,  -- 1: finisce al riposo lungo o al «Nuovo giorno» (durata fino a 24 ore)
  condizioni     TEXT NOT NULL DEFAULT '[]',  -- condizioni messe dall'effetto
  modifiche      TEXT NOT NULL DEFAULT '',    -- come effetti_scontro.modifiche (effetti_valori.go)
  creato_il      INTEGER NOT NULL
);
CREATE INDEX effetti_persistenti_pg ON effetti_persistenti(personaggio_id);
