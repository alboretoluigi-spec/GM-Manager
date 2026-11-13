package server

// Simulatore di dadi. I tiri li fa il server, così il risultato è lo stesso per tutti e non si
// può "aggiustare" dal telefono. Durante uno scontro il tiro arriva a tutti i partecipanti della
// sessione (in qualunque sezione dell'app si trovino): con un 20 naturale sul d20 compare
// "CRITICO", con un 1 lo schermo trema e compare "OOOPS". Il Master può tirare in segreto.

import (
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"gasterbless/internal/cripto"
)

// tiraDado restituisce un numero da 1 a facce (variabile per poterla sostituire nei test).
var tiraDado = func(facce int) int { return rand.IntN(facce) + 1 }

var facceAmmesse = map[int]bool{4: true, 6: true, 8: true, 10: true, 12: true, 20: true, 100: true}

type gruppoDadi struct {
	N      int   `json:"n"`
	Facce  int   `json:"facce"`
	Valori []int `json:"valori,omitempty"`
	Scarto []int `json:"scarto,omitempty"` // d20 scartati con vantaggio o svantaggio
}

type tiroDadi struct {
	Sessione   string       `json:"sessione,omitempty"`
	Chi        string       `json:"chi"`
	ChiID      string       `json:"chiId"`
	Etichetta  string       `json:"etichetta,omitempty"`
	Dadi       []gruppoDadi `json:"dadi"`
	Mod        int          `json:"mod"`
	Modo       string       `json:"modo"` // normale, vantaggio, svantaggio
	Totale     int          `json:"totale"`
	Formula    string       `json:"formula"`
	Critico    bool         `json:"critico,omitempty"`
	Fallimento bool         `json:"fallimento,omitempty"`
	Scontro    bool         `json:"scontro"` // tirato durante uno scontro: effetti per tutti
	Segreto    bool         `json:"segreto,omitempty"`
	ID         string       `json:"id,omitempty"`
	Sfortuna   bool         `json:"sfortuna,omitempty"` // 1 naturale in scontro: pulsante "Che sfortuna"
	Reale      bool         `json:"reale,omitempty"`    // segnato dal Master per un dado reale
	Scheda     bool         `json:"scheda,omitempty"`   // tiro fatto cliccando una voce della scheda
	Colore     string       `json:"colore,omitempty"`   // set di dadi di chi tira, «#corpo/#numeri» (dadi_aspetto.go)
	Classe     string       `json:"classe,omitempty"`   // classe del personaggio di chi tira: effetto del d20
}

func (s *Server) rotteDadi() {
	s.Hub.Registra("dadi.tira", s.wsTiraDadi)
	s.Hub.Registra("dadi.reale", s.wsDadoReale)
	s.Hub.Registra("dadi.manuale", s.wsTiroManuale)
}

// wsDadoReale: un giocatore ha tirato un dado vero al tavolo e ha fatto 20 o 1. Il Master lo segna e
// l'app reagisce come per un tiro fatto nell'app: CRITICO o OOOPS!!! per tutti e, con l'1, il tasto
// "Che sfortuna" sullo schermo di quel giocatore.
func (s *Server) wsDadoReale(c *Cliente, d json.RawMessage) (any, error) {
	r, err := leggiIniz(d)
	if err != nil {
		return nil, err
	}
	if err := s.soloMaster(c.U, r.Sessione); err != nil {
		return nil, err
	}
	if r.Nome != "critico" && r.Nome != "fallimento" {
		return nil, errDatiNonValidi
	}
	comb, err := s.richiediScontro(r.Sessione)
	if err != nil {
		return nil, err
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM combattenti WHERE combattimento_id = ? AND personaggio_id = ?`, comb, r.Personaggio).Scan(&n)
	if n == 0 {
		return nil, errore(http.StatusBadRequest, "Il personaggio non è nello scontro.")
	}
	_, prop := s.datiPG(r.Personaggio)
	var nome string
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, r.Personaggio).Scan(&nome)
	v := 20
	if r.Nome == "fallimento" {
		v = 1
	}
	t := tiroDadi{Sessione: r.Sessione, Chi: nome, ChiID: prop, Dadi: []gruppoDadi{{N: 1, Facce: 20, Valori: []int{v}}}, Modo: "normale",
		Totale: v, Formula: "1d20 (dado reale)", Critico: v == 20, Fallimento: v == 1, Scontro: true, Reale: true}
	if prop != "" {
		s.decoraTiro(&t, prop, r.Personaggio, r.Sessione)
	}
	if t.Fallimento && prop != "" {
		t.ID, t.Sfortuna = cripto.ID(), true
		registraSfortuna(t.ID, prop, r.Sessione, nome)
	}
	for _, id := range s.destinatariSessione(r.Sessione) {
		s.Hub.AUtente(id, Msg{Tipo: "dadi.risultato", Dati: t})
	}
	testo := nome + ": 20 naturale con un dado reale · CRITICO"
	if t.Fallimento {
		testo = nome + ": 1 naturale con un dado reale · OOOPS!!!"
	}
	s.registro(comb, c.U, testo)
	return t, nil
}

// sessioneConScontro: la sessione indicata se l'utente vi partecipa, altrimenti quella con uno
// scontro attivo a cui partecipa (la più recente).
func (s *Server) sessioneConScontro(u *Utente, sessione string) (string, bool) {
	if sessione != "" {
		if _, ok := s.partecipa(u, sessione); !ok {
			return "", false
		}
		_, _, _, _, attivo := s.scontroAttivo(sessione)
		return sessione, attivo
	}
	rows, err := s.DB.Query(`SELECT sessione_id FROM combattimenti WHERE attivo = 1 ORDER BY creato_il DESC`)
	if err != nil {
		return "", false
	}
	var cand []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		cand = append(cand, id)
	}
	rows.Close()
	for _, id := range cand {
		if _, ok := s.partecipa(u, id); ok {
			return id, true
		}
	}
	return "", false
}

// destinatariSessione: Master e partecipanti della sessione.
func (s *Server) destinatariSessione(sessione string) []string {
	ids := map[string]bool{}
	rows, err := s.DB.Query(`SELECT utente_id FROM partecipanti WHERE sessione_id = ? UNION SELECT id FROM utenti WHERE ruolo = 'master'`, sessione)
	if err == nil {
		for rows.Next() {
			var id string
			rows.Scan(&id)
			ids[id] = true
		}
		rows.Close()
	}
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	return out
}

func (s *Server) wsTiraDadi(c *Cliente, d json.RawMessage) (any, error) {
	var in struct {
		Sessione  string       `json:"sessione"`
		Dadi      []gruppoDadi `json:"dadi"`
		Mod       int          `json:"mod"`
		Modo      string       `json:"modo"`
		Etichetta string       `json:"etichetta"`
		Segreto   bool         `json:"segreto"`
		// Tiro dalla scheda (tiro salvezza o abilità): lo vedono sempre chi tira e il Master; se il Master tira
		// dalla scheda di un giocatore, lo vede anche quel giocatore (richiesta dell'utente, 2026-11-06).
		Scheda      bool   `json:"scheda"`
		Personaggio string `json:"personaggio"`
	}
	if len(d) > 4<<10 || json.Unmarshal(d, &in) != nil {
		return nil, errDatiNonValidi
	}
	tot := 0
	if len(in.Dadi) == 0 || len(in.Dadi) > 7 {
		return nil, errore(http.StatusBadRequest, "Scegli almeno un dado.")
	}
	for _, g := range in.Dadi {
		if !facceAmmesse[g.Facce] || g.N < 1 || g.N > 20 {
			return nil, errore(http.StatusBadRequest, "Dadi non validi (da 1 a 20 per tipo: d4, d6, d8, d10, d12, d20, d100).")
		}
		tot += g.N
	}
	if tot > 50 || in.Mod < -50 || in.Mod > 50 {
		return nil, errore(http.StatusBadRequest, "Al massimo 50 dadi e modificatore tra −50 e +50.")
	}
	if in.Modo != "vantaggio" && in.Modo != "svantaggio" {
		in.Modo = "normale"
	}
	etich, err := rigaSingola(in.Etichetta, "Descrizione", 0, 40)
	if err != nil {
		return nil, err
	}
	t := tiroDadi{Chi: c.U.Nome, ChiID: c.U.ID, Etichetta: etich, Mod: in.Mod, Modo: in.Modo, Segreto: in.Segreto && c.U.Master(), Scheda: in.Scheda}
	var parti []string
	somma := in.Mod
	for _, g := range in.Dadi {
		g.Valori, g.Scarto = nil, nil
		for i := 0; i < g.N; i++ {
			v := tiraDado(g.Facce)
			// Vantaggio e svantaggio valgono per i d20: se ne tirano due e si tiene il migliore o il peggiore.
			if g.Facce == 20 && in.Modo != "normale" {
				w := tiraDado(20)
				if (in.Modo == "vantaggio") == (w > v) {
					v, w = w, v
				}
				g.Scarto = append(g.Scarto, w)
			}
			g.Valori = append(g.Valori, v)
			somma += v
			if g.Facce == 20 {
				if v == 20 {
					t.Critico = true
				} else if v == 1 {
					t.Fallimento = true
				}
			}
		}
		parti = append(parti, strconv.Itoa(g.N)+"d"+strconv.Itoa(g.Facce))
		t.Dadi = append(t.Dadi, g)
	}
	if t.Critico { // con più d20, un 20 vince su un 1
		t.Fallimento = false
	}
	t.Formula = strings.Join(parti, " + ")
	if in.Mod != 0 {
		t.Formula += " " + conSegno(in.Mod)
	}
	if in.Modo != "normale" {
		t.Formula += " (" + in.Modo + ")"
	}
	t.Totale = somma

	return s.inviaTiro(c.U, t, in.Sessione, in.Personaggio), nil
}

// inviaTiro manda il risultato a chi deve vederlo: in scontro a tutta la sessione (con il registro e, con un 1
// naturale, la tabella della sfortuna); fuori dallo scontro a chi tira e, per i tiri dalla scheda, anche al Master
// e al giocatore del personaggio. Usata dal tiro automatico (wsTiraDadi) e da quello manuale (wsTiroManuale).
func (s *Server) inviaTiro(u *Utente, t tiroDadi, sessione, personaggio string) tiroDadi {
	sess, scontro := s.sessioneConScontro(u, sessione)
	t.Sessione, t.Scontro = sess, scontro
	if u.Master() && personaggio == "" {
		t.Colore = s.coloreDadi(u.ID) // il Master: niente classe (tira per i mostri)
	} else {
		s.decoraTiro(&t, u.ID, personaggio, sess)
	}
	// Chi fa 1 naturale in scontro (giocatore o Master) può tirare sulla tabella della sfortuna (una volta).
	if scontro && t.Fallimento && !t.Segreto {
		t.ID, t.Sfortuna = cripto.ID(), true
		registraSfortuna(t.ID, u.ID, sess, u.Nome)
	}
	m := Msg{Tipo: "dadi.risultato", Dati: t}
	if scontro && !t.Segreto {
		for _, id := range s.destinatariSessione(sess) {
			s.Hub.AUtente(id, m)
		}
		if comb, _, _, _, ok := s.scontroAttivo(sess); ok {
			testo := "tira " + t.Formula + " = " + strconv.Itoa(t.Totale)
			if t.Critico {
				testo += " · CRITICO"
			} else if t.Fallimento {
				testo += " · 1 naturale"
			}
			s.registro(comb, u, testo)
		}
	} else {
		// Fuori dallo scontro (o tiro segreto del Master) il risultato lo vedono chi tira e, per i tiri dei giocatori, il
		// Master: anche quelli del pannello «Dadi» (segnalazione dell'utente, 2026-11-12: «il Master non li vede»).
		// Gli altri giocatori no.
		s.Hub.AUtente(u.ID, m)
		if !t.Segreto {
			if !u.Master() {
				s.Hub.AlMaster(m)
			} else if t.Scheda && personaggio != "" { // il Master tira dalla scheda di un giocatore: lo vede anche lui
				if _, prop := s.datiPG(personaggio); prop != "" && prop != u.ID {
					s.Hub.AUtente(prop, m)
				}
			}
		}
	}
	return t
}

// wsTiroManuale: tiro dalla scheda fatto con un dado vero (richiesta dell'utente, 2026-11-06). Il giocatore scrive il
// risultato del d20 (con vantaggio o svantaggio, quello tenuto); l'app aggiunge il modificatore della scheda e lo
// mostra come gli altri tiri dalla scheda, con CRITICO e OOOPS!!! e la dicitura «dado reale».
func (s *Server) wsTiroManuale(c *Cliente, d json.RawMessage) (any, error) {
	var in struct {
		Sessione    string `json:"sessione"`
		D20         int    `json:"d20"`
		Mod         int    `json:"mod"`
		Modo        string `json:"modo"`
		Etichetta   string `json:"etichetta"`
		Personaggio string `json:"personaggio"`
	}
	if len(d) > 2<<10 || json.Unmarshal(d, &in) != nil {
		return nil, errDatiNonValidi
	}
	if in.D20 < 1 || in.D20 > 20 {
		return nil, errore(http.StatusBadRequest, "Scrivi il numero uscito sul d20 (da 1 a 20).")
	}
	if in.Mod < -50 || in.Mod > 50 {
		return nil, errDatiNonValidi
	}
	if in.Modo != "vantaggio" && in.Modo != "svantaggio" {
		in.Modo = "normale"
	}
	etich, err := rigaSingola(in.Etichetta, "Descrizione", 0, 40)
	if err != nil {
		return nil, err
	}
	t := tiroDadi{Chi: c.U.Nome, ChiID: c.U.ID, Etichetta: etich, Mod: in.Mod, Modo: in.Modo, Scheda: true, Reale: true,
		Dadi: []gruppoDadi{{N: 1, Facce: 20, Valori: []int{in.D20}}}, Totale: in.D20 + in.Mod,
		Critico: in.D20 == 20, Fallimento: in.D20 == 1}
	t.Formula = "1d20"
	if in.Mod != 0 {
		t.Formula += " " + conSegno(in.Mod)
	}
	if in.Modo != "normale" {
		t.Formula += " (" + in.Modo + ")"
	}
	t.Formula += " · dado reale"
	return s.inviaTiro(c.U, t, in.Sessione, in.Personaggio), nil
}
