package server

import (
	"testing"
)

// Effetti fuori dallo scontro (decisione n. 2 della Parte 2): alla fine dello scontro gli effetti lunghi passano ai PG,
// valgono nella scheda e nello scontro dopo, finiscono con la concentrazione persa, il riposo lungo, il nuovo giorno o il
// Master; quelli brevi e quelli sui mostri finiscono con lo scontro.
func TestEffettiFuoriDalloScontro(t *testing.T) {
	sc := nuovoScontro(t) // Arin: CA 16, Destrezza 14
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Orco", "quanti": 1, "ca": 13, "pf": 60})
	vm := sc.vista(sc.m)
	idA, idO := cerca(vm, "Arin")["id"].(string), cerca(vm, "Orco")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 30})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idO, "valore": 1})
	sc.az(sc.m, 200, "iniz.avvia", nil)
	effetto := func(chi *cliente, comb, bersaglio string, v map[string]any, azione string) {
		t.Helper()
		sc.az(chi, 200, "effetto.nuovo", map[string]any{"combattente": comb, "bersagli": []string{bersaglio}, "valori": v, "azione": azione})
	}
	effetto(sc.luca, idA, idA, map[string]any{"nome": "Armatura magica", "tempo": "altro", "round": 4800, "mod": map[string]any{"caMin": 18}}, "fds-0001")
	effetto(sc.luca, idA, idA, map[string]any{"nome": "Pelle coriacea", "tempo": "altro", "conc": true, "round": 600, "mod": map[string]any{"caMin": 17}}, "fds-0002")
	effetto(sc.luca, idA, idA, map[string]any{"nome": "Benedizione", "tempo": "altro", "round": 10, "mod": map[string]any{"dadoTS": "1d4"}}, "fds-0003")
	effetto(sc.luca, idA, idA, map[string]any{"nome": "Aiuto", "tempo": "altro", "round": 4800, "mod": map[string]any{"bonusTS": 1}}, "fds-0004")
	effetto(sc.m, idO, idO, map[string]any{"nome": "Scudo della fede", "tempo": "altro", "round": 4800, "mod": map[string]any{"caBonus": 2}}, "fds-0005")
	if a := cerca(sc.vista(sc.luca), "Arin"); a["ca"].(float64) != 18 {
		t.Fatalf("CA di Arin nello scontro: %v", a)
	}

	// Fine dello scontro: restano Armatura magica, Pelle coriacea (Arin è concentrato) e Aiuto; Benedizione e lo scudo
	// dell'orco finiscono.
	sc.az(sc.m, 200, "iniz.fine", nil)
	var n int
	sc.a.s.DB.QueryRow(`SELECT COUNT(*) FROM effetti_persistenti`).Scan(&n)
	if n != 3 {
		t.Fatalf("effetti che durano: %d", n)
	}
	ef, _ := sc.luca.deve(200, "GET", "/personaggi/"+sc.pg, nil)["effetti"].(map[string]any)
	if ef == nil || ef["ca"].(float64) != 18 || ef["caOrig"].(float64) != 16 || len(ef["voci"].([]any)) != 3 {
		t.Fatalf("scheda fuori dallo scontro: %v", ef)
	}

	// Scontro successivo: gli effetti valgono ancora e si vedono nel tracker.
	sc.az(sc.m, 200, "iniz.nuovo", nil)
	v := sc.vista(sc.luca)
	a := cerca(v, "Arin")
	if a == nil || a["ca"].(float64) != 18 || a["caOrig"].(float64) != 16 {
		t.Fatalf("CA nello scontro dopo: %v", a)
	}
	dur, _ := v["durevoli"].([]any)
	if len(dur) != 3 {
		t.Fatalf("effetti che durano nel tracker: %v", v["durevoli"])
	}
	idA = a["id"].(string)

	// Arin non è più concentrato: finisce la Pelle coriacea, la CA resta 18 per l'Armatura magica.
	sc.az(sc.m, 200, "pg.condizioni", map[string]any{"combattente": idA, "condizioni": []string{}})
	if d, _ := sc.vista(sc.m)["durevoli"].([]any); len(d) != 2 {
		t.Fatalf("dopo la concentrazione persa: %v", d)
	}

	// Aiuto: Sara (Segretaria, non è suo) non può terminarlo; Luca sì (lanciato dal suo PG, senza condizioni).
	var idAiuto string
	for _, x := range sc.vista(sc.luca)["durevoli"].([]any) {
		e := x.(map[string]any)
		if e["nome"] == "Aiuto" {
			idAiuto = e["id"].(string)
			if e["mio"] != true || e["giornaliero"] != true {
				t.Fatalf("Aiuto per Luca: %v", e)
			}
		}
	}
	sc.sara.deve(403, "DELETE", "/effetti-durevoli/"+idAiuto, nil)
	sc.luca.deve(200, "DELETE", "/effetti-durevoli/"+idAiuto, nil)
	sc.luca.deve(404, "DELETE", "/effetti-durevoli/"+idAiuto, nil)

	// Riposo lungo: finisce l'Armatura magica (fino a 24 ore); la scheda torna alla CA 16.
	sc.az(sc.luca, 200, "pg.riposo_lungo", map[string]any{"combattente": idA})
	sc.a.s.DB.QueryRow(`SELECT COUNT(*) FROM effetti_persistenti`).Scan(&n)
	if n != 0 {
		t.Fatalf("dopo il riposo lungo restano %d effetti", n)
	}
	if ef := sc.luca.deve(200, "GET", "/personaggi/"+sc.pg, nil)["effetti"]; ef != nil {
		t.Fatalf("scheda dopo il riposo: %v", ef)
	}

	// Nuovo giorno del gruppo: finiscono gli effetti fino a 24 ore, non quelli senza limite.
	var gruppo string
	sc.a.s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, sc.ses).Scan(&gruppo)
	sc.a.s.DB.Exec(`UPDATE personaggi SET gruppo_id = ? WHERE id = ?`, gruppo, sc.pg)
	sc.a.s.DB.Exec(`INSERT INTO effetti_persistenti(id, personaggio_id, nome, giornaliero, creato_il) VALUES('e1', ?, 'Legame protettivo', 1, 0), ('e2', ?, 'Maledizione', 0, 0)`, sc.pg, sc.pg)
	sc.a.s.fineDurevoliGiorno(&Utente{ID: "x", Nome: "Master"}, gruppo)
	var resta string
	sc.a.s.DB.QueryRow(`SELECT GROUP_CONCAT(nome) FROM effetti_persistenti`).Scan(&resta)
	if resta != "Maledizione" {
		t.Fatalf("dopo il nuovo giorno: %q", resta)
	}
	sc.m.deve(200, "DELETE", "/effetti-durevoli/e2", nil)
}
