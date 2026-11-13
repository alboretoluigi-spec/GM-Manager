package server

// Pagine pronte dei manuali (fase M2, docs/PIANO_MEDIA_MANUALI.md, 2026-11-11). Al tavolo ogni telefono apriva il PDF a
// pezzi da 1 MB: con manuali da centinaia di MB pieni di immagini, ogni pagina costava molti MB. Ora il PC del Master
// (nel browser, con lo stesso pdf.js) trasforma una volta le pagine dei manuali condivisi in immagini WebP leggere e ne
// estrae il testo per la ricerca; i telefoni sfogliano quelle. Restano nella cartella dei dati (non in Media) e si
// leggono solo con lo stesso permesso del PDF. Se il PDF cambia (nome del file diverso) si ricomincia da capo.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxPagine      = 3000
	maxImmaginePag = 4 << 20
	maxTestoPagine = 40 << 20
)

type infoPagine struct {
	File      string `json:"file"`      // il PDF da cui vengono (cambia se il manuale viene sostituito)
	Totale    int    `json:"totale"`    // pagine del PDF
	Larghezza int    `json:"larghezza"` // in pixel
}

// statoPagine: quello che serve alla pagina (anche nell'elenco dei manuali).
type statoPagine struct {
	Totale   int    `json:"totale"`
	Pronte   int    `json:"pronte"`
	Testo    bool   `json:"testo"`
	Versione string `json:"versione,omitempty"`
	Mancanti []int  `json:"mancanti,omitempty"` // solo per il Master: le pagine ancora da preparare
}

func (s *Server) cartellaPagine(id string) string {
	return filepath.Join(s.Cfg.CartellaDati, "manuali_pagine", filepath.Base(id))
}

func (s *Server) leggiInfoPagine(id string) (infoPagine, bool) {
	var in infoPagine
	b, err := os.ReadFile(filepath.Join(s.cartellaPagine(id), "info.json"))
	if err != nil || json.Unmarshal(b, &in) != nil {
		return in, false
	}
	return in, true
}

// filePDF: il nome del file del manuale ("" se non esiste).
func (s *Server) filePDF(id string) string {
	var f string
	s.DB.QueryRow(`SELECT file FROM manuali WHERE id = ?`, id).Scan(&f)
	return f
}

// statoPagineDi: pagine pronte di un manuale; quelle di un PDF diverso da quello attuale non contano.
func (s *Server) statoPagineDi(id string, conMancanti bool) statoPagine {
	in, ok := s.leggiInfoPagine(id)
	if !ok || in.File != s.filePDF(id) || in.Totale <= 0 {
		return statoPagine{}
	}
	st := statoPagine{Totale: in.Totale, Versione: strings.TrimSuffix(in.File, ".pdf")}
	dir := s.cartellaPagine(id)
	for n := 1; n <= in.Totale; n++ {
		if fileEsiste(filepath.Join(dir, nomePagina(n))) {
			st.Pronte++
		} else if conMancanti {
			st.Mancanti = append(st.Mancanti, n)
		}
	}
	st.Testo = fileEsiste(filepath.Join(dir, "testo.json"))
	return st
}

func nomePagina(n int) string { return "p" + strconv.Itoa(n) + ".webp" }

func (s *Server) rottePagineManuali(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/manuali/{id}/pagine", api(s.apiStatoPagine))
	mux.Handle("PUT /api/v1/manuali/{id}/pagine/{n}", api(s.apiSalvaPagina))
	mux.Handle("GET /api/v1/manuali/{id}/pagine/{n}", http.HandlerFunc(s.servipagina))
	mux.Handle("PUT /api/v1/manuali/{id}/testo", api(s.apiSalvaTesto))
	mux.Handle("GET /api/v1/manuali/{id}/testo", http.HandlerFunc(s.serviTesto))
}

func (s *Server) apiStatoPagine(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.puoLeggereManuale(u, id) {
		return nil, errNonTrovato
	}
	return s.statoPagineDi(id, u.Master()), nil
}

// apiSalvaPagina: il PC del Master manda una pagina in WebP (?totale=N&larghezza=W&file=<file del PDF letto>).
func (s *Server) apiSalvaPagina(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	file := s.filePDF(id)
	if file == "" {
		return nil, errNonTrovato
	}
	q := r.URL.Query()
	n, _ := strconv.Atoi(r.PathValue("n"))
	tot, _ := strconv.Atoi(q.Get("totale"))
	larg, _ := strconv.Atoi(q.Get("larghezza"))
	if tot < 1 || tot > maxPagine || n < 1 || n > tot || larg < 100 || larg > 4000 {
		return nil, errDatiNonValidi
	}
	if q.Get("file") != "" && q.Get("file") != file {
		return nil, errore(http.StatusConflict, "Il manuale è stato sostituito: la preparazione riparte con il PDF nuovo.")
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, maxImmaginePag+1))
	if err != nil || len(b) > maxImmaginePag {
		return nil, errore(http.StatusRequestEntityTooLarge, "Pagina troppo grande.")
	}
	if len(b) < 12 || !bytes.Equal(b[:4], []byte("RIFF")) || !bytes.Equal(b[8:12], []byte("WEBP")) {
		return nil, errore(http.StatusUnsupportedMediaType, "La pagina deve essere un'immagine WebP.")
	}
	dir := s.cartellaPagine(id)
	in, ok := s.leggiInfoPagine(id)
	if !ok || in.File != file || in.Totale != tot || in.Larghezza != larg {
		os.RemoveAll(dir) // PDF nuovo o preparazione diversa: si ricomincia
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
		j, _ := json.Marshal(infoPagine{File: file, Totale: tot, Larghezza: larg})
		if err := os.WriteFile(filepath.Join(dir, "info.json"), j, 0o600); err != nil {
			return nil, err
		}
	}
	if err := scriviAtomico(filepath.Join(dir, nomePagina(n)), b); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// apiSalvaTesto: il testo di tutte le pagine, per la ricerca sui telefoni ({"pagine": ["testo p.1", ...]}).
func (s *Server) apiSalvaTesto(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	in, ok := s.leggiInfoPagine(id)
	if !ok || in.File != s.filePDF(id) {
		return nil, errore(http.StatusConflict, "Prima vanno preparate le pagine.")
	}
	var dati struct {
		Pagine []string `json:"pagine"`
	}
	if err := leggiJSONMax(r, &dati, maxTestoPagine); err != nil {
		return nil, err
	}
	if len(dati.Pagine) != in.Totale {
		return nil, errDatiNonValidi
	}
	b, _ := json.Marshal(dati)
	if err := scriviAtomico(filepath.Join(s.cartellaPagine(id), "testo.json"), b); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// servipagina: con ?v=<versione> giusta la pagina resta per sempre nella memoria del telefono (cambia solo se cambia il PDF).
func (s *Server) servipagina(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.puoLeggereManuale(utenteDa(r.Context()), id) {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	in, ok := s.leggiInfoPagine(id)
	n, _ := strconv.Atoi(r.PathValue("n"))
	if !ok || in.File != s.filePDF(id) || n < 1 || n > in.Totale {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	p := filepath.Join(s.cartellaPagine(id), nomePagina(n))
	if !fileEsiste(p) {
		http.Error(w, "Pagina non ancora pronta.", http.StatusNotFound)
		return
	}
	if r.URL.Query().Get("v") == strings.TrimSuffix(in.File, ".pdf") {
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, no-cache")
	}
	w.Header().Set("Content-Type", "image/webp")
	http.ServeFile(w, r, p)
}

func (s *Server) serviTesto(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.puoLeggereManuale(utenteDa(r.Context()), id) {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	in, ok := s.leggiInfoPagine(id)
	p := filepath.Join(s.cartellaPagine(id), "testo.json")
	if !ok || in.File != s.filePDF(id) || !fileEsiste(p) {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeFile(w, r, p)
}

// scriviAtomico: prima in un file temporaneo, poi lo si rinomina (niente pagine a metà se Gaster Bless si ferma).
func scriviAtomico(p string, b []byte) error {
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
