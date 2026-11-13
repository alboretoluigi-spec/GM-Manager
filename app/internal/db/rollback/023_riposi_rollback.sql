-- Rollback della migrazione 023 (riposi). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 023_riposi_rollback.sql
BEGIN;
DROP TABLE IF EXISTS riposi;
ALTER TABLE personaggi DROP COLUMN ultimo_riposo_lungo;
DELETE FROM versione_schema WHERE versione = 23;
COMMIT;
