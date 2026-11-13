package server

// Sezione «PNG e mostri» dell'app (solo Master). Usa le stesse tabelle del bestiario della mappa
// (mappa_bestiario.go) e in più:
//   - la scheda strutturata di una voce (statistiche, tratti, azioni), salvata in
//     bestiario_personalizzazioni.scheda_dati: compilata a mano, presa da un modello ufficiale (Nobile,
//     Guardia, Cavaliere…) o letta dal manuale PDF del Master (la lettura la fa la pagina, sul PC);
//   - i PNG nuovi creati da un modello (bestiario_voci.base);
//   - la pagina del manuale trovata dalla ricerca automatica (indice del manuale).
// Il testo dei manuali resta sul PC del Master: non viene mai mandato ai giocatori.

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const maxSchedaCreatura = 64 << 10

type potereCreatura struct {
	N string `json:"n"`
	D string `json:"d"`
}

// schedaCreatura: lo stesso formato delle schede di web/carta/bestiario.json.
type schedaCreatura struct {
	Taglia   string           `json:"taglia"`
	Tipo     string           `json:"tipo"`
	All      string           `json:"all"`
	CA       string           `json:"ca"`
	PF       string           `json:"pf"`
	Vel      string           `json:"vel"`
	Car      [6]int           `json:"car"`
	TS       string           `json:"ts"`
	Abil     string           `json:"abil"`
	Vuln     string           `json:"vuln"`
	Res      string           `json:"res"`
	Imm      string           `json:"imm"`
	Immc     string           `json:"immc"`
	Sensi    string           `json:"sensi"`
	Ling     string           `json:"ling"`
	GS       string           `json:"gs"`
	Tratti   []potereCreatura `json:"tratti"`
	Azioni   []potereCreatura `json:"azioni"`
	Reazioni []potereCreatura `json:"reazioni"`
	Legg     []potereCreatura `json:"legg"`
	Nota     string           `json:"nota,omitempty"` // per esempio «testo nel manuale, p. …»
}

// validaScheda controlla lunghezze e valori e restituisce la scheda in JSON compatto.
func validaScheda(raw json.RawMessage) (string, error) {
	if len(raw) > maxSchedaCreatura {
		return "", errore(http.StatusRequestEntityTooLarge, "Scheda troppo lunga.")
	}
	var sc schedaCreatura
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sc); err != nil {
		return "", errore(http.StatusBadRequest, "Scheda non valida.")
	}
	brevi := []*string{&sc.Taglia, &sc.Tipo, &sc.All, &sc.CA, &sc.PF, &sc.Vel, &sc.GS, &sc.Nota}
	for _, p := range brevi {
		*p = strings.TrimSpace(*p)
		if len([]rune(*p)) > 200 || strings.ContainsAny(*p, "\r\n") {
			return "", errore(http.StatusBadRequest, "Un campo breve della scheda è troppo lungo o va su più righe.")
		}
	}
	for _, p := range []*string{&sc.TS, &sc.Abil, &sc.Vuln, &sc.Res, &sc.Imm, &sc.Immc, &sc.Sensi, &sc.Ling} {
		*p = strings.TrimSpace(*p)
		if len([]rune(*p)) > 600 {
			return "", errore(http.StatusBadRequest, "Un campo della scheda è troppo lungo.")
		}
	}
	for _, v := range sc.Car {
		if v < 0 || v > 30 {
			return "", errore(http.StatusBadRequest, "Le caratteristiche vanno da 1 a 30.")
		}
	}
	for _, l := range []*[]potereCreatura{&sc.Tratti, &sc.Azioni, &sc.Reazioni, &sc.Legg} {
		if *l == nil {
			*l = []potereCreatura{}
		}
		if len(*l) > 40 {
			return "", errore(http.StatusBadRequest, "Troppe voci in un elenco della scheda (massimo 40).")
		}
		for i := range *l {
			x := &(*l)[i]
			x.N, x.D = strings.TrimSpace(x.N), strings.TrimSpace(x.D)
			if len([]rune(x.N)) > 120 || len([]rune(x.D)) > 5000 || x.N == "" && x.D == "" {
				return "", errore(http.StatusBadRequest, "Tratto o azione non valida (nome fino a 120 caratteri, testo fino a 5000).")
			}
		}
	}
	b, _ := json.Marshal(sc)
	return string(b), nil
}

func (s *Server) rotteSezioneBestiario(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/bestiario", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "master"); err != nil {
			return nil, err
		}
		return s.datiBestiario()
	}))
	mux.Handle("POST /api/v1/bestiario/voci", api(s.apiNuovaVoce))
	mux.Handle("PATCH /api/v1/bestiario/voci/{id}", api(s.apiModificaVoce))
	mux.Handle("DELETE /api/v1/bestiario/voci/{id}", api(s.apiEliminaVoce))
	mux.Handle("PUT /api/v1/bestiario/voci/{id}/scheda", api(s.apiSchedaVoce))
	mux.Handle("PUT /api/v1/bestiario/voci/{id}/immagine", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "master"); err != nil {
			return nil, err
		}
		rel, err := s.caricaImmagineVoce(r.PathValue("id"), r.Body)
		if err != nil {
			return nil, err
		}
		return map[string]string{"img": rel}, nil
	}))
	mux.Handle("DELETE /api/v1/bestiario/voci/{id}/immagine", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "master"); err != nil {
			return nil, err
		}
		s.togliImmagineBestiario(r.PathValue("id"))
		return nil, nil
	}))
	mux.Handle("POST /api/v1/bestiario/manuali/{id}/indice", api(s.apiIndiceManuale))
	mux.Handle("PUT /api/v1/bestiario/voci/{id}/raccolta", api(s.apiRaccoltaVoce))
}

// apiNuovaVoce: PNG o mostro nuovo, facoltativamente da un modello (la sua scheda viene copiata e si
// può modificare) o con una scheda già compilata (per esempio letta dal manuale).
func (s *Server) apiNuovaVoce(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Nome     string          `json:"nome"`
		Cat      string          `json:"cat"`
		Ruolo    string          `json:"ruolo"`
		Base     string          `json:"base"`
		Scheda   json.RawMessage `json:"scheda"`
		Raccolta string          `json:"raccolta"` // solo PNG: ufficiale | reloaded
	}
	if err := leggiJSONMax(r, &in, maxSchedaCreatura+4096); err != nil {
		return nil, err
	}
	nome, err := rigaSingola(in.Nome, "Nome", 1, 80)
	if err != nil {
		return nil, err
	}
	ruolo, err := rigaSingola(in.Ruolo, "Ruolo", 0, 120)
	if err != nil {
		return nil, err
	}
	if in.Cat != "png" && in.Cat != "mostro" || !raccoltaValida(in.Raccolta) || in.Cat != "png" && in.Raccolta != "" {
		return nil, errDatiNonValidi
	}
	scheda := ""
	if in.Base != "" {
		if !s.voceBaseEsiste(in.Base) {
			return nil, errore(http.StatusBadRequest, "Modello non trovato.")
		}
		if b := s.schedaBase(in.Base); b != nil {
			if scheda, err = validaScheda(b); err != nil {
				return nil, err
			}
		}
	}
	if len(in.Scheda) > 0 && string(in.Scheda) != "null" {
		if scheda, err = validaScheda(in.Scheda); err != nil {
			return nil, err
		}
	}
	id := "pers-" + cripto.ID()
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO bestiario_voci(id, nome, cat, ruolo, base, creato_il) VALUES(?,?,?,?,?,?)`, id, nome, in.Cat, ruolo, in.Base, db.Ora()); err != nil {
			return err
		}
		if scheda != "" || in.Raccolta != "" {
			_, err := tx.Exec(`INSERT INTO bestiario_personalizzazioni(voce_id, scheda_dati, raccolta) VALUES(?,?,?)`, id, scheda, in.Raccolta)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{"id": id}, nil
}

// apiModificaVoce: nome e ruolo (solo voci personali), note del Master, manuale e pagina del libro.
func (s *Server) apiModificaVoce(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.voceEsiste(id) {
		return nil, errNonTrovato
	}
	var in struct {
		Nome  *string `json:"nome"`
		Ruolo *string `json:"ruolo"`
		Note  *string `json:"note"`
		Man   *string `json:"man"`
		Page  *int    `json:"page"`
	}
	if err := leggiJSONMax(r, &in, 64<<10); err != nil {
		return nil, err
	}
	personale := !s.voceBaseEsiste(id)
	if (in.Nome != nil || in.Ruolo != nil) && !personale {
		return nil, errore(http.StatusBadRequest, "Nome e ruolo si cambiano solo nelle voci create da te.")
	}
	if in.Nome != nil {
		n, err := rigaSingola(*in.Nome, "Nome", 1, 80)
		if err != nil {
			return nil, err
		}
		s.DB.Exec(`UPDATE bestiario_voci SET nome = ? WHERE id = ?`, n, id)
	}
	if in.Ruolo != nil {
		n, err := rigaSingola(*in.Ruolo, "Ruolo", 0, 120)
		if err != nil {
			return nil, err
		}
		s.DB.Exec(`UPDATE bestiario_voci SET ruolo = ? WHERE id = ?`, n, id)
	}
	s.DB.Exec(`INSERT OR IGNORE INTO bestiario_personalizzazioni(voce_id) VALUES(?)`, id)
	if in.Note != nil {
		t, err := testo(*in.Note, "Note", 0, 30000)
		if err != nil {
			return nil, err
		}
		s.DB.Exec(`UPDATE bestiario_personalizzazioni SET note = ? WHERE voce_id = ?`, t, id)
	}
	if in.Man != nil || in.Page != nil {
		man, pag := "", 0
		if in.Man != nil {
			man = *in.Man
		}
		if in.Page != nil {
			pag = *in.Page
		}
		if pag < 0 || pag > 5000 {
			return nil, errore(http.StatusBadRequest, "Pagina non valida.")
		}
		if man != "" {
			var n int
			s.DB.QueryRow(`SELECT COUNT(*) FROM manuali WHERE id = ?`, man).Scan(&n)
			if n == 0 {
				return nil, errore(http.StatusBadRequest, "Manuale non trovato.")
			}
		}
		s.DB.Exec(`UPDATE bestiario_personalizzazioni SET manuale_id = ?, pagina = ? WHERE voce_id = ?`, nullo(man), pag, id)
	}
	return nil, nil
}

func (s *Server) apiEliminaVoce(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	res, err := s.DB.Exec(`DELETE FROM bestiario_voci WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errore(http.StatusBadRequest, "Si possono eliminare solo le voci create da te.")
	}
	s.togliImmagineBestiario(id)
	s.DB.Exec(`DELETE FROM bestiario_personalizzazioni WHERE voce_id = ?`, id)
	return nil, nil
}

// apiSchedaVoce salva la scheda strutturata di una voce (null = torna alla scheda di base).
func (s *Server) apiSchedaVoce(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.voceEsiste(id) {
		return nil, errNonTrovato
	}
	var in struct {
		Scheda json.RawMessage `json:"scheda"`
	}
	if err := leggiJSONMax(r, &in, maxSchedaCreatura+1024); err != nil {
		return nil, err
	}
	sc := ""
	if len(in.Scheda) > 0 && string(in.Scheda) != "null" {
		var err error
		if sc, err = validaScheda(in.Scheda); err != nil {
			return nil, err
		}
	}
	s.DB.Exec(`INSERT OR IGNORE INTO bestiario_personalizzazioni(voce_id) VALUES(?)`, id)
	if _, err := s.DB.Exec(`UPDATE bestiario_personalizzazioni SET scheda_dati = ? WHERE voce_id = ?`, sc, id); err != nil {
		return nil, err
	}
	return nil, nil
}

// apiIndiceManuale ricorda la pagina del PDF in cui la ricerca automatica ha trovato una voce.
func (s *Server) apiIndiceManuale(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Voce   string `json:"voce"`
		Pagina int    `json:"pagina"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if !s.voceEsiste(in.Voce) || in.Pagina < 1 || in.Pagina > 5000 {
		return nil, errDatiNonValidi
	}
	id := r.PathValue("id")
	return nil, db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		var ind string
		if tx.QueryRow(`SELECT indice FROM manuali WHERE id = ?`, id).Scan(&ind) != nil {
			return errNonTrovato
		}
		m := map[string]int{}
		json.Unmarshal([]byte(ind), &m)
		if len(m) >= 5000 {
			return errore(http.StatusBadRequest, "Indice del manuale pieno.")
		}
		m[in.Voce] = in.Pagina
		b, _ := json.Marshal(m)
		_, err := tx.Exec(`UPDATE manuali SET indice = ? WHERE id = ?`, string(b), id)
		return err
	})
}

// ===== Raccolta dei PNG (2026-11-13) =====
// «🎭 PNG in scena» divide i PNG in due raccolte: «Raccolta principale» e «Raccolta alternativa». I PNG del bestiario di Gaster Bless e
// quelli nuovi senza scelta stanno nella campagna ufficiale; il Master sposta gli altri con un clic.

func raccoltaValida(r string) bool { return r == "" || r == "ufficiale" || r == "reloaded" }

func (s *Server) apiRaccoltaVoce(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Raccolta string `json:"raccolta"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !raccoltaValida(in.Raccolta) {
		return nil, errDatiNonValidi
	}
	if !s.voceEsiste(id) {
		return nil, errNonTrovato
	}
	s.DB.Exec(`INSERT OR IGNORE INTO bestiario_personalizzazioni(voce_id) VALUES(?)`, id)
	if _, err := s.DB.Exec(`UPDATE bestiario_personalizzazioni SET raccolta = ? WHERE voce_id = ?`, in.Raccolta, id); err != nil {
		return nil, err
	}
	return map[string]string{"raccolta": in.Raccolta}, nil
}
