-- Rollback della migrazione 010 (conteggio dei fallimenti consecutivi). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 010_sanita_fallimenti_rollback.sql
BEGIN;
ALTER TABLE sanita_pg DROP COLUMN fallimenti_consecutivi;
DELETE FROM versione_schema WHERE versione = 10;
COMMIT;
