package server

import (
	"strings"
	"testing"
)

// Fase 2 (2026-11-12): Scudo di fuoco (danni a chi colpisce in mischia), Eroismo (PF temporanei a inizio turno),
// Anatema (TS su Carisma: chi riesce non è colpito).
func TestAnatemaEroismoScudoDiFuoco(t *testing.T) {
	sc := nuovoScontro(t) // Arin: CA 16, PF 20
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Orco", "quanti": 1, "ca": 13, "pf": 60,
		"profilo": map[string]any{"ts": map[string]any{"CAR": -1}, "attacchi": []any{
			map[string]any{"nome": "Ascia", "bonus": 5, "portata": "1,5 m", "danni": []any{map[string]any{"dadi": "1d12", "mod": 3, "tipo": "tagliente"}}},
			map[string]any{"nome": "Giavellotto", "bonus": 5, "portata": "gittata 9/36 m", "danni": []any{map[string]any{"dadi": "1d6", "mod": 3, "tipo": "perforante"}}}}}})
	vm := sc.vista(sc.m)
	idA, idO := cerca(vm, "Arin")["id"].(string), cerca(vm, "Orco")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 30})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idO, "valore": 1})
	sc.az(sc.m, 200, "iniz.avvia", nil)
	effetto := func(chi *cliente, comb, bersaglio string, v map[string]any, azione string) map[string]any {
		t.Helper()
		return sc.az(chi, 200, "effetto.nuovo", map[string]any{"combattente": comb, "bersagli": []string{bersaglio}, "valori": v, "azione": azione})
	}

	// Scudo di fuoco caldo su Arin: resistenza al freddo, 2d8 da fuoco a chi lo colpisce in mischia.
	effetto(sc.luca, idA, idA, map[string]any{"nome": "Scudo di fuoco", "tempo": "altro", "round": 100,
		"mod": map[string]any{"res": []string{"freddo"}, "ritorsione": map[string]any{"dadi": "2d8", "tipo": "fuoco"}}}, "fs2-0001")
	// L'orco colpisce Arin con l'ascia: 15, 4 + 3 = 7 taglienti; Arin a 13. Lo scudo: 3 + 4 = 7 fuoco all'orco (60 → 53).
	tiriDecisi(t, 15, 4, 3, 4)
	e := sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idO, "bersaglio": idA, "attacco": 0, "forza": true, "azione": "fs2-0002"})
	rit, _ := e["ritorsione"].([]any)
	if e["risultato"] != "colpito" || len(rit) != 1 || !strings.Contains(rit[0].(string), "Orco subisce 7 danni") || !strings.Contains(rit[0].(string), "Scudo di fuoco di Arin") {
		t.Fatalf("Scudo di fuoco in mischia: %v", e)
	}
	if pf := cerca(sc.vista(sc.m), "Orco")["pf"].(float64); pf != 53 {
		t.Fatalf("PF dell'orco dopo lo scudo: %v", pf)
	}
	// Il giavellotto (a distanza) non fa scattare lo scudo.
	tiriDecisi(t, 15, 2)
	e = sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idO, "bersaglio": idA, "attacco": 1, "forza": true, "azione": "fs2-0003"})
	if e["ritorsione"] != nil || cerca(sc.vista(sc.m), "Orco")["pf"].(float64) != 53 {
		t.Fatalf("a distanza lo scudo non scatta: %v", e)
	}

	// Eroismo su Arin (4 PF temporanei a inizio turno): al suo turno successivo li riceve.
	effetto(sc.luca, idA, idA, map[string]any{"nome": "Eroismo", "tempo": "altro", "conc": true, "round": 10,
		"mod": map[string]any{"immCond": []string{"Spaventato"}, "pfTempTurno": 4}}, "fs2-0004")
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil) // orco
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil) // Arin, round 2
	if a := cerca(sc.vista(sc.m), "Arin"); a["pfTemp"] == nil || a["pfTemp"].(float64) != 4 {
		t.Fatalf("PF temporanei di Eroismo: %v", a)
	}
	var reg []string
	for _, x := range sc.vista(sc.m)["registro"].([]any) {
		reg = append(reg, x.(string))
	}
	if !strings.Contains(strings.Join(reg, "\n"), "Arin riceve 4 PF temporanei (Eroismo)") {
		t.Fatalf("registro: %v", reg)
	}

	// Anatema sull'orco (TS su Carisma CD 13, −1): 18 − 1 = 17 riesce, nessuno è colpito.
	tiriDecisi(t, 18)
	r := effetto(sc.luca, idA, idO, map[string]any{"nome": "Anatema", "tempo": "altro", "conc": true, "round": 10, "tsCar": "CAR", "tsCd": 13,
		"mod": map[string]any{"dadoAtt": "-1d4", "dadoTS": "-1d4"}}, "fs2-0005")
	if !strings.Contains(r["testo"].(string), "nessuno è colpito") {
		t.Fatalf("Anatema riuscito: %v", r)
	}
	// 5 − 1 = 4 fallisce: l'effetto è sull'orco.
	tiriDecisi(t, 5)
	r = effetto(sc.luca, idA, idO, map[string]any{"nome": "Anatema", "tempo": "altro", "conc": true, "round": 10, "tsCar": "CAR", "tsCd": 13,
		"mod": map[string]any{"dadoAtt": "-1d4", "dadoTS": "-1d4"}}, "fs2-0006")
	if !strings.Contains(r["testo"].(string), "Orco fallisce") {
		t.Fatalf("Anatema fallito: %v", r)
	}
	trovato := false
	for _, x := range sc.vista(sc.m)["effetti"].([]any) {
		if x.(map[string]any)["nome"] == "Anatema" {
			trovato = true
		}
	}
	if !trovato {
		t.Fatal("Anatema in corso sull'orco")
	}
}
