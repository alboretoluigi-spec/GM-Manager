package server

import (
	"encoding/json"
	"strings"
	"testing"
)

// Decisione dell'utente del 2026-11-12: un mostro nascosto che attacca un PG (o gli lancia un incantesimo) → il giocatore
// di quel PG riceve l'esito senza il nome del mostro, l'attacco, il tiro e il bonus (danni e tipo sì); gli altri giocatori
// niente; il Master tutto.
func TestMostroNascostoCheAttacca(t *testing.T) {
	s := nuovaScena(t, nil, bandito("Vampiro", nil))
	v := s.id["Vampiro"]
	s.az(s.m, 200, "iniz.nascondi", map[string]any{"combattente": v, "nascosto": true})
	cm, cl, cs := s.clienteWS(s.m), s.clienteWS(s.luca), s.clienteWS(s.sara)
	for _, c := range []*Cliente{cm, cl, cs} {
		c.canali["sessione:"+s.ses] = true
		ultimiMessaggi(c, "")
	}
	esito := func(m []string) map[string]any {
		t.Helper()
		if len(m) != 1 {
			t.Fatalf("atteso un messaggio: %v", m)
		}
		var x struct{ Dati map[string]any }
		json.Unmarshal([]byte(m[0]), &x)
		return x.Dati
	}

	// Il vampiro morde Arin: 15 + 5 contro CA 16, Maglio 4d6 + 8 = 12 + 8 = 20 contundenti.
	s.attacca(s.m, 200, v, s.idA, map[string]any{"attacco": 1, "forza": true}, 15, 3, 3, 3, 3)
	if e := esito(ultimiMessaggi(cm, "attacco.esito")); e["nomeAttaccante"] != "Vampiro" || e["anonimo"] != nil {
		t.Fatalf("il Master vede tutto: %v", e)
	}
	e := esito(ultimiMessaggi(cl, "attacco.esito"))
	if e["anonimo"] != true || e["nomeAttaccante"] != "Qualcosa" || e["attaccante"] != "" || e["attacco"] != "" || e["d20"] != nil || e["totale"].(float64) != 0 ||
		e["totaleDanni"].(float64) != 20 || e["danni"].([]any)[0].(map[string]any)["tipo"] != "contundente" || e["nomeBersaglio"] != "Arin" {
		t.Fatalf("Luca: esito senza nome con i danni: %v", e)
	}
	if b, _ := json.Marshal(e); strings.Contains(string(b), "Vampiro") || strings.Contains(string(b), "Maglio") || strings.Contains(string(b), v) {
		t.Fatalf("nell'esito anonimo non deve esserci il mostro: %s", b)
	}
	if m := ultimiMessaggi(cs, "attacco.esito"); len(m) != 0 {
		t.Fatalf("Sara (non colpita) non riceve niente: %v", m)
	}
	// Mancato: lo sa lo stesso («qualcosa ti manca»).
	s.attacca(s.m, 200, v, s.idA, map[string]any{"forza": true}, 2)
	if e := esito(ultimiMessaggi(cl, "attacco.esito")); e["risultato"] != "mancato" || e["anonimo"] != true {
		t.Fatalf("mancato anonimo: %v", e)
	}

	// Un mostro nascosto attaccato da Arin: come prima, l'esito resta al Master e a chi attacca (risposta diretta).
	ultimiMessaggi(cs, "")
	s.attacca(s.m, 200, s.idA, v, map[string]any{"forza": true}, 15, 4)
	if m := ultimiMessaggi(cs, "attacco.esito"); len(m) != 0 {
		t.Fatalf("bersaglio nascosto: niente ai giocatori: %v", m)
	}
	ultimiMessaggi(cl, "")

	// Incantesimo del vampiro nascosto su Arin (TS su Saggezza fallito, 2d6 necrotici).
	inc := map[string]any{"nome": "Tocco dell'ombra", "tempo": "altro", "tipo": "ts", "cd": 15, "car": "SAG", "danni": []any{map[string]any{"dadi": "2d6", "tipo": "necrotico"}}}
	r := s.ts(s.m, 200, v, inc, []string{s.idA}, 4, 4, 2)
	s.applica(s.m, 200, r["gettone"].(string), nil)
	e = esito(ultimiMessaggi(cl, "incantesimo.esito"))
	rr := e["righe"].([]any)[0].(map[string]any)
	if e["anonimo"] != true || e["nomeIncantatore"] != "Qualcosa" || e["incantatore"] != "" || e["incantesimo"] != "un incantesimo" ||
		rr["totaleDanni"].(float64) != 8 || rr["d20"] != nil || e["tiro"].([]any)[0].(map[string]any)["tipo"] != "necrotico" {
		t.Fatalf("incantesimo anonimo per Luca: %v", e)
	}
	if b, _ := json.Marshal(e); strings.Contains(string(b), "Vampiro") || strings.Contains(string(b), "Tocco") || strings.Contains(string(b), "2d6") {
		t.Fatalf("nell'esito anonimo non deve esserci il mostro: %s", b)
	}
	if m := ultimiMessaggi(cs, "incantesimo.esito"); len(m) != 0 {
		t.Fatalf("Sara non riceve l'incantesimo nascosto: %v", m)
	}
	// il Master lo riceve dal canale e come autore del lancio (la pagina scarta il doppione per «id»)
	if e := esito(ultimiMessaggi(cm, "incantesimo.esito")[:1]); e["nomeIncantatore"] != "Vampiro" {
		t.Fatalf("il Master vede l'incantatore: %v", e)
	}
}
