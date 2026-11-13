package server

import (
	"testing"
)

func moneteDi(c *cliente, pg string) map[string]any {
	d := c.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
	m, _ := d["coins"].(map[string]any)
	return m
}

// Finanze: la modifica manuale del giocatore aspetta il Master; acquisti scalati (gratis se li assegna il Master);
// monete insufficienti con il codice per l'immagine «povero»; trasferimenti; registro.
func TestFinanze(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	altro := tv.nuovoPG(tv.sara, tv.g1)

	// Il giocatore scrive 50 mo: la scheda non cambia, il Master riceve la richiesta
	cm := tv.clienteWS(tv.m)
	ultimiMessaggi(cm, "")
	r := tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"coins.MO": "50", "f.nome": "Arin"}})
	if r["monetePendenti"] != true {
		t.Fatalf("modifica delle monete in attesa: %v", r)
	}
	if m := moneteDi(tv.luca, pg); m != nil && m["MO"] == "50" {
		t.Fatal("le monete non cambiano prima dell'autorizzazione")
	}
	if len(ultimiMessaggi(cm, "finanze.richiesta")) != 1 {
		t.Fatal("il Master riceve l'avviso della richiesta")
	}
	f := tv.m.deve(200, "GET", "/finanze", nil)
	ric := f["richieste"].([]any)
	if len(ric) != 1 || ric[0].(map[string]any)["dopo"] != "50 mo" {
		t.Fatalf("richiesta: %v", ric)
	}
	if c, _ := tv.luca.chiama("POST", "/finanze-richieste/"+ric[0].(map[string]any)["id"].(string), map[string]any{"autorizza": true}); c != 403 {
		t.Fatal("solo il Master autorizza")
	}
	// Rifiutata: niente cambia. Poi una nuova richiesta autorizzata: 50 mo.
	tv.m.deve(200, "POST", "/finanze-richieste/"+ric[0].(map[string]any)["id"].(string), map[string]any{"autorizza": false})
	if m := moneteDi(tv.luca, pg); m != nil && m["MO"] == "50" {
		t.Fatal("rifiutata")
	}
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"coins.MO": "50"}})
	id := tv.m.deve(200, "GET", "/finanze", nil)["richieste"].([]any)[0].(map[string]any)["id"].(string)
	tv.m.deve(200, "POST", "/finanze-richieste/"+id, map[string]any{"autorizza": true})
	if moneteDi(tv.luca, pg)["MO"] != "50" {
		t.Fatal("autorizzata: 50 mo")
	}
	// Il Master cambia le monete dalla scheda: subito, senza richiesta
	tv.m.deve(200, "PATCH", "/personaggi/"+altro+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"coins.MO": "10"}})
	if moneteDi(tv.m, altro)["MO"] != "10" {
		t.Fatal("il Master modifica direttamente")
	}

	// Acquisto dall'elenco: 2 × Torcia (1 mr) + Spada lunga (15 mo)
	tv.luca.deve(200, "POST", "/finanze/"+pg+"/acquista", map[string]any{"nome": "Spada lunga", "prezzo": "15 mo", "quantita": 1})
	tv.luca.deve(200, "POST", "/finanze/"+pg+"/acquista", map[string]any{"nome": "Torcia", "prezzo": "1 mr", "quantita": 2})
	tv.luca.deve(200, "POST", "/finanze/"+pg+"/acquista", map[string]any{"nome": "Torcia", "prezzo": "1 mr", "quantita": 1})
	d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
	m := d["coins"].(map[string]any)
	if m["MO"] != "34" || m["MA"] != "9" || m["MR"] != "7" || d["eq"] != "1 × Spada lunga\n3 × Torcia" {
		t.Fatalf("acquisti: monete %v, equipaggiamento %q", m, d["eq"])
	}
	// Monete insufficienti: codice per la pagina, niente cambia
	c, out := tv.luca.chiama("POST", "/finanze/"+pg+"/acquista", map[string]any{"nome": "Armatura a piastre", "prezzo": "1500 mo", "quantita": 1})
	if c != 409 || out["codice"] != "monete_insufficienti" {
		t.Fatalf("monete insufficienti: %d %v", c, out)
	}
	// Assegnato dal Master: gratis
	tv.m.deve(200, "POST", "/finanze/"+pg+"/acquista", map[string]any{"nome": "Armatura a piastre", "prezzo": "1500 mo", "quantita": 1})
	if moneteDi(tv.luca, pg)["MO"] != "34" {
		t.Fatal("l'oggetto assegnato dal Master non costa")
	}
	// Un altro giocatore non usa le monete di Arin
	if c, _ := tv.sara.chiama("POST", "/finanze/"+pg+"/trasferisci", map[string]any{"a": altro, "rame": 100}); c != 404 {
		t.Fatal("solo il proprietario")
	}

	// Trasferimento: 4 mo a Sara; poi tutto
	tv.luca.deve(200, "POST", "/finanze/"+pg+"/trasferisci", map[string]any{"a": altro, "rame": 400})
	if moneteDi(tv.m, altro)["MO"] != "14" {
		t.Fatalf("Sara riceve 4 mo: %v", moneteDi(tv.m, altro))
	}
	if c, out := tv.luca.chiama("POST", "/finanze/"+pg+"/trasferisci", map[string]any{"a": altro, "rame": 100000}); c != 409 || out["codice"] != "monete_insufficienti" {
		t.Fatal("non si trasferisce più di quanto si ha")
	}
	tv.luca.deve(200, "POST", "/finanze/"+pg+"/trasferisci", map[string]any{"a": altro, "tutto": true})
	if tot := rameDi(map[string]int{"MO": atoi(moneteDi(tv.m, altro)["MO"]), "MA": atoi(moneteDi(tv.m, altro)["MA"]), "MR": atoi(moneteDi(tv.m, altro)["MR"])}); tot != 1400+3097 {
		t.Fatalf("Sara riceve tutto: %d rame", tot)
	}
	reg := tv.luca.deve(200, "GET", "/finanze", nil)["registro"].([]any)
	if len(reg) < 6 {
		t.Fatalf("registro dei movimenti: %d voci", len(reg))
	}
	// Il Master dà o toglie monete direttamente
	tv.m.deve(200, "POST", "/finanze/"+pg+"/master", map[string]any{"rame": 1000, "motivo": "taglia su un lupo mannaro"})
	if moneteDi(tv.luca, pg)["MO"] != "10" {
		t.Fatal("il Master dà 10 mo")
	}
}

func atoi(v any) int {
	s, _ := v.(string)
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// Ricompense: un mostro con GS sconfitto (0 PF) dà GS × 10 mo divisi tra i personaggi dello scontro, una volta sola.
func TestRicompensaMostro(t *testing.T) {
	sc := nuovoScontro(t)
	v := sc.vista(sc.m)
	if cerca(v, "Arin") == nil {
		sc.az(sc.m, 200, "iniz.aggiungi_pg", map[string]any{"personaggio": sc.pg})
	}
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Lupo crudele", "quanti": 1, "ca": 14, "pf": 37, "gs": 1})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Ratto", "quanti": 1, "ca": 10, "pf": 1, "gs": 0})
	lupo := cerca(sc.vista(sc.m), "Lupo crudele")["id"].(string)
	ratto := cerca(sc.vista(sc.m), "Ratto")["id"].(string)
	sc.az(sc.m, 200, "mostro.danno", map[string]any{"combattente": lupo, "valore": 30})
	if m := moneteDi(sc.luca, sc.pg); m != nil && m["MO"] != "" && m["MO"] != nil {
		t.Fatalf("niente ricompensa finché il mostro è in piedi: %v", m)
	}
	sc.az(sc.m, 200, "mostro.danno", map[string]any{"combattente": lupo, "valore": 30})
	if moneteDi(sc.luca, sc.pg)["MO"] != "10" {
		t.Fatalf("GS 1 × 10 / 1 personaggio = 10 mo: %v", moneteDi(sc.luca, sc.pg))
	}
	// curato e sconfitto di nuovo: niente seconda ricompensa; GS 0: niente
	sc.az(sc.m, 200, "mostro.cura", map[string]any{"combattente": lupo, "valore": 5})
	sc.az(sc.m, 200, "mostro.danno", map[string]any{"combattente": lupo, "valore": 5})
	sc.az(sc.m, 200, "mostro.danno", map[string]any{"combattente": ratto, "valore": 5})
	if moneteDi(sc.luca, sc.pg)["MO"] != "10" {
		t.Fatalf("una ricompensa sola: %v", moneteDi(sc.luca, sc.pg))
	}
	if c, _ := sc.m.chiama("POST", "/sessioni/"+sc.ses+"/iniziativa", map[string]any{"tipo": "iniz.aggiungi_mostri", "dati": map[string]any{"nome": "X", "quanti": 1, "ca": 10, "pf": 1, "gs": 31}}); c != 400 {
		t.Fatal("GS fuori scala")
	}
}
