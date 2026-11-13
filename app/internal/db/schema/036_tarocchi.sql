-- «🃏 Tarocchi della veggente» (richiesta dell'utente, 2026-11-13): la lettura delle carte di «l'avventura».
--   tarocchi_testi:   i testi della lettura presi dal manuale del Master (testo OCR) e corretti da lui. Chiavi pos:1…pos:5,
--                     tesoro:<carta comune>, alleato:<trionfo>[:b], antagonista:<trionfo>. «letto» = parole di la veggente
--                     (proiettore e giocatori), «note» = per il Master; modificato = 1 se corretto a mano.
--   tarocchi_letture: le letture di ogni gruppo: 5 carte (JSON, posizioni 1…5), quante rivelate, alleato «a»/«b» della
--                     carta 4, se è in corso, se la scena va anche sui dispositivi dei giocatori.
-- Le immagini delle carte stanno in Dati\file\tarocchi (caricate dal Master).
-- Rollback: internal/db/rollback/036_tarocchi_rollback.sql
CREATE TABLE tarocchi_testi (
	chiave        TEXT PRIMARY KEY,
	letto         TEXT NOT NULL DEFAULT '',
	note          TEXT NOT NULL DEFAULT '',
	modificato    INTEGER NOT NULL DEFAULT 0,
	aggiornato_il INTEGER NOT NULL
);
CREATE TABLE tarocchi_letture (
	id        TEXT PRIMARY KEY,
	gruppo_id TEXT NOT NULL REFERENCES gruppi(id) ON DELETE CASCADE,
	carte     TEXT NOT NULL,
	scelte    INTEGER NOT NULL DEFAULT 0,
	rivelate  INTEGER NOT NULL DEFAULT 0,
	variante  TEXT NOT NULL DEFAULT 'a' CHECK (variante IN ('a', 'b')),
	attiva    INTEGER NOT NULL DEFAULT 1,
	giocatori INTEGER NOT NULL DEFAULT 1,
	creato_il INTEGER NOT NULL,
	chiusa_il INTEGER
);
CREATE INDEX tarocchi_letture_gruppo ON tarocchi_letture(gruppo_id, creato_il);
