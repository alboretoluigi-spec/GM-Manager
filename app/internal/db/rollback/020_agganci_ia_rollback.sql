-- Rollback della migrazione 020 (agganci per l'antagonista scritti dall'IA). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 020_agganci_ia_rollback.sql
-- Si perdono gli agganci scritti dall'IA (si possono rigenerare).
BEGIN;
DROP TABLE IF EXISTS ia_agganci_bg;
ALTER TABLE ia_conversazioni DROP COLUMN nuova;
DELETE FROM versione_schema WHERE versione = 20;
COMMIT;
