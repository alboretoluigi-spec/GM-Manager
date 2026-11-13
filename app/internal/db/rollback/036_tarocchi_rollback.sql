-- Rollback della migrazione 036 (tarocchi di la veggente). Da eseguire SOLO a server fermo, dopo un backup:
--   sqlite3 gasterbless.db < 036_tarocchi_rollback.sql
-- Le immagini delle carte in Dati\file\tarocchi restano: cancellarle a mano se non servono più.
BEGIN;
DROP TABLE tarocchi_letture;
DROP TABLE tarocchi_testi;
DELETE FROM versione_schema WHERE versione = 36;
COMMIT;
