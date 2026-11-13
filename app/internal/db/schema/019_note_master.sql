-- Appunti del Master su ogni personaggio (2026-11-09): per esempio gli agganci per l'antagonista proposti dall'assistente IA.
-- Li vede solo il Master (sezione Background, «Scheda per il Master»); l'assistente IA li legge nel contesto.
-- Rollback: internal/db/rollback/019_note_master_rollback.sql
CREATE TABLE note_master_pg (
  personaggio_id TEXT PRIMARY KEY REFERENCES personaggi(id) ON DELETE CASCADE,
  testo          TEXT NOT NULL,
  modificato_il  INTEGER NOT NULL
);
