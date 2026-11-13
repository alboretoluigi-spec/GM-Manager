-- Regole automatiche della Sanità mentale (richiesta dell'utente, 2026-11-06): tre tiri salvezza falliti di
-- seguito su Sanità mentale aumentano il Terrore di 2. Si conta qui; un successo azzera il conteggio.
-- Rollback: internal/db/rollback/010_sanita_fallimenti_rollback.sql
ALTER TABLE sanita_pg ADD COLUMN fallimenti_consecutivi INTEGER NOT NULL DEFAULT 0;
