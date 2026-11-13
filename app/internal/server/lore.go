package server

// Codice della campagna (richiesta dell'utente del 2026-11-12, fase L1; migrazione 033). La lore del Master — scritta
// anche con ChatGPT — in schede collegate: luoghi, PNG, fazioni, eventi, trame, oggetti, segreti, cronologia. Ogni
// scheda ha il testo del Master (segreti compresi), quello per i giocatori, campi «chiave: valore», collegamenti ad altre
// schede (per nome), etichette e, per ogni gruppo, a che punto è (conosciuto, incontrato, concluso). Solo il Master.
// Non tocca le altre funzioni: tabelle sue, nessun dato esistente cambiato.
//
// Importazione e esportazione in un Markdown leggibile (lo stesso formato che si chiede a ChatGPT):
//
//	# PNG: Aldric Borinovich
//	Luogo: Villaggio di Nebbiavalle
//	Collegamenti: Elara Voss; Villaggio di Nebbiavalle
//	Etichette: alleato, nobile
//
//	## Per il Master
//	… (segreti, intenzioni, cosa sa davvero)
//
//	## Per i giocatori
//	… (ciò che si sa di lui)
//
// Le righe «Chiave: valore» stanno subito sotto il titolo, fino alla prima riga vuota. Altri titoli «## …» finiscono nel
// testo del Master. L'importazione mostra prima un'anteprima (nuove, uguali, diverse) e aggiunge senza cancellare; le
// schede diverse si aggiornano solo se il Master lo sceglie.

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const (
	maxSchedeLore   = 5000
	maxTestoLore    = 30000
	maxImportaLore  = 4 << 20
	maxCampiLore    = 20
	maxCollegamenti = 40
	maxEtichette    = 20
)

// tipiLore: il tipo e i nomi con cui si può scrivere nel titolo («# PNG: …», «# Località: …»).
var tipiLore = map[string][]string{
	"luogo":      {"luogo", "luoghi", "località", "localita", "location", "posto"},
	"png":        {"png", "personaggio", "personaggio non giocante", "npc"},
	"fazione":    {"fazione", "fazioni", "organizzazione", "gruppo", "setta", "casata"},
	"evento":     {"evento", "eventi", "scena", "incontro"},
	"trama":      {"trama", "arco", "arco narrativo", "missione", "quest", "filone"},
	"oggetto":    {"oggetto", "artefatto", "reliquia"},
	"segreto":    {"segreto", "indizio", "rivelazione", "mistero"},
	"cronologia": {"cronologia", "linea del tempo", "data", "storia", "passato"},
}

var ordineTipiLore = []string{"luogo", "png", "fazione", "evento", "trama", "oggetto", "segreto", "cronologia"}

var titoloTipoLore = map[string]string{"luogo": "Luogo", "png": "PNG", "fazione": "Fazione", "evento": "Evento", "trama": "Trama",
	"oggetto": "Oggetto", "segreto": "Segreto", "cronologia": "Cronologia"}

var statiLore = []string{"conosciuto", "incontrato", "concluso"}

type schedaLore struct {
	ID             string               `json:"id"`
	Tipo           string               `json:"tipo"`
	Nome           string               `json:"nome"`
	TestoMaster    string               `json:"testoMaster"`
	TestoGiocatori string               `json:"testoGiocatori"`
	Campi          map[string]string    `json:"campi"`
	Collegamenti   []string             `json:"collegamenti"`
	Etichette      []string             `json:"etichette"`
	Stati          map[string]statoLore `json:"stati,omitempty"`   // gruppo → stato
	Innesco        *innescoLore         `json:"innesco,omitempty"` // come Gaster Bless capisce il campo «Innesco» (lore_promemoria.go)
	Modificato     int64                `json:"modificato,omitempty"`
}

type statoLore struct {
	Stato string `json:"stato"`
	Nota  string `json:"nota,omitempty"`
}

func (s *Server) rotteLore(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/lore", api(s.apiLore))
	mux.Handle("PUT /api/v1/lore/{id}", api(s.apiSalvaLore))
	mux.Handle("DELETE /api/v1/lore/{id}", api(s.apiEliminaLore))
	mux.Handle("PUT /api/v1/lore/{id}/stato", api(s.apiStatoLore))
	mux.Handle("POST /api/v1/lore/importa", api(s.apiImportaLore))
	mux.Handle("GET /api/v1/lore/esporta", api(s.apiEsportaLore))
	mux.Handle("POST /api/v1/lore/{id}/promemoria", api(s.apiPromemoriaLore))
}

// tipoLore riconosce il tipo da come lo si scrive (anche al plurale o in inglese).
func tipoLore(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	for k, v := range tipiLore {
		if slices.Contains(v, t) {
			return k
		}
	}
	return ""
}

// pulisciLore: limiti e forma di una scheda (dal Master o dall'importazione).
func pulisciLore(x schedaLore) (schedaLore, string) {
	x.Tipo = tipoLore(x.Tipo)
	if x.Tipo == "" {
		return x, "Tipo di scheda non riconosciuto."
	}
	x.Nome = strings.Join(strings.Fields(x.Nome), " ")
	if x.Nome == "" || utf8.RuneCountInString(x.Nome) > 100 {
		return x, "Il nome è obbligatorio (al massimo 100 caratteri)."
	}
	x.TestoMaster = strings.TrimSpace(strings.ReplaceAll(x.TestoMaster, "\r\n", "\n"))
	x.TestoGiocatori = strings.TrimSpace(strings.ReplaceAll(x.TestoGiocatori, "\r\n", "\n"))
	if utf8.RuneCountInString(x.TestoMaster) > maxTestoLore || utf8.RuneCountInString(x.TestoGiocatori) > maxTestoLore {
		return x, "Testo troppo lungo (al massimo 30.000 caratteri per parte)."
	}
	campi := map[string]string{}
	for k, v := range x.Campi {
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k != "" && v != "" && len(campi) < maxCampiLore {
			campi[limita(k, 40)] = limita(v, 300)
		}
	}
	x.Campi = campi
	x.Collegamenti = pulisciElenco(x.Collegamenti, maxCollegamenti, 100, x.Nome)
	x.Etichette = pulisciElenco(x.Etichette, maxEtichette, 40, "")
	return x, ""
}

func pulisciElenco(l []string, max, lung int, escludi string) []string {
	out := []string{}
	for _, v := range l {
		v = strings.Join(strings.Fields(v), " ")
		if v == "" || strings.EqualFold(v, escludi) || slices.ContainsFunc(out, func(x string) bool { return strings.EqualFold(x, v) }) || len(out) >= max {
			continue
		}
		out = append(out, limita(v, lung))
	}
	return out
}

func (s *Server) caricaLore(dove string, args ...any) ([]schedaLore, error) {
	rows, err := s.DB.Query(`SELECT id, tipo, nome, testo_master, testo_giocatori, campi, collegamenti, etichette, modificato_il FROM lore_schede `+dove, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []schedaLore{}
	for rows.Next() {
		var x schedaLore
		var campi, coll, et string
		if err := rows.Scan(&x.ID, &x.Tipo, &x.Nome, &x.TestoMaster, &x.TestoGiocatori, &campi, &coll, &et, &x.Modificato); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(campi), &x.Campi)
		json.Unmarshal([]byte(coll), &x.Collegamenti)
		json.Unmarshal([]byte(et), &x.Etichette)
		out = append(out, x)
	}
	return out, rows.Err()
}

// apiLore: tutte le schede con lo stato per gruppo, e i gruppi.
func (s *Server) apiLore(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	l, err := s.caricaLore(`ORDER BY nome COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	stati := map[string]map[string]statoLore{}
	rows, err := s.DB.Query(`SELECT scheda_id, gruppo_id, stato, nota FROM lore_stato`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sc, g string
		var x statoLore
		rows.Scan(&sc, &g, &x.Stato, &x.Nota)
		if stati[sc] == nil {
			stati[sc] = map[string]statoLore{}
		}
		stati[sc][g] = x
	}
	rows.Close()
	for i := range l {
		l[i].Stati = stati[l[i].ID]
		l[i].Innesco = s.leggiInnesco(l[i].Campi)
	}
	type gruppo struct {
		ID   string `json:"id"`
		Nome string `json:"nome"`
	}
	gruppi := []gruppo{}
	gr, err := s.DB.Query(`SELECT id, nome FROM gruppi ORDER BY nome COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	for gr.Next() {
		var g gruppo
		gr.Scan(&g.ID, &g.Nome)
		gruppi = append(gruppi, g)
	}
	gr.Close()
	return map[string]any{"schede": l, "gruppi": gruppi, "tipi": ordineTipiLore, "promemoria": s.promemoriaAperti()}, nil
}

func marshalLore(x schedaLore) (string, string, string) {
	c, _ := json.Marshal(x.Campi)
	k, _ := json.Marshal(x.Collegamenti)
	e, _ := json.Marshal(x.Etichette)
	return string(c), string(k), string(e)
}

// scriviLore crea (id vuoto) o aggiorna una scheda; errore 409 se c'è già una scheda dello stesso tipo con quel nome.
func (s *Server) scriviLore(tx *sql.Tx, x schedaLore) (string, error) {
	var altro string
	tx.QueryRow(`SELECT id FROM lore_schede WHERE tipo = ? AND nome = ? COLLATE NOCASE`, x.Tipo, x.Nome).Scan(&altro)
	if altro != "" && altro != x.ID {
		return "", errore(http.StatusConflict, "C'è già una scheda «"+titoloTipoLore[x.Tipo]+": "+x.Nome+"».")
	}
	c, k, e := marshalLore(x)
	ora := db.Ora()
	if x.ID == "" {
		var n int
		tx.QueryRow(`SELECT COUNT(*) FROM lore_schede`).Scan(&n)
		if n >= maxSchedeLore {
			return "", errore(http.StatusConflict, "Il Codice della campagna ha già 5.000 schede.")
		}
		x.ID = cripto.ID()
		_, err := tx.Exec(`INSERT INTO lore_schede(id, tipo, nome, testo_master, testo_giocatori, campi, collegamenti, etichette, creato_il, modificato_il)
			VALUES(?,?,?,?,?,?,?,?,?,?)`, x.ID, x.Tipo, x.Nome, x.TestoMaster, x.TestoGiocatori, c, k, e, ora, ora)
		return x.ID, err
	}
	res, err := tx.Exec(`UPDATE lore_schede SET tipo = ?, nome = ?, testo_master = ?, testo_giocatori = ?, campi = ?, collegamenti = ?, etichette = ?,
		modificato_il = ? WHERE id = ?`, x.Tipo, x.Nome, x.TestoMaster, x.TestoGiocatori, c, k, e, ora, x.ID)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", errNonTrovato
	}
	return x.ID, nil
}

// apiSalvaLore: PUT /lore/nuova crea, PUT /lore/{id} aggiorna.
func (s *Server) apiSalvaLore(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in schedaLore
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	x, msg := pulisciLore(in)
	if msg != "" {
		return nil, errore(http.StatusBadRequest, msg)
	}
	x.ID = ""
	if id := r.PathValue("id"); id != "nuova" {
		x.ID = id
	}
	var id string
	err := db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		var err error
		id, err = s.scriviLore(tx, x)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (s *Server) apiEliminaLore(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`DELETE FROM lore_schede WHERE id = ?`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errNonTrovato
	}
	return nil, nil
}

// apiStatoLore: a che punto è un gruppo con una scheda (stato vuoto = sconosciuto).
func (s *Server) apiStatoLore(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Gruppo string `json:"gruppo"`
		Stato  string `json:"stato"`
		Nota   string `json:"nota"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var ok int
	s.DB.QueryRow(`SELECT (SELECT COUNT(*) FROM lore_schede WHERE id = ?) * (SELECT COUNT(*) FROM gruppi WHERE id = ?)`, id, in.Gruppo).Scan(&ok)
	if ok == 0 {
		return nil, errNonTrovato
	}
	if in.Stato == "" {
		_, err := s.DB.Exec(`DELETE FROM lore_stato WHERE scheda_id = ? AND gruppo_id = ?`, id, in.Gruppo)
		return nil, err
	}
	if !slices.Contains(statiLore, in.Stato) {
		return nil, errDatiNonValidi
	}
	_, err := s.DB.Exec(`INSERT INTO lore_stato(scheda_id, gruppo_id, stato, nota, modificato_il) VALUES(?,?,?,?,?)
		ON CONFLICT(scheda_id, gruppo_id) DO UPDATE SET stato = excluded.stato, nota = excluded.nota, modificato_il = excluded.modificato_il`,
		id, in.Gruppo, in.Stato, limita(strings.TrimSpace(in.Nota), 500), db.Ora())
	return nil, err
}

// ===== Importazione ed esportazione in Markdown =====

var (
	reTitoloLore  = regexp.MustCompile(`^#\s+([^:#][^:]{0,40}?)\s*:\s*(.+?)\s*#*\s*$`)
	reSezioneLore = regexp.MustCompile(`^##\s+(.+?)\s*#*\s*$`)
	reCampoLore   = regexp.MustCompile(`^[-*]?\s*\**([A-Za-zÀ-ÿ' ]{1,30}?)\**\s*:\s*(.+)$`)
)

var (
	sezioniMasterLore    = []string{"per il master", "master", "segreti", "segreto", "solo master", "note del master", "per il dm", "dm"}
	sezioniGiocatoriLore = []string{"per i giocatori", "giocatori", "cosa sanno i giocatori", "cosa si sa", "pubblico", "handout", "per i pg"}
)

type erroreLore struct {
	Riga  int    `json:"riga"`
	Testo string `json:"testo"`
}

// leggiMarkdownLore trasforma il testo in schede; le righe che non si capiscono finiscono negli errori (non bloccano).
func leggiMarkdownLore(testo string) ([]schedaLore, []erroreLore) {
	var out []schedaLore
	var errori []erroreLore
	var cur *schedaLore
	var parte string // "meta", "master", "giocatori"
	var master, gioc []string
	chiudi := func() {
		if cur == nil {
			return
		}
		cur.TestoMaster, cur.TestoGiocatori = strings.Join(master, "\n"), strings.Join(gioc, "\n")
		out = append(out, *cur)
		cur, master, gioc = nil, nil, nil
	}
	for i, riga := range strings.Split(strings.ReplaceAll(testo, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(riga)
		if strings.HasPrefix(t, "```") { // i bordi del blocco di codice in cui risponde ChatGPT
			continue
		}
		if strings.HasPrefix(t, "# ") || t == "#" {
			chiudi()
			m := reTitoloLore.FindStringSubmatch(t)
			if m == nil || tipoLore(m[1]) == "" {
				errori = append(errori, erroreLore{i + 1, "Titolo non riconosciuto (serve «# Tipo: Nome», con Tipo tra Luogo, PNG, Fazione, Evento, Trama, Oggetto, Segreto, Cronologia): " + limita(t, 80)})
				continue
			}
			cur = &schedaLore{Tipo: tipoLore(m[1]), Nome: m[2], Campi: map[string]string{}}
			parte = "meta"
			continue
		}
		if cur == nil {
			continue // testo prima della prima scheda (introduzione): si ignora
		}
		if m := reSezioneLore.FindStringSubmatch(t); m != nil && !strings.HasPrefix(t, "###") {
			nome := strings.ToLower(strings.Trim(m[1], "*_: "))
			switch {
			case slices.Contains(sezioniGiocatoriLore, nome):
				parte = "giocatori"
			case slices.Contains(sezioniMasterLore, nome):
				parte = "master"
			default: // un'altra sezione: resta nel testo del Master con il suo titolo
				parte = "master"
				master = append(master, "### "+strings.Trim(m[1], "*_ "))
			}
			continue
		}
		if parte == "meta" {
			if t == "" {
				parte = "master"
				continue
			}
			if m := reCampoLore.FindStringSubmatch(t); m != nil {
				k, v := strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
				switch strings.ToLower(k) {
				case "collegamenti", "collegato a", "collegati", "legami", "link":
					cur.Collegamenti = append(cur.Collegamenti, dividiLore(v)...)
				case "etichette", "tag", "tags", "parole chiave":
					cur.Etichette = append(cur.Etichette, dividiLore(v)...)
				default:
					cur.Campi[k] = v
				}
				continue
			}
			parte = "master" // nessuna riga vuota dopo i campi: da qui è testo
		}
		if parte == "giocatori" {
			gioc = append(gioc, riga)
		} else {
			master = append(master, riga)
		}
	}
	chiudi()
	return out, errori
}

func dividiLore(v string) []string {
	f := func(r rune) bool { return r == ';' || r == ',' }
	if strings.Contains(v, ";") {
		f = func(r rune) bool { return r == ';' }
	}
	var out []string
	for _, x := range strings.FieldsFunc(v, f) {
		if x = strings.Trim(strings.TrimSpace(x), "[]*_"); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// uguali: due schede con lo stesso contenuto (lo stato per gruppo non conta).
func ugualiLore(a, b schedaLore) bool {
	ca, ka, ea := marshalLore(a)
	cb, kb, eb := marshalLore(b)
	return a.TestoMaster == b.TestoMaster && a.TestoGiocatori == b.TestoGiocatori && ca == cb && ka == kb && ea == eb
}

// apiImportaLore: anteprima (applica false) o importazione; aggiunge le nuove, aggiorna le diverse solo con «aggiorna».
func (s *Server) apiImportaLore(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImportaLore+4096)
	var in struct {
		Testo    string `json:"testo"`
		Applica  bool   `json:"applica"`
		Aggiorna bool   `json:"aggiorna"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	lette, errori := leggiMarkdownLore(in.Testo)
	type voce struct {
		Tipo string `json:"tipo"`
		Nome string `json:"nome"`
	}
	nuove, diverse, uguali := []voce{}, []voce{}, []voce{}
	esistenti, err := s.caricaLore("")
	if err != nil {
		return nil, err
	}
	trova := func(x schedaLore) *schedaLore {
		for i := range esistenti {
			if esistenti[i].Tipo == x.Tipo && strings.EqualFold(esistenti[i].Nome, x.Nome) {
				return &esistenti[i]
			}
		}
		return nil
	}
	var daScrivere []schedaLore
	visti := map[string]bool{}
	for _, x := range lette {
		x, msg := pulisciLore(x)
		if msg != "" {
			errori = append(errori, erroreLore{0, x.Nome + ": " + msg})
			continue
		}
		k := x.Tipo + "|" + strings.ToLower(x.Nome)
		if visti[k] {
			errori = append(errori, erroreLore{0, "«" + titoloTipoLore[x.Tipo] + ": " + x.Nome + "» compare due volte: vale la prima."})
			continue
		}
		visti[k] = true
		v := voce{x.Tipo, x.Nome}
		switch e := trova(x); {
		case e == nil:
			nuove = append(nuove, v)
			daScrivere = append(daScrivere, x)
		case ugualiLore(*e, x):
			uguali = append(uguali, v)
		default:
			diverse = append(diverse, v)
			if in.Aggiorna {
				x.ID, x.Nome = e.ID, e.Nome
				daScrivere = append(daScrivere, x)
			}
		}
	}
	if errori == nil {
		errori = []erroreLore{}
	}
	out := map[string]any{"nuove": nuove, "diverse": diverse, "uguali": len(uguali), "errori": errori, "applicato": false}
	if !in.Applica {
		return out, nil
	}
	if len(esistenti)+len(nuove) > maxSchedeLore {
		return nil, errore(http.StatusConflict, "Il Codice della campagna avrebbe più di 5.000 schede.")
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		for _, x := range daScrivere {
			if _, err := s.scriviLore(tx, x); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out["applicato"] = true
	return out, nil
}

// testoMarkdownLore: le schede nello stesso formato dell'importazione (per ChatGPT o per un altro PC).
func testoMarkdownLore(l []schedaLore) string {
	sort.SliceStable(l, func(i, j int) bool {
		a, b := slices.Index(ordineTipiLore, l[i].Tipo), slices.Index(ordineTipiLore, l[j].Tipo)
		if a != b {
			return a < b
		}
		return strings.ToLower(l[i].Nome) < strings.ToLower(l[j].Nome)
	})
	var b strings.Builder
	for _, x := range l {
		b.WriteString("# " + titoloTipoLore[x.Tipo] + ": " + x.Nome + "\n")
		chiavi := make([]string, 0, len(x.Campi))
		for k := range x.Campi {
			chiavi = append(chiavi, k)
		}
		sort.Strings(chiavi)
		for _, k := range chiavi {
			b.WriteString(k + ": " + x.Campi[k] + "\n")
		}
		if len(x.Collegamenti) > 0 {
			b.WriteString("Collegamenti: " + strings.Join(x.Collegamenti, "; ") + "\n")
		}
		if len(x.Etichette) > 0 {
			b.WriteString("Etichette: " + strings.Join(x.Etichette, ", ") + "\n")
		}
		if x.TestoMaster != "" {
			b.WriteString("\n## Per il Master\n" + x.TestoMaster + "\n")
		}
		if x.TestoGiocatori != "" {
			b.WriteString("\n## Per i giocatori\n" + x.TestoGiocatori + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (s *Server) apiEsportaLore(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	l, err := s.caricaLore("")
	if err != nil {
		return nil, err
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="Codice_della_campagna.md"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(testoMarkdownLore(l)))
	return nil, errGiaRisposto
}
