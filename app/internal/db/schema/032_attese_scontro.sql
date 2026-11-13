-- Scontri lasciati a metà (decisione dell'utente del 2026-11-12): il colpo critico che aspetta la scelta del modo e i
-- tiri salvezza che aspettano la conferma del Master stanno anche qui, così un riavvio di Gaster Bless non li perde.
--   gettone:          lo stesso della pagina (monouso: la riga si cancella quando si usa)
--   tipo:             «critico» o «ts»
--   combattimento_id: lo scontro a cui appartiene
--   dati:             JSON con tutto ciò che serve per riprendere (internal/server/attese_scontro.go)
--   scade:            istante (secondi Unix) oltre il quale non vale più (20 minuti il critico, 30 i tiri salvezza)
-- Rollback: internal/db/rollback/032_attese_scontro_rollback.sql
CREATE TABLE attese_scontro (
	gettone          TEXT PRIMARY KEY,
	tipo             TEXT NOT NULL CHECK (tipo IN ('critico', 'ts')),
	combattimento_id TEXT NOT NULL,
	dati             TEXT NOT NULL,
	scade            INTEGER NOT NULL
);
CREATE INDEX attese_scontro_comb ON attese_scontro(combattimento_id);
