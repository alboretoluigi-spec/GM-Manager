package server

// MY RULEBOOKS (docs/RULEBOOK_LIBRARY.md): archivio personale dei documenti del Master, scaricati dal catalogo (L2) o
// importati da lui (L3). Tabelle regolamenti_documenti e regolamenti_categorie (migrazione 038); file in
// Dati\regolamenti\file. Solo il Master.

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
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gasterbless/internal/cripto"
)

type docMio struct {
	ID         string   `json:"id"`
	Titolo     string   `json:"titolo"`
	Gioco      string   `json:"gioco"`
	Edizione   string   `json:"edizione"`
	Tipologia  string   `json:"tipologia"`
	Lingua     string   `json:"lingua"`
	Editore    string   `json:"editore"`
	Formato    string   `json:"formato"`
	Dimensione int64    `json:"dimensione"`
	Origine    string   `json:"origine"`
	CatalogoID string   `json:"catalogoId,omitempty"`
	FonteURL   string   `json:"fonteUrl,omitempty"`
	Licenza    string   `json:"licenza,omitempty"`
	Categoria  string   `json:"categoria,omitempty"`
	VersioneDi string   `json:"versioneDi,omitempty"`
	Versione   string   `json:"versione,omitempty"`
	Note       string   `json:"note,omitempty"`
	CreatoIl   int64    `json:"creatoIl"`
	Testo      bool     `json:"testo"`    // c'è il testo per la ricerca e la lettura
	Campagne   []string `json:"campagne"` // campagne che usano il documento (L4)
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
	mux.Handle("POST /api/v1/regolamenti/miei", api(s.apiImportaRegolamento))
	mux.Handle("GET /api/v1/regolamenti/miei/cerca", api(s.apiCercaMieiRegolamenti))
	mux.Handle("PATCH /api/v1/regolamenti/miei/{id}", api(s.apiModificaMioRegolamento))
	mux.Handle("PUT /api/v1/regolamenti/miei/{id}/file", api(s.apiSostituisciMioRegolamento))
	mux.Handle("DELETE /api/v1/regolamenti/miei/{id}", api(s.apiEliminaMioRegolamento))
	mux.Handle("GET /api/v1/regolamenti/miei/{id}/testo", api(s.apiTestoMioRegolamento))
	mux.Handle("POST /api/v1/regolamenti/miei/{id}/testo", api(s.apiSalvaTestoMioRegolamento))
	mux.Handle("GET /api/v1/regolamenti/categorie", api(s.apiCategorieRegolamenti))
	mux.Handle("POST /api/v1/regolamenti/categorie", api(s.apiNuovaCategoriaRegolamenti))
	mux.Handle("PATCH /api/v1/regolamenti/categorie/{id}", api(s.apiRinominaCategoriaRegolamenti))
	mux.Handle("DELETE /api/v1/regolamenti/categorie/{id}", api(s.apiEliminaCategoriaRegolamenti))
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
		_, e := os.Stat(s.fileTestoRegolamento(d.ID))
		d.Testo = e == nil
		out = append(out, d)
	}
	rows.Close()
	for i := range out {
		out[i].Campagne = s.campagneDelDocumento(out[i].ID)
	}
	return map[string]any{"documenti": out, "tipologie": tipologieMie(s)}, rows.Err()
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

// ===== Passo L3: importazione, modifica, sostituzione, eliminazione, testo, ricerca, categorie =====

const maxImportRegolamenti = 200 << 20 // 200 MB

var estensioniRegolamenti = map[string]string{".pdf": "pdf", ".docx": "docx", ".epub": "epub", ".txt": "txt", ".text": "txt",
	".md": "md", ".markdown": "md", ".rtf": "rtf", ".html": "html", ".htm": "html", ".xhtml": "html"}

// tipologieMie: le tipologie del catalogo più quelle solo personali.
func tipologieMie(s *Server) map[string]string {
	t := map[string]string{}
	for k, v := range s.catalogoRegolamenti().Tipologie {
		t[k] = v
	}
	t["personale"] = "Documento personalizzato del Master"
	t["altro"] = "Altro"
	return t
}

func (s *Server) fileTestoRegolamento(id string) string {
	return filepath.Join(s.cartellaRegolamenti("testo"), filepath.Base(id)+".json")
}

func errFile(msg string) error { return errore(http.StatusBadRequest, msg) }

// controllaFileRegolamento: formato dall'estensione, contenuto coerente, file non danneggiato; per i formati di testo
// restituisce anche il testo (diviso in parti) per la ricerca e la lettura. I PDF mandano il testo dopo (dal browser).
func controllaFileRegolamento(nome string, b []byte) (string, []parteTesto, error) {
	formato, ok := estensioniRegolamenti[strings.ToLower(path.Ext(nome))]
	if !ok {
		return "", nil, errore(http.StatusUnsupportedMediaType, "Formato non supportato: usa PDF, DOCX, EPUB, RTF, TXT, MD o HTML.")
	}
	if len(b) == 0 {
		return "", nil, errFile("Il file è vuoto.")
	}
	danneggiato := errFile("Il file sembra danneggiato o non è davvero un " + strings.ToUpper(formato) + ": non è stato importato.")
	switch formato {
	case "pdf":
		coda := b
		if len(coda) > 2048 {
			coda = coda[len(coda)-2048:]
		}
		if !bytes.HasPrefix(b, []byte("%PDF-")) || !bytes.Contains(coda, []byte("%%EOF")) {
			return "", nil, danneggiato
		}
		return formato, nil, nil
	case "docx", "epub":
		zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			return "", nil, danneggiato
		}
		serve := map[string]string{"docx": "word/document.xml", "epub": "mimetype"}[formato]
		trovato := false
		for _, f := range zr.File {
			if f.Name == serve {
				trovato = true
			}
		}
		if !trovato {
			return "", nil, danneggiato
		}
		parti, _, err := testoDaFile("x."+formato, b)
		if err != nil {
			return "", nil, danneggiato
		}
		return formato, parti, nil
	case "rtf":
		t, err := testoDaRTF(b)
		if err != nil {
			return "", nil, danneggiato
		}
		return formato, partiDaRighe(strings.Split(t, "\n")), nil
	default: // txt, md, html
		inizio := b
		if len(inizio) > 8192 {
			inizio = inizio[:8192]
		}
		if bytes.IndexByte(inizio, 0) >= 0 {
			return "", nil, errFile("Il file non è un testo leggibile: non è stato importato.")
		}
		est := map[string]string{"txt": "x.txt", "md": "x.md", "html": "x.html"}[formato]
		parti, _, err := testoDaFile(est, b)
		if err != nil {
			return "", nil, danneggiato
		}
		return formato, parti, nil
	}
}

func improntaFile(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func (s *Server) salvaTestoRegolamento(id string, parti []parteTesto) error {
	if len(parti) == 0 {
		os.Remove(s.fileTestoRegolamento(id))
		return nil
	}
	if err := os.MkdirAll(s.cartellaRegolamenti("testo"), 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(parti)
	testiRegolamenti.togli(id)
	return scriviAtomico(s.fileTestoRegolamento(id), b)
}

// metadati comuni a importazione e modifica
type datiMioRegolamento struct {
	Titolo     *string `json:"titolo"`
	Gioco      *string `json:"gioco"`
	Edizione   *string `json:"edizione"`
	Tipologia  *string `json:"tipologia"`
	Lingua     *string `json:"lingua"`
	Editore    *string `json:"editore"`
	Categoria  *string `json:"categoria"`
	VersioneDi *string `json:"versioneDi"`
	Versione   *string `json:"versione"`
	Note       *string `json:"note"`
}

func breve(p *string, max int) (string, bool) {
	if p == nil {
		return "", false
	}
	v := strings.TrimSpace(*p)
	if utf8.RuneCountInString(v) > max {
		return "", true
	}
	return v, false
}

// controllaDati verifica lunghezze, tipologia, categoria e versione (un livello: il principale non è a sua volta una versione).
func (s *Server) controllaDati(id string, d datiMioRegolamento) error {
	for _, c := range []struct {
		p   *string
		max int
	}{{d.Titolo, 200}, {d.Gioco, 100}, {d.Edizione, 60}, {d.Lingua, 30}, {d.Editore, 100}, {d.Versione, 60}, {d.Note, 4000}} {
		if _, lungo := breve(c.p, c.max); lungo {
			return errore(http.StatusBadRequest, "Un campo è troppo lungo.")
		}
	}
	if d.Titolo != nil && strings.TrimSpace(*d.Titolo) == "" {
		return errore(http.StatusBadRequest, "Il titolo non può essere vuoto.")
	}
	if d.Tipologia != nil {
		if _, ok := tipologieMie(s)[*d.Tipologia]; !ok {
			return errore(http.StatusBadRequest, "Tipologia di documento sconosciuta.")
		}
	}
	if d.Categoria != nil && *d.Categoria != "" {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM regolamenti_categorie WHERE id = ?`, *d.Categoria).Scan(&n)
		if n == 0 {
			return errore(http.StatusBadRequest, "Categoria inesistente.")
		}
	}
	if d.VersioneDi != nil && *d.VersioneDi != "" {
		if *d.VersioneDi == id {
			return errore(http.StatusBadRequest, "Un documento non può essere una versione di sé stesso.")
		}
		var padre string
		err := s.DB.QueryRow(`SELECT COALESCE(versione_di, '') FROM regolamenti_documenti WHERE id = ?`, *d.VersioneDi).Scan(&padre)
		if err != nil {
			return errore(http.StatusBadRequest, "Il documento principale indicato non esiste.")
		}
		if padre != "" {
			return errore(http.StatusBadRequest, "Scegli come principale il documento di riferimento, non una sua versione.")
		}
		var figli int
		s.DB.QueryRow(`SELECT COUNT(*) FROM regolamenti_documenti WHERE versione_di = ?`, id).Scan(&figli)
		if id != "" && figli > 0 {
			return errore(http.StatusBadRequest, "Questo documento ha già delle versioni: non può diventare la versione di un altro.")
		}
	}
	return nil
}

func valore(p *string, predefinito string) string {
	if p == nil {
		return predefinito
	}
	return strings.TrimSpace(*p)
}

func nulla(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *Server) leggiFileCaricato(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.ContentLength > maxImportRegolamenti {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande: massimo 200 MB.")
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxImportRegolamenti))
	if err != nil {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande o caricamento interrotto: massimo 200 MB.")
	}
	return b, nil
}

func (s *Server) doppione(hash, tranne string) error {
	var id, titolo string
	if err := s.DB.QueryRow(`SELECT id, titolo FROM regolamenti_documenti WHERE impronta = ? AND id != ?`, hash, tranne).Scan(&id, &titolo); err == nil {
		return &ErrAPI{Codice: http.StatusConflict, Msg: "Questo file è già nella libreria: «" + titolo + "».", Tipo: "doppione"}
	}
	return nil
}

// apiImportaRegolamento: POST /regolamenti/miei?file=<nome>&titolo=…&gioco=…; il corpo è il file.
func (s *Server) apiImportaRegolamento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	q := r.URL.Query()
	nomeFile := filepath.Base(q.Get("file"))
	str := func(k string) *string {
		if !q.Has(k) {
			return nil
		}
		v := q.Get(k)
		return &v
	}
	d := datiMioRegolamento{Titolo: str("titolo"), Gioco: str("gioco"), Edizione: str("edizione"), Tipologia: str("tipologia"),
		Lingua: str("lingua"), Editore: str("editore"), Categoria: str("categoria"), VersioneDi: str("versioneDi"), Versione: str("versione"), Note: str("note")}
	if d.Titolo == nil || strings.TrimSpace(*d.Titolo) == "" {
		t := strings.TrimSuffix(nomeFile, path.Ext(nomeFile))
		d.Titolo = &t
	}
	if err := s.controllaDati("", d); err != nil {
		return nil, err
	}
	b, err := s.leggiFileCaricato(w, r)
	if err != nil {
		return nil, err
	}
	formato, parti, err := controllaFileRegolamento(nomeFile, b)
	if err != nil {
		return nil, err
	}
	hash := improntaFile(b)
	if err := s.doppione(hash, ""); err != nil {
		return nil, err
	}
	id := cripto.ID()
	nome := id + "." + formato
	if err := os.MkdirAll(s.cartellaRegolamenti("file"), 0o700); err != nil {
		return nil, err
	}
	if err := scriviAtomico(filepath.Join(s.cartellaRegolamenti("file"), nome), b); err != nil {
		return nil, err
	}
	ora := time.Now().Unix()
	_, err = s.DB.Exec(`INSERT INTO regolamenti_documenti (id, titolo, gioco, edizione, tipologia, lingua, editore, formato, file,
		dimensione, impronta, origine, categoria_id, versione_di, versione, note, creato_il, aggiornato_il)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'importato', ?, ?, ?, ?, ?, ?)`,
		id, valore(d.Titolo, ""), valore(d.Gioco, ""), valore(d.Edizione, ""), valore(d.Tipologia, "personale"), valore(d.Lingua, ""),
		valore(d.Editore, ""), formato, nome, len(b), hash, nulla(valore(d.Categoria, "")), nulla(valore(d.VersioneDi, "")),
		valore(d.Versione, ""), valore(d.Note, ""), ora, ora)
	if err != nil {
		os.Remove(filepath.Join(s.cartellaRegolamenti("file"), nome))
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, s.doppione(hash, "")
		}
		return nil, err
	}
	if err := s.salvaTestoRegolamento(id, parti); err != nil {
		return nil, err
	}
	s.evento("regolamento_importa", u.ID, valore(d.Titolo, ""))
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.miei", Dati: map[string]any{}})
	doc, err := s.docMio(id)
	doc.Testo = len(parti) > 0
	return doc, err
}

func (s *Server) apiModificaMioRegolamento(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	vecchio, err := s.docMio(id)
	if err != nil {
		return nil, err
	}
	var d datiMioRegolamento
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&d); err != nil {
		return nil, errDatiNonValidi
	}
	if err := s.controllaDati(id, d); err != nil {
		return nil, err
	}
	_, err = s.DB.Exec(`UPDATE regolamenti_documenti SET titolo = ?, gioco = ?, edizione = ?, tipologia = ?, lingua = ?, editore = ?,
		categoria_id = ?, versione_di = ?, versione = ?, note = ?, aggiornato_il = ? WHERE id = ?`,
		valore(d.Titolo, vecchio.Titolo), valore(d.Gioco, vecchio.Gioco), valore(d.Edizione, vecchio.Edizione),
		valore(d.Tipologia, vecchio.Tipologia), valore(d.Lingua, vecchio.Lingua), valore(d.Editore, vecchio.Editore),
		nulla(valore(d.Categoria, vecchio.Categoria)), nulla(valore(d.VersioneDi, vecchio.VersioneDi)),
		valore(d.Versione, vecchio.Versione), valore(d.Note, vecchio.Note), time.Now().Unix(), id)
	if err != nil {
		return nil, err
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.miei", Dati: map[string]any{}})
	return s.docMio(id)
}

// apiSostituisciMioRegolamento: PUT /regolamenti/miei/{id}/file?file=<nome>: nuovo file, stessi dati.
func (s *Server) apiSostituisciMioRegolamento(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	vecchio, err := s.docMio(id)
	if err != nil {
		return nil, err
	}
	b, err := s.leggiFileCaricato(w, r)
	if err != nil {
		return nil, err
	}
	formato, parti, err := controllaFileRegolamento(filepath.Base(r.URL.Query().Get("file")), b)
	if err != nil {
		return nil, err
	}
	hash := improntaFile(b)
	if err := s.doppione(hash, id); err != nil {
		return nil, err
	}
	nome := id + "." + formato
	if err := scriviAtomico(filepath.Join(s.cartellaRegolamenti("file"), nome), b); err != nil {
		return nil, err
	}
	if nome != vecchio.file {
		os.Remove(filepath.Join(s.cartellaRegolamenti("file"), filepath.Base(vecchio.file)))
	}
	if _, err := s.DB.Exec(`UPDATE regolamenti_documenti SET formato = ?, file = ?, dimensione = ?, impronta = ?, aggiornato_il = ?
		WHERE id = ?`, formato, nome, len(b), hash, time.Now().Unix(), id); err != nil {
		return nil, err
	}
	if err := s.salvaTestoRegolamento(id, parti); err != nil {
		return nil, err
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.miei", Dati: map[string]any{}})
	doc, err := s.docMio(id)
	doc.Testo = len(parti) > 0
	return doc, err
}

func (s *Server) apiEliminaMioRegolamento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	d, err := s.docMio(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if _, err := s.DB.Exec(`DELETE FROM regolamenti_documenti WHERE id = ?`, d.ID); err != nil {
		return nil, err
	}
	os.Remove(filepath.Join(s.cartellaRegolamenti("file"), filepath.Base(d.file)))
	os.Remove(s.fileTestoRegolamento(d.ID))
	testiRegolamenti.togli(d.ID)
	s.evento("regolamento_elimina", u.ID, d.Titolo)
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.miei", Dati: map[string]any{}})
	return map[string]any{"eliminato": d.ID}, nil
}

// ----- testo per la lettura e la ricerca -----

type cacheTesti struct {
	mu sync.Mutex
	m  map[string][]parteTesto
}

var testiRegolamenti = &cacheTesti{m: map[string][]parteTesto{}}

func (c *cacheTesti) togli(id string) { c.mu.Lock(); delete(c.m, id); c.mu.Unlock() }

func (s *Server) testoRegolamento(id string) []parteTesto {
	testiRegolamenti.mu.Lock()
	defer testiRegolamenti.mu.Unlock()
	if p, ok := testiRegolamenti.m[id]; ok {
		return p
	}
	var p []parteTesto
	if b, err := os.ReadFile(s.fileTestoRegolamento(id)); err == nil {
		json.Unmarshal(b, &p)
	}
	testiRegolamenti.m[id] = p
	return p
}

// apiTestoMioRegolamento: le parti del testo, a blocchi (?da=0&n=40), per leggere i documenti che non sono PDF.
func (s *Server) apiTestoMioRegolamento(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	d, err := s.docMio(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	p := s.testoRegolamento(d.ID)
	da, n := intero(r.URL.Query().Get("da"), 0), intero(r.URL.Query().Get("n"), 40)
	if n < 1 || n > 200 {
		n = 40
	}
	if da < 0 || da > len(p) {
		da = len(p)
	}
	fine := da + n
	if fine > len(p) {
		fine = len(p)
	}
	return map[string]any{"titolo": d.Titolo, "totale": len(p), "da": da, "parti": p[da:fine]}, nil
}

func intero(s string, pred int) int {
	n := 0
	if s == "" {
		return pred
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return pred
		}
		n = n*10 + int(c-'0')
		if n > 1<<30 {
			return pred
		}
	}
	return n
}

// apiSalvaTestoMioRegolamento: il testo delle pagine di un PDF, letto nel browser con pdf.js ({"pagine": [...]}).
func (s *Server) apiSalvaTestoMioRegolamento(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	d, err := s.docMio(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if d.Formato != "pdf" {
		return nil, errore(http.StatusBadRequest, "Il testo si manda così solo per i PDF.")
	}
	var in struct {
		Pagine []string `json:"pagine"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<20)).Decode(&in); err != nil || len(in.Pagine) == 0 || len(in.Pagine) > 5000 {
		return nil, errDatiNonValidi
	}
	parti := testoDaPagine(in.Pagine)
	if err := s.salvaTestoRegolamento(d.ID, parti); err != nil {
		return nil, err
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.miei", Dati: map[string]any{}})
	return map[string]any{"parti": len(parti)}, nil
}

type risultatoMio struct {
	Doc       string `json:"doc"`
	Titolo    string `json:"titolo"`
	Gioco     string `json:"gioco"`
	Dove      string `json:"dove,omitempty"`
	Pagina    int    `json:"pagina,omitempty"`
	Parte     int    `json:"parte"`
	Estratto  string `json:"estratto"`
	NelTitolo bool   `json:"nelTitolo,omitempty"`
}

// apiCercaMieiRegolamenti: GET /regolamenti/miei/cerca?q=… cerca tutte le parole nel titolo e nel testo dei documenti
// (senza distinguere maiuscole e accenti); al massimo 3 punti per documento e 40 risultati.
func (s *Server) apiCercaMieiRegolamenti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	parole := strings.Fields(normalizza(r.URL.Query().Get("q")))
	if len(parole) == 0 || len(parole) > 12 {
		return nil, errore(http.StatusBadRequest, "Scrivi da 1 a 12 parole da cercare.")
	}
	rows, err := s.DB.Query(`SELECT id, titolo, gioco FROM regolamenti_documenti ORDER BY titolo COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	type voce struct{ id, titolo, gioco string }
	var docs []voce
	for rows.Next() {
		var v voce
		rows.Scan(&v.id, &v.titolo, &v.gioco)
		docs = append(docs, v)
	}
	rows.Close()
	tutte := func(t string) bool {
		for _, p := range parole {
			if !strings.Contains(t, p) {
				return false
			}
		}
		return true
	}
	out := []risultatoMio{}
	for _, d := range docs {
		if tutte(normalizza(d.titolo + " " + d.gioco)) {
			out = append(out, risultatoMio{Doc: d.id, Titolo: d.titolo, Gioco: d.gioco, NelTitolo: true, Parte: -1})
		}
		trovati := 0
		for i, p := range s.testoRegolamento(d.id) {
			if trovati >= 3 || len(out) >= 40 {
				break
			}
			n := []rune(senzaAccenti.Replace(strings.ToLower(p.T)))
			if !tutte(string(n)) {
				continue
			}
			trovati++
			out = append(out, risultatoMio{Doc: d.id, Titolo: d.titolo, Gioco: d.gioco, Dove: p.E, Pagina: p.P, Parte: i,
				Estratto: estratto([]rune(p.T), n, []rune(parole[0]))})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].NelTitolo && !out[b].NelTitolo })
	return map[string]any{"risultati": out}, nil
}

// estratto: circa 240 caratteri attorno alla prima parola trovata.
func estratto(orig, norm, parola []rune) string {
	pos := strings.Index(string(norm), string(parola))
	if pos < 0 || len(orig) != len(norm) {
		if len(orig) > 240 {
			return string(orig[:240]) + "…"
		}
		return string(orig)
	}
	i := utf8.RuneCountInString(string(norm)[:pos])
	da, a := i-100, i+140
	pre, post := "…", "…"
	if da <= 0 {
		da, pre = 0, ""
	}
	if a >= len(orig) {
		a, post = len(orig), ""
	}
	return pre + strings.TrimSpace(string(orig[da:a])) + post
}

// ----- categorie -----

func (s *Server) apiCategorieRegolamenti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT c.id, c.nome, (SELECT COUNT(*) FROM regolamenti_documenti d WHERE d.categoria_id = c.id)
		FROM regolamenti_categorie c ORDER BY c.nome COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, nome string
		var n int
		rows.Scan(&id, &nome, &n)
		out = append(out, map[string]any{"id": id, "nome": nome, "documenti": n})
	}
	return map[string]any{"categorie": out}, rows.Err()
}

func nomeCategoria(r *http.Request) (string, error) {
	var in struct {
		Nome string `json:"nome"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
		return "", errDatiNonValidi
	}
	n := strings.TrimSpace(in.Nome)
	if n == "" || utf8.RuneCountInString(n) > 60 {
		return "", errore(http.StatusBadRequest, "Il nome della categoria va da 1 a 60 caratteri.")
	}
	return n, nil
}

func (s *Server) apiNuovaCategoriaRegolamenti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	n, err := nomeCategoria(r)
	if err != nil {
		return nil, err
	}
	id := cripto.ID()
	if _, err := s.DB.Exec(`INSERT INTO regolamenti_categorie (id, nome, creato_il) VALUES (?, ?, ?)`, id, n, time.Now().Unix()); err != nil {
		return nil, errore(http.StatusConflict, "Esiste già una categoria con questo nome.")
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.miei", Dati: map[string]any{}})
	return map[string]any{"id": id, "nome": n}, nil
}

func (s *Server) apiRinominaCategoriaRegolamenti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	n, err := nomeCategoria(r)
	if err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`UPDATE regolamenti_categorie SET nome = ? WHERE id = ?`, n, r.PathValue("id"))
	if err != nil {
		return nil, errore(http.StatusConflict, "Esiste già una categoria con questo nome.")
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return nil, errore(http.StatusNotFound, "Categoria inesistente.")
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.miei", Dati: map[string]any{}})
	return map[string]any{"id": r.PathValue("id"), "nome": n}, nil
}

// apiEliminaCategoriaRegolamenti: i documenti restano, senza categoria.
func (s *Server) apiEliminaCategoriaRegolamenti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`DELETE FROM regolamenti_categorie WHERE id = ?`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if k, _ := res.RowsAffected(); k == 0 {
		return nil, errore(http.StatusNotFound, "Categoria inesistente.")
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.miei", Dati: map[string]any{}})
	return map[string]any{"eliminata": r.PathValue("id")}, nil
}
