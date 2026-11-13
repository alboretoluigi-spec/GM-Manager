package server

// Preparazione della sessione (fase L5 del Codice della campagna, 2026-11-12; solo Master). In una pagina, per un gruppo
// e la sua prossima sessione (o quella in corso), tutto quello che serve prima di giocare:
//   - chi viene (personaggi confermati, con livello, classe, PF e condizioni);
//   - dove e quando: la località del segnalino del gruppo sulla mappa e il giorno di gioco;
//   - dove eravamo rimasti: l'ultimo riassunto;
//   - i promemoria aperti degli eventi del Codice (lore_promemoria.go);
//   - le schede del Codice pertinenti al gruppo, con il motivo (stessa scelta dell'assistente IA, lore_ia.go);
//   - le «Decisioni aperte» che le riguardano e i bilanciamenti salvati per il gruppo (bilanciamento.go);
//   - le note della sessione (le stesse di «Sessioni») e la domanda pronta per chiedere all'IA una scaletta.
// PNG e mostri da tenere a portata di mano li trova la pagina dalle schede (web/ui/codice_link.js).

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

const maxSchedePreparazione = 30

func (s *Server) rottePreparazione(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/preparazione", api(s.apiPreparazione))
}

type schedaPrep struct {
	ID         string            `json:"id"`
	Tipo       string            `json:"tipo"`
	Nome       string            `json:"nome"`
	Campi      map[string]string `json:"campi"`
	Etichette  []string          `json:"etichette"`
	Stato      string            `json:"stato,omitempty"`
	Motivi     []string          `json:"motivi"`
	Sintesi    string            `json:"sintesi"`
	TestoPieno string            `json:"testo,omitempty"` // solo per decisioni aperte e bilanciamenti
}

func sintesiLore(t string, n int) string {
	t = strings.Join(strings.Fields(t), " ")
	if r := []rune(t); len(r) > n {
		return string(r[:n]) + "…"
	}
	return t
}

func (s *Server) apiPreparazione(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	gruppo := r.URL.Query().Get("gruppo")
	var nomeGruppo string
	if s.DB.QueryRow(`SELECT nome FROM gruppi WHERE id = ?`, gruppo).Scan(&nomeGruppo) != nil {
		return nil, errNonTrovato
	}
	out := map[string]any{"gruppo": gruppo, "nomeGruppo": nomeGruppo}

	// sessioni non concluse e quella scelta (indicata, in corso o la prossima)
	sessioni := []map[string]any{}
	if rows, err := s.DB.Query(`SELECT id, titolo, stato, inizio_il FROM sessioni_gioco WHERE gruppo_id = ? AND stato != 'conclusa' ORDER BY inizio_il`, gruppo); err == nil {
		for rows.Next() {
			var id, t, st string
			var in int64
			rows.Scan(&id, &t, &st, &in)
			sessioni = append(sessioni, map[string]any{"id": id, "titolo": t, "stato": st, "inizio": in})
		}
		rows.Close()
	}
	out["sessioni"] = sessioni
	ses, _, _ := s.sessioneBil(gruppo, r.URL.Query().Get("sessione"))
	var titoloSes string
	if ses != "" {
		var luogoSes, note, stato string
		var inizio int64
		s.DB.QueryRow(`SELECT titolo, inizio_il, luogo, note, stato FROM sessioni_gioco WHERE id = ?`, ses).Scan(&titoloSes, &inizio, &luogoSes, &note, &stato)
		out["sessione"] = map[string]any{"id": ses, "titolo": titoloSes, "inizio": inizio, "luogo": luogoSes, "note": note, "stato": stato}
	}

	// chi viene: i confermati della sessione; senza sessione, i personaggi attivi del gruppo
	type pgPrep struct {
		ID         string   `json:"id"`
		Nome       string   `json:"nome"`
		Giocatore  string   `json:"giocatore"`
		Classi     string   `json:"classi"`
		Livello    int      `json:"livello"`
		PF         *int     `json:"pf"`
		PFMax      int      `json:"pfMax"`
		Condizioni []string `json:"condizioni"`
		Confermato bool     `json:"confermato"`
	}
	presenti := []pgPrep{}
	q := `SELECT p.id, p.nome, COALESCE(u.nome,''), COALESCE(v.riepilogo,'{}'), sg.pf, COALESCE(sg.condizioni,'[]'), %s
		FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id
		LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		LEFT JOIN stato_in_gioco sg ON sg.personaggio_id = p.id `
	var rows *sql.Rows
	var err error
	if ses != "" {
		rows, err = s.DB.Query(fmt.Sprintf(q, "pa.confermato")+`JOIN partecipanti pa ON pa.personaggio_id = p.id WHERE pa.sessione_id = ? ORDER BY p.nome`, ses)
	}
	if ses == "" || err != nil {
		rows, err = s.DB.Query(fmt.Sprintf(q, "0")+`WHERE p.gruppo_id = ? AND p.stato = 'attivo' AND p.proprietario_id IS NOT NULL ORDER BY p.nome`, gruppo)
	}
	if err == nil {
		for rows.Next() {
			var x pgPrep
			var riep, cond string
			var conf int
			rows.Scan(&x.ID, &x.Nome, &x.Giocatore, &riep, &x.PF, &cond, &conf)
			var rp riepilogo
			json.Unmarshal([]byte(riep), &rp)
			x.Classi, x.Livello, x.PFMax, x.Confermato = rp.Classe, rp.Livello, rp.PFMax, conf == 1
			json.Unmarshal([]byte(cond), &x.Condizioni)
			if x.Condizioni == nil {
				x.Condizioni = []string{}
			}
			presenti = append(presenti, x)
		}
		rows.Close()
	}
	out["presenti"] = presenti

	// dove e quando
	ore, giorno := s.oreGiocoGruppo(gruppo)
	out["giorno"], out["ore"] = giorno, ore

	// dove eravamo rimasti
	var rTit, rTesto, rAut string
	var rQuando int64
	if s.DB.QueryRow(`SELECT titolo, testo, autore_nome, creato_il FROM riassunti WHERE gruppo_id = ? ORDER BY creato_il DESC LIMIT 1`, gruppo).Scan(&rTit, &rTesto, &rAut, &rQuando) == nil {
		out["riassunto"] = map[string]any{"titolo": rTit, "testo": sintesiLore(rTesto, 1500), "autore": rAut, "quando": rQuando}
	}

	// promemoria del gruppo
	prom := []promemoriaLore{}
	for _, p := range s.promemoriaAperti() {
		if p.Gruppo == gruppo {
			prom = append(prom, p)
		}
	}
	out["promemoria"] = prom

	// Codice: schede pertinenti, decisioni aperte, bilanciamenti del gruppo
	schede, decisioni, bilanciamenti := []schedaPrep{}, []schedaPrep{}, []schedaPrep{}
	a := s.analizzaLore(gruppo, "")
	luogo := s.luogoDelGruppo(gruppo)
	if a != nil {
		luogo = a.luogo
		l := append([]schedaLore(nil), a.l...)
		sort.SliceStable(l, func(i, j int) bool { return a.punti[l[i].ID] > a.punti[l[j].ID] })
		for _, x := range l {
			p := schedaPrep{ID: x.ID, Tipo: x.Tipo, Nome: x.Nome, Campi: x.Campi, Etichette: x.Etichette, Stato: a.stati[x.ID].Stato,
				Motivi: a.motivi[x.ID], Sintesi: sintesiLore(x.TestoMaster, 220)}
			if p.Motivi == nil {
				p.Motivi = []string{}
			}
			switch {
			case x.Tipo == "trama" && strings.HasPrefix(x.Nome, "Bilanciamento – "):
				if x.Campi["Gruppo"] == nomeGruppo {
					p.TestoPieno = x.TestoMaster
					bilanciamenti = append(bilanciamenti, p)
				}
			case x.Tipo == "segreto" && strings.Contains(strings.ToLower(x.Nome), "decisioni aperte"):
				if a.punti[x.ID] > 0 {
					p.TestoPieno = x.TestoMaster
					decisioni = append(decisioni, p)
				}
			case a.punti[x.ID] > 0 && len(schede) < maxSchedePreparazione:
				schede = append(schede, p)
			}
		}
	}
	out["luogo"] = luogo
	out["schede"], out["decisioni"], out["bilanciamenti"] = schede, decisioni, bilanciamenti

	// domanda pronta per l'IA (non inviata): cita le schede principali, così l'assistente le riceve tutte (lore_ia.go)
	var nomi []string
	for _, x := range prom {
		nomi = append(nomi, "«"+x.Nome+"»")
	}
	for _, x := range schede {
		if len(nomi) >= 8 {
			break
		}
		if n := "«" + x.Nome + "»"; !strings.Contains(strings.Join(nomi, ","), n) {
			nomi = append(nomi, n)
		}
	}
	d := "Prepara la scaletta della prossima sessione del gruppo " + nomeGruppo
	if titoloSes != "" {
		d += " («" + titoloSes + "»)"
	}
	d += "."
	if luogo != "" {
		d += " Il gruppo si trova a " + luogo + "."
	}
	d += " Parti dall'ultimo riassunto"
	if len(nomi) > 0 {
		d += " e tieni conto di " + strings.Join(nomi, ", ")
	}
	d += ". Proponi da 3 a 5 scene, ognuna con PNG, indizi da far trovare e possibili sviluppi; per le «Decisioni aperte» proponi opzioni dichiarandole tali, senza deciderle. Tieni segreto ciò che è «Per il Master»."
	out["domandaIA"] = d
	return out, nil
}
