-- Immagine di ogni gruppo, mostrata accanto al nome nell'elenco delle sessioni.
-- immagine: nome del file nella cartella dei dati ('' = nessuna immagine).
-- Rollback: internal/db/rollback/008_immagine_gruppo_rollback.sql
ALTER TABLE gruppi ADD COLUMN immagine TEXT NOT NULL DEFAULT '';
