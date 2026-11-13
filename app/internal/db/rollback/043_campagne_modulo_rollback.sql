-- Rollback della migrazione 043 (modulo di gioco delle campagne). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 043_campagne_modulo_rollback.sql
BEGIN;
ALTER TABLE campagne DROP COLUMN modulo;
DELETE FROM impostazioni WHERE chiave = 'campagna_attiva';
DELETE FROM versione_schema WHERE versione = 43;
COMMIT;
