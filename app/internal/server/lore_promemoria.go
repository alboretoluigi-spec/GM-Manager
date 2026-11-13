package server

// Eventi del Codice della campagna con «Innesco:» e promemoria per gruppo (fase L3, 2026-11-12; migrazione 034).
// Gaster Bless capisce tre inneschi, scritti come vengono (anche da ChatGPT):
//   - arrivo in una località della mappa: «quando il gruppo arriva a Borgofosco», «all'ingresso nel Villaggio di Nebbiavalle» →
//     scatta quando un segnalino del gruppo è vicino a quella località (mappa_segnalino.go);
//   - inizio sessione: «alla prossima sessione», «all'inizio della sessione» → quando una sessione del gruppo passa a
//     «in corso» (gruppi.go);
//   - giorno di gioco: «dal giorno 5», «giorno 12» → quando il giorno del gruppo arriva a quel numero (viaggi, riposi).
// Un innesco scatta una volta per scheda e gruppo, solo se per quel gruppo la scheda non è già «incontrato» o
// «concluso»; lascia un promemoria al Master (avviso in tempo reale e riquadro nel Codice), che lo segna avvenuto
// (stato «incontrato» per quel gruppo) o lo ignora. Gli altri inneschi restano testo per il Master.

import (
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"gasterbless/internal/db"
)

const raggioArrivoLore = 40.0 // px sulla mappa: il gruppo è «arrivato» in una località

type innescoLore struct {
	Tipo   string `json:"tipo"`             // arrivo, sessione, giorno
	Luogo  string `json:"luogo,omitempty"`  // nome della località della mappa
	Giorno int    `json:"giorno,omitempty"` // giorno di gioco
	Testo  string `json:"testo"`            // come lo ricorda Gaster Bless
	k      string
}

var (
	reArrivoLore  = regexp.MustCompile(`\b(arriv|giung|entr|raggiung|torn|rientr|mett\w* piede|visit)\w*`)
	reGiornoLore  = regexp.MustCompile(`\bgiorno\s+(\d{1,4})\b`)
	reSessioneLor = regexp.MustCompile(`\bsession[ei]\b`)
)

// leggiInnesco interpreta il campo «Innesco» (o «Quando», se dice «arriva a…»/«giorno N»/«sessione»).
func (s *Server) leggiInnesco(campi map[string]string) *innescoLore {
	var testo string
	for k, v := range campi {
		if strings.EqualFold(strings.TrimSpace(k), "innesco") {
			testo = v
		}
	}
	if testo == "" {
		return nil
	}
	t := normalizza(testo)
	if reArrivoLore.MatchString(t) {
		var best *localita
		for i, l := range s.localita() {
			n := normalizza(l.N)
			if n != "" && strings.Contains(t, n) && (best == nil || len(n) > len(normalizza(best.N))) {
				best = &s.localita()[i]
			}
		}
		if best != nil {
			return &innescoLore{Tipo: "arrivo", Luogo: best.N, Testo: "all'arrivo del gruppo a " + best.N, k: best.K}
		}
	}
	if m := reGiornoLore.FindStringSubmatch(t); m != nil {
		g, _ := strconv.Atoi(m[1])
		if g > 0 {
			return &innescoLore{Tipo: "giorno", Giorno: g, Testo: "dal giorno di gioco " + m[1]}
		}
	}
	if reSessioneLor.MatchString(t) {
		return &innescoLore{Tipo: "sessione", Testo: "all'inizio della prossima sessione del gruppo"}
	}
	return nil
}

// controllaInneschi: per un gruppo, crea i promemoria degli eventi il cui innesco è vero adesso. tipo: arrivo, sessione,
// giorno (si controllano solo gli inneschi di quel tipo). seg: i segnalini del gruppo (per gli arrivi).
func (s *Server) controllaInneschi(gruppo, tipo string, seg *datiSegnalini) {
	l, err := s.caricaLore(`WHERE campi LIKE '%nnesco%'`)
	if err != nil || len(l) == 0 {
		return
	}
	var giorno int
	if tipo == "giorno" {
		_, giorno = s.oreGiocoGruppo(gruppo)
	}
	var nomeGruppo string
	s.DB.QueryRow(`SELECT nome FROM gruppi WHERE id = ?`, gruppo).Scan(&nomeGruppo)
	for _, x := range l {
		in := s.leggiInnesco(x.Campi)
		if in == nil || in.Tipo != tipo {
			continue
		}
		vero := false
		switch tipo {
		case "sessione":
			vero = true
		case "giorno":
			vero = giorno >= in.Giorno
		case "arrivo":
			a := s.cercaLocalita(in.k)
			if a != nil && seg != nil {
				for _, p := range seg.Segnalini {
					if math.Hypot(p.X-a.X, p.Y-a.Y) <= raggioArrivoLore {
						vero = true
					}
				}
			}
		}
		if !vero {
			continue
		}
		var stato string
		s.DB.QueryRow(`SELECT stato FROM lore_stato WHERE scheda_id = ? AND gruppo_id = ?`, x.ID, gruppo).Scan(&stato)
		if stato == "incontrato" || stato == "concluso" {
			continue
		}
		res, err := s.DB.Exec(`INSERT OR IGNORE INTO lore_promemoria(scheda_id, gruppo_id, motivo, creato_il) VALUES(?,?,?,?)`, x.ID, gruppo, in.Testo, db.Ora())
		if err != nil {
			continue
		}
		if n, _ := res.RowsAffected(); n == 1 {
			s.Hub.AlMaster(Msg{Tipo: "lore.promemoria", Dati: map[string]any{"scheda": x.ID, "nome": x.Nome, "tipo": x.Tipo,
				"gruppo": gruppo, "nomeGruppo": nomeGruppo, "motivo": in.Testo}})
		}
	}
}

// controllaInneschiSessione: una sessione è iniziata.
func (s *Server) controllaInneschiSessione(sessione string) {
	var g string
	if s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, sessione).Scan(&g) == nil {
		s.controllaInneschi(g, "sessione", nil)
	}
}

type promemoriaLore struct {
	Scheda     string `json:"scheda"`
	Nome       string `json:"nome"`
	Tipo       string `json:"tipo"`
	Gruppo     string `json:"gruppo"`
	NomeGruppo string `json:"nomeGruppo"`
	Motivo     string `json:"motivo"`
	Creato     int64  `json:"creato"`
}

func (s *Server) promemoriaAperti() []promemoriaLore {
	out := []promemoriaLore{}
	rows, err := s.DB.Query(`SELECT p.scheda_id, l.nome, l.tipo, p.gruppo_id, g.nome, p.motivo, p.creato_il FROM lore_promemoria p
		JOIN lore_schede l ON l.id = p.scheda_id JOIN gruppi g ON g.id = p.gruppo_id WHERE p.chiuso = 0 ORDER BY p.creato_il`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var x promemoriaLore
		if rows.Scan(&x.Scheda, &x.Nome, &x.Tipo, &x.Gruppo, &x.NomeGruppo, &x.Motivo, &x.Creato) == nil {
			out = append(out, x)
		}
	}
	return out
}

// apiPromemoriaLore: POST /lore/{id}/promemoria {gruppo, azione: avvenuto | ignora | riapri}.
func (s *Server) apiPromemoriaLore(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Gruppo string `json:"gruppo"`
		Azione string `json:"azione"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	switch in.Azione {
	case "avvenuto": // l'evento è successo per quel gruppo
		res, _ := s.DB.Exec(`UPDATE lore_promemoria SET chiuso = 1 WHERE scheda_id = ? AND gruppo_id = ?`, id, in.Gruppo)
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, errNonTrovato
		}
		s.DB.Exec(`INSERT INTO lore_stato(scheda_id, gruppo_id, stato, nota, modificato_il) VALUES(?,?,'incontrato','',?)
			ON CONFLICT(scheda_id, gruppo_id) DO UPDATE SET stato = 'incontrato', modificato_il = excluded.modificato_il`, id, in.Gruppo, db.Ora())
	case "ignora":
		res, _ := s.DB.Exec(`UPDATE lore_promemoria SET chiuso = 1 WHERE scheda_id = ? AND gruppo_id = ?`, id, in.Gruppo)
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, errNonTrovato
		}
	case "riapri": // l'innesco può scattare di nuovo per quel gruppo
		s.DB.Exec(`DELETE FROM lore_promemoria WHERE scheda_id = ? AND gruppo_id = ?`, id, in.Gruppo)
	default:
		return nil, errDatiNonValidi
	}
	return nil, nil
}
