package server

// Lancio di un incantesimo dalla scheda (richiesta dell'utente, 2026-11-06): il giocatore (o il Master) preme
// "Lancia incantesimo" o "Rituale" nel riquadro dell'incantesimo; la scheda sottrae lo slot e qui la
// descrizione viene mandata sugli schermi del Master e di tutti i giocatori del gruppo del personaggio.
// Se il personaggio è in uno scontro in corso, il lancio finisce anche nel registro dello scontro.

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

type incantesimoLanciato struct {
	Nome         string `json:"nome"`
	Inglese      string `json:"en"`
	Livello      int    `json:"livello"`
	Slot         int    `json:"slot"`    // livello dello slot usato (0: trucchetto o rituale)
	Rituale      bool   `json:"rituale"` // lanciato come rituale (nessuno slot, 10 minuti in più)
	Intestazione string `json:"intestazione"`
	Tempo        string `json:"tempo"`
	Gittata      string `json:"gittata"`
	Componenti   string `json:"componenti"`
	Durata       string `json:"durata"`
	Descrizione  string `json:"descrizione"`
	Superiori    string `json:"superiori"`
}

func taglia(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > n {
		s = string([]rune(s)[:n])
	}
	return s
}

func (s *Server) rotteIncantesimi() {
	s.Hub.Registra("incantesimo.lancia", s.wsLanciaIncantesimo)
}

func (s *Server) wsLanciaIncantesimo(c *Cliente, d json.RawMessage) (any, error) {
	var in struct {
		Personaggio string              `json:"personaggio"`
		Inc         incantesimoLanciato `json:"inc"`
	}
	if len(d) > 16<<10 || json.Unmarshal(d, &in) != nil || in.Personaggio == "" {
		return nil, errDatiNonValidi
	}
	if !s.puoModificarePersonaggio(c.U, in.Personaggio) {
		return nil, errNonTrovato
	}
	i := in.Inc
	i.Nome, i.Inglese = taglia(i.Nome, 80), taglia(i.Inglese, 80)
	i.Intestazione, i.Tempo, i.Gittata = taglia(i.Intestazione, 120), taglia(i.Tempo, 80), taglia(i.Gittata, 80)
	i.Componenti, i.Durata = taglia(i.Componenti, 200), taglia(i.Durata, 80)
	i.Descrizione, i.Superiori = taglia(i.Descrizione, 6000), taglia(i.Superiori, 2000)
	if i.Nome == "" || i.Livello < 0 || i.Livello > 9 || i.Slot < 0 || i.Slot > 9 {
		return nil, errDatiNonValidi
	}
	var nomePG, gruppo string
	s.DB.QueryRow(`SELECT nome, COALESCE(gruppo_id,'') FROM personaggi WHERE id = ?`, in.Personaggio).Scan(&nomePG, &gruppo)
	msg := Msg{Tipo: "incantesimo.lanciato", Dati: map[string]any{"personaggio": in.Personaggio, "pg": nomePG, "chi": c.U.Nome, "chiId": c.U.ID, "inc": i}}
	// destinatari: Master e giocatori del gruppo del personaggio (una volta sola ciascuno)
	ids := map[string]bool{}
	rows, err := s.DB.Query(`SELECT utente_id FROM iscrizioni WHERE gruppo_id = ? UNION SELECT id FROM utenti WHERE ruolo = 'master'`, gruppo)
	if err == nil {
		for rows.Next() {
			var id string
			rows.Scan(&id)
			ids[id] = true
		}
		rows.Close()
	}
	ids[c.U.ID] = true
	for id := range ids {
		s.Hub.AUtente(id, msg)
	}
	// registro dello scontro, se il personaggio è in uno scontro in corso
	// Con un effetto automatico (incantesimi_scontro.go) la pagina della scheda apre la scelta dei bersagli: le servono
	// sessione e combattente dello scontro in corso.
	var comb, ses, chi string
	out := map[string]any{"inviato": len(ids)}
	if s.DB.QueryRow(`SELECT k.id, k.sessione_id, x.id FROM combattenti x JOIN combattimenti k ON k.id = x.combattimento_id
		WHERE x.personaggio_id = ? AND k.attivo = 1 ORDER BY k.creato_il DESC LIMIT 1`, in.Personaggio).Scan(&comb, &ses, &chi) == nil {
		t := nomePG + " lancia " + i.Nome
		if i.Rituale {
			t += " come rituale"
		} else if i.Slot > 0 {
			t += " (slot di " + strconv.Itoa(i.Slot) + "° livello)"
		}
		s.registro(comb, c.U, t)
		if _, ok := s.partecipa(c.U, ses); ok {
			out["scontro"] = map[string]string{"sessione": ses, "combattente": chi}
		}
	}
	return out, nil
}
