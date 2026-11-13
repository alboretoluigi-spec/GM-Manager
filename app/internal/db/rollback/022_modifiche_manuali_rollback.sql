-- Rollback della migrazione 022 («Modifica manuale» della scheda). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 022_modifiche_manuali_rollback.sql
-- Le richieste ancora in attesa vanno perse; le schede (comprese le modifiche già approvate) restano.
BEGIN;
DROP TABLE IF EXISTS richieste_scheda;
DELETE FROM versione_schema WHERE versione = 22;
COMMIT;
