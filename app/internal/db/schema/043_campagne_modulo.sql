-- Interfaccia adattiva (fase 3c, passo C4a, 2026-11-13): ogni campagna ha il suo modulo di gioco (registro in
-- web/sistemi/moduli.json); l'interfaccia mostra solo le sezioni di quel modulo. Le campagne già esistenti restano
-- sulla quinta edizione, come funzionavano finora. La campagna attiva è nell'impostazione «campagna_attiva».
-- Rollback: internal/db/rollback/043_campagne_modulo_rollback.sql
ALTER TABLE campagne ADD COLUMN modulo TEXT NOT NULL DEFAULT 'd20-5e';
