package server

import (
	"encoding/json"
	"testing"
)

func elencoDocumenti(c *cliente) []map[string]any {
	var out []map[string]any
	for _, x := range c.deve(200, "GET", "/documenti", nil)["lista"].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

// Documenti: ogni giocatore vede solo i suoi, il Master tutti; consegna a un altro giocatore con notifica e copia nel
// Backup; il Master rinomina, sostituisce, elimina e condivide.
func TestDocumenti(t *testing.T) {
	tv := nuovoTavolo(t)
	pdf := []byte("%PDF-1.4\n% prova\n")
	png := []byte("\x89PNG\r\n\x1a\nprova")
	pgLuca := tv.luca.deve(200, "POST", "/personaggi", map[string]any{"gruppo": tv.g1, "nome": "Arin"})["id"].(string)

	// Formati: PDF, Word, immagini sì; il resto no.
	if c, _ := tv.luca.carica("/documenti", []byte("<script>alert(1)</script>"), "x.html"); c != 415 {
		t.Fatalf("formato non ammesso: %d", c)
	}
	c, corpo := tv.luca.carica("/documenti", pdf, "Mappa del tesoro.pdf")
	if c != 200 {
		t.Fatalf("caricamento: %d %s", c, corpo)
	}
	var doc map[string]any
	json.Unmarshal([]byte(corpo), &doc)
	id := doc["id"].(string)
	if c, _ := tv.luca.carica("/documenti", docxFinto(), "Diario.docx"); c != 200 {
		t.Fatal("docx")
	}
	if c, _ := tv.sara.carica("/documenti", png, "ritratto.png"); c != 200 {
		t.Fatal("png")
	}

	// Ognuno vede solo i suoi; il Master vede tutto e le cartelle dei giocatori.
	if l := elencoDocumenti(tv.luca); len(l) != 2 {
		t.Fatalf("Luca vede %d documenti", len(l))
	}
	if l := elencoDocumenti(tv.sara); len(l) != 1 || l[0]["nome"] != "ritratto.png" {
		t.Fatalf("Sara: %v", l)
	}
	if l := elencoDocumenti(tv.m); len(l) != 3 {
		t.Fatalf("il Master vede tutti i documenti: %d", len(l))
	}
	if g := tv.m.deve(200, "GET", "/documenti", nil)["giocatori"].([]any); len(g) != 3 {
		t.Fatalf("cartelle dei giocatori: %v", g)
	}
	if c, _ := tv.sara.scarica("/documenti/" + id + "/file"); c != 404 {
		t.Fatal("Sara non apre i documenti di Luca")
	}
	tv.sara.deve(404, "DELETE", "/documenti/"+id, nil)
	if c, b := tv.luca.scarica("/documenti/" + id + "/file"); c != 200 || string(b) != string(pdf) {
		t.Fatal("Luca apre il suo PDF")
	}

	// Consegna a Sara (stesso gruppo), non a Ugo (altro gruppo): Sara riceve la copia e la notifica, Luca la tiene nel Backup.
	tv.luca.deve(400, "POST", "/documenti/"+id+"/consegna", map[string]any{"a": []string{tv.a.idUtente(tv.ugo)}})
	cs := tv.clienteWS(tv.sara)
	ultimiMessaggi(cs, "")
	r := tv.luca.deve(200, "POST", "/documenti/"+id+"/consegna", map[string]any{"a": []string{tv.a.idUtente(tv.sara)}, "personaggio": pgLuca})
	if r["da"] != "Arin" {
		t.Fatalf("consegnato da Arin: %v", r)
	}
	msg := ultimiMessaggi(cs, "documento.consegnato")
	if len(msg) != 1 || !json.Valid([]byte(msg[0])) || !contieneTesto(msg[0], "Arin ti ha consegnato un documento") {
		t.Fatalf("notifica a Sara: %v", msg)
	}
	ls := elencoDocumenti(tv.sara)
	var ricevuto map[string]any
	for _, d := range ls {
		if d["nome"] == "Mappa del tesoro.pdf" {
			ricevuto = d
		}
	}
	if ricevuto == nil || ricevuto["nuovo"] != true || ricevuto["daPersonaggio"] != "Arin" || ricevuto["cartella"] != "" {
		t.Fatalf("documento ricevuto: %v", ls)
	}
	if c, b := tv.sara.scarica("/documenti/" + ricevuto["id"].(string) + "/file"); c != 200 || string(b) != string(pdf) {
		t.Fatal("Sara apre il documento ricevuto")
	}
	tv.sara.deve(200, "POST", "/documenti/"+ricevuto["id"].(string)+"/visto", nil)
	var backup map[string]any
	for _, d := range elencoDocumenti(tv.luca) {
		if d["id"] == id {
			backup = d
		}
	}
	if backup == nil || backup["cartella"] != "backup" || backup["aUtente"] != "Sara" {
		t.Fatalf("copia nel Backup di Luca: %v", backup)
	}
	tv.luca.deve(400, "POST", "/documenti/"+id+"/consegna", map[string]any{"a": []string{tv.a.idUtente(tv.sara)}})

	// Il Master rinomina, sostituisce, condivide con due giocatori, elimina.
	idSara := ls[0]["id"].(string)
	for _, d := range ls {
		if d["nome"] == "ritratto.png" {
			idSara = d["id"].(string)
		}
	}
	tv.m.deve(200, "PATCH", "/documenti/"+idSara, map[string]any{"nome": "Ritratto di Elara"})
	if c, _ := tv.m.carica("/documenti/"+idSara+"/file", pdf, "nuovo.pdf"); c != 200 {
		t.Fatal("il Master sostituisce il file")
	}
	for _, d := range elencoDocumenti(tv.sara) {
		if d["id"] == idSara && (d["nome"] != "Ritratto di Elara" || d["tipo"] != "pdf") {
			t.Fatalf("modificato dal Master: %v", d)
		}
	}
	c, corpo = tv.m.carica("/documenti", pdf, "Lettera di Borin.pdf")
	json.Unmarshal([]byte(corpo), &doc)
	tv.m.deve(200, "POST", "/documenti/"+doc["id"].(string)+"/consegna", map[string]any{"a": []string{tv.a.idUtente(tv.luca), tv.a.idUtente(tv.ugo)}})
	for _, g := range []*cliente{tv.luca, tv.ugo} {
		trovato := false
		for _, d := range elencoDocumenti(g) {
			trovato = trovato || d["nome"] == "Lettera di Borin.pdf" && d["daPersonaggio"] == "Il Master"
		}
		if !trovato {
			t.Fatal("documento condiviso dal Master")
		}
	}
	tv.m.deve(200, "DELETE", "/documenti/"+idSara, nil)
	if c, _ := tv.sara.scarica("/documenti/" + idSara + "/file"); c != 404 {
		t.Fatal("eliminato dal Master")
	}
}

func contieneTesto(s, t string) bool {
	var m map[string]any
	if json.Unmarshal([]byte(s), &m) != nil {
		return false
	}
	d, _ := m["dati"].(map[string]any)
	return d != nil && d["testo"] == t
}
