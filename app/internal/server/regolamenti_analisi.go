package server

// Analisi dei regolamenti (docs/RULEBOOK_LIBRARY.md, passo L5). Architettura modulare e senza IA obbligatoria:
//   - un «analizzatore» legge il testo di un documento di MY RULEBOOKS (le parti con capitolo e pagina) e restituisce
//     PROPOSTE nelle 13 categorie, ognuna con il riferimento alla fonte;
//   - nella prima versione c'è l'analizzatore «parole_chiave» (titoli e capitoli riconosciuti da parole in italiano e
//     in inglese); quello con l'IA è predisposto ma non attivo;
//   - il Master può scrivere proposte a mano;
//   - nessuna regola viene applicata: ogni proposta resta «proposta» finché il Master non la verifica e la approva (o la
//     rifiuta). Solo le approvate escono da GET /campagne/{id}/regole, che userà il motore.
// Tabella regole_proposte (migrazione 040). Solo il Master.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"gasterbless/internal/cripto"
)

// categorieRegole: le 13 categorie dell'analisi (ordine di presentazione).
var categorieRegole = []struct{ ID, Nome string }{
	{"attributi", "Attributi e caratteristiche"}, {"abilita", "Abilità e competenze"},
	{"classi", "Classi, archetipi e professioni"}, {"origini", "Razze, stirpi e origini"},
	{"equipaggiamento", "Equipaggiamento"}, {"armi_armature", "Armi e armature"}, {"magie", "Magie e capacità"},
	{"combattimento", "Regole di combattimento"}, {"iniziativa", "Iniziativa"}, {"condizioni", "Condizioni ed effetti"},
	{"progressione", "Progressione dei personaggi"}, {"creazione", "Creazione dei personaggi"}, {"speciali", "Regole speciali"},
}

func categoriaValida(c string) bool {
	for _, x := range categorieRegole {
		if x.ID == c {
			return true
		}
	}
	return false
}

// propostaRegola: ciò che un analizzatore restituisce.
type propostaRegola struct {
	Categoria string
	Nome      string
	Dove      string
	Pagina    int
	Parte     int
	Estratto  string
}

// analizzatoreRegole: un modo di leggere un documento. Disponibile dice se si può usare adesso (per esempio l'IA solo
// quando è configurata e attivata).
type analizzatoreRegole interface {
	ID() string
	Nome() string
	Disponibile(s *Server) (bool, string)
	Analizza(s *Server, d docMio, parti []parteTesto) ([]propostaRegola, error)
}

var analizzatori = []analizzatoreRegole{analizzatoreParole{}, analizzatoreIA{}}

func trovaAnalizzatore(id string) analizzatoreRegole {
	for _, a := range analizzatori {
		if a.ID() == id {
			return a
		}
	}
	return nil
}

// ----- analizzatore per parole chiave (senza IA) -----

type analizzatoreParole struct{}

func (analizzatoreParole) ID() string                         { return "parole_chiave" }
func (analizzatoreParole) Nome() string                       { return "Parole chiave (senza IA)" }
func (analizzatoreParole) Disponibile(*Server) (bool, string) { return true, "" }

// paroleCategorie: radici (senza accenti, minuscole) che, in un titolo, indicano la categoria. Italiano e inglese.
var paroleCategorie = map[string][]string{
	"creazione":       {"creazione del personaggio", "creare un personaggio", "creare il personaggio", "character creation", "creating a character"},
	"iniziativa":      {"iniziativa", "initiative"},
	"condizioni":      {"condizioni", "condizione", "conditions", "status effects", "stati alterati"},
	"armi_armature":   {"armi", "armatur", "weapons", "armor", "armour", "scudi"},
	"equipaggiamento": {"equipaggiamento", "attrezzatura", "equipment", "gear", "inventario"},
	"magie":           {"incantesim", "magia", "magie", "spells", "spellcasting", "poteri", "psionic", "rituali"},
	"attributi":       {"caratteristiche", "attributi", "ability scores", "attributes", "statistiche"},
	"abilita":         {"abilita", "competenze", "skills", "proficienc"},
	"classi":          {"classi", "classe", "archetip", "professioni", "mestieri", "classes", "careers", "archetypes"},
	"origini":         {"razze", "razza", "stirpi", "stirpe", "origini", "specie", "ancestr", "species", "races", "lineage", "popoli"},
	"progressione":    {"progressione", "livelli", "avanzamento", "esperienza", "level up", "advancement", "experience", "leveling"},
	"combattimento":   {"combattimento", "combat", "scontro", "attacchi", "danni", "damage"},
	"speciali":        {"regole speciali", "regole opzionali", "regole facoltative", "optional rules", "variant rules", "varianti"},
}

// ordine dei controlli: le espressioni più specifiche prima
var ordineCategorie = []string{"creazione", "iniziativa", "condizioni", "armi_armature", "equipaggiamento", "magie", "attributi",
	"abilita", "classi", "origini", "progressione", "speciali", "combattimento"}

// pareTitoloRegola: un paragrafo breve, senza punto finale, che può essere il titolo di una sezione.
func pareTitoloRegola(par string) bool {
	n := utf8.RuneCountInString(par)
	return n >= 3 && n <= 80 && !strings.HasSuffix(par, ".") && !strings.Contains(par, "\n")
}

func categoriaDiTitolo(t string) string {
	n := normalizza(t)
	if n == "" {
		return ""
	}
	for _, c := range ordineCategorie {
		for _, w := range paroleCategorie[c] {
			if strings.Contains(n, w) {
				return c
			}
		}
	}
	return ""
}

func (analizzatoreParole) Analizza(_ *Server, _ docMio, parti []parteTesto) ([]propostaRegola, error) {
	out := []propostaRegola{}
	visti := map[string]bool{}
	aggiungi := func(titolo string, p parteTesto, i int, estratto string) {
		c := categoriaDiTitolo(titolo)
		if c == "" || len(out) >= 300 {
			return
		}
		chiave := c + "|" + normalizza(titolo)
		if visti[chiave] {
			return
		}
		visti[chiave] = true
		out = append(out, propostaRegola{Categoria: c, Nome: accorcia(titolo, 120), Dove: p.E, Pagina: p.P, Parte: i, Estratto: accorcia(estratto, 400)})
	}
	for i, p := range parti {
		paragrafi := strings.Split(strings.TrimSpace(p.T), "\n\n")
		for k, par := range paragrafi {
			par = strings.TrimSpace(par)
			if !pareTitoloRegola(par) {
				continue
			}
			seguito := strings.TrimSpace(strings.Join(paragrafi[k+1:], "\n\n"))
			if seguito == "" {
				continue
			}
			aggiungi(par, p, i, seguito)
		}
		// il capitolo stesso (es. «Capitolo 3: Combattimento»)
		aggiungi(p.E, p, i, strings.TrimSpace(p.T))
	}
	return out, nil
}

// ----- analizzatore con l'IA: predisposto, non attivo nella prima versione -----

type analizzatoreIA struct{}

func (analizzatoreIA) ID() string   { return "ia" }
func (analizzatoreIA) Nome() string { return "Assistente IA (facoltativo)" }
func (analizzatoreIA) Disponibile(*Server) (bool, string) {
	return false, "Non ancora disponibile: arriverà in una versione successiva e userà l'assistente IA già configurato in Impostazioni IA."
}
func (analizzatoreIA) Analizza(*Server, docMio, []parteTesto) ([]propostaRegola, error) {
	return nil, errors.New("analizzatore con l'IA non disponibile")
}

// ----- API -----

func (s *Server) rotteAnalisiRegole(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/regolamenti/analisi", api(s.apiProposteRegole))
	mux.Handle("POST /api/v1/regolamenti/analisi/avvia", api(s.apiAvviaAnalisi))
	mux.Handle("POST /api/v1/regolamenti/analisi", api(s.apiNuovaProposta))
	mux.Handle("PATCH /api/v1/regolamenti/analisi/{id}", api(s.apiModificaProposta))
	mux.Handle("DELETE /api/v1/regolamenti/analisi/{id}", api(s.apiEliminaProposta))
	mux.Handle("GET /api/v1/campagne/{id}/regole", api(s.apiRegoleApprovate))
}

type vistaProposta struct {
	ID              string `json:"id"`
	Campagna        string `json:"campagna,omitempty"`
	Documento       string `json:"documento,omitempty"`
	DocumentoTitolo string `json:"documentoTitolo"`
	Pagina          int    `json:"pagina,omitempty"`
	Parte           int    `json:"parte"`
	Dove            string `json:"dove,omitempty"`
	Categoria       string `json:"categoria"`
	Nome            string `json:"nome"`
	Testo           string `json:"testo"`
	Estratto        string `json:"estratto,omitempty"`
	Origine         string `json:"origine"`
	Stato           string `json:"stato"`
	Nota            string `json:"nota,omitempty"`
	CreatoIl        int64  `json:"creatoIl"`
	DecisoIl        int64  `json:"decisoIl,omitempty"`
}

const colonneProposta = `id, COALESCE(campagna_id, ''), COALESCE(documento_id, ''), documento_titolo, pagina, parte, dove, categoria, nome,
	testo, estratto, origine, stato, nota, creato_il, deciso_il`

func scanProposta(sc interface{ Scan(...any) error }) (vistaProposta, error) {
	var p vistaProposta
	err := sc.Scan(&p.ID, &p.Campagna, &p.Documento, &p.DocumentoTitolo, &p.Pagina, &p.Parte, &p.Dove, &p.Categoria, &p.Nome,
		&p.Testo, &p.Estratto, &p.Origine, &p.Stato, &p.Nota, &p.CreatoIl, &p.DecisoIl)
	return p, err
}

func (s *Server) elencoProposte(where string, arg ...any) ([]vistaProposta, error) {
	rows, err := s.DB.Query(`SELECT `+colonneProposta+` FROM regole_proposte `+where+` ORDER BY categoria, documento_titolo, pagina, parte, creato_il`, arg...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []vistaProposta{}
	for rows.Next() {
		p, err := scanProposta(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// apiProposteRegole: filtri facoltativi documento, campagna, stato, categoria. Con le categorie e gli analizzatori.
func (s *Server) apiProposteRegole(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	q := r.URL.Query()
	cond, arg := []string{}, []any{}
	for _, f := range []struct{ par, col string }{{"documento", "documento_id"}, {"campagna", "campagna_id"}, {"stato", "stato"}, {"categoria", "categoria"}} {
		if v := q.Get(f.par); v != "" {
			cond = append(cond, f.col+" = ?")
			arg = append(arg, v)
		}
	}
	where := ""
	if len(cond) > 0 {
		where = "WHERE " + strings.Join(cond, " AND ")
	}
	p, err := s.elencoProposte(where, arg...)
	if err != nil {
		return nil, err
	}
	an := []map[string]any{}
	for _, a := range analizzatori {
		ok, perche := a.Disponibile(s)
		an = append(an, map[string]any{"id": a.ID(), "nome": a.Nome(), "disponibile": ok, "motivo": perche})
	}
	conta := map[string]int{}
	if rows, err := s.DB.Query(`SELECT stato, COUNT(*) FROM regole_proposte GROUP BY stato`); err == nil {
		for rows.Next() {
			var st string
			var n int
			rows.Scan(&st, &n)
			conta[st] = n
		}
		rows.Close()
	}
	return map[string]any{"proposte": p, "categorie": categorieRegole, "analizzatori": an, "conteggi": conta}, nil
}

// apiAvviaAnalisi: {"documento": id, "analizzatore": "parole_chiave", "campagna": id facoltativo}. Le proposte già
// presenti per lo stesso documento, categoria e titolo non si ripetono. Niente viene approvato.
func (s *Server) apiAvviaAnalisi(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Documento    string `json:"documento"`
		Analizzatore string `json:"analizzatore"`
		Campagna     string `json:"campagna"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
		return nil, errDatiNonValidi
	}
	d, err := s.docMio(in.Documento)
	if err != nil {
		return nil, err
	}
	if in.Campagna != "" {
		if err := s.campagnaEsiste(in.Campagna); err != nil {
			return nil, err
		}
	}
	if in.Analizzatore == "" {
		in.Analizzatore = "parole_chiave"
	}
	a := trovaAnalizzatore(in.Analizzatore)
	if a == nil {
		return nil, errore(http.StatusBadRequest, "Analizzatore sconosciuto.")
	}
	if ok, perche := a.Disponibile(s); !ok {
		return nil, errore(http.StatusConflict, perche)
	}
	parti := s.testoRegolamento(d.ID)
	if len(parti) == 0 {
		return nil, errore(http.StatusConflict, "Il documento non ha ancora il testo per la ricerca: per i PDF usa «Prepara la ricerca» in My Rulebooks.")
	}
	prop, err := a.Analizza(s, d, parti)
	if err != nil {
		return nil, errore(http.StatusBadGateway, "Analisi non riuscita: "+err.Error())
	}
	nuove, gia := 0, 0
	ora := time.Now().Unix()
	for _, p := range prop {
		if !categoriaValida(p.Categoria) {
			continue
		}
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM regole_proposte WHERE documento_id = ? AND categoria = ? AND nome = ? AND parte = ?`, d.ID, p.Categoria, p.Nome, p.Parte).Scan(&n)
		if n > 0 {
			gia++
			continue
		}
		_, err := s.DB.Exec(`INSERT INTO regole_proposte (id, campagna_id, documento_id, documento_titolo, pagina, parte, dove, categoria, nome, estratto, origine, creato_il)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, cripto.ID(), nulla(in.Campagna), d.ID, d.Titolo, p.Pagina, p.Parte, p.Dove, p.Categoria, p.Nome, p.Estratto, a.ID(), ora)
		if err != nil {
			return nil, err
		}
		nuove++
	}
	s.evento("regole_analisi", u.ID, d.Titolo)
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.analisi", Dati: map[string]any{}})
	return map[string]any{"nuove": nuove, "giaPresenti": gia, "analizzatore": a.ID()}, nil
}

type datiProposta struct {
	Documento *string `json:"documento"`
	Campagna  *string `json:"campagna"`
	Pagina    *int    `json:"pagina"`
	Dove      *string `json:"dove"`
	Categoria *string `json:"categoria"`
	Nome      *string `json:"nome"`
	Testo     *string `json:"testo"`
	Stato     *string `json:"stato"`
	Nota      *string `json:"nota"`
}

func (s *Server) controllaProposta(d datiProposta) error {
	if d.Categoria != nil && !categoriaValida(*d.Categoria) {
		return errore(http.StatusBadRequest, "Categoria sconosciuta.")
	}
	if d.Nome != nil && (strings.TrimSpace(*d.Nome) == "" || utf8.RuneCountInString(*d.Nome) > 200) {
		return errore(http.StatusBadRequest, "Il nome della regola va da 1 a 200 caratteri.")
	}
	if d.Testo != nil && utf8.RuneCountInString(*d.Testo) > 20000 || d.Nota != nil && utf8.RuneCountInString(*d.Nota) > 2000 ||
		d.Dove != nil && utf8.RuneCountInString(*d.Dove) > 200 {
		return errore(http.StatusBadRequest, "Un campo è troppo lungo.")
	}
	if d.Stato != nil && *d.Stato != "proposta" && *d.Stato != "approvata" && *d.Stato != "rifiutata" {
		return errore(http.StatusBadRequest, "Stato sconosciuto.")
	}
	if d.Pagina != nil && (*d.Pagina < 0 || *d.Pagina > 100000) {
		return errore(http.StatusBadRequest, "Pagina non valida.")
	}
	if d.Campagna != nil && *d.Campagna != "" {
		if err := s.campagnaEsiste(*d.Campagna); err != nil {
			return err
		}
	}
	return nil
}

// apiNuovaProposta: proposta scritta dal Master (stato «proposta»: la approva lui dopo averla riletta).
func (s *Server) apiNuovaProposta(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var d datiProposta
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&d); err != nil {
		return nil, errDatiNonValidi
	}
	if d.Categoria == nil || d.Nome == nil {
		return nil, errore(http.StatusBadRequest, "Servono categoria e nome della regola.")
	}
	if err := s.controllaProposta(d); err != nil {
		return nil, err
	}
	titolo := ""
	if d.Documento != nil && *d.Documento != "" {
		doc, err := s.docMio(*d.Documento)
		if err != nil {
			return nil, err
		}
		titolo = doc.Titolo
	}
	pag := 0
	if d.Pagina != nil {
		pag = *d.Pagina
	}
	id := cripto.ID()
	_, err := s.DB.Exec(`INSERT INTO regole_proposte (id, campagna_id, documento_id, documento_titolo, pagina, dove, categoria, nome, testo, origine, creato_il)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'manuale', ?)`, id, nulla(valore(d.Campagna, "")), nulla(valore(d.Documento, "")), titolo, pag,
		valore(d.Dove, ""), *d.Categoria, strings.TrimSpace(*d.Nome), valore(d.Testo, ""), time.Now().Unix())
	if err != nil {
		return nil, err
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.analisi", Dati: map[string]any{}})
	p, err := s.elencoProposte(`WHERE id = ?`, id)
	if err != nil || len(p) == 0 {
		return nil, err
	}
	return p[0], nil
}

// apiModificaProposta: correzione dei campi e decisione del Master (approvata / rifiutata / di nuovo proposta).
func (s *Server) apiModificaProposta(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	vecchie, err := s.elencoProposte(`WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(vecchie) == 0 {
		return nil, errore(http.StatusNotFound, "Proposta non trovata.")
	}
	v := vecchie[0]
	var d datiProposta
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&d); err != nil {
		return nil, errDatiNonValidi
	}
	if err := s.controllaProposta(d); err != nil {
		return nil, err
	}
	stato, deciso := v.Stato, v.DecisoIl
	if d.Stato != nil && *d.Stato != v.Stato {
		stato, deciso = *d.Stato, time.Now().Unix()
		if stato == "proposta" {
			deciso = 0
		}
	}
	pag := v.Pagina
	if d.Pagina != nil {
		pag = *d.Pagina
	}
	nome := v.Nome
	if d.Nome != nil {
		nome = strings.TrimSpace(*d.Nome)
	}
	_, err = s.DB.Exec(`UPDATE regole_proposte SET campagna_id = ?, pagina = ?, dove = ?, categoria = ?, nome = ?, testo = ?, stato = ?, nota = ?, deciso_il = ?
		WHERE id = ?`, nulla(valore(d.Campagna, v.Campagna)), pag, valore(d.Dove, v.Dove), valore(d.Categoria, v.Categoria), nome,
		valore(d.Testo, v.Testo), stato, valore(d.Nota, v.Nota), deciso, id)
	if err != nil {
		return nil, err
	}
	if stato != v.Stato {
		s.evento("regola_"+stato, u.ID, nome)
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.analisi", Dati: map[string]any{}})
	p, _ := s.elencoProposte(`WHERE id = ?`, id)
	return p[0], nil
}

func (s *Server) apiEliminaProposta(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`DELETE FROM regole_proposte WHERE id = ?`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errore(http.StatusNotFound, "Proposta non trovata.")
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.analisi", Dati: map[string]any{}})
	return map[string]any{"eliminata": r.PathValue("id")}, nil
}

// apiRegoleApprovate: le sole regole APPROVATE di una campagna (quelle della campagna e quelle senza campagna dei
// documenti che la campagna usa), con la fonte. È l'unico punto da cui il motore potrà leggerle.
func (s *Server) apiRegoleApprovate(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if err := s.campagnaEsiste(id); err != nil {
		return nil, err
	}
	p, err := s.elencoProposte(`WHERE stato = 'approvata' AND (campagna_id = ? OR (campagna_id IS NULL AND documento_id IN
		(SELECT regolamento_id FROM campagne WHERE id = ? UNION SELECT documento_id FROM campagna_documenti WHERE campagna_id = ?)))`, id, id, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"regole": p, "categorie": categorieRegole}, nil
}
