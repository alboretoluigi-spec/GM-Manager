-- Modifica manuale della scheda (2026-11-11): con l'approvazione attiva il giocatore cambia da solo solo i campi di
-- gioco; il resto lo modifica con «Modifica manuale» e, quando la spegne, le modifiche arrivano al Master come
-- richiesta. Il Master approva o rifiuta voce per voce; le voci approvate diventano una nuova versione della scheda.
-- Rollback: internal/db/rollback/022_modifiche_manuali_rollback.sql
CREATE TABLE richieste_scheda (
  id             TEXT PRIMARY KEY,
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  utente_id      TEXT,                    -- giocatore che ha chiesto le modifiche
  voci           TEXT NOT NULL,           -- in attesa: [{"campo":..,"prima":..,"dopo":..,"rimosso":bool}]
  esiti          TEXT NOT NULL DEFAULT '[]', -- già decise: [{"campo":..,"prima":..,"dopo":..,"rimosso":bool,"esito":"approvata"|"rifiutata"}]
  stato          TEXT NOT NULL DEFAULT 'in_attesa' CHECK (stato IN ('in_attesa','decisa','annullata')),
  creato_il      INTEGER NOT NULL,
  aggiornato_il  INTEGER NOT NULL,
  deciso_il      INTEGER,
  deciso_da      TEXT
);
CREATE INDEX richieste_scheda_pg ON richieste_scheda(personaggio_id, stato);
CREATE INDEX richieste_scheda_stato ON richieste_scheda(stato, aggiornato_il);
