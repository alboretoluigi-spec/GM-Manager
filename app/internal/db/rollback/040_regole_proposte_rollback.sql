-- Rollback della migrazione 040 (proposte di regole). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 040_regole_proposte_rollback.sql
BEGIN;
DROP TABLE regole_proposte;
DELETE FROM versione_schema WHERE versione = 40;
COMMIT;
