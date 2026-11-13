package server

import (
	"os"
	"testing"
)

// Mappa tra una sessione e l'altra (2026-11-08): il giocatore del gruppo la consulta, senza modificare nulla;
// chi non è del gruppo non la vede.
func TestMappaConsultazione(t *testing.T) {
	a := nuovoAmbiente(t)
	a.s.Web = os.DirFS("testdata/web") // rete stradale vera (mask.png)
	m := a.master()
	g := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	g2 := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Venerdì"})["id"].(string)
	l := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	m.deve(200, "POST", "/utenti/"+a.idUtente(l)+"/approva", map[string]any{"gruppi": []string{g}})
	q := "?sessione=g:" + g
	l.deve(200, "GET", "/mappa/me"+q, nil)
	l.deve(200, "GET", "/mappa/state"+q, nil)
	l.deve(200, "POST", "/mappa/strada"+q, map[string]any{"op": "aggancia", "x": 100, "y": 100, "r": 5})
	l.deve(403, "POST", "/mappa/party"+q, map[string]any{"azione": "posiziona", "x": 500, "y": 500})
	l.deve(404, "GET", "/mappa/state?sessione=g:"+g2, nil)
	// Il Master in preparazione può ancora modificare.
	m.deve(200, "POST", "/mappa/party"+q, map[string]any{"azione": "posiziona", "x": 500, "y": 500})
}

// Segnalino del gruppo (2026-11-08): dentro di default tutti i partecipanti della sessione (anche chi non ha scelto
// il personaggio), fuori chi ha detto «Non posso più venire»; la divisione vale solo nella sua sessione.
func TestSegnalinoTuttiIPresenti(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	l := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	sa := a.giocatore("Sara", "333 7654321", "192.168.1.21")
	for _, c := range []*cliente{l, sa} {
		m.deve(200, "POST", "/utenti/"+a.idUtente(c)+"/approva", map[string]any{"gruppi": []string{g}})
	}
	l.deve(200, "POST", "/personaggi", map[string]any{"gruppo": g, "nome": "Arin"})
	pgS := sa.deve(200, "POST", "/personaggi", map[string]any{"gruppo": g, "nome": "Bryn"})["id"].(string)
	ses := m.deve(200, "POST", "/gruppi/"+g+"/sessioni", map[string]any{"titolo": "Uno", "inizio": 1893456000})["id"].(string)
	nomi := func(sessione string) [][]string {
		st := l.deve(200, "GET", "/mappa/state?sessione="+sessione, nil)
		var out [][]string
		p, _ := st["party"].(map[string]any)
		if p == nil {
			return nil
		}
		for _, x := range p["segnalini"].([]any) {
			var n []string
			for _, q := range x.(map[string]any)["pg"].([]any) {
				n = append(n, q.(map[string]any)["nome"].(string))
			}
			out = append(out, n)
		}
		return out
	}
	m.deve(200, "POST", "/mappa/party?sessione="+ses, map[string]any{"azione": "posiziona", "x": 500, "y": 500})
	if n := nomi(ses); len(n) != 1 || len(n[0]) != 2 {
		t.Fatalf("tutti nel segnalino anche senza aver scelto il personaggio: %v", n)
	}
	sa.deve(200, "POST", "/sessioni/"+ses+"/conferma", map[string]any{"confermato": false, "personaggio": pgS})
	if n := nomi(ses); len(n[0]) != 1 || n[0][0] != "Arin" {
		t.Fatalf("chi non viene esce dal segnalino: %v", n)
	}
	sa.deve(200, "POST", "/sessioni/"+ses+"/conferma", map[string]any{"confermato": true, "personaggio": pgS})
	m.deve(200, "POST", "/mappa/party?sessione="+ses, map[string]any{"azione": "dividi", "id": "g", "x": 600, "y": 600, "pg": []string{pgS}})
	if n := nomi(ses); len(n) != 2 {
		t.Fatalf("diviso: %v", n)
	}
	ses2 := m.deve(200, "POST", "/gruppi/"+g+"/sessioni", map[string]any{"titolo": "Due", "inizio": 1893556000})["id"].(string)
	if n := nomi(ses2); len(n) != 1 || len(n[0]) != 2 {
		t.Fatalf("nella sessione nuova il gruppo riparte unito: %v", n)
	}
}
