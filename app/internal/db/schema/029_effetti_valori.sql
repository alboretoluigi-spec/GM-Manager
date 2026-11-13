-- Effetti che cambiano i valori (Parte 2 del piano di combattimento, F2; internal/server/effetti_valori.go, 2026-11-12):
-- modifiche che un effetto in corso dà ai suoi bersagli, in JSON (CA: bonus, base + Destrezza, minimo; resistenze e
-- immunità; immunità alle condizioni; vantaggio e bonus ai TS; dadi in più a tiri per colpire e TS; svantaggio a chi
-- attacca; danni in mischia dell'ira). Vuoto per gli effetti di prima (solo condizioni). Il valore originale resta nella
-- scheda o nel combattente: le modifiche si applicano quando si caricano i combattenti.
-- Rollback: internal/db/rollback/029_effetti_valori_rollback.sql
ALTER TABLE effetti_scontro ADD COLUMN modifiche TEXT NOT NULL DEFAULT '';
