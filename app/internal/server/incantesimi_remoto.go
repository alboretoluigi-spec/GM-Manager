package server

// Incantesimi automatici anche senza la scheda aperta (fase 9b, gioco da remoto, decisione dell'utente del 2026-11-12).
// La scheda manda nel riepilogo («spellsAuto», «slots») gli incantesimi del PG che si risolvono da soli, già calcolati a ogni
// livello di slot (bonus, CD, danni, cure), e gli slot totali per livello. Il tracker li mostra al proprietario e al Master
// (campi «incantesimiAuto» e «slot» della vista) e la pagina del gioco da remoto li lancia con i comandi di sempre
// (attacco.tira con «incantesimo», incantesimo.ts, incantesimo.cura) e «usaSlot»: lo slot lo toglie il server (nello
// stato in gioco condiviso con la scheda), solo se è libero e solo se il lancio va a buon fine.

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gasterbless/internal/db"
)

// profiloIncAuto: l'incantesimo pronto per un livello di slot (Slot impostato), con i bersagli massimi e il riassunto.
type profiloIncAuto struct {
	incantesimoScontro
	Bersagli  int    `json:"bersagli,omitempty"`
	Riassunto string `json:"riassunto,omitempty"`
}

type incAuto struct {
	Nome    string           `json:"nome"`
	Livello int              `json:"livello"`
	Livelli []profiloIncAuto `json:"livelli"`
}

// slotPG: slot di un livello, totali e usati (stato in gioco).
type slotPG struct {
	Livello int `json:"livello"`
	Tot     int `json:"tot"`
	Usati   int `json:"usati"`
}

// validaIncAuto: al massimo 40 incantesimi, ognuno con al massimo 10 livelli validi; solo i tipi che si risolvono da soli.
func validaIncAuto(in []incAuto) []incAuto {
	var out []incAuto
	for _, x := range in {
		if len(out) >= 40 {
			break
		}
		x.Nome = limita(x.Nome, 60)
		if x.Nome == "" || x.Livello < 0 || x.Livello > 9 {
			continue
		}
		var lv []profiloIncAuto
		for _, p := range x.Livelli {
			if len(lv) >= 10 {
				break
			}
			p.Nome, p.Livello = x.Nome, x.Livello
			if x.Livello == 0 {
				p.Slot = 0
			} else if p.Slot < x.Livello || p.Slot > 9 {
				continue
			}
			v, err := validaIncantesimoScontro(&p.incantesimoScontro)
			if err != nil {
				continue
			}
			p.incantesimoScontro = v
			p.Lancio = ""
			if p.Bersagli < 0 || p.Bersagli > 20 {
				p.Bersagli = 0
			}
			p.Riassunto = limita(p.Riassunto, 200)
			lv = append(lv, p)
		}
		if len(lv) > 0 {
			x.Livelli = lv
			out = append(out, x)
		}
	}
	return out
}

// validaSlot: slot totali per livello (1–9, al massimo 9 per livello).
func validaSlot(in map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range in {
		if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= 9 && v >= 1 && v <= 9 {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// slotDelPG: gli slot di un PG con quelli già usati (stato_in_gioco.slot_usati).
func (s *Server) slotDelPG(pg string, tot map[string]int) []slotPG {
	if len(tot) == 0 {
		return nil
	}
	var raw string
	s.DB.QueryRow(`SELECT COALESCE(slot_usati,'{}') FROM stato_in_gioco WHERE personaggio_id = ?`, pg).Scan(&raw)
	usati := map[string]int{}
	json.Unmarshal([]byte(raw), &usati)
	var out []slotPG
	for lv := 1; lv <= 9; lv++ {
		k := strconv.Itoa(lv)
		if tot[k] > 0 {
			out = append(out, slotPG{Livello: lv, Tot: tot[k], Usati: min(usati[k], tot[k])})
		}
	}
	return out
}

// controllaSlot: con «usaSlot» un PG lancia un incantesimo di 1° livello o più solo se ha uno slot libero del livello scelto.
func (s *Server) controllaSlot(a *vistaCombattente, inc incantesimoScontro) error {
	if inc.Livello == 0 {
		return nil
	}
	if a.Tipo != "pg" || a.Personaggio == "" {
		return errDatiNonValidi
	}
	for _, x := range s.slotDelPG(a.Personaggio, a.slotTot) {
		if x.Livello == inc.Slot && x.Usati < x.Tot {
			return nil
		}
	}
	return errore(http.StatusConflict, "Nessuno slot di "+strconv.Itoa(inc.Slot)+"° livello libero: recupera gli slot con un riposo.")
}

// usaSlot toglie lo slot (dopo che il lancio è andato a buon fine) e avvisa la scheda e il tracker.
func (s *Server) usaSlot(u *Utente, a *vistaCombattente, inc incantesimoScontro) {
	if inc.Livello == 0 || a.Personaggio == "" {
		return
	}
	var raw string
	s.DB.QueryRow(`SELECT COALESCE(slot_usati,'{}') FROM stato_in_gioco WHERE personaggio_id = ?`, a.Personaggio).Scan(&raw)
	usati := map[string]int{}
	json.Unmarshal([]byte(raw), &usati)
	k := strconv.Itoa(inc.Slot)
	usati[k]++
	b, _ := json.Marshal(usati)
	s.DB.Exec(`UPDATE stato_in_gioco SET slot_usati = ?, modificato_il = ?, modificato_da = ? WHERE personaggio_id = ?`, string(b), db.Ora(), u.ID, a.Personaggio)
	s.avvisaStatoInGioco(a.Personaggio, a.Proprietario)
}
