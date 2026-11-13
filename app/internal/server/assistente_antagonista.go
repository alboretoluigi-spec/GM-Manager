package server

// «🧛 Parla con l'antagonista» (richiesta dell'utente, 2026-11-09). Le conversazioni con modo 'antagonista' usano, al posto delle
// istruzioni dell'Assistente del Master, il system prompt dell'antagonista: quello scritto dal Master in «⚙ Impostazioni IA»
// (impostazione «ia_prompt_antagonista») o, se vuoto, il testo predefinito prompt_antagonista.md, dato dall'utente.
// Alla fine si aggiunge la situazione del gruppo: l'antagonista «osserva la campagna» e la usa per consigliare il Master (§0 del
// prompt, 2026-11-09: l'antagonista consigliere del Master, consapevole del suo ruolo e di quello del Master). Tutto il resto (gruppi separati, salvataggio, fornitori) è come l'Assistente.

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed prompt_antagonista.md
var promptAntagonistaPredefinito string

const (
	maxPromptAntagonista   = 60000 // caratteri del prompt modificabile
	maxContestoAntagonista = 12000 // caratteri della situazione del gruppo aggiunta (il prompt è già lungo: Ollama ha un contesto limitato)
)

func (s *Server) promptAntagonista() (testo string, personalizzato bool) {
	return s.testoIA("ia_prompt_antagonista", promptAntagonistaPredefinito)
}

// sistemaAntagonista: prompt dell'antagonista + regola dei gruppi separati + situazione del gruppo (fuori dal personaggio).
func (s *Server) sistemaAntagonista(gid, nome string) string {
	p, _ := s.promptAntagonista()
	ctx := accorciaContesto(s.contestoGioco(gid, nome), maxContestoAntagonista)
	return strings.TrimSpace(p) + "\n\n---\nCIÒ CHE GASTER BLESS TI RIFERISCE SULLA CAMPAGNA\n" +
		"Chi ti scrive è il Master della campagna, salvo che ti chieda di parlare a un personaggio o di recitare una scena. Qui sotto c'è la " +
		"situazione attuale del gruppo di forestieri, presa dai dati di Gaster Bless: è ciò che hai osservato e che ti è stato riferito, e lo usi " +
		"per consigliare il Master. Nelle scene con i personaggi decidi tu, dall'antagonista, cosa rivelare. Rispondi sempre in italiano.\n\n" +
		regolaGruppo(nome) + "\n\nSITUAZIONE DEL GRUPPO:\n" + ctx
}

// modoValido: assistente (Cronista della campagna), antagonista («Parla con l'antagonista») o canone (Archivista, identita_ia.go).
func modoValido(m string) string {
	if m == "antagonista" || m == "canone" {
		return m
	}
	return "assistente"
}

// GET /assistente/antagonista: il prompt attuale; PUT: lo salva (vuoto = torna al testo predefinito).
func (s *Server) apiIAAntagonista(w http.ResponseWriter, r *http.Request) (any, error) {
	return s.apiTestoIA("ia_prompt_antagonista", promptAntagonistaPredefinito)(w, r)
}
