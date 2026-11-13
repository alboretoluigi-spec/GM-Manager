-- Rollback della migrazione 042 (pacchetti personali). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 042_contenuti_personali_rollback.sql
-- I pacchetti personali restano come pacchetti normali.
BEGIN;
ALTER TABLE contenuti_pacchetti DROP COLUMN aggiornato_il;
ALTER TABLE contenuti_pacchetti DROP COLUMN personale;
DELETE FROM versione_schema WHERE versione = 42;
COMMIT;
