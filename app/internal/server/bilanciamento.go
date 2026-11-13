package server

// Bilanciamento delle avventure (richiesta dell'utente del 2026-11-12). Parte GENERALE, riusabile per ogni «modello di
// avventura» (il primo è bilanciamento_orfanotrofio.go). Legge i personaggi presenti alla sessione (o quelli attivi del
// gruppo) dalle loro schede, senza chiedere nulla al Master, e ne stima la potenza:
//   - soglie di PE per incontro (facile, media, difficile, mortale) e PE della giornata, secondo il LIVELLO di ogni PG,
//     come prevedono le regole della 5e: il livello di un PG non è un GS e non viene mai convertito in GS;
//   - ruoli da 0 a 3 (cura, controllo, indagine, protezione, danno, sociale, contro il soprannaturale), dalle classi
//     (multiclasse in proporzione ai livelli), dalle abilità con competenza e dagli incantesimi disponibili;
//   - risorse: PF, slot (e quanti ne sono già spesi, se la sessione è in corso), incantesimi utili;
//   - un fattore di ruolo (dal −25% al +15%) che corregge le soglie: la sola somma dei livelli non basta.
// Le regole sono sintesi scritte con parole nostre; i numeri delle tabelle sono quelli della 5e (regole 2014).

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// soglie di PE per personaggio, per livello: facile, media, difficile, mortale
var soglieLivello = [21][4]int{{}, {25, 50, 75, 100}, {50, 100, 150, 200}, {75, 150, 225, 400}, {125, 250, 375, 500},
	{250, 500, 750, 1100}, {300, 600, 900, 1400}, {350, 750, 1100, 1700}, {450, 900, 1400, 2100}, {550, 1100, 1600, 2400},
	{600, 1200, 1900, 2800}, {800, 1600, 2400, 3600}, {1000, 2000, 3000, 4500}, {1100, 2200, 3400, 5100},
	{1250, 2500, 3800, 5700}, {1400, 2800, 4300, 6400}, {1600, 3200, 4800, 7200}, {2000, 3900, 5900, 8800},
	{2100, 4200, 6300, 9500}, {2400, 4900, 7300, 10900}, {2800, 5700, 8500, 12700}}

// PE che un personaggio regge in una giornata di avventura (con un paio di riposi brevi), per livello
var peGiornata = [21]int{0, 300, 600, 1200, 1700, 3500, 4000, 5000, 6000, 7500, 9000, 10500, 11500, 13500, 15000, 18000,
	20000, 25000, 27000, 30000, 40000}

var livelliDifficolta = []string{"Facile", "Media", "Difficile", "Molto difficile", "Estrema"}

// moltiplicatore dei PE per il numero di avversari, spostato di un passo con meno di 3 o con 6 e più personaggi
func moltiplicatoreIncontro(nMostri, nPG int) float64 {
	passi := []float64{0.5, 1, 1.5, 2, 2.5, 3, 4, 5}
	i := 1
	switch {
	case nMostri <= 1:
		i = 1
	case nMostri == 2:
		i = 2
	case nMostri <= 6:
		i = 3
	case nMostri <= 10:
		i = 4
	case nMostri <= 14:
		i = 5
	default:
		i = 6
	}
	if nPG < 3 {
		i++
	} else if nPG >= 6 {
		i--
	}
	return passi[max(0, min(len(passi)-1, i))]
}

var ruoliOrdine = []string{"cura", "controllo", "indagine", "protezione", "danno", "sociale", "soprannaturale"}
var nomiRuoli = map[string]string{"cura": "cura", "controllo": "controllo", "indagine": "indagine ed esplorazione",
	"protezione": "protezione", "danno": "danno", "sociale": "sociale", "soprannaturale": "contro il soprannaturale"}

// ruoli tipici di ogni classe (0-3)
var ruoliClasse = map[string]map[string]int{
	"artefice":  {"protezione": 2, "indagine": 2, "cura": 1, "soprannaturale": 1, "danno": 1},
	"barbaro":   {"danno": 3, "protezione": 2},
	"bardo":     {"sociale": 3, "controllo": 2, "cura": 2, "indagine": 1},
	"chierico":  {"cura": 3, "soprannaturale": 3, "protezione": 2, "controllo": 1, "danno": 1},
	"druido":    {"cura": 2, "controllo": 2, "indagine": 2, "soprannaturale": 1, "danno": 1},
	"guerriero": {"danno": 3, "protezione": 2},
	"ladro":     {"indagine": 3, "danno": 2, "sociale": 2},
	"mago":      {"controllo": 3, "danno": 2, "indagine": 2, "soprannaturale": 1},
	"monaco":    {"danno": 2, "controllo": 1, "indagine": 1, "protezione": 1},
	"paladino":  {"protezione": 3, "danno": 2, "soprannaturale": 2, "cura": 2, "sociale": 2},
	"ranger":    {"danno": 2, "indagine": 3, "cura": 1},
	"stregone":  {"danno": 3, "controllo": 2, "sociale": 2},
	"warlock":   {"danno": 2, "controllo": 1, "sociale": 2, "soprannaturale": 1},
}

// codice delle classi nell'elenco degli incantesimi (web/regole/dati.js)
var codiceClasseInc = map[string]string{"artefice": "a", "bardo": "b", "chierico": "c", "druido": "d", "paladino": "p",
	"ranger": "r", "stregone": "s", "warlock": "w", "mago": "m"}

// classi che preparano ogni giorno dall'intero elenco della classe
var classiPreparate = map[string]bool{"chierico": true, "druido": true, "paladino": true, "artefice": true}

// incantesimi che contano per il bilanciamento: nome italiano, livello, classi, ruoli; Salva: libera un bambino
// dall'influenza (bilanciamento_orfanotrofio.go)
type incBil struct {
	It     string
	Liv    int
	Classi string
	Ruoli  []string
	Salva  bool
}

var incantesimiBil = map[string]incBil{
	"Cure Wounds":                   {"Cura ferite", 1, "abcdpr", []string{"cura"}, false},
	"Healing Word":                  {"Parola guaritrice", 1, "bcd", []string{"cura"}, false},
	"Prayer of Healing":             {"Preghiera di guarigione", 2, "cp", []string{"cura"}, false},
	"Mass Healing Word":             {"Parola guaritrice di massa", 3, "bc", []string{"cura"}, false},
	"Mass Cure Wounds":              {"Cura ferite di massa", 5, "bcd", []string{"cura"}, false},
	"Aura of Vitality":              {"Aura di vitalità", 3, "cdp", []string{"cura"}, false},
	"Revivify":                      {"Rinascita", 3, "acdpr", []string{"cura"}, false},
	"Lesser Restoration":            {"Ristorare inferiore", 2, "abcdpr", []string{"soprannaturale"}, true},
	"Protection from Evil and Good": {"Protezione dal bene e dal male", 1, "cdpwm", []string{"soprannaturale", "protezione"}, true},
	"Remove Curse":                  {"Rimuovi maledizione", 3, "cpwm", []string{"soprannaturale"}, true},
	"Dispel Evil and Good":          {"Dissolvi il bene e il male", 5, "cp", []string{"soprannaturale"}, true},
	"Greater Restoration":           {"Ristorare superiore", 5, "abcdr", []string{"soprannaturale"}, true},
	"Magic Circle":                  {"Cerchio magico", 3, "cpwm", []string{"soprannaturale"}, false},
	"Detect Evil and Good":          {"Individuazione del bene e del male", 1, "cp", []string{"indagine", "soprannaturale"}, false},
	"Detect Magic":                  {"Individuazione del magico", 1, "abcdprsm", []string{"indagine"}, false},
	"Locate Object":                 {"Localizzare oggetto", 2, "bcdprm", []string{"indagine"}, false},
	"Speak with Dead":               {"Parlare con i morti", 3, "bcm", []string{"indagine"}, false},
	"Augury":                        {"Augurio", 2, "cdm", []string{"indagine"}, false},
	"Clairvoyance":                  {"Chiaroveggenza", 3, "bcsm", []string{"indagine"}, false},
	"Zone of Truth":                 {"Zona di verità", 2, "bcp", []string{"sociale"}, false},
	"Detect Thoughts":               {"Individuazione dei pensieri", 2, "bsm", []string{"sociale", "indagine"}, false},
	"Calm Emotions":                 {"Calmare emozioni", 2, "bc", []string{"sociale", "controllo"}, false},
	"Sleep":                         {"Sonno", 1, "bsm", []string{"controllo"}, false},
	"Hold Person":                   {"Blocca persone", 2, "bcdswm", []string{"controllo"}, false},
	"Web":                           {"Ragnatela", 2, "asm", []string{"controllo"}, false},
	"Hypnotic Pattern":              {"Trama ipnotica", 3, "bswm", []string{"controllo"}, false},
	"Command":                       {"Comando", 1, "bcp", []string{"controllo"}, false},
	"Banishment":                    {"Esilio", 4, "cpswm", []string{"controllo"}, false},
	"Slow":                          {"Lentezza", 3, "bsm", []string{"controllo"}, false},
	"Entangle":                      {"Intralciare", 1, "dr", []string{"controllo"}, false},
	"Spirit Guardians":              {"Guardiani spirituali", 3, "c", []string{"danno", "protezione"}, false},
	"Sanctuary":                     {"Santuario", 1, "ac", []string{"protezione"}, false},
	"Shield of Faith":               {"Scudo della fede", 1, "cp", []string{"protezione"}, false},
	"Aid":                           {"Aiuto", 2, "abcpr", []string{"protezione"}, false},
	"Warding Bond":                  {"Legame protettivo", 2, "cp", []string{"protezione"}, false},
}

// abilità → caratteristica
var carAbilita = map[string]string{"Acrobazia": "DES", "Addestrare Animali": "SAG", "Arcano": "INT", "Atletica": "FOR",
	"Furtività": "DES", "Indagare": "INT", "Inganno": "CAR", "Intimidire": "CAR", "Intrattenere": "CAR", "Intuizione": "SAG",
	"Medicina": "SAG", "Natura": "INT", "Percezione": "SAG", "Persuasione": "CAR", "Rapidità di Mano": "DES",
	"Religione": "INT", "Sopravvivenza": "SAG", "Storia": "INT"}

type classeLiv struct {
	Nome    string
	Livello int
}

// pgBil: un personaggio come lo vede il bilanciamento
type pgBil struct {
	ID          string         `json:"id"`
	Nome        string         `json:"nome"`
	Classi      string         `json:"classi"`
	Livello     int            `json:"livello"`
	CA          int            `json:"ca"`
	PFMax       int            `json:"pfMax"`
	PF          int            `json:"pf"`
	Ruoli       map[string]int `json:"ruoli"`
	Incantesimi []string       `json:"incantesimi,omitempty"`
	Slot        int            `json:"slot"`
	SlotUsati   int            `json:"slotUsati"`
	classi      []classeLiv
	car         map[string]int
	abil        []string
	comp        int
	percPass    int
	slotLiv     map[int]int     // slot per livello (con quelli del patto)
	inc         map[string]bool // incantesimi disponibili (chiave inglese)
}

var reClasseLiv = regexp.MustCompile(`^\s*(.*?)\s*(\d+)?\s*$`)

// leggiClassi: «Chierico» oppure «Guerriero 3 / Chierico 2»
func leggiClassi(testo string, livello int) []classeLiv {
	var out []classeLiv
	parti := strings.Split(testo, "/")
	for _, p := range parti {
		m := reClasseLiv.FindStringSubmatch(p)
		if m == nil || m[1] == "" {
			continue
		}
		nome := ""
		low := strings.ToLower(m[1])
		for k := range ruoliClasse {
			if strings.HasPrefix(low, k) {
				nome = k
			}
		}
		if nome == "" {
			continue
		}
		lv, _ := strconv.Atoi(m[2])
		if lv == 0 && len(parti) == 1 {
			lv = livello
		}
		out = append(out, classeLiv{nome, max(lv, 1)})
	}
	return out
}

// bonus di un personaggio in un'abilità (caratteristica + competenza, se ce l'ha)
func (p *pgBil) bonus(abilita string) int {
	b := 0
	if v := p.car[carAbilita[abilita]]; v > 0 {
		b = Modificatore(v)
	}
	if slices.Contains(p.abil, abilita) {
		b += p.comp
	}
	return b
}

// livello massimo degli incantesimi che una classe preparata ha a quel livello di classe
func livelloMaxIncClasse(classe string, lv int) int {
	switch classe {
	case "chierico", "druido":
		return min(9, (lv+1)/2)
	case "paladino":
		if lv < 2 {
			return 0
		}
		return min(5, (lv-1)/4+1)
	case "artefice":
		return min(5, (lv-1)/4+1)
	}
	return 0
}

func (p *pgBil) ha(en string) bool { return p.inc[en] }

// slot utilizzabili per un incantesimo di quel livello (anche quelli del patto)
func (p *pgBil) slotDaLivello(liv int) int {
	n := 0
	for l, k := range p.slotLiv {
		if l >= liv {
			n += k
		}
	}
	return n
}

// caricaPGBil legge i personaggi indicati dalle schede (versione corrente e stato in gioco).
func (s *Server) caricaPGBil(ids []string) []*pgBil {
	var out []*pgBil
	for _, id := range ids {
		var nome, dati, riep string
		var pf *int
		var usati string
		if s.DB.QueryRow(`SELECT p.nome, COALESCE(v.dati,'{}'), COALESCE(v.riepilogo,'{}'), sg.pf, COALESCE(sg.slot_usati,'{}')
			FROM personaggi p LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
			LEFT JOIN stato_in_gioco sg ON sg.personaggio_id = p.id WHERE p.id = ?`, id).Scan(&nome, &dati, &riep, &pf, &usati) != nil {
			continue
		}
		var rp riepilogo
		json.Unmarshal([]byte(riep), &rp)
		p := &pgBil{ID: id, Nome: nome, Classi: rp.Classe, Livello: max(1, min(20, rp.Livello)), CA: rp.CA, PFMax: rp.PFMax,
			car: rp.Car, abil: rp.Abilita, percPass: rp.PercPass, slotLiv: map[int]int{}, inc: map[string]bool{}}
		p.comp = BonusCompetenza(p.Livello)
		p.PF = p.PFMax
		if pf != nil {
			p.PF = *pf
		}
		p.classi = leggiClassi(rp.Classe, p.Livello)
		for k, n := range rp.Slot {
			if l, err := strconv.Atoi(k); err == nil && l >= 1 && l <= 9 && n > 0 {
				p.slotLiv[l] += n
				p.Slot += n
			}
		}
		if rp.Patto != nil {
			p.slotLiv[rp.Patto.Lv] += rp.Patto.N
			p.Slot += rp.Patto.N
		}
		var u map[string]int
		if json.Unmarshal([]byte(usati), &u) == nil {
			for _, n := range u {
				p.SlotUsati += max(0, n)
			}
		}
		p.SlotUsati = min(p.SlotUsati, p.Slot)
		// incantesimi scritti nella scheda (chiave inglese, o nome scritto a mano)
		var d struct {
			Spl map[string][]struct {
				K string `json:"k"`
				C string `json:"c"`
			} `json:"spl"`
		}
		json.Unmarshal([]byte(dati), &d)
		perIt := map[string]string{}
		for en, x := range incantesimiBil {
			perIt[normalizza(x.It)] = en
		}
		for _, l := range d.Spl {
			for _, x := range l {
				if _, ok := incantesimiBil[x.K]; ok {
					p.inc[x.K] = true
				} else if en, ok := perIt[normalizza(x.C)]; ok {
					p.inc[en] = true
				}
			}
		}
		// classi che preparano: tutto l'elenco della classe fino al livello che possono lanciare
		for _, c := range p.classi {
			if !classiPreparate[c.Nome] {
				continue
			}
			mx := livelloMaxIncClasse(c.Nome, c.Livello)
			for en, x := range incantesimiBil {
				if x.Liv <= mx && strings.Contains(x.Classi, codiceClasseInc[c.Nome]) {
					p.inc[en] = true
				}
			}
		}
		for en := range p.inc {
			p.Incantesimi = append(p.Incantesimi, incantesimiBil[en].It)
		}
		sort.Strings(p.Incantesimi)
		p.Ruoli = ruoliPG(p)
		out = append(out, p)
	}
	return out
}

// ruoliPG: dalle classi (in proporzione ai livelli), +1 per gli incantesimi e per le abilità forti; massimo 3
func ruoliPG(p *pgBil) map[string]int {
	tot := 0
	for _, c := range p.classi {
		tot += c.Livello
	}
	f := map[string]float64{}
	for _, c := range p.classi {
		for k, w := range ruoliClasse[c.Nome] {
			f[k] += float64(w*c.Livello) / float64(max(tot, 1))
		}
	}
	out := map[string]int{}
	for _, k := range ruoliOrdine {
		out[k] = int(math.Round(f[k]))
	}
	piu := map[string]bool{}
	for en := range p.inc {
		for _, k := range incantesimiBil[en].Ruoli {
			piu[k] = true
		}
	}
	if p.bonus("Indagare") >= 5 || p.percPass >= 15 {
		piu["indagine"] = true
	}
	if max(p.bonus("Persuasione"), p.bonus("Intuizione")) >= 5 {
		piu["sociale"] = true
	}
	if p.CA >= 17 {
		piu["protezione"] = true
	}
	for k := range piu {
		out[k]++
	}
	for k, v := range out {
		out[k] = min(3, v)
	}
	return out
}

// rigaBil: una voce della scheda di bilanciamento con il valore e il perché
type rigaBil struct {
	Voce   string `json:"voce"`
	Valore string `json:"valore"`
	Perche string `json:"perche,omitempty"`
}

type sezioneBil struct {
	Titolo string    `json:"titolo"`
	Righe  []rigaBil `json:"righe"`
}

// gruppoBil: la potenza del gruppo
type gruppoBil struct {
	PG           []*pgBil
	Soglie       [4]int // somma delle soglie dei PG
	Giornata     int
	LivelloMedio float64
	Fattore      float64 // correzione dei ruoli e delle risorse
	Correzioni   []rigaBil
	RuoliMax     map[string]int
	RuoliSomma   map[string]int
	Equivalente  int // livello di un gruppo standard di 4 PG con la stessa potenza
}

func nuovoGruppoBil(pg []*pgBil) *gruppoBil {
	g := &gruppoBil{PG: pg, RuoliMax: map[string]int{}, RuoliSomma: map[string]int{}, Fattore: 1}
	pfTot, pfMax, slot, usati, liv := 0, 0, 0, 0, 0
	for _, p := range pg {
		for i := 0; i < 4; i++ {
			g.Soglie[i] += soglieLivello[p.Livello][i]
		}
		g.Giornata += peGiornata[p.Livello]
		liv += p.Livello
		for k, v := range p.Ruoli {
			g.RuoliMax[k] = max(g.RuoliMax[k], v)
			g.RuoliSomma[k] += v
		}
		pfTot += max(0, p.PF)
		pfMax += p.PFMax
		slot += p.Slot
		usati += p.SlotUsati
	}
	if len(pg) == 0 {
		return g
	}
	g.LivelloMedio = float64(liv) / float64(len(pg))
	corr := func(v float64, voce, perche string) {
		g.Fattore += v
		g.Correzioni = append(g.Correzioni, rigaBil{voce, fmt.Sprintf("%+.0f%%", v*100), perche})
	}
	if g.RuoliMax["cura"] < 2 {
		corr(-0.10, "Poca cura", "nessun personaggio cura in modo affidabile: ogni ferita pesa di più e il gruppo regge meno scontri di fila")
	} else if g.RuoliSomma["cura"] >= 5 {
		corr(0.05, "Cura abbondante", "più personaggi sanno curare: il gruppo recupera tra uno scontro e l'altro")
	}
	if g.RuoliMax["protezione"] <= 1 {
		corr(-0.05, "Nessuna prima linea", "manca chi assorbe i colpi: i nemici raggiungono facilmente i personaggi fragili")
	} else if g.RuoliMax["protezione"] >= 3 {
		corr(0.05, "Prima linea solida", "c'è chi tiene i nemici lontano dagli altri")
	}
	if g.RuoliMax["controllo"] >= 3 {
		corr(0.05, "Controllo del campo", "incantesimi che bloccano o disperdono i nemici dimezzano il peso dei gruppi numerosi")
	}
	if g.RuoliMax["soprannaturale"] >= 3 {
		corr(0.05, "Armi contro il soprannaturale", "magie che proteggono dal male e scacciano i non morti")
	}
	if pfMax > 0 && float64(pfTot)/float64(pfMax) < 0.6 {
		corr(-0.10, "Gruppo ferito", fmt.Sprintf("adesso il gruppo ha %d PF su %d: senza un riposo parte svantaggiato", pfTot, pfMax))
	}
	if slot > 0 && float64(usati)/float64(slot) > 0.5 {
		corr(-0.05, "Magia già spesa", fmt.Sprintf("sono già stati usati %d slot su %d", usati, slot))
	}
	g.Fattore = math.Max(0.75, math.Min(1.15, g.Fattore))
	// gruppo standard equivalente: 4 PG di livello X con soglie «media» simili
	eff := float64(g.Soglie[1]) * g.Fattore
	best, diff := 1, math.MaxFloat64
	for l := 1; l <= 20; l++ {
		if d := math.Abs(float64(4*soglieLivello[l][1]) - eff); d < diff {
			best, diff = l, d
		}
	}
	g.Equivalente = best
	return g
}

// soglia: x = 0 facile, 1 media, 2 difficile, 3 mortale (in mezzo si interpola; oltre 3, +50% per punto), corretta dal fattore
func (g *gruppoBil) soglia(x float64) float64 {
	s := g.Soglie
	var v float64
	switch {
	case x <= 0:
		v = float64(s[0]) * math.Max(0.25, 1+x)
	case x >= 3:
		v = float64(s[3]) * (1 + (x-3)*0.5)
	default:
		i := int(x)
		v = float64(s[i]) + (x-float64(i))*float64(s[i+1]-s[i])
	}
	return v * g.Fattore
}

// classificaPE: difficoltà di un incontro dai PE corretti (0 Facile … 4 Estrema)
func (g *gruppoBil) classificaPE(adj float64) int {
	switch {
	case adj < g.soglia(1):
		return 0
	case adj < g.soglia(2):
		return 1
	case adj < g.soglia(3):
		return 2
	case adj < g.soglia(3)*1.5:
		return 3
	}
	return 4
}

func (g *gruppoBil) migliore(f func(p *pgBil) int) (int, string) {
	b, chi := -99, ""
	for _, p := range g.PG {
		if v := f(p); v > b {
			b, chi = v, p.Nome
		}
	}
	return b, chi
}

func (g *gruppoBil) qualcunoHa(en ...string) []string {
	var out []string
	for _, p := range g.PG {
		for _, e := range en {
			if p.ha(e) && !slices.Contains(out, p.Nome) {
				out = append(out, p.Nome)
			}
		}
	}
	return out
}

// probProva: probabilità di superare una CD con un bonus (con vantaggio se qualcuno aiuta)
func probProva(bonus, cd int, vantaggio bool) float64 {
	p := math.Max(0.05, math.Min(0.95, float64(21-(cd-bonus))/20))
	if vantaggio {
		p = 1 - (1-p)*(1-p)
	}
	return p
}

func classificaProb(p float64) int {
	switch {
	case p >= 0.85:
		return 0
	case p >= 0.65:
		return 1
	case p >= 0.45:
		return 2
	case p >= 0.25:
		return 3
	}
	return 4
}

func conSegnoInt(n int) string {
	if n >= 0 {
		return "+" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// righeGruppo: la sezione «Gruppo» comune a tutti i modelli
func (g *gruppoBil) righeGruppo(origine string) []rigaBil {
	r := []rigaBil{{"Personaggi", strconv.Itoa(len(g.PG)), origine}}
	for _, p := range g.PG {
		var ruoli []string
		for _, k := range ruoliOrdine {
			if p.Ruoli[k] >= 2 {
				ruoli = append(ruoli, nomiRuoli[k]+strings.Repeat("●", p.Ruoli[k]))
			}
		}
		val := fmt.Sprintf("livello %d", p.Livello)
		if p.Classi != "" {
			val = p.Classi + " · " + val
		}
		val += fmt.Sprintf(" · CA %d · PF %d/%d", p.CA, p.PF, p.PFMax)
		if p.Slot > 0 {
			val += fmt.Sprintf(" · slot %d/%d", p.Slot-p.SlotUsati, p.Slot)
		}
		perche := "ruoli forti: " + strings.Join(ruoli, ", ")
		if len(ruoli) == 0 {
			perche = "nessun ruolo spiccato"
		}
		if len(p.Incantesimi) > 0 {
			perche += "; incantesimi utili: " + strings.Join(p.Incantesimi, ", ")
		}
		r = append(r, rigaBil{p.Nome, val, perche})
	}
	r = append(r, rigaBil{"Livello medio", fmt.Sprintf("%.1f", g.LivelloMedio),
		"i personaggi hanno un livello, non un GS: il GS vale solo per i mostri; qui si usano le soglie di PE che la 5e dà per ogni livello"})
	r = append(r, rigaBil{"Soglie di PE del gruppo", fmt.Sprintf("facile %d · media %d · difficile %d · mortale %d", g.Soglie[0], g.Soglie[1], g.Soglie[2], g.Soglie[3]),
		"somma delle soglie di ogni personaggio per il suo livello"})
	r = append(r, rigaBil{"PE della giornata", strconv.Itoa(g.Giornata), "quanti PE di avversari il gruppo regge in una giornata con un paio di riposi brevi"})
	var ruoli []string
	for _, k := range ruoliOrdine {
		ruoli = append(ruoli, fmt.Sprintf("%s %d", nomiRuoli[k], g.RuoliMax[k]))
	}
	r = append(r, rigaBil{"Ruoli coperti (migliore del gruppo, 0-3)", strings.Join(ruoli, " · "), "dalle classi, dalle abilità con competenza e dagli incantesimi disponibili"})
	for _, c := range g.Correzioni {
		r = append(r, rigaBil{"Correzione: " + c.Voce, c.Valore, c.Perche})
	}
	r = append(r, rigaBil{"Potenza effettiva", fmt.Sprintf("fattore %.2f · come 4 personaggi di livello %d", g.Fattore, g.Equivalente),
		"le soglie corrette dal fattore paragonate a quelle di un gruppo standard di quattro personaggi"})
	return r
}

// ===== modelli e API =====

type esitoBil struct {
	Modello      string       `json:"modello"`
	Titolo       string       `json:"titolo"`
	Gruppo       string       `json:"gruppo"`
	NomeGruppo   string       `json:"nomeGruppo"`
	Obiettivo    int          `json:"obiettivo"`
	Consigliato  int          `json:"consigliato"`
	Complessiva  string       `json:"complessiva"`
	Sezioni      []sezioneBil `json:"sezioni"`
	Collegamenti []string     `json:"collegamenti"`
	DomandaIA    string       `json:"domandaIA"`
}

type modelloBil struct {
	ID     string
	Nome   string
	Schede []string // schede del Codice da cui si apre
	// obiettivo 1 Media, 2 Difficile, 3 Molto difficile; 0 = quello consigliato
	Calcola func(g *gruppoBil, obiettivo int, origine string) *esitoBil
}

var modelliBil = map[string]*modelloBil{}

func (s *Server) rotteBilanciamento(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/bilanciamento", api(s.apiModelliBil))
	mux.Handle("GET /api/v1/bilanciamento/gruppo", api(s.apiGruppoBil))
	mux.Handle("POST /api/v1/bilanciamento/{modello}", api(s.apiCalcolaBil))
	mux.Handle("POST /api/v1/bilanciamento/{modello}/salva", api(s.apiSalvaBil))
}

func (s *Server) apiModelliBil(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, m := range modelliBil {
		out = append(out, map[string]any{"id": m.ID, "nome": m.Nome, "schede": m.Schede})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["nome"].(string) < out[j]["nome"].(string) })
	return out, nil
}

// sessioneBil: la sessione da usare (quella indicata, o in corso, o la prossima pianificata)
func (s *Server) sessioneBil(gruppo, sessione string) (id, titolo, stato string) {
	q := `SELECT id, titolo, stato FROM sessioni_gioco WHERE gruppo_id = ? `
	if sessione != "" {
		s.DB.QueryRow(q+`AND id = ?`, gruppo, sessione).Scan(&id, &titolo, &stato)
		return
	}
	if s.DB.QueryRow(q+`AND stato = 'in_corso' ORDER BY inizio_il DESC LIMIT 1`, gruppo).Scan(&id, &titolo, &stato) == nil {
		return
	}
	s.DB.QueryRow(q+`AND stato = 'pianificata' ORDER BY inizio_il LIMIT 1`, gruppo).Scan(&id, &titolo, &stato)
	return
}

// apiGruppoBil: GET /bilanciamento/gruppo?gruppo=&sessione= → sessioni del gruppo e personaggi, con chi è presente
func (s *Server) apiGruppoBil(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	gruppo := r.URL.Query().Get("gruppo")
	sessioni := []map[string]any{}
	if rows, err := s.DB.Query(`SELECT id, titolo, stato, inizio_il FROM sessioni_gioco WHERE gruppo_id = ? AND stato != 'conclusa' ORDER BY inizio_il`, gruppo); err == nil {
		for rows.Next() {
			var id, t, st string
			var in int64
			rows.Scan(&id, &t, &st, &in)
			sessioni = append(sessioni, map[string]any{"id": id, "titolo": t, "stato": st, "inizio": in})
		}
		rows.Close()
	}
	ses, _, _ := s.sessioneBil(gruppo, r.URL.Query().Get("sessione"))
	presenti := map[string]bool{}
	if ses != "" {
		if rows, err := s.DB.Query(`SELECT personaggio_id FROM partecipanti WHERE sessione_id = ? AND personaggio_id IS NOT NULL`, ses); err == nil {
			for rows.Next() {
				var id string
				rows.Scan(&id)
				presenti[id] = true
			}
			rows.Close()
		}
	}
	pg := []map[string]any{}
	if rows, err := s.DB.Query(`SELECT p.id, p.nome, COALESCE(v.riepilogo,'{}') FROM personaggi p
		LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		WHERE p.gruppo_id = ? AND p.stato = 'attivo' AND p.proprietario_id IS NOT NULL ORDER BY p.nome`, gruppo); err == nil {
		for rows.Next() {
			var id, nome, riep string
			rows.Scan(&id, &nome, &riep)
			var rp riepilogo
			json.Unmarshal([]byte(riep), &rp)
			pg = append(pg, map[string]any{"id": id, "nome": nome, "classi": rp.Classe, "livello": rp.Livello,
				"presente": presenti[id] || len(presenti) == 0})
		}
		rows.Close()
	}
	return map[string]any{"sessioni": sessioni, "sessione": ses, "pg": pg}, nil
}

type richiestaBil struct {
	Gruppo    string   `json:"gruppo"`
	Sessione  string   `json:"sessione"`
	PG        []string `json:"pg"`
	Obiettivo int      `json:"obiettivo"`
}

func (s *Server) calcolaBil(r *http.Request) (*esitoBil, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	m := modelliBil[r.PathValue("modello")]
	if m == nil {
		return nil, errNonTrovato
	}
	var in richiestaBil
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	var nomeGruppo string
	if s.DB.QueryRow(`SELECT nome FROM gruppi WHERE id = ?`, in.Gruppo).Scan(&nomeGruppo) != nil {
		return nil, errore(http.StatusBadRequest, "Scegli un gruppo.")
	}
	if in.Obiettivo < 0 || in.Obiettivo > 3 || len(in.PG) > 12 {
		return nil, errDatiNonValidi
	}
	// solo personaggi attivi di quel gruppo (gruppi separati)
	var ids []string
	for _, id := range in.PG {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM personaggi WHERE id = ? AND gruppo_id = ? AND stato = 'attivo'`, id, in.Gruppo).Scan(&n)
		if n == 1 && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, errore(http.StatusBadRequest, "Scegli almeno un personaggio presente.")
	}
	origine := "personaggi scelti dal Master tra quelli attivi del gruppo"
	if ses, titolo, _ := s.sessioneBil(in.Gruppo, in.Sessione); ses != "" {
		origine = "dalla sessione «" + titolo + "» (si possono togliere gli assenti)"
	}
	g := nuovoGruppoBil(s.caricaPGBil(ids))
	e := m.Calcola(g, in.Obiettivo, origine)
	e.Gruppo, e.NomeGruppo = in.Gruppo, nomeGruppo
	return e, nil
}

func (s *Server) apiCalcolaBil(w http.ResponseWriter, r *http.Request) (any, error) {
	return s.calcolaBil(r)
}

// testoBil: la scheda in Markdown (per il Codice e per l'IA)
func testoBil(e *esitoBil) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Gruppo: %s. Difficoltà scelta: %s (consigliata: %s). Difficoltà complessiva: %s.\n",
		e.NomeGruppo, livelliDifficolta[e.Obiettivo], livelliDifficolta[e.Consigliato], e.Complessiva)
	for _, sz := range e.Sezioni {
		b.WriteString("\n" + strings.ToUpper(sz.Titolo) + "\n")
		for _, r := range sz.Righe {
			b.WriteString("- " + r.Voce + ": " + r.Valore)
			if r.Perche != "" {
				b.WriteString(" (perché: " + r.Perche + ")")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// apiSalvaBil: salva (o aggiorna) la scheda come «Trama: Bilanciamento – <modello> (<gruppo>)» del Codice
func (s *Server) apiSalvaBil(w http.ResponseWriter, r *http.Request) (any, error) {
	e, err := s.calcolaBil(r)
	if err != nil {
		return nil, err
	}
	nome := "Bilanciamento – " + e.Titolo + " (" + e.NomeGruppo + ")"
	x := schedaLore{Tipo: "trama", Nome: nome, TestoMaster: testoBil(e),
		Campi: map[string]string{"Gruppo": e.NomeGruppo, "Difficoltà": e.Complessiva + " (scelta: " + livelliDifficolta[e.Obiettivo] + ")",
			"Ruolo": "Bilanciamento calcolato da Gaster Bless per questo gruppo"},
		Collegamenti: e.Collegamenti, Etichette: []string{"approvato", "campagna"}}
	x, msg := pulisciLore(x)
	if msg != "" {
		return nil, errore(http.StatusBadRequest, msg)
	}
	s.DB.QueryRow(`SELECT id FROM lore_schede WHERE tipo = 'trama' AND nome = ? COLLATE NOCASE`, nome).Scan(&x.ID)
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	id, err := s.scriviLore(tx, x)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "nome": nome, "domandaIA": e.DomandaIA}, nil
}
