package server

// Servizio di Sanità mentale, Terrore e pazzia: impostazioni della campagna, stato dei personaggi,
// eventi di paura, risoluzione autoritativa del tiro (una sola, salvata prima dell'invio in tempo
// reale), condizioni applicate dal Terrore, pazzie, catalogo dei testi, immagine e registro.
// Le regole stanno in sanita_regole.go.

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

// ===== Impostazioni della campagna =====

type ConfigSanita struct {
	Attiva                bool   `json:"attiva"`                // Sanità mentale come settima caratteristica
	Metodo                string `json:"metodo"`                // DMG_STANDARD (ufficiale) o WISDOM_DERIVED (homebrew)
	Terrore               bool   `json:"terrore"`               // scala homebrew 0–7
	Pazzia                bool   `json:"pazzia"`                // gestione delle pazzie
	CompetenzaPredefinita bool   `json:"competenzaPredefinita"` // competenza nel TS per i nuovi personaggi
	FallbackSaggezza      bool   `json:"fallbackSaggezza"`      // creature senza Sanità: Saggezza (ufficiale)
	RegolaResistenza      bool   `json:"regolaResistenza"`      // homebrew: immuni a Spaventato
	RegolaSpaventato      bool   `json:"regolaSpaventato"`      // homebrew: Spaventato soprannaturale → TS, +3 Terrore
	RegolaRimozione       bool   `json:"regolaRimozione"`       // homebrew: togliere Spaventato toglie 3 Terrore
	AumentaCDPrecedente   bool   `json:"aumentaCdPrecedente"`   // homebrew: nuova pazzia → +1 alla CD della precedente
	PazziaCronica         bool   `json:"pazziaCronica"`         // homebrew, disattivata
	Tabelle               string `json:"tabelle"`               // homebrew o dmg
	CDIntensita           [6]int `json:"cdIntensita"`
	TerroreIntensita      [6]int `json:"terroreIntensita"`
	CDBarcollante         int    `json:"cdBarcollante"`
	CDSvenuto             int    `json:"cdSvenuto"`
	Versione              int    `json:"versione"`
}

func configPredefinita() ConfigSanita {
	// Regola della campagna (decisa dall'utente il 2026-11-06): Sanità mentale = 10 + modificatore di Saggezza.
	// Il metodo ufficiale DMG (settima caratteristica generata) resta selezionabile nelle Impostazioni.
	return ConfigSanita{Attiva: true, Metodo: MetodoSaggezza, Pazzia: true, FallbackSaggezza: true, AumentaCDPrecedente: true,
		Tabelle: "homebrew", CDIntensita: CDIntensita, TerroreIntensita: TerroreIntensita, CDBarcollante: 15, CDSvenuto: 15, Versione: 1}
}

func (s *Server) configSanita() ConfigSanita {
	c := configPredefinita()
	if v, err := s.impostazione("sanita_config"); err == nil && v != "" {
		json.Unmarshal([]byte(v), &c)
	}
	// Dal 2026-11-06 la Sanità mentale è una funzione separata dalla scheda: il valore è sempre
	// 10 + modificatore di Saggezza (o quello scritto a mano dal Master). Il metodo DMG non c'è più.
	c.Metodo = MetodoSaggezza
	return c
}

func validaConfig(c ConfigSanita) error {
	if c.Metodo != MetodoDMG && c.Metodo != MetodoSaggezza {
		return errore(http.StatusBadRequest, "Metodo del punteggio non valido.")
	}
	if c.Tabelle != "homebrew" && c.Tabelle != "dmg" {
		return errore(http.StatusBadRequest, "Tabelle di pazzia non valide.")
	}
	for i := range 6 {
		if c.CDIntensita[i] < 1 || c.CDIntensita[i] > 40 || c.TerroreIntensita[i] < 0 || c.TerroreIntensita[i] > 7 {
			return errore(http.StatusBadRequest, "CD (1–40) o Terrore (0–7) delle intensità non validi.")
		}
	}
	if c.CDBarcollante < 1 || c.CDBarcollante > 40 || c.CDSvenuto < 1 || c.CDSvenuto > 40 {
		return errore(http.StatusBadRequest, "CD non valida.")
	}
	return nil
}

// ===== Stato del personaggio =====

type statoSanita struct {
	competente, immuneTerrore, immuneSpaventato bool
	bonus, terrore                              int
	manuale                                     *int
	condTerrore                                 []string
}

func (s *Server) leggiSanitaPG(pg string) statoSanita {
	st := statoSanita{competente: s.configSanita().CompetenzaPredefinita}
	var comp, it, is int
	var man sql.NullInt64
	var cond string
	if s.DB.QueryRow(`SELECT competente, bonus_temp, punteggio_manuale, terrore, immune_terrore, immune_spaventato, condizioni_terrore FROM sanita_pg WHERE personaggio_id = ?`, pg).
		Scan(&comp, &st.bonus, &man, &st.terrore, &it, &is, &cond) == nil {
		st.competente, st.immuneTerrore, st.immuneSpaventato = comp == 1, it == 1, is == 1
		if man.Valid {
			n := int(man.Int64)
			st.manuale = &n
		}
		json.Unmarshal([]byte(cond), &st.condTerrore)
	}
	return st
}

// assicuraSanitaPG crea la riga con i valori predefiniti, se manca.
func (s *Server) assicuraSanitaPG(q interface {
	Exec(string, ...any) (sql.Result, error)
}, pg string) {
	comp := 0
	if s.configSanita().CompetenzaPredefinita {
		comp = 1
	}
	q.Exec(`INSERT OR IGNORE INTO sanita_pg(personaggio_id, competente, modificato_il) VALUES(?,?,?)`, pg, comp, db.Ora())
}

// punteggioSanita: inserito a mano dal Master, altrimenti 10 + modificatore di Saggezza.
// La Saggezza arriva dal riepilogo di qualsiasi scheda (quella di Gaster Bless o un plugin): la scheda non
// contiene più nulla sulla Sanità mentale.
func (s *Server) punteggioSanita(pg string, cfg ConfigSanita, st statoSanita, r Riepilogo) (*int, string) {
	if st.manuale != nil {
		return st.manuale, MetodoManuale
	}
	if v, ok := r.Car["SAG"]; ok {
		n := PunteggioDaSaggezza(v)
		return &n, MetodoSaggezza
	}
	return nil, MetodoSaggezza
}

func (s *Server) registraSanita(pg string, u *Utente, azione string, dati any) {
	b, _ := json.Marshal(dati)
	attore := ""
	if u != nil {
		attore = u.ID
	}
	s.DB.Exec(`INSERT INTO registro_sanita(personaggio_id, attore_id, azione, dati, creato_il) VALUES(?,?,?,?,?)`, pg, attore, azione, string(b), db.Ora())
}

// notificaSanita: proprietario e Master rileggono lo stato (tutto è già salvato).
func (s *Server) notificaSanita(pg, motivo string) {
	var prop string
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, pg).Scan(&prop)
	m := Msg{Tipo: "sanita.stato", Dati: map[string]string{"personaggio": pg, "motivo": motivo}}
	if prop != "" {
		s.Hub.AUtente(prop, m)
	}
	s.Hub.AlMaster(m)
}

// impostaTerrore cambia il Terrore (0–7), registra prima/dopo/motivo e aggiorna le condizioni.
func (s *Server) impostaTerrore(pg string, u *Utente, nuovo int, azione, motivo string, extra map[string]any) (prima, dopo int, err error) {
	prima, dopo, err = s.impostaTerroreBase(pg, u, nuovo, azione, motivo, extra)
	if err == nil && dopo > prima {
		s.pazzieAutomatiche(pg, u, "", prima, dopo, false) // soglie 4 e 6 (sanita_automatiche.go)
	}
	return
}

// impostaTerroreBase cambia il Terrore senza le follie automatiche (chi la usa le assegna da sé).
func (s *Server) impostaTerroreBase(pg string, u *Utente, nuovo int, azione, motivo string, extra map[string]any) (prima, dopo int, err error) {
	s.assicuraSanitaPG(s.DB, pg)
	st := s.leggiSanitaPG(pg)
	prima, dopo = st.terrore, LimitaTerrore(nuovo)
	if _, err = s.DB.Exec(`UPDATE sanita_pg SET terrore = ?, modificato_il = ? WHERE personaggio_id = ?`, dopo, db.Ora(), pg); err != nil {
		return
	}
	d := map[string]any{"prima": prima, "dopo": dopo, "motivo": motivo}
	for k, v := range extra {
		d[k] = v
	}
	s.registraSanita(pg, u, azione, d)
	s.applicaCondizioniTerrore(pg, u, dopo)
	return
}

// applicaCondizioniTerrore: Paralizzato da 6, Privo di sensi a 7, nello stato in gioco del PG (lo stesso
// usato da scheda e tracker). Si tolgono solo le condizioni messe dal Terrore.
func (s *Server) applicaCondizioniTerrore(pg string, u *Utente, livello int) {
	cfg, st := s.configSanita(), s.leggiSanitaPG(pg)
	var volute []string
	if cfg.Terrore {
		volute = CondizioniTerrore(livello)
	}
	var attuali []string
	json.Unmarshal(s.statoInGioco(pg).Condizioni, &attuali)
	nuove := []string{}
	for _, c := range attuali {
		if slices.Contains(st.condTerrore, c) && !slices.Contains(volute, c) {
			continue // messa dal Terrore e non più dovuta
		}
		nuove = append(nuove, c)
	}
	aggiunte := []string{}
	for _, c := range volute {
		if !slices.Contains(nuove, c) {
			nuove = append(nuove, c)
			aggiunte = append(aggiunte, c)
		} else if slices.Contains(st.condTerrore, c) {
			aggiunte = append(aggiunte, c)
		}
	}
	b, _ := json.Marshal(aggiunte)
	s.DB.Exec(`UPDATE sanita_pg SET condizioni_terrore = ? WHERE personaggio_id = ?`, string(b), pg)
	if !slices.Equal(nuove, attuali) {
		s.applicaCondizioniSanita(pg, nuove)
	}
}

// ===== Viste =====

type vistaPazzia struct {
	ID            string `json:"id"`
	Tipo          string `json:"tipo"`
	Categoria     string `json:"categoria"`
	Descrizione   string `json:"descrizione"`
	Tabella       string `json:"tabella"`
	Durata        string `json:"durata"`
	ScadeIl       int64  `json:"scadeIl,omitempty"`
	CDRecupero    int    `json:"cdRecupero"`
	Attiva        bool   `json:"attiva"`
	Evento        string `json:"evento,omitempty"`
	Creata        int64  `json:"creata"`
	Chiusa        int64  `json:"chiusa,omitempty"`
	Motivo        string `json:"motivo,omitempty"`
	TestoMancante bool   `json:"testoMancante,omitempty"`
}

type vistaEvento struct {
	ID              string          `json:"id"`
	Fonte           string          `json:"fonte"`
	Descrizione     string          `json:"descrizione"`
	Intensita       int             `json:"intensita"`
	CD              int             `json:"cd"`
	Terrore         int             `json:"terroreFallimento"`
	Caratteristica  string          `json:"caratteristica"`
	Vantaggio       string          `json:"vantaggio"`
	Modificatore    int             `json:"modificatore"`
	Competenza      string          `json:"competenza"`
	EffettoSuccesso string          `json:"effettoSuccesso,omitempty"`
	Stato           string          `json:"stato"`
	Creato          int64           `json:"creato"`
	Risolto         int64           `json:"risolto,omitempty"`
	Risoluzione     json.RawMessage `json:"risoluzione,omitempty"`
	Ripete          string          `json:"ripete,omitempty"`
}

// Risoluzione: il risultato completo del tiro, uguale per giocatore e Master.
type Risoluzione struct {
	Evento        string               `json:"evento"`
	Personaggio   string               `json:"personaggio"`
	Nome          string               `json:"nome"`
	Fonte         string               `json:"fonte"`
	Descrizione   string               `json:"descrizione"`
	TS            EsitoTS              `json:"ts"`
	Esito         string               `json:"esito"` // SUPERATO o FALLITO
	TerrorePrima  int                  `json:"terrorePrima"`
	TerroreAgg    int                  `json:"terroreAggiunto"`
	TerroreDopo   int                  `json:"terroreDopo"`
	Livello       LivelloTerrore       `json:"livello"`
	Effetti       []LivelloTerrore     `json:"effetti"`
	Condizioni    []string             `json:"condizioni,omitempty"`
	Resistenze    []string             `json:"resistenze,omitempty"`
	EffettoSucc   string               `json:"effettoSuccesso,omitempty"`
	Pazzia        []SuggerimentoPazzia `json:"pazzia,omitempty"`
	Automatiche   []string             `json:"automatiche,omitempty"`     // regole automatiche applicate (sanita_automatiche.go)
	PazzieAuto    []*vistaPazzia       `json:"pazzieAssegnate,omitempty"` // follie assegnate in automatico
	TerroreAttivo bool                 `json:"terroreAttivo"`
	Quando        int64                `json:"quando"`
}

type vistaSanita struct {
	Personaggio map[string]any   `json:"personaggio"`
	Config      ConfigSanita     `json:"config"`
	Punteggio   *int             `json:"punteggio"`
	Metodo      string           `json:"metodo"`
	Mod         *int             `json:"mod"`
	Competente  bool             `json:"competente"`
	Prof        int              `json:"prof"`
	BonusTemp   int              `json:"bonusTemp"`
	BonusTS     *int             `json:"bonusTS"`
	Terrore     int              `json:"terrore"`
	Livello     LivelloTerrore   `json:"livello"`
	Effetti     []LivelloTerrore `json:"effetti"`
	Scala       []LivelloTerrore `json:"scala"`
	ImmuneTerr  bool             `json:"immuneTerrore"`
	ImmuneSpav  bool             `json:"immuneSpaventato"`
	Manuale     *int             `json:"punteggioManuale,omitempty"`
	Pazzie      []vistaPazzia    `json:"pazzie"`
	Condizioni  []string         `json:"condizioni"` // condizioni attive legate a paura e Terrore
	Evento      *vistaEvento     `json:"evento"`
	Ultimo      *vistaEvento     `json:"ultimo"`
	Storico     []map[string]any `json:"storico"`
	Immagine    bool             `json:"immagine"`
	Master      bool             `json:"master"`
	Categorie   []string         `json:"categorie"`
}

func (s *Server) pazzieDi(pg string, anche bool) []vistaPazzia {
	q := `SELECT id, tipo, categoria, descrizione, tabella, durata_valore, durata_unita, COALESCE(scade_il,0), cd_recupero, attiva, COALESCE(evento_id,''), creato_il, COALESCE(chiusa_il,0), motivo_chiusura FROM pazzie WHERE personaggio_id = ?`
	if !anche {
		q += ` AND attiva = 1`
	}
	rows, err := s.DB.Query(q+` ORDER BY attiva DESC, creato_il DESC LIMIT 50`, pg)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []vistaPazzia{}
	for rows.Next() {
		var p vistaPazzia
		var dv int
		var du string
		var att int
		rows.Scan(&p.ID, &p.Tipo, &p.Categoria, &p.Descrizione, &p.Tabella, &dv, &du, &p.ScadeIl, &p.CDRecupero, &att, &p.Evento, &p.Creata, &p.Chiusa, &p.Motivo)
		p.Attiva = att == 1
		switch {
		case dv > 0:
			p.Durata = strconv.Itoa(dv) + " " + du
		case p.Tipo == "indeterminata":
			p.Durata = "fino a cura"
		default:
			p.Durata = "a discrezione del Master"
		}
		p.TestoMancante = strings.TrimSpace(p.Descrizione) == ""
		out = append(out, p)
	}
	return out
}

// scadenzePazzie chiude le pazzie temporanee scadute (tempo reale).
func (s *Server) scadenzePazzie(pg string) {
	rows, err := s.DB.Query(`SELECT id FROM pazzie WHERE personaggio_id = ? AND attiva = 1 AND scade_il IS NOT NULL AND scade_il <= ?`, pg, db.Ora())
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		s.DB.Exec(`UPDATE pazzie SET attiva = 0, chiusa_il = ?, motivo_chiusura = 'scaduta' WHERE id = ?`, db.Ora(), id)
		s.registraSanita(pg, nil, "pazzia_scaduta", map[string]any{"pazzia": id})
	}
}

func leggiEvento(row interface{ Scan(...any) error }) (*vistaEvento, error) {
	var e vistaEvento
	var ris sql.NullString
	var risolto sql.NullInt64
	var rip sql.NullString
	err := row.Scan(&e.ID, &e.Fonte, &e.Descrizione, &e.Intensita, &e.CD, &e.Terrore, &e.Caratteristica, &e.Vantaggio, &e.Modificatore, &e.Competenza, &e.EffettoSuccesso, &e.Stato, &e.Creato, &risolto, &ris, &rip)
	if err != nil {
		return nil, err
	}
	if ris.Valid {
		e.Risoluzione = json.RawMessage(ris.String)
	}
	e.Risolto, e.Ripete = risolto.Int64, rip.String
	return &e, nil
}

const colEvento = `id, fonte, descrizione, intensita, cd, terrore_fallimento, caratteristica, vantaggio, modificatore, competenza, effetto_successo, stato, creato_il, risolto_il, risoluzione, ripete`

func (s *Server) vistaSanita(u *Utente, pg string) (*vistaSanita, error) {
	if !s.puoLeggerePersonaggio(u, pg) {
		return nil, errNonTrovato
	}
	s.scadenzePazzie(pg)
	cfg, st := s.configSanita(), s.leggiSanitaPG(pg)
	r, prop := s.datiPG(pg)
	var nome string
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&nome)
	v := &vistaSanita{Config: cfg, Competente: st.competente, Prof: BonusCompetenza(r.Livello), BonusTemp: st.bonus, Terrore: st.terrore,
		Livello: LivelliTerrore[st.terrore], Effetti: EffettiAttivi(st.terrore), Scala: LivelliTerrore, ImmuneTerr: st.immuneTerrore, ImmuneSpav: st.immuneSpaventato,
		Manuale: st.manuale, Master: u.Master(), Storico: []map[string]any{}}
	if v.Effetti == nil {
		v.Effetti = []LivelloTerrore{}
	}
	for _, c := range CategoriePazzia {
		v.Categorie = append(v.Categorie, c.Nome)
	}
	v.Personaggio = map[string]any{"id": pg, "nome": nome, "mio": prop == u.ID, "ritratto": r.Ritratto}
	v.Punteggio, v.Metodo = s.punteggioSanita(pg, cfg, st, r)
	if v.Punteggio != nil {
		m := Modificatore(*v.Punteggio)
		b := m + st.bonus
		if st.competente {
			b += v.Prof
		}
		v.Mod, v.BonusTS = &m, &b
	}
	v.Pazzie = s.pazzieDi(pg, u.Master())
	v.Condizioni = condizioniPaura(s.statoInGioco(pg).Condizioni, st.condTerrore)
	if e, err := leggiEvento(s.DB.QueryRow(`SELECT `+colEvento+` FROM eventi_paura WHERE personaggio_id = ? AND stato IN ('READY','ROLLING','DRAFT','ERROR') ORDER BY creato_il DESC LIMIT 1`, pg)); err == nil {
		if e.Stato != "DRAFT" || u.Master() {
			v.Evento = e
		}
	}
	if e, err := leggiEvento(s.DB.QueryRow(`SELECT `+colEvento+` FROM eventi_paura WHERE personaggio_id = ? AND stato = 'RESOLVED' ORDER BY risolto_il DESC LIMIT 1`, pg)); err == nil {
		v.Ultimo = e
	}
	limite := 15
	if u.Master() {
		limite = 60
	}
	rows, err := s.DB.Query(`SELECT r.azione, r.dati, r.creato_il, COALESCE(u.nome,'') FROM registro_sanita r LEFT JOIN utenti u ON u.id = r.attore_id
		WHERE r.personaggio_id = ? ORDER BY r.id DESC LIMIT ?`, pg, limite)
	if err == nil {
		for rows.Next() {
			var az, dati, chi string
			var t int64
			rows.Scan(&az, &dati, &t, &chi)
			var d map[string]any
			json.Unmarshal([]byte(dati), &d)
			v.Storico = append(v.Storico, map[string]any{"azione": az, "dati": d, "quando": t, "chi": chi})
		}
		rows.Close()
	}
	f, _ := s.impostazione("sanita_immagine")
	v.Immagine = f != ""
	return v, nil
}

// condizioniPaura: tra le condizioni attive del personaggio, quelle messe dal Terrore e quelle di paura
// (Spaventato), nell'ordine in cui compaiono.
func condizioniPaura(attive json.RawMessage, daTerrore []string) []string {
	var l []string
	json.Unmarshal(attive, &l)
	out := []string{}
	for _, c := range l {
		if slices.Contains(daTerrore, c) || strings.EqualFold(c, "Spaventato") {
			out = append(out, c)
		}
	}
	return out
}

// ===== Rotte =====

func (s *Server) rotteSanita(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/sanita/config", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "approvato"); err != nil {
			return nil, err
		}
		return s.configSanita(), nil
	}))
	mux.Handle("PUT /api/v1/sanita/config", api(s.apiConfigSanita))
	mux.Handle("GET /api/v1/sanita/pg/{id}", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		u, err := richiedi(r, "approvato")
		if err != nil {
			return nil, err
		}
		return s.vistaSanita(u, r.PathValue("id"))
	}))
	mux.Handle("PUT /api/v1/sanita/pg/{id}", api(s.apiSanitaPG))
	mux.Handle("POST /api/v1/sanita/pg/{id}/terrore", api(s.apiTerrore))
	mux.Handle("POST /api/v1/sanita/pg/{id}/tiro", api(s.apiTiroSanita))
	mux.Handle("POST /api/v1/sanita/pg/{id}/pazzie", api(s.apiNuovaPazzia))
	mux.Handle("PATCH /api/v1/sanita/pazzie/{id}", api(s.apiModificaPazzia))
	mux.Handle("POST /api/v1/sanita/pazzie/{id}/recupero", api(s.apiRecuperoPazzia))
	mux.Handle("POST /api/v1/sanita/eventi", api(s.apiNuovoEvento))
	mux.Handle("PATCH /api/v1/sanita/eventi/{id}", api(s.apiModificaEvento))
	mux.Handle("POST /api/v1/sanita/eventi/{id}/ripeti", api(s.apiRipetiEvento))
	mux.Handle("GET /api/v1/sanita/catalogo", api(s.apiCatalogo))
	mux.Handle("PUT /api/v1/sanita/catalogo", api(s.apiSalvaCatalogo))
	mux.Handle("PUT /api/v1/sanita/immagine", api(s.apiCaricaImmagineSanita))
	mux.Handle("DELETE /api/v1/sanita/immagine", api(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "master"); err != nil {
			return nil, err
		}
		if f, _ := s.impostazione("sanita_immagine"); f != "" {
			s.eliminaFileDati(f)
		}
		return nil, s.impostaImpostazione("sanita_immagine", "")
	}))
	mux.Handle("GET /api/v1/sanita/immagine", http.HandlerFunc(s.immagineSanita))
	s.Hub.Registra("sanita.apri", s.wsApriSanita)
	s.Hub.Registra("sanita.soccombi", s.wsSoccombi)
}

func (s *Server) apiConfigSanita(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var c ConfigSanita
	if err := leggiJSON(r, &c); err != nil {
		return nil, err
	}
	if c.Metodo == "" || c.Metodo == MetodoDMG {
		c.Metodo = MetodoSaggezza // le impostazioni salvate da versioni precedenti restano accettate
	}
	if err := validaConfig(c); err != nil {
		return nil, err
	}
	c.Versione = s.configSanita().Versione + 1
	b, _ := json.Marshal(c)
	if err := s.impostaImpostazione("sanita_config", string(b)); err != nil {
		return nil, err
	}
	s.registraSanita("", u, "config", c)
	// Con il Terrore attivato o disattivato cambiano le condizioni applicate: si riallineano.
	rows, _ := s.DB.Query(`SELECT personaggio_id, terrore FROM sanita_pg`)
	type pt struct {
		id string
		t  int
	}
	var l []pt
	if rows != nil {
		for rows.Next() {
			var x pt
			rows.Scan(&x.id, &x.t)
			l = append(l, x)
		}
		rows.Close()
	}
	for _, x := range l {
		s.applicaCondizioniTerrore(x.id, u, x.t)
	}
	s.Hub.AlMaster(Msg{Tipo: "sanita.config"})
	for _, g := range s.gruppiDi(&Utente{Ruolo: "master", Stato: "approvato"}) {
		s.Hub.Pubblica("gruppo:"+g, Msg{Tipo: "sanita.config"})
	}
	return c, nil
}

// apiSanitaPG: competenza, bonus temporaneo, punteggio manuale e immunità (solo Master, registrato).
func (s *Server) apiSanitaPG(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, pg) {
		return nil, errNonTrovato
	}
	var in struct {
		Competente       *bool  `json:"competente"`
		BonusTemp        *int   `json:"bonusTemp"`
		PunteggioManuale *int   `json:"punteggioManuale"`
		TogliManuale     bool   `json:"togliManuale"`
		ImmuneTerrore    *bool  `json:"immuneTerrore"`
		ImmuneSpaventato *bool  `json:"immuneSpaventato"`
		Motivo           string `json:"motivo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	s.assicuraSanitaPG(s.DB, pg)
	b2i := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	if in.Competente != nil {
		s.DB.Exec(`UPDATE sanita_pg SET competente = ? WHERE personaggio_id = ?`, b2i(*in.Competente), pg)
	}
	if in.BonusTemp != nil {
		if *in.BonusTemp < -10 || *in.BonusTemp > 10 {
			return nil, errore(http.StatusBadRequest, "Bonus temporaneo tra −10 e +10.")
		}
		s.DB.Exec(`UPDATE sanita_pg SET bonus_temp = ? WHERE personaggio_id = ?`, *in.BonusTemp, pg)
	}
	if in.TogliManuale {
		s.DB.Exec(`UPDATE sanita_pg SET punteggio_manuale = NULL WHERE personaggio_id = ?`, pg)
	} else if in.PunteggioManuale != nil {
		if *in.PunteggioManuale < 1 || *in.PunteggioManuale > 30 {
			return nil, errore(http.StatusBadRequest, "Punteggio tra 1 e 30.")
		}
		s.DB.Exec(`UPDATE sanita_pg SET punteggio_manuale = ? WHERE personaggio_id = ?`, *in.PunteggioManuale, pg)
	}
	if in.ImmuneTerrore != nil {
		s.DB.Exec(`UPDATE sanita_pg SET immune_terrore = ? WHERE personaggio_id = ?`, b2i(*in.ImmuneTerrore), pg)
	}
	if in.ImmuneSpaventato != nil {
		s.DB.Exec(`UPDATE sanita_pg SET immune_spaventato = ? WHERE personaggio_id = ?`, b2i(*in.ImmuneSpaventato), pg)
	}
	s.DB.Exec(`UPDATE sanita_pg SET modificato_il = ? WHERE personaggio_id = ?`, db.Ora(), pg)
	s.registraSanita(pg, u, "impostazioni_pg", in)
	s.notificaSanita(pg, "impostazioni")
	return s.vistaSanita(u, pg)
}

// apiTerrore: correzione manuale (con motivo) e recuperi previsti dalla specifica (homebrew).
func (s *Server) apiTerrore(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, pg) {
		return nil, errNonTrovato
	}
	var in struct {
		Tipo   string `json:"tipo"`
		Valore *int   `json:"valore"`
		Motivo string `json:"motivo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	cfg, st := s.configSanita(), s.leggiSanitaPG(pg)
	motivo := strings.TrimSpace(in.Motivo)
	if len([]rune(motivo)) > 300 {
		return nil, errore(http.StatusBadRequest, "Motivo troppo lungo.")
	}
	nuovo := st.terrore
	testo := map[string]string{"fonte_scomparsa": "la fonte della paura è scomparsa e il personaggio è al sicuro", "riposo_lungo": "riposo lungo in un luogo sicuro",
		"dieci_giorni": "10 giorni di gioco senza nuova esposizione", "aiuto_alleato": "un alleato ha superato la prova di Saggezza (Intuizione o Persuasione) CD 15",
		"rimozione_spaventato": "un effetto ha rimosso la condizione Spaventato (regola opzionale)", "risveglio": "un alleato lo ha risvegliato (Medicina CD 15)"}
	switch in.Tipo {
	case "manuale":
		if in.Valore == nil || *in.Valore < 0 || *in.Valore > TerroreMax {
			return nil, errore(http.StatusBadRequest, "Terrore tra 0 e 7.")
		}
		if motivo == "" {
			return nil, errore(http.StatusBadRequest, "Scrivi il motivo della correzione.")
		}
		nuovo = *in.Valore
	case "fonte_scomparsa", "riposo_lungo", "dieci_giorni", "aiuto_alleato":
		nuovo--
	case "rimozione_spaventato":
		if !cfg.RegolaRimozione {
			return nil, errore(http.StatusBadRequest, "La regola opzionale sulla rimozione di Spaventato non è attiva.")
		}
		nuovo -= 3
	case "risveglio":
		if st.terrore != 7 {
			return nil, errore(http.StatusBadRequest, "Il personaggio non è svenuto.")
		}
		nuovo = 6
	default:
		return nil, errDatiNonValidi
	}
	if motivo == "" {
		motivo = testo[in.Tipo]
	}
	prima, dopo, err := s.impostaTerrore(pg, u, nuovo, "terrore_"+in.Tipo, motivo, nil)
	if err != nil {
		return nil, err
	}
	s.notificaSanita(pg, "terrore")
	return map[string]int{"prima": prima, "dopo": dopo}, nil
}

// ===== Eventi di paura =====

type richiestaEvento struct {
	Personaggi      []string `json:"personaggi"`
	Sessione        string   `json:"sessione"`
	Fonte           string   `json:"fonte"`
	Descrizione     string   `json:"descrizione"`
	Intensita       int      `json:"intensita"`
	CD              *int     `json:"cd"`
	Terrore         *int     `json:"terroreFallimento"`
	Caratteristica  string   `json:"caratteristica"`
	Vantaggio       string   `json:"vantaggio"`
	Modificatore    int      `json:"modificatore"`
	Competenza      string   `json:"competenza"`
	EffettoSuccesso string   `json:"effettoSuccesso"`
	Bozza           bool     `json:"bozza"`
}

func (s *Server) apiNuovoEvento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in richiestaEvento
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	cfg := s.configSanita()
	fonte, err := rigaSingola(in.Fonte, "Fonte del terrore", 1, 80)
	if err != nil {
		return nil, err
	}
	if len([]rune(in.Descrizione)) > 1000 || len([]rune(in.EffettoSuccesso)) > 300 {
		return nil, errore(http.StatusBadRequest, "Testo troppo lungo.")
	}
	if in.Intensita < 0 || in.Intensita > 5 || len(in.Personaggi) == 0 || len(in.Personaggi) > 20 {
		return nil, errore(http.StatusBadRequest, "Scegli intensità (0–5) e almeno un personaggio.")
	}
	cd, terr := cfg.CDIntensita[in.Intensita], cfg.TerroreIntensita[in.Intensita]
	if in.CD != nil {
		cd = *in.CD
	}
	if in.Terrore != nil {
		terr = *in.Terrore
	}
	if cd < 1 || cd > 40 || terr < 0 || terr > 7 || in.Modificatore < -20 || in.Modificatore > 20 {
		return nil, errore(http.StatusBadRequest, "CD (1–40), Terrore (0–7) o modificatore non validi.")
	}
	if in.Caratteristica == "" {
		in.Caratteristica = "SAN"
	}
	if in.Caratteristica != "SAN" && in.Caratteristica != "SAG" && in.Caratteristica != "CAR" {
		return nil, errDatiNonValidi
	}
	if in.Vantaggio == "" {
		in.Vantaggio = "normale"
	}
	if in.Competenza == "" {
		in.Competenza = "scheda"
	}
	stato := "READY"
	if in.Bozza {
		stato = "DRAFT"
	}
	var sess any
	if in.Sessione != "" {
		sess = in.Sessione
	}
	var ids []string
	for _, pg := range in.Personaggi {
		if !s.puoLeggerePersonaggio(u, pg) {
			return nil, errNonTrovato
		}
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM eventi_paura WHERE personaggio_id = ? AND stato IN ('READY','ROLLING')`, pg).Scan(&n)
		if n > 0 && stato == "READY" {
			return nil, errore(http.StatusConflict, "Un personaggio ha già un evento di paura aperto: risolvilo o annullalo prima.")
		}
	}
	for _, pg := range in.Personaggi {
		id := cripto.ID()
		if _, err := s.DB.Exec(`INSERT INTO eventi_paura(id, sessione_id, personaggio_id, fonte, descrizione, intensita, cd, terrore_fallimento, caratteristica, vantaggio, modificatore, competenza, effetto_successo, stato, creato_da, creato_il)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, sess, pg, fonte, in.Descrizione, in.Intensita, cd, terr, in.Caratteristica, in.Vantaggio, in.Modificatore, in.Competenza, in.EffettoSuccesso, stato, u.ID, db.Ora()); err != nil {
			return nil, err
		}
		s.registraSanita(pg, u, "evento_creato", map[string]any{"evento": id, "fonte": fonte, "intensita": in.Intensita, "cd": cd, "stato": stato,
			"caratteristica": in.Caratteristica, "vantaggio": in.Vantaggio, "modificatore": in.Modificatore, "competenza": in.Competenza})
		ids = append(ids, id)
		s.avvisaEvento(pg, stato, fonte)
	}
	return map[string]any{"eventi": ids}, nil
}

func (s *Server) avvisaEvento(pg, stato, fonte string) {
	if stato == "READY" {
		var prop string
		s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, pg).Scan(&prop)
		if prop != "" {
			s.Hub.AUtente(prop, Msg{Tipo: "sanita.evento", Dati: map[string]string{"personaggio": pg, "fonte": fonte}})
		}
	}
	s.notificaSanita(pg, "evento")
}

// apiModificaEvento: il Master attiva una bozza, cambia parametri prima del tiro o annulla.
func (s *Server) apiModificaEvento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var pg, stato, fonte string
	if s.DB.QueryRow(`SELECT personaggio_id, stato, fonte FROM eventi_paura WHERE id = ?`, id).Scan(&pg, &stato, &fonte) != nil {
		return nil, errNonTrovato
	}
	var in struct {
		Stato        string  `json:"stato"`
		CD           *int    `json:"cd"`
		Vantaggio    *string `json:"vantaggio"`
		Modificatore *int    `json:"modificatore"`
		Competenza   *string `json:"competenza"`
		Motivo       string  `json:"motivo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if stato != "DRAFT" && stato != "READY" && stato != "ERROR" {
		return nil, errore(http.StatusConflict, "L'evento è già "+map[string]string{"ROLLING": "in risoluzione", "RESOLVED": "risolto", "CANCELLED": "annullato"}[stato]+": per ripetere il tiro usa «Ripeti il tiro».")
	}
	if in.CD != nil && (*in.CD < 1 || *in.CD > 40) || in.Modificatore != nil && (*in.Modificatore < -20 || *in.Modificatore > 20) ||
		in.Vantaggio != nil && *in.Vantaggio != "normale" && *in.Vantaggio != "vantaggio" && *in.Vantaggio != "svantaggio" ||
		in.Competenza != nil && *in.Competenza != "scheda" && *in.Competenza != "si" && *in.Competenza != "no" {
		return nil, errDatiNonValidi
	}
	if in.CD != nil {
		s.DB.Exec(`UPDATE eventi_paura SET cd = ? WHERE id = ?`, *in.CD, id)
	}
	if in.Vantaggio != nil {
		s.DB.Exec(`UPDATE eventi_paura SET vantaggio = ? WHERE id = ?`, *in.Vantaggio, id)
	}
	if in.Modificatore != nil {
		s.DB.Exec(`UPDATE eventi_paura SET modificatore = ? WHERE id = ?`, *in.Modificatore, id)
	}
	if in.Competenza != nil {
		s.DB.Exec(`UPDATE eventi_paura SET competenza = ? WHERE id = ?`, *in.Competenza, id)
	}
	switch in.Stato {
	case "":
	case "READY", "CANCELLED":
		if in.Stato == "READY" {
			var n int
			s.DB.QueryRow(`SELECT COUNT(*) FROM eventi_paura WHERE personaggio_id = ? AND stato IN ('READY','ROLLING') AND id <> ?`, pg, id).Scan(&n)
			if n > 0 {
				return nil, errore(http.StatusConflict, "Il personaggio ha già un altro evento aperto.")
			}
		}
		s.DB.Exec(`UPDATE eventi_paura SET stato = ? WHERE id = ?`, in.Stato, id)
	default:
		return nil, errDatiNonValidi
	}
	s.registraSanita(pg, u, "evento_modificato", map[string]any{"evento": id, "modifiche": in})
	s.avvisaEvento(pg, in.Stato, fonte)
	return nil, nil
}

// apiRipetiEvento: ripetizione esplicita e registrata di un tiro già risolto (nuovo evento READY con gli
// stessi parametri); a richiesta il Terrore torna al valore di prima del tiro precedente.
func (s *Server) apiRipetiEvento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var in struct {
		Motivo            string `json:"motivo"`
		RipristinaTerrore bool   `json:"ripristinaTerrore"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Motivo) == "" {
		return nil, errore(http.StatusBadRequest, "Scrivi il motivo della ripetizione.")
	}
	var pg, stato string
	var ris sql.NullString
	if s.DB.QueryRow(`SELECT personaggio_id, stato, risoluzione FROM eventi_paura WHERE id = ?`, id).Scan(&pg, &stato, &ris) != nil {
		return nil, errNonTrovato
	}
	if stato != "RESOLVED" {
		return nil, errore(http.StatusConflict, "Si ripete solo un tiro già risolto.")
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM eventi_paura WHERE personaggio_id = ? AND stato IN ('READY','ROLLING')`, pg).Scan(&n)
	if n > 0 {
		return nil, errore(http.StatusConflict, "Il personaggio ha già un evento aperto.")
	}
	nuovo := cripto.ID()
	if _, err := s.DB.Exec(`INSERT INTO eventi_paura(id, sessione_id, personaggio_id, fonte, descrizione, intensita, cd, terrore_fallimento, caratteristica, vantaggio, modificatore, competenza, effetto_successo, stato, ripete, creato_da, creato_il)
		SELECT ?, sessione_id, personaggio_id, fonte, descrizione, intensita, cd, terrore_fallimento, caratteristica, vantaggio, modificatore, competenza, effetto_successo, 'READY', id, ?, ? FROM eventi_paura WHERE id = ?`,
		nuovo, u.ID, db.Ora(), id); err != nil {
		return nil, err
	}
	s.registraSanita(pg, u, "evento_ripetuto", map[string]any{"precedente": id, "nuovo": nuovo, "motivo": in.Motivo, "risultatoPrecedente": json.RawMessage(ris.String)})
	if in.RipristinaTerrore && ris.Valid {
		var prec Risoluzione
		if json.Unmarshal([]byte(ris.String), &prec) == nil {
			s.impostaTerrore(pg, u, prec.TerrorePrima, "terrore_manuale", "ripetizione del tiro: "+in.Motivo, map[string]any{"evento": id})
		}
	}
	var fonte string
	s.DB.QueryRow(`SELECT fonte FROM eventi_paura WHERE id = ?`, nuovo).Scan(&fonte)
	s.avvisaEvento(pg, "READY", fonte)
	return map[string]string{"evento": nuovo}, nil
}

// wsApriSanita: il giocatore apre la schermata; il Master riceve l'avviso e vede la stessa schermata.
func (s *Server) wsApriSanita(c *Cliente, d json.RawMessage) (any, error) {
	var in struct {
		Personaggio string `json:"personaggio"`
	}
	if json.Unmarshal(d, &in) != nil || in.Personaggio == "" {
		return nil, errDatiNonValidi
	}
	v, err := s.vistaSanita(c.U, in.Personaggio)
	if err != nil {
		return nil, err
	}
	if !c.U.Master() {
		s.Hub.AlMaster(Msg{Tipo: "sanita.aperta", Dati: map[string]string{"personaggio": in.Personaggio, "nome": v.Personaggio["nome"].(string), "da": c.U.Nome}})
		s.registraSanita(in.Personaggio, c.U, "schermata_aperta", map[string]any{})
	}
	return v, nil
}

// wsSoccombi: "Soccombi alla paura". Un solo tiro per evento, fatto qui: doppio clic, richieste
// ripetute, ricaricamento della pagina o riconnessione restituiscono lo stesso risultato già salvato.
func (s *Server) wsSoccombi(c *Cliente, d json.RawMessage) (any, error) {
	var in struct {
		Evento string `json:"evento"`
		Chiave string `json:"chiave"`
	}
	if json.Unmarshal(d, &in) != nil || in.Evento == "" || len(in.Chiave) > 80 {
		return nil, errDatiNonValidi
	}
	var pg, stato string
	var ris sql.NullString
	if s.DB.QueryRow(`SELECT personaggio_id, stato, risoluzione FROM eventi_paura WHERE id = ?`, in.Evento).Scan(&pg, &stato, &ris) != nil {
		return nil, errNonTrovato
	}
	_, prop := s.datiPG(pg)
	if prop != c.U.ID && !c.U.Master() {
		return nil, errore(http.StatusForbidden, "Puoi tirare solo per il tuo personaggio.")
	}
	switch stato {
	case "RESOLVED":
		return json.RawMessage(ris.String), nil // già risolto: stesso risultato, nessun nuovo tiro
	case "ROLLING":
		return nil, errore(http.StatusConflict, "Il tiro è già in corso.")
	case "DRAFT":
		return nil, errore(http.StatusConflict, "Il Master non ha ancora attivato l'evento.")
	case "CANCELLED":
		return nil, errore(http.StatusConflict, "Il Master ha annullato l'evento.")
	case "ERROR":
		return nil, errore(http.StatusConflict, "La risoluzione precedente non è riuscita: il Master deve riattivare l'evento.")
	}
	// READY → ROLLING in modo atomico: solo una richiesta può passare.
	res, err := s.DB.Exec(`UPDATE eventi_paura SET stato = 'ROLLING', chiave = ? WHERE id = ? AND stato = 'READY'`, in.Chiave, in.Evento)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return s.wsSoccombi(c, d) // un'altra richiesta è arrivata prima: si legge il suo esito
	}
	out, err := s.risolviEvento(c.U, in.Evento)
	if err != nil {
		s.DB.Exec(`UPDATE eventi_paura SET stato = 'ERROR' WHERE id = ? AND stato = 'ROLLING'`, in.Evento)
		s.registraSanita(pg, c.U, "evento_errore", map[string]any{"evento": in.Evento, "errore": err.Error()})
		s.notificaSanita(pg, "errore")
		return nil, err
	}
	s.notificaSanita(pg, "risolto")
	return out, nil
}

func (s *Server) risolviEvento(u *Utente, evento string) (*Risoluzione, error) {
	e, err := leggiEvento(s.DB.QueryRow(`SELECT `+colEvento+` FROM eventi_paura WHERE id = ?`, evento))
	if err != nil {
		return nil, err
	}
	var pg string
	s.DB.QueryRow(`SELECT personaggio_id FROM eventi_paura WHERE id = ?`, evento).Scan(&pg)
	cfg := s.configSanita()
	s.assicuraSanitaPG(s.DB, pg)
	st := s.leggiSanitaPG(pg)
	rp, _ := s.datiPG(pg)
	var nome string
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&nome)
	san, _ := s.punteggioSanita(pg, cfg, st, rp)
	punt, car, err := PunteggioPerTS(e.Caratteristica, san, rp.Car, false)
	if err != nil {
		return nil, errore(http.StatusConflict, "Manca il punteggio di "+map[string]string{"SAN": "Sanità mentale", "SAG": "Saggezza", "CAR": "Carisma"}[e.Caratteristica]+
			" nella scheda: compilala o chiedi al Master di inserirlo a mano.")
	}
	competente := false
	switch e.Competenza {
	case "si":
		competente = true
	case "scheda":
		if car == "SAN" {
			competente = st.competente
		} else {
			competente = slices.Contains(rp.TS, car)
		}
	}
	vant, resistenze := e.Vantaggio, []string{}
	if cfg.RegolaResistenza && st.immuneSpaventato {
		vant = CombinaVantaggio(vant, "vantaggio")
		resistenze = append(resistenze, "Immune a Spaventato (regola opzionale): vantaggio e 1 Terrore in meno")
	}
	if st.immuneTerrore {
		resistenze = append(resistenze, "Immunità completa al Terrore")
	}
	bonus := 0
	if car == "SAN" {
		bonus = st.bonus
	}
	ts := TiroSalvezza(tiraDado, car, punt, competente, BonusCompetenza(rp.Livello), bonus, e.Modificatore, e.CD, vant)
	ris := &Risoluzione{Evento: evento, Personaggio: pg, Nome: nome, Fonte: e.Fonte, Descrizione: e.Descrizione, TS: ts, TerrorePrima: st.terrore,
		Resistenze: resistenze, TerroreAttivo: cfg.Terrore, Quando: db.Ora()}
	if ts.Successo {
		ris.Esito, ris.EffettoSucc = "SUPERATO", e.EffettoSuccesso
	} else {
		ris.Esito = "FALLITO"
	}
	if cfg.Terrore {
		ris.TerroreAgg = TerroreGuadagnato(ts.Successo, e.Terrore, st.immuneSpaventato, cfg.RegolaResistenza, st.immuneTerrore)
	}
	// Regole automatiche: fallito di 5 o più (+1), tre fallimenti di seguito (+2).
	extra, fallimenti, motivi := RegoleTSSanita(ts, s.fallimentiConsecutivi(pg))
	if cfg.Terrore && !st.immuneTerrore && extra > 0 {
		ris.TerroreAgg += extra
		ris.Automatiche = append(ris.Automatiche, motivi...)
	}
	ris.TerroreDopo = LimitaTerrore(st.terrore + ris.TerroreAgg)
	ris.Livello, ris.Effetti = LivelliTerrore[ris.TerroreDopo], EffettiAttivi(ris.TerroreDopo)
	if cfg.Terrore {
		ris.Condizioni = CondizioniTerrore(ris.TerroreDopo)
	}
	if cfg.Terrore && cfg.Pazzia {
		var tmp int
		s.DB.QueryRow(`SELECT COUNT(*) FROM pazzie WHERE personaggio_id = ? AND attiva = 1 AND tipo = 'temporanea'`, pg).Scan(&tmp)
		// le follie temporanea (soglia 4) e indeterminata (soglia 6) ora si assegnano da sole: resta il suggerimento della duratura
		for _, x := range SogliePazzia(st.terrore, ris.TerroreDopo, e.Intensita, tmp > 0) {
			if x.Tipo == "duratura" {
				ris.Pazzia = append(ris.Pazzia, x)
			}
		}
	}
	b, _ := json.Marshal(ris)
	// Prima si salva tutto, poi si avvisano i dispositivi.
	err = db.Tx(context.Background(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE eventi_paura SET stato = 'RESOLVED', risolto_il = ?, risoluzione = ? WHERE id = ? AND stato = 'ROLLING'`, ris.Quando, string(b), evento); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE sanita_pg SET terrore = ?, fallimenti_consecutivi = ?, modificato_il = ? WHERE personaggio_id = ?`, ris.TerroreDopo, fallimenti, ris.Quando, pg)
		return err
	})
	if err != nil {
		return nil, err
	}
	// Follie automatiche (1 sul d20, soglie 4 e 6): dopo il salvataggio, poi si aggiorna la risoluzione salvata.
	if pz, mot := s.pazzieAutomatiche(pg, u, evento, st.terrore, ris.TerroreDopo, UnoNaturale(ts) && !st.immuneTerrore); len(mot) > 0 {
		ris.PazzieAuto, ris.Automatiche = pz, append(ris.Automatiche, mot...)
		b, _ = json.Marshal(ris)
		s.DB.Exec(`UPDATE eventi_paura SET risoluzione = ? WHERE id = ?`, string(b), evento)
	}
	s.registraSanita(pg, u, "tiro_paura", ris)
	if ris.TerroreDopo != st.terrore {
		s.registraSanita(pg, u, "terrore_evento", map[string]any{"prima": st.terrore, "dopo": ris.TerroreDopo, "motivo": "evento: " + e.Fonte, "evento": evento})
	}
	s.applicaCondizioniTerrore(pg, u, ris.TerroreDopo)
	return ris, nil
}

// ===== Tiri richiesti dagli effetti (Barcollante, Svenuto, pazzia, Spaventato soprannaturale) =====

func (s *Server) apiTiroSanita(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, pg) {
		return nil, errNonTrovato
	}
	var in struct {
		Tipo      string `json:"tipo"`
		Evento    string `json:"evento"`
		CD        int    `json:"cd"`
		Vantaggio string `json:"vantaggio"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	cfg := s.configSanita()
	s.assicuraSanitaPG(s.DB, pg)
	st := s.leggiSanitaPG(pg)
	rp, _ := s.datiPG(pg)
	prof := BonusCompetenza(rp.Livello)
	tsCar := func(car string, cd int) (EsitoTS, error) {
		v, ok := rp.Car[car]
		if !ok {
			return EsitoTS{}, errore(http.StatusConflict, "Manca il punteggio della caratteristica nella scheda.")
		}
		return TiroSalvezza(tiraDado, car, v, slices.Contains(rp.TS, car), prof, 0, 0, cd, "normale"), nil
	}
	out := map[string]any{"tipo": in.Tipo}
	switch in.Tipo {
	case "sanita": // tiro salvezza su Sanità mentale chiesto dal giocatore o dal Master (CD facoltativa)
		if in.CD < 0 || in.CD > 40 {
			return nil, errore(http.StatusBadRequest, "La CD va da 1 a 40 (oppure lasciala vuota).")
		}
		if in.Vantaggio == "" {
			in.Vantaggio = "normale"
		}
		if in.Vantaggio != "normale" && in.Vantaggio != "vantaggio" && in.Vantaggio != "svantaggio" {
			return nil, errDatiNonValidi
		}
		san, _ := s.punteggioSanita(pg, cfg, st, rp)
		if san == nil {
			return nil, errore(http.StatusConflict, "Manca la Saggezza nella scheda: senza non si calcola la Sanità mentale.")
		}
		e := TiroSalvezza(tiraDado, "SAN", *san, st.competente, prof, st.bonus, 0, in.CD, in.Vantaggio)
		out["ts"] = e
		s.registraSanita(pg, u, "tiro_sanita", map[string]any{"ts": e})
		if in.CD > 0 { // regole automatiche (sanita_automatiche.go): servono una CD e un esito
			extra, fallimenti, motivi := RegoleTSSanita(e, s.fallimentiConsecutivi(pg))
			s.DB.Exec(`UPDATE sanita_pg SET fallimenti_consecutivi = ? WHERE personaggio_id = ?`, fallimenti, pg)
			prima, dopo := st.terrore, st.terrore
			var auto []string
			if cfg.Terrore && !st.immuneTerrore && extra > 0 {
				prima, dopo, _ = s.impostaTerroreBase(pg, u, st.terrore+extra, "terrore_regole", strings.Join(motivi, "; "), map[string]any{"ts": e})
				auto = append(auto, motivi...)
				out["terrore"] = map[string]int{"prima": prima, "dopo": dopo}
			}
			if pz, mot := s.pazzieAutomatiche(pg, u, "", prima, dopo, UnoNaturale(e) && !st.immuneTerrore); len(mot) > 0 {
				auto = append(auto, mot...)
				out["pazzieAssegnate"] = pz
			}
			if len(auto) > 0 {
				out["automatiche"] = auto
			}
		}
	case "barcollante": // inizio turno a Terrore 4+: TS Saggezza CD 15, successo → −1
		if !cfg.Terrore || st.terrore < 4 || st.terrore > 5 {
			return nil, errore(http.StatusConflict, "Questo tiro vale a Terrore 4 o 5.")
		}
		e, err := tsCar("SAG", cfg.CDBarcollante)
		if err != nil {
			return nil, err
		}
		out["ts"] = e
		if e.Successo {
			s.impostaTerrore(pg, u, st.terrore-1, "terrore_barcollante", "TS di Saggezza superato all'inizio del turno", map[string]any{"ts": e})
		} else {
			s.registraSanita(pg, u, "tiro_barcollante", map[string]any{"ts": e})
		}
	case "svenuto": // fine turno a Terrore 7: TS Costituzione CD 15, successo → 6
		if !cfg.Terrore || st.terrore != 7 {
			return nil, errore(http.StatusConflict, "Il personaggio non è svenuto.")
		}
		e, err := tsCar("COS", cfg.CDSvenuto)
		if err != nil {
			return nil, err
		}
		out["ts"] = e
		if e.Successo {
			s.impostaTerrore(pg, u, 6, "terrore_svenuto", "TS di Costituzione superato a fine turno: si riprende", map[string]any{"ts": e})
		} else {
			s.registraSanita(pg, u, "tiro_svenuto", map[string]any{"ts": e})
		}
	case "carisma_pazzia": // soglia 4: il Master chiede un TS di Carisma contro la CD dell'evento
		if !u.Master() {
			return nil, errVietato
		}
		var cd int
		var stato string
		if s.DB.QueryRow(`SELECT cd, stato FROM eventi_paura WHERE id = ? AND personaggio_id = ?`, in.Evento, pg).Scan(&cd, &stato) != nil || stato != "RESOLVED" {
			return nil, errore(http.StatusBadRequest, "Scegli l'evento risolto che ha portato il Terrore a 4.")
		}
		e, err := tsCar("CAR", cd)
		if err != nil {
			return nil, err
		}
		out["ts"] = e
		s.registraSanita(pg, u, "tiro_carisma_pazzia", map[string]any{"ts": e, "evento": in.Evento})
		if !e.Successo && cfg.Pazzia {
			p, err := s.creaPazzia(pg, u, nuovaPazzia{Tipo: "temporanea", Casuale: true, Evento: in.Evento, Conferma: true})
			if err != nil {
				out["pazziaErrore"] = err.Error()
			} else {
				out["pazzia"] = p
			}
		}
	case "spaventato_soprannaturale": // regola opzionale: TS Sanità mentale, fallimento → +3
		if !u.Master() {
			return nil, errVietato
		}
		if !cfg.RegolaSpaventato || !cfg.Terrore {
			return nil, errore(http.StatusConflict, "La regola opzionale su Spaventato soprannaturale non è attiva.")
		}
		if in.CD < 1 || in.CD > 40 {
			return nil, errore(http.StatusBadRequest, "Indica la CD dell'effetto.")
		}
		san, _ := s.punteggioSanita(pg, cfg, st, rp)
		if san == nil {
			return nil, errore(http.StatusConflict, "Manca il punteggio di Sanità mentale.")
		}
		e := TiroSalvezza(tiraDado, "SAN", *san, st.competente, prof, st.bonus, 0, in.CD, "normale")
		out["ts"] = e
		if !e.Successo {
			s.impostaTerrore(pg, u, st.terrore+3, "terrore_spaventato", "Spaventato da un effetto soprannaturale (regola opzionale): TS fallito", map[string]any{"ts": e})
		} else {
			s.registraSanita(pg, u, "tiro_spaventato", map[string]any{"ts": e})
		}
	default:
		return nil, errDatiNonValidi
	}
	s.notificaSanita(pg, "tiro")
	// Il risultato lo vedono il Master e il giocatore del personaggio, chiunque abbia tirato.
	var nome, prop string
	s.DB.QueryRow(`SELECT nome, COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, pg).Scan(&nome, &prop)
	out["nome"], out["da"] = nome, u.Nome
	m := Msg{Tipo: "sanita.tiro", Dati: map[string]any{"personaggio": pg, "nome": nome, "da": u.Nome, "esito": out}}
	s.Hub.AlMaster(m)
	if prop != "" && !(u.Master() && prop == u.ID) {
		s.Hub.AUtente(prop, m)
	}
	return out, nil
}

// ===== Pazzie =====

type nuovaPazzia struct {
	Tipo        string `json:"tipo"`
	Categoria   string `json:"categoria"`
	Descrizione string `json:"descrizione"`
	Casuale     bool   `json:"casuale"`
	Evento      string `json:"evento"`
	CDRecupero  int    `json:"cdRecupero"`
	Conferma    bool   `json:"conferma"` // obbligatoria per duratura e indeterminata
	Override    bool   `json:"override"` // più di una pazzia per lo stesso evento
}

func (s *Server) creaPazzia(pg string, u *Utente, in nuovaPazzia) (*vistaPazzia, error) {
	cfg := s.configSanita()
	switch in.Tipo {
	case "temporanea", "duratura", "indeterminata":
	case "cronica":
		if !cfg.PazziaCronica {
			return nil, errore(http.StatusBadRequest, "La pazzia cronica (homebrew) non è attiva.")
		}
	default:
		return nil, errDatiNonValidi
	}
	if (in.Tipo == "duratura" || in.Tipo == "indeterminata") && !in.Conferma {
		return nil, errore(http.StatusConflict, "Serve la conferma del Master per una pazzia "+in.Tipo+".")
	}
	if in.Evento != "" && !in.Override {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM pazzie WHERE evento_id = ?`, in.Evento).Scan(&n)
		if n > 0 {
			return nil, errore(http.StatusConflict, "Questo evento ha già generato una pazzia (spunta «eccezione del Master» per aggiungerne un'altra).")
		}
	}
	tabella := cfg.Tabelle
	if tabella == "dmg" {
		tabella = "dmg_" + in.Tipo
	}
	dettagli := map[string]any{}
	cat, desc := strings.TrimSpace(in.Categoria), strings.TrimSpace(in.Descrizione)
	if in.Casuale {
		if cfg.Tabelle == "homebrew" {
			d20 := tiraDado(20)
			cat = CategoriaDaD20(d20)
			dettagli["d20"] = d20
		} else {
			cat = "Tabella DMG (" + in.Tipo + ")"
		}
		d100 := tiraDado(100)
		voce := VoceDaD100(d100)
		dettagli["d100"], dettagli["voce"] = d100, voce
		chiaveCat := cat
		if cfg.Tabelle == "dmg" {
			chiaveCat = in.Tipo
		}
		s.DB.QueryRow(`SELECT testo FROM pazzia_catalogo WHERE tabella = ? AND categoria = ? AND voce = ?`, tabella, chiaveCat, voce).Scan(&desc)
		if desc == "" {
			dettagli["testoMancante"] = true
		}
	} else if cat == "" {
		return nil, errore(http.StatusBadRequest, "Scegli una categoria o la generazione casuale.")
	}
	if len([]rune(cat)) > 80 || len([]rune(desc)) > 1000 {
		return nil, errore(http.StatusBadRequest, "Testo troppo lungo.")
	}
	dv, du := DurataPazzia(in.Tipo, tiraDado)
	var scade any
	if in.Tipo == "temporanea" {
		scade = db.Ora() + int64(dv)*60
	}
	cd := in.CDRecupero
	if cd == 0 && in.Evento != "" {
		s.DB.QueryRow(`SELECT cd FROM eventi_paura WHERE id = ?`, in.Evento).Scan(&cd)
	}
	if cd == 0 {
		cd = 15
	}
	if cd < 1 || cd > 40 {
		return nil, errore(http.StatusBadRequest, "CD di recupero tra 1 e 40.")
	}
	if cfg.AumentaCDPrecedente {
		rows, _ := s.DB.Query(`SELECT id, cd_recupero FROM pazzie WHERE personaggio_id = ? AND attiva = 1`, pg)
		type p2 struct {
			id string
			cd int
		}
		var prec []p2
		if rows != nil {
			for rows.Next() {
				var x p2
				rows.Scan(&x.id, &x.cd)
				prec = append(prec, x)
			}
			rows.Close()
		}
		for _, x := range prec {
			s.DB.Exec(`UPDATE pazzie SET cd_recupero = ? WHERE id = ?`, min(40, x.cd+1), x.id)
			s.registraSanita(pg, u, "pazzia_cd_aumentata", map[string]any{"pazzia": x.id, "prima": x.cd, "dopo": min(40, x.cd+1)})
		}
	}
	id := cripto.ID()
	var ev any
	if in.Evento != "" {
		ev = in.Evento
	}
	if _, err := s.DB.Exec(`INSERT INTO pazzie(id, personaggio_id, tipo, categoria, descrizione, tabella, durata_valore, durata_unita, scade_il, cd_recupero, evento_id, creato_il)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, id, pg, in.Tipo, cat, desc, tabella, dv, du, scade, cd, ev, db.Ora()); err != nil {
		return nil, err
	}
	dettagli["pazzia"], dettagli["tipo"], dettagli["categoria"], dettagli["durata"], dettagli["cd"] = id, in.Tipo, cat, strconv.Itoa(dv)+" "+du, cd
	s.registraSanita(pg, u, "pazzia_assegnata", dettagli)
	for _, p := range s.pazzieDi(pg, true) {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, errors.New("pazzia non trovata")
}

func (s *Server) apiNuovaPazzia(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, pg) {
		return nil, errNonTrovato
	}
	var in nuovaPazzia
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	p, err := s.creaPazzia(pg, u, in)
	if err != nil {
		return nil, err
	}
	s.notificaSanita(pg, "pazzia")
	return p, nil
}

// apiModificaPazzia: testo, CD di recupero, rimozione (cura) con motivo. Solo Master, registrato.
func (s *Server) apiModificaPazzia(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var pg string
	if s.DB.QueryRow(`SELECT personaggio_id FROM pazzie WHERE id = ?`, id).Scan(&pg) != nil {
		return nil, errNonTrovato
	}
	var in struct {
		Descrizione *string `json:"descrizione"`
		Categoria   *string `json:"categoria"`
		CDRecupero  *int    `json:"cdRecupero"`
		Rimuovi     bool    `json:"rimuovi"`
		Motivo      string  `json:"motivo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Descrizione != nil {
		if len([]rune(*in.Descrizione)) > 1000 {
			return nil, errore(http.StatusBadRequest, "Testo troppo lungo.")
		}
		s.DB.Exec(`UPDATE pazzie SET descrizione = ? WHERE id = ?`, *in.Descrizione, id)
	}
	if in.Categoria != nil {
		s.DB.Exec(`UPDATE pazzie SET categoria = ? WHERE id = ?`, *in.Categoria, id)
	}
	if in.CDRecupero != nil {
		if *in.CDRecupero < 0 || *in.CDRecupero > 40 {
			return nil, errDatiNonValidi
		}
		s.DB.Exec(`UPDATE pazzie SET cd_recupero = ? WHERE id = ?`, *in.CDRecupero, id)
	}
	if in.Rimuovi {
		m := strings.TrimSpace(in.Motivo)
		if m == "" {
			m = "rimossa dal Master"
		}
		s.DB.Exec(`UPDATE pazzie SET attiva = 0, chiusa_il = ?, motivo_chiusura = ? WHERE id = ?`, db.Ora(), m, id)
	}
	s.registraSanita(pg, u, "pazzia_modificata", map[string]any{"pazzia": id, "modifiche": in})
	s.notificaSanita(pg, "pazzia")
	return nil, nil
}

// apiRecuperoPazzia (HOMEBREW): ogni 10 giorni di gioco TS di Saggezza contro la CD; successo → CD −1d4.
func (s *Server) apiRecuperoPazzia(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var pg string
	var cd, att int
	if s.DB.QueryRow(`SELECT personaggio_id, cd_recupero, attiva FROM pazzie WHERE id = ?`, id).Scan(&pg, &cd, &att) != nil || att != 1 {
		return nil, errNonTrovato
	}
	var in struct {
		DataGioco string `json:"dataGioco"`
	}
	leggiJSON(r, &in)
	rp, _ := s.datiPG(pg)
	v, ok := rp.Car["SAG"]
	if !ok {
		return nil, errore(http.StatusConflict, "Manca la Saggezza nella scheda.")
	}
	e := TiroSalvezza(tiraDado, "SAG", v, slices.Contains(rp.TS, "SAG"), BonusCompetenza(rp.Livello), 0, 0, cd, "normale")
	d4 := 0
	if e.Successo {
		d4 = tiraDado(4)
	}
	nuova, guarita := RecuperoPazzia(cd, e.Successo, d4)
	s.DB.Exec(`UPDATE pazzie SET cd_recupero = ? WHERE id = ?`, nuova, id)
	if guarita {
		s.DB.Exec(`UPDATE pazzie SET attiva = 0, chiusa_il = ?, motivo_chiusura = 'recupero completato' WHERE id = ?`, db.Ora(), id)
	}
	out := map[string]any{"pazzia": id, "ts": e, "cdPrima": cd, "riduzione": d4, "cdDopo": nuova, "guarita": guarita, "dataGioco": in.DataGioco}
	s.registraSanita(pg, u, "pazzia_recupero", out)
	s.notificaSanita(pg, "pazzia")
	return out, nil
}

// ===== Catalogo dei testi delle tabelle (scritto dal Master) =====

func tabelleCatalogo() map[string][]string {
	cat := []string{}
	for _, c := range CategoriePazzia {
		cat = append(cat, c.Nome)
	}
	return map[string][]string{"homebrew": cat, "dmg_temporanea": {"temporanea"}, "dmg_duratura": {"duratura"}, "dmg_indeterminata": {"indeterminata"}}
}

func (s *Server) apiCatalogo(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	out := map[string]map[string][]string{}
	for t, cats := range tabelleCatalogo() {
		out[t] = map[string][]string{}
		for _, c := range cats {
			out[t][c] = make([]string, 10)
		}
	}
	rows, err := s.DB.Query(`SELECT tabella, categoria, voce, testo FROM pazzia_catalogo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t, c, x string
		var v int
		rows.Scan(&t, &c, &v, &x)
		if out[t] != nil && out[t][c] != nil {
			out[t][c][v-1] = x
		}
	}
	return out, nil
}

func (s *Server) apiSalvaCatalogo(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Tabella   string   `json:"tabella"`
		Categoria string   `json:"categoria"`
		Voci      []string `json:"voci"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if !slices.Contains(tabelleCatalogo()[in.Tabella], in.Categoria) || len(in.Voci) != 10 {
		return nil, errDatiNonValidi
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		for i, t := range in.Voci {
			t = strings.TrimSpace(t)
			if len([]rune(t)) > 1000 {
				return errore(http.StatusBadRequest, "Voce troppo lunga.")
			}
			if t == "" {
				tx.Exec(`DELETE FROM pazzia_catalogo WHERE tabella = ? AND categoria = ? AND voce = ?`, in.Tabella, in.Categoria, i+1)
				continue
			}
			if _, err := tx.Exec(`INSERT INTO pazzia_catalogo(tabella, categoria, voce, testo) VALUES(?,?,?,?)
				ON CONFLICT(tabella, categoria, voce) DO UPDATE SET testo = excluded.testo`, in.Tabella, in.Categoria, i+1, t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.registraSanita("", u, "catalogo", map[string]string{"tabella": in.Tabella, "categoria": in.Categoria})
	return nil, nil
}

// ===== Immagine della schermata (caricata dal Master) =====

func (s *Server) apiCaricaImmagineSanita(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	br := bufio.NewReaderSize(r.Body, 64<<10)
	inizio, _ := br.Peek(16)
	tipo, est := tipoMedia(inizio)
	if tipo != "img" {
		return nil, errore(http.StatusUnsupportedMediaType, "Carica un'immagine JPG, PNG, WebP o GIF.")
	}
	b, err := io.ReadAll(io.LimitReader(br, 20<<20+1))
	if err != nil || len(b) > 20<<20 {
		return nil, errore(http.StatusRequestEntityTooLarge, "Immagine troppo grande: massimo 20 MB.")
	}
	file := hex.EncodeToString(cripto.Casuali(8)) + "." + est
	p, err := s.percorsoFileDati(file)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return nil, err
	}
	if vecchio, _ := s.impostazione("sanita_immagine"); vecchio != "" {
		s.eliminaFileDati(vecchio)
	}
	if err := s.impostaImpostazione("sanita_immagine", file); err != nil {
		return nil, err
	}
	s.registraSanita("", u, "immagine", map[string]any{"byte": len(b)})
	return nil, nil
}

func (s *Server) immagineSanita(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	f, _ := s.impostazione("sanita_immagine")
	if u == nil || !u.Approvato() || f == "" {
		http.Error(w, "Immagine non caricata.", http.StatusNotFound)
		return
	}
	p, err := s.percorsoFileDati(f)
	if err != nil {
		http.Error(w, "Errore.", http.StatusInternalServerError)
		return
	}
	fh, err := os.Open(p)
	if err != nil {
		http.Error(w, "Immagine mancante.", http.StatusNotFound)
		return
	}
	defer fh.Close()
	w.Header().Set("Content-Type", tipiContenuto[f[strings.LastIndex(f, ".")+1:]])
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, "", time.Time{}, fh)
}
