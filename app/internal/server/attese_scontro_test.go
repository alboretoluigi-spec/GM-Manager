package server

import (
	"encoding/json"
	"testing"
	"time"
)

// riavvioSimulato svuota la memoria degli attacchi come un riavvio di Gaster Bless (il database resta).
func riavvioSimulato() {
	m := attacchiInCorso
	m.serie.Lock()
	m.critici, m.fatte, m.quando, m.lanci, m.risolti = map[string]*critico{}, map[string]esitoAttacco{}, map[string]time.Time{}, map[string]*lancioInCorso{}, map[string]any{}
	m.tsMu.Lock()
	m.ts = map[string]*tsInAttesa{}
	m.tsMu.Unlock()
	m.serie.Unlock()
}

// Decisione dell'utente del 2026-11-12 (migrazione 032): il critico da scegliere e i tiri salvezza da confermare
// sopravvivono a un riavvio di Gaster Bless.
func TestAtteseDopoUnRiavvio(t *testing.T) {
	s := nuovaScena(t, nil, bandito("Bandito", map[string]any{"ts": map[string]any{"DES": 1}}), bandito("Ombra", nil))
	conta := func() (n int) { s.a.s.DB.QueryRow(`SELECT COUNT(*) FROM attese_scontro`).Scan(&n); return }

	// Critico di Arin sul bandito, poi Gaster Bless si riavvia prima della scelta.
	g := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 20)["gettone"].(string)
	if conta() != 1 {
		t.Fatalf("critico salvato: %d", conta())
	}
	riavvioSimulato()
	e := s.critico(s.luca, 200, g, "dadi", 4, 6) // 4 + 6 + 3 = 13
	if e["totaleDanni"].(float64) != 13 || s.pf("Bandito") != 17 || conta() != 0 {
		t.Fatalf("critico dopo il riavvio: %v, PF %v, righe %d", e, s.pf("Bandito"), conta())
	}
	riavvioSimulato()
	s.critico(s.luca, 409, g, "dadi", 8, 8) // monouso anche dopo un altro riavvio
	if s.pf("Bandito") != 17 {
		t.Fatal("il critico non si applica due volte")
	}

	// Palla di fuoco di Arin: il TS aspetta il Master; riavvio; il Master lo vede ancora e lo applica.
	s.az(s.m, 200, "iniz.prossimo_turno", nil)
	s.az(s.m, 200, "iniz.prossimo_turno", nil)
	s.az(s.m, 200, "iniz.prossimo_turno", nil) // round 2, Arin
	palla := map[string]any{"nome": "Palla di fuoco", "livello": 3, "slot": 3, "tempo": "azione", "tipo": "ts", "cd": 13, "car": "DES", "seRiesce": "meta",
		"danni": []any{map[string]any{"dadi": "2d6", "tipo": "fuoco"}}}
	s.ts(s.luca, 200, s.idA, palla, []string{s.id["Bandito"]}, 3, 3, 2) // 6 danni, TS 3 fallito
	riavvioSimulato()
	p := s.inAttesa()
	if r := riga0(p); r["riuscito"] != false || r["totale"].(float64) != 3 {
		t.Fatalf("TS in attesa dopo il riavvio: %v", p)
	}
	s.applica(s.m, 200, p["gettone"].(string), nil)
	if s.pf("Bandito") != 11 || conta() != 0 || s.vista(s.m)["incantesimiInAttesa"] != nil {
		t.Fatalf("applicato dopo il riavvio: PF %v, righe %d", s.pf("Bandito"), conta())
	}

	// Critico di un mostro nascosto su Arin: dopo il riavvio l'esito resta anonimo per Luca.
	s.az(s.m, 200, "iniz.nascondi", map[string]any{"combattente": s.id["Ombra"], "nascosto": true})
	g = s.attacca(s.m, 200, s.id["Ombra"], s.idA, map[string]any{"forza": true}, 20)["gettone"].(string)
	riavvioSimulato()
	cl := s.clienteWS(s.luca)
	cl.canali["sessione:"+s.ses] = true
	s.critico(s.m, 200, g, "dadi", 2, 2)
	m := ultimiMessaggi(cl, "attacco.esito")
	var x struct{ Dati map[string]any }
	if len(m) == 1 {
		json.Unmarshal([]byte(m[0]), &x)
	}
	if x.Dati["anonimo"] != true || x.Dati["nomeAttaccante"] != "Qualcosa" {
		t.Fatalf("mostro nascosto dopo il riavvio: %v", m)
	}

	// Le attese scadute si cancellano da sole.
	s.a.s.DB.Exec(`INSERT INTO attese_scontro(gettone, tipo, combattimento_id, dati, scade) VALUES('vecchio', 'critico', 'x', '{}', 1)`)
	s.vista(s.m)
	var n int
	s.a.s.DB.QueryRow(`SELECT COUNT(*) FROM attese_scontro WHERE gettone = 'vecchio'`).Scan(&n)
	if n != 0 {
		t.Fatal("attesa scaduta non cancellata")
	}
}
