-- Rollback della migrazione 019 (appunti del Master sui personaggi). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 019_note_master_rollback.sql
-- Si perdono gli appunti del Master sui personaggi.
BEGIN;
DROP TABLE IF EXISTS note_master_pg;
DELETE FROM versione_schema WHERE versione = 19;
COMMIT;
