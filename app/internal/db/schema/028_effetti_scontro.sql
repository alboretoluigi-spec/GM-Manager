-- Effetti in corso nello scontro (Parte 2 del piano di combattimento: effetti persistenti; internal/server/effetti_scontro.go).
-- Un incantesimo che lascia condizioni (Blocca persone: paralizzato) diventa un effetto: chi lo ha lanciato (combattente),
-- i bersagli con le condizioni che ha messo davvero, se dipende dalla concentrazione, il round in cui finisce (0 = finché
-- non si termina) e il tiro salvezza che i bersagli ripetono alla fine di ogni loro turno. Finendo, l'effetto toglie solo
-- le condizioni che aveva messo.
-- Rollback: internal/db/rollback/028_effetti_scontro_rollback.sql
CREATE TABLE effetti_scontro (
  id               TEXT PRIMARY KEY,
  combattimento_id TEXT NOT NULL REFERENCES combattimenti(id) ON DELETE CASCADE,
  incantatore_id   TEXT NOT NULL,
  nome             TEXT NOT NULL,
  bersagli         TEXT NOT NULL DEFAULT '[]',
  conc             INTEGER NOT NULL DEFAULT 0,
  inizio_round     INTEGER NOT NULL DEFAULT 0,
  fine_round       INTEGER NOT NULL DEFAULT 0,
  ts               TEXT NOT NULL DEFAULT '',
  creato_il        INTEGER NOT NULL
);
CREATE INDEX effetti_scontro_combattimento ON effetti_scontro(combattimento_id);
