package server

// Background privato del personaggio (PDF o Word .docx).
// Lo vedono solo il giocatore autore e il Master. Il file è salvato cifrato nella cartella dati,
// con un nome scelto dal server: il nome originale resta solo come informazione.

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const maxBackground = 15 << 20

func (s *Server) rotteBackground(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/background", api(s.apiElencoBackground))
	mux.Handle("GET /api/v1/personaggi/{id}/background/info", api(s.apiInfoBackground))
	mux.Handle("GET /api/v1/personaggi/{id}/background", http.HandlerFunc(s.scaricaBackground))
	mux.Handle("PUT /api/v1/personaggi/{id}/background", api(s.apiCaricaBackground))
	mux.Handle("DELETE /api/v1/personaggi/{id}/background", api(s.apiEliminaBackground))
}

type infoBackground struct {
	Personaggio   string `json:"personaggio"`
	NomePG        string `json:"nomePersonaggio"`
	NomeGiocatore string `json:"nomeGiocatore"`
	Classe        string `json:"classe"`
	Nome          string `json:"nome"`
	Tipo          string `json:"tipo"`
	Dimensione    int64  `json:"dimensione"`
	Quando        int64  `json:"quando"`
}

const selBackground = `SELECT b.personaggio_id, p.nome, COALESCE(u.nome,''), COALESCE(v.riepilogo,'{}'), b.nome_originale, b.tipo, b.dimensione, b.caricato_il
	FROM background b JOIN personaggi p ON p.id = b.personaggio_id LEFT JOIN utenti u ON u.id = p.proprietario_id
	LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente`

func scanBackground(sc interface{ Scan(...any) error }) (*infoBackground, error) {
	b := &infoBackground{}
	var riep string
	if err := sc.Scan(&b.Personaggio, &b.NomePG, &b.NomeGiocatore, &riep, &b.Nome, &b.Tipo, &b.Dimensione, &b.Quando); err != nil {
		return nil, err
	}
	var r riepilogo
	if jsonUnmarshal(riep, &r) == nil && r.Classe != "" {
		b.Classe = r.Classe
		if r.Livello > 0 {
			b.Classe += " " + strconv.Itoa(r.Livello)
		}
	}
	return b, nil
}

// apiElencoBackground: solo il Master vede l'elenco di tutti i background.
func (s *Server) apiElencoBackground(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(selBackground + ` ORDER BY b.caricato_il DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*infoBackground{}
	for rows.Next() {
		b, err := scanBackground(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func (s *Server) apiInfoBackground(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, id) {
		return nil, errNonTrovato
	}
	b, err := scanBackground(s.DB.QueryRow(selBackground+` WHERE b.personaggio_id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]any{"background": nil}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"background": b}, nil
}

// scaricaBackground restituisce il file decifrato. Va sempre scaricato, mai mostrato dal browser
// come pagina: il visualizzatore della scheda lo apre in modo sicuro.
func (s *Server) scaricaBackground(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	id := r.PathValue("id")
	if !u.Approvato() || !s.puoLeggerePersonaggio(u, id) {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	var file, tipo string
	if s.DB.QueryRow(`SELECT file, tipo FROM background WHERE personaggio_id = ?`, id).Scan(&file, &tipo) != nil {
		http.Error(w, "Nessun background.", http.StatusNotFound)
		return
	}
	dati, err := s.leggiFileCifrato(file, "background:"+id)
	if err != nil {
		http.Error(w, "File non leggibile.", http.StatusInternalServerError)
		return
	}
	var prop string
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, id).Scan(&prop)
	if u.Master() && prop != u.ID {
		s.registroDati(u.ID, "background_letto", prop)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="background.`+tipo+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(dati)
}

// apiCaricaBackground: solo il giocatore autore carica o sostituisce il proprio background.
func (s *Server) apiCaricaBackground(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var prop string
	if s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, id).Scan(&prop) != nil || !s.puoLeggerePersonaggio(u, id) {
		return nil, errNonTrovato
	}
	if prop != u.ID {
		return nil, errore(http.StatusForbidden, "Il background lo carica solo il giocatore del personaggio.")
	}
	nome, _ := url.QueryUnescape(r.Header.Get("X-Nome-File"))
	dati, err := io.ReadAll(io.LimitReader(r.Body, maxBackground+1))
	if err != nil {
		return nil, errore(http.StatusBadRequest, "Caricamento interrotto.")
	}
	if len(dati) > maxBackground {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande: massimo 15 MB.")
	}
	tipo := tipoBackground(dati)
	if tipo == "" {
		return nil, errore(http.StatusUnsupportedMediaType, "Formato non supportato: carica un PDF o un documento Word (.docx).")
	}
	nome = nomeOriginaleSicuro(nome, tipo)
	file := cripto.ID() + ".bin"
	if err := s.scriviFileCifrato(file, dati, "background:"+id); err != nil {
		return nil, err
	}
	somma := sha256.Sum256(dati)
	var vecchio string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		tx.QueryRow(`SELECT file FROM background WHERE personaggio_id = ?`, id).Scan(&vecchio)
		_, err := tx.Exec(`INSERT INTO background(personaggio_id, file, tipo, nome_originale, dimensione, sha256, caricato_da, caricato_il) VALUES(?,?,?,?,?,?,?,?)
			ON CONFLICT(personaggio_id) DO UPDATE SET file = excluded.file, tipo = excluded.tipo, nome_originale = excluded.nome_originale,
			dimensione = excluded.dimensione, sha256 = excluded.sha256, caricato_da = excluded.caricato_da, caricato_il = excluded.caricato_il`,
			id, file, tipo, nome, len(dati), hex.EncodeToString(somma[:]), u.ID, db.Ora())
		return err
	})
	if err != nil {
		s.eliminaFileDati(file)
		return nil, err
	}
	if vecchio != "" {
		s.eliminaFileDati(vecchio)
	}
	s.Hub.AlMaster(Msg{Tipo: "background.aggiornato", Dati: map[string]string{"personaggio": id}})
	return s.apiInfoBackground(w, r)
}

func (s *Server) apiEliminaBackground(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var prop, file string
	if s.DB.QueryRow(`SELECT COALESCE(p.proprietario_id,''), b.file FROM personaggi p JOIN background b ON b.personaggio_id = p.id WHERE p.id = ?`, id).Scan(&prop, &file) != nil ||
		!s.puoLeggerePersonaggio(u, id) {
		return nil, errNonTrovato
	}
	if prop != u.ID {
		return nil, errore(http.StatusForbidden, "Il background lo elimina solo il giocatore del personaggio.")
	}
	if _, err := s.DB.Exec(`DELETE FROM background WHERE personaggio_id = ?`, id); err != nil {
		return nil, err
	}
	s.eliminaFileDati(file)
	s.Hub.AlMaster(Msg{Tipo: "background.aggiornato", Dati: map[string]string{"personaggio": id}})
	return nil, nil
}

// tipoBackground riconosce il formato dal contenuto, non dal nome: "pdf", "docx" oppure "".
func tipoBackground(b []byte) string {
	if bytes.HasPrefix(b, []byte("%PDF-")) {
		return "pdf"
	}
	if !bytes.HasPrefix(b, []byte("PK\x03\x04")) {
		return ""
	}
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return ""
	}
	for _, f := range z.File {
		if f.Name == "word/document.xml" {
			return "docx"
		}
	}
	return ""
}

func (s *Server) scriviFileCifrato(nome string, dati []byte, contesto string) error {
	p, err := s.percorsoFileDati(nome)
	if err != nil {
		return err
	}
	return os.WriteFile(p, s.Chiavi.Cifra(dati, contesto), 0o600)
}

func (s *Server) leggiFileCifrato(nome, contesto string) ([]byte, error) {
	p, err := s.percorsoFileDati(nome)
	if err != nil {
		return nil, err
	}
	c, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return s.Chiavi.Decifra(c, contesto)
}

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

// nomeOriginaleSicuro pulisce il nome del file mostrato all'utente (non usato per salvare su disco).
func nomeOriginaleSicuro(n, tipo string) string {
	n = filepath.Base(strings.ReplaceAll(n, "\\", "/"))
	var b strings.Builder
	for _, r := range n {
		if r >= 32 && r != '/' && r != '\\' && r != ':' && r != '*' && r != '?' && r != '"' && r != '<' && r != '>' && r != '|' {
			b.WriteRune(r)
		}
	}
	n = strings.TrimSpace(b.String())
	if r := []rune(n); len(r) > 100 {
		n = string(r[:100])
	}
	if n == "" || n == "." {
		n = "background." + tipo
	}
	return n
}
