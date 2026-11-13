package server

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// Analisi dei regolamenti (passo L5): l'analizzatore per parole chiave trova le sezioni nelle categorie giuste con la
// fonte; ripetere l'analisi non duplica; niente è approvato da solo; il Master corregge, approva, rifiuta e scrive
// proposte; solo le approvate escono per la campagna; l'IA non è disponibile e non è obbligatoria.
func TestAnalisiRegolamenti(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	testo := "Capitolo 1\n\nCreazione del personaggio\n\nScegli un concetto, poi distribuisci i punti tra le caratteristiche.\n\n" +
		"Iniziativa\n\nAll'inizio dello scontro ognuno tira un dado e aggiunge la sua prontezza.\n\n" +
		"Conditions\n\nBlinded: the creature cannot see.\n\n" +
		"Armi e armature\n\nLe armi pesanti richiedono due mani.\n\n" +
		"Incantesimi\n\nOgni incantesimo costa punti di energia.\n\n" +
		"Una sezione qualsiasi\n\nTesto che non riguarda nessuna categoria in particolare.\n"
	cod, _, b := m.grezzo("POST", "/regolamenti/miei?"+url.Values{"file": {"regole del gioco.txt"}}.Encode(), []byte(testo), "application/octet-stream")
	var doc map[string]any
	json.Unmarshal(b, &doc)
	if cod != 200 {
		t.Fatalf("importazione: %d %s", cod, b)
	}
	id := doc["id"].(string)
	r := m.deve(200, "POST", "/regolamenti/analisi/avvia", map[string]any{"documento": id})
	if r["nuove"].(float64) < 5 {
		t.Fatalf("analisi: %v", r)
	}
	el := m.deve(200, "GET", "/regolamenti/analisi?documento="+id, nil)
	cat := map[string]map[string]any{}
	for _, x := range el["proposte"].([]any) {
		p := x.(map[string]any)
		cat[p["categoria"].(string)] = p
		if p["stato"] != "proposta" || p["origine"] != "parole_chiave" || p["documentoTitolo"] != "regole del gioco" || p["estratto"] == "" {
			t.Fatalf("proposta senza fonte o già decisa: %v", p)
		}
	}
	for _, c := range []string{"creazione", "iniziativa", "condizioni", "armi_armature", "magie"} {
		if cat[c] == nil {
			t.Fatalf("categoria %s non trovata: %v", c, cat)
		}
	}
	if !strings.Contains(cat["iniziativa"]["estratto"].(string), "tira un dado") {
		t.Fatalf("estratto dell'iniziativa: %v", cat["iniziativa"])
	}
	if len(el["categorie"].([]any)) != 13 {
		t.Fatalf("categorie: %v", el["categorie"])
	}
	ia := false
	for _, x := range el["analizzatori"].([]any) {
		if x.(map[string]any)["id"] == "ia" && x.(map[string]any)["disponibile"] == false {
			ia = true
		}
	}
	if !ia {
		t.Fatal("l'analizzatore con l'IA deve risultare non disponibile")
	}
	if r := m.deve(200, "POST", "/regolamenti/analisi/avvia", map[string]any{"documento": id}); r["nuove"] != 0.0 || r["giaPresenti"].(float64) < 5 {
		t.Fatalf("seconda analisi: %v", r)
	}
	m.deve(409, "POST", "/regolamenti/analisi/avvia", map[string]any{"documento": id, "analizzatore": "ia"})
	m.deve(400, "POST", "/regolamenti/analisi/avvia", map[string]any{"documento": id, "analizzatore": "boh"})
	m.deve(404, "POST", "/regolamenti/analisi/avvia", map[string]any{"documento": "inesistente"})

	// campagna che usa il documento; nessuna regola approvata finché il Master non decide
	camp := m.deve(200, "POST", "/campagne", map[string]any{"nome": "Prova"})["id"].(string)
	m.deve(200, "PUT", "/campagne/"+camp+"/documenti", map[string]any{"documento": id, "ruolo": "supplementare", "presente": true})
	if n := len(m.deve(200, "GET", "/campagne/"+camp+"/regole", nil)["regole"].([]any)); n != 0 {
		t.Fatalf("regole approvate senza decisione del Master: %d", n)
	}
	ini, cond := cat["iniziativa"]["id"].(string), cat["condizioni"]["id"].(string)
	p := m.deve(200, "PATCH", "/regolamenti/analisi/"+ini, map[string]any{"testo": "Ognuno tira 1d10 + prontezza; si agisce dal più alto.", "stato": "approvata"})
	if p["stato"] != "approvata" || p["decisoIl"] == nil || p["testo"] == "" {
		t.Fatalf("approvazione: %v", p)
	}
	m.deve(200, "PATCH", "/regolamenti/analisi/"+cond, map[string]any{"stato": "rifiutata", "nota": "Usiamo le nostre condizioni."})
	m.deve(400, "PATCH", "/regolamenti/analisi/"+ini, map[string]any{"stato": "boh"})
	m.deve(400, "PATCH", "/regolamenti/analisi/"+ini, map[string]any{"categoria": "boh"})
	m.deve(404, "PATCH", "/regolamenti/analisi/inesistente", map[string]any{"stato": "approvata"})
	// proposta scritta dal Master: anche lei parte come proposta
	man := m.deve(200, "POST", "/regolamenti/analisi", map[string]any{"categoria": "speciali", "nome": "Riposo in zona pericolosa", "testo": "Nessun recupero completo.", "documento": id, "pagina": 12, "campagna": camp})
	if man["stato"] != "proposta" || man["origine"] != "manuale" || man["pagina"] != 12.0 {
		t.Fatalf("proposta manuale: %v", man)
	}
	m.deve(400, "POST", "/regolamenti/analisi", map[string]any{"categoria": "speciali"})
	m.deve(200, "PATCH", "/regolamenti/analisi/"+man["id"].(string), map[string]any{"stato": "approvata"})
	reg := m.deve(200, "GET", "/campagne/"+camp+"/regole", nil)["regole"].([]any)
	nomi := []string{}
	for _, x := range reg {
		nomi = append(nomi, x.(map[string]any)["nome"].(string))
	}
	if len(reg) != 2 || !strings.Contains(strings.Join(nomi, "|"), "Iniziativa") || !strings.Contains(strings.Join(nomi, "|"), "Riposo in zona pericolosa") {
		t.Fatalf("regole approvate della campagna: %v", nomi)
	}
	// filtri
	if n := len(m.deve(200, "GET", "/regolamenti/analisi?stato=rifiutata", nil)["proposte"].([]any)); n != 1 {
		t.Fatalf("filtro stato: %d", n)
	}
	// documento eliminato: le proposte restano con il titolo della fonte
	m.deve(200, "DELETE", "/regolamenti/miei/"+id, nil)
	for _, x := range m.deve(200, "GET", "/regolamenti/analisi", nil)["proposte"].([]any) {
		if x.(map[string]any)["documentoTitolo"] != "regole del gioco" || x.(map[string]any)["documento"] != nil {
			t.Fatalf("dopo l'eliminazione del documento: %v", x)
		}
	}
	m.deve(200, "DELETE", "/regolamenti/analisi/"+cond, nil)
	m.deve(404, "DELETE", "/regolamenti/analisi/"+cond, nil)
	a.giocatore("Luca", "333 1234567", "192.168.1.20").deve(403, "GET", "/regolamenti/analisi", nil)
}
