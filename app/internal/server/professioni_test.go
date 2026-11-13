package server

import (
	"testing"
)

// Professioni: solo chi ha competenza e strumento (o il Master); raccolta, formule sbloccate dal Master, creazione
// riuscita o fallita (con e senza rimborso), uso delle dosi, estrazione con la fiala, monete tolte dalla scheda,
// prodotti nell'equipaggiamento, scadenza degli ingredienti.
func TestProfessioni(t *testing.T) {
	tv := nuovoTavolo(t)
	vero := tiraDado
	defer func() { tiraDado = vero }()
	pg := tv.nuovoPG(tv.luca, tv.g1)
	// livello 5 (BC +3), Intelligenza 16 (+3), Saggezza 12 (+1), competenza in Natura; kit da avvelenatore ed erborista pronti
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1,
		"modifiche": map[string]any{"coins.MO": "200", "eq": "Kit da avvelenatore\nBorsa da erborista"},
		"riepilogo": map[string]any{"name": "Helm", "level": 5, "abilities": map[string]any{"FOR": 10, "DES": 16, "COS": 12, "INT": 16, "SAG": 12, "CAR": 10},
			"tools": []string{"avvelenatore", "erborista", "inventato!"}, "skillProfs": []string{"Natura", "Furtività"}}})
	// le monete scritte dal giocatore diventano una richiesta: il Master la autorizza
	ric := tv.m.deve(200, "GET", "/finanze", nil)["richieste"].([]any)
	if len(ric) != 1 {
		t.Fatalf("richiesta di modifica delle monete: %v", ric)
	}
	tv.m.deve(200, "POST", "/finanze-richieste/"+ric[0].(map[string]any)["id"].(string), map[string]any{"autorizza": true})
	senza := tv.nuovoPG(tv.luca, tv.g1)
	base := "/professioni/" + pg + "/avvelenatore"
	moneteMO := func() string {
		d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
		return d["coins"].(map[string]any)["MO"].(string)
	}
	quantita := func(tipo, voce string) int {
		n := 0
		for _, x := range tv.luca.deve(200, "GET", base, nil)["inventario"].([]any) {
			v := x.(map[string]any)
			if v["tipo"] == tipo && v["voce"] == voce {
				n += int(v["quantita"].(float64))
			}
		}
		return n
	}

	// Permessi
	if c, _ := tv.sara.chiama("GET", base, nil); c != 404 {
		t.Fatal("un altro giocatore non vede la professione")
	}
	if c, o := tv.luca.chiama("GET", "/professioni/"+senza+"/avvelenatore", nil); c != 403 {
		t.Fatalf("senza competenza e strumento la professione non c'è: %d %v", c, o)
	}
	tv.m.deve(200, "GET", "/professioni/"+senza+"/avvelenatore", nil) // il Master le vede sempre tutte
	el := tv.luca.deve(200, "GET", "/professioni", nil)["personaggi"].([]any)
	if len(el) != 1 || len(el[0].(map[string]any)["professioni"].([]any)) != 2 {
		t.Fatalf("elenco del giocatore: solo il personaggio con professioni, senza id inventati: %v", el)
	}
	if n := len(tv.m.deve(200, "GET", "/professioni", nil)["personaggi"].([]any)); n != 2 {
		t.Fatalf("il Master vede tutti i personaggi: %d", n)
	}

	// Raccolta: Natura (+3 Int, +3 competenza). 12 + 6 = 18 contro CD 15: 1 lotto. 20 + 6 = 26 (≥ 25): 2 lotti.
	tiraDado = sequenza(12)
	r := tv.luca.deve(200, "POST", base+"/raccolta", map[string]any{"ingrediente": "felce_ombra", "modo": "viaggio", "cd": 15, "abilita": "Natura"})
	if r["lotti"] != 1.0 || r["tiro"].(map[string]any)["totale"] != 18.0 {
		t.Fatalf("raccolta: %v", r)
	}
	if c, _ := tv.luca.chiama("POST", base+"/raccolta", map[string]any{"ingrediente": "lume_cripta", "modo": "viaggio", "cd": 15, "abilita": "Natura"}); c != 409 {
		t.Fatal("in viaggio una sola raccolta al giorno")
	}
	tiraDado = sequenza(20)
	r = tv.luca.deve(200, "POST", base+"/raccolta", map[string]any{"ingrediente": "lume_cripta", "modo": "dedicata", "cd": 15, "abilita": "Natura"})
	if r["lotti"] != 2.0 {
		t.Fatalf("10 oltre la CD: 2 lotti: %v", r)
	}
	if c, _ := tv.luca.chiama("POST", base+"/raccolta", map[string]any{"ingrediente": "ragni", "modo": "dedicata", "cd": 15, "abilita": "Natura"}); c != 400 {
		t.Fatal("le ghiandole di ragno si cercano solo con Sopravvivenza")
	}

	// Formule: le sblocca il Master
	if c, _ := tv.luca.chiama("POST", base+"/fabbrica", map[string]any{"formula": "essenza_spettrale"}); c != 403 {
		t.Fatal("formula non ancora conosciuta")
	}
	tv.m.deve(200, "PUT", base+"/formule", map[string]any{"formula": "essenza_spettrale", "conosciuta": true})
	// Creazione riuscita con gli ingredienti raccolti: costo ridotto 25 mo, 10 + 6 = 16 contro CD 13, 3 dosi (BC)
	tiraDado = sequenza(10)
	r = tv.luca.deve(200, "POST", base+"/fabbrica", map[string]any{"formula": "essenza_spettrale"})
	if r["dosi"] != 3.0 || moneteMO() != "175" || quantita("dose", "essenza_spettrale") != 3 || quantita("ingrediente", "felce_ombra") != 0 || quantita("ingrediente", "lume_cripta") != 1 {
		t.Fatalf("creazione: %v, monete %s", r, moneteMO())
	}
	tv.luca.deve(200, "POST", base+"/applica", map[string]any{"formula": "essenza_spettrale"})
	if quantita("dose", "essenza_spettrale") != 2 {
		t.Fatal("una dose applicata")
	}

	// Creazione fallita: tutto comprato (60 mo). 1 + 6 = 7 contro CD 14 (di 5 o più): niente rimborso.
	tv.m.deve(200, "PUT", base+"/formule", map[string]any{"formula": "paralizzante", "conosciuta": true})
	tiraDado = sequenza(1)
	tv.luca.deve(200, "POST", base+"/fabbrica", map[string]any{"formula": "paralizzante"})
	if moneteMO() != "115" || quantita("dose", "paralizzante") != 0 {
		t.Fatalf("fallimento di 5 o più: tutto perso, monete %s", moneteMO())
	}
	// 6 + 6 = 12 contro CD 14: si recupera metà dei materiali comprati (30 mo)
	tiraDado = sequenza(6)
	tv.luca.deve(200, "POST", base+"/fabbrica", map[string]any{"formula": "paralizzante"})
	if moneteMO() != "85" {
		t.Fatalf("fallimento: recuperata metà dei materiali, monete %s", moneteMO())
	}

	// Erborista: pozione di guarigione (25 mo, nessuna prova) nell'equipaggiamento
	tv.luca.deve(200, "POST", "/professioni/"+pg+"/erborista/fabbrica", map[string]any{"formula": "pozione_guarigione"})
	d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
	if d["eq"] != "Kit da avvelenatore\nBorsa da erborista\n1 × Pozione di prova" || moneteMO() != "60" {
		t.Fatalf("pozione nell'equipaggiamento: %q, monete %s", d["eq"], moneteMO())
	}

	// Estrazione: serve una fiala vuota
	if c, _ := tv.luca.chiama("POST", base+"/estrazione", map[string]any{"secrezione": "sacca_ragno_gigante"}); c != 409 {
		t.Fatal("senza fiala niente estrazione")
	}
	tv.luca.deve(200, "POST", base+"/fiale", map[string]any{"n": 2})
	tiraDado = sequenza(15)
	tv.luca.deve(200, "POST", base+"/estrazione", map[string]any{"secrezione": "sacca_ragno_gigante"})
	if quantita("secrezione", "sacca_ragno_gigante") != 1 || quantita("fiala", "vuota") != 1 || moneteMO() != "58" {
		t.Fatalf("estrazione: 1 secrezione, 1 fiala usata, monete %s", moneteMO())
	}
	// la sacca di un animale comune richiede prima la tana
	if c, _ := tv.luca.chiama("POST", base+"/estrazione", map[string]any{"secrezione": "sacca_animale"}); c != 409 {
		t.Fatal("prima la tana")
	}
	// monete insufficienti: niente cambia
	tv.m.deve(200, "PUT", base+"/formule", map[string]any{"formula": "predatore_potenziato", "conosciuta": true})
	tv.m.deve(200, "POST", base+"/inventario", map[string]any{"tipo": "secrezione", "voce": "ghiandola_pericolosa", "quantita": 1})
	if c, o := tv.luca.chiama("POST", base+"/fabbrica", map[string]any{"formula": "predatore_potenziato"}); c != 409 || quantita("secrezione", "ghiandola_pericolosa") != 1 {
		t.Fatalf("senza 300 mo la ghiandola non si consuma: %d %v %d", c, o, quantita("secrezione", "ghiandola_pericolosa"))
	}
	if c, _ := tv.luca.chiama("POST", base+"/inventario", map[string]any{"tipo": "speciale", "voce": "ectoplasma", "quantita": 1}); c != 403 {
		t.Fatal("solo il Master corregge l'inventario")
	}

	// Giorni di gioco: gli ingredienti durano 7 giorni; le dosi no
	if c, _ := tv.luca.chiama("POST", "/professioni-giorno", nil); c != 403 {
		t.Fatal("solo il Master fa passare i giorni")
	}
	for range 7 {
		tv.m.deve(200, "POST", "/professioni-giorno", nil)
	}
	if quantita("ingrediente", "lume_cripta") != 0 || quantita("secrezione", "sacca_ragno_gigante") != 0 || quantita("dose", "essenza_spettrale") != 2 {
		t.Fatal("dopo 7 giorni ingredienti e secrezioni scaduti, dosi intatte")
	}
}
