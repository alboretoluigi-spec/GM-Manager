-- Scheda libera (fase 3c, passo C4b, 2026-11-13): ogni campagna può avere un modello di scheda (sezioni, campi, tiri,
-- punti ferita, difesa, iniziativa) deciso dal Master, nel formato descritto in docs/SCHEDA_LIBERA.md. Lo usano i
-- personaggi delle campagne con un gioco diverso dalla quinta edizione. Vuoto = modello di base dell'app.
-- Rollback: internal/db/rollback/044_modello_scheda_rollback.sql
ALTER TABLE campagne ADD COLUMN modello_scheda TEXT NOT NULL DEFAULT '';
-- Un gruppo nuovo senza campagna va nella campagna attiva (scelta dal Master all'accesso, C4a); se non c'è, nella prima.
DROP TRIGGER gruppi_campagna_predefinita;
CREATE TRIGGER gruppi_campagna_predefinita AFTER INSERT ON gruppi WHEN NEW.campagna_id IS NULL
BEGIN
  UPDATE gruppi SET campagna_id = COALESCE(
    (SELECT c.id FROM impostazioni i JOIN campagne c ON c.id = i.valore WHERE i.chiave = 'campagna_attiva'),
    (SELECT id FROM campagne ORDER BY creato_il, id LIMIT 1)) WHERE id = NEW.id;
END;
