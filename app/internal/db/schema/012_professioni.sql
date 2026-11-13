-- Professioni (raccolta e fabbricazione): inventario per personaggio, formule conosciute, contatori del giorno di
-- gioco e registro. Catalogo in web/regole/professioni.json. Rollback: internal/db/rollback/012_professioni_rollback.sql
CREATE TABLE prof_inventario (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  professione    TEXT NOT NULL,
  tipo           TEXT NOT NULL CHECK (tipo IN ('ingrediente','secrezione','speciale','dose','fiala','tana')),
  voce           TEXT NOT NULL,              -- id dell'ingrediente, della secrezione o della formula (dosi)
  variante       TEXT NOT NULL DEFAULT '',   -- per le dosi: ingrediente facoltativo usato (ectoplasma…)
  quantita       INTEGER NOT NULL,
  giorni         INTEGER,                    -- giorni di gioco rimasti (NULL = non scade)
  creato_il      INTEGER NOT NULL
);
CREATE INDEX prof_inventario_pg ON prof_inventario(personaggio_id, professione);

CREATE TABLE prof_formule (               -- formule sbloccate dal Master (le ricette ufficiali sono già note)
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  formula        TEXT NOT NULL,
  PRIMARY KEY (personaggio_id, formula)
);

CREATE TABLE prof_giorno (                -- contatori del giorno di gioco (si azzerano con «Passa un giorno»)
  personaggio_id TEXT PRIMARY KEY REFERENCES personaggi(id) ON DELETE CASCADE,
  viaggio        INTEGER NOT NULL DEFAULT 0,  -- raccolte in viaggio (1 al giorno)
  dedicate       INTEGER NOT NULL DEFAULT 0,  -- ricerche dedicate (4 al giorno)
  lunghe         INTEGER NOT NULL DEFAULT 0   -- preparazioni da 8 ore (1 al giorno)
);

CREATE TABLE prof_registro (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  attore_id      TEXT,
  professione    TEXT NOT NULL,
  azione         TEXT NOT NULL,
  testo          TEXT NOT NULL,
  creato_il      INTEGER NOT NULL
);
CREATE INDEX prof_registro_pg ON prof_registro(personaggio_id, id);
