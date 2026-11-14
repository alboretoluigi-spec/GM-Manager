package server

import (
	"strings"
	"testing"
)

// Valute del gioco (fase 3c, C4d): nei giochi diversi dalla quinta edizione il denaro è il campo «denaro» del modello;
// il Master dà e toglie (mai sotto zero), i giocatori trasferiscono a chi ha la stessa valuta; scheda e registro
// aggiornati; i personaggi della quinta edizione non compaiono; permessi.
func TestValute(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	camp := m.deve(200, "POST", "/campagne", map[string]any{"nome": "Notte senza stelle", "modulo": "libera"})["id"].(string)
	gr := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Giovedì"})["id"].(string)
	gr5 := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	m.deve(200, "PUT", "/gruppi/"+gr+"/campagna", map[string]any{"campagna": camp})
	m.deve(200, "PUT", "/gruppi/"+gr5+"/campagna", map[string]any{"campagna": "principale"})
	g := a.giocatore("Lia", "3330000001", "192.168.1.20")
	m.deve(200, "POST", "/utenti/"+a.idUtente(g)+"/approva", map[string]any{"gruppi": []string{gr, gr5}})
	arin := g.deve(200, "POST", "/personaggi", map[string]any{"gruppo": gr, "nome": "Arin"})["id"].(string)
	bea := g.deve(200, "POST", "/personaggi", map[string]any{"gruppo": gr, "nome": "Bea"})["id"].(string)
	brom := g.deve(200, "POST", "/personaggi", map[string]any{"gruppo": gr5, "nome": "Brom"})["id"].(string)

	valore := func(c *cliente, pg string) float64 {
		t.Helper()
		for _, p := range c.deve(200, "GET", "/valute", nil)["personaggi"].([]any) {
			x := p.(map[string]any)
			if x["id"] == pg {
				return x["valute"].([]any)[0].(map[string]any)["valore"].(float64)
			}
		}
		t.Fatalf("%s non è nell'elenco delle valute", pg)
		return 0
	}
	el := m.deve(200, "GET", "/valute", nil)["personaggi"].([]any)
	if len(el) != 2 || el[0].(map[string]any)["valute"].([]any)[0].(map[string]any)["nome"] != "Denaro" {
		t.Fatalf("elenco (senza il personaggio della quinta edizione): %v", el)
	}
	if valore(m, arin) != 0 {
		t.Fatal("si parte da zero")
	}
	r := m.deve(200, "POST", "/valute/"+arin+"/master", map[string]any{"valuta": "denaro", "importo": 50, "motivo": "ricompensa"})
	if !strings.Contains(r["testo"].(string), "Il Master dà 50 Denaro a Arin. Motivo: ricompensa.") || valore(g, arin) != 50 {
		t.Fatalf("il Master dà: %v", r)
	}
	if d := g.deve(200, "GET", "/personaggi/"+arin, nil)["dati"].(map[string]any)["campi"].(map[string]any)["denaro"]; d != float64(50) {
		t.Fatalf("la scheda ha il denaro nuovo: %v", d)
	}
	if e := m.deve(409, "POST", "/valute/"+arin+"/master", map[string]any{"valuta": "denaro", "importo": -80})["errore"].(string); !strings.Contains(e, "Arin ha solo 50 Denaro") {
		t.Fatalf("sotto zero: %s", e)
	}
	m.deve(400, "POST", "/valute/"+arin+"/master", map[string]any{"valuta": "oro", "importo": 5})
	g.deve(403, "POST", "/valute/"+arin+"/master", map[string]any{"valuta": "denaro", "importo": 5})
	// Trasferimenti.
	g.deve(200, "POST", "/valute/"+arin+"/trasferisci", map[string]any{"a": bea, "valuta": "denaro", "importo": 20})
	if valore(g, arin) != 30 || valore(g, bea) != 20 {
		t.Fatal("trasferimento")
	}
	if e := g.deve(400, "POST", "/valute/"+arin+"/trasferisci", map[string]any{"a": brom, "valuta": "denaro", "importo": 5})["errore"].(string); !strings.Contains(e, "Brom non usa la valuta") {
		t.Fatalf("verso un personaggio della quinta edizione: %s", e)
	}
	g.deve(409, "POST", "/valute/"+arin+"/trasferisci", map[string]any{"a": bea, "valuta": "denaro", "importo": 999})
	g.deve(400, "POST", "/valute/"+arin+"/trasferisci", map[string]any{"a": bea, "valuta": "denaro", "importo": 0})
	reg := g.deve(200, "GET", "/valute", nil)["registro"].([]any)
	if len(reg) != 3 || !strings.Contains(reg[0].(map[string]any)["testo"].(string), "Arin trasferisce 20 Denaro a Bea.") {
		t.Fatalf("registro: %v", reg)
	}
	// Un altro giocatore non vede né usa i personaggi di Lia.
	h := a.giocatore("Teo", "3330000002", "192.168.1.21")
	m.deve(200, "POST", "/utenti/"+a.idUtente(h)+"/approva", map[string]any{"gruppi": []string{gr}})
	if n := len(h.deve(200, "GET", "/valute", nil)["personaggi"].([]any)); n != 0 {
		t.Fatalf("Teo vede %d personaggi non suoi", n)
	}
	h.deve(404, "POST", "/valute/"+arin+"/trasferisci", map[string]any{"a": bea, "valuta": "denaro", "importo": 1})
}
