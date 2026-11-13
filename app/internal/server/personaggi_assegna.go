package server

// Assegnazione di una scheda (richiesta dell'utente, 2026-11-06): il Master crea o prepara un personaggio e lo
// assegna a un giocatore (o lo riprende per sé). Vedere e modificare le schede dei giocatori il Master lo
// può già fare (permessi.go: puoLeggerePersonaggio / puoModificarePersonaggio).
// POST /api/v1/personaggi/{id}/assegna {proprietario, gruppo}: proprietario vuoto = il Master stesso.

import (
	"database/sql"
	"net/http"

	"gasterbless/internal/db"
)

func (s *Server) apiAssegnaPersonaggio(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var vecchio, nome string
	if s.DB.QueryRow(`SELECT COALESCE(proprietario_id,''), nome FROM personaggi WHERE id = ?`, id).Scan(&vecchio, &nome) != nil {
		return nil, errNonTrovato
	}
	var in struct{ Proprietario, Gruppo string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	prop := u.ID
	if in.Proprietario != "" && in.Proprietario != u.ID {
		if st, err := s.giocatore(in.Proprietario); err != nil || st != "approvato" {
			return nil, errore(http.StatusBadRequest, "Si può assegnare solo a un giocatore approvato.")
		}
		prop = in.Proprietario
	}
	gruppo := in.Gruppo
	if prop != u.ID {
		if gruppo != "" {
			var n int
			s.DB.QueryRow(`SELECT COUNT(*) FROM iscrizioni WHERE utente_id = ? AND gruppo_id = ?`, prop, gruppo).Scan(&n)
			if n == 0 {
				return nil, errore(http.StatusBadRequest, "Il giocatore non fa parte di quel gruppo.")
			}
		} else if g := s.gruppiDi(&Utente{ID: prop, Ruolo: "giocatore", Stato: "approvato"}); len(g) == 1 {
			gruppo = g[0] // un solo gruppo: si assegna da sé
		}
	} else if gruppo != "" {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM gruppi WHERE id = ?`, gruppo).Scan(&n)
		if n == 0 {
			return nil, errore(http.StatusBadRequest, "Gruppo non valido.")
		}
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE personaggi SET proprietario_id = ?, gruppo_id = ?, modificato_il = ? WHERE id = ?`, prop, nullo(gruppo), db.Ora(), id); err != nil {
			return err
		}
		// Il vecchio proprietario non partecipa più alle sessioni con questo personaggio.
		if vecchio != "" && vecchio != prop {
			_, err := tx.Exec(`UPDATE partecipanti SET personaggio_id = NULL WHERE utente_id = ? AND personaggio_id = ?`, vecchio, id)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var versione int
	s.DB.QueryRow(`SELECT versione_corrente FROM personaggi WHERE id = ?`, id).Scan(&versione)
	s.avvisaScheda(id, prop, versione, u, []string{"proprietario"})
	if vecchio != "" && vecchio != prop {
		s.avvisaScheda(id, vecchio, versione, u, []string{"proprietario"})
	}
	s.evento("schede", u.ID, "personaggio "+nome+" assegnato")
	return map[string]any{"proprietario": prop, "gruppo": gruppo}, nil
}

// Eliminazione di una scheda (richiesta dell'utente, 2026-11-06): solo il Master, con il nome del personaggio
// scritto come conferma. Si cancellano scheda, versioni, stato in gioco, Sanità, presenza negli scontri (in
// cascata nel database) e il file del background privato. Irreversibile: resta solo nei backup.
// DELETE /api/v1/personaggi/{id} {conferma: "<nome del personaggio>"}
func (s *Server) apiEliminaPersonaggio(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var prop, nome string
	if s.DB.QueryRow(`SELECT COALESCE(proprietario_id,''), nome FROM personaggi WHERE id = ?`, id).Scan(&prop, &nome) != nil {
		return nil, errNonTrovato
	}
	var in struct{ Conferma string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Conferma != nome {
		return nil, errore(http.StatusBadRequest, "Per eliminare scrivi esattamente il nome del personaggio: "+nome+".")
	}
	var fileBg string
	s.DB.QueryRow(`SELECT file FROM background WHERE personaggio_id = ?`, id).Scan(&fileBg)
	// sessioni con uno scontro in corso in cui c'è il personaggio: si aggiornano dopo
	var sessioni []string
	if rows, err := s.DB.Query(`SELECT DISTINCT k.sessione_id FROM combattenti x JOIN combattimenti k ON k.id = x.combattimento_id
		WHERE x.personaggio_id = ? AND k.attivo = 1`, id); err == nil {
		for rows.Next() {
			var x string
			rows.Scan(&x)
			sessioni = append(sessioni, x)
		}
		rows.Close()
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM background WHERE personaggio_id = ?`, id); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM personaggi WHERE id = ?`, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	if fileBg != "" {
		s.eliminaFileDati(fileBg)
	}
	m := Msg{Tipo: "scheda.eliminata", Dati: map[string]any{"personaggio": id, "nome": nome, "autore": u.Nome}}
	if prop != "" {
		s.Hub.AUtente(prop, m)
	}
	s.Hub.AlMaster(m)
	for _, x := range sessioni {
		s.pubblicaCombattimento(x)
	}
	s.evento("schede", u.ID, "personaggio "+nome+" eliminato")
	return nil, nil
}
