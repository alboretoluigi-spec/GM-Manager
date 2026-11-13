package server

// File dell'app più veloci (richiesta dell'utente, 2026-11-08: «velocizzare l'app»).
// Prima pagine, script, stili e dati (circa 5 MB) si riscaricavano interi a ogni apertura: i file dentro il programma
// non hanno data di modifica, quindi il browser non poteva chiedere «è cambiato?». Ora ogni file ha un'impronta (ETag,
// dal contenuto): se non è cambiato il server risponde «304, usa la tua copia» senza rimandarlo. Inoltre testi, script,
// stili, dati e disegni SVG viaggiano compressi (gzip): circa un quarto del peso. Le immagini e i video sono già compressi.

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
)

type fileStatico struct {
	etag string
	gz   []byte // versione compressa (nil se non conviene)
	tipo string
}

var comprimibili = map[string]bool{".js": true, ".mjs": true, ".css": true, ".json": true, ".html": true, ".svg": true, ".txt": true,
	".webmanifest": true, ".ttf": true, ".pfb": true, ".map": true, ".xml": true, ".md": true}

type cacheStatiche struct {
	mu       sync.Mutex
	file     map[string]*fileStatico
	versione string
}

// infoStatico: impronta e versione compressa di un file dell'app (calcolate una volta: il contenuto non cambia finché
// non si aggiorna il programma). nome è relativo alla radice di s.Web; le cartelle diventano la loro index.html.
func (s *Server) infoStatico(nome string) *fileStatico {
	if st, err := fs.Stat(s.Web, nome); err == nil && st.IsDir() {
		nome = path.Join(nome, "index.html")
	}
	s.fileApp.mu.Lock()
	defer s.fileApp.mu.Unlock()
	if s.fileApp.file == nil {
		s.fileApp.file = map[string]*fileStatico{}
	}
	if f, ok := s.fileApp.file[nome]; ok {
		return f
	}
	dati, err := fs.ReadFile(s.Web, nome)
	if err != nil {
		s.fileApp.file[nome] = nil
		return nil
	}
	h := sha256.Sum256(dati)
	f := &fileStatico{etag: `"` + hex.EncodeToString(h[:12]) + `"`, tipo: mime.TypeByExtension(path.Ext(nome))}
	if comprimibili[strings.ToLower(path.Ext(nome))] && len(dati) > 1024 {
		var b bytes.Buffer
		z, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
		z.Write(dati)
		z.Close()
		if b.Len() < len(dati)*9/10 {
			f.gz = b.Bytes()
		}
	}
	if f.tipo == "" && strings.HasSuffix(nome, ".webmanifest") {
		f.tipo = "application/manifest+json"
	}
	s.fileApp.file[nome] = f
	return f
}

// serviStatico risponde con il file nome di s.Web: 304 se il browser ha già la stessa versione, compresso se il browser
// lo accetta; altrimenti passa al FileServer (che gestisce anche le richieste parziali dei video).
func (s *Server) serviStatico(w http.ResponseWriter, r *http.Request, fsrv http.Handler, nome string) {
	f := s.infoStatico(nome)
	if f == nil {
		fsrv.ServeHTTP(w, r)
		return
	}
	w.Header().Set("ETag", f.etag)
	if f.gz == nil || r.Header.Get("Range") != "" {
		fsrv.ServeHTTP(w, r) // con l'ETag già impostato risponde da solo 304 a If-None-Match
		return
	}
	w.Header().Add("Vary", "Accept-Encoding")
	if inm := r.Header.Get("If-None-Match"); inm != "" && strings.Contains(inm, f.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if !accettaGzip(r) || strings.HasSuffix(r.URL.Path, "/index.html") {
		fsrv.ServeHTTP(w, r)
		return
	}
	if f.tipo != "" {
		w.Header().Set("Content-Type", f.tipo)
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Length", strconv.Itoa(len(f.gz)))
	if r.Method == http.MethodHead {
		return
	}
	w.Write(f.gz)
}

func accettaGzip(r *http.Request) bool {
	for _, p := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if v, q, _ := strings.Cut(strings.TrimSpace(p), ";"); strings.EqualFold(strings.TrimSpace(v), "gzip") {
			return !strings.Contains(strings.ReplaceAll(q, " ", ""), "q=0") || strings.Contains(q, "q=0.")
		}
	}
	return false
}

// versioneApp: impronta di tutti i file dell'app (cambia solo con un aggiornamento del programma), più l'ora di avvio del
// server. Le pagine la ricontrollano ogni 5 minuti (web/ui/aggiorna.js) e si ricaricano da sole se è cambiata.
func (s *Server) versioneApp() string {
	s.fileApp.mu.Lock()
	v := s.fileApp.versione
	s.fileApp.mu.Unlock()
	if v != "" {
		return v
	}
	h := sha256.New()
	fs.WalkDir(s.Web, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if b, err := fs.ReadFile(s.Web, p); err == nil {
			h.Write([]byte(p))
			h.Write(b)
		}
		return nil
	})
	v = hex.EncodeToString(h.Sum(nil)[:8]) + "-" + strconv.FormatInt(s.avviatoIl.UnixMilli(), 36)
	s.fileApp.mu.Lock()
	s.fileApp.versione = v
	s.fileApp.mu.Unlock()
	return v
}
