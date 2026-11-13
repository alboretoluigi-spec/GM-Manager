-- Rollback della migrazione 018 (assistente IA del Master). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 018_assistente_rollback.sql
-- Si perdono le conversazioni con l'assistente; le impostazioni dell'IA restano (sono innocue).
BEGIN;
DROP TABLE IF EXISTS ia_conversazioni;
DELETE FROM versione_schema WHERE versione = 18;
COMMIT;
