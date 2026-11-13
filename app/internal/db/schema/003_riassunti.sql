-- Riassunti delle sessioni scritti dai giocatori (richiesta utente 2026-11-05).
-- Ogni riassunto appartiene a un gruppo (lo leggono tutti i suoi giocatori e il Master) ed è firmato dal suo autore.
-- Rollback: internal/db/rollback/003_riassunti_rollback.sql (portato dal ramo main-ke8sy4, dove era il n. 4)
CREATE TABLE riassunti (
  id            TEXT PRIMARY KEY,
  gruppo_id     TEXT NOT NULL REFERENCES gruppi(id) ON DELETE CASCADE,
  sessione_id   TEXT REFERENCES sessioni_gioco(id) ON DELETE SET NULL,
  autore_id     TEXT REFERENCES utenti(id) ON DELETE SET NULL,
  autore_nome   TEXT NOT NULL,           -- firma "Scriba": resta anche se l'account viene cancellato
  titolo        TEXT NOT NULL DEFAULT '',
  testo         TEXT NOT NULL,
  creato_il     INTEGER NOT NULL,
  modificato_il INTEGER NOT NULL
);
CREATE INDEX riassunti_per_gruppo ON riassunti(gruppo_id, creato_il);
