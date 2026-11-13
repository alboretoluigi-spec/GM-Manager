package server

// Collegamento tra dadi e sfortuna (scritti dall'altro account sul ramo main-ke8sy4, con un tracker
// diverso) e il tracker di questa versione (iniziativa.go). Le funzioni hanno i nomi usati in dadi.go e
// sfortuna.go, così quei file restano quasi uguali all'originale.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"

	"gasterbless/internal/db"
)

// datiIniz: i campi dei comandi dei dadi che riguardano lo scontro.
type datiIniz struct {
	Sessione    string `json:"sessione"`
	Nome        string `json:"nome"`
	Personaggio string `json:"personaggio"`
}

func leggiIniz(d json.RawMessage) (datiIniz, error) {
	var r datiIniz
	if len(d) > 4<<10 || json.Unmarshal(d, &r) != nil || r.Sessione == "" {
		return r, errDatiNonValidi
	}
	return r, nil
}

// soloMaster: errore se l'utente non è il Master di questa sessione.
func (s *Server) soloMaster(u *Utente, sessione string) error {
	ruolo, _ := s.ruoloIn(u, sessione)
	if ruolo == "" {
		return errNonTrovato
	}
	if ruolo != "master" {
		return errVietato
	}
	return nil
}

// scontroAttivo: lo scontro in corso nella sessione (ok = false se non c'è).
func (s *Server) scontroAttivo(sessione string) (id string, round, turni int, turnoDi string, ok bool) {
	id, round, turni, turnoDi, err := s.combattimentoAttivo(s.DB, sessione)
	return id, round, turni, turnoDi, err == nil
}

// richiediScontro: lo scontro in corso, oppure l'errore "nessun combattimento".
func (s *Server) richiediScontro(sessione string) (string, error) {
	id, _, _, _, err := s.combattimentoAttivo(s.DB, sessione)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errNessunCombattimento
	}
	return id, err
}

// datiPG: riepilogo della scheda e proprietario del personaggio.
func (s *Server) datiPG(pg string) (r riepilogo, prop string) {
	var riep string
	s.DB.QueryRow(`SELECT COALESCE(p.proprietario_id,''), COALESCE(v.riepilogo,'{}') FROM personaggi p
		LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, pg).Scan(&prop, &riep)
	json.Unmarshal([]byte(riep), &r)
	// ritratto: c'è un'immagine nella versione corrente della scheda (serve alla schermata della Sanità)
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		WHERE p.id = ? AND v.dati LIKE '%"img":"data:image/%'`, pg).Scan(&n)
	r.Ritratto = n > 0
	return r, prop
}

// registro scrive nel registro dello scontro e aggiorna chi lo sta guardando.
func (s *Server) registro(comb string, u *Utente, testo string) {
	s.DB.Exec(`INSERT INTO registro_combattimento(combattimento_id, autore_id, testo, creato_il) VALUES(?,?,?,?)`, comb, u.ID, u.Nome+": "+testo, db.Ora())
	var sessione string
	if s.DB.QueryRow(`SELECT sessione_id FROM combattimenti WHERE id = ?`, comb).Scan(&sessione) == nil {
		s.pubblicaCombattimento(sessione)
	}
}

func conSegno(n int) string {
	if n < 0 {
		return "−" + strconv.Itoa(-n)
	}
	return "+" + strconv.Itoa(n)
}
