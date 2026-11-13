package server

// Musica di sottofondo e canti (Viandante, bardico) per il tavolo, come nella mappa originale.
//
// - La musica esce dal PC del Master (dalla sua finestra della mappa o dalla finestra di proiezione):
//   sui telefoni dei giocatori non suona nulla. Lo stato (brano, in pausa, combattimento) è uno solo per
//   tutto il tavolo e vive in memoria; le impostazioni (volume, ordine, uscita, brani di combattimento,
//   musica delle località, blocco dei canti) sono salvate nel database (impostazione "musica").
// - I brani stanno in GasterBlessDati\Musica (sottocartella Combattimento per gli scontri, Canti\Viandante e
//   Canti\Bardo per i canti). Il file audio lo scarica solo il Master (/musica/...).
// - Un canto alla volta: lo avvia il Master o un giocatore (se il Master non l'ha bloccato); tutti vedono
//   chi l'ha avviato.

import (
	"encoding/json"
	"io"
	"io/fs"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"

	"gasterbless/internal/rete"
)

var estensioniAudio = map[string]bool{".mp3": true, ".ogg": true, ".m4a": true, ".wav": true, ".flac": true, ".opus": true, ".oga": true, ".aac": true}

const maxBrano = 300 << 20

type impostazioniMusica struct {
	Volume float64           `json:"volume"`
	Mode   string            `json:"mode"` // playlist, shuffle, repeat
	Out    string            `json:"out"`  // master, proj
	Fight  []string          `json:"fight"`
	Locs   map[string]string `json:"locs"`
	Lock   map[string]bool   `json:"songLock"`
}

type statoMusica struct {
	File    string `json:"file"`
	Playing bool   `json:"playing"`
	Fight   bool   `json:"fight"`
	prima   string // brano da riprendere alla fine del combattimento
	dopo    string // musica di località da far partire alla fine del combattimento
}

type musica struct {
	mu    sync.Mutex
	cfg   impostazioniMusica
	stato statoMusica
	canto string // "viandante", "bardo" o ""
	video string // video del canto in corso (percorso relativo a Musica), "" se è un brano audio
	letta bool
}

// Video nei canti (dal ramo main-ke8sy4): il Master, invece di un brano a caso, può condividere un video
// della cartella del canto. Il video si apre su tutti gli schermi del tavolo con il volume a zero; l'audio
// esce solo dove suona la musica del tavolo. Il file si scarica solo mentre il canto è in corso.
var estensioniVideoCanti = map[string]bool{".mp4": true, ".webm": true, ".m4v": true}

func (s *Server) videoCanti(kind string) []string {
	cartella := map[string]string{"viandante": "Viandante", "bardo": "Bardo"}[kind]
	if cartella == "" {
		return nil
	}
	var out []string
	voci, _ := os.ReadDir(filepath.Join(s.cartellaMusica(), "Canti", cartella))
	for _, v := range voci {
		if !v.IsDir() && estensioniVideoCanti[strings.ToLower(filepath.Ext(v.Name()))] {
			out = append(out, "Canti/"+cartella+"/"+v.Name())
		}
	}
	return out
}

// cantoVideoFile serve il video del canto in corso a chi partecipa alla mappa (Range compreso).
func (s *Server) cantoVideoFile(w http.ResponseWriter, r *http.Request) {
	if _, _, _, err := s.contestoMappa(r, ""); err != nil {
		http.Error(w, "Non disponibile.", http.StatusNotFound)
		return
	}
	m := s.mus()
	m.mu.Lock()
	f := m.video
	m.mu.Unlock()
	if f == "" {
		http.Error(w, "Nessun video in corso.", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	u := utenteDa(r.Context())
	s.serviFileMedia(w, r, filepath.Join(s.cartellaMusica(), filepath.FromSlash(f)), u == nil || !u.Master())
}

func (s *Server) rotteMusica(mux *http.ServeMux) {
	m := func(metodo, p string, h gestore) { mux.Handle(metodo+" /api/v1/mappa/"+p, api(h)) }
	m("GET", "music", s.musicaStato)
	m("POST", "music/cmd", s.musicaComando)
	m("POST", "music/folder", s.musicaApriCartella)
	m("POST", "music/upload", s.musicaCarica)
	m("GET", "song", s.cantiStato)
	mux.Handle("GET /api/v1/mappa/songvideo", http.HandlerFunc(s.cantoVideoFile))
	m("POST", "song", s.cantiComando)
	mux.Handle("GET /musica/{percorso...}", http.HandlerFunc(s.musicaFile))
}

func (s *Server) cartellaMusica() string { return filepath.Join(s.Cfg.CartellaMedia, "Musica") }

// mus restituisce lo stato della musica, leggendo le impostazioni salvate la prima volta.
func (s *Server) mus() *musica {
	s.muMusica.Do(func() { s.musica = &musica{} })
	m := s.musica
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.letta {
		m.cfg = impostazioniMusica{Volume: 0.6, Mode: "playlist", Out: "master", Fight: []string{}, Locs: map[string]string{}, Lock: map[string]bool{}}
		if v, _ := s.impostazione("musica"); v != "" {
			json.Unmarshal([]byte(v), &m.cfg)
		}
		if m.cfg.Locs == nil {
			m.cfg.Locs = map[string]string{}
		}
		if m.cfg.Lock == nil {
			m.cfg.Lock = map[string]bool{}
		}
		if m.cfg.Fight == nil {
			m.cfg.Fight = []string{}
		}
		m.letta = true
	}
	return m
}

func (s *Server) salvaMusica(m *musica) {
	b, _ := json.Marshal(m.cfg)
	s.impostaImpostazione("musica", string(b))
}

// brani: tutti i file audio della cartella Musica, tranne i canti (percorsi relativi con "/").
func (s *Server) brani() []string {
	var out []string
	radice := s.cartellaMusica()
	filepath.WalkDir(radice, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if rel, _ := filepath.Rel(radice, p); strings.EqualFold(filepath.ToSlash(rel), "Canti") {
				return filepath.SkipDir
			}
			return nil
		}
		if estensioniAudio[strings.ToLower(filepath.Ext(p))] {
			rel, _ := filepath.Rel(radice, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// braniCombattimento: quelli scelti dal Master oppure, se nessuno, quelli della cartella Combattimento.
func (s *Server) braniCombattimento(m *musica, tutti []string) []string {
	esiste := map[string]bool{}
	for _, f := range tutti {
		esiste[f] = true
	}
	var out []string
	for _, f := range m.cfg.Fight {
		if esiste[f] {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		for _, f := range tutti {
			if strings.HasPrefix(strings.ToLower(f), "combattimento/") {
				out = append(out, f)
			}
		}
	}
	return out
}

func (s *Server) vistaMusica(m *musica, r *http.Request) map[string]any {
	tutti := s.brani()
	dir := s.cartellaMusica()
	return map[string]any{"files": tutti, "fight": s.braniCombattimento(m, tutti), "locs": m.cfg.Locs,
		"cfg":   map[string]any{"volume": m.cfg.Volume, "mode": m.cfg.Mode, "out": m.cfg.Out, "fight": m.cfg.Fight},
		"state": m.stato, "dir": dir, "local": rete.Locale(r)}
}

func (s *Server) avvisaMusica(m *musica) {
	s.avvisaTutti(map[string]any{"t": "music", "s": m.stato,
		"c": map[string]any{"volume": m.cfg.Volume, "mode": m.cfg.Mode, "out": m.cfg.Out, "fight": m.cfg.Fight}})
}

// avvisaTutti invia un evento della mappa a tutti i gruppi (musica e canti valgono per tutto il tavolo).
func (s *Server) avvisaTutti(ev map[string]any) {
	rows, err := s.DB.Query(`SELECT id FROM gruppi WHERE archiviato = 0`)
	if err != nil {
		return
	}
	var gg []string
	for rows.Next() {
		var g string
		rows.Scan(&g)
		gg = append(gg, g)
	}
	rows.Close()
	for _, g := range gg {
		s.avvisaMappa(g, ev)
	}
}

func (s *Server) musicaStato(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, ""); err != nil {
		return nil, err
	}
	m := s.mus()
	m.mu.Lock()
	defer m.mu.Unlock()
	return s.vistaMusica(m, r), nil
}

// musicaComando: play, pause, resume, next, prev, stop, ended, volume, mode, out, fight, setfight, loc, setloc.
func (s *Server) musicaComando(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Op    string   `json:"op"`
		File  string   `json:"file"`
		Value float64  `json:"value"`
		Text  string   `json:"text"`
		On    bool     `json:"on"`
		Files []string `json:"files"`
		K     string   `json:"k"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	m := s.mus()
	m.mu.Lock()
	defer m.mu.Unlock()
	tutti := s.brani()
	esiste := func(f string) bool {
		for _, x := range tutti {
			if x == f {
				return true
			}
		}
		return false
	}
	elenco := func() []string { // brani tra cui scegliere il prossimo
		if m.stato.Fight {
			if l := s.braniCombattimento(m, tutti); len(l) > 0 {
				return l
			}
		}
		var l []string
		for _, f := range tutti {
			if !strings.HasPrefix(strings.ToLower(f), "combattimento/") {
				l = append(l, f)
			}
		}
		if len(l) == 0 {
			return tutti
		}
		return l
	}
	passo := func(dir int) {
		l := elenco()
		if len(l) == 0 {
			m.stato.File, m.stato.Playing = "", false
			return
		}
		if m.cfg.Mode == "shuffle" && len(l) > 1 {
			for {
				f := l[rand.IntN(len(l))]
				if f != m.stato.File {
					m.stato.File = f
					break
				}
			}
		} else if m.cfg.Mode != "repeat" || m.stato.File == "" {
			i := -1
			for k, f := range l {
				if f == m.stato.File {
					i = k
				}
			}
			m.stato.File = l[((i+dir)%len(l)+len(l))%len(l)]
		}
		m.stato.Playing = true
	}
	salva := false
	switch in.Op {
	case "play":
		if !esiste(in.File) {
			return nil, errNonTrovato
		}
		m.stato.File, m.stato.Playing = in.File, true
	case "pause":
		m.stato.Playing = false
	case "resume":
		if m.stato.File == "" {
			passo(1)
		} else {
			m.stato.Playing = true
		}
	case "next":
		passo(1)
	case "prev":
		passo(-1)
	case "stop":
		m.stato.File, m.stato.Playing = "", false
	case "ended":
		if in.File == m.stato.File && m.stato.Playing {
			passo(1)
		}
	case "volume":
		if in.Value < 0 || in.Value > 1 {
			return nil, errDatiNonValidi
		}
		m.cfg.Volume, salva = in.Value, true
	case "mode":
		if in.Text != "playlist" && in.Text != "shuffle" && in.Text != "repeat" {
			return nil, errDatiNonValidi
		}
		m.cfg.Mode, salva = in.Text, true
	case "out":
		if in.Text != "master" && in.Text != "proj" {
			return nil, errDatiNonValidi
		}
		m.cfg.Out, salva = in.Text, true
	case "fight":
		s.modoCombattimento(m, in.On, tutti)
	case "setfight":
		var l []string
		for _, f := range in.Files {
			if esiste(f) {
				l = append(l, f)
			}
		}
		if l == nil {
			l = []string{}
		}
		m.cfg.Fight, salva = l, true
	case "loc": // aperto o proiettato un video della località: parte la sua musica
		f := m.cfg.Locs[in.K]
		if f == "" || !esiste(f) {
			return nil, nil
		}
		if m.stato.Fight {
			m.stato.dopo = f
		} else {
			m.stato.File, m.stato.Playing = f, true
		}
	case "setloc":
		if s.cercaLocalita(in.K) == nil {
			return nil, errNonTrovato
		}
		if in.File == "" {
			delete(m.cfg.Locs, in.K)
		} else if !esiste(in.File) {
			return nil, errNonTrovato
		} else {
			m.cfg.Locs[in.K] = in.File
		}
		salva = true
	default:
		return nil, errDatiNonValidi
	}
	if salva {
		s.salvaMusica(m)
	}
	s.avvisaMusica(m)
	return s.vistaMusica(m, r), nil
}

// modoCombattimento accende o spegne la musica dello scontro (chi chiama tiene m.mu). Accesa: si ricorda il brano di prima
// e parte un brano di combattimento; spenta: la musica della località in attesa, oppure il brano di prima.
func (s *Server) modoCombattimento(m *musica, on bool, tutti []string) {
	if on == m.stato.Fight {
		return
	}
	if on {
		m.stato.prima = m.stato.File
		m.stato.Fight = true
		if l := s.braniCombattimento(m, tutti); len(l) > 0 {
			m.stato.File, m.stato.Playing = l[rand.IntN(len(l))], true
		}
		return
	}
	m.stato.Fight = false
	if m.stato.dopo != "" {
		m.stato.File = m.stato.dopo
	} else {
		m.stato.File = m.stato.prima
	}
	m.stato.dopo, m.stato.prima = "", ""
	m.stato.Playing = m.stato.File != ""
}

// musicaScontro: all'inizio dello scontro dal tracker la musica passa da sola al combattimento (senza ripartire se lo è
// già), alla fine torna quella di prima (richiesta dell'utente, sezione 3 della specifica del combattimento). Restituisce
// lo stato, così il tracker del Master può suonarla lui se nessuna finestra della mappa la sta suonando.
func (s *Server) musicaScontro(on bool) statoMusica {
	m := s.mus()
	m.mu.Lock()
	defer m.mu.Unlock()
	if on != m.stato.Fight {
		s.modoCombattimento(m, on, s.brani())
		s.avvisaMusica(m)
	}
	return m.stato
}

// musicaApriCartella apre la cartella della musica in Esplora file (solo sul PC di Gaster Bless).
func (s *Server) musicaApriCartella(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, "master"); err != nil {
		return nil, err
	}
	if !rete.Locale(r) || runtime.GOOS != "windows" {
		return nil, errore(http.StatusBadRequest, "La cartella si apre solo dal PC su cui gira Gaster Bless: "+s.cartellaMusica())
	}
	os.MkdirAll(s.cartellaMusica(), 0o755)
	if err := exec.Command("explorer", s.cartellaMusica()).Start(); err != nil {
		return nil, errore(http.StatusInternalServerError, "Impossibile aprire la cartella.")
	}
	return nil, nil
}

// musicaCarica: un brano caricato dal Master (nella cartella Musica o Musica\Combattimento).
func (s *Server) musicaCarica(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, "master"); err != nil {
		return nil, err
	}
	nome := filepath.Base(strings.ReplaceAll(r.URL.Query().Get("name"), "\\", "/"))
	nome = nomeCartella(strings.TrimSpace(nome))
	if !estensioniAudio[strings.ToLower(filepath.Ext(nome))] || len(nome) > 150 || strings.HasPrefix(nome, ".") {
		return nil, errore(http.StatusUnsupportedMediaType, "Formato non supportato: MP3, OGG, M4A, WAV, FLAC, OPUS.")
	}
	dir := s.cartellaMusica()
	if r.URL.Query().Get("fight") == "1" {
		dir = filepath.Join(dir, "Combattimento")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, nome)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if os.IsExist(err) {
		return nil, errore(http.StatusConflict, "Esiste già un brano con questo nome.")
	}
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(f, io.LimitReader(r.Body, maxBrano+1))
	f.Close()
	if err != nil || n > maxBrano || n == 0 {
		os.Remove(p)
		return nil, errore(http.StatusBadRequest, "Caricamento non riuscito (massimo 300 MB).")
	}
	return nil, nil
}

// musicaFile: i brani li scarica solo il Master (sono suonati dal suo PC o dalla proiezione).
func (s *Server) musicaFile(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	if !u.Master() {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	rel := r.PathValue("percorso")
	pulito := path.Clean("/" + strings.ReplaceAll(rel, "\\", "/"))[1:]
	if pulito == "" || pulito != rel || strings.Contains(pulito, "..") || !estensioniAudio[strings.ToLower(path.Ext(pulito))] {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, filepath.Join(s.cartellaMusica(), filepath.FromSlash(pulito)))
}

// ===== Canti =====

func (s *Server) cantiStato(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, ""); err != nil {
		return nil, err
	}
	m := s.mus()
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]any{"song": m.canto, "songLock": m.cfg.Lock, "video": m.video != ""}
	if u := utenteDa(r.Context()); u != nil && u.Master() {
		out["videos"] = map[string][]string{"viandante": s.videoCanti("viandante"), "bardo": s.videoCanti("bardo")}
	}
	return out, nil
}

func (s *Server) cantiComando(w http.ResponseWriter, r *http.Request) (any, error) {
	u, _, _, err := s.contestoMappa(r, "")
	if err != nil {
		return nil, err
	}
	var in struct {
		Op    string `json:"op"`
		Kind  string `json:"kind"`
		On    bool   `json:"on"`
		Video string `json:"video"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	m := s.mus()
	m.mu.Lock()
	defer m.mu.Unlock()
	switch in.Op {
	case "lock":
		if !u.Master() {
			return nil, errVietato
		}
		if in.Kind != "viandante" && in.Kind != "bardo" {
			return nil, errDatiNonValidi
		}
		m.cfg.Lock[in.Kind] = in.On
		s.salvaMusica(m)
		s.avvisaTutti(map[string]any{"t": "songs", "lock": m.cfg.Lock})
		return nil, nil
	case "stop":
		if !u.Master() {
			return nil, errVietato
		}
		m.canto, m.video = "", ""
		s.avvisaTutti(map[string]any{"t": "songend"})
		return nil, nil
	case "end": // il canto è finito sul PC del Master
		if !u.Master() {
			return nil, errVietato
		}
		m.canto, m.video = "", ""
		s.avvisaTutti(map[string]any{"t": "songend"})
		return nil, nil
	case "":
	default:
		return nil, errDatiNonValidi
	}
	if in.Kind != "viandante" && in.Kind != "bardo" {
		return nil, errDatiNonValidi
	}
	if m.cfg.Lock[in.Kind] && !u.Master() {
		return nil, errore(http.StatusForbidden, "Il Master ha bloccato questo canto.")
	}
	if m.canto != "" {
		return nil, errore(http.StatusConflict, "C'è già un canto in corso: si canta uno alla volta.")
	}
	nome := map[string]string{"viandante": "Canto Viandante", "bardo": "Canto bardico"}[in.Kind]
	if in.Video != "" { // video scelto dal Master tra quelli della cartella del canto
		if !u.Master() {
			return nil, errVietato
		}
		if !slices.Contains(s.videoCanti(in.Kind), in.Video) {
			return nil, errore(http.StatusBadRequest, "Video non trovato nella cartella del canto.")
		}
		m.canto, m.video = in.Kind, in.Video
		s.avvisaTutti(map[string]any{"t": "song", "kind": in.Kind, "name": nome, "video": true, "by": u.Nome})
		return nil, nil
	}
	cartella := map[string]string{"viandante": "Viandante", "bardo": "Bardo"}[in.Kind]
	var brani []string
	radice := filepath.Join(s.cartellaMusica(), "Canti", cartella)
	voci, _ := os.ReadDir(radice)
	for _, v := range voci {
		if !v.IsDir() && estensioniAudio[strings.ToLower(filepath.Ext(v.Name()))] {
			brani = append(brani, "Canti/"+cartella+"/"+v.Name())
		}
	}
	if len(brani) == 0 {
		return nil, errore(http.StatusConflict, "Nessun brano nella cartella Musica\\Canti\\"+cartella+" del PC del Master.")
	}
	f := brani[rand.IntN(len(brani))]
	m.canto, m.video = in.Kind, ""
	s.avvisaTutti(map[string]any{"t": "song", "kind": in.Kind, "name": nome, "file": f, "by": u.Nome})
	return nil, nil
}
