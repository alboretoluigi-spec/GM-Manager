-- Sezione «PNG e mostri»: scheda strutturata (statistiche) per ogni voce e modello dei PNG creati dal Master.
-- scheda_dati: JSON nel formato di web/carta/bestiario.json (taglia, tipo, ca, pf, car, tratti, azioni…);
--   '' = si usa la scheda di base. base: voce del bestiario usata come modello (per esempio mm-noble).
-- Rollback: internal/db/rollback/007_bestiario_schede_rollback.sql
ALTER TABLE bestiario_personalizzazioni ADD COLUMN scheda_dati TEXT NOT NULL DEFAULT '';
ALTER TABLE bestiario_voci ADD COLUMN base TEXT NOT NULL DEFAULT '';
