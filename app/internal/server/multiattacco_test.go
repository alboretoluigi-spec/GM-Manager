package server

import (
	"strings"
	"testing"
)

// Fase 3 (2026-11-12): Multiattacco con attacchi diversi. Un orso gufo: un morso e due artigli per turno; il server
// tiene il conto per tipo, chiede conferma se si sfora, riparte al turno dopo; la sequenza la vede solo il Master.
func TestMultiattaccoConAttacchiDiversi(t *testing.T) {
	sc := nuovoScontro(t) // Arin: CA 16, PF 20
	danni := func(d, tipo string) []any { return []any{map[string]any{"dadi": d, "mod": 0, "tipo": tipo}} }
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Orso gufo", "quanti": 1, "ca": 13, "pf": 59,
		"profilo": map[string]any{"multi": 3, "attacchi": []any{
			map[string]any{"nome": "Becco", "bonus": 7, "danni": danni("1d10", "perforante")},
			map[string]any{"nome": "Artigli", "bonus": 7, "danni": danni("2d8", "tagliente")}},
			"sequenza": []any{map[string]any{"attacco": 0, "volte": 1}, map[string]any{"attacco": 1, "volte": 2}}}})
	// una sequenza che non torna con gli attacchi per azione si scarta (resta il solo conteggio)
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Orso storto", "quanti": 1, "ca": 13, "pf": 59,
		"profilo": map[string]any{"multi": 3, "attacchi": []any{map[string]any{"nome": "Becco", "bonus": 7, "danni": danni("1d10", "perforante")}},
			"sequenza": []any{map[string]any{"attacco": 0, "volte": 2}}}})
	vm := sc.vista(sc.m)
	orso := cerca(vm, "Orso gufo")
	if seq, _ := orso["multiattacco"].([]any); len(seq) != 2 {
		t.Fatalf("sequenza per il Master: %v", orso["multiattacco"])
	}
	if cerca(vm, "Orso storto")["multiattacco"] != nil {
		t.Fatal("sequenza sbagliata scartata")
	}
	if cerca(sc.vista(sc.luca), "Orso gufo")["multiattacco"] != nil {
		t.Fatal("i giocatori non vedono il Multiattacco dei mostri")
	}
	idO, idA := orso["id"].(string), cerca(vm, "Arin")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idO, "valore": 30})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 10})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": cerca(vm, "Orso storto")["id"].(string), "valore": 1})
	sc.az(sc.m, 200, "iniz.avvia", nil)
	n := 0
	attacca := func(i int, forza bool) map[string]any {
		t.Helper()
		n++
		tiriDecisi(t, 2, 1, 1) // manca sempre (2 + 7 = 9 contro CA 16): conta solo la sequenza
		return sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idO, "bersaglio": idA, "attacco": i, "forza": forza, "azione": "multi-00" + itoa(n)})
	}
	if e := attacca(0, false); e["serveConferma"] != nil || e["attacchiRimasti"].(float64) != 2 {
		t.Fatalf("becco: %v", e)
	}
	// un secondo becco sfora la sequenza: conferma con il testo della sequenza
	e := attacca(0, false)
	if c, _ := e["serveConferma"].(string); !strings.Contains(c, "1 Becco e 2 Artigli") || !strings.Contains(c, "«Becco» l'ha già usato 1 volta") {
		t.Fatalf("secondo becco: %v", e)
	}
	if e := attacca(1, false); e["serveConferma"] != nil {
		t.Fatalf("primi artigli: %v", e)
	}
	if e := attacca(1, false); e["serveConferma"] != nil || e["attacchiRimasti"].(float64) != 0 {
		t.Fatalf("secondi artigli: %v", e)
	}
	if eco := cerca(sc.vista(sc.m), "Orso gufo")["economia"].(map[string]any); len(eco["seq"].([]any)) != 2 || eco["seq"].([]any)[1].(float64) != 2 {
		t.Fatalf("conto per tipo nella vista: %v", eco)
	}
	// il turno dopo il conto riparte
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil) // di nuovo l'orso, round 2
	if eco := cerca(sc.vista(sc.m), "Orso gufo")["economia"].(map[string]any); eco["seq"] != nil {
		t.Fatalf("nuovo turno: il conto riparte: %v", eco)
	}
	if e := attacca(1, false); e["serveConferma"] != nil {
		t.Fatalf("artigli al turno dopo: %v", e)
	}
	// il Master può sempre forzare
	attacca(1, false)
	if e := attacca(1, true); e["serveConferma"] != nil || e["risultato"] == nil {
		t.Fatalf("forzato: %v", e)
	}
}
