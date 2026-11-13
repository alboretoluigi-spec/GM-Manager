package server

// RPG RULEBOOK LIBRARY, passo L1 (docs/RULEBOOK_LIBRARY.md, 13/11/2026): catalogo dei giochi di ruolo e dei documenti
// gratuiti pubblicati dagli editori. Il catalogo contiene solo RIFERIMENTI (titolo, editore, licenza, indirizzo della
// fonte ufficiale, data dell'ultima verifica): nessun documento è incluso nell'app. È un file di dati separato dal codice
// (catalogo_regolamenti.json, incorporato); il Master può sostituirlo con una versione più recente mettendo
// Dati\regolamenti\catalogo.json (se non è valido si usa quello incorporato e lo si scrive nel registro).
// Il download diretto (passo L2) vale solo per le voci con downloadDiretto = true. Solo il Master.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed catalogo_regolamenti.json
var catalogoIncorporato []byte

type docCatalogo struct {
	ID              string `json:"id"`
	Titolo          string `json:"titolo"`
	Edizione        string `json:"edizione"`
	Editore         string `json:"editore,omitempty"` // se diverso da quello del gioco
	Tipologia       string `json:"tipologia"`
	Lingua          string `json:"lingua"`
	Formato         string `json:"formato"`
	URL             string `json:"url"`
	Licenza         string `json:"licenza"`
	Condizioni      string `json:"condizioni"`
	Gratuito        bool   `json:"gratuito"`
	DownloadDiretto bool   `json:"downloadDiretto"`
	VerificatoIl    string `json:"verificatoIl"` // AAAA-MM-GG; vuoto = indirizzo da verificare
}

type giocoCatalogo struct {
	ID               string        `json:"id"`
	Nome             string        `json:"nome"`
	Editore          string        `json:"editore"`
	Generi           []string      `json:"generi"`
	Edizioni         []string      `json:"edizioni"`
	Sito             string        `json:"sito"`
	SitoVerificatoIl string        `json:"sitoVerificatoIl"`
	Documenti        []docCatalogo `json:"documenti"`
}

type catalogoRegolamenti struct {
	Versione     int               `json:"versione"`
	AggiornatoIl string            `json:"aggiornatoIl"`
	Avviso       string            `json:"avviso"`
	Tipologie    map[string]string `json:"tipologie"`
	Giochi       []giocoCatalogo   `json:"giochi"`
}

func (s *Server) rotteRegolamenti(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/regolamenti/catalogo", api(s.apiCatalogoRegolamenti))
}

// leggiCatalogo legge e controlla un catalogo: id unici, tipologie note, indirizzi https, date valide.
func leggiCatalogo(b []byte) (*catalogoRegolamenti, error) {
	var c catalogoRegolamenti
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("catalogo non leggibile: %w", err)
	}
	if c.Versione != 1 || len(c.Giochi) == 0 || len(c.Tipologie) == 0 {
		return nil, fmt.Errorf("catalogo senza versione 1, giochi o tipologie")
	}
	visti := map[string]bool{}
	data := func(d string) bool { _, err := time.Parse("2006-01-02", d); return d == "" || err == nil }
	for _, g := range c.Giochi {
		if g.ID == "" || g.Nome == "" || visti["g:"+g.ID] {
			return nil, fmt.Errorf("gioco senza id o nome, o id doppio: %q", g.ID)
		}
		visti["g:"+g.ID] = true
		if !httpsValido(g.Sito) || !data(g.SitoVerificatoIl) {
			return nil, fmt.Errorf("%s: sito o data di verifica non validi", g.ID)
		}
		for _, d := range g.Documenti {
			if d.ID == "" || d.Titolo == "" || visti["d:"+d.ID] {
				return nil, fmt.Errorf("%s: documento senza id o titolo, o id doppio: %q", g.ID, d.ID)
			}
			visti["d:"+d.ID] = true
			if _, ok := c.Tipologie[d.Tipologia]; !ok {
				return nil, fmt.Errorf("%s: tipologia sconosciuta %q", d.ID, d.Tipologia)
			}
			if !httpsValido(d.URL) || !data(d.VerificatoIl) || d.Lingua == "" || d.Formato == "" || d.Licenza == "" {
				return nil, fmt.Errorf("%s: indirizzo, data, lingua, formato o licenza non validi", d.ID)
			}
			if d.DownloadDiretto && (!d.Gratuito || d.VerificatoIl == "") {
				return nil, fmt.Errorf("%s: il download diretto vale solo per documenti gratuiti e verificati", d.ID)
			}
		}
	}
	return &c, nil
}

func httpsValido(u string) bool {
	p, err := url.Parse(u)
	return err == nil && p.Scheme == "https" && p.Host != "" && p.User == nil
}

// catalogoRegolamenti: la copia locale aggiornata (se valida) oppure quella incorporata.
func (s *Server) catalogoRegolamenti() *catalogoRegolamenti {
	if b, err := os.ReadFile(filepath.Join(s.Cfg.CartellaDati, "regolamenti", "catalogo.json")); err == nil {
		if c, err := leggiCatalogo(b); err == nil {
			return c
		} else {
			slog.Warn("catalogo dei regolamenti locale non valido: uso quello incorporato", "errore", err)
		}
	}
	c, err := leggiCatalogo(catalogoIncorporato)
	if err != nil { // controllato dalle prove: non succede
		slog.Error("catalogo dei regolamenti incorporato non valido", "errore", err)
		return &catalogoRegolamenti{Versione: 1, Tipologie: map[string]string{}}
	}
	return c
}

func contieneVoce(l []string, v string) bool {
	for _, x := range l {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}

// apiCatalogoRegolamenti: catalogo filtrato. Filtri (tutti facoltativi): gioco (parte del nome), genere, editore,
// lingua, tipologia, edizione, gratuito=1. Un gioco resta se rispetta i filtri sul gioco e, quando ci sono filtri sui
// documenti, ha almeno un documento che li rispetta. In «filtri» le scelte possibili, per i menu della pagina.
func (s *Server) apiCatalogoRegolamenti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	c := s.catalogoRegolamenti()
	q := r.URL.Query()
	nome := strings.ToLower(strings.TrimSpace(q.Get("gioco")))
	genere, editore, lingua := q.Get("genere"), q.Get("editore"), q.Get("lingua")
	tipologia, edizione, gratuito := q.Get("tipologia"), q.Get("edizione"), q.Get("gratuito") == "1"
	filtriDoc := lingua != "" || tipologia != "" || gratuito
	insiemi := map[string]map[string]bool{"generi": {}, "editori": {}, "lingue": {}, "tipologie": {}, "edizioni": {}}
	out := []giocoCatalogo{}
	for _, g := range c.Giochi {
		insiemi["editori"][g.Editore] = true
		for _, x := range g.Generi {
			insiemi["generi"][x] = true
		}
		for _, x := range g.Edizioni {
			insiemi["edizioni"][x] = true
		}
		for _, d := range g.Documenti {
			insiemi["lingue"][d.Lingua] = true
			insiemi["tipologie"][d.Tipologia] = true
			if d.Editore != "" {
				insiemi["editori"][d.Editore] = true
			}
		}
		if nome != "" && !strings.Contains(strings.ToLower(g.Nome), nome) {
			continue
		}
		if genere != "" && !contieneVoce(g.Generi, genere) {
			continue
		}
		if edizione != "" && !contieneVoce(g.Edizioni, edizione) {
			continue
		}
		docs := []docCatalogo{}
		for _, d := range g.Documenti {
			ed := d.Editore
			if ed == "" {
				ed = g.Editore
			}
			if (lingua != "" && !strings.EqualFold(d.Lingua, lingua)) || (tipologia != "" && d.Tipologia != tipologia) ||
				(gratuito && !d.Gratuito) || (edizione != "" && d.Edizione != "" && !strings.EqualFold(d.Edizione, edizione)) {
				continue
			}
			if editore != "" && !strings.EqualFold(ed, editore) && !strings.EqualFold(g.Editore, editore) {
				continue
			}
			docs = append(docs, d)
		}
		if editore != "" && !strings.EqualFold(g.Editore, editore) && len(docs) == 0 {
			continue
		}
		if filtriDoc && len(docs) == 0 {
			continue
		}
		g.Documenti = docs
		out = append(out, g)
	}
	sort.SliceStable(out, func(a, b int) bool { return strings.ToLower(out[a].Nome) < strings.ToLower(out[b].Nome) })
	filtri := map[string][]string{}
	for k, m := range insiemi {
		l := make([]string, 0, len(m))
		for v := range m {
			l = append(l, v)
		}
		sort.Strings(l)
		filtri[k] = l
	}
	return map[string]any{"versione": c.Versione, "aggiornatoIl": c.AggiornatoIl, "avviso": c.Avviso, "tipologie": c.Tipologie,
		"giochi": out, "filtri": filtri, "totale": len(c.Giochi)}, nil
}
