package server

// «📥 Importa dai manuali» (fase M3, docs/PIANO_IMPORTA_MANUALI.md, 2026-11-11). La lettura dei PDF la fa il browser del
// Master sul PC (web/ui/importa_manuali.js, con pdf.js): qui arrivano i candidati (mostri e PNG; poi incantesimi, oggetti
// magici e regole). Il server li confronta con i dati attuali e prepara il riepilogo («nuovo», «uguale», «cambiato»,
// «a_mano» = cambiato a mano dal Master dopo l'ultima importazione o mai importato, «non_valido»); il Master sceglie le
// voci e le conferma. Gli elementi ricordano manuale e pagina e restano anche se il manuale viene eliminato.
// Il testo letto dai manuali non esce mai verso i giocatori: tutte queste API sono solo per il Master.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"

	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const (
	maxImportazione = 48 << 20
	maxCandidati    = 5000
	maxTestoElem    = 60 << 10
)

type candidatoManuale struct {
	Tipo      string          `json:"tipo"`
	Sottotipo string          `json:"sottotipo,omitempty"`
	Nome      string          `json:"nome"`
	Manuale   string          `json:"manuale"`
	Pagina    int             `json:"pagina"`
	Dati      json.RawMessage `json:"dati"`
	Testo     string          `json:"testo,omitempty"`
	Incerto   bool            `json:"incerto,omitempty"` // lettura dubbia: il Master controlli
	Motivi    []string        `json:"motivi,omitempty"`  // perché è dubbia (per il riepilogo e la diagnostica)
	Traccia   string          `json:"traccia,omitempty"` // dove e perché la lettura si è fermata (diagnostica)
	Base      json.RawMessage `json:"base,omitempty"`    // incantesimi: la voce dell'archivio della scheda con lo stesso nome (la trova la pagina)
}

type voceRiepilogo struct {
	candidatoManuale
	Chiave      string          `json:"chiave"`
	Voce        string          `json:"voce,omitempty"`     // a cosa si applica in Gaster Bless (mostri: id della voce del bestiario)
	NomeVoce    string          `json:"nomeVoce,omitempty"` // nome attuale in Gaster Bless
	Stato       string          `json:"stato"`
	Campi       []string        `json:"campi,omitempty"` // valori che cambiano
	Attuale     json.RawMessage `json:"attuale,omitempty"`
	ManualeNome string          `json:"manualeNome"`
	Doppio      string          `json:"doppio,omitempty"` // la stessa voce letta anche da un altro manuale/pagina
	Applicata   bool            `json:"applicata,omitempty"`
	Errore      string          `json:"errore,omitempty"` // perché non è valida
}

var nonAlfanumerico = regexp.MustCompile(`[^a-z0-9]+`)

// chiaveNome: minuscole, senza accenti né punteggiatura (come normalizza() di web/ui/manuale_scheda.js).
func chiaveNome(s string) string {
	return strings.TrimSpace(nonAlfanumerico.ReplaceAllString(normalizza(s), " "))
}

func improntaDati(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:12])
}

// ===== Nomi del bestiario (voci di base dei due cataloghi e voci personali del Master) =====

var (
	nomiBestUna ricaricabile      // si rilegge quando cambiano i pacchetti di contenuti (contenuti.go)
	nomiBest    map[string]string // chiave del nome (italiano o inglese) → id della voce di base
	nomiVoce    map[string]string // id → nome italiano
)

func (s *Server) caricaNomiBestiario() {
	nomiBestUna.Do(func() {
		nomiBest, nomiVoce = map[string]string{}, map[string]string{}
		for _, f := range []string{"carta/bestiario.json", "carta/mostri.json"} { // bestiario dell'avventura prima: vince
			b, err := s.leggiWeb(f)
			if err != nil {
				continue
			}
			var j struct {
				Voci []struct{ ID, Nome, En string }
			}
			json.Unmarshal(b, &j)
			for _, v := range j.Voci {
				for _, n := range []string{v.Nome, v.En} {
					if k := chiaveNome(n); k != "" {
						if _, c := nomiBest[k]; !c {
							nomiBest[k] = v.ID
						}
					}
				}
				if _, c := nomiVoce[v.ID]; !c {
					nomiVoce[v.ID] = v.Nome
				}
			}
		}
	})
}

// voceBestiarioPer: la voce del bestiario con questo nome (prima le voci personali del Master, poi quelle di base).
func (s *Server) voceBestiarioPer(nome string) (id, nomeVoce string) {
	id, nomeVoce, _ = s.voceBestiarioEsatta(nome)
	return
}

// voceBestiarioEsatta: come voceBestiarioPer, e dice se il nome coincide (esatto) o è solo simile.
func (s *Server) voceBestiarioEsatta(nome string) (id, nomeVoce string, esatto bool) {
	k := chiaveNome(nome)
	rows, err := s.DB.Query(`SELECT id, nome FROM bestiario_voci`)
	if err == nil {
		for rows.Next() {
			var i, n string
			rows.Scan(&i, &n)
			if id == "" && chiaveNome(n) == k {
				id, nomeVoce = i, n
			}
		}
		rows.Close()
	}
	if id != "" {
		return id, nomeVoce, true
	}
	s.caricaNomiBestiario()
	if i, ok := nomiBest[k]; ok {
		return i, nomiVoce[i], true
	}
	// Nomi storpiati dall'OCR («Drago dj rgento antico»): la voce con il nome più simile, se è chiaramente l'unica vicina.
	soglia := 1
	if n := len([]rune(k)); n > 16 {
		soglia = 3
	} else if n > 8 {
		soglia = 2
	}
	migliore, dMin, pari := "", soglia+1, false
	for nome, i := range nomiBest {
		d := distanzaNomi(k, nome, soglia)
		if d < dMin {
			migliore, dMin, pari = i, d, false
		} else if d == dMin && i != migliore {
			pari = true
		}
	}
	if migliore != "" && !pari {
		return migliore, nomiVoce[migliore], false
	}
	return "", "", false
}

// distanzaNomi: quante lettere bisogna cambiare, aggiungere o togliere per passare da a a b (Levenshtein); oltre max
// si ferma e restituisce max+1.
func distanzaNomi(a, b string, max int) int {
	x, y := []rune(a), []rune(b)
	if d := len(x) - len(y); d > max || -d > max {
		return max + 1
	}
	prec := make([]int, len(y)+1)
	cor := make([]int, len(y)+1)
	for j := range prec {
		prec[j] = j
	}
	for i := 1; i <= len(x); i++ {
		cor[0] = i
		minimo := cor[0]
		for j := 1; j <= len(y); j++ {
			c := 1
			if x[i-1] == y[j-1] {
				c = 0
			}
			cor[j] = min(prec[j]+1, cor[j-1]+1, prec[j-1]+c)
			minimo = min(minimo, cor[j])
		}
		if minimo > max {
			return max + 1
		}
		prec, cor = cor, prec
	}
	return prec[len(y)]
}

// schedaAttuale: la scheda in uso per una voce (quella salvata dal Master, altrimenti quella di base) e se è personale.
func (s *Server) schedaAttuale(voce string) (scheda string, personale bool) {
	var sd sql.NullString
	s.DB.QueryRow(`SELECT scheda_dati FROM bestiario_personalizzazioni WHERE voce_id = ?`, voce).Scan(&sd)
	if sd.Valid && sd.String != "" {
		return sd.String, true
	}
	if b := s.schedaBase(voce); len(b) > 0 {
		if v, err := validaScheda(b); err == nil {
			return v, false
		}
		return string(b), false
	}
	return "", false
}

// campiDiversi: i campi di due schede di creatura che cambiano (nomi italiani, per il riepilogo).
func campiDiversi(a, b string) []string {
	var x, y map[string]json.RawMessage
	json.Unmarshal([]byte(a), &x)
	json.Unmarshal([]byte(b), &y)
	nomi := map[string]string{"taglia": "Taglia", "tipo": "Tipo", "all": "Allineamento", "ca": "CA", "pf": "PF", "vel": "Velocità",
		"car": "Caratteristiche", "ts": "Tiri salvezza", "abil": "Abilità", "vuln": "Vulnerabilità", "res": "Resistenze", "imm": "Immunità",
		"immc": "Immunità alle condizioni", "sensi": "Sensi", "ling": "Linguaggi", "gs": "Sfida", "tratti": "Tratti", "azioni": "Azioni",
		"reazioni": "Reazioni", "legg": "Azioni leggendarie"}
	ordine := []string{"taglia", "tipo", "all", "ca", "pf", "vel", "car", "ts", "abil", "vuln", "res", "imm", "immc", "sensi", "ling", "gs", "tratti", "azioni", "reazioni", "legg"}
	var out []string
	for _, k := range ordine {
		if string(x[k]) != string(y[k]) {
			out = append(out, nomi[k])
		}
	}
	return out
}

// ===== API =====

func (s *Server) rotteImportazione(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/importazione/confronta", api(s.apiConfrontaImportazione))
	mux.Handle("GET /api/v1/importazione", api(s.apiRiepilogoImportazione))
	mux.Handle("POST /api/v1/importazione/applica", api(s.apiApplicaImportazione))
	mux.Handle("GET /api/v1/elementi-manuali", api(s.apiElementiManuali))
	mux.Handle("GET /api/v1/regole/incantesimi", api(s.apiIncantesimiImportati))
	mux.Handle("GET /api/v1/elementi-manuali/testo", api(s.apiTestoElemento))
}

// apiConfrontaImportazione riceve tutti i candidati letti e restituisce (e salva) il riepilogo.
func (s *Server) apiConfrontaImportazione(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Candidati []candidatoManuale `json:"candidati"`
	}
	if err := leggiJSONMax(r, &in, maxImportazione); err != nil {
		return nil, err
	}
	if len(in.Candidati) > maxCandidati {
		return nil, errore(http.StatusRequestEntityTooLarge, "Troppi elementi in una volta sola.")
	}
	manuali := map[string]string{}
	if rows, err := s.DB.Query(`SELECT id, nome FROM manuali`); err == nil {
		for rows.Next() {
			var i, n string
			rows.Scan(&i, &n)
			manuali[i] = n
		}
		rows.Close()
	}
	voci := make([]voceRiepilogo, 0, len(in.Candidati))
	visti := map[string]int{}
	for _, c := range in.Candidati {
		v := s.confronta(c)
		v.ManualeNome = manuali[c.Manuale]
		if v.Chiave == "" {
			continue
		}
		if i, ok := visti[c.Tipo+"|"+v.Chiave]; ok {
			v.Doppio = voci[i].ManualeNome + ", p. " + itoa(voci[i].Pagina)
		} else {
			visti[c.Tipo+"|"+v.Chiave] = len(voci)
		}
		voci = append(voci, v)
	}
	b, _ := json.Marshal(voci)
	s.DB.Exec(`DELETE FROM importazioni`)
	if _, err := s.DB.Exec(`INSERT INTO importazioni(id, creata_il, voci) VALUES(?,?,?)`, cripto.ID(), db.Ora(), string(b)); err != nil {
		return nil, err
	}
	return map[string]any{"voci": voci, "creata": db.Ora()}, nil
}

// confronta: stato di un candidato rispetto a ciò che Gaster Bless ha adesso.
func (s *Server) confronta(c candidatoManuale) voceRiepilogo {
	c.Nome = strings.TrimSpace(c.Nome)
	if len(c.Traccia) > 600 {
		c.Traccia = c.Traccia[:600]
	}
	if len(c.Motivi) > 10 {
		c.Motivi = c.Motivi[:10]
	}
	if len([]rune(c.Nome)) > 120 {
		c.Nome = string([]rune(c.Nome)[:120])
	}
	if len(c.Testo) > maxTestoElem {
		c.Testo = c.Testo[:maxTestoElem]
	}
	v := voceRiepilogo{candidatoManuale: c, Chiave: chiaveNome(c.Nome)}
	switch c.Tipo {
	case "mostro":
		nuova, err := validaScheda(c.Dati)
		if err != nil {
			v.Stato, v.Errore = "non_valido", err.Error()
			return v
		}
		v.Dati = json.RawMessage(nuova)
		var esatto bool
		v.Voce, v.NomeVoce, esatto = s.voceBestiarioEsatta(c.Nome)
		// nome preso dall'intestazione della pagina ma identico a una creatura del bestiario: non è più un dubbio
		if esatto && len(v.Motivi) == 1 && v.Motivi[0] == "nome indovinato" {
			v.Incerto, v.Motivi = false, nil
		}
		if v.Voce == "" {
			v.Stato = "nuovo"
			return v
		}
		attuale, personale := s.schedaAttuale(v.Voce)
		v.Attuale = json.RawMessage(attuale)
		if attuale == "" {
			attuale = "{}"
			v.Attuale = nil
		}
		var imp string
		s.DB.QueryRow(`SELECT impronta FROM elementi_manuali WHERE tipo = 'mostro' AND voce = ?`, v.Voce).Scan(&imp)
		v.Campi = campiDiversi(attuale, nuova)
		var notaAttuale struct {
			Nota string `json:"nota"`
		}
		json.Unmarshal([]byte(attuale), &notaAttuale)
		// stessi valori ma senza la provenienza (confermata prima che Gaster Bless la scrivesse): si propone di aggiornarla
		if len(v.Campi) == 0 && c.Manuale != "" && !strings.HasPrefix(notaAttuale.Nota, "Importata da") {
			v.Campi = []string{"Provenienza"}
		}
		switch {
		case len(v.Campi) == 0:
			v.Stato = "uguale"
		case personale && imp != improntaDati(attuale):
			v.Stato = "a_mano" // il Master l'ha scritta o corretta lui: di serie non si tocca
		default:
			v.Stato = "cambiato"
		}
	case "incantesimo":
		s.confrontaIncantesimo(&v)
	case "oggetto":
		s.confrontaOggetto(&v)
	case "regola":
		s.confrontaRegola(&v)
	default:
		v.Stato = "non_valido"
	}
	return v
}

// ===== Incantesimi (M3.2) =====

// valoriIncantesimo: i valori che vanno nelle schede (codici di web/regole/dati.js). Hi e Mat sono testo del manuale:
// li vede solo il Master.
type valoriIncantesimo struct {
	Voce string `json:"voce,omitempty"` // chiave inglese dell'archivio della scheda (vuota per un incantesimo nuovo)
	L    int    `json:"l"`
	Sc   string `json:"sc"`
	T    string `json:"t"`
	R    string `json:"r"`
	Cp   string `json:"cp"`
	D    string `json:"d"`
	Cl   string `json:"cl"`
	Hi   string `json:"hi,omitempty"`
	Mat  string `json:"mat,omitempty"`
}

var (
	scuoleValide = "ACDEVINT"
	classiValide = "abcdprswm"
)

func validaIncantesimo(raw json.RawMessage) (valoriIncantesimo, error) {
	var x valoriIncantesimo
	if len(raw) > 8<<10 || json.Unmarshal(raw, &x) != nil {
		return x, errDatiNonValidi
	}
	taglia := func(s string, n int) string {
		s = strings.TrimSpace(s)
		if r := []rune(s); len(r) > n {
			return string(r[:n])
		}
		return s
	}
	x.T, x.R, x.D, x.Hi, x.Mat, x.Voce = taglia(x.T, 40), taglia(x.R, 60), taglia(x.D, 40), taglia(x.Hi, 1500), taglia(x.Mat, 300), taglia(x.Voce, 60)
	if x.L < 0 || x.L > 9 || len(x.Sc) != 1 || !strings.Contains(scuoleValide, x.Sc) || len(x.Cp) > 3 || strings.Trim(x.Cp, "VSM") != "" {
		return x, errDatiNonValidi
	}
	cl := ""
	for _, c := range x.Cl {
		if strings.ContainsRune(classiValide, c) && !strings.ContainsRune(cl, c) {
			cl += string(c)
		}
	}
	x.Cl = cl
	return x, nil
}

// campiIncantesimo: che cosa cambia tra due versioni (nomi italiani, per il riepilogo). Le classi lette contano solo se
// aggiungono qualcosa (gli elenchi letti da un PDF possono essere incompleti).
func campiIncantesimo(a, b valoriIncantesimo) []string {
	var out []string
	for _, c := range []struct {
		x, y, n string
	}{{strconv.Itoa(a.L), strconv.Itoa(b.L), "Livello"}, {a.Sc, b.Sc, "Scuola"}, {a.T, b.T, "Tempo di lancio"}, {a.R, b.R, "Gittata"},
		{a.Cp, b.Cp, "Componenti"}, {a.D, b.D, "Durata"}} {
		if !strings.EqualFold(strings.Join(strings.Fields(c.x), " "), strings.Join(strings.Fields(c.y), " ")) {
			out = append(out, c.n)
		}
	}
	for _, c := range b.Cl {
		if !strings.ContainsRune(a.Cl, c) {
			out = append(out, "Classi")
			break
		}
	}
	return out
}

func (s *Server) confrontaIncantesimo(v *voceRiepilogo) {
	nuovo, err := validaIncantesimo(v.Dati)
	if err != nil || v.Chiave == "" {
		v.Stato, v.Errore = "non_valido", "valori dell'incantesimo non validi"
		return
	}
	var base valoriIncantesimo
	haBase := len(v.Base) > 0 && json.Unmarshal(v.Base, &base) == nil && base.Voce != ""
	if haBase {
		nuovo.Voce, v.Voce, v.NomeVoce = base.Voce, base.Voce, ""
	}
	b, _ := json.Marshal(nuovo)
	v.Dati = b
	// già importato: si confronta con quello (valori e testo)
	var dati, testo string
	if s.DB.QueryRow(`SELECT dati, testo FROM elementi_manuali WHERE tipo = 'incantesimo' AND chiave = ?`, v.Chiave).Scan(&dati, &testo) == nil {
		var prima valoriIncantesimo
		json.Unmarshal([]byte(dati), &prima)
		v.Attuale = json.RawMessage(dati)
		v.Campi = campiIncantesimo(prima, nuovo)
		if testo != v.Testo || prima.Hi != nuovo.Hi {
			v.Campi = append(v.Campi, "Testo del manuale")
		}
		v.Stato = "uguale"
		if len(v.Campi) > 0 {
			v.Stato = "cambiato"
		}
		return
	}
	if !haBase {
		v.Stato = "nuovo" // Gaster Bless non lo conosce: si aggiunge all'elenco delle schede
		return
	}
	ab, _ := json.Marshal(base)
	v.Attuale = ab
	v.Campi = append(campiIncantesimo(base, nuovo), "Testo del manuale") // il testo del manuale è sempre una novità
	v.Stato = "cambiato"
}

// ===== Oggetti magici (M3.3) =====

// valoriOggetto: i valori letti dalla riga sotto il nome («Oggetto meraviglioso, raro (richiede sintonia)»), con i codici
// di web/regole/oggetti.js. Vanno nell'archivio degli oggetti magici del Master; il testo del manuale resta a lui.
type valoriOggetto struct {
	Tipo      string `json:"tipo"`
	Sottotipo string `json:"sottotipo,omitempty"`
	Rar       string `json:"rar"`
	Sint      bool   `json:"sint"`
	SintDa    string `json:"sintDa,omitempty"` // «da parte di un incantatore»: solo per il riepilogo
	// bonus riconosciuti nel testo dalla pagina (OggettiMagici.suggerisci): modificatori tecnici e bonus di arma/armatura/scudo
	Mod   json.RawMessage `json:"mod,omitempty"`
	Bonus int             `json:"bonus,omitempty"`
}

var (
	slotDiTipo = map[string]string{"arma": "mani", "armatura": "armatura", "scudo": "scudo", "anello": "anello", "amuleto": "collo", "mantello": "mantello",
		"cintura": "cintura", "guanti": "guanti", "stivali": "stivali", "elmo": "testa", "bacchetta": "altro", "bastone": "mani", "meraviglioso": "altro",
		"accessorio": "altro", "altro": "altro"} // come SLOT_DI_TIPO di web/regole/oggetti.js
	raritaValide = map[string]bool{"comune": true, "non_comune": true, "raro": true, "molto_raro": true, "leggendario": true, "artefatto": true}
)

func validaValoriOggetto(raw json.RawMessage) (valoriOggetto, error) {
	var x valoriOggetto
	if len(raw) > 4<<10 || json.Unmarshal(raw, &x) != nil || slotDiTipo[x.Tipo] == "" || !raritaValide[x.Rar] {
		return x, errDatiNonValidi
	}
	x.Sottotipo = strings.TrimSpace(x.Sottotipo)
	if r := []rune(x.Sottotipo); len(r) > 80 {
		x.Sottotipo = string(r[:80])
	}
	if r := []rune(strings.TrimSpace(x.SintDa)); len(r) > 120 {
		x.SintDa = string(r[:120])
	}
	var mod []json.RawMessage // solo un elenco breve; i singoli modificatori li ricontrolla la scheda (sanificaMod)
	if len(x.Mod) == 0 || json.Unmarshal(x.Mod, &mod) != nil || len(mod) == 0 || len(mod) > 20 {
		x.Mod = nil
	}
	if x.Bonus < 0 || x.Bonus > 3 {
		x.Bonus = 0
	}
	return x, nil
}

// conBonusLetti mette nell'oggetto dell'archivio (mappa JSON) i bonus letti, senza toccare ciò che c'è già.
func conBonusLetti(m map[string]any, x valoriOggetto) {
	if len(x.Mod) > 0 {
		if l, _ := m["mod"].([]any); len(l) == 0 {
			var mod []any
			json.Unmarshal(x.Mod, &mod)
			m["mod"] = mod
		}
	}
	if x.Bonus > 0 {
		sotto := func(k string) map[string]any {
			o, _ := m[k].(map[string]any)
			if o == nil {
				o = map[string]any{}
				m[k] = o
			}
			return o
		}
		zero := func(v any) bool { f, _ := v.(float64); i, _ := v.(int); return f == 0 && i == 0 }
		switch x.Tipo {
		case "arma":
			if a := sotto("arma"); zero(a["bAtt"]) && zero(a["bDanno"]) {
				a["bAtt"], a["bDanno"] = x.Bonus, x.Bonus
			}
		case "armatura":
			if a := sotto("armatura"); zero(a["bonus"]) {
				a["bonus"] = x.Bonus
			}
		case "scudo":
			if a := sotto("scudo"); zero(a["bonus"]) {
				a["bonus"] = x.Bonus
			}
		}
	}
}

// valoriInArchivio: gli stessi valori presi da un oggetto dell'archivio.
func valoriInArchivio(dati string) valoriOggetto {
	var x valoriOggetto
	json.Unmarshal([]byte(dati), &x)
	x.SintDa = ""
	return x
}

func campiOggettoDiversi(a, b valoriOggetto) []string {
	var out []string
	if a.Tipo != b.Tipo {
		out = append(out, "Tipologia")
	}
	if !strings.EqualFold(strings.TrimSpace(a.Sottotipo), strings.TrimSpace(b.Sottotipo)) {
		out = append(out, "Sottotipo")
	}
	if a.Rar != b.Rar {
		out = append(out, "Rarità")
	}
	if a.Sint != b.Sint {
		out = append(out, "Sintonizzazione")
	}
	return out
}

// confrontaOggetto: con l'oggetto dell'archivio che ha lo stesso nome. «a_mano»: il Master l'ha cambiato dopo l'ultima
// importazione, oppure l'aveva creato lui con valori diversi.
func (s *Server) confrontaOggetto(v *voceRiepilogo) {
	nuovo, err := validaValoriOggetto(v.Dati)
	if err != nil || v.Chiave == "" {
		v.Stato, v.Errore = "non_valido", "valori dell'oggetto non validi"
		return
	}
	b, _ := json.Marshal(nuovo)
	v.Dati = b
	var prima, testoPrima string
	importato := s.DB.QueryRow(`SELECT dati, testo FROM elementi_manuali WHERE tipo = 'oggetto' AND chiave = ?`, v.Chiave).Scan(&prima, &testoPrima) == nil
	var id, nome, dati string
	if s.DB.QueryRow(`SELECT id, nome, dati FROM oggetti_magici WHERE chiave = ? ORDER BY aggiornato_il DESC LIMIT 1`, v.Chiave).Scan(&id, &nome, &dati) != nil {
		v.Stato = "nuovo" // non c'è nell'archivio (o il Master l'ha tolto): si crea
		return
	}
	att := valoriInArchivio(dati)
	ab, _ := json.Marshal(att)
	v.Voce, v.NomeVoce, v.Attuale = id, nome, ab
	v.Campi = campiOggettoDiversi(att, nuovo)
	var l []json.RawMessage
	var b2 struct {
		Arma     struct{ BAtt, BDanno float64 } `json:"arma"`
		Armatura struct{ Bonus float64 }        `json:"armatura"`
		Scudo    struct{ Bonus float64 }        `json:"scudo"`
	}
	json.Unmarshal([]byte(dati), &b2)
	senzaMod := json.Unmarshal(att.Mod, &l) != nil || len(l) == 0
	senzaBonus := b2.Arma.BAtt == 0 && b2.Arma.BDanno == 0 && b2.Armatura.Bonus == 0 && b2.Scudo.Bonus == 0
	// i bonus letti si aggiungono solo a un oggetto che non ne ha: non decidono «a mano»
	bonusNuovi := (len(nuovo.Mod) > 0 && senzaMod) || (nuovo.Bonus > 0 && senzaBonus)
	conBonus := func() {
		if bonusNuovi {
			v.Campi = append(v.Campi, "Bonus calcolati")
		}
	}
	if importato {
		if len(campiOggettoDiversi(valoriInArchivio(prima), att)) > 0 {
			v.Stato = "a_mano"
			return
		}
		conBonus()
		if testoPrima != v.Testo {
			v.Campi = append(v.Campi, "Testo del manuale")
		}
		v.Stato = "uguale"
		if len(v.Campi) > 0 {
			v.Stato = "cambiato"
		}
		return
	}
	if len(v.Campi) > 0 {
		v.Stato = "a_mano"
		return
	}
	conBonus()
	v.Campi = append(v.Campi, "Testo del manuale") // stessi valori: si collega il testo del manuale
	v.Stato = "cambiato"
}

// ===== Regole (M3.4) =====

// Le regole sono solo testo di consultazione per il Master (talenti, condizioni, background, razze, privilegi): non cambiano
// nessun calcolo della scheda. Si riconoscono dal nome già noto a Gaster Bless (la pagina conosce gli elenchi della scheda).
var sottotipiRegola = map[string]string{"talento": "Talento", "condizione": "Condizione", "background": "Background",
	"privilegio_background": "Privilegio di background", "razza": "Razza", "tratto_razziale": "Tratto razziale", "privilegio": "Privilegio di classe"}

func (s *Server) confrontaRegola(v *voceRiepilogo) {
	if sottotipiRegola[v.Sottotipo] == "" || v.Chiave == "" {
		v.Stato, v.Errore = "non_valido", "tipo di regola sconosciuto"
		return
	}
	if strings.TrimSpace(v.Testo) == "" {
		v.Stato, v.Errore = "non_valido", "testo non letto"
		return
	}
	b, _ := json.Marshal(map[string]string{"sottotipo": v.Sottotipo})
	v.Dati = b
	var prima string
	if s.DB.QueryRow(`SELECT testo FROM elementi_manuali WHERE tipo = 'regola' AND chiave = ?`, v.Chiave).Scan(&prima) != nil {
		v.Stato = "nuovo"
		return
	}
	v.Stato = "uguale"
	if prima != v.Testo {
		v.Stato, v.Campi = "cambiato", []string{"Testo del manuale"}
	}
}

func (s *Server) apiRiepilogoImportazione(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var voci string
	var creata int64
	if s.DB.QueryRow(`SELECT voci, creata_il FROM importazioni ORDER BY creata_il DESC LIMIT 1`).Scan(&voci, &creata) != nil {
		return map[string]any{"voci": []any{}}, nil
	}
	return map[string]any{"voci": json.RawMessage(voci), "creata": creata}, nil
}

// apiApplicaImportazione applica le voci scelte del riepilogo salvato ({"indici": [0, 3, ...]}).
func (s *Server) apiApplicaImportazione(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Indici []int `json:"indici"`
	}
	if err := leggiJSONMax(r, &in, 1<<20); err != nil {
		return nil, err
	}
	var testo, id string
	if s.DB.QueryRow(`SELECT id, voci FROM importazioni ORDER BY creata_il DESC LIMIT 1`).Scan(&id, &testo) != nil {
		return nil, errore(http.StatusConflict, "Non c'è un riepilogo da confermare: rifai l'importazione.")
	}
	var voci []voceRiepilogo
	if json.Unmarshal([]byte(testo), &voci) != nil {
		return nil, errore(http.StatusConflict, "Riepilogo non leggibile: rifai l'importazione.")
	}
	sort.Ints(in.Indici)
	fatte, saltate := 0, 0
	err := db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		for _, i := range in.Indici {
			if i < 0 || i >= len(voci) || voci[i].Applicata || voci[i].Stato == "non_valido" || voci[i].Stato == "uguale" {
				saltate++
				continue
			}
			if err := s.applicaVoce(tx, &voci[i]); err != nil {
				return err
			}
			voci[i].Applicata = true
			fatte++
		}
		b, _ := json.Marshal(voci)
		_, err := tx.Exec(`UPDATE importazioni SET voci = ? WHERE id = ?`, string(b), id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"applicate": fatte, "saltate": saltate}, nil
}

// applicaVoce salva un elemento e lo porta dove serve in Gaster Bless.
func (s *Server) applicaVoce(tx *sql.Tx, v *voceRiepilogo) error {
	ora := db.Ora()
	switch v.Tipo {
	case "mostro":
		// Provenienza scritta nella scheda (si vede nel bestiario e nella Mappa); il confronto delle importazioni la ignora.
		var conNota schedaCreatura
		if json.Unmarshal(v.Dati, &conNota) == nil && v.ManualeNome != "" {
			conNota.Nota = "Importata da «" + v.ManualeNome + "», pagina " + strconv.Itoa(v.Pagina) + " del PDF"
			if b, err := json.Marshal(conNota); err == nil {
				v.Dati = b
			}
		}
		sc, err := validaScheda(v.Dati)
		if err != nil {
			return nil
		}
		if v.Voce == "" { // creatura che Gaster Bless non conosce: voce nuova del Master
			v.Voce = cripto.ID()
			if _, err := tx.Exec(`INSERT INTO bestiario_voci(id, nome, cat, ruolo, base, creato_il) VALUES(?,?,?,?,?,?)`, v.Voce, v.Nome, "mostro", "", "", ora); err != nil {
				return err
			}
		}
		tx.Exec(`INSERT OR IGNORE INTO bestiario_personalizzazioni(voce_id) VALUES(?)`, v.Voce)
		if _, err := tx.Exec(`UPDATE bestiario_personalizzazioni SET scheda_dati = ?, manuale_id = ?, pagina = ? WHERE voce_id = ?`,
			sc, nullo(v.Manuale), paginaLibro(tx, v.Manuale, v.Pagina), v.Voce); err != nil {
			return err
		}
		ricordaPaginaIndice(tx, v.Manuale, v.Voce, v.Pagina)
		return salvaElemento(tx, v, sc, ora)
	case "incantesimo":
		x, err := validaIncantesimo(v.Dati)
		if err != nil {
			return nil
		}
		b, _ := json.Marshal(x)
		if v.Voce == "" {
			v.Voce = "man:" + v.Chiave // incantesimo nuovo: la scheda lo aggiunge all'elenco con questa chiave
		}
		return salvaElemento(tx, v, string(b), ora)
	case "oggetto":
		x, err := validaValoriOggetto(v.Dati)
		if err != nil {
			return nil
		}
		var dati string
		if v.Voce != "" && tx.QueryRow(`SELECT dati FROM oggetti_magici WHERE id = ?`, v.Voce).Scan(&dati) == nil {
			// oggetto già nell'archivio: cambiano solo i valori letti, la versione sale (chi lo ha lo riceve con «Consegna»)
			var m map[string]any
			if json.Unmarshal([]byte(dati), &m) != nil {
				return nil
			}
			if t, _ := m["tipo"].(string); t != x.Tipo {
				m["tipo"], m["slot"] = x.Tipo, slotDiTipo[x.Tipo]
			}
			if x.Sottotipo != "" {
				m["sottotipo"] = x.Sottotipo
			}
			m["rar"], m["sint"] = x.Rar, x.Sint
			conBonusLetti(m, x)
			ver, _ := m["v"].(float64)
			m["v"] = int(ver) + 1
			b, _ := json.Marshal(m)
			if _, err := tx.Exec(`UPDATE oggetti_magici SET dati = ?, manuale_nome = ?, pagina = ?, aggiornato_il = ? WHERE id = ?`, string(b), v.ManualeNome, v.Pagina, ora, v.Voce); err != nil {
				return err
			}
		} else {
			o, b, err := validaOggettoMagico(nuovoOggettoDaManuale(v.Nome, x))
			if err != nil {
				return nil
			}
			v.Voce = o.ID
			if _, err := tx.Exec(`INSERT INTO oggetti_magici(id, nome, chiave, dati, manuale_nome, pagina, creato_il, aggiornato_il) VALUES(?,?,?,?,?,?,?,?)`,
				o.ID, o.Nome, chiaveNome(o.Nome), string(b), v.ManualeNome, v.Pagina, ora, ora); err != nil {
				return err
			}
		}
		x.SintDa, x.Mod, x.Bonus = "", nil, 0 // nell'elemento solo i valori confrontati
		b, _ := json.Marshal(x)
		return salvaElemento(tx, v, string(b), ora)
	case "regola":
		if sottotipiRegola[v.Sottotipo] == "" || strings.TrimSpace(v.Testo) == "" {
			return nil
		}
		b, _ := json.Marshal(map[string]string{"sottotipo": v.Sottotipo})
		return salvaElemento(tx, v, string(b), ora)
	}
	return nil
}

// paginaLibro: la pagina del libro corrispondente a una pagina del PDF, secondo la calibrazione del manuale (pagina del
// libro di riferimento = pagina del PDF di riferimento, eventualmente due pagine del libro per pagina del PDF). Il bestiario
// salva le pagine del libro; la Mappa le riconverte (pdfPage in web/carta/carta.js).
func paginaLibro(tx *sql.Tx, manuale string, pdf int) int {
	var refLibro, refPdf, doppia int
	if tx.QueryRow(`SELECT ref_libro, ref_pdf, doppia FROM manuali WHERE id = ?`, manuale).Scan(&refLibro, &refPdf, &doppia) != nil {
		return pdf
	}
	libro := pdf - refPdf + refLibro
	if doppia == 1 {
		libro = 2 * (pdf - refPdf + refLibro/2)
	}
	if libro < 1 {
		return pdf
	}
	return libro
}

// ricordaPaginaIndice: la pagina del PDF finisce nell'indice del manuale, come per la ricerca automatica, così nel
// bestiario compare «📖 Apri a pag. N del PDF».
func ricordaPaginaIndice(tx *sql.Tx, manuale, voce string, pdf int) {
	var ind string
	if tx.QueryRow(`SELECT indice FROM manuali WHERE id = ?`, manuale).Scan(&ind) != nil {
		return
	}
	m := map[string]int{}
	json.Unmarshal([]byte(ind), &m)
	m[voce] = pdf
	if b, err := json.Marshal(m); err == nil && len(b) < 512<<10 {
		tx.Exec(`UPDATE manuali SET indice = ? WHERE id = ?`, string(b), manuale)
	}
}

func salvaElemento(tx *sql.Tx, v *voceRiepilogo, dati string, ora int64) error {
	_, err := tx.Exec(`INSERT INTO elementi_manuali(id, tipo, sottotipo, chiave, nome, voce, manuale_id, manuale_nome, pagina, dati, testo, impronta, importato_il, aggiornato_il)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(tipo, chiave) DO UPDATE SET sottotipo = excluded.sottotipo, nome = excluded.nome, voce = excluded.voce, manuale_id = excluded.manuale_id,
		  manuale_nome = excluded.manuale_nome, pagina = excluded.pagina, dati = excluded.dati, testo = excluded.testo, impronta = excluded.impronta,
		  aggiornato_il = excluded.aggiornato_il`,
		cripto.ID(), v.Tipo, v.Sottotipo, v.Chiave, v.Nome, v.Voce, nullo(v.Manuale), v.ManualeNome, v.Pagina, dati, v.Testo, improntaDati(dati), ora, ora)
	return err
}

// apiElementiManuali: gli elementi importati (senza il testo, che si chiede uno per uno con ?id=).
func (s *Server) apiElementiManuali(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	q := r.URL.Query()
	if id := q.Get("id"); id != "" {
		var e struct {
			Nome, Testo, ManualeNome string
			Pagina                   int
		}
		if s.DB.QueryRow(`SELECT nome, testo, manuale_nome, pagina FROM elementi_manuali WHERE id = ?`, id).Scan(&e.Nome, &e.Testo, &e.ManualeNome, &e.Pagina) != nil {
			return nil, errNonTrovato
		}
		return map[string]any{"nome": e.Nome, "testo": e.Testo, "manualeNome": e.ManualeNome, "pagina": e.Pagina}, nil
	}
	sqlq := `SELECT id, tipo, sottotipo, nome, voce, COALESCE(manuale_id,''), manuale_nome, pagina, aggiornato_il FROM elementi_manuali`
	var args []any
	if t := q.Get("tipo"); t != "" {
		sqlq += ` WHERE tipo = ?`
		args = append(args, t)
	}
	rows, err := s.DB.Query(sqlq+` ORDER BY tipo, nome`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, tipo, sotto, nome, voce, man, manNome string
		var pag int
		var agg int64
		rows.Scan(&id, &tipo, &sotto, &nome, &voce, &man, &manNome, &pag, &agg)
		out = append(out, map[string]any{"id": id, "tipo": tipo, "sottotipo": sotto, "nome": nome, "voce": voce, "manuale": man,
			"manualeNome": manNome, "eliminato": man == "", "pagina": pag, "aggiornato": agg})
	}
	return out, nil
}

// apiIncantesimiImportati: i valori degli incantesimi importati, per le schede di tutti (senza il testo del manuale).
func (s *Server) apiIncantesimiImportati(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "approvato"); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT nome, voce, dati, manuale_nome, pagina FROM elementi_manuali WHERE tipo = 'incantesimo' ORDER BY nome`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var nome, voce, dati, man string
		var pag int
		rows.Scan(&nome, &voce, &dati, &man, &pag)
		var x valoriIncantesimo
		if json.Unmarshal([]byte(dati), &x) != nil {
			continue
		}
		out = append(out, map[string]any{"nome": nome, "voce": voce, "l": x.L, "sc": x.Sc, "t": x.T, "r": x.R, "cp": x.Cp, "d": x.D, "cl": x.Cl,
			"fonte": man, "pagina": pag, "nuovo": strings.HasPrefix(voce, "man:")})
	}
	return out, nil
}

// apiTestoElemento: il testo del manuale di un elemento (solo il Master), per voce o per nome: ?tipo=incantesimo&voce=Fireball.
func (s *Server) apiTestoElemento(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	q := r.URL.Query()
	var nome, testo, dati, man string
	var pag int
	err := s.DB.QueryRow(`SELECT nome, testo, dati, manuale_nome, pagina FROM elementi_manuali WHERE tipo = ? AND (voce = ? AND ? <> '' OR chiave = ?) LIMIT 1`,
		q.Get("tipo"), q.Get("voce"), q.Get("voce"), chiaveNome(q.Get("nome"))).Scan(&nome, &testo, &dati, &man, &pag)
	if err != nil {
		return nil, errNonTrovato
	}
	var x valoriIncantesimo
	json.Unmarshal([]byte(dati), &x)
	return map[string]any{"nome": nome, "testo": testo, "superiori": x.Hi, "materiali": x.Mat, "manualeNome": man, "pagina": pag}, nil
}
