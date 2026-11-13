package server

// Testo OCR di un manuale (2026-11-11). Per i PDF scansionati il testo nascosto può avere errori (per «La Maledizione di
// l'antagonista» le tabelle delle caratteristiche dell'appendice D mancavano o erano storpiate). Il Master può caricare accanto al
// manuale un file JSON «gasterbless-ocr» (righe con testo e riquadro in punti PDF, origine in basso a sinistra): «📥 Importa dai
// manuali» e «Dal manuale…» del bestiario leggono quelle righe al posto del testo del PDF. Il PDF non cambia. Si conserva
// solo il necessario (pagina, riquadro, testo) in forma compatta, in Dati\manuali_ocr: si cancella con il manuale e quando
// il PDF viene sostituito. Solo il Master.

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gasterbless/internal/db"
)

const maxOCR = 64 << 20

type rigaOCR [5]any // x0, y0, x1, y1, testo

type paginaOCR struct {
	P int       `json:"p"`
	L []rigaOCR `json:"l"`
}

type fileOCRCompatto struct {
	Formato  string      `json:"formato"`
	Titolo   string      `json:"titolo,omitempty"`
	Caricato int64       `json:"caricato"`
	Pagine   []paginaOCR `json:"pagine"`
}

// cartella a parte: quella delle pagine si svuota quando la preparazione riparte da capo
func (s *Server) percorsoOCR(id string) string {
	return filepath.Join(s.Cfg.CartellaDati, "manuali_ocr", filepath.Base(id)+".json")
}

func (s *Server) rotteManualiOCR(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/manuali/{id}/ocr", http.HandlerFunc(s.fileOCR))
	mux.Handle("PUT /api/v1/manuali/{id}/ocr", api(s.apiCaricaOCR))
	mux.Handle("DELETE /api/v1/manuali/{id}/ocr", api(s.apiTogliOCR))
}

// compattaOCR legge un file «gasterbless-ocr» (formato del 2026: pagine → blocchi → paragrafi → righe con testo e bbox).
func compattaOCR(raw []byte) (fileOCRCompatto, int, error) {
	var in struct {
		Format string `json:"format"`
		Title  string `json:"title"`
		Pages  []struct {
			PdfPage int `json:"pdf_page"`
			Blocks  []struct {
				Paragraphs []struct {
					Lines []struct {
						Text string    `json:"text"`
						Bbox []float64 `json:"bbox"`
					} `json:"lines"`
				} `json:"paragraphs"`
			} `json:"blocks"`
		} `json:"pages"`
	}
	out := fileOCRCompatto{Formato: "gasterbless-ocr-compatto"}
	if json.Unmarshal(raw, &in) != nil || in.Format != "gasterbless-ocr" || len(in.Pages) == 0 || len(in.Pages) > 5000 {
		return out, 0, errore(http.StatusBadRequest, "Questo non è un file di testo OCR di Gaster Bless (formato «gasterbless-ocr»).")
	}
	out.Titolo = limitaTesto(in.Title, 120)
	righe := 0
	for _, p := range in.Pages {
		if p.PdfPage < 1 || p.PdfPage > 100000 {
			continue
		}
		pg := paginaOCR{P: p.PdfPage, L: []rigaOCR{}}
		for _, b := range p.Blocks {
			for _, par := range b.Paragraphs {
				for _, l := range par.Lines {
					t := strings.TrimSpace(l.Text)
					if t == "" || len(l.Bbox) != 4 || !utf8.ValidString(t) {
						continue
					}
					if len(t) > 2000 {
						t = t[:2000]
					}
					ok := true
					for _, v := range l.Bbox {
						if math.IsNaN(v) || math.IsInf(v, 0) || v < -100 || v > 20000 {
							ok = false
						}
					}
					if !ok {
						continue
					}
					r1 := func(v float64) float64 { return math.Round(v*10) / 10 }
					pg.L = append(pg.L, rigaOCR{r1(l.Bbox[0]), r1(l.Bbox[1]), r1(l.Bbox[2]), r1(l.Bbox[3]), t})
					righe++
				}
			}
		}
		out.Pagine = append(out.Pagine, pg)
	}
	if righe == 0 {
		return out, 0, errore(http.StatusBadRequest, "Il file di testo OCR non contiene righe.")
	}
	return out, righe, nil
}

func (s *Server) apiCaricaOCR(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM manuali WHERE id = ?`, id).Scan(&n)
	if n == 0 {
		return nil, errNonTrovato
	}
	if r.ContentLength > maxOCR {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande: massimo 64 MB.")
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxOCR))
	if err != nil {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande o interrotto: massimo 64 MB.")
	}
	f, righe, err := compattaOCR(raw)
	if err != nil {
		return nil, err
	}
	f.Caricato = db.Ora()
	b, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(s.percorsoOCR(id)), 0o700); err != nil {
		return nil, err
	}
	if err := scriviAtomico(s.percorsoOCR(id), b); err != nil {
		return nil, err
	}
	s.avvisaManuali()
	return map[string]any{"pagine": len(f.Pagine), "righe": righe}, nil
}

func (s *Server) apiTogliOCR(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	if err := os.Remove(s.percorsoOCR(r.PathValue("id"))); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	s.avvisaManuali()
	return nil, nil
}

// fileOCR: il testo compatto, solo al Master (lo usano l'importazione e il bestiario). 404 se non c'è.
// Senza testo OCR (o per chi non è il Master) risponde 204, «niente»: un 404 finirebbe come errore nella console del
// browser a ogni apertura di un manuale (2026-11-12).
func (s *Server) fileOCR(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	p := s.percorsoOCR(r.PathValue("id"))
	if u == nil || !u.Master() || !fileEsiste(p) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeFile(w, r, p)
}

// infoOCR per l'elenco dei manuali (solo il Master): pagine e data del testo OCR caricato, o nil.
func (s *Server) infoOCR(id string) *struct {
	Pagine   int   `json:"pagine"`
	Caricato int64 `json:"caricato"`
} {
	b, err := os.ReadFile(s.percorsoOCR(id))
	if err != nil {
		return nil
	}
	var f struct {
		Caricato int64             `json:"caricato"`
		Pagine   []json.RawMessage `json:"pagine"`
	}
	if json.Unmarshal(b, &f) != nil {
		return nil
	}
	return &struct {
		Pagine   int   `json:"pagine"`
		Caricato int64 `json:"caricato"`
	}{len(f.Pagine), f.Caricato}
}
