-- Rollback della migrazione 011 (risorse di scheda esterna). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 011_arcana_risorse_rollback.sql
BEGIN;
ALTER TABLE stato_in_gioco DROP COLUMN risorse_arcana;
DELETE FROM versione_schema WHERE versione = 11;
COMMIT;
