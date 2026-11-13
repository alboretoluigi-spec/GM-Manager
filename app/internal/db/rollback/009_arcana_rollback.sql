-- Rollback della migrazione 009 (scheda esterna). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 009_arcana_rollback.sql
-- I dispositivi abbinati vanno abbinati di nuovo dopo un eventuale ritorno alla 009.
BEGIN;
DROP TABLE arcana_eventi;
DROP TABLE arcana_dispositivi;
DROP TABLE arcana_codici;
DELETE FROM versione_schema WHERE versione = 9;
COMMIT;
