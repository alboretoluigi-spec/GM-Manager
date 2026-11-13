package server

// Professioni (richiesta dell'utente, 2026-11-07): raccolta e fabbricazione per chi ha competenza in uno strumento
// e lo strumento nell'equipaggiamento (lo calcola la scheda e lo manda nel riepilogo, "professioni").
//
// - Il catalogo è web/regole/professioni.json (lo legge anche la pagina). Avvelenatore: regolamento dei veleni del
//   DM (Helm Starang) con le regole ufficiali; erborista, alchimista, cuoco e artigiani: regole ufficiali; le voci
//   "proposta" sono regole della casa proposte, da confermare dal Master.
// - I tiri li fa il server; l'esito arriva al Master e al giocatore del personaggio.
// - Inventario della professione (ingredienti con scadenza in giorni di gioco, secrezioni, fiale, dosi) nelle
//   tabelle prof_*; monete e oggetti prodotti stanno nella scheda: il server crea una nuova versione della scheda.
// - Il Master vede e usa tutte le professioni, per qualsiasi personaggio; sblocca le formule e fa passare i giorni.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"gasterbless/internal/db"
)

type ingrDef struct {
	ID   string   `json:"id"`
	N    string   `json:"n"`
	Dove string   `json:"dove"`
	CD   int      `json:"cd"`
	Abil []string `json:"abil"`
}

type secrDef struct {
	ID   string `json:"id"`
	N    string `json:"n"`
	CD   int    `json:"cd"`
	Tana int    `json:"tana"`
}

type opzDef struct {
	ID      string `json:"id"`
	Effetto string `json:"effetto"`
}

type formDef struct {
	ID           string   `json:"id"`
	N            string   `json:"n"`
	Tempo        string   `json:"tempo"`
	CD           int      `json:"cd"`
	Pieno        float64  `json:"pieno"`
	Ridotto      *float64 `json:"ridotto"`
	Raccoglibili []string `json:"raccoglibili"`
	Secrezioni   []string `json:"secrezioni"`
	Opzionali    []opzDef `json:"opzionali"`
	Dosi         any      `json:"dosi"` // "BC" oppure un numero
	Bonus5       bool     `json:"bonus5"`
	Talento      bool     `json:"talento"`
	Conosciuta   bool     `json:"conosciuta"`
	Proposta     bool     `json:"proposta"`
	Prodotto     string   `json:"prodotto"`
	TS           string   `json:"ts"`
	Effetto      string   `json:"effetto"`
	Immuni       bool     `json:"immuni"`
}

type profDef struct {
	ID          string    `json:"-"`
	N           string    `json:"n"`
	Raccolta    bool      `json:"raccolta"`
	Estrazione  bool      `json:"estrazione"`
	Dosi        bool      `json:"dosi"`
	Cibo        bool      `json:"cibo"`
	Generico    bool      `json:"-"`
	Ingredienti []ingrDef `json:"ingredienti"`
	Secrezioni  []secrDef `json:"secrezioni"`
	Speciali    []ingrDef `json:"speciali"`
	Formule     []formDef `json:"formule"`
}

type catalogoProf struct {
	Professioni map[string]*profDef `json:"professioni"`
	Artigiani   []struct {
		ID string `json:"id"`
		N  string `json:"n"`
	} `json:"artigiani"`
}

var (
	profUna sync.Once
	profCat catalogoProf
)

func (s *Server) catalogoProfessioni() *catalogoProf {
	profUna.Do(func() {
		b, err := fs.ReadFile(s.Web, "regole/professioni.json")
		if err == nil {
			json.Unmarshal(b, &profCat)
		}
		if profCat.Professioni == nil {
			profCat.Professioni = map[string]*profDef{}
		}
		for id, p := range profCat.Professioni {
			p.ID = id
		}
		for _, a := range profCat.Artigiani {
			if _, c := profCat.Professioni[a.ID]; !c {
				profCat.Professioni[a.ID] = &profDef{ID: a.ID, N: a.N, Generico: true}
			}
		}
	})
	return &profCat
}

func (p *profDef) formula(id string) *formDef {
	for i := range p.Formule {
		if p.Formule[i].ID == id {
			return &p.Formule[i]
		}
	}
	return nil
}
func (p *profDef) ingrediente(id string) *ingrDef {
	for i := range p.Ingredienti {
		if p.Ingredienti[i].ID == id {
			return &p.Ingredienti[i]
		}
	}
	return nil
}
func (p *profDef) secrezione(id string) *secrDef {
	for i := range p.Secrezioni {
		if p.Secrezioni[i].ID == id {
			return &p.Secrezioni[i]
		}
	}
	return nil
}

// nomeVoce: il nome leggibile di un ingrediente, secrezione, speciale o formula.
func (p *profDef) nomeVoce(id string) string {
	if x := p.ingrediente(id); x != nil {
		return x.N
	}
	if x := p.secrezione(id); x != nil {
		return x.N
	}
	for _, x := range p.Speciali {
		if x.ID == id {
			return x.N
		}
	}
	if x := p.formula(id); x != nil {
		return x.N
	}
	if id == "vuota" {
		return "Fiala vuota"
	}
	return id
}

// ===== Contesto: chi può usare quale professione =====

type ctxProf struct {
	u    *Utente
	pg   string
	nome string
	prop string
	def  *profDef
	rp   riepilogo
	bc   int
}

func (c *ctxProf) mod(car string) int {
	v, ok := c.rp.Car[car]
	if !ok {
		v = 10
	}
	return Modificatore(v)
}
func (c *ctxProf) competente(abil string) bool { return slices.Contains(c.rp.Abilita, abil) }

// contestoProf: il Master usa tutte le professioni per qualsiasi personaggio; il giocatore solo per i suoi
// personaggi attivi che hanno la professione (competenza e strumento, secondo la scheda).
func (s *Server) contestoProf(r *http.Request, scrittura bool) (*ctxProf, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := r.PathValue("pg")
	if !s.puoLeggerePersonaggio(u, pg) || scrittura && !u.Master() && !s.puoModificarePersonaggio(u, pg) {
		return nil, errNonTrovato
	}
	def := s.catalogoProfessioni().Professioni[r.PathValue("prof")]
	if def == nil {
		return nil, errNonTrovato
	}
	rp, prop := s.datiPG(pg)
	if !u.Master() && !slices.Contains(rp.Professioni, def.ID) {
		return nil, errore(http.StatusForbidden, "Per questa professione servono la competenza e lo strumento nell'equipaggiamento.")
	}
	c := &ctxProf{u: u, pg: pg, prop: prop, def: def, rp: rp, bc: BonusCompetenza(rp.Livello)}
	s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, pg).Scan(&c.nome)
	return c, nil
}

// ===== Inventario =====

type voceInv struct {
	ID       int64  `json:"id"`
	Tipo     string `json:"tipo"`
	Voce     string `json:"voce"`
	Nome     string `json:"nome"`
	Variante string `json:"variante,omitempty"`
	Quantita int    `json:"quantita"`
	Giorni   *int   `json:"giorni,omitempty"`
}

func (s *Server) inventarioProf(pg string, def *profDef) []voceInv {
	out := []voceInv{}
	rows, err := s.DB.Query(`SELECT id, tipo, voce, variante, quantita, giorni FROM prof_inventario
		WHERE personaggio_id = ? AND professione = ? AND quantita > 0 ORDER BY tipo, voce, giorni`, pg, def.ID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var v voceInv
		var g sql.NullInt64
		rows.Scan(&v.ID, &v.Tipo, &v.Voce, &v.Variante, &v.Quantita, &g)
		if g.Valid {
			n := int(g.Int64)
			v.Giorni = &n
		}
		v.Nome = def.nomeVoce(v.Voce)
		out = append(out, v)
	}
	return out
}

// quanti: quantità disponibile di una voce (tutte le righe).
func quanti(tx *sql.Tx, pg, prof, tipo, voce string) int {
	var n int
	tx.QueryRow(`SELECT COALESCE(SUM(quantita),0) FROM prof_inventario WHERE personaggio_id = ? AND professione = ? AND tipo = ? AND voce = ? AND quantita > 0`,
		pg, prof, tipo, voce).Scan(&n)
	return n
}

// consuma toglie n unità, prima dalle righe che scadono prima.
func consuma(tx *sql.Tx, pg, prof, tipo, voce string, n int) error {
	rows, err := tx.Query(`SELECT id, quantita FROM prof_inventario WHERE personaggio_id = ? AND professione = ? AND tipo = ? AND voce = ? AND quantita > 0
		ORDER BY giorni IS NULL, giorni, id`, pg, prof, tipo, voce)
	if err != nil {
		return err
	}
	type riga struct {
		id int64
		q  int
	}
	var l []riga
	for rows.Next() {
		var x riga
		rows.Scan(&x.id, &x.q)
		l = append(l, x)
	}
	rows.Close()
	for _, x := range l {
		if n <= 0 {
			break
		}
		t := min(n, x.q)
		if _, err := tx.Exec(`UPDATE prof_inventario SET quantita = quantita - ? WHERE id = ?`, t, x.id); err != nil {
			return err
		}
		n -= t
	}
	if n > 0 {
		return errore(http.StatusConflict, "Inventario insufficiente.")
	}
	tx.Exec(`DELETE FROM prof_inventario WHERE personaggio_id = ? AND quantita <= 0`, pg)
	return nil
}

func aggiungiInv(tx *sql.Tx, pg, prof, tipo, voce, variante string, n int, giorni *int) error {
	_, err := tx.Exec(`INSERT INTO prof_inventario(personaggio_id, professione, tipo, voce, variante, quantita, giorni, creato_il) VALUES(?,?,?,?,?,?,?,?)`,
		pg, prof, tipo, voce, variante, n, giorni, db.Ora())
	return err
}

func setteGiorni() *int { n := 7; return &n }

// ===== Scheda: monete e oggetti =====

var valoreMoneta = map[string]int{"MR": 1, "MA": 10, "ME": 50, "MO": 100, "MP": 1000}

func leggiMoneta(p piatto, k string) int {
	v, ok := p["coins."+k]
	if !ok {
		return 0
	}
	var x any
	json.Unmarshal(v, &x)
	switch t := x.(type) {
	case float64:
		return max(0, int(t))
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return max(0, n)
	}
	return 0
}

func scriviMoneta(p piatto, k string, n int) {
	s := ""
	if n > 0 {
		s = strconv.Itoa(n)
	}
	b, _ := json.Marshal(s)
	p["coins."+k] = b
}

// totaleMonete in monete di rame.
func totaleMonete(p piatto) int {
	t := 0
	for k, v := range valoreMoneta {
		t += leggiMoneta(p, k) * v
	}
	return t
}

// paga toglie il costo (in rame): prima le monete piccole, il resto torna in mo, ma, mr.
func paga(p piatto, costo int) bool {
	c := map[string]int{}
	for k := range valoreMoneta {
		c[k] = leggiMoneta(p, k)
	}
	if totaleMonete(p) < costo {
		return false
	}
	rem := costo
	for _, k := range []string{"MR", "MA", "ME", "MO", "MP"} {
		if rem <= 0 {
			break
		}
		n := min(c[k], (rem+valoreMoneta[k]-1)/valoreMoneta[k])
		c[k] -= n
		rem -= n * valoreMoneta[k]
	}
	resto := -rem
	for _, k := range []string{"MO", "MA", "MR"} {
		n := resto / valoreMoneta[k]
		c[k] += n
		resto -= n * valoreMoneta[k]
	}
	for k, n := range c {
		scriviMoneta(p, k, n)
	}
	return true
}

// rimborsa aggiunge monete (in rame) come mo, ma, mr.
func rimborsa(p piatto, rame int) {
	for _, k := range []string{"MO", "MA", "MR"} {
		n := rame / valoreMoneta[k]
		if n > 0 {
			scriviMoneta(p, k, leggiMoneta(p, k)+n)
		}
		rame -= n * valoreMoneta[k]
	}
}

func aggiungiOggetto(p piatto, riga string) {
	var eq string
	json.Unmarshal(p["eq"], &eq)
	eq = strings.TrimRight(eq, " \n")
	if eq != "" {
		eq += "\n"
	}
	b, _ := json.Marshal(eq + riga)
	p["eq"] = b
}

func rame(mo float64) int { return int(math.Round(mo * 100)) }

func testoMo(rame int) string {
	if rame%100 == 0 {
		return strconv.Itoa(rame/100) + " mo"
	}
	return strings.Replace(strconv.FormatFloat(float64(rame)/100, 'f', 2, 64), ".", ",", 1) + " mo"
}

// moneteScheda: il totale delle monete (in rame) nella versione corrente della scheda.
func moneteScheda(tx *sql.Tx, pg string) int {
	var dati string
	tx.QueryRow(`SELECT v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, pg).Scan(&dati)
	p, err := leggiPiatto(dati)
	if err != nil {
		return 0
	}
	return totaleMonete(p)
}

// modificaScheda applica f ai dati della versione corrente e salva una nuova versione (nota "professione"),
// avvisando chi ha la scheda aperta. f restituisce false per non salvare.
func (s *Server) modificaScheda(tx *sql.Tx, pg string, u *Utente, f func(p piatto) (bool, error)) (int, []string, error) {
	return s.modificaSchedaNota(tx, pg, u, "professione", f)
}

// modificaSchedaNota: come modificaScheda, con la nota della versione scelta da chi chiama.
func (s *Server) modificaSchedaNota(tx *sql.Tx, pg string, u *Utente, nota string, f func(p piatto) (bool, error)) (int, []string, error) {
	var corr int
	if err := tx.QueryRow(`SELECT versione_corrente FROM personaggi WHERE id = ?`, pg).Scan(&corr); err != nil {
		return 0, nil, errNonTrovato
	}
	var dati, riep string
	if err := tx.QueryRow(`SELECT dati, riepilogo FROM schede_versioni WHERE personaggio_id = ? AND numero = ?`, pg, corr).Scan(&dati, &riep); err != nil {
		return 0, nil, err
	}
	p, err := leggiPiatto(dati)
	if err != nil {
		return 0, nil, err
	}
	prima := piatto{}
	for k, v := range p {
		prima[k] = v
	}
	ok, err := f(p)
	if err != nil || !ok {
		return 0, nil, err
	}
	cambiati := differenze(prima, p)
	if len(cambiati) == 0 {
		return 0, nil, nil
	}
	campi := make([]string, 0, len(cambiati))
	for k := range cambiati {
		campi = append(campi, k)
	}
	slices.Sort(campi)
	b, err := ricomponi(p)
	if err != nil {
		return 0, nil, err
	}
	cj, _ := json.Marshal(campi)
	ora := db.Ora()
	if _, err := tx.Exec(`INSERT INTO schede_versioni(personaggio_id, numero, dati, riepilogo, campi, autore_id, nota, creato_il) VALUES(?,?,?,?,?,?,?,?)`,
		pg, corr+1, string(b), riep, string(cj), u.ID, nota, ora); err != nil {
		return 0, nil, err
	}
	if _, err := tx.Exec(`UPDATE personaggi SET versione_corrente = ?, modificato_il = ? WHERE id = ?`, corr+1, ora, pg); err != nil {
		return 0, nil, err
	}
	return corr + 1, campi, nil
}

// ===== Tiri ed esiti =====

type tiroProf struct {
	Etichetta string `json:"etichetta"`
	D20       []int  `json:"d20"`
	Tenuto    int    `json:"tenuto"`
	Mod       int    `json:"mod"`
	Totale    int    `json:"totale"`
	CD        int    `json:"cd"`
	Riuscito  bool   `json:"riuscito"`
	Vantaggio bool   `json:"vantaggio,omitempty"`
}

func tiraProva(etichetta string, mod, cd int, vantaggio bool) tiroProf {
	t := tiroProf{Etichetta: etichetta, Mod: mod, CD: cd, Vantaggio: vantaggio}
	a := tiraDado(20)
	t.D20, t.Tenuto = []int{a}, a
	if vantaggio {
		b := tiraDado(20)
		t.D20 = append(t.D20, b)
		t.Tenuto = max(a, b)
	}
	t.Totale = t.Tenuto + mod
	t.Riuscito = t.Totale >= cd
	return t
}

// esito registra e manda il risultato al Master e al giocatore del personaggio.
func (s *Server) esitoProf(c *ctxProf, azione, testo string, tiro *tiroProf, extra map[string]any) map[string]any {
	s.DB.Exec(`INSERT INTO prof_registro(personaggio_id, attore_id, professione, azione, testo, creato_il) VALUES(?,?,?,?,?,?)`,
		c.pg, c.u.ID, c.def.ID, azione, testo, db.Ora())
	out := map[string]any{"personaggio": c.pg, "nome": c.nome, "professione": c.def.ID, "nomeProfessione": c.def.N, "azione": azione,
		"testo": testo, "da": c.u.Nome}
	if tiro != nil {
		out["tiro"] = tiro
	}
	for k, v := range extra {
		out[k] = v
	}
	m := Msg{Tipo: "professione.esito", Dati: out}
	s.Hub.AlMaster(m)
	if c.prop != "" && !(c.u.Master() && c.prop == c.u.ID) {
		s.Hub.AUtente(c.prop, m)
	}
	return out
}

// ===== Rotte =====

func (s *Server) rotteProfessioni(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/professioni", api(s.apiElencoProfessioni))
	mux.Handle("GET /api/v1/professioni/{pg}/{prof}", api(s.apiVistaProfessione))
	mux.Handle("POST /api/v1/professioni/{pg}/{prof}/raccolta", api(s.apiRaccolta))
	mux.Handle("POST /api/v1/professioni/{pg}/{prof}/tana", api(s.apiTana))
	mux.Handle("POST /api/v1/professioni/{pg}/{prof}/estrazione", api(s.apiEstrazione))
	mux.Handle("POST /api/v1/professioni/{pg}/{prof}/fiale", api(s.apiFiale))
	mux.Handle("POST /api/v1/professioni/{pg}/{prof}/fabbrica", api(s.apiFabbrica))
	mux.Handle("POST /api/v1/professioni/{pg}/{prof}/applica", api(s.apiApplicaDose))
	mux.Handle("PUT /api/v1/professioni/{pg}/{prof}/formule", api(s.apiFormulaConosciuta))
	mux.Handle("POST /api/v1/professioni/{pg}/{prof}/inventario", api(s.apiCorreggiInventario))
	mux.Handle("POST /api/v1/professioni-giorno", api(s.apiPassaGiorno))
}

// contaProfessioni: quante professioni hanno i personaggi attivi del giocatore.
func (s *Server) contaProfessioni(u *Utente) int {
	rows, err := s.DB.Query(`SELECT COALESCE(v.riepilogo,'{}') FROM personaggi p LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		WHERE p.proprietario_id = ? AND p.stato = 'attivo'`, u.ID)
	if err != nil {
		return 0
	}
	defer rows.Close()
	cat, n := s.catalogoProfessioni(), 0
	for rows.Next() {
		var riep string
		rows.Scan(&riep)
		var rp riepilogo
		json.Unmarshal([]byte(riep), &rp)
		for _, x := range rp.Professioni {
			if cat.Professioni[x] != nil {
				n++
			}
		}
	}
	return n
}

// apiElencoProfessioni: per il giocatore i suoi personaggi con almeno una professione; per il Master tutti i
// personaggi attivi con le loro professioni.
func (s *Server) apiElencoProfessioni(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	q := `SELECT p.id, p.nome, COALESCE(u.nome,''), COALESCE(v.riepilogo,'{}') FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id
		LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.stato = 'attivo'`
	args := []any{}
	if !u.Master() {
		q += ` AND p.proprietario_id = ?`
		args = append(args, u.ID)
	}
	rows, err := s.DB.Query(q+` ORDER BY p.nome`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cat := s.catalogoProfessioni()
	out := []map[string]any{}
	for rows.Next() {
		var id, nome, gioc, riep string
		rows.Scan(&id, &nome, &gioc, &riep)
		var rp riepilogo
		json.Unmarshal([]byte(riep), &rp)
		prof := []string{}
		for _, x := range rp.Professioni {
			if cat.Professioni[x] != nil {
				prof = append(prof, x)
			}
		}
		if len(prof) == 0 && !u.Master() {
			continue
		}
		out = append(out, map[string]any{"id": id, "nome": nome, "giocatore": gioc, "professioni": prof, "livello": rp.Livello})
	}
	return map[string]any{"personaggi": out, "master": u.Master()}, nil
}

func (s *Server) apiVistaProfessione(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := s.contestoProf(r, false)
	if err != nil {
		return nil, err
	}
	formule := []map[string]any{}
	sbloccate := map[string]bool{}
	rows, _ := s.DB.Query(`SELECT formula FROM prof_formule WHERE personaggio_id = ?`, c.pg)
	if rows != nil {
		for rows.Next() {
			var f string
			rows.Scan(&f)
			sbloccate[f] = true
		}
		rows.Close()
	}
	for _, f := range c.def.Formule {
		formule = append(formule, map[string]any{"id": f.ID, "conosciuta": s.formulaNota(c, &f, sbloccate)})
	}
	var gv, gd, gl int
	s.DB.QueryRow(`SELECT viaggio, dedicate, lunghe FROM prof_giorno WHERE personaggio_id = ?`, c.pg).Scan(&gv, &gd, &gl)
	var dati string
	s.DB.QueryRow(`SELECT v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, c.pg).Scan(&dati)
	p, _ := leggiPiatto(dati)
	registro := []map[string]any{}
	rows, _ = s.DB.Query(`SELECT r.azione, r.testo, r.creato_il, COALESCE(u.nome,'') FROM prof_registro r LEFT JOIN utenti u ON u.id = r.attore_id
		WHERE r.personaggio_id = ? AND r.professione = ? ORDER BY r.id DESC LIMIT 20`, c.pg, c.def.ID)
	if rows != nil {
		for rows.Next() {
			var a, t, chi string
			var q int64
			rows.Scan(&a, &t, &q, &chi)
			registro = append(registro, map[string]any{"azione": a, "testo": t, "quando": q, "chi": chi})
		}
		rows.Close()
	}
	return map[string]any{
		"personaggio": map[string]any{"id": c.pg, "nome": c.nome}, "professione": c.def.ID, "nomeProfessione": c.def.N,
		"attiva": slices.Contains(c.rp.Professioni, c.def.ID), "master": c.u.Master(), "bc": c.bc, "livello": c.rp.Livello,
		"mod": map[string]int{"INT": c.mod("INT"), "SAG": c.mod("SAG")}, "abilita": c.rp.Abilita,
		"monete": totaleMonete(p), "inventario": s.inventarioProf(c.pg, c.def), "formule": formule,
		"giorno": map[string]int{"viaggio": gv, "dedicate": gd, "lunghe": gl}, "registro": registro,
	}, nil
}

// formulaNota: le ricette ufficiali sono note; le formule del quaderno e quelle del talento le sblocca il Master.
func (s *Server) formulaNota(c *ctxProf, f *formDef, sbloccate map[string]bool) bool {
	if f.Conosciuta && !f.Talento {
		return true
	}
	if sbloccate == nil {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM prof_formule WHERE personaggio_id = ? AND formula = ?`, c.pg, f.ID).Scan(&n)
		return n > 0
	}
	return sbloccate[f.ID]
}

func contaGiorno(tx *sql.Tx, pg, campo string, limite int, msg string) error {
	tx.Exec(`INSERT OR IGNORE INTO prof_giorno(personaggio_id) VALUES(?)`, pg)
	var n int
	tx.QueryRow(`SELECT `+campo+` FROM prof_giorno WHERE personaggio_id = ?`, pg).Scan(&n)
	if n >= limite {
		return errore(http.StatusConflict, msg)
	}
	_, err := tx.Exec(`UPDATE prof_giorno SET `+campo+` = `+campo+` + 1 WHERE personaggio_id = ?`, pg)
	return err
}

// apiRaccolta: foraggiamento (in viaggio o ricerca dedicata).
func (s *Server) apiRaccolta(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := s.contestoProf(r, true)
	if err != nil {
		return nil, err
	}
	var in struct {
		Ingrediente string `json:"ingrediente"`
		Modo        string `json:"modo"`
		CD          int    `json:"cd"`
		Abilita     string `json:"abilita"`
		Aiuto       bool   `json:"aiuto"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if !c.def.Raccolta {
		return nil, errore(http.StatusBadRequest, "Questa professione non ha la raccolta.")
	}
	ing := c.def.ingrediente(in.Ingrediente)
	if ing == nil || !slices.Contains(ing.Abil, in.Abilita) || (in.CD != 10 && in.CD != 15 && in.CD != 20) || (in.Modo != "viaggio" && in.Modo != "dedicata") {
		return nil, errDatiNonValidi
	}
	car := map[string]string{"Sopravvivenza": "SAG", "Natura": "INT"}[in.Abilita]
	mod := c.mod(car)
	if c.competente(in.Abilita) {
		mod += c.bc
	}
	var tiro tiroProf
	var testo string
	extra := map[string]any{}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if in.Modo == "viaggio" {
			if err := contaGiorno(tx, c.pg, "viaggio", 1, "In viaggio si fa una sola raccolta al giorno: usa «Passa un giorno» (Master) o la ricerca dedicata."); err != nil {
				return err
			}
		} else if err := contaGiorno(tx, c.pg, "dedicate", 4, "Ricerca dedicata: al massimo 4 prove al giorno."); err != nil {
			return err
		}
		tiro = tiraProva(ing.N, mod, in.CD, in.Aiuto)
		if !tiro.Riuscito {
			testo = fmt.Sprintf("Raccolta di %s: %d contro CD %d, nessun ingrediente.", ing.N, tiro.Totale, in.CD)
			return nil
		}
		if c.def.Cibo { // ricerca di cibo (DMG p. 111): 1d6 + mod Sag libbre di cibo e galloni d'acqua
			cibo := max(0, tiraDado(6)+c.mod("SAG"))
			acqua := max(0, tiraDado(6)+c.mod("SAG"))
			testo = fmt.Sprintf("Ricerca di cibo: %d contro CD %d, trovati %d libbre di cibo (circa %s kg) e %d galloni d'acqua (circa %d litri).",
				tiro.Totale, in.CD, cibo, strings.Replace(strconv.FormatFloat(float64(cibo)*0.45, 'f', 1, 64), ".", ",", 1), acqua, acqua*4)
			extra["cibo"], extra["acqua"] = cibo, acqua
			_, _, err := s.modificaScheda(tx, c.pg, c.u, func(p piatto) (bool, error) {
				aggiungiOggetto(p, fmt.Sprintf("Cibo raccolto: %d libbre · acqua: %d galloni", cibo, acqua))
				return true, nil
			})
			return err
		}
		lotti := 1
		if tiro.Totale >= in.CD+10 {
			lotti = 2
		}
		extra["lotti"] = lotti
		testo = fmt.Sprintf("Raccolta di %s: %d contro CD %d, %d lott%s (buon%s per 7 giorni).", ing.N, tiro.Totale, in.CD, lotti, map[bool]string{true: "o", false: "i"}[lotti == 1], map[bool]string{true: "o", false: "i"}[lotti == 1])
		return aggiungiInv(tx, c.pg, c.def.ID, "ingrediente", ing.ID, "", lotti, setteGiorni())
	})
	if err != nil {
		return nil, err
	}
	s.avvisaSchedaSeServe(c)
	return s.esitoProf(c, "raccolta", testo, &tiro, extra), nil
}

// avvisaSchedaSeServe: dopo una modifica della scheda, chi la ha aperta la ricarica.
func (s *Server) avvisaSchedaSeServe(c *ctxProf) {
	var ver int
	var nota string
	if s.DB.QueryRow(`SELECT p.versione_corrente, v.nota FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente WHERE p.id = ?`, c.pg).Scan(&ver, &nota) == nil &&
		nota == "professione" {
		s.avvisaSchedaDa(c.pg, c.prop, ver, c.u, []string{"coins", "eq"}, true)
	}
}

// apiTana: trovare la tana di un animale velenoso (1 ora di ricerca dedicata, Sopravvivenza).
func (s *Server) apiTana(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := s.contestoProf(r, true)
	if err != nil {
		return nil, err
	}
	var in struct {
		Secrezione string `json:"secrezione"`
		Aiuto      bool   `json:"aiuto"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	sec := c.def.secrezione(in.Secrezione)
	if sec == nil || sec.Tana == 0 {
		return nil, errDatiNonValidi
	}
	mod := c.mod("SAG")
	if c.competente("Sopravvivenza") {
		mod += c.bc
	}
	var tiro tiroProf
	var testo string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if err := contaGiorno(tx, c.pg, "dedicate", 4, "Ricerca dedicata: al massimo 4 prove al giorno."); err != nil {
			return err
		}
		tiro = tiraProva("Trovare la tana", mod, sec.Tana, in.Aiuto)
		if !tiro.Riuscito {
			testo = fmt.Sprintf("Ricerca della tana (%s): %d contro CD %d, nessuna tana trovata.", sec.N, tiro.Totale, sec.Tana)
			return nil
		}
		testo = fmt.Sprintf("Tana trovata (%s): %d contro CD %d. Ora si può estrarre la secrezione.", sec.N, tiro.Totale, sec.Tana)
		uno := 1
		return aggiungiInv(tx, c.pg, c.def.ID, "tana", sec.ID, "", 1, &uno)
	})
	if err != nil {
		return nil, err
	}
	return s.esitoProf(c, "tana", testo, &tiro, nil), nil
}

// apiEstrazione: secrezione da una creatura morta o incapacitata (DMG p. 258).
func (s *Server) apiEstrazione(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := s.contestoProf(r, true)
	if err != nil {
		return nil, err
	}
	var in struct {
		Secrezione string `json:"secrezione"`
		Aiuto      bool   `json:"aiuto"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	sec := c.def.secrezione(in.Secrezione)
	if !c.def.Estrazione || sec == nil {
		return nil, errDatiNonValidi
	}
	mod := c.mod("INT") + c.bc // competenza in Natura, oppure il kit da avvelenatore: il bonus si aggiunge una volta
	var tiro tiroProf
	var testo string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if sec.Tana > 0 && quanti(tx, c.pg, c.def.ID, "tana", sec.ID) == 0 {
			return errore(http.StatusConflict, "Prima bisogna trovare la tana dell'animale.")
		}
		if quanti(tx, c.pg, c.def.ID, "fiala", "vuota") == 0 {
			return errore(http.StatusConflict, "Serve una fiala vuota per conservare la secrezione (1 mo).")
		}
		tiro = tiraProva("Estrazione: "+sec.N, mod, sec.CD, in.Aiuto)
		switch {
		case tiro.Riuscito:
			if sec.Tana > 0 {
				if err := consuma(tx, c.pg, c.def.ID, "tana", sec.ID, 1); err != nil {
					return err
				}
			}
			if err := consuma(tx, c.pg, c.def.ID, "fiala", "vuota", 1); err != nil {
				return err
			}
			testo = fmt.Sprintf("Estrazione (%s): %d contro CD %d, 1 dose di secrezione (buona per 7 giorni).", sec.N, tiro.Totale, sec.CD)
			return aggiungiInv(tx, c.pg, c.def.ID, "secrezione", sec.ID, "", 1, setteGiorni())
		case tiro.Totale <= sec.CD-5:
			testo = fmt.Sprintf("Estrazione (%s): %d contro CD %d, fallita di 5 o più: chi estrae subisce il veleno della creatura (lo applica il Master).", sec.N, tiro.Totale, sec.CD)
		default:
			testo = fmt.Sprintf("Estrazione (%s): %d contro CD %d, nessuna secrezione.", sec.N, tiro.Totale, sec.CD)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.esitoProf(c, "estrazione", testo, &tiro, nil), nil
}

// apiFiale: compra fiale vuote (1 mo l'una, PHB).
func (s *Server) apiFiale(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := s.contestoProf(r, true)
	if err != nil {
		return nil, err
	}
	var in struct {
		N int `json:"n"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.N < 1 || in.N > 20 {
		return nil, errore(http.StatusBadRequest, "Da 1 a 20 fiale.")
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		_, _, err := s.modificaScheda(tx, c.pg, c.u, func(p piatto) (bool, error) {
			if !paga(p, in.N*100) {
				return false, erroreMonete(in.N * 100)
			}
			return true, nil
		})
		if err != nil {
			return err
		}
		s.registraFinanze(tx, c.pg, c.u, "professione", -in.N*100, fmt.Sprintf("%d fiale vuote", in.N))
		return aggiungiInv(tx, c.pg, c.def.ID, "fiala", "vuota", "", in.N, nil)
	})
	if err != nil {
		return nil, err
	}
	s.avvisaSchedaSeServe(c)
	return s.esitoProf(c, "fiale", fmt.Sprintf("Comprate %d fiale vuote (%s).", in.N, testoMo(in.N*100)), nil, nil), nil
}

func dosiFormula(f *formDef, bc int) int {
	switch v := f.Dosi.(type) {
	case float64:
		return int(v)
	case string:
		if v == "BC" {
			return bc
		}
	}
	return 1
}

var nomeOggettoValido = regexp.MustCompile(`^[^\n\r<>]{1,80}$`)

// apiFabbrica: preparazione di una formula (o di un oggetto, per gli artigiani).
func (s *Server) apiFabbrica(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := s.contestoProf(r, true)
	if err != nil {
		return nil, err
	}
	var in struct {
		Formula   string   `json:"formula"`
		Opzionali []string `json:"opzionali"`
		Raccolti  *bool    `json:"raccolti"` // usare gli ingredienti raccolti (costo ridotto); predefinito sì
		Oggetto   string   `json:"oggetto"`  // artigiani: oggetto e prezzo
		Prezzo    float64  `json:"prezzo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if c.def.Generico || in.Formula == "generico" && c.def.ID == "alchimista" {
		return s.fabbricaGenerico(r, c, strings.TrimSpace(in.Oggetto), in.Prezzo)
	}
	f := c.def.formula(in.Formula)
	if f == nil {
		return nil, errNonTrovato
	}
	if !c.u.Master() && !s.formulaNota(c, f, nil) {
		return nil, errore(http.StatusForbidden, "Questa formula non è ancora conosciuta: la sblocca il Master.")
	}
	var tiro *tiroProf
	var testo string
	extra := map[string]any{}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if strings.HasPrefix(f.Tempo, "8 ore") {
			if err := contaGiorno(tx, c.pg, "lunghe", 1, "Le preparazioni da 8 ore: al massimo 1 lotto al giorno."); err != nil {
				return err
			}
		}
		for _, sid := range f.Secrezioni {
			if quanti(tx, c.pg, c.def.ID, "secrezione", sid) == 0 {
				return errore(http.StatusConflict, "Manca la secrezione: "+c.def.nomeVoce(sid)+" (non si può comprare).")
			}
		}
		usaRaccolti := in.Raccolti == nil || *in.Raccolti
		ridotto := f.Ridotto != nil && usaRaccolti && len(f.Raccoglibili) > 0
		for _, iid := range f.Raccoglibili {
			if quanti(tx, c.pg, c.def.ID, "ingrediente", iid) == 0 {
				ridotto = false
			}
		}
		costo := rame(f.Pieno)
		if ridotto {
			costo = rame(*f.Ridotto)
		}
		varianti := []string{}
		for _, o := range in.Opzionali {
			ok := false
			for _, od := range f.Opzionali {
				if od.ID == o {
					ok = true
				}
			}
			if !ok {
				return errDatiNonValidi
			}
			tipo := "speciale"
			if c.def.secrezione(o) != nil {
				tipo = "secrezione"
			}
			if quanti(tx, c.pg, c.def.ID, tipo, o) == 0 {
				return errore(http.StatusConflict, "Manca l'ingrediente facoltativo: "+c.def.nomeVoce(o)+".")
			}
			if err := consuma(tx, c.pg, c.def.ID, tipo, o, 1); err != nil {
				return err
			}
			varianti = append(varianti, o)
		}
		// materiali: secrezioni e ingredienti raccolti si consumano comunque
		for _, sid := range f.Secrezioni {
			if err := consuma(tx, c.pg, c.def.ID, "secrezione", sid, 1); err != nil {
				return err
			}
		}
		if ridotto {
			for _, iid := range f.Raccoglibili {
				if err := consuma(tx, c.pg, c.def.ID, "ingrediente", iid, 1); err != nil {
					return err
				}
			}
		}
		// requisiti prima della prova: le monete per la parte da comprare
		if moneteScheda(tx, c.pg) < costo {
			return erroreMonete(costo)
		}
		// la prova (se la formula la richiede): Intelligenza con il kit + bonus di competenza
		dosi := dosiFormula(f, c.bc)
		riuscita := true
		if f.CD > 0 {
			t := tiraProva("Creazione: "+f.N, c.mod("INT")+c.bc, f.CD, false)
			tiro = &t
			riuscita = t.Riuscito
			if riuscita && f.Bonus5 && t.Totale >= f.CD+5 {
				dosi = 2
			}
		}
		rimborso := 0
		switch {
		case riuscita:
		case tiro.Totale <= f.CD-5:
		default:
			rimborso = costo / 2
		}
		_, _, err := s.modificaScheda(tx, c.pg, c.u, func(p piatto) (bool, error) {
			if !paga(p, costo) {
				return false, erroreMonete(costo)
			}
			if rimborso > 0 {
				rimborsa(p, rimborso)
			}
			if riuscita && f.Prodotto != "" {
				aggiungiOggetto(p, fmt.Sprintf("%d × %s", dosi, f.Prodotto))
			}
			return true, nil
		})
		if err != nil {
			return err
		}
		s.registraFinanze(tx, c.pg, c.u, "professione", rimborso-costo, c.def.N+": materiali per "+f.N+map[bool]string{true: "", false: " (creazione fallita)"}[riuscita])
		costoTesto := testoMo(costo)
		if ridotto {
			costoTesto += " (costo ridotto, ingredienti raccolti)"
		}
		switch {
		case riuscita:
			if c.def.Dosi && f.Prodotto == "" {
				if err := aggiungiInv(tx, c.pg, c.def.ID, "dose", f.ID, strings.Join(varianti, ","), dosi, nil); err != nil {
					return err
				}
			}
			testo = fmt.Sprintf("%s: preparat%s %d dos%s. Speso %s; tempo: %s.", f.N, map[bool]string{true: "a", false: "e"}[dosi == 1], dosi, map[bool]string{true: "e", false: "i"}[dosi == 1], costoTesto, f.Tempo)
			if f.Prodotto != "" {
				testo = fmt.Sprintf("%s: preparato e aggiunto all'equipaggiamento. Speso %s; tempo: %s.", f.N, costoTesto, f.Tempo)
			}
			extra["dosi"] = dosi
		case rimborso > 0:
			testo = fmt.Sprintf("%s: creazione fallita (%d contro CD %d). Tempo perso, recuperati %s dei materiali comprati; gli ingredienti raccolti sono persi.", f.N, tiro.Totale, f.CD, testoMo(rimborso))
		default:
			testo = fmt.Sprintf("%s: creazione fallita di 5 o più (%d contro CD %d). Tutti i materiali persi; il Master può far fare il TS della formula a chi la preparava.", f.N, tiro.Totale, f.CD)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.avvisaSchedaSeServe(c)
	return s.esitoProf(c, "fabbricazione", testo, tiro, extra), nil
}

// fabbricaGenerico: regola generale del Manuale del Giocatore (materiali a metà del prezzo, 5 mo di lavoro al giorno).
func (s *Server) fabbricaGenerico(r *http.Request, c *ctxProf, oggetto string, prezzo float64) (any, error) {
	if !nomeOggettoValido.MatchString(oggetto) || prezzo <= 0 || prezzo > 100000 {
		return nil, errore(http.StatusBadRequest, "Scrivi l'oggetto e il suo prezzo in mo.")
	}
	costo := rame(prezzo) / 2
	giorni := int(math.Ceil(prezzo / 5))
	err := db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		_, _, err := s.modificaScheda(tx, c.pg, c.u, func(p piatto) (bool, error) {
			if !paga(p, costo) {
				return false, erroreMonete(costo)
			}
			aggiungiOggetto(p, "1 × "+oggetto)
			return true, nil
		})
		if err == nil {
			s.registraFinanze(tx, c.pg, c.u, "professione", -costo, c.def.N+": materiali per «"+oggetto+"»")
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	s.avvisaSchedaSeServe(c)
	testo := fmt.Sprintf("%s: «%s» fabbricato e aggiunto all'equipaggiamento. Materiali %s; tempo: %d giorni di lavoro (Manuale del Giocatore) o %d settimane (supplemento).",
		c.def.N, oggetto, testoMo(costo), giorni, max(1, int(math.Ceil(prezzo/50))))
	return s.esitoProf(c, "fabbricazione", testo, nil, nil), nil
}

// apiApplicaDose: una dose sull'arma o su una munizione (si consuma).
func (s *Server) apiApplicaDose(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := s.contestoProf(r, true)
	if err != nil {
		return nil, err
	}
	var in struct {
		Formula  string `json:"formula"`
		Variante string `json:"variante"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	f := c.def.formula(in.Formula)
	if f == nil || !c.def.Dosi {
		return nil, errDatiNonValidi
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		var id int64
		if tx.QueryRow(`SELECT id FROM prof_inventario WHERE personaggio_id = ? AND professione = ? AND tipo = 'dose' AND voce = ? AND variante = ? AND quantita > 0 LIMIT 1`,
			c.pg, c.def.ID, f.ID, in.Variante).Scan(&id) != nil {
			return errore(http.StatusConflict, "Nessuna dose disponibile.")
		}
		_, err := tx.Exec(`UPDATE prof_inventario SET quantita = quantita - 1 WHERE id = ?`, id)
		tx.Exec(`DELETE FROM prof_inventario WHERE personaggio_id = ? AND quantita <= 0`, c.pg)
		return err
	})
	if err != nil {
		return nil, err
	}
	testo := fmt.Sprintf("%s applicato a un'arma o a una munizione (azione bonus con il talento Avvelenatore, altrimenti un'azione). Dura 1 minuto o fino al colpo. TS: %s. %s",
		f.N, f.TS, f.Effetto)
	if in.Variante != "" {
		for _, o := range f.Opzionali {
			if strings.Contains(in.Variante, o.ID) {
				testo += " Variante: " + o.Effetto + "."
			}
		}
	}
	if f.Immuni {
		testo += " Non ha effetto su non morti e costrutti."
	}
	return s.esitoProf(c, "uso", testo, nil, nil), nil
}

// ===== Strumenti del Master =====

func (s *Server) apiFormulaConosciuta(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := s.contestoProf(r, true)
	if err != nil {
		return nil, err
	}
	if !c.u.Master() {
		return nil, errVietato
	}
	var in struct {
		Formula    string `json:"formula"`
		Conosciuta bool   `json:"conosciuta"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	f := c.def.formula(in.Formula)
	if f == nil {
		return nil, errNonTrovato
	}
	if in.Conosciuta {
		s.DB.Exec(`INSERT OR IGNORE INTO prof_formule(personaggio_id, formula) VALUES(?,?)`, c.pg, f.ID)
	} else {
		s.DB.Exec(`DELETE FROM prof_formule WHERE personaggio_id = ? AND formula = ?`, c.pg, f.ID)
	}
	stato := "sbloccata"
	if !in.Conosciuta {
		stato = "tolta"
	}
	return s.esitoProf(c, "formula", "Formula "+stato+" dal Master: "+f.N+".", nil, nil), nil
}

// apiCorreggiInventario: il Master aggiunge o toglie voci (per esempio ectoplasma o terra vampirica, «a discrezione del DM»).
func (s *Server) apiCorreggiInventario(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := s.contestoProf(r, true)
	if err != nil {
		return nil, err
	}
	if !c.u.Master() {
		return nil, errVietato
	}
	var in struct {
		Tipo     string `json:"tipo"`
		Voce     string `json:"voce"`
		Variante string `json:"variante"`
		Quantita int    `json:"quantita"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	valida := false
	switch in.Tipo {
	case "ingrediente":
		valida = c.def.ingrediente(in.Voce) != nil
	case "secrezione":
		valida = c.def.secrezione(in.Voce) != nil
	case "speciale":
		for _, x := range c.def.Speciali {
			valida = valida || x.ID == in.Voce
		}
	case "fiala":
		valida = in.Voce == "vuota"
	case "dose":
		valida = c.def.formula(in.Voce) != nil
	}
	if !valida || in.Quantita == 0 || in.Quantita < -50 || in.Quantita > 50 {
		return nil, errDatiNonValidi
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		if in.Quantita < 0 {
			return consuma(tx, c.pg, c.def.ID, in.Tipo, in.Voce, -in.Quantita)
		}
		var g *int
		if in.Tipo == "ingrediente" || in.Tipo == "secrezione" || in.Tipo == "speciale" {
			g = setteGiorni()
		}
		return aggiungiInv(tx, c.pg, c.def.ID, in.Tipo, in.Voce, in.Variante, in.Quantita, g)
	})
	if err != nil {
		return nil, err
	}
	return s.esitoProf(c, "inventario", fmt.Sprintf("Il Master ha %s %d × %s.", map[bool]string{true: "aggiunto", false: "tolto"}[in.Quantita > 0], abs(in.Quantita), c.def.nomeVoce(in.Voce)), nil, nil), nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// apiPassaGiorno: un giorno di gioco per tutti: gli ingredienti invecchiano (scadono a 0), i contatori si azzerano.
func (s *Server) apiPassaGiorno(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	scaduti, err := s.passaGiornoGioco(u)
	if err != nil {
		return nil, err
	}
	return map[string]any{"scaduti": scaduti}, nil
}

// passaGiornoGioco: un giorno di gioco per tutti (Professioni; anche dai viaggi del segnalino, 2026-11-08).
func (s *Server) passaGiornoGioco(u *Utente) (int64, error) {
	var scaduti int64
	err := db.Tx(context.Background(), s.DB, func(tx *sql.Tx) error {
		tx.Exec(`UPDATE prof_inventario SET giorni = giorni - 1 WHERE giorni IS NOT NULL`)
		res, _ := tx.Exec(`DELETE FROM prof_inventario WHERE giorni IS NOT NULL AND giorni <= 0 AND tipo <> 'tana'`)
		if res != nil {
			scaduti, _ = res.RowsAffected()
		}
		tx.Exec(`DELETE FROM prof_inventario WHERE tipo = 'tana'`)
		_, err := tx.Exec(`DELETE FROM prof_giorno`)
		return err
	})
	if err != nil {
		return 0, err
	}
	m := Msg{Tipo: "professione.giorno", Dati: map[string]any{"da": u.Nome, "scaduti": scaduti}}
	s.Hub.AlMaster(m)
	for _, g := range s.gruppiDi(u) {
		s.Hub.Pubblica("gruppo:"+g, m)
	}
	return scaduti, nil
}
