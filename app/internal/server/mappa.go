package server

// Mappa di Gaster Bless. La pagina originale (web/carta/, copia di 02_mappa_originale) parla con questo
// server con le sue chiamate di sempre (/api/state, /api/group, /api/loc, /api/item...), che qui
// diventano /api/v1/mappa/... con la sessione di gioco indicata (?sessione=...).
//
// Differenze rispetto al server originale (andato perso):
//   - niente password di gruppo né link d'invito: valgono account, gruppi e permessi della nuova app;
//   - lo stato della mappa (nebbia, nomi e media svelati, scala, proiezione) è per GRUPPO, nel database;
//   - per i giocatori il server manda SOLO ciò che è svelato e fuori dalla nebbia (nomi, descrizioni,
//     media): ciò che non devono vedere non arriva proprio sul telefono;
//   - i file multimediali si leggono dalla cartella GasterBlessDati esistente (cartella_media) e un
//     giocatore può aprire solo quelli svelati al suo gruppo o in proiezione;
//   - gli aggiornamenti arrivano con /api/v1/mappa/events (Server-Sent Events, come l'originale).
//
// Bestiario, manuali, musica e canti arrivano nei passi successivi del blocco: per ora la pagina riceve
// risposte "vuote" valide e un messaggio chiaro se prova a modificarli.

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
	"gasterbless/internal/rete"
)

const maxCaricamentoMedia = 2 << 30 // 2 GB (i video delle località possono essere grandi)

func (s *Server) rotteMappa(mux *http.ServeMux) {
	m := func(metodo, p string, h gestore) { mux.Handle(metodo+" /api/v1/mappa/"+p, api(h)) }
	m("GET", "me", s.mappaMe)
	m("GET", "state", s.mappaStato)
	m("POST", "group", s.mappaGruppo)
	m("POST", "party", s.mappaSegnalino)
	m("POST", "strada", s.mappaStrada)
	mux.Handle("GET /api/v1/mappa/immagine", http.HandlerFunc(s.immagineMappa))
	m("POST", "loc", s.mappaLocalita)
	m("POST", "item", s.mappaMedia)
	m("DELETE", "item", s.mappaEliminaMedia)
	m("POST", "upload", s.mappaCarica)
	m("GET", "folders", s.mappaCartelle)
	m("POST", "folders", s.mappaCartelle)
	m("GET", "proj", s.mappaProiezione)
	m("POST", "proj", s.mappaProiezione)
	m("POST", "projcmd", s.mappaComandoProiezione)
	mux.Handle("GET /api/v1/mappa/events", http.HandlerFunc(s.mappaEventi))
	mux.Handle("GET /media/{percorso...}", http.HandlerFunc(s.servimedia))
	// Pacchetto di installazione: arriva con l'ultimo blocco della Fase 3.
	m("GET", "package", func(w http.ResponseWriter, r *http.Request) (any, error) {
		return map[string]any{"running": false, "files": 0, "done": 0, "total": 0}, nil
	})
	m("POST", "package", func(w http.ResponseWriter, r *http.Request) (any, error) {
		return nil, errore(http.StatusNotImplemented, "Il pacchetto di installazione arriva con l'ultimo blocco della Fase 3.")
	})
	s.rotteMusica(mux)
	s.rotteBestiario(mux)
	s.rotteSezioneBestiario(mux) // sezione «PNG e mostri» dell'app
}

// ===== Località (dal file locs.json della mappa) =====

type localita struct {
	K  string  `json:"k"`
	N  string  `json:"n"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	B  bit     `json:"b"`  // ha una mappa locale (img/lm/<k>.webp)
	Mj bit     `json:"mj"` // località principale
}

// bit: in locs.json i sì/no sono scritti come 0 e 1.
type bit bool

func (b *bit) UnmarshalJSON(d []byte) error {
	v := strings.TrimSpace(string(d))
	*b = bit(v == "1" || v == "true")
	return nil
}

var (
	locsUna  sync.Once
	locsDati []localita
)

func (s *Server) localita() []localita {
	locsUna.Do(func() {
		b, err := fs.ReadFile(s.Web, "carta/locs.json")
		if err != nil {
			slog.Warn("mappa: locs.json non trovato", "errore", err)
			return
		}
		var l []localita
		if err := json.Unmarshal(b, &l); err != nil {
			// Il file può essere un oggetto con l'elenco dentro.
			var o struct{ Locs []localita }
			json.Unmarshal(b, &o)
			l = o.Locs
		}
		locsDati = l
	})
	return locsDati
}

func (s *Server) cercaLocalita(k string) *localita {
	for i, l := range s.localita() {
		if l.K == k {
			return &s.localita()[i]
		}
	}
	return nil
}

// ===== Stato del gruppo =====

type statoMappa struct {
	Gruppo   string
	Nebbia   []byte
	Scala    float64
	Nomi     map[string]bool
	Media    map[string]bool
	Nascoste map[string]bool
	Proiez   map[string]any
	Seg      *datiSegnalini // segnalini del gruppo (vedi mappa_segnalino.go)
}

func (s *Server) leggiStatoMappa(gruppo string) (*statoMappa, error) {
	st := &statoMappa{Gruppo: gruppo, Nomi: map[string]bool{}, Media: map[string]bool{}, Nascoste: map[string]bool{}, Proiez: map[string]any{"kind": "none"}, Seg: &datiSegnalini{}}
	var nomi, media, nasc, proj, pos string
	err := s.DB.QueryRow(`SELECT nebbia, scala, nomi_svelati, media_svelati, mappe_nascoste, proiezione, posizione FROM stato_mappa WHERE gruppo_id = ?`, gruppo).
		Scan(&st.Nebbia, &st.Scala, &nomi, &media, &nasc, &proj, &pos)
	if errors.Is(err, sql.ErrNoRows) {
		// Gruppo creato prima della mappa: si crea lo stato vuoto.
		_, err = s.DB.Exec(`INSERT OR IGNORE INTO stato_mappa(gruppo_id, nebbia, modificato_il) VALUES(?,?,?)`, gruppo, nebbiaVuota(), db.Ora())
		st.Nebbia, st.Scala = nebbiaVuota(), 0.25
		return st, err
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(nomi), &st.Nomi)
	json.Unmarshal([]byte(media), &st.Media)
	json.Unmarshal([]byte(nasc), &st.Nascoste)
	json.Unmarshal([]byte(proj), &st.Proiez)
	st.Seg = leggiSegnalini(pos)
	if len(st.Nebbia) != byteNebbia {
		st.Nebbia = nebbiaVuota()
	}
	return st, nil
}

// svelata: la cella della nebbia che contiene il punto (x, y) è scoperta.
func (st *statoMappa) svelata(x, y float64) bool {
	c := min(colonneNebbia-1, max(0, int(x)/cellaNebbia))
	r := min(righeNebbia-1, max(0, int(y)/cellaNebbia))
	i := r*colonneNebbia + c
	return st.Nebbia[i>>3]&(1<<(i&7)) != 0
}

// ruoloMappa è come ruoloIn, ma il Master può aprire la mappa di un gruppo anche senza una sessione
// aperta (per prepararla tra una serata e l'altra) indicando "g:<id del gruppo>" al posto della sessione.
func (s *Server) ruoloMappa(u *Utente, sessione string) (ruolo, gruppo string) {
	if g, ok := strings.CutPrefix(sessione, "g:"); ok {
		var n int
		if u.Master() && s.DB.QueryRow(`SELECT COUNT(*) FROM gruppi WHERE id = ? AND archiviato = 0`, g).Scan(&n) == nil && n > 0 {
			return "master", g
		}
		// Consultazione (2026-11-08): un giocatore del gruppo può guardare la mappa del suo gruppo anche tra una
		// sessione e l'altra (solo ciò che è svelato, senza modificare nulla: vedi contestoMappa).
		if u.Approvato() && slices.Contains(s.gruppiDi(u), g) && s.DB.QueryRow(`SELECT COUNT(*) FROM gruppi WHERE id = ? AND archiviato = 0`, g).Scan(&n) == nil && n > 0 {
			return "giocatore", g
		}
		return "", ""
	}
	return s.ruoloIn(u, sessione)
}

// contestoMappa: ruolo nella sessione e gruppo. Errore se l'utente non partecipa.
func (s *Server) contestoMappa(r *http.Request, livello string) (*Utente, string, string, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, "", "", err
	}
	sessione := r.URL.Query().Get("sessione")
	ruolo, gruppo := s.ruoloMappa(u, sessione)
	if ruolo == "" {
		return nil, "", "", errNonTrovato
	}
	if livello == "master" && ruolo != "master" {
		return nil, "", "", errVietato
	}
	// Fuori dalle sessioni il giocatore consulta soltanto: niente segnalino, viaggi o altre modifiche (i percorsi
	// «Da … a …» si possono calcolare).
	if ruolo != "master" && strings.HasPrefix(sessione, "g:") && r.Method != http.MethodGet && !strings.HasSuffix(r.URL.Path, "/strada") {
		return nil, "", "", errore(http.StatusForbidden, "Fuori dalle sessioni la mappa si può solo consultare.")
	}
	return u, sessione, gruppo, nil
}

func (s *Server) mappaMe(w http.ResponseWriter, r *http.Request) (any, error) {
	u, sessione, gruppo, err := s.contestoMappa(r, "")
	if err != nil {
		return nil, err
	}
	var nomeG, titolo string
	s.DB.QueryRow(`SELECT nome FROM gruppi WHERE id = ?`, gruppo).Scan(&nomeG)
	s.DB.QueryRow(`SELECT titolo FROM sessioni_gioco WHERE id = ?`, sessione).Scan(&titolo)
	if strings.HasPrefix(sessione, "g:") {
		titolo = "preparazione"
		if !u.Master() {
			titolo = "consultazione"
		}
	}
	ruolo := "player"
	if u.Master() {
		ruolo = "master"
	}
	var lan []string
	for _, ip := range rete.IndirizziLocali() {
		lan = append(lan, "https://"+ip.String()+":"+strconv.Itoa(s.Cfg.PortaHTTPS))
	}
	return map[string]any{"role": ruolo, "hosting": map[string]string{"id": gruppo, "name": nomeG + " · " + titolo}, "lan": lan, "local": rete.Locale(r)}, nil
}

type voceMedia struct {
	ID   string `json:"id"`
	K    string `json:"k"`
	File string `json:"file,omitempty"`
	URL  string `json:"url,omitempty"`
	Cap  string `json:"cap"`
}

// contenuti: descrizioni, note e media di tutte le località.
func (s *Server) contenuti() (map[string]map[string]any, error) {
	out := map[string]map[string]any{}
	voce := func(k string) map[string]any {
		if out[k] == nil {
			out[k] = map[string]any{"desc": "", "notes": "", "items": []voceMedia{}}
		}
		return out[k]
	}
	rows, err := s.DB.Query(`SELECT localita, descrizione, note_master FROM localita_contenuti`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k, d, n string
		rows.Scan(&k, &d, &n)
		v := voce(k)
		v["desc"], v["notes"] = d, n
	}
	rows.Close()
	rows, err = s.DB.Query(`SELECT id, localita, tipo, file, url, didascalia FROM media ORDER BY localita, ordine`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m voceMedia
		var loc string
		rows.Scan(&m.ID, &loc, &m.K, &m.File, &m.URL, &m.Cap)
		v := voce(loc)
		v["items"] = append(v["items"].([]voceMedia), m)
	}
	return out, nil
}

func (s *Server) mappaStato(w http.ResponseWriter, r *http.Request) (any, error) {
	u, _, gruppo, err := s.contestoMappa(r, "")
	if err != nil {
		return nil, err
	}
	st, err := s.leggiStatoMappa(gruppo)
	if err != nil {
		return nil, err
	}
	cont, err := s.contenuti()
	if err != nil {
		return nil, err
	}
	nebbia := base64.StdEncoding.EncodeToString(st.Nebbia)
	gruppoSegnalino := s.segnalinoGruppo(r.URL.Query().Get("sessione"), st)
	if u.Master() {
		var nome, giorno string
		s.DB.QueryRow(`SELECT nome, giorno FROM gruppi WHERE id = ?`, gruppo).Scan(&nome, &giorno)
		return map[string]any{
			"content": cont,
			"group":   map[string]string{"id": gruppo, "name": nome, "day": giorno},
			"g": map[string]any{"fog": nebbia, "miHex": st.Scala, "pubNames": st.Nomi, "pubItems": st.Media,
				"hideBuiltin": st.Nascoste, "proj": st.Proiez, "party": gruppoSegnalino},
		}, nil
	}
	// Giocatore: solo le località fuori dalla nebbia, e di queste solo ciò che è svelato.
	proj, _ := st.Proiez["id"].(string)
	projK, _ := st.Proiez["k"].(string)
	locs := map[string]any{}
	for _, l := range s.localita() {
		if !st.svelata(l.X, l.Y) {
			continue
		}
		nome := st.Nomi[l.K]
		c := cont[l.K]
		items := []voceMedia{}
		if c != nil {
			for _, m := range c["items"].([]voceMedia) {
				if st.Media[m.ID] || m.ID == proj {
					items = append(items, m)
				}
			}
		}
		builtin := bool(l.B) && (nome && !st.Nascoste[l.K] || projK == l.K && st.Proiez["kind"] == "builtin")
		if !nome && len(items) == 0 && !builtin {
			continue
		}
		v := map[string]any{"name": nome, "builtin": builtin, "items": items}
		if nome && c != nil {
			v["desc"] = c["desc"]
		}
		locs[l.K] = v
	}
	out := map[string]any{"fog": nebbia, "miHex": st.Scala, "locs": locs, "party": gruppoSegnalino}
	// Proiezione "sui dispositivi dei giocatori": la ricevono anche aprendo la mappa dopo l'avvio.
	if st.Proiez["dove"] == "giocatori" && st.Proiez["kind"] != "none" {
		out["proj"] = st.Proiez
		// Il video o l'immagine proiettati arrivano anche se la località è ancora sotto la nebbia
		// (prima il telefono non trovava l'elemento e il video non partiva).
		if proj != "" {
			for _, c := range cont {
				for _, m := range c["items"].([]voceMedia) {
					if m.ID == proj {
						out["projItem"] = m
					}
				}
			}
		}
	}
	return out, nil
}

// mappaGruppo: nebbia, scala e cosa è svelato (solo Master).
func (s *Server) mappaGruppo(w http.ResponseWriter, r *http.Request) (any, error) {
	_, _, gruppo, err := s.contestoMappa(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Fog *string `json:"fog"`
		// Base: la nebbia da cui il dispositivo è partito. Se c'è, si applicano solo le celle che il
		// dispositivo ha cambiato rispetto alla base: due dispositivi del Master non si cancellano a vicenda.
		Base  *string  `json:"base"`
		MiHex *float64 `json:"miHex"`
		Kind  string   `json:"kind"`
		Key   string   `json:"key"`
		Val   bool     `json:"val"`
		Pos   *struct {
			X, Y float64
		} `json:"pos"`
		Togli bool `json:"togliPos"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	st, err := s.leggiStatoMappa(gruppo)
	if err != nil {
		return nil, err
	}
	switch {
	case in.Togli: // (forma precedente: ora c'è POST /mappa/party)
		st.Seg.Segnalini = nil
		s.salvaSegnalini(gruppo, st.Seg)
	case in.Pos != nil:
		if !dentroMappa(in.Pos.X, in.Pos.Y) {
			return nil, errore(http.StatusBadRequest, "Posizione fuori dalla mappa.")
		}
		if len(st.Seg.Segnalini) == 0 {
			st.Seg.Segnalini = []segnalino{{ID: "g"}}
		}
		st.Seg.Segnalini[0].X, st.Seg.Segnalini[0].Y = float64(int(in.Pos.X)), float64(int(in.Pos.Y))
		s.salvaSegnalini(gruppo, st.Seg)
	case in.Fog != nil:
		b, err := base64.StdEncoding.DecodeString(*in.Fog)
		if err != nil || len(b) > byteNebbia {
			return nil, errore(http.StatusBadRequest, "Nebbia non valida.")
		}
		neb := nebbiaVuota()
		copy(neb, b)
		var base []byte
		if in.Base != nil {
			bb, err := base64.StdEncoding.DecodeString(*in.Base)
			if err != nil || len(bb) > byteNebbia {
				return nil, errore(http.StatusBadRequest, "Nebbia non valida.")
			}
			base = nebbiaVuota()
			copy(base, bb)
		}
		unita, err := s.salvaNebbia(gruppo, neb, base)
		if err != nil {
			return nil, err
		}
		s.avvisaMappa(gruppo, map[string]any{"t": "state"})
		return map[string]any{"fog": base64.StdEncoding.EncodeToString(unita)}, nil
	case in.MiHex != nil:
		v := *in.MiHex
		if v < 0.05 || v > 10 {
			return nil, errore(http.StatusBadRequest, "Scala non valida.")
		}
		s.DB.Exec(`UPDATE stato_mappa SET scala = ?, modificato_il = ? WHERE gruppo_id = ?`, v, db.Ora(), gruppo)
	case in.Kind == "name" || in.Kind == "builtin" || in.Kind == "item":
		col, mp := "nomi_svelati", st.Nomi
		switch in.Kind {
		case "builtin":
			// Le mappe locali sono visibili di norma: si memorizzano quelle NASCOSTE.
			col, mp = "mappe_nascoste", st.Nascoste
			in.Val = !in.Val
		case "item":
			col, mp = "media_svelati", st.Media
		}
		if in.Kind == "item" {
			var n int
			s.DB.QueryRow(`SELECT COUNT(*) FROM media WHERE id = ?`, in.Key).Scan(&n)
			if n == 0 {
				return nil, errNonTrovato
			}
		} else if s.cercaLocalita(in.Key) == nil {
			return nil, errNonTrovato
		}
		if in.Val {
			mp[in.Key] = true
		} else {
			delete(mp, in.Key)
		}
		b, _ := json.Marshal(mp)
		s.DB.Exec(`UPDATE stato_mappa SET `+col+` = ?, modificato_il = ? WHERE gruppo_id = ?`, string(b), db.Ora(), gruppo)
	default:
		return nil, errDatiNonValidi
	}
	s.avvisaMappa(gruppo, map[string]any{"t": "state"})
	return nil, nil
}

// mappaLocalita: descrizione per i giocatori e note del Master (valgono per tutti i gruppi).
func (s *Server) mappaLocalita(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, "master"); err != nil {
		return nil, err
	}
	var in struct{ K, Desc, Notes string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if s.cercaLocalita(in.K) == nil {
		return nil, errNonTrovato
	}
	d, err := testo(in.Desc, "Descrizione", 0, 20000)
	if err != nil {
		return nil, err
	}
	n, err := testo(in.Notes, "Note", 0, 20000)
	if err != nil {
		return nil, err
	}
	s.DB.Exec(`INSERT INTO localita_contenuti(localita, descrizione, note_master) VALUES(?,?,?)
		ON CONFLICT(localita) DO UPDATE SET descrizione = excluded.descrizione, note_master = excluded.note_master`, in.K, d, n)
	s.avvisaTuttiMappa()
	return nil, nil
}

// mappaMedia: aggiunge un link o un file già presente nelle cartelle; cambia didascalia o ordine.
func (s *Server) mappaMedia(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		K, URL, File, ID string
		Cap              *string
		Move             int
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	switch {
	case in.ID != "" && in.Cap != nil:
		c, err := rigaSingola(*in.Cap, "Didascalia", 0, 200)
		if err != nil {
			return nil, err
		}
		s.DB.Exec(`UPDATE media SET didascalia = ? WHERE id = ?`, c, in.ID)
	case in.ID != "" && in.Move != 0:
		var loc string
		var ord int
		if s.DB.QueryRow(`SELECT localita, ordine FROM media WHERE id = ?`, in.ID).Scan(&loc, &ord) != nil {
			return nil, errNonTrovato
		}
		// Scambia con il vicino nella direzione richiesta.
		q := `SELECT id, ordine FROM media WHERE localita = ? AND ordine < ? ORDER BY ordine DESC LIMIT 1`
		if in.Move > 0 {
			q = `SELECT id, ordine FROM media WHERE localita = ? AND ordine > ? ORDER BY ordine ASC LIMIT 1`
		}
		var altro string
		var ord2 int
		if s.DB.QueryRow(q, loc, ord).Scan(&altro, &ord2) == nil {
			s.DB.Exec(`UPDATE media SET ordine = ? WHERE id = ?`, ord2, in.ID)
			s.DB.Exec(`UPDATE media SET ordine = ? WHERE id = ?`, ord, altro)
		}
	case in.K != "" && in.URL != "":
		if s.cercaLocalita(in.K) == nil {
			return nil, errNonTrovato
		}
		u := strings.TrimSpace(in.URL)
		if len(u) > 1000 || !(strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")) {
			return nil, errore(http.StatusBadRequest, "Indirizzo non valido.")
		}
		c, err := rigaSingola(deref(in.Cap), "Titolo", 0, 200)
		if err != nil {
			return nil, err
		}
		s.aggiungiMedia(in.K, "link", "", u, c)
	case in.K != "" && in.File != "":
		if s.cercaLocalita(in.K) == nil {
			return nil, errNonTrovato
		}
		rel, tipo, err := s.fileMediaValido(in.File)
		if err != nil {
			return nil, err
		}
		s.aggiungiMedia(in.K, tipo, rel, "", strings.TrimSuffix(path.Base(rel), path.Ext(rel)))
	default:
		return nil, errDatiNonValidi
	}
	s.avvisaTuttiMappa()
	return nil, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Server) aggiungiMedia(loc, tipo, file, url, cap string) string {
	id := cripto.ID()
	var ord int
	s.DB.QueryRow(`SELECT COALESCE(MAX(ordine), 0) + 1 FROM media WHERE localita = ?`, loc).Scan(&ord)
	s.DB.Exec(`INSERT INTO media(id, localita, tipo, file, url, didascalia, ordine, creato_il) VALUES(?,?,?,?,?,?,?,?)`,
		id, loc, tipo, file, url, cap, ord, db.Ora())
	return id
}

// mappaEliminaMedia toglie l'elemento dalla località. I file messi a mano nelle cartelle restano sul disco.
func (s *Server) mappaEliminaMedia(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, "master"); err != nil {
		return nil, err
	}
	s.DB.Exec(`DELETE FROM media WHERE id = ?`, r.URL.Query().Get("id"))
	s.avvisaTuttiMappa()
	return nil, nil
}

// ===== File multimediali =====

var estensioniMedia = map[string]string{".jpg": "img", ".jpeg": "img", ".png": "img", ".webp": "img", ".gif": "img",
	".mp4": "vid", ".webm": "vid", ".mov": "vid", ".m4v": "vid"}

// percorsoMedia trasforma un percorso relativo ("Immagini/Pietralta/a.webp") in un percorso sicuro dentro la
// cartella dei media. Rifiuta percorsi assoluti, "..", e nomi strani.
func (s *Server) percorsoMedia(rel string) (string, string, error) {
	rel = strings.ReplaceAll(rel, "\\", "/")
	pulito := path.Clean("/" + rel)[1:]
	if pulito == "" || pulito != strings.TrimPrefix(rel, "./") || strings.Contains(pulito, "..") || strings.ContainsAny(pulito, ":\x00") {
		return "", "", errore(http.StatusBadRequest, "Percorso non valido.")
	}
	return filepath.Join(s.Cfg.CartellaMedia, filepath.FromSlash(pulito)), pulito, nil
}

func (s *Server) fileMediaValido(rel string) (string, string, error) {
	p, pulito, err := s.percorsoMedia(rel)
	if err != nil {
		return "", "", err
	}
	tipo := estensioniMedia[strings.ToLower(filepath.Ext(pulito))]
	if tipo == "" || !(strings.HasPrefix(pulito, "Immagini/") || strings.HasPrefix(pulito, "Video/")) {
		return "", "", errore(http.StatusBadRequest, "Si possono collegare solo immagini e video delle cartelle Immagini e Video.")
	}
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return "", "", errNonTrovato
	}
	return pulito, tipo, nil
}

// mappaCarica: immagine o video caricato dal Master per una località (salvato in Immagini\<località> o Video\<località>).
func (s *Server) mappaCarica(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, "master"); err != nil {
		return nil, err
	}
	l := s.cercaLocalita(r.URL.Query().Get("k"))
	if l == nil {
		return nil, errNonTrovato
	}
	cap, _ := rigaSingola(r.URL.Query().Get("cap"), "Didascalia", 0, 200)
	body := io.LimitReader(r.Body, maxCaricamentoMedia+1)
	testa := make([]byte, 512)
	n, _ := io.ReadFull(body, testa)
	testa = testa[:n]
	ct := http.DetectContentType(testa)
	ext, cartella := "", ""
	switch {
	case ct == "image/jpeg":
		ext, cartella = ".jpg", "Immagini"
	case ct == "image/png":
		ext, cartella = ".png", "Immagini"
	case ct == "image/webp":
		ext, cartella = ".webp", "Immagini"
	case ct == "image/gif":
		ext, cartella = ".gif", "Immagini"
	case ct == "video/webm":
		ext, cartella = ".webm", "Video"
	case ct == "video/mp4" || isMP4(testa):
		ext, cartella = ".mp4", "Video"
	default:
		return nil, errore(http.StatusUnsupportedMediaType, "Formato non supportato: carica immagini (JPG, PNG, WebP, GIF) o video (MP4, WebM).")
	}
	dir := filepath.Join(s.Cfg.CartellaMedia, cartella, nomeCartella(l.N))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cartella dei media: %w", err)
	}
	nome := cripto.ID() + ext
	f, err := os.OpenFile(filepath.Join(dir, nome), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	scritti, err := io.Copy(f, io.MultiReader(strings.NewReader(string(testa)), body))
	f.Close()
	if err != nil || scritti > maxCaricamentoMedia {
		os.Remove(filepath.Join(dir, nome))
		if scritti > maxCaricamentoMedia {
			return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande (massimo 2 GB).")
		}
		return nil, errore(http.StatusBadRequest, "Caricamento interrotto.")
	}
	rel := cartella + "/" + nomeCartella(l.N) + "/" + nome
	tipo := "img"
	if cartella == "Video" {
		tipo = "vid"
	}
	id := s.aggiungiMedia(l.K, tipo, rel, "", cap)
	s.avvisaTuttiMappa()
	s.svegliaMedia() // versione per i telefoni del file appena caricato (media_preparazione.go)
	return map[string]string{"id": id}, nil
}

func isMP4(b []byte) bool { return len(b) > 12 && string(b[4:8]) == "ftyp" }

// nomeCartella: il nome della località usabile come cartella su Windows.
func nomeCartella(n string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, n)
}

// normalizza: minuscole e senza accenti, per riconoscere le cartelle con il nome di una località.
func normalizza(s string) string {
	return senzaAccenti.Replace(strings.ToLower(strings.TrimSpace(s)))
}

// Lettere accentate più comuni (italiano e nomi di Gaster Bless), senza librerie esterne.
var senzaAccenti = strings.NewReplacer("à", "a", "á", "a", "â", "a", "ä", "a", "ã", "a", "è", "e", "é", "e", "ê", "e", "ë", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i", "ò", "o", "ó", "o", "ô", "o", "ö", "o", "õ", "o", "ù", "u", "ú", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n", "’", "'")

// mappaCartelle: GET = file sciolti nelle cartelle Immagini e Video non ancora collegati;
// POST = collega da sé i file delle sottocartelle con il nome di una località.
func (s *Server) mappaCartelle(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, _, _, err := s.contestoMappa(r, "master"); err != nil {
		return nil, err
	}
	collegati := map[string]bool{}
	rows, _ := s.DB.Query(`SELECT file FROM media WHERE file <> ''`)
	for rows != nil && rows.Next() {
		var f string
		rows.Scan(&f)
		collegati[f] = true
	}
	if rows != nil {
		rows.Close()
	}
	perNome := map[string]*localita{}
	for i, l := range s.localita() {
		perNome[normalizza(l.N)] = &s.localita()[i]
		perNome[normalizza(l.K)] = &s.localita()[i]
	}
	var sciolti []string
	nuovi := 0
	for _, base := range []string{"Immagini", "Video"} {
		radice := filepath.Join(s.Cfg.CartellaMedia, base)
		filepath.WalkDir(radice, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if strings.EqualFold(d.Name(), "Bestiario") {
					return filepath.SkipDir
				}
				return nil
			}
			tipo := estensioniMedia[strings.ToLower(filepath.Ext(p))]
			if tipo == "" {
				return nil
			}
			relOS, _ := filepath.Rel(s.Cfg.CartellaMedia, p)
			rel := filepath.ToSlash(relOS)
			if collegati[rel] {
				return nil
			}
			parti := strings.Split(rel, "/")
			if r.Method == http.MethodPost && len(parti) >= 3 {
				if l := perNome[normalizza(parti[1])]; l != nil {
					s.aggiungiMedia(l.K, tipo, rel, "", strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
					collegati[rel] = true
					nuovi++
					return nil
				}
			}
			sciolti = append(sciolti, rel)
			return nil
		})
	}
	sort.Strings(sciolti)
	if sciolti == nil {
		sciolti = []string{}
	}
	if nuovi > 0 {
		s.avvisaTuttiMappa()
	}
	if r.Method == http.MethodPost {
		s.svegliaMedia() // «Cerca nelle cartelle»: prepara anche i file copiati a mano
	}
	return map[string]any{"loose": sciolti, "linked": nuovi}, nil
}

// servimedia restituisce un file della cartella dei media. Il Master li vede tutti; un giocatore solo
// quelli svelati (o in proiezione) a un gruppo di cui fa parte.
func (s *Server) servimedia(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	if !u.Approvato() {
		http.Error(w, "Accesso richiesto.", http.StatusUnauthorized)
		return
	}
	p, rel, err := s.percorsoMedia(r.PathValue("percorso"))
	if err != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	if !u.Master() && !s.mediaVisibile(u, rel) {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	if estensioniMedia[strings.ToLower(filepath.Ext(rel))] == "" {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	// Un giorno nella memoria del dispositivo (2026-11-11): rivedere un video nella stessa serata non lo riscarica;
	// il giorno dopo si ricontrolla (risposta «non cambiato» se è lo stesso file).
	w.Header().Set("Cache-Control", "private, max-age=86400")
	// Ai giocatori la versione per i telefoni se c'è, al Master la copia leggera (media_leggeri.go); richieste parziali comprese.
	s.serviFileMedia(w, r, p, !u.Master())
}

func (s *Server) mediaVisibile(u *Utente, rel string) bool {
	if strings.HasPrefix(rel, "Immagini/Bestiario/") { // immagini del bestiario: solo se mostrate o proiettate
		for _, g := range s.gruppiDi(u) {
			if s.immagineBestiarioVisibile(g, rel) {
				return true
			}
		}
		return false
	}
	var id string
	if s.DB.QueryRow(`SELECT id FROM media WHERE file = ?`, rel).Scan(&id) != nil {
		return false
	}
	for _, g := range s.gruppiDi(u) {
		st, err := s.leggiStatoMappa(g)
		if err != nil {
			continue
		}
		if st.Media[id] || st.Proiez["id"] == id {
			return true
		}
	}
	return false
}

// ===== Proiezione =====

func (s *Server) mappaProiezione(w http.ResponseWriter, r *http.Request) (any, error) {
	_, sessione, gruppo, err := s.contestoMappa(r, "master")
	if err != nil {
		return nil, err
	}
	st, err := s.leggiStatoMappa(gruppo)
	if err != nil {
		return nil, err
	}
	if r.Method == http.MethodGet {
		return st.Proiez, nil
	}
	var in struct{ Kind, K, ID, File, Cap, Dove string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	// Dove si proietta (scelta del Master): sui dispositivi dei giocatori (audio solo sul Master), su un
	// secondo schermo collegato al PC, oppure sul proiettore Wowlink (finestra trasmessa senza fili).
	switch in.Dove {
	case "giocatori", "schermo", "wowlink":
	case "":
		in.Dove = "schermo"
	default:
		return nil, errDatiNonValidi
	}
	p := map[string]any{"kind": in.Kind, "dove": in.Dove}
	switch in.Kind {
	case "none", "map":
	case "beast":
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM bestiario_personalizzazioni WHERE immagine = ? AND immagine <> ''`, in.File).Scan(&n)
		if n == 0 {
			return nil, errNonTrovato
		}
		c, _ := rigaSingola(in.Cap, "Didascalia", 0, 80)
		p["file"], p["cap"] = in.File, c
	case "builtin":
		l := s.cercaLocalita(in.K)
		if l == nil || !bool(l.B) {
			return nil, errNonTrovato
		}
		p["k"] = in.K
	case "item":
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM media WHERE id = ? AND tipo <> 'link'`, in.ID).Scan(&n)
		if n == 0 {
			return nil, errNonTrovato
		}
		p["id"] = in.ID
	default:
		return nil, errDatiNonValidi
	}
	b, _ := json.Marshal(p)
	s.DB.Exec(`UPDATE stato_mappa SET proiezione = ?, modificato_il = ? WHERE gruppo_id = ?`, string(b), db.Ora(), gruppo)
	s.avvisaMappa(gruppo, map[string]any{"t": "proj", "p": p})
	s.avvisaMappa(gruppo, map[string]any{"t": "state"}) // ciò che si proietta diventa visibile ai giocatori
	// I giocatori della sessione che non hanno la Mappa aperta ci vengono portati (app.js, «mappa.chiamata»).
	if in.Dove == "giocatori" && in.Kind != "none" && !strings.HasPrefix(sessione, "g:") {
		s.chiamaGiocatori(sessione, "mappa.chiamata", nil)
	}
	return p, nil
}

func (s *Server) mappaComandoProiezione(w http.ResponseWriter, r *http.Request) (any, error) {
	_, _, gruppo, err := s.contestoMappa(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct{ C string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	switch in.C {
	case "play", "pause", "restart", "mute":
	default:
		return nil, errDatiNonValidi
	}
	s.avvisaMappa(gruppo, map[string]any{"t": "cmd", "c": in.C})
	return nil, nil
}

// ===== Aggiornamenti in tempo reale (Server-Sent Events, come la pagina originale) =====

func (s *Server) avvisaMappa(gruppo string, ev map[string]any) {
	s.Hub.Pubblica("mappa:"+gruppo, Msg{Tipo: "mappa", Dati: ev})
}

// avvisaTuttiMappa: i contenuti delle località valgono per tutti i gruppi.
func (s *Server) avvisaTuttiMappa() {
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
		s.avvisaMappa(g, map[string]any{"t": "state"})
	}
}

func (s *Server) mappaEventi(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	if !u.Approvato() {
		http.Error(w, "Accesso richiesto.", http.StatusUnauthorized)
		return
	}
	_, gruppo := s.ruoloMappa(u, r.URL.Query().Get("sessione"))
	if gruppo == "" {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Non supportato.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	c := &Cliente{U: u, ip: rete.IPRichiesta(r), invio: make(chan []byte, 64), canali: map[string]bool{"mappa:" + gruppo: true}, chiudi: cancel}
	s.Hub.mu.Lock()
	s.Hub.clienti[c] = true
	s.Hub.mu.Unlock()
	defer func() {
		s.Hub.mu.Lock()
		delete(s.Hub.clienti, c)
		s.Hub.mu.Unlock()
	}()
	fmt.Fprint(w, ": gasterbless\n\n")
	fl.Flush()
	t := time.NewTicker(25 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !s.utenteAncoraValido(u.ID) {
				return
			}
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case b := <-c.invio:
			var m struct {
				Tipo string          `json:"tipo"`
				Dati json.RawMessage `json:"dati"`
			}
			if json.Unmarshal(b, &m) != nil || m.Tipo != "mappa" {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", m.Dati)
			fl.Flush()
		}
	}
}

// mappaLocaleVisibile: la mappa locale della località k si può mostrare all'utente (Master sempre;
// giocatore se svelata al suo gruppo, fuori dalla nebbia, oppure in proiezione).
func (s *Server) mappaLocaleVisibile(u *Utente, k string) bool {
	if u.Master() {
		return true
	}
	l := s.cercaLocalita(k)
	if !u.Approvato() || l == nil {
		return false
	}
	for _, g := range s.gruppiDi(u) {
		st, err := s.leggiStatoMappa(g)
		if err != nil {
			continue
		}
		if st.Proiez["kind"] == "builtin" && st.Proiez["k"] == k {
			return true
		}
		if st.Nomi[k] && !st.Nascoste[k] && st.svelata(l.X, l.Y) {
			return true
		}
	}
	return false
}

// salvaNebbia scrive la nebbia del gruppo. Senza base la sostituisce; con la base unisce: per ogni cella
// che il dispositivo ha cambiato (nuova diversa dalla base) vale la nuova, per le altre resta quella sul
// server (magari cambiata nel frattempo da un altro dispositivo del Master). Restituisce la nebbia salvata.
func (s *Server) salvaNebbia(gruppo string, nuova, base []byte) ([]byte, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	unita := nuova
	if base != nil {
		var attuale []byte
		if err := tx.QueryRow(`SELECT nebbia FROM stato_mappa WHERE gruppo_id = ?`, gruppo).Scan(&attuale); err != nil {
			return nil, err
		}
		if len(attuale) != byteNebbia {
			attuale = nebbiaVuota()
		}
		unita = make([]byte, byteNebbia)
		for i := range unita {
			cambiate := nuova[i] ^ base[i]
			unita[i] = attuale[i]&^cambiate | nuova[i]&cambiate
		}
	}
	if _, err := tx.Exec(`UPDATE stato_mappa SET nebbia = ?, modificato_il = ? WHERE gruppo_id = ?`, unita, db.Ora(), gruppo); err != nil {
		return nil, err
	}
	return unita, tx.Commit()
}
