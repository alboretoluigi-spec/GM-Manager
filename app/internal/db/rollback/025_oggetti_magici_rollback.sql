-- Rollback della migrazione 025 (archivio degli oggetti magici). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 025_oggetti_magici_rollback.sql
-- Gli oggetti già consegnati restano nelle schede dei personaggi.
BEGIN;
DROP TABLE IF EXISTS oggetti_magici;
DELETE FROM versione_schema WHERE versione = 25;
COMMIT;
