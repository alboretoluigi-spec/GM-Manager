package server

// Controllo delle schede all'accesso in sessione (richiesta dell'utente, 2026-11-07).
//
// Ogni versione della scheda salvata da un giocatore porta due segni: «offline» (modifiche fatte sul telefono senza
// rete e inviate dopo) e «fuori sessione» (nessuna sessione del suo gruppo era in corso). Quando il giocatore entra in
// una sessione in corso (il Master la avvia, lui conferma la presenza o si collega all'app), Gaster Bless confronta i campi
// importanti della scheda con quelli dell'ultimo controllo. Solo se trova modifiche offline o fuori sessione il Master
// riceve un avviso con il prima/dopo e sceglie «Va bene» oppure «Ripristina». Altrimenti non succede nulla.

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

// Campi che non contano per il controllo: aspetto, testi personali e campi già gestiti altrove
// (monete: richiesta al Master; stato in gioco: PF, slot, tiri contro la morte, ispirazione).
var campiNonControllati = map[string]bool{"v": true, "img": true, "sym": true, "san": true, "terra": true, "coins": true}

// Tra i campi liberi (f.*) contano solo quelli che cambiano il personaggio.
var campiLiberiControllati = map[string]bool{"f.pe": true, "f.tesoro": true, "f.altreComp": true}

func campoControllato(k string) bool {
	if campoInGioco(k) {
		return false
	}
	testa, _, _ := strings.Cut(k, ".")
	if testa == "f" {
		return campiLiberiControllati[k]
	}
	return !campiNonControllati[testa]
}

type differenzaScheda struct {
	Campo   string          `json:"campo"`
	Prima   json.RawMessage `json:"prima"`
	Dopo    json.RawMessage `json:"dopo"`
	Offline bool            `json:"offline"`
}

var muControllo sync.Mutex // un controllo alla volta: accesso e conferma possono arrivare insieme

// fuoriSessione: il giocatore non ha una sessione in corso in nessuno dei suoi gruppi.
func (s *Server) fuoriSessione(u *Utente) bool {
	if u == nil || u.Master() {
		return false
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM sessioni_gioco g JOIN iscrizioni i ON i.gruppo_id = g.gruppo_id
		WHERE i.utente_id = ? AND g.stato = 'in_corso'`, u.ID).Scan(&n)
	return n == 0
}

// controllaScheda confronta la scheda con l'ultimo controllo. Restituisce l'id dell'avviso creato ("" se non serve).
func (s *Server) controllaScheda(pg, sessione string) string {
	muControllo.Lock()
	defer muControllo.Unlock()
	var vc, corr int
	var prop, nome string
	if s.DB.QueryRow(`SELECT versione_controllata, versione_corrente, COALESCE(proprietario_id,''), nome FROM personaggi WHERE id = ?`, pg).
		Scan(&vc, &corr, &prop, &nome) != nil || corr <= vc {
		return ""
	}
	segna := func() { s.DB.Exec(`UPDATE personaggi SET versione_controllata = ? WHERE id = ?`, corr, pg) }
	if vc == 0 { // personaggio nuovo, mai controllato: la scheda di oggi diventa il punto di partenza
		segna()
		return ""
	}
	// Un avviso ancora aperto per lo stesso personaggio si unisce al nuovo (il «prima» resta quello più vecchio).
	var apertoID string
	var apertoDa int
	if s.DB.QueryRow(`SELECT id, versione_da FROM controlli_scheda WHERE personaggio_id = ? AND stato = 'aperto' ORDER BY creato_il DESC LIMIT 1`, pg).
		Scan(&apertoID, &apertoDa) == nil && apertoDa < vc {
		vc = apertoDa
	}
	// Versione di partenza: quella dell'ultimo controllo (o la più vicina rimasta prima di essa).
	var datiPrima string
	if s.DB.QueryRow(`SELECT dati FROM schede_versioni WHERE personaggio_id = ? AND numero <= ? ORDER BY numero DESC LIMIT 1`, pg, vc).Scan(&datiPrima) != nil {
		segna()
		return ""
	}
	var datiDopo string
	s.DB.QueryRow(`SELECT dati FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, pg, corr).Scan(&datiDopo)
	prima, err1 := leggiPiatto(datiPrima)
	dopo, err2 := leggiPiatto(datiDopo)
	if err1 != nil || err2 != nil {
		segna()
		return ""
	}
	// Quali campi sono stati cambiati offline o fuori sessione dopo il controllo?
	segnati, segnatiOffline := map[string]bool{}, map[string]bool{}
	tutti, tuttiOffline, offline := false, false, false
	righe, err := s.DB.Query(`SELECT campi, nota, offline FROM schede_versioni
		WHERE personaggio_id = ? AND numero > ? AND numero <= ? AND (offline = 1 OR fuori_sessione = 1)`, pg, vc, corr)
	if err != nil {
		return ""
	}
	for righe.Next() {
		var campi, nota string
		var off int
		righe.Scan(&campi, &nota, &off)
		var l []string
		json.Unmarshal([]byte(campi), &l)
		if len(l) == 0 || l[0] == "*" || strings.HasPrefix(nota, "ripristinata") { // ripristino di una versione: tutta la scheda
			tutti = true
			tuttiOffline = tuttiOffline || off == 1
		}
		for _, k := range l {
			segnati[k] = true
			if off == 1 {
				segnatiOffline[k] = true
			}
		}
		offline = offline || off == 1
	}
	righe.Close()

	var diff []differenzaScheda
	for k := range differenze(prima, dopo) {
		if !campoControllato(k) || !(tutti || segnati[k]) {
			continue
		}
		diff = append(diff, differenzaScheda{Campo: k, Prima: prima[k], Dopo: dopo[k], Offline: segnatiOffline[k] || tuttiOffline})
	}
	if len(diff) == 0 {
		segna()
		return ""
	}
	sort.Slice(diff, func(i, j int) bool { return diff[i].Campo < diff[j].Campo })
	bd, _ := json.Marshal(diff)
	id := cripto.ID()
	off := 0
	if offline {
		off = 1
	}
	ora := db.Ora()
	if apertoID != "" {
		s.DB.Exec(`UPDATE controlli_scheda SET stato = 'superato', deciso_il = ? WHERE id = ?`, ora, apertoID)
	}
	if _, err := s.DB.Exec(`INSERT INTO controlli_scheda(id, personaggio_id, utente_id, sessione_id, versione_da, versione_a, offline, differenze, creato_il)
		VALUES(?,?,?,?,?,?,?,?,?)`, id, pg, nullo(prop), nullo(sessione), vc, corr, off, string(bd), ora); err != nil {
		return ""
	}
	segna()
	s.Hub.AlMaster(Msg{Tipo: "scheda.controllo", Dati: s.vistaControllo(id)})
	return id
}

// controlloAllAccesso: il giocatore si collega o conferma la presenza; si controllano i personaggi con cui partecipa
// alle sessioni in corso.
func (s *Server) controlloAllAccesso(u *Utente) {
	if u == nil || u.Master() {
		return
	}
	righe, err := s.DB.Query(`SELECT p.sessione_id, p.personaggio_id FROM partecipanti p JOIN sessioni_gioco g ON g.id = p.sessione_id
		WHERE p.utente_id = ? AND g.stato = 'in_corso' AND p.personaggio_id IS NOT NULL`, u.ID)
	if err != nil {
		return
	}
	type coppia struct{ sess, pg string }
	var l []coppia
	for righe.Next() {
		var c coppia
		righe.Scan(&c.sess, &c.pg)
		l = append(l, c)
	}
	righe.Close()
	for _, c := range l {
		s.controllaScheda(c.pg, c.sess)
	}
}

// controlloAvvioSessione: il Master avvia la sessione; si controllano tutti i personaggi dei partecipanti.
func (s *Server) controlloAvvioSessione(sessione string) {
	righe, err := s.DB.Query(`SELECT personaggio_id FROM partecipanti WHERE sessione_id = ? AND personaggio_id IS NOT NULL`, sessione)
	if err != nil {
		return
	}
	var l []string
	for righe.Next() {
		var pg string
		righe.Scan(&pg)
		l = append(l, pg)
	}
	righe.Close()
	for _, pg := range l {
		s.controllaScheda(pg, sessione)
	}
}

func (s *Server) vistaControllo(id string) map[string]any {
	var pg, utente, sess, diff, stato, nome, giocatore string
	var da, a, off int
	var creato int64
	if s.DB.QueryRow(`SELECT c.personaggio_id, COALESCE(c.utente_id,''), COALESCE(c.sessione_id,''), c.differenze, c.stato, c.versione_da, c.versione_a,
		c.offline, c.creato_il, COALESCE(p.nome,''), COALESCE(u.nome,'')
		FROM controlli_scheda c LEFT JOIN personaggi p ON p.id = c.personaggio_id LEFT JOIN utenti u ON u.id = c.utente_id WHERE c.id = ?`, id).
		Scan(&pg, &utente, &sess, &diff, &stato, &da, &a, &off, &creato, &nome, &giocatore) != nil {
		return nil
	}
	var titolo string
	s.DB.QueryRow(`SELECT titolo FROM sessioni_gioco WHERE id = ?`, sess).Scan(&titolo)
	return map[string]any{"id": id, "personaggio": pg, "nome": nome, "giocatore": giocatore, "sessione": titolo,
		"da": da, "a": a, "offline": off == 1, "differenze": json.RawMessage(diff), "stato": stato, "creato": creato}
}

func (s *Server) rotteControlloSchede(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/controlli-schede", api(s.apiControlliSchede))
	mux.Handle("POST /api/v1/controlli-schede/{id}", api(s.apiDecidiControllo))
}

// apiControlliSchede: gli avvisi aperti e gli ultimi decisi (solo Master).
func (s *Server) apiControlliSchede(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	righe, err := s.DB.Query(`SELECT id FROM controlli_scheda WHERE stato = 'aperto'
		UNION ALL SELECT id FROM (SELECT id FROM controlli_scheda WHERE stato IN ('accettato','ripristinato') ORDER BY deciso_il DESC LIMIT 20)`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for righe.Next() {
		var id string
		righe.Scan(&id)
		ids = append(ids, id)
	}
	righe.Close()
	out := []map[string]any{}
	for _, id := range ids {
		if v := s.vistaControllo(id); v != nil {
			out = append(out, v)
		}
	}
	return map[string]any{"lista": out}, nil
}

// apiDecidiControllo: «accetta» (le modifiche restano) o «ripristina» (i campi segnalati tornano come prima).
func (s *Server) apiDecidiControllo(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Azione string `json:"azione"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Azione != "accetta" && in.Azione != "ripristina" {
		return nil, errDatiNonValidi
	}
	id := r.PathValue("id")
	var pg, utente, diffJSON, nome string
	if s.DB.QueryRow(`SELECT c.personaggio_id, COALESCE(c.utente_id,''), c.differenze, COALESCE(p.nome,'')
		FROM controlli_scheda c LEFT JOIN personaggi p ON p.id = c.personaggio_id WHERE c.id = ? AND c.stato = 'aperto'`, id).
		Scan(&pg, &utente, &diffJSON, &nome) != nil {
		return nil, errore(http.StatusConflict, "Avviso già deciso o non trovato.")
	}
	var diff []differenzaScheda
	json.Unmarshal([]byte(diffJSON), &diff)
	stato, testo := "accettato", "Modifiche alla scheda di "+nome+" accettate."
	var nuova int
	var campi []string
	var prop string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if in.Azione == "ripristina" {
			stato, testo = "ripristinato", "Il Master ha riportato alcuni campi della scheda di "+nome+" ai valori del controllo precedente."
			n, c, err := s.modificaSchedaNota(tx, pg, u, "ripristino dopo il controllo", func(p piatto) (bool, error) {
				for _, d := range diff {
					if len(d.Prima) == 0 || string(d.Prima) == "null" {
						delete(p, d.Campo)
					} else {
						p[d.Campo] = d.Prima
					}
				}
				return true, nil
			})
			if err != nil {
				return err
			}
			nuova, campi = n, c
			if n > 0 { // la versione del ripristino è già controllata
				if _, err := tx.Exec(`UPDATE personaggi SET versione_controllata = ? WHERE id = ? AND versione_controllata < ?`, n, pg, n); err != nil {
					return err
				}
			}
		}
		_, err := tx.Exec(`UPDATE controlli_scheda SET stato = ?, deciso_il = ? WHERE id = ?`, stato, db.Ora(), id)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, pg).Scan(&prop)
	if nuova > 0 {
		s.avvisaSchedaDa(pg, prop, nuova, u, campi, true)
		if utente != "" {
			s.Hub.AUtente(utente, Msg{Tipo: "scheda.controllo.esito", Dati: map[string]any{"personaggio": pg, "testo": testo}})
		}
	}
	s.Hub.AlMaster(Msg{Tipo: "scheda.controllo.deciso", Dati: map[string]any{"id": id, "stato": stato}})
	return map[string]any{"testo": testo, "stato": stato}, nil
}
