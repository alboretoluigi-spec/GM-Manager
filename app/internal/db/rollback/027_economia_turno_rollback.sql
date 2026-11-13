-- Rollback della migrazione 027 (economia del turno). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 027_economia_turno_rollback.sql
BEGIN;
ALTER TABLE combattenti DROP COLUMN economia;
DELETE FROM versione_schema WHERE versione = 27;
COMMIT;
