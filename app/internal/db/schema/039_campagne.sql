-- Campagne (docs/RULEBOOK_LIBRARY.md, passo L4, 2026-11-13): ogni campagna ha sistema di gioco ed edizione (testo libero:
-- nessun sistema è obbligatorio per il motore), regolamento principale, documenti supplementari e schede personalizzate
-- (da MY RULEBOOKS; lo stesso documento può stare in più campagne), regole aggiuntive del Master e impostazioni (JSON).
-- I gruppi esistenti vanno nella «Campagna principale», creata qui; un gruppo nuovo senza campagna va nella prima
-- campagna creata (trigger). Rollback: internal/db/rollback/039_campagne_rollback.sql
CREATE TABLE campagne (
  id             TEXT PRIMARY KEY,
  nome           TEXT NOT NULL UNIQUE,
  descrizione    TEXT NOT NULL DEFAULT '',
  sistema        TEXT NOT NULL DEFAULT '',
  edizione       TEXT NOT NULL DEFAULT '',
  regolamento_id TEXT REFERENCES regolamenti_documenti(id) ON DELETE SET NULL,
  regole_master  TEXT NOT NULL DEFAULT '',
  impostazioni   TEXT NOT NULL DEFAULT '{}',
  creato_il      INTEGER NOT NULL,
  aggiornato_il  INTEGER NOT NULL
);
CREATE TABLE campagna_documenti (
  campagna_id  TEXT NOT NULL REFERENCES campagne(id) ON DELETE CASCADE,
  documento_id TEXT NOT NULL REFERENCES regolamenti_documenti(id) ON DELETE CASCADE,
  ruolo        TEXT NOT NULL CHECK (ruolo IN ('supplementare', 'scheda')),
  PRIMARY KEY (campagna_id, documento_id, ruolo)
);
CREATE INDEX campagna_documenti_doc ON campagna_documenti(documento_id);
INSERT INTO campagne (id, nome, creato_il, aggiornato_il) VALUES ('principale', 'Campagna principale', strftime('%s', 'now'), strftime('%s', 'now'));
ALTER TABLE gruppi ADD COLUMN campagna_id TEXT REFERENCES campagne(id) ON DELETE SET NULL;
UPDATE gruppi SET campagna_id = 'principale';
CREATE TRIGGER gruppi_campagna_predefinita AFTER INSERT ON gruppi WHEN NEW.campagna_id IS NULL
BEGIN
  UPDATE gruppi SET campagna_id = (SELECT id FROM campagne ORDER BY creato_il, id LIMIT 1) WHERE id = NEW.id;
END;
