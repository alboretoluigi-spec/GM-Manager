package server

// Indebolimento (richiesta dell'utente, 2026-11-07): il livello (0-6) vive nelle condizioni dello stato in gioco
// («Indebolimento N», una sola alla volta), così scheda, tracker e Master vedono lo stesso valore. Quando il livello
// di un personaggio sale (dalla scheda, dal tracker, dal Master o da una scheda esterna), il giocatore riceve «indebolimento»:
// sul suo schermo nero per 1 secondo, poi l'immagine «stanco» (web/ui/indebolimento.js).

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
)

func livelloIndebolimento(cond string) int {
	var l []string
	json.Unmarshal([]byte(cond), &l)
	max := 0
	for _, c := range l {
		if n, err := strconv.Atoi(strings.TrimPrefix(c, "Indebolimento ")); err == nil && strings.HasPrefix(c, "Indebolimento ") && n > max && n <= 6 {
			max = n
		}
	}
	return max
}

// conLivelloIndebolimento sostituisce il livello nelle condizioni (0 = nessuno).
func conLivelloIndebolimento(cond string, n int) string {
	var l []string
	json.Unmarshal([]byte(cond), &l)
	out := []string{}
	for _, c := range l {
		if !strings.HasPrefix(c, "Indebolimento ") {
			out = append(out, c)
		}
	}
	if n > 6 {
		n = 6
	}
	if n > 0 {
		out = append(out, "Indebolimento "+strconv.Itoa(n))
	}
	b, _ := json.Marshal(out)
	return string(b)
}

type livelliIndebolimento struct {
	mu sync.Mutex
	m  map[string]int
}

// caricaIndebolimento legge i livelli di tutti i personaggi (all'avvio del server).
func (s *Server) caricaIndebolimento() {
	m := map[string]int{}
	if righe, err := s.DB.Query(`SELECT personaggio_id, condizioni FROM stato_in_gioco`); err == nil {
		for righe.Next() {
			var id, c string
			righe.Scan(&id, &c)
			m[id] = livelloIndebolimento(c)
		}
		righe.Close()
	}
	s.indeb.mu.Lock()
	s.indeb.m = m
	s.indeb.mu.Unlock()
}

// controllaIndebolimento: dopo ogni cambio dello stato in gioco, avvisa il giocatore se il livello è salito.
func (s *Server) controllaIndebolimento(pg, proprietario, cond string) {
	n := livelloIndebolimento(cond)
	s.indeb.mu.Lock()
	if s.indeb.m == nil { // (non dovrebbe succedere: si carica all'avvio)
		s.indeb.m = map[string]int{}
	}
	prima := s.indeb.m[pg]
	s.indeb.m[pg] = n
	s.indeb.mu.Unlock()
	if n <= prima || proprietario == "" {
		return
	}
	var nome string
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&nome)
	s.Hub.AUtente(proprietario, Msg{Tipo: "indebolimento", Dati: map[string]any{"personaggio": pg, "nome": nome, "livello": n, "prima": prima}})
}
