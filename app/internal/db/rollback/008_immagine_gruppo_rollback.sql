-- Rollback della migrazione 008 (immagine dei gruppi). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 008_immagine_gruppo_rollback.sql
-- I file delle immagini restano nella cartella dei dati (si possono cancellare a mano).
BEGIN;
ALTER TABLE gruppi DROP COLUMN immagine;
DELETE FROM versione_schema WHERE versione = 8;
COMMIT;
