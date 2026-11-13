-- Raccolta dei PNG per «🎭 PNG in scena» (richiesta dell'utente, 2026-11-13): «ufficiale» (raccolta principale)
-- o «reloaded» (raccolta alternativa). Vuoto: ufficiale per i PNG del bestiario di Gaster Bless e per quelli nuovi del Master.
-- Rollback: internal/db/rollback/037_raccolta_png_rollback.sql
ALTER TABLE bestiario_personalizzazioni ADD COLUMN raccolta TEXT NOT NULL DEFAULT '';
