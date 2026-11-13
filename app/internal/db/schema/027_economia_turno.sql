-- Economia del turno nel tracker (specifica del combattimento dell'utente, 2026-11-11, docs/PIANO_COMBATTIMENTO.md):
-- azione, azione bonus e reazione già usate dal combattente, in JSON {"a":true,"b":false,"r":true}. Si azzerano
-- all'inizio del suo turno. Gli attacchi automatici (internal/server/attacchi.go) segnano da soli l'azione (attacco nel
-- proprio turno) o la reazione (attacco fuori turno, per esempio di opportunità); le altre si segnano a mano.
-- Rollback: internal/db/rollback/027_economia_turno_rollback.sql
ALTER TABLE combattenti ADD COLUMN economia TEXT NOT NULL DEFAULT '{}';
