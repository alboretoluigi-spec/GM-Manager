-- Rollback della migrazione 037 (raccolta dei PNG). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 037_raccolta_png_rollback.sql
BEGIN;
ALTER TABLE bestiario_personalizzazioni DROP COLUMN raccolta;
DELETE FROM versione_schema WHERE versione = 37;
COMMIT;
