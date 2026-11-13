-- «📥 Importa dai manuali» (2026-11-11, docs/PIANO_IMPORTA_MANUALI.md): elementi letti dai PDF del Master (mostri e PNG,
-- incantesimi, oggetti magici, regole), con manuale e pagina di provenienza. Restano anche se il manuale viene
-- eliminato (manuale_id diventa NULL, manuale_nome resta). Il testo del manuale lo vede solo il Master.
-- Rollback: internal/db/rollback/024_elementi_manuali_rollback.sql
CREATE TABLE elementi_manuali (
  id            TEXT PRIMARY KEY,
  tipo          TEXT NOT NULL CHECK (tipo IN ('mostro','incantesimo','oggetto','regola')),
  sottotipo     TEXT NOT NULL DEFAULT '',
  chiave        TEXT NOT NULL,             -- nome normalizzato (minuscole, senza accenti)
  nome          TEXT NOT NULL,
  voce          TEXT NOT NULL DEFAULT '',  -- a cosa è applicato in Gaster Bless (per i mostri: id della voce del bestiario)
  manuale_id    TEXT,
  manuale_nome  TEXT NOT NULL DEFAULT '',
  pagina        INTEGER NOT NULL DEFAULT 0,
  dati          TEXT NOT NULL DEFAULT '{}', -- valori letti (JSON)
  testo         TEXT NOT NULL DEFAULT '',   -- testo letto, solo per il Master
  impronta      TEXT NOT NULL DEFAULT '',   -- impronta dei valori applicati (per sapere se poi sono stati cambiati a mano)
  importato_il  INTEGER NOT NULL,
  aggiornato_il INTEGER NOT NULL,
  UNIQUE (tipo, chiave)
);
CREATE INDEX elementi_manuali_manuale ON elementi_manuali(manuale_id);
-- L'ultimo riepilogo di un'importazione, per riprenderlo dopo aver chiuso la pagina.
CREATE TABLE importazioni (
  id         TEXT PRIMARY KEY,
  creata_il  INTEGER NOT NULL,
  voci       TEXT NOT NULL                -- JSON: candidati con stato e confronto
);
