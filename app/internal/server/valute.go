package server

// Valute del gioco (fase 3c, passo C4d): nei giochi diversi dalla quinta edizione il denaro è nei campi del modello della
// scheda libera indicati in «valute» (es. «denaro», «crediti»). La sezione Finanze mostra le valute dei personaggi; il
// Master dà o toglie, i giocatori trasferiscono ad altri personaggi che hanno la stessa valuta. Ogni movimento va nel
// registro delle Finanze e nella scheda (nuova versione, chi l'ha aperta la vede subito). Le monete della quinta edizione
// restano in finanze.go.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"gasterbless/internal/db"
)

type valutaPG struct {
	ID     string `json:"id"`
	Nome   string `json:"nome"`
	Valore int    `json:"valore"`
}

// valuteModello: le valute del modello della campagna del personaggio (id → nome); nessuna per la quinta edizione.
func (s *Server) valuteModello(pg string) (ordine []string, nomi map[string]string) {
	camp, mod := s.giocoDelPersonaggio(pg)
	nomi = map[string]string{}
	if mod == "d20-5e" || camp == "" {
		return nil, nomi
	}
	var m modelloScheda
	json.Unmarshal(s.modelloDellaCampagna(camp), &m)
	for _, v := range m.Valute {
		for _, sz := range m.Sezioni {
			for _, c := range sz.Campi {
				if c.ID == v {
					ordine = append(ordine, v)
					nomi[v] = c.Nome
				}
			}
		}
	}
	return ordine, nomi
}

func valoreCampo(p piatto, id string) int {
	raw, ok := p["campi."+id]
	if !ok {
		return 0
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return int(n)
	}
	var t string
	if json.Unmarshal(raw, &t) == nil {
		v, _ := strconv.Atoi(t)
		return v
	}
	return 0
}

func (s *Server) piattoCorrente(q interface {
	QueryRow(string, ...any) *sql.Row
}, pg string) piatto {
	var dati string
	q.QueryRow(`SELECT v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, pg).Scan(&dati)
	p, err := leggiPiatto(dati)
	if err != nil {
		return piatto{}
	}
	return p
}

func (s *Server) rotteValute(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/valute", api(s.apiValute))
	mux.Handle("POST /api/v1/valute/{pg}/master", api(s.apiValutaMaster))
	mux.Handle("POST /api/v1/valute/{pg}/trasferisci", api(s.apiValutaTrasferisci))
}

func (s *Server) apiValute(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	q := `SELECT p.id, p.nome, COALESCE(u.nome,'') FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id WHERE p.stato = 'attivo' AND p.scheda = ''`
	args := []any{}
	if !u.Master() {
		q += ` AND p.proprietario_id = ?`
		args = append(args, u.ID)
	}
	rows, err := s.DB.Query(q+` ORDER BY p.nome COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	type riga struct{ id, nome, gioc string }
	tutti := []riga{}
	for rows.Next() {
		var x riga
		rows.Scan(&x.id, &x.nome, &x.gioc)
		tutti = append(tutti, x)
	}
	rows.Close()
	pgs, ids := []map[string]any{}, []any{}
	for _, x := range tutti {
		ordine, nomi := s.valuteModello(x.id)
		if len(ordine) == 0 {
			continue
		}
		p := s.piattoCorrente(s.DB, x.id)
		v := []valutaPG{}
		for _, id := range ordine {
			v = append(v, valutaPG{ID: id, Nome: nomi[id], Valore: valoreCampo(p, id)})
		}
		pgs = append(pgs, map[string]any{"id": x.id, "nome": x.nome, "giocatore": x.gioc, "valute": v})
		ids = append(ids, x.id)
	}
	reg := []map[string]any{}
	if len(ids) > 0 {
		segnaposto := "?"
		for i := 1; i < len(ids); i++ {
			segnaposto += ",?"
		}
		if rows, err := s.DB.Query(`SELECT r.personaggio_id, p.nome, r.tipo, r.importo, r.testo, r.creato_il FROM registro_finanze r
			JOIN personaggi p ON p.id = r.personaggio_id WHERE r.personaggio_id IN (`+segnaposto+`) ORDER BY r.id DESC LIMIT 80`, ids...); err == nil {
			for rows.Next() {
				var pg, nome, tipo, testo string
				var imp int
				var quando int64
				rows.Scan(&pg, &nome, &tipo, &imp, &testo, &quando)
				reg = append(reg, map[string]any{"personaggio": pg, "nome": nome, "tipo": tipo, "importo": imp, "testo": testo, "quando": quando})
			}
			rows.Close()
		}
	}
	return map[string]any{"personaggi": pgs, "registro": reg}, nil
}

// cambiaValuta aggiunge delta (anche negativo) alla valuta del personaggio dentro la transazione; non si va sotto zero.
func (s *Server) cambiaValuta(tx *sql.Tx, pg string, u *Utente, valuta, nomeValuta, nomePG string, delta int) error {
	_, _, err := s.modificaSchedaNota(tx, pg, u, "finanze", func(p piatto) (bool, error) {
		v := valoreCampo(p, valuta)
		if v+delta < 0 {
			return false, errore(http.StatusConflict, fmt.Sprintf("%s ha solo %d %s.", nomePG, v, nomeValuta))
		}
		b, _ := json.Marshal(v + delta)
		p["campi."+valuta] = b
		return true, nil
	})
	return err
}

func (s *Server) avvisaValuta(pg string, u *Utente, testo string) {
	var prop, nome string
	var ver int
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,''), nome, versione_corrente FROM personaggi WHERE id = ?`, pg).Scan(&prop, &nome, &ver)
	m := Msg{Tipo: "finanze.movimento", Dati: map[string]any{"personaggio": pg, "nome": nome, "testo": testo}}
	s.Hub.AlMaster(m)
	if prop != "" {
		s.Hub.AUtente(prop, m)
	}
	s.avvisaSchedaDa(pg, prop, ver, u, []string{"campi"}, true)
}

func (s *Server) apiValutaMaster(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("pg")
	var in struct {
		Valuta  string `json:"valuta"`
		Importo int    `json:"importo"` // positivo = dà, negativo = toglie
		Motivo  string `json:"motivo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	motivo, err := rigaSingola(in.Motivo, "Motivo", 0, 120)
	if err != nil {
		return nil, err
	}
	_, nomi := s.valuteModello(pg)
	if nomi[in.Valuta] == "" || in.Importo == 0 || abs(in.Importo) > 100000000 {
		return nil, errDatiNonValidi
	}
	var nome, testo string
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&nome)
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if err := s.cambiaValuta(tx, pg, u, in.Valuta, nomi[in.Valuta], nome, in.Importo); err != nil {
			return err
		}
		verbo := "dà"
		if in.Importo < 0 {
			verbo = "toglie"
		}
		testo = fmt.Sprintf("Il Master %s %d %s a %s.", verbo, abs(in.Importo), nomi[in.Valuta], nome)
		if motivo != "" {
			testo += " Motivo: " + motivo + "."
		}
		s.registraFinanze(tx, pg, u, "master", in.Importo, testo)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.avvisaValuta(pg, u, testo)
	return map[string]any{"testo": testo}, nil
}

func (s *Server) apiValutaTrasferisci(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("pg")
	if !s.puoUsareMonete(u, pg) {
		return nil, errNonTrovato
	}
	var in struct {
		A       string `json:"a"`
		Valuta  string `json:"valuta"`
		Importo int    `json:"importo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	var stato, nomeA, nomeDa string
	if s.DB.QueryRow(`SELECT stato, nome FROM personaggi WHERE id = ?`, in.A).Scan(&stato, &nomeA) != nil || stato != "attivo" || in.A == pg {
		return nil, errore(http.StatusBadRequest, "Scegli un altro personaggio attivo.")
	}
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&nomeDa)
	_, nomi := s.valuteModello(pg)
	_, nomiA := s.valuteModello(in.A)
	if nomi[in.Valuta] == "" {
		return nil, errDatiNonValidi
	}
	if nomiA[in.Valuta] == "" {
		return nil, errore(http.StatusBadRequest, nomeA+" non usa la valuta «"+nomi[in.Valuta]+"».")
	}
	if in.Importo < 1 || in.Importo > 100000000 {
		return nil, errore(http.StatusBadRequest, "Importo non valido.")
	}
	var testo string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if err := s.cambiaValuta(tx, pg, u, in.Valuta, nomi[in.Valuta], nomeDa, -in.Importo); err != nil {
			return err
		}
		if err := s.cambiaValuta(tx, in.A, u, in.Valuta, nomiA[in.Valuta], nomeA, in.Importo); err != nil {
			return err
		}
		testo = fmt.Sprintf("%s trasferisce %d %s a %s.", nomeDa, in.Importo, nomi[in.Valuta], nomeA)
		s.registraFinanze(tx, pg, u, "trasferimento", -in.Importo, testo)
		s.registraFinanze(tx, in.A, u, "trasferimento", in.Importo, testo)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.avvisaValuta(pg, u, testo)
	s.avvisaValuta(in.A, u, testo)
	return map[string]any{"testo": testo}, nil
}
