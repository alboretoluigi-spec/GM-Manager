package server

// «Saluta il gruppo» (richiesta dell'utente, 2026-11-06): il Master, a fine serata, apre su tutti gli schermi
// della sessione l'immagine di saluto su sfondo nero con la scritta «Alla prossima sessione...».

import (
	"encoding/json"
	"net/http"
)

func (s *Server) rotteSaluto() {
	s.Hub.Registra("saluto.invia", s.wsSaluto)
}

func (s *Server) wsSaluto(c *Cliente, d json.RawMessage) (any, error) {
	if !c.U.Master() {
		return nil, errVietato
	}
	var in struct {
		Sessione string `json:"sessione"`
	}
	if len(d) > 1<<10 || json.Unmarshal(d, &in) != nil || in.Sessione == "" {
		return nil, errDatiNonValidi
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM sessioni_gioco WHERE id = ?`, in.Sessione).Scan(&n)
	if n == 0 {
		return nil, errore(http.StatusNotFound, "Sessione non trovata.")
	}
	m := Msg{Tipo: "saluto", Dati: map[string]string{"sessione": in.Sessione, "testo": "Alla prossima sessione..."}}
	dest := s.destinatariSessione(in.Sessione) // partecipanti della sessione + Master
	for _, id := range dest {
		s.Hub.AUtente(id, m)
	}
	return map[string]int{"destinatari": len(dest) - 1}, nil
}

// chiamaGiocatori manda un messaggio ai giocatori della sessione (non al Master): app.js li porta alla pagina
// giusta. «iniz.chiamata» quando il Master crea o avvia lo scontro (→ tracker), «mappa.chiamata» quando proietta
// sui dispositivi dei giocatori (→ Mappa). Richiesta dell'utente, 2026-11-06.
func (s *Server) chiamaGiocatori(sessione, tipo string, dati map[string]any) {
	if dati == nil {
		dati = map[string]any{}
	}
	dati["sessione"] = sessione
	m := Msg{Tipo: tipo, Dati: dati}
	rows, err := s.DB.Query(`SELECT utente_id FROM partecipanti WHERE sessione_id = ?`, sessione)
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		s.Hub.AUtente(id, m)
	}
}
