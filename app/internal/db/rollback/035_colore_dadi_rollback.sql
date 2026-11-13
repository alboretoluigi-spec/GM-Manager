-- Rollback della migrazione 035 (colore dei dadi). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 035_colore_dadi_rollback.sql
BEGIN;
ALTER TABLE utenti DROP COLUMN colore_dadi;
DELETE FROM versione_schema WHERE versione = 35;
COMMIT;
