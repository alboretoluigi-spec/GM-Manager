package server

import "testing"

func TestRiassunti(t *testing.T) {
	tv := nuovoTavolo(t)
	sess := tv.m.deve(200, "POST", "/gruppi/"+tv.g1+"/sessioni", map[string]any{"titolo": "La Tana del Lupo", "inizio": 1893456000})["id"].(string)
	altra := tv.m.deve(200, "POST", "/gruppi/"+tv.g2+"/sessioni", map[string]any{"titolo": "Altro tavolo", "inizio": 1893456000})["id"].(string)
	tv.luca.deve(400, "POST", "/riassunti", map[string]any{"gruppo": tv.g2, "testo": "x"})                    // non è il suo gruppo
	tv.luca.deve(400, "POST", "/riassunti", map[string]any{"gruppo": tv.g1, "testo": "  "})                   // vuoto
	tv.luca.deve(400, "POST", "/riassunti", map[string]any{"gruppo": tv.g1, "testo": "x", "sessione": altra}) // sessione di un altro gruppo
	cl := tv.clienteWS(tv.sara)
	cl.canali["gruppo:"+tv.g1] = true
	ultimiMessaggi(cl, "")
	id := tv.luca.deve(200, "POST", "/riassunti", map[string]any{"gruppo": tv.g1, "sessione": sess, "titolo": "Sessione 1", "testo": "Siamo arrivati a Gaster Bless.\r\nAldric ci ha accolti."})["id"].(string)
	if len(ultimiMessaggi(cl, "riassunti.aggiornati")) != 1 {
		t.Fatal("il gruppo è avvisato")
	}
	l := tv.sara.deve(200, "GET", "/riassunti", nil)["lista"].([]any)
	if len(l) != 1 {
		t.Fatal("Sara legge il riassunto del gruppo")
	}
	r := l[0].(map[string]any)
	if r["scriba"] != "Luca" || r["mio"] != nil || r["sessioneTitolo"] != "La Tana del Lupo" || r["testo"] != "Siamo arrivati a Gaster Bless.\nAldric ci ha accolti." {
		t.Fatalf("riassunto firmato dallo scriba: %v", r)
	}
	if l := tv.ugo.deve(200, "GET", "/riassunti", nil)["lista"].([]any); len(l) != 0 {
		t.Fatal("un altro gruppo non lo vede")
	}
	if l := tv.m.deve(200, "GET", "/riassunti", nil)["lista"].([]any); len(l) != 1 {
		t.Fatal("il Master lo vede")
	}
	tv.ugo.deve(404, "GET", "/riassunti?gruppo="+tv.g1, nil)
	// Solo l'autore modifica; autore o Master eliminano.
	tv.sara.deve(403, "PUT", "/riassunti/"+id, map[string]any{"testo": "cambiato"})
	tv.luca.deve(200, "PUT", "/riassunti/"+id, map[string]any{"titolo": "Sessione 1 (rivisto)", "testo": "Testo rivisto"})
	if l := tv.luca.deve(200, "GET", "/riassunti?gruppo="+tv.g1, nil)["lista"].([]any); l[0].(map[string]any)["testo"] != "Testo rivisto" || l[0].(map[string]any)["mio"] != true {
		t.Fatal("modificato dall'autore")
	}
	tv.sara.deve(403, "DELETE", "/riassunti/"+id, nil)
	tv.ugo.deve(404, "DELETE", "/riassunti/"+id, nil)
	id2 := tv.sara.deve(200, "POST", "/riassunti", map[string]any{"gruppo": tv.g1, "testo": "Il mio riassunto"})["id"].(string)
	tv.m.deve(200, "DELETE", "/riassunti/"+id2, nil)
	tv.luca.deve(200, "DELETE", "/riassunti/"+id, nil)
	if l := tv.sara.deve(200, "GET", "/riassunti", nil)["lista"].([]any); len(l) != 0 {
		t.Fatal("eliminati")
	}
}

// clienteWS per il tavolo di prova: stessa simulazione usata dai test dei dadi (dadi_test.go).
func (tv *tavolo) clienteWS(c *cliente) *Cliente { return (&scontro{a: tv.a}).clienteWS(c) }

// ws e wsOK per il tavolo di prova (stesse funzioni dei test dei dadi), usate anche dai test della Sanità.
func (tv *tavolo) ws(c *Cliente, tipo string, dati map[string]any) (any, error) {
	return (&scontro{a: tv.a}).ws(c, tipo, dati)
}

func (tv *tavolo) wsOK(c *Cliente, tipo string, dati map[string]any) any {
	tv.a.t.Helper()
	return (&scontro{a: tv.a}).wsOK(c, tipo, dati)
}
