package server

// Aspetto dei dadi 3D (richiesta dell'utente, 2026-11-12; migrazione 035). Ogni utente sceglie il colore del suo set
// («#corpo/#numeri»): il server lo mette in ogni tiro, così tutti vedono i dadi di chi tira nel suo colore. Nel tiro va
// anche la classe del personaggio di chi tira (la principale, per livelli), che decide il piccolo effetto grafico del
// d20 (web/ui/dadi3d.js).

import (
	"net/http"
	"regexp"
	"strings"

	"gasterbless/internal/db"
)

var reColoreDadi = regexp.MustCompile(`^#[0-9a-f]{6}/#[0-9a-f]{6}$`)

func (s *Server) rotteAspettoDadi(mux *http.ServeMux) {
	mux.Handle("PUT /api/v1/io/dadi", api(s.apiColoreDadi))
}

func (s *Server) coloreDadi(utente string) string {
	var c string
	s.DB.QueryRow(`SELECT colore_dadi FROM utenti WHERE id = ?`, utente).Scan(&c)
	return c
}

// apiColoreDadi: PUT /io/dadi {colore: "#corpo/#numeri" | ""}
func (s *Server) apiColoreDadi(w http.ResponseWriter, r *http.Request) (any, error) {
	u := utenteDa(r.Context())
	if u == nil || !(u.Master() || u.Approvato()) {
		return nil, errore(http.StatusForbidden, "Accesso non consentito.")
	}
	var in struct {
		Colore string `json:"colore"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	in.Colore = strings.ToLower(strings.TrimSpace(in.Colore))
	if in.Colore != "" && !reColoreDadi.MatchString(in.Colore) {
		return nil, errDatiNonValidi
	}
	s.DB.Exec(`UPDATE utenti SET colore_dadi = ?, modificato_il = ? WHERE id = ?`, in.Colore, db.Ora(), u.ID)
	return map[string]any{"colore": in.Colore}, nil
}

// classeDi: la classe principale (quella con più livelli) di un personaggio, in minuscolo («mago»), o "".
func (s *Server) classeDi(pg string) string {
	if pg == "" {
		return ""
	}
	rp, _ := s.datiPG(pg)
	best, lv := "", 0
	for _, c := range leggiClassi(rp.Classe, rp.Livello) {
		if c.Livello > lv {
			best, lv = c.Nome, c.Livello
		}
	}
	return best
}

// decoraTiro: colore dei dadi di chi tira e classe del suo personaggio (quello indicato, o quello scelto per la
// sessione, o l'unico personaggio attivo del giocatore).
func (s *Server) decoraTiro(t *tiroDadi, utente, personaggio, sessione string) {
	t.Colore = s.coloreDadi(utente)
	if personaggio == "" && sessione != "" {
		s.DB.QueryRow(`SELECT COALESCE(personaggio_id,'') FROM partecipanti WHERE sessione_id = ? AND utente_id = ?`, sessione, utente).Scan(&personaggio)
	}
	if personaggio == "" {
		var ids []string
		if rows, err := s.DB.Query(`SELECT id FROM personaggi WHERE proprietario_id = ? AND stato = 'attivo' LIMIT 2`, utente); err == nil {
			for rows.Next() {
				var id string
				rows.Scan(&id)
				ids = append(ids, id)
			}
			rows.Close()
		}
		if len(ids) == 1 {
			personaggio = ids[0]
		}
	}
	t.Classe = s.classeDi(personaggio)
}

// aspettoTiratore: colore e classe per i dadi 3D di un combattente: un PG con il set del suo giocatore e la sua classe, un
// mostro con il set di chi lo muove (il Master), senza classe.
func (s *Server) aspettoTiratore(c *vistaCombattente, u *Utente) (string, string) {
	if c.Tipo == "pg" && c.Proprietario != "" {
		return s.coloreDadi(c.Proprietario), s.classeDi(c.Personaggio)
	}
	return s.coloreDadi(u.ID), ""
}
