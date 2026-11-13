package server

// Riposo lungo e riposo breve (richiesta dell'utente, 2026-11-11): tasti nella barra in alto dell'app.
//
// Un giocatore chiede il riposo per uno dei suoi personaggi e per i compagni del gruppo che sceglie; la richiesta va al
// Master, che approva o rifiuta (basta lui: i compagni ricevono solo un avviso). Il Master può far riposare subito.
// Approvato il riposo (regole quinta edizione 2014):
//   - lungo (8 ore di gioco): PF al massimo, PF temporanei finiti, tiri contro la morte azzerati, tutti gli slot,
//     recupero di metà dei Dadi Vita totali (almeno 1), Indebolimento −1;
//   - breve (1 ora di gioco): slot della Magia del Patto; poi ogni personaggio sceglie quanti Dadi Vita spendere
//     (il server tira dado + modificatore di Costituzione, mai sopra i PF massimi).
// Passa il tempo di gioco del gruppo (tempo_gruppo, come i viaggi: ogni 24 ore un giorno nuovo con «Passa un giorno»).
// Avvisi al Master prima di approvare: combattimento in corso, riposo lungo meno di 24 ore di gioco fa, personaggio a
// 0 PF (non beneficia del riposo lungo); il Master può approvare lo stesso («forza»).
// I Dadi Vita usati sono il campo libero della scheda «f.hdUsati» (numero); i Dadi Vita totali, il modificatore di
// Costituzione e gli slot del patto arrivano dal riepilogo della scheda (dadiVita, modCos, patto).

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const (
	oreRiposoLungo  = 8.0
	oreRiposoBreve  = 1.0
	maxPartecipanti = 20
)

func (s *Server) rotteRiposi(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/riposi", api(s.apiRiposi))
	mux.Handle("POST /api/v1/riposi", api(s.apiChiediRiposo))
	mux.Handle("POST /api/v1/riposi/{id}", api(s.apiDecidiRiposo))
	mux.Handle("DELETE /api/v1/riposi/{id}", api(s.apiRitiraRiposo))
	mux.Handle("POST /api/v1/riposi/{id}/dadi-vita", api(s.apiDadiVita))
}

// gruppiRiposo: i gruppi (non archiviati) in cui l'utente può chiedere un riposo (Master: tutti).
func (s *Server) gruppiRiposo(u *Utente) []struct{ ID, Nome string } {
	q, args := `SELECT id, nome FROM gruppi WHERE archiviato = 0 ORDER BY nome`, []any{}
	if !u.Master() {
		q, args = `SELECT g.id, g.nome FROM gruppi g JOIN iscrizioni i ON i.gruppo_id = g.id WHERE i.utente_id = ? AND g.archiviato = 0 ORDER BY g.nome`, []any{u.ID}
	}
	var out []struct{ ID, Nome string }
	righe, err := s.DB.Query(q, args...)
	if err != nil {
		return nil
	}
	defer righe.Close()
	for righe.Next() {
		var g struct{ ID, Nome string }
		righe.Scan(&g.ID, &g.Nome)
		out = append(out, g)
	}
	return out
}

type pgRiposo struct {
	ID           string `json:"id"`
	Nome         string `json:"nome"`
	Proprietario string `json:"proprietario,omitempty"`
	Giocatore    string `json:"giocatore,omitempty"`
	Mio          bool   `json:"mio,omitempty"`
}

func (s *Server) personaggiGruppo(gruppo, uid string) []pgRiposo {
	righe, err := s.DB.Query(`SELECT p.id, p.nome, COALESCE(p.proprietario_id,''), COALESCE(u.nome,'') FROM personaggi p
		LEFT JOIN utenti u ON u.id = p.proprietario_id WHERE p.gruppo_id = ? AND p.stato = 'attivo' ORDER BY p.nome`, gruppo)
	if err != nil {
		return nil
	}
	defer righe.Close()
	out := []pgRiposo{}
	for righe.Next() {
		var p pgRiposo
		righe.Scan(&p.ID, &p.Nome, &p.Proprietario, &p.Giocatore)
		p.Mio = p.Proprietario != "" && p.Proprietario == uid
		out = append(out, p)
	}
	return out
}

func (s *Server) combattimentoNelGruppo(gruppo string) bool {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM combattimenti k JOIN sessioni_gioco g ON g.id = k.sessione_id WHERE g.gruppo_id = ? AND k.attivo = 1`, gruppo).Scan(&n)
	return n > 0
}

func (s *Server) oreGiocoGruppo(gruppo string) (float64, int) {
	var ore float64
	giorno := 1
	s.DB.QueryRow(`SELECT ore_gioco, giorno FROM tempo_gruppo WHERE gruppo_id = ?`, gruppo).Scan(&ore, &giorno)
	return ore, giorno
}

// avvisiRiposo: ciò che il Master deve sapere prima di approvare.
func (s *Server) avvisiRiposo(gruppo, tipo string, pgs []string) []string {
	var out []string
	if s.combattimentoNelGruppo(gruppo) {
		out = append(out, "C'è un combattimento in corso in una sessione del gruppo.")
	}
	if tipo != "lungo" {
		return out
	}
	ore, _ := s.oreGiocoGruppo(gruppo)
	for _, pg := range pgs {
		var nome string
		var ultimo sql.NullFloat64
		s.DB.QueryRow(`SELECT nome, ultimo_riposo_lungo FROM personaggi WHERE id = ?`, pg).Scan(&nome, &ultimo)
		if ultimo.Valid && ore-ultimo.Float64 < 24 {
			out = append(out, fmt.Sprintf("%s: ultimo riposo lungo %s di gioco fa (ne serve uno ogni 24 ore).", nome, durataOre(ore-ultimo.Float64)))
		}
		if p, ok := s.leggiInGioco(pg)["hpCur"]; ok && strings.Trim(string(p), `"`) == "0" {
			out = append(out, nome+" ha 0 PF: per le regole non beneficia del riposo lungo.")
		}
	}
	return out
}

func durataOre(h float64) string {
	if h < 1 {
		return strconv.Itoa(int(math.Round(h*60))) + " minuti"
	}
	if math.Abs(h-math.Round(h)) < 0.05 {
		if math.Round(h) == 1 {
			return "1 ora"
		}
		return strconv.Itoa(int(math.Round(h))) + " ore"
	}
	return strconv.FormatFloat(h, 'f', 1, 64) + " ore"
}

func nomiTipo(tipo string) (string, float64) {
	if tipo == "lungo" {
		return "riposo lungo", oreRiposoLungo
	}
	return "riposo breve", oreRiposoBreve
}

func (s *Server) vistaRiposo(id string) map[string]any {
	var gruppo, tipo, utente, pgsJSON, stato, esito, dadi, nomeGruppo, chi string
	var creato int64
	if s.DB.QueryRow(`SELECT r.gruppo_id, r.tipo, COALESCE(r.utente_id,''), r.personaggi, r.stato, r.esito, r.dadi_spesi, r.creato_il, COALESCE(g.nome,''), COALESCE(u.nome,'')
		FROM riposi r LEFT JOIN gruppi g ON g.id = r.gruppo_id LEFT JOIN utenti u ON u.id = r.utente_id WHERE r.id = ?`, id).
		Scan(&gruppo, &tipo, &utente, &pgsJSON, &stato, &esito, &dadi, &creato, &nomeGruppo, &chi) != nil {
		return nil
	}
	var ids []string
	json.Unmarshal([]byte(pgsJSON), &ids)
	tutti := map[string]pgRiposo{}
	for _, p := range s.personaggiGruppo(gruppo, "") {
		tutti[p.ID] = p
	}
	pgs := []pgRiposo{}
	for _, x := range ids {
		if p, ok := tutti[x]; ok {
			pgs = append(pgs, p)
		} else {
			var n string
			s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, x).Scan(&n)
			pgs = append(pgs, pgRiposo{ID: x, Nome: n})
		}
	}
	v := map[string]any{"id": id, "gruppo": gruppo, "nomeGruppo": nomeGruppo, "tipo": tipo, "chi": chi, "utente": utente, "personaggi": pgs,
		"stato": stato, "esito": esito, "dadiSpesi": json.RawMessage(dadi), "creato": creato}
	if stato == "in_attesa" {
		v["avvisi"] = s.avvisiRiposo(gruppo, tipo, ids)
	}
	return v
}

// apiRiposi: gruppi e personaggi tra cui scegliere, richieste in attesa, riposi brevi dove spendere ancora Dadi Vita.
func (s *Server) apiRiposi(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	gruppi := []map[string]any{}
	ids := []any{}
	for _, g := range s.gruppiRiposo(u) {
		ore, giorno := s.oreGiocoGruppo(g.ID)
		gruppi = append(gruppi, map[string]any{"id": g.ID, "nome": g.Nome, "personaggi": s.personaggiGruppo(g.ID, u.ID),
			"combattimento": s.combattimentoNelGruppo(g.ID), "giorno": giorno, "ora": math.Mod(ore, 24)})
		ids = append(ids, g.ID)
	}
	attesa, brevi := []map[string]any{}, []map[string]any{}
	if len(ids) > 0 {
		segni := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		righe, err := s.DB.Query(`SELECT id FROM riposi WHERE gruppo_id IN (`+segni+`) AND (stato = 'in_attesa' OR (stato = 'approvato' AND tipo = 'breve' AND deciso_il > ?)) ORDER BY creato_il`,
			append(ids, time.Now().Add(-12*time.Hour).Unix())...)
		if err == nil {
			var l []string
			for righe.Next() {
				var id string
				righe.Scan(&id)
				l = append(l, id)
			}
			righe.Close()
			for _, id := range l {
				v := s.vistaRiposo(id)
				if v == nil {
					continue
				}
				if v["stato"] == "in_attesa" {
					attesa = append(attesa, v)
				} else if da := s.daSpendere(v, u); len(da) > 0 {
					v["daSpendere"] = da
					brevi = append(brevi, v)
				}
			}
		}
	}
	return map[string]any{"gruppi": gruppi, "inAttesa": attesa, "dadiVita": brevi, "master": u.Master()}, nil
}

// daSpendere: i personaggi del riposo breve che l'utente può ancora far spendere (suoi; per il Master anche quelli senza giocatore).
func (s *Server) daSpendere(v map[string]any, u *Utente) []map[string]any {
	var spesi map[string]int
	json.Unmarshal(v["dadiSpesi"].(json.RawMessage), &spesi)
	out := []map[string]any{}
	for _, p := range v["personaggi"].([]pgRiposo) {
		if _, fatto := spesi[p.ID]; fatto {
			continue
		}
		if p.Proprietario == u.ID || u.Master() && p.Proprietario == "" {
			out = append(out, s.infoDadiVita(p.ID, p.Nome))
		}
	}
	return out
}

type dadoVita struct {
	F int `json:"f"` // facce (6, 8, 10, 12)
	N int `json:"n"` // quanti in totale (livelli della classe)
}
type slotPatto struct {
	Lv int `json:"lv"`
	N  int `json:"n"`
}

// datiRiposo: riepilogo e Dadi Vita usati della versione corrente della scheda.
func (s *Server) datiRiposo(pg string) (riepilogo, int) {
	var riep, dati string
	s.DB.QueryRow(`SELECT v.riepilogo, v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, pg).Scan(&riep, &dati)
	var r riepilogo
	json.Unmarshal([]byte(riep), &r)
	usati := 0
	if p, err := leggiPiatto(dati); err == nil {
		usati = dadiUsatiDa(p["f.hdUsati"])
	}
	return r, usati
}

// dadiUsatiDa: il campo «usati» della scheda è testo libero: si legge il primo numero («2», «2d10» → 2).
func dadiUsatiDa(v json.RawMessage) int {
	var t string
	if json.Unmarshal(v, &t) != nil {
		var n float64
		if json.Unmarshal(v, &n) == nil {
			return max(0, int(n))
		}
		return 0
	}
	t = strings.TrimSpace(t)
	i := 0
	for i < len(t) && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	n, _ := strconv.Atoi(t[:i])
	return max(0, n)
}

func totaleDadi(l []dadoVita) int {
	t := 0
	for _, d := range l {
		t += d.N
	}
	return t
}

func (s *Server) infoDadiVita(pg, nome string) map[string]any {
	r, usati := s.datiRiposo(pg)
	tot := totaleDadi(r.DadiVita)
	g := s.leggiInGioco(pg)
	pf := r.PFMax
	if v, ok := g["hpCur"]; ok {
		if n, ok := numeroONull(v).(int64); ok {
			pf = int(n)
		}
	}
	mc := 0
	if r.ModCos != nil {
		mc = *r.ModCos
	}
	return map[string]any{"id": pg, "nome": nome, "dadiVita": r.DadiVita, "totale": tot, "usati": min(usati, tot), "liberi": max(0, tot-usati),
		"modCos": mc, "pf": pf, "pfMax": r.PFMax, "senzaDati": tot == 0}
}

// apiChiediRiposo: il giocatore chiede (o il Master fa subito) un riposo per i personaggi scelti.
func (s *Server) apiChiediRiposo(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	var in struct {
		Tipo, Gruppo string
		Personaggi   []string
		Forza        bool
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Tipo != "lungo" && in.Tipo != "breve" {
		return nil, errDatiNonValidi
	}
	visibile := false
	for _, g := range s.gruppiRiposo(u) {
		visibile = visibile || g.ID == in.Gruppo
	}
	if !visibile {
		return nil, errNonTrovato
	}
	if len(in.Personaggi) == 0 || len(in.Personaggi) > maxPartecipanti {
		return nil, errore(http.StatusBadRequest, "Scegli almeno un personaggio.")
	}
	delGruppo := map[string]pgRiposo{}
	for _, p := range s.personaggiGruppo(in.Gruppo, u.ID) {
		delGruppo[p.ID] = p
	}
	visti, mio := map[string]bool{}, false
	var pgs []string
	for _, x := range in.Personaggi {
		p, ok := delGruppo[x]
		if !ok {
			return nil, errore(http.StatusBadRequest, "Un personaggio scelto non è attivo in questo gruppo.")
		}
		if visti[x] {
			continue
		}
		visti[x] = true
		mio = mio || p.Mio
		pgs = append(pgs, x)
	}
	if !u.Master() && !mio {
		return nil, errore(http.StatusBadRequest, "Nel riposo deve esserci almeno uno dei tuoi personaggi.")
	}
	if !u.Master() {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM riposi WHERE gruppo_id = ? AND stato = 'in_attesa'`, in.Gruppo).Scan(&n)
		if n > 0 {
			return nil, errore(http.StatusConflict, "C'è già una richiesta di riposo in attesa del Master per questo gruppo.")
		}
	}
	b, _ := json.Marshal(pgs)
	id := cripto.ID()
	if _, err := s.DB.Exec(`INSERT INTO riposi(id, gruppo_id, tipo, utente_id, personaggi, creato_il) VALUES(?,?,?,?,?,?)`, id, in.Gruppo, in.Tipo, u.ID, string(b), db.Ora()); err != nil {
		return nil, err
	}
	nome, _ := nomiTipo(in.Tipo)
	if u.Master() { // il Master non chiede: fa riposare
		if av := s.avvisiRiposo(in.Gruppo, in.Tipo, pgs); len(av) > 0 && !in.Forza {
			s.DB.Exec(`UPDATE riposi SET stato = 'annullato' WHERE id = ?`, id)
			return nil, errore(http.StatusConflict, "Da confermare: "+strings.Join(av, " "))
		}
		return s.eseguiRiposo(u, id)
	}
	vista := s.vistaRiposo(id)
	s.Hub.AlMaster(Msg{Tipo: "riposo.richiesta", Dati: vista})
	for _, x := range pgs { // i compagni scelti lo sanno
		if p := delGruppo[x]; p.Proprietario != "" && p.Proprietario != u.ID {
			s.Hub.AUtente(p.Proprietario, Msg{Tipo: "riposo.info", Dati: map[string]any{"testo": u.Nome + " ha chiesto al Master un " + nome + " anche per " + p.Nome + "."}})
		}
	}
	s.evento("riposi", u.ID, "richiesta di "+nome)
	return map[string]any{"riposo": vista, "testo": "Richiesta di " + nome + " inviata al Master: aspetta la sua autorizzazione."}, nil
}

// apiDecidiRiposo: il Master approva («forza» se ci sono avvisi) o rifiuta.
func (s *Server) apiDecidiRiposo(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Azione string
		Forza  bool
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var gruppo, tipo, pgsJSON, utente string
	if s.DB.QueryRow(`SELECT gruppo_id, tipo, personaggi, COALESCE(utente_id,'') FROM riposi WHERE id = ? AND stato = 'in_attesa'`, id).Scan(&gruppo, &tipo, &pgsJSON, &utente) != nil {
		return nil, errore(http.StatusConflict, "Richiesta già decisa, ritirata o non trovata.")
	}
	nome, _ := nomiTipo(tipo)
	switch in.Azione {
	case "rifiuta":
		testo := "Il Master non ha autorizzato il " + nome + "."
		s.DB.Exec(`UPDATE riposi SET stato = 'rifiutato', esito = ?, deciso_il = ?, deciso_da = ? WHERE id = ?`, testo, db.Ora(), u.ID, id)
		s.avvisaPartecipanti(id, Msg{Tipo: "riposo.esito", Dati: map[string]any{"id": id, "gruppo": gruppo, "testo": testo}})
		s.Hub.AlMaster(Msg{Tipo: "riposo.deciso", Dati: map[string]any{"id": id}})
		return map[string]any{"testo": testo}, nil
	case "approva":
		var pgs []string
		json.Unmarshal([]byte(pgsJSON), &pgs)
		if av := s.avvisiRiposo(gruppo, tipo, pgs); len(av) > 0 && !in.Forza {
			return nil, errore(http.StatusConflict, "Da confermare: "+strings.Join(av, " "))
		}
		return s.eseguiRiposo(u, id)
	}
	return nil, errDatiNonValidi
}

// apiRitiraRiposo: chi ha chiesto il riposo lo ritira finché è in attesa.
func (s *Server) apiRitiraRiposo(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	res, err := s.DB.Exec(`UPDATE riposi SET stato = 'annullato', deciso_il = ? WHERE id = ? AND stato = 'in_attesa' AND (utente_id = ? OR ?)`, db.Ora(), id, u.ID, u.Master())
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errore(http.StatusConflict, "Nessuna richiesta di riposo da ritirare.")
	}
	s.Hub.AlMaster(Msg{Tipo: "riposo.deciso", Dati: map[string]any{"id": id}})
	return map[string]any{"testo": "Richiesta di riposo ritirata."}, nil
}

// avvisaPartecipanti: messaggio ai giocatori dei personaggi del riposo e a chi l'ha chiesto.
func (s *Server) avvisaPartecipanti(id string, m Msg) {
	var pgsJSON, utente string
	s.DB.QueryRow(`SELECT personaggi, COALESCE(utente_id,'') FROM riposi WHERE id = ?`, id).Scan(&pgsJSON, &utente)
	var pgs []string
	json.Unmarshal([]byte(pgsJSON), &pgs)
	fatti := map[string]bool{}
	if utente != "" {
		s.Hub.AUtente(utente, m)
		fatti[utente] = true
	}
	for _, pg := range pgs {
		var prop string
		s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, pg).Scan(&prop)
		if prop != "" && !fatti[prop] {
			s.Hub.AUtente(prop, m)
			fatti[prop] = true
		}
	}
}

// eseguiRiposo applica gli effetti a tutti i partecipanti e fa passare il tempo di gioco del gruppo.
func (s *Server) eseguiRiposo(u *Utente, id string) (any, error) {
	var gruppo, tipo, pgsJSON string
	if s.DB.QueryRow(`SELECT gruppo_id, tipo, personaggi FROM riposi WHERE id = ?`, id).Scan(&gruppo, &tipo, &pgsJSON) != nil {
		return nil, errNonTrovato
	}
	var pgs []string
	json.Unmarshal([]byte(pgsJSON), &pgs)
	nome, ore := nomiTipo(tipo)
	oreOra, _ := s.oreGiocoGruppo(gruppo)
	oreDopo := oreOra + ore
	type cambio struct {
		pg, prop string
		vers     int
		campi    []string
	}
	var cambi []cambio
	var righe []string
	// Le schede si leggono prima della transazione (nella transazione si usa solo tx).
	type lettura struct {
		r     riepilogo
		usati int
	}
	letture := map[string]lettura{}
	for _, pg := range pgs {
		r, usati := s.datiRiposo(pg)
		letture[pg] = lettura{r, usati}
	}
	err := db.Tx(context.Background(), s.DB, func(tx *sql.Tx) error {
		for _, pg := range pgs {
			var nomePG, prop string
			tx.QueryRow(`SELECT nome, COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, pg).Scan(&nomePG, &prop)
			r, usati := letture[pg].r, letture[pg].usati
			tot := totaleDadi(r.DadiVita)
			if _, err := tx.Exec(`INSERT OR IGNORE INTO stato_in_gioco(personaggio_id, modificato_il) VALUES(?,?)`, pg, db.Ora()); err != nil {
				return err
			}
			var cond, slot string
			tx.QueryRow(`SELECT condizioni, slot_usati FROM stato_in_gioco WHERE personaggio_id = ?`, pg).Scan(&cond, &slot)
			c := cambio{pg: pg, prop: prop}
			if tipo == "lungo" {
				var pf any // PF al massimo (NULL se il massimo non è noto: la scheda mostra il massimo)
				if r.PFMax > 0 {
					pf = r.PFMax
				}
				ind := livelloIndebolimento(cond)
				if _, err := tx.Exec(`UPDATE stato_in_gioco SET pf = ?, pf_temp = 0, ts_morte = '{"s":0,"f":0}', slot_usati = '{}', condizioni = ?, modificato_il = ?, modificato_da = ? WHERE personaggio_id = ?`,
					pf, conLivelloIndebolimento(cond, max(0, ind-1)), db.Ora(), u.ID, pg); err != nil {
					return err
				}
				t := nomePG + ": PF al massimo, slot recuperati"
				if ind > 0 {
					t += ", Indebolimento " + strconv.Itoa(ind) + " → " + strconv.Itoa(ind-1)
				}
				if tot > 0 {
					rec := max(1, tot/2)
					nuovi := max(0, usati-rec)
					if nuovi != usati {
						n, campi, err := s.modificaSchedaNota(tx, pg, u, "riposo lungo", func(p piatto) (bool, error) {
							b, _ := json.Marshal(strconv.Itoa(nuovi))
							p["f.hdUsati"] = b
							return true, nil
						})
						if err != nil {
							return err
						}
						c.vers, c.campi = n, campi
					}
					t += fmt.Sprintf(", Dadi Vita usati %d → %d (su %d)", usati, nuovi, tot)
				}
				if _, err := tx.Exec(`UPDATE personaggi SET ultimo_riposo_lungo = ? WHERE id = ?`, oreDopo, pg); err != nil {
					return err
				}
				righe = append(righe, t)
			} else {
				t := nomePG + ": può spendere Dadi Vita"
				if r.Patto != nil && r.Patto.N > 0 {
					m := map[string]int{}
					json.Unmarshal([]byte(slot), &m)
					k := strconv.Itoa(r.Patto.Lv)
					if m[k] > 0 {
						m[k] = max(0, m[k]-r.Patto.N)
						b, _ := json.Marshal(m)
						if _, err := tx.Exec(`UPDATE stato_in_gioco SET slot_usati = ?, modificato_il = ?, modificato_da = ? WHERE personaggio_id = ?`, string(b), db.Ora(), u.ID, pg); err != nil {
							return err
						}
						t += ", slot del patto recuperati"
					}
				}
				righe = append(righe, t)
			}
			cambi = append(cambi, c)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	giorno, nuovi := s.avanzaTempoGruppo(gruppo, ore, u)
	testo := strings.ToUpper(nome[:1]) + nome[1:] + ": passa" + map[bool]string{true: "no 8 ore", false: " 1 ora"}[tipo == "lungo"] + " di gioco"
	if nuovi > 0 {
		testo += fmt.Sprintf(" ed è il giorno %d", giorno)
	}
	testo += ". " + strings.Join(righe, "; ") + "."
	s.DB.Exec(`UPDATE riposi SET stato = 'approvato', esito = ?, deciso_il = ?, deciso_da = ? WHERE id = ?`, testo, db.Ora(), u.ID, id)
	if tipo == "lungo" { // finiscono gli effetti fino a 24 ore dei personaggi che riposano (effetti_persistenti.go)
		for _, c := range cambi {
			s.fineDurevoliRiposo(u, c.pg)
		}
	}
	for _, c := range cambi {
		if c.vers > 0 {
			s.avvisaSchedaDa(c.pg, c.prop, c.vers, u, c.campi, true)
		}
		s.avvisaStatoInGioco(c.pg, c.prop)
	}
	vista := s.vistaRiposo(id)
	dati := map[string]any{"id": id, "gruppo": gruppo, "tipo": tipo, "testo": testo, "giorno": giorno, "personaggi": vista["personaggi"]}
	s.Hub.Pubblica("gruppo:"+gruppo, Msg{Tipo: "riposo.fatto", Dati: dati})
	s.Hub.AlMaster(Msg{Tipo: "riposo.fatto", Dati: dati})
	s.Hub.AlMaster(Msg{Tipo: "riposo.deciso", Dati: map[string]any{"id": id}})
	s.evento("riposi", u.ID, nome+" approvato")
	return map[string]any{"testo": testo, "riposo": vista}, nil
}

// avanzaTempoGruppo: come i viaggi, le ore si sommano al tempo di gioco del gruppo; ogni 24 ore un giorno nuovo.
func (s *Server) avanzaTempoGruppo(gruppo string, ore float64, u *Utente) (int, int) {
	s.DB.Exec(`INSERT OR IGNORE INTO tempo_gruppo(gruppo_id, modificato_il) VALUES(?,?)`, gruppo, db.Ora())
	prima, giorno := s.oreGiocoGruppo(gruppo)
	dopo := prima + ore
	nuovi := int(math.Floor(dopo/24)) - int(math.Floor(prima/24))
	if nuovi > 0 {
		giorno += nuovi
		s.DB.Exec(`UPDATE tempo_gruppo SET ore_gioco = ?, ore_marcia = 0, giorno = ?, modificato_il = ? WHERE gruppo_id = ?`, dopo, giorno, db.Ora(), gruppo)
	} else {
		s.DB.Exec(`UPDATE tempo_gruppo SET ore_gioco = ?, modificato_il = ? WHERE gruppo_id = ?`, dopo, db.Ora(), gruppo)
	}
	for i := 0; i < nuovi && i < 30; i++ {
		s.passaGiornoGioco(u)
	}
	if nuovi > 0 {
		s.fineDurevoliGiorno(u, gruppo)               // finiscono gli effetti fino a 24 ore (effetti_persistenti.go)
		go s.controllaInneschi(gruppo, "giorno", nil) // eventi del Codice «dal giorno N» (lore_promemoria.go)
	}
	s.avvisaMappa(gruppo, map[string]any{"t": "viaggi"})
	return giorno, nuovi
}

// apiDadiVita: riposo breve approvato; il giocatore (o il Master per i personaggi senza giocatore) spende Dadi Vita.
// {personaggio, dadi: {"10": 2, "6": 1}} → tiri di dado + Costituzione (mai sotto 0 per dado), PF fino al massimo.
func (s *Server) apiDadiVita(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	var in struct {
		Personaggio string
		Dadi        map[string]int
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var pgsJSON, spesiJSON, gruppo string
	var deciso int64
	if s.DB.QueryRow(`SELECT personaggi, dadi_spesi, gruppo_id, COALESCE(deciso_il,0) FROM riposi WHERE id = ? AND stato = 'approvato' AND tipo = 'breve'`, id).Scan(&pgsJSON, &spesiJSON, &gruppo, &deciso) != nil {
		return nil, errore(http.StatusConflict, "Non c'è un riposo breve approvato.")
	}
	if time.Now().Unix()-deciso > 12*3600 {
		return nil, errore(http.StatusConflict, "Questo riposo breve è finito da troppo tempo.")
	}
	var pgs []string
	json.Unmarshal([]byte(pgsJSON), &pgs)
	presente := false
	for _, x := range pgs {
		presente = presente || x == in.Personaggio
	}
	var prop, nomePG string
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,''), nome FROM personaggi WHERE id = ?`, in.Personaggio).Scan(&prop, &nomePG)
	if !presente || !(prop == u.ID || u.Master()) {
		return nil, errNonTrovato
	}
	spesi := map[string]int{}
	json.Unmarshal([]byte(spesiJSON), &spesi)
	if _, fatto := spesi[in.Personaggio]; fatto {
		return nil, errore(http.StatusConflict, "I Dadi Vita di "+nomePG+" per questo riposo sono già stati spesi.")
	}
	rp, usati := s.datiRiposo(in.Personaggio)
	tot := totaleDadi(rp.DadiVita)
	liberi := max(0, tot-usati)
	perFacce := map[int]int{}
	for _, d := range rp.DadiVita {
		perFacce[d.F] += d.N
	}
	n := 0
	facce := []int{}
	for k, q := range in.Dadi {
		f, err := strconv.Atoi(k)
		if err != nil || q < 0 || q > perFacce[f] {
			return nil, errore(http.StatusBadRequest, "Dadi Vita non validi.")
		}
		for i := 0; i < q; i++ {
			facce = append(facce, f)
		}
		n += q
	}
	if n > liberi {
		return nil, errore(http.StatusBadRequest, fmt.Sprintf("%s ha solo %d Dadi Vita liberi.", nomePG, liberi))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(facce)))
	mc := 0
	if rp.ModCos != nil {
		mc = *rp.ModCos
	}
	tiri, curati := []string{}, 0
	type gruppoDadi struct { // per i dadi 3D della pagina (dadi D2): come i gruppi di dadi.risultato
		N      int   `json:"n"`
		Facce  int   `json:"facce"`
		Valori []int `json:"valori"`
	}
	gruppi := []gruppoDadi{}
	for _, f := range facce {
		t := tiraDado(f)
		v := max(0, t+mc)
		curati += v
		tiri = append(tiri, fmt.Sprintf("d%d: %d%+d = %d", f, t, mc, v))
		if len(gruppi) == 0 || gruppi[len(gruppi)-1].Facce != f {
			gruppi = append(gruppi, gruppoDadi{Facce: f})
		}
		g := &gruppi[len(gruppi)-1]
		g.N, g.Valori = g.N+1, append(g.Valori, t)
	}
	pfPrima, pfDopo := 0, 0
	var vers int
	var campi []string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO stato_in_gioco(personaggio_id, modificato_il) VALUES(?,?)`, in.Personaggio, db.Ora()); err != nil {
			return err
		}
		var pf sql.NullInt64
		tx.QueryRow(`SELECT pf FROM stato_in_gioco WHERE personaggio_id = ?`, in.Personaggio).Scan(&pf)
		pfPrima = rp.PFMax
		if pf.Valid {
			pfPrima = int(pf.Int64)
		}
		pfDopo = pfPrima + curati
		if rp.PFMax > 0 {
			pfDopo = min(pfDopo, rp.PFMax)
		}
		if n > 0 {
			if _, err := tx.Exec(`UPDATE stato_in_gioco SET pf = ?, modificato_il = ?, modificato_da = ? WHERE personaggio_id = ?`, pfDopo, db.Ora(), u.ID, in.Personaggio); err != nil {
				return err
			}
			var err error
			vers, campi, err = s.modificaSchedaNota(tx, in.Personaggio, u, "riposo breve: Dadi Vita", func(p piatto) (bool, error) {
				b, _ := json.Marshal(strconv.Itoa(usati + n))
				p["f.hdUsati"] = b
				return true, nil
			})
			if err != nil {
				return err
			}
		}
		spesi[in.Personaggio] = n
		b, _ := json.Marshal(spesi)
		_, err := tx.Exec(`UPDATE riposi SET dadi_spesi = ? WHERE id = ?`, string(b), id)
		return err
	})
	if err != nil {
		return nil, err
	}
	testo := nomePG + " non spende Dadi Vita."
	if n > 0 {
		testo = fmt.Sprintf("%s spende %d Dadi Vita (%s): +%d PF, ora %d/%d.", nomePG, n, strings.Join(tiri, ", "), pfDopo-pfPrima, pfDopo, rp.PFMax)
		if vers > 0 {
			s.avvisaSchedaDa(in.Personaggio, prop, vers, u, campi, true)
		}
		s.avvisaStatoInGioco(in.Personaggio, prop)
	}
	s.Hub.AlMaster(Msg{Tipo: "riposo.dadi", Dati: map[string]any{"id": id, "testo": testo}})
	return map[string]any{"testo": testo, "pf": pfDopo, "curati": pfDopo - pfPrima, "tiri": tiri, "dadi": gruppi, "modCos": mc, "tirato": curati}, nil
}
