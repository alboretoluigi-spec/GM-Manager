package server

import (
	"strings"
	"testing"
)

// Interfaccia adattiva (fase 3c, C4a): senza scelta vale la prima campagna (quinta edizione, con le sue sezioni e i
// riposi); il Master sceglie una campagna con la scheda libera e Master e giocatori vedono solo le sezioni comuni;
// gioco della campagna validato; modifica del gioco della campagna attiva; eliminazione della campagna attiva; solo il
// Master sceglie.
func TestGiocoAttivo(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := a.giocatore("Lia", "3330000001", "192.168.1.20")
	gr := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	m.deve(200, "POST", "/utenti/"+a.idUtente(g)+"/approva", map[string]any{"gruppi": []string{gr}})
	gioco := func(c *cliente) map[string]any { return c.deve(200, "GET", "/io", nil)["gioco"].(map[string]any) }
	ha := func(l any, v string) bool {
		for _, x := range l.([]any) {
			if x == v {
				return true
			}
		}
		return false
	}
	x := gioco(m)
	if x["scelta"] != false || x["campagna"].(map[string]any)["id"] != "principale" || x["modulo"].(map[string]any)["id"] != "d20-5e" ||
		!ha(x["sezioni"], "contenuti") || !ha(x["sezioni"], "sessioni") || !ha(x["barra"], "riposi") {
		t.Fatalf("gioco predefinito: %v", x)
	}
	if len(m.deve(200, "GET", "/gioco", nil)["moduli"].([]any)) < 10 {
		t.Fatal("il Master riceve l'elenco dei giochi")
	}
	if _, ok := g.deve(200, "GET", "/gioco", nil)["moduli"]; ok {
		t.Fatal("i giocatori non ricevono l'elenco dei giochi")
	}
	m.deve(400, "POST", "/campagne", map[string]any{"nome": "Notte", "modulo": "inventato"})
	id := m.deve(200, "POST", "/campagne", map[string]any{"nome": "Notte senza stelle", "modulo": "libera"})["id"].(string)
	g.deve(403, "PUT", "/gioco", map[string]any{"campagna": id})
	m.deve(404, "PUT", "/gioco", map[string]any{"campagna": "nessuna"})
	x = m.deve(200, "PUT", "/gioco", map[string]any{"campagna": id})
	if x["scelta"] != true || x["modulo"].(map[string]any)["id"] != "libera" || ha(x["sezioni"], "contenuti") || ha(x["sezioni"], "oggetti_magici") ||
		!ha(x["sezioni"], "mappa") || len(x["barra"].([]any)) != 0 {
		t.Fatalf("scheda libera: %v", x)
	}
	if y := gioco(g); y["campagna"].(map[string]any)["nome"] != "Notte senza stelle" || ha(y["sezioni"], "sanita") {
		t.Fatalf("il giocatore segue la scelta del Master: %v", y)
	}
	// Cambia il gioco della campagna attiva: d100 accende la sanità mentale.
	m.deve(200, "PATCH", "/campagne/"+id, map[string]any{"modulo": "d100"})
	if y := gioco(g); !ha(y["sezioni"], "sanita") || y["modulo"].(map[string]any)["pronto"] != true {
		t.Fatalf("gioco d100: %v", y)
	}
	m.deve(400, "PATCH", "/campagne/"+id, map[string]any{"modulo": ""})
	cs := m.deve(200, "GET", "/campagne", nil)["campagne"].([]any)
	if !strings.Contains(cs[1].(map[string]any)["modulo"].(string), "d100") {
		t.Fatalf("il gioco della campagna si vede nell'elenco: %v", cs)
	}
	// Un gruppo nuovo va nella campagna attiva.
	gn := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Venerdì"})["id"].(string)
	trovato := false
	for _, c := range m.deve(200, "GET", "/campagne", nil)["campagne"].([]any) {
		for _, x := range c.(map[string]any)["gruppi"].([]any) {
			trovato = trovato || x.(map[string]any)["id"] == gn && c.(map[string]any)["id"] == id
		}
	}
	if !trovato {
		t.Fatal("il gruppo nuovo deve andare nella campagna attiva")
	}
	m.deve(200, "PUT", "/gruppi/"+gn+"/campagna", map[string]any{"campagna": "principale"})
	// Eliminata la campagna attiva: si torna alla prima, da scegliere di nuovo.
	m.deve(200, "DELETE", "/campagne/"+id, nil)
	if y := gioco(m); y["scelta"] != false || y["campagna"].(map[string]any)["id"] != "principale" {
		t.Fatalf("dopo l'eliminazione: %v", y)
	}
}
