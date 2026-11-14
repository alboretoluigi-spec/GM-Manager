package server

// Interfaccia adattiva (fase 3c, passo C4a, docs/INTERFACCIA_ADATTIVA.md). Ogni campagna ha un modulo di gioco (registro
// in web/sistemi/moduli.json); all'accesso il Master sceglie la campagna da giocare e l'interfaccia di tutti (Master e
// giocatori) mostra solo le sezioni e i pulsanti di quel modulo. La campagna attiva è l'impostazione «campagna_attiva»;
// se manca vale la prima campagna creata.

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
)

type moduloGioco struct {
	ID        string   `json:"id"`
	Nome      string   `json:"nome"`
	Meccanica string   `json:"meccanica"`
	Pronto    bool     `json:"pronto"`
	Tiro      string   `json:"tiro,omitempty"` // tiro della famiglia (meccaniche.go)
	Sezioni   []string `json:"sezioni"`
	Barra     []string `json:"barra"`
}

type registroModuli struct {
	Comuni []string      `json:"comuni"`
	Moduli []moduloGioco `json:"moduli"`
}

// moduliDiGioco legge il registro (file piccolo, incorporato nel programma). Senza il file valgono i due moduli di base.
func (s *Server) moduliDiGioco() *registroModuli {
	var reg registroModuli
	if b, err := fs.ReadFile(s.Web, "sistemi/moduli.json"); err == nil {
		json.Unmarshal(b, &reg)
	}
	if len(reg.Moduli) == 0 {
		reg.Moduli = []moduloGioco{{ID: "d20-5e", Nome: "Quinta edizione (d20)", Pronto: true}, {ID: "libera", Nome: "Scheda libera (qualsiasi gioco)", Pronto: true}}
	}
	return &reg
}

func (s *Server) moduloGioco(id string) *moduloGioco {
	reg := s.moduliDiGioco()
	for i := range reg.Moduli {
		if reg.Moduli[i].ID == id {
			return &reg.Moduli[i]
		}
	}
	return nil
}

// campagnaAttiva: la campagna scelta dal Master (scelta = true) oppure, se non l'ha ancora scelta o è stata eliminata,
// la prima campagna creata.
func (s *Server) campagnaAttiva() (id string, scelta bool) {
	if v, _ := s.impostazione("campagna_attiva"); v != "" {
		if s.campagnaEsiste(v) == nil {
			return v, true
		}
	}
	s.DB.QueryRow(`SELECT id FROM campagne ORDER BY creato_il, id LIMIT 1`).Scan(&id)
	return id, false
}

// infoGioco: ciò che serve alle pagine per adattarsi (campagna, modulo, sezioni e pulsanti accesi).
func (s *Server) infoGioco() map[string]any {
	id, scelta := s.campagnaAttiva()
	var nome, modulo string
	s.DB.QueryRow(`SELECT nome, modulo FROM campagne WHERE id = ?`, id).Scan(&nome, &modulo)
	m := s.moduloGioco(modulo)
	if m == nil {
		m = s.moduloGioco("libera")
	}
	reg := s.moduliDiGioco()
	sez := append(append([]string{}, reg.Comuni...), m.Sezioni...)
	barra := append([]string{}, m.Barra...)
	return map[string]any{"campagna": map[string]string{"id": id, "nome": nome}, "modulo": m, "sezioni": sez, "barra": barra, "scelta": scelta}
}

// avvisaGioco manda a tutti i dispositivi collegati il gioco attivo: le pagine si ridisegnano.
func (s *Server) avvisaGioco() { s.Hub.ATutti(Msg{Tipo: "gioco", Dati: s.infoGioco()}) }

func (s *Server) rotteGioco(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/gioco", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		u, err := richiedi(r, "approvato")
		if err != nil {
			return nil, err
		}
		out := s.infoGioco()
		if u.Master() {
			out["moduli"] = s.moduliDiGioco().Moduli
		}
		return out, nil
	}))
	mux.Handle("PUT /api/v1/gioco", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		u, err := richiedi(r, "master")
		if err != nil {
			return nil, err
		}
		var in struct {
			Campagna string `json:"campagna"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
			return nil, errDatiNonValidi
		}
		if err := s.campagnaEsiste(in.Campagna); err != nil {
			return nil, err
		}
		if err := s.impostaImpostazione("campagna_attiva", in.Campagna); err != nil {
			return nil, err
		}
		s.evento("gioco_scelto", u.ID, in.Campagna)
		s.avvisaGioco()
		return s.infoGioco(), nil
	}))
}
