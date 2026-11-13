-- Rollback della migrazione 015 (documenti). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 015_documenti_rollback.sql
-- Si perde l'elenco dei documenti; i file cifrati restano nella cartella dati e si possono cancellare a mano.
BEGIN;
DROP TABLE IF EXISTS documenti;
DELETE FROM versione_schema WHERE versione = 15;
COMMIT;
