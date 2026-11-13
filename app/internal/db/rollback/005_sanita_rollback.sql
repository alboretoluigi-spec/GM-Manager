-- Rollback della migrazione 005 (nel ramo main-ke8sy4 era la 003) (Sanità mentale, Terrore e pazzia).
-- Da eseguire SOLO a server fermo, dopo un backup di gasterbless.db, con: sqlite3 gasterbless.db < 005_sanita_rollback.sql
-- Elimina le tabelle nuove e la riga della versione: i dati di Sanità, Terrore e pazzia vanno persi,
-- il resto del database resta intatto. Il campo "san" eventualmente presente nelle schede viene ignorato.
BEGIN;
DROP TABLE IF EXISTS registro_sanita;
DROP TABLE IF EXISTS pazzia_catalogo;
DROP TABLE IF EXISTS pazzie;
DROP TABLE IF EXISTS eventi_paura;
DROP TABLE IF EXISTS sanita_pg;
DELETE FROM versione_schema WHERE versione = 5;
COMMIT;
