-- Riassunti firmati dal personaggio, «Ottimo lavoro» del Master; Background in testo (2026-11-09).
-- Rollback: internal/db/rollback/017_riassunti_background_rollback.sql
ALTER TABLE riassunti ADD COLUMN personaggio_id TEXT REFERENCES personaggi(id) ON DELETE SET NULL; -- chi lo firma
ALTER TABLE riassunti ADD COLUMN ottimo INTEGER NOT NULL DEFAULT 0;   -- «Ottimo lavoro» del Master (dà l'ispirazione)
-- Background del personaggio in testo: lo vedono il giocatore, il Master e chi il giocatore sceglie.
CREATE TABLE background_testi (
  personaggio_id TEXT PRIMARY KEY REFERENCES personaggi(id) ON DELETE CASCADE,
  titolo         TEXT NOT NULL DEFAULT '',
  testo          TEXT NOT NULL,              -- testo con una formattazione leggera (# titoli, - elenchi, paragrafi)
  origine        TEXT NOT NULL DEFAULT '',   -- nome del file caricato, se c'è
  condiviso      TEXT NOT NULL DEFAULT '[]', -- ["tutti"] oppure id degli utenti con cui è condiviso
  creato_il      INTEGER NOT NULL,
  modificato_il  INTEGER NOT NULL
);
