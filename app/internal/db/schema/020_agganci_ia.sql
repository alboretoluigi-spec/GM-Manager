-- Agganci per l'antagonista scritti dall'assistente IA per ogni background (2026-11-09): sostituiscono l'analisi automatica a
-- temi fissi. Si rigenerano da soli quando cambiano il background, la scheda o i riassunti del gruppo.
-- Rollback: internal/db/rollback/020_agganci_ia_rollback.sql
CREATE TABLE ia_agganci_bg (
  personaggio_id TEXT PRIMARY KEY REFERENCES personaggi(id) ON DELETE CASCADE,
  testo          TEXT NOT NULL,              -- testo formattato (# titoli, - elenchi)
  fornitore      TEXT NOT NULL DEFAULT '',   -- servizio e modello che l'hanno scritto
  impronta       TEXT NOT NULL DEFAULT '',   -- impronta dei dati usati: se cambia, gli agganci vanno rifatti
  generato_il    INTEGER NOT NULL
);
-- Dopo ogni riassunto nuovo l'IA scrive da sola una conversazione per il Master («idee per l'antagonista»): resta segnata
-- come nuova finché il Master non la apre.
ALTER TABLE ia_conversazioni ADD COLUMN nuova INTEGER NOT NULL DEFAULT 0;
