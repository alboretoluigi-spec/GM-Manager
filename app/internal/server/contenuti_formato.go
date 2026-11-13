package server

// Formato aperto dei pacchetti di contenuti «gasterbless-contenuti», versione 1 (fase 3c, passo C1; descrizione
// completa per gli utenti in docs/FORMATO_CONTENUTI.md). Il file usa nomi di campo leggibili in italiano; qui si
// controlla ogni voce e la si converte nella forma compatta che la scheda della quinta edizione legge da
// web/regole/dati.js e web/regole/talenti.js. Nessun contenuto è incluso nell'app: arriva tutto dal file dell'utente.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	formatoContenuti         = "gasterbless-contenuti"
	versioneFormatoContenuti = 1
	sistemaQuintaEdizione    = "d20-5e"
	maxVociPerTipo           = 5000
	maxTestoContenuto        = 20000
)

var (
	caratteristiche5e = []string{"FOR", "DES", "COS", "INT", "SAG", "CAR"}
	abilita5e         = []string{"Acrobazia", "Addestrare Animali", "Arcano", "Atletica", "Furtività", "Indagare", "Inganno",
		"Intimidire", "Intrattenere", "Intuizione", "Medicina", "Natura", "Percezione", "Persuasione", "Rapidità di Mano",
		"Religione", "Sopravvivenza", "Storia"}
	reIDContenuto = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)
	// Codici delle liste di incantesimi già noti alla scheda (artefice, bardo, chierico, druido, paladino, ranger,
	// stregone, warlock, mago); un pacchetto può aggiungerne altri con «listeIncantesimi».
	listeIncantesimiBase = map[string]string{"a": "Artefice", "b": "Bardo", "c": "Chierico", "d": "Druido", "p": "Paladino",
		"r": "Ranger", "s": "Stregone", "w": "Warlock", "m": "Mago"}
	// Comportamenti automatici della scheda che una sottoclasse o una razza può attivare (vedi il documento del formato).
	agganciSottoclasse = []string{"moon", "champ", "bm", "psi", "rune", "drac", "hex", "initCar", "initInt", "initSag"}
	agganciRazza       = []string{"dragon", "dwarf", "tief", "drow", "lineage"}
	effettiTalento     = []string{"init", "speed", "hpLvl", "pass", "saveAb", "armor", "medDex3", "unarmored", "res"}
)

// ----- Struttura del file (campi leggibili) -----

type pacchettoFile struct {
	Formato          string            `json:"formato"`
	VersioneFormato  int               `json:"versioneFormato"`
	Sistema          string            `json:"sistema"`
	Nome             string            `json:"nome"`
	Autore           string            `json:"autore"`
	Versione         string            `json:"versione"`
	Licenza          string            `json:"licenza"`
	Descrizione      string            `json:"descrizione"`
	Fonti            map[string]string `json:"fonti"`
	ListeIncantesimi map[string]string `json:"listeIncantesimi"`
	ConosciutiTerzo  []int             `json:"conosciutiSottoclassiIncantatrici"`
	DescrGenerali    map[string]string `json:"descrizioniGenerali"`
	DescrRisorse     map[string]string `json:"descrizioniRisorse"`
	Classi           []json.RawMessage `json:"classi"`
	Razze            []json.RawMessage `json:"razze"`
	Background       []json.RawMessage `json:"background"`
	Incantesimi      []json.RawMessage `json:"incantesimi"`
	Armi             []json.RawMessage `json:"armi"`
	Armature         []json.RawMessage `json:"armature"`
	Talenti          []json.RawMessage `json:"talenti"`
	Stili            []json.RawMessage `json:"stili"`
}

type incantatoreFile struct {
	Tipo           string `json:"tipo"`
	Caratteristica string `json:"caratteristica"`
	Preparazione   string `json:"preparazione"`
	Lista          string `json:"lista"`
	Conosciuti     []int  `json:"conosciuti"`
	Trucchetti     []int  `json:"trucchetti"`
}

type sottoclasseFile struct {
	Nome                 string              `json:"nome"`
	Fonte                string              `json:"fonte"`
	Privilegi            map[string][]string `json:"privilegi"`
	Descrizioni          []string            `json:"descrizioni"`
	Armature             []string            `json:"armature"`
	Armi                 []string            `json:"armi"`
	ArmiSpecifiche       []string            `json:"armiSpecifiche"`
	AbilitaDaScegliere   int                 `json:"abilitaDaScegliere"`
	AbilitaFisse         []string            `json:"abilitaFisse"`
	MaestrieExtra        int                 `json:"maestrieExtra"`
	MaestrieFisse        []string            `json:"maestrieFisse"`
	AttaccoExtra         bool                `json:"attaccoExtra"`
	StiliDiCombattimento int                 `json:"stiliDiCombattimento"`
	Incantatore          *incantatoreFile    `json:"incantatore"`
	Agganci              map[string]int      `json:"agganci"`
}

type classeFile struct {
	ID                   string              `json:"id"`
	Nome                 string              `json:"nome"`
	Fonte                string              `json:"fonte"`
	DadoVita             int                 `json:"dadoVita"`
	TiriSalvezza         []string            `json:"tiriSalvezza"`
	Armature             []string            `json:"armature"`
	Armi                 []string            `json:"armi"`
	ArmiSpecifiche       []string            `json:"armiSpecifiche"`
	TestoArmature        string              `json:"testoArmature"`
	TestoArmi            string              `json:"testoArmi"`
	Strumenti            string              `json:"strumenti"`
	AbilitaDaScegliere   int                 `json:"abilitaDaScegliere"`
	Abilita              []string            `json:"abilita"`
	Lingue               []string            `json:"lingue"`
	LivelliAumento       []int               `json:"livelliAumento"`
	StiliDiCombattimento int                 `json:"stiliDiCombattimento"`
	Incantatore          *incantatoreFile    `json:"incantatore"`
	NomeSottoclasse      string              `json:"nomeSottoclasse"`
	LivelloSottoclasse   int                 `json:"livelloSottoclasse"`
	Privilegi            map[string][]string `json:"privilegi"`
	PrivilegiOpzionali   map[string][]string `json:"privilegiOpzionali"`
	Equipaggiamento      []string            `json:"equipaggiamento"`
	Descrizioni          map[string]string   `json:"descrizioni"`
	Sottoclassi          []json.RawMessage   `json:"sottoclassi"`
}

type bonusLiberiFile struct {
	Numero  int      `json:"numero"`
	Valore  int      `json:"valore"`
	Escluse []string `json:"escluse"`
}

type magiaInnataFile struct {
	Nome           string `json:"nome"`
	Caratteristica string `json:"caratteristica"`
	Incantesimi    []struct {
		Livello int    `json:"livello"`
		Testo   string `json:"testo"`
	} `json:"incantesimi"`
}

type razzaFile struct {
	ID                 string            `json:"id"`
	Nome               string            `json:"nome"`
	Fonte              string            `json:"fonte"`
	Caratteristiche    map[string]int    `json:"caratteristiche"`
	Velocita           *float64          `json:"velocita"`
	Taglia             string            `json:"taglia"`
	Scurovisione       float64           `json:"scurovisione"`
	Lingue             []string          `json:"lingue"`
	LingueExtra        int               `json:"lingueExtra"`
	AbilitaFisse       []string          `json:"abilitaFisse"`
	AbilitaDaScegliere int               `json:"abilitaDaScegliere"`
	BonusLiberi        *bonusLiberiFile  `json:"bonusLiberi"`
	BonusLiberiValori  []int             `json:"bonusLiberiValori"`
	BonusFissi         bool              `json:"bonusFissi"`
	Tratti             []string          `json:"tratti"`
	MagiaInnata        *magiaInnataFile  `json:"magiaInnata"`
	ArmiSpecifiche     []string          `json:"armiSpecifiche"`
	Armature           []string          `json:"armature"`
	Strumenti          string            `json:"strumenti"`
	Talento            bool              `json:"talento"`
	ArmaturaNaturale   int               `json:"armaturaNaturale"`
	PFPerLivello       int               `json:"pfPerLivello"`
	Agganci            map[string]int    `json:"agganci"`
	Descrizioni        map[string]string `json:"descrizioni"`
	Sottorazze         []json.RawMessage `json:"sottorazze"`
}

type backgroundFile struct {
	ID                    string   `json:"id"`
	Nome                  string   `json:"nome"`
	Abilita               []string `json:"abilita"`
	AbilitaDaScegliere    int      `json:"abilitaDaScegliere"`
	LingueExtra           int      `json:"lingueExtra"`
	Strumenti             string   `json:"strumenti"`
	Privilegio            string   `json:"privilegio"`
	DescrizionePrivilegio string   `json:"descrizionePrivilegio"`
	Equipaggiamento       string   `json:"equipaggiamento"`
	Monete                int      `json:"monete"`
}

type incantesimoFile struct {
	Nome             string          `json:"nome"`
	NomeInglese      string          `json:"nomeInglese"`
	Livello          *int            `json:"livello"`
	Scuola           string          `json:"scuola"`
	Tempo            string          `json:"tempo"`
	Rituale          bool            `json:"rituale"`
	Gittata          string          `json:"gittata"`
	Componenti       string          `json:"componenti"`
	Durata           string          `json:"durata"`
	Concentrazione   bool            `json:"concentrazione"`
	Classi           json.RawMessage `json:"classi"`
	Fonte            string          `json:"fonte"`
	Descrizione      string          `json:"descrizione"`
	LivelliSuperiori string          `json:"livelliSuperiori"`
}

type armaFile struct {
	Nome      string `json:"nome"`
	Categoria string `json:"categoria"`
	Tipo      string `json:"tipo"`
	Danno     string `json:"danno"`
	TipoDanno string `json:"tipoDanno"`
	Proprieta string `json:"proprieta"`
	Accurata  bool   `json:"accurata"`
	Leggera   bool   `json:"leggera"`
	Pesante   bool   `json:"pesante"`
	DueMani   bool   `json:"dueMani"`
	Lancio    bool   `json:"lancio"`
	Versatile bool   `json:"versatile"`
	Portata   bool   `json:"portata"`
}

type armaturaFile struct {
	Nome                string `json:"nome"`
	Tipo                string `json:"tipo"`
	CA                  int    `json:"ca"`
	Forza               int    `json:"forza"`
	SvantaggioFurtivita bool   `json:"svantaggioFurtivita"`
}

type talentoFile struct {
	Nome            string          `json:"nome"`
	NomeInglese     string          `json:"nomeInglese"`
	Fonte           string          `json:"fonte"`
	Caratteristiche json.RawMessage `json:"caratteristiche"`
	Effetti         map[string]any  `json:"effetti"`
	Descrizione     string          `json:"descrizione"`
}

type stileFile struct {
	Nome        string `json:"nome"`
	Descrizione string `json:"descrizione"`
	Opzionale   bool   `json:"opzionale"`
}

// ----- Forma per la scheda (web/regole/dati.js, web/regole/talenti.js) -----

type datiScheda struct {
	Classi      map[string]map[string]any `json:"classi"`
	Razze       map[string]map[string]any `json:"razze"`
	Background  map[string]map[string]any `json:"bg"`
	Incantesimi [][]any                   `json:"incantesimi"`
	Armi        [][]any                   `json:"armi"`
	Armature    [][]any                   `json:"armature"`
	Stili       [][]any                   `json:"stili"`
	StiliT      [][]any                   `json:"stiliT"`
	Talenti     [][]any                   `json:"talenti"`
	Conosciuti  map[string][]int          `json:"conosciuti"`
	Trucchetti  map[string][]int          `json:"trucchetti"`
	Codici      map[string]string         `json:"codici"`
	NomiListe   map[string]string         `json:"nomiListe"`
	Fonti       map[string]string         `json:"fonti"`
	FDGen       map[string]string         `json:"fdGen"`
	FDCls       map[string]string         `json:"fdCls"`
	FDSub       map[string][]string       `json:"fdSub"`
	FDRace      map[string]string         `json:"fdRace"`
	FDBg        map[string]string         `json:"fdBg"`
	FDRes       map[string]string         `json:"fdRes"`
}

func nuoviDatiScheda() *datiScheda {
	return &datiScheda{Classi: map[string]map[string]any{}, Razze: map[string]map[string]any{}, Background: map[string]map[string]any{},
		Incantesimi: [][]any{}, Armi: [][]any{}, Armature: [][]any{}, Stili: [][]any{}, StiliT: [][]any{}, Talenti: [][]any{},
		Conosciuti: map[string][]int{}, Trucchetti: map[string][]int{}, Codici: map[string]string{}, NomiListe: map[string]string{},
		Fonti: map[string]string{}, FDGen: map[string]string{}, FDCls: map[string]string{}, FDSub: map[string][]string{},
		FDRace: map[string]string{}, FDBg: map[string]string{}, FDRes: map[string]string{}}
}

// pacchettoLetto: intestazione, dati convertiti, conteggi per tipo e avvisi (problemi che non impediscono l'importazione).
type pacchettoLetto struct {
	Nome, Sistema, Autore, Versione, Licenza, Descrizione string
	Dati                                                  *datiScheda
	Conteggi                                              map[string]int
	Avvisi                                                []string
}

// controllore raccoglie gli errori con il punto preciso del file in cui si trovano.
type controllore struct {
	errori []string
	avvisi []string
	dove   string
}

func (c *controllore) err(f string, a ...any) {
	if len(c.errori) < 50 {
		c.errori = append(c.errori, c.dove+fmt.Sprintf(f, a...))
	}
}
func (c *controllore) avv(f string, a ...any) {
	if len(c.avvisi) < 100 {
		c.avvisi = append(c.avvisi, c.dove+fmt.Sprintf(f, a...))
	}
}

func (c *controllore) testo(campo, v string, obbligatorio bool) string {
	v = strings.TrimSpace(v)
	if obbligatorio && v == "" {
		c.err("manca «%s»", campo)
	}
	if utf8.RuneCountInString(v) > maxTestoContenuto {
		c.err("«%s» è troppo lungo (al massimo %d caratteri)", campo, maxTestoContenuto)
	}
	return v
}

func (c *controllore) caratteristica(campo, v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	for _, x := range caratteristiche5e {
		if x == v {
			return v
		}
	}
	c.err("«%s»: «%s» non è una caratteristica (usa FOR, DES, COS, INT, SAG o CAR)", campo, v)
	return v
}

func (c *controllore) caratteristiche(campo string, v []string) []string {
	out := []string{}
	for _, x := range v {
		out = append(out, c.caratteristica(campo, x))
	}
	return out
}

func (c *controllore) codici(campo string, v []string, ammessi ...string) []string {
	out := []string{}
	for _, x := range v {
		x = strings.ToLower(strings.TrimSpace(x))
		ok := false
		for _, a := range ammessi {
			ok = ok || a == x
		}
		if !ok {
			c.err("«%s»: «%s» non è ammesso (valori possibili: %s)", campo, x, strings.Join(ammessi, ", "))
		}
		out = append(out, x)
	}
	return out
}

func (c *controllore) abilita(campo string, v []string) []string {
	out := []string{}
	for _, x := range v {
		x = strings.TrimSpace(x)
		noto := false
		for _, a := range abilita5e {
			noto = noto || a == x
		}
		if !noto {
			c.avv("«%s»: l'abilità «%s» non è tra quelle della scheda; la scheda non potrà calcolarla", campo, x)
		}
		out = append(out, x)
	}
	return out
}

func (c *controllore) elencoTesti(campo string, v []string) []string {
	out := []string{}
	for _, x := range v {
		if x = c.testo(campo, x, false); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func (c *controllore) intervallo(campo string, v, min, max int) int {
	if v < min || v > max {
		c.err("«%s» deve essere tra %d e %d (è %d)", campo, min, max, v)
	}
	return v
}

// privilegi {"1": ["A", "B"], "3": ["C"]} → "1:A|B;3:C", la forma letta dalla scheda.
func (c *controllore) privilegi(campo string, m map[string][]string) string {
	type liv struct {
		l int
		n []string
	}
	ll := []liv{}
	for k, v := range m {
		l, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil || l < 1 || l > 20 {
			c.err("«%s»: «%s» non è un livello da 1 a 20", campo, k)
			continue
		}
		nomi := []string{}
		for _, n := range v {
			n = c.testo(campo, n, false)
			if strings.ContainsAny(n, "|;") {
				c.err("«%s»: il nome «%s» non può contenere | o ;", campo, n)
			}
			if n != "" {
				nomi = append(nomi, n)
			}
		}
		if len(nomi) > 0 {
			ll = append(ll, liv{l, nomi})
		}
	}
	sort.Slice(ll, func(i, j int) bool { return ll[i].l < ll[j].l })
	p := []string{}
	for _, x := range ll {
		p = append(p, strconv.Itoa(x.l)+":"+strings.Join(x.n, "|"))
	}
	return strings.Join(p, ";")
}

// decodifica legge una voce senza accettare campi sconosciuti (un nome scritto male si scopre subito).
func decodifica(raw json.RawMessage, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return erroreJSONLeggibile(err)
	}
	return nil
}

func erroreJSONLeggibile(err error) error {
	var te *json.UnmarshalTypeError
	var se *json.SyntaxError
	switch {
	case errors.As(err, &te):
		tipo := map[string]string{"string": "un testo tra virgolette", "int": "un numero intero", "float64": "un numero",
			"bool": "true o false", "slice": "un elenco [ ]", "map": "un oggetto { }", "struct": "un oggetto { }"}[te.Type.Kind().String()]
		if tipo == "" {
			tipo = te.Type.String()
		}
		campo := te.Field
		if campo == "" {
			campo = "voce"
		}
		return fmt.Errorf("«%s» deve essere %s", campo, tipo)
	case errors.As(err, &se):
		return fmt.Errorf("il file non è un JSON valido (carattere %d)", se.Offset)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return fmt.Errorf("campo sconosciuto %s (controlla come è scritto)", strings.TrimPrefix(err.Error(), "json: unknown field "))
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return fmt.Errorf("il file è vuoto o incompleto")
	}
	return err
}

func nomeVoce(raw json.RawMessage) string {
	var x struct {
		Nome string `json:"nome"`
		ID   string `json:"id"`
	}
	json.Unmarshal(raw, &x)
	if x.Nome != "" {
		return " («" + accorcia(x.Nome, 40) + "»)"
	}
	if x.ID != "" {
		return " («" + accorcia(x.ID, 40) + "»)"
	}
	return ""
}

// leggiPacchetto controlla l'intero file e lo converte. Restituisce tutti gli errori trovati (fino a 50) in un colpo.
func leggiPacchetto(b []byte) (*pacchettoLetto, error) {
	var p pacchettoFile
	if err := decodifica(b, &p); err != nil {
		return nil, fmt.Errorf("Il file non è un pacchetto di contenuti valido: %v.", err)
	}
	c := &controllore{}
	if p.Formato != formatoContenuti {
		return nil, fmt.Errorf("Il file non è un pacchetto di contenuti di Gaster Bless: «formato» deve essere «%s».", formatoContenuti)
	}
	if p.VersioneFormato != versioneFormatoContenuti {
		return nil, fmt.Errorf("Versione del formato %d non supportata: questa versione dell'app legge la versione %d.", p.VersioneFormato, versioneFormatoContenuti)
	}
	if p.Sistema != sistemaQuintaEdizione {
		return nil, fmt.Errorf("Sistema di gioco «%s» non ancora supportato: per ora i pacchetti valgono per «%s» (quinta edizione).", p.Sistema, sistemaQuintaEdizione)
	}
	out := &pacchettoLetto{Sistema: p.Sistema, Dati: nuoviDatiScheda(), Conteggi: map[string]int{}}
	out.Nome = c.testo("nome", p.Nome, true)
	out.Autore = c.testo("autore", p.Autore, false)
	out.Versione = c.testo("versione", p.Versione, false)
	out.Licenza = c.testo("licenza", p.Licenza, false)
	out.Descrizione = c.testo("descrizione", p.Descrizione, false)
	d := out.Dati
	for k, v := range p.Fonti {
		d.Fonti[c.testo("fonti", k, true)] = c.testo("fonti", v, true)
	}
	for k, v := range p.ListeIncantesimi {
		if utf8.RuneCountInString(k) != 1 || k < "a" || k > "z" {
			c.err("«listeIncantesimi»: il codice «%s» deve essere una sola lettera minuscola", k)
		}
		d.NomiListe[k] = c.testo("listeIncantesimi", v, true)
	}
	if len(p.ConosciutiTerzo) > 0 {
		if len(p.ConosciutiTerzo) != 20 {
			c.err("«conosciutiSottoclassiIncantatrici» deve avere 20 numeri, uno per livello")
		}
		d.Conosciuti["third"] = p.ConosciutiTerzo
	}
	for k, v := range p.DescrGenerali {
		d.FDGen[c.testo("descrizioniGenerali", k, true)] = c.testo("descrizioniGenerali", v, false)
	}
	for k, v := range p.DescrRisorse {
		d.FDRes[c.testo("descrizioniRisorse", k, true)] = c.testo("descrizioniRisorse", v, false)
	}
	tipi := []struct {
		nome string
		voci []json.RawMessage
		fn   func(*controllore, json.RawMessage, *datiScheda) error
	}{
		{"classi", p.Classi, convertiClasse}, {"razze", p.Razze, convertiRazza}, {"background", p.Background, convertiBackground},
		{"incantesimi", p.Incantesimi, convertiIncantesimo}, {"armi", p.Armi, convertiArma}, {"armature", p.Armature, convertiArmatura},
		{"talenti", p.Talenti, convertiTalento}, {"stili", p.Stili, convertiStile},
	}
	totale := 0
	for _, t := range tipi {
		if len(t.voci) > maxVociPerTipo {
			c.err("«%s»: troppe voci (al massimo %d)", t.nome, maxVociPerTipo)
			continue
		}
		for i, raw := range t.voci {
			c.dove = fmt.Sprintf("%s n. %d%s: ", t.nome, i+1, nomeVoce(raw))
			if err := t.fn(c, raw, d); err != nil {
				c.err("%v", err)
			}
		}
		c.dove = ""
		out.Conteggi[t.nome] = len(t.voci)
		totale += len(t.voci)
	}
	if totale == 0 && len(c.errori) == 0 {
		c.err("il pacchetto non contiene nessuna voce (classi, razze, background, incantesimi, armi, armature, talenti o stili)")
	}
	// Codici di lista usati dalle classi e dagli incantesimi: devono esistere (base o dichiarati nel pacchetto).
	for id, cod := range d.Codici {
		if listeIncantesimiBase[cod] == "" && d.NomiListe[cod] == "" {
			c.err("classe «%s»: la lista di incantesimi «%s» non esiste; dichiarala in «listeIncantesimi»", id, cod)
		}
	}
	if len(c.errori) > 0 {
		return nil, fmt.Errorf("Il pacchetto contiene errori:\n• %s", strings.Join(c.errori, "\n• "))
	}
	out.Avvisi = c.avvisi
	return out, nil
}

func (c *controllore) incantatore(campo string, x *incantatoreFile, sottoclasse bool) map[string]any {
	m := map[string]any{}
	tipi := []string{"full", "half", "third", "pact", "art"}
	if sottoclasse {
		tipi = []string{"third"}
	}
	m["cast"] = c.codici(campo+".tipo", []string{x.Tipo}, tipi...)[0]
	m["sab"] = c.caratteristica(campo+".caratteristica", x.Caratteristica)
	if !sottoclasse {
		m["prep"] = c.codici(campo+".preparazione", []string{x.Preparazione}, "full", "half", "known")[0]
	}
	return m
}

func convertiClasse(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x classeFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	if !reIDContenuto.MatchString(x.ID) {
		c.err("«id» deve essere fatto di lettere minuscole senza accenti, cifre, - o _ (es. «guerriero»)")
	}
	k := map[string]any{
		"n": c.testo("nome", x.Nome, true), "src": c.testo("fonte", x.Fonte, false),
		"hd": c.intervallo("dadoVita", x.DadoVita, 4, 20), "saves": c.caratteristiche("tiriSalvezza", x.TiriSalvezza),
		"armorP": c.codici("armature", x.Armature, "l", "m", "p", "sc"), "weapP": c.codici("armi", x.Armi, "s", "g"),
		"weapN": c.elencoTesti("armiSpecifiche", x.ArmiSpecifiche), "armorT": c.testo("testoArmature", x.TestoArmature, false),
		"weapT": c.testo("testoArmi", x.TestoArmi, false), "tools": c.testo("strumenti", x.Strumenti, false),
		"skN": c.intervallo("abilitaDaScegliere", x.AbilitaDaScegliere, 0, 18), "sk": c.abilita("abilita", x.Abilita),
		"subT": c.testo("nomeSottoclasse", x.NomeSottoclasse, false), "subL": 3,
		"f": c.privilegi("privilegi", x.Privilegi), "eq": strings.Join(c.elencoTesti("equipaggiamento", x.Equipaggiamento), "\n"),
		"subs": []any{},
	}
	if x.LivelloSottoclasse != 0 {
		k["subL"] = c.intervallo("livelloSottoclasse", x.LivelloSottoclasse, 1, 20)
	}
	if len(x.PrivilegiOpzionali) > 0 {
		k["topt"] = c.privilegi("privilegiOpzionali", x.PrivilegiOpzionali)
	}
	if len(x.Lingue) > 0 {
		k["lang"] = c.elencoTesti("lingue", x.Lingue)
	}
	if len(x.LivelliAumento) > 0 {
		for _, l := range x.LivelliAumento {
			c.intervallo("livelliAumento", l, 1, 20)
		}
		k["asi"] = x.LivelliAumento
	}
	if x.StiliDiCombattimento > 0 {
		k["style"] = c.intervallo("stiliDiCombattimento", x.StiliDiCombattimento, 1, 20)
	}
	if x.Incantatore != nil {
		for kk, v := range c.incantatore("incantatore", x.Incantatore, false) {
			k[kk] = v
		}
		if l := strings.TrimSpace(x.Incantatore.Lista); l != "" {
			d.Codici[x.ID] = l
		}
		if n := len(x.Incantatore.Conosciuti); n > 0 {
			if n != 20 {
				c.err("«incantatore.conosciuti» deve avere 20 numeri, uno per livello")
			}
			d.Conosciuti[x.ID] = x.Incantatore.Conosciuti
		}
		if n := len(x.Incantatore.Trucchetti); n > 0 {
			if n != 3 {
				c.err("«incantatore.trucchetti» deve avere 3 numeri: trucchetti al 1° livello e i due livelli in cui ne guadagni uno in più")
			}
			d.Trucchetti[x.ID] = x.Incantatore.Trucchetti
		}
	}
	for n, t := range x.Descrizioni {
		d.FDCls[x.ID+"|"+c.testo("descrizioni", n, true)] = c.testo("descrizioni", t, false)
	}
	subs := []any{}
	for i, rs := range x.Sottoclassi {
		var s sottoclasseFile
		if err := decodifica(rs, &s); err != nil {
			c.err("sottoclasse n. %d: %v", i+1, err)
			continue
		}
		nome := c.testo("sottoclassi.nome", s.Nome, true)
		extra := map[string]any{}
		if len(s.Armature) > 0 {
			extra["armorP"] = c.codici("sottoclassi.armature", s.Armature, "l", "m", "p", "sc")
		}
		if len(s.Armi) > 0 {
			extra["weapP"] = c.codici("sottoclassi.armi", s.Armi, "s", "g")
		}
		if len(s.ArmiSpecifiche) > 0 {
			extra["weapN"] = c.elencoTesti("sottoclassi.armiSpecifiche", s.ArmiSpecifiche)
		}
		if s.AbilitaDaScegliere > 0 {
			extra["skP"] = s.AbilitaDaScegliere
		}
		if len(s.AbilitaFisse) > 0 {
			extra["skFix"] = c.abilita("sottoclassi.abilitaFisse", s.AbilitaFisse)
		}
		if s.MaestrieExtra > 0 {
			extra["exP"] = s.MaestrieExtra
		}
		if len(s.MaestrieFisse) > 0 {
			extra["exFix"] = c.abilita("sottoclassi.maestrieFisse", s.MaestrieFisse)
		}
		if s.AttaccoExtra {
			extra["ea"] = 1
		}
		if s.StiliDiCombattimento > 0 {
			extra["style"] = s.StiliDiCombattimento
		}
		if s.Incantatore != nil {
			for kk, v := range c.incantatore("sottoclassi.incantatore", s.Incantatore, true) {
				extra[kk] = v
			}
		}
		for a, v := range s.Agganci {
			if !contiene(agganciSottoclasse, a) {
				c.err("sottoclasse «%s»: aggancio «%s» sconosciuto (possibili: %s)", nome, a, strings.Join(agganciSottoclasse, ", "))
			}
			extra[a] = v
		}
		voce := []any{nome, c.testo("sottoclassi.fonte", s.Fonte, false), c.privilegi("sottoclassi.privilegi", s.Privilegi)}
		if len(extra) > 0 {
			voce = append(voce, extra)
		}
		subs = append(subs, voce)
		if len(s.Descrizioni) > 0 {
			d.FDSub[nome] = s.Descrizioni
		}
	}
	k["subs"] = subs
	d.Classi[x.ID] = k
	return nil
}

func (c *controllore) campiRazza(x *razzaFile, d *datiScheda, sotto bool) map[string]any {
	r := map[string]any{"n": c.testo("nome", x.Nome, true)}
	if !sotto {
		r["src"] = c.testo("fonte", x.Fonte, false)
		r["speed"] = 9.0
		r["size"] = "Media"
		r["lang"] = []string{}
	}
	if len(x.Caratteristiche) > 0 {
		asi := [][]any{}
		for _, a := range caratteristiche5e {
			if v, ok := x.Caratteristiche[a]; ok {
				asi = append(asi, []any{a, c.intervallo("caratteristiche", v, -10, 10)})
			}
		}
		for a := range x.Caratteristiche {
			if !contiene(caratteristiche5e, a) {
				c.err("«caratteristiche»: «%s» non è una caratteristica (usa FOR, DES, COS, INT, SAG o CAR)", a)
			}
		}
		r["asi"] = asi
	} else if !sotto {
		r["asi"] = [][]any{}
	}
	if x.Velocita != nil {
		if *x.Velocita <= 0 || *x.Velocita > 60 {
			c.err("«velocita» è in metri e deve essere tra 0 e 60")
		}
		r["speed"] = *x.Velocita
	}
	if x.Taglia != "" {
		r["size"] = c.testo("taglia", x.Taglia, false)
	}
	if x.Scurovisione > 0 {
		r["dv"] = x.Scurovisione
	}
	if len(x.Lingue) > 0 {
		r["lang"] = c.elencoTesti("lingue", x.Lingue)
	}
	if x.LingueExtra > 0 {
		r["langX"] = x.LingueExtra
	}
	if len(x.AbilitaFisse) > 0 {
		r["skFix"] = c.abilita("abilitaFisse", x.AbilitaFisse)
	}
	if x.AbilitaDaScegliere > 0 {
		r["skP"] = x.AbilitaDaScegliere
	}
	if x.BonusLiberi != nil {
		r["flex"] = map[string]any{"n": c.intervallo("bonusLiberi.numero", x.BonusLiberi.Numero, 1, 6),
			"v": c.intervallo("bonusLiberi.valore", x.BonusLiberi.Valore, 1, 4), "excl": c.caratteristiche("bonusLiberi.escluse", x.BonusLiberi.Escluse)}
	}
	if len(x.BonusLiberiValori) > 0 {
		r["flexL"] = x.BonusLiberiValori
	}
	if x.BonusFissi {
		r["noMove"] = 1
	}
	if len(x.Tratti) > 0 || !sotto {
		r["t"] = c.elencoTesti("tratti", x.Tratti)
	}
	if m := x.MagiaInnata; m != nil {
		inn := [][]any{}
		for _, i := range m.Incantesimi {
			inn = append(inn, []any{c.intervallo("magiaInnata.livello", i.Livello, 1, 20), c.testo("magiaInnata.testo", i.Testo, true)})
		}
		r["inn"] = inn
		if m.Caratteristica != "" {
			r["innC"] = c.testo("magiaInnata.caratteristica", m.Caratteristica, false)
		}
		if m.Nome != "" {
			r["innN"] = c.testo("magiaInnata.nome", m.Nome, false)
		}
	}
	if len(x.ArmiSpecifiche) > 0 {
		r["weapN"] = c.elencoTesti("armiSpecifiche", x.ArmiSpecifiche)
	}
	if len(x.Armature) > 0 {
		r["armorP"] = c.codici("armature", x.Armature, "l", "m", "p", "sc")
	}
	if x.Strumenti != "" {
		r["tools"] = c.testo("strumenti", x.Strumenti, false)
	}
	if x.Talento {
		r["feat"] = 1
	}
	if x.ArmaturaNaturale > 0 {
		r["natAc"] = c.intervallo("armaturaNaturale", x.ArmaturaNaturale, 1, 30)
	}
	if x.PFPerLivello > 0 {
		r["hpLvl"] = x.PFPerLivello
	}
	for a, v := range x.Agganci {
		if !contiene(agganciRazza, a) {
			c.err("aggancio «%s» sconosciuto (possibili: %s)", a, strings.Join(agganciRazza, ", "))
		}
		r[a] = v
	}
	for n, t := range x.Descrizioni {
		d.FDRace[c.testo("descrizioni", n, true)] = c.testo("descrizioni", t, false)
	}
	return r
}

func convertiRazza(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x razzaFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	if !reIDContenuto.MatchString(x.ID) {
		c.err("«id» deve essere fatto di lettere minuscole senza accenti, cifre, - o _ (es. «elfo»)")
	}
	r := c.campiRazza(&x, d, false)
	if len(x.Sottorazze) > 0 {
		subs := map[string]any{}
		for i, rs := range x.Sottorazze {
			var s razzaFile
			if err := decodifica(rs, &s); err != nil {
				c.err("sottorazza n. %d: %v", i+1, err)
				continue
			}
			if len(s.Sottorazze) > 0 {
				c.err("sottorazza n. %d: una sottorazza non può avere altre sottorazze", i+1)
			}
			if !reIDContenuto.MatchString(s.ID) {
				c.err("sottorazza n. %d: «id» deve essere fatto di lettere minuscole senza accenti, cifre, - o _", i+1)
			}
			subs[s.ID] = c.campiRazza(&s, d, true)
		}
		r["subs"] = subs
	}
	d.Razze[x.ID] = r
	return nil
}

func convertiBackground(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x backgroundFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	if !reIDContenuto.MatchString(x.ID) {
		c.err("«id» deve essere fatto di lettere minuscole senza accenti, cifre, - o _ (es. «studioso»)")
	}
	b := map[string]any{"n": c.testo("nome", x.Nome, true), "sk": c.abilita("abilita", x.Abilita), "tools": c.testo("strumenti", x.Strumenti, false),
		"feat": c.testo("privilegio", x.Privilegio, false), "eq": c.testo("equipaggiamento", x.Equipaggiamento, false), "mo": x.Monete}
	if x.AbilitaDaScegliere > 0 {
		b["skP"] = x.AbilitaDaScegliere
	}
	if x.LingueExtra > 0 {
		b["langX"] = x.LingueExtra
	}
	if x.Monete < 0 {
		c.err("«monete» non può essere negativo")
	}
	if p := strings.TrimSpace(x.Privilegio); p != "" && x.DescrizionePrivilegio != "" {
		d.FDBg[p] = c.testo("descrizionePrivilegio", x.DescrizionePrivilegio, false)
	}
	d.Background[x.ID] = b
	return nil
}

var (
	scuole5e = map[string]string{"abiurazione": "A", "evocazione": "C", "divinazione": "D", "ammaliamento": "E", "invocazione": "V",
		"illusione": "I", "necromanzia": "N", "trasmutazione": "T"}
	tempi5e  = map[string]string{"azione": "a", "azione bonus": "ab", "reazione": "re"}
	reDurata = regexp.MustCompile(`^(I|P|S|C?\d+[rmhg])( o (I|P|S|C?\d+[rmhg]))?$`)
)

func convertiIncantesimo(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x incantesimoFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	nome := c.testo("nome", x.Nome, true)
	en := c.testo("nomeInglese", x.NomeInglese, false)
	if en == "" {
		en = nome
	}
	liv := 0
	if x.Livello == nil {
		c.err("manca «livello» (0 per i trucchetti)")
	} else {
		liv = c.intervallo("livello", *x.Livello, 0, 9)
	}
	sc := strings.TrimSpace(x.Scuola)
	if v, ok := scuole5e[strings.ToLower(sc)]; ok {
		sc = v
	} else if !strings.Contains("ACDEVINT", sc) || len(sc) != 1 {
		c.err("«scuola»: «%s» non riconosciuta (es. «evocazione» oppure la lettera A, C, D, E, V, I, N, T)", sc)
	}
	t := strings.TrimSpace(x.Tempo)
	if v, ok := tempi5e[strings.ToLower(t)]; ok {
		t = v
	}
	if t == "" {
		c.err("manca «tempo» (es. «azione», «azione bonus», «reazione», «1m»)")
	}
	if x.Rituale && !strings.HasPrefix(t, "*") {
		t = "*" + t
	}
	du := strings.TrimSpace(x.Durata)
	switch strings.ToLower(du) {
	case "istantanea":
		du = "I"
	case "permanente":
		du = "P"
	case "speciale":
		du = "S"
	}
	if x.Concentrazione && !strings.HasPrefix(du, "C") {
		du = "C" + du
	}
	if !reDurata.MatchString(du) {
		c.avv("«durata» «%s» non è nella forma prevista (es. I, P, S, 1m, C10m, 8h): la scheda la mostrerà così com'è", du)
	}
	// classi: "bcm" oppure ["b", "c", "m"]
	cl := ""
	var cs string
	var ca []string
	if len(x.Classi) > 0 && json.Unmarshal(x.Classi, &cs) == nil {
		cl = cs
	} else if json.Unmarshal(x.Classi, &ca) == nil {
		cl = strings.Join(ca, "")
	} else if len(x.Classi) > 0 {
		c.err("«classi» deve essere un testo come «bcm» o un elenco come [\"b\", \"c\", \"m\"]")
	}
	for _, r := range cl {
		k := string(r)
		if listeIncantesimiBase[k] == "" && d.NomiListe[k] == "" {
			c.err("«classi»: la lista «%s» non esiste; dichiarala in «listeIncantesimi»", k)
		}
	}
	v := []any{nome, en, liv, sc, t, c.testo("gittata", x.Gittata, false), c.testo("componenti", x.Componenti, false), du, cl,
		c.testo("fonte", x.Fonte, false), c.testo("descrizione", x.Descrizione, false)}
	if hi := c.testo("livelliSuperiori", x.LivelliSuperiori, false); hi != "" {
		v = append(v, hi)
	}
	d.Incantesimi = append(d.Incantesimi, v)
	return nil
}

var reDanno = regexp.MustCompile(`^(\d+(d\d+)?)?$`)

func convertiArma(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x armaFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	cat := map[string]string{"semplice": "s", "guerra": "g"}[strings.ToLower(strings.TrimSpace(x.Categoria))]
	if cat == "" {
		c.err("«categoria» deve essere «semplice» o «guerra»")
	}
	tipo := map[string]string{"mischia": "m", "distanza": "d"}[strings.ToLower(strings.TrimSpace(x.Tipo))]
	if tipo == "" {
		c.err("«tipo» deve essere «mischia» o «distanza»")
	}
	danno := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(x.Danno)), " ", "")
	if !reDanno.MatchString(danno) {
		c.err("«danno» deve essere come «1d8», «2d6» o un numero fisso")
	}
	fl := ""
	for _, f := range []struct {
		on bool
		c  string
	}{{x.Leggera, "l"}, {x.Lancio, "t"}, {x.Versatile, "v"}, {x.Accurata, "f"}, {x.DueMani, "2"}, {x.Pesante, "h"}, {x.Portata, "r"}} {
		if f.on {
			fl += f.c
		}
	}
	d.Armi = append(d.Armi, []any{c.testo("nome", x.Nome, true), cat, tipo, danno, c.testo("tipoDanno", x.TipoDanno, false), c.testo("proprieta", x.Proprieta, false), fl})
	return nil
}

func convertiArmatura(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x armaturaFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	tipo := map[string]string{"leggera": "l", "media": "m", "pesante": "p"}[strings.ToLower(strings.TrimSpace(x.Tipo))]
	if tipo == "" {
		c.err("«tipo» deve essere «leggera», «media» o «pesante»")
	}
	sv := 0
	if x.SvantaggioFurtivita {
		sv = 1
	}
	d.Armature = append(d.Armature, []any{c.testo("nome", x.Nome, true), tipo, c.intervallo("ca", x.CA, 1, 30), c.intervallo("forza", x.Forza, 0, 30), sv})
	return nil
}

func convertiTalento(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x talentoFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	nome := c.testo("nome", x.Nome, true)
	en := c.testo("nomeInglese", x.NomeInglese, false)
	if en == "" {
		en = nome
	}
	var ab any
	var s string
	var l []string
	switch {
	case len(x.Caratteristiche) == 0 || string(x.Caratteristiche) == "null":
	case json.Unmarshal(x.Caratteristiche, &s) == nil:
		if s != "*" {
			c.err("«caratteristiche» deve essere \"*\" (qualsiasi) o un elenco come [\"FOR\", \"DES\"]")
		}
		ab = "*"
	case json.Unmarshal(x.Caratteristiche, &l) == nil:
		ab = c.caratteristiche("caratteristiche", l)
	default:
		c.err("«caratteristiche» deve essere \"*\" (qualsiasi) o un elenco come [\"FOR\", \"DES\"]")
	}
	fx := map[string]any{}
	for k, v := range x.Effetti {
		if !contiene(effettiTalento, k) {
			c.err("«effetti»: «%s» sconosciuto (possibili: %s)", k, strings.Join(effettiTalento, ", "))
		}
		fx[k] = v
	}
	d.Talenti = append(d.Talenti, []any{nome, en, c.testo("fonte", x.Fonte, false), ab, fx, c.testo("descrizione", x.Descrizione, false)})
	return nil
}

func convertiStile(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x stileFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	v := []any{c.testo("nome", x.Nome, true), c.testo("descrizione", x.Descrizione, false)}
	if x.Opzionale {
		d.StiliT = append(d.StiliT, v)
	} else {
		d.Stili = append(d.Stili, v)
	}
	return nil
}

// unisci aggiunge i dati di un pacchetto a quelli dei pacchetti precedenti: a parità di chiave (id o nome) vince il
// pacchetto importato dopo. Restituisce le chiavi sovrascritte, per segnalarle al Master.
func (d *datiScheda) unisci(o *datiScheda) []string {
	sovr := []string{}
	unisciMappa := func(tipo string, a, b map[string]map[string]any) {
		for k, v := range b {
			if _, ok := a[k]; ok {
				sovr = append(sovr, tipo+" «"+k+"»")
			}
			a[k] = v
		}
	}
	unisciMappa("classe", d.Classi, o.Classi)
	unisciMappa("razza", d.Razze, o.Razze)
	unisciMappa("background", d.Background, o.Background)
	unisciElenco := func(tipo string, a *[][]any, b [][]any) {
		pos := map[string]int{}
		for i, v := range *a {
			pos[strings.ToLower(fmt.Sprint(v[0]))] = i
		}
		for _, v := range b {
			k := strings.ToLower(fmt.Sprint(v[0]))
			if i, ok := pos[k]; ok {
				(*a)[i] = v
				sovr = append(sovr, tipo+" «"+fmt.Sprint(v[0])+"»")
				continue
			}
			pos[k] = len(*a)
			*a = append(*a, v)
		}
	}
	unisciElenco("incantesimo", &d.Incantesimi, o.Incantesimi)
	unisciElenco("arma", &d.Armi, o.Armi)
	unisciElenco("armatura", &d.Armature, o.Armature)
	unisciElenco("stile", &d.Stili, o.Stili)
	unisciElenco("stile", &d.StiliT, o.StiliT)
	unisciElenco("talento", &d.Talenti, o.Talenti)
	for k, v := range o.Conosciuti {
		d.Conosciuti[k] = v
	}
	for k, v := range o.Trucchetti {
		d.Trucchetti[k] = v
	}
	for _, p := range []struct{ a, b map[string]string }{{d.Codici, o.Codici}, {d.NomiListe, o.NomiListe}, {d.Fonti, o.Fonti},
		{d.FDGen, o.FDGen}, {d.FDCls, o.FDCls}, {d.FDRace, o.FDRace}, {d.FDBg, o.FDBg}, {d.FDRes, o.FDRes}} {
		for k, v := range p.b {
			p.a[k] = v
		}
	}
	for k, v := range o.FDSub {
		d.FDSub[k] = v
	}
	return sovr
}
