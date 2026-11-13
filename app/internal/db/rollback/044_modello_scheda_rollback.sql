-- Rollback della migrazione 044 (modello della scheda libera). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 044_modello_scheda_rollback.sql
BEGIN;
ALTER TABLE campagne DROP COLUMN modello_scheda;
DROP TRIGGER gruppi_campagna_predefinita;
CREATE TRIGGER gruppi_campagna_predefinita AFTER INSERT ON gruppi WHEN NEW.campagna_id IS NULL
BEGIN
  UPDATE gruppi SET campagna_id = (SELECT id FROM campagne ORDER BY creato_il, id LIMIT 1) WHERE id = NEW.id;
END;
DELETE FROM versione_schema WHERE versione = 44;
COMMIT;
