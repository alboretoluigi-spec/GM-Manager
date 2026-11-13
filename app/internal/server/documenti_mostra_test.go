package server

import (
	"encoding/json"
	"testing"
)

// «Mostra» un documento (2026-11-09): i compagni scelti lo guardano (il file si apre), senza riceverne una copia;
// chi non è stato scelto o non è compagno non lo vede.
func TestMostraDocumento(t *testing.T) {
	tv := nuovoTavolo(t)
	c, corpo := tv.luca.carica("/documenti", []byte("%PDF-1.4\n% prova\n"), "Lettera.pdf")
	if c != 200 {
		t.Fatalf("caricamento: %d", c)
	}
	var doc map[string]any
	json.Unmarshal([]byte(corpo), &doc)
	id := doc["id"].(string)
	if c, _ := tv.sara.scarica("/documenti/" + id + "/file"); c != 404 {
		t.Fatal("prima di mostrarlo Sara non lo vede")
	}
	tv.luca.deve(400, "POST", "/documenti/"+id+"/mostra", map[string]any{"a": []string{tv.a.idUtente(tv.ugo)}})
	cs := tv.clienteWS(tv.sara)
	ultimiMessaggi(cs, "")
	r := tv.luca.deve(200, "POST", "/documenti/"+id+"/mostra", map[string]any{"a": []string{"tutti"}})
	if r["mostrato"] != 1.0 {
		t.Fatalf("mostrato a: %v", r)
	}
	if m := ultimiMessaggi(cs, "documento.mostrato"); len(m) != 1 {
		t.Fatalf("avviso a Sara: %v", m)
	}
	if c, _ := tv.sara.scarica("/documenti/" + id + "/file"); c != 200 {
		t.Fatal("Sara lo guarda")
	}
	if c, _ := tv.ugo.scarica("/documenti/" + id + "/file"); c != 404 {
		t.Fatal("Ugo (altro gruppo) no")
	}
	if l := elencoDocumenti(tv.sara); len(l) != 0 {
		t.Fatalf("Sara non ne riceve una copia: %v", l)
	}
}
