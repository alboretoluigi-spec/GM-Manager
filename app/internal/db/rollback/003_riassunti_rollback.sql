-- Rollback della migrazione 003 (Riassunti; nel ramo main-ke8sy4 era la 004).
-- Da eseguire SOLO a server fermo, dopo un backup di gasterbless.db, con: sqlite3 gasterbless.db < 003_riassunti_rollback.sql
-- I riassunti scritti vanno persi; il resto del database resta intatto.
BEGIN;
DROP TABLE IF EXISTS riassunti;
DELETE FROM versione_schema WHERE versione = 3;
COMMIT;
