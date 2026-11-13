-- «🧛 Parla con l'antagonista» (richiesta dell'utente, 2026-11-09): accanto all'Assistente IA del Master, conversazioni in cui
-- l'IA interpreta l'antagonista con il system prompt scritto dal Master (impostazione «ia_prompt_antagonista»; se vuota vale il
-- testo predefinito in internal/server/prompt_antagonista.md). modo: 'assistente' (predefinito) o 'antagonista'.
-- Rollback: internal/db/rollback/021_ia_antagonista_rollback.sql
ALTER TABLE ia_conversazioni ADD COLUMN modo TEXT NOT NULL DEFAULT 'assistente';
