package server

// Effetti fuori dallo scontro (decisione dell'utente n. 2 della Parte 2 del piano di combattimento; 2026-11-12;
// migrazione 030).
// Alla fine di un combattimento gli effetti brevi (fino a 1 minuto) e quelli con un tiro salvezza ripetuto finiscono
// (effetti_scontro.go). Gli altri (Armatura magica 8 ore, Aiuto, Legame protettivo, Pelle coriacea…) passano ai personaggi
// dei giocatori che ne sono bersaglio e diventano «effetti che durano»:
//   - valgono fuori dallo scontro (CA cambiata nella scheda, con l'originale) e in ogni scontro successivo (CA, difese,
//     bonus ai TS e agli attacchi: applicaEffettiValori);
//   - se durano fino a 24 ore finiscono al riposo lungo del personaggio o al «Nuovo giorno» del suo gruppo;
//   - quelli a concentrazione finiscono anche quando l'incantatore (un PG) non è più «Concentrato»;
//   - il Master li termina quando vuole (anche il giocatore per gli effetti lanciati dal suo personaggio).
// Finendo, tolgono le condizioni che avevano messo e la scheda torna ai valori di prima. Gli effetti sui mostri finiscono
// con lo scontro.

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

type effettoDurevole struct {
	ID          string            `json:"id"`
	Personaggio string            `json:"personaggio"`
	NomePG      string            `json:"nomePG"`
	Nome        string            `json:"nome"`
	Incantatore string            `json:"incantatore,omitempty"` // nome
	IncPG       string            `json:"-"`                     // personaggio dell'incantatore, se è un PG
	Conc        bool              `json:"conc,omitempty"`
	Giornaliero bool              `json:"giornaliero,omitempty"` // finisce al riposo lungo o al «Nuovo giorno»
	Condizioni  []string          `json:"condizioni,omitempty"`
	Mod         *modificheEffetto `json:"mod,omitempty"`
	Testo       string            `json:"testo,omitempty"` // modifiche in breve
	Mio         bool              `json:"mio,omitempty"`   // l'utente può terminarlo
}

func (s *Server) rotteEffettiPersistenti(mux *http.ServeMux) {
	mux.Handle("DELETE /api/v1/effetti-durevoli/{id}", api(s.apiTerminaDurevole))
}

// rendiDurevoli (da effettiAFineScontro): gli effetti che restano dopo la fine dello scontro passano ai PG bersaglio.
func (s *Server) rendiDurevoli(u *Utente, comb string) {
	effetti := s.caricaEffetti(comb)
	if len(effetti) == 0 {
		return
	}
	tutti, _ := s.caricaCombattenti(comb)
	trova := func(id string) *vistaCombattente {
		for _, c := range tutti {
			if c.ID == id {
				return c
			}
		}
		return nil
	}
	for _, e := range effetti {
		inc := trova(e.Incantatore)
		incPG := ""
		if inc != nil && inc.Tipo == "pg" {
			incPG = inc.Personaggio
		}
		if e.Conc && (inc == nil || inc.Tipo != "pg" || !slices.Contains(inc.Condizioni, "Concentrato")) {
			// la concentrazione di un mostro (o persa) non sopravvive allo scontro
			s.terminaEffetto(u, comb, e, "fine dello scontro")
			continue
		}
		giorn := e.Fine > 0 && e.Fine-e.Inizio <= maxRoundEffetto
		mod := ""
		if e.Mod != nil && !e.Mod.vuota() {
			b, _ := json.Marshal(e.Mod)
			mod = string(b)
		}
		passati := []string{}
		for _, b := range e.Bersagli {
			c := trova(b.ID)
			if c == nil || c.Tipo != "pg" || c.Personaggio == "" {
				continue
			}
			cond, _ := json.Marshal(b.Condizioni)
			s.DB.Exec(`INSERT INTO effetti_persistenti(id, personaggio_id, nome, incantatore, incantatore_pg, conc, giornaliero, condizioni, modifiche, creato_il)
				VALUES(?,?,?,?,?,?,?,?,?,?)`, cripto.ID(), c.Personaggio, e.Nome, e.NomeInc, nullo(incPG), boolInt(e.Conc), boolInt(giorn), string(cond), mod, db.Ora())
			passati = append(passati, c.Nome)
			s.avvisaStatoInGioco(c.Personaggio, c.Proprietario)
		}
		// l'effetto lascia lo scontro senza togliere condizioni: ora le tiene l'effetto che dura
		s.DB.Exec(`DELETE FROM effetti_scontro WHERE id = ?`, e.ID)
		if len(passati) > 0 {
			fine := "finché il Master non lo termina"
			if giorn {
				fine = "fino al riposo lungo o al nuovo giorno"
			}
			s.scriviRegistro(comb, u, e.Nome+" continua dopo lo scontro su "+strings.Join(passati, ", ")+" ("+fine+")")
		}
	}
}

// durevoliDi: gli effetti che durano sui personaggi indicati.
func (s *Server) durevoliDi(pgs []string) []effettoDurevole {
	if len(pgs) == 0 {
		return nil
	}
	q := `SELECT e.id, e.personaggio_id, COALESCE(p.nome,''), e.nome, e.incantatore, COALESCE(e.incantatore_pg,''), e.conc, e.giornaliero,
		e.condizioni, e.modifiche FROM effetti_persistenti e LEFT JOIN personaggi p ON p.id = e.personaggio_id
		WHERE e.personaggio_id IN (?` + strings.Repeat(",?", len(pgs)-1) + `) ORDER BY e.creato_il, e.id`
	args := make([]any, len(pgs))
	for i, x := range pgs {
		args[i] = x
	}
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []effettoDurevole
	for rows.Next() {
		var e effettoDurevole
		var conc, giorn int
		var cond, mod string
		if rows.Scan(&e.ID, &e.Personaggio, &e.NomePG, &e.Nome, &e.Incantatore, &e.IncPG, &conc, &giorn, &cond, &mod) != nil {
			continue
		}
		e.Conc, e.Giornaliero = conc == 1, giorn == 1
		json.Unmarshal([]byte(cond), &e.Condizioni)
		if mod != "" {
			var m modificheEffetto
			if json.Unmarshal([]byte(mod), &m) == nil {
				e.Mod = &m
				e.Testo = testoModifiche(m)
			}
		}
		out = append(out, e)
	}
	return out
}

// terminaDurevole toglie l'effetto e le condizioni che aveva messo (se nessun altro effetto le tiene).
func (s *Server) terminaDurevole(u *Utente, e effettoDurevole, motivo string) {
	s.DB.Exec(`DELETE FROM effetti_persistenti WHERE id = ?`, e.ID)
	if len(e.Condizioni) > 0 {
		altri := s.durevoliDi([]string{e.Personaggio})
		var cs string
		s.DB.QueryRow(`SELECT condizioni FROM stato_in_gioco WHERE personaggio_id = ?`, e.Personaggio).Scan(&cs)
		var att []string
		json.Unmarshal([]byte(cs), &att)
		resta := []string{}
		for _, x := range att {
			tenuta := slices.ContainsFunc(altri, func(o effettoDurevole) bool { return slices.Contains(o.Condizioni, x) })
			if slices.Contains(e.Condizioni, x) && !tenuta {
				continue
			}
			resta = append(resta, x)
		}
		if len(resta) != len(att) {
			b, _ := json.Marshal(resta)
			s.DB.Exec(`UPDATE stato_in_gioco SET condizioni = ?, modificato_il = ?, modificato_da = ? WHERE personaggio_id = ?`, string(b), db.Ora(), u.ID, e.Personaggio)
		}
	}
	var prop string
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, e.Personaggio).Scan(&prop)
	s.avvisaStatoInGioco(e.Personaggio, prop)
	s.ripubblicaPerPersonaggio(e.Personaggio)
	s.evento("effetto_durevole", u.ID, e.Nome+" su "+e.NomePG+": "+motivo)
}

// fineDurevoliRiposo: riposo lungo del personaggio → finiscono i suoi effetti di al massimo 24 ore.
func (s *Server) fineDurevoliRiposo(u *Utente, pg string) {
	for _, e := range s.durevoliDi([]string{pg}) {
		if e.Giornaliero {
			s.terminaDurevole(u, e, "riposo lungo")
		}
	}
}

// fineDurevoliGiorno: nuovo giorno di gioco del gruppo → finiscono gli effetti di al massimo 24 ore dei suoi personaggi.
func (s *Server) fineDurevoliGiorno(u *Utente, gruppo string) {
	rows, err := s.DB.Query(`SELECT id FROM personaggi WHERE gruppo_id = ?`, gruppo)
	if err != nil {
		return
	}
	var pgs []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		pgs = append(pgs, id)
	}
	rows.Close()
	for _, e := range s.durevoliDi(pgs) {
		if e.Giornaliero {
			s.terminaDurevole(u, e, "nuovo giorno")
		}
	}
}

// controllaConcDurevoli: gli effetti a concentrazione finiscono quando l'incantatore non è più «Concentrato».
func (s *Server) controllaConcDurevoli(u *Utente, incPG string, condizioni []string) {
	if incPG == "" || slices.Contains(condizioni, "Concentrato") {
		return
	}
	rows, err := s.DB.Query(`SELECT DISTINCT personaggio_id FROM effetti_persistenti WHERE incantatore_pg = ? AND conc = 1`, incPG)
	if err != nil {
		return
	}
	var pgs []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		pgs = append(pgs, id)
	}
	rows.Close()
	for _, e := range s.durevoliDi(pgs) {
		if e.Conc && e.IncPG == incPG {
			s.terminaDurevole(u, e, "concentrazione persa")
		}
	}
}

// voceDurevoli: gli effetti che durano dei PG combattenti, nello stesso formato di applicaEffettiValori.
func (s *Server) modificheDurevoli(pg string) []struct {
	nome string
	m    modificheEffetto
} {
	var out []struct {
		nome string
		m    modificheEffetto
	}
	for _, e := range s.durevoliDi([]string{pg}) {
		if e.Mod != nil {
			out = append(out, struct {
				nome string
				m    modificheEffetto
			}{e.Nome, *e.Mod})
		}
	}
	return out
}

// apiTerminaDurevole: il Master, o il proprietario del PG che lo ha lanciato (se non mette condizioni).
func (s *Server) apiTerminaDurevole(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	var pg string
	if s.DB.QueryRow(`SELECT personaggio_id FROM effetti_persistenti WHERE id = ?`, r.PathValue("id")).Scan(&pg) != nil {
		return nil, errNonTrovato
	}
	for _, e := range s.durevoliDi([]string{pg}) {
		if e.ID != r.PathValue("id") {
			continue
		}
		if !u.Master() {
			var prop string
			s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, e.IncPG).Scan(&prop)
			if e.IncPG == "" || prop != u.ID || len(e.Condizioni) > 0 {
				return nil, errVietato
			}
		}
		s.terminaDurevole(u, e, "terminato da "+u.Nome)
		return nil, nil
	}
	return nil, errNonTrovato
}

// durevoliVisibili: per la vista del tracker; «mio» se l'utente può terminarlo.
func (s *Server) durevoliVisibili(pgs []string, ruolo, uid string) []effettoDurevole {
	l := s.durevoliDi(pgs)
	for i := range l {
		if ruolo == "master" {
			l[i].Mio = true
			continue
		}
		var prop string
		s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, l[i].IncPG).Scan(&prop)
		l[i].Mio = l[i].IncPG != "" && prop == uid && len(l[i].Condizioni) == 0
	}
	return l
}
