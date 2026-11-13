package server

import (
	"encoding/json"
	"net/url"
	"testing"
	"testing/fstest"
)

// Campagne (passo L4): la «Campagna principale» esiste e contiene i gruppi; i gruppi nuovi ci finiscono da soli;
// sistema, edizione, regolamento, regole e impostazioni; lo stesso documento in più campagne; spostamento dei gruppi;
// eliminazione solo senza gruppi e mai dell'ultima; un documento eliminato sparisce dalle campagne; solo il Master;
// tutto resta dopo il riavvio.
func TestCampagne(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g1 := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	elenco := func() []any { return m.deve(200, "GET", "/campagne", nil)["campagne"].([]any) }
	cs := elenco()
	if len(cs) != 1 || cs[0].(map[string]any)["nome"] != "Campagna principale" || len(cs[0].(map[string]any)["gruppi"].([]any)) != 1 {
		t.Fatalf("campagna principale con il gruppo nuovo: %v", cs)
	}
	carica := func(nome, testo string) string {
		t.Helper()
		cod, _, b := m.grezzo("POST", "/regolamenti/miei?"+url.Values{"file": {nome}}.Encode(), []byte(testo), "application/octet-stream")
		var out map[string]any
		json.Unmarshal(b, &out)
		if cod != 200 {
			t.Fatalf("%s: %d %s", nome, cod, b)
		}
		return out["id"].(string)
	}
	reg := carica("regole.txt", "Il regolamento base del gioco.")
	sup := carica("supplemento.txt", "Regole aggiuntive per i viaggi.")
	sch := carica("scheda.txt", "Scheda del personaggio personalizzata.")

	nuova := m.deve(200, "POST", "/campagne", map[string]any{"nome": "Le terre del nord", "sistema": "Gioco di prova", "edizione": "2", "descrizione": "Seconda campagna"})["id"].(string)
	m.deve(409, "POST", "/campagne", map[string]any{"nome": "Le terre del nord"})
	m.deve(400, "POST", "/campagne", map[string]any{"nome": " "})
	m.deve(400, "POST", "/campagne", map[string]any{})
	c := m.deve(200, "PATCH", "/campagne/"+nuova, map[string]any{"regolamento": reg, "regoleMaster": "Il riposo lungo richiede un luogo sicuro.",
		"impostazioni": map[string]any{"dadoIniziativa": "d20", "salute": "punti ferita", "sanita": false}})
	if c["regolamento"] != reg || c["sistema"] != "Gioco di prova" || c["impostazioni"].(map[string]any)["dadoIniziativa"] != "d20" {
		t.Fatalf("modifica: %v", c)
	}
	m.deve(400, "PATCH", "/campagne/"+nuova, map[string]any{"regolamento": "inesistente"})
	m.deve(400, "PATCH", "/campagne/"+nuova, map[string]any{"impostazioni": []int{1, 2}})
	m.deve(404, "PATCH", "/campagne/inesistente", map[string]any{"nome": "x"})

	// lo stesso documento in due campagne, con ruoli diversi
	for _, x := range []struct{ c, d, r string }{{nuova, sup, "supplementare"}, {"principale", sup, "supplementare"}, {nuova, sch, "scheda"}} {
		m.deve(200, "PUT", "/campagne/"+x.c+"/documenti", map[string]any{"documento": x.d, "ruolo": x.r, "presente": true})
	}
	m.deve(200, "PUT", "/campagne/"+nuova+"/documenti", map[string]any{"documento": sup, "ruolo": "supplementare", "presente": true}) // già presente: nessun doppione
	m.deve(400, "PUT", "/campagne/"+nuova+"/documenti", map[string]any{"documento": sup, "ruolo": "boh", "presente": true})
	m.deve(404, "PUT", "/campagne/"+nuova+"/documenti", map[string]any{"documento": "inesistente", "ruolo": "scheda", "presente": true})
	per := map[string]map[string]any{}
	for _, x := range elenco() {
		per[x.(map[string]any)["id"].(string)] = x.(map[string]any)
	}
	if len(per[nuova]["supplementari"].([]any)) != 1 || len(per[nuova]["schede"].([]any)) != 1 || len(per["principale"]["supplementari"].([]any)) != 1 {
		t.Fatalf("documenti delle campagne: %v", per)
	}
	usi := map[string]int{}
	for _, d := range m.deve(200, "GET", "/regolamenti/miei", nil)["documenti"].([]any) {
		usi[d.(map[string]any)["id"].(string)] = len(d.(map[string]any)["campagne"].([]any))
	}
	if usi[sup] != 2 || usi[reg] != 1 || usi[sch] != 1 {
		t.Fatalf("campagne dei documenti in My Rulebooks: %v", usi)
	}
	m.deve(200, "PUT", "/campagne/principale/documenti", map[string]any{"documento": sup, "ruolo": "supplementare", "presente": false})

	// gruppi: spostamento; eliminazione solo senza gruppi; mai l'ultima
	m.deve(409, "DELETE", "/campagne/principale", nil)
	m.deve(200, "PUT", "/gruppi/"+g1+"/campagna", map[string]any{"campagna": nuova})
	m.deve(404, "PUT", "/gruppi/"+g1+"/campagna", map[string]any{"campagna": "inesistente"})
	m.deve(404, "PUT", "/gruppi/inesistente/campagna", map[string]any{"campagna": nuova})
	m.deve(200, "DELETE", "/campagne/principale", nil)
	m.deve(409, "DELETE", "/campagne/"+nuova, nil) // ha il gruppo
	g2 := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Venerdì"})["id"].(string)
	cs = elenco()
	if len(cs) != 1 || len(cs[0].(map[string]any)["gruppi"].([]any)) != 2 {
		t.Fatalf("il gruppo nuovo va nella prima campagna: %v %s", cs, g2)
	}

	// un documento eliminato sparisce dalle campagne (anche come regolamento)
	m.deve(200, "DELETE", "/regolamenti/miei/"+reg, nil)
	m.deve(200, "DELETE", "/regolamenti/miei/"+sch, nil)
	c = elenco()[0].(map[string]any)
	if c["regolamento"] != nil || len(c["schede"].([]any)) != 0 || len(c["supplementari"].([]any)) != 1 {
		t.Fatalf("dopo l'eliminazione dei documenti: %v", c)
	}

	g := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	g.deve(403, "GET", "/campagne", nil)
	g.deve(403, "POST", "/campagne", map[string]any{"nome": "x"})

	cfg := a.s.Cfg
	a.s.Chiudi()
	s2, err := Nuovo(cfg, fstest.MapFS{"index.html": {Data: []byte("x")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Chiudi()
	var n, gr int
	var regole string
	s2.DB.QueryRow(`SELECT COUNT(*), MAX(regole_master) FROM campagne`).Scan(&n, &regole)
	s2.DB.QueryRow(`SELECT COUNT(*) FROM gruppi WHERE campagna_id = ?`, nuova).Scan(&gr)
	if n != 1 || regole != "Il riposo lungo richiede un luogo sicuro." || gr != 2 {
		t.Fatalf("dopo il riavvio: %d campagne, %d gruppi, regole %q", n, gr, regole)
	}
}
