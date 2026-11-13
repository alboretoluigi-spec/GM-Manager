package server

import (
	"encoding/json"
	"testing"
)

// Regole pure della «Modifica manuale»: quali campi sono di gioco e come si trattano gli oggetti magici.
func TestModificaManualeRegole(t *testing.T) {
	raw := func(s string) json.RawMessage { return compatta(json.RawMessage(s)) }
	a := raw(`[{"o":{"id":"ogm-1","nome":"Spada"},"eq":false,"sint":false},{"o":{"id":"ogm-2","nome":"Anello"},"eq":true}]`)
	if !magSoloStato(a, raw(`[{"o":{"id":"ogm-1","nome":"Spada"},"eq":true,"sint":true,"car":3},{"o":{"id":"ogm-2","nome":"Anello"},"eq":false}]`)) {
		t.Fatal("equipaggiare, sintonizzare e usare cariche è gioco")
	}
	if magSoloStato(a, raw(`[{"o":{"id":"ogm-1","nome":"Spada +3"},"eq":false},{"o":{"id":"ogm-2","nome":"Anello"},"eq":true}]`)) {
		t.Fatal("cambiare i dati di un oggetto non è gioco")
	}
	if magSoloStato(a, raw(`[{"o":{"id":"ogm-1","nome":"Spada"},"eq":false}]`)) || magSoloStato(nil, raw(`[{"o":{"id":"x"}}]`)) {
		t.Fatal("togliere o aggiungere un oggetto non è gioco")
	}
	if !magSoloStato(nil, raw(`[]`)) && !magSoloStato(nil, nil) {
		t.Fatal("nessun oggetto: nessuna differenza")
	}

	attuale := piatto{"lvl": raw(`2`), "base.FOR": raw(`14`), "hpCur": raw(`"10"`), "mag": a, "f.storia": raw(`"Nato a Borgofosco"`)}
	mod := map[string]json.RawMessage{"lvl": raw(`5`), "hpCur": raw(`"7"`), "spp.Luce": raw(`true`), "coins.MO": raw(`"9"`), "base.FOR": raw(`14`),
		"mag": raw(`[{"o":{"id":"ogm-1","nome":"Spada"},"eq":true},{"o":{"id":"ogm-2","nome":"Anello"},"eq":true}]`), "terra": raw(`"Foresta"`)}
	ign, rim := filtraNonDiGioco(mod, []string{"f.storia", "slotU.1", "f.inesistente"}, attuale)
	if len(ign) != 3 || ign[0] != "f.storia" || ign[1] != "lvl" || ign[2] != "terra" {
		t.Fatalf("ignorati: %v", ign)
	}
	for _, k := range []string{"hpCur", "spp.Luce", "coins.MO", "mag", "base.FOR"} {
		if _, ok := mod[k]; !ok {
			t.Fatalf("%s doveva restare (gioco o invariato): %v", k, mod)
		}
	}
	if len(rim) != 1 || rim[0] != "slotU.1" {
		t.Fatalf("rimossi rimasti: %v", rim)
	}

	// Elenco degli oggetti (2026-11-11): equipaggiare è gioco, armatura e scudo indossati anche; rinominare no.
	inv := raw(`[{"id":"inv-1","n":"Cotta di maglia","q":1,"eq":false,"k":"armatura","ref":"Cotta di maglia"}]`)
	att2 := piatto{"inv": inv, "armor": raw(`""`), "shield": raw(`false`)}
	mod2 := map[string]json.RawMessage{"inv": raw(`[{"id":"inv-1","n":"Cotta di maglia","q":1,"eq":true,"k":"armatura","ref":"Cotta di maglia"}]`), "armor": raw(`"Cotta di maglia"`), "shield": raw(`true`)}
	if ign, _ := filtraNonDiGioco(mod2, nil, att2); len(ign) != 0 {
		t.Fatalf("equipaggiare dall'elenco è gioco: %v", ign)
	}
	mod3 := map[string]json.RawMessage{"inv": raw(`[{"id":"inv-1","n":"Cotta di mithral","q":1,"eq":false,"k":"armatura","ref":"Cotta di maglia"}]`)}
	if ign, _ := filtraNonDiGioco(mod3, nil, att2); len(ign) != 1 || ign[0] != "inv" {
		t.Fatalf("rinominare un oggetto va approvato: %v", ign)
	}

	// Applicando una richiesta su «mag», gli oggetti già presenti tengono lo stato attuale; quelli nuovi il loro.
	ora := raw(`[{"o":{"id":"ogm-1","nome":"Spada"},"eq":true,"sint":true,"car":1}]`)
	nuovo := raw(`[{"o":{"id":"ogm-1","nome":"Spada +1"},"eq":false},{"o":{"id":"ogm-9","nome":"Mantello"},"eq":true}]`)
	var l []map[string]any
	json.Unmarshal(magConStatoAttuale(nuovo, ora), &l)
	if len(l) != 2 || l[0]["eq"] != true || l[0]["sint"] != true || l[0]["car"] != 1.0 || l[0]["o"].(map[string]any)["nome"] != "Spada +1" || l[1]["eq"] != true {
		t.Fatalf("stato conservato: %v", l)
	}
}

// Percorso completo: blocco dei campi non di gioco, richiesta, unione, decisione voce per voce, ripristino, ritiro.
func TestModificaManuale(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"lvl": 2, "base.FOR": 14, "cls": "guerriero"}})
	// Approvazione spenta (come nei test delle altre funzioni): il giocatore salva tutto da solo.
	if r := tv.luca.deve(200, "GET", "/personaggi/"+pg+"/richiesta-modifiche", nil); r["approvazione"] != false {
		t.Fatalf("approvazione spenta: %v", r)
	}
	tv.luca.deve(403, "PATCH", "/impostazioni", map[string]any{"approvazioneSchede": true})
	if r := tv.m.deve(200, "PATCH", "/impostazioni", map[string]any{"approvazioneSchede": true}); r["approvazioneSchede"] != true {
		t.Fatalf("impostazione: %v", r)
	}
	if r := tv.luca.deve(200, "GET", "/personaggi/"+pg+"/richiesta-modifiche", nil); r["approvazione"] != true || r["richiesta"] != nil {
		t.Fatalf("approvazione attiva, nessuna richiesta: %v", r)
	}
	if r := tv.m.deve(200, "GET", "/personaggi/"+pg+"/richiesta-modifiche", nil); r["approvazione"] != false {
		t.Fatal("il Master non passa dall'approvazione")
	}

	// Salvataggio normale: i PF valgono, il livello no (torna indietro come ignorato).
	r := tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"lvl": 9, "hpCur": "5"}})
	if ign, _ := r["ignorati"].([]any); len(ign) != 1 || ign[0] != "lvl" {
		t.Fatalf("ignorati: %v", r)
	}
	d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
	if d["lvl"] != 2.0 || d["hpCur"] != "5" {
		t.Fatalf("livello invariato, PF aggiornati: lvl=%v hpCur=%v", d["lvl"], d["hpCur"])
	}

	// Il giocatore spegne «Modifica manuale»: richiesta al Master, in tempo reale.
	cm := tv.clienteWS(tv.m)
	cl := tv.clienteWS(tv.luca)
	ultimiMessaggi(cm, "")
	ultimiMessaggi(cl, "")
	r = tv.luca.deve(200, "POST", "/personaggi/"+pg+"/richiesta-modifiche", map[string]any{"modifiche": map[string]any{"lvl": 3, "base.FOR": 16, "hpCur": "1", "cls": "guerriero"}})
	ric := r["richiesta"].(map[string]any)
	voci := ric["voci"].([]any)
	if len(voci) != 2 || voci[0].(map[string]any)["campo"] != "base.FOR" || voci[1].(map[string]any)["campo"] != "lvl" || voci[1].(map[string]any)["prima"] != 2.0 {
		t.Fatalf("voci (senza PF né campi invariati): %v", voci)
	}
	if len(ultimiMessaggi(cm, "scheda.richiesta")) != 1 {
		t.Fatal("il Master riceve il report in tempo reale")
	}
	if d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any); d["lvl"] != 2.0 {
		t.Fatal("finché il Master non decide vale la scheda di prima")
	}
	// Secondo invio: si unisce alla richiesta aperta; la Forza riportata a 14 esce dalla richiesta.
	r = tv.luca.deve(200, "POST", "/personaggi/"+pg+"/richiesta-modifiche", map[string]any{"modifiche": map[string]any{"lvl": 4, "base.FOR": 14, "f.storia": "Nato a Borgofosco"}})
	ric = r["richiesta"].(map[string]any)
	campi := map[string]map[string]any{}
	for _, v := range ric["voci"].([]any) {
		campi[v.(map[string]any)["campo"].(string)] = v.(map[string]any)
	}
	if len(campi) != 2 || campi["lvl"]["dopo"] != 4.0 || campi["lvl"]["prima"] != 2.0 || campi["f.storia"] == nil {
		t.Fatalf("richiesta unita: %v", campi)
	}
	l := tv.m.deve(200, "GET", "/richieste-schede", nil)["lista"].([]any)
	if len(l) != 1 || l[0].(map[string]any)["id"] != ric["id"] || l[0].(map[string]any)["giocatore"] != "Luca" {
		t.Fatalf("una sola richiesta in attesa: %v", l)
	}

	// Solo il Master decide.
	id := ric["id"].(string)
	tv.luca.deve(403, "GET", "/richieste-schede", nil)
	tv.luca.deve(403, "POST", "/richieste-schede/"+id, map[string]any{"approvati": []string{"lvl"}})
	tv.sara.deve(404, "GET", "/personaggi/"+pg+"/richiesta-modifiche", nil)
	tv.m.deve(400, "POST", "/richieste-schede/"+id, map[string]any{"approvati": []string{"lvl"}, "rifiutati": []string{"lvl"}})

	// Voce per voce: livello approvato; la storia resta in attesa.
	r = tv.m.deve(200, "POST", "/richieste-schede/"+id, map[string]any{"approvati": []string{"lvl"}})
	if r["richiesta"].(map[string]any)["stato"] != "in_attesa" {
		t.Fatalf("resta in attesa la storia: %v", r)
	}
	d = tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
	if d["lvl"] != 4.0 || d["hpCur"] != "5" || d["base"].(map[string]any)["FOR"] != 14.0 {
		t.Fatalf("livello 4 approvato, il resto invariato: %v", d)
	}
	if len(ultimiMessaggi(cl, "scheda.richiesta.esito")) != 1 {
		t.Fatal("il giocatore riceve l'esito")
	}
	// Rifiutata la storia: richiesta chiusa; deciderla di nuovo non si può.
	tv.m.deve(200, "POST", "/richieste-schede/"+id, map[string]any{"rifiutati": []string{"f.storia"}})
	if d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any); d["f"] != nil && d["f"].(map[string]any)["storia"] == "Nato a Borgofosco" {
		t.Fatal("la storia rifiutata non entra nella scheda")
	}
	tv.m.deve(409, "POST", "/richieste-schede/"+id, map[string]any{"approvati": []string{"f.storia"}})
	if r := tv.luca.deve(200, "GET", "/personaggi/"+pg+"/richiesta-modifiche", nil); r["richiesta"] != nil {
		t.Fatal("nessuna richiesta in attesa")
	}
	es := tv.m.deve(200, "GET", "/richieste-schede", nil)["lista"].([]any)[0].(map[string]any)["esiti"].([]any)
	if len(es) != 2 || es[0].(map[string]any)["esito"] != "approvata" || es[1].(map[string]any)["esito"] != "rifiutata" {
		t.Fatalf("esiti nello storico: %v", es)
	}

	// Ripristino di una vecchia versione da parte del giocatore: diventa una richiesta (livello 2).
	piuVecchia := 1 << 30
	for _, x := range tv.luca.deve(200, "GET", "/personaggi/"+pg+"/versioni", nil)["lista"].([]any) {
		if n := int(x.(map[string]any)["numero"].(float64)); n < piuVecchia {
			piuVecchia = n
		}
	}
	r = tv.luca.deve(200, "POST", "/personaggi/"+pg+"/ripristina/"+itoa(piuVecchia), map[string]any{})
	if r["richiesta"] == nil {
		t.Fatalf("ripristino come richiesta: %v", r)
	}
	if d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any); d["lvl"] != 4.0 {
		t.Fatal("il ripristino non vale finché il Master non lo approva")
	}
	// Il giocatore ritira la richiesta.
	tv.luca.deve(200, "DELETE", "/personaggi/"+pg+"/richiesta-modifiche", nil)
	tv.luca.deve(409, "DELETE", "/personaggi/"+pg+"/richiesta-modifiche", nil)

	// Il Master modifica direttamente; spegnendo l'approvazione il giocatore torna a salvare tutto.
	tv.m.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.m, pg), "modifiche": map[string]any{"lvl": 6}})
	tv.m.deve(400, "POST", "/personaggi/"+pg+"/richiesta-modifiche", map[string]any{"modifiche": map[string]any{"lvl": 7}})
	tv.m.deve(200, "PATCH", "/impostazioni", map[string]any{"approvazioneSchede": false})
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"lvl": 7}})
	if d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any); d["lvl"] != 7.0 {
		t.Fatal("approvazione spenta: salvataggio diretto")
	}
}
