-- Attacchi automatici dal tracker (specifica dell'utente del 2026-11-11, docs/PIANO_COMBATTIMENTO.md, fase B).
-- profilo: per i mostri, attacchi e difese letti dalla loro scheda quando il Master li aggiunge (JSON, vedi
--          internal/server/attacchi.go); i PG usano gli attacchi calcolati dalla scheda (riepilogo).
-- attacchi_usati / attacchi_turno: quanti attacchi il combattente ha già fatto nel turno numero attacchi_turno
--          (si azzera da solo quando il turno cambia).
-- Rollback: internal/db/rollback/026_attacchi_automatici_rollback.sql
ALTER TABLE combattenti ADD COLUMN profilo TEXT NOT NULL DEFAULT '';
ALTER TABLE combattenti ADD COLUMN attacchi_usati INTEGER NOT NULL DEFAULT 0;
ALTER TABLE combattenti ADD COLUMN attacchi_turno INTEGER NOT NULL DEFAULT -1;
