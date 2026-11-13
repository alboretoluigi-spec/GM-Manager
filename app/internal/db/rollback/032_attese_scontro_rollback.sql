-- Rollback della migrazione 032 (critici e tiri salvezza in attesa salvati nel database). Da eseguire SOLO a server
-- fermo, dopo un backup:  sqlite3 gasterbless.db < 032_attese_scontro_rollback.sql
BEGIN;
DROP TABLE IF EXISTS attese_scontro;
DELETE FROM versione_schema WHERE versione = 32;
COMMIT;
