package server

// Viaggi del segnalino del gruppo (richiesta dell'utente, 2026-11-08).
//
// Quando il segnalino di un gruppo viene spostato sulla mappa di più di 1/4 di miglio, la pagina della Mappa calcola il
// percorso (strade e sentieri o fuori strada, come «Da … a …») con il tempo di viaggio e i tratti rallentati dal
// terreno, e lo manda insieme allo spostamento. Il server lo registra nel diario del gruppo, fa avanzare il tempo di
// gioco (ogni 24 ore di viaggio passa un giorno di gioco, come «Passa un giorno» delle Professioni) e, se ci sono eventi
// (arrivo a una località, terreno che rallenta, regole del Monte Gelo, marcia forzata), manda al gruppo e al Master
// l'avviso a tutto schermo. Senza eventi arriva solo un avviso breve.

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"gasterbless/internal/db"
)

const viaggioMinimo = 0.25 // miglia: sotto, lo spostamento è solo un aggiustamento della posizione

type pezzoViaggio struct {
	Tipo string  `json:"tipo"`
	Nome string  `json:"nome"`
	Mi   float64 `json:"mi"`
	F    float64 `json:"f"` // frazione della velocità (1, 3/4, 1/2)
}

type viaggioIn struct {
	Miglia float64        `json:"miglia"`
	Ore    float64        `json:"ore"`
	Fatica float64        `json:"fatica"`
	MPH    float64        `json:"mph"`
	Pezzi  []pezzoViaggio `json:"pezzi"`
	Arrivo string         `json:"arrivo"`
	Gelo   *struct {
		Mi    float64 `json:"mi"`
		Alta  float64 `json:"alta"`
		Quota float64 `json:"quota"`
	} `json:"gelo"`
}

type eventoViaggio struct {
	Tipo   string `json:"tipo"` // arrivo, terreno, gelo, marcia, giorno
	Icona  string `json:"icona"`
	Titolo string `json:"titolo"`
	Testo  string `json:"testo"`
	Nome   string `json:"-"` // (arrivo) nome vero della località, mostrato ai giocatori solo se svelato
	Chiave string `json:"-"`
}

var tipiPezzo = map[string]string{"strada": "strada", "pianura": "pianura", "foresta": "foresta", "collina": "collina", "montagna": "montagna",
	"acqua": "acqua", "gelo": "salita ghiacciata del Monte Gelo", "stradaCollina": "strada in collina", "stradaMontagna": "strada in montagna"}

func (v *viaggioIn) valido() bool {
	if v == nil || v.Miglia < 0 || v.Miglia > 300 || v.Ore < 0 || v.Ore > 500 || v.Fatica < v.Ore-1e-6 || v.Fatica > 1000 || len(v.Pezzi) > 200 {
		return false
	}
	if v.MPH != 2 && v.MPH != 3 && v.MPH != 4 {
		return false
	}
	for i := range v.Pezzi {
		p := &v.Pezzi[i]
		if _, ok := tipiPezzo[p.Tipo]; !ok || p.Mi < 0 || p.Mi > 300 || (p.F != 1 && p.F != .75 && p.F != .5) {
			return false
		}
		if r := []rune(strings.TrimSpace(p.Nome)); len(r) > 60 {
			p.Nome = string(r[:60])
		}
	}
	return len(v.Arrivo) <= 80
}

func mi1(x float64) string { return strings.Replace(fmt.Sprintf("%.1f", x), ".", ",", 1) }

func durata(h float64) string {
	m := int(math.Round(h * 60))
	return fmt.Sprintf("%d h %02d min", m/60, m%60)
}

// registraViaggio: diario, tempo di gioco, eventi e avvisi. Chiamata dopo lo spostamento del segnalino.
func (s *Server) registraViaggio(u *Utente, gruppo, sessione string, da, a [2]float64, v *viaggioIn) {
	if v == nil || v.Miglia < viaggioMinimo || !v.valido() {
		return
	}
	var eventi []eventoViaggio
	// Terreno che rallenta (esclusa la salita del Gelo, che ha il suo avviso)
	var lenti []string
	for _, p := range v.Pezzi {
		if p.F < 1 && p.Tipo != "gelo" && p.Mi >= 0.05 {
			fr := "metà velocità"
			if p.F == .75 {
				fr = "3/4 della velocità"
			}
			nome := ""
			if p.Nome != "" {
				nome = " (" + p.Nome + ")"
			}
			lenti = append(lenti, fmt.Sprintf("%s mi di %s%s a %s", mi1(p.Mi), tipiPezzo[p.Tipo], nome, fr))
		}
	}
	if len(lenti) > 0 {
		eventi = append(eventi, eventoViaggio{Tipo: "terreno", Icona: "⛰️", Titolo: "Terreno difficile",
			Testo: strings.Join(lenti, "; ") + ". Fuori strada foresta, collina, montagna e acqua dimezzano la velocità; su strada la collina la porta a 3/4 e la montagna a metà."})
	}
	if v.Gelo != nil && v.Gelo.Mi >= 0.05 {
		t := fmt.Sprintf("%s mi sulla salita ghiacciata (quota %s m): −20 °C, raffiche di vento gelato, ghiaccio e neve, metà velocità. Ogni ora: TS su Costituzione CD 10 contro il freddo e su Forza o Destrezza CD 12 contro il vento (il Master li tira dalla Mappa).",
			mi1(v.Gelo.Mi), migliaia(v.Gelo.Quota))
		if v.Gelo.Alta >= 0.05 {
			t += " Alta quota: ogni ora conta doppia per la fatica; contro il vento svantaggio e, con un 1 naturale, si precipita."
		}
		eventi = append(eventi, eventoViaggio{Tipo: "gelo", Icona: "❄️", Titolo: "Regole speciali del Monte Gelo", Testo: t})
	}
	// Arrivo a una località
	if v.Arrivo != "" {
		if l := s.cercaLocalita(v.Arrivo); l != nil && math.Hypot(l.X-a[0], l.Y-a[1]) <= 30 {
			eventi = append(eventi, eventoViaggio{Tipo: "arrivo", Icona: "📍", Titolo: "Arrivo", Nome: l.N, Chiave: l.K})
		}
	}
	// Tempo di gioco: ore di viaggio, marcia del giorno, giorni
	s.DB.Exec(`INSERT OR IGNORE INTO tempo_gruppo(gruppo_id, modificato_il) VALUES(?,?)`, gruppo, db.Ora())
	var oreGioco, marcia float64
	var giorno int
	s.DB.QueryRow(`SELECT ore_gioco, ore_marcia, giorno FROM tempo_gruppo WHERE gruppo_id = ?`, gruppo).Scan(&oreGioco, &marcia, &giorno)
	dopo := oreGioco + v.Ore
	nuoviGiorni := int(math.Floor(dopo/24)) - int(math.Floor(oreGioco/24))
	primaMarcia := marcia
	if nuoviGiorni > 0 {
		marcia = math.Min(v.Fatica, math.Mod(dopo, 24)) // ore di marcia del nuovo giorno
		primaMarcia = 0
		giorno += nuoviGiorni
	} else {
		marcia += v.Fatica
	}
	if marcia > 8+1e-6 {
		extra := marcia - math.Max(8, primaMarcia)
		cd := 10 + int(math.Ceil(marcia-8))
		eventi = append(eventi, eventoViaggio{Tipo: "marcia", Icona: "🥵", Titolo: "Marcia forzata",
			Testo: fmt.Sprintf("Oggi il gruppo ha marciato %s (oltre le 8 ore: %s in più con questo viaggio). Alla fine di ogni ora oltre le 8, TS su Costituzione con CD 10 + 1 per ogni ora in più (ora CD %d): se fallisce, 1 livello di indebolimento.",
				durata(marcia), durata(extra), cd)})
	}
	if nuoviGiorni > 0 {
		eventi = append(eventi, eventoViaggio{Tipo: "giorno", Icona: "🌅", Titolo: "Nuovo giorno di gioco",
			Testo: fmt.Sprintf("Con questo viaggio passa %s: ora è il giorno %d. Gli ingredienti delle professioni invecchiano.", map[bool]string{true: "un giorno", false: fmt.Sprintf("%d giorni", nuoviGiorni)}[nuoviGiorni == 1], giorno)})
	}
	s.DB.Exec(`UPDATE tempo_gruppo SET ore_gioco = ?, ore_marcia = ?, giorno = ?, modificato_il = ? WHERE gruppo_id = ?`, dopo, marcia, giorno, db.Ora(), gruppo)
	for i := 0; i < nuoviGiorni && i < 30; i++ {
		s.passaGiornoGioco(u)
	}
	if nuoviGiorni > 0 {
		s.fineDurevoliGiorno(u, gruppo)               // finiscono gli effetti fino a 24 ore (effetti_persistenti.go)
		go s.controllaInneschi(gruppo, "giorno", nil) // eventi del Codice «dal giorno N» (lore_promemoria.go)
	}
	// Diario
	pubblici := []map[string]any{}
	for _, e := range eventi {
		pubblici = append(pubblici, map[string]any{"tipo": e.Tipo, "icona": e.Icona, "titolo": e.Titolo, "testo": e.Testo, "k": e.Chiave})
	}
	det, _ := json.Marshal(map[string]any{"pezzi": v.Pezzi, "eventi": pubblici, "mph": v.MPH})
	arrivoK := ""
	for _, e := range eventi {
		if e.Tipo == "arrivo" {
			arrivoK = e.Chiave
		}
	}
	s.DB.Exec(`INSERT INTO viaggi(gruppo_id, sessione_id, utente_id, da_x, da_y, a_x, a_y, arrivo, miglia, ore, fatica, giorno, dettagli, creato_il)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, gruppo, nullo(sessione), u.ID, da[0], da[1], a[0], a[1], arrivoK, v.Miglia, v.Ore, v.Fatica, giorno, string(det), db.Ora())
	// Avviso: al gruppo e al Master; il nome della località arriva ai giocatori solo se svelato
	st, _ := s.leggiStatoMappa(gruppo)
	riepilogo := map[string]any{"miglia": v.Miglia, "ore": v.Ore, "giorno": giorno, "oreMarcia": marcia, "da": u.Nome, "quando": time.Now().UnixMilli()}
	s.Hub.PubblicaPer("gruppo:"+gruppo, func(c *Cliente) *Msg {
		ev := []map[string]any{}
		for _, e := range eventi {
			x := map[string]any{"tipo": e.Tipo, "icona": e.Icona, "titolo": e.Titolo, "testo": e.Testo}
			if e.Tipo == "arrivo" {
				nome := e.Nome
				if !c.U.Master() && (st == nil || !st.Nomi[e.Chiave]) {
					nome = "un luogo ancora sconosciuto"
				}
				x["titolo"] = "Arrivo: " + nome
				x["testo"] = "Il gruppo è arrivato a " + nome + "."
			}
			ev = append(ev, x)
		}
		return &Msg{Tipo: "viaggio.avviso", Dati: map[string]any{"gruppo": gruppo, "eventi": ev, "riepilogo": riepilogo}}
	})
	s.avvisaMappa(gruppo, map[string]any{"t": "viaggi"})
}

func migliaia(x float64) string {
	s := fmt.Sprintf("%d", int(math.Round(x)))
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune('.')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (s *Server) rotteViaggi(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/mappa/viaggi", api(s.apiViaggi))
	mux.Handle("POST /api/v1/mappa/viaggi/giorno", api(s.apiNuovoGiornoViaggio))
	mux.Handle("DELETE /api/v1/mappa/viaggi/ultimo", api(s.apiAnnullaViaggio))
}

// apiViaggi: tempo di gioco del gruppo e ultimi viaggi (i nomi delle località ai giocatori solo se svelati).
func (s *Server) apiViaggi(w http.ResponseWriter, r *http.Request) (any, error) {
	u, _, gruppo, err := s.contestoMappa(r, "")
	if err != nil {
		return nil, err
	}
	var oreGioco, marcia float64
	giorno := 1
	s.DB.QueryRow(`SELECT ore_gioco, ore_marcia, giorno FROM tempo_gruppo WHERE gruppo_id = ?`, gruppo).Scan(&oreGioco, &marcia, &giorno)
	var tot float64
	s.DB.QueryRow(`SELECT COALESCE(SUM(miglia),0) FROM viaggi WHERE gruppo_id = ?`, gruppo).Scan(&tot)
	st, _ := s.leggiStatoMappa(gruppo)
	righe, err := s.DB.Query(`SELECT v.id, v.arrivo, v.miglia, v.ore, v.fatica, v.giorno, v.dettagli, v.creato_il, COALESCE(x.nome,'')
		FROM viaggi v LEFT JOIN utenti x ON x.id = v.utente_id WHERE v.gruppo_id = ? ORDER BY v.id DESC LIMIT 30`, gruppo)
	if err != nil {
		return nil, err
	}
	defer righe.Close()
	lista := []map[string]any{}
	for righe.Next() {
		var id int64
		var arr, det, chi string
		var mi, ore, fat float64
		var g int
		var quando int64
		righe.Scan(&id, &arr, &mi, &ore, &fat, &g, &det, &quando, &chi)
		var d struct {
			Eventi []map[string]any `json:"eventi"`
		}
		json.Unmarshal([]byte(det), &d)
		nome := ""
		if l := s.cercaLocalita(arr); l != nil {
			nome = l.N
			if !u.Master() && (st == nil || !st.Nomi[arr]) {
				nome = "un luogo ancora sconosciuto"
			}
		}
		tipi := []string{}
		for _, e := range d.Eventi {
			if t, _ := e["tipo"].(string); t != "" && t != "arrivo" {
				tipi = append(tipi, t)
			}
		}
		lista = append(lista, map[string]any{"id": id, "arrivo": nome, "miglia": mi, "ore": ore, "fatica": fat, "giorno": g, "quando": quando, "chi": chi, "eventi": tipi})
	}
	return map[string]any{"giorno": giorno, "oreGioco": oreGioco, "oreMarcia": marcia, "migliaTotali": tot, "lista": lista}, nil
}

// apiNuovoGiornoViaggio (Master): il gruppo si accampa: comincia un nuovo giorno di gioco, la marcia riparte da zero.
func (s *Server) apiNuovoGiornoViaggio(w http.ResponseWriter, r *http.Request) (any, error) {
	u, _, gruppo, err := s.contestoMappa(r, "master")
	if err != nil {
		return nil, err
	}
	s.DB.Exec(`INSERT OR IGNORE INTO tempo_gruppo(gruppo_id, modificato_il) VALUES(?,?)`, gruppo, db.Ora())
	s.DB.Exec(`UPDATE tempo_gruppo SET ore_gioco = (CAST(ore_gioco / 24 AS INTEGER) + 1) * 24, ore_marcia = 0, giorno = giorno + 1, modificato_il = ? WHERE gruppo_id = ?`, db.Ora(), gruppo)
	s.passaGiornoGioco(u)
	s.fineDurevoliGiorno(u, gruppo)               // finiscono gli effetti fino a 24 ore (effetti_persistenti.go)
	go s.controllaInneschi(gruppo, "giorno", nil) // eventi del Codice «dal giorno N» (lore_promemoria.go)
	var giorno int
	s.DB.QueryRow(`SELECT giorno FROM tempo_gruppo WHERE gruppo_id = ?`, gruppo).Scan(&giorno)
	s.avvisaMappa(gruppo, map[string]any{"t": "viaggi"})
	return map[string]any{"giorno": giorno}, nil
}

// apiAnnullaViaggio (Master): toglie l'ultimo viaggio (per esempio un segnalino spostato per sbaglio) e le sue ore.
// Il segnalino resta dov'è: lo si riporta indietro trascinandolo (di meno di 1/4 di miglio non conta) o con «Posiziona».
func (s *Server) apiAnnullaViaggio(w http.ResponseWriter, r *http.Request) (any, error) {
	_, _, gruppo, err := s.contestoMappa(r, "master")
	if err != nil {
		return nil, err
	}
	var id int64
	var ore, fat float64
	var g int
	if s.DB.QueryRow(`SELECT id, ore, fatica, giorno FROM viaggi WHERE gruppo_id = ? ORDER BY id DESC LIMIT 1`, gruppo).Scan(&id, &ore, &fat, &g) != nil {
		return nil, errore(http.StatusConflict, "Nessun viaggio da annullare.")
	}
	s.DB.Exec(`DELETE FROM viaggi WHERE id = ?`, id)
	s.DB.Exec(`UPDATE tempo_gruppo SET ore_gioco = MAX(0, ore_gioco - ?), ore_marcia = MAX(0, ore_marcia - ?), modificato_il = ? WHERE gruppo_id = ? AND giorno = ?`, ore, fat, db.Ora(), gruppo, g)
	s.avvisaMappa(gruppo, map[string]any{"t": "viaggi"})
	return map[string]any{"annullato": id}, nil
}
