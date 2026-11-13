-- Rollback della migrazione 007 (schede del bestiario). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 007_bestiario_schede_rollback.sql
-- Si perdono le schede compilate nella sezione «PNG e mostri» e il modello dei PNG creati dal Master
-- (le voci, le immagini e le note restano).
BEGIN;
ALTER TABLE bestiario_personalizzazioni DROP COLUMN scheda_dati;
ALTER TABLE bestiario_voci DROP COLUMN base;
DELETE FROM versione_schema WHERE versione = 7;
COMMIT;
