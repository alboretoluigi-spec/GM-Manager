-- Codice della campagna (richiesta dell'utente del 2026-11-12, fase L1): la lore scritta dal Master (anche con ChatGPT) in
-- schede collegate: luoghi, PNG, fazioni, eventi, trame, oggetti, segreti, cronologia. Solo il Master.
--   testo_master / testo_giocatori: ciò che sa il Master (segreti compresi) e ciò che possono sapere i giocatori
--   campi:        coppie «chiave: valore» (luogo, fazione, quando…), JSON
--   collegamenti: nomi di altre schede, JSON
--   etichette:    parole chiave, JSON
-- lore_stato: a che punto è ogni gruppo con quella scheda (conosciuto, incontrato, concluso), con una nota.
-- Rollback: internal/db/rollback/033_codice_campagna_rollback.sql
CREATE TABLE lore_schede (
	id              TEXT PRIMARY KEY,
	tipo            TEXT NOT NULL CHECK (tipo IN ('luogo', 'png', 'fazione', 'evento', 'trama', 'oggetto', 'segreto', 'cronologia')),
	nome            TEXT NOT NULL,
	testo_master    TEXT NOT NULL DEFAULT '',
	testo_giocatori TEXT NOT NULL DEFAULT '',
	campi           TEXT NOT NULL DEFAULT '{}',
	collegamenti    TEXT NOT NULL DEFAULT '[]',
	etichette       TEXT NOT NULL DEFAULT '[]',
	creato_il       INTEGER NOT NULL,
	modificato_il   INTEGER NOT NULL
);
CREATE UNIQUE INDEX lore_schede_nome ON lore_schede(tipo, nome COLLATE NOCASE);
CREATE TABLE lore_stato (
	scheda_id     TEXT NOT NULL REFERENCES lore_schede(id) ON DELETE CASCADE,
	gruppo_id     TEXT NOT NULL REFERENCES gruppi(id) ON DELETE CASCADE,
	stato         TEXT NOT NULL CHECK (stato IN ('conosciuto', 'incontrato', 'concluso')),
	nota          TEXT NOT NULL DEFAULT '',
	modificato_il INTEGER NOT NULL,
	PRIMARY KEY (scheda_id, gruppo_id)
);
