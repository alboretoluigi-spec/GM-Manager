-- Portato dal ramo main-ke8sy4 (dove era la migrazione 002).
-- Manuali in PDF caricati dal Master e condivisione con i giocatori (richiesta dell'utente, 2026-11-05).
-- La condivisione vale per tutti i giocatori approvati (condiviso_tutti) oppure per quelli elencati.
ALTER TABLE manuali ADD COLUMN nome_originale TEXT NOT NULL DEFAULT '';
ALTER TABLE manuali ADD COLUMN dimensione INTEGER NOT NULL DEFAULT 0;
ALTER TABLE manuali ADD COLUMN condiviso_tutti INTEGER NOT NULL DEFAULT 0;

CREATE TABLE manuali_condivisi (
  manuale_id TEXT NOT NULL REFERENCES manuali(id) ON DELETE CASCADE,
  utente_id  TEXT NOT NULL REFERENCES utenti(id) ON DELETE CASCADE,
  dal        INTEGER NOT NULL,
  PRIMARY KEY (manuale_id, utente_id)
);
CREATE INDEX manuali_condivisi_per_utente ON manuali_condivisi(utente_id);
