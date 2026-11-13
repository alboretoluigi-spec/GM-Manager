package server

// Avvisi ai giocatori via WhatsApp, inviati DAL MASTER con i link wa.me (nessuna libreria non ufficiale,
// nessun invio automatico): il server prepara il testo personalizzato per ogni partecipante e il link;
// il Master apre WhatsApp, invia, e segna "inviato" o "saltato". Resta il registro di ogni avviso.
// Il modulo per la Cloud API di Meta NON è attivo (richiede account business e costi: vedi
// docs/FASE_2_PROGETTO.md, sezione 12): l'interfaccia ProviderNotifiche è il punto in cui aggiungerlo.
//
// Anche il file calendario (.ics) della sessione è qui.

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

// ProviderNotifiche: oggi c'è solo l'invio guidato con wa.me (il Master conferma ogni messaggio).
type ProviderNotifiche interface {
	Nome() string
}

const modelloPredefinito = "Ciao {nome}! «{sessione}» sarà {data}, ore {ora}, presso {luogo}. {note}"

func (s *Server) rotteAvvisi(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/modelli-avviso", api(s.apiModelli))
	mux.Handle("PUT /api/v1/modelli-avviso", api(s.apiSalvaModelli))
	mux.Handle("POST /api/v1/sessioni/{id}/avvisi/anteprima", api(s.apiAnteprimaAvviso))
	mux.Handle("POST /api/v1/sessioni/{id}/avvisi", api(s.apiCreaAvviso))
	mux.Handle("GET /api/v1/sessioni/{id}/avvisi", api(s.apiAvvisiSessione))
	mux.Handle("PATCH /api/v1/avvisi/{id}/destinatari/{u}", api(s.apiSegnaAvviso))
	mux.Handle("GET /api/v1/sessioni/{id}/calendario.ics", http.HandlerFunc(s.calendario))
	mux.Handle("GET /api/v1/promemoria", api(s.apiPromemoria))
}

// ===== Modelli =====

type modello struct {
	ID    string `json:"id"`
	Nome  string `json:"nome"`
	Testo string `json:"testo"`
}

func (s *Server) modelli() []modello {
	out := []modello{}
	rows, err := s.DB.Query(`SELECT id, nome, testo FROM modelli_avviso ORDER BY nome`)
	if err == nil {
		for rows.Next() {
			var m modello
			rows.Scan(&m.ID, &m.Nome, &m.Testo)
			out = append(out, m)
		}
		rows.Close()
	}
	if len(out) == 0 {
		out = append(out, modello{ID: "predefinito", Nome: "Promemoria della sessione", Testo: modelloPredefinito})
	}
	return out
}

func (s *Server) apiModelli(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	return s.modelli(), nil
}

func (s *Server) apiSalvaModelli(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	var in []modello
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if len(in) > 20 {
		return nil, errore(http.StatusBadRequest, "Al massimo 20 modelli.")
	}
	err := db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		tx.Exec(`DELETE FROM modelli_avviso`)
		for _, m := range in {
			nome, err := rigaSingola(m.Nome, "Nome del modello", 1, 60)
			if err != nil {
				return err
			}
			t, err := testo(m.Testo, "Testo del modello", 1, 1000)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO modelli_avviso(id, nome, testo) VALUES(?,?,?)`, cripto.ID(), nome, t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.modelli(), nil
}

// ===== Testo personalizzato =====

var giorniIT = []string{"domenica", "lunedì", "martedì", "mercoledì", "giovedì", "venerdì", "sabato"}
var mesiIT = []string{"gennaio", "febbraio", "marzo", "aprile", "maggio", "giugno", "luglio", "agosto", "settembre", "ottobre", "novembre", "dicembre"}

func dataLungaIT(t time.Time) string {
	return fmt.Sprintf("%s %d %s %d", giorniIT[t.Weekday()], t.Day(), mesiIT[t.Month()-1], t.Year())
}

type datiSessioneAvviso struct {
	ID, Titolo, Luogo, Note, Gruppo string
	Inizio                          time.Time
}

func (s *Server) sessionePerAvviso(id string) (*datiSessioneAvviso, error) {
	d := &datiSessioneAvviso{ID: id}
	var inizio int64
	err := s.DB.QueryRow(`SELECT x.titolo, x.luogo, x.note, x.inizio_il, g.nome FROM sessioni_gioco x JOIN gruppi g ON g.id = x.gruppo_id WHERE x.id = ?`, id).
		Scan(&d.Titolo, &d.Luogo, &d.Note, &inizio, &d.Gruppo)
	if err != nil {
		return nil, errNonTrovato
	}
	d.Inizio = time.Unix(inizio, 0)
	return d, nil
}

// componi sostituisce i segnaposto; quelli vuoti spariscono senza lasciare spazi doppi.
func componi(modello, nome string, d *datiSessioneAvviso) string {
	luogo := d.Luogo
	if luogo == "" {
		luogo = "il solito posto"
	}
	t := strings.NewReplacer("{nome}", nome, "{sessione}", d.Titolo, "{data}", dataLungaIT(d.Inizio),
		"{ora}", d.Inizio.Format("15:04"), "{luogo}", luogo, "{note}", d.Note, "{gruppo}", d.Gruppo).Replace(modello)
	t = strings.Join(strings.Fields(strings.ReplaceAll(t, "\n", " \n ")), " ")
	return strings.TrimSpace(strings.ReplaceAll(t, " \n ", "\n"))
}

// linkWhatsApp: wa.me con il numero (solo cifre, senza +) e il testo già scritto.
func linkWhatsApp(e164, testo string) string {
	num := strings.TrimPrefix(e164, "+")
	if num == "" {
		return "https://wa.me/?text=" + url.QueryEscape(testo)
	}
	return "https://wa.me/" + num + "?text=" + url.QueryEscape(testo)
}

type destinatario struct {
	Utente   string `json:"utente"`
	Nome     string `json:"nome"`
	Telefono string `json:"telefono"` // mascherato
	Testo    string `json:"testo"`
	Link     string `json:"link,omitempty"`
	Stato    string `json:"stato,omitempty"`
	Motivo   string `json:"motivo,omitempty"` // perché non si può inviare
}

// destinatari: i partecipanti della sessione (giocatori), con il testo personalizzato.
func (s *Server) destinatari(sessione, modello string, conLink bool, master string) ([]destinatario, error) {
	d, err := s.sessionePerAvviso(sessione)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT u.id, u.nome, COALESCE(u.telefono_finale,''), u.avvisi_whatsapp, u.telefono_cifrato IS NOT NULL
		FROM partecipanti p JOIN utenti u ON u.id = p.utente_id WHERE p.sessione_id = ? AND u.ruolo = 'giocatore' AND u.stato = 'approvato' ORDER BY u.nome`, sessione)
	if err != nil {
		return nil, err
	}
	var out []destinatario
	for rows.Next() {
		var x destinatario
		var av int
		var tel bool
		rows.Scan(&x.Utente, &x.Nome, &x.Telefono, &av, &tel)
		x.Testo = componi(modello, x.Nome, d)
		if !tel {
			x.Motivo = "senza numero"
		} else if av == 0 {
			x.Motivo = "ha disattivato gli avvisi WhatsApp"
		}
		out = append(out, x)
	}
	rows.Close()
	if conLink {
		for i := range out {
			if out[i].Motivo != "" {
				continue
			}
			tel, err := s.telefonoDi(out[i].Utente)
			if err != nil || tel == "" {
				out[i].Motivo = "numero non leggibile"
				continue
			}
			out[i].Link = linkWhatsApp(tel, out[i].Testo)
			s.registroDati(master, "telefono_usato_per_avviso", out[i].Utente)
		}
	}
	return out, nil
}

func (s *Server) apiAnteprimaAvviso(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	var in struct{ Testo string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	t, err := testo(in.Testo, "Testo", 1, 1000)
	if err != nil {
		return nil, err
	}
	dd, err := s.destinatari(r.PathValue("id"), t, false, "")
	if err != nil {
		return nil, err
	}
	d, _ := s.sessionePerAvviso(r.PathValue("id"))
	return map[string]any{"destinatari": dd, "gruppo": linkWhatsApp("", componi(strings.ReplaceAll(t, "{nome}", "a tutti"), "", d))}, nil
}

// apiCreaAvviso registra l'avviso e restituisce i link wa.me da aprire uno per uno.
func (s *Server) apiCreaAvviso(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	var in struct{ Testo string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	t, err := testo(in.Testo, "Testo", 1, 1000)
	if err != nil {
		return nil, err
	}
	sid := r.PathValue("id")
	dd, err := s.destinatari(sid, t, true, m.ID)
	if err != nil {
		return nil, err
	}
	id := cripto.ID()
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO avvisi(id, sessione_id, testo_modello, creato_da, creato_il) VALUES(?,?,?,?,?)`, id, sid, t, m.ID, db.Ora()); err != nil {
			return err
		}
		for _, x := range dd {
			stato := "da_inviare"
			if x.Link == "" {
				stato = "saltato"
			}
			if _, err := tx.Exec(`INSERT INTO avvisi_destinatari(avviso_id, utente_id, testo, stato) VALUES(?,?,?,?)`, id, x.Utente, x.Testo, stato); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for i := range dd {
		dd[i].Stato = map[bool]string{true: "da_inviare", false: "saltato"}[dd[i].Link != ""]
	}
	return map[string]any{"id": id, "destinatari": dd}, nil
}

// apiAvvisiSessione: gli avvisi già preparati per la sessione, con lo stato per destinatario.
func (s *Server) apiAvvisiSessione(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT id, testo_modello, creato_il FROM avvisi WHERE sessione_id = ? ORDER BY creato_il DESC LIMIT 20`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	type avviso struct {
		ID          string         `json:"id"`
		Testo       string         `json:"testo"`
		Quando      int64          `json:"quando"`
		Destinatari []destinatario `json:"destinatari"`
	}
	var out []*avviso
	for rows.Next() {
		a := &avviso{}
		rows.Scan(&a.ID, &a.Testo, &a.Quando)
		out = append(out, a)
	}
	rows.Close()
	for _, a := range out {
		rr, err := s.DB.Query(`SELECT d.utente_id, u.nome, COALESCE(u.telefono_finale,''), d.testo, d.stato FROM avvisi_destinatari d JOIN utenti u ON u.id = d.utente_id WHERE d.avviso_id = ? ORDER BY u.nome`, a.ID)
		if err != nil {
			continue
		}
		for rr.Next() {
			var x destinatario
			rr.Scan(&x.Utente, &x.Nome, &x.Telefono, &x.Testo, &x.Stato)
			a.Destinatari = append(a.Destinatari, x)
		}
		rr.Close()
		for i, x := range a.Destinatari { // i link si ricostruiscono solo per chi deve ancora riceverlo
			if x.Stato == "da_inviare" {
				if tel, err := s.telefonoDi(x.Utente); err == nil && tel != "" {
					a.Destinatari[i].Link = linkWhatsApp(tel, x.Testo)
					s.registroDati(m.ID, "telefono_usato_per_avviso", x.Utente)
				}
			}
		}
	}
	if out == nil {
		out = []*avviso{}
	}
	return out, nil
}

func (s *Server) apiSegnaAvviso(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	var in struct{ Stato string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Stato != "inviato" && in.Stato != "saltato" && in.Stato != "da_inviare" {
		return nil, errDatiNonValidi
	}
	res, _ := s.DB.Exec(`UPDATE avvisi_destinatari SET stato = ?, segnato_il = ? WHERE avviso_id = ? AND utente_id = ?`, in.Stato, db.Ora(), r.PathValue("id"), r.PathValue("u"))
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errNonTrovato
	}
	return nil, nil
}

// apiPromemoria: sessioni di domani senza avviso ancora preparato (banner per il Master).
func (s *Server) apiPromemoria(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	ora := time.Now()
	domani := time.Date(ora.Year(), ora.Month(), ora.Day()+1, 0, 0, 0, 0, time.Local)
	rows, err := s.DB.Query(`SELECT x.id, x.titolo, x.inizio_il FROM sessioni_gioco x JOIN gruppi g ON g.id = x.gruppo_id
		WHERE x.stato = 'pianificata' AND g.archiviato = 0 AND x.inizio_il >= ? AND x.inizio_il < ?
		AND NOT EXISTS (SELECT 1 FROM avvisi a WHERE a.sessione_id = x.id AND a.creato_il > ?) ORDER BY x.inizio_il`,
		domani.Unix(), domani.AddDate(0, 0, 1).Unix(), ora.Add(-48*time.Hour).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, t string
		var i int64
		rows.Scan(&id, &t, &i)
		out = append(out, map[string]any{"id": id, "titolo": t, "inizio": i})
	}
	return out, nil
}

// ===== Calendario =====

func icsTesto(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`, "\r", "").Replace(s)
}

// calendario: file .ics della sessione, per aggiungerla al calendario del telefono.
func (s *Server) calendario(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	id := r.PathValue("id")
	var gruppo string
	if s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, id).Scan(&gruppo) != nil || !s.membro(u, gruppo) {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	d, err := s.sessionePerAvviso(id)
	if err != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	f := "20060102T150405Z"
	var b strings.Builder
	riga := func(x string) { b.WriteString(x + "\r\n") }
	riga("BEGIN:VCALENDAR")
	riga("VERSION:2.0")
	riga("PRODID:-//GasterBless//Sessioni//IT")
	riga("BEGIN:VEVENT")
	riga("UID:" + id + "@gasterbless")
	riga("DTSTAMP:" + time.Now().UTC().Format(f))
	riga("DTSTART:" + d.Inizio.UTC().Format(f))
	riga("DTEND:" + d.Inizio.Add(4*time.Hour).UTC().Format(f))
	riga("SUMMARY:" + icsTesto(d.Titolo+" · "+d.Gruppo))
	if d.Luogo != "" {
		riga("LOCATION:" + icsTesto(d.Luogo))
	}
	if d.Note != "" {
		riga("DESCRIPTION:" + icsTesto(d.Note))
	}
	riga("BEGIN:VALARM")
	riga("TRIGGER:-PT2H")
	riga("ACTION:DISPLAY")
	riga("DESCRIPTION:" + icsTesto(d.Titolo))
	riga("END:VALARM")
	riga("END:VEVENT")
	riga("END:VCALENDAR")
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="sessione.ics"`)
	w.Write([]byte(b.String()))
}
