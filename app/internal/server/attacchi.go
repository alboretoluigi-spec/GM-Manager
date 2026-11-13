package server

// Attacchi automatici dal tracker (richiesta dell'utente, sezioni 5–7 della specifica del combattimento;
// docs/PIANO_COMBATTIMENTO.md). Un solo motore, sul server, per giocatori e Master:
//   - «attacco.tira»: attaccante, bersaglio, attacco scelto (profilo), normale/vantaggio/svantaggio. Il server tira il d20,
//     somma il bonus del profilo e confronta con la CA del bersaglio: l'1 naturale manca sempre, il 20 naturale è critico.
//     Se colpisce tira i danni di ogni componente e, per ogni tipo, applica immunità (0), resistenza (metà, per difetto) e
//     vulnerabilità (doppio), tenendo conto di «da attacchi non magici». Poi scala i PF del bersaglio (per i PG prima i PF
//     temporanei), scrive il registro dello scontro e manda l'esito a tutti i presenti (ai giocatori senza CA e PF dei mostri).
//   - Critico: i danni non si tirano subito. Il server dà un gettone monouso; «attacco.critico» con la scelta (tirare due
//     volte i dadi, regola del 2014, oppure raddoppiare il risultato, variante) tira e applica una volta sola.
//   - La stessa «azione» (id scelto dalla pagina) non si applica due volte (doppio clic, rete che ripete il messaggio).
//   - Attacchi per turno: PG dagli «Attacchi per azione» della scheda (Attacco Extra), mostri dal Multiattacco letto dalla
//     loro scheda. Il giocatore attacca solo con il suo PG e nel suo turno; il Master con chiunque, anche fuori turno
//     (attacchi di opportunità), e può superare il numero confermando («forza»).
// Profili: i PG li calcola la scheda (riepilogo «attacks»); i mostri li legge la pagina del Master dalla loro scheda quando li
// aggiunge allo scontro (web/regole/attacchi_mostri.js) e qui si ricontrollano.

import (
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"gasterbless/internal/cripto"
)

type componenteDanno struct {
	Dadi string `json:"dadi,omitempty"` // «2d6»; vuoto = solo il numero fisso
	Mod  int    `json:"mod"`
	Tipo string `json:"tipo,omitempty"`
}

type profiloAttacco struct {
	Nome    string `json:"nome"`
	Bonus   int    `json:"bonus"`
	Portata string `json:"portata,omitempty"`
	Magica  bool   `json:"magica,omitempty"`
	// Argentata: arma d'argento (2026-11-12); supera le difese «da attacchi non magici non argentati» (lupi mannari…)
	Argentata bool              `json:"argentata,omitempty"`
	Danni     []componenteDanno `json:"danni"`
	// incantesimo: attacco di un incantesimo (attacco.tira con «incantesimo»), per la resistenza ai danni degli incantesimi
	incantesimo bool
}

type difesa struct {
	Tipo      string `json:"tipo"`
	NonMagici bool   `json:"nonMagici,omitempty"` // vale solo contro gli attacchi non magici
	// NonArgentate: vale solo contro gli attacchi non magici E non argentati (lupi mannari, diavoli…; 2026-11-12)
	NonArgentate bool `json:"nonArgentate,omitempty"`
}

// colpoArma: com'è l'arma (o l'incantesimo) che infligge i danni, per le difese.
type colpoArma struct {
	magica, argentata, incantesimo bool
}

// sufissiDifesa: «contundente (non magici)», «contundente (non magici e non argentati)».
func leggiVoceDifesa(t string) difesa {
	t = strings.TrimSpace(t)
	if strings.HasSuffix(t, "(non magici e non argentati)") {
		return difesa{Tipo: strings.TrimSpace(strings.TrimSuffix(t, "(non magici e non argentati)")), NonMagici: true, NonArgentate: true}
	}
	nm := strings.HasSuffix(t, "(non magici)")
	return difesa{Tipo: strings.TrimSpace(strings.TrimSuffix(t, "(non magici)")), NonMagici: nm}
}

type difeseCombattente struct {
	Res  []difesa `json:"res,omitempty"`
	Imm  []difesa `json:"imm,omitempty"`
	Vuln []difesa `json:"vuln,omitempty"`
	// Incantesimi: resistenza ai danni degli incantesimi (Aura di interdizione del paladino degli Antichi, Resistenza agli
	// incantesimi del mago abiuratore; difese dei personaggi del 2026-11-12, web/regole/difese.js)
	Incantesimi bool `json:"incantesimi,omitempty"`
}

// economiaTurno: azione, azione bonus e reazione già usate (colonna combattenti.economia, si azzera all'inizio del turno).
type economiaTurno struct {
	A bool `json:"a"` // azione
	B bool `json:"b"` // azione bonus
	R bool `json:"r"` // reazione
	// Multiattacco con attacchi diversi (2026-11-12): quante volte è stato usato ogni attacco (indice) in questo turno
	Seq []int `json:"seq,omitempty"`
}

// passoMulti: un passo della sequenza del Multiattacco («uno con il morso e due con gli artigli»).
type passoMulti struct {
	Attacco int `json:"attacco"` // indice in Attacchi
	Volte   int `json:"volte"`
}

// profiloMostro: colonna combattenti.profilo.
type profiloMostro struct {
	Attacchi []profiloAttacco  `json:"attacchi"`
	Multi    int               `json:"multi,omitempty"` // attacchi per azione con il Multiattacco (0 = uno)
	Sequenza []passoMulti      `json:"sequenza,omitempty"`
	Difese   difeseCombattente `json:"difese"`
	// tiri salvezza contro gli incantesimi (incantesimi_scontro.go): bonus per caratteristica, Resistenza alla magia,
	// Resistenza leggendaria (usi al giorno), immunità alle condizioni
	TS          map[string]int `json:"ts,omitempty"`
	ResMagia    bool           `json:"resMagia,omitempty"`
	Leggendaria int            `json:"leggendaria,omitempty"`
	ImmCond     []string       `json:"immCond,omitempty"`
	// incantesimi della creatura (tratti «Incantesimi»): CD, attacco, gruppi con slot o usi al giorno e quelli già usati
	Incantesimi *incantesimiMostro `json:"incantesimi,omitempty"`
}

type gruppoIncantesimi struct {
	Etichetta string   `json:"etichetta"` // «Trucchetti», «3°», «1/giorno», «A volontà»
	Livello   int      `json:"livello"`   // -1: innati (al livello dell'incantesimo)
	Usi       int      `json:"usi"`       // slot o usi al giorno; 0 = a volontà
	Nomi      []string `json:"nomi"`
	// valori del tratto da cui viene il gruppo (una creatura può avere incantesimi innati e da classe con CD diverse);
	// 0 = quelli della creatura
	CD  int `json:"cd,omitempty"`
	Att int `json:"att,omitempty"`
	Mod int `json:"mod,omitempty"`
}

type incantesimiMostro struct {
	CD     int                 `json:"cd"`
	Att    int                 `json:"att"`
	Mod    int                 `json:"mod"`
	Liv    int                 `json:"liv"` // livello da incantatore (trucchetti)
	Gruppi []gruppoIncantesimi `json:"gruppi"`
	Usati  map[string]int      `json:"usati,omitempty"` // indice del gruppo → slot o usi già spesi in questo scontro
}

var (
	reDadiAttacco = regexp.MustCompile(`^([1-9]\d?)d(4|6|8|10|12|20|100)$`)
	reNumero      = regexp.MustCompile(`^\d{1,2}$`)
	reAzione      = regexp.MustCompile(`^[A-Za-z0-9_-]{6,40}$`)
	tipiDanno     = map[string]bool{"acido": true, "contundente": true, "freddo": true, "fuoco": true, "forza": true, "fulmine": true,
		"necrotico": true, "perforante": true, "veleno": true, "psichico": true, "radioso": true, "tagliente": true, "tuono": true}
	pluraliDanno = map[string]string{"contundenti": "contundente", "taglienti": "tagliente", "perforanti": "perforante",
		"necrotici": "necrotico", "psichici": "psichico", "radiosi": "radioso", "fulmini": "fulmine", "elettrico": "fulmine",
		"elettricità": "fulmine", "freddi": "freddo", "acidi": "acido", "veleni": "veleno"}
)

// tipoDanno porta il tipo a una forma sola: «danni da fuoco», «Taglienti» → fuoco, tagliente. Sconosciuto: «».
func tipoDanno(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	t = strings.TrimPrefix(t, "danni ")
	t = strings.TrimPrefix(t, "da ")
	if p, ok := pluraliDanno[t]; ok {
		t = p
	}
	if tipiDanno[t] {
		return t
	}
	return ""
}

// Limiti dei valori che arrivano dalle pagine (rapporto di diagnosi del 2026-11-12, punto 3.4): il caso più grande delle
// regole è Sciame di meteore (20d6 + 20d6) e Sfera vetriolica al 9° (20d4); bonus e modificatori ben oltre i massimi dei
// manuali. Oltre, il componente si scarta.
const (
	maxDadiComponente = 20
	maxDadiAttacco    = 40
)

// valoriInsoliti: nota per il Master quando un PG manda valori oltre quelli plausibili con le regole del 2014 (competenza +6,
// caratteristica fino a +9 con una Cintura della forza dei giganti, oggetto +3, stile Tiro +2: attacco +17 al massimo nei casi
// estremi; CD 23; danni di un'arma oltre 12 dadi). Non blocca: il Master controlla la scheda.
func valoriInsoliti(bonus, cd int, danni []componenteDanno) string {
	n := 0
	for _, c := range danni {
		n += dadiDi(c.Dadi)
	}
	switch {
	case bonus > 17:
		return " ⚠ bonus " + conSegno(bonus) + " insolito per un personaggio: controllare la scheda"
	case cd > 23:
		return " ⚠ CD " + itoa(cd) + " insolita per un personaggio: controllare la scheda"
	case n > 12 && cd == 0 && bonus != 0:
		return " ⚠ " + itoa(n) + " dadi di danno insoliti per un'arma: controllare la scheda"
	}
	return ""
}

// dadiDi: quanti dadi ha un'espressione «NdM» (0 per un numero fisso).
func dadiDi(espr string) int {
	if m := reDadiAttacco.FindStringSubmatch(espr); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func validaProfili(l []profiloAttacco) []profiloAttacco {
	out := []profiloAttacco{}
	for _, p := range l {
		if len(out) >= 30 {
			break
		}
		p.Nome = limita(p.Nome, 60)
		if p.Nome == "" || p.Bonus < -10 || p.Bonus > 25 {
			continue
		}
		p.Portata = limita(p.Portata, 40)
		danni := []componenteDanno{}
		totale := 0
		for _, c := range p.Danni {
			if len(danni) >= 6 {
				break
			}
			c.Dadi = strings.ToLower(strings.ReplaceAll(c.Dadi, " ", ""))
			if reNumero.MatchString(c.Dadi) { // «1» (colpo senz'armi): numero fisso
				n, _ := strconv.Atoi(c.Dadi)
				c.Dadi, c.Mod = "", c.Mod+n
			}
			if c.Dadi != "" && !reDadiAttacco.MatchString(c.Dadi) || c.Mod < -10 || c.Mod > 30 || c.Dadi == "" && c.Mod <= 0 {
				continue
			}
			if n := dadiDi(c.Dadi); n > maxDadiComponente || totale+n > maxDadiAttacco {
				continue
			} else {
				totale += n
			}
			c.Tipo = tipoDanno(c.Tipo)
			danni = append(danni, c)
		}
		p.Danni = danni
		out = append(out, p)
	}
	return out
}

func validaDifese(d difeseCombattente) difeseCombattente {
	pulisci := func(l []difesa) []difesa {
		var out []difesa
		visti := map[string]bool{}
		for _, x := range l {
			if t := tipoDanno(x.Tipo); t != "" && !visti[t] {
				visti[t] = true
				out = append(out, difesa{Tipo: t, NonMagici: x.NonMagici || x.NonArgentate, NonArgentate: x.NonArgentate})
			}
		}
		return out
	}
	return difeseCombattente{Res: pulisci(d.Res), Imm: pulisci(d.Imm), Vuln: pulisci(d.Vuln), Incantesimi: d.Incantesimi}
}

// tiraDadi tira «NdM» (volte = 2 per il critico con i dadi raddoppiati): somma e singoli risultati.
func tiraDadi(espr string, volte int) (int, []int) {
	m := reDadiAttacco.FindStringSubmatch(espr)
	if m == nil {
		return 0, nil
	}
	n, _ := strconv.Atoi(m[1])
	f, _ := strconv.Atoi(m[2])
	tot, l := 0, []int{}
	for i := 0; i < n*volte; i++ {
		x := dado(f)
		tot += x
		l = append(l, x)
	}
	return tot, l
}

// applicaDifese: immunità, poi resistenza (metà per difetto), poi vulnerabilità (doppio).
func applicaDifese(tipo string, v int, magica bool, d difeseCombattente) (int, string) {
	return applicaDifeseDa(tipo, v, colpoArma{magica: magica}, d)
}

// applicaDifeseDa: come applicaDifese; incantesimo = il danno viene da un incantesimo (resistenza ai danni degli incantesimi,
// che non si somma a quella per tipo: la metà si applica una volta sola).
func applicaDifeseDa(tipo string, v int, k colpoArma, d difeseCombattente) (int, string) {
	magica, incantesimo := k.magica, k.incantesimo
	if tipo == "" {
		if incantesimo && d.Incantesimi {
			return v / 2, "resistenza ai danni degli incantesimi, metà"
		}
		return v, ""
	}
	// una difesa «non magici» non vale contro le armi magiche; «non magici non argentati» neanche contro quelle d'argento
	vale := func(l []difesa) bool {
		for _, x := range l {
			if x.Tipo == tipo && !(x.NonMagici && magica) && !(x.NonArgentate && k.argentata) {
				return true
			}
		}
		return false
	}
	if vale(d.Imm) {
		return 0, "immunità"
	}
	nota := ""
	if vale(d.Res) {
		v, nota = v/2, "resistenza, metà"
	} else if incantesimo && d.Incantesimi {
		v, nota = v/2, "resistenza ai danni degli incantesimi, metà"
	}
	if vale(d.Vuln) {
		v *= 2
		if nota != "" {
			nota += "; "
		}
		nota += "vulnerabilità, doppio"
	}
	return v, nota
}

// ===== esito =====

type parteDanno struct {
	Tipo   string `json:"tipo,omitempty"`
	Espr   string `json:"espr"`
	Dadi   []int  `json:"dadi,omitempty"`
	Tirato int    `json:"tirato"`
	Finale int    `json:"finale"`
	Nota   string `json:"nota,omitempty"`
}

type esitoAttacco struct {
	Sessione string `json:"sessione"`
	Azione   string `json:"azione,omitempty"`
	// dadi dei danni cambiati nella conferma («1d8 → 2d8»): dadi_conferma.go
	DadiCambiati string       `json:"dadiCambiati,omitempty"`
	Attaccante   string       `json:"attaccante"`
	Bersaglio    string       `json:"bersaglio"`
	NomeA        string       `json:"nomeAttaccante"`
	NomeB        string       `json:"nomeBersaglio"`
	Attacco      string       `json:"attacco"`
	Modo         string       `json:"modo"`
	D20          []int        `json:"d20"`
	Naturale     int          `json:"naturale"`
	Bonus        int          `json:"bonus"`
	Totale       int          `json:"totale"`
	CA           *int         `json:"ca,omitempty"`
	Risultato    string       `json:"risultato"`         // mancato, colpito, critico
	Gettone      string       `json:"gettone,omitempty"` // critico in attesa della scelta (solo a chi ha attaccato e al Master)
	Danni        []parteDanno `json:"danni,omitempty"`
	TotDanni     int          `json:"totaleDanni"`
	PFPrima      *int64       `json:"pfPrima,omitempty"`
	PFDopo       *int64       `json:"pfDopo,omitempty"`
	Sconfitto    bool         `json:"sconfitto,omitempty"`
	ModoCrit     string       `json:"modoCritico,omitempty"`
	Rimasti      int          `json:"attacchiRimasti"`
	PerAzione    int          `json:"attacchiPerAzione"`
	Reazione     bool         `json:"reazione,omitempty"` // attacco fuori dal proprio turno (usa la reazione)
	// il bersaglio era concentrato: TS di Costituzione con questa CD per mantenere l'incantesimo
	Concentrazione int `json:"concentrazione,omitempty"`
	// perché vantaggio, svantaggio o critico automatico (condizioni di attaccante e bersaglio)
	Fonti []string `json:"fonti,omitempty"`
	// danni subiti da chi attacca (Scudo di fuoco del bersaglio)
	Ritorsione []string `json:"ritorsione,omitempty"`
	// attacco di un mostro nascosto, come lo riceve il giocatore del PG colpito: senza chi, come e con cosa
	Anonimo bool `json:"anonimo,omitempty"`
	// dadi 3D visti da tutti (dadi_aspetto.go): set di chi tira (il giocatore del PG, il Master per i mostri) e classe del PG
	Colore    string `json:"colore,omitempty"`
	Classe    string `json:"classe,omitempty"`
	nascosto  bool
	nascostoA bool   // chi attacca è nascosto
	propB     string // proprietario del PG colpito
	autore    string
	mostroB   bool
}

// propPG: il proprietario del combattente se è un PG.
func propPG(b *vistaCombattente) string {
	if b.Tipo != "pg" {
		return ""
	}
	return b.Proprietario
}

// anonimo (decisione dell'utente del 2026-11-12): un mostro nascosto attacca un PG; il giocatore di quel PG riceve
// l'esito senza nome, attacco, tiro e bonus di chi attacca (danni e tipo sì: li sente sulla pelle).
func (e esitoAttacco) anonimo() esitoAttacco {
	e.Anonimo = true
	e.Attaccante, e.NomeA, e.Attacco, e.Modo = "", "Qualcosa", "", ""
	e.D20, e.Naturale, e.Bonus, e.Totale = nil, 0, 0, 0
	e.Gettone, e.Fonti, e.Ritorsione = "", nil, nil
	e.Rimasti, e.PerAzione, e.Reazione = 0, 0, false
	return e
}

// perRuolo toglie a chi non deve vederli CA e PF dei mostri e il gettone del critico.
func (e esitoAttacco) perRuolo(ruolo, uid string) esitoAttacco {
	if ruolo == "giocatore" && e.mostroB {
		e.CA, e.PFPrima, e.PFDopo = nil, nil, nil
	}
	if ruolo != "master" && uid != e.autore {
		e.Gettone = ""
	}
	return e
}

type critico struct {
	comb, attaccante, bersaglio, autore string
	profilo                             profiloAttacco
	esito                               esitoAttacco
	scade                               time.Time
}

// memoriaAttacchi: critici in attesa e azioni già eseguite. serie fa passare un attacco alla volta (i PF letti e scritti
// da due attacchi contemporanei non si pestano i piedi).
type memoriaAttacchi struct {
	serie   sync.Mutex
	critici map[string]*critico
	fatte   map[string]esitoAttacco
	quando  map[string]time.Time
	lanci   map[string]*lancioInCorso // incantesimi con più raggi (incantesimi_scontro.go)
	ts      map[string]*tsInAttesa    // tiri salvezza in attesa della conferma del Master (protetti da tsMu: li legge anche la vista)
	tsMu    sync.Mutex
	risolti map[string]any // risposte già date agli incantesimi (stessa azione ripetuta)
}

var attacchiInCorso = &memoriaAttacchi{critici: map[string]*critico{}, fatte: map[string]esitoAttacco{}, quando: map[string]time.Time{},
	lanci: map[string]*lancioInCorso{}, ts: map[string]*tsInAttesa{}, risolti: map[string]any{}}

func (m *memoriaAttacchi) pulisci() {
	ora := time.Now()
	for k, c := range m.critici {
		if ora.After(c.scade) {
			delete(m.critici, k)
		}
	}
	for k, t := range m.quando {
		if ora.Sub(t) > 30*time.Minute {
			delete(m.quando, k)
			delete(m.fatte, k)
			delete(m.risolti, k)
		}
	}
	for k, l := range m.lanci {
		if ora.Sub(l.quando) > 30*time.Minute {
			delete(m.lanci, k)
		}
	}
	m.tsMu.Lock()
	for k, p := range m.ts {
		if ora.After(p.scade) {
			delete(m.ts, k)
		}
	}
	m.tsMu.Unlock()
}

// attacca esegue «attacco.tira» e «attacco.critico» (da azioneIniziativa, dopo i controlli su sessione e combattimento).
func (s *Server) attacca(u *Utente, tipo, sessione, comb, ruolo string, d datiAzione, registro func(string)) (any, error) {
	master := ruolo == "master"
	if d.Azione != "" && !reAzione.MatchString(d.Azione) || d.Extra < -10 || d.Extra > 10 {
		return nil, errDatiNonValidi
	}
	m := attacchiInCorso
	m.serie.Lock()
	defer m.serie.Unlock()
	m.pulisci()
	if e, ok := m.fatte[d.Azione]; ok && d.Azione != "" { // la stessa azione ripetuta: stesso esito, niente danni in più
		return e.perRuolo(ruolo, u.ID), nil
	}
	// stato aggiornato (un attacco appena finito può aver cambiato PF e turni)
	_, _, turni, turnoDi, err := s.combattimentoAttivo(s.DB, sessione)
	if err != nil {
		return nil, errNessunCombattimento
	}
	tutti, err := s.caricaCombattenti(comb)
	if err != nil {
		return nil, err
	}
	trova := func(id string) *vistaCombattente {
		for _, c := range tutti {
			if c.ID == id {
				return c
			}
		}
		return nil
	}

	var a, b *vistaCombattente
	var prof profiloAttacco
	var esito esitoAttacco
	volte, raddoppia := 1, false

	if tipo == "attacco.critico" {
		if m.critici[d.Gettone] == nil { // dopo un riavvio di Gaster Bless: dal database (attese_scontro.go)
			s.ripristinaAttese(comb, true)
		}
		c := m.critici[d.Gettone]
		if c == nil || c.comb != comb {
			return nil, errore(http.StatusConflict, "Questo colpo critico è già stato risolto o è scaduto.")
		}
		if c.autore != u.ID && !master {
			return nil, errVietato
		}
		a, b = trova(c.attaccante), trova(c.bersaglio)
		if a == nil || b == nil {
			delete(m.critici, d.Gettone)
			s.togliAttesa(d.Gettone)
			return nil, errore(http.StatusConflict, "L'attaccante o il bersaglio non è più nello scontro.")
		}
		prof, esito = c.profilo, c.esito
		switch d.Modo {
		case "dadi":
			volte, esito.ModoCrit = 2, "dadi tirati due volte"
		case "raddoppia":
			raddoppia, esito.ModoCrit = true, "risultato dei dadi raddoppiato"
		default:
			return nil, errore(http.StatusBadRequest, "Scegli come calcolare il critico.")
		}
		delete(m.critici, d.Gettone) // monouso
		s.togliAttesa(d.Gettone)
		esito.Gettone, esito.Azione = "", d.Azione
	} else {
		a, b = trova(d.Combattente), trova(d.Bersaglio)
		if a == nil || b == nil || b.Nascosto && !master {
			return nil, errNonTrovato
		}
		if a.ID == b.ID {
			return nil, errore(http.StatusBadRequest, "Scegli un bersaglio diverso dall'attaccante.")
		}
		if a.Tipo == "pg" && a.Proprietario != u.ID && !master || a.Tipo == "mostro" && !master {
			return nil, errVietato
		}
		if turni == 0 || turnoDi == "" {
			return nil, errore(http.StatusConflict, "Il combattimento non è ancora iniziato.")
		}
		// Incantesimo con tiro per colpire (incantesimi_scontro.go): stesso tiro e stessi danni, ma usa l'azione, l'azione
		// bonus o la reazione del suo tempo di lancio; i raggi dello stesso lancio non la usano di nuovo.
		var inc *incantesimoScontro
		if d.Incantesimo != nil {
			x, err := validaIncantesimoScontro(d.Incantesimo)
			if err != nil || x.Tipo != "attacco" {
				return nil, errDatiNonValidi
			}
			inc = &x
		}
		if !master && turnoDi != a.ID && !(inc != nil && inc.Tempo == "reazione") {
			return nil, errore(http.StatusConflict, "Non è il turno di "+a.Nome+".")
		}
		// condizioni che impediscono di agire (incapacitato, paralizzato, stordito…): il Master può forzare
		if c := nonPuoAgire(a); c != "" && !(master && d.Forza) {
			if master {
				return map[string]any{"serveConferma": a.Nome + " è " + c + ": non può attaccare. Attaccare lo stesso?"}, nil
			}
			return nil, errore(http.StatusConflict, a.Nome+" è "+c+": non può attaccare.")
		}
		if inc != nil {
			portata := "distanza"
			if inc.Mischia {
				portata = "1,5 m"
			}
			prof = profiloAttacco{Nome: "✨ " + inc.Nome, Bonus: inc.Bonus, Portata: portata, Magica: true, Danni: inc.Danni, incantesimo: true}
			rimasti, risposta, err := s.lancioConRaggi(u, comb, a, *inc, turnoDi, master, d.Forza, d.UsaSlot)
			if err != nil || risposta != nil {
				return risposta, err
			}
			esito = esitoAttacco{Sessione: sessione, Azione: d.Azione, Attaccante: a.ID, Bersaglio: b.ID, NomeA: a.Nome, NomeB: b.Nome,
				Attacco: prof.Nome, Modo: "normale", Bonus: prof.Bonus + d.Extra, Rimasti: rimasti, PerAzione: inc.Raggi,
				autore: u.ID, mostroB: b.Tipo == "mostro", nascosto: a.Nascosto || b.Nascosto, nascostoA: a.Nascosto, propB: propPG(b), Reazione: inc.Tempo == "reazione"}
		} else if len(a.Attacchi) == 0 {
			if a.Tipo == "pg" {
				return nil, errore(http.StatusConflict, "La scheda di "+a.Nome+" non ha ancora mandato i suoi attacchi: aprila una volta, si aggiornano da soli.")
			}
			return nil, errore(http.StatusConflict, a.Nome+" non ha attacchi letti dalla scheda: aggiungilo dal bestiario oppure tira a mano.")
		}
		if inc == nil && (d.Attacco < 0 || d.Attacco >= len(a.Attacchi)) {
			return nil, errore(http.StatusBadRequest, "Attacco non valido.")
		}
		if inc == nil {
			if r, err := s.attaccoConArma(a, &prof, &esito, d, sessione, turni, turnoDi, master, u, b); err != nil || r != nil {
				return r, err
			}
			// numero dei dadi dei danni scelto nella conferma (dadi D2: dadi_conferma.go)
			if x, nota := conNumeroDadi(prof.Danni, d.DadiDanni); nota != "" {
				prof.Danni, esito.DadiCambiati = x, nota
			}
		}
		esito.Colore, esito.Classe = s.aspettoTiratore(a, u)
		// tiro per colpire: la scelta della pagina e le condizioni di attaccante e bersaglio (si annullano a vicenda)
		modo, fonti, critAuto := modoConCondizioni(d.Modo, a, b, aDistanza(prof))
		esito.Fonti = fonti
		t1 := dado(20)
		esito.D20, esito.Naturale = []int{t1}, t1
		if modo == "vantaggio" || modo == "svantaggio" {
			t2 := dado(20)
			esito.D20, esito.Modo = append(esito.D20, t2), modo
			if modo == "vantaggio" && t2 > t1 || modo == "svantaggio" && t2 < t1 {
				esito.Naturale = t2
			}
		}
		if a.mod != nil && len(a.mod.dadiAtt) > 0 { // dadi in più degli effetti in corso (Benedizione): effetti_valori.go
			x, testi := tiraDadiEffetto(a.mod.dadiAtt)
			esito.Bonus += x
			esito.Fonti = append(esito.Fonti, testi...)
		}
		esito.Totale = esito.Naturale + esito.Bonus
		ca := 10
		if b.CA != nil {
			ca = *b.CA
		}
		esito.CA = &ca
		switch {
		case esito.Naturale == 1:
			esito.Risultato = "mancato"
		case esito.Naturale == 20:
			esito.Risultato = "critico"
		case esito.Totale >= ca:
			esito.Risultato = "colpito"
			if critAuto { // da vicino contro un bersaglio paralizzato o privo di sensi: ogni colpo è critico
				esito.Risultato = "critico"
				esito.Fonti = append(esito.Fonti, "colpo critico automatico: "+b.Nome+" è "+critAutoMotivo(b)+" e l'attacco è da vicino")
			}
		default:
			esito.Risultato = "mancato"
		}
		t := a.Nome + " attacca " + b.Nome + " (" + prof.Nome + "): d20 " + itoa(esito.Naturale)
		if esito.Reazione {
			t = a.Nome + " attacca " + b.Nome + " con la reazione (" + prof.Nome + "): d20 " + itoa(esito.Naturale)
		}
		if len(esito.D20) > 1 {
			t += " (" + esito.Modo + ": " + itoa(esito.D20[0]) + " e " + itoa(esito.D20[1]) + ")"
		}
		t += " " + conSegno(esito.Bonus) + " = " + itoa(esito.Totale) + " contro CA " + itoa(ca) + ": " +
			map[string]string{"mancato": "mancato", "colpito": "colpito", "critico": "COLPO CRITICO"}[esito.Risultato]
		if esito.Naturale == 1 {
			t += " (1 naturale)"
		}
		if len(esito.Fonti) > 0 {
			t += " [" + strings.Join(esito.Fonti, "; ") + "]"
		}
		if esito.DadiCambiati != "" {
			t += " (dadi dei danni scelti: " + esito.DadiCambiati + ")"
		}
		if a.Tipo == "pg" {
			t += valoriInsoliti(prof.Bonus, 0, prof.Danni)
		}
		if esito.Risultato != "mancato" && len(prof.Danni) == 0 {
			t += " (nessun danno da tirare: l'effetto lo applica il Master)"
		}
		registro(t)
		if esito.Risultato == "critico" && len(prof.Danni) > 0 { // i danni dopo la scelta del modo
			esito.Gettone = cripto.ID()
			m.critici[esito.Gettone] = &critico{comb: comb, attaccante: a.ID, bersaglio: b.ID, autore: u.ID, profilo: prof, esito: esito,
				scade: time.Now().Add(20 * time.Minute)}
			s.salvaCritico(esito.Gettone, m.critici[esito.Gettone])
		}
	}

	if (esito.Risultato == "colpito" || tipo == "attacco.critico") && len(prof.Danni) > 0 {
		if err := s.applicaDanni(u, comb, a, b, prof, volte, raddoppia, &esito, registro); err != nil {
			return nil, err
		}
	}
	if d.Azione != "" {
		m.fatte[d.Azione], m.quando[d.Azione] = esito, time.Now()
	}
	s.pubblicaEsito(sessione, esito)
	s.pubblicaCombattimento(sessione)
	return esito.perRuolo(ruolo, u.ID), nil
}

// applicaDanni tira i danni del profilo, applica le difese del bersaglio e scala i suoi PF.
func (s *Server) applicaDanni(u *Utente, comb string, a, b *vistaCombattente, prof profiloAttacco, volte int, raddoppia bool, esito *esitoAttacco, registro func(string)) error {
	for _, c := range prof.Danni {
		x, dadi := tiraDadi(c.Dadi, volte)
		if raddoppia {
			x *= 2
		}
		espr := c.Dadi
		if mm := reDadiAttacco.FindStringSubmatch(c.Dadi); mm != nil && volte == 2 {
			n, _ := strconv.Atoi(mm[1])
			espr = itoa(n*2) + "d" + mm[2]
		}
		if raddoppia && espr != "" {
			espr = "2×" + espr
		}
		if c.Mod != 0 || espr == "" {
			if espr == "" {
				espr = itoa(c.Mod)
			} else {
				espr += conSegno(c.Mod)
			}
		}
		esito.Danni = append(esito.Danni, parteDanno{Tipo: c.Tipo, Espr: espr, Dadi: dadi, Tirato: max(0, x+c.Mod)})
	}
	// danni in più in mischia degli effetti in corso (ira): effetti_valori.go
	if a.mod != nil && a.mod.mischia > 0 && !prof.incantesimo && !aDistanza(prof) && len(prof.Danni) > 0 {
		esito.Danni = append(esito.Danni, parteDanno{Tipo: prof.Danni[0].Tipo, Espr: "+" + itoa(a.mod.mischia) + " " + strings.ToLower(a.mod.nomeMischia),
			Tirato: a.mod.mischia})
	}
	r, err := s.danneggia(u, comb, b, esito.Danni, colpoArma{magica: prof.Magica, argentata: prof.Argentata, incantesimo: prof.incantesimo})
	if err != nil {
		return err
	}
	esito.TotDanni, esito.PFPrima, esito.PFDopo, esito.Sconfitto, esito.Concentrazione = r.tot, r.prima, &r.dopo, r.sconfitto, r.cdConc
	t := b.Nome + " subisce " + r.testo()
	if esito.ModoCrit != "" {
		t += " (critico: " + esito.ModoCrit + ")"
	}
	registro(t + r.coda(b))
	if b.Tipo == "mostro" && esito.Sconfitto {
		if x := s.ricompensaMostro(comb, b.ID, u); x != "" {
			registro(x)
		}
	}
	// Scudo di fuoco (effetti_valori.go): chi colpisce in mischia il bersaglio subisce i danni dell'effetto
	if b.mod != nil && len(b.mod.ritorsioni) > 0 && !aDistanza(prof) {
		for _, rt := range b.mod.ritorsioni {
			x, dadi := tiraDadi(rt.Dadi, 1)
			parti := []parteDanno{{Tipo: rt.Tipo, Espr: rt.Dadi, Dadi: dadi, Tirato: x}}
			ra, err := s.danneggia(u, comb, a, parti, colpoArma{magica: true, incantesimo: true})
			if err != nil {
				return err
			}
			t := a.Nome + " subisce " + ra.testo() + " da " + rt.nome + " di " + b.Nome + ra.coda(a)
			esito.Ritorsione = append(esito.Ritorsione, t)
			registro(t)
		}
	}
	return nil
}

// danniInflitti: risultato di danneggia.
type danniInflitti struct {
	parti     []parteDanno
	tot       int
	prima     *int64
	dopo      int64
	sconfitto bool
	cdConc    int // > 0: il bersaglio era concentrato, TS di Costituzione con questa CD
}

func (r danniInflitti) testo() string {
	l := []string{}
	for _, p := range r.parti {
		t := itoa(p.Finale)
		if p.Tipo != "" {
			t += " " + p.Tipo
		}
		if p.Nota != "" {
			t += " (" + p.Nota + ")"
		}
		l = append(l, t)
	}
	t := itoa(r.tot) + " danni"
	if len(l) > 1 || len(l) == 1 && strings.Contains(l[0], " ") {
		t += ": " + strings.Join(l, ", ")
	}
	return t
}

func (r danniInflitti) coda(b *vistaCombattente) string {
	t := ""
	if r.sconfitto {
		t += ". " + b.Nome + " è a 0 PF"
	} else if r.cdConc > 0 {
		t += ". Concentrazione: TS di Costituzione CD " + itoa(r.cdConc)
	}
	return t
}

// danneggia applica a un bersaglio i danni già tirati (parti[i].Tirato): difese per tipo, PF (per i PG prima i temporanei,
// la scheda aperta si aggiorna), concentrazione. Scrive in parti il valore finale e la nota delle difese.
func (s *Server) danneggia(u *Utente, comb string, b *vistaCombattente, parti []parteDanno, k colpoArma) (danniInflitti, error) {
	r := danniInflitti{parti: parti}
	for i := range parti {
		parti[i].Finale, parti[i].Nota = applicaDifeseDa(parti[i].Tipo, parti[i].Tirato, k, b.difese)
		r.tot += parti[i].Finale
	}
	if b.PF != nil {
		p := *b.PF
		r.prima = &p
	}
	if b.Tipo == "mostro" {
		s.DB.Exec(`UPDATE combattenti SET pf = MAX(0, pf - ?) WHERE id = ?`, r.tot, b.ID)
		s.DB.QueryRow(`SELECT pf FROM combattenti WHERE id = ?`, b.ID).Scan(&r.dopo)
	} else {
		if r.tot > 0 {
			v := r.tot
			if err := s.modificaPG(u, b, "pg.danno", datiAzione{Valore: &v}); err != nil {
				return r, err
			}
		}
		r.dopo = *b.PF
		s.DB.QueryRow(`SELECT pf FROM stato_in_gioco WHERE personaggio_id = ? AND pf IS NOT NULL`, b.Personaggio).Scan(&r.dopo)
	}
	r.sconfitto = r.dopo == 0 && b.PF != nil && *b.PF > 0
	if r.tot > 0 && !r.sconfitto && slices.Contains(b.Condizioni, "Concentrato") {
		r.cdConc = max(10, r.tot/2)
	}
	if r.sconfitto && slices.Contains(b.Condizioni, "Concentrato") { // a 0 PF la concentrazione finisce (effetti_scontro.go)
		s.rimuoviCondizioni(u, b, []string{"Concentrato"})
		s.finisceConcentrazione(u, comb, b.ID, b.Nome+" è a 0 PF")
	}
	if b.Tipo == "pg" && r.tot > 0 {
		s.avvisaStatoInGioco(b.Personaggio, b.Proprietario) // scheda aperta del giocatore
	}
	return r, nil
}

// attaccoConArma: attacco con uno dei profili del combattente (arma o attacco del mostro). Nel suo turno usa l'azione Attacco,
// con gli attacchi per azione (Attacco Extra, Multiattacco); fuori turno (solo il Master) usa la reazione. Restituisce una
// richiesta di conferma per il Master quando si va oltre i limiti.
func (s *Server) attaccoConArma(a *vistaCombattente, prof *profiloAttacco, esito *esitoAttacco, d datiAzione, sessione string, turni int, turnoDi string,
	master bool, u *Utente, b *vistaCombattente) (any, error) {
	*prof = a.Attacchi[d.Attacco]
	usati := a.usati
	if a.turnoUsati != turni {
		usati = 0
	}
	eco := economiaTurno{}
	if a.Economia != nil {
		eco = *a.Economia
	}
	// Il Master può andare oltre i limiti (azioni bonus, effetti speciali): la pagina chiede conferma e riprova con «forza».
	chiedi := func(t string) (any, error) {
		return map[string]any{"serveConferma": a.Nome + " " + t + " Attaccare lo stesso?"}, nil
	}
	rimasti := 0
	if turnoDi == a.ID { // nel suo turno: l'azione Attacco, con gli attacchi per azione (Attacco Extra, Multiattacco)
		if usati >= a.PerAzione && !(master && d.Forza) {
			if master {
				return chiedi("ha già fatto i suoi " + itoa(a.PerAzione) + " attacchi in questo turno.")
			}
			return nil, errore(http.StatusConflict, "Hai già fatto i tuoi "+itoa(a.PerAzione)+" attacchi in questo turno.")
		}
		if usati == 0 && eco.A && !(master && d.Forza) {
			if master {
				return chiedi("ha già usato l'azione in questo turno.")
			}
			return nil, errore(http.StatusConflict, "Hai già usato l'azione in questo turno (è segnata nel tracker: se è un errore toglila).")
		}
		// Multiattacco con attacchi diversi: ogni attacco quante volte lo prevede la sequenza
		if len(a.Multiattacco) > 0 {
			if usati == 0 || len(eco.Seq) != len(a.Attacchi) {
				eco.Seq = make([]int, len(a.Attacchi))
			}
			prev := 0
			for _, x := range a.Multiattacco {
				if x.Attacco == d.Attacco {
					prev = x.Volte
				}
			}
			if eco.Seq[d.Attacco] >= prev && !(master && d.Forza) {
				t := "con il Multiattacco fa " + testoSequenza(a) + ": "
				if prev == 0 {
					t += "«" + prof.Nome + "» non ne fa parte."
				} else {
					t += "«" + prof.Nome + "» l'ha già usato " + itoa(eco.Seq[d.Attacco]) + (map[bool]string{true: " volta.", false: " volte."}[eco.Seq[d.Attacco] == 1])
				}
				return chiedi(t)
			}
			eco.Seq[d.Attacco]++
		}
		eco.A = true
		rimasti = max(0, a.PerAzione-usati-1)
		s.DB.Exec(`UPDATE combattenti SET attacchi_usati = ?, attacchi_turno = ? WHERE id = ?`, usati+1, turni, a.ID)
	} else { // fuori dal suo turno (solo il Master): è una reazione, per esempio un attacco di opportunità
		if eco.R && !d.Forza {
			return chiedi("ha già usato la reazione in questo round.")
		}
		eco.R = true
	}
	be, _ := json.Marshal(eco)
	s.DB.Exec(`UPDATE combattenti SET economia = ? WHERE id = ?`, string(be), a.ID)
	*esito = esitoAttacco{Sessione: sessione, Azione: d.Azione, Attaccante: a.ID, Bersaglio: b.ID, NomeA: a.Nome, NomeB: b.Nome,
		Attacco: prof.Nome, Modo: "normale", Bonus: prof.Bonus + d.Extra, Rimasti: rimasti, PerAzione: a.PerAzione,
		autore: u.ID, mostroB: b.Tipo == "mostro", nascosto: a.Nascosto || b.Nascosto, nascostoA: a.Nascosto, propB: propPG(b), Reazione: turnoDi != a.ID}
	return nil, nil
}

// testoSequenza: «1 Morso e 2 Artiglio».
func testoSequenza(a *vistaCombattente) string {
	p := []string{}
	for _, x := range a.Multiattacco {
		if x.Attacco < len(a.Attacchi) {
			p = append(p, itoa(x.Volte)+" "+a.Attacchi[x.Attacco].Nome)
		}
	}
	return strings.Join(p, " e ")
}

// usaIncantesimoMostro («mostro.incantesimo», solo il Master): spende uno slot o un uso al giorno del gruppo scelto.
func (s *Server) usaIncantesimoMostro(u *Utente, c *vistaCombattente, gruppo int, forza bool, registro func(string)) (any, error) {
	var p profiloMostro
	var raw string
	s.DB.QueryRow(`SELECT profilo FROM combattenti WHERE id = ?`, c.ID).Scan(&raw)
	if json.Unmarshal([]byte(raw), &p) != nil || p.Incantesimi == nil || gruppo < 0 || gruppo >= len(p.Incantesimi.Gruppi) {
		return nil, errDatiNonValidi
	}
	g := p.Incantesimi.Gruppi[gruppo]
	if g.Usi == 0 { // trucchetti e incantesimi a volontà
		return map[string]any{"rimasti": -1}, nil
	}
	if p.Incantesimi.Usati == nil {
		p.Incantesimi.Usati = map[string]int{}
	}
	k := itoa(gruppo)
	cosa := "uno slot di " + g.Etichetta
	if g.Livello < 0 {
		cosa = "un uso di «" + g.Etichetta + "»"
	}
	if p.Incantesimi.Usati[k] >= g.Usi && !forza {
		return map[string]any{"serveConferma": c.Nome + " ha già usato tutti gli slot o gli usi di «" + g.Etichetta + "». Lanciare lo stesso?"}, nil
	}
	p.Incantesimi.Usati[k]++
	b, _ := json.Marshal(p)
	s.DB.Exec(`UPDATE combattenti SET profilo = ? WHERE id = ?`, string(b), c.ID)
	rimasti := max(0, g.Usi-p.Incantesimi.Usati[k])
	registro(c.Nome + " usa " + cosa + " (ne restano " + itoa(rimasti) + " di " + itoa(g.Usi) + ")")
	return map[string]any{"rimasti": rimasti}, nil
}

// ===== conseguenze delle condizioni (regole 2014, sintesi in web/sezioni/iniziativa.js, CONDIZIONI) =====

var reDistanzaMischia = regexp.MustCompile(`^\d+(,\d+)? m(\s|$)`) // «1,5 m», «3 m · lancio 6/18 m»

var condizioniSenzaAzioni = []string{"Incapacitato", "Paralizzato", "Pietrificato", "Stordito", "Privo di sensi"}

// nonPuoAgire: la condizione che impedisce azioni e reazioni (minuscolo), oppure "".
func nonPuoAgire(c *vistaCombattente) string {
	for _, x := range condizioniSenzaAzioni {
		if slices.Contains(c.Condizioni, x) {
			return strings.ToLower(x)
		}
	}
	return ""
}

func indebolimentoDi(c *vistaCombattente) int {
	for i := 6; i >= 1; i-- {
		if slices.Contains(c.Condizioni, "Indebolimento "+itoa(i)) {
			return i
		}
	}
	return 0
}

// aDistanza: attacco a distanza (gittata «24/96 m», «gittata …», incantesimo a distanza); altrimenti in mischia.
func aDistanza(p profiloAttacco) bool {
	t := strings.ToLower(strings.TrimSpace(p.Portata))
	if t == "" || strings.HasPrefix(t, "portata") || t == "mischia" || reDistanzaMischia.MatchString(t) {
		return false
	}
	return strings.Contains(t, "gittata") || strings.Contains(t, "/") || t == "distanza"
}

func critAutoMotivo(b *vistaCombattente) string {
	if slices.Contains(b.Condizioni, "Paralizzato") {
		return "paralizzato"
	}
	return "privo di sensi"
}

// modoConCondizioni unisce la scelta della pagina (vantaggio/svantaggio, per esempio Attacco irruento) con le condizioni:
// qualsiasi vantaggio più qualsiasi svantaggio = tiro normale. critAuto: da vicino contro paralizzato o privo di sensi.
func modoConCondizioni(scelto string, a, b *vistaCombattente, distanza bool) (modo string, fonti []string, critAuto bool) {
	adv, dis := 0, 0
	ha := func(c *vistaCombattente, n string) bool { return slices.Contains(c.Condizioni, n) }
	piu := func(t string) { adv++; fonti = append(fonti, t) }
	meno := func(t string) { dis++; fonti = append(fonti, t) }
	switch scelto {
	case "vantaggio":
		piu("vantaggio scelto")
	case "svantaggio":
		meno("svantaggio scelto")
	}
	for _, n := range []string{"Accecato", "Avvelenato", "Prono", "Trattenuto", "Spaventato"} {
		if ha(a, n) {
			meno("svantaggio: " + a.Nome + " è " + strings.ToLower(n))
		}
	}
	if indebolimentoDi(a) >= 3 {
		meno("svantaggio: indebolimento di " + a.Nome)
	}
	if ha(a, "Invisibile") {
		piu("vantaggio: " + a.Nome + " è invisibile")
	}
	for _, n := range []string{"Accecato", "Paralizzato", "Pietrificato", "Trattenuto", "Stordito", "Privo di sensi"} {
		if ha(b, n) {
			piu("vantaggio: " + b.Nome + " è " + strings.ToLower(n))
		}
	}
	if ha(b, "Invisibile") {
		meno("svantaggio: " + b.Nome + " è invisibile")
	}
	if b.mod != nil && b.mod.svantContro != "" { // effetto in corso sul bersaglio (Sfocatura): effetti_valori.go
		meno("svantaggio: " + b.mod.svantContro + " su " + b.Nome)
	}
	if ha(b, "Prono") {
		if distanza {
			meno("svantaggio: " + b.Nome + " è prono e l'attacco è a distanza")
		} else {
			piu("vantaggio: " + b.Nome + " è prono e l'attacco è da vicino")
		}
	}
	critAuto = !distanza && (ha(b, "Paralizzato") || ha(b, "Privo di sensi"))
	switch {
	case adv > 0 && dis > 0:
		modo = "normale"
		if len(fonti) > 0 {
			fonti = append(fonti, "vantaggio e svantaggio si annullano")
		}
	case adv > 0:
		modo = "vantaggio"
	case dis > 0:
		modo = "svantaggio"
	default:
		modo = "normale"
	}
	if (scelto == "vantaggio" || scelto == "svantaggio") && len(fonti) == 1 {
		fonti = nil // solo la scelta della pagina: niente da spiegare
	}
	return
}

// pubblicaEsito manda l'esito a tutti i presenti nella sessione, filtrato per ruolo; se c'è di mezzo un mostro nascosto,
// solo al Master.
func (s *Server) pubblicaEsito(sessione string, e esitoAttacco) {
	per := func(u *Utente) *esitoAttacco {
		ruolo, _ := s.ruoloIn(u, sessione)
		if ruolo == "" {
			return nil
		}
		if e.nascosto && ruolo != "master" {
			// solo il giocatore del PG colpito da un mostro nascosto, senza sapere chi è stato
			if e.nascostoA && e.propB != "" && e.propB == u.ID {
				x := e.perRuolo(ruolo, u.ID).anonimo()
				return &x
			}
			return nil
		}
		x := e.perRuolo(ruolo, u.ID)
		return &x
	}
	s.Hub.PubblicaPer("sessione:"+sessione, func(c *Cliente) *Msg {
		if c.U == nil {
			return nil
		}
		if x := per(c.U); x != nil {
			return &Msg{Tipo: "attacco.esito", Dati: *x}
		}
		return nil
	})
	// i giocatori del gioco da remoto (ponte_mqtt.go)
	s.remoto.evento(sessione, "attacco.esito", func(u *Utente) any {
		if x := per(u); x != nil {
			return *x
		}
		return nil
	})
}

// ===== profili nel riepilogo della scheda e nella colonna combattenti.profilo =====

// profiliDalRiepilogo legge «attacks», «attacksPerAction» e «defenses» mandati dalla scheda.
func profiliDalRiepilogo(raw json.RawMessage, per any, dif json.RawMessage) ([]profiloAttacco, int, *difeseCombattente) {
	var l []struct {
		Nome      string  `json:"nome"`
		Bonus     float64 `json:"bonus"`
		Portata   string  `json:"portata"`
		Magica    bool    `json:"magica"`
		Argentata bool    `json:"argentata"`
		Danni     []struct {
			Dadi any     `json:"dadi"`
			Mod  float64 `json:"mod"`
			Tipo string  `json:"tipo"`
		} `json:"danni"`
	}
	json.Unmarshal(raw, &l)
	var out []profiloAttacco
	for _, x := range l {
		p := profiloAttacco{Nome: x.Nome, Bonus: int(math.Round(x.Bonus)), Portata: x.Portata, Magica: x.Magica, Argentata: x.Argentata}
		for _, c := range x.Danni {
			dadi := ""
			switch v := c.Dadi.(type) {
			case string:
				dadi = v
			case float64:
				dadi = strconv.Itoa(int(v))
			}
			p.Danni = append(p.Danni, componenteDanno{Dadi: dadi, Mod: int(math.Round(c.Mod)), Tipo: c.Tipo})
		}
		out = append(out, p)
	}
	out = validaProfili(out)
	if len(out) == 0 {
		out = nil
	}
	n := 0
	if f, ok := per.(float64); ok && f >= 1 && f <= 8 {
		n = int(f)
	}
	var d struct {
		Res, Imm, Vuln []string
		Incantesimi    bool
	}
	var difese *difeseCombattente
	if json.Unmarshal(dif, &d) == nil {
		// «fuoco», «contundente (non magici)», «contundente (non magici e non argentati)» (web/regole/difese.js)
		voci := func(l []string) []difesa {
			var out []difesa
			for _, t := range l {
				out = append(out, leggiVoceDifesa(t))
				if len(out) >= 20 {
					break
				}
			}
			return out
		}
		x := validaDifese(difeseCombattente{Res: voci(d.Res), Imm: voci(d.Imm), Vuln: voci(d.Vuln), Incantesimi: d.Incantesimi})
		if len(x.Res)+len(x.Imm)+len(x.Vuln) > 0 || x.Incantesimi {
			difese = &x
		}
	}
	return out, n, difese
}

// validaProfiloMostro ricontrolla il profilo che la pagina del Master ha letto dalla scheda del mostro.
func validaProfiloMostro(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" || len(raw) > 64<<10 {
		return ""
	}
	var p profiloMostro
	if json.Unmarshal(raw, &p) != nil {
		return ""
	}
	p.Attacchi = validaProfili(p.Attacchi)
	if p.Multi < 0 || p.Multi > 10 {
		p.Multi = 0
	}
	p.Difese = validaDifese(p.Difese)
	// sequenza del Multiattacco: indici validi, volte da 1 a 10, la somma uguale agli attacchi per azione
	seq, tot := []passoMulti{}, 0
	for _, x := range p.Sequenza {
		if x.Attacco < 0 || x.Attacco >= len(p.Attacchi) || x.Volte < 1 || x.Volte > 10 || slices.ContainsFunc(seq, func(y passoMulti) bool { return y.Attacco == x.Attacco }) {
			seq = nil
			break
		}
		seq, tot = append(seq, x), tot+x.Volte
	}
	if len(seq) == 0 || tot != max(1, p.Multi) {
		seq = nil
	}
	p.Sequenza = seq
	ts := map[string]int{}
	for k, v := range p.TS {
		if caratteristicheTS[k] != "" && v >= -10 && v <= 25 {
			ts[k] = v
		}
	}
	p.TS = ts
	if p.Leggendaria < 0 || p.Leggendaria > 5 {
		p.Leggendaria = 0
	}
	imm := []string{}
	for _, c := range p.ImmCond {
		if condizioniAmmesse[c] && !slices.Contains(imm, c) {
			imm = append(imm, c)
		}
	}
	p.ImmCond = imm
	if x := p.Incantesimi; x != nil {
		g := []gruppoIncantesimi{}
		for _, y := range x.Gruppi {
			y.Etichetta = limita(y.Etichetta, 30)
			if y.Etichetta == "" || y.Livello < -1 || y.Livello > 9 || y.Usi < 0 || y.Usi > 9 || len(g) >= 12 ||
				y.CD < 0 || y.CD > 30 || y.Att < -5 || y.Att > 20 || y.Mod < -5 || y.Mod > 10 {
				continue
			}
			nomi := []string{}
			for _, n := range y.Nomi {
				if n = limita(n, 60); n != "" && len(nomi) < 20 {
					nomi = append(nomi, n)
				}
			}
			if len(nomi) > 0 {
				y.Nomi = nomi
				g = append(g, y)
			}
		}
		if len(g) == 0 || x.CD < 0 || x.CD > 30 || x.Att < -5 || x.Att > 20 || x.Mod < -5 || x.Mod > 10 || x.Liv < 0 || x.Liv > 30 {
			p.Incantesimi = nil
		} else {
			x.Gruppi, x.Usati = g, nil
		}
	}
	if len(p.Attacchi) == 0 && len(p.Difese.Res)+len(p.Difese.Imm)+len(p.Difese.Vuln) == 0 && len(p.TS) == 0 && p.Incantesimi == nil {
		return ""
	}
	b, _ := json.Marshal(p)
	return string(b)
}

// impostaAttacchi completa un combattente letto dal database con attacchi, attacchi per azione e difese.
func (v *vistaCombattente) impostaAttacchi(profilo string, r *riepilogo) {
	v.PerAzione = 1
	if v.Tipo == "mostro" {
		var p profiloMostro
		if json.Unmarshal([]byte(profilo), &p) == nil {
			v.Attacchi, v.difese, v.Incantesimi, v.Multiattacco = p.Attacchi, p.Difese, p.Incantesimi, p.Sequenza
			if p.Multi > 1 {
				v.PerAzione = p.Multi
			}
		}
		return
	}
	if r != nil {
		v.Attacchi = r.Attacchi
		if r.AttacchiPerAzione > 1 {
			v.PerAzione = r.AttacchiPerAzione
		}
		if r.Difese != nil {
			v.difese = *r.Difese
		}
	}
}
