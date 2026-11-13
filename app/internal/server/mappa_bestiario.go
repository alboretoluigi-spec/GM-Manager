package server

// Bestiario (PNG e mostri) e manuali PDF del Master, per la pagina della mappa (web/carta/).
//
// - Le 182 voci di base sono nel file web/carta/bestiario.json (schede SRD 5.1, CC-BY 4.0); il server
//   salva solo ciò che il Master aggiunge: immagine, scheda trascritta, note, pagina del manuale, voci
//   personali (tabelle bestiario_personalizzazioni e bestiario_voci).
// - Tutto è solo per il Master, tranne le immagini che il Master decide di MOSTRARE ai giocatori o di
//   proiettare: solo quelle (elenco in stato_mappa.mostrati) diventano scaricabili da un giocatore.
// - I manuali PDF restano nella cartella Manuali di GasterBlessDati e si aprono solo dal Master
//   (/manuali/<file>, con le richieste parziali che servono al visualizzatore PDF).
// - La ricerca delle pagine delle schede la fa la pagina, sul PC del Master: qui si salva il risultato.

import (
	"encoding/json"
	"io"

	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const (
	maxImmagineBestiario = 25 << 20
	maxManuale           = 1 << 30 // i manuali scansionati possono essere grandi
)

func (s *Server) rotteBestiario(mux *http.ServeMux) {
	m := func(metodo, p string, h gestore) { mux.Handle(metodo+" /api/v1/mappa/"+p, api(h)) }
	m("GET", "best", s.bestiario)
	m("POST", "best", s.bestiarioModifica)
	m("DELETE", "best", s.bestiarioEliminaVoce)
	m("POST", "best/img", s.bestiarioCaricaImmagine)
	m("PUT", "best/img", s.bestiarioCaricaImmagine)
	m("DELETE", "best/img", s.bestiarioEliminaImmagine)
	m("POST", "best/show", s.bestiarioMostra)
	m("PUT", "manual", s.manualeCarica)
	m("POST", "manual", s.manualeModifica)
	m("DELETE", "manual", s.manualeElimina)
	mux.Handle("GET /manuali/{file}", http.HandlerFunc(s.manualeFile))
}

// ===== Voci di base (bestiario.json) =====

var (
	bestUna    ricaricabile // si rilegge quando cambiano i pacchetti di contenuti (contenuti.go)
	bestIDs    map[string]bool
	bestSchede map[string]json.RawMessage // schede di base (SRD), per i modelli dei PNG
)

func (s *Server) caricaBestiarioBase() {
	bestUna.Do(func() {
		bestIDs, bestSchede = map[string]bool{}, map[string]json.RawMessage{}
		// bestiario dell'avventura e catalogo dei mostri (mostri.json, convertito da strumenti/converti_mostri.py)
		for _, f := range []string{"carta/bestiario.json", "carta/mostri.json"} {
			b, err := s.leggiWeb(f)
			if err != nil {
				continue
			}
			var j struct {
				Voci []struct {
					ID     string
					Scheda json.RawMessage
				}
			}
			json.Unmarshal(b, &j)
			for _, v := range j.Voci {
				bestIDs[v.ID] = true
				if len(v.Scheda) > 0 && string(v.Scheda) != "null" {
					bestSchede[v.ID] = v.Scheda
				}
			}
		}
	})
}

func (s *Server) voceBaseEsiste(id string) bool {
	s.caricaBestiarioBase()
	return bestIDs[id]
}

// schedaBase: la scheda di una voce di base, se ce l'ha.
func (s *Server) schedaBase(id string) json.RawMessage {
	s.caricaBestiarioBase()
	return bestSchede[id]
}

// voceEsiste: voce di base o voce personale del Master.
func (s *Server) voceEsiste(id string) bool {
	if s.voceBaseEsiste(id) {
		return true
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM bestiario_voci WHERE id = ?`, id).Scan(&n)
	return n > 0
}

func (s *Server) richiediMasterMappa(r *http.Request) error {
	_, _, _, err := s.contestoMappa(r, "master")
	return err
}

// ===== Lettura =====

type vistaManuale struct {
	ID      string         `json:"id"`
	Nome    string         `json:"nome"`
	Tipo    string         `json:"tipo"`
	File    string         `json:"file"`
	RefBook int            `json:"refBook"`
	RefPdf  int            `json:"refPdf"`
	Spread  bool           `json:"spread"`
	Index   map[string]int `json:"index"`
	Indexed bool           `json:"indexed"`
	NoText  bool           `json:"noText"`
}

func (s *Server) bestiario(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := s.richiediMasterMappa(r); err != nil {
		return nil, err
	}
	return s.datiBestiario()
}

// datiBestiario: ciò che il Master ha aggiunto alle voci (anche la scheda strutturata, "sheet"), le voci
// personali e i manuali. Lo usano la mappa e la sezione «PNG e mostri».
func (s *Server) datiBestiario() (any, error) {
	entries := map[string]map[string]any{}
	rows, err := s.DB.Query(`SELECT voce_id, immagine, note, scheda, COALESCE(manuale_id,''), pagina, scheda_dati, raccolta FROM bestiario_personalizzazioni`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, img, note, scheda, man, dati, racc string
		var pag int
		rows.Scan(&id, &img, &note, &scheda, &man, &pag, &dati, &racc)
		e := map[string]any{}
		if racc != "" { // «🎭 PNG in scena»: ufficiale o reloaded (bestiario.go, apiRaccoltaVoce)
			e["raccolta"] = racc
		}
		if img != "" {
			e["img"] = img
		}
		if note != "" {
			e["notes"] = note
		}
		if scheda != "" {
			e["custom"] = scheda
		}
		if man != "" {
			e["man"] = man
		}
		if pag > 0 {
			e["page"] = pag
		}
		if dati != "" {
			e["sheet"] = json.RawMessage(dati)
		}
		entries[id] = e
	}
	rows.Close()
	extras := []map[string]string{}
	rows, err = s.DB.Query(`SELECT id, nome, cat, ruolo, base FROM bestiario_voci ORDER BY nome`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, nome, cat, ruolo, base string
		rows.Scan(&id, &nome, &cat, &ruolo, &base)
		extras = append(extras, map[string]string{"id": id, "nome": nome, "cat": cat, "ruolo": ruolo, "base": base})
	}
	rows.Close()
	man, err := s.manuali()
	if err != nil {
		return nil, err
	}
	return map[string]any{"entries": entries, "extras": extras, "manuals": man}, nil
}

func (s *Server) manuali() ([]vistaManuale, error) {
	rows, err := s.DB.Query(`SELECT id, nome, tipo, file, ref_libro, ref_pdf, doppia, indice, indicizzato, senza_testo FROM manuali ORDER BY creato_il`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []vistaManuale{}
	for rows.Next() {
		var m vistaManuale
		var ind string
		var dop, idx, nt int
		rows.Scan(&m.ID, &m.Nome, &m.Tipo, &m.File, &m.RefBook, &m.RefPdf, &dop, &ind, &idx, &nt)
		m.Spread, m.Indexed, m.NoText = dop == 1, idx == 1, nt == 1
		json.Unmarshal([]byte(ind), &m.Index)
		if m.Index == nil {
			m.Index = map[string]int{}
		}
		out = append(out, m)
	}
	return out, nil
}

// ===== Voci, schede trascritte, note, pagina del manuale =====

func (s *Server) bestiarioModifica(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := s.richiediMasterMappa(r); err != nil {
		return nil, err
	}
	var in struct {
		New *struct {
			Nome, Cat, Ruolo string
		} `json:"new"`
		ID     string  `json:"id"`
		Custom *string `json:"custom"`
		Notes  *string `json:"notes"`
		Man    *string `json:"man"`
		Page   *int    `json:"page"`
	}
	if err := leggiJSONMax(r, &in, 256<<10); err != nil {
		return nil, err
	}
	if in.New != nil {
		nome, err := rigaSingola(in.New.Nome, "Nome", 1, 80)
		if err != nil {
			return nil, err
		}
		ruolo, err := rigaSingola(in.New.Ruolo, "Ruolo", 0, 120)
		if err != nil {
			return nil, err
		}
		if in.New.Cat != "png" && in.New.Cat != "mostro" {
			return nil, errDatiNonValidi
		}
		id := "pers-" + cripto.ID()
		if _, err := s.DB.Exec(`INSERT INTO bestiario_voci(id, nome, cat, ruolo, creato_il) VALUES(?,?,?,?,?)`, id, nome, in.New.Cat, ruolo, db.Ora()); err != nil {
			return nil, err
		}
		return map[string]string{"id": id}, nil
	}
	if !s.voceEsiste(in.ID) {
		return nil, errNonTrovato
	}
	s.DB.Exec(`INSERT OR IGNORE INTO bestiario_personalizzazioni(voce_id) VALUES(?)`, in.ID)
	if in.Custom != nil {
		t, err := testo(*in.Custom, "Scheda", 0, 30000)
		if err != nil {
			return nil, err
		}
		s.DB.Exec(`UPDATE bestiario_personalizzazioni SET scheda = ? WHERE voce_id = ?`, t, in.ID)
	}
	if in.Notes != nil {
		t, err := testo(*in.Notes, "Note", 0, 30000)
		if err != nil {
			return nil, err
		}
		s.DB.Exec(`UPDATE bestiario_personalizzazioni SET note = ? WHERE voce_id = ?`, t, in.ID)
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
		s.DB.Exec(`UPDATE bestiario_personalizzazioni SET manuale_id = ?, pagina = ? WHERE voce_id = ?`, nullo(man), pag, in.ID)
	}
	return nil, nil
}

// bestiarioEliminaVoce: solo le voci personali si eliminano (con la loro immagine).
func (s *Server) bestiarioEliminaVoce(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := s.richiediMasterMappa(r); err != nil {
		return nil, err
	}
	id := r.URL.Query().Get("id")
	res, err := s.DB.Exec(`DELETE FROM bestiario_voci WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errore(http.StatusBadRequest, "Si possono eliminare solo le voci personali.")
	}
	s.togliImmagineBestiario(id)
	s.DB.Exec(`DELETE FROM bestiario_personalizzazioni WHERE voce_id = ?`, id)
	return nil, nil
}

// ===== Immagini =====

var nomeVoceSicuro = regexp.MustCompile(`[^a-z0-9-]+`)

func (s *Server) bestiarioCaricaImmagine(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := s.richiediMasterMappa(r); err != nil {
		return nil, err
	}
	rel, err := s.caricaImmagineVoce(r.URL.Query().Get("id"), r.Body)
	if err != nil {
		return nil, err
	}
	return map[string]string{"img": rel}, nil
}

// caricaImmagineVoce salva l'immagine di una voce del bestiario in Immagini\Bestiario e restituisce il percorso.
func (s *Server) caricaImmagineVoce(id string, corpo io.Reader) (string, error) {
	if !s.voceEsiste(id) {
		return "", errNonTrovato
	}
	dati, err := io.ReadAll(io.LimitReader(corpo, maxImmagineBestiario+1))
	if err != nil {
		return "", errore(http.StatusBadRequest, "Caricamento interrotto.")
	}
	if len(dati) > maxImmagineBestiario {
		return "", errore(http.StatusRequestEntityTooLarge, "Immagine troppo grande (massimo 25 MB).")
	}
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif"}[http.DetectContentType(dati)]
	if ext == "" {
		return "", errore(http.StatusUnsupportedMediaType, "Formato non supportato: carica un'immagine JPG, PNG, WebP o GIF.")
	}
	dir := filepath.Join(s.Cfg.CartellaMedia, "Immagini", "Bestiario")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Nome leggibile (l'id della voce) più un suffisso casuale, così il browser non mostra la vecchia immagine.
	nome := nomeVoceSicuro.ReplaceAllString(strings.ToLower(id), "-") + "-" + cripto.ID()[:6] + ext
	if err := os.WriteFile(filepath.Join(dir, nome), dati, 0o644); err != nil {
		return "", err
	}
	s.togliImmagineBestiario(id) // la vecchia immagine caricata dall'app si cancella
	rel := "Immagini/Bestiario/" + nome
	s.DB.Exec(`INSERT INTO bestiario_personalizzazioni(voce_id, immagine) VALUES(?,?)
		ON CONFLICT(voce_id) DO UPDATE SET immagine = excluded.immagine`, id, rel)
	return rel, nil
}

func (s *Server) bestiarioEliminaImmagine(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := s.richiediMasterMappa(r); err != nil {
		return nil, err
	}
	s.togliImmagineBestiario(r.URL.Query().Get("id"))
	return nil, nil
}

// togliImmagineBestiario cancella l'immagine della voce (solo se è nella cartella Immagini\Bestiario).
func (s *Server) togliImmagineBestiario(id string) {
	var rel string
	if s.DB.QueryRow(`SELECT immagine FROM bestiario_personalizzazioni WHERE voce_id = ?`, id).Scan(&rel) != nil || rel == "" {
		return
	}
	s.DB.Exec(`UPDATE bestiario_personalizzazioni SET immagine = '' WHERE voce_id = ?`, id)
	if p, pulito, err := s.percorsoMedia(rel); err == nil && strings.HasPrefix(pulito, "Immagini/Bestiario/") {
		os.Remove(p)
	}
}

// bestiarioMostra apre (o chiude) immagini del bestiario sui dispositivi dei giocatori del gruppo.
func (s *Server) bestiarioMostra(w http.ResponseWriter, r *http.Request) (any, error) {
	_, _, gruppo, err := s.contestoMappa(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Hide  bool `json:"hide"`
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Hide {
		s.DB.Exec(`UPDATE stato_mappa SET mostrati = '[]', modificato_il = ? WHERE gruppo_id = ?`, db.Ora(), gruppo)
		s.avvisaMappa(gruppo, map[string]any{"t": "hide"})
		return nil, nil
	}
	if len(in.Items) == 0 || len(in.Items) > 30 {
		return nil, errDatiNonValidi
	}
	var items []map[string]string
	var file []string
	for _, it := range in.Items {
		var img string
		s.DB.QueryRow(`SELECT immagine FROM bestiario_personalizzazioni WHERE voce_id = ?`, it.ID).Scan(&img)
		if img == "" {
			continue
		}
		nome, _ := rigaSingola(it.Name, "Nome", 0, 80)
		items = append(items, map[string]string{"file": img, "name": nome})
		file = append(file, img)
	}
	if len(items) == 0 {
		return nil, errore(http.StatusBadRequest, "Carica prima un'immagine.")
	}
	b, _ := json.Marshal(file)
	s.DB.Exec(`UPDATE stato_mappa SET mostrati = ?, modificato_il = ? WHERE gruppo_id = ?`, string(b), db.Ora(), gruppo)
	s.avvisaMappa(gruppo, map[string]any{"t": "show", "items": items})
	return map[string]int{"shown": len(items)}, nil
}

// immagineBestiarioVisibile: il file è un'immagine del bestiario mostrata o proiettata al gruppo.
func (s *Server) immagineBestiarioVisibile(gruppo, rel string) bool {
	if s.scene.visibile(gruppo, rel) { // PNG in scena (scena_png.go)
		return true
	}
	var mostrati, proj string
	if s.DB.QueryRow(`SELECT mostrati, proiezione FROM stato_mappa WHERE gruppo_id = ?`, gruppo).Scan(&mostrati, &proj) != nil {
		return false
	}
	var l []string
	json.Unmarshal([]byte(mostrati), &l)
	for _, f := range l {
		if f == rel {
			return true
		}
	}
	var p struct{ Kind, File string }
	json.Unmarshal([]byte(proj), &p)
	return p.Kind == "beast" && p.File == rel
}

// ===== Manuali PDF =====

var nomeManualeSicuro = regexp.MustCompile(`^[a-z0-9]{16}\.pdf$`)

func (s *Server) manualeCarica(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := s.richiediMasterMappa(r); err != nil {
		return nil, err
	}
	nome, err := rigaSingola(r.URL.Query().Get("nome"), "Nome del manuale", 1, 120)
	if err != nil {
		return nil, err
	}
	tipo := r.URL.Query().Get("tipo")
	if tipo != "mm" && tipo != "avventura" && tipo != "altro" {
		tipo = "altro"
	}
	dir := filepath.Join(s.Cfg.CartellaMedia, "Manuali")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	file := cripto.ID() + ".pdf"
	p := filepath.Join(dir, file)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	testa := make([]byte, 5)
	n, _ := io.ReadFull(r.Body, testa)
	if n < 5 || string(testa) != "%PDF-" {
		f.Close()
		os.Remove(p)
		return nil, errore(http.StatusUnsupportedMediaType, "Il file non è un PDF.")
	}
	f.Write(testa)
	scritti, err := io.Copy(f, io.LimitReader(r.Body, maxManuale))
	f.Close()
	if err != nil || scritti >= maxManuale {
		os.Remove(p)
		return nil, errore(http.StatusBadRequest, "Caricamento non riuscito o file troppo grande (massimo 1 GB).")
	}
	id := cripto.ID()
	if _, err := s.DB.Exec(`INSERT INTO manuali(id, nome, tipo, file, creato_il) VALUES(?,?,?,?,?)`, id, nome, tipo, file, db.Ora()); err != nil {
		os.Remove(p)
		return nil, err
	}
	return map[string]string{"id": id, "file": file}, nil
}

func (s *Server) manualeModifica(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := s.richiediMasterMappa(r); err != nil {
		return nil, err
	}
	var in struct {
		ID      string          `json:"id"`
		Nome    *string         `json:"nome"`
		Tipo    *string         `json:"tipo"`
		RefBook *int            `json:"refBook"`
		RefPdf  *int            `json:"refPdf"`
		Spread  *bool           `json:"spread"`
		Index   json.RawMessage `json:"index"`
		NoText  *bool           `json:"noText"`
	}
	if err := leggiJSONMax(r, &in, 512<<10); err != nil {
		return nil, err
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM manuali WHERE id = ?`, in.ID).Scan(&n)
	if n == 0 {
		return nil, errNonTrovato
	}
	set := func(col string, v any) { s.DB.Exec(`UPDATE manuali SET `+col+` = ? WHERE id = ?`, v, in.ID) }
	if in.Nome != nil {
		nome, err := rigaSingola(*in.Nome, "Nome del manuale", 1, 120)
		if err != nil {
			return nil, err
		}
		set("nome", nome)
	}
	if in.Tipo != nil {
		if *in.Tipo != "mm" && *in.Tipo != "avventura" && *in.Tipo != "altro" {
			return nil, errDatiNonValidi
		}
		set("tipo", *in.Tipo)
	}
	for _, x := range []struct {
		v   *int
		col string
	}{{in.RefBook, "ref_libro"}, {in.RefPdf, "ref_pdf"}} {
		if x.v != nil {
			if *x.v < 1 || *x.v > 5000 {
				return nil, errore(http.StatusBadRequest, "Numero di pagina non valido.")
			}
			set(x.col, *x.v)
		}
	}
	if in.Spread != nil {
		set("doppia", boolInt(*in.Spread))
	}
	if len(in.Index) > 0 && string(in.Index) != "null" {
		var idx map[string]int
		if json.Unmarshal(in.Index, &idx) != nil || len(idx) > 2000 {
			return nil, errDatiNonValidi
		}
		pulito := map[string]int{}
		for k, v := range idx {
			if v >= 1 && v <= 5000 && s.voceEsiste(k) {
				pulito[k] = v
			}
		}
		b, _ := json.Marshal(pulito)
		set("indice", string(b))
		set("indicizzato", 1)
	}
	if in.NoText != nil {
		set("senza_testo", boolInt(*in.NoText))
	}
	return nil, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Server) manualeElimina(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := s.richiediMasterMappa(r); err != nil {
		return nil, err
	}
	id := r.URL.Query().Get("id")
	var file string
	if s.DB.QueryRow(`SELECT file FROM manuali WHERE id = ?`, id).Scan(&file) != nil {
		return nil, errNonTrovato
	}
	s.DB.Exec(`DELETE FROM manuali WHERE id = ?`, id)
	s.DB.Exec(`UPDATE bestiario_personalizzazioni SET manuale_id = NULL WHERE manuale_id = ?`, id)
	os.RemoveAll(s.cartellaPagine(id))                                                  // pagine pronte per i telefoni (manuali_pagine.go)
	s.DB.Exec(`UPDATE elementi_manuali SET manuale_id = NULL WHERE manuale_id = ?`, id) // gli elementi importati restano
	if nomeManualeSicuro.MatchString(file) {                                            // solo i file caricati dall'app
		os.Remove(filepath.Join(s.Cfg.CartellaMedia, "Manuali", file))
	}
	return nil, nil
}

// manualeFile: il PDF si apre solo dal Master (mai dai giocatori), con le richieste parziali.
func (s *Server) manualeFile(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	if !u.Master() || !u.Approvato() {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	file := r.PathValue("file")
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM manuali WHERE file = ?`, file).Scan(&n)
	if n == 0 || strings.ContainsAny(file, `/\:`) {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeFile(w, r, filepath.Join(s.Cfg.CartellaMedia, "Manuali", file))
}
