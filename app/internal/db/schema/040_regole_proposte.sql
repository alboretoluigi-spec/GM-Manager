-- Analisi dei regolamenti (docs/RULEBOOK_LIBRARY.md, passo L5, 2026-11-13): regole individuate nei documenti di MY RULEBOOKS
-- (da un analizzatore o scritte dal Master) nelle 13 categorie. Restano «proposta» finché il Master non le approva o le
-- rifiuta: solo quelle approvate potranno essere usate dal motore. Ogni proposta tiene il riferimento alla fonte
-- (documento, titolo copiato, pagina o punto) anche se il documento viene eliminato.
-- Rollback: internal/db/rollback/040_regole_proposte_rollback.sql
CREATE TABLE regole_proposte (
  id               TEXT PRIMARY KEY,
  campagna_id      TEXT REFERENCES campagne(id) ON DELETE CASCADE,
  documento_id     TEXT REFERENCES regolamenti_documenti(id) ON DELETE SET NULL,
  documento_titolo TEXT NOT NULL DEFAULT '',
  pagina           INTEGER NOT NULL DEFAULT 0,
  parte            INTEGER NOT NULL DEFAULT -1,
  dove             TEXT NOT NULL DEFAULT '',
  categoria        TEXT NOT NULL,
  nome             TEXT NOT NULL,
  testo            TEXT NOT NULL DEFAULT '',
  estratto         TEXT NOT NULL DEFAULT '',
  dati             TEXT NOT NULL DEFAULT '{}',
  origine          TEXT NOT NULL CHECK (origine IN ('manuale', 'parole_chiave', 'ia')),
  stato            TEXT NOT NULL DEFAULT 'proposta' CHECK (stato IN ('proposta', 'approvata', 'rifiutata')),
  nota             TEXT NOT NULL DEFAULT '',
  creato_il        INTEGER NOT NULL,
  deciso_il        INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX regole_proposte_doc ON regole_proposte(documento_id, categoria);
CREATE INDEX regole_proposte_stato ON regole_proposte(stato, categoria);
