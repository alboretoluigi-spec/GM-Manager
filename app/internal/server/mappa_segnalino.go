package server

// Segnalino del gruppo sulla mappa.
//
// Ogni gruppo ha un segnalino principale e, se il gruppo si divide, altri segnalini con una parte dei
// PG. Il principale contiene tutti i PG della sessione che non sono in un altro segnalino. Master e
// giocatori della sessione possono spostare, dividere e riunire; solo il Master ne cambia l'aspetto
// (nome, colore, simbolo) o lo toglie. Tutto sta nella colonna stato_mappa.posizione (JSON).

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const maxSegnalini = 8

type segnalino struct {
	ID string   `json:"id"`
	X  float64  `json:"x"`
	Y  float64  `json:"y"`
	PG []string `json:"pg,omitempty"` // solo per i segnalini nati da una divisione
}

type datiSegnalini struct {
	Segnalini []segnalino `json:"segnalini"` // il primo è il principale
	Nome      string      `json:"nome,omitempty"`
	Colore    string      `json:"colore,omitempty"`
	Simbolo   string      `json:"simbolo,omitempty"`
	// Sessione in cui il gruppo è stato diviso (2026-11-08): in una sessione nuova il gruppo riparte unito.
	Sessione string `json:"sessione,omitempty"`
}

// perSessione: in una sessione diversa da quella della divisione restano solo il segnalino principale (con tutti).
// Nella preparazione/consultazione («g:…») si vede com'era.
func (d *datiSegnalini) perSessione(sessione string) {
	if len(d.Segnalini) > 1 && !strings.HasPrefix(sessione, "g:") && d.Sessione != sessione {
		d.Segnalini = d.Segnalini[:1]
		d.Sessione = ""
	}
}

// leggiSegnalini interpreta la colonna posizione (anche nel vecchio formato {"x":..,"y":..}).
func leggiSegnalini(v string) *datiSegnalini {
	d := &datiSegnalini{}
	if v == "" {
		return d
	}
	var vecchio struct{ X, Y *float64 }
	if json.Unmarshal([]byte(v), &vecchio) == nil && vecchio.X != nil && vecchio.Y != nil {
		d.Segnalini = []segnalino{{ID: "g", X: *vecchio.X, Y: *vecchio.Y}}
		return d
	}
	json.Unmarshal([]byte(v), d)
	return d
}

func (s *Server) salvaSegnalini(gruppo string, d *datiSegnalini) {
	if len(d.Segnalini) == 0 && d.Nome == "" && d.Colore == "" && d.Simbolo == "" {
		s.DB.Exec(`UPDATE stato_mappa SET posizione = '', modificato_il = ? WHERE gruppo_id = ?`, db.Ora(), gruppo)
	} else {
		b, _ := json.Marshal(d)
		s.DB.Exec(`UPDATE stato_mappa SET posizione = ?, modificato_il = ? WHERE gruppo_id = ?`, string(b), db.Ora(), gruppo)
	}
	s.avvisaMappa(gruppo, map[string]any{"t": "state"})
	go s.controllaInneschi(gruppo, "arrivo", d) // eventi del Codice «quando il gruppo arriva a…» (lore_promemoria.go)
}

func dentroMappa(x, y float64) bool { return x >= 0 && x <= mappaL && y >= 0 && y <= mappaA }

// mappaSegnalino: POST /api/v1/mappa/party {azione, ...}.
func (s *Server) mappaSegnalino(w http.ResponseWriter, r *http.Request) (any, error) {
	u, sessione, gruppo, err := s.contestoMappa(r, "")
	if err != nil {
		return nil, err
	}
	var in struct {
		Azione  string     `json:"azione"`
		ID      string     `json:"id"`
		X       float64    `json:"x"`
		Y       float64    `json:"y"`
		PG      []string   `json:"pg"`
		Nome    string     `json:"nome"`
		Colore  string     `json:"colore"`
		Simbolo string     `json:"simbolo"`
		Viaggio *viaggioIn `json:"viaggio"` // percorso calcolato dalla pagina per lo spostamento (viaggi.go)
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	st, err := s.leggiStatoMappa(gruppo)
	if err != nil {
		return nil, err
	}
	d := st.Seg
	d.perSessione(sessione)
	trova := func(id string) int {
		for i, x := range d.Segnalini {
			if x.ID == id {
				return i
			}
		}
		return -1
	}
	fuori := errore(http.StatusBadRequest, "Posizione fuori dalla mappa.")
	var viaggio *viaggioIn
	var daX, daY float64
	switch in.Azione {
	case "posiziona": // il segnalino principale va in (x, y); se non c'è, si crea
		if !dentroMappa(in.X, in.Y) {
			return nil, fuori
		}
		if len(d.Segnalini) == 0 {
			d.Segnalini = []segnalino{{ID: "g"}}
			in.Viaggio = nil // prima posizione: non è un viaggio
		}
		daX, daY = d.Segnalini[0].X, d.Segnalini[0].Y
		d.Segnalini[0].X, d.Segnalini[0].Y = float64(int(in.X)), float64(int(in.Y))
		viaggio = in.Viaggio
	case "muovi":
		i := trova(in.ID)
		if i < 0 {
			return nil, errNonTrovato
		}
		if !dentroMappa(in.X, in.Y) {
			return nil, fuori
		}
		daX, daY = d.Segnalini[i].X, d.Segnalini[i].Y
		d.Segnalini[i].X, d.Segnalini[i].Y = float64(int(in.X)), float64(int(in.Y))
		viaggio = in.Viaggio
	case "dividi": // i PG indicati lasciano il segnalino id e formano un nuovo segnalino in (x, y)
		d.Sessione = sessione
		i := trova(in.ID)
		if i < 0 {
			return nil, errNonTrovato
		}
		if len(d.Segnalini) >= maxSegnalini {
			return nil, errore(http.StatusBadRequest, "Troppi segnalini: riunisci prima qualche parte del gruppo.")
		}
		if !dentroMappa(in.X, in.Y) {
			return nil, fuori
		}
		presenti := map[string]bool{}
		for _, p := range s.pgDelSegnalino(sessione, d, i) {
			presenti[p] = true
		}
		var scelti []string
		for _, p := range in.PG {
			if presenti[p] {
				scelti = append(scelti, p)
				delete(presenti, p)
			}
		}
		if len(scelti) == 0 || len(presenti) == 0 {
			return nil, errore(http.StatusBadRequest, "Scegli chi si separa: almeno un personaggio, ma non tutti.")
		}
		if i > 0 { // da un segnalino secondario si tolgono i PG che se ne vanno
			resto := []string{}
			for _, p := range d.Segnalini[i].PG {
				if presenti[p] {
					resto = append(resto, p)
				}
			}
			d.Segnalini[i].PG = resto
		}
		d.Segnalini = append(d.Segnalini, segnalino{ID: cripto.ID()[:8], X: float64(int(in.X)), Y: float64(int(in.Y)), PG: scelti})
	case "riunisci": // un segnalino secondario torna nel principale
		i := trova(in.ID)
		if i <= 0 {
			return nil, errNonTrovato
		}
		d.Segnalini = append(d.Segnalini[:i], d.Segnalini[i+1:]...)
	case "aspetto":
		if !u.Master() {
			return nil, errVietato
		}
		nome, err := rigaSingola(in.Nome, "Nome", 0, 40)
		if err != nil {
			return nil, err
		}
		simbolo := strings.TrimSpace(in.Simbolo)
		if utf8.RuneCountInString(simbolo) > 4 || strings.ContainsAny(simbolo, "<>&\"'") {
			return nil, errore(http.StatusBadRequest, "Il simbolo deve essere un'emoji o al massimo 4 caratteri.")
		}
		if in.Colore != "" && !coloreValido(in.Colore) {
			return nil, errore(http.StatusBadRequest, "Colore non valido.")
		}
		d.Nome, d.Colore, d.Simbolo = nome, strings.ToLower(in.Colore), simbolo
	case "togli":
		if !u.Master() {
			return nil, errVietato
		}
		d.Segnalini = nil
	default:
		return nil, errDatiNonValidi
	}
	s.salvaSegnalini(gruppo, d)
	if viaggio != nil {
		s.registraViaggio(u, gruppo, sessione, [2]float64{daX, daY}, [2]float64{float64(int(in.X)), float64(int(in.Y))}, viaggio)
	}
	return nil, nil
}

// pgDelSegnalino: i PG (della sessione) che stanno nel segnalino i.
func (s *Server) pgDelSegnalino(sessione string, d *datiSegnalini, i int) []string {
	tutti := s.pgSessione(sessione)
	if i > 0 {
		in := map[string]bool{}
		for _, p := range d.Segnalini[i].PG {
			in[p] = true
		}
		var out []string
		for _, p := range tutti {
			if in[p.id] {
				out = append(out, p.id)
			}
		}
		return out
	}
	altrove := map[string]bool{}
	for _, x := range d.Segnalini[1:] {
		for _, p := range x.PG {
			altrove[p] = true
		}
	}
	var out []string
	for _, p := range tutti {
		if !altrove[p.id] {
			out = append(out, p.id)
		}
	}
	return out
}

type pgSegnalino struct {
	id, nome string
	ritratto bool
}

// pgSessione: i personaggi della sessione (nome e presenza del ritratto). Ci sono tutti i partecipanti tranne chi ha
// detto «Non posso più venire» (confermato = -1); chi non ha scelto il personaggio entra con il suo personaggio attivo
// più recente del gruppo (o, se non ne ha nel gruppo, il più recente in assoluto) (2026-11-08).
func (s *Server) pgSessione(sessione string) []pgSegnalino {
	var out []pgSegnalino
	rows, err := s.DB.Query(`SELECT DISTINCT p.id, p.nome, v.dati LIKE '%"img":"data:image/%' FROM partecipanti x
		JOIN sessioni_gioco sg ON sg.id = x.sessione_id
		JOIN personaggi p ON p.id = COALESCE(x.personaggio_id, (SELECT q.id FROM personaggi q WHERE q.proprietario_id = x.utente_id
			AND q.stato = 'attivo' ORDER BY (q.gruppo_id = sg.gruppo_id) DESC, q.modificato_il DESC LIMIT 1))
		LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		WHERE x.sessione_id = ? AND x.confermato >= 0 AND p.stato = 'attivo' ORDER BY p.nome`, sessione)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var p pgSegnalino
		rows.Scan(&p.id, &p.nome, &p.ritratto)
		out = append(out, p)
	}
	return out
}

// segnalinoGruppo: segnalini con i loro PG (nome e ritratto dalle schede) e aspetto del gruppo.
// Il ritratto lo possono vedere tutti i compagni di gruppo (vedi ritratto()).
func (s *Server) segnalinoGruppo(sessione string, st *statoMappa) any {
	d := st.Seg
	d.perSessione(sessione)
	if len(d.Segnalini) == 0 && d.Nome == "" && d.Colore == "" && d.Simbolo == "" {
		return nil
	}
	tutti := s.pgSessione(sessione)
	vista := func(p pgSegnalino) map[string]any {
		return map[string]any{"id": p.id, "nome": p.nome, "ritratto": p.ritratto}
	}
	altrove := map[string]bool{}
	for _, x := range d.Segnalini[min(1, len(d.Segnalini)):] {
		for _, p := range x.PG {
			altrove[p] = true
		}
	}
	segn := []map[string]any{}
	for i, x := range d.Segnalini {
		pg := []map[string]any{}
		dentro := map[string]bool{}
		for _, p := range x.PG {
			dentro[p] = true
		}
		for _, p := range tutti {
			if i == 0 && !altrove[p.id] || i > 0 && dentro[p.id] {
				pg = append(pg, vista(p))
			}
		}
		if i > 0 && len(pg) == 0 {
			continue // chi era in questo segnalino non partecipa più alla sessione
		}
		segn = append(segn, map[string]any{"id": x.ID, "x": x.X, "y": x.Y, "pg": pg, "principale": i == 0})
	}
	tuttiV := []map[string]any{}
	for _, p := range tutti {
		tuttiV = append(tuttiV, vista(p))
	}
	return map[string]any{"segnalini": segn, "nome": d.Nome, "colore": d.Colore, "simbolo": d.Simbolo, "pgTutti": tuttiV}
}
