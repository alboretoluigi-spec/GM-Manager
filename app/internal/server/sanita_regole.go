package server

// Motore delle regole di Sanità mentale, Terrore e pazzia: solo funzioni pure, senza database né rete,
// così si provano da sole (sanita_regole_test.go). Le parti ufficiali (DMG 2014) e quelle homebrew
// (specifica dell'utente) sono separate e marcate.

import "fmt"

// ===== Caratteristica Sanità mentale (UFFICIALE DMG 2014, cap. 9) =====

// Metodi di assegnazione del punteggio.
const (
	MetodoDMG      = "DMG_STANDARD"   // ufficiale: settima caratteristica, generata come le altre
	MetodoSaggezza = "WISDOM_DERIVED" // HOMEBREW: 10 + modificatore di Saggezza
	MetodoManuale  = "MANUAL"         // inserito dal Master
)

// Modificatore: la normale tabella delle caratteristiche, floor((punteggio − 10) / 2).
func Modificatore(punteggio int) int {
	d := punteggio - 10
	if d < 0 {
		return -((-d + 1) / 2)
	}
	return d / 2
}

// PunteggioDaSaggezza (HOMEBREW, metodo semplificato): 10 + modificatore di Saggezza.
func PunteggioDaSaggezza(saggezza int) int { return 10 + Modificatore(saggezza) }

// ArrayStandardConSanita (UFFICIALE): un 11 in più nell'array per la caratteristica aggiunta.
func ArrayStandardConSanita() []int { return []int{15, 14, 13, 12, 10, 8, 11} }

// BudgetPointBuyConSanita (UFFICIALE): 3 punti in più per la caratteristica aggiunta.
func BudgetPointBuyConSanita(budget int) int { return budget + 3 }

// BonusCompetenza per livello del personaggio.
func BonusCompetenza(livello int) int {
	if livello < 1 {
		livello = 1
	}
	return (livello+3)/4 + 1
}

// ===== Tiro salvezza =====

type EsitoTS struct {
	D20            []int  `json:"d20"`            // uno o due d20 (vantaggio/svantaggio)
	Scelto         int    `json:"scelto"`         // il d20 tenuto
	Vantaggio      string `json:"vantaggio"`      // normale, vantaggio, svantaggio
	Caratteristica string `json:"caratteristica"` // SAN, SAG, CAR, COS
	Punteggio      int    `json:"punteggio"`
	ModCar         int    `json:"modCaratteristica"`
	Competenza     int    `json:"competenza"`   // bonus di competenza applicato (0 se non competente)
	Bonus          int    `json:"bonus"`        // bonus/malus temporaneo del personaggio
	Situazionale   int    `json:"situazionale"` // modificatore dell'evento
	Totale         int    `json:"totale"`
	CD             int    `json:"cd"`
	Successo       bool   `json:"successo"`
}

// CombinaVantaggio: vantaggio e svantaggio insieme si annullano (regola base del d20).
func CombinaVantaggio(a, b string) string {
	if a == "normale" || a == "" {
		return b
	}
	if b == "normale" || b == "" || a == b {
		return a
	}
	return "normale"
}

// TiroSalvezza risolve d20 (+ secondo d20 con vantaggio/svantaggio) + modificatori contro la CD.
// Un 20 o un 1 naturale non hanno effetti speciali: i tiri salvezza non hanno critici.
func TiroSalvezza(tira func(int) int, car string, punteggio int, competente bool, prof, bonus, situazionale, cd int, vantaggio string) EsitoTS {
	e := EsitoTS{Vantaggio: vantaggio, Caratteristica: car, Punteggio: punteggio, ModCar: Modificatore(punteggio), Bonus: bonus, Situazionale: situazionale, CD: cd}
	if e.Vantaggio == "" {
		e.Vantaggio = "normale"
	}
	a := tira(20)
	e.D20, e.Scelto = []int{a}, a
	if e.Vantaggio != "normale" {
		b := tira(20)
		e.D20 = append(e.D20, b)
		if (e.Vantaggio == "vantaggio") == (b > a) {
			e.Scelto = b
		}
	}
	if competente {
		e.Competenza = prof
	}
	e.Totale = e.Scelto + e.ModCar + e.Competenza + e.Bonus + e.Situazionale
	e.Successo = e.Totale >= cd
	return e
}

// PunteggioPerTS sceglie il punteggio da usare. Per le creature senza Sanità mentale la variante
// UFFICIALE indica di usare la Saggezza (fallback, se abilitato).
func PunteggioPerTS(car string, sanita *int, abil map[string]int, fallbackSaggezza bool) (int, string, error) {
	switch car {
	case "SAN":
		if sanita != nil {
			return *sanita, "SAN", nil
		}
		if fallbackSaggezza {
			if v, ok := abil["SAG"]; ok {
				return v, "SAG", nil
			}
		}
		return 0, "", fmt.Errorf("punteggio di Sanità mentale mancante")
	case "SAG", "CAR", "COS":
		if v, ok := abil[car]; ok {
			return v, car, nil
		}
		return 0, "", fmt.Errorf("punteggio di %s mancante", car)
	}
	return 0, "", fmt.Errorf("caratteristica sconosciuta")
}

// ===== Scala del Terrore 0–7 (HOMEBREW) =====

type LivelloTerrore struct {
	Livello    int      `json:"livello"`
	Nome       string   `json:"nome"`
	Effetto    string   `json:"effetto"`
	Condizioni []string `json:"condizioni,omitempty"` // condizioni ufficiali applicate dal motore condizioni
	Ufficiale  bool     `json:"ufficiale,omitempty"`  // l'effetto applica una condizione ufficiale
}

// TerroreMax è il valore massimo della scala.
const TerroreMax = 7

// LivelliTerrore: la tabella della specifica (configurazione predefinita).
var LivelliTerrore = []LivelloTerrore{
	{0, "Sereno", "Nessun effetto di Terrore.", nil, false},
	{1, "Turbato", "Per avvicinarsi volontariamente alla fonte della paura deve superare un TS di Saggezza contro la CD della fonte (un tiro per scena o per cambiamento significativo).", nil, false},
	{2, "Timoroso", "Non può avvicinarsi volontariamente alla fonte senza superare un TS di Saggezza contro la CD della fonte. Finché la fonte è percepibile ha svantaggio alle prove di caratteristica che richiedono calma o concentrazione.", nil, false},
	{3, "Impaurito", "Finché la fonte è visibile o chiaramente percepibile ha svantaggio ai tiri per colpire (effetto homebrew: non è la condizione Spaventato).", nil, false},
	{4, "Barcollante", "Lascia cadere un oggetto impugnato a sua scelta, se plausibile. Non può usare reazioni. All'inizio del proprio turno può fare un TS di Saggezza CD 15: con un successo il Terrore scende di 1. Arrivando a 4 riceve in automatico una follia temporanea casuale (a 6 una follia permanente).", nil, false},
	{5, "Sconvolto", "Deve usare l'azione per lo Scatto e allontanarsi dalla fonte per un percorso ragionevolmente sicuro; senza via di fuga può usarla per mettersi al riparo. Mantiene le limitazioni dei livelli precedenti.", nil, false},
	{6, "Paralizzato dal terrore", "Ha la condizione Paralizzato finché il Terrore resta a 6 o più, con tutti i suoi effetti standard.", []string{"Paralizzato"}, true},
	{7, "Svenuto", "Ha la condizione Privo di sensi (Incosciente). Alla fine di ogni suo turno ripete un TS di Costituzione CD 15: con un successo si riprende e il Terrore scende a 6. Un alleato adiacente può usare un'azione e superare una prova di Saggezza (Medicina) CD 15 per risvegliarlo (Terrore a 6). Nessun danno automatico per il fallimento.", []string{"Privo di sensi"}, true},
}

// EffettiAttivi: gli effetti sono cumulativi, quindi si elencano tutti i livelli da 1 al livello attuale.
func EffettiAttivi(livello int) []LivelloTerrore {
	livello = LimitaTerrore(livello)
	var out []LivelloTerrore
	out = append(out, LivelliTerrore[1:livello+1]...)
	return out
}

// CondizioniTerrore: condizioni ufficiali da applicare al livello dato (Paralizzato da 6, Privo di sensi a 7).
func CondizioniTerrore(livello int) []string {
	var c []string
	for _, l := range EffettiAttivi(livello) {
		c = append(c, l.Condizioni...)
	}
	return c
}

// LimitaTerrore tiene il valore nella scala 0–7.
func LimitaTerrore(v int) int { return min(TerroreMax, max(0, v)) }

// Intensità degli eventi (HOMEBREW): CD predefinita e Terrore aggiunto in caso di fallimento.
var (
	CDIntensita      = [6]int{10, 15, 20, 22, 25, 25}
	TerroreIntensita = [6]int{1, 1, 2, 3, 4, 5}
)

// TerroreGuadagnato: Terrore aggiunto da un tiro. Con la regola delle resistenze attiva, chi è immune a
// Spaventato ne riceve 1 in meno; l'immunità completa al Terrore lo annulla.
func TerroreGuadagnato(successo bool, perFallimento int, immuneSpaventato, regolaResistenza, immuneTerrore bool) int {
	if successo || immuneTerrore {
		return 0
	}
	if immuneSpaventato && regolaResistenza {
		return max(0, perFallimento-1)
	}
	return perFallimento
}

// Suggerimento di pazzia (HOMEBREW 7.1): mai automatico per duratura e indeterminata.
type SuggerimentoPazzia struct {
	Tipo     string `json:"tipo"`     // temporanea, duratura, indeterminata
	Verifica string `json:"verifica"` // cosa deve fare il Master
	Motivo   string `json:"motivo"`
	Conferma bool   `json:"conferma"` // serve la conferma esplicita del Master
}

// SogliePazzia: verifiche che il Master può richiedere dopo il tiro, con la motivazione.
func SogliePazzia(prima, dopo, intensita int, haTemporanea bool) []SuggerimentoPazzia {
	var s []SuggerimentoPazzia
	if dopo >= 4 && prima < 4 {
		s = append(s, SuggerimentoPazzia{"temporanea", "TS di Carisma contro la CD dell'evento: se fallito, pazzia temporanea", fmt.Sprintf("Il Terrore è salito a %d (soglia 4).", dopo), false})
	}
	if dopo >= 5 && prima < 5 && (haTemporanea || intensita >= 4) {
		mot := "evento particolarmente grave"
		if haTemporanea {
			mot = "il personaggio ha già una pazzia temporanea"
		}
		s = append(s, SuggerimentoPazzia{"duratura", "Pazzia duratura (da confermare)", fmt.Sprintf("Terrore %d (soglia 5) e %s.", dopo, mot), true})
	}
	if dopo >= 6 && prima < 6 {
		s = append(s, SuggerimentoPazzia{"indeterminata", "Pazzia indeterminata (da confermare)", fmt.Sprintf("Terrore %d (soglia 6–7).", dopo), true})
	}
	return s
}

// ===== Pazzia =====

// CategoriePazzia (HOMEBREW): categorie e intervalli del d20.
var CategoriePazzia = []struct {
	Nome  string
	Da, A int
}{
	{"Fobia", 1, 5}, {"Ossessione", 6, 8}, {"Comportamento erratico", 9, 11}, {"Allucinazioni", 12, 13},
	{"Paranoia", 14, 15}, {"Illusione", 16, 17}, {"Dissociazione", 18, 19}, {"Odio irrazionale", 20, 20},
}

func CategoriaDaD20(n int) string {
	for _, c := range CategoriePazzia {
		if n >= c.Da && n <= c.A {
			return c.Nome
		}
	}
	return ""
}

// VoceDaD100: 01–10 → voce 1, …, 91–00 → voce 10.
func VoceDaD100(n int) int { return (min(100, max(1, n))-1)/10 + 1 }

// DurataPazzia (UFFICIALE DMG 2014): temporanea 1d10 minuti, duratura 1d10 × 10 ore, indeterminata fino a cura.
func DurataPazzia(tipo string, tira func(int) int) (valore int, unita string) {
	switch tipo {
	case "temporanea":
		return tira(10), "minuti"
	case "duratura":
		return tira(10) * 10, "ore"
	}
	return 0, ""
}

// RecuperoPazzia (HOMEBREW 7.2): ogni 10 giorni di gioco TS di Saggezza contro la CD di recupero;
// con un successo la CD scende di 1d4; a 0 la pazzia termina.
func RecuperoPazzia(cd int, successo bool, d4 int) (nuovaCD int, guarita bool) {
	if !successo {
		return cd, false
	}
	nuovaCD = max(0, cd-d4)
	return nuovaCD, nuovaCD == 0
}
