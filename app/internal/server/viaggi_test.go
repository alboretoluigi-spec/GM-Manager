package server

import (
	"encoding/json"
	"strings"
	"testing"
)

// Viaggi del segnalino: registro, tempo di gioco (24 ore = un giorno), eventi e avviso per gruppo e Master
// (nome della località ai giocatori solo se svelato); sotto 1/4 di miglio non è un viaggio.
func TestViaggi(t *testing.T) {
	sc := nuovoScontro(t)
	s := sc.a.s
	var gr string
	s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, sc.ses).Scan(&gr)
	cl := sc.clienteWS(sc.luca)
	cm := sc.clienteWS(sc.m)
	s.Hub.mu.Lock()
	cl.canali["gruppo:"+gr] = true
	s.Hub.mu.Unlock()
	ultimiMessaggi(cl, "")
	ultimiMessaggi(cm, "")
	avvisi := func(c *Cliente) []map[string]any {
		var out []map[string]any
		for _, m := range ultimiMessaggi(c, "viaggio.avviso") {
			var x struct {
				Dati map[string]any `json:"dati"`
			}
			json.Unmarshal([]byte(m), &x)
			out = append(out, x.Dati)
		}
		return out
	}
	tipi := func(d map[string]any) string {
		var t []string
		for _, e := range d["eventi"].([]any) {
			t = append(t, e.(map[string]any)["tipo"].(string))
		}
		return strings.Join(t, ",")
	}
	viaggio := func(mi, ore float64, pezzi []map[string]any, arrivo string) map[string]any {
		return map[string]any{"miglia": mi, "ore": ore, "fatica": ore, "mph": 3, "pezzi": pezzi, "arrivo": arrivo}
	}
	strada := func(mi float64) map[string]any { return map[string]any{"tipo": "strada", "nome": "", "mi": mi, "f": 1} }

	// Prima posizione: non è un viaggio.
	sc.mappa(sc.m, 200, "POST", "party", map[string]any{"azione": "posiziona", "x": 1100, "y": 600, "viaggio": viaggio(5, 2, nil, "")})
	if n := len(avvisi(cl)); n != 0 {
		t.Fatalf("prima posizione: %d avvisi", n)
	}
	// Al Villaggio di Nebbiavalle con un tratto di foresta: terreno difficile e arrivo.
	v := viaggio(5, 2.5, []map[string]any{strada(3), {"tipo": "foresta", "nome": "Foresta di Selvascura", "mi": 2, "f": .5}}, "villaggio-di-gasterbless")
	sc.mappa(sc.luca, 200, "POST", "party", map[string]any{"azione": "muovi", "id": "g", "x": 1245, "y": 620, "viaggio": v})
	a := avvisi(cl)
	if len(a) != 1 || tipi(a[0]) != "terreno,arrivo" || !strings.Contains(string(mustJSON(a[0])), "un luogo ancora sconosciuto") {
		t.Fatalf("avviso al giocatore: %v", a)
	}
	if m := avvisi(cm); len(m) != 1 || !strings.Contains(string(mustJSON(m[0])), "Villaggio di Nebbiavalle") {
		t.Fatalf("avviso al Master con il nome vero: %v", m)
	}
	// Spostamento minimo: niente.
	sc.mappa(sc.luca, 200, "POST", "party", map[string]any{"azione": "muovi", "id": "g", "x": 1248, "y": 621, "viaggio": viaggio(.1, .03, nil, "")})
	// Dati non validi: ignorati (lo spostamento resta).
	sc.mappa(sc.luca, 200, "POST", "party", map[string]any{"azione": "muovi", "id": "g", "x": 1249, "y": 622, "viaggio": map[string]any{"miglia": 3, "ore": 1, "fatica": 1, "mph": 9}})
	if n := len(avvisi(cl)); n != 0 {
		t.Fatalf("spostamenti che non sono viaggi: %d avvisi", n)
	}
	d := sc.mappa(sc.luca, 200, "GET", "viaggi", nil)
	if d["giorno"] != 1.0 || d["oreGioco"] != 2.5 || len(d["lista"].([]any)) != 1 || d["lista"].([]any)[0].(map[string]any)["arrivo"] != "un luogo ancora sconosciuto" {
		t.Fatalf("diario: %v", d)
	}
	// Altre 7 ore di strada: 9 h 30 di marcia → marcia forzata (CD 12).
	sc.mappa(sc.luca, 200, "POST", "party", map[string]any{"azione": "muovi", "id": "g", "x": 1000, "y": 500, "viaggio": viaggio(21, 7, []map[string]any{strada(21)}, "")})
	a = avvisi(cl)
	if len(a) != 1 || tipi(a[0]) != "marcia" || !strings.Contains(string(mustJSON(a[0])), "CD 12") {
		t.Fatalf("marcia forzata: %v", a)
	}
	// Altre 15 ore: si passa la mezzanotte → giorno 2 (anche per le Professioni).
	sc.mappa(sc.luca, 200, "POST", "party", map[string]any{"azione": "muovi", "id": "g", "x": 800, "y": 500, "viaggio": viaggio(45, 15, []map[string]any{strada(45)}, "")})
	a = avvisi(cl)
	if len(a) != 1 || !strings.Contains(tipi(a[0]), "giorno") || a[0]["riepilogo"].(map[string]any)["giorno"] != 2.0 {
		t.Fatalf("nuovo giorno: %v", a)
	}
	// Il Master: nuovo giorno a mano (il gruppo si accampa); il giocatore no.
	sc.mappa(sc.luca, 403, "POST", "viaggi/giorno", map[string]any{})
	if r := sc.mappa(sc.m, 200, "POST", "viaggi/giorno", map[string]any{}); r["giorno"] != 3.0 {
		t.Fatalf("nuovo giorno del Master: %v", r)
	}
	d = sc.mappa(sc.m, 200, "GET", "viaggi", nil)
	if d["oreMarcia"] != 0.0 || d["oreGioco"] != 48.0 || len(d["lista"].([]any)) != 3 || d["lista"].([]any)[2].(map[string]any)["arrivo"] != "Villaggio di Nebbiavalle" {
		t.Fatalf("diario del Master: %v", d)
	}
	// Annulla l'ultimo viaggio (Master).
	sc.mappa(sc.luca, 403, "DELETE", "viaggi/ultimo", nil)
	sc.mappa(sc.m, 200, "DELETE", "viaggi/ultimo", nil)
	if d = sc.mappa(sc.m, 200, "GET", "viaggi", nil); len(d["lista"].([]any)) != 2 {
		t.Fatalf("annullato: %v", d)
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
