package server

// Campagne (docs/RULEBOOK_LIBRARY.md, passo L4): per ogni campagna sistema di gioco, edizione, regolamento principale,
// documenti supplementari e schede personalizzate (da MY RULEBOOKS: lo stesso documento può stare in più campagne),
// regole aggiuntive del Master, impostazioni (oggetto JSON libero) e i gruppi che la giocano. Il sistema di gioco è un
// testo libero: il motore non dipende da nessun regolamento. Migrazione 039. Solo il Master.

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"gasterbless/internal/cripto"
)

type vistaCampagna struct {
	ID            string              `json:"id"`
	Nome          string              `json:"nome"`
	Descrizione   string              `json:"descrizione"`
	Sistema       string              `json:"sistema"`
	Edizione      string              `json:"edizione"`
	Regolamento   string              `json:"regolamento,omitempty"`
	RegoleMaster  string              `json:"regoleMaster"`
	Impostazioni  json.RawMessage     `json:"impostazioni"`
	Supplementari []string            `json:"supplementari"`
	Schede        []string            `json:"schede"`
	Gruppi        []map[string]string `json:"gruppi"`
	CreatoIl      int64               `json:"creatoIl"`
}

func (s *Server) rotteCampagne(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/campagne", api(s.apiCampagne))
	mux.Handle("POST /api/v1/campagne", api(s.apiNuovaCampagna))
	mux.Handle("PATCH /api/v1/campagne/{id}", api(s.apiModificaCampagna))
	mux.Handle("DELETE /api/v1/campagne/{id}", api(s.apiEliminaCampagna))
	mux.Handle("PUT /api/v1/campagne/{id}/documenti", api(s.apiDocumentoCampagna))
	mux.Handle("PUT /api/v1/gruppi/{id}/campagna", api(s.apiCampagnaDelGruppo))
}

func (s *Server) campagne() ([]vistaCampagna, error) {
	rows, err := s.DB.Query(`SELECT id, nome, descrizione, sistema, edizione, COALESCE(regolamento_id, ''), regole_master, impostazioni, creato_il
		FROM campagne ORDER BY creato_il, nome COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	out := []vistaCampagna{}
	idx := map[string]int{}
	for rows.Next() {
		var c vistaCampagna
		var imp string
		if err := rows.Scan(&c.ID, &c.Nome, &c.Descrizione, &c.Sistema, &c.Edizione, &c.Regolamento, &c.RegoleMaster, &imp, &c.CreatoIl); err != nil {
			rows.Close()
			return nil, err
		}
		c.Impostazioni = json.RawMessage(imp)
		c.Supplementari, c.Schede, c.Gruppi = []string{}, []string{}, []map[string]string{}
		idx[c.ID] = len(out)
		out = append(out, c)
	}
	rows.Close()
	if r, err := s.DB.Query(`SELECT campagna_id, documento_id, ruolo FROM campagna_documenti ORDER BY rowid`); err == nil {
		for r.Next() {
			var c, d, ruolo string
			r.Scan(&c, &d, &ruolo)
			if i, ok := idx[c]; ok {
				if ruolo == "scheda" {
					out[i].Schede = append(out[i].Schede, d)
				} else {
					out[i].Supplementari = append(out[i].Supplementari, d)
				}
			}
		}
		r.Close()
	}
	if r, err := s.DB.Query(`SELECT id, nome, COALESCE(campagna_id, '') FROM gruppi WHERE archiviato = 0 ORDER BY nome COLLATE NOCASE`); err == nil {
		for r.Next() {
			var id, nome, c string
			r.Scan(&id, &nome, &c)
			if i, ok := idx[c]; ok {
				out[i].Gruppi = append(out[i].Gruppi, map[string]string{"id": id, "nome": nome})
			}
		}
		r.Close()
	}
	return out, nil
}

func (s *Server) apiCampagne(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	c, err := s.campagne()
	if err != nil {
		return nil, err
	}
	senza := []map[string]string{} // gruppi senza campagna (la loro campagna è stata eliminata prima della regola attuale)
	if rr, err := s.DB.Query(`SELECT id, nome FROM gruppi WHERE archiviato = 0 AND campagna_id IS NULL ORDER BY nome`); err == nil {
		for rr.Next() {
			var id, nome string
			rr.Scan(&id, &nome)
			senza = append(senza, map[string]string{"id": id, "nome": nome})
		}
		rr.Close()
	}
	return map[string]any{"campagne": c, "gruppiSenzaCampagna": senza}, nil
}

type datiCampagna struct {
	Nome         *string          `json:"nome"`
	Descrizione  *string          `json:"descrizione"`
	Sistema      *string          `json:"sistema"`
	Edizione     *string          `json:"edizione"`
	Regolamento  *string          `json:"regolamento"`
	RegoleMaster *string          `json:"regoleMaster"`
	Impostazioni *json.RawMessage `json:"impostazioni"`
}

func (s *Server) leggiDatiCampagna(r *http.Request) (datiCampagna, error) {
	var d datiCampagna
	if err := json.NewDecoder(io.LimitReader(r.Body, 128<<10)).Decode(&d); err != nil {
		return d, errDatiNonValidi
	}
	lim := []struct {
		p   *string
		max int
	}{{d.Nome, 80}, {d.Descrizione, 2000}, {d.Sistema, 100}, {d.Edizione, 60}, {d.RegoleMaster, 50000}}
	for _, c := range lim {
		if c.p != nil && utf8.RuneCountInString(strings.TrimSpace(*c.p)) > c.max {
			return d, errore(http.StatusBadRequest, "Un campo è troppo lungo.")
		}
	}
	if d.Nome != nil && strings.TrimSpace(*d.Nome) == "" {
		return d, errore(http.StatusBadRequest, "Il nome della campagna non può essere vuoto.")
	}
	if d.Regolamento != nil && *d.Regolamento != "" {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM regolamenti_documenti WHERE id = ?`, *d.Regolamento).Scan(&n)
		if n == 0 {
			return d, errore(http.StatusBadRequest, "Il regolamento scelto non è in My Rulebooks.")
		}
	}
	if d.Impostazioni != nil {
		var o map[string]any
		if len(*d.Impostazioni) > 16<<10 || json.Unmarshal(*d.Impostazioni, &o) != nil || o == nil {
			return d, errore(http.StatusBadRequest, "Le impostazioni devono essere un elenco di voci (oggetto JSON) di al massimo 16 KB.")
		}
	}
	return d, nil
}

func (s *Server) apiNuovaCampagna(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	d, err := s.leggiDatiCampagna(r)
	if err != nil {
		return nil, err
	}
	if d.Nome == nil {
		return nil, errore(http.StatusBadRequest, "Scrivi il nome della campagna.")
	}
	id, ora := cripto.ID(), time.Now().Unix()
	imp := "{}"
	if d.Impostazioni != nil {
		imp = string(*d.Impostazioni)
	}
	_, err = s.DB.Exec(`INSERT INTO campagne (id, nome, descrizione, sistema, edizione, regolamento_id, regole_master, impostazioni, creato_il, aggiornato_il)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, strings.TrimSpace(*d.Nome), valore(d.Descrizione, ""), valore(d.Sistema, ""),
		valore(d.Edizione, ""), nulla(valore(d.Regolamento, "")), valore(d.RegoleMaster, ""), imp, ora, ora)
	if err != nil {
		return nil, errore(http.StatusConflict, "Esiste già una campagna con questo nome.")
	}
	s.evento("campagna_crea", u.ID, strings.TrimSpace(*d.Nome))
	s.Hub.AlMaster(Msg{Tipo: "campagne", Dati: map[string]any{}})
	return map[string]any{"id": id}, nil
}

func (s *Server) campagnaEsiste(id string) error {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM campagne WHERE id = ?`, id).Scan(&n)
	if n == 0 {
		return errore(http.StatusNotFound, "Campagna non trovata.")
	}
	return nil
}

func (s *Server) apiModificaCampagna(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if err := s.campagnaEsiste(id); err != nil {
		return nil, err
	}
	d, err := s.leggiDatiCampagna(r)
	if err != nil {
		return nil, err
	}
	set, arg := []string{}, []any{}
	add := func(col string, v any) { set = append(set, col+" = ?"); arg = append(arg, v) }
	if d.Nome != nil {
		add("nome", strings.TrimSpace(*d.Nome))
	}
	if d.Descrizione != nil {
		add("descrizione", strings.TrimSpace(*d.Descrizione))
	}
	if d.Sistema != nil {
		add("sistema", strings.TrimSpace(*d.Sistema))
	}
	if d.Edizione != nil {
		add("edizione", strings.TrimSpace(*d.Edizione))
	}
	if d.Regolamento != nil {
		add("regolamento_id", nulla(*d.Regolamento))
	}
	if d.RegoleMaster != nil {
		add("regole_master", *d.RegoleMaster)
	}
	if d.Impostazioni != nil {
		add("impostazioni", string(*d.Impostazioni))
	}
	add("aggiornato_il", time.Now().Unix())
	arg = append(arg, id)
	if _, err := s.DB.Exec(`UPDATE campagne SET `+strings.Join(set, ", ")+` WHERE id = ?`, arg...); err != nil {
		return nil, errore(http.StatusConflict, "Esiste già una campagna con questo nome.")
	}
	s.Hub.AlMaster(Msg{Tipo: "campagne", Dati: map[string]any{}})
	c, _ := s.campagne()
	for _, x := range c {
		if x.ID == id {
			return x, nil
		}
	}
	return nil, errore(http.StatusNotFound, "Campagna non trovata.")
}

// apiEliminaCampagna: si elimina solo una campagna senza gruppi (prima si spostano) e mai l'ultima rimasta.
// I documenti restano in My Rulebooks.
func (s *Server) apiEliminaCampagna(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if err := s.campagnaEsiste(id); err != nil {
		return nil, err
	}
	var gruppi, tot int
	s.DB.QueryRow(`SELECT COUNT(*) FROM gruppi WHERE campagna_id = ? AND archiviato = 0`, id).Scan(&gruppi)
	s.DB.QueryRow(`SELECT COUNT(*) FROM campagne`).Scan(&tot)
	if gruppi > 0 {
		return nil, errore(http.StatusConflict, "La campagna ha ancora dei gruppi: spostali in un'altra campagna prima di eliminarla.")
	}
	if tot <= 1 {
		return nil, errore(http.StatusConflict, "Deve restare almeno una campagna.")
	}
	if _, err := s.DB.Exec(`DELETE FROM campagne WHERE id = ?`, id); err != nil {
		return nil, err
	}
	s.evento("campagna_elimina", u.ID, id)
	s.Hub.AlMaster(Msg{Tipo: "campagne", Dati: map[string]any{}})
	return map[string]any{"eliminata": id}, nil
}

// apiDocumentoCampagna: {"documento": id, "ruolo": "supplementare"|"scheda", "presente": true|false}.
func (s *Server) apiDocumentoCampagna(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if err := s.campagnaEsiste(id); err != nil {
		return nil, err
	}
	var in struct {
		Documento string `json:"documento"`
		Ruolo     string `json:"ruolo"`
		Presente  bool   `json:"presente"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil || (in.Ruolo != "supplementare" && in.Ruolo != "scheda") {
		return nil, errDatiNonValidi
	}
	if _, err := s.docMio(in.Documento); err != nil {
		return nil, err
	}
	var err error
	if in.Presente {
		_, err = s.DB.Exec(`INSERT OR IGNORE INTO campagna_documenti (campagna_id, documento_id, ruolo) VALUES (?, ?, ?)`, id, in.Documento, in.Ruolo)
	} else {
		_, err = s.DB.Exec(`DELETE FROM campagna_documenti WHERE campagna_id = ? AND documento_id = ? AND ruolo = ?`, id, in.Documento, in.Ruolo)
	}
	if err != nil {
		return nil, err
	}
	s.Hub.AlMaster(Msg{Tipo: "campagne", Dati: map[string]any{}})
	return map[string]any{"ok": true}, nil
}

// apiCampagnaDelGruppo: {"campagna": id} sposta il gruppo in un'altra campagna.
func (s *Server) apiCampagnaDelGruppo(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Campagna string `json:"campagna"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
		return nil, errDatiNonValidi
	}
	if err := s.campagnaEsiste(in.Campagna); err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`UPDATE gruppi SET campagna_id = ? WHERE id = ?`, in.Campagna, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errore(http.StatusNotFound, "Gruppo non trovato.")
	}
	s.Hub.AlMaster(Msg{Tipo: "campagne", Dati: map[string]any{}})
	return map[string]any{"ok": true}, nil
}

// campagneDelDocumento: le campagne che usano un documento (regolamento principale, supplementare o scheda).
func (s *Server) campagneDelDocumento(doc string) []string {
	out := []string{}
	rows, err := s.DB.Query(`SELECT id FROM campagne WHERE regolamento_id = ? UNION SELECT campagna_id FROM campagna_documenti WHERE documento_id = ?`, doc, doc)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if rows.Scan(&c) == nil {
			out = append(out, c)
		}
	}
	return out
}
