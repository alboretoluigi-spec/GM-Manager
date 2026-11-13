package server

import (
	"database/sql"
	"net/http"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

func (s *Server) rotteGruppi(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/gruppi", api(s.apiGruppi))
	mux.Handle("POST /api/v1/gruppi", api(s.apiNuovoGruppo))
	mux.Handle("PATCH /api/v1/gruppi/{id}", api(s.apiModificaGruppo))
	mux.Handle("DELETE /api/v1/gruppi/{id}", api(s.apiArchiviaGruppo))
	mux.Handle("GET /api/v1/gruppi/{id}/sessioni", api(s.apiSessioni))
	mux.Handle("POST /api/v1/gruppi/{id}/sessioni", api(s.apiNuovaSessione))
	mux.Handle("GET /api/v1/sessioni/{id}", api(s.apiSessione))
	mux.Handle("PATCH /api/v1/sessioni/{id}", api(s.apiModificaSessione))
	mux.Handle("DELETE /api/v1/sessioni/{id}", api(s.apiEliminaSessione))
	mux.Handle("PUT /api/v1/sessioni/{id}/partecipanti", api(s.apiPartecipanti))
	mux.Handle("POST /api/v1/sessioni/{id}/conferma", api(s.apiConfermaPresenza))
	s.rotteImmagineGruppo(mux)
}

type vistaGruppo struct {
	ID         string `json:"id"`
	Nome       string `json:"nome"`
	Giorno     string `json:"giorno"`
	Colore     string `json:"colore"`
	Giocatori  int    `json:"giocatori"`
	Segretario bool   `json:"segretario"`
	Immagine   string `json:"immagine,omitempty"` // nome del file: cambia a ogni caricamento (serve per la cache)
}

func (s *Server) apiGruppi(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	out := []vistaGruppo{}
	for _, id := range s.gruppiDi(u) {
		var v vistaGruppo
		if s.DB.QueryRow(`SELECT id, nome, giorno, colore, immagine FROM gruppi WHERE id = ?`, id).Scan(&v.ID, &v.Nome, &v.Giorno, &v.Colore, &v.Immagine) != nil {
			continue
		}
		s.DB.QueryRow(`SELECT COUNT(*) FROM iscrizioni i JOIN utenti u ON u.id = i.utente_id WHERE i.gruppo_id = ? AND u.stato = 'approvato'`, id).Scan(&v.Giocatori)
		v.Segretario = s.segretario(u, id)
		out = append(out, v)
	}
	return out, nil
}

var giorniValidi = map[string]bool{"": true, "Lunedì": true, "Martedì": true, "Mercoledì": true, "Giovedì": true, "Venerdì": true, "Sabato": true, "Domenica": true}

func (s *Server) leggiGruppo(r *http.Request) (nome, giorno, colore string, err error) {
	var in struct{ Nome, Giorno, Colore string }
	if err = leggiJSON(r, &in); err != nil {
		return
	}
	if nome, err = rigaSingola(in.Nome, "Nome del gruppo", 1, 60); err != nil {
		return
	}
	if !giorniValidi[in.Giorno] {
		err = errore(http.StatusBadRequest, "Giorno non valido.")
		return
	}
	if in.Colore != "" && !coloreValido(in.Colore) {
		err = errore(http.StatusBadRequest, "Colore non valido.")
		return
	}
	return nome, in.Giorno, in.Colore, nil
}

func coloreValido(c string) bool {
	if len(c) != 7 || c[0] != '#' {
		return false
	}
	for _, r := range c[1:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

func (s *Server) apiNuovoGruppo(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	nome, giorno, colore, err := s.leggiGruppo(r)
	if err != nil {
		return nil, err
	}
	id := cripto.ID()
	ora := db.Ora()
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO gruppi(id, nome, giorno, colore, creato_il) VALUES(?,?,?,?,?)`, id, nome, giorno, colore, ora); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO stato_mappa(gruppo_id, nebbia, modificato_il) VALUES(?,?,?)`, id, nebbiaVuota(), ora)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.Hub.AlMaster(Msg{Tipo: "gruppi.aggiornati"})
	return map[string]string{"id": id}, nil
}

func (s *Server) apiModificaGruppo(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	nome, giorno, colore, err := s.leggiGruppo(r)
	if err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`UPDATE gruppi SET nome = ?, giorno = ?, colore = ? WHERE id = ?`, nome, giorno, colore, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errNonTrovato
	}
	return nil, nil
}

// apiArchiviaGruppo nasconde il gruppo senza cancellare i dati (recuperabile dal database).
func (s *Server) apiArchiviaGruppo(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	s.DB.Exec(`UPDATE gruppi SET archiviato = 1 WHERE id = ?`, r.PathValue("id"))
	return nil, nil
}

// ===== Sessioni di gioco =====

type vistaSessione struct {
	ID           string              `json:"id"`
	Gruppo       string              `json:"gruppo"`
	Titolo       string              `json:"titolo"`
	Inizio       int64               `json:"inizio"`
	Luogo        string              `json:"luogo"`
	Note         string              `json:"note"`
	Stato        string              `json:"stato"`
	MQTT         bool                `json:"mqttRemoto"`
	Partecipo    bool                `json:"partecipo"`
	Partecipanti []vistaPartecipante `json:"partecipanti"`
}

type vistaPartecipante struct {
	Utente      string `json:"utente"`
	Nome        string `json:"nome"`
	Personaggio string `json:"personaggio,omitempty"`
	NomePG      string `json:"nomePersonaggio,omitempty"`
	Confermato  bool   `json:"confermato"`
}

func (s *Server) caricaSessioneGioco(id string, u *Utente) (*vistaSessione, error) {
	v := &vistaSessione{ID: id, Partecipanti: []vistaPartecipante{}}
	var mq int
	err := s.DB.QueryRow(`SELECT gruppo_id, titolo, inizio_il, luogo, note, stato, mqtt_remoto FROM sessioni_gioco WHERE id = ?`, id).
		Scan(&v.Gruppo, &v.Titolo, &v.Inizio, &v.Luogo, &v.Note, &v.Stato, &mq)
	if err != nil {
		return nil, errNonTrovato
	}
	v.MQTT = mq == 1
	if !s.membro(u, v.Gruppo) {
		return nil, errNonTrovato // non si rivela nemmeno che la sessione esiste
	}
	rows, err := s.DB.Query(`SELECT p.utente_id, u.nome, COALESCE(p.personaggio_id,''), COALESCE(pg.nome,''), p.confermato
		FROM partecipanti p JOIN utenti u ON u.id = p.utente_id LEFT JOIN personaggi pg ON pg.id = p.personaggio_id
		WHERE p.sessione_id = ? ORDER BY u.nome`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p vistaPartecipante
		var c int
		rows.Scan(&p.Utente, &p.Nome, &p.Personaggio, &p.NomePG, &c)
		p.Confermato = c == 1
		if p.Utente == u.ID {
			v.Partecipo = true
		}
		v.Partecipanti = append(v.Partecipanti, p)
	}
	return v, nil
}

func (s *Server) apiSessioni(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	g := r.PathValue("id")
	if !s.membro(u, g) {
		return nil, errNonTrovato
	}
	rows, err := s.DB.Query(`SELECT id FROM sessioni_gioco WHERE gruppo_id = ? ORDER BY inizio_il DESC LIMIT 100`, g)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	out := []*vistaSessione{}
	for _, id := range ids {
		if v, err := s.caricaSessioneGioco(id, u); err == nil {
			out = append(out, v)
		}
	}
	return out, nil
}

func (s *Server) apiSessione(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	return s.caricaSessioneGioco(r.PathValue("id"), u)
}

type datiSessione struct {
	Titolo *string
	Inizio *int64
	Luogo  *string
	Note   *string
	Stato  *string
	MQTT   *bool `json:"mqttRemoto"`
}

func (s *Server) apiNuovaSessione(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	g := r.PathValue("id")
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM gruppi WHERE id = ? AND archiviato = 0`, g).Scan(&n)
	if n == 0 {
		return nil, errNonTrovato
	}
	var in datiSessione
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Titolo == nil || in.Inizio == nil {
		return nil, errore(http.StatusBadRequest, "Titolo, data e ora sono obbligatori.")
	}
	titolo, err := rigaSingola(*in.Titolo, "Titolo", 1, 80)
	if err != nil {
		return nil, err
	}
	luogo, note := "", ""
	if in.Luogo != nil {
		if luogo, err = rigaSingola(*in.Luogo, "Luogo", 0, 120); err != nil {
			return nil, err
		}
	}
	if in.Note != nil {
		if note, err = testo(*in.Note, "Note", 0, 1000); err != nil {
			return nil, err
		}
	}
	if *in.Inizio < time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix() || *in.Inizio > time.Now().AddDate(5, 0, 0).Unix() {
		return nil, errore(http.StatusBadRequest, "Data non valida.")
	}
	id := cripto.ID()
	ora := db.Ora()
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO sessioni_gioco(id, gruppo_id, titolo, inizio_il, luogo, note, creato_il) VALUES(?,?,?,?,?,?,?)`,
			id, g, titolo, *in.Inizio, luogo, note, ora); err != nil {
			return err
		}
		// Per comodità si iscrivono subito tutti i giocatori approvati del gruppo (il Master può togliere chi manca).
		_, err := tx.Exec(`INSERT INTO partecipanti(sessione_id, utente_id) SELECT ?, i.utente_id FROM iscrizioni i
			JOIN utenti u ON u.id = i.utente_id WHERE i.gruppo_id = ? AND u.stato = 'approvato'`, id, g)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.Hub.Pubblica("gruppo:"+g, Msg{Tipo: "sessioni.aggiornate", Dati: map[string]string{"gruppo": g}})
	return map[string]string{"id": id}, nil
}

func (s *Server) apiModificaSessione(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var g string
	if s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, id).Scan(&g) != nil {
		return nil, errNonTrovato
	}
	var in datiSessione
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	set := func(col string, v any) { s.DB.Exec(`UPDATE sessioni_gioco SET `+col+` = ? WHERE id = ?`, v, id) }
	if in.Titolo != nil {
		t, err := rigaSingola(*in.Titolo, "Titolo", 1, 80)
		if err != nil {
			return nil, err
		}
		set("titolo", t)
	}
	if in.Inizio != nil {
		if *in.Inizio < time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix() || *in.Inizio > time.Now().AddDate(5, 0, 0).Unix() {
			return nil, errore(http.StatusBadRequest, "Data non valida.")
		}
		set("inizio_il", *in.Inizio)
	}
	if in.Luogo != nil {
		l, err := rigaSingola(*in.Luogo, "Luogo", 0, 120)
		if err != nil {
			return nil, err
		}
		set("luogo", l)
	}
	if in.Note != nil {
		n, err := testo(*in.Note, "Note", 0, 1000)
		if err != nil {
			return nil, err
		}
		set("note", n)
	}
	if in.Stato != nil {
		switch *in.Stato {
		case "pianificata", "in_corso", "conclusa":
			set("stato", *in.Stato)
		default:
			return nil, errDatiNonValidi
		}
	}
	if in.MQTT != nil {
		v := 0
		if *in.MQTT {
			v = 1
		}
		set("mqtt_remoto", v)
	}
	if in.Stato != nil && *in.Stato == "in_corso" {
		go s.controlloAvvioSessione(id)    // controllo delle schede dei partecipanti
		go s.controllaInneschiSessione(id) // eventi del Codice «alla prossima sessione» (lore_promemoria.go)
	}
	if in.MQTT != nil || in.Stato != nil {
		// Il gioco da remoto resta acceso solo finché la sessione non è conclusa.
		var mq int
		var stato string
		s.DB.QueryRow(`SELECT mqtt_remoto, stato FROM sessioni_gioco WHERE id = ?`, id).Scan(&mq, &stato)
		s.remoto.aggiorna(id, mq == 1 && stato != "conclusa")
	}
	s.Hub.Pubblica("gruppo:"+g, Msg{Tipo: "sessioni.aggiornate", Dati: map[string]string{"gruppo": g, "sessione": id}})
	return nil, nil
}

func (s *Server) apiEliminaSessione(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var g string
	if s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, id).Scan(&g) != nil {
		return nil, errNonTrovato
	}
	s.remoto.aggiorna(id, false)
	if _, err := s.DB.Exec(`DELETE FROM sessioni_gioco WHERE id = ?`, id); err != nil {
		return nil, err
	}
	s.Hub.Pubblica("gruppo:"+g, Msg{Tipo: "sessioni.aggiornate", Dati: map[string]string{"gruppo": g, "sessione": id}})
	return nil, nil
}

// apiPartecipanti imposta chi partecipa (solo giocatori del gruppo) e con quale personaggio.
func (s *Server) apiPartecipanti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var g string
	if s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, id).Scan(&g) != nil {
		return nil, errNonTrovato
	}
	var in []struct{ Utente, Personaggio string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	err := db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		vecchi := map[string]int{}
		rows, _ := tx.Query(`SELECT utente_id, confermato FROM partecipanti WHERE sessione_id = ?`, id)
		for rows != nil && rows.Next() {
			var u string
			var c int
			rows.Scan(&u, &c)
			vecchi[u] = c
		}
		if rows != nil {
			rows.Close()
		}
		if _, err := tx.Exec(`DELETE FROM partecipanti WHERE sessione_id = ?`, id); err != nil {
			return err
		}
		for _, p := range in {
			var n int
			tx.QueryRow(`SELECT COUNT(*) FROM iscrizioni WHERE utente_id = ? AND gruppo_id = ?`, p.Utente, g).Scan(&n)
			if n == 0 {
				return errore(http.StatusBadRequest, "Un partecipante non fa parte del gruppo.")
			}
			if p.Personaggio != "" {
				tx.QueryRow(`SELECT COUNT(*) FROM personaggi WHERE id = ? AND proprietario_id = ?`, p.Personaggio, p.Utente).Scan(&n)
				if n == 0 {
					return errore(http.StatusBadRequest, "Il personaggio scelto non appartiene a quel giocatore.")
				}
			}
			if _, err := tx.Exec(`INSERT INTO partecipanti(sessione_id, utente_id, personaggio_id, confermato) VALUES(?,?,?,?)`,
				id, p.Utente, nullo(p.Personaggio), vecchi[p.Utente]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.Hub.Pubblica("gruppo:"+g, Msg{Tipo: "sessioni.aggiornate", Dati: map[string]string{"gruppo": g, "sessione": id}})
	return nil, nil
}

// apiConfermaPresenza: il giocatore conferma la presenza e sceglie il personaggio.
func (s *Server) apiConfermaPresenza(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	g, ok := s.partecipa(u, id)
	if !ok || u.Master() {
		return nil, errNonTrovato
	}
	var in struct {
		Confermato  bool
		Personaggio string
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Personaggio != "" {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM personaggi WHERE id = ? AND proprietario_id = ? AND stato = 'attivo'`, in.Personaggio, u.ID).Scan(&n)
		if n == 0 {
			return nil, errore(http.StatusBadRequest, "Personaggio non valido.")
		}
	}
	c := -1 // «Non posso più venire»: esce anche dal segnalino del gruppo (0 = non ha ancora risposto)
	if in.Confermato {
		c = 1
	}
	s.DB.Exec(`UPDATE partecipanti SET confermato = ?, personaggio_id = ? WHERE sessione_id = ? AND utente_id = ?`, c, nullo(in.Personaggio), id, u.ID)
	if in.Confermato && in.Personaggio != "" {
		go s.controlloAllAccesso(u)
	}
	s.Hub.Pubblica("gruppo:"+g, Msg{Tipo: "sessioni.aggiornate", Dati: map[string]string{"gruppo": g, "sessione": id}})
	return nil, nil
}
