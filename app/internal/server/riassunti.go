package server

// Riassunti delle sessioni: ogni giocatore (e il Master) scrive o carica un testo, che tutti i membri del
// gruppo leggono con la firma "Scriba: <nome>". Solo l'autore lo modifica; l'autore o il Master lo eliminano.

import (
	"database/sql"
	"net/http"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const maxRiassunto = 50000

type vistaRiassunto struct {
	ID             string `json:"id"`
	Gruppo         string `json:"gruppo"`
	GruppoNome     string `json:"gruppoNome"`
	Sessione       string `json:"sessione,omitempty"`
	SessioneTitolo string `json:"sessioneTitolo,omitempty"`
	Titolo         string `json:"titolo"`
	Testo          string `json:"testo"`
	Scriba         string `json:"scriba"`
	Personaggio    string `json:"personaggio,omitempty"` // personaggio che firma (2026-11-09)
	Ottimo         bool   `json:"ottimo,omitempty"`      // «Ottimo lavoro» del Master
	Mio            bool   `json:"mio,omitempty"`
	Creato         int64  `json:"creato"`
	Modificato     int64  `json:"modificato"`
}

func (s *Server) rotteRiassunti(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/riassunti", api(s.apiRiassunti))
	mux.Handle("POST /api/v1/riassunti", api(s.apiNuovoRiassunto))
	mux.Handle("PUT /api/v1/riassunti/{id}", api(s.apiModificaRiassunto))
	mux.Handle("DELETE /api/v1/riassunti/{id}", api(s.apiEliminaRiassunto))
	mux.Handle("PATCH /api/v1/riassunti/{id}/firma", api(s.apiFirmaRiassunto))  // Master: cambia la firma
	mux.Handle("POST /api/v1/riassunti/{id}/ottimo", api(s.apiOttimoRiassunto)) // Master: «Ottimo lavoro» → ispirazione
}

func (s *Server) apiRiassunti(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	gruppi := s.gruppiDi(u)
	if g := r.URL.Query().Get("gruppo"); g != "" {
		if !s.membro(u, g) {
			return nil, errNonTrovato
		}
		gruppi = []string{g}
	}
	out := []vistaRiassunto{}
	if len(gruppi) == 0 {
		return map[string]any{"lista": out}, nil
	}
	q := `SELECT r.id, r.gruppo_id, g.nome, COALESCE(r.sessione_id,''), COALESCE(sg.titolo,''), r.titolo, r.testo, r.autore_nome,
		COALESCE(r.autore_id,''), r.creato_il, r.modificato_il, COALESCE(r.personaggio_id,''), r.ottimo
		FROM riassunti r JOIN gruppi g ON g.id = r.gruppo_id LEFT JOIN sessioni_gioco sg ON sg.id = r.sessione_id
		WHERE r.gruppo_id IN (?` + strings.Repeat(",?", len(gruppi)-1) + `) ORDER BY r.creato_il DESC, r.id`
	args := make([]any, len(gruppi))
	for i, g := range gruppi {
		args[i] = g
	}
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v vistaRiassunto
		var autore string
		if err := rows.Scan(&v.ID, &v.Gruppo, &v.GruppoNome, &v.Sessione, &v.SessioneTitolo, &v.Titolo, &v.Testo, &v.Scriba, &autore, &v.Creato, &v.Modificato, &v.Personaggio, &v.Ottimo); err != nil {
			return nil, err
		}
		v.Mio = autore == u.ID
		out = append(out, v)
	}
	return map[string]any{"lista": out}, nil
}

type datiRiassunto struct {
	Gruppo      string `json:"gruppo"`
	Sessione    string `json:"sessione"`
	Titolo      string `json:"titolo"`
	Testo       string `json:"testo"`
	Personaggio string `json:"personaggio"` // facoltativo: il riassunto è firmato con il nome del personaggio
}

// firmaPersonaggio: nome del personaggio con cui l'utente firma (deve essere suo e attivo).
func (s *Server) firmaPersonaggio(u *Utente, pg string) (string, error) {
	if pg == "" {
		return "", nil
	}
	var nome string
	if s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ? AND proprietario_id = ? AND stato = 'attivo'`, pg, u.ID).Scan(&nome) != nil {
		return "", errore(http.StatusBadRequest, "Scegli uno dei tuoi personaggi.")
	}
	return nome, nil
}

// controlla pulisce titolo e testo e verifica che la sessione (facoltativa) sia del gruppo.
func (s *Server) controllaRiassunto(in *datiRiassunto, gruppo string) (sessione sql.NullString, err error) {
	in.Testo = strings.ReplaceAll(strings.ReplaceAll(in.Testo, "\r\n", "\n"), "\r", "\n")
	if in.Titolo, err = rigaSingola(in.Titolo, "Titolo", 0, 120); err != nil {
		return
	}
	if in.Testo, err = testo(in.Testo, "Riassunto", 1, maxRiassunto); err != nil {
		return
	}
	if in.Sessione != "" {
		var g string
		if s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, in.Sessione).Scan(&g) != nil || g != gruppo {
			return sessione, errore(http.StatusBadRequest, "La sessione scelta non è di questo gruppo.")
		}
		sessione = sql.NullString{String: in.Sessione, Valid: true}
	}
	return
}

func (s *Server) avvisaRiassunti(gruppo string) {
	s.Hub.Pubblica("gruppo:"+gruppo, Msg{Tipo: "riassunti.aggiornati", Dati: map[string]string{"gruppo": gruppo}})
	s.programmaAgganciGruppo(gruppo) // l'IA rilegge i riassunti per gli agganci per l'antagonista (agganci_ia.go)
}

func (s *Server) apiNuovoRiassunto(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	var in datiRiassunto
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if !s.membro(u, in.Gruppo) {
		return nil, errore(http.StatusBadRequest, "Scegli uno dei tuoi gruppi.")
	}
	sess, err := s.controllaRiassunto(&in, in.Gruppo)
	if err != nil {
		return nil, err
	}
	firma, err := s.firmaPersonaggio(u, in.Personaggio)
	if err != nil {
		return nil, err
	}
	if firma == "" {
		firma = u.Nome
	}
	id, ora := cripto.ID(), db.Ora()
	if _, err := s.DB.Exec(`INSERT INTO riassunti(id, gruppo_id, sessione_id, autore_id, autore_nome, titolo, testo, creato_il, modificato_il, personaggio_id) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		id, in.Gruppo, sess, u.ID, firma, in.Titolo, in.Testo, ora, ora, nullo(in.Personaggio)); err != nil {
		return nil, err
	}
	s.avvisaRiassunti(in.Gruppo)
	s.programmaProposta(id) // l'IA legge il riassunto nuovo e propone idee al Master (agganci_ia.go)
	return map[string]string{"id": id}, nil
}

func (s *Server) riassuntoDi(id string) (gruppo, autore string, ok bool) {
	return gruppo, autore, s.DB.QueryRow(`SELECT gruppo_id, COALESCE(autore_id,'') FROM riassunti WHERE id = ?`, id).Scan(&gruppo, &autore) == nil
}

func (s *Server) apiModificaRiassunto(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	g, autore, ok := s.riassuntoDi(r.PathValue("id"))
	if !ok || !s.membro(u, g) {
		return nil, errNonTrovato
	}
	if autore != u.ID {
		return nil, errore(http.StatusForbidden, "Solo lo scriba che l'ha scritto può modificare questo riassunto.")
	}
	var in datiRiassunto
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	sess, err := s.controllaRiassunto(&in, g)
	if err != nil {
		return nil, err
	}
	if _, err := s.DB.Exec(`UPDATE riassunti SET sessione_id = ?, titolo = ?, testo = ?, modificato_il = ? WHERE id = ?`,
		sess, in.Titolo, in.Testo, db.Ora(), r.PathValue("id")); err != nil {
		return nil, err
	}
	s.avvisaRiassunti(g)
	return nil, nil
}

func (s *Server) apiEliminaRiassunto(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	g, autore, ok := s.riassuntoDi(r.PathValue("id"))
	if !ok || !s.membro(u, g) {
		return nil, errNonTrovato
	}
	if autore != u.ID && !u.Master() {
		return nil, errore(http.StatusForbidden, "Solo lo scriba o il Master possono eliminare questo riassunto.")
	}
	if _, err := s.DB.Exec(`DELETE FROM riassunti WHERE id = ?`, r.PathValue("id")); err != nil {
		return nil, err
	}
	s.avvisaRiassunti(g)
	return nil, nil
}

// apiFirmaRiassunto (Master): cambia la firma di un riassunto (per esempio «Arin, scriba del gruppo»).
func (s *Server) apiFirmaRiassunto(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	g, _, ok := s.riassuntoDi(r.PathValue("id"))
	if !ok || !s.membro(u, g) {
		return nil, errNonTrovato
	}
	var in struct {
		Firma string `json:"firma"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Firma, err = rigaSingola(in.Firma, "Firma", 1, 80); err != nil {
		return nil, err
	}
	if _, err := s.DB.Exec(`UPDATE riassunti SET autore_nome = ? WHERE id = ?`, in.Firma, r.PathValue("id")); err != nil {
		return nil, err
	}
	s.avvisaRiassunti(g)
	return nil, nil
}

// apiOttimoRiassunto (Master): segna (o toglie) «Ottimo lavoro». Segnandolo, il personaggio che firma riceve
// l'ispirazione sulla sua scheda (nelle regole 2014 l'ispirazione c'è o non c'è: se l'ha già, resta com'è).
func (s *Server) apiOttimoRiassunto(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var g, pg, autore, titolo string
	var prima int
	if s.DB.QueryRow(`SELECT gruppo_id, COALESCE(personaggio_id,''), COALESCE(autore_id,''), titolo, ottimo FROM riassunti WHERE id = ?`, id).Scan(&g, &pg, &autore, &titolo, &prima) != nil {
		return nil, errNonTrovato
	}
	var in struct {
		Ottimo bool `json:"ottimo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	v := 0
	if in.Ottimo {
		v = 1
	}
	ispirato, nomePG := false, ""
	var prop string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE riassunti SET ottimo = ? WHERE id = ?`, v, id); err != nil {
			return err
		}
		if !in.Ottimo || prima == 1 || pg == "" {
			return nil
		}
		if tx.QueryRow(`SELECT nome, COALESCE(proprietario_id,'') FROM personaggi WHERE id = ? AND stato = 'attivo'`, pg).Scan(&nomePG, &prop) != nil {
			return nil // personaggio non più attivo: resta solo il riconoscimento
		}
		// L'ispirazione sta nello stato in gioco del personaggio (come i punti ferita), non nelle versioni della scheda.
		if _, err := tx.Exec(`INSERT OR IGNORE INTO stato_in_gioco(personaggio_id, modificato_il) VALUES(?,?)`, pg, db.Ora()); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE stato_in_gioco SET ispirazione = 1, modificato_il = ?, modificato_da = ? WHERE personaggio_id = ?`, db.Ora(), u.ID, pg); err != nil {
			return err
		}
		ispirato = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if ispirato {
		s.avvisaStatoInGioco(pg, prop) // la scheda aperta e il tracker si aggiornano
	}
	if in.Ottimo && prima == 0 && autore != "" {
		testo := "⭐ Il Master ha segnato il tuo riassunto «" + titolo + "» come Ottimo lavoro!"
		if titolo == "" {
			testo = "⭐ Il Master ha segnato il tuo riassunto come Ottimo lavoro!"
		}
		if ispirato {
			testo += " " + nomePG + " riceve l'ispirazione."
		}
		s.Hub.AUtente(autore, Msg{Tipo: "riassunto.ottimo", Dati: map[string]any{"testo": testo, "personaggio": pg}})
	}
	s.avvisaRiassunti(g)
	return map[string]any{"ottimo": in.Ottimo, "ispirazione": ispirato, "personaggio": nomePG}, nil
}
