package server

import "testing"

func TestRegoleTSSanita(t *testing.T) {
	f := func(totale, cd int) EsitoTS {
		return EsitoTS{Totale: totale, CD: cd, Successo: totale >= cd, Scelto: 10}
	}
	if e, n, _ := RegoleTSSanita(f(11, 15), 0); e != 0 || n != 1 {
		t.Fatalf("fallito di 4: niente Terrore in più, 1 fallimento: %d %d", e, n)
	}
	if e, n, m := RegoleTSSanita(f(10, 15), 0); e != 1 || n != 1 || len(m) != 1 {
		t.Fatalf("fallito di 5: +1: %d %d %v", e, n, m)
	}
	if e, n, _ := RegoleTSSanita(f(14, 15), 2); e != 2 || n != 0 {
		t.Fatalf("terzo fallimento di seguito: +2 e conteggio da capo: %d %d", e, n)
	}
	if e, n, m := RegoleTSSanita(f(3, 15), 2); e != 3 || n != 0 || len(m) != 2 {
		t.Fatalf("terzo fallimento e di 5 o più: +3: %d %d %v", e, n, m)
	}
	if e, n, _ := RegoleTSSanita(f(15, 15), 2); e != 0 || n != 0 {
		t.Fatalf("un successo azzera il conteggio: %d %d", e, n)
	}
	if !UnoNaturale(EsitoTS{Scelto: 1, Successo: false}) || UnoNaturale(EsitoTS{Scelto: 1, Successo: true}) {
		t.Fatal("1 naturale solo se fallito")
	}
}

// Il TS su Sanità mentale con CD (chiesto dal Master o dal giocatore) applica le regole; le soglie del Terrore
// assegnano le follie anche quando è il Master a cambiare il Terrore.
func TestSanitaRegoleAutomatiche(t *testing.T) {
	tv, pg, _, _, _ := preparaSanita(t)
	attivaTerrore(tv)
	conta := func(tipo string) int {
		var n int
		tv.a.s.DB.QueryRow(`SELECT COUNT(*) FROM pazzie WHERE personaggio_id = ? AND tipo = ? AND attiva = 1`, pg, tipo).Scan(&n)
		return n
	}
	terrore := func() float64 { return tv.m.deve(200, "GET", "/sanita/pg/"+pg, nil)["terrore"].(float64) }
	tira := func(cd int, dadi ...int) map[string]any {
		vero := tiraDado
		tiraDado = sequenza(dadi...)
		defer func() { tiraDado = vero }()
		return tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/tiro", map[string]any{"tipo": "sanita", "cd": cd})
	}
	// Sanità 14 (+2). Senza CD nessuna regola.
	tira(0, 1)
	if terrore() != 0 || conta("temporanea") != 0 {
		t.Fatal("senza CD non si applicano regole")
	}
	// 1º fallimento di 1 (CD 13, d20 10 → 12): nulla. 2º fallimento di 1: nulla. 3º di 1: +2.
	tira(13, 10)
	tira(13, 10)
	if terrore() != 0 {
		t.Fatal("due fallimenti di poco: Terrore invariato")
	}
	if out := tira(13, 10); terrore() != 2 || out["automatiche"] == nil {
		t.Fatalf("terzo fallimento di seguito: Terrore +2: %v", out)
	}
	// Successo: il conteggio riparte (due fallimenti dopo non bastano).
	tira(13, 15)
	tira(13, 10)
	tira(13, 10)
	if terrore() != 2 {
		t.Fatal("il successo azzera i fallimenti consecutivi")
	}
	// Fallito di 5 o più (CD 20, d20 5 → 7, e terzo fallimento): +3 → 5, soglia 4 → follia temporanea (d20, d100, d10).
	out := tira(20, 5, 3, 50, 6)
	if terrore() != 5 || conta("temporanea") != 1 || out["pazzieAssegnate"] == nil {
		t.Fatalf("fallito di 13 e terzo fallimento: Terrore 5 e follia temporanea: %v", out)
	}
	// 1 sul d20 (CD 10): follia temporanea; fallito di 7 → +1 → 6: follia permanente (indeterminata).
	out = tira(10, 1, 4, 20, 3, 6, 70)
	if terrore() != 6 || conta("temporanea") != 2 || conta("indeterminata") != 1 {
		t.Fatalf("1 naturale e soglia 6: %v", out)
	}
	// Il Master riporta il Terrore a 3 e poi a 4: la soglia 4 assegna di nuovo una follia temporanea.
	tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "manuale", "valore": 3, "motivo": "riposo"})
	vero := tiraDado
	tiraDado = sequenza(9, 30, 2)
	tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "manuale", "valore": 4, "motivo": "visione"})
	tiraDado = vero
	if conta("temporanea") != 3 {
		t.Fatal("soglia 4 raggiunta dal Master: follia temporanea")
	}
}
