package server

import (
	"testing"
)

// Riassunti (2026-11-09): firma del personaggio, firma cambiata dal Master, «Ottimo lavoro» → ispirazione.
func TestRiassuntiOttimoLavoro(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	l := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	m.deve(200, "POST", "/utenti/"+a.idUtente(l)+"/approva", map[string]any{"gruppi": []string{g}})
	pg := l.deve(200, "POST", "/personaggi", map[string]any{"gruppo": g, "nome": "Arin"})["id"].(string)
	l.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(l, pg), "modifiche": map[string]any{"insp": false}, "riepilogo": map[string]any{"class": "Chierico", "level": 1}})
	altro := m.deve(200, "POST", "/personaggi", map[string]any{"nome": "PNG"})["id"].(string)
	l.deve(400, "POST", "/riassunti", map[string]any{"gruppo": g, "testo": "x", "personaggio": altro})
	id := l.deve(200, "POST", "/riassunti", map[string]any{"gruppo": g, "titolo": "Borgofosco", "testo": "# Arrivo\n\nSiamo arrivati.", "personaggio": pg})["id"].(string)
	lista := l.deve(200, "GET", "/riassunti", nil)["lista"].([]any)
	if r := lista[0].(map[string]any); r["scriba"] != "Arin" || r["personaggio"] != pg {
		t.Fatalf("firma del personaggio: %v", r)
	}
	l.deve(403, "PATCH", "/riassunti/"+id+"/firma", map[string]any{"firma": "Io"})
	m.deve(200, "PATCH", "/riassunti/"+id+"/firma", map[string]any{"firma": "Arin, scriba del gruppo"})
	l.deve(403, "POST", "/riassunti/"+id+"/ottimo", map[string]any{"ottimo": true})
	r := m.deve(200, "POST", "/riassunti/"+id+"/ottimo", map[string]any{"ottimo": true})
	if r["ispirazione"] != true {
		t.Fatalf("ispirazione: %v", r)
	}
	if d := l.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any); d["insp"] != true {
		t.Fatalf("ispirazione nella scheda: %v", d["insp"])
	}
	lista = l.deve(200, "GET", "/riassunti", nil)["lista"].([]any)
	if r := lista[0].(map[string]any); r["ottimo"] != true || r["scriba"] != "Arin, scriba del gruppo" {
		t.Fatalf("riassunto: %v", r)
	}
}

// Background in testo: lo vedono giocatore e Master (che può caricarlo al suo posto); condivisione; analisi solo al
// Master con sintesi e agganci.
func TestBackgroundTesto(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	l := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	sa := a.giocatore("Sara", "333 7654321", "192.168.1.21")
	for _, c := range []*cliente{l, sa} {
		m.deve(200, "POST", "/utenti/"+a.idUtente(c)+"/approva", map[string]any{"gruppi": []string{g}})
	}
	pg := l.deve(200, "POST", "/personaggi", map[string]any{"gruppo": g, "nome": "Arin"})["id"].(string)
	testo := "# Infanzia\n\nArin è cresciuto a Daggerford con la madre Elena, sacerdotessa di Lathander.\n\n" +
		"Quando la madre fu uccisa dai banditi, Arin giurò vendetta contro il loro capo, Varek il Rosso.\n\n" +
		"Da allora porta con sé il simbolo sacro di Elena e teme di non essere degno della sua fede."
	// Il Master lo carica al posto del giocatore: resta del giocatore.
	m.deve(200, "PUT", "/personaggi/"+pg+"/background-testo", map[string]any{"titolo": "La storia di Arin", "testo": testo, "origine": "arin.pdf"})
	sa.deve(404, "PUT", "/personaggi/"+pg+"/background-testo", map[string]any{"testo": "no"})
	vede := func(c *cliente) []any { return c.deve(200, "GET", "/background-testi", nil)["lista"].([]any) }
	if l1 := vede(l); len(l1) != 1 || l1[0].(map[string]any)["mio"] != true || l1[0].(map[string]any)["agganciIA"] != nil {
		t.Fatalf("il giocatore lo vede (senza la scheda del Master): %v", l1)
	}
	if len(vede(sa)) != 0 {
		t.Fatal("gli altri non lo vedono finché non è condiviso")
	}
	mm := vede(m)[0].(map[string]any)
	// Senza IA scelta la scheda per il Master non ha agganci e lo dice (gli agganci li scrive l'IA: agganci_ia.go).
	if ag := mm["agganciIA"].(map[string]any); ag["iaScelta"] != false || ag["testo"] != nil {
		t.Fatalf("scheda per il Master senza IA: %v", ag)
	}
	l.deve(200, "PATCH", "/personaggi/"+pg+"/background-testo", map[string]any{"condiviso": []string{a.idUtente(sa)}})
	if s1 := vede(sa); len(s1) != 1 || s1[0].(map[string]any)["modificabile"] != false {
		t.Fatalf("condiviso con Sara, in sola lettura: %v", s1)
	}
	sa.deve(404, "DELETE", "/personaggi/"+pg+"/background-testo", nil)
	l.deve(200, "PATCH", "/personaggi/"+pg+"/background-testo", map[string]any{"condiviso": []string{}})
	if len(vede(sa)) != 0 {
		t.Fatal("condivisione tolta")
	}
	l.deve(200, "PATCH", "/personaggi/"+pg+"/background-testo", map[string]any{"condiviso": []string{"tutti"}})
	if len(vede(sa)) != 1 {
		t.Fatal("condiviso con tutto il gruppo")
	}
	l.deve(200, "DELETE", "/personaggi/"+pg+"/background-testo", nil)
	if len(vede(m)) != 0 {
		t.Fatal("eliminato")
	}
}
