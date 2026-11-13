-- Rollback della migrazione 029 (effetti che cambiano i valori). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 029_effetti_valori_rollback.sql
BEGIN;
ALTER TABLE effetti_scontro DROP COLUMN modifiche;
DELETE FROM versione_schema WHERE versione = 29;
COMMIT;
