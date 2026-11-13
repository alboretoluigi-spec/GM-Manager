-- Integrazione con una scheda esterna.
-- arcana_codici: codici di abbinamento monouso (solo l'impronta SHA-256), validi 15 minuti.
-- arcana_dispositivi: un dispositivo abbinato a un personaggio; token salvato solo come impronta.
--   sessione_dal > 0 = sessione della scheda esterna aperta: i PF del personaggio li comanda una scheda esterna.
-- arcana_eventi: id degli eventi già ricevuti, per riconoscere i doppioni (si cancellano dopo 7 giorni).
-- Rollback: internal/db/rollback/009_arcana_rollback.sql
CREATE TABLE arcana_codici (
  impronta       TEXT PRIMARY KEY,
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  creato_il      INTEGER NOT NULL,
  scade_il       INTEGER NOT NULL
);
CREATE TABLE arcana_dispositivi (
  id             TEXT PRIMARY KEY,
  personaggio_id TEXT NOT NULL REFERENCES personaggi(id) ON DELETE CASCADE,
  token          TEXT NOT NULL UNIQUE,
  dispositivo    TEXT NOT NULL,
  nome           TEXT NOT NULL,
  creato_il      INTEGER NOT NULL,
  ultimo_il      INTEGER NOT NULL DEFAULT 0,
  sessione_dal   INTEGER NOT NULL DEFAULT 0,
  seq            INTEGER NOT NULL DEFAULT -1
);
CREATE INDEX arcana_dispositivi_pg ON arcana_dispositivi(personaggio_id);
CREATE TABLE arcana_eventi (
  dispositivo_id TEXT NOT NULL REFERENCES arcana_dispositivi(id) ON DELETE CASCADE,
  evento         TEXT NOT NULL,
  ricevuto_il    INTEGER NOT NULL,
  PRIMARY KEY (dispositivo_id, evento)
);
