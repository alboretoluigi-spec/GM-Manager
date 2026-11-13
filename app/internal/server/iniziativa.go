package server

// Tracker dell'iniziativa: combattimenti per sessione di gioco, turni e round, PG presi dalle schede,
// mostri (anche nascosti), danni e cure, condizioni, tiri contro la morte, riposo lungo, messaggi privati.
//
// Lo stato vive sul server (database): chiudere una pagina non interrompe lo scontro.
// Tutti i comandi passano da azioneIniziativa, che controlla i permessi (docs/FASE_2_PROGETTO.md, sez. 6):
//   - Master: tutto;
//   - Segretario del gruppo: turni, PF e condizioni di tutti i PG della sessione e dei mostri visibili;
//   - giocatore: iniziativa, PF, condizioni, tiri contro la morte e riposo del PROPRIO personaggio.
// Ognuno riceve solo ciò che può vedere: i mostri nascosti solo il Master; i PF esatti dei mostri solo
// Master e Segretario (gli altri vedono "illeso", "ferito"...); i messaggi privati solo i due interessati.
//
// I comandi arrivano dal canale in tempo reale (tipo = nome del comando) oppure da
// POST /api/v1/sessioni/{id}/iniziativa {"tipo": ..., "dati": {...}}.

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"slices"
	"sort"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

func (s *Server) rotteIniziativa(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/sessioni/{id}/combattimento", api(s.apiCombattimento))
	mux.Handle("POST /api/v1/sessioni/{id}/iniziativa", api(s.apiAzioneIniziativa))
	mux.Handle("GET /api/v1/sessioni/{id}/messaggi", api(s.apiMessaggi))
	mux.Handle("GET /api/v1/sessioni/{id}/combattenti/{c}/scheda", api(s.apiSchedaCombattente)) // scheda intera del mostro
	for _, t := range comandiIniziativa {
		tipo := t
		s.Hub.Registra(tipo, func(c *Cliente, dati json.RawMessage) (any, error) {
			return s.azioneIniziativa(c.U, tipo, "", dati)
		})
	}
	// Entrare in una sessione: iscrive la connessione al canale della sessione (solo se permesso).
	s.Hub.Registra("sessione.entra", func(c *Cliente, dati json.RawMessage) (any, error) {
		var in struct{ Sessione string }
		if json.Unmarshal(dati, &in) != nil {
			return nil, errDatiNonValidi
		}
		if _, ok := s.partecipa(c.U, in.Sessione); !ok {
			return nil, errNonTrovato
		}
		s.Hub.Disiscrivi(c, "sessione:")
		s.Hub.Iscrivi(c, "sessione:"+in.Sessione)
		return map[string]any{"vista": s.vistaCombattimento(in.Sessione, c.U), "messaggi": s.messaggiDi(in.Sessione, c.U)}, nil
	})
	s.DopoInGioco = s.ripubblicaPerPersonaggio
	// Dadi e tabella della sfortuna (portati dal ramo main-ke8sy4: dadi.go, sfortuna.go).
	s.rotteDadi()
	s.rotteSaluto() // saluto.go
	s.rotteSfortuna(mux)
	s.rotteIncantesimi() // lancio degli incantesimi dalla scheda (incantesimi_lancio.go)
}

var comandiIniziativa = []string{
	"iniz.nuovo", "iniz.aggiungi_pg", "iniz.aggiungi_mostri", "iniz.iniziativa", "iniz.avvia", "iniz.ordine_pari",
	"iniz.prossimo_turno", "iniz.turno_precedente", "iniz.nascondi", "iniz.rimuovi", "iniz.fine",
	"pg.danno", "pg.cura", "pg.temp", "pg.condizioni", "pg.ts_morte", "pg.riposo_lungo",
	"mostro.danno", "mostro.cura", "mostro.condizioni",
	"attacco.tira", "attacco.critico", "iniz.economia", // attacchi automatici ed economia del turno (attacchi.go)
	"incantesimo.ts", "incantesimo.applica", "incantesimo.cura", // incantesimi nello scontro (incantesimi_scontro.go)
	"effetto.termina", "mostro.incantesimo", // effetti in corso (effetti_scontro.go), slot dei mostri (attacchi.go)
	"effetto.nuovo", "pg.ira", // effetti che cambiano i valori, ira del barbaro (effetti_valori.go)
	"messaggio.invia", "messaggio.letto",
	"iniz.mira", // il Master sceglie il bersaglio di un mostro: «X prende di mira Y» sugli schermi (mira.go)
}

// ===== Vista =====

type vistaCombattente struct {
	ID           string   `json:"id"`
	Tipo         string   `json:"tipo"`
	Nome         string   `json:"nome"`
	Iniziativa   *int64   `json:"iniziativa"`
	Naturale     *int     `json:"naturale,omitempty"` // d20 del tiro d'iniziativa: 20 primo, 1 ultimo (iniziativa_pareggi.go)
	Nascosto     bool     `json:"nascosto,omitempty"`
	Personaggio  string   `json:"personaggio,omitempty"`
	Proprietario string   `json:"proprietario,omitempty"`
	Giocatore    string   `json:"giocatore,omitempty"`
	Mio          bool     `json:"mio,omitempty"`
	HaRitratto   bool     `json:"haRitratto,omitempty"`
	Classe       string   `json:"classe,omitempty"` // classe dal riepilogo della scheda (icona della classe)
	CA           *int     `json:"ca,omitempty"`
	CAOrig       *int     `json:"caOrig,omitempty"`    // CA senza gli effetti in corso (effetti_valori.go)
	EffettiCA    []string `json:"effettiCA,omitempty"` // effetti in corso che cambiano la CA
	Ira          bool     `json:"ira,omitempty"`       // PG con l'ira del barbaro (riepilogo della scheda): tasto «Ira»
	PF           *int64   `json:"pf,omitempty"`
	PFMax        *int     `json:"pfMax,omitempty"`
	PFTemp       int      `json:"pfTemp,omitempty"`
	Salute       string   `json:"salute,omitempty"` // per chi non vede i PF esatti di un mostro
	Condizioni   []string `json:"condizioni"`
	TSMorte      *struct {
		S int `json:"s"`
		F int `json:"f"`
	} `json:"tsMorte,omitempty"`
	BonusIniz   *int   `json:"bonusIniziativa,omitempty"`
	PercPassiva *int   `json:"percezionePassiva,omitempty"`
	Terrore     int    `json:"terrore,omitempty"` // livello di Terrore (Sanità mentale, se la scala è attiva)
	TerroreNome string `json:"terroreNome,omitempty"`
	// Attacchi automatici (attacchi.go): profili d'attacco (PG: dalla scheda; mostri: letti dal bestiario), attacchi per
	// azione e già fatti in questo turno. I profili li vedono il Master e il proprietario del PG.
	Attacchi  []profiloAttacco `json:"attacchi,omitempty"`
	PerAzione int              `json:"attacchiPerAzione,omitempty"`
	// Multiattacco con attacchi diversi (attacchi.go): sequenza e volte già usate in questo turno (solo il Master)
	Multiattacco []passoMulti   `json:"multiattacco,omitempty"`
	Usati        int            `json:"attacchiUsati,omitempty"`
	Economia     *economiaTurno `json:"economia,omitempty"` // azione, azione bonus, reazione già usate in questo turno
	// incantesimi dei mostri con slot e usi già spesi (solo il Master)
	Incantesimi *incantesimiMostro `json:"incantesimi,omitempty"`
	// incantesimi automatici dei PG e slot (incantesimi_remoto.go): al Master e al proprietario
	IncAuto    []incAuto `json:"incantesimiAuto,omitempty"`
	Slot       []slotPG  `json:"slot,omitempty"`
	slotTot    map[string]int
	spareggio  int
	ordinePari int // ordine deciso a mano in un pareggio (iniziativa_pareggi.go)
	difese     difeseCombattente
	usati      int
	turnoUsati int
	mod        *modAttive // modifiche degli effetti in corso (effetti_valori.go)
	desMod     *int       // modificatore di Destrezza del PG (Armatura magica)
}

type vistaComb struct {
	Sessione      string `json:"sessione"`
	Combattimento *struct {
		ID      string `json:"id"`
		Round   int    `json:"round"`
		Turni   int    `json:"turni"`
		TurnoDi string `json:"turnoDi"`
		Avviato bool   `json:"avviato"`
	} `json:"combattimento"`
	Combattenti []*vistaCombattente `json:"combattenti"`
	Ruolo       string              `json:"ruolo"` // master, segretario, giocatore
	Registro    []string            `json:"registro,omitempty"`
	// solo il Master: incantesimi con tiri salvezza che aspettano la sua conferma (incantesimi_scontro.go)
	IncInAttesa []esitoIncantesimo `json:"incantesimiInAttesa,omitempty"`
	// effetti in corso (effetti_scontro.go): ai giocatori senza quelli dei mostri nascosti
	Effetti []vistaEffetto `json:"effetti,omitempty"`
	// effetti che durano dopo gli scontri, sui PG dello scontro (effetti_persistenti.go)
	Durevoli []effettoDurevole `json:"durevoli,omitempty"`
	// pareggi d'iniziativa da decidere (iniziativa_pareggi.go)
	Pareggi []gruppoPari `json:"pareggi,omitempty"`
}

// ruoloIn: master, segretario o giocatore nella sessione (o "" se non partecipa).
func (s *Server) ruoloIn(u *Utente, sessione string) (ruolo, gruppo string) {
	g, ok := s.partecipa(u, sessione)
	if !ok {
		return "", ""
	}
	if u.Master() {
		return "master", g
	}
	if s.segretario(u, g) {
		return "segretario", g
	}
	return "giocatore", g
}

func (s *Server) combattimentoAttivo(q interface {
	QueryRow(string, ...any) *sql.Row
}, sessione string) (id string, round, turni int, turnoDi string, err error) {
	var td sql.NullString
	err = q.QueryRow(`SELECT id, round, turni, turno_di FROM combattimenti WHERE sessione_id = ? AND attivo = 1`, sessione).Scan(&id, &round, &turni, &td)
	turnoDi = td.String
	return
}

// caricaCombattenti legge tutti i combattenti (senza filtri) in ordine di iniziativa.
func (s *Server) caricaCombattenti(comb string) ([]*vistaCombattente, error) {
	rows, err := s.DB.Query(`SELECT c.id, c.tipo, c.nome, c.iniziativa, c.spareggio, c.nascosto, c.ca, c.pf, c.pf_max, c.condizioni,
		COALESCE(c.personaggio_id,''), COALESCE(p.proprietario_id,''), COALESCE(u.nome,''), COALESCE(v.riepilogo,'{}'),
		COALESCE(sg.pf, -100000), COALESCE(sg.pf_temp, 0), COALESCE(sg.condizioni,'[]'), COALESCE(sg.ts_morte,'{}'),
		p.id IS NOT NULL AND v.dati LIKE '%"img":"data:image/%', c.profilo, c.attacchi_usati, c.attacchi_turno, c.economia,
		c.iniz_naturale, c.ordine_pari
		FROM combattenti c
		LEFT JOIN personaggi p ON p.id = c.personaggio_id
		LEFT JOIN utenti u ON u.id = p.proprietario_id
		LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		LEFT JOIN stato_in_gioco sg ON sg.personaggio_id = p.id
		WHERE c.combattimento_id = ?`, comb)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*vistaCombattente
	for rows.Next() {
		v := &vistaCombattente{}
		var ini sql.NullInt64
		var nasc int
		var ca, pf, pfMax int
		var condM, riep, condPG, ts string
		var pfPG int64
		var temp int
		var profilo, eco string
		var nat sql.NullInt64
		if err := rows.Scan(&v.ID, &v.Tipo, &v.Nome, &ini, &v.spareggio, &nasc, &ca, &pf, &pfMax, &condM,
			&v.Personaggio, &v.Proprietario, &v.Giocatore, &riep, &pfPG, &temp, &condPG, &ts, &v.HaRitratto,
			&profilo, &v.usati, &v.turnoUsati, &eco, &nat, &v.ordinePari); err != nil {
			return nil, err
		}
		if ini.Valid {
			v.Iniziativa = &ini.Int64
			if nat.Valid {
				v.Naturale = intP(int(nat.Int64))
			}
		}
		v.Economia = &economiaTurno{}
		json.Unmarshal([]byte(eco), v.Economia)
		v.Nascosto = nasc == 1
		if v.Tipo == "mostro" {
			v.CA, v.PFMax = &ca, &pfMax
			p := int64(pf)
			v.PF = &p
			json.Unmarshal([]byte(condM), &v.Condizioni)
			v.impostaAttacchi(profilo, nil)
		} else {
			var r riepilogo
			json.Unmarshal([]byte(riep), &r)
			v.impostaAttacchi("", &r)
			v.CA, v.PFMax, v.BonusIniz, v.PercPassiva = intP(r.CA), intP(r.PFMax), intP(r.Iniziativa), intP(r.PercPass)
			v.Classe = r.Classe
			v.Ira = r.Ira != nil
			v.IncAuto, v.slotTot = r.IncAuto, r.Slot
			if r.Des > 0 {
				v.desMod = intP(Modificatore(r.Des))
			}
			if pfPG == -100000 { // PF attuali non impostati: valgono i massimi
				m := int64(r.PFMax)
				v.PF = &m
			} else {
				v.PF = &pfPG
			}
			v.PFTemp = temp
			json.Unmarshal([]byte(condPG), &v.Condizioni)
			var t struct{ S, F int }
			json.Unmarshal([]byte(ts), &t)
			v.TSMorte = &struct {
				S int `json:"s"`
				F int `json:"f"`
			}{t.S, t.F}
		}
		if v.Condizioni == nil {
			v.Condizioni = []string{}
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	s.applicaEffettiValori(comb, out) // CA, difese e bonus degli effetti in corso
	ordina(out)
	return out, nil
}

func intP(n int) *int { return &n }

// ordina: 20 naturale prima di tutti, 1 naturale dopo tutti; poi iniziativa più alta; a parità l'ordine deciso a mano
// (iniziativa_pareggi.go), poi la Destrezza (spareggio), poi il nome. Chi non ha ancora un'iniziativa va in fondo.
func ordina(l []*vistaCombattente) {
	sort.SliceStable(l, func(i, j int) bool {
		a, b := l[i], l[j]
		if (a.Iniziativa == nil) != (b.Iniziativa == nil) {
			return a.Iniziativa != nil
		}
		if pa, pb := prioritaNaturale(a), prioritaNaturale(b); pa != pb {
			return pa > pb
		}
		if a.Iniziativa != nil && *a.Iniziativa != *b.Iniziativa {
			return *a.Iniziativa > *b.Iniziativa
		}
		if a.ordinePari != b.ordinePari {
			return a.ordinePari > b.ordinePari
		}
		if a.spareggio != b.spareggio {
			return a.spareggio > b.spareggio
		}
		return a.Nome < b.Nome
	})
}

// salute descrive un mostro senza rivelarne i PF.
func salute(pf, max int64) string {
	switch {
	case pf <= 0:
		return "a terra"
	case pf >= max:
		return "illeso"
	case pf*2 > max:
		return "ferito"
	default:
		return "gravemente ferito"
	}
}

// vistaCombattimento costruisce ciò che l'utente può vedere.
func (s *Server) vistaCombattimento(sessione string, u *Utente) *vistaComb {
	ruolo, _ := s.ruoloIn(u, sessione)
	uid := ""
	if u != nil {
		uid = u.ID
	}
	return s.vistaPerRuolo(sessione, ruolo, uid)
}

// vistaPerRuolo filtra il combattimento per un ruolo: master, segretario, giocatore oppure "remoto"
// (vista dei giocatori, ma con il proprietario dei PG, che il ponte MQTT sostituisce con un alias).
func (s *Server) vistaPerRuolo(sessione, ruolo, uid string) *vistaComb {
	remoto := ruolo == "remoto"
	if remoto {
		ruolo = "giocatore"
	}
	v := &vistaComb{Sessione: sessione, Ruolo: ruolo, Combattenti: []*vistaCombattente{}}
	if ruolo == "" {
		return v
	}
	id, round, turni, turnoDi, err := s.combattimentoAttivo(s.DB, sessione)
	if err != nil {
		return v
	}
	tutti, err := s.caricaCombattenti(id)
	if err != nil {
		return v
	}
	v.Combattimento = &struct {
		ID      string `json:"id"`
		Round   int    `json:"round"`
		Turni   int    `json:"turni"`
		TurnoDi string `json:"turnoDi"`
		Avviato bool   `json:"avviato"`
	}{id, round, turni, turnoDi, round > 0}
	terrore := s.configSanita().Terrore // scala del Terrore attiva (sanita.go)
	for _, c := range tutti {
		if c.Nascosto && ruolo != "master" {
			if c.ID == turnoDi {
				v.Combattimento.TurnoDi = "" // non si rivela di chi è il turno
			}
			continue
		}
		cc := *c
		cc.Mio = c.Proprietario != "" && c.Proprietario == uid
		if c.turnoUsati == turni {
			cc.Usati = c.usati
		}
		if ruolo != "master" && !cc.Mio { // i profili d'attacco: al Master e al proprietario del PG
			cc.Attacchi, cc.PerAzione, cc.Usati, cc.Multiattacco, cc.IncAuto = nil, 0, 0, nil, nil
		} else if len(cc.IncAuto) > 0 {
			cc.Slot = s.slotDelPG(c.Personaggio, c.slotTot)
		}
		if c.turnoUsati != turni && cc.Economia != nil { // un altro turno: il conto del Multiattacco riparte
			e := *cc.Economia
			e.Seq = nil
			cc.Economia = &e
		}
		if ruolo != "master" {
			cc.Incantesimi = nil
		}
		if c.Tipo == "mostro" && ruolo == "giocatore" { // le azioni dei mostri: Master e Segretario
			cc.Economia = nil
		}
		if c.Tipo == "mostro" && ruolo == "giocatore" {
			cc.Salute = salute(*c.PF, int64(*c.PFMax))
			cc.PF, cc.PFMax, cc.CA, cc.CAOrig, cc.EffettiCA = nil, nil, nil, nil, nil
		}
		if ruolo != "master" && !remoto {
			cc.Proprietario = ""
		}
		if terrore && c.Tipo == "pg" && c.Personaggio != "" {
			if t := s.leggiSanitaPG(c.Personaggio).terrore; t > 0 && t < len(LivelliTerrore) {
				cc.Terrore, cc.TerroreNome = t, LivelliTerrore[t].Nome
			}
		}
		v.Combattenti = append(v.Combattenti, &cc)
	}
	v.Effetti = effettiVisibili(s.caricaEffetti(id), tutti, ruolo)
	pgs := []string{}
	for _, c := range tutti {
		if c.Tipo == "pg" && c.Personaggio != "" {
			pgs = append(pgs, c.Personaggio)
		}
	}
	v.Durevoli = s.durevoliVisibili(pgs, ruolo, uid)
	v.Pareggi = pareggiPerRuolo(tutti, ruolo, uid)
	if ruolo == "master" {
		v.IncInAttesa = s.incantesimiInAttesa(id)
		rows, err := s.DB.Query(`SELECT testo FROM registro_combattimento WHERE combattimento_id = ? ORDER BY id DESC LIMIT 30`, id)
		if err == nil {
			for rows.Next() {
				var t string
				rows.Scan(&t)
				v.Registro = append(v.Registro, t)
			}
			rows.Close()
		}
	}
	return v
}

// pubblica invia a ogni connessione della sessione la sua vista.
func (s *Server) pubblicaCombattimento(sessione string) {
	s.remoto.pubblica(sessione)
	s.Hub.PubblicaPer("sessione:"+sessione, func(c *Cliente) *Msg {
		return &Msg{Tipo: "iniz.stato", Dati: s.vistaCombattimento(sessione, c.U)}
	})
}

// ripubblicaPerPersonaggio aggiorna i tracker delle sessioni in cui combatte un PG (PF cambiati dalla scheda).
func (s *Server) ripubblicaPerPersonaggio(pg string) {
	rows, err := s.DB.Query(`SELECT DISTINCT k.sessione_id FROM combattenti c JOIN combattimenti k ON k.id = c.combattimento_id
		WHERE c.personaggio_id = ? AND k.attivo = 1`, pg)
	if err != nil {
		return
	}
	var ses []string
	for rows.Next() {
		var x string
		rows.Scan(&x)
		ses = append(ses, x)
	}
	rows.Close()
	for _, x := range ses {
		s.pubblicaCombattimento(x)
	}
}

// ===== API =====

func (s *Server) apiCombattimento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	if _, ok := s.partecipa(u, r.PathValue("id")); !ok {
		return nil, errNonTrovato
	}
	return s.vistaCombattimento(r.PathValue("id"), u), nil
}

func (s *Server) apiAzioneIniziativa(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	var in struct {
		Tipo string
		Dati json.RawMessage
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	known := false
	for _, t := range comandiIniziativa {
		known = known || t == in.Tipo
	}
	if !known {
		return nil, errDatiNonValidi
	}
	return s.azioneIniziativa(u, in.Tipo, r.PathValue("id"), in.Dati)
}

func (s *Server) apiMessaggi(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	if _, ok := s.partecipa(u, r.PathValue("id")); !ok {
		return nil, errNonTrovato
	}
	return s.messaggiDi(r.PathValue("id"), u), nil
}

// ===== Comandi =====

type datiAzione struct {
	Sessione    string   `json:"sessione"`
	Combattente string   `json:"combattente"`
	Personaggio string   `json:"personaggio"`
	Valore      *int     `json:"valore"`
	Tira        bool     `json:"tira"`
	Nascosto    *bool    `json:"nascosto"`
	Nome        string   `json:"nome"`
	Quanti      int      `json:"quanti"`
	CA          int      `json:"ca"`
	PF          int      `json:"pf"`
	BonusIniz   int      `json:"bonusIniziativa"`
	GS          *float64 `json:"gs"` // Grado di Sfida del mostro (ricompense automatiche)
	Condizioni  []string `json:"condizioni"`
	S           *int     `json:"s"`
	F           *int     `json:"f"`
	A           string   `json:"a"`
	Testo       string   `json:"testo"`
	Messaggio   string   `json:"messaggio"`
	// mostri aggiunti dal bestiario: attacchi e difese letti dalla scheda (attacchi.go)
	Profilo json.RawMessage `json:"profilo"`
	// attacchi automatici (attacchi.go)
	Bersaglio string `json:"bersaglio"`
	Attacco   int    `json:"attacco"`
	Modo      string `json:"modo"`
	Extra     int    `json:"extra"`
	Azione    string `json:"azione"`
	Forza     bool   `json:"forza"`
	Gettone   string `json:"gettone"`
	// economia del turno segnata a mano (iniz.economia)
	Economia *economiaTurno `json:"economia"`
	// incantesimi nello scontro (incantesimi_scontro.go)
	Incantesimo   *incantesimoScontro `json:"incantesimo"`
	UsaSlot       bool                `json:"usaSlot"` // il server toglie lo slot del PG (incantesimi_remoto.go)
	Bersagli      []string            `json:"bersagli"`
	Riusciti      map[string]bool     `json:"riusciti"`
	ConCondizioni bool                `json:"applicaCondizioni"`
	Effetto       string              `json:"effetto"`   // effetto.termina
	Valori        *effettoValori      `json:"valori"`    // effetto.nuovo (effetti_valori.go)
	DadiDanni     []int               `json:"dadiDanni"` // attacco.tira: dadi dei danni scelti nella conferma (dadi_conferma.go)
	Gruppo        *int                `json:"gruppo"`    // mostro.incantesimo: gruppo di incantesimi (slot)
	// tiri d'iniziativa e pareggi (iniziativa_pareggi.go)
	Naturale *int            `json:"naturale"` // d20 di un'iniziativa scritta a mano (20 primo, 1 ultimo)
	Ordine   []string        `json:"ordine"`   // iniz.ordine_pari: dal primo all'ultimo
	Scheda   json.RawMessage `json:"scheda"`   // iniz.aggiungi_mostri: scheda intera della creatura
}

var errNessunCombattimento = errore(http.StatusConflict, "Non c'è un combattimento in corso in questa sessione.")

// azioneIniziativa esegue un comando dopo aver controllato i permessi. sessione può arrivare dal
// percorso (API) o dai dati (tempo reale).
func (s *Server) azioneIniziativa(u *Utente, tipo, sessione string, raw json.RawMessage) (any, error) {
	if u == nil || !u.Approvato() || !s.utenteAncoraValido(u.ID) {
		return nil, errVietato
	}
	var d datiAzione
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, errDatiNonValidi
		}
	}
	if sessione == "" {
		sessione = d.Sessione
	}
	ruolo, gruppo := s.ruoloIn(u, sessione)
	if ruolo == "" {
		return nil, errNonTrovato
	}
	master := ruolo == "master"
	segr := ruolo == "segretario"

	if strings.HasPrefix(tipo, "messaggio.") {
		return s.azioneMessaggio(u, tipo, sessione, gruppo, master, d)
	}
	if tipo == "iniz.nuovo" {
		if !master {
			return nil, errVietato
		}
		return s.nuovoCombattimento(u, sessione)
	}

	comb, round, turni, turnoDi, err := s.combattimentoAttivo(s.DB, sessione)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNessunCombattimento
	}
	if err != nil {
		return nil, err
	}
	// Combattente indicato (deve appartenere a questo combattimento).
	var cb *vistaCombattente
	tutti, err := s.caricaCombattenti(comb)
	if err != nil {
		return nil, err
	}
	trova := func() error {
		for _, c := range tutti {
			if c.ID == d.Combattente || d.Combattente == "" && d.Personaggio != "" && c.Personaggio == d.Personaggio {
				cb = c
				return nil
			}
		}
		return errNonTrovato
	}
	mioPG := func() bool { return cb != nil && cb.Tipo == "pg" && cb.Proprietario == u.ID }
	// Chi può toccare un PG: proprietario, Segretario, Master. Un mostro: Master; Segretario se visibile.
	puoPG := func() bool { return cb != nil && cb.Tipo == "pg" && (master || segr || mioPG()) }
	puoMostro := func() bool { return cb != nil && cb.Tipo == "mostro" && (master || segr && !cb.Nascosto) }

	registro := func(t string) {
		s.DB.Exec(`INSERT INTO registro_combattimento(combattimento_id, autore_id, testo, creato_il) VALUES(?,?,?,?)`, comb, u.ID, u.Nome+": "+t, db.Ora())
	}
	fatto := func() (any, error) {
		s.pubblicaCombattimento(sessione)
		return nil, nil
	}

	switch tipo {
	case "iniz.aggiungi_pg":
		if !master {
			return nil, errVietato
		}
		if err := s.aggiungiPG(comb, sessione, d.Personaggio); err != nil {
			return nil, err
		}
		return fatto()

	case "iniz.aggiungi_mostri":
		if !master {
			return nil, errVietato
		}
		nome, err := rigaSingola(d.Nome, "Nome", 1, 40)
		if err != nil {
			return nil, err
		}
		if d.Quanti < 1 || d.Quanti > 20 || d.CA < 0 || d.CA > 40 || d.PF < 1 || d.PF > 2000 || d.BonusIniz < -10 || d.BonusIniz > 20 {
			return nil, errore(http.StatusBadRequest, "Valori non validi: da 1 a 20 mostri, CA 0-40, PF 1-2000.")
		}
		nasc := 0
		if d.Nascosto != nil && *d.Nascosto {
			nasc = 1
		}
		gs := 0.0
		if d.GS != nil {
			if *d.GS < 0 || *d.GS > 30 {
				return nil, errore(http.StatusBadRequest, "Il GS va da 0 a 30.")
			}
			gs = *d.GS
		}
		profilo := validaProfiloMostro(d.Profilo)
		schedaM := validaSchedaMostro(d.Scheda)
		for i := 1; i <= d.Quanti; i++ {
			n := nome
			if d.Quanti > 1 {
				n = nome + " " + itoa(i)
			}
			var ini, nat any
			if round > 0 || d.Tira { // a combattimento avviato i nuovi arrivati tirano subito, ognuno il suo d20
				d20 := dado(20)
				ini, nat = d20+d.BonusIniz, d20
				registro(n + ": iniziativa d20 (" + itoa(d20) + ") " + conSegno(d.BonusIniz) + " = " + itoa(d20+d.BonusIniz) + testoNaturale(d20))
			}
			if _, err := s.DB.Exec(`INSERT INTO combattenti(id, combattimento_id, tipo, nome, iniziativa, iniz_naturale, spareggio, nascosto, ca, pf, pf_max, gs, profilo, scheda_mostro, creato_il)
				VALUES(?,?,'mostro',?,?,?,?,?,?,?,?,?,?,?,?)`, cripto.ID(), comb, n, ini, nat, 10+2*d.BonusIniz, nasc, d.CA, d.PF, d.PF, gs, profilo, schedaM, db.Ora()); err != nil {
				return nil, err
			}
		}
		registro("aggiunge " + itoa(d.Quanti) + " × " + nome)
		return fatto()

	case "iniz.iniziativa":
		if err := trova(); err != nil {
			return nil, err
		}
		if !(master || mioPG()) {
			return nil, errVietato
		}
		var v int
		var nat any // d20 del tiro: con il tiro di Gaster Bless si sa; scritto a mano solo se indicato
		testoNat := ""
		if d.Tira {
			b := 0
			if cb.BonusIniz != nil {
				b = *cb.BonusIniz
			} else if cb.Tipo == "mostro" {
				b = (cb.spareggio - 10) / 2
			}
			d20 := dado(20)
			v, nat, testoNat = d20+b, d20, " (d20 "+itoa(d20)+")"+testoNaturale(d20)
		} else if d.Valore != nil && *d.Valore >= -10 && *d.Valore <= 50 {
			v = *d.Valore
			if d.Naturale != nil && *d.Naturale >= 1 && *d.Naturale <= 20 {
				nat, testoNat = *d.Naturale, testoNaturale(*d.Naturale)
			}
		} else {
			return nil, errore(http.StatusBadRequest, "Iniziativa non valida.")
		}
		// l'ordine deciso in un pareggio non vale più: con il nuovo valore si decide di nuovo
		s.DB.Exec(`UPDATE combattenti SET iniziativa = ?, iniz_naturale = ?, ordine_pari = 0 WHERE id = ?`, v, nat, cb.ID)
		registro(cb.Nome + ": iniziativa " + itoa(v) + testoNat)
		s.pubblicaCombattimento(sessione)
		return map[string]int{"iniziativa": v}, nil

	case "iniz.ordine_pari":
		return s.ordinePari(u, sessione, comb, master, tutti, d.Ordine, registro)

	case "iniz.avvia":
		if !master {
			return nil, errVietato
		}
		// Chi non ha ancora un'iniziativa tira ora (PG compresi, con il loro bonus).
		for _, c := range tutti {
			if c.Iniziativa == nil {
				b := 0
				if c.BonusIniz != nil {
					b = *c.BonusIniz
				} else if c.Tipo == "mostro" {
					b = (c.spareggio - 10) / 2
				}
				d20 := dado(20)
				s.DB.Exec(`UPDATE combattenti SET iniziativa = ?, iniz_naturale = ?, ordine_pari = 0 WHERE id = ?`, d20+b, d20, c.ID)
				registro("iniziativa di " + c.Nome + ": d20 (" + itoa(d20) + ") " + conSegno(b) + " = " + itoa(d20+b) + testoNaturale(d20))
			}
		}
		tutti, _ = s.caricaCombattenti(comb)
		if len(tutti) == 0 {
			return nil, errore(http.StatusBadRequest, "Aggiungi almeno un combattente.")
		}
		// pareggi con un PG: prima si decide l'ordine (il Master può partire lo stesso: vale la Destrezza)
		if p := pareggi(tutti); len(p) > 0 && !d.Forza {
			s.pubblicaCombattimento(sessione)
			return map[string]any{"pareggi": len(p), "testo": "Ci sono pareggi d'iniziativa da decidere: l'ordine lo sceglie il Master (con i mostri) o i giocatori (tra loro). Poi «Inizia» di nuovo."}, nil
		}
		s.DB.Exec(`UPDATE combattimenti SET round = 1, turni = 1, turno_di = ? WHERE id = ?`, tutti[0].ID, comb)
		s.DB.Exec(`UPDATE combattenti SET economia = '{}', attacchi_usati = 0, attacchi_turno = -1 WHERE combattimento_id = ?`, comb)
		registro("inizia il combattimento")
		s.avvisaTurno(tutti[0], sessione)
		s.chiamaGiocatori(sessione, "iniz.chiamata", map[string]any{"fase": "inizio"})
		mus := s.musicaScontro(true) // musica dello scontro (mappa_musica.go)
		s.pubblicaCombattimento(sessione)
		return map[string]any{"musica": mus}, nil

	case "iniz.prossimo_turno", "iniz.turno_precedente":
		if !(master || segr) {
			return nil, errVietato
		}
		if round == 0 || len(tutti) == 0 {
			return nil, errore(http.StatusConflict, "Il combattimento non è ancora iniziato.")
		}
		i := 0
		for k, c := range tutti {
			if c.ID == turnoDi {
				i = k
			}
		}
		finito := tutti[i] // il turno che finisce (TS ripetuti degli effetti in corso)
		if tipo == "iniz.prossimo_turno" {
			i++
			turni++
			if i >= len(tutti) {
				i = 0
				round++
			}
		} else {
			if turni <= 1 {
				return nil, errore(http.StatusConflict, "Siamo al primo turno.")
			}
			i--
			turni--
			if i < 0 {
				i = len(tutti) - 1
				round--
			}
		}
		s.DB.Exec(`UPDATE combattimenti SET round = ?, turni = ?, turno_di = ? WHERE id = ?`, round, turni, tutti[i].ID, comb)
		if tipo == "iniz.prossimo_turno" { // all'inizio del suo turno tornano azione, azione bonus e reazione
			s.DB.Exec(`UPDATE combattenti SET economia = '{}' WHERE id = ?`, tutti[i].ID)
			s.effettiDopoTurno(u, comb, finito, tutti[i], round)
			s.avvisaTurno(tutti[i], sessione)
		}
		return fatto()

	case "iniz.nascondi":
		if err := trova(); err != nil {
			return nil, err
		}
		if !master || d.Nascosto == nil {
			return nil, errVietato
		}
		v := 0
		if *d.Nascosto {
			v = 1
		}
		s.DB.Exec(`UPDATE combattenti SET nascosto = ? WHERE id = ?`, v, cb.ID)
		return fatto()

	case "iniz.rimuovi":
		if err := trova(); err != nil {
			return nil, err
		}
		if !master {
			return nil, errVietato
		}
		if cb.ID == turnoDi && len(tutti) > 1 { // il turno passa al successivo
			for k, c := range tutti {
				if c.ID == cb.ID {
					s.DB.Exec(`UPDATE combattimenti SET turno_di = ? WHERE id = ?`, tutti[(k+1)%len(tutti)].ID, comb)
					s.DB.Exec(`UPDATE combattenti SET economia = '{}' WHERE id = ?`, tutti[(k+1)%len(tutti)].ID)
				}
			}
		}
		s.DB.Exec(`DELETE FROM combattenti WHERE id = ?`, cb.ID)
		registro("toglie " + cb.Nome)
		s.controllaConcentrazioni(u, comb) // i suoi effetti a concentrazione finiscono
		return fatto()

	case "iniz.fine":
		if !master {
			return nil, errVietato
		}
		s.effettiAFineScontro(u, comb) // gli effetti brevi finiscono con lo scontro
		s.DB.Exec(`UPDATE combattimenti SET attivo = 0 WHERE id = ?`, comb)
		registro("fine del combattimento")
		s.musicaScontro(false) // torna la musica di prima
		return fatto()

	case "pg.danno", "pg.cura", "pg.temp", "pg.condizioni", "pg.ts_morte", "pg.riposo_lungo":
		if err := trova(); err != nil {
			return nil, err
		}
		if !puoPG() {
			return nil, errVietato
		}
		if tipo == "pg.riposo_lungo" && !(master || mioPG()) {
			return nil, errVietato
		}
		if err := s.modificaPG(u, cb, tipo, d); err != nil {
			return nil, err
		}
		if tipo == "pg.condizioni" || tipo == "pg.danno" {
			s.controllaConcentrazioni(u, comb) // tolto «Concentrato» (o a 0 PF): finiscono i suoi effetti
		}
		if tipo == "pg.riposo_lungo" {
			s.fineDurevoliRiposo(u, cb.Personaggio) // finiscono i suoi effetti fino a 24 ore (effetti_persistenti.go)
		}
		if tipo == "pg.danno" || tipo == "pg.cura" {
			registro(cb.Nome + map[string]string{"pg.danno": ": −", "pg.cura": ": +"}[tipo] + itoa(*d.Valore) + " PF")
		}
		var prop string
		s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, cb.Personaggio).Scan(&prop)
		s.avvisaStatoInGioco(cb.Personaggio, prop) // scheda aperta + tracker (tramite DopoInGioco)
		return nil, nil

	case "attacco.tira", "attacco.critico":
		return s.attacca(u, tipo, sessione, comb, ruolo, d, registro)

	case "iniz.mira":
		if err := trova(); err != nil {
			return nil, err
		}
		if !master || cb.Tipo != "mostro" {
			return nil, errVietato
		}
		return s.prendeDiMira(sessione, cb, tutti, d)

	case "incantesimo.ts", "incantesimo.applica", "incantesimo.cura":
		return s.incantesimoScontro(u, tipo, sessione, comb, ruolo, d, registro)

	case "mostro.incantesimo": // il Master spende uno slot (o un uso al giorno) di un mostro
		if err := trova(); err != nil {
			return nil, err
		}
		if !master || cb.Tipo != "mostro" || d.Gruppo == nil {
			return nil, errVietato
		}
		r, err := s.usaIncantesimoMostro(u, cb, *d.Gruppo, d.Forza, registro)
		if err == nil {
			s.pubblicaCombattimento(sessione)
		}
		return r, err

	case "effetto.termina": // il Master termina un effetto in corso; il giocatore i suoi senza condizioni (ira, Scudo della fede…)
		for _, e := range s.caricaEffetti(comb) {
			if e.ID != d.Effetto {
				continue
			}
			motivo := "terminato dal Master"
			if !master {
				d.Combattente, d.Personaggio = e.Incantatore, ""
				if trova() != nil || !puoPG() || slices.ContainsFunc(e.Bersagli, func(b bersaglioEffetto) bool { return len(b.Condizioni) > 0 }) {
					return nil, errVietato
				}
				motivo = "terminato da " + u.Nome
			}
			s.terminaEffetto(u, comb, e, motivo)
			return fatto()
		}
		return nil, errNonTrovato

	case "effetto.nuovo": // effetto che cambia i valori (incantesimo, Master): effetti_valori.go
		if err := trova(); err != nil {
			return nil, err
		}
		if !(puoPG() || cb.Tipo == "mostro" && master) {
			return nil, errVietato
		}
		return s.nuovoEffettoValori(u, sessione, comb, round, turnoDi, master, cb, tutti, d, registro)

	case "pg.ira": // il barbaro entra in ira (o ne esce): effetti_valori.go
		if err := trova(); err != nil {
			return nil, err
		}
		if !puoPG() {
			return nil, errVietato
		}
		return s.iraPG(u, sessione, comb, round, turnoDi, master, cb, tutti, d, registro)

	case "iniz.economia": // segna a mano azione, azione bonus, reazione (PG: proprietario, Segretario, Master; mostri: Master)
		if err := trova(); err != nil {
			return nil, err
		}
		if d.Economia == nil {
			return nil, errDatiNonValidi
		}
		if !(puoPG() || cb.Tipo == "mostro" && master) {
			return nil, errVietato
		}
		b, _ := json.Marshal(d.Economia)
		s.DB.Exec(`UPDATE combattenti SET economia = ? WHERE id = ?`, string(b), cb.ID)
		return fatto()

	case "mostro.danno", "mostro.cura", "mostro.condizioni":
		if err := trova(); err != nil {
			return nil, err
		}
		if !puoMostro() {
			return nil, errVietato
		}
		switch tipo {
		case "mostro.condizioni":
			c, err := condizioniValide(d.Condizioni)
			if err != nil {
				return nil, err
			}
			b, _ := json.Marshal(c)
			s.DB.Exec(`UPDATE combattenti SET condizioni = ? WHERE id = ?`, string(b), cb.ID)
			s.controllaConcentrazioni(u, comb)
		default:
			if d.Valore == nil || *d.Valore < 0 || *d.Valore > 9999 {
				return nil, errDatiNonValidi
			}
			pf := *cb.PF
			if tipo == "mostro.danno" {
				pf = max(0, pf-int64(*d.Valore))
			} else {
				pf = min(int64(*cb.PFMax), pf+int64(*d.Valore))
			}
			s.DB.Exec(`UPDATE combattenti SET pf = ? WHERE id = ?`, pf, cb.ID)
			registro(cb.Nome + map[string]string{"mostro.danno": ": −", "mostro.cura": ": +"}[tipo] + itoa(*d.Valore) + " PF")
			if tipo == "mostro.danno" && pf == 0 && *cb.PF > 0 { // mostro sconfitto: ricompensa al gruppo
				if t := s.ricompensaMostro(comb, cb.ID, u); t != "" {
					registro(t)
				}
			}
		}
		return fatto()
	}
	return nil, errDatiNonValidi
}

func (s *Server) nuovoCombattimento(u *Utente, sessione string) (any, error) {
	var stato string
	s.DB.QueryRow(`SELECT stato FROM sessioni_gioco WHERE id = ?`, sessione).Scan(&stato)
	if stato == "conclusa" {
		return nil, errore(http.StatusConflict, "La sessione è conclusa.")
	}
	id := cripto.ID()
	err := db.Tx(context.Background(), s.DB, func(tx *sql.Tx) error {
		tx.Exec(`UPDATE combattimenti SET attivo = 0 WHERE sessione_id = ? AND attivo = 1`, sessione)
		if _, err := tx.Exec(`INSERT INTO combattimenti(id, sessione_id, creato_il) VALUES(?,?,?)`, id, sessione, db.Ora()); err != nil {
			return err
		}
		// Entrano subito i PG dei partecipanti che hanno scelto il personaggio.
		_, err := tx.Exec(`INSERT INTO combattenti(id, combattimento_id, tipo, personaggio_id, nome, spareggio, creato_il)
			SELECT lower(hex(randomblob(8))), ?, 'pg', p.id, p.nome, 0, ? FROM partecipanti x JOIN personaggi p ON p.id = x.personaggio_id
			WHERE x.sessione_id = ? AND p.stato = 'attivo'`, id, db.Ora(), sessione)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.aggiornaSpareggi(id)
	s.DB.Exec(`INSERT INTO registro_combattimento(combattimento_id, autore_id, testo, creato_il) VALUES(?,?,?,?)`, id, u.ID, u.Nome+": nuovo combattimento", db.Ora())
	s.pubblicaCombattimento(sessione)
	s.chiamaGiocatori(sessione, "iniz.chiamata", map[string]any{"fase": "preparazione"})
	return map[string]string{"id": id}, nil
}

// aggiungiPG aggiunge un personaggio del gruppo della sessione.
func (s *Server) aggiungiPG(comb, sessione, pg string) error {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM personaggi p JOIN sessioni_gioco x ON x.id = ? JOIN iscrizioni i ON i.gruppo_id = x.gruppo_id AND i.utente_id = p.proprietario_id
		WHERE p.id = ? AND p.stato = 'attivo'`, sessione, pg).Scan(&n)
	if n == 0 {
		return errore(http.StatusBadRequest, "Il personaggio non appartiene a un giocatore del gruppo.")
	}
	s.DB.QueryRow(`SELECT COUNT(*) FROM combattenti WHERE combattimento_id = ? AND personaggio_id = ?`, comb, pg).Scan(&n)
	if n > 0 {
		return errore(http.StatusConflict, "Il personaggio è già nel combattimento.")
	}
	var nome string
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&nome)
	if _, err := s.DB.Exec(`INSERT INTO combattenti(id, combattimento_id, tipo, personaggio_id, nome, creato_il) VALUES(?,?,'pg',?,?,?)`,
		cripto.ID(), comb, pg, nome, db.Ora()); err != nil {
		return err
	}
	s.aggiornaSpareggi(comb)
	return nil
}

// aggiornaSpareggi usa il punteggio di Destrezza della scheda per gli spareggi dei PG.
func (s *Server) aggiornaSpareggi(comb string) {
	s.DB.Exec(`UPDATE combattenti SET spareggio = COALESCE((SELECT json_extract(v.riepilogo, '$.des') FROM personaggi p
		JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = combattenti.personaggio_id), 10)
		WHERE combattimento_id = ? AND tipo = 'pg'`, comb)
}

// modificaPG applica danni, cure, PF temporanei, condizioni, tiri contro la morte e riposo lungo.
// I PF stanno in stato_in_gioco: scheda e tracker vedono lo stesso dato.
func (s *Server) modificaPG(u *Utente, cb *vistaCombattente, tipo string, d datiAzione) error {
	pg := cb.Personaggio
	pf, temp := *cb.PF, int64(cb.PFTemp)
	max := int64(0)
	if cb.PFMax != nil {
		max = int64(*cb.PFMax)
	}
	set := map[string]any{}
	val := func() (int64, error) {
		if d.Valore == nil || *d.Valore < 0 || *d.Valore > 9999 {
			return 0, errore(http.StatusBadRequest, "Valore non valido.")
		}
		return int64(*d.Valore), nil
	}
	switch tipo {
	case "pg.danno":
		v, err := val()
		if err != nil {
			return err
		}
		assorbiti := min(temp, v) // i PF temporanei si consumano per primi
		temp -= assorbiti
		pf = max0(pf - (v - assorbiti))
		set["pf"], set["pf_temp"] = pf, temp
	case "pg.cura":
		v, err := val()
		if err != nil {
			return err
		}
		if pf <= 0 { // chi torna sopra lo 0 azzera i tiri contro la morte
			set["ts_morte"] = `{"s":0,"f":0}`
		}
		pf += v
		if max > 0 && pf > max {
			pf = max
		}
		set["pf"] = pf
	case "pg.temp":
		v, err := val()
		if err != nil {
			return err
		}
		set["pf_temp"] = v // i PF temporanei non si sommano: si sceglie il valore
	case "pg.condizioni":
		c, err := condizioniValide(d.Condizioni)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(c)
		set["condizioni"] = string(b)
	case "pg.ts_morte":
		if d.S == nil || d.F == nil || *d.S < 0 || *d.S > 3 || *d.F < 0 || *d.F > 3 {
			return errDatiNonValidi
		}
		b, _ := json.Marshal(map[string]int{"s": *d.S, "f": *d.F})
		set["ts_morte"] = string(b)
	case "pg.riposo_lungo":
		set["pf"], set["pf_temp"], set["ts_morte"], set["slot_usati"] = max, 0, `{"s":0,"f":0}`, "{}"
	}
	for col, v := range set {
		if _, err := s.DB.Exec(`UPDATE stato_in_gioco SET `+col+` = ?, modificato_il = ?, modificato_da = ? WHERE personaggio_id = ?`, v, db.Ora(), u.ID, pg); err != nil {
			return err
		}
	}
	return nil
}

func max0(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}

// avvisaTurno: "è il tuo turno" al giocatore del PG (con suono nella pagina).
func (s *Server) avvisaTurno(c *vistaCombattente, sessione string) {
	if c.Tipo == "pg" && c.Proprietario != "" {
		s.Hub.AUtente(c.Proprietario, Msg{Tipo: "iniz.turno", Dati: map[string]string{"sessione": sessione, "nome": c.Nome}})
	}
}

// dado tira un dado a n facce con un generatore sicuro (variabile: i test degli attacchi la sostituiscono con tiri decisi).
var dado = func(n int) int {
	x, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 1
	}
	return int(x.Int64()) + 1
}

// ===== Messaggi privati =====
// Giocatore → Master e Master → giocatore. Il server li consegna SOLO ai due interessati (punto S1).

type vistaMessaggio struct {
	ID     string `json:"id"`
	Da     string `json:"da"`
	DaNome string `json:"daNome"`
	A      string `json:"a"`
	ANome  string `json:"aNome"`
	Testo  string `json:"testo"`
	Quando int64  `json:"quando"`
	Letto  bool   `json:"letto"`
	Miei   bool   `json:"mio"`
}

func (s *Server) masterID() string {
	var id string
	s.DB.QueryRow(`SELECT id FROM utenti WHERE ruolo = 'master' LIMIT 1`).Scan(&id)
	return id
}

func (s *Server) messaggiDi(sessione string, u *Utente) []vistaMessaggio {
	q := `SELECT m.id, COALESCE(m.mittente_id,''), COALESCE(a.nome,''), COALESCE(m.destinatario_id,''), COALESCE(b.nome,''), m.testo, m.creato_il, m.letto
		FROM messaggi m LEFT JOIN utenti a ON a.id = m.mittente_id LEFT JOIN utenti b ON b.id = m.destinatario_id
		WHERE m.sessione_id = ? AND (m.mittente_id = ? OR m.destinatario_id = ?) ORDER BY m.creato_il, m.rowid LIMIT 500`
	rows, err := s.DB.Query(q, sessione, u.ID, u.ID)
	out := []vistaMessaggio{}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var m vistaMessaggio
		var l int
		rows.Scan(&m.ID, &m.Da, &m.DaNome, &m.A, &m.ANome, &m.Testo, &m.Quando, &l)
		m.Letto, m.Miei = l == 1, m.Da == u.ID
		out = append(out, m)
	}
	return out
}

func (s *Server) azioneMessaggio(u *Utente, tipo, sessione, gruppo string, master bool, d datiAzione) (any, error) {
	if tipo == "messaggio.letto" {
		s.DB.Exec(`UPDATE messaggi SET letto = 1 WHERE id = ? AND destinatario_id = ? AND sessione_id = ?`, d.Messaggio, u.ID, sessione)
		return nil, nil
	}
	testo, err := testo(d.Testo, "Messaggio", 1, 1000)
	if err != nil {
		return nil, err
	}
	dest := s.masterID()
	if master {
		// Il Master scrive a un giocatore che partecipa alla sessione.
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM partecipanti WHERE sessione_id = ? AND utente_id = ?`, sessione, d.A).Scan(&n)
		if n == 0 {
			return nil, errore(http.StatusBadRequest, "Il destinatario non partecipa a questa sessione.")
		}
		dest = d.A
	}
	if dest == "" || dest == u.ID {
		return nil, errDatiNonValidi
	}
	id := cripto.ID()
	if _, err := s.DB.Exec(`INSERT INTO messaggi(id, sessione_id, mittente_id, destinatario_id, testo, creato_il) VALUES(?,?,?,?,?,?)`,
		id, sessione, u.ID, dest, testo, db.Ora()); err != nil {
		return nil, err
	}
	var nomeDest string
	s.DB.QueryRow(`SELECT nome FROM utenti WHERE id = ?`, dest).Scan(&nomeDest)
	m := vistaMessaggio{ID: id, Da: u.ID, DaNome: u.Nome, A: dest, ANome: nomeDest, Testo: testo, Quando: db.Ora()}
	s.Hub.AUtente(dest, Msg{Tipo: "messaggio.nuovo", Dati: map[string]any{"sessione": sessione, "messaggio": m}})
	mio := m
	mio.Miei = true
	s.Hub.AUtente(u.ID, Msg{Tipo: "messaggio.nuovo", Dati: map[string]any{"sessione": sessione, "messaggio": mio}})
	return mio, nil
}
