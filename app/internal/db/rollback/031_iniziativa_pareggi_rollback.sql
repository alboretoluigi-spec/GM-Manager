-- Rollback della migrazione 031 (20/1 naturale, pareggi, scheda dei mostri nel tracker). Da eseguire SOLO a server fermo,
-- dopo un backup:  sqlite3 gasterbless.db < 031_iniziativa_pareggi_rollback.sql
BEGIN;
ALTER TABLE combattenti DROP COLUMN scheda_mostro;
ALTER TABLE combattenti DROP COLUMN ordine_pari;
ALTER TABLE combattenti DROP COLUMN iniz_naturale;
DELETE FROM versione_schema WHERE versione = 31;
COMMIT;
