package server

// «Modifica manuale» della scheda (richiesta dell'utente, 2026-11-11).
//
// Con l'approvazione attiva (impostazione «approvazione_schede», di serie sì) un giocatore cambia da solo solo i campi
// di gioco della sua scheda: PF, PF temporanei, tiri contro la morte, slot usati, ispirazione, indebolimento
// (campiInGioco), incantesimi preparati, equipaggiamento/sintonia/attivazione/cariche degli oggetti magici e le monete
// (che hanno già la loro richiesta al Master, finanze.go). Tutti gli altri campi li modifica attivando «Modifica
// manuale» nella pagina della scheda: le modifiche restano sul suo dispositivo e, quando spegne il tasto, arrivano al
// Master come una richiesta con il prima/dopo di ogni campo. Il Master approva o rifiuta voce per voce; le voci
// approvate diventano una nuova versione della scheda (scritta dal Master, quindi senza segni per il controllo
// all'accesso in sessione). Finché il Master non decide, la scheda valida resta quella di prima.
//
// Una sola richiesta in attesa per personaggio: un nuovo invio si unisce a quella aperta (il «prima» resta quello
// della prima richiesta; un campo riportato al valore attuale esce dalla richiesta).
// Il Master non passa mai dall'approvazione. Le schede alternative (plugin) e scheda esterna scrivono con le loro regole.

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

// Campi (prima parte del percorso) che il giocatore cambia da solo anche con l'approvazione attiva, oltre a campiInGioco.
// «v» ed «eqAuto» li scrive la scheda da sé; «san» è un campo storico (la Sanità mentale vive nelle sue tabelle).
// «armor» e «shield» (armatura e scudo indossati) e lo stato dell'elenco degli oggetti «inv» sono equipaggiamento (2026-11-11).
// resRip: tipo della Resilienza immonda del warlock, che si cambia dopo ogni riposo (difese del 2026-11-12)
var campiLiberiGiocatore = map[string]bool{"v": true, "eqAuto": true, "san": true, "spp": true, "coins": true, "armor": true, "shield": true, "resRip": true}

// campoDiGioco: il giocatore lo può cambiare senza approvazione («mag» solo se cambia lo stato degli oggetti: magSoloStato).
func campoDiGioco(k string) bool {
	if campoInGioco(k) || k == "f.hdUsati" { // Dadi Vita usati: si spendono giocando (riposi.go)
		return true
	}
	testa, _, _ := strings.Cut(k, ".")
	return campiLiberiGiocatore[testa]
}

// approvazioneSchede: vale l'impostazione del Master; se non l'ha mai scelta, il valore del file di configurazione.
func (s *Server) approvazioneSchede() bool {
	v, _ := s.impostazione("approvazione_schede")
	if v == "" {
		return s.Cfg.ApprovazioneSchede
	}
	return v != "no"
}

// Stato di un oggetto magico che il giocatore cambia giocando (web/scheda/oggetti_ui.js: {o, eq, sint, on, car}).
var statoOggettoMagico = map[string]bool{"eq": true, "sint": true, "on": true, "car": true}

func vociMagiche(v json.RawMessage) ([]map[string]json.RawMessage, bool) {
	if len(bytes.TrimSpace(v)) == 0 || string(bytes.TrimSpace(v)) == "null" {
		return nil, true
	}
	var l []map[string]json.RawMessage
	if json.Unmarshal(v, &l) != nil {
		return nil, false
	}
	return l, true
}

// magSoloStato: stessi oggetti, nello stesso ordine e con gli stessi dati; cambiano solo equipaggiato, sintonia,
// attivazione o cariche.
func magSoloStato(a, b json.RawMessage) bool {
	la, ok1 := vociMagiche(a)
	lb, ok2 := vociMagiche(b)
	if !ok1 || !ok2 || len(la) != len(lb) {
		return false
	}
	for i := range la {
		for k := range la[i] {
			if _, ok := lb[i][k]; !ok && !statoOggettoMagico[k] {
				return false
			}
		}
		for k, v := range lb[i] {
			if statoOggettoMagico[k] {
				continue
			}
			if !uguale(compatta(la[i][k]), compatta(v)) {
				return false
			}
		}
	}
	return true
}

// invSoloStato: elenco degli oggetti della scheda con gli stessi oggetti; cambia solo «equipaggiato» (eq).
func invSoloStato(a, b json.RawMessage) bool {
	la, ok1 := vociMagiche(a)
	lb, ok2 := vociMagiche(b)
	if !ok1 || !ok2 || len(la) != len(lb) {
		return false
	}
	for i := range la {
		for k := range la[i] {
			if _, ok := lb[i][k]; !ok && k != "eq" {
				return false
			}
		}
		for k, v := range lb[i] {
			if k != "eq" && !uguale(compatta(la[i][k]), compatta(v)) {
				return false
			}
		}
	}
	return true
}

// idOggettoMagico: l'id dell'oggetto di una voce di «mag» ("" se manca).
func idOggettoMagico(voce map[string]json.RawMessage) string {
	var o struct {
		ID string `json:"id"`
	}
	json.Unmarshal(voce["o"], &o)
	return o.ID
}

// magConStatoAttuale: applicando una richiesta su «mag», gli oggetti già presenti tengono lo stato di adesso
// (il giocatore può averli equipaggiati o aver usato cariche mentre aspettava il Master).
func magConStatoAttuale(nuovo, attuale json.RawMessage) json.RawMessage {
	ln, ok1 := vociMagiche(nuovo)
	la, ok2 := vociMagiche(attuale)
	if !ok1 || !ok2 || ln == nil {
		return nuovo
	}
	stato := map[string]map[string]json.RawMessage{}
	for _, v := range la {
		if id := idOggettoMagico(v); id != "" {
			stato[id] = v
		}
	}
	for i, v := range ln {
		if vecchio, ok := stato[idOggettoMagico(v)]; ok && idOggettoMagico(v) != "" {
			for k := range statoOggettoMagico {
				if x, ok := vecchio[k]; ok {
					ln[i][k] = x
				} else {
					delete(ln[i], k)
				}
			}
		}
	}
	b, err := json.Marshal(ln)
	if err != nil {
		return nuovo
	}
	return compatta(b)
}

// filtraNonDiGioco toglie da un salvataggio del giocatore i campi che richiedono l'approvazione e li restituisce.
func filtraNonDiGioco(mod map[string]json.RawMessage, rimossi []string, attuale piatto) (ignorati, restoRimossi []string) {
	for k, v := range mod {
		if campoDiGioco(k) || k == "mag" && magSoloStato(attuale["mag"], v) || k == "inv" && invSoloStato(attuale["inv"], v) {
			continue
		}
		if uguale(attuale[k], v) {
			continue
		}
		ignorati = append(ignorati, k)
		delete(mod, k)
	}
	restoRimossi = rimossi[:0]
	for _, k := range rimossi {
		if campoDiGioco(k) {
			restoRimossi = append(restoRimossi, k)
			continue
		}
		if _, c := attuale[k]; c {
			ignorati = append(ignorati, k)
		}
	}
	sort.Strings(ignorati)
	return ignorati, restoRimossi
}

type voceRichiesta struct {
	Campo   string          `json:"campo"`
	Prima   json.RawMessage `json:"prima"`
	Dopo    json.RawMessage `json:"dopo"`
	Rimosso bool            `json:"rimosso,omitempty"`
	Esito   string          `json:"esito,omitempty"` // solo negli esiti: approvata o rifiutata
}

func (s *Server) rotteModificheManuali(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/personaggi/{id}/richiesta-modifiche", api(s.apiRichiestaModifiche))
	mux.Handle("POST /api/v1/personaggi/{id}/richiesta-modifiche", api(s.apiInviaRichiestaModifiche))
	mux.Handle("DELETE /api/v1/personaggi/{id}/richiesta-modifiche", api(s.apiRitiraRichiestaModifiche))
	mux.Handle("GET /api/v1/richieste-schede", api(s.apiRichiesteSchede))
	mux.Handle("POST /api/v1/richieste-schede/{id}", api(s.apiDecidiRichiestaScheda))
}

func (s *Server) vistaRichiestaScheda(id string) map[string]any {
	var pg, utente, voci, esiti, stato, nome, giocatore, decisoDa string
	var creato, aggiornato int64
	var deciso sql.NullInt64
	if s.DB.QueryRow(`SELECT r.personaggio_id, COALESCE(r.utente_id,''), r.voci, r.esiti, r.stato, r.creato_il, r.aggiornato_il, r.deciso_il,
		COALESCE(d.nome,''), COALESCE(p.nome,''), COALESCE(u.nome,'')
		FROM richieste_scheda r LEFT JOIN personaggi p ON p.id = r.personaggio_id LEFT JOIN utenti u ON u.id = r.utente_id
		LEFT JOIN utenti d ON d.id = r.deciso_da WHERE r.id = ?`, id).
		Scan(&pg, &utente, &voci, &esiti, &stato, &creato, &aggiornato, &deciso, &decisoDa, &nome, &giocatore) != nil {
		return nil
	}
	v := map[string]any{"id": id, "personaggio": pg, "nome": nome, "giocatore": giocatore, "voci": json.RawMessage(voci),
		"esiti": json.RawMessage(esiti), "stato": stato, "creato": creato, "aggiornato": aggiornato, "decisoDa": decisoDa}
	if deciso.Valid {
		v["deciso"] = deciso.Int64
	}
	return v
}

// richiestaAperta: id e voci della richiesta in attesa del personaggio ("" se non ce n'è).
func richiestaAperta(q interface {
	QueryRow(string, ...any) *sql.Row
}, pg string) (string, []voceRichiesta) {
	var id, voci string
	if q.QueryRow(`SELECT id, voci FROM richieste_scheda WHERE personaggio_id = ? AND stato = 'in_attesa' ORDER BY creato_il DESC LIMIT 1`, pg).Scan(&id, &voci) != nil {
		return "", nil
	}
	var l []voceRichiesta
	json.Unmarshal([]byte(voci), &l)
	return id, l
}

// apiRichiestaModifiche: la richiesta in attesa del personaggio (per l'avviso «in attesa» nella scheda) e se
// l'approvazione è attiva per chi guarda.
func (s *Server) apiRichiestaModifiche(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, pg) {
		return nil, errNonTrovato
	}
	out := map[string]any{"approvazione": s.approvazioneSchede() && !u.Master(), "richiesta": nil}
	if id, _ := richiestaAperta(s.DB, pg); id != "" {
		out["richiesta"] = s.vistaRichiestaScheda(id)
	}
	return out, nil
}

// registraRichiesta unisce le modifiche proposte alla richiesta aperta (o ne crea una) e restituisce l'id e le voci.
func (s *Server) registraRichiesta(tx *sql.Tx, pg string, u *Utente, mod map[string]json.RawMessage, rimossi []string) (string, int, error) {
	var dati string
	if err := tx.QueryRow(`SELECT v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, pg).Scan(&dati); err != nil {
		return "", 0, errNonTrovato
	}
	attuale, err := leggiPiatto(dati)
	if err != nil {
		return "", 0, err
	}
	id, voci := richiestaAperta(tx, pg)
	perCampo := map[string]voceRichiesta{}
	for _, v := range voci {
		perCampo[v.Campo] = v
	}
	proponi := func(k string, dopo json.RawMessage, rimosso bool) {
		if campoInGioco(k) || strings.HasPrefix(k, "coins.") || k == "coins" { // hanno le loro strade (stato in gioco, finanze)
			return
		}
		prima, c := attuale[k]
		if rimosso && !c || !rimosso && c && uguale(prima, dopo) { // torna come adesso: esce dalla richiesta
			delete(perCampo, k)
			return
		}
		v, gia := perCampo[k]
		if !gia {
			v = voceRichiesta{Campo: k, Prima: prima}
		}
		v.Dopo, v.Rimosso = dopo, rimosso
		if rimosso {
			v.Dopo = nil
		}
		perCampo[k] = v
	}
	for k, v := range mod {
		proponi(k, compatta(v), false)
	}
	for _, k := range rimossi {
		proponi(k, nil, true)
	}
	l := make([]voceRichiesta, 0, len(perCampo))
	for _, v := range perCampo {
		l = append(l, v)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].Campo < l[j].Campo })
	ora := db.Ora()
	if len(l) == 0 {
		if id != "" {
			_, err = tx.Exec(`UPDATE richieste_scheda SET stato = 'annullata', voci = '[]', aggiornato_il = ? WHERE id = ?`, ora, id)
		}
		return "", 0, err
	}
	b, _ := json.Marshal(l)
	if id == "" {
		id = cripto.ID()
		_, err = tx.Exec(`INSERT INTO richieste_scheda(id, personaggio_id, utente_id, voci, creato_il, aggiornato_il) VALUES(?,?,?,?,?,?)`, id, pg, nullo(u.ID), string(b), ora, ora)
	} else {
		_, err = tx.Exec(`UPDATE richieste_scheda SET voci = ?, utente_id = ?, aggiornato_il = ? WHERE id = ?`, string(b), nullo(u.ID), ora, id)
	}
	return id, len(l), err
}

// apiInviaRichiestaModifiche: il giocatore spegne «Modifica manuale»; le modifiche fatte arrivano al Master.
func (s *Server) apiInviaRichiestaModifiche(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("id")
	if !s.puoModificarePersonaggio(u, pg) {
		if s.puoLeggerePersonaggio(u, pg) {
			return nil, errore(http.StatusForbidden, "Questo personaggio non è più attivo: la scheda si può solo consultare.")
		}
		return nil, errNonTrovato
	}
	if u.Master() {
		return nil, errore(http.StatusBadRequest, "Il Master modifica le schede direttamente, senza richiesta.")
	}
	var in struct {
		Modifiche map[string]json.RawMessage
		Rimossi   []string
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
	}
	for _, k := range in.Rimossi {
		if !campoValido(k) {
			return nil, errDatiNonValidi
		}
	}
	var id string
	var n int
	if err := db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		var e error
		id, n, e = s.registraRichiesta(tx, pg, u, in.Modifiche, in.Rimossi)
		return e
	}); err != nil {
		return nil, err
	}
	if id == "" {
		s.Hub.AlMaster(Msg{Tipo: "scheda.richiesta.ritirata", Dati: map[string]any{"personaggio": pg}})
		return map[string]any{"richiesta": nil, "testo": "Nessuna modifica da approvare: la scheda è uguale a quella valida."}, nil
	}
	vista := s.vistaRichiestaScheda(id)
	s.Hub.AlMaster(Msg{Tipo: "scheda.richiesta", Dati: vista})
	s.evento("schede", u.ID, "richiesta di modifica manuale: "+strconv.Itoa(n)+" campi")
	return map[string]any{"richiesta": vista, "testo": "Modifiche inviate al Master: valgono quando le approva."}, nil
}

// apiRitiraRichiestaModifiche: il giocatore ritira la sua richiesta ancora in attesa.
func (s *Server) apiRitiraRichiestaModifiche(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("id")
	if !s.puoLeggerePersonaggio(u, pg) {
		return nil, errNonTrovato
	}
	id, _ := richiestaAperta(s.DB, pg)
	if id == "" {
		return nil, errore(http.StatusConflict, "Nessuna richiesta in attesa.")
	}
	s.DB.Exec(`UPDATE richieste_scheda SET stato = 'annullata', aggiornato_il = ? WHERE id = ? AND stato = 'in_attesa'`, db.Ora(), id)
	s.Hub.AlMaster(Msg{Tipo: "scheda.richiesta.ritirata", Dati: map[string]any{"id": id, "personaggio": pg}})
	return map[string]any{"testo": "Richiesta ritirata."}, nil
}

// apiRichiesteSchede: le richieste in attesa e le ultime decise (solo Master).
func (s *Server) apiRichiesteSchede(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	righe, err := s.DB.Query(`SELECT id FROM richieste_scheda WHERE stato = 'in_attesa'
		UNION ALL SELECT id FROM (SELECT id FROM richieste_scheda WHERE stato = 'decisa' ORDER BY deciso_il DESC LIMIT 20)`)
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
		if v := s.vistaRichiestaScheda(id); v != nil {
			out = append(out, v)
		}
	}
	return map[string]any{"lista": out, "approvazione": s.approvazioneSchede()}, nil
}

// apiDecidiRichiestaScheda: il Master approva e rifiuta voce per voce. Le voci non indicate restano in attesa.
func (s *Server) apiDecidiRichiestaScheda(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Approvati []string
		Rifiutati []string
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	decisione := map[string]string{}
	for _, k := range in.Approvati {
		decisione[k] = "approvata"
	}
	for _, k := range in.Rifiutati {
		if decisione[k] != "" {
			return nil, errore(http.StatusBadRequest, "Una voce non può essere insieme approvata e rifiutata.")
		}
		decisione[k] = "rifiutata"
	}
	if len(decisione) == 0 {
		return nil, errore(http.StatusBadRequest, "Scegli almeno una voce da approvare o rifiutare.")
	}
	id := r.PathValue("id")
	var pg, utente, vociJSON, esitiJSON, nome, giocatore string
	if s.DB.QueryRow(`SELECT r.personaggio_id, COALESCE(r.utente_id,''), r.voci, r.esiti, COALESCE(p.nome,''), COALESCE(u.nome,'')
		FROM richieste_scheda r LEFT JOIN personaggi p ON p.id = r.personaggio_id LEFT JOIN utenti u ON u.id = r.utente_id
		WHERE r.id = ? AND r.stato = 'in_attesa'`, id).Scan(&pg, &utente, &vociJSON, &esitiJSON, &nome, &giocatore) != nil {
		return nil, errore(http.StatusConflict, "Richiesta già decisa, ritirata o non trovata.")
	}
	var voci, esiti []voceRichiesta
	json.Unmarshal([]byte(vociJSON), &voci)
	json.Unmarshal([]byte(esitiJSON), &esiti)
	var restano, approvate []voceRichiesta
	nRif := 0
	for _, v := range voci {
		switch decisione[v.Campo] {
		case "approvata":
			approvate = append(approvate, v)
			v.Esito = "approvata"
			esiti = append(esiti, v)
		case "rifiutata":
			nRif++
			v.Esito = "rifiutata"
			esiti = append(esiti, v)
		default:
			restano = append(restano, v)
		}
	}
	if len(approvate)+nRif == 0 {
		return nil, errore(http.StatusConflict, "Le voci indicate non sono più nella richiesta: riapri il report.")
	}
	var nuova int
	var campi []string
	var prop string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if len(approvate) > 0 {
			nota := "modifiche manuali approvate"
			if giocatore != "" {
				nota += " (chieste da " + giocatore + ")"
			}
			n, c, err := s.modificaSchedaNota(tx, pg, u, nota, func(p piatto) (bool, error) {
				for _, v := range approvate {
					if v.Rimosso {
						delete(p, v.Campo)
						continue
					}
					if v.Campo == "mag" {
						p[v.Campo] = magConStatoAttuale(v.Dopo, p["mag"])
						continue
					}
					p[v.Campo] = v.Dopo
				}
				return true, nil
			})
			if err != nil {
				return err
			}
			nuova, campi = n, c
		}
		bv, _ := json.Marshal(restano)
		be, _ := json.Marshal(esiti)
		stato := "in_attesa"
		if len(restano) == 0 {
			bv, stato = []byte("[]"), "decisa"
		}
		ora := db.Ora()
		_, err := tx.Exec(`UPDATE richieste_scheda SET voci = ?, esiti = ?, stato = ?, aggiornato_il = ?, deciso_il = ?, deciso_da = ? WHERE id = ?`,
			string(bv), string(be), stato, ora, ora, u.ID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, pg).Scan(&prop)
	if nuova > 0 {
		s.avvisaSchedaDa(pg, prop, nuova, u, campi, true)
	}
	conta := func(n int, uno, tanti string) string {
		if n == 1 {
			return "1 " + uno
		}
		return strconv.Itoa(n) + " " + tanti
	}
	testo := "Scheda di " + nome + ": " + conta(len(approvate), "modifica approvata", "modifiche approvate") + ", " + conta(nRif, "rifiutata", "rifiutate")
	if len(restano) > 0 {
		testo += ", " + conta(len(restano), "ancora in attesa", "ancora in attesa")
	}
	testo += "."
	vista := s.vistaRichiestaScheda(id)
	if utente != "" {
		var decise []voceRichiesta
		for _, v := range esiti[len(esiti)-len(approvate)-nRif:] {
			decise = append(decise, v)
		}
		s.Hub.AUtente(utente, Msg{Tipo: "scheda.richiesta.esito", Dati: map[string]any{"personaggio": pg, "nome": nome, "testo": testo, "voci": decise}})
	}
	s.Hub.AlMaster(Msg{Tipo: "scheda.richiesta.decisa", Dati: vista})
	return map[string]any{"testo": testo, "richiesta": vista, "versione": nuova}, nil
}

// richiestaDaRipristino: con l'approvazione attiva, il ripristino di una vecchia versione chiesto da un giocatore
// diventa una richiesta al Master con le differenze (solo i campi che richiedono l'approvazione).
func (s *Server) richiestaDaRipristino(r *http.Request, u *Utente, pg string, n int) (any, error) {
	var datiVecchi, datiOra string
	if s.DB.QueryRow(`SELECT dati FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, pg, n).Scan(&datiVecchi) != nil {
		return nil, errNonTrovato
	}
	s.DB.QueryRow(`SELECT v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, pg).Scan(&datiOra)
	vecchia, err1 := leggiPiatto(datiVecchi)
	ora, err2 := leggiPiatto(datiOra)
	if err1 != nil || err2 != nil {
		return nil, errDatiNonValidi
	}
	mod := map[string]json.RawMessage{}
	var rimossi []string
	for k := range differenze(ora, vecchia) {
		if campoDiGioco(k) {
			continue
		}
		if v, c := vecchia[k]; c {
			mod[k] = v
		} else {
			rimossi = append(rimossi, k)
		}
	}
	var id string
	if err := db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		var e error
		id, _, e = s.registraRichiesta(tx, pg, u, mod, rimossi)
		return e
	}); err != nil {
		return nil, err
	}
	if id == "" {
		return map[string]any{"richiesta": nil, "testo": "La versione scelta non cambia nulla che richieda il Master."}, nil
	}
	vista := s.vistaRichiestaScheda(id)
	s.Hub.AlMaster(Msg{Tipo: "scheda.richiesta", Dati: vista})
	return map[string]any{"richiesta": vista, "testo": "Ripristino inviato al Master come richiesta: vale quando lo approva."}, nil
}
