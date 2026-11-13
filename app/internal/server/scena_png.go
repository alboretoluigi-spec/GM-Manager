package server

// «PNG in scena» (richiesta dell'utente, 2026-11-12). Quando il Master interpreta un PNG (o fa comparire un mostro), lo
// sceglie da un elenco e con un clic la sua immagine compare sugli schermi dei giocatori del gruppo: senza riquadro né
// sfondo (Gaster Bless resta visibile dietro), alta quasi tutto lo schermo, larga circa metà, su un lato (web/ui/scena.js).
// L'immagine è quella della voce di «PNG e mostri» (bestiario_personalizzazioni.immagine). Lo stato è in memoria: chi
// apre Gaster Bless dopo la vede lo stesso; un riavvio la toglie.

import (
	"net/http"
	"strings"
	"sync"

	"gasterbless/internal/db"
)

// Più immagini insieme (richiesta dell'utente, 2026-11-13): ogni gruppo ha un elenco (al massimo maxScena), nell'ordine in
// cui il Master le ha messe; ognuna su un lato (sinistra, centro, destra). Con «aggiungi» un PNG si aggiunge agli altri (o,
// se c'è già, cambia lato e nome); senza, prende il posto di tutti (come prima). DELETE con ?voce= toglie solo quello.
// Messaggi: «scena.png» e, per il proiettore, «t: scena» con l'elenco intero («scene») e l'ultima («scena», compatibilità).

const maxScena = 6

type scenaPNG struct {
	Gruppo string `json:"gruppo"`
	Voce   string `json:"voce"`
	File   string `json:"file"`
	URL    string `json:"url"`
	Nome   string `json:"nome"`
	Lato   string `json:"lato"` // sinistra, centro, destra
	Quando int64  `json:"quando"`
}

type scenePNG struct {
	mu sync.Mutex
	m  map[string][]scenaPNG // gruppo → PNG in scena, nell'ordine in cui sono stati messi
}

// visibile: il file è l'immagine di un PNG in scena nel gruppo (mappa_bestiario.go, immagineBestiarioVisibile)
func (x *scenePNG) visibile(g, file string) bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	for _, v := range x.m[g] {
		if v.File == file {
			return true
		}
	}
	return false
}

func (s *Server) rotteScenaPNG(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/scena", api(s.apiScena))
	mux.Handle("POST /api/v1/scena", api(s.apiMettiInScena))
	mux.Handle("DELETE /api/v1/scena", api(s.apiTogliDallaScena))
}

// apiScena: i PNG in scena nei gruppi di chi chiede (il Master: tutti), elenco piatto nell'ordine di ogni gruppo
func (s *Server) apiScena(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	out := []scenaPNG{}
	s.scene.mu.Lock()
	defer s.scene.mu.Unlock()
	if u.Master() {
		for _, l := range s.scene.m {
			out = append(out, l...)
		}
		return out, nil
	}
	for _, g := range s.gruppiDi(u) {
		out = append(out, s.scene.m[g]...)
	}
	return out, nil
}

// annunciaScena: l'elenco del gruppo ai giocatori, al Master e alla finestra di proiezione
func (s *Server) annunciaScena(g string, l []scenaPNG) {
	dati := map[string]any{"gruppo": g, "attiva": len(l) > 0, "scene": l}
	mappa := map[string]any{"t": "scena", "attiva": len(l) > 0, "scene": l}
	if len(l) > 0 {
		dati["scena"], mappa["scena"] = l[len(l)-1], l[len(l)-1]
	}
	s.Hub.Pubblica("gruppo:"+g, Msg{Tipo: "scena.png", Dati: dati})
	// anche sul proiettore: la finestra di proiezione della Mappa ascolta il canale della mappa del gruppo (2026-11-12)
	s.avvisaMappa(g, mappa)
}

func (s *Server) apiMettiInScena(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Gruppo   string `json:"gruppo"`
		Voce     string `json:"voce"`
		Nome     string `json:"nome"`
		Lato     string `json:"lato"`
		Aggiungi bool   `json:"aggiungi"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM gruppi WHERE id = ?`, in.Gruppo).Scan(&n)
	if n == 0 {
		return nil, errore(http.StatusBadRequest, "Scegli un gruppo.")
	}
	var img string
	s.DB.QueryRow(`SELECT immagine FROM bestiario_personalizzazioni WHERE voce_id = ?`, in.Voce).Scan(&img)
	if img == "" || !strings.HasPrefix(img, "Immagini/") {
		return nil, errore(http.StatusBadRequest, "Questa voce non ha un'immagine: caricala in «PNG e mostri».")
	}
	nome, err := rigaSingola(in.Nome, "Nome", 0, 80)
	if err != nil {
		return nil, err
	}
	if in.Lato != "sinistra" && in.Lato != "centro" {
		in.Lato = "destra"
	}
	sc := scenaPNG{Gruppo: in.Gruppo, Voce: in.Voce, File: img, URL: "/media/" + img, Nome: nome, Lato: in.Lato, Quando: db.Ora()}
	s.scene.mu.Lock()
	if s.scene.m == nil {
		s.scene.m = map[string][]scenaPNG{}
	}
	l := s.scene.m[in.Gruppo]
	if !in.Aggiungi {
		l = nil
	}
	c := -1
	for i, v := range l {
		if v.Voce == in.Voce {
			c = i
		}
	}
	switch {
	case c >= 0: // c'è già: cambia lato e nome, resta al suo posto
		l[c].Lato, l[c].Nome, l[c].Quando = sc.Lato, sc.Nome, sc.Quando
		sc = l[c]
	case len(l) >= maxScena:
		s.scene.mu.Unlock()
		return nil, errore(http.StatusConflict, "Al massimo 6 immagini insieme: togline una.")
	default:
		l = append(l, sc)
	}
	s.scene.m[in.Gruppo] = l
	copia := append([]scenaPNG(nil), l...)
	s.scene.mu.Unlock()
	s.annunciaScena(in.Gruppo, copia)
	return sc, nil
}

// apiTogliDallaScena: ?gruppo=… toglie tutti; con &voce=… solo quel PNG
func (s *Server) apiTogliDallaScena(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	g, voce := r.URL.Query().Get("gruppo"), r.URL.Query().Get("voce")
	s.scene.mu.Lock()
	var resto []scenaPNG
	if voce != "" {
		for _, v := range s.scene.m[g] {
			if v.Voce != voce {
				resto = append(resto, v)
			}
		}
	}
	if len(resto) == 0 {
		delete(s.scene.m, g)
	} else {
		s.scene.m[g] = resto
	}
	s.scene.mu.Unlock()
	s.annunciaScena(g, resto)
	return nil, nil
}
