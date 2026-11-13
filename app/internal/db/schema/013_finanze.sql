-- Finanze (2026-11-07): richieste di modifica manuale delle monete (le autorizza il Master), registro dei movimenti
-- (acquisti, trasferimenti, ricompense, professioni) e Grado di Sfida dei mostri del tracker per le ricompense.
-- Rollback: internal/db/rollback/013_finanze_rollback.sql
CREATE TABLE richieste_monete (
  id             TEXT PRIMARY KEY,
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  utente_id      TEXT,
  prima          TEXT NOT NULL,          -- {"MR":..,"MA":..,"ME":..,"MO":..,"MP":..}
  dopo           TEXT NOT NULL,
  stato          TEXT NOT NULL DEFAULT 'in_attesa' CHECK (stato IN ('in_attesa','autorizzata','rifiutata','superata')),
  creato_il      INTEGER NOT NULL,
  deciso_il      INTEGER
);
CREATE INDEX richieste_monete_stato ON richieste_monete(stato, creato_il);

CREATE TABLE registro_finanze (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  attore_id      TEXT,
  tipo           TEXT NOT NULL,          -- modifica, acquisto, trasferimento, ricompensa, professione, master
  importo        INTEGER NOT NULL,       -- in monete di rame, positivo = entrata
  testo          TEXT NOT NULL,
  creato_il      INTEGER NOT NULL
);
CREATE INDEX registro_finanze_pg ON registro_finanze(personaggio_id, id);

ALTER TABLE combattenti ADD COLUMN gs REAL NOT NULL DEFAULT 0;          -- Grado di Sfida del mostro
ALTER TABLE combattenti ADD COLUMN ricompensato INTEGER NOT NULL DEFAULT 0;
