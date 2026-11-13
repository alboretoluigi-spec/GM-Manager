-- Rollback della migrazione 012 (professioni). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 012_professioni_rollback.sql
-- Si perdono inventari delle professioni (ingredienti, secrezioni, dosi), formule sbloccate e registro.
-- Monete e oggetti già messi nella scheda restano.
BEGIN;
DROP TABLE IF EXISTS prof_registro;
DROP TABLE IF EXISTS prof_giorno;
DROP TABLE IF EXISTS prof_formule;
DROP TABLE IF EXISTS prof_inventario;
DELETE FROM versione_schema WHERE versione = 12;
COMMIT;
