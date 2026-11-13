package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gasterbless/internal/mqtt"
)

// evento aspetta un esito dello scontro inoltrato al giocatore remoto che soddisfa «ok».
func (g *giocatoreLontano) evento(cosa string, ok func(e map[string]any) bool) map[string]any {
	g.t.Helper()
	for fine := time.Now().Add(5 * time.Second); time.Now().Before(fine); time.Sleep(20 * time.Millisecond) {
		g.mu.Lock()
		for _, e := range g.eventi {
			if ok(e) {
				g.mu.Unlock()
				return e
			}
		}
		g.mu.Unlock()
	}
	g.t.Fatalf("il giocatore remoto non ha ricevuto: %s (eventi %v)", cosa, g.eventi)
	return nil
}

// Decisione dell'utente del 2026-11-12: anche nel gioco da remoto (MQTT) gli attacchi automatici con la scelta del
// critico, le azioni del turno, l'ira e gli esiti degli scontri; stesse regole dell'app.
func TestGiocoDaRemotoAttacchi(t *testing.T) {
	broker := mqtt.NuovoBrokerDiProva()
	hs := httptest.NewServer(broker)
	defer hs.Close()
	url := "ws" + strings.TrimPrefix(hs.URL, "http")

	s := nuovaScena(t, map[string]any{"rage": map[string]any{"res": []string{"tagliente"}, "danniMischia": 2}}, bandito("Lupo", nil), bandito("Ombra", nil))
	s.a.s.Cfg.BrokerMQTT = []string{url}
	s.az(s.m, 200, "iniz.nascondi", map[string]any{"combattente": s.id["Ombra"], "nascosto": true})
	s.m.deve(200, "PATCH", "/sessioni/"+s.ses, map[string]any{"mqttRemoto": true})
	conf := s.m.deve(200, "POST", "/sessioni/"+s.ses+"/remoto/giocatori", map[string]any{"utente": s.idLuca})["configurazione"].(map[string]any)
	g := collegaLontano(t, url, conf)

	// Lo stato del giocatore contiene gli attacchi e le azioni del turno del suo PG (non quelli dei mostri), niente Ombra.
	st := g.aspetta("gli attacchi di Arin", func(x map[string]any) bool {
		if x == nil {
			return false
		}
		a := cerca(x["vista"].(map[string]any), "Arin")
		return a != nil && a["attacchi"] != nil
	})
	v := st["vista"].(map[string]any)
	if a := cerca(v, "Arin"); len(a["attacchi"].([]any)) != 4 || a["economia"] == nil || a["ira"] != true || a["mio"] != true || a["proprietario"] != nil {
		t.Fatalf("Arin per il giocatore remoto: %v", a)
	}
	if l := cerca(v, "Lupo"); l["attacchi"] != nil || l["pf"] != nil || cerca(v, "Ombra") != nil {
		t.Fatalf("i mostri restano coperti: %v", v)
	}

	// Attacco di Arin da remoto: 15 + 5 contro CA 12, 4 + 3 = 7.
	tiriDecisi(t, 15, 4)
	g.azione("r1", "attacco.tira", map[string]any{"combattente": s.idA, "bersaglio": s.id["Lupo"], "attacco": 0, "azione": "remoto-0001"}, time.Now())
	e := g.esito("r1")
	ris, _ := e["risultato"].(map[string]any)
	if e["ok"] != true || ris["risultato"] != "colpito" || ris["totaleDanni"].(float64) != 7 || ris["ca"] != nil {
		t.Fatalf("attacco da remoto: %v", e)
	}
	if s.pf("Lupo") != 23 {
		t.Fatalf("PF del lupo: %v", s.pf("Lupo"))
	}
	// Le stesse regole dell'app: un solo attacco per azione.
	g.azione("r2", "attacco.tira", map[string]any{"combattente": s.idA, "bersaglio": s.id["Lupo"], "azione": "remoto-0002"}, time.Now())
	if e := g.esito("r2"); e["ok"] != false || !strings.Contains(e["errore"].(string), "attacchi") {
		t.Fatalf("secondo attacco: %v", e)
	}
	// Azioni del turno e ira (azione bonus).
	g.azione("r3", "pg.ira", map[string]any{"combattente": s.idA, "azione": "remoto-0003"}, time.Now())
	if e := g.esito("r3"); e["ok"] != true {
		t.Fatalf("ira da remoto: %v", e)
	}
	g.azione("r4", "iniz.economia", map[string]any{"combattente": s.idA, "economia": map[string]any{"a": true, "b": true, "r": true}}, time.Now())
	if e := g.esito("r4"); e["ok"] != true || s.economia(s.m, "Arin")["r"] != true {
		t.Fatalf("azioni del turno da remoto: %v", e)
	}
	// Non muove i mostri.
	g.azione("r5", "attacco.tira", map[string]any{"combattente": s.id["Lupo"], "bersaglio": s.idA, "azione": "remoto-0005"}, time.Now())
	if e := g.esito("r5"); e["ok"] != false {
		t.Fatalf("un mostro da remoto: %v", e)
	}

	// Critico da remoto al turno dopo: gettone, poi la scelta del modo.
	for range 3 {
		s.az(s.m, 200, "iniz.prossimo_turno", nil)
	}
	tiriDecisi(t, 20)
	g.azione("r6", "attacco.tira", map[string]any{"combattente": s.idA, "bersaglio": s.id["Lupo"], "attacco": 0, "azione": "remoto-0006"}, time.Now())
	gt, _ := g.esito("r6")["risultato"].(map[string]any)["gettone"].(string)
	if gt == "" {
		t.Fatal("critico da remoto: manca il gettone")
	}
	tiriDecisi(t, 4, 6)
	g.azione("r7", "attacco.critico", map[string]any{"gettone": gt, "modo": "dadi", "azione": "remoto-0007"}, time.Now())
	if r := g.esito("r7")["risultato"].(map[string]any); r["totaleDanni"].(float64) != 15 { // 4 + 6 + 3 + 2 dell'ira
		t.Fatalf("critico scelto da remoto: %v", r)
	}

	// Esiti degli altri: il lupo attacca Arin (il Master); l'Ombra nascosta lo colpisce (anonimo).
	tiriDecisi(t, 15, 4)
	s.attacca(s.m, 200, s.id["Lupo"], s.idA, map[string]any{"forza": true})
	g.evento("l'attacco del lupo", func(e map[string]any) bool {
		d := e["dati"].(map[string]any)
		return e["evento"] == "attacco.esito" && d["nomeAttaccante"] == "Lupo" && d["nomeBersaglio"] == "Arin"
	})
	tiriDecisi(t, 15, 4)
	s.attacca(s.m, 200, s.id["Ombra"], s.idA, map[string]any{"forza": true})
	an := g.evento("il colpo anonimo", func(e map[string]any) bool { return e["dati"].(map[string]any)["anonimo"] == true })
	if d := an["dati"].(map[string]any); d["nomeAttaccante"] != "Qualcosa" || d["attaccante"] != "" {
		t.Fatalf("colpo anonimo da remoto: %v", d)
	}
	g.mu.Lock()
	for _, e := range g.eventi {
		if strings.Contains(strings.ToLower(e["dati"].(map[string]any)["nomeAttaccante"].(string)), "ombra") {
			g.mu.Unlock()
			t.Fatalf("il nome del mostro nascosto non deve arrivare: %v", e)
		}
	}
	g.mu.Unlock()
}
