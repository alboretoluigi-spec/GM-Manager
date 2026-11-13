-- Posizione del gruppo sulla mappa (segnalino con i ritratti dei PG), per ogni gruppo.
-- Valore JSON {"x":..,"y":..} in pixel della mappa, oppure '' se non posizionato.
ALTER TABLE stato_mappa ADD COLUMN posizione TEXT NOT NULL DEFAULT '';
