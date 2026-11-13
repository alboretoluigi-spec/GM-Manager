-- Rollback della migrazione 016 (viaggi). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 016_viaggi_rollback.sql
-- Si perdono il diario dei viaggi e il tempo di gioco dei gruppi. Segnalini e mappe restano come sono.
BEGIN;
DROP TABLE IF EXISTS viaggi;
DROP TABLE IF EXISTS tempo_gruppo;
DELETE FROM versione_schema WHERE versione = 16;
COMMIT;
