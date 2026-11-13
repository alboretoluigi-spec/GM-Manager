-- Rollback della migrazione 039 (campagne). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 039_campagne_rollback.sql
BEGIN;
DROP TRIGGER gruppi_campagna_predefinita;
ALTER TABLE gruppi DROP COLUMN campagna_id;
DROP TABLE campagna_documenti;
DROP TABLE campagne;
DELETE FROM versione_schema WHERE versione = 39;
COMMIT;
