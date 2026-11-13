-- Assistente IA del Master (2026-11-09): conversazioni salvate sul PC.
-- Rollback: internal/db/rollback/018_assistente_rollback.sql
-- Le impostazioni (fornitore, modello, indirizzo) stanno nella tabella impostazioni; le chiavi dei servizi online sono
-- cifrate con la chiave di Gaster Bless (impostazioni «ia_chiave_<fornitore>»).
CREATE TABLE ia_conversazioni (
  id            TEXT PRIMARY KEY,
  gruppo_id     TEXT REFERENCES gruppi(id) ON DELETE SET NULL, -- gruppo di cui l'IA conosceva la situazione
  titolo        TEXT NOT NULL DEFAULT '',
  messaggi      TEXT NOT NULL DEFAULT '[]', -- [{"ruolo":"master"|"ia","testo":"...","fornitore":"...","il":...}]
  creato_il     INTEGER NOT NULL,
  modificato_il INTEGER NOT NULL
);
CREATE INDEX ia_conversazioni_mod ON ia_conversazioni(modificato_il);
