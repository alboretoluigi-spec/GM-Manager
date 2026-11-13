-- Rollback della migrazione 021 («Parla con l'antagonista»). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 021_ia_antagonista_rollback.sql
-- Le conversazioni con l'antagonista restano, ma diventano normali conversazioni con l'Assistente.
BEGIN;
ALTER TABLE ia_conversazioni DROP COLUMN modo;
DELETE FROM versione_schema WHERE versione = 21;
COMMIT;
