-- Rollback della migrazione 017 (riassunti firmati, background in testo). Da eseguire SOLO a server fermo, dopo un
-- backup:  sqlite3 gasterbless.db < 017_riassunti_background_rollback.sql
-- Si perdono i background in testo, la firma del personaggio e gli «Ottimo lavoro» dei riassunti (i testi restano).
BEGIN;
DROP TABLE IF EXISTS background_testi;
ALTER TABLE riassunti DROP COLUMN ottimo;
ALTER TABLE riassunti DROP COLUMN personaggio_id;
DELETE FROM versione_schema WHERE versione = 17;
COMMIT;
