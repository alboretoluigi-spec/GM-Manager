-- Rollback della migrazione 038 (documenti di MY RULEBOOKS). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 038_regolamenti_rollback.sql
-- I file in Dati\regolamenti\file restano: si possono cancellare a mano.
BEGIN;
DROP TABLE regolamenti_documenti;
DROP TABLE regolamenti_categorie;
DELETE FROM versione_schema WHERE versione = 38;
COMMIT;
