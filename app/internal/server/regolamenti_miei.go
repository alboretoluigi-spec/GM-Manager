package server

// MY RULEBOOKS (docs/RULEBOOK_LIBRARY.md): archivio personale dei documenti del Master, scaricati dal catalogo (L2) o
// importati da lui (L3). Tabelle regolamenti_documenti e regolamenti_categorie (migrazione 038); file in
// Dati\regolamenti\file. Solo il Master.

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type docMio struct {
	ID         string `json:"id"`
	Titolo     string `json:"titolo"`
	Gioco      string `json:"gioco"`
	Edizione   string `json:"edizione"`
	Tipologia  string `json:"tipologia"`
	Lingua     string `json:"lingua"`
	Editore    string `json:"editore"`
	Formato    string `json:"formato"`
	Dimensione int64  `json:"dimensione"`
	Origine    string `json:"origine"`
	CatalogoID string `json:"catalogoId,omitempty"`
	FonteURL   string `json:"fonteUrl,omitempty"`
	Licenza    string `json:"licenza,omitempty"`
	Categoria  string `json:"categoria,omitempty"`
	VersioneDi string `json:"versioneDi,omitempty"`
	Versione   string `json:"versione,omitempty"`
	Note       string `json:"note,omitempty"`
	CreatoIl   int64  `json:"creatoIl"`
	file       string
}

const colonneDocMio = `id, titolo, gioco, edizione, tipologia, lingua, editore, formato, dimensione, origine, catalogo_id, fonte_url,
	licenza, COALESCE(categoria_id, ''), COALESCE(versione_di, ''), versione, note, creato_il, file`

func scanDocMio(sc interface{ Scan(...any) error }) (docMio, error) {
	var d docMio
	err := sc.Scan(&d.ID, &d.Titolo, &d.Gioco, &d.Edizione, &d.Tipologia, &d.Lingua, &d.Editore, &d.Formato, &d.Dimensione,
		&d.Origine, &d.CatalogoID, &d.FonteURL, &d.Licenza, &d.Categoria, &d.VersioneDi, &d.Versione, &d.Note, &d.CreatoIl, &d.file)
	return d, err
}

func (s *Server) rotteMieiRegolamenti(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/regolamenti/miei", api(s.apiMieiRegolamenti))
	mux.HandleFunc("GET /api/v1/regolamenti/miei/{id}/file", s.fileMioRegolamento)
}

func (s *Server) docMio(id string) (docMio, error) {
	d, err := scanDocMio(s.DB.QueryRow(`SELECT `+colonneDocMio+` FROM regolamenti_documenti WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return d, errore(http.StatusNotFound, "Documento non trovato.")
	}
	return d, err
}

func (s *Server) apiMieiRegolamenti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT ` + colonneDocMio + ` FROM regolamenti_documenti ORDER BY gioco COLLATE NOCASE, titolo COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []docMio{}
	for rows.Next() {
		d, err := scanDocMio(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return map[string]any{"documenti": out}, rows.Err()
}

var tipiMimeRegolamenti = map[string]string{"pdf": "application/pdf", "zip": "application/zip", "epub": "application/epub+zip",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "txt": "text/plain; charset=utf-8",
	"md": "text/plain; charset=utf-8", "rtf": "application/rtf", "html": "text/plain; charset=utf-8"}

// fileMioRegolamento: il file per la consultazione (inline) o il salvataggio (?scarica=1). L'HTML si manda come testo.
func (s *Server) fileMioRegolamento(w http.ResponseWriter, r *http.Request) {
	if _, err := richiedi(r, "master"); err != nil {
		http.Error(w, "Solo il Master.", http.StatusForbidden)
		return
	}
	d, err := s.docMio(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Documento non trovato.", http.StatusNotFound)
		return
	}
	p := filepath.Join(s.cartellaRegolamenti("file"), filepath.Base(d.file))
	f, err := os.Open(p)
	if err != nil {
		http.Error(w, "File mancante sul disco.", http.StatusNotFound)
		return
	}
	defer f.Close()
	tipo := tipiMimeRegolamenti[d.Formato]
	if tipo == "" {
		tipo = "application/octet-stream"
	}
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	modo := "inline"
	if r.URL.Query().Get("scarica") == "1" {
		modo = "attachment"
	}
	nome := strings.Map(func(c rune) rune {
		if strings.ContainsRune(`"\/:*?<>|`, c) || c < 32 {
			return '_'
		}
		return c
	}, d.Titolo) + "." + d.Formato
	w.Header().Set("Content-Disposition", modo+`; filename="`+nome+`"`)
	fi, _ := f.Stat()
	http.ServeContent(w, r, nome, fi.ModTime(), f)
}
