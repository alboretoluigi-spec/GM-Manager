package server

// Effetti in corso nello scontro (Parte 2 del piano di combattimento: effetti persistenti; migrazione 028).
// Un incantesimo che lascia condizioni diventa un effetto con incantatore, bersagli e condizioni messe davvero:
//   - concentrazione: finisce quando l'incantatore non è più «Concentrato» (tolto a mano, nuovo incantesimo a concentrazione,
//     0 PF) o esce dallo scontro;
//   - durata in round: finisce all'inizio del turno dell'incantatore nel round di fine (o dopo, se non c'è più);
//   - tiro salvezza ripetuto: alla fine di ogni turno di un bersaglio il server lo ritira; se riesce, il bersaglio si libera;
//   - il Master può terminarlo («effetto.termina»).
// Finendo, l'effetto toglie solo le condizioni che aveva messo (non quelle messe a mano o da altri effetti ancora in corso).
// Fuori dallo scontro (decisione dell'utente del 2026-11-11): alla fine del combattimento terminano gli effetti brevi (fino a
// 1 minuto); gli altri passano ai personaggi come effetti che durano (effetti_persistenti.go, dal 2026-11-12).

import (
	"encoding/json"
	"slices"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

type bersaglioEffetto struct {
	ID         string   `json:"id"`
	Nome       string   `json:"nome"`
	Condizioni []string `json:"condizioni"`
}

type tsEffetto struct {
	Car string `json:"car"`
	CD  int    `json:"cd"`
}

type vistaEffetto struct {
	ID          string             `json:"id"`
	Nome        string             `json:"nome"`
	Incantatore string             `json:"incantatore"`
	NomeInc     string             `json:"nomeIncantatore"`
	Bersagli    []bersaglioEffetto `json:"bersagli"`
	Conc        bool               `json:"conc,omitempty"`
	Inizio      int                `json:"inizioRound"`
	Fine        int                `json:"fineRound,omitempty"` // 0: finché non si termina
	TS          *tsEffetto         `json:"ts,omitempty"`
	Mod         *modificheEffetto  `json:"mod,omitempty"` // modifiche ai valori dei bersagli (effetti_valori.go)
}

// scriviRegistro: una riga nel registro dello scontro (senza ripubblicare: lo fa chi ha chiamato).
func (s *Server) scriviRegistro(comb string, u *Utente, t string) {
	s.DB.Exec(`INSERT INTO registro_combattimento(combattimento_id, autore_id, testo, creato_il) VALUES(?,?,?,?)`, comb, u.ID, u.Nome+": "+t, db.Ora())
}

func (s *Server) caricaEffetti(comb string) []vistaEffetto {
	rows, err := s.DB.Query(`SELECT e.id, e.nome, e.incantatore_id, COALESCE(c.nome, ''), e.bersagli, e.conc, e.inizio_round, e.fine_round, e.ts, e.modifiche
		FROM effetti_scontro e LEFT JOIN combattenti c ON c.id = e.incantatore_id WHERE e.combattimento_id = ? ORDER BY e.creato_il, e.id`, comb)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []vistaEffetto
	for rows.Next() {
		var e vistaEffetto
		var b, ts, mod string
		var conc int
		if rows.Scan(&e.ID, &e.Nome, &e.Incantatore, &e.NomeInc, &b, &conc, &e.Inizio, &e.Fine, &ts, &mod) != nil {
			continue
		}
		if mod != "" {
			var m modificheEffetto
			if json.Unmarshal([]byte(mod), &m) == nil {
				e.Mod = &m
			}
		}
		json.Unmarshal([]byte(b), &e.Bersagli)
		e.Conc = conc == 1
		if ts != "" {
			var t tsEffetto
			if json.Unmarshal([]byte(ts), &t) == nil && t.Car != "" {
				e.TS = &t
			}
		}
		out = append(out, e)
	}
	return out
}

// nuovoEffetto registra un effetto (incantesimo.applica). round: round in corso; inc.Round: durata in round (0 = istantaneo o
// senza limite in round).
func (s *Server) nuovoEffetto(comb string, a *vistaCombattente, inc incantesimoScontro, bersagli []bersaglioEffetto, round int) {
	if len(bersagli) == 0 || !(inc.Round > 0 || inc.Conc || inc.Ripeti) {
		return
	}
	fine := 0
	if inc.Round > 0 {
		fine = round + inc.Round
	}
	ts := ""
	if inc.Ripeti && inc.Tipo == "ts" {
		b, _ := json.Marshal(tsEffetto{inc.Car, inc.CD})
		ts = string(b)
	}
	b, _ := json.Marshal(bersagli)
	conc := 0
	if inc.Conc {
		conc = 1
	}
	s.DB.Exec(`INSERT INTO effetti_scontro(id, combattimento_id, incantatore_id, nome, bersagli, conc, inizio_round, fine_round, ts, creato_il)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, cripto.ID(), comb, a.ID, inc.Nome, string(b), conc, round, fine, ts, db.Ora())
}

// rimuoviCondizioni toglie condizioni a un combattente (PG: stato in gioco). Restituisce quelle tolte.
func (s *Server) rimuoviCondizioni(u *Utente, c *vistaCombattente, via []string) []string {
	l, tolte := []string{}, []string{}
	for _, x := range c.Condizioni {
		if slices.Contains(via, x) {
			tolte = append(tolte, x)
		} else {
			l = append(l, x)
		}
	}
	if len(tolte) == 0 {
		return nil
	}
	if c.Tipo == "mostro" {
		b, _ := json.Marshal(l)
		s.DB.Exec(`UPDATE combattenti SET condizioni = ? WHERE id = ?`, string(b), c.ID)
	} else if s.modificaPG(u, c, "pg.condizioni", datiAzione{Condizioni: l}) != nil {
		return nil
	} else {
		s.avvisaStatoInGioco(c.Personaggio, c.Proprietario)
	}
	c.Condizioni = l
	return tolte
}

// terminaEffetto toglie ai bersagli le condizioni dell'effetto (se nessun altro effetto in corso le tiene) e lo cancella.
func (s *Server) terminaEffetto(u *Utente, comb string, e vistaEffetto, motivo string) {
	s.DB.Exec(`DELETE FROM effetti_scontro WHERE id = ?`, e.ID)
	tutti, _ := s.caricaCombattenti(comb)
	altri := s.caricaEffetti(comb) // ancora in corso (questo è già cancellato)
	parti := []string{}
	for _, b := range e.Bersagli {
		var c *vistaCombattente
		for _, x := range tutti {
			if x.ID == b.ID {
				c = x
			}
		}
		if c == nil {
			continue
		}
		via := []string{}
		for _, n := range b.Condizioni {
			tenuta := slices.ContainsFunc(altri, func(o vistaEffetto) bool {
				return slices.ContainsFunc(o.Bersagli, func(x bersaglioEffetto) bool { return x.ID == b.ID && slices.Contains(x.Condizioni, n) })
			})
			if !tenuta {
				via = append(via, n)
			}
		}
		if tolte := s.rimuoviCondizioni(u, c, via); len(tolte) > 0 {
			parti = append(parti, c.Nome+" non è più "+strings.ToLower(strings.Join(tolte, ", ")))
		} else if e.Mod != nil && c.Tipo == "pg" && c.Personaggio != "" { // modifiche finite: la scheda aperta torna ai valori di prima
			s.avvisaStatoInGioco(c.Personaggio, c.Proprietario)
		}
	}
	t := "termina " + e.Nome
	if e.NomeInc != "" {
		t += " di " + e.NomeInc
	}
	t += " (" + motivo + ")"
	if len(parti) > 0 {
		t += ": " + strings.Join(parti, "; ")
	}
	s.scriviRegistro(comb, u, t)
}

// finisceConcentrazione termina gli effetti a concentrazione di un incantatore.
func (s *Server) finisceConcentrazione(u *Utente, comb, incantatore, motivo string) {
	for _, e := range s.caricaEffetti(comb) {
		if e.Conc && e.Incantatore == incantatore {
			s.terminaEffetto(u, comb, e, motivo)
		}
	}
}

// controllaConcentrazioni: dopo un cambio di condizioni o PF, termina gli effetti a concentrazione di chi non è più
// concentrato (o è uscito dallo scontro).
func (s *Server) controllaConcentrazioni(u *Utente, comb string) {
	if tutti, err := s.caricaCombattenti(comb); err == nil { // effetti che durano da scontri precedenti
		for _, c := range tutti {
			if c.Tipo == "pg" {
				s.controllaConcDurevoli(u, c.Personaggio, c.Condizioni)
			}
		}
	}
	effetti := s.caricaEffetti(comb)
	if len(effetti) == 0 {
		return
	}
	tutti, _ := s.caricaCombattenti(comb)
	for _, e := range effetti {
		if !e.Conc {
			continue
		}
		i := slices.IndexFunc(tutti, func(c *vistaCombattente) bool { return c.ID == e.Incantatore })
		switch {
		case i < 0:
			s.terminaEffetto(u, comb, e, "l'incantatore non è più nello scontro")
		case !slices.Contains(tutti[i].Condizioni, "Concentrato"):
			s.terminaEffetto(u, comb, e, "concentrazione persa")
		}
	}
}

// effettiDopoTurno (iniz.prossimo_turno): il bersaglio il cui turno è finito ripete i TS; scadono gli effetti a tempo; chi
// comincia il turno con Eroismo riceve i PF temporanei (se ne ha meno).
func (s *Server) effettiDopoTurno(u *Utente, comb string, finito, nuovo *vistaCombattente, round int) {
	if nuovo != nil && nuovo.Tipo == "pg" && nuovo.mod != nil && nuovo.mod.pfTurno > 0 && nuovo.PFTemp < nuovo.mod.pfTurno {
		v := nuovo.mod.pfTurno
		if s.modificaPG(u, nuovo, "pg.temp", datiAzione{Valore: &v}) == nil {
			s.avvisaStatoInGioco(nuovo.Personaggio, nuovo.Proprietario)
			s.scriviRegistro(comb, u, nuovo.Nome+" riceve "+itoa(v)+" PF temporanei ("+nuovo.mod.nomePFTurno+")")
		}
	}
	for _, e := range s.caricaEffetti(comb) {
		if e.TS != nil && finito != nil {
			k := slices.IndexFunc(e.Bersagli, func(b bersaglioEffetto) bool { return b.ID == finito.ID })
			if k >= 0 && len(e.Bersagli[k].Condizioni) > 0 {
				r, _ := s.tiraTS(finito, e.TS.Car, e.TS.CD, e.Bersagli[k].Condizioni)
				t := finito.Nome + " ripete il TS su " + caratteristicheTS[e.TS.Car] + " contro " + e.Nome + ": "
				if r.Nota != "" && r.D20 == nil {
					t += r.Nota
				} else {
					t += itoa(r.Totale) + " contro CD " + itoa(e.TS.CD)
				}
				if r.Riuscito {
					solo := vistaEffetto{ID: "", Nome: e.Nome, NomeInc: e.NomeInc, Bersagli: []bersaglioEffetto{e.Bersagli[k]}}
					e.Bersagli = slices.Delete(e.Bersagli, k, k+1)
					b, _ := json.Marshal(e.Bersagli)
					s.DB.Exec(`UPDATE effetti_scontro SET bersagli = ? WHERE id = ?`, string(b), e.ID)
					s.scriviRegistro(comb, u, t+", riesce")
					s.terminaEffetto(u, comb, solo, finito.Nome+" si libera")
					if len(e.Bersagli) == 0 {
						s.DB.Exec(`DELETE FROM effetti_scontro WHERE id = ?`, e.ID)
						continue
					}
				} else {
					s.scriviRegistro(comb, u, t+", fallisce: resta "+strings.ToLower(strings.Join(e.Bersagli[k].Condizioni, ", ")))
				}
			}
		}
		if e.Fine > 0 && (nuovo != nil && nuovo.ID == e.Incantatore && round >= e.Fine || round > e.Fine) {
			s.terminaEffetto(u, comb, e, "durata finita")
		}
	}
}

// effettiAFineScontro: alla fine del combattimento terminano gli effetti brevi (fino a 1 minuto) e quelli con un TS
// ripetuto; gli altri passano ai personaggi bersaglio come effetti che durano (effetti_persistenti.go).
func (s *Server) effettiAFineScontro(u *Utente, comb string) {
	for _, e := range s.caricaEffetti(comb) {
		if e.Fine > 0 && e.Fine-e.Inizio <= 10 || e.TS != nil {
			s.terminaEffetto(u, comb, e, "fine dello scontro")
		}
	}
	s.rendiDurevoli(u, comb)
}

// effettiVisibili: ai giocatori niente effetti di mostri nascosti né bersagli nascosti.
func effettiVisibili(l []vistaEffetto, tutti []*vistaCombattente, ruolo string) []vistaEffetto {
	if ruolo == "master" {
		return l
	}
	nascosto := map[string]bool{}
	for _, c := range tutti {
		nascosto[c.ID] = c.Nascosto
	}
	var out []vistaEffetto
	for _, e := range l {
		if nascosto[e.Incantatore] {
			continue
		}
		x := e
		x.Bersagli = nil
		for _, b := range e.Bersagli {
			if !nascosto[b.ID] {
				x.Bersagli = append(x.Bersagli, b)
			}
		}
		if len(x.Bersagli) > 0 {
			out = append(out, x)
		}
	}
	return out
}
