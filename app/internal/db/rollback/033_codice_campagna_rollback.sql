-- Rollback della migrazione 033 (Codice della campagna). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 033_codice_campagna_rollback.sql
BEGIN;
DROP TABLE IF EXISTS lore_stato;
DROP TABLE IF EXISTS lore_schede;
DELETE FROM versione_schema WHERE versione = 33;
COMMIT;
