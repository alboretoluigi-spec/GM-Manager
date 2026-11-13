package server

// Finanze (richiesta dell'utente, 2026-11-07).
// - Le monete stanno nella scheda (coins.MR … coins.MP). Se un giocatore le cambia a mano, la modifica non si applica:
//   diventa una richiesta che il Master autorizza o rifiuta (apiSalvaScheda → richiestaMonete).
// - Ciò che ha un costo (acquisti dall'elenco, professioni, trasferimenti) scala le monete in automatico; se non
//   bastano, l'errore ha il codice "monete_insufficienti" e la pagina mostra l'immagine «povero».
// - Trasferimenti tra personaggi; ricompense automatiche quando un mostro del tracker scende a 0 PF:
//   GS × 10 mo divisi tra i personaggi presenti nello scontro.
// - Ogni movimento va nel registro (registro_finanze), visibile al Master.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

var ordineMonete = []string{"MR", "MA", "ME", "MO", "MP"}

func moneteDa(p piatto) map[string]int {
	m := map[string]int{}
	for _, k := range ordineMonete {
		m[k] = leggiMoneta(p, k)
	}
	return m
}

func rameDi(m map[string]int) int {
	t := 0
	for k, v := range m {
		t += v * valoreMoneta[k]
	}
	return t
}

func testoMonete(m map[string]int) string {
	parti := []string{}
	for i := len(ordineMonete) - 1; i >= 0; i-- {
		if k := ordineMonete[i]; m[k] > 0 {
			parti = append(parti, strconv.Itoa(m[k])+" "+strings.ToLower(k))
		}
	}
	if len(parti) == 0 {
		return "nessuna moneta"
	}
	return strings.Join(parti, ", ")
}

func (s *Server) registraFinanze(q interface {
	Exec(string, ...any) (sql.Result, error)
}, pg string, u *Utente, tipo string, importo int, testo string) {
	attore := ""
	if u != nil {
		attore = u.ID
	}
	q.Exec(`INSERT INTO registro_finanze(personaggio_id, attore_id, tipo, importo, testo, creato_il) VALUES(?,?,?,?,?,?)`, pg, attore, tipo, importo, testo, db.Ora())
}

// avvisaFinanze: il proprietario del personaggio e il Master ricevono il movimento; chi ha la scheda aperta la ricarica.
func (s *Server) avvisaFinanze(pg string, u *Utente, testo string) {
	var prop, nome string
	var ver int
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,''), nome, versione_corrente FROM personaggi WHERE id = ?`, pg).Scan(&prop, &nome, &ver)
	m := Msg{Tipo: "finanze.movimento", Dati: map[string]any{"personaggio": pg, "nome": nome, "testo": testo}}
	s.Hub.AlMaster(m)
	if prop != "" {
		s.Hub.AUtente(prop, m)
	}
	s.avvisaSchedaDa(pg, prop, ver, u, []string{"coins", "eq"}, true)
}

// ===== Richieste di modifica manuale (giocatori) =====

// separaMonete toglie dalle modifiche di un giocatore i campi delle monete e restituisce i valori proposti.
func separaMonete(mod map[string]json.RawMessage, rimossi []string) (map[string]json.RawMessage, []string) {
	prop := map[string]json.RawMessage{}
	for k, v := range mod {
		if strings.HasPrefix(k, "coins.") || k == "coins" {
			prop[k] = v
			delete(mod, k)
		}
	}
	resto := rimossi[:0]
	for _, k := range rimossi {
		if !(strings.HasPrefix(k, "coins.") || k == "coins") {
			resto = append(resto, k)
		}
	}
	return prop, resto
}

// richiestaMonete registra la modifica proposta (sostituisce quella ancora in attesa) e avvisa il Master.
func (s *Server) richiestaMonete(pg string, u *Utente, proposte map[string]json.RawMessage) bool {
	var dati string
	s.DB.QueryRow(`SELECT v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, pg).Scan(&dati)
	p, err := leggiPiatto(dati)
	if err != nil {
		return false
	}
	prima := moneteDa(p)
	dopoP := piatto{}
	for k, v := range p {
		dopoP[k] = v
	}
	for k, v := range proposte {
		if k == "coins" { // oggetto intero
			var o map[string]json.RawMessage
			json.Unmarshal(v, &o)
			for kk, vv := range o {
				dopoP["coins."+kk] = vv
			}
			continue
		}
		dopoP[k] = v
	}
	dopo := moneteDa(dopoP)
	uguali := true
	for _, k := range ordineMonete {
		uguali = uguali && prima[k] == dopo[k]
	}
	if uguali {
		return false
	}
	bp, _ := json.Marshal(prima)
	bd, _ := json.Marshal(dopo)
	id := cripto.ID()
	s.DB.Exec(`UPDATE richieste_monete SET stato = 'superata', deciso_il = ? WHERE personaggio_id = ? AND stato = 'in_attesa'`, db.Ora(), pg)
	s.DB.Exec(`INSERT INTO richieste_monete(id, personaggio_id, utente_id, prima, dopo, creato_il) VALUES(?,?,?,?,?,?)`, id, pg, u.ID, string(bp), string(bd), db.Ora())
	var nome string
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&nome)
	s.Hub.AlMaster(Msg{Tipo: "finanze.richiesta", Dati: map[string]any{"id": id, "personaggio": pg, "nome": nome, "da": u.Nome,
		"prima": testoMonete(prima), "dopo": testoMonete(dopo), "differenza": testoMo(rameDi(dopo) - rameDi(prima))}})
	return true
}

func (s *Server) rotteFinanze(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/finanze", api(s.apiFinanze))
	mux.Handle("POST /api/v1/finanze-richieste/{id}", api(s.apiDecidiRichiesta))
	mux.Handle("GET /api/v1/finanze/{pg}/destinatari", api(s.apiDestinatari))
	mux.Handle("POST /api/v1/finanze/{pg}/trasferisci", api(s.apiTrasferisci))
	mux.Handle("POST /api/v1/finanze/{pg}/acquista", api(s.apiAcquista))
	mux.Handle("POST /api/v1/finanze/{pg}/master", api(s.apiMonetaMaster))
	mux.Handle("PUT /api/v1/finanze/impostazioni", api(s.apiImpostazioniFinanze))
}

func (s *Server) ricompenseAttive() bool {
	v, _ := s.impostazione("ricompense_auto")
	return v != "no"
}

// apiFinanze: il Master vede richieste, monete di tutti e registro; il giocatore le monete e il registro dei suoi.
func (s *Server) apiFinanze(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	q := `SELECT p.id, p.nome, COALESCE(u.nome,''), COALESCE(v.dati,'{}') FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id
		LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.stato = 'attivo'`
	args := []any{}
	if !u.Master() {
		q += ` AND p.proprietario_id = ?`
		args = append(args, u.ID)
	}
	rows, err := s.DB.Query(q+` ORDER BY u.nome, p.nome`, args...)
	if err != nil {
		return nil, err
	}
	pgs := []map[string]any{}
	ids := []string{}
	for rows.Next() {
		var id, nome, gioc, dati string
		rows.Scan(&id, &nome, &gioc, &dati)
		p, _ := leggiPiatto(dati)
		m := moneteDa(p)
		pgs = append(pgs, map[string]any{"id": id, "nome": nome, "giocatore": gioc, "monete": m, "rame": rameDi(m)})
		ids = append(ids, id)
	}
	rows.Close()
	out := map[string]any{"personaggi": pgs, "master": u.Master(), "ricompenseAuto": s.ricompenseAttive()}
	reg := []map[string]any{}
	rq := `SELECT r.personaggio_id, p.nome, r.tipo, r.importo, r.testo, r.creato_il, COALESCE(u.nome,'') FROM registro_finanze r
		JOIN personaggi p ON p.id = r.personaggio_id LEFT JOIN utenti u ON u.id = r.attore_id`
	rargs := []any{}
	if !u.Master() {
		rq += ` WHERE p.proprietario_id = ?`
		rargs = append(rargs, u.ID)
	}
	if rows, err = s.DB.Query(rq+` ORDER BY r.id DESC LIMIT 80`, rargs...); err == nil {
		for rows.Next() {
			var pg, nome, tipo, testo, chi string
			var imp int
			var q int64
			rows.Scan(&pg, &nome, &tipo, &imp, &testo, &q, &chi)
			reg = append(reg, map[string]any{"personaggio": pg, "nome": nome, "tipo": tipo, "importo": imp, "testo": testo, "quando": q, "chi": chi})
		}
		rows.Close()
	}
	out["registro"] = reg
	ric := []map[string]any{}
	if rows, err = s.DB.Query(`SELECT r.id, r.personaggio_id, p.nome, COALESCE(u.nome,''), r.prima, r.dopo, r.creato_il FROM richieste_monete r
		JOIN personaggi p ON p.id = r.personaggio_id LEFT JOIN utenti u ON u.id = r.utente_id WHERE r.stato = 'in_attesa' ORDER BY r.creato_il`); err == nil {
		for rows.Next() {
			var id, pg, nome, chi, pr, dp string
			var q int64
			rows.Scan(&id, &pg, &nome, &chi, &pr, &dp, &q)
			var a, b map[string]int
			json.Unmarshal([]byte(pr), &a)
			json.Unmarshal([]byte(dp), &b)
			if !u.Master() && !contiene(ids, pg) {
				continue
			}
			ric = append(ric, map[string]any{"id": id, "personaggio": pg, "nome": nome, "da": chi, "prima": testoMonete(a), "dopo": testoMonete(b),
				"differenza": testoMo(rameDi(b) - rameDi(a)), "quando": q})
		}
		rows.Close()
	}
	out["richieste"] = ric
	return out, nil
}

func contiene(l []string, x string) bool {
	for _, y := range l {
		if y == x {
			return true
		}
	}
	return false
}

// apiDecidiRichiesta: il Master autorizza (le monete diventano quelle proposte) o rifiuta.
func (s *Server) apiDecidiRichiesta(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Autorizza bool `json:"autorizza"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	var pg, utente, prima, dopo string
	if s.DB.QueryRow(`SELECT personaggio_id, COALESCE(utente_id,''), prima, dopo FROM richieste_monete WHERE id = ? AND stato = 'in_attesa'`, r.PathValue("id")).
		Scan(&pg, &utente, &prima, &dopo) != nil {
		return nil, errore(http.StatusConflict, "Richiesta già decisa o non trovata.")
	}
	var a, b map[string]int
	json.Unmarshal([]byte(prima), &a)
	json.Unmarshal([]byte(dopo), &b)
	testo := "Modifica delle monete rifiutata dal Master (" + testoMonete(a) + " → " + testoMonete(b) + ")."
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		stato := "rifiutata"
		if in.Autorizza {
			stato = "autorizzata"
			testo = "Modifica delle monete autorizzata dal Master: " + testoMonete(a) + " → " + testoMonete(b) + "."
			if _, _, err := s.modificaScheda(tx, pg, u, func(p piatto) (bool, error) {
				for k, v := range b {
					scriviMoneta(p, k, v)
				}
				return true, nil
			}); err != nil {
				return err
			}
			s.registraFinanze(tx, pg, u, "modifica", rameDi(b)-rameDi(a), testo)
		}
		_, err := tx.Exec(`UPDATE richieste_monete SET stato = ?, deciso_il = ? WHERE id = ?`, stato, db.Ora(), r.PathValue("id"))
		return err
	})
	if err != nil {
		return nil, err
	}
	if utente != "" {
		s.Hub.AUtente(utente, Msg{Tipo: "finanze.decisione", Dati: map[string]any{"personaggio": pg, "autorizzata": in.Autorizza, "testo": testo}})
	}
	s.avvisaFinanze(pg, u, testo)
	return map[string]any{"testo": testo}, nil
}

// puoUsareMonete: il proprietario (personaggio attivo) o il Master.
func (s *Server) puoUsareMonete(u *Utente, pg string) bool {
	return u.Master() && s.puoLeggerePersonaggio(u, pg) || s.puoModificarePersonaggio(u, pg)
}

func (s *Server) apiDestinatari(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("pg")
	if !s.puoUsareMonete(u, pg) {
		return nil, errNonTrovato
	}
	rows, err := s.DB.Query(`SELECT p.id, p.nome, COALESCE(u.nome,'') FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id
		WHERE p.stato = 'attivo' AND p.id <> ? ORDER BY p.nome`, pg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, nome, gioc string
		rows.Scan(&id, &nome, &gioc)
		out = append(out, map[string]string{"id": id, "nome": nome, "giocatore": gioc})
	}
	return out, nil
}

// apiTrasferisci: una parte o tutte le monete a un altro personaggio (in monete di rame; si pagano prima le piccole).
func (s *Server) apiTrasferisci(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("pg")
	if !s.puoUsareMonete(u, pg) {
		return nil, errNonTrovato
	}
	var in struct {
		A     string `json:"a"`
		Rame  int    `json:"rame"`
		Tutto bool   `json:"tutto"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	var stato, nomeA, nomeDa string
	if s.DB.QueryRow(`SELECT stato, nome FROM personaggi WHERE id = ?`, in.A).Scan(&stato, &nomeA) != nil || stato != "attivo" || in.A == pg {
		return nil, errore(http.StatusBadRequest, "Scegli un altro personaggio attivo.")
	}
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&nomeDa)
	if !in.Tutto && (in.Rame < 1 || in.Rame > 100000000) {
		return nil, errore(http.StatusBadRequest, "Importo non valido.")
	}
	var testo string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		totale := moneteScheda(tx, pg)
		if in.Tutto {
			in.Rame = totale
		}
		if in.Rame < 1 {
			return erroreMonete(1)
		}
		if totale < in.Rame {
			return erroreMonete(in.Rame)
		}
		if _, _, err := s.modificaScheda(tx, pg, u, func(p piatto) (bool, error) {
			if in.Tutto {
				for _, k := range ordineMonete {
					scriviMoneta(p, k, 0)
				}
				return true, nil
			}
			return paga(p, in.Rame), nil
		}); err != nil {
			return err
		}
		// chi riceve ottiene l'importo in mo, ma e mr
		if _, _, err := s.modificaScheda(tx, in.A, u, func(p piatto) (bool, error) { rimborsa(p, in.Rame); return true, nil }); err != nil {
			return err
		}
		testo = fmt.Sprintf("%s trasferisce %s a %s.", nomeDa, testoMo(in.Rame), nomeA)
		s.registraFinanze(tx, pg, u, "trasferimento", -in.Rame, testo)
		s.registraFinanze(tx, in.A, u, "trasferimento", in.Rame, testo)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.avvisaFinanze(pg, u, testo)
	s.avvisaFinanze(in.A, u, testo)
	return map[string]any{"testo": testo}, nil
}

var prezzoRe = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(mp|mo|me|ma|mr)?`)

// rameDaPrezzo legge un prezzo come "5 mo", "2 ma", "1 ma – 1 mo" (si usa il primo valore) o "2 – 35 mo".
func rameDaPrezzo(t string) (int, bool) {
	m := prezzoRe.FindAllStringSubmatch(t, -1)
	if len(m) == 0 {
		return 0, false
	}
	n, err := strconv.ParseFloat(strings.Replace(m[0][1], ",", ".", 1), 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	unita := strings.ToUpper(m[0][2])
	if unita == "" { // "2 – 35 mo": l'unità è quella dopo
		for _, x := range m[1:] {
			if x[2] != "" {
				unita = strings.ToUpper(x[2])
				break
			}
		}
	}
	v, ok := valoreMoneta[unita]
	if !ok {
		return 0, false
	}
	return int(math.Round(n * float64(v))), true
}

var oggettoValido = regexp.MustCompile(`^[^\n\r<>]{1,100}$`)

// aggiungiQuantita: come nella scheda, la riga «N × oggetto» cresce se c'è già.
func aggiungiQuantita(p piatto, nome string, q int) {
	var eq string
	json.Unmarshal(p["eq"], &eq)
	righe := strings.Split(strings.TrimRight(eq, " \n"), "\n")
	re := regexp.MustCompile(`(?i)^\s*(\d+)\s*[×x]\s*` + regexp.QuoteMeta(nome) + `\s*$`)
	fatto := false
	for i, r := range righe {
		if m := re.FindStringSubmatch(r); m != nil {
			n, _ := strconv.Atoi(m[1])
			righe[i] = strconv.Itoa(n+q) + " × " + nome
			fatto = true
			break
		}
	}
	if !fatto {
		if len(righe) == 1 && righe[0] == "" {
			righe = righe[:0]
		}
		righe = append(righe, strconv.Itoa(q)+" × "+nome)
	}
	b, _ := json.Marshal(strings.Join(righe, "\n"))
	p["eq"] = b
}

// apiAcquista: oggetto scelto dall'elenco dell'equipaggiamento. Il giocatore paga; il Master lo assegna gratis.
func (s *Server) apiAcquista(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("pg")
	if !s.puoUsareMonete(u, pg) {
		return nil, errNonTrovato
	}
	var in struct {
		Nome     string `json:"nome"`
		Prezzo   string `json:"prezzo"`
		Quantita int    `json:"quantita"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	in.Nome = strings.TrimSpace(in.Nome)
	if !oggettoValido.MatchString(in.Nome) || in.Quantita < 1 || in.Quantita > 999 {
		return nil, errDatiNonValidi
	}
	unit, ok := rameDaPrezzo(in.Prezzo)
	if !ok && !u.Master() {
		return nil, errore(http.StatusBadRequest, "Prezzo non valido.")
	}
	costo := unit * in.Quantita
	if u.Master() {
		costo = 0 // assegnato dal Master: non si paga
	}
	var testo string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if costo > 0 && moneteScheda(tx, pg) < costo {
			return erroreMonete(costo)
		}
		if _, _, err := s.modificaScheda(tx, pg, u, func(p piatto) (bool, error) {
			if costo > 0 && !paga(p, costo) {
				return false, erroreMonete(costo)
			}
			aggiungiQuantita(p, in.Nome, in.Quantita)
			return true, nil
		}); err != nil {
			return err
		}
		if costo > 0 {
			testo = fmt.Sprintf("Acquistato %d × %s per %s.", in.Quantita, in.Nome, testoMo(costo))
			s.registraFinanze(tx, pg, u, "acquisto", -costo, testo)
		} else {
			testo = fmt.Sprintf("Il Master assegna %d × %s (gratis).", in.Quantita, in.Nome)
			s.registraFinanze(tx, pg, u, "master", 0, testo)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.avvisaFinanze(pg, u, testo)
	return map[string]any{"testo": testo, "costo": costo}, nil
}

// apiMonetaMaster: il Master dà o toglie monete direttamente (senza richiesta).
func (s *Server) apiMonetaMaster(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("pg")
	var in struct {
		Rame   int    `json:"rame"` // positivo = dà, negativo = toglie
		Motivo string `json:"motivo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	motivo, err := rigaSingola(in.Motivo, "Motivo", 0, 120)
	if err != nil {
		return nil, err
	}
	if in.Rame == 0 || in.Rame > 100000000 || in.Rame < -100000000 || !s.puoLeggerePersonaggio(u, pg) {
		return nil, errDatiNonValidi
	}
	var nome, testo string
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&nome)
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if in.Rame < 0 && moneteScheda(tx, pg) < -in.Rame {
			return erroreMonete(-in.Rame)
		}
		if _, _, err := s.modificaScheda(tx, pg, u, func(p piatto) (bool, error) {
			if in.Rame < 0 {
				return paga(p, -in.Rame), nil
			}
			rimborsa(p, in.Rame)
			return true, nil
		}); err != nil {
			return err
		}
		verbo := "dà"
		if in.Rame < 0 {
			verbo = "toglie"
		}
		testo = fmt.Sprintf("Il Master %s %s a %s.", verbo, testoMo(abs(in.Rame)), nome)
		if motivo != "" {
			testo += " Motivo: " + motivo + "."
		}
		s.registraFinanze(tx, pg, u, "master", in.Rame, testo)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.avvisaFinanze(pg, u, testo)
	return map[string]any{"testo": testo}, nil
}

func (s *Server) apiImpostazioniFinanze(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		RicompenseAuto bool `json:"ricompenseAuto"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	v := "si"
	if !in.RicompenseAuto {
		v = "no"
	}
	return nil, s.impostaImpostazione("ricompense_auto", v)
}

// ===== Ricompense automatiche =====

// ricompensaMostro: il mostro è appena sceso a 0 PF. Il gruppo riceve GS × 10 mo, divisi tra i personaggi presenti
// nello scontro (ciascuno GS × 10 / numero di giocatori presenti, arrotondato per difetto al rame). Una volta sola.
func (s *Server) ricompensaMostro(comb, combattente string, u *Utente) string {
	if !s.ricompenseAttive() {
		return ""
	}
	var gs float64
	var gia int
	var nome string
	if s.DB.QueryRow(`SELECT gs, ricompensato, nome FROM combattenti WHERE id = ? AND tipo = 'mostro'`, combattente).Scan(&gs, &gia, &nome) != nil || gs <= 0 || gia == 1 {
		return ""
	}
	rows, err := s.DB.Query(`SELECT personaggio_id FROM combattenti WHERE combattimento_id = ? AND tipo = 'pg' AND personaggio_id IS NOT NULL`, comb)
	if err != nil {
		return ""
	}
	var pgs []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		pgs = append(pgs, id)
	}
	rows.Close()
	if len(pgs) == 0 {
		return ""
	}
	totale := int(math.Round(gs * 10 * 100)) // GS × 10 mo, in rame
	quota := totale / len(pgs)
	testo := fmt.Sprintf("Ricompensa per %s (GS %s): %s divise tra %d personaggi, %s a testa.", nome, testoGS(gs), testoMo(totale), len(pgs), testoMo(quota))
	err = db.Tx(context.Background(), s.DB, func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE combattenti SET ricompensato = 1 WHERE id = ? AND ricompensato = 0`, combattente)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errGiaRisposto // un'altra richiesta l'ha già pagata
		}
		if quota <= 0 {
			return nil
		}
		for _, pg := range pgs {
			if _, _, err := s.modificaScheda(tx, pg, u, func(p piatto) (bool, error) { rimborsa(p, quota); return true, nil }); err != nil {
				return err
			}
			s.registraFinanze(tx, pg, u, "ricompensa", quota, testo)
		}
		return nil
	})
	if err != nil {
		return ""
	}
	for _, pg := range pgs {
		s.avvisaFinanze(pg, u, testo)
	}
	return testo
}

func testoGS(gs float64) string {
	switch gs {
	case 0.125:
		return "1/8"
	case 0.25:
		return "1/4"
	case 0.5:
		return "1/2"
	}
	return strconv.FormatFloat(gs, 'f', -1, 64)
}
