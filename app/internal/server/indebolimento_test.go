package server

import (
	"strings"
	"testing"
)

// Indebolimento: livello nella scheda (campo in gioco «exh») e nelle condizioni del tracker; quando sale, il
// giocatore riceve l'avviso per l'effetto a schermo; quando scende, no.
func TestIndebolimento(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	cl := tv.clienteWS(tv.luca)
	ultimiMessaggi(cl, "")

	// Dalla scheda: livello 2 → condizione «Indebolimento 2» e avviso al giocatore.
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"exh": 2}})
	v := tv.m.deve(200, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{})
	if c := v["condizioni"].([]any); len(c) != 1 || c[0] != "Indebolimento 2" {
		t.Fatalf("condizioni: %v", c)
	}
	m := ultimiMessaggi(cl, "indebolimento")
	if len(m) != 1 || !strings.Contains(m[0], `"livello":2`) || !strings.Contains(m[0], "Arin") {
		t.Fatalf("avviso dell'indebolimento: %v", m)
	}
	if d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any); d["exh"] != 2.0 {
		t.Fatalf("la scheda legge il livello: %v", d["exh"])
	}
	// Assegnato dal Master (condizioni, come dal tracker): 3, con le altre condizioni che restano.
	tv.m.deve(200, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{"condizioni": []string{"Avvelenato", "Indebolimento 3"}})
	if m := ultimiMessaggi(cl, "indebolimento"); len(m) != 1 || !strings.Contains(m[0], `"livello":3`) {
		t.Fatalf("assegnato dal Master: %v", m)
	}
	// Dopo il Master, il giocatore lo alza dalla scheda: 4.
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"exh": 4}})
	if m := ultimiMessaggi(cl, "indebolimento"); len(m) != 1 || !strings.Contains(m[0], `"livello":4`) {
		t.Fatalf("dalla scheda dopo il Master: %v", m)
	}
	// Scende a 1 (riposo lungo): nessun effetto; le altre condizioni restano.
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"exh": 1}})
	if m := ultimiMessaggi(cl, "indebolimento"); len(m) != 0 {
		t.Fatalf("se scende non c'è effetto: %v", m)
	}
	v = tv.m.deve(200, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{})
	if c := v["condizioni"].([]any); len(c) != 2 || c[0] != "Avvelenato" || c[1] != "Indebolimento 1" {
		t.Fatalf("condizioni dopo il livello 1: %v", c)
	}
}
