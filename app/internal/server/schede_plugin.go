package server

// Schede alternative (plugin): schede del personaggio fatte da altri, che si usano al posto di quella di
// Gaster Bless per i personaggi che le scelgono. Ognuna è una cartella in Media\SchedePlugin\<id>\ con un
// manifest.json e i suoi file statici; parla con Gaster Bless con lo stesso protocollo postMessage della scheda
// di Gaster Bless (vedi kit-scheda\PROTOCOLLO.md).
//
// Sicurezza: le pagine dei plugin si servono con una CSP "sandbox" (origine opaca): non leggono i cookie,
// non possono chiamare le API di Gaster Bless né collegarsi a Internet (fetch solo verso la propria cartella),
// non possono
// aprire finestre né navigare la pagina principale. Ricevono solo i dati del personaggio aperto.
// Il Master le abilita una per una.

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"gasterbless/internal/db"
)

const versioneProtocolloScheda = 1

var (
	idPluginValido = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	// Funzioni facoltative del protocollo che un plugin dichiara di supportare.
	funzioniPlugin = map[string]bool{"pf": true, "tiri": true, "incantesimi": true, "background": true, "sanita": true, "tema": true, "stampa": true}
	tipiPlugin     = map[string]string{"html": "text/html; charset=utf-8", "js": "text/javascript; charset=utf-8", "mjs": "text/javascript; charset=utf-8",
		"css": "text/css; charset=utf-8", "json": "application/json", "png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg",
		"gif": "image/gif", "webp": "image/webp", "svg": "image/svg+xml", "ico": "image/x-icon", "woff2": "font/woff2", "woff": "font/woff",
		"ttf": "font/ttf", "otf": "font/otf", "txt": "text/plain; charset=utf-8", "map": "application/json", "wasm": "application/wasm"}
)

// cspPlugin: la pagina gira in una sandbox senza accesso ai dati di Gaster Bless. Con fetch può leggere solo i
// file della propria cartella (per esempio i suoi dati in .json); niente Internet, niente API di Gaster Bless.
func cspPlugin(host, id string) string {
	cartella := "https://" + host + "/plugin-schede/" + id + "/"
	return "sandbox allow-scripts allow-modals allow-forms allow-downloads; default-src 'self' data: blob:; " +
		"script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; " +
		"connect-src " + cartella + "; frame-src 'none'; worker-src 'none'; object-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'self'"
}

type manifestPlugin struct {
	Nome        string   `json:"nome"`
	Autore      string   `json:"autore"`
	Versione    string   `json:"versione"`
	Descrizione string   `json:"descrizione"`
	Protocollo  int      `json:"protocollo"`
	Ingresso    string   `json:"ingresso"`
	Funzioni    []string `json:"funzioni"`
}

type vistaPlugin struct {
	ID          string   `json:"id"`
	Nome        string   `json:"nome"`
	Autore      string   `json:"autore,omitempty"`
	Versione    string   `json:"versione,omitempty"`
	Descrizione string   `json:"descrizione,omitempty"`
	Funzioni    []string `json:"funzioni"`
	URL         string   `json:"url,omitempty"` // pagina da aprire nel riquadro
	Abilitato   bool     `json:"abilitato"`
	Errore      string   `json:"errore,omitempty"`     // solo per il Master: perché non si può usare
	Personaggi  int      `json:"personaggi,omitempty"` // solo per il Master: quanti la usano
}

func (s *Server) cartellaPlugin() string { return filepath.Join(s.Cfg.CartellaMedia, "SchedePlugin") }

// leggiPlugin legge e controlla il manifest di una cartella.
func (s *Server) leggiPlugin(id string) vistaPlugin {
	v := vistaPlugin{ID: id, Nome: id, Funzioni: []string{}}
	if !idPluginValido.MatchString(id) {
		v.Errore = "nome della cartella non valido: solo lettere minuscole, cifre e trattini (massimo 40)"
		return v
	}
	root, err := os.OpenRoot(filepath.Join(s.cartellaPlugin(), id))
	if err != nil {
		v.Errore = "cartella non leggibile"
		return v
	}
	defer root.Close()
	b, err := fs.ReadFile(root.FS(), "manifest.json")
	if err != nil {
		v.Errore = "manca manifest.json"
		return v
	}
	var m manifestPlugin
	if err := json.Unmarshal(b, &m); err != nil {
		v.Errore = "manifest.json non valido: " + err.Error()
		return v
	}
	if m.Nome = strings.TrimSpace(m.Nome); m.Nome == "" || len([]rune(m.Nome)) > 60 {
		v.Errore = "manifest.json: \"nome\" mancante o troppo lungo"
		return v
	}
	v.Nome, v.Autore, v.Versione, v.Descrizione = m.Nome, limita(m.Autore, 60), limita(m.Versione, 20), limita(m.Descrizione, 300)
	if m.Protocollo != versioneProtocolloScheda {
		v.Errore = "manifest.json: \"protocollo\" deve essere 1"
		return v
	}
	if m.Ingresso == "" {
		m.Ingresso = "index.html"
	}
	if !relativoSicuro(m.Ingresso) || estensione(m.Ingresso) != "html" {
		v.Errore = "manifest.json: \"ingresso\" deve essere una pagina .html dentro la cartella"
		return v
	}
	if st, err := root.Stat(filepath.FromSlash(m.Ingresso)); err != nil || !st.Mode().IsRegular() {
		v.Errore = "manca la pagina " + m.Ingresso
		return v
	}
	for _, f := range m.Funzioni {
		if funzioniPlugin[f] {
			v.Funzioni = append(v.Funzioni, f)
		}
	}
	v.URL = "/plugin-schede/" + id + "/" + m.Ingresso
	return v
}

func (s *Server) pluginAbilitati() map[string]bool {
	m := map[string]bool{}
	if v, _ := s.impostazione("schede_plugin_abilitati"); v != "" {
		json.Unmarshal([]byte(v), &m)
	}
	return m
}

// elencoPlugin: le cartelle presenti in Media\SchedePlugin, in ordine di nome.
func (s *Server) elencoPlugin() []vistaPlugin {
	out := []vistaPlugin{}
	voci, _ := os.ReadDir(s.cartellaPlugin())
	ab := s.pluginAbilitati()
	for _, d := range voci {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		v := s.leggiPlugin(d.Name())
		v.Abilitato = ab[v.ID] && v.Errore == ""
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nome < out[j].Nome })
	return out
}

func (s *Server) pluginAbilitato(id string) bool {
	if !s.pluginAbilitati()[id] {
		return false
	}
	return s.leggiPlugin(id).Errore == ""
}

func (s *Server) rotteSchedePlugin(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/schede-plugin", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		u, err := richiedi(r, "approvato")
		if err != nil {
			return nil, err
		}
		tutti := s.elencoPlugin()
		if u.Master() {
			for i := range tutti {
				s.DB.QueryRow(`SELECT COUNT(*) FROM personaggi WHERE scheda = ?`, tutti[i].ID).Scan(&tutti[i].Personaggi)
			}
			return map[string]any{"lista": tutti, "cartella": s.cartellaPlugin()}, nil
		}
		ok := []vistaPlugin{}
		for _, p := range tutti {
			if p.Abilitato {
				ok = append(ok, p)
			}
		}
		return map[string]any{"lista": ok}, nil
	}))
	mux.Handle("PUT /api/v1/schede-plugin/{id}", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		u, err := richiedi(r, "master")
		if err != nil {
			return nil, err
		}
		var in struct{ Abilitato bool }
		if err := leggiJSON(r, &in); err != nil {
			return nil, err
		}
		id := r.PathValue("id")
		if p := s.leggiPlugin(id); in.Abilitato && p.Errore != "" {
			return nil, errore(http.StatusBadRequest, "Non si può abilitare: "+p.Errore+".")
		}
		m := s.pluginAbilitati()
		imposta(m, id, in.Abilitato)
		b, _ := json.Marshal(m)
		if err := s.impostaImpostazione("schede_plugin_abilitati", string(b)); err != nil {
			return nil, err
		}
		stato := "disabilitata"
		if in.Abilitato {
			stato = "abilitata"
		}
		s.evento("sistema", u.ID, "scheda alternativa "+id+" "+stato)
		return nil, nil
	}))
	// Il Master cambia la scheda con cui si apre un personaggio (i dati salvati restano quelli di prima).
	mux.Handle("PUT /api/v1/personaggi/{id}/tipo-scheda", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "master"); err != nil {
			return nil, err
		}
		var in struct{ Scheda string }
		if err := leggiJSON(r, &in); err != nil {
			return nil, err
		}
		if in.Scheda != "" && !s.pluginAbilitato(in.Scheda) {
			return nil, errore(http.StatusBadRequest, "Questa scheda alternativa non è abilitata.")
		}
		res, err := s.DB.Exec(`UPDATE personaggi SET scheda = ?, modificato_il = ? WHERE id = ?`, in.Scheda, db.Ora(), r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, errNonTrovato
		}
		return nil, nil
	}))
	mux.Handle("GET /plugin-schede/{id}/{file...}", http.HandlerFunc(s.filePlugin))
}

// filePlugin serve i file di un plugin abilitato. Senza accesso: sono solo codice statico e, dentro la
// sandbox, il browser non manda comunque i cookie. Il filtro di rete resta quello dell'app.
func (s *Server) filePlugin(w http.ResponseWriter, r *http.Request) {
	id, f := r.PathValue("id"), r.PathValue("file")
	tipo := tipiPlugin[estensione(f)]
	if !idPluginValido.MatchString(id) || !relativoSicuro(f) || tipo == "" || !s.pluginAbilitato(id) {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	root, err := os.OpenRoot(filepath.Join(s.cartellaPlugin(), id))
	if err != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	defer root.Close()
	fh, err := root.Open(filepath.FromSlash(path.Clean(f)))
	if err != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	defer fh.Close()
	st, err := fh.Stat()
	if err != nil || !st.Mode().IsRegular() {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Content-Security-Policy", cspPlugin(r.Host, id))
	// La sandbox ha origine "null": per leggere con fetch i file della propria cartella serve il permesso CORS.
	// Sono solo codice e dati pubblici della scheda, senza cookie (le altre cartelle le blocca la CSP).
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "", st.ModTime(), fh)
}

func estensione(p string) string { return strings.TrimPrefix(strings.ToLower(path.Ext(p)), ".") }

// relativoSicuro: percorso con "/" relativo a una cartella, senza "..", senza unità, barre iniziali o file nascosti.
func relativoSicuro(p string) bool {
	if p == "" || len(p) > 400 || !utf8.ValidString(p) || strings.ContainsAny(p, "\\:\x00") || strings.HasPrefix(p, "/") || path.Clean(p) != p {
		return false
	}
	for _, r := range p {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, parte := range strings.Split(p, "/") {
		if parte == "" || parte == "." || parte == ".." || strings.HasPrefix(parte, ".") {
			return false
		}
	}
	return true
}

func imposta(m map[string]bool, k string, v bool) {
	if v {
		m[k] = true
	} else {
		delete(m, k)
	}
}
