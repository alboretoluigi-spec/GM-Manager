package server

// Background in testo (richiesta dell'utente, 2026-11-09).
// Il giocatore (o il Master al suo posto: il background resta del giocatore) carica un file di testo, Word o PDF; la
// pagina ne estrae il testo con una formattazione leggera (# titoli, - elenchi, paragrafi) e lo pubblica nella sezione
// Background. Lo vedono il giocatore e il Master; il giocatore può condividerlo con altri giocatori dei suoi gruppi o
// con tutto il gruppo del personaggio.
//
// Per il Master: la «Scheda per il Master» con sintesi, agganci per l'antagonista e opportunità dalle ultime sessioni, scritta
// dall'assistente IA e aggiornata da sola (agganci_ia.go). Prima del 2026-11-09 era un'analisi a temi fissi.

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"gasterbless/internal/db"
)

const maxBackgroundTesto = 60000

func (s *Server) rotteBackgroundTesto(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/background-testi", api(s.apiBackgroundTesti))
	mux.Handle("PUT /api/v1/personaggi/{id}/background-testo", api(s.apiSalvaBackgroundTesto))
	mux.Handle("PATCH /api/v1/personaggi/{id}/background-testo", api(s.apiCondividiBackgroundTesto))
	mux.Handle("DELETE /api/v1/personaggi/{id}/background-testo", api(s.apiEliminaBackgroundTesto))
}

type pgBackground struct {
	id, nome, proprietario, propNome, gruppo string
}

func (s *Server) pgPerBackground(id string) (*pgBackground, bool) {
	p := &pgBackground{id: id}
	err := s.DB.QueryRow(`SELECT p.nome, COALESCE(p.proprietario_id,''), COALESCE(u.nome,''), COALESCE(p.gruppo_id,'')
		FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id WHERE p.id = ? AND p.stato != 'archiviato'`, id).
		Scan(&p.nome, &p.proprietario, &p.propNome, &p.gruppo)
	return p, err == nil
}

// puoModificare: il proprietario del personaggio o il Master (che carica «come se fosse il giocatore»).
func puoModificareBackground(u *Utente, p *pgBackground) bool {
	return u.Master() || p.proprietario == u.ID
}

// puoLeggere: proprietario, Master, chi è nell'elenco di condivisione o, con «tutti», i membri del gruppo del personaggio.
func (s *Server) puoLeggereBackground(u *Utente, p *pgBackground, condiviso []string) bool {
	if puoModificareBackground(u, p) || slices.Contains(condiviso, u.ID) {
		return true
	}
	return slices.Contains(condiviso, "tutti") && p.gruppo != "" && s.membro(u, p.gruppo)
}

func (s *Server) apiBackgroundTesti(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT b.personaggio_id, b.titolo, b.testo, b.origine, b.condiviso, b.modificato_il
		FROM background_testi b JOIN personaggi p ON p.id = b.personaggio_id WHERE p.stato != 'archiviato' ORDER BY p.nome`)
	if err != nil {
		return nil, err
	}
	type riga struct {
		pg, titolo, testo, origine, cond string
		mod                              int64
	}
	var tutte []riga
	for rows.Next() {
		var x riga
		rows.Scan(&x.pg, &x.titolo, &x.testo, &x.origine, &x.cond, &x.mod)
		tutte = append(tutte, x)
	}
	rows.Close()
	out := []map[string]any{}
	for _, x := range tutte {
		p, ok := s.pgPerBackground(x.pg)
		if !ok {
			continue
		}
		var cond []string
		json.Unmarshal([]byte(x.cond), &cond)
		if !s.puoLeggereBackground(u, p, cond) {
			continue
		}
		v := map[string]any{"personaggio": p.id, "nome": p.nome, "giocatore": p.propNome, "titolo": x.titolo, "testo": x.testo,
			"origine": x.origine, "modificato": x.mod, "mio": p.proprietario == u.ID, "modificabile": puoModificareBackground(u, p)}
		if puoModificareBackground(u, p) {
			v["condiviso"] = cond
		}
		if u.Master() {
			v["agganciIA"] = s.statoAgganci(p.id) // scritti dall'IA (agganci_ia.go)
		}
		out = append(out, v)
	}
	// Personaggi su cui l'utente può caricare un background (i suoi; per il Master quelli dei giocatori e i suoi PNG).
	q := `SELECT p.id, p.nome, COALESCE(u.nome,'') FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id
		WHERE p.stato = 'attivo' AND p.proprietario_id = ? ORDER BY p.nome`
	args := []any{u.ID}
	if u.Master() {
		q = `SELECT p.id, p.nome, COALESCE(u.nome,'') FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id
			WHERE p.stato = 'attivo' ORDER BY u.nome, p.nome`
		args = nil
	}
	pr, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer pr.Close()
	personaggi := []map[string]string{}
	for pr.Next() {
		var id, nome, gioc string
		pr.Scan(&id, &nome, &gioc)
		personaggi = append(personaggi, map[string]string{"id": id, "nome": nome, "giocatore": gioc})
	}
	risposta := map[string]any{"lista": out, "personaggi": personaggi}
	if u.Master() {
		// Appunti del Master sui personaggi (2026-11-09, assistente IA): sulla scheda del background, oppure a parte
		// per i personaggi che un background non ce l'hanno.
		conBg := map[string]bool{}
		for _, v := range out {
			conBg[v["personaggio"].(string)] = true
		}
		note := []map[string]any{}
		if nr, err := s.DB.Query(`SELECT n.personaggio_id, p.nome, n.testo, n.modificato_il FROM note_master_pg n
			JOIN personaggi p ON p.id = n.personaggio_id WHERE p.stato != 'archiviato' ORDER BY p.nome`); err == nil {
			for nr.Next() {
				var id, nome, t string
				var mod int64
				nr.Scan(&id, &nome, &t, &mod)
				for _, v := range out {
					if v["personaggio"] == id {
						v["noteMaster"] = t
					}
				}
				if !conBg[id] {
					note = append(note, map[string]any{"personaggio": id, "nome": nome, "testo": t, "modificato": mod})
				}
			}
			nr.Close()
		}
		risposta["noteSenzaBackground"] = note
	}
	return risposta, nil
}

func (s *Server) avvisaBackgroundTesto(p *pgBackground, cond []string) {
	m := Msg{Tipo: "background.aggiornato", Dati: map[string]string{"personaggio": p.id}}
	s.Hub.AlMaster(m)
	if p.proprietario != "" {
		s.Hub.AUtente(p.proprietario, m)
	}
	for _, c := range cond {
		if c == "tutti" && p.gruppo != "" {
			s.Hub.Pubblica("gruppo:"+p.gruppo, m)
		} else if c != "tutti" {
			s.Hub.AUtente(c, m)
		}
	}
}

func (s *Server) apiSalvaBackgroundTesto(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	p, ok := s.pgPerBackground(r.PathValue("id"))
	if !ok || !puoModificareBackground(u, p) {
		return nil, errNonTrovato
	}
	var in struct {
		Titolo  string `json:"titolo"`
		Testo   string `json:"testo"`
		Origine string `json:"origine"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	in.Testo = strings.ReplaceAll(strings.ReplaceAll(in.Testo, "\r\n", "\n"), "\r", "\n")
	if in.Titolo, err = rigaSingola(in.Titolo, "Titolo", 0, 120); err != nil {
		return nil, err
	}
	if in.Testo, err = testo(in.Testo, "Background", 1, maxBackgroundTesto); err != nil {
		return nil, err
	}
	if in.Origine, err = rigaSingola(in.Origine, "Nome del file", 0, 160); err != nil {
		return nil, err
	}
	ora := db.Ora()
	if _, err := s.DB.Exec(`INSERT INTO background_testi(personaggio_id, titolo, testo, origine, creato_il, modificato_il) VALUES(?,?,?,?,?,?)
		ON CONFLICT(personaggio_id) DO UPDATE SET titolo = excluded.titolo, testo = excluded.testo, origine = excluded.origine, modificato_il = excluded.modificato_il`,
		p.id, in.Titolo, in.Testo, in.Origine, ora, ora); err != nil {
		return nil, err
	}
	var cs string
	s.DB.QueryRow(`SELECT condiviso FROM background_testi WHERE personaggio_id = ?`, p.id).Scan(&cs)
	var cond []string
	json.Unmarshal([]byte(cs), &cond)
	s.avvisaBackgroundTesto(p, cond)
	s.programmaAgganci(p.id, false) // l'IA riscrive gli agganci per l'antagonista
	return nil, nil
}

// apiCondividiBackgroundTesto: il giocatore (o il Master) sceglie con chi condividerlo: ["tutti"] (il gruppo del
// personaggio) oppure alcuni giocatori dei gruppi del proprietario; [] = solo giocatore e Master.
func (s *Server) apiCondividiBackgroundTesto(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	p, ok := s.pgPerBackground(r.PathValue("id"))
	if !ok || !puoModificareBackground(u, p) {
		return nil, errNonTrovato
	}
	var in struct {
		Condiviso []string `json:"condiviso"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	var cond []string
	var prop *Utente
	if p.proprietario != "" {
		prop = s.utentePerID(p.proprietario)
	}
	for _, c := range in.Condiviso {
		if c == "tutti" {
			cond = []string{"tutti"}
			break
		}
		if slices.Contains(cond, c) || c == p.proprietario {
			continue
		}
		altro := s.utentePerID(c)
		if altro == nil || !altro.Approvato() {
			return nil, errore(http.StatusBadRequest, "Giocatore non valido.")
		}
		comune := false
		if prop != nil {
			for _, g := range s.gruppiDi(prop) {
				comune = comune || s.membro(altro, g)
			}
		}
		if !comune && !u.Master() {
			return nil, errore(http.StatusBadRequest, "Puoi condividerlo solo con i giocatori dei tuoi gruppi.")
		}
		cond = append(cond, c)
	}
	if cond == nil {
		cond = []string{}
	}
	b, _ := json.Marshal(cond)
	res, err := s.DB.Exec(`UPDATE background_testi SET condiviso = ? WHERE personaggio_id = ?`, string(b), p.id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errNonTrovato
	}
	s.avvisaBackgroundTesto(p, cond)
	return map[string]any{"condiviso": cond}, nil
}

func (s *Server) apiEliminaBackgroundTesto(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	p, ok := s.pgPerBackground(r.PathValue("id"))
	if !ok || !puoModificareBackground(u, p) {
		return nil, errNonTrovato
	}
	var cs string
	s.DB.QueryRow(`SELECT condiviso FROM background_testi WHERE personaggio_id = ?`, p.id).Scan(&cs)
	var cond []string
	json.Unmarshal([]byte(cs), &cond)
	if _, err := s.DB.Exec(`DELETE FROM background_testi WHERE personaggio_id = ?`, p.id); err != nil {
		return nil, err
	}
	s.avvisaBackgroundTesto(p, cond)
	return nil, nil
}

func accorcia(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n])) + "…"
}
