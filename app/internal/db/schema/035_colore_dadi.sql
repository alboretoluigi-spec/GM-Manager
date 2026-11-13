-- Colore del set di dadi di ogni utente (dadi 3D, 2026-11-12): «#corpo/#numeri» o vuoto (set predefinito). Chi vede un
-- tiro lo vede con i dadi del colore di chi ha tirato.
-- Rollback: internal/db/rollback/035_colore_dadi_rollback.sql
ALTER TABLE utenti ADD COLUMN colore_dadi TEXT NOT NULL DEFAULT '';
