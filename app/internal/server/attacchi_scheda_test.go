package server

import (
	"fmt"
	"strings"
	"testing"
)

// Verifica richiesta dall'utente (2026-11-12): gli attacchi della scheda del personaggio contro la CA del bersaglio, con il
// tipo di danno dell'arma (tagliente, contundente…) e i danni elementali/magici separati, ognuno con resistenze,
// immunità e vulnerabilità del bersaglio; armi magiche e argentate contro le difese «da attacchi non magici (non
// argentati)».
func TestAttacchiDallaScheda(t *testing.T) {
	sc := nuovoScontro(t)
	// Arin: Spada lunga +1 di fuoco (magica: 1d8+4 tagliente + 1d6 fuoco), Mazza d'argento (non magica, argentata),
	// Ascia (comune). Così li manda la scheda nel riepilogo.
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"lvl": 3},
		"riepilogo": map[string]any{"class": "Guerriero", "level": 3, "ac": 16, "hpMax": 30, "initiative": 2, "dex": 14,
			"attacks": []any{
				map[string]any{"nome": "Spada lunga di fuoco +1", "bonus": 6, "portata": "1,5 m", "magica": true,
					"danni": []any{map[string]any{"dadi": "1d8", "mod": 4, "tipo": "tagliente"}, map[string]any{"dadi": "1d6", "mod": 0, "tipo": "fuoco"}}},
				map[string]any{"nome": "Mazza d'argento", "bonus": 5, "portata": "1,5 m", "argentata": true,
					"danni": []any{map[string]any{"dadi": "1d6", "mod": 3, "tipo": "contundente"}}},
				map[string]any{"nome": "Ascia", "bonus": 5, "portata": "1,5 m",
					"danni": []any{map[string]any{"dadi": "1d6", "mod": 3, "tipo": "tagliente"}}}},
			"attacksPerAction": 1}})
	// Lupo mannaro (immune ai danni fisici da attacchi non magici non argentati), scheletro (vulnerabile ai contundenti,
	// immune al veleno), golem di carne (resistente al fuoco? qui: resistenza al fuoco, CA 9).
	fisici := func() []any {
		l := []any{}
		for _, x := range []string{"contundente", "perforante", "tagliente"} {
			l = append(l, map[string]any{"tipo": x, "nonMagici": true, "nonArgentate": true})
		}
		return l
	}
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Lupo mannaro", "quanti": 1, "ca": 12, "pf": 58,
		"profilo": map[string]any{"difese": map[string]any{"imm": fisici()}}})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Scheletro", "quanti": 1, "ca": 13, "pf": 30,
		"profilo": map[string]any{"difese": map[string]any{"vuln": []any{map[string]any{"tipo": "contundente"}}, "imm": []any{map[string]any{"tipo": "veleno"}}}}})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Golem", "quanti": 1, "ca": 9, "pf": 90,
		"profilo": map[string]any{"difese": map[string]any{"res": []any{map[string]any{"tipo": "fuoco"}}}}})
	vm := sc.vista(sc.m)
	idA := cerca(vm, "Arin")["id"].(string)
	idL, idS, idG := cerca(vm, "Lupo mannaro")["id"].(string), cerca(vm, "Scheletro")["id"].(string), cerca(vm, "Golem")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 30})
	for _, id := range []string{idL, idS, idG} {
		sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": id, "valore": 1})
	}
	sc.az(sc.m, 200, "iniz.avvia", nil)
	n := 0
	attacca := func(bersaglio string, attacco int, dadi ...int) map[string]any {
		t.Helper()
		n++
		tiriDecisi(t, dadi...)
		return sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": bersaglio, "attacco": attacco, "forza": true, "azione": fmt.Sprintf("scheda-%03d", n)})
	}
	danni := func(e map[string]any) string {
		var p []string
		for _, x := range e["danni"].([]any) {
			d := x.(map[string]any)
			s := fmt.Sprintf("%v %v=%v", d["tipo"], d["tirato"], d["finale"])
			if d["nota"] != nil {
				s += " (" + d["nota"].(string) + ")"
			}
			p = append(p, s)
		}
		return strings.Join(p, "; ")
	}
	pf := func(nome string) float64 { return cerca(sc.vista(sc.m), nome)["pf"].(float64) }

	// CA: 5 + 5 = 10 contro CA 12 del lupo mannaro: mancato, nessun danno.
	if e := attacca(idL, 2, 5); e["risultato"] != "mancato" || e["totaleDanni"].(float64) != 0 || pf("Lupo mannaro") != 58 {
		t.Fatalf("attacco sotto la CA: %v", e)
	}
	// 7 + 5 = 12 contro CA 12: colpito (pari alla CA colpisce). Ascia comune: il lupo mannaro è immune.
	e := attacca(idL, 2, 7, 4)
	if e["risultato"] != "colpito" || e["totaleDanni"].(float64) != 0 || !strings.Contains(danni(e), "tagliente 7=0 (immunità)") || pf("Lupo mannaro") != 58 {
		t.Fatalf("arma comune contro il lupo mannaro: %v → %s", e, danni(e))
	}
	// Mazza d'argento (non magica): l'immunità «non argentati» non vale, 4 + 3 = 7 contundenti.
	e = attacca(idL, 1, 15, 4)
	if e["totaleDanni"].(float64) != 7 || danni(e) != "contundente 7=7" || pf("Lupo mannaro") != 51 {
		t.Fatalf("arma argentata contro il lupo mannaro: %v → %s", e, danni(e))
	}
	// Spada magica: tagliente pieno (5 + 4 = 9) più fuoco a parte (3) = 12.
	e = attacca(idL, 0, 15, 5, 3)
	if e["totaleDanni"].(float64) != 12 || danni(e) != "tagliente 9=9; fuoco 3=3" || pf("Lupo mannaro") != 39 {
		t.Fatalf("arma magica contro il lupo mannaro: %v → %s", e, danni(e))
	}
	// Scheletro vulnerabile ai contundenti: la mazza fa il doppio (4 + 3 = 7 → 14); l'ascia tagliente normale (7).
	e = attacca(idS, 1, 15, 4)
	if e["totaleDanni"].(float64) != 14 || !strings.Contains(danni(e), "contundente 7=14 (vulnerabilità, doppio)") || pf("Scheletro") != 16 {
		t.Fatalf("vulnerabilità: %v → %s", e, danni(e))
	}
	e = attacca(idS, 2, 15, 4)
	if e["totaleDanni"].(float64) != 7 || pf("Scheletro") != 9 {
		t.Fatalf("scheletro e ascia: %v → %s", e, danni(e))
	}
	// Golem resistente al fuoco: il tagliente della spada pieno (6 + 4 = 10), il fuoco a metà per difetto (5 → 2) = 12.
	e = attacca(idG, 0, 15, 6, 5)
	if e["totaleDanni"].(float64) != 12 || danni(e) != "tagliente 10=10; fuoco 5=2 (resistenza, metà)" || pf("Golem") != 78 {
		t.Fatalf("resistenza al solo tipo elementale: %v → %s", e, danni(e))
	}
	// Difese dei PG dalla scheda (Resilienza immonda): «(non magici e non argentati)».
	if d := leggiVoceDifesa("tagliente (non magici e non argentati)"); d != (difesa{Tipo: "tagliente", NonMagici: true, NonArgentate: true}) {
		t.Fatalf("voce di difesa: %+v", d)
	}
	if v, _ := applicaDifeseDa("tagliente", 8, colpoArma{argentata: true}, difeseCombattente{Res: []difesa{{Tipo: "tagliente", NonMagici: true}}}); v != 4 {
		t.Fatalf("«non magici» (senza argento) vale anche contro l'argento: %d", v)
	}
	// CA alta: 1 naturale manca sempre anche con un bonus alto; il registro riporta tutto.
	if e := attacca(idG, 0, 1); e["risultato"] != "mancato" {
		t.Fatalf("1 naturale: %v", e)
	}
	reg := []string{}
	for _, x := range sc.vista(sc.m)["registro"].([]any) {
		reg = append(reg, x.(string))
	}
	if r := strings.Join(reg, "\n"); !strings.Contains(r, "Lupo mannaro subisce 0 danni") || !strings.Contains(r, "Golem subisce 12 danni") {
		t.Fatalf("registro dello scontro: %s", r)
	}
}
