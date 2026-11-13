-- Rollback della migrazione 030 (effetti fuori dallo scontro). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 030_effetti_persistenti_rollback.sql
-- Si perdono gli effetti che duravano dopo gli scontri (le condizioni restano sui personaggi).
BEGIN;
DROP TABLE IF EXISTS effetti_persistenti;
DELETE FROM versione_schema WHERE versione = 30;
COMMIT;
