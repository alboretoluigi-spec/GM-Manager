package server

import (
	"reflect"
	"testing"
)

// dadi truccati: restituiscono i valori in ordine
func sequenza(v ...int) func(int) int {
	return func(int) int { x := v[0]; v = v[1:]; return x }
}

func TestModificatoreTutti(t *testing.T) {
	// valori limite 1–30 e modificatori negativi
	attesi := map[int]int{1: -5, 2: -4, 3: -4, 8: -1, 9: -1, 10: 0, 11: 0, 12: 1, 13: 1, 16: 3, 20: 5, 29: 9, 30: 10}
	for p, m := range attesi {
		if got := Modificatore(p); got != m {
			t.Errorf("Modificatore(%d) = %d, atteso %d", p, got, m)
		}
	}
}

func TestSanitaMetodi(t *testing.T) {
	// SAN-01: array standard con un 11 in più
	if a := ArrayStandardConSanita(); len(a) != 7 || a[6] != 11 {
		t.Fatalf("SAN-01: %v", a)
	}
	// SAN-02: point buy con 3 punti in più
	if BudgetPointBuyConSanita(27) != 30 {
		t.Fatal("SAN-02")
	}
	// SAN-03: Saggezza 16 → Sanità 13 → +1
	if p := PunteggioDaSaggezza(16); p != 13 || Modificatore(p) != 1 {
		t.Fatalf("SAN-03: %d", p)
	}
	if PunteggioDaSaggezza(7) != 8 {
		t.Fatal("Saggezza 7 (−2) → 8")
	}
}

func TestTiroSalvezzaSanita(t *testing.T) {
	// SAN-04: senza competenza: d20 + mod + situazionale
	e := TiroSalvezza(sequenza(12), "SAN", 14, false, 3, 0, 1, 16, "normale")
	if e.Totale != 12+2+1 || e.Competenza != 0 || e.Successo {
		t.Fatalf("SAN-04: %+v", e)
	}
	// SAN-05: con competenza, una sola volta
	// pari alla CD = superato
	if e = TiroSalvezza(sequenza(13), "SAN", 14, false, 3, 0, 1, 16, "normale"); !e.Successo {
		t.Fatal("totale pari alla CD: superato")
	}
	e = TiroSalvezza(sequenza(12), "SAN", 14, true, 3, 0, 0, 15, "normale")
	if e.Totale != 12+2+3 || e.Competenza != 3 || !e.Successo {
		t.Fatalf("SAN-05: %+v", e)
	}
	// TER-05: vantaggio e svantaggio: due d20, scelta corretta
	if e = TiroSalvezza(sequenza(4, 17), "SAN", 10, false, 2, 0, 0, 10, "vantaggio"); e.Scelto != 17 || !reflect.DeepEqual(e.D20, []int{4, 17}) {
		t.Fatalf("vantaggio: %+v", e)
	}
	if e = TiroSalvezza(sequenza(4, 17), "SAN", 10, false, 2, 0, 0, 10, "svantaggio"); e.Scelto != 4 || e.Successo {
		t.Fatalf("svantaggio: %+v", e)
	}
	// un 20 naturale non è un successo automatico (niente critici nei TS)
	if e = TiroSalvezza(sequenza(20), "SAN", 1, false, 2, 0, 0, 30, "normale"); e.Successo {
		t.Fatal("20 naturale non è successo automatico")
	}
	if e = TiroSalvezza(sequenza(1), "SAN", 30, false, 2, 0, 0, 5, "normale"); !e.Successo {
		t.Fatal("1 naturale non è fallimento automatico")
	}
	// bonus temporaneo e modificatore negativo
	if e = TiroSalvezza(sequenza(10), "SAN", 6, false, 2, -1, 0, 10, "normale"); e.Totale != 10-2-1 {
		t.Fatalf("bonus negativo: %+v", e)
	}
	if CombinaVantaggio("vantaggio", "svantaggio") != "normale" || CombinaVantaggio("normale", "vantaggio") != "vantaggio" {
		t.Fatal("vantaggio e svantaggio si annullano")
	}
}

func TestFallbackSaggezza(t *testing.T) {
	// SAN-06: creatura senza Sanità mentale → Saggezza, se abilitato
	abil := map[string]int{"SAG": 14}
	if v, c, err := PunteggioPerTS("SAN", nil, abil, true); err != nil || v != 14 || c != "SAG" {
		t.Fatal("SAN-06")
	}
	if _, _, err := PunteggioPerTS("SAN", nil, abil, false); err == nil {
		t.Fatal("senza fallback serve il punteggio")
	}
	s := 12
	if v, _, _ := PunteggioPerTS("SAN", &s, abil, true); v != 12 {
		t.Fatal("con il punteggio si usa quello")
	}
}

func TestSoglieTerrore(t *testing.T) {
	nomi := []string{"Sereno", "Turbato", "Timoroso", "Impaurito", "Barcollante", "Sconvolto", "Paralizzato dal terrore", "Svenuto"}
	for l := 0; l <= 7; l++ {
		if LivelliTerrore[l].Nome != nomi[l] || len(EffettiAttivi(l)) != l {
			t.Errorf("livello %d: %s, %d effetti (cumulativi)", l, LivelliTerrore[l].Nome, len(EffettiAttivi(l)))
		}
	}
	cond := map[int][]string{0: nil, 5: nil, 6: {"Paralizzato"}, 7: {"Paralizzato", "Privo di sensi"}}
	for l, c := range cond {
		if !reflect.DeepEqual(CondizioniTerrore(l), c) {
			t.Errorf("condizioni al livello %d: %v", l, CondizioniTerrore(l))
		}
	}
	// TER-01: intensità 2 → +2
	if g := TerroreGuadagnato(false, TerroreIntensita[2], false, false, false); g != 2 {
		t.Fatal("TER-01")
	}
	// TER-02: da 6 oltre 7 → 7 e Incosciente
	if v := LimitaTerrore(6 + TerroreIntensita[4]); v != 7 || CondizioniTerrore(v)[1] != "Privo di sensi" {
		t.Fatal("TER-02")
	}
	if LimitaTerrore(-3) != 0 {
		t.Fatal("mai sotto 0")
	}
	// TER-03: successo → niente Terrore
	if TerroreGuadagnato(true, 3, false, false, false) != 0 {
		t.Fatal("TER-03")
	}
	// resistenze (regola opzionale) e immunità completa
	if TerroreGuadagnato(false, 1, true, true, false) != 0 || TerroreGuadagnato(false, 3, true, true, false) != 2 {
		t.Fatal("immune a Spaventato con la regola: −1 fino a 0")
	}
	if TerroreGuadagnato(false, 3, true, false, false) != 3 {
		t.Fatal("regola disattivata: l'immunità a Spaventato non cambia il Terrore")
	}
	if TerroreGuadagnato(false, 3, false, false, true) != 0 {
		t.Fatal("immunità completa al Terrore")
	}
	if CDIntensita != [6]int{10, 15, 20, 22, 25, 25} || TerroreIntensita != [6]int{1, 1, 2, 3, 4, 5} {
		t.Fatal("tabella delle intensità")
	}
}

func TestSogliePazzia(t *testing.T) {
	// MAD-01: Terrore a 4 → richiesta di verifica (TS Carisma), non assegnazione
	s := SogliePazzia(3, 4, 1, false)
	if len(s) != 1 || s[0].Tipo != "temporanea" || s[0].Conferma {
		t.Fatalf("MAD-01: %+v", s)
	}
	if len(SogliePazzia(4, 4, 1, false)) != 0 {
		t.Fatal("restando a 4 non si ripete la verifica")
	}
	s = SogliePazzia(4, 5, 1, true)
	if len(s) != 1 || s[0].Tipo != "duratura" || !s[0].Conferma {
		t.Fatalf("duratura con pazzia temporanea già presente: %+v", s)
	}
	if len(SogliePazzia(4, 5, 1, false)) != 0 {
		t.Fatal("a 5 senza pazzia temporanea e con evento non grave: niente")
	}
	s = SogliePazzia(2, 7, 4, false)
	if len(s) != 3 || s[2].Tipo != "indeterminata" || !s[2].Conferma {
		t.Fatalf("da 2 a 7: %+v", s)
	}
	// categorie d20
	for n, c := range map[int]string{1: "Fobia", 5: "Fobia", 6: "Ossessione", 11: "Comportamento erratico", 13: "Allucinazioni", 15: "Paranoia", 17: "Illusione", 19: "Dissociazione", 20: "Odio irrazionale"} {
		if CategoriaDaD20(n) != c {
			t.Errorf("d20 %d → %s", n, CategoriaDaD20(n))
		}
	}
	for n, v := range map[int]int{1: 1, 10: 1, 11: 2, 55: 6, 91: 10, 100: 10} {
		if VoceDaD100(n) != v {
			t.Errorf("d100 %d → voce %d", n, VoceDaD100(n))
		}
	}
	// MAD-02: temporanea 1d10 minuti; duratura 1d10 × 10 ore
	if v, u := DurataPazzia("temporanea", sequenza(7)); v != 7 || u != "minuti" {
		t.Fatal("MAD-02")
	}
	if v, u := DurataPazzia("duratura", sequenza(3)); v != 30 || u != "ore" {
		t.Fatal("duratura")
	}
	if v, _ := DurataPazzia("indeterminata", sequenza()); v != 0 {
		t.Fatal("indeterminata: fino a cura")
	}
	if cd, g := RecuperoPazzia(5, true, 3); cd != 2 || g {
		t.Fatal("recupero: CD −1d4")
	}
	if cd, g := RecuperoPazzia(2, true, 4); cd != 0 || !g {
		t.Fatal("recupero: a 0 guarita")
	}
	if cd, _ := RecuperoPazzia(5, false, 4); cd != 5 {
		t.Fatal("recupero fallito: CD invariata")
	}
	if BonusCompetenza(1) != 2 || BonusCompetenza(5) != 3 || BonusCompetenza(17) != 6 {
		t.Fatal("bonus di competenza")
	}
}
