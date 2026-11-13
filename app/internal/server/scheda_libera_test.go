package server

import (
	"encoding/json"
	"strings"
	"testing"
)

// Formule dei tiri della scheda libera: dadi ammessi, numeri, limiti.
func TestFormuleSchedaLibera(t *testing.T) {
	buone := map[string]int{"2d6 + 1d4 - 2": 3, "d20": 1, "1d100+5": 2, " 3d6 −1 ": 2}
	for f, n := range buone {
		x, err := leggiFormula(f)
		if err != nil || len(x) != n {
			t.Errorf("«%s»: %v %v", f, x, err)
		}
	}
	for _, f := range []string{"", "3", "1d7", "21d6", "2d6 + forza", "30d6+30d6"} {
		if _, err := leggiFormula(f); err == nil {
			t.Errorf("«%s» doveva essere rifiutata", f)
		}
	}
	for i := 0; i < 50; i++ {
		v, nat, testo := tiroIniziativa("1d10 + 3", 0)
		if v < 4 || v > 13 || nat != nil || !strings.Contains(testo, "1d10 + 3") {
			t.Fatalf("iniziativa con la formula: %d %v %s", v, nat, testo)
		}
		if v, nat, _ := tiroIniziativa("", 2); v < 3 || v > 22 || nat == nil {
			t.Fatalf("iniziativa d20 + bonus: %d %v", v, nat)
		}
	}
}

// Scheda libera (fase 3c, C4b): modello di base, modello personalizzato con i controlli, gioco e modello di un
// personaggio, ritorno al modello di base, solo il Master cambia il modello.
func TestSchedaLibera(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	camp := m.deve(200, "POST", "/campagne", map[string]any{"nome": "Notte senza stelle", "modulo": "libera"})["id"].(string)
	x := m.deve(200, "GET", "/campagne/"+camp+"/modello-scheda", nil)
	if x["personalizzato"] != false || len(x["modello"].(map[string]any)["sezioni"].([]any)) == 0 {
		t.Fatalf("modello di base: %v", x)
	}
	salva := func(corpo string, atteso int) string {
		t.Helper()
		cod, _, b := m.grezzo("PUT", "/campagne/"+camp+"/modello-scheda", []byte(corpo), "application/json")
		if cod != atteso {
			t.Fatalf("modello: atteso %d, ottenuto %d (%s)", atteso, cod, b)
		}
		return string(b)
	}
	sez := func(campi string) string { return `{"sezioni":[{"titolo":"Valori","campi":[` + campi + `]}]` }
	casi := map[string]string{
		sez(`{"id":"forza","nome":"Forza","tipo":"numero"},{"id":"forza","nome":"Ancora","tipo":"numero"}`) + `}`:          "«forza» è già usato",
		sez(`{"id":"Forza","nome":"Forza","tipo":"numero"}`) + `}`:                                                         "l'identificativo deve essere",
		sez(`{"id":"forza","nome":"Forza","tipo":"colore"}`) + `}`:                                                         "tipo «colore» sconosciuto",
		sez(`{"id":"forza","nome":"Forza","tipo":"numero","tiro":"1d7 + {forza}"}`) + `}`:                                  "tiro del campo «Forza»: «1d7»",
		sez(`{"id":"forza","nome":"Forza","tipo":"numero"}`) + `,"tiri":[{"nome":"Colpo","formula":"2d6 + {destrezza}"}]}`: "«{destrezza}» non è un campo numerico",
		sez(`{"id":"nome","nome":"Nome","tipo":"testo"}`) + `,"pf":"nome"}`:                                                "i punti ferita devono essere",
		sez(`{"id":"forza","nome":"Forza","tipo":"numero"}`) + `,"colore":"blu"}`:                                          `campo sconosciuto \"colore\"`,
		`{"sezioni":[]}`: "da 1 a 20 sezioni",
	}
	for corpo, atteso := range casi {
		if e := salva(corpo, 400); !strings.Contains(e, atteso) {
			t.Errorf("%s: atteso «%s», ottenuto %s", corpo, atteso, e)
		}
	}
	buono := `{"sezioni":[{"titolo":"Valori","campi":[{"id":"forza","nome":"Forza","tipo":"numero","min":0,"max":10,"predefinito":3,"tiro":"{forza}d6"},
		{"id":"riflessi","nome":"Riflessi","tipo":"numero","predefinito":2},{"id":"vita","nome":"Vita","tipo":"risorsa","predefinito":12},
		{"id":"armatura","nome":"Armatura","tipo":"numero","predefinito":1}]}],
		"tiri":[{"nome":"Colpo","formula":"2d6 + {forza}"}],"pf":"vita","difesa":"armatura","iniziativa":"1d10 + {riflessi}"}`
	if r := salva(buono, 200); !strings.Contains(r, `"personalizzato":true`) || !strings.Contains(r, `"iniziativa":"1d10 + {riflessi}"`) {
		t.Fatalf("modello salvato: %s", r)
	}
	// Personaggio di un gruppo della campagna: gioco e modello della campagna.
	gr := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Giovedì"})["id"].(string)
	gr2 := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	g := a.giocatore("Lia", "3330000001", "192.168.1.20")
	m.deve(200, "POST", "/utenti/"+a.idUtente(g)+"/approva", map[string]any{"gruppi": []string{gr, gr2}})
	m.deve(200, "PUT", "/gruppi/"+gr+"/campagna", map[string]any{"campagna": camp})
	pg := g.deve(200, "POST", "/personaggi", map[string]any{"gruppo": gr, "nome": "Arin"})["id"].(string)
	info := g.deve(200, "GET", "/personaggi/"+pg, nil)
	if gi := info["gioco"].(map[string]any); gi["campagna"] != camp || gi["modulo"] != "libera" {
		t.Fatalf("gioco del personaggio: %v", info["gioco"])
	}
	ms := g.deve(200, "GET", "/personaggi/"+pg+"/modello-scheda", nil)
	b, _ := json.Marshal(ms["modello"])
	if ms["nomeModulo"] != "Scheda libera (qualsiasi gioco)" || !strings.Contains(string(b), `"riflessi"`) {
		t.Fatalf("modello per la scheda: %v", ms)
	}
	g.deve(403, "PUT", "/campagne/"+camp+"/modello-scheda", map[string]any{})
	g.deve(403, "GET", "/campagne/"+camp+"/modello-scheda", nil)
	// Un personaggio della quinta edizione.
	m.deve(200, "PUT", "/gruppi/"+gr2+"/campagna", map[string]any{"campagna": "principale"})
	pg2 := g.deve(200, "POST", "/personaggi", map[string]any{"gruppo": gr2, "nome": "Brom"})["id"].(string)
	if gi := m.deve(200, "GET", "/personaggi/"+pg2, nil)["gioco"].(map[string]any); gi["modulo"] != "d20-5e" {
		t.Fatalf("personaggio della quinta edizione: %v", gi)
	}
	// Ritorno al modello di base.
	if r := salva("null", 200); !strings.Contains(r, `"personalizzato":false`) {
		t.Fatalf("modello di base: %s", r)
	}
}
