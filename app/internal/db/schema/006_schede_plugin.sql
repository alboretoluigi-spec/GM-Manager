-- Schede alternative (plugin): ogni personaggio indica con quale scheda si apre.
-- '' = la scheda di Gaster Bless; altrimenti il nome della cartella del plugin in Media\SchedePlugin.
-- Rollback: internal/db/rollback/006_schede_plugin_rollback.sql
ALTER TABLE personaggi ADD COLUMN scheda TEXT NOT NULL DEFAULT '';
