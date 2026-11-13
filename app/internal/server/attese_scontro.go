package server

// Scontri lasciati a metà (decisione dell'utente del 2026-11-12; migrazione 032): il colpo critico che aspetta la scelta del
// modo («Tira due volte i dadi» / «Raddoppia») e i tiri salvezza che aspettano la conferma del Master vivono in memoria
// (attacchiInCorso) e, insieme, nella tabella attese_scontro. Se Gaster Bless si riavvia, la memoria è vuota: al primo uso
// (attacco.critico, incantesimo.applica, vista del Master) le attese dello scontro tornano dalla tabella. La riga si
// cancella quando l'attesa si usa; quelle scadute si cancellano da sole.
// Non si salvano (restano solo in memoria): i raggi rimasti di un incantesimo con più raggi e le risposte già date alla
// stessa azione (doppio clic); dopo un riavvio il Master può forzare il raggio successivo.

import (
	"encoding/json"
	"time"
)

// criticoSalvato: il critico con i campi non esportati dell'esito e del profilo.
type criticoSalvato struct {
	Comb, Attaccante, Bersaglio, Autore string
	Profilo                             profiloAttacco
	ProfIncantesimo                     bool
	Esito                               esitoAttacco
	Nascosto, NascostoA, MostroB        bool
	PropB, AutoreEsito                  string
}

type rigaSalvata struct {
	Nascosto bool
	Prop     string
}

type tsSalvato struct {
	Comb, Sessione, Autore string
	Inc                    incantesimoScontro
	Esito                  esitoIncantesimo
	IncNascosto            bool
	Righe                  []rigaSalvata
}

func (s *Server) salvaAttesa(gettone, tipo, comb string, dati any, scade time.Time) {
	b, err := json.Marshal(dati)
	if err != nil {
		return
	}
	s.DB.Exec(`INSERT OR REPLACE INTO attese_scontro(gettone, tipo, combattimento_id, dati, scade) VALUES(?,?,?,?,?)`,
		gettone, tipo, comb, string(b), scade.Unix())
}

// salvaCritico: chi chiama tiene attacchiInCorso.serie.
func (s *Server) salvaCritico(gettone string, c *critico) {
	s.salvaAttesa(gettone, "critico", c.comb, criticoSalvato{Comb: c.comb, Attaccante: c.attaccante, Bersaglio: c.bersaglio, Autore: c.autore,
		Profilo: c.profilo, ProfIncantesimo: c.profilo.incantesimo, Esito: c.esito, Nascosto: c.esito.nascosto, NascostoA: c.esito.nascostoA,
		MostroB: c.esito.mostroB, PropB: c.esito.propB, AutoreEsito: c.esito.autore}, c.scade)
}

func (s *Server) salvaTS(gettone string, p *tsInAttesa) {
	x := tsSalvato{Comb: p.comb, Sessione: p.sessione, Autore: p.autore, Inc: p.inc, Esito: p.esito, IncNascosto: p.esito.incNascosto}
	for _, r := range p.esito.Righe {
		x.Righe = append(x.Righe, rigaSalvata{Nascosto: r.nascosto, Prop: r.prop})
	}
	s.salvaAttesa(gettone, "ts", p.comb, x, p.scade)
}

// togliAttesa: l'attesa è stata usata (monouso) o non vale più.
func (s *Server) togliAttesa(gettone string) {
	s.DB.Exec(`DELETE FROM attese_scontro WHERE gettone = ?`, gettone)
}

// ripristinaAttese rimette in memoria le attese dello scontro salvate e non scadute che la memoria non ha (dopo un
// riavvio). conCritici: solo chi tiene attacchiInCorso.serie può toccare i critici; i TS sono protetti da tsMu.
func (s *Server) ripristinaAttese(comb string, conCritici bool) {
	ora := time.Now()
	s.DB.Exec(`DELETE FROM attese_scontro WHERE scade < ?`, ora.Unix())
	rows, err := s.DB.Query(`SELECT gettone, tipo, dati, scade FROM attese_scontro WHERE combattimento_id = ?`, comb)
	if err != nil {
		return
	}
	type riga struct {
		g, tipo, dati string
		scade         int64
	}
	var l []riga
	for rows.Next() {
		var r riga
		if rows.Scan(&r.g, &r.tipo, &r.dati, &r.scade) == nil {
			l = append(l, r)
		}
	}
	rows.Close()
	m := attacchiInCorso
	for _, r := range l {
		scade := time.Unix(r.scade, 0)
		switch r.tipo {
		case "critico":
			if !conCritici || m.critici[r.g] != nil {
				continue
			}
			var x criticoSalvato
			if json.Unmarshal([]byte(r.dati), &x) != nil {
				continue
			}
			x.Profilo.incantesimo = x.ProfIncantesimo
			x.Esito.nascosto, x.Esito.nascostoA, x.Esito.mostroB, x.Esito.propB, x.Esito.autore = x.Nascosto, x.NascostoA, x.MostroB, x.PropB, x.AutoreEsito
			m.critici[r.g] = &critico{comb: x.Comb, attaccante: x.Attaccante, bersaglio: x.Bersaglio, autore: x.Autore, profilo: x.Profilo, esito: x.Esito, scade: scade}
		case "ts":
			var x tsSalvato
			if json.Unmarshal([]byte(r.dati), &x) != nil {
				continue
			}
			x.Esito.incNascosto = x.IncNascosto
			for i := range x.Esito.Righe {
				if i < len(x.Righe) {
					x.Esito.Righe[i].nascosto, x.Esito.Righe[i].prop = x.Righe[i].Nascosto, x.Righe[i].Prop
				}
			}
			m.tsMu.Lock()
			if m.ts[r.g] == nil {
				m.ts[r.g] = &tsInAttesa{comb: x.Comb, sessione: x.Sessione, autore: x.Autore, inc: x.Inc, esito: x.Esito, scade: scade}
			}
			m.tsMu.Unlock()
		}
	}
}
