-- Documenti (2026-11-07): file PDF, Word e immagini caricati dal Master e dai giocatori. Ogni giocatore vede solo i suoi;
-- il Master vede le cartelle di tutti. Un giocatore può consegnare un suo documento a un altro giocatore: il destinatario
-- riceve una copia (con la notifica), il mittente ne conserva una copia nella sottocartella «Backup».
-- Rollback: internal/db/rollback/015_documenti_rollback.sql
CREATE TABLE documenti (
  id              TEXT PRIMARY KEY,
  proprietario_id TEXT NOT NULL REFERENCES utenti(id) ON DELETE CASCADE,
  cartella        TEXT NOT NULL DEFAULT '' CHECK (cartella IN ('','backup')),
  nome            TEXT NOT NULL,          -- nome mostrato (scelto da chi carica, modificabile)
  tipo            TEXT NOT NULL,          -- pdf, docx, doc, png, jpg, gif, webp
  dimensione      INTEGER NOT NULL,
  file            TEXT NOT NULL,          -- nome del file cifrato nella cartella dati
  da_utente_id    TEXT,                   -- chi l'ha consegnato (documento ricevuto)
  da_personaggio  TEXT NOT NULL DEFAULT '',
  a_utente_id     TEXT,                   -- (Backup) a chi è stato consegnato
  visto           INTEGER NOT NULL DEFAULT 1,  -- 0 = ricevuto e non ancora aperto: notifica al prossimo accesso
  creato_il       INTEGER NOT NULL,
  modificato_il   INTEGER NOT NULL
);
CREATE INDEX documenti_proprietario ON documenti(proprietario_id, cartella);
