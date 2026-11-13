package server

// Personaggi e schede: versioni, unione delle modifiche contemporanee, stato in gioco, ritratto.
//
// I dati della scheda sono il JSON prodotto dalla scheda 5e (01_scheda). Per confrontarli il server
// li "appiattisce" in percorsi: le chiavi principali ("lvl", "cls", "wp"...) e, per gli oggetti, un
// livello più in profondità ("f.nome", "base.FOR", "sk.Atletica"...). Ogni percorso è un "campo".
//
// Regole delle modifiche contemporanee (docs/FASE_2_PROGETTO.md, sezione 5):
//   - la pagina invia solo i campi cambiati e la versione da cui è partita (base);
//   - se nel frattempo altri hanno cambiato campi DIVERSI, le modifiche si uniscono;
//   - se hanno cambiato LO STESSO campo con un valore diverso, il server non sovrascrive e
//     restituisce il conflitto: la pagina chiede chi tenere e reinvia indicando i campi da forzare.
//
// I campi "in gioco" (PF, PF temporanei, tiri contro la morte, slot usati, ispirazione) cambiano
// spesso: non creano versioni ma stanno in stato_in_gioco, condiviso con il tracker dell'iniziativa.

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const (
	maxScheda        = 6 << 20 // la scheda contiene anche il ritratto (immagine in base64)
	maxCampiModifica = 2000
	finestraVersioni = 10 * 60 // secondi: le modifiche di seguito dello stesso autore si raggruppano
)

// Campi della scheda che vivono in stato_in_gioco invece che nelle versioni.
// «exh» è il livello di indebolimento della scheda: vive nelle condizioni («Indebolimento N»), come nel tracker.
var campiInGioco = map[string]bool{"hpCur": true, "hpTmp": true, "ds": true, "slotU": true, "insp": true, "exh": true}

func campoInGioco(p string) bool {
	k, _, _ := strings.Cut(p, ".")
	return campiInGioco[k]
}

func (s *Server) rottePersonaggi(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/personaggi", api(s.apiPersonaggi))
	mux.Handle("POST /api/v1/personaggi", api(s.apiNuovoPersonaggio))
	mux.Handle("GET /api/v1/personaggi/{id}", api(s.apiPersonaggio))
	mux.Handle("PATCH /api/v1/personaggi/{id}/scheda", api(s.apiSalvaScheda))
	mux.Handle("GET /api/v1/personaggi/{id}/versioni", api(s.apiVersioni))
	mux.Handle("GET /api/v1/personaggi/{id}/versioni/{n}", api(s.apiVersione))
	mux.Handle("POST /api/v1/personaggi/{id}/ripristina/{n}", api(s.apiRipristina))
	mux.Handle("PATCH /api/v1/personaggi/{id}/stato-in-gioco", api(s.apiStatoInGioco))
	mux.Handle("POST /api/v1/personaggi/{id}/stato", api(s.apiStatoPersonaggio))
	mux.Handle("POST /api/v1/personaggi/{id}/assegna", api(s.apiAssegnaPersonaggio)) // personaggi_assegna.go
	mux.Handle("DELETE /api/v1/personaggi/{id}", api(s.apiEliminaPersonaggio))       // personaggi_assegna.go
	mux.Handle("GET /api/v1/personaggi/{id}/ritratto", http.HandlerFunc(s.ritratto))
	s.rotteBackground(mux)
}

// ===== Appiattimento dei dati della scheda =====

type piatto map[string]json.RawMessage

// appiattisci trasforma la scheda in campi. Gli oggetti vuoti diventano un campo con valore {}.
func appiattisci(dati map[string]json.RawMessage) piatto {
	out := piatto{}
	for k, v := range dati {
		v = bytes.TrimSpace(v)
		if len(v) > 0 && v[0] == '{' {
			var sotto map[string]json.RawMessage
			if json.Unmarshal(v, &sotto) == nil && len(sotto) > 0 {
				for k2, v2 := range sotto {
					out[k+"."+k2] = compatta(v2)
				}
				continue
			}
		}
		out[k] = compatta(v)
	}
	return out
}

// ricomponi è l'operazione inversa di appiattisci.
func ricomponi(p piatto) ([]byte, error) {
	top := map[string]json.RawMessage{}
	sotto := map[string]map[string]json.RawMessage{}
	for k, v := range p {
		if a, b, ok := strings.Cut(k, "."); ok {
			if sotto[a] == nil {
				sotto[a] = map[string]json.RawMessage{}
			}
			sotto[a][b] = v
		} else {
			top[k] = v
		}
	}
	for k, m := range sotto {
		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		top[k] = b
	}
	return json.Marshal(top)
}

// compatta normalizza la rappresentazione JSON, così due valori uguali hanno gli stessi byte.
func compatta(v json.RawMessage) json.RawMessage {
	var x any
	d := json.NewDecoder(bytes.NewReader(v))
	d.UseNumber()
	if d.Decode(&x) != nil {
		return v
	}
	b, err := json.Marshal(x)
	if err != nil {
		return v
	}
	return b
}

func uguale(a, b json.RawMessage) bool { return bytes.Equal(a, b) }

func leggiPiatto(dati string) (piatto, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(dati), &m); err != nil {
		return nil, err
	}
	return appiattisci(m), nil
}

// ===== Viste =====

type riepilogo struct {
	Classe     string `json:"classe,omitempty"`
	Sottocl    string `json:"sottoclasse,omitempty"`
	Livello    int    `json:"livello,omitempty"`
	Razza      string `json:"razza,omitempty"`
	CA         int    `json:"ca,omitempty"`
	PFMax      int    `json:"pfMax,omitempty"`
	Iniziativa int    `json:"iniziativa"`
	Des        int    `json:"des,omitempty"`
	Velocita   any    `json:"velocita,omitempty"`
	PercPass   int    `json:"percezionePassiva,omitempty"`
	CDInc      *int   `json:"cdIncantesimi,omitempty"`
	// Per i riposi (riposi.go, 2026-11-11): Dadi Vita totali per tipo, modificatore di Costituzione, slot del patto.
	DadiVita []dadoVita `json:"dadiVita,omitempty"`
	ModCos   *int       `json:"modCos,omitempty"`
	Patto    *slotPatto `json:"patto,omitempty"`
	AttInc   *int       `json:"attaccoIncantesimi,omitempty"`
	// Per Sanità mentale e Terrore (portati dal ramo main-ke8sy4): punteggi delle sei caratteristiche,
	// Sanità mentale con il metodo usato, competenze nei tiri salvezza.
	Car       map[string]int `json:"car,omitempty"`
	San       *int           `json:"san,omitempty"`
	SanMetodo string         `json:"sanMetodo,omitempty"`
	TS        []string       `json:"ts,omitempty"`
	// Professioni pronte (competenza nello strumento e strumento nell'equipaggiamento) e abilità con competenza,
	// calcolate dalla scheda: servono alla funzione «Professioni».
	Professioni []string `json:"professioni,omitempty"`
	Abilita     []string `json:"abilita,omitempty"`
	// Attacchi automatici dal tracker (attacchi.go): profili d'attacco calcolati dalla scheda, attacchi per azione
	// (Attacco Extra), resistenze e immunità ai danni (oggetti magici).
	Attacchi          []profiloAttacco   `json:"attacchi,omitempty"`
	AttacchiPerAzione int                `json:"attacchiPerAzione,omitempty"`
	Difese            *difeseCombattente `json:"difese,omitempty"`
	// Difese dei personaggi (2026-11-12, web/regole/difese.js): tiri salvezza come li calcola la scheda (competenza, Aura di
	// protezione, oggetti), immunità alle condizioni, condizioni contro cui ha vantaggio ai TS, caratteristiche con vantaggio
	// ai TS contro gli incantesimi.
	TSTot    map[string]int `json:"tsTot,omitempty"`
	ImmCond  []string       `json:"immCond,omitempty"`
	VantTS   []string       `json:"vantTS,omitempty"`
	ResMagia []string       `json:"resMagia,omitempty"`
	// Ira del barbaro (effetti_valori.go): modifiche calcolate dalla scheda (resistenze, vantaggio ai TS su Forza, danni in
	// mischia; Totem dell'orso, Berserker). Presente solo per i barbari.
	Ira *modificheEffetto `json:"ira,omitempty"`
	// Incantesimi automatici pronti per ogni livello di slot e slot totali (incantesimi_remoto.go, 2026-11-12).
	IncAuto  []incAuto      `json:"incAuto,omitempty"`
	Slot     map[string]int `json:"slot,omitempty"`
	Ritratto bool           `json:"-"` // calcolato da datiPG, non salvato
}

type vistaPersonaggio struct {
	ID            string    `json:"id"`
	Nome          string    `json:"nome"`
	Stato         string    `json:"stato"`
	Proprietario  string    `json:"proprietario"`
	NomeGiocatore string    `json:"nomeGiocatore"`
	Gruppo        string    `json:"gruppo,omitempty"`
	Versione      int       `json:"versione"`
	Modificato    int64     `json:"modificato"`
	Riepilogo     riepilogo `json:"riepilogo"`
	HaRitratto    bool      `json:"haRitratto"`
	HaBackground  bool      `json:"haBackground"`
	Scheda        string    `json:"scheda,omitempty"` // scheda alternativa (plugin); vuoto = scheda di Gaster Bless
}

const selPersonaggio = `SELECT p.id, p.nome, p.stato, COALESCE(p.proprietario_id,''), COALESCE(u.nome,''), COALESCE(p.gruppo_id,''),
	p.versione_corrente, p.modificato_il, COALESCE(v.riepilogo,'{}'), p.ritratto <> '',
	EXISTS(SELECT 1 FROM background b WHERE b.personaggio_id = p.id), p.scheda
	FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id
	LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente`

func scanPersonaggio(sc interface{ Scan(...any) error }) (*vistaPersonaggio, error) {
	v := &vistaPersonaggio{}
	var riep string
	if err := sc.Scan(&v.ID, &v.Nome, &v.Stato, &v.Proprietario, &v.NomeGiocatore, &v.Gruppo, &v.Versione, &v.Modificato, &riep, &v.HaRitratto, &v.HaBackground, &v.Scheda); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(riep), &v.Riepilogo)
	return v, nil
}

// apiPersonaggi: il giocatore vede i propri personaggi, il Master tutti (anche quelli anonimi).
func (s *Server) apiPersonaggi(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	q, args := selPersonaggio+` WHERE p.proprietario_id = ? ORDER BY p.stato <> 'attivo', p.nome`, []any{u.ID}
	if u.Master() {
		q, args = selPersonaggio+` ORDER BY p.stato <> 'attivo', u.nome, p.nome`, nil
	}
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*vistaPersonaggio{}
	for rows.Next() {
		v, err := scanPersonaggio(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// apiNuovoPersonaggio crea un personaggio con la scheda vuota (o con quella importata).
// Il Master può crearlo per conto di un giocatore indicando "proprietario".
func (s *Server) apiNuovoPersonaggio(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	var in struct {
		Nome         string
		Proprietario string
		Gruppo       string
		Dati         json.RawMessage
		Scheda       string // scheda alternativa (plugin) abilitata dal Master; vuoto = scheda di Gaster Bless
	}
	if err := leggiJSONMax(r, &in, maxScheda); err != nil {
		return nil, err
	}
	nome, err := rigaSingola(in.Nome, "Nome del personaggio", 1, 60)
	if err != nil {
		return nil, err
	}
	if in.Scheda != "" && !s.pluginAbilitato(in.Scheda) {
		return nil, errore(http.StatusBadRequest, "Questa scheda alternativa non è disponibile: chiedi al Master di abilitarla.")
	}
	prop := u.ID
	if in.Proprietario != "" && in.Proprietario != u.ID {
		if !u.Master() {
			return nil, errVietato
		}
		if st, err := s.giocatore(in.Proprietario); err != nil || st != "approvato" {
			return nil, errore(http.StatusBadRequest, "Giocatore non valido.")
		}
		prop = in.Proprietario
	}
	if in.Gruppo != "" {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM iscrizioni WHERE utente_id = ? AND gruppo_id = ?`, prop, in.Gruppo).Scan(&n)
		if n == 0 {
			return nil, errore(http.StatusBadRequest, "Il giocatore non fa parte di quel gruppo.")
		}
	} else if g := s.gruppiDi(&Utente{ID: prop, Ruolo: "giocatore", Stato: "approvato"}); len(g) == 1 {
		in.Gruppo = g[0] // un solo gruppo: si assegna da sé
	}
	dati := map[string]json.RawMessage{}
	if len(in.Dati) > 0 && string(in.Dati) != "null" {
		if err := json.Unmarshal(in.Dati, &dati); err != nil {
			return nil, errore(http.StatusBadRequest, "La scheda importata non è valida.")
		}
	}
	// Il nome scelto vale anche dentro la scheda.
	f := map[string]json.RawMessage{}
	if v, ok := dati["f"]; ok {
		json.Unmarshal(v, &f)
	}
	f["nome"], _ = json.Marshal(nome)
	dati["f"], _ = json.Marshal(f)
	piatti := appiattisci(dati)
	gioco := estraiInGioco(piatti)
	b, err := ricomponi(piatti)
	if err != nil {
		return nil, err
	}
	id := cripto.ID()
	ora := db.Ora()
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO personaggi(id, proprietario_id, gruppo_id, nome, versione_corrente, creato_il, modificato_il, scheda) VALUES(?,?,?,?,1,?,?,?)`,
			id, prop, nullo(in.Gruppo), nome, ora, ora, in.Scheda); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO schede_versioni(personaggio_id, numero, dati, riepilogo, campi, autore_id, nota, creato_il) VALUES(?,1,?,'{}','[]',?,?,?)`,
			id, string(b), u.ID, "creata", ora); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO stato_in_gioco(personaggio_id, modificato_il, modificato_da) VALUES(?,?,?)`, id, ora, u.ID)
		if err != nil {
			return err
		}
		return applicaInGioco(tx, id, gioco, u.ID)
	})
	if err != nil {
		return nil, err
	}
	s.avvisaScheda(id, prop, 1, u, nil)
	return map[string]any{"id": id, "versione": 1}, nil
}

// apiPersonaggio restituisce la scheda completa (versione corrente + stato in gioco).
func (s *Server) apiPersonaggio(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, id) {
		return nil, errNonTrovato
	}
	v, err := scanPersonaggio(s.DB.QueryRow(selPersonaggio+` WHERE p.id = ?`, id))
	if err != nil {
		return nil, errNonTrovato
	}
	dati, err := s.schedaCompleta(id, v.Versione)
	if err != nil {
		return nil, err
	}
	return map[string]any{"personaggio": v, "dati": json.RawMessage(dati), "modificabile": s.puoModificarePersonaggio(u, id),
		"effetti": s.effettiDelPG(id)}, nil // effetti in corso: la scheda mostra la CA cambiata con l'originale (effetti_valori.go)
}

// schedaCompleta unisce la versione indicata e lo stato in gioco attuale.
func (s *Server) schedaCompleta(id string, numero int) ([]byte, error) {
	var dati string
	if err := s.DB.QueryRow(`SELECT dati FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, id, numero).Scan(&dati); err != nil {
		return nil, err
	}
	p, err := leggiPiatto(dati)
	if err != nil {
		return nil, err
	}
	for k, v := range s.leggiInGioco(id) {
		p[k] = v
	}
	return ricomponi(p)
}

// ===== Salvataggio con unione delle modifiche =====

type conflitto struct {
	Campo   string          `json:"campo"`
	Tuo     json.RawMessage `json:"tuo"`
	Attuale json.RawMessage `json:"attuale"`
	Autore  string          `json:"autore"`
}

func (s *Server) apiSalvaScheda(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.puoModificarePersonaggio(u, id) {
		if s.puoLeggerePersonaggio(u, id) {
			return nil, errore(http.StatusForbidden, "Questo personaggio non è più attivo: la scheda si può solo consultare.")
		}
		return nil, errNonTrovato
	}
	var in struct {
		Base      int
		Modifiche map[string]json.RawMessage
		Rimossi   []string
		Forza     []string
		Riepilogo json.RawMessage
		Offline   bool // modifiche fatte senza rete sul dispositivo del giocatore e inviate ora
	}
	if err := leggiJSONMax(r, &in, maxScheda); err != nil {
		return nil, err
	}
	if len(in.Modifiche)+len(in.Rimossi) > maxCampiModifica {
		return nil, errDatiNonValidi
	}
	for k := range in.Modifiche {
		if !campoValido(k) {
			return nil, errDatiNonValidi
		}
		in.Modifiche[k] = compatta(in.Modifiche[k])
	}
	for _, k := range in.Rimossi {
		if !campoValido(k) {
			return nil, errDatiNonValidi
		}
	}
	forza := map[string]bool{}
	for _, k := range in.Forza {
		forza[k] = true
	}
	// Finanze: un giocatore non cambia le monete da solo; la modifica diventa una richiesta al Master.
	// Segni per il controllo all'accesso in sessione (controllo_schede.go).
	segnoOffline, segnoFuori := 0, 0
	if !u.Master() {
		if in.Offline {
			segnoOffline = 1
		}
		if s.fuoriSessione(u) {
			segnoFuori = 1
		}
	}
	var moneteProposte map[string]json.RawMessage
	if !u.Master() {
		moneteProposte, in.Rimossi = separaMonete(in.Modifiche, in.Rimossi)
	}
	// Modifica manuale (modifiche_manuali.go): con l'approvazione attiva il giocatore salva da solo solo i campi di
	// gioco; gli altri si ignorano (la pagina li riporta ai valori validi) e passano dalla richiesta al Master.
	filtra := !u.Master() && s.approvazioneSchede()
	var ignorati []string

	var risposta map[string]any
	var nuovaVers int
	var proprietario string
	var campiCambiati []string
	cambiatoGioco, riepilogoAggiornato := false, false
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		var corr int
		if err := tx.QueryRow(`SELECT versione_corrente, COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, id).Scan(&corr, &proprietario); err != nil {
			return errNonTrovato
		}
		var datiCorr, autoreCorr string
		var creatoCorr int64
		var notaCorr string
		var offCorr, fuoriCorr, controllata int
		if err := tx.QueryRow(`SELECT v.dati, COALESCE(v.autore_id,''), v.creato_il, v.nota, v.offline, v.fuori_sessione, p.versione_controllata
			FROM schede_versioni v JOIN personaggi p ON p.id = v.personaggio_id WHERE v.personaggio_id = ? AND v.numero = ?`, id, corr).
			Scan(&datiCorr, &autoreCorr, &creatoCorr, &notaCorr, &offCorr, &fuoriCorr, &controllata); err != nil {
			return err
		}
		attuale, err := leggiPiatto(datiCorr)
		if err != nil {
			return err
		}
		// Campi in gioco: valgono subito, senza versioni né conflitti (l'ultima modifica vince).
		gioco := piatto{}
		for k, v := range in.Modifiche {
			if campoInGioco(k) {
				gioco[k] = v
				delete(in.Modifiche, k)
			}
		}
		if len(gioco) > 0 {
			if err := applicaInGioco(tx, id, gioco, u.ID); err != nil {
				return err
			}
			cambiatoGioco = true
		}
		if filtra {
			ignorati, in.Rimossi = filtraNonDiGioco(in.Modifiche, in.Rimossi, attuale)
		}

		// Che cosa è cambiato da quando la pagina ha caricato la sua versione?
		var cambiatiDaAltri map[string]bool // nil = versione di partenza sconosciuta
		autoreAltri := ""
		if in.Base == corr {
			cambiatiDaAltri = map[string]bool{}
		} else if in.Base > 0 && in.Base < corr {
			var datiBase string
			if tx.QueryRow(`SELECT dati FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, id, in.Base).Scan(&datiBase) == nil {
				if base, err := leggiPiatto(datiBase); err == nil {
					cambiatiDaAltri = differenze(base, attuale)
				}
			}
			tx.QueryRow(`SELECT COALESCE(u.nome,'') FROM schede_versioni v LEFT JOIN utenti u ON u.id = v.autore_id WHERE v.personaggio_id = ? AND v.numero = ?`, id, corr).Scan(&autoreAltri)
		} else if in.Base > corr {
			return errore(http.StatusBadRequest, "Versione di partenza non valida: ricarica la scheda.")
		}

		var conflitti []conflitto
		controlla := func(k string, nuovo json.RawMessage) {
			vecchio, c := attuale[k]
			if uguale(vecchio, nuovo) || forza[k] {
				return
			}
			if cambiatiDaAltri == nil && c || cambiatiDaAltri[k] {
				conflitti = append(conflitti, conflitto{Campo: k, Tuo: nuovo, Attuale: vecchio, Autore: autoreAltri})
			}
		}
		for k, v := range in.Modifiche {
			controlla(k, v)
		}
		for _, k := range in.Rimossi {
			if !campoInGioco(k) {
				controlla(k, nil)
			}
		}
		if len(conflitti) > 0 {
			sort.Slice(conflitti, func(i, j int) bool { return conflitti[i].Campo < conflitti[j].Campo })
			risposta = map[string]any{"conflitti": conflitti, "versione": corr}
			return nil
		}

		// Applica le modifiche.
		nuovo := piatto{}
		for k, v := range attuale {
			nuovo[k] = v
		}
		for k, v := range in.Modifiche {
			nuovo[k] = v
		}
		for _, k := range in.Rimossi {
			if !campoInGioco(k) {
				delete(nuovo, k)
			}
		}
		cambiati := differenze(attuale, nuovo)
		if len(cambiati) == 0 {
			// Nessun campo cambiato, ma la scheda aperta può mandare il riepilogo aggiornato (per esempio un
			// personaggio appena creato o salvato prima che il riepilogo contenesse le caratteristiche):
			// si aggiorna quello della versione corrente, senza crearne una nuova.
			if len(in.Riepilogo) > 0 && string(in.Riepilogo) != "null" {
				riep := pulisciRiepilogo(in.Riepilogo)
				var prima string
				tx.QueryRow(`SELECT riepilogo FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, id, corr).Scan(&prima)
				if riep != prima {
					if _, err := tx.Exec(`UPDATE schede_versioni SET riepilogo = ? WHERE personaggio_id = ? AND numero = ?`, riep, id, corr); err != nil {
						return err
					}
					riepilogoAggiornato = true
				}
			}
			nuovaVers = corr
			risposta = map[string]any{"versione": corr}
			return nil
		}
		for k := range cambiati {
			campiCambiati = append(campiCambiati, k)
		}
		sort.Strings(campiCambiati)
		b, err := ricomponi(nuovo)
		if err != nil {
			return err
		}
		riep := pulisciRiepilogo(in.Riepilogo)
		nome := ""
		if v, ok := nuovo["f.nome"]; ok {
			json.Unmarshal(v, &nome)
		}
		nome = strings.TrimSpace(nome)
		if len([]rune(nome)) > 60 {
			nome = string([]rune(nome)[:60])
		}
		ora := db.Ora()
		nuovaVers = corr + 1
		campiJSON, _ := json.Marshal(campiCambiati)
		if _, err := tx.Exec(`INSERT INTO schede_versioni(personaggio_id, numero, dati, riepilogo, campi, autore_id, nota, creato_il, offline, fuori_sessione) VALUES(?,?,?,?,?,?,'',?,?,?)`,
			id, nuovaVers, string(b), riep, string(campiJSON), u.ID, ora, segnoOffline, segnoFuori); err != nil {
			return err
		}
		if nome != "" {
			_, err = tx.Exec(`UPDATE personaggi SET versione_corrente = ?, modificato_il = ?, nome = ? WHERE id = ?`, nuovaVers, ora, nome, id)
		} else {
			_, err = tx.Exec(`UPDATE personaggi SET versione_corrente = ?, modificato_il = ? WHERE id = ?`, nuovaVers, ora, id)
		}
		if err != nil {
			return err
		}
		// Le modifiche di seguito dello stesso autore si raggruppano: resta la prima di ogni
		// gruppo (punto di ritorno) e l'ultima. La versione intermedia si toglie.
		var autorePrec string
		var creatoPrec int64
		// (Mai la versione dell'ultimo controllo, né versioni con segni diversi: servono al controllo in sessione.)
		if corr > 1 && autoreCorr == u.ID && notaCorr == "" && ora-creatoCorr < finestraVersioni &&
			corr != controllata && offCorr == segnoOffline && fuoriCorr == segnoFuori &&
			tx.QueryRow(`SELECT COALESCE(autore_id,''), creato_il FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, id, corr-1).Scan(&autorePrec, &creatoPrec) == nil &&
			autorePrec == u.ID && creatoCorr-creatoPrec < finestraVersioni {
			var campiPrec string
			tx.QueryRow(`SELECT campi FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, id, corr).Scan(&campiPrec)
			tutti := unisciCampi(campiPrec, campiCambiati)
			if _, err := tx.Exec(`UPDATE schede_versioni SET campi = ? WHERE personaggio_id = ? AND numero = ?`, tutti, id, nuovaVers); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, id, corr); err != nil {
				return err
			}
		}
		risposta = map[string]any{"versione": nuovaVers, "campi": campiCambiati}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, ok := risposta["conflitti"]; ok {
		scriviJSON(w, http.StatusConflict, risposta)
		return nil, errGiaRisposto
	}
	if len(ignorati) > 0 {
		risposta["ignorati"] = ignorati
	}
	if nuovaVers > 0 && len(campiCambiati) > 0 {
		s.avvisaScheda(id, proprietario, nuovaVers, u, campiCambiati)
	}
	if cambiatoGioco {
		s.avvisaStatoInGioco(id, proprietario)
	}
	if segnoOffline == 1 && len(campiCambiati) > 0 && segnoFuori == 0 {
		go s.controlloAllAccesso(u) // modifiche offline arrivate durante una sessione in corso
	}
	if riepilogoAggiornato {
		s.notificaSanita(id, "riepilogo") // la Sanità mentale dipende dalla Saggezza del riepilogo
	}
	if len(moneteProposte) > 0 && s.richiestaMonete(id, u, moneteProposte) {
		risposta["monetePendenti"] = true
	}
	return risposta, nil
}

// campoValido: percorsi semplici, al massimo un punto, senza caratteri strani.
func campoValido(k string) bool {
	if k == "" || len(k) > 120 || strings.Count(k, ".") > 1 || strings.HasPrefix(k, ".") || strings.HasSuffix(k, ".") {
		return false
	}
	for _, c := range k {
		if c < 0x20 || c == '"' || c == '\\' {
			return false
		}
	}
	return true
}

// differenze: campi aggiunti, tolti o cambiati tra a e b.
func differenze(a, b piatto) map[string]bool {
	out := map[string]bool{}
	for k, v := range a {
		if w, ok := b[k]; !ok || !uguale(v, w) {
			out[k] = true
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			out[k] = true
		}
	}
	return out
}

func unisciCampi(js string, altri []string) string {
	var a []string
	json.Unmarshal([]byte(js), &a)
	m := map[string]bool{}
	for _, k := range append(a, altri...) {
		m[k] = true
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	b, _ := json.Marshal(out)
	return string(b)
}

// pulisciRiepilogo tiene solo i campi utili a elenchi e tracker (niente ritratto: è nella scheda).
func pulisciRiepilogo(raw json.RawMessage) string {
	var in struct {
		Class, Subclass, Race string
		Level, AC, HPMax, Dex int
		Initiative, Passive   int
		PassivePerception     int
		Speed                 any
		SpellDC, SpellAttack  *int
		Abilities             map[string]any
		Sanity                *float64
		SanityMethod          string
		SaveProfs             []any
		Tools, SkillProfs     []any
		HitDice               []dadoVita // riposi (2026-11-11)
		ConMod                *int
		PactSlots             *slotPatto
		Attacks, Defenses     json.RawMessage // attacchi automatici (attacchi.go)
		AttacksPerAction      any
		Saves                 map[string]any // difese dei personaggi (2026-11-12)
		CondImm, SaveAdv      []any
		MagicRes              []any
		Rage                  *modificheEffetto // ira del barbaro (effetti_valori.go)
		SpellsAuto            []incAuto         // incantesimi automatici (incantesimi_remoto.go)
		Slots                 map[string]int
	}
	json.Unmarshal(raw, &in)
	r := riepilogo{Classe: limita(in.Class, 40), Sottocl: limita(in.Subclass, 60), Livello: in.Level, Razza: limita(in.Race, 40),
		CA: in.AC, PFMax: in.HPMax, Iniziativa: in.Initiative, Des: in.Dex, PercPass: in.PassivePerception, CDInc: in.SpellDC, AttInc: in.SpellAttack,
		IncAuto: validaIncAuto(in.SpellsAuto), Slot: validaSlot(in.Slots)}
	for _, d := range in.HitDice {
		if (d.F == 6 || d.F == 8 || d.F == 10 || d.F == 12) && d.N >= 1 && d.N <= 20 && len(r.DadiVita) < 4 {
			r.DadiVita = append(r.DadiVita, d)
		}
	}
	if in.ConMod != nil && *in.ConMod >= -5 && *in.ConMod <= 10 {
		r.ModCos = in.ConMod
	}
	if p := in.PactSlots; p != nil && p.Lv >= 1 && p.Lv <= 5 && p.N >= 1 && p.N <= 4 {
		r.Patto = p
	}
	switch v := in.Speed.(type) {
	case float64, string:
		r.Velocita = v
	}
	valido := map[string]bool{"FOR": true, "DES": true, "COS": true, "INT": true, "SAG": true, "CAR": true}
	for k, v := range in.Abilities {
		if f, ok := v.(float64); ok && valido[k] && f >= 1 && f <= 30 {
			if r.Car == nil {
				r.Car = map[string]int{}
			}
			r.Car[k] = int(f)
		}
	}
	if in.Sanity != nil && *in.Sanity >= 1 && *in.Sanity <= 30 {
		n := int(*in.Sanity)
		r.San = &n
	}
	if m := in.SanityMethod; m == MetodoDMG || m == MetodoSaggezza || m == MetodoManuale {
		r.SanMetodo = m
	}
	for _, x := range in.SaveProfs {
		if k, _ := x.(string); valido[k] {
			r.TS = append(r.TS, k)
		}
	}
	for _, x := range in.Tools {
		if k, _ := x.(string); idProfessioneValido.MatchString(k) && len(r.Professioni) < 40 && !slices.Contains(r.Professioni, k) {
			r.Professioni = append(r.Professioni, k)
		}
	}
	for _, x := range in.SkillProfs {
		if k, _ := x.(string); k != "" && len([]rune(k)) <= 30 && len(r.Abilita) < 30 && !slices.Contains(r.Abilita, k) {
			r.Abilita = append(r.Abilita, k)
		}
	}
	r.Attacchi, r.AttacchiPerAzione, r.Difese = profiliDalRiepilogo(in.Attacks, in.AttacksPerAction, in.Defenses)
	for k, v := range in.Saves {
		if f, ok := v.(float64); ok && valido[k] && f >= -10 && f <= 25 {
			if r.TSTot == nil {
				r.TSTot = map[string]int{}
			}
			r.TSTot[k] = int(math.Round(f))
		}
	}
	condizioni := func(l []any) []string {
		var out []string
		for _, x := range l {
			if c, _ := x.(string); condizioniAmmesse[c] && !strings.HasPrefix(c, "Indebolimento") && c != "Concentrato" && !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
		return out
	}
	r.ImmCond, r.VantTS = condizioni(in.CondImm), condizioni(in.SaveAdv)
	if in.Rage != nil {
		if x := validaModifiche(*in.Rage); !x.vuota() {
			r.Ira = &x
		}
	}
	for _, x := range in.MagicRes {
		if k, _ := x.(string); valido[k] && !slices.Contains(r.ResMagia, k) {
			r.ResMagia = append(r.ResMagia, k)
		}
	}
	b, _ := json.Marshal(r)
	return string(b)
}

var idProfessioneValido = regexp.MustCompile(`^[a-z_]{1,30}$`)

func limita(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > n {
		return string([]rune(s)[:n])
	}
	return s
}

// ===== Storico =====

func (s *Server) apiVersioni(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, id) {
		return nil, errNonTrovato
	}
	rows, err := s.DB.Query(`SELECT v.numero, COALESCE(u.nome,''), v.autore_id = ?, v.creato_il, v.nota, v.campi
		FROM schede_versioni v LEFT JOIN utenti u ON u.id = v.autore_id WHERE v.personaggio_id = ? ORDER BY v.numero DESC LIMIT 200`, u.ID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type voce struct {
		Numero int      `json:"numero"`
		Autore string   `json:"autore"`
		Mia    bool     `json:"mia"`
		Quando int64    `json:"quando"`
		Nota   string   `json:"nota"`
		Campi  []string `json:"campi"`
	}
	out := []voce{}
	for rows.Next() {
		var v voce
		var campi string
		var mia sql.NullBool
		rows.Scan(&v.Numero, &v.Autore, &mia, &v.Quando, &v.Nota, &campi)
		v.Mia = mia.Bool
		json.Unmarshal([]byte(campi), &v.Campi)
		if v.Campi == nil {
			v.Campi = []string{}
		}
		out = append(out, v)
	}
	return out, nil
}

// apiVersione restituisce i dati di una versione precedente (per consultarla prima di ripristinarla).
func (s *Server) apiVersione(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, id) {
		return nil, errNonTrovato
	}
	var dati string
	if s.DB.QueryRow(`SELECT dati FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, id, r.PathValue("n")).Scan(&dati) != nil {
		return nil, errNonTrovato
	}
	return map[string]any{"dati": json.RawMessage(dati)}, nil
}

// apiRipristina crea una nuova versione uguale a una precedente (lo storico non si perde mai).
func (s *Server) apiRipristina(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.puoModificarePersonaggio(u, id) {
		return nil, errNonTrovato
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		return nil, errNonTrovato
	}
	if !u.Master() && s.approvazioneSchede() { // con l'approvazione attiva il ripristino diventa una richiesta al Master
		return s.richiestaDaRipristino(r, u, id, n)
	}
	var nuova int
	var prop string
	fuori := 0 // segno per il controllo all'accesso in sessione
	if s.fuoriSessione(u) {
		fuori = 1
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		var dati, riep string
		if err := tx.QueryRow(`SELECT dati, riepilogo FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, id, n).Scan(&dati, &riep); err != nil {
			return errNonTrovato
		}
		var corr int
		tx.QueryRow(`SELECT versione_corrente, COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, id).Scan(&corr, &prop)
		nuova = corr + 1
		ora := db.Ora()
		if _, err := tx.Exec(`INSERT INTO schede_versioni(personaggio_id, numero, dati, riepilogo, campi, autore_id, nota, creato_il, fuori_sessione) VALUES(?,?,?,?,'[]',?,?,?,?)`,
			id, nuova, dati, riep, u.ID, "ripristinata dalla versione "+strconv.Itoa(n), ora, fuori); err != nil {
			return err
		}
		nome := ""
		if p, err := leggiPiatto(dati); err == nil {
			json.Unmarshal(p["f.nome"], &nome)
		}
		if nome = limita(nome, 60); nome == "" {
			_, err := tx.Exec(`UPDATE personaggi SET versione_corrente = ?, modificato_il = ? WHERE id = ?`, nuova, ora, id)
			return err
		}
		_, err := tx.Exec(`UPDATE personaggi SET versione_corrente = ?, modificato_il = ?, nome = ? WHERE id = ?`, nuova, ora, nome, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.avvisaScheda(id, prop, nuova, u, []string{"*"})
	return map[string]int{"versione": nuova}, nil
}

// apiStatoPersonaggio: ritirare, segnare morto, riattivare. Archiviare è solo del Master.
func (s *Server) apiStatoPersonaggio(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, id) {
		return nil, errNonTrovato
	}
	var in struct{ Stato string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	var prop string
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, id).Scan(&prop)
	// Il giocatore può ritirare o segnare morto il proprio personaggio attivo; riattivarlo o archiviarlo
	// è una decisione del Master (regola presa dal lavoro della sessione sul PC).
	switch in.Stato {
	case "ritirato", "morto":
		if !s.puoModificarePersonaggio(u, id) {
			return nil, errVietato
		}
	case "attivo", "archiviato":
		if !u.Master() {
			return nil, errore(http.StatusForbidden, "Solo il Master può riattivare o archiviare un personaggio.")
		}
	default:
		return nil, errDatiNonValidi
	}
	if _, err := s.DB.Exec(`UPDATE personaggi SET stato = ?, modificato_il = ? WHERE id = ?`, in.Stato, db.Ora(), id); err != nil {
		return nil, err
	}
	s.avvisaScheda(id, prop, 0, u, []string{"stato"})
	return nil, nil
}

// ritratto restituisce l'immagine del personaggio (salvata nella scheda come data URL).
func (s *Server) ritratto(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	id := r.PathValue("id")
	// Il ritratto è "pubblico" al tavolo: lo vedono anche i compagni di gruppo (segnalini, tracker).
	var gruppo string
	s.DB.QueryRow(`SELECT COALESCE(gruppo_id,'') FROM personaggi WHERE id = ?`, id).Scan(&gruppo)
	if !u.Approvato() || !(s.puoLeggerePersonaggio(u, id) || s.membro(u, gruppo)) {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	var dati string
	if s.DB.QueryRow(`SELECT v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, id).Scan(&dati) != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	var sch struct{ Img string }
	json.Unmarshal([]byte(dati), &sch)
	tipo, b64, ok := strings.Cut(strings.TrimPrefix(sch.Img, "data:"), ";base64,")
	if !ok || !(tipo == "image/jpeg" || tipo == "image/png" || tipo == "image/webp") {
		http.Error(w, "Nessun ritratto.", http.StatusNotFound)
		return
	}
	img, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || http.DetectContentType(img) != tipo && !(tipo == "image/webp" && http.DetectContentType(img) == "image/webp") {
		http.Error(w, "Ritratto non valido.", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.Write(img)
}

// ===== Stato in gioco =====

// estraiInGioco toglie dalla scheda i campi in gioco e li restituisce a parte.
func estraiInGioco(p piatto) piatto {
	out := piatto{}
	for k, v := range p {
		if campoInGioco(k) {
			out[k] = v
			delete(p, k)
		}
	}
	return out
}

// applicaInGioco scrive in stato_in_gioco i campi della scheda che vivono lì.
func applicaInGioco(tx *sql.Tx, id string, g piatto, autore string) error {
	if len(g) == 0 {
		return nil
	}
	set := func(col string, v any) error {
		_, err := tx.Exec(`UPDATE stato_in_gioco SET `+col+` = ?, modificato_il = ?, modificato_da = ? WHERE personaggio_id = ?`, v, db.Ora(), autore, id)
		return err
	}
	if v, ok := g["hpCur"]; ok {
		if err := set("pf", numeroONull(v)); err != nil {
			return err
		}
	}
	if v, ok := g["hpTmp"]; ok {
		n, _ := numeroONull(v).(int64)
		if err := set("pf_temp", n); err != nil {
			return err
		}
	}
	if v, ok := g["insp"]; ok {
		var b bool
		json.Unmarshal(v, &b)
		i := 0
		if b {
			i = 1
		}
		if err := set("ispirazione", i); err != nil {
			return err
		}
	}
	if v, ok := g["exh"]; ok {
		var n float64
		json.Unmarshal(v, &n)
		var cond string
		tx.QueryRow(`SELECT condizioni FROM stato_in_gioco WHERE personaggio_id = ?`, id).Scan(&cond)
		if err := set("condizioni", conLivelloIndebolimento(cond, int(n))); err != nil {
			return err
		}
	}
	// Oggetti appiattiti (ds.s, slotU.1...): si ricompongono e si salvano interi.
	for _, k := range []struct{ campo, col string }{{"ds", "ts_morte"}, {"slotU", "slot_usati"}} {
		var corrente string
		tx.QueryRow(`SELECT `+k.col+` FROM stato_in_gioco WHERE personaggio_id = ?`, id).Scan(&corrente)
		m := map[string]json.RawMessage{}
		json.Unmarshal([]byte(corrente), &m)
		cambiato := false
		for p, v := range g {
			if p == k.campo {
				m = map[string]json.RawMessage{}
				json.Unmarshal(v, &m)
				cambiato = true
			} else if strings.HasPrefix(p, k.campo+".") {
				m[strings.TrimPrefix(p, k.campo+".")] = v
				cambiato = true
			}
		}
		if cambiato {
			b, _ := json.Marshal(m)
			if err := set(k.col, string(b)); err != nil {
				return err
			}
		}
	}
	return nil
}

// numeroONull: la scheda salva i PF come testo ("" = non impostati).
func numeroONull(v json.RawMessage) any {
	var x any
	json.Unmarshal(v, &x)
	switch t := x.(type) {
	case float64:
		return int64(t)
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
			return n
		}
	}
	return nil
}

// leggiInGioco restituisce lo stato in gioco nel formato della scheda.
func (s *Server) leggiInGioco(id string) piatto {
	var pf sql.NullInt64
	var temp, isp int
	var ts, slot, cond string
	if s.DB.QueryRow(`SELECT pf, pf_temp, ts_morte, slot_usati, ispirazione, condizioni FROM stato_in_gioco WHERE personaggio_id = ?`, id).Scan(&pf, &temp, &ts, &slot, &isp, &cond) != nil {
		return piatto{}
	}
	out := piatto{}
	out["exh"], _ = json.Marshal(livelloIndebolimento(cond))
	if pf.Valid {
		out["hpCur"], _ = json.Marshal(strconv.FormatInt(pf.Int64, 10))
	} else {
		out["hpCur"] = json.RawMessage(`""`)
	}
	if temp != 0 {
		out["hpTmp"], _ = json.Marshal(strconv.Itoa(temp))
	} else {
		out["hpTmp"] = json.RawMessage(`""`)
	}
	out["insp"], _ = json.Marshal(isp == 1)
	for campo, js := range map[string]string{"ds": ts, "slotU": slot} {
		m := map[string]json.RawMessage{}
		json.Unmarshal([]byte(js), &m)
		if len(m) == 0 {
			out[campo] = json.RawMessage(`{}`)
		}
		for k, v := range m {
			out[campo+"."+k] = compatta(v)
		}
	}
	return out
}

type vistaInGioco struct {
	Personaggio string          `json:"personaggio"`
	PF          *int64          `json:"pf"`
	PFTemp      int             `json:"pfTemp"`
	TSMorte     json.RawMessage `json:"tsMorte"`
	Condizioni  json.RawMessage `json:"condizioni"`
	SlotUsati   json.RawMessage `json:"slotUsati"`
	Ispirazione bool            `json:"ispirazione"`
	Effetti     *effettiPG      `json:"effetti,omitempty"` // effetti in corso nello scontro attivo (effetti_valori.go)
}

func (s *Server) vistaInGioco(id string) *vistaInGioco {
	v := &vistaInGioco{Personaggio: id}
	var pf sql.NullInt64
	var ts, cond, slot string
	var isp int
	if s.DB.QueryRow(`SELECT pf, pf_temp, ts_morte, condizioni, slot_usati, ispirazione FROM stato_in_gioco WHERE personaggio_id = ?`, id).
		Scan(&pf, &v.PFTemp, &ts, &cond, &slot, &isp) != nil {
		return nil
	}
	if pf.Valid {
		v.PF = &pf.Int64
	}
	v.TSMorte, v.Condizioni, v.SlotUsati, v.Ispirazione = json.RawMessage(ts), json.RawMessage(cond), json.RawMessage(slot), isp == 1
	v.Effetti = s.effettiDelPG(id)
	return v
}

// apiStatoInGioco: PF, temporanei, condizioni... dal tracker dell'iniziativa o dalla scheda.
func (s *Server) apiStatoInGioco(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !s.puoModificarePersonaggio(u, id) {
		return nil, errNonTrovato
	}
	var in struct {
		PF          *int64              `json:"pf"`
		PFNullo     bool                `json:"pfNullo"`
		PFTemp      *int                `json:"pfTemp"`
		TSMorte     *struct{ S, F int } `json:"tsMorte"`
		Condizioni  []string            `json:"condizioni"`
		SlotUsati   json.RawMessage     `json:"slotUsati"`
		Ispirazione *bool               `json:"ispirazione"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		set := func(col string, v any) error {
			_, err := tx.Exec(`UPDATE stato_in_gioco SET `+col+` = ?, modificato_il = ?, modificato_da = ? WHERE personaggio_id = ?`, v, db.Ora(), u.ID, id)
			return err
		}
		if in.PFNullo {
			if err := set("pf", nil); err != nil {
				return err
			}
		} else if in.PF != nil {
			if *in.PF < -999 || *in.PF > 9999 {
				return errDatiNonValidi
			}
			if err := set("pf", *in.PF); err != nil {
				return err
			}
		}
		if in.PFTemp != nil {
			if *in.PFTemp < 0 || *in.PFTemp > 999 {
				return errDatiNonValidi
			}
			if err := set("pf_temp", *in.PFTemp); err != nil {
				return err
			}
		}
		if in.TSMorte != nil {
			if in.TSMorte.S < 0 || in.TSMorte.S > 3 || in.TSMorte.F < 0 || in.TSMorte.F > 3 {
				return errDatiNonValidi
			}
			b, _ := json.Marshal(map[string]int{"s": in.TSMorte.S, "f": in.TSMorte.F})
			if err := set("ts_morte", string(b)); err != nil {
				return err
			}
		}
		if in.SlotUsati != nil {
			var m map[string]int
			if json.Unmarshal(in.SlotUsati, &m) != nil || len(m) > 12 {
				return errDatiNonValidi
			}
			for k, v := range m {
				if len(k) > 4 || v < 0 || v > 9 {
					return errDatiNonValidi
				}
			}
			b, _ := json.Marshal(m)
			if err := set("slot_usati", string(b)); err != nil {
				return err
			}
		}
		if in.Condizioni != nil {
			pulite, err := condizioniValide(in.Condizioni)
			if err != nil {
				return err
			}
			b, _ := json.Marshal(pulite)
			if err := set("condizioni", string(b)); err != nil {
				return err
			}
		}
		if in.Ispirazione != nil {
			i := 0
			if *in.Ispirazione {
				i = 1
			}
			if err := set("ispirazione", i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var prop string
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, id).Scan(&prop)
	s.avvisaStatoInGioco(id, prop)
	return s.vistaInGioco(id), nil
}

// Condizioni ammesse (regole 2014) più "Concentrato" e i livelli di indebolimento.
var condizioniAmmesse = map[string]bool{"Accecato": true, "Affascinato": true, "Assordato": true, "Spaventato": true, "Afferrato": true,
	"Incapacitato": true, "Invisibile": true, "Paralizzato": true, "Pietrificato": true, "Avvelenato": true, "Prono": true,
	"Trattenuto": true, "Stordito": true, "Privo di sensi": true, "Concentrato": true,
	"Indebolimento 1": true, "Indebolimento 2": true, "Indebolimento 3": true, "Indebolimento 4": true, "Indebolimento 5": true, "Indebolimento 6": true}

func condizioniValide(c []string) ([]string, error) {
	visti := map[string]bool{}
	out := []string{}
	for _, x := range c {
		if !condizioniAmmesse[x] {
			return nil, errore(http.StatusBadRequest, "Condizione sconosciuta.")
		}
		if !visti[x] {
			visti[x] = true
			out = append(out, x)
		}
	}
	return out, nil
}

// ===== Avvisi in tempo reale =====

func (s *Server) avvisaScheda(id, proprietario string, versione int, autore *Utente, campi []string) {
	s.avvisaSchedaDa(id, proprietario, versione, autore, campi, false)
}

// avvisaSchedaDa: server = true quando la modifica l'ha fatta il server (monete, acquisti, professioni): la pagina
// aperta la applica sempre, anche se l'autore è lo stesso utente.
func (s *Server) avvisaSchedaDa(id, proprietario string, versione int, autore *Utente, campi []string, server bool) {
	m := Msg{Tipo: "scheda.aggiornata", Dati: map[string]any{"personaggio": id, "versione": versione,
		"autore": autore.Nome, "autoreId": autore.ID, "campi": campi, "quando": time.Now().Unix(), "server": server}}
	if proprietario != "" {
		s.Hub.AUtente(proprietario, m)
	}
	s.Hub.AlMaster(m)
}

func (s *Server) avvisaStatoInGioco(id, proprietario string) {
	v := s.vistaInGioco(id)
	if v == nil {
		return
	}
	m := Msg{Tipo: "pg.stato", Dati: v}
	if proprietario != "" {
		s.Hub.AUtente(proprietario, m)
	}
	s.Hub.AlMaster(m)
	s.controllaIndebolimento(id, proprietario, string(v.Condizioni))
	if s.DopoInGioco != nil {
		s.DopoInGioco(id) // aggiorna il tracker della sessione in corso
	}
}
