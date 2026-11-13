package server

import (
	"strings"
	"testing"
)

// Dadi D2 (2026-11-12): il numero dei dadi dei danni scelto nella conferma dell'attacco.
func TestDadiDellaConferma(t *testing.T) {
	l := []componenteDanno{{Dadi: "1d8", Mod: 3, Tipo: "tagliente"}, {Dadi: "2d6", Tipo: "fuoco"}, {Mod: 2, Tipo: "forza"}}
	if x, nota := conNumeroDadi(l, []int{2, 0, 5}); nota != "1d8 → 2d8" || x[0].Dadi != "2d8" || x[1].Dadi != "2d6" || x[2].Dadi != "" || l[0].Dadi != "1d8" {
		t.Fatalf("un dado in più: %v %q (originale %v)", x, nota, l)
	}
	if x, _ := conNumeroDadi(l, []int{20, 20}); x[0].Dadi != "20d8" || x[1].Dadi != "20d6" { // 40 dadi: il massimo per un attacco
		t.Fatalf("al limite: %v", x)
	}
	for _, n := range [][]int{nil, {1, 2}, {21}, {0, 0, 0, 0, 0, 0, 0, 0, 0}} { // niente da cambiare, troppi dadi, elenco troppo lungo
		if x, nota := conNumeroDadi(l, n); nota != "" || x[0].Dadi != "1d8" || x[1].Dadi != "2d6" {
			t.Fatalf("%v: %v %q", n, x, nota)
		}
	}

	sc := nuovoScontro(t)
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"lvl": 3},
		"riepilogo": map[string]any{"class": "Guerriero", "level": 3, "ac": 16, "hpMax": 30, "initiative": 2, "dex": 14,
			"attacks": []any{map[string]any{"nome": "Spada lunga", "bonus": 5, "portata": "1,5 m", "danni": []any{map[string]any{"dadi": "1d8", "mod": 3, "tipo": "tagliente"}}}}}})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Ogre", "quanti": 1, "ca": 11, "pf": 59})
	vm := sc.vista(sc.m)
	idA, idO := cerca(vm, "Arin")["id"].(string), cerca(vm, "Ogre")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 20})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idO, "valore": 5})
	sc.az(sc.m, 200, "iniz.avvia", nil)
	// Arin usa la spada a due mani: 2d8 invece di 1d8. 15 + 5 colpisce; 4 + 5 + 3 = 12.
	tiriDecisi(t, 15, 4, 5)
	e := sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idO, "attacco": 0, "dadiDanni": []int{2}, "azione": "d2c-0001"})
	if e["totaleDanni"].(float64) != 12 || e["dadiCambiati"] != "1d8 → 2d8" {
		t.Fatalf("attacco con un dado in più: %v", e)
	}
	// l'espressione porta anche il modificatore (come negli altri attacchi): «2d8+3»
	if d := e["danni"].([]any)[0].(map[string]any); d["espr"] != "2d8+3" || len(d["dadi"].([]any)) != 2 {
		t.Fatalf("componente dei danni: %v", d)
	}
	// il registro arriva dal più recente: si cerca in tutte le righe
	var righe []string
	for _, x := range sc.vista(sc.m)["registro"].([]any) {
		righe = append(righe, x.(string))
	}
	if reg := strings.Join(righe, "\n"); !strings.Contains(reg, "dadi dei danni scelti: 1d8 → 2d8") {
		t.Fatalf("il cambio resta nel registro:\n%s", reg)
	}
}
