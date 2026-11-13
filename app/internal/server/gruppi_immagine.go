package server

// Immagine di un gruppo: la carica il Master, la vedono il Master e i giocatori del gruppo (elenco delle
// sessioni). Il file sta nella cartella dei dati, come l'immagine della Sanità mentale (entra nei backup).

import (
	"bufio"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gasterbless/internal/cripto"
)

const maxImmagineGruppo = 10 << 20

func (s *Server) rotteImmagineGruppo(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/gruppi/{id}/immagine", api(s.apiCaricaImmagineGruppo))
	mux.Handle("DELETE /api/v1/gruppi/{id}/immagine", api(s.apiTogliImmagineGruppo))
	mux.Handle("GET /api/v1/gruppi/{id}/immagine", http.HandlerFunc(s.immagineGruppo))
}

func (s *Server) apiCaricaImmagineGruppo(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var vecchio string
	if s.DB.QueryRow(`SELECT immagine FROM gruppi WHERE id = ? AND archiviato = 0`, id).Scan(&vecchio) != nil {
		return nil, errNonTrovato
	}
	br := bufio.NewReaderSize(r.Body, 64<<10)
	inizio, _ := br.Peek(16)
	tipo, est := tipoMedia(inizio)
	if tipo != "img" {
		return nil, errore(http.StatusUnsupportedMediaType, "Carica un'immagine JPG, PNG, WebP o GIF.")
	}
	b, err := io.ReadAll(io.LimitReader(br, maxImmagineGruppo+1))
	if err != nil || len(b) > maxImmagineGruppo {
		return nil, errore(http.StatusRequestEntityTooLarge, "Immagine troppo grande: massimo 10 MB.")
	}
	file := hex.EncodeToString(cripto.Casuali(8)) + "." + est
	p, err := s.percorsoFileDati(file)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return nil, err
	}
	if _, err := s.DB.Exec(`UPDATE gruppi SET immagine = ? WHERE id = ?`, file, id); err != nil {
		s.eliminaFileDati(file)
		return nil, err
	}
	if vecchio != "" {
		s.eliminaFileDati(vecchio)
	}
	s.avvisaGruppo(id)
	return map[string]string{"immagine": file}, nil
}

func (s *Server) apiTogliImmagineGruppo(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var vecchio string
	if s.DB.QueryRow(`SELECT immagine FROM gruppi WHERE id = ?`, id).Scan(&vecchio) != nil {
		return nil, errNonTrovato
	}
	s.DB.Exec(`UPDATE gruppi SET immagine = '' WHERE id = ?`, id)
	if vecchio != "" {
		s.eliminaFileDati(vecchio)
	}
	s.avvisaGruppo(id)
	return nil, nil
}

// avvisaGruppo: il Master e i giocatori del gruppo ridisegnano l'elenco delle sessioni.
func (s *Server) avvisaGruppo(id string) {
	m := Msg{Tipo: "gruppi.aggiornati"}
	s.Hub.AlMaster(m)
	s.Hub.Pubblica("gruppo:"+id, m)
}

func (s *Server) immagineGruppo(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	id := r.PathValue("id")
	var f string
	if !s.membro(u, id) || s.DB.QueryRow(`SELECT immagine FROM gruppi WHERE id = ?`, id).Scan(&f) != nil || f == "" {
		http.Error(w, "Immagine non trovata.", http.StatusNotFound)
		return
	}
	p, err := s.percorsoFileDati(f)
	if err != nil {
		http.Error(w, "Errore.", http.StatusInternalServerError)
		return
	}
	fh, err := os.Open(p)
	if err != nil {
		http.Error(w, "Immagine mancante.", http.StatusNotFound)
		return
	}
	defer fh.Close()
	w.Header().Set("Content-Type", tipiContenuto[f[strings.LastIndex(f, ".")+1:]])
	w.Header().Set("Cache-Control", "private, max-age=86400") // il nome del file cambia a ogni caricamento (?v=)
	http.ServeContent(w, r, "", time.Time{}, fh)
}
