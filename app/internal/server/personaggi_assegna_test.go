package server

import "testing"

// Il Master crea una scheda, la assegna a un giocatore, la vede e la modifica; i giocatori non assegnano.
func TestAssegnaPersonaggio(t *testing.T) {
	tv := nuovoTavolo(t)
	idLuca, idUgo := tv.a.idUtente(tv.luca), tv.a.idUtente(tv.ugo)
	conta := func(c *cliente) int { return len(c.deve(200, "GET", "/personaggi", nil)["lista"].([]any)) }
	// scheda preparata dal Master per sé
	pg := tv.m.deve(200, "POST", "/personaggi", map[string]any{"nome": "Elara"})["id"].(string)
	if conta(tv.luca) != 0 {
		t.Fatal("prima dell'assegnazione Luca non la vede")
	}
	tv.luca.deve(403, "POST", "/personaggi/"+pg+"/assegna", map[string]any{"proprietario": idLuca})
	tv.m.deve(400, "POST", "/personaggi/"+pg+"/assegna", map[string]any{"proprietario": idUgo, "gruppo": tv.g1}) // Ugo non è del gruppo 1
	out := tv.m.deve(200, "POST", "/personaggi/"+pg+"/assegna", map[string]any{"proprietario": idLuca})
	if out["gruppo"] != tv.g1 {
		t.Fatalf("un solo gruppo: si assegna da sé (%v)", out["gruppo"])
	}
	if conta(tv.luca) != 1 || conta(tv.sara) != 0 {
		t.Fatal("dopo l'assegnazione la vede Luca, non Sara")
	}
	// Luca la modifica; il Master la vede e la modifica a sua volta
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"cls": "mago"}})
	tv.m.deve(200, "GET", "/personaggi/"+pg, nil)
	tv.m.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"lvl": 2}})
	// il Master la riprende per sé: Luca non la vede più
	tv.m.deve(200, "POST", "/personaggi/"+pg+"/assegna", map[string]any{"proprietario": ""})
	if conta(tv.luca) != 0 {
		t.Fatal("ripresa dal Master: Luca non la vede più")
	}
	tv.luca.deve(404, "GET", "/personaggi/"+pg, nil)
	tv.m.deve(404, "POST", "/personaggi/non-esiste/assegna", map[string]any{"proprietario": idLuca})
}

// Solo il Master elimina una scheda, scrivendo il nome come conferma; sparisce anche dallo scontro.
func TestEliminaPersonaggio(t *testing.T) {
	sc := nuovoScontro(t)
	if c := len(combattenti(sc.vista(sc.m))); c != 1 {
		t.Fatalf("Arin è nello scontro: %d", c)
	}
	sc.luca.deve(403, "DELETE", "/personaggi/"+sc.pg, map[string]any{"conferma": "Arin"})
	sc.m.deve(400, "DELETE", "/personaggi/"+sc.pg, map[string]any{"conferma": "arin"})
	sc.m.deve(200, "DELETE", "/personaggi/"+sc.pg, map[string]any{"conferma": "Arin"})
	sc.m.deve(404, "GET", "/personaggi/"+sc.pg, nil)
	if l := sc.luca.deve(200, "GET", "/personaggi", nil)["lista"].([]any); len(l) != 0 {
		t.Fatal("Luca non ha più il personaggio")
	}
	if c := len(combattenti(sc.vista(sc.m))); c != 0 {
		t.Fatalf("il personaggio esce dallo scontro: %d", c)
	}
	sc.m.deve(404, "DELETE", "/personaggi/"+sc.pg, map[string]any{"conferma": "Arin"})
}
