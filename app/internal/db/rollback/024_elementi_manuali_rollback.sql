-- Rollback della migrazione 024 (elementi dai manuali). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 024_elementi_manuali_rollback.sql
BEGIN;
DROP TABLE IF EXISTS importazioni;
DROP TABLE IF EXISTS elementi_manuali;
DELETE FROM versione_schema WHERE versione = 24;
COMMIT;
