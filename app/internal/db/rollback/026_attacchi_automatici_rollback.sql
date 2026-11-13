-- Rollback della migrazione 026 (attacchi automatici dal tracker). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 026_attacchi_automatici_rollback.sql
BEGIN;
ALTER TABLE combattenti DROP COLUMN attacchi_turno;
ALTER TABLE combattenti DROP COLUMN attacchi_usati;
ALTER TABLE combattenti DROP COLUMN profilo;
DELETE FROM versione_schema WHERE versione = 26;
COMMIT;
