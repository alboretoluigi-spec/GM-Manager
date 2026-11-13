-- Rollback della migrazione 028 (effetti in corso nello scontro). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 028_effetti_scontro_rollback.sql
-- Le condizioni già messe ai combattenti restano.
BEGIN;
DROP TABLE IF EXISTS effetti_scontro;
DELETE FROM versione_schema WHERE versione = 28;
COMMIT;
