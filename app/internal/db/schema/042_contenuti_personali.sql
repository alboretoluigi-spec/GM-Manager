-- Pacchetti personali (fase 3c, passo C2, 2026-11-13): pacchetti creati dal Master nell'app e riempiti a mano, voce per
-- voce, senza scrivere il file. Possono essere vuoti; la loro impronta porta in fondo l'id del pacchetto, così non
-- vengono mai scambiati per un doppione di un file importato.
-- Rollback: internal/db/rollback/042_contenuti_personali_rollback.sql
ALTER TABLE contenuti_pacchetti ADD COLUMN personale INTEGER NOT NULL DEFAULT 0;
ALTER TABLE contenuti_pacchetti ADD COLUMN aggiornato_il INTEGER NOT NULL DEFAULT 0;
