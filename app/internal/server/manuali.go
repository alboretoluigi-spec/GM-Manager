package server

// Manuali in PDF: li carica il Master e li consulta nell'app. Ogni manuale può essere condiviso
// con tutti i giocatori approvati o solo con alcuni; la condivisione si toglie in qualsiasi momento.
// Il permesso si controlla a ogni richiesta del file, quindi revocarla ha effetto subito.
//
// Portato dal ramo main-ke8sy4 (lavoro dell'altro account). Differenze in questa versione: i PDF stanno
// nella cartella Manuali dei contenuti (la stessa usata dalla mappa e dall'importazione di GasterBlessDati),
// e la calibrazione delle pagine resta nel bestiario della mappa (mappa_bestiario.go).

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

var tipiManuale = map[string]string{"mm": "Manuale dei Mostri", "avventura": "Avventura", "altro": "Regole e altro"}

type vistaManualeApp struct {
	ID             string      `json:"id"`
	Nome           string      `json:"nome"`
	Tipo           string      `json:"tipo"`
	NomeOriginale  string      `json:"nomeOriginale,omitempty"`
	Dimensione     int64       `json:"dimensione"`
	Caricato       int64       `json:"caricato"`
	CondivisoTutti bool        `json:"condivisoTutti,omitempty"`
	Utenti         []string    `json:"utenti,omitempty"` // solo per il Master: con chi è condiviso
	Pagine         statoPagine `json:"pagine"`           // pagine pronte per i telefoni (manuali_pagine.go)
	OCR            any         `json:"ocr,omitempty"`    // solo per il Master: testo OCR caricato (manuali_ocr.go)
}

func (s *Server) rotteManuali(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/manuali", api(s.apiManuali))
	mux.Handle("POST /api/v1/manuali", api(s.apiCaricaManuale))
	mux.Handle("PATCH /api/v1/manuali/{id}", api(s.apiModificaManuale))
	mux.Handle("DELETE /api/v1/manuali/{id}", api(s.apiEliminaManuale))
	mux.Handle("GET /api/v1/manuali/{id}/file", http.HandlerFunc(s.fileManuale))
	mux.Handle("PUT /api/v1/manuali/{id}/file", api(s.apiSostituisciManuale))
	s.rotteManualiOCR(mux) // testo OCR corretto accanto al PDF (2026-11-11)
}

// puoLeggereManuale: il Master sempre; un giocatore approvato se il manuale è condiviso con tutti o con lui.
func (s *Server) puoLeggereManuale(u *Utente, id string) bool {
	if u == nil || !u.Approvato() {
		return false
	}
	var tutti int
	if s.DB.QueryRow(`SELECT condiviso_tutti FROM manuali WHERE id = ?`, id).Scan(&tutti) != nil {
		return false
	}
	if u.Master() || tutti == 1 {
		return true
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM manuali_condivisi WHERE manuale_id = ? AND utente_id = ?`, id, u.ID).Scan(&n)
	return n > 0
}

func (s *Server) apiManuali(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	q := `SELECT id, nome, tipo, nome_originale, dimensione, creato_il, condiviso_tutti FROM manuali`
	var args []any
	if !u.Master() {
		q += ` WHERE condiviso_tutti = 1 OR id IN (SELECT manuale_id FROM manuali_condivisi WHERE utente_id = ?)`
		args = append(args, u.ID)
	}
	rows, err := s.DB.Query(q+` ORDER BY tipo, nome`, args...)
	if err != nil {
		return nil, err
	}
	out := []vistaManualeApp{}
	for rows.Next() {
		var m vistaManualeApp
		var tutti int
		rows.Scan(&m.ID, &m.Nome, &m.Tipo, &m.NomeOriginale, &m.Dimensione, &m.Caricato, &tutti)
		if u.Master() {
			m.CondivisoTutti = tutti == 1
		} else {
			m.NomeOriginale = "" // ai giocatori basta il titolo
		}
		out = append(out, m)
	}
	rows.Close()
	for i := range out {
		if u.Master() {
			out[i].Utenti = s.condivisiCon(out[i].ID)
			if o := s.infoOCR(out[i].ID); o != nil {
				out[i].OCR = o
			}
		}
		out[i].Pagine = s.statoPagineDi(out[i].ID, false)
	}
	return out, nil
}

func (s *Server) condivisiCon(id string) []string {
	rows, err := s.DB.Query(`SELECT utente_id FROM manuali_condivisi WHERE manuale_id = ? ORDER BY dal`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	l := []string{}
	for rows.Next() {
		var x string
		rows.Scan(&x)
		l = append(l, x)
	}
	return l
}

// apiCaricaManuale riceve il PDF come corpo della richiesta (nome e tipo nei parametri dell'indirizzo)
// e lo scrive su disco a pezzi, senza tenerlo tutto in memoria.
func (s *Server) apiCaricaManuale(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	if r.ContentLength > maxManuale {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande: massimo 1 GB.")
	}
	tipo := r.URL.Query().Get("tipo")
	if _, ok := tipiManuale[tipo]; !ok {
		tipo = "altro"
	}
	orig := nomeOriginaleSicuro(r.URL.Query().Get("file"), "pdf")
	nome, err := rigaSingola(r.URL.Query().Get("nome"), "Titolo", 0, 80)
	if err != nil {
		return nil, err
	}
	if nome == "" {
		nome = strings.TrimSuffix(strings.TrimSuffix(orig, ".pdf"), ".PDF")
	}
	br := bufio.NewReaderSize(r.Body, 64<<10)
	inizio, _ := br.Peek(5)
	if !bytes.Equal(inizio, []byte("%PDF-")) {
		return nil, errore(http.StatusUnsupportedMediaType, "Il file non è un PDF.")
	}
	file := hex.EncodeToString(cripto.Casuali(8)) + ".pdf"
	p, err := s.percorsoManuale(file)
	if err != nil {
		return nil, err
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(f, io.LimitReader(br, maxManuale+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil || n > maxManuale {
		os.Remove(p)
		if n > maxManuale {
			return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande: massimo 1 GB.")
		}
		return nil, errore(http.StatusBadRequest, "Caricamento interrotto.")
	}
	id := cripto.ID()
	if _, err := s.DB.Exec(`INSERT INTO manuali(id, nome, tipo, file, nome_originale, dimensione, creato_il) VALUES(?,?,?,?,?,?,?)`,
		id, nome, tipo, file, orig, n, db.Ora()); err != nil {
		os.Remove(p)
		return nil, err
	}
	return map[string]any{"id": id, "nome": nome}, nil
}

// apiModificaManuale: titolo, tipo e condivisione (tutti i giocatori o un elenco).
func (s *Server) apiModificaManuale(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var in struct {
		Nome           *string   `json:"nome"`
		Tipo           *string   `json:"tipo"`
		CondivisoTutti *bool     `json:"condivisoTutti"`
		Utenti         *[]string `json:"utenti"`
	}
	var raw map[string]json.RawMessage
	if err := leggiJSON(r, &raw); err != nil {
		return nil, err
	}
	base := map[string]json.RawMessage{}
	for k, v := range raw {
		switch k {
		case "nome", "tipo", "condivisoTutti", "utenti":
			base[k] = v
		case "refLibro", "refPdf", "doppia", "indice", "indicizzato", "senzaTesto": // calibrazione (bestiario)
		default:
			return nil, errDatiNonValidi
		}
	}
	if b, _ := json.Marshal(base); json.Unmarshal(b, &in) != nil {
		return nil, errDatiNonValidi
	}
	var esiste int
	s.DB.QueryRow(`SELECT COUNT(*) FROM manuali WHERE id = ?`, id).Scan(&esiste)
	if esiste == 0 {
		return nil, errNonTrovato
	}
	err := db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if in.Nome != nil {
			n, err := rigaSingola(*in.Nome, "Titolo", 1, 80)
			if err != nil {
				return err
			}
			tx.Exec(`UPDATE manuali SET nome = ? WHERE id = ?`, n, id)
		}
		if in.Tipo != nil {
			if _, ok := tipiManuale[*in.Tipo]; !ok {
				return errDatiNonValidi
			}
			tx.Exec(`UPDATE manuali SET tipo = ? WHERE id = ?`, *in.Tipo, id)
		}
		if in.CondivisoTutti != nil {
			v := 0
			if *in.CondivisoTutti {
				v = 1
			}
			tx.Exec(`UPDATE manuali SET condiviso_tutti = ? WHERE id = ?`, v, id)
		}
		if in.Utenti != nil {
			if len(*in.Utenti) > 200 {
				return errDatiNonValidi
			}
			if _, err := tx.Exec(`DELETE FROM manuali_condivisi WHERE manuale_id = ?`, id); err != nil {
				return err
			}
			for _, uid := range *in.Utenti {
				var ok int
				tx.QueryRow(`SELECT COUNT(*) FROM utenti WHERE id = ? AND ruolo <> 'master' AND stato = 'approvato'`, uid).Scan(&ok)
				if ok == 0 {
					return errore(http.StatusBadRequest, "Puoi condividere solo con giocatori approvati.")
				}
				if _, err := tx.Exec(`INSERT OR IGNORE INTO manuali_condivisi(manuale_id, utente_id, dal) VALUES(?,?,?)`, id, uid, db.Ora()); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.avvisaManuali()
	return nil, nil
}

func (s *Server) apiEliminaManuale(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var file string
	if s.DB.QueryRow(`SELECT file FROM manuali WHERE id = ?`, r.PathValue("id")).Scan(&file) != nil {
		return nil, errNonTrovato
	}
	s.DB.Exec(`DELETE FROM manuali WHERE id = ?`, r.PathValue("id"))
	if p, err := s.percorsoManuale(file); err == nil {
		os.Remove(p)
	}
	os.RemoveAll(s.cartellaPagine(r.PathValue("id"))) // pagine pronte per i telefoni
	os.Remove(s.percorsoOCR(r.PathValue("id")))       // testo OCR caricato a parte
	// Gli elementi importati restano, con il nome del manuale (importazione.go).
	s.DB.Exec(`UPDATE elementi_manuali SET manuale_id = NULL WHERE manuale_id = ?`, r.PathValue("id"))
	s.avvisaManuali()
	return nil, nil
}

// avvisaManuali dice a tutte le pagine aperte di rileggere l'elenco (condivisione cambiata).
func (s *Server) avvisaManuali() {
	s.Hub.mu.RLock()
	ids := map[string]bool{}
	for c := range s.Hub.clienti {
		ids[c.U.ID] = true
	}
	s.Hub.mu.RUnlock()
	for id := range ids {
		s.Hub.AUtente(id, Msg{Tipo: "manuali.aggiornati"})
	}
}

// fileManuale serve il PDF (anche a pezzi, con le richieste Range che pdf.js usa per i file grandi).
func (s *Server) fileManuale(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	id := r.PathValue("id")
	if !s.puoLeggereManuale(u, id) {
		scriviJSON(w, http.StatusNotFound, map[string]string{"errore": "Manuale non trovato o non condiviso."})
		return
	}
	var file, nome string
	var quando int64
	s.DB.QueryRow(`SELECT file, nome, creato_il FROM manuali WHERE id = ?`, id).Scan(&file, &nome, &quando)
	p, err := s.percorsoManuale(file)
	if err != nil {
		http.Error(w, "Errore.", http.StatusInternalServerError)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			scriviJSON(w, http.StatusNotFound, map[string]string{"errore": "File del manuale mancante."})
			return
		}
		http.Error(w, "Errore.", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+url.PathEscape(nome+".pdf"))
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, "", time.Unix(quando, 0), f)
}

// percorsoManuale: il PDF nella cartella Manuali dei contenuti (solo nomi semplici, niente percorsi).
func (s *Server) percorsoManuale(file string) (string, error) {
	if file == "" || file != filepath.Base(file) || strings.ContainsAny(file, `/\:`) || strings.HasPrefix(file, ".") {
		return "", errore(http.StatusBadRequest, "Nome di file non valido.")
	}
	return filepath.Join(s.Cfg.CartellaMedia, "Manuali", file), nil
}

// apiSostituisciManuale: «🔁 Sostituisci il PDF» (2026-11-11). Stesso manuale (nome, tipo, condivisione, calibrazione,
// elementi importati), file nuovo. Le pagine per i telefoni si rifanno da sole perché il nome del file cambia
// (manuali_pagine.go); poi il Master rilancia «Importa dai manuali» su questo manuale per vedere le differenze.
func (s *Server) apiSostituisciManuale(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var vecchio string
	if s.DB.QueryRow(`SELECT file FROM manuali WHERE id = ?`, id).Scan(&vecchio) != nil {
		return nil, errNonTrovato
	}
	if r.ContentLength > maxManuale {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande: massimo 1 GB.")
	}
	br := bufio.NewReaderSize(r.Body, 64<<10)
	if inizio, _ := br.Peek(5); !bytes.Equal(inizio, []byte("%PDF-")) {
		return nil, errore(http.StatusUnsupportedMediaType, "Il file non è un PDF.")
	}
	file := hex.EncodeToString(cripto.Casuali(8)) + ".pdf"
	p, err := s.percorsoManuale(file)
	if err != nil {
		return nil, err
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(f, io.LimitReader(br, maxManuale+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil || n > maxManuale {
		os.Remove(p)
		return nil, errore(http.StatusBadRequest, "Caricamento interrotto o file troppo grande.")
	}
	orig := nomeOriginaleSicuro(r.URL.Query().Get("file"), "pdf")
	if _, err := s.DB.Exec(`UPDATE manuali SET file = ?, dimensione = ?, nome_originale = CASE WHEN ? = '' THEN nome_originale ELSE ? END, creato_il = ? WHERE id = ?`,
		file, n, orig, orig, db.Ora(), id); err != nil {
		os.Remove(p)
		return nil, err
	}
	if v, err := s.percorsoManuale(vecchio); err == nil {
		os.Remove(v)
	}
	os.RemoveAll(s.cartellaPagine(id))
	os.Remove(s.percorsoOCR(id)) // il testo OCR era del PDF vecchio
	s.avvisaManuali()
	return map[string]any{"id": id, "dimensione": n}, nil
}
