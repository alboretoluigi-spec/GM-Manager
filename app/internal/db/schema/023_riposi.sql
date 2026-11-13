-- Riposo lungo e riposo breve dalla barra in alto (2026-11-11): il giocatore chiede il riposo per sé e per i compagni
-- che sceglie; il Master approva (o rifiuta); approvato, gli effetti valgono per tutti i partecipanti e passa il tempo
-- di gioco del gruppo (8 ore o 1 ora). Il riposo breve chiede poi a ognuno quanti Dadi Vita spendere.
-- Rollback: internal/db/rollback/023_riposi_rollback.sql
CREATE TABLE riposi (
  id          TEXT PRIMARY KEY,
  gruppo_id   TEXT NOT NULL REFERENCES gruppi(id) ON DELETE CASCADE,
  tipo        TEXT NOT NULL CHECK (tipo IN ('lungo','breve')),
  utente_id   TEXT,                       -- chi lo ha chiesto
  personaggi  TEXT NOT NULL,              -- ["id", ...] partecipanti
  stato       TEXT NOT NULL DEFAULT 'in_attesa' CHECK (stato IN ('in_attesa','approvato','rifiutato','annullato')),
  esito       TEXT NOT NULL DEFAULT '',   -- testo del risultato (per giocatori e Master)
  dadi_spesi  TEXT NOT NULL DEFAULT '{}', -- riposo breve: {"id personaggio": dadi vita spesi} (una volta per riposo)
  creato_il   INTEGER NOT NULL,
  deciso_il   INTEGER,
  deciso_da   TEXT
);
CREATE INDEX riposi_gruppo ON riposi(gruppo_id, stato);
CREATE INDEX riposi_stato ON riposi(stato, creato_il);
-- Ore di gioco del gruppo alla fine dell'ultimo riposo lungo (regola: al massimo uno ogni 24 ore di gioco). NULL = mai.
ALTER TABLE personaggi ADD COLUMN ultimo_riposo_lungo REAL;
