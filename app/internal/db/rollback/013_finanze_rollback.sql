-- Rollback della migrazione 013 (finanze). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 013_finanze_rollback.sql
-- Si perdono le richieste di modifica delle monete, il registro dei movimenti e il GS dei mostri del tracker.
-- Le monete nelle schede restano come sono.
BEGIN;
DROP TABLE IF EXISTS registro_finanze;
DROP TABLE IF EXISTS richieste_monete;
ALTER TABLE combattenti DROP COLUMN gs;
ALTER TABLE combattenti DROP COLUMN ricompensato;
DELETE FROM versione_schema WHERE versione = 13;
COMMIT;
