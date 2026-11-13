package server

// Scheda libera (fase 3c, passo C4b, docs/SCHEDA_LIBERA.md): per i giochi senza un modulo dedicato il Master decide
// il modello della scheda della campagna (sezioni con campi, tiri con formule, punti ferita, difesa, iniziativa). La
// pagina è web/scheda_libera/ e parla con l'app con lo stesso protocollo della scheda della quinta edizione. Le formule
// dei tiri usano i dadi del server (d4, d6, d8, d10, d12, d20, d100), i valori dei campi tra graffe e un modificatore.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type campoModello struct {
	ID          string `json:"id"`
	Nome        string `json:"nome"`
	Tipo        string `json:"tipo"` // testo, area, numero, risorsa, spunta, elenco
	Min         *int   `json:"min,omitempty"`
	Max         *int   `json:"max,omitempty"`
	Predefinito any    `json:"predefinito,omitempty"`
	Aiuto       string `json:"aiuto,omitempty"`
	Tiro        string `json:"tiro,omitempty"` // formula del tiro con un clic sul campo (es. «1d20 + {forza}»)
}

type sezioneModello struct {
	Titolo string         `json:"titolo"`
	Campi  []campoModello `json:"campi"`
}

type tiroModello struct {
	Nome    string `json:"nome"`
	Formula string `json:"formula"`
}

type modelloScheda struct {
	Versione   int              `json:"versione"`
	Sezioni    []sezioneModello `json:"sezioni"`
	Tiri       []tiroModello    `json:"tiri"`
	PF         string           `json:"pf"`         // id di un campo «risorsa» o «numero»: i punti ferita massimi
	Difesa     string           `json:"difesa"`     // id di un campo «numero»: la difesa (come la CA) per il tracker
	Iniziativa string           `json:"iniziativa"` // formula dell'iniziativa; vuota = la decide il Master
}

var (
	reIDCampo    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,29}$`)
	reRifCampo   = regexp.MustCompile(`\{([a-z][a-z0-9_]{0,29})\}`)
	reTermine    = regexp.MustCompile(`^(\d{0,2})d(\d{1,3})$`)
	tipiCampo    = map[string]bool{"testo": true, "area": true, "numero": true, "risorsa": true, "spunta": true, "elenco": true}
	facceFormula = map[int]bool{4: true, 6: true, 8: true, 10: true, 12: true, 20: true, 100: true}
)

// modelloBase: la scheda libera di partenza, da adattare al gioco (campi generici, nessun contenuto di gioco).
func modelloBase() modelloScheda {
	zero, cento := 0, 100
	return modelloScheda{Versione: 1, Sezioni: []sezioneModello{
		{Titolo: "Personaggio", Campi: []campoModello{{ID: "concetto", Nome: "Concetto", Tipo: "testo"}, {ID: "aspetto", Nome: "Aspetto e storia", Tipo: "area"}}},
		{Titolo: "Valori", Campi: []campoModello{
			{ID: "corpo", Nome: "Corpo", Tipo: "numero", Min: &zero, Max: &cento, Predefinito: 0, Tiro: "1d20 + {corpo}"},
			{ID: "mente", Nome: "Mente", Tipo: "numero", Min: &zero, Max: &cento, Predefinito: 0, Tiro: "1d20 + {mente}"},
			{ID: "spirito", Nome: "Spirito", Tipo: "numero", Min: &zero, Max: &cento, Predefinito: 0, Tiro: "1d20 + {spirito}"}}},
		{Titolo: "Condizione", Campi: []campoModello{{ID: "salute", Nome: "Salute", Tipo: "risorsa", Predefinito: 10}, {ID: "difesa", Nome: "Difesa", Tipo: "numero", Predefinito: 10}}},
		{Titolo: "Equipaggiamento e note", Campi: []campoModello{{ID: "equipaggiamento", Nome: "Equipaggiamento", Tipo: "elenco"}, {ID: "note", Nome: "Note", Tipo: "area"}}},
	}, PF: "salute", Difesa: "difesa", Iniziativa: "1d20 + {mente}"}
}

// termineFormula: un gruppo di dadi o un numero, con il segno.
type termineFormula struct {
	Segno int
	N     int
	Facce int // 0 = numero fisso (N)
}

// leggiFormula legge «2d6 + 1d4 - 2» (dopo la sostituzione dei campi). Dadi ammessi: d4, d6, d8, d10, d12, d20, d100.
func leggiFormula(f string) ([]termineFormula, error) {
	s := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(f)), " ", "")
	s = strings.ReplaceAll(s, "−", "-")
	if s == "" {
		return nil, fmt.Errorf("formula vuota")
	}
	if s[0] != '+' && s[0] != '-' {
		s = "+" + s
	}
	out := []termineFormula{}
	dadi := 0
	for len(s) > 0 {
		segno := 1
		if s[0] == '-' {
			segno = -1
		}
		s = s[1:]
		j := strings.IndexAny(s, "+-")
		if j < 0 {
			j = len(s)
		}
		t := s[:j]
		s = s[j:]
		if m := reTermine.FindStringSubmatch(t); m != nil {
			n := 1
			if m[1] != "" {
				n, _ = strconv.Atoi(m[1])
			}
			fc, _ := strconv.Atoi(m[2])
			if !facceFormula[fc] || n < 1 || n > 20 {
				return nil, fmt.Errorf("«%s»: dadi ammessi da 1 a 20 per tipo, d4, d6, d8, d10, d12, d20 o d100", t)
			}
			dadi += n
			out = append(out, termineFormula{Segno: segno, N: n, Facce: fc})
		} else if v, err := strconv.Atoi(t); err == nil && v >= 0 && v <= 999 {
			out = append(out, termineFormula{Segno: segno, N: v})
		} else {
			return nil, fmt.Errorf("«%s» non è un dado (es. 2d6) né un numero", t)
		}
	}
	if dadi == 0 {
		return nil, fmt.Errorf("la formula non ha dadi")
	}
	if dadi > 50 {
		return nil, fmt.Errorf("al massimo 50 dadi")
	}
	return out, nil
}

// controllaFormula: formula del modello, con i riferimenti ai campi numerici (sostituiti da 1 per il controllo, così
// «{forza}d6», tanti dadi quanto vale il campo, è valida).
func controllaFormula(f string, numerici map[string]bool) error {
	for _, m := range reRifCampo.FindAllStringSubmatch(f, -1) {
		if !numerici[m[1]] {
			return fmt.Errorf("«{%s}» non è un campo numerico del modello", m[1])
		}
	}
	_, err := leggiFormula(reRifCampo.ReplaceAllString(f, "1"))
	return err
}

// validaModello controlla un modello e lo restituisce in JSON compatto.
func validaModello(raw []byte) ([]byte, error) {
	var m modelloScheda
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return nil, fmt.Errorf("modello non valido: %v", erroreJSONLeggibile(err))
	}
	m.Versione = 1
	if len(m.Sezioni) == 0 || len(m.Sezioni) > 20 {
		return nil, fmt.Errorf("il modello deve avere da 1 a 20 sezioni")
	}
	ids, numerici, risorse := map[string]bool{}, map[string]bool{}, map[string]bool{}
	tot := 0
	for i, sz := range m.Sezioni {
		if strings.TrimSpace(sz.Titolo) == "" || utf8.RuneCountInString(sz.Titolo) > 60 {
			return nil, fmt.Errorf("sezione n. %d: serve un titolo (al massimo 60 caratteri)", i+1)
		}
		for j, c := range sz.Campi {
			tot++
			dove := fmt.Sprintf("sezione «%s», campo n. %d", sz.Titolo, j+1)
			if !reIDCampo.MatchString(c.ID) {
				return nil, fmt.Errorf("%s: l'identificativo deve essere fatto di lettere minuscole senza accenti, cifre e _ (es. «forza»)", dove)
			}
			if ids[c.ID] {
				return nil, fmt.Errorf("%s: l'identificativo «%s» è già usato", dove, c.ID)
			}
			ids[c.ID] = true
			if strings.TrimSpace(c.Nome) == "" || utf8.RuneCountInString(c.Nome) > 60 || utf8.RuneCountInString(c.Aiuto) > 300 {
				return nil, fmt.Errorf("%s: serve un nome (al massimo 60 caratteri)", dove)
			}
			if !tipiCampo[c.Tipo] {
				return nil, fmt.Errorf("%s: tipo «%s» sconosciuto (testo, area, numero, risorsa, spunta, elenco)", dove, c.Tipo)
			}
			if c.Tipo == "numero" || c.Tipo == "risorsa" {
				numerici[c.ID] = true
			}
			if c.Tipo == "risorsa" {
				risorse[c.ID] = true
			}
			if c.Min != nil && c.Max != nil && *c.Min > *c.Max {
				return nil, fmt.Errorf("%s: il minimo supera il massimo", dove)
			}
		}
	}
	if tot == 0 || tot > 200 {
		return nil, fmt.Errorf("il modello deve avere da 1 a 200 campi")
	}
	for _, sz := range m.Sezioni {
		for _, c := range sz.Campi {
			if c.Tiro != "" {
				if err := controllaFormula(c.Tiro, numerici); err != nil {
					return nil, fmt.Errorf("tiro del campo «%s»: %v", c.Nome, err)
				}
			}
		}
	}
	if len(m.Tiri) > 60 {
		return nil, fmt.Errorf("al massimo 60 tiri")
	}
	for i, t := range m.Tiri {
		if strings.TrimSpace(t.Nome) == "" || utf8.RuneCountInString(t.Nome) > 40 {
			return nil, fmt.Errorf("tiro n. %d: serve un nome (al massimo 40 caratteri)", i+1)
		}
		if err := controllaFormula(t.Formula, numerici); err != nil {
			return nil, fmt.Errorf("tiro «%s»: %v", t.Nome, err)
		}
	}
	if m.PF != "" && !numerici[m.PF] {
		return nil, fmt.Errorf("i punti ferita devono essere un campo «risorsa» o «numero» del modello")
	}
	if m.Difesa != "" && !numerici[m.Difesa] {
		return nil, fmt.Errorf("la difesa deve essere un campo «numero» del modello")
	}
	if m.Iniziativa != "" {
		if err := controllaFormula(m.Iniziativa, numerici); err != nil {
			return nil, fmt.Errorf("iniziativa: %v", err)
		}
	}
	return json.Marshal(m)
}

// modelloDellaCampagna: il modello salvato oppure quello di base.
func (s *Server) modelloDellaCampagna(id string) json.RawMessage {
	var m string
	s.DB.QueryRow(`SELECT modello_scheda FROM campagne WHERE id = ?`, id).Scan(&m)
	if m == "" {
		b, _ := json.Marshal(modelloBase())
		return b
	}
	return json.RawMessage(m)
}

// giocoDelPersonaggio: campagna e modulo del gruppo del personaggio; senza gruppo, la campagna attiva.
func (s *Server) giocoDelPersonaggio(pg string) (campagna, modulo string) {
	s.DB.QueryRow(`SELECT c.id, c.modulo FROM personaggi p JOIN gruppi g ON g.id = p.gruppo_id JOIN campagne c ON c.id = g.campagna_id
		WHERE p.id = ?`, pg).Scan(&campagna, &modulo)
	if campagna == "" {
		campagna, _ = s.campagnaAttiva()
		s.DB.QueryRow(`SELECT modulo FROM campagne WHERE id = ?`, campagna).Scan(&modulo)
	}
	return
}

func (s *Server) rotteSchedaLibera(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/campagne/{id}/modello-scheda", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "master"); err != nil {
			return nil, err
		}
		if err := s.campagnaEsiste(r.PathValue("id")); err != nil {
			return nil, err
		}
		var m string
		s.DB.QueryRow(`SELECT modello_scheda FROM campagne WHERE id = ?`, r.PathValue("id")).Scan(&m)
		return map[string]any{"modello": s.modelloDellaCampagna(r.PathValue("id")), "personalizzato": m != ""}, nil
	}))
	mux.Handle("PUT /api/v1/campagne/{id}/modello-scheda", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		u, err := richiedi(r, "master")
		if err != nil {
			return nil, err
		}
		if err := s.campagnaEsiste(r.PathValue("id")); err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, 256<<10))
		if err != nil {
			return nil, errDatiNonValidi
		}
		salva := ""
		if strings.TrimSpace(string(b)) != "null" { // null = torna al modello di base
			v, err := validaModello(b)
			if err != nil {
				return nil, errore(http.StatusBadRequest, "Il modello ha un errore: "+err.Error()+".")
			}
			salva = string(v)
		}
		s.DB.Exec(`UPDATE campagne SET modello_scheda = ? WHERE id = ?`, salva, r.PathValue("id"))
		s.evento("modello_scheda", u.ID, r.PathValue("id"))
		s.Hub.ATutti(Msg{Tipo: "modello.scheda", Dati: map[string]any{"campagna": r.PathValue("id")}})
		return map[string]any{"modello": s.modelloDellaCampagna(r.PathValue("id")), "personalizzato": salva != ""}, nil
	}))
	// Il modello della scheda di un personaggio (quello della campagna del suo gruppo): lo legge la scheda libera.
	mux.Handle("GET /api/v1/personaggi/{id}/modello-scheda", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		u, err := richiedi(r, "approvato")
		if err != nil {
			return nil, err
		}
		if !s.puoLeggerePersonaggio(u, r.PathValue("id")) {
			return nil, errNonTrovato
		}
		c, m := s.giocoDelPersonaggio(r.PathValue("id"))
		nome := m
		if x := s.moduloGioco(m); x != nil {
			nome = x.Nome
		}
		return map[string]any{"campagna": c, "modulo": m, "nomeModulo": nome, "modello": s.modelloDellaCampagna(c)}, nil
	}))
}

// tiroIniziativa: con la formula della scheda libera (es. «1d10 + 3», già con i valori) oppure d20 + bonus.
func tiroIniziativa(formula string, bonus int) (v int, nat any, testo string) {
	if formula != "" {
		if t, err := leggiFormula(formula); err == nil {
			parti := []string{}
			for _, x := range t {
				if x.Facce == 0 {
					v += x.Segno * x.N
					continue
				}
				val := []string{}
				for i := 0; i < x.N; i++ {
					d := dado(x.Facce)
					v += x.Segno * d
					val = append(val, itoa(d))
				}
				parti = append(parti, itoa(x.N)+"d"+itoa(x.Facce)+" ["+strings.Join(val, ", ")+"]")
			}
			return v, nil, " (" + formula + ": " + strings.Join(parti, " ") + ")"
		}
	}
	d20 := dado(20)
	return d20 + bonus, d20, " (d20 " + itoa(d20) + ")" + testoNaturale(d20)
}
