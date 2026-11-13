package server

// Tutte le regole su "chi può vedere o modificare cosa" passano da queste funzioni,
// così si possono controllare e provare in un unico punto.

// gruppiDi restituisce i gruppi accessibili: tutti per il Master, le iscrizioni per i giocatori approvati.
func (s *Server) gruppiDi(u *Utente) []string {
	if u == nil || !u.Approvato() {
		return nil
	}
	q := `SELECT gruppo_id FROM iscrizioni WHERE utente_id = ?`
	args := []any{u.ID}
	if u.Master() {
		q, args = `SELECT id FROM gruppi WHERE archiviato = 0`, nil
	}
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var g string
		rows.Scan(&g)
		out = append(out, g)
	}
	return out
}

// membro dice se l'utente può accedere ai dati di un gruppo.
func (s *Server) membro(u *Utente, gruppo string) bool {
	if u == nil || !u.Approvato() || gruppo == "" {
		return false
	}
	if u.Master() {
		return true
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM iscrizioni WHERE utente_id = ? AND gruppo_id = ?`, u.ID, gruppo).Scan(&n)
	return n > 0
}

// segretario dice se l'utente è Segretario del gruppo.
func (s *Server) segretario(u *Utente, gruppo string) bool {
	if u == nil || !u.Approvato() {
		return false
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM iscrizioni WHERE utente_id = ? AND gruppo_id = ? AND segretario = 1`, u.ID, gruppo).Scan(&n)
	return n > 0
}

// partecipa dice se l'utente può entrare in una sessione di gioco (mappa e iniziativa):
// il Master sempre; i giocatori se iscritti al gruppo e alla sessione.
func (s *Server) partecipa(u *Utente, sessione string) (gruppo string, ok bool) {
	if u == nil || !u.Approvato() {
		return "", false
	}
	if err := s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, sessione).Scan(&gruppo); err != nil {
		return "", false
	}
	if u.Master() {
		return gruppo, true
	}
	if !s.membro(u, gruppo) {
		return "", false
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM partecipanti WHERE sessione_id = ? AND utente_id = ?`, sessione, u.ID).Scan(&n)
	return gruppo, n > 0
}

// puoLeggerePersonaggio: autore e Master.
func (s *Server) puoLeggerePersonaggio(u *Utente, pg string) bool {
	if u == nil || !u.Approvato() {
		return false
	}
	if u.Master() {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM personaggi WHERE id = ?`, pg).Scan(&n)
		return n > 0
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM personaggi WHERE id = ? AND proprietario_id = ?`, pg, u.ID).Scan(&n)
	return n > 0
}

// puoModificarePersonaggio: autore e Master, ma non sui personaggi ritirati, morti o archiviati.
func (s *Server) puoModificarePersonaggio(u *Utente, pg string) bool {
	if !s.puoLeggerePersonaggio(u, pg) {
		return false
	}
	var stato string
	s.DB.QueryRow(`SELECT stato FROM personaggi WHERE id = ?`, pg).Scan(&stato)
	return stato == "attivo"
}

// utenteAncoraValido ricontrolla lo stato dell'account (per le connessioni già aperte).
func (s *Server) utenteAncoraValido(id string) bool {
	var stato string
	if err := s.DB.QueryRow(`SELECT stato FROM utenti WHERE id = ?`, id).Scan(&stato); err != nil {
		return false
	}
	return stato == "approvato"
}
