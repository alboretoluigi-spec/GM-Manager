package server

// Biblioteca dell'assistente IA (2026-11-13). Due scaffali separati, uno per ognuna delle due identità dell'Assistente
// (identita_ia.go):
//   - «canone»: i testi ufficiali che il Master possiede (manuale dell'avventura, romanzi, compendi, Tomo interattivo),
//     per l'Archivista del canone;
//   - «campagna»: i documenti della campagna homebrew del Master, per il Cronista della campagna.
// Il Master carica le sue copie (EPUB, DOCX, TXT/MD, HTML, PDF letto nel browser, testo OCR) o collega un manuale già
// caricato in Gaster Bless. Gaster Bless le divide in brani (biblioteca_testi.go) e a ogni domanda passa all'IA solo i brani
// pertinenti, con il riferimento (libro, capitolo o pagina) da citare. I testi stanno in Dati\biblioteca (un file per
// fonte) e non escono dal PC, salvo i brani inviati al servizio di IA scelto dal Master. Solo il Master.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const (
	maxFileBib      = 96 << 20 // un file caricato
	maxCaratteriBib = 12 << 20 // testo di una fonte
	maxFontiBib     = 200
)

var tipiFonteBib = map[string]string{"manuale": "manuale dell'avventura", "romanzo": "romanzo", "compendio": "compendio",
	"tomo": "Tomo interattivo", "regole": "manuale di regole", "appunti": "appunti del Master", "altro": "altro"}

// fonteBib: una fonte della biblioteca (il testo sta a parte, in <id>.json).
type fonteBib struct {
	ID        string `json:"id"`
	Titolo    string `json:"titolo"`
	Tipo      string `json:"tipo"`
	Dominio   string `json:"dominio"` // canone | campagna
	File      string `json:"file,omitempty"`
	Manuale   string `json:"manuale,omitempty"` // manuale di Gaster Bless da cui viene il testo
	Parti     int    `json:"parti"`
	Pezzi     int    `json:"pezzi"`
	Caratteri int    `json:"caratteri"`
	Capitoli  bool   `json:"capitoli"` // false: libro senza capitoli riconosciuti (si cita la posizione)
	Attiva    bool   `json:"attiva"`
	Caricato  int64  `json:"caricato"`
}

// biblioteca: elenco delle fonti e indice di ricerca, costruito alla prima domanda dopo ogni cambiamento.
type biblioteca struct {
	mu    sync.Mutex
	fonti []fonteBib
	letta bool
	ix    *indiceBib
}

type indiceBib struct {
	testi indiceTesti
	fonti []fonteBib     // solo quelle attive, nello stesso ordine dell'indice
	parti [][]parteTesto // senza testo: solo dove (E) e pagina (P)
	pezzi [][]pezzoBib   // per fonte
	lung  []int          // caratteri di ogni fonte
	pos   map[string]int // id della fonte → posizione
}

func (s *Server) cartellaBib() string { return filepath.Join(s.Cfg.CartellaDati, "biblioteca") }

func (s *Server) rotteBiblioteca(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/biblioteca", api(s.apiBiblioteca))
	mux.Handle("POST /api/v1/biblioteca", api(s.apiCaricaBib))
	mux.Handle("POST /api/v1/biblioteca/manuale", api(s.apiCollegaManualeBib))
	mux.Handle("PATCH /api/v1/biblioteca/{id}", api(s.apiModificaBib))
	mux.Handle("DELETE /api/v1/biblioteca/{id}", api(s.apiEliminaBib))
	mux.Handle("GET /api/v1/biblioteca/cerca", api(s.apiCercaBib))
	mux.Handle("GET /api/v1/biblioteca/{id}/brano/{n}", api(s.apiBranoBib))
}

// ===== Elenco delle fonti =====

// fontiBib: copia dell'elenco (lo legge dal disco la prima volta).
func (s *Server) fontiBib() []fonteBib {
	s.bib.mu.Lock()
	defer s.bib.mu.Unlock()
	s.leggiFontiBibLocked()
	return append([]fonteBib(nil), s.bib.fonti...)
}

func (s *Server) leggiFontiBibLocked() {
	if s.bib.letta {
		return
	}
	s.bib.letta = true
	b, err := os.ReadFile(filepath.Join(s.cartellaBib(), "fonti.json"))
	if err == nil {
		json.Unmarshal(b, &s.bib.fonti)
	}
}

func (s *Server) salvaFontiBibLocked() error {
	if err := os.MkdirAll(s.cartellaBib(), 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(s.bib.fonti)
	if err := scriviAtomico(filepath.Join(s.cartellaBib(), "fonti.json"), b); err != nil {
		return err
	}
	s.bib.ix = nil // l'indice si rifà alla prossima ricerca
	return nil
}

func (s *Server) apiBiblioteca(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	f := s.fontiBib()
	sort.SliceStable(f, func(a, b int) bool {
		if f[a].Dominio != f[b].Dominio {
			return f[a].Dominio == "canone"
		}
		return strings.ToLower(f[a].Titolo) < strings.ToLower(f[b].Titolo)
	})
	return map[string]any{"fonti": f, "tipi": tipiFonteBib, "scambio": s.scambioIA()}, nil
}

// ===== Caricamento =====

// datiFonte: titolo, tipo e dominio dalla richiesta (query string per i file, JSON per le pagine e i manuali).
func datiFonte(titolo, tipo, dominio, file string) (fonteBib, error) {
	t, err := testo(titolo, "Titolo", 0, 140)
	if err != nil {
		return fonteBib{}, err
	}
	if t == "" {
		t = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	}
	if t == "" {
		return fonteBib{}, errore(http.StatusBadRequest, "Scrivi il titolo del testo.")
	}
	if _, ok := tipiFonteBib[tipo]; !ok {
		tipo = "altro"
	}
	if dominio != "campagna" {
		dominio = "canone"
	}
	return fonteBib{Titolo: t, Tipo: tipo, Dominio: dominio, File: accorcia(filepath.Base(file), 160)}, nil
}

// apiCaricaBib: un file nel corpo della richiesta (?titolo=&tipo=&dominio=&file=nome.epub), oppure, con ?formato=pagine,
// le pagine di un PDF lette nel browser: {"pagine": ["testo p.1", …]} (anche ?manuale=<id> se vengono da un manuale).
func (s *Server) apiCaricaBib(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	q := r.URL.Query()
	f, err := datiFonte(q.Get("titolo"), q.Get("tipo"), q.Get("dominio"), q.Get("file"))
	if err != nil {
		return nil, err
	}
	if r.ContentLength > maxFileBib {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande: massimo 96 MB.")
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxFileBib))
	if err != nil {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande o caricamento interrotto: massimo 96 MB.")
	}
	var parti []parteTesto
	switch {
	case q.Get("formato") == "pagine":
		var in struct {
			Pagine []string `json:"pagine"`
		}
		if json.Unmarshal(raw, &in) != nil || len(in.Pagine) == 0 || len(in.Pagine) > maxPagine {
			return nil, errDatiNonValidi
		}
		parti = testoDaPagine(in.Pagine)
		if m := q.Get("manuale"); m != "" && s.esisteManuale(m) {
			f.Manuale = m
		}
	case strings.EqualFold(filepath.Ext(f.File), ".json"):
		o, _, e := compattaOCR(raw)
		if e != nil {
			return nil, errore(http.StatusBadRequest, "Il file .json non è un testo OCR di Gaster Bless («gasterbless-ocr»).")
		}
		parti = testoDaOCR(o)
	default:
		var tit string
		parti, tit, err = testoDaFile(f.File, raw)
		if err == errFormatoBib {
			return nil, errore(http.StatusBadRequest, "Formato non riconosciuto: usa EPUB, PDF, DOCX, TXT, MD o HTML (per i PDF scansionati, il testo OCR in .json).")
		} else if err != nil {
			return nil, errore(http.StatusBadRequest, "Non riesco a leggere il file: "+err.Error()+".")
		}
		if q.Get("titolo") == "" && tit != "" {
			f.Titolo = accorcia(tit, 140)
		}
	}
	out, err := s.salvaFonteBib(f, parti)
	if err != nil {
		return nil, err
	}
	s.evento("biblioteca_carica", u.ID, out.Titolo)
	return out, nil
}

// apiCollegaManualeBib: il testo di un manuale già caricato in Gaster Bless (testo OCR se c'è, altrimenti il testo delle
// pagine preparate). Se non c'è nessuno dei due risponde 409: la pagina allora legge il PDF nel browser e lo manda
// con ?formato=pagine&manuale=<id>.
func (s *Server) apiCollegaManualeBib(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Manuale string `json:"manuale"`
		Titolo  string `json:"titolo"`
		Tipo    string `json:"tipo"`
		Dominio string `json:"dominio"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	var nome, file string
	if s.DB.QueryRow(`SELECT nome, file FROM manuali WHERE id = ?`, in.Manuale).Scan(&nome, &file) != nil {
		return nil, errNonTrovato
	}
	if in.Titolo == "" {
		in.Titolo = nome
	}
	f, err := datiFonte(in.Titolo, in.Tipo, in.Dominio, file)
	if err != nil {
		return nil, err
	}
	f.Manuale = in.Manuale
	var parti []parteTesto
	if b, err := os.ReadFile(s.percorsoOCR(in.Manuale)); err == nil {
		var o fileOCRCompatto
		if json.Unmarshal(b, &o) == nil {
			parti = testoDaOCR(o)
		}
	}
	if len(parti) == 0 {
		if pi, ok := s.leggiInfoPagine(in.Manuale); ok && pi.File == s.filePDF(in.Manuale) {
			if b, err := os.ReadFile(filepath.Join(s.cartellaPagine(in.Manuale), "testo.json")); err == nil {
				var t struct {
					Pagine []string `json:"pagine"`
				}
				if json.Unmarshal(b, &t) == nil {
					parti = testoDaPagine(t.Pagine)
				}
			}
		}
	}
	if len(parti) == 0 {
		return nil, errore(http.StatusConflict, "Il manuale non ha ancora il testo pronto: lo leggo dal PDF.")
	}
	out, err := s.salvaFonteBib(f, parti)
	if err != nil {
		return nil, err
	}
	s.evento("biblioteca_carica", u.ID, out.Titolo)
	return out, nil
}

func (s *Server) esisteManuale(id string) bool {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM manuali WHERE id = ?`, id).Scan(&n)
	return n > 0
}

// salvaFonteBib scrive il testo e aggiunge la fonte all'elenco. Lo stesso manuale o lo stesso file con lo stesso
// titolo nello stesso scaffale prendono il posto della versione precedente.
func (s *Server) salvaFonteBib(f fonteBib, parti []parteTesto) (fonteBib, error) {
	car := 0
	capitoli := false
	for _, p := range parti {
		car += utf8.RuneCountInString(p.T)
		if p.E != "" {
			capitoli = true
		}
	}
	if car < 200 {
		return f, errore(http.StatusBadRequest, "Nel file non c'è abbastanza testo leggibile (se è un PDF scansionato serve il testo OCR).")
	}
	if car > maxCaratteriBib {
		return f, errore(http.StatusRequestEntityTooLarge, "Il testo è troppo lungo (oltre 12 milioni di caratteri): dividilo in più file.")
	}
	f.Parti, f.Pezzi, f.Caratteri, f.Capitoli, f.Attiva, f.Caricato = len(parti), len(pezziDi(parti)), car, capitoli, true, db.Ora()
	s.bib.mu.Lock()
	defer s.bib.mu.Unlock()
	s.leggiFontiBibLocked()
	sostituita := -1
	for i, x := range s.bib.fonti {
		if x.Dominio == f.Dominio && ((f.Manuale != "" && x.Manuale == f.Manuale) || (f.Manuale == "" && x.Manuale == "" && x.File == f.File && x.Titolo == f.Titolo)) {
			sostituita = i
			f.ID = x.ID
		}
	}
	if sostituita < 0 && len(s.bib.fonti) >= maxFontiBib {
		return f, errore(http.StatusConflict, "La biblioteca è piena (200 testi): togline qualcuno.")
	}
	if f.ID == "" {
		f.ID = cripto.ID()
	}
	if err := os.MkdirAll(s.cartellaBib(), 0o700); err != nil {
		return f, err
	}
	b, _ := json.Marshal(struct {
		Parti []parteTesto `json:"parti"`
	}{parti})
	if err := scriviAtomico(filepath.Join(s.cartellaBib(), f.ID+".json"), b); err != nil {
		return f, err
	}
	if sostituita >= 0 {
		s.bib.fonti[sostituita] = f
	} else {
		s.bib.fonti = append(s.bib.fonti, f)
	}
	return f, s.salvaFontiBibLocked()
}

func (s *Server) apiModificaBib(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Titolo  *string `json:"titolo"`
		Tipo    *string `json:"tipo"`
		Dominio *string `json:"dominio"`
		Attiva  *bool   `json:"attiva"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	s.bib.mu.Lock()
	defer s.bib.mu.Unlock()
	s.leggiFontiBibLocked()
	for i := range s.bib.fonti {
		f := &s.bib.fonti[i]
		if f.ID != r.PathValue("id") {
			continue
		}
		if in.Titolo != nil {
			t, err := testo(*in.Titolo, "Titolo", 1, 140)
			if err != nil {
				return nil, err
			}
			f.Titolo = t
		}
		if in.Tipo != nil {
			if _, ok := tipiFonteBib[*in.Tipo]; !ok {
				return nil, errDatiNonValidi
			}
			f.Tipo = *in.Tipo
		}
		if in.Dominio != nil {
			if *in.Dominio != "canone" && *in.Dominio != "campagna" {
				return nil, errDatiNonValidi
			}
			f.Dominio = *in.Dominio
		}
		if in.Attiva != nil {
			f.Attiva = *in.Attiva
		}
		out := *f
		return out, s.salvaFontiBibLocked()
	}
	return nil, errNonTrovato
}

func (s *Server) apiEliminaBib(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	s.bib.mu.Lock()
	defer s.bib.mu.Unlock()
	s.leggiFontiBibLocked()
	for i, f := range s.bib.fonti {
		if f.ID == r.PathValue("id") {
			s.bib.fonti = append(s.bib.fonti[:i], s.bib.fonti[i+1:]...)
			if err := s.salvaFontiBibLocked(); err != nil {
				return nil, err
			}
			os.Remove(filepath.Join(s.cartellaBib(), f.ID+".json"))
			s.evento("biblioteca_togli", u.ID, f.Titolo)
			return nil, nil
		}
	}
	return nil, errNonTrovato
}

// ===== Indice =====

// indiceBiblioteca: l'indice delle fonti attive (lo costruisce se manca; per qualche megabyte di testo, un attimo).
func (s *Server) indiceBiblioteca() *indiceBib {
	s.bib.mu.Lock()
	defer s.bib.mu.Unlock()
	s.leggiFontiBibLocked()
	if s.bib.ix != nil {
		return s.bib.ix
	}
	ix := &indiceBib{pos: map[string]int{}}
	for _, f := range s.bib.fonti {
		if !f.Attiva {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.cartellaBib(), f.ID+".json"))
		if err != nil {
			continue
		}
		var t struct {
			Parti []parteTesto `json:"parti"`
		}
		if json.Unmarshal(b, &t) != nil {
			continue
		}
		pz := pezziDi(t.Parti)
		n := len(ix.fonti)
		ix.testi.aggiungi(n, pz)
		lung := 0
		for i := range t.Parti {
			lung += len(t.Parti[i].T)
			t.Parti[i].T = "" // il testo resta solo nei brani
		}
		ix.fonti = append(ix.fonti, f)
		ix.parti = append(ix.parti, t.Parti)
		ix.pezzi = append(ix.pezzi, pz)
		ix.lung = append(ix.lung, max(lung, 1))
		ix.pos[f.ID] = n
	}
	s.bib.ix = ix
	return ix
}

// passoBib: un brano trovato, come lo vedono l'IA (Rif, Dove, Testo) e la pagina (fonti sotto la risposta).
type passoBib struct {
	Rif     string  `json:"rif,omitempty"`
	Fonte   string  `json:"fonte"`
	Pezzo   int     `json:"pezzo"`
	Fino    int     `json:"fino,omitempty"` // ultimo brano unito (brani consecutivi)
	Titolo  string  `json:"titolo"`
	Tipo    string  `json:"tipo"`
	Dominio string  `json:"dominio"`
	Dove    string  `json:"dove"`
	Pagina  int     `json:"pagina,omitempty"`
	Manuale string  `json:"manuale,omitempty"`
	Testo   string  `json:"testo,omitempty"`
	Punti   float64 `json:"punti,omitempty"`
}

// dove: «cap. 3 · Il ritorno» / «p. 45 · Capitolo 4» / «circa al 35%» (libro senza capitoli).
func (ix *indiceBib) dove(fonte, pezzo int) (string, int) {
	pz := ix.pezzi[fonte][pezzo]
	p := ix.parti[fonte][pz.Parte]
	if p.E != "" {
		return p.E, p.P
	}
	return fmt.Sprintf("circa al %d%% del testo", min(100, pz.Inizio*100/ix.lung[fonte])), p.P
}

func (ix *indiceBib) passo(fonte, pezzo int) passoBib {
	f := ix.fonti[fonte]
	d, pag := ix.dove(fonte, pezzo)
	return passoBib{Fonte: f.ID, Pezzo: pezzo, Titolo: f.Titolo, Tipo: f.Tipo, Dominio: f.Dominio, Dove: d, Pagina: pag, Manuale: f.Manuale,
		Testo: ix.pezzi[fonte][pezzo].Testo}
}

// paroleVuoteDomanda: verbi e parole tipiche delle domande, che nei testi non aiutano a trovare il brano giusto.
var paroleVuoteDomanda = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`qual quale quali chiama chiamava chiamano chiamato succede succedere succederebbe trova trovano dimmi
	spiegami spiega parlami racconta raccontami descrivi elenca sai sapere vorrei voglio puoi potresti dire dici fa fanno fare fatto
	esattamente secondo libro romanzo manuale testo testi canone campagna`) {
		m[radiceIt(senzaAccenti.Replace(w))] = true
	}
	return m
}()

// terminiBib: le parole della domanda (peso 1), del messaggio precedente (peso basso) e, facoltativi, i nomi
// alternativi dati dal Codice e le parole chiave scritte dall'IA.
func terminiBib(domanda, precedente string, alias []string) []termineQ {
	q := domandaBib(domanda, 1, nil)
	if precedente != "" {
		q = domandaBib(precedente, 0.35, q)
	}
	for _, a := range alias {
		q = aggiunteBib(a, 0.8, q)
	}
	out := q[:0]
	for _, t := range q {
		if !paroleVuoteDomanda[t.w] {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return q // la domanda era fatta solo di quelle parole: meglio cercarle che niente
	}
	return out
}

// cercaBib: i brani migliori di uno scaffale ("" = tutti), già uniti quando sono consecutivi, entro limite caratteri.
// soglia: copertura minima (quanta parte della domanda il brano contiene) per tenere un brano.
func (s *Server) cercaBib(q []termineQ, dominio string, limite int, soglia float64) []passoBib {
	ix := s.indiceBiblioteca()
	if len(ix.fonti) == 0 || len(q) == 0 {
		return nil
	}
	ris := ix.testi.cerca(q, 40, func(f int) bool { return dominio == "" || ix.fonti[f].Dominio == dominio })
	if len(ris) == 0 || ris[0].copertura < soglia {
		return nil
	}
	migliore := ris[0].punti
	type scelto struct {
		f, p  int
		punti float64
	}
	var scelti []scelto
	usati := 0
	presi := map[[2]int]bool{}
	for _, r := range ris {
		if r.punti < migliore*0.22 || r.copertura < soglia*0.6 {
			continue
		}
		pz := ix.testi.pezzi[r.i]
		n := len(pz.testo)
		if usati+n > limite {
			continue
		}
		scelti = append(scelti, scelto{pz.fonte, pz.pezzo, r.punti})
		presi[[2]int{pz.fonte, pz.pezzo}] = true
		usati += n
	}
	// il seguito dei due brani migliori, se c'è posto (spesso la risposta continua nel brano dopo)
	for i := 0; i < len(scelti) && i < 2; i++ {
		x := scelti[i]
		if x.p+1 < len(ix.pezzi[x.f]) && !presi[[2]int{x.f, x.p + 1}] && ix.pezzi[x.f][x.p+1].Parte == ix.pezzi[x.f][x.p].Parte {
			if n := len(ix.pezzi[x.f][x.p+1].Testo); usati+n <= limite {
				scelti = append(scelti, scelto{x.f, x.p + 1, x.punti * 0.9})
				presi[[2]int{x.f, x.p + 1}] = true
				usati += n
			}
		}
	}
	// brani consecutivi della stessa parte diventano un passo solo
	sort.Slice(scelti, func(a, b int) bool {
		if scelti[a].f != scelti[b].f {
			return scelti[a].f < scelti[b].f
		}
		return scelti[a].p < scelti[b].p
	})
	var out []passoBib
	for i := 0; i < len(scelti); {
		x := scelti[i]
		p := ix.passo(x.f, x.p)
		p.Punti = x.punti
		j := i + 1
		for j < len(scelti) && scelti[j].f == x.f && scelti[j].p == scelti[j-1].p+1 && ix.pezzi[x.f][scelti[j].p].Parte == ix.pezzi[x.f][x.p].Parte {
			p.Testo += "\n" + ix.pezzi[x.f][scelti[j].p].Testo
			p.Fino = scelti[j].p
			p.Punti = max(p.Punti, scelti[j].punti)
			j++
		}
		out = append(out, p)
		i = j
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Punti > out[b].Punti })
	return out
}

// testoPassi: i passi per l'IA, numerati con il prefisso (C = canone, H = campagna). Restituisce anche i passi con il
// riferimento assegnato (senza testo), per le fonti sotto la risposta.
func testoPassi(passi []passoBib, prefisso string, primo int) (string, []passoBib) {
	var b strings.Builder
	out := make([]passoBib, 0, len(passi))
	for i, p := range passi {
		p.Rif = prefisso + strconv.Itoa(primo+i)
		fmt.Fprintf(&b, "\n\n[%s] «%s» (%s) — %s\n%s", p.Rif, p.Titolo, tipiFonteBib[p.Tipo], p.Dove, p.Testo)
		p.Testo, p.Punti = "", 0
		out = append(out, p)
	}
	return b.String(), out
}

// apiCercaBib: «🔎 Cerca nei testi» della biblioteca (anche senza IA). ?q=…&dominio=canone|campagna|
func (s *Server) apiCercaBib(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	d := r.URL.Query().Get("q")
	if strings.TrimSpace(d) == "" {
		return []passoBib{}, nil
	}
	dom := r.URL.Query().Get("dominio")
	if dom != "canone" && dom != "campagna" {
		dom = ""
	}
	p := s.cercaBib(terminiBib(accorcia(d, 500), "", nil), dom, 14000, 0)
	if len(p) > 12 {
		p = p[:12]
	}
	if p == nil {
		p = []passoBib{}
	}
	return p, nil
}

// apiBranoBib: un brano (con quelli uniti fino a ?fino=), per leggere la fonte citata in una risposta.
func (s *Server) apiBranoBib(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	ix := s.indiceBiblioteca()
	f, ok := ix.pos[r.PathValue("id")]
	n, err := strconv.Atoi(r.PathValue("n"))
	if !ok || err != nil || n < 0 || n >= len(ix.pezzi[f]) {
		return nil, errore(http.StatusNotFound, "Il brano non c'è più: forse il testo è stato tolto o ricaricato.")
	}
	p := ix.passo(f, n)
	fino, _ := strconv.Atoi(r.URL.Query().Get("fino"))
	for k := n + 1; k <= fino && k < len(ix.pezzi[f]) && k <= n+8; k++ {
		p.Testo += "\n" + ix.pezzi[f][k].Testo
		p.Fino = k
	}
	return p, nil
}
