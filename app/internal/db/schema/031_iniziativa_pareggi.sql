-- Tracker dell'iniziativa (richieste dell'utente del 2026-11-12):
--   iniz_naturale: il d20 del tiro d'iniziativa (NULL se il valore è stato scritto a mano senza indicarlo); con 20 il
--                  combattente agisce per primo, con 1 per ultimo;
--   ordine_pari:   ordine deciso a mano nei pareggi (il Master tra PG e mostri/PNG, i giocatori tra i loro PG): più alto prima;
--   scheda_mostro: la scheda intera della creatura dal bestiario (tratti, azioni…), per la finestra del mostro.
-- Rollback: internal/db/rollback/031_iniziativa_pareggi_rollback.sql
ALTER TABLE combattenti ADD COLUMN iniz_naturale INTEGER;
ALTER TABLE combattenti ADD COLUMN ordine_pari INTEGER NOT NULL DEFAULT 0;
ALTER TABLE combattenti ADD COLUMN scheda_mostro TEXT NOT NULL DEFAULT '';
