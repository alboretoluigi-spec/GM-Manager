-- Controllo delle schede all'accesso in sessione (2026-11-07): le versioni fatte fuori sessione o senza rete sono
-- segnate; all'avvio della sessione (e quando il giocatore si collega o conferma la presenza) si confrontano i campi
-- importanti con l'ultimo controllo e, solo se c'è qualcosa, il Master riceve un avviso.
-- Rollback: internal/db/rollback/014_controllo_schede_rollback.sql
ALTER TABLE schede_versioni ADD COLUMN offline INTEGER NOT NULL DEFAULT 0;         -- modifiche fatte senza rete
ALTER TABLE schede_versioni ADD COLUMN fuori_sessione INTEGER NOT NULL DEFAULT 0;  -- dal giocatore, nessuna sessione in corso
ALTER TABLE personaggi ADD COLUMN versione_controllata INTEGER NOT NULL DEFAULT 0;  -- versione dell'ultimo controllo
UPDATE personaggi SET versione_controllata = versione_corrente;                     -- le schede di oggi sono il punto di partenza

CREATE TABLE controlli_scheda (
  id             TEXT PRIMARY KEY,
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  utente_id      TEXT,                   -- giocatore proprietario al momento del controllo
  sessione_id    TEXT,
  versione_da    INTEGER NOT NULL,
  versione_a     INTEGER NOT NULL,
  offline        INTEGER NOT NULL DEFAULT 0,
  differenze     TEXT NOT NULL,          -- [{"campo":..,"prima":..,"dopo":..,"offline":bool}]
  stato          TEXT NOT NULL DEFAULT 'aperto' CHECK (stato IN ('aperto','accettato','ripristinato','superato')),
  creato_il      INTEGER NOT NULL,
  deciso_il      INTEGER
);
CREATE INDEX controlli_scheda_stato ON controlli_scheda(stato, creato_il);
