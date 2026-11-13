-- Rollback della migrazione 041 (pacchetti di contenuti). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 041_contenuti_rollback.sql
BEGIN;
DROP TABLE contenuti_pacchetti;
DELETE FROM versione_schema WHERE versione = 41;
COMMIT;
