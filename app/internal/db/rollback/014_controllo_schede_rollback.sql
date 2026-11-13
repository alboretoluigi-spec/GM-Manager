-- Rollback della migrazione 014 (controllo delle schede). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 014_controllo_schede_rollback.sql
-- Si perdono gli avvisi di controllo e i segni «offline» / «fuori sessione» delle versioni. Le schede restano come sono.
BEGIN;
DROP TABLE IF EXISTS controlli_scheda;
ALTER TABLE schede_versioni DROP COLUMN offline;
ALTER TABLE schede_versioni DROP COLUMN fuori_sessione;
ALTER TABLE personaggi DROP COLUMN versione_controllata;
DELETE FROM versione_schema WHERE versione = 14;
COMMIT;
