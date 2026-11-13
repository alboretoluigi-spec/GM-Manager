-- Rollback della migrazione 034 (promemoria degli eventi del Codice). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 034_promemoria_lore_rollback.sql
BEGIN;
DROP TABLE IF EXISTS lore_promemoria;
DELETE FROM versione_schema WHERE versione = 34;
COMMIT;
