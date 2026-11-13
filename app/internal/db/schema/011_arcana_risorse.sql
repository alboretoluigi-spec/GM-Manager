-- scheda esterna, schema 2: risorse del personaggio mandate da una scheda esterna (Ki, Ira, Dadi vita…), mappa id → { max, used }.
-- Le mostra il tracker del Master durante la sessione della scheda esterna. Rollback: internal/db/rollback/011_arcana_risorse_rollback.sql
ALTER TABLE stato_in_gioco ADD COLUMN risorse_arcana TEXT NOT NULL DEFAULT '{}';
