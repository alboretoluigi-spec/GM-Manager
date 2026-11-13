package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

func (s *Server) rotteUtenti(mux *http.ServeMux) {
	// Profilo e diritti del giocatore
	mux.Handle("GET /api/v1/profilo", api(s.apiProfilo))
	mux.Handle("PATCH /api/v1/profilo", api(s.apiProfiloModifica))
	mux.Handle("GET /api/v1/profilo/esporta", api(s.apiEsporta))
	mux.Handle("POST /api/v1/profilo/cancellazione", api(s.apiChiediCancellazione))
	// Pannello del Master
	mux.Handle("GET /api/v1/utenti", api(s.apiUtenti))
	mux.Handle("POST /api/v1/utenti/{id}/approva", api(s.apiApprova))
	mux.Handle("POST /api/v1/utenti/{id}/stato", api(s.apiCambiaStato))
	mux.Handle("PUT /api/v1/utenti/{id}/gruppi", api(s.apiSposta))
	mux.Handle("GET /api/v1/utenti/{id}/telefono", api(s.apiTelefono))
	mux.Handle("POST /api/v1/utenti/{id}/codice-reimpostazione", api(s.apiCodiceReimpostazione))
	mux.Handle("DELETE /api/v1/utenti/{id}", api(s.apiEliminaUtente))
	mux.Handle("GET /api/v1/richieste-privacy", api(s.apiRichiestePrivacy))
	mux.Handle("GET /api/v1/registro-dati", api(s.apiRegistroDati))
	// Inviti
	mux.Handle("GET /api/v1/inviti", api(s.apiInviti))
	mux.Handle("POST /api/v1/inviti", api(s.apiNuovoInvito))
	mux.Handle("DELETE /api/v1/inviti/{id}", api(s.apiRevocaInvito))
}

// ===== Profilo =====

func (s *Server) apiProfilo(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "qualsiasi")
	if err != nil {
		return nil, err
	}
	var mask sql.NullString
	var avvisi int
	var creato int64
	s.DB.QueryRow(`SELECT telefono_finale, avvisi_whatsapp, creato_il FROM utenti WHERE id = ?`, u.ID).Scan(&mask, &avvisi, &creato)
	tel := ""
	if !u.Master() {
		tel, _ = s.telefonoDi(u.ID) // il proprio numero si vede in chiaro
	}
	var richiesta int
	s.DB.QueryRow(`SELECT COUNT(*) FROM richieste_privacy WHERE utente_id = ? AND stato = 'aperta'`, u.ID).Scan(&richiesta)
	return map[string]any{"nome": u.Nome, "telefono": tel, "telefonoMascherato": mask.String, "avvisi": avvisi == 1,
		"stato": u.Stato, "creato": creato, "cancellazioneRichiesta": richiesta > 0}, nil
}

func (s *Server) apiProfiloModifica(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "qualsiasi")
	if err != nil {
		return nil, err
	}
	var in struct {
		Nome     *string
		Telefono *string
		Avvisi   *bool
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	ora := db.Ora()
	if in.Nome != nil {
		n, err := rigaSingola(*in.Nome, "Nome", 2, 40)
		if err != nil {
			return nil, err
		}
		s.DB.Exec(`UPDATE utenti SET nome = ?, modificato_il = ? WHERE id = ?`, n, ora, u.ID)
	}
	if in.Telefono != nil && !u.Master() {
		tel, err := NormalizzaTelefono(*in.Telefono)
		if err != nil {
			return nil, err
		}
		imp := s.Chiavi.Impronta(tel)
		var altro int
		s.DB.QueryRow(`SELECT COUNT(*) FROM utenti WHERE telefono_impronta = ? AND id <> ?`, imp, u.ID).Scan(&altro)
		if altro > 0 {
			return nil, errore(http.StatusConflict, "Questo numero è già usato da un altro account.")
		}
		s.DB.Exec(`UPDATE utenti SET telefono_cifrato = ?, telefono_impronta = ?, telefono_finale = ?, modificato_il = ? WHERE id = ?`,
			s.Chiavi.Cifra([]byte(tel), "telefono:"+u.ID), imp, MascheraTelefono(tel), ora, u.ID)
	}
	if in.Avvisi != nil {
		v := 0
		if *in.Avvisi {
			v = 1
		}
		s.DB.Exec(`UPDATE utenti SET avvisi_whatsapp = ?, modificato_il = ? WHERE id = ?`, v, ora, u.ID)
	}
	return s.apiProfilo(w, r)
}

// telefonoDi decifra il numero di un utente (uso interno: mai nei log).
func (s *Server) telefonoDi(id string) (string, error) {
	var cif []byte
	if err := s.DB.QueryRow(`SELECT telefono_cifrato FROM utenti WHERE id = ?`, id).Scan(&cif); err != nil || len(cif) == 0 {
		return "", err
	}
	b, err := s.Chiavi.Decifra(cif, "telefono:"+id)
	return string(b), err
}

// apiEsporta: tutti i dati del giocatore in JSON (account, consensi, personaggi con l'ultima versione).
func (s *Server) apiEsporta(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "qualsiasi")
	if err != nil {
		return nil, err
	}
	out, err := s.datiPersonali(u.ID)
	if err != nil {
		return nil, err
	}
	s.registroDati(u.ID, "esportazione_propri_dati", u.ID)
	w.Header().Set("Content-Disposition", `attachment; filename="gasterbless-miei-dati.json"`)
	return out, nil
}

func (s *Server) datiPersonali(id string) (map[string]any, error) {
	var nome, ruolo, stato string
	var avvisi int
	var creato int64
	var ultimo sql.NullInt64
	if err := s.DB.QueryRow(`SELECT nome, ruolo, stato, avvisi_whatsapp, creato_il, ultimo_accesso FROM utenti WHERE id = ?`, id).
		Scan(&nome, &ruolo, &stato, &avvisi, &creato, &ultimo); err != nil {
		return nil, errNonTrovato
	}
	tel, _ := s.telefonoDi(id)
	account := map[string]any{"nome": nome, "telefono": tel, "ruolo": ruolo, "stato": stato, "avvisiWhatsApp": avvisi == 1,
		"registrato": time.Unix(creato, 0).Format(time.RFC3339)}
	if ultimo.Valid {
		account["ultimoAccesso"] = time.Unix(ultimo.Int64, 0).Format(time.RFC3339)
	}
	var consensi []map[string]any
	rows, _ := s.DB.Query(`SELECT versione, accettato_il FROM consensi WHERE utente_id = ? ORDER BY accettato_il`, id)
	for rows != nil && rows.Next() {
		var v string
		var t int64
		rows.Scan(&v, &t)
		consensi = append(consensi, map[string]any{"versioneInformativa": v, "accettato": time.Unix(t, 0).Format(time.RFC3339)})
	}
	if rows != nil {
		rows.Close()
	}
	var gruppi []string
	rows, _ = s.DB.Query(`SELECT g.nome FROM iscrizioni i JOIN gruppi g ON g.id = i.gruppo_id WHERE i.utente_id = ?`, id)
	for rows != nil && rows.Next() {
		var g string
		rows.Scan(&g)
		gruppi = append(gruppi, g)
	}
	if rows != nil {
		rows.Close()
	}
	var pgs []map[string]any
	rows, _ = s.DB.Query(`SELECT p.id, p.nome, p.stato, COALESCE(v.dati, '{}') FROM personaggi p
		LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.proprietario_id = ?`, id)
	for rows != nil && rows.Next() {
		var pid, pn, ps, dati string
		rows.Scan(&pid, &pn, &ps, &dati)
		pgs = append(pgs, map[string]any{"id": pid, "nome": pn, "stato": ps, "scheda": json.RawMessage(dati)})
	}
	if rows != nil {
		rows.Close()
	}
	return map[string]any{"esportato": time.Now().Format(time.RFC3339), "account": account, "consensi": consensi,
		"gruppi": gruppi, "personaggi": pgs}, nil
}

func (s *Server) apiChiediCancellazione(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "qualsiasi")
	if err != nil {
		return nil, err
	}
	if u.Master() {
		return nil, errore(http.StatusBadRequest, "L'account del Master non si cancella da qui.")
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM richieste_privacy WHERE utente_id = ? AND stato = 'aperta'`, u.ID).Scan(&n)
	if n == 0 {
		s.DB.Exec(`INSERT INTO richieste_privacy(id, utente_id, nome, tipo, creato_il) VALUES(?,?,?,'cancellazione',?)`, cripto.ID(), u.ID, u.Nome, db.Ora())
		s.Hub.AlMaster(Msg{Tipo: "privacy.richiesta", Dati: map[string]any{"nome": u.Nome}})
	}
	return nil, nil
}

// ===== Pannello del Master =====

type vistaUtente struct {
	ID         string   `json:"id"`
	Nome       string   `json:"nome"`
	Telefono   string   `json:"telefono"` // sempre mascherato in questo elenco
	Stato      string   `json:"stato"`
	Avvisi     bool     `json:"avvisi"`
	Creato     int64    `json:"creato"`
	Ultimo     int64    `json:"ultimoAccesso"`
	Gruppi     []string `json:"gruppi"`
	Segretario []string `json:"segretario"`
	Proposto   string   `json:"gruppoProposto,omitempty"`
	Inattivo   bool     `json:"inattivo"`
}

func (s *Server) richiediMaster(r *http.Request) (*Utente, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	if s.Cfg.PannelloSoloPC && !u.locale {
		return nil, errore(http.StatusForbidden, "Il pannello del Master è raggiungibile solo dal PC su cui gira Gaster Bless.")
	}
	return u, nil
}

func (s *Server) apiUtenti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	limiteInattivi := db.Ora() - int64(s.Cfg.InattiviMesi)*30*86400
	rows, err := s.DB.Query(`SELECT id, nome, COALESCE(telefono_finale,''), stato, avvisi_whatsapp, creato_il, COALESCE(ultimo_accesso, 0)
		FROM utenti WHERE ruolo = 'giocatore' ORDER BY stato = 'in_attesa' DESC, creato_il DESC`)
	if err != nil {
		return nil, err
	}
	var out []*vistaUtente
	idx := map[string]*vistaUtente{}
	for rows.Next() {
		v := &vistaUtente{Gruppi: []string{}, Segretario: []string{}}
		var a int
		rows.Scan(&v.ID, &v.Nome, &v.Telefono, &v.Stato, &a, &v.Creato, &v.Ultimo)
		v.Avvisi = a == 1
		rif := v.Ultimo
		if rif == 0 {
			rif = v.Creato
		}
		v.Inattivo = v.Stato == "approvato" && rif < limiteInattivi
		out = append(out, v)
		idx[v.ID] = v
	}
	rows.Close()
	rows, _ = s.DB.Query(`SELECT utente_id, gruppo_id, segretario FROM iscrizioni`)
	for rows != nil && rows.Next() {
		var uid, g string
		var sg int
		rows.Scan(&uid, &g, &sg)
		if v := idx[uid]; v != nil {
			v.Gruppi = append(v.Gruppi, g)
			if sg == 1 {
				v.Segretario = append(v.Segretario, g)
			}
		}
	}
	if rows != nil {
		rows.Close()
	}
	rows, _ = s.DB.Query(`SELECT chiave, valore FROM impostazioni WHERE chiave LIKE 'gruppo_proposto:%'`)
	for rows != nil && rows.Next() {
		var k, g string
		rows.Scan(&k, &g)
		if v := idx[strings.TrimPrefix(k, "gruppo_proposto:")]; v != nil {
			v.Proposto = g
		}
	}
	if rows != nil {
		rows.Close()
	}
	if out == nil {
		out = []*vistaUtente{}
	}
	return out, nil
}

func (s *Server) giocatore(id string) (stato string, err error) {
	err = s.DB.QueryRow(`SELECT stato FROM utenti WHERE id = ? AND ruolo = 'giocatore'`, id).Scan(&stato)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errNonTrovato
	}
	return stato, err
}

// impostaGruppi sostituisce le iscrizioni di un utente (mantenendo il ruolo di Segretario dove resta).
func (s *Server) impostaGruppi(tx *sql.Tx, id string, gruppi []string, segretario []string) error {
	seg := map[string]bool{}
	for _, g := range segretario {
		seg[g] = true
	}
	if _, err := tx.Exec(`DELETE FROM iscrizioni WHERE utente_id = ?`, id); err != nil {
		return err
	}
	visti := map[string]bool{}
	for _, g := range gruppi {
		if visti[g] {
			continue
		}
		visti[g] = true
		var n int
		tx.QueryRow(`SELECT COUNT(*) FROM gruppi WHERE id = ? AND archiviato = 0`, g).Scan(&n)
		if n == 0 {
			return errore(http.StatusBadRequest, "Gruppo inesistente.")
		}
		sv := 0
		if seg[g] {
			sv = 1
		}
		if _, err := tx.Exec(`INSERT INTO iscrizioni(utente_id, gruppo_id, segretario, dal) VALUES(?,?,?,?)`, id, g, sv, db.Ora()); err != nil {
			return err
		}
		// Chi entra in un gruppo partecipa anche alle sue sessioni non ancora concluse (il Master può
		// sempre toglierlo da "Partecipanti…").
		if _, err := tx.Exec(`INSERT OR IGNORE INTO partecipanti(sessione_id, utente_id)
			SELECT id, ? FROM sessioni_gioco WHERE gruppo_id = ? AND stato <> 'conclusa'`, id, g); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) apiApprova(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var in struct{ Gruppi []string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if len(in.Gruppi) == 0 {
		return nil, errore(http.StatusBadRequest, "Scegli almeno un gruppo.")
	}
	if _, err := s.giocatore(id); err != nil {
		return nil, err
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if err := s.impostaGruppi(tx, id, in.Gruppi, nil); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE utenti SET stato = 'approvato', modificato_il = ? WHERE id = ?`, db.Ora(), id)
		if err == nil {
			_, err = tx.Exec(`DELETE FROM impostazioni WHERE chiave = ?`, "gruppo_proposto:"+id)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	s.evento("approvazione", m.ID, id)
	s.Hub.ChiudiUtente(id) // la pagina si ricollega con i nuovi permessi
	return nil, nil
}

// apiCambiaStato: rifiuta, blocca, sospendi, riattiva.
func (s *Server) apiCambiaStato(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var in struct{ Stato string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	attuale, err := s.giocatore(id)
	if err != nil {
		return nil, err
	}
	switch in.Stato {
	case "rifiutato", "bloccato", "sospeso":
	case "approvato":
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM iscrizioni WHERE utente_id = ?`, id).Scan(&n)
		if n == 0 {
			return nil, errore(http.StatusBadRequest, "Per riattivare un account usa «Approva» e scegli i gruppi.")
		}
	default:
		return nil, errDatiNonValidi
	}
	if attuale == in.Stato {
		return nil, nil
	}
	s.DB.Exec(`UPDATE utenti SET stato = ?, modificato_il = ? WHERE id = ?`, in.Stato, db.Ora(), id)
	if in.Stato != "approvato" {
		s.DB.Exec(`DELETE FROM sessioni_accesso WHERE utente_id = ?`, id)
		s.Hub.ChiudiUtente(id)
	}
	s.evento("stato_account", m.ID, id+" → "+in.Stato)
	return nil, nil
}

func (s *Server) apiSposta(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var in struct{ Gruppi, Segretario []string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if _, err := s.giocatore(id); err != nil {
		return nil, err
	}
	if err := db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error { return s.impostaGruppi(tx, id, in.Gruppi, in.Segretario) }); err != nil {
		return nil, err
	}
	s.Hub.ChiudiUtente(id)
	return nil, nil
}

// apiTelefono mostra il numero in chiaro al Master e lo annota nel registro degli accessi ai dati.
func (s *Server) apiTelefono(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if _, err := s.giocatore(id); err != nil {
		return nil, err
	}
	tel, err := s.telefonoDi(id)
	if err != nil {
		return nil, err
	}
	s.registroDati(m.ID, "telefono_visto", id)
	return map[string]string{"telefono": tel}, nil
}

// apiCodiceReimpostazione genera un codice monouso, valido 24 ore, da dare a voce al giocatore.
func (s *Server) apiCodiceReimpostazione(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if _, err := s.giocatore(id); err != nil {
		return nil, err
	}
	codice := codiceLeggibile(10)
	ora := db.Ora()
	s.DB.Exec(`UPDATE codici_reimpostazione SET usato_il = ? WHERE utente_id = ? AND usato_il IS NULL`, ora, id) // annulla i precedenti
	s.DB.Exec(`INSERT INTO codici_reimpostazione(id, utente_id, codice_hash, scade_il, creato_il) VALUES(?,?,?,?,?)`,
		cripto.ID(), id, cripto.HashToken(codice), ora+86400, ora)
	s.evento("codice_reimpostazione", m.ID, id)
	return map[string]any{"codice": codice[:5] + "-" + codice[5:], "scade": ora + 86400}, nil
}

// codiceLeggibile genera un codice senza caratteri ambigui (niente 0/O, 1/I/L).
func codiceLeggibile(n int) string {
	const alfabeto = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	b := cripto.Casuali(n)
	out := make([]byte, n)
	for i := range b {
		out[i] = alfabeto[int(b[i])%len(alfabeto)]
	}
	return string(out)
}

// apiEliminaUtente cancella l'account. "schede": "archivia" (anonime) oppure "elimina".
func (s *Server) apiEliminaUtente(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	schede := r.URL.Query().Get("schede")
	if schede != "archivia" && schede != "elimina" {
		return nil, errore(http.StatusBadRequest, "Scegli se archiviare in forma anonima o eliminare le schede.")
	}
	if _, err := s.giocatore(id); err != nil {
		return nil, err
	}
	var fileBg []string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT b.file FROM background b JOIN personaggi p ON p.id = b.personaggio_id WHERE p.proprietario_id = ?`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var f string
			rows.Scan(&f)
			fileBg = append(fileBg, f)
		}
		rows.Close()
		// Il background è un dato personale: si elimina in ogni caso.
		if _, err := tx.Exec(`DELETE FROM background WHERE personaggio_id IN (SELECT id FROM personaggi WHERE proprietario_id = ?)`, id); err != nil {
			return err
		}
		if schede == "elimina" {
			if _, err := tx.Exec(`DELETE FROM personaggi WHERE proprietario_id = ?`, id); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(`UPDATE personaggi SET stato = 'anonimo', proprietario_id = NULL, modificato_il = ? WHERE proprietario_id = ?`, db.Ora(), id); err != nil {
				return err
			}
			// Le versioni della scheda non devono più indicare l'autore.
			if _, err := tx.Exec(`UPDATE schede_versioni SET autore_id = NULL WHERE autore_id = ?`, id); err != nil {
				return err
			}
		}
		// Riassunti: con le schede eliminate spariscono anche loro; altrimenti restano senza il nome.
		if schede == "elimina" {
			tx.Exec(`DELETE FROM riassunti WHERE autore_id = ?`, id)
		} else {
			tx.Exec(`UPDATE riassunti SET autore_id = NULL, autore_nome = 'Scriba anonimo' WHERE autore_id = ?`, id)
		}
		tx.Exec(`UPDATE richieste_privacy SET stato = 'eseguita', chiuso_il = ?, utente_id = NULL, nome = '(cancellato)' WHERE utente_id = ?`, db.Ora(), id)
		tx.Exec(`DELETE FROM impostazioni WHERE chiave = ?`, "gruppo_proposto:"+id)
		tx.Exec(`UPDATE registro_eventi SET utente_id = NULL WHERE utente_id = ?`, id)
		_, err = tx.Exec(`DELETE FROM utenti WHERE id = ?`, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	for _, f := range fileBg {
		s.eliminaFileDati(f)
	}
	s.Hub.ChiudiUtente(id)
	s.registroDati(m.ID, "cancellazione_account_"+schede, "")
	return nil, nil
}

func (s *Server) apiRichiestePrivacy(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT id, COALESCE(utente_id,''), nome, tipo, stato, creato_il FROM richieste_privacy ORDER BY creato_il DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, uid, nome, tipo, stato string
		var t int64
		rows.Scan(&id, &uid, &nome, &tipo, &stato, &t)
		out = append(out, map[string]any{"id": id, "utente": uid, "nome": nome, "tipo": tipo, "stato": stato, "creato": t})
	}
	return out, nil
}

func (s *Server) registroDati(attore, azione, interessato string) {
	s.DB.Exec(`INSERT INTO registro_dati_personali(attore_id, azione, utente_interessato, creato_il) VALUES(?,?,?,?)`,
		nullo(attore), azione, nullo(interessato), db.Ora())
}

func (s *Server) apiRegistroDati(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT r.azione, COALESCE(u.nome, ''), r.creato_il FROM registro_dati_personali r
		LEFT JOIN utenti u ON u.id = r.utente_interessato ORDER BY r.id DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var a, n string
		var t int64
		rows.Scan(&a, &n, &t)
		out = append(out, map[string]any{"azione": a, "interessato": n, "quando": t})
	}
	return out, nil
}

// ===== Inviti =====

func (s *Server) apiInviti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT id, COALESCE(gruppo_id,''), nota, COALESCE(scade_il,0), COALESCE(usi_max,0), usi, revocato, creato_il FROM inviti ORDER BY creato_il DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, g, nota string
		var scade, max, usi, rev, creato int64
		rows.Scan(&id, &g, &nota, &scade, &max, &usi, &rev, &creato)
		out = append(out, map[string]any{"id": id, "gruppo": g, "nota": nota, "scade": scade, "usiMax": max, "usi": usi, "revocato": rev == 1, "creato": creato})
	}
	return out, nil
}

// apiNuovoInvito crea un codice d'invito. Il codice in chiaro si vede solo ora: nel database c'è l'hash.
func (s *Server) apiNuovoInvito(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	var in struct {
		Gruppo string
		Nota   string
		Giorni int
		UsiMax int
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	nota, err := rigaSingola(in.Nota, "Nota", 0, 80)
	if err != nil {
		return nil, err
	}
	if in.Gruppo != "" {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM gruppi WHERE id = ?`, in.Gruppo).Scan(&n)
		if n == 0 {
			return nil, errore(http.StatusBadRequest, "Gruppo inesistente.")
		}
	}
	codice := codiceLeggibile(12)
	ora := db.Ora()
	var scade, max any
	if in.Giorni > 0 {
		scade = ora + int64(in.Giorni)*86400
	}
	if in.UsiMax > 0 {
		max = in.UsiMax
	}
	id := cripto.ID()
	if _, err := s.DB.Exec(`INSERT INTO inviti(id, codice_hash, gruppo_id, nota, scade_il, usi_max, creato_il) VALUES(?,?,?,?,?,?,?)`,
		id, cripto.HashToken(codice), nullo(in.Gruppo), nota, scade, max, ora); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "codice": codice}, nil
}

func (s *Server) apiRevocaInvito(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	s.DB.Exec(`UPDATE inviti SET revocato = 1 WHERE id = ?`, r.PathValue("id"))
	return nil, nil
}

// apiInvitoInfo dice se un codice d'invito è valido (per la pagina di registrazione).
func (s *Server) apiInvitoInfo(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := s.limiteControlla("ip:" + ipStringa(r)); err != nil {
		return nil, err
	}
	g, err := s.usaInvito(r.PathValue("codice"), false)
	if err != nil {
		s.limiteFallito("ip:" + ipStringa(r))
		return nil, err
	}
	nome := ""
	if g != "" {
		s.DB.QueryRow(`SELECT nome FROM gruppi WHERE id = ?`, g).Scan(&nome)
	}
	return map[string]any{"valido": true, "gruppo": nome}, nil
}

// usaInvito verifica un codice; se conta è true ne incrementa l'uso. Restituisce il gruppo proposto.
func (s *Server) usaInvito(codice string, conta bool) (string, error) {
	codice = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(codice), "-", ""))
	var id, g string
	var scade, max sql.NullInt64
	var usi, rev int64
	err := s.DB.QueryRow(`SELECT id, COALESCE(gruppo_id,''), scade_il, usi_max, usi, revocato FROM inviti WHERE codice_hash = ?`,
		cripto.HashToken(codice)).Scan(&id, &g, &scade, &max, &usi, &rev)
	if err != nil || rev == 1 || (scade.Valid && scade.Int64 < db.Ora()) || (max.Valid && usi >= max.Int64) {
		return "", errore(http.StatusBadRequest, "Il link d'invito non è valido o è scaduto. Chiedine uno nuovo al Master.")
	}
	if conta {
		s.DB.Exec(`UPDATE inviti SET usi = usi + 1 WHERE id = ?`, id)
	}
	return g, nil
}
