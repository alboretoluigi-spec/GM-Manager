-- Portato dal ramo main-ke8sy4 (dove era la migrazione 003; qui è la 005).
-- Sanità mentale, Terrore e pazzia (specifica utente 2026-11-05).
-- Solo tabelle nuove: i personaggi esistenti restano invariati. Il punteggio di Sanità mentale sta nella
-- scheda (dati del personaggio, campo "san"); qui c'è lo stato gestito dal Master e dal server.
-- Rollback: internal/db/rollback/005_sanita_rollback.sql

-- Stato per personaggio (assente = valori predefiniti sicuri: Terrore 0, nessuna competenza).
CREATE TABLE sanita_pg (
  personaggio_id    TEXT PRIMARY KEY REFERENCES personaggi(id) ON DELETE CASCADE,
  competente        INTEGER NOT NULL DEFAULT 0,          -- competenza nel TS su Sanità mentale (decide il Master)
  bonus_temp        INTEGER NOT NULL DEFAULT 0,          -- bonus/malus temporaneo al TS
  punteggio_manuale INTEGER,                             -- inserito dal Master (NULL = dalla scheda)
  terrore           INTEGER NOT NULL DEFAULT 0 CHECK (terrore BETWEEN 0 AND 7),
  immune_terrore    INTEGER NOT NULL DEFAULT 0,          -- immunità completa al sistema Terrore
  immune_spaventato INTEGER NOT NULL DEFAULT 0,          -- immune alla condizione Spaventato
  condizioni_terrore TEXT NOT NULL DEFAULT '[]',         -- condizioni aggiunte dal Terrore (per toglierle solo loro)
  modificato_il     INTEGER NOT NULL DEFAULT 0
);

-- Eventi di paura: DRAFT → READY → ROLLING → RESOLVED, oppure CANCELLED / ERROR.
CREATE TABLE eventi_paura (
  id                 TEXT PRIMARY KEY,
  sessione_id        TEXT REFERENCES sessioni_gioco(id) ON DELETE SET NULL,
  personaggio_id     TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  fonte              TEXT NOT NULL,
  descrizione        TEXT NOT NULL DEFAULT '',
  intensita          INTEGER NOT NULL CHECK (intensita BETWEEN 0 AND 5),
  cd                 INTEGER NOT NULL CHECK (cd BETWEEN 1 AND 40),
  terrore_fallimento INTEGER NOT NULL CHECK (terrore_fallimento BETWEEN 0 AND 7),
  caratteristica     TEXT NOT NULL DEFAULT 'SAN' CHECK (caratteristica IN ('SAN','SAG','CAR')),
  vantaggio          TEXT NOT NULL DEFAULT 'normale' CHECK (vantaggio IN ('normale','vantaggio','svantaggio')),
  modificatore       INTEGER NOT NULL DEFAULT 0,
  competenza         TEXT NOT NULL DEFAULT 'scheda' CHECK (competenza IN ('scheda','si','no')),
  effetto_successo   TEXT NOT NULL DEFAULT '',
  stato              TEXT NOT NULL CHECK (stato IN ('DRAFT','READY','ROLLING','RESOLVED','CANCELLED','ERROR')),
  ripete             TEXT,                               -- evento di cui questo è la ripetizione
  creato_da          TEXT,
  creato_il          INTEGER NOT NULL,
  risolto_il         INTEGER,
  risoluzione        TEXT,                               -- JSON del risultato, scritto prima dell'invio in tempo reale
  chiave             TEXT                                -- chiave di idempotenza della richiesta del giocatore
);
CREATE INDEX eventi_paura_pg ON eventi_paura(personaggio_id, creato_il);

-- Pazzie: temporanea, duratura, indeterminata (DMG 2014) e cronica (homebrew, disattivata).
CREATE TABLE pazzie (
  id              TEXT PRIMARY KEY,
  personaggio_id  TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  tipo            TEXT NOT NULL CHECK (tipo IN ('temporanea','duratura','indeterminata','cronica')),
  categoria       TEXT NOT NULL DEFAULT '',
  descrizione     TEXT NOT NULL DEFAULT '',
  tabella         TEXT NOT NULL DEFAULT 'homebrew',
  durata_valore   INTEGER NOT NULL DEFAULT 0,
  durata_unita    TEXT NOT NULL DEFAULT '',
  scade_il        INTEGER,                               -- solo pazzia temporanea (minuti reali)
  cd_recupero     INTEGER NOT NULL DEFAULT 0,
  attiva          INTEGER NOT NULL DEFAULT 1,
  evento_id       TEXT,
  creato_il       INTEGER NOT NULL,
  chiusa_il       INTEGER,
  motivo_chiusura TEXT NOT NULL DEFAULT ''
);
CREATE INDEX pazzie_pg ON pazzie(personaggio_id, attiva);

-- Catalogo dei testi delle tabelle di pazzia (scritto dal Master; vuoto all'inizio).
CREATE TABLE pazzia_catalogo (
  tabella   TEXT NOT NULL,
  categoria TEXT NOT NULL,
  voce      INTEGER NOT NULL CHECK (voce BETWEEN 1 AND 10),
  testo     TEXT NOT NULL,
  PRIMARY KEY (tabella, categoria, voce)
);

-- Registro di ogni azione e modifica (audit log).
CREATE TABLE registro_sanita (
  id             INTEGER PRIMARY KEY,
  personaggio_id TEXT,
  attore_id      TEXT,
  azione         TEXT NOT NULL,
  dati           TEXT NOT NULL DEFAULT '{}',
  creato_il      INTEGER NOT NULL
);
CREATE INDEX registro_sanita_pg ON registro_sanita(personaggio_id, id);
