-- Promemoria degli eventi del Codice della campagna (fase L3, 2026-11-12): un evento con «Innesco:» (arrivo in un luogo,
-- inizio sessione, giorno di gioco) che scatta per un gruppo lascia qui un promemoria per il Master. Uno solo per scheda e
-- gruppo: chiuso (avvenuto o ignorato) non scatta più.
-- Rollback: internal/db/rollback/034_promemoria_lore_rollback.sql
CREATE TABLE lore_promemoria (
	scheda_id TEXT NOT NULL REFERENCES lore_schede(id) ON DELETE CASCADE,
	gruppo_id TEXT NOT NULL REFERENCES gruppi(id) ON DELETE CASCADE,
	motivo    TEXT NOT NULL,
	creato_il INTEGER NOT NULL,
	chiuso    INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (scheda_id, gruppo_id)
);
