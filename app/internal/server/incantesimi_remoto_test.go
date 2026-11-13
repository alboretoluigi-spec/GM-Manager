package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gasterbless/internal/mqtt"
)

// Fase 9b (2026-11-12): incantesimi automatici mandati dalla scheda nel riepilogo, lanciati dal gioco da remoto con lo
// slot tolto dal server.
func TestIncantesimiDaRemoto(t *testing.T) {
	broker := mqtt.NuovoBrokerDiProva()
	hs := httptest.NewServer(broker)
	defer hs.Close()
	url := "ws" + strings.TrimPrefix(hs.URL, "http")

	danni := func(d, tipo string) []any { return []any{map[string]any{"dadi": d, "mod": 0, "tipo": tipo}} }
	spells := []any{
		map[string]any{"nome": "Fiamma sacra", "livello": 0, "livelli": []any{map[string]any{"slot": 0, "tempo": "azione", "tipo": "ts", "cd": 13, "car": "DES", "danni": danni("1d8", "radiante")}}},
		map[string]any{"nome": "Raggio rovente", "livello": 2, "livelli": []any{
			map[string]any{"slot": 2, "tempo": "azione", "tipo": "attacco", "bonus": 5, "danni": danni("2d6", "fuoco")},
			map[string]any{"slot": 1, "tempo": "azione", "tipo": "attacco", "bonus": 5}}}, // slot sotto il livello: scartato
		map[string]any{"nome": "Cura ferite", "livello": 1, "livelli": []any{map[string]any{"slot": 1, "tempo": "azione", "tipo": "cura", "cura": map[string]any{"dadi": "1d8", "mod": 3}, "bersagli": 1}}},
		map[string]any{"nome": "Fuori tipo", "livello": 1, "livelli": []any{map[string]any{"slot": 1, "tipo": "boh"}}}, // scartato
	}
	s := nuovaScena(t, map[string]any{"spellsAuto": spells, "slots": map[string]any{"1": 1, "2": 1, "10": 4}}, bandito("Bandito", map[string]any{"ts": map[string]any{"DES": 0}}))
	a := cerca(s.vista(s.luca), "Arin")
	ia, _ := a["incantesimiAuto"].([]any)
	if len(ia) != 3 || len(ia[1].(map[string]any)["livelli"].([]any)) != 1 || len(a["slot"].([]any)) != 2 {
		t.Fatalf("incantesimi automatici e slot per il proprietario: %v %v", a["incantesimiAuto"], a["slot"])
	}
	if x := cerca(s.vista(s.sara), "Arin"); x["incantesimiAuto"] != nil || x["slot"] != nil {
		t.Fatal("gli altri giocatori non vedono gli incantesimi di Arin")
	}

	s.a.s.Cfg.BrokerMQTT = []string{url}
	s.m.deve(200, "PATCH", "/sessioni/"+s.ses, map[string]any{"mqttRemoto": true})
	g := collegaLontano(t, url, s.m.deve(200, "POST", "/sessioni/"+s.ses+"/remoto/giocatori", map[string]any{"utente": s.idLuca})["configurazione"].(map[string]any))
	st := g.aspetta("gli incantesimi di Arin", func(x map[string]any) bool {
		return x != nil && cerca(x["vista"].(map[string]any), "Arin")["incantesimiAuto"] != nil
	})
	prof := func(i int) map[string]any {
		x := cerca(st["vista"].(map[string]any), "Arin")["incantesimiAuto"].([]any)[i].(map[string]any)
		p := x["livelli"].([]any)[0].(map[string]any)
		p["nome"], p["livello"] = x["nome"], x["livello"]
		return p
	}
	slotUsati := func() map[string]any {
		m := map[string]any{}
		for _, x := range cerca(s.vista(s.luca), "Arin")["slot"].([]any) {
			y := x.(map[string]any)
			m[itoa(int(y["livello"].(float64)))] = y["usati"]
		}
		return m
	}

	// Raggio rovente (2° livello) con due raggi: uno slot di 2° solo, anche per il secondo raggio.
	rr := prof(1)
	rr["raggi"], rr["lancio"] = 2, "lancio-remoto-01"
	tiriDecisi(t, 15, 3, 3)
	g.azione("i1", "attacco.tira", map[string]any{"combattente": s.idA, "bersaglio": s.id["Bandito"], "incantesimo": rr, "usaSlot": true, "azione": "remoto-inc-01"}, time.Now())
	if e := g.esito("i1"); e["ok"] != true || e["risultato"].(map[string]any)["attacchiRimasti"].(float64) != 1 {
		t.Fatalf("primo raggio: %v", e)
	}
	tiriDecisi(t, 15, 2, 2)
	g.azione("i2", "attacco.tira", map[string]any{"combattente": s.idA, "bersaglio": s.id["Bandito"], "incantesimo": rr, "usaSlot": true, "azione": "remoto-inc-02"}, time.Now())
	if e := g.esito("i2"); e["ok"] != true {
		t.Fatalf("secondo raggio: %v", e)
	}
	if u := slotUsati(); u["2"].(float64) != 1 || u["1"].(float64) != 0 || s.pf("Bandito") != 20 {
		t.Fatalf("uno slot di 2°, 10 danni: %v, PF %v", u, s.pf("Bandito"))
	}
	// La scheda vede lo slot usato (stato in gioco condiviso).
	var raw string
	s.a.s.DB.QueryRow(`SELECT slot_usati FROM stato_in_gioco WHERE personaggio_id = ?`, s.pg).Scan(&raw)
	if raw != `{"2":1}` {
		t.Fatalf("slot nello stato in gioco: %s", raw)
	}

	// Turno dopo: Raggio rovente di nuovo, ma lo slot di 2° è finito → rifiutato, nessuna azione usata.
	for range 2 {
		s.az(s.m, 200, "iniz.prossimo_turno", nil)
	}
	rr["lancio"] = "lancio-remoto-02"
	g.azione("i3", "attacco.tira", map[string]any{"combattente": s.idA, "bersaglio": s.id["Bandito"], "incantesimo": rr, "usaSlot": true, "azione": "remoto-inc-03"}, time.Now())
	if e := g.esito("i3"); e["ok"] != false || !strings.Contains(e["errore"].(string), "slot di 2°") {
		t.Fatalf("senza slot: %v", e)
	}
	if s.economia(s.m, "Arin")["a"] == true {
		t.Fatal("rifiutato per lo slot: l'azione resta libera")
	}
	// Fiamma sacra (trucchetto, TS): nessuno slot; il TS aspetta il Master.
	tiriDecisi(t, 4, 2)
	g.azione("i4", "incantesimo.ts", map[string]any{"combattente": s.idA, "incantesimo": prof(0), "bersagli": []string{s.id["Bandito"]}, "usaSlot": true, "azione": "remoto-inc-04"}, time.Now())
	if e := g.esito("i4"); e["ok"] != true || e["risultato"].(map[string]any)["inAttesa"] != true {
		t.Fatalf("trucchetto con TS: %v", e)
	}
	s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
	if s.pf("Bandito") != 16 {
		t.Fatalf("Fiamma sacra: %v", s.pf("Bandito"))
	}
	// Cura ferite (1° livello, azione già usata): rifiutata per l'azione, lo slot resta.
	g.azione("i5", "incantesimo.cura", map[string]any{"combattente": s.idA, "incantesimo": prof(2), "bersagli": []string{s.idA}, "usaSlot": true, "azione": "remoto-inc-05"}, time.Now())
	if e := g.esito("i5"); e["ok"] != false {
		t.Fatalf("cura con l'azione già usata: %v", e)
	}
	if u := slotUsati(); u["1"].(float64) != 0 {
		t.Fatalf("lo slot di 1° resta libero: %v", u)
	}
}
