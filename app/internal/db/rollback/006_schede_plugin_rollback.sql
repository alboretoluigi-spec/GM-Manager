-- Rollback della migrazione 006 (schede alternative). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 006_schede_plugin_rollback.sql
-- I personaggi tornano tutti alla scheda di Gaster Bless (i dati salvati dalle schede alternative restano, ma
-- la scheda di Gaster Bless potrebbe non saperli leggere).
BEGIN;
ALTER TABLE personaggi DROP COLUMN scheda;
DELETE FROM versione_schema WHERE versione = 6;
COMMIT;
