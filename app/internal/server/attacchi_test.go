package server

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// tiriDecisi sostituisce i dadi con una sequenza nota (finita la sequenza: il massimo del dado).
func tiriDecisi(t *testing.T, l ...int) {
	vecchio := dado
	dado = func(n int) int {
		if len(l) == 0 {
			return n
		}
		x := l[0]
		l = l[1:]
		return x
	}
	t.Cleanup(func() { dado = vecchio })
}

func TestDifeseEProfili(t *testing.T) {
	d := difeseCombattente{Imm: []difesa{{Tipo: "veleno"}}, Res: []difesa{{Tipo: "tagliente", NonMagici: true}, {Tipo: "fuoco"}}, Vuln: []difesa{{Tipo: "fuoco"}, {Tipo: "radioso"}}}
	casi := []struct {
		tipo   string
		v      int
		magica bool
		atteso int
	}{{"veleno", 10, false, 0}, {"tagliente", 11, false, 5}, {"tagliente", 11, true, 11}, {"radioso", 7, false, 14}, {"fuoco", 9, false, 8}, {"", 6, false, 6}, {"perforante", 6, false, 6}}
	for _, c := range casi {
		if v, _ := applicaDifese(c.tipo, c.v, c.magica, d); v != c.atteso {
			t.Errorf("%s %d (magica %v): atteso %d, ottenuto %d", c.tipo, c.v, c.magica, c.atteso, v)
		}
	}
	if tipoDanno("danni da Fuoco") != "fuoco" || tipoDanno("Taglienti") != "tagliente" || tipoDanno("mistico") != "" {
		t.Fatal("tipi di danno non riconosciuti")
	}
	p := validaProfili([]profiloAttacco{
		{Nome: "Colpo senz'armi", Bonus: 4, Danni: []componenteDanno{{Dadi: "1", Mod: 2, Tipo: "contundente"}}},
		{Nome: "Spada", Bonus: 5, Danni: []componenteDanno{{Dadi: "1d8", Mod: 3, Tipo: "taglienti"}, {Dadi: "3d7", Tipo: "fuoco"}, {Dadi: "2d6", Tipo: "fuoco"}}},
		{Nome: "", Bonus: 2},
		{Nome: "Troppo", Bonus: 99},
	})
	if len(p) != 2 || p[0].Danni[0].Dadi != "" || p[0].Danni[0].Mod != 3 || len(p[1].Danni) != 2 || p[1].Danni[0].Tipo != "tagliente" {
		t.Fatalf("profili non validati come atteso: %+v", p)
	}
	// limiti (diagnosi 3.4): al massimo 20 dadi per componente e 40 per attacco
	q := validaProfili([]profiloAttacco{{Nome: "Sciame", Bonus: 5, Danni: []componenteDanno{{Dadi: "20d6", Tipo: "fuoco"}, {Dadi: "20d6", Tipo: "contundente"}, {Dadi: "1d6", Tipo: "freddo"}}},
		{Nome: "Troppi", Bonus: 5, Danni: []componenteDanno{{Dadi: "21d6", Tipo: "fuoco"}, {Dadi: "1d8", Mod: 31}}}})
	if len(q) != 2 || len(q[0].Danni) != 2 || len(q[1].Danni) != 0 {
		t.Fatalf("limiti dei dadi: %+v", q)
	}
	if valoriInsoliti(9, 0, nil) != "" || valoriInsoliti(18, 0, nil) == "" || valoriInsoliti(0, 24, nil) == "" || valoriInsoliti(0, 23, nil) != "" {
		t.Fatal("avvisi per i valori insoliti")
	}
	if validaProfiloMostro([]byte(`{"attacchi":[],"difese":{}}`)) != "" || validaProfiloMostro([]byte(`nonjson`)) != "" {
		t.Fatal("un profilo vuoto o rotto non si salva")
	}
}

func TestAttacchiAutomatici(t *testing.T) {
	sc := nuovoScontro(t)
	// La scheda di Arin manda attacchi, attacchi per azione e difese (resistenza al fuoco).
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"lvl": 3},
		"riepilogo": map[string]any{"class": "Chierico", "level": 3, "ac": 16, "hpMax": 20, "initiative": 2, "dex": 14,
			"attacks": []any{map[string]any{"nome": "Mazza", "bonus": 5, "car": "FOR", "competente": true, "portata": "1,5 m",
				"danni": []any{map[string]any{"dadi": "1d6", "mod": 3, "tipo": "contundente"}}}},
			"attacksPerAction": 1, "defenses": map[string]any{"res": []string{"fuoco"}, "imm": []string{}}}})
	// Lo zombi arriva dal bestiario con il suo attacco e le sue difese.
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Zombi", "quanti": 1, "ca": 8, "pf": 22,
		"profilo": map[string]any{"attacchi": []any{map[string]any{"nome": "Schianto", "bonus": 3, "danni": []any{map[string]any{"dadi": "1d6", "mod": 1, "tipo": "contundenti"}}}},
			"difese": map[string]any{"imm": []any{map[string]any{"tipo": "veleno"}}}}})
	vm := sc.vista(sc.m)
	arin, zombi := cerca(vm, "Arin"), cerca(vm, "Zombi")
	if len(zombi["attacchi"].([]any)) != 1 || len(arin["attacchi"].([]any)) != 1 {
		t.Fatalf("il Master vede gli attacchi di tutti: %v %v", arin, zombi)
	}
	vl := sc.vista(sc.luca)
	if cerca(vl, "Zombi")["attacchi"] != nil || cerca(vl, "Arin")["attacchi"] == nil {
		t.Fatal("il giocatore vede gli attacchi del suo PG, non quelli dei mostri")
	}
	idA, idZ := arin["id"].(string), zombi["id"].(string)
	// Prima dell'inizio non si attacca.
	sc.az(sc.luca, 409, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "azione": "prova-000"})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 30})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idZ, "valore": 1})
	sc.az(sc.m, 200, "iniz.avvia", nil)

	// Turno di Arin: d20 15 + 5 = 20 contro CA 8, colpito; 1d6 = 4 + 3 = 7 contundenti → zombi a 15.
	tiriDecisi(t, 15, 4)
	e := sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "attacco": 0, "azione": "prova-001"})
	if e["risultato"] != "colpito" || e["totale"].(float64) != 20 || e["totaleDanni"].(float64) != 7 || e["ca"] != nil || e["pfDopo"] != nil {
		t.Fatalf("esito per il giocatore sbagliato (senza CA e PF del mostro): %v", e)
	}
	if z := cerca(sc.vista(sc.m), "Zombi"); z["pf"].(float64) != 15 {
		t.Fatalf("zombi a 15 PF: %v", z["pf"])
	}
	if e := cerca(sc.vista(sc.luca), "Arin")["economia"].(map[string]any); e["a"] != true || e["r"] != false {
		t.Fatalf("l'attacco nel proprio turno usa l'azione: %v", e)
	}
	// La stessa azione ripetuta non tira né toglie altri PF.
	sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "attacco": 0, "azione": "prova-001"})
	if z := cerca(sc.vista(sc.m), "Zombi"); z["pf"].(float64) != 15 {
		t.Fatalf("doppio clic: lo zombi deve restare a 15: %v", z["pf"])
	}
	// Un solo attacco per azione; la Segretaria non attacca con Arin; Ugo non è nella sessione.
	sc.az(sc.luca, 409, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "azione": "prova-002"})
	sc.az(sc.sara, 403, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "azione": "prova-003"})
	sc.az(sc.ugo, 404, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "azione": "prova-004"})
	// Il giocatore non muove i mostri.
	sc.az(sc.luca, 403, "attacco.tira", map[string]any{"combattente": idZ, "bersaglio": idA, "azione": "prova-005"})

	// Il Master attacca con lo zombi anche fuori turno: 20 naturale, critico. I danni aspettano la scelta.
	tiriDecisi(t, 20, 6, 5)
	e = sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idZ, "bersaglio": idA, "azione": "prova-006"})
	g, _ := e["gettone"].(string)
	if e["risultato"] != "critico" || g == "" || e["danni"] != nil {
		t.Fatalf("critico: atteso il gettone e nessun danno: %v", e)
	}
	if a := cerca(sc.vista(sc.m), "Arin"); a["pf"].(float64) != 20 {
		t.Fatal("prima della scelta i PF non cambiano")
	}
	// Dadi tirati due volte: 6 + 5 + 1 = 12 contundenti → Arin a 8 (la scheda vede gli stessi PF).
	sc.az(sc.luca, 403, "attacco.critico", map[string]any{"gettone": g, "modo": "dadi", "azione": "prova-007"})
	e = sc.az(sc.m, 200, "attacco.critico", map[string]any{"gettone": g, "modo": "dadi", "azione": "prova-008"})
	if e["totaleDanni"].(float64) != 12 || e["modoCritico"] == nil {
		t.Fatalf("critico con i dadi due volte: attesi 12 danni: %v", e)
	}
	if a := cerca(sc.vista(sc.luca), "Arin"); a["pf"].(float64) != 8 {
		t.Fatalf("Arin a 8 PF: %v", a["pf"])
	}
	sc.az(sc.m, 409, "attacco.critico", map[string]any{"gettone": g, "modo": "dadi", "azione": "prova-009"})

	if e := cerca(sc.vista(sc.m), "Zombi")["economia"].(map[string]any); e["r"] != true || e["a"] != false {
		t.Fatalf("l'attacco fuori turno usa la reazione: %v", e)
	}
	if cerca(sc.vista(sc.luca), "Zombi")["economia"] != nil {
		t.Fatal("le azioni dei mostri non si mostrano ai giocatori")
	}
	// Secondo attacco dello zombi fuori turno (reazione già usata): il server chiede conferma; con «forza» va.
	if r := sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idZ, "bersaglio": idA, "azione": "prova-010"}); r["serveConferma"] == nil {
		t.Fatalf("attesa la richiesta di conferma: %v", r)
	}
	tiriDecisi(t, 18, 2) // svantaggio: vale il 2 → 5 contro CA 16, mancato
	e = sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idZ, "bersaglio": idA, "modo": "svantaggio", "forza": true, "azione": "prova-011"})
	if e["risultato"] != "mancato" || e["naturale"].(float64) != 2 || e["ca"].(float64) != 16 {
		t.Fatalf("svantaggio: atteso mancato con il 2: %v", e)
	}
	// Turno dello zombi: Arin non può attaccare.
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	sc.az(sc.luca, 409, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "azione": "prova-012"})
	// Round 2, di nuovo Arin: azione, azione bonus e reazione tornano. Se il giocatore segna l'azione come usata (per
	// esempio per Scatto), non attacca finché non la toglie. L'1 naturale manca sempre.
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	if e := cerca(sc.vista(sc.luca), "Arin")["economia"].(map[string]any); e["a"] != false {
		t.Fatalf("all'inizio del turno l'azione torna: %v", e)
	}
	sc.az(sc.luca, 200, "iniz.economia", map[string]any{"combattente": idA, "economia": map[string]any{"a": true, "b": true}})
	sc.az(sc.luca, 409, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "azione": "prova-014"})
	sc.az(sc.luca, 403, "iniz.economia", map[string]any{"combattente": idZ, "economia": map[string]any{"r": true}})
	sc.az(sc.ugo, 404, "iniz.economia", map[string]any{"combattente": idA, "economia": map[string]any{}})
	sc.az(sc.sara, 200, "iniz.economia", map[string]any{"combattente": idA, "economia": map[string]any{"b": true}})
	tiriDecisi(t, 1)
	if e = sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "attacco": 0, "azione": "prova-013"}); e["risultato"] != "mancato" {
		t.Fatalf("1 naturale: mancato: %v", e)
	}
	reg := strings.Join(func() []string {
		var l []string
		for _, x := range sc.vista(sc.m)["registro"].([]any) {
			l = append(l, x.(string))
		}
		return l
	}(), "\n")
	if !strings.Contains(reg, "COLPO CRITICO") || !strings.Contains(reg, "Arin subisce 12 danni") || !strings.Contains(reg, "Zombi subisce 7 danni") {
		t.Fatalf("registro incompleto:\n%s", reg)
	}
}

func TestIncantesimiNelloScontro(t *testing.T) {
	sc := nuovoScontro(t)
	// Arin: Saggezza 16 con competenza nei TS su Saggezza (livello 3: +2), Destrezza 14.
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"lvl": 3},
		"riepilogo": map[string]any{"class": "Chierico", "level": 3, "ac": 16, "hpMax": 20, "initiative": 2, "dex": 14,
			"abilities": map[string]any{"SAG": 16, "DES": 14}, "saveProfs": []string{"SAG"}}})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Zombi", "quanti": 1, "ca": 8, "pf": 22,
		"profilo": map[string]any{"attacchi": []any{map[string]any{"nome": "Schianto", "bonus": 3, "danni": []any{map[string]any{"dadi": "1d6", "mod": 1, "tipo": "contundente"}}}},
			"ts": map[string]any{"DES": -2, "SAG": -2}, "immCond": []string{"Avvelenato"}, "difese": map[string]any{"imm": []any{map[string]any{"tipo": "veleno"}}}}})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Lupo", "quanti": 1, "ca": 13, "pf": 30,
		"profilo": map[string]any{"ts": map[string]any{"DES": 2}, "resMagia": true}})
	vm := sc.vista(sc.m)
	idA, idZ, idL := cerca(vm, "Arin")["id"].(string), cerca(vm, "Zombi")["id"].(string), cerca(vm, "Lupo")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 30})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idZ, "valore": 1})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idL, "valore": 2})
	sc.az(sc.m, 200, "iniz.avvia", nil)

	// Palla di fuoco (TS su Destrezza CD 13, metà se riesce): danni tirati una volta (8 × 3 = 24); zombi 5 − 2 = 3, fallisce;
	// lupo con Resistenza alla magia (vantaggio) 10 e 12 → 12 + 2 = 14, riesce.
	tiriDecisi(t, 3, 3, 3, 3, 3, 3, 3, 3, 5, 10, 12)
	palla := map[string]any{"nome": "Palla di fuoco", "livello": 3, "slot": 3, "tempo": "azione", "tipo": "ts", "cd": 13, "car": "DES", "seRiesce": "meta",
		"danni": []any{map[string]any{"dadi": "8d6", "tipo": "fuoco"}}}
	r := sc.az(sc.luca, 200, "incantesimo.ts", map[string]any{"combattente": idA, "incantesimo": palla, "bersagli": []string{idZ, idL}, "azione": "inc-0001"})
	if r["inAttesa"] != true {
		t.Fatalf("il giocatore aspetta la conferma del Master: %v", r)
	}
	if cerca(sc.vista(sc.luca), "Arin")["economia"].(map[string]any)["a"] != true {
		t.Fatal("Palla di fuoco usa l'azione")
	}
	if sc.vista(sc.luca)["incantesimiInAttesa"] != nil {
		t.Fatal("i tiri salvezza in attesa li vede solo il Master")
	}
	att := sc.vista(sc.m)["incantesimiInAttesa"].([]any)
	if len(att) != 1 {
		t.Fatalf("un incantesimo in attesa per il Master: %v", att)
	}
	p := att[0].(map[string]any)
	righe := p["righe"].([]any)
	z, l := righe[0].(map[string]any), righe[1].(map[string]any)
	if z["riuscito"] != false || z["totale"].(float64) != 3 || l["riuscito"] != true || l["totale"].(float64) != 14 || len(l["d20"].([]any)) != 2 {
		t.Fatalf("tiri salvezza: %v", righe)
	}
	if cerca(sc.vista(sc.m), "Zombi")["pf"].(float64) != 22 {
		t.Fatal("prima della conferma i PF non cambiano")
	}
	g := p["gettone"].(string)
	sc.az(sc.luca, 403, "incantesimo.applica", map[string]any{"gettone": g, "azione": "inc-0002"})
	// Il Master cambia l'esito dello zombi (riesce): metà, 12; il lupo riesce: 12.
	e := sc.az(sc.m, 200, "incantesimo.applica", map[string]any{"gettone": g, "riusciti": map[string]bool{idZ: true}, "applicaCondizioni": true, "azione": "inc-0003"})
	if e["applicato"] != true {
		t.Fatalf("applicato: %v", e)
	}
	vm = sc.vista(sc.m)
	if cerca(vm, "Zombi")["pf"].(float64) != 10 || cerca(vm, "Lupo")["pf"].(float64) != 18 {
		t.Fatalf("metà dei danni a chi riesce: zombi %v, lupo %v", cerca(vm, "Zombi")["pf"], cerca(vm, "Lupo")["pf"])
	}
	sc.az(sc.m, 409, "incantesimo.applica", map[string]any{"gettone": g, "azione": "inc-0004"})

	// Blocca persone (azione bonus solo per la prova, concentrazione): lo zombi fallisce; paralizzato sì, avvelenato no (immune).
	tiriDecisi(t, 2)
	blocca := map[string]any{"nome": "Blocca persone", "livello": 2, "slot": 2, "tempo": "bonus", "conc": true, "tipo": "ts", "cd": 13, "car": "SAG",
		"condizioni": []string{"Paralizzato", "Avvelenato"}}
	sc.az(sc.luca, 409, "incantesimo.ts", map[string]any{"combattente": idA, "incantesimo": map[string]any{"nome": "Blocca persone", "tempo": "azione", "tipo": "ts", "cd": 13, "car": "SAG"},
		"bersagli": []string{idZ}, "azione": "inc-0005"}) // l'azione è già usata
	sc.az(sc.luca, 200, "incantesimo.ts", map[string]any{"combattente": idA, "incantesimo": blocca, "bersagli": []string{idZ}, "azione": "inc-0006"})
	g = sc.vista(sc.m)["incantesimiInAttesa"].([]any)[0].(map[string]any)["gettone"].(string)
	sc.az(sc.m, 200, "incantesimo.applica", map[string]any{"gettone": g, "applicaCondizioni": true, "azione": "inc-0007"})
	vm = sc.vista(sc.m)
	zc := cerca(vm, "Zombi")["condizioni"].([]any)
	if len(zc) != 1 || zc[0] != "Paralizzato" {
		t.Fatalf("condizioni dello zombi: %v", zc)
	}
	if ac := cerca(vm, "Arin")["condizioni"].([]any); len(ac) != 1 || ac[0] != "Concentrato" {
		t.Fatalf("chi lancia un incantesimo a concentrazione è concentrato: %v", ac)
	}

	// Cura ferite (il Master per Arin, tempo «altro»): 1d8 + 3 = 8; Arin da 10 a 18.
	sc.az(sc.m, 200, "pg.danno", map[string]any{"combattente": idA, "valore": 10})
	tiriDecisi(t, 5)
	e = sc.az(sc.m, 200, "incantesimo.cura", map[string]any{"combattente": idA, "incantesimo": map[string]any{"nome": "Cura ferite", "tempo": "altro", "tipo": "cura",
		"cura": map[string]any{"dadi": "1d8", "mod": 3}}, "bersagli": []string{idA}, "azione": "inc-0008"})
	if cerca(sc.vista(sc.m), "Arin")["pf"].(float64) != 18 || e["righe"].([]any)[0].(map[string]any)["cura"].(float64) != 8 {
		t.Fatalf("cura: %v", e)
	}

	// Dardo di fuoco con due raggi, come reazione (anche fuori turno per il giocatore): il primo usa la reazione, il secondo no.
	// Lo zombi è paralizzato: vantaggio (due d20); a distanza, niente critico automatico.
	dardo := map[string]any{"nome": "Dardo di fuoco", "livello": 0, "tempo": "reazione", "tipo": "attacco", "bonus": 5, "raggi": 2, "lancio": "lancio-0001",
		"danni": []any{map[string]any{"dadi": "1d10", "tipo": "fuoco"}}}
	tiriDecisi(t, 15, 3, 6, 1, 1)
	e = sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "incantesimo": dardo, "azione": "inc-0009"})
	if e["risultato"] != "colpito" || e["modo"] != "vantaggio" || e["totaleDanni"].(float64) != 6 || e["attacchiRimasti"].(float64) != 1 {
		t.Fatalf("primo raggio: %v", e)
	}
	if e = sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "incantesimo": dardo, "azione": "inc-0010"}); e["risultato"] != "mancato" {
		t.Fatalf("secondo raggio: %v", e)
	}
	sc.az(sc.luca, 409, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "incantesimo": dardo, "azione": "inc-0011"})
	if cerca(sc.vista(sc.m), "Zombi")["pf"].(float64) != 4 || cerca(sc.vista(sc.luca), "Arin")["economia"].(map[string]any)["r"] != true {
		t.Fatal("il dardo toglie 6 PF allo zombi e usa la reazione di Arin")
	}

	// Arin è concentrato: colpito dallo zombi (15 + 3 contro CA 16, 4 + 1 = 5 danni), l'esito chiede il TS di Costituzione CD 10.
	// Lo zombi è paralizzato: senza «forza» il Master riceve la richiesta di conferma.
	if r := sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idZ, "bersaglio": idA, "azione": "inc-0012"}); r["serveConferma"] == nil {
		t.Fatalf("un paralizzato non attacca senza conferma: %v", r)
	}
	tiriDecisi(t, 15, 4)
	e = sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idZ, "bersaglio": idA, "forza": true, "azione": "inc-0014"})
	if e["concentrazione"] == nil || e["concentrazione"].(float64) != 10 {
		t.Fatalf("TS di concentrazione: %v", e)
	}

	// Bonus del PG ai TS: Saggezza 16 (+3) con competenza (+2) = +5.
	tiriDecisi(t, 9)
	e = sc.az(sc.m, 200, "incantesimo.ts", map[string]any{"combattente": idZ, "incantesimo": map[string]any{"nome": "Comando", "tempo": "altro", "tipo": "ts", "cd": 15, "car": "SAG",
		"condizioni": []string{"Prono"}}, "bersagli": []string{idA}, "forza": true, "azione": "inc-0013"})
	if rr := e["righe"].([]any)[0].(map[string]any); rr["bonus"].(float64) != 5 || rr["totale"].(float64) != 14 || rr["riuscito"] != false {
		t.Fatalf("TS di Arin: %v", rr)
	}
}

// Conseguenze delle condizioni negli attacchi e nei tiri salvezza.
func TestCondizioniNegliAttacchi(t *testing.T) {
	sc := nuovoScontro(t)
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"lvl": 3},
		"riepilogo": map[string]any{"class": "Guerriero", "level": 3, "ac": 16, "hpMax": 20, "initiative": 2, "dex": 14, "attacksPerAction": 2,
			"attacks": []any{
				map[string]any{"nome": "Mazza", "bonus": 5, "portata": "1,5 m", "danni": []any{map[string]any{"dadi": "1d6", "mod": 3, "tipo": "contundente"}}},
				map[string]any{"nome": "Arco lungo", "bonus": 5, "portata": "45/180 m", "danni": []any{map[string]any{"dadi": "1d8", "mod": 3, "tipo": "perforante"}}}}}})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Zombi", "quanti": 1, "ca": 8, "pf": 60,
		"profilo": map[string]any{"ts": map[string]any{"DES": -2}, "attacchi": []any{map[string]any{"nome": "Schianto", "bonus": 3, "danni": []any{map[string]any{"dadi": "1d6", "mod": 1, "tipo": "contundente"}}}}}})
	vm := sc.vista(sc.m)
	idA, idZ := cerca(vm, "Arin")["id"].(string), cerca(vm, "Zombi")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 30})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idZ, "valore": 1})
	sc.az(sc.m, 200, "iniz.avvia", nil)
	sc.az(sc.m, 200, "mostro.condizioni", map[string]any{"combattente": idZ, "condizioni": []string{"Prono"}})

	// Zombi prono: in mischia vantaggio (3 e 18 → 18), con l'arco svantaggio (18 e 2 → 2 + 5 = 7 contro CA 8, mancato).
	tiriDecisi(t, 3, 18, 4)
	e := sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "attacco": 0, "azione": "cond-0001"})
	if e["modo"] != "vantaggio" || e["naturale"].(float64) != 18 || e["risultato"] != "colpito" || len(e["fonti"].([]any)) != 1 {
		t.Fatalf("mischia contro un prono: %v", e)
	}
	tiriDecisi(t, 18, 2)
	e = sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "attacco": 1, "azione": "cond-0002"})
	if e["modo"] != "svantaggio" || e["risultato"] != "mancato" {
		t.Fatalf("arco contro un prono: %v", e)
	}
	// Zombi paralizzato: vantaggio e, da vicino, ogni colpo è critico (gettone per la scelta).
	sc.az(sc.m, 200, "mostro.condizioni", map[string]any{"combattente": idZ, "condizioni": []string{"Paralizzato"}})
	tiriDecisi(t, 5, 10)
	e = sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "attacco": 0, "forza": true, "azione": "cond-0003"})
	if e["risultato"] != "critico" || e["gettone"] == nil || e["naturale"].(float64) != 10 {
		t.Fatalf("critico automatico contro un paralizzato: %v", e)
	}
	// Arin avvelenato che sceglie vantaggio: si annullano, un solo d20.
	sc.az(sc.m, 200, "mostro.condizioni", map[string]any{"combattente": idZ, "condizioni": []string{}})
	sc.az(sc.m, 200, "pg.condizioni", map[string]any{"combattente": idA, "condizioni": []string{"Avvelenato"}})
	tiriDecisi(t, 4)
	e = sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "attacco": 0, "modo": "vantaggio", "forza": true, "azione": "cond-0004"})
	if e["modo"] != "normale" || len(e["d20"].([]any)) != 1 {
		t.Fatalf("vantaggio e svantaggio si annullano: %v", e)
	}
	// Arin stordito non attacca; lo zombi paralizzato fallisce da solo il TS su Destrezza (nessun d20).
	sc.az(sc.m, 200, "pg.condizioni", map[string]any{"combattente": idA, "condizioni": []string{"Stordito"}})
	sc.az(sc.luca, 409, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idZ, "attacco": 0, "azione": "cond-0005"})
	sc.az(sc.m, 200, "mostro.condizioni", map[string]any{"combattente": idZ, "condizioni": []string{"Paralizzato"}})
	e = sc.az(sc.m, 200, "incantesimo.ts", map[string]any{"combattente": idA, "incantesimo": map[string]any{"nome": "Onda tonante", "tempo": "altro", "tipo": "ts", "cd": 13, "car": "DES",
		"danni": []any{map[string]any{"dadi": "2d8", "tipo": "tuono"}}}, "bersagli": []string{idZ}, "forza": true, "azione": "cond-0006"})
	if r := e["righe"].([]any)[0].(map[string]any); r["riuscito"] != false || r["d20"] != nil || r["nota"] == nil {
		t.Fatalf("TS su Destrezza di un paralizzato: %v", r)
	}
}

// Effetti in corso: TS ripetuto alla fine del turno, fine della concentrazione, durata in round, 0 PF.
func TestEffettiInCorso(t *testing.T) {
	sc := nuovoScontro(t)
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Zombi", "quanti": 1, "ca": 8, "pf": 60,
		"profilo": map[string]any{"ts": map[string]any{"SAG": -2}, "attacchi": []any{map[string]any{"nome": "Schianto", "bonus": 3, "danni": []any{map[string]any{"dadi": "1d6", "mod": 1, "tipo": "contundente"}}}}}})
	vm := sc.vista(sc.m)
	idA, idZ := cerca(vm, "Arin")["id"].(string), cerca(vm, "Zombi")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 30})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idZ, "valore": 1})
	sc.az(sc.m, 200, "iniz.avvia", nil)
	blocca := map[string]any{"nome": "Blocca persone", "livello": 2, "slot": 2, "tempo": "azione", "conc": true, "round": 10, "ripeti": true, "tipo": "ts", "cd": 13, "car": "SAG",
		"condizioni": []string{"Paralizzato"}}
	lancia := func(inc map[string]any, chi *cliente, azione string, d20 ...int) {
		t.Helper()
		tiriDecisi(t, d20...)
		sc.az(chi, 200, "incantesimo.ts", map[string]any{"combattente": idA, "incantesimo": inc, "bersagli": []string{idZ}, "azione": azione})
		g := sc.vista(sc.m)["incantesimiInAttesa"].([]any)[0].(map[string]any)["gettone"].(string)
		sc.az(sc.m, 200, "incantesimo.applica", map[string]any{"gettone": g, "applicaCondizioni": true, "azione": azione + "-ok"})
	}
	condZ := func() []any { return cerca(sc.vista(sc.m), "Zombi")["condizioni"].([]any) }
	effetti := func() []any { x, _ := sc.vista(sc.m)["effetti"].([]any); return x }

	// Round 1: lo zombi fallisce (2 − 2), è paralizzato; l'effetto è in corso (concentrazione, TS ripetuto, fino al round 11).
	lancia(blocca, sc.luca, "eff-0001", 2)
	if len(condZ()) != 1 || len(effetti()) != 1 {
		t.Fatalf("effetto in corso: %v %v", condZ(), effetti())
	}
	if e := effetti()[0].(map[string]any); e["conc"] != true || e["fineRound"].(float64) != 11 || e["ts"].(map[string]any)["car"] != "SAG" {
		t.Fatalf("dati dell'effetto: %v", e)
	}
	if len(sc.vista(sc.luca)["effetti"].([]any)) != 1 {
		t.Fatal("i giocatori vedono gli effetti sui mostri visibili")
	}
	// Fine del turno di Arin: niente; fine del turno dello zombi: ripete il TS (3 − 2 = 1, fallisce), poi (18 − 2 = 16) si libera.
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	tiriDecisi(t, 3)
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	if len(condZ()) != 1 {
		t.Fatal("TS ripetuto fallito: resta paralizzato")
	}
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	tiriDecisi(t, 18)
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	if len(condZ()) != 0 || len(effetti()) != 0 {
		t.Fatalf("TS ripetuto riuscito: libero, effetto finito: %v %v", condZ(), effetti())
	}
	// Round 3: di nuovo paralizzato; Arin perde la concentrazione (tolta a mano) → lo zombi si libera.
	lancia(blocca, sc.luca, "eff-0002", 2)
	if len(condZ()) != 1 {
		t.Fatal("di nuovo paralizzato")
	}
	sc.az(sc.luca, 200, "pg.condizioni", map[string]any{"combattente": idA, "condizioni": []string{}})
	if len(condZ()) != 0 || len(effetti()) != 0 {
		t.Fatalf("concentrazione persa: %v %v", condZ(), effetti())
	}
	// Durata: trattenuto per 1 round (senza concentrazione) → finisce all'inizio del prossimo turno di Arin.
	lancia(map[string]any{"nome": "Ragnatela breve", "tempo": "altro", "round": 1, "tipo": "ts", "cd": 13, "car": "DES", "condizioni": []string{"Trattenuto"}}, sc.m, "eff-0003", 5)
	if len(condZ()) != 1 || condZ()[0] != "Trattenuto" {
		t.Fatalf("trattenuto: %v", condZ())
	}
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil) // turno dello zombi, round 3
	if len(condZ()) != 1 {
		t.Fatal("dura fino al prossimo turno di Arin")
	}
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil) // round 4, turno di Arin
	if len(condZ()) != 0 {
		t.Fatalf("durata finita: %v", condZ())
	}
	// A 0 PF la concentrazione finisce: Arin (5 PF) colpito dallo zombi per 7.
	lancia(blocca, sc.luca, "eff-0004", 2)
	sc.az(sc.m, 200, "pg.danno", map[string]any{"combattente": idA, "valore": 15})
	tiriDecisi(t, 15, 6)
	e := sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idZ, "bersaglio": idA, "forza": true, "azione": "eff-0005"})
	if e["sconfitto"] != true || len(condZ()) != 0 || len(effetti()) != 0 {
		t.Fatalf("a 0 PF finisce la concentrazione: %v %v %v", e, condZ(), effetti())
	}
	if a := cerca(sc.vista(sc.m), "Arin")["condizioni"].([]any); len(a) != 0 {
		t.Fatalf("Arin non è più concentrato: %v", a)
	}
	// Il Master termina un effetto a mano.
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	sc.az(sc.m, 200, "pg.cura", map[string]any{"combattente": idA, "valore": 20})
	lancia(blocca, sc.luca, "eff-0006", 2)
	id := effetti()[0].(map[string]any)["id"].(string)
	sc.az(sc.luca, 403, "effetto.termina", map[string]any{"effetto": id})
	sc.az(sc.m, 200, "effetto.termina", map[string]any{"effetto": id})
	if len(condZ()) != 0 || len(effetti()) != 0 {
		t.Fatal("effetto terminato dal Master")
	}
}

// Incantesimi dei mostri: slot e usi al giorno spesi dal Master.
func TestIncantesimiDeiMostri(t *testing.T) {
	sc := nuovoScontro(t)
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Mago", "quanti": 1, "ca": 12, "pf": 40,
		"profilo": map[string]any{"incantesimi": map[string]any{"cd": 15, "att": 7, "mod": 3, "liv": 9, "gruppi": []any{
			map[string]any{"etichetta": "Trucchetti", "livello": 0, "usi": 0, "nomi": []string{"raggio di gelo"}},
			map[string]any{"etichetta": "3°", "livello": 3, "usi": 2, "nomi": []string{"palla di fuoco", "controincantesimo"}},
			map[string]any{"etichetta": "1/giorno", "livello": -1, "usi": 1, "nomi": []string{"blocca persone"}},
			map[string]any{"etichetta": "", "livello": 2, "usi": 1, "nomi": []string{"vuoto"}}}}}})
	m := cerca(sc.vista(sc.m), "Mago")
	inc, _ := m["incantesimi"].(map[string]any)
	if inc == nil || len(inc["gruppi"].([]any)) != 3 || inc["cd"].(float64) != 15 {
		t.Fatalf("incantesimi del mostro (senza il gruppo senza nome): %v", m["incantesimi"])
	}
	if cerca(sc.vista(sc.luca), "Mago")["incantesimi"] != nil {
		t.Fatal("gli incantesimi dei mostri li vede solo il Master")
	}
	id := m["id"].(string)
	usa := func(c *cliente, codice, g int, forza bool) map[string]any {
		return sc.az(c, codice, "mostro.incantesimo", map[string]any{"combattente": id, "gruppo": g, "forza": forza})
	}
	if r := usa(sc.m, 200, 0, false); r["rimasti"].(float64) != -1 {
		t.Fatalf("trucchetto: a volontà: %v", r)
	}
	if r := usa(sc.m, 200, 1, false); r["rimasti"].(float64) != 1 {
		t.Fatalf("primo slot di 3°: %v", r)
	}
	usa(sc.m, 200, 1, false)
	if r := usa(sc.m, 200, 1, false); r["serveConferma"] == nil {
		t.Fatalf("slot finiti: serve conferma: %v", r)
	}
	usa(sc.m, 200, 1, true)
	usa(sc.m, 200, 2, false)
	usa(sc.luca, 403, 2, false)
	usa(sc.m, 400, 9, false)
	u := cerca(sc.vista(sc.m), "Mago")["incantesimi"].(map[string]any)["usati"].(map[string]any)
	if u["1"].(float64) != 3 || u["2"].(float64) != 1 {
		t.Fatalf("usati: %v", u)
	}
}

// Difese dei personaggi mandate dalla scheda (2026-11-12, web/regole/difese.js): TS calcolati dalla scheda, vantaggio contro
// le condizioni e contro gli incantesimi, immunità alle condizioni, resistenze solo contro gli attacchi non magici,
// resistenza ai danni degli incantesimi.
func TestDifeseDeiPersonaggi(t *testing.T) {
	// Il riepilogo tiene solo i valori validi.
	var rp riepilogo
	json.Unmarshal([]byte(pulisciRiepilogo(json.RawMessage(`{"saves":{"FOR":0,"SAG":6,"CAR":30,"XYZ":2},
		"condImm":["Spaventato","Concentrato","Indebolimento 1","Volante","Spaventato"],"saveAdv":["Paralizzato"],"magicRes":["SAG","XYZ"],
		"defenses":{"res":["tagliente (non magici)","fuoco","boh"],"imm":["veleno"],"incantesimi":true}}`))), &rp)
	if _, c := rp.TSTot["FOR"]; len(rp.TSTot) != 2 || !c || rp.TSTot["SAG"] != 6 {
		t.Fatalf("TS della scheda: %v", rp.TSTot)
	}
	if !slices.Equal(rp.ImmCond, []string{"Spaventato"}) || !slices.Equal(rp.VantTS, []string{"Paralizzato"}) || !slices.Equal(rp.ResMagia, []string{"SAG"}) {
		t.Fatalf("condizioni e magia: %v %v %v", rp.ImmCond, rp.VantTS, rp.ResMagia)
	}
	if d := rp.Difese; d == nil || len(d.Res) != 2 || d.Res[0] != (difesa{Tipo: "tagliente", NonMagici: true}) || d.Res[1] != (difesa{Tipo: "fuoco"}) ||
		len(d.Imm) != 1 || !d.Incantesimi {
		t.Fatalf("difese: %+v", rp.Difese)
	}
	// Resistenza ai danni degli incantesimi: non si somma a quella per tipo.
	d := difeseCombattente{Res: []difesa{{Tipo: "fuoco"}}, Incantesimi: true}
	for _, c := range []struct {
		tipo        string
		incantesimo bool
		v, atteso   int
	}{{"fuoco", true, 10, 5}, {"tuono", true, 10, 5}, {"tuono", false, 10, 10}, {"", true, 9, 4}, {"fuoco", false, 10, 5}} {
		if v, _ := applicaDifeseDa(c.tipo, c.v, colpoArma{magica: true, incantesimo: c.incantesimo}, d); v != c.atteso {
			t.Errorf("%s (incantesimo %v): %d invece di %d", c.tipo, c.incantesimo, v, c.atteso)
		}
	}

	sc := nuovoScontro(t)
	// Arin: TS su Saggezza +6 (dalla scheda), immune a spaventato, vantaggio contro paralizzato e contro gli incantesimi sui TS
	// mentali; resistenza ai taglienti non magici e ai danni degli incantesimi.
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"lvl": 3},
		"riepilogo": map[string]any{"class": "Paladino", "level": 3, "ac": 16, "hpMax": 40, "initiative": 2, "dex": 14,
			"abilities": map[string]any{"SAG": 10, "COS": 12}, "saveProfs": []string{"SAG"},
			"saves":   map[string]any{"FOR": 2, "DES": 2, "COS": 1, "INT": 0, "SAG": 6, "CAR": 4},
			"condImm": []string{"Spaventato"}, "saveAdv": []string{"Paralizzato"}, "magicRes": []string{"INT", "SAG", "CAR"},
			"defenses": map[string]any{"res": []string{"tagliente (non magici)"}, "incantesimi": true}}})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Bandito", "quanti": 1, "ca": 12, "pf": 11,
		"profilo": map[string]any{"attacchi": []any{
			map[string]any{"nome": "Scimitarra", "bonus": 3, "danni": []any{map[string]any{"dadi": "1d6", "mod": 1, "tipo": "tagliente"}}},
			map[string]any{"nome": "Lama maledetta", "bonus": 3, "magica": true, "danni": []any{map[string]any{"dadi": "1d6", "mod": 1, "tipo": "tagliente"}}}}}})
	vm := sc.vista(sc.m)
	idA, idB := cerca(vm, "Arin")["id"].(string), cerca(vm, "Bandito")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 30})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idB, "valore": 1})
	sc.az(sc.m, 200, "iniz.avvia", nil)

	// Scimitarra (non magica): 15 + 3 colpisce, 5 + 1 = 6 taglienti, metà = 3. Lama maledetta (magica): 6 interi.
	tiriDecisi(t, 15, 5)
	if e := sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idB, "bersaglio": idA, "attacco": 0, "forza": true, "azione": "dif-0001"}); e["totaleDanni"].(float64) != 3 {
		t.Fatalf("scimitarra contro la resistenza ai non magici: %v", e)
	}
	tiriDecisi(t, 15, 5)
	if e := sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idB, "bersaglio": idA, "attacco": 1, "forza": true, "azione": "dif-0002"}); e["totaleDanni"].(float64) != 6 {
		t.Fatalf("lama magica: %v", e)
	}
	if pf := cerca(sc.vista(sc.m), "Arin")["pf"].(float64); pf != 31 {
		t.Fatalf("PF di Arin: %v", pf)
	}

	// Blocca persone (TS su Saggezza CD 15, paralizzato): vantaggio (4 e 9), 9 + 6 = 15, riesce.
	tiriDecisi(t, 4, 9)
	e := sc.az(sc.m, 200, "incantesimo.ts", map[string]any{"combattente": idB, "incantesimo": map[string]any{"nome": "Blocca persone", "tempo": "altro", "tipo": "ts",
		"cd": 15, "car": "SAG", "condizioni": []string{"Paralizzato"}}, "bersagli": []string{idA}, "azione": "dif-0003"})
	if rr := e["righe"].([]any)[0].(map[string]any); rr["bonus"].(float64) != 6 || len(rr["d20"].([]any)) != 2 || rr["totale"].(float64) != 15 ||
		rr["riuscito"] != true || rr["nota"] != "vantaggio contro paralizzato" || rr["resMagia"] != true {
		t.Fatalf("TS contro Blocca persone: %v", rr)
	}
	// Paura (spaventato): fallisce (vantaggio contro gli incantesimi: 2 e 3, 3 + 6 = 9), ma è immune a spaventato.
	tiriDecisi(t, 2, 3)
	e = sc.az(sc.m, 200, "incantesimo.ts", map[string]any{"combattente": idB, "incantesimo": map[string]any{"nome": "Paura", "tempo": "altro", "tipo": "ts",
		"cd": 15, "car": "SAG", "condizioni": []string{"Spaventato"}}, "bersagli": []string{idA}, "azione": "dif-0004"})
	if rr := e["righe"].([]any)[0].(map[string]any); rr["riuscito"] != false || rr["totale"].(float64) != 9 || rr["nota"] != nil ||
		len(rr["immuneA"].([]any)) != 1 || rr["immuneA"].([]any)[0] != "Spaventato" {
		t.Fatalf("TS contro Paura: %v", rr)
	}
	// Onda tonante (TS su Costituzione CD 15, 2d8 tuono): danni 4 + 4 = 8, TS 3 + 1 fallito (niente vantaggio su Costituzione);
	// resistenza ai danni degli incantesimi: 4. Arin da 31 a 27.
	tiriDecisi(t, 4, 4, 3)
	e = sc.az(sc.m, 200, "incantesimo.ts", map[string]any{"combattente": idB, "incantesimo": map[string]any{"nome": "Onda tonante", "tempo": "altro", "tipo": "ts",
		"cd": 15, "car": "COS", "seRiesce": "meta", "danni": []any{map[string]any{"dadi": "2d8", "tipo": "tuono"}}}, "bersagli": []string{idA}, "azione": "dif-0005"})
	if rr := e["righe"].([]any)[0].(map[string]any); rr["riuscito"] != false || len(rr["d20"].([]any)) != 1 || rr["totale"].(float64) != 4 {
		t.Fatalf("TS contro Onda tonante: %v", rr)
	}
	e = sc.az(sc.m, 200, "incantesimo.applica", map[string]any{"gettone": e["gettone"].(string), "azione": "dif-0006"})
	if rr := e["righe"].([]any)[0].(map[string]any); rr["totaleDanni"].(float64) != 4 {
		t.Fatalf("danni dell'Onda tonante: %v", rr)
	}
	if pf := cerca(sc.vista(sc.m), "Arin")["pf"].(float64); pf != 27 {
		t.Fatalf("PF di Arin dopo l'Onda tonante: %v", pf)
	}
}

// Effetti che cambiano i valori (2026-11-12, effetti_valori.go): CA con il valore originale, ira del barbaro, Benedizione,
// Sfocatura, Armatura magica e Pelle coriacea, fine dell'effetto da parte del giocatore.
func TestEffettiCheCambianoIValori(t *testing.T) {
	m := validaModifiche(modificheEffetto{CABonus: 11, CABase: 13, CAMin: 30, Res: []string{"Taglienti", "fuoco (non magici)", "boh"}, ImmCond: []string{"Spaventato", "Concentrato"},
		VantTS: []string{"FOR", "XYZ"}, BonusTS: 9, DadoTS: "-1d4", DadoAtt: "1d5", DanniMischia: 3})
	if m.CABonus != 0 || m.CABase != 13 || m.CAMin != 0 || !slices.Equal(m.Res, []string{"tagliente", "fuoco (non magici)"}) || !slices.Equal(m.ImmCond, []string{"Spaventato"}) ||
		!slices.Equal(m.VantTS, []string{"FOR"}) || m.BonusTS != 0 || m.DadoTS != "-1d4" || m.DadoAtt != "" || m.DanniMischia != 3 {
		t.Fatalf("modifiche validate: %+v", m)
	}
	if x := testoModifiche(modificheEffetto{CABonus: 2, DadoAtt: "1d4", DadoTS: "-1d4"}); x != "CA +2, −1d4 ai TS, +1d4 ai tiri per colpire" {
		t.Fatalf("testo: %q", x)
	}
	d := unisciDifese(difeseCombattente{Res: []difesa{{Tipo: "tagliente", NonMagici: true}}}, []difesa{{Tipo: "tagliente"}, {Tipo: "fuoco"}}, nil)
	if len(d.Res) != 2 || d.Res[0] != (difesa{Tipo: "tagliente"}) {
		t.Fatalf("difese unite: %+v", d)
	}

	sc := nuovoScontro(t)
	// Arin: CA 12, Destrezza 14, TS su Forza +5, ascia bipenne +5 (1d12 + 3), ira dalla scheda.
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"lvl": 3},
		"riepilogo": map[string]any{"class": "Barbaro", "level": 3, "ac": 12, "hpMax": 40, "initiative": 2, "dex": 14,
			"saves":   map[string]any{"FOR": 5, "DES": 2, "COS": 4, "INT": 0, "SAG": 1, "CAR": -1},
			"attacks": []any{map[string]any{"nome": "Ascia bipenne", "bonus": 5, "portata": "1,5 m", "danni": []any{map[string]any{"dadi": "1d12", "mod": 3, "tipo": "tagliente"}}}},
			"rage":    map[string]any{"res": []string{"contundente", "perforante", "tagliente"}, "vantTS": []string{"FOR"}, "danniMischia": 2}}})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Orco", "quanti": 1, "ca": 13, "pf": 60,
		"profilo": map[string]any{"ts": map[string]any{"SAG": -1}, "attacchi": []any{map[string]any{"nome": "Ascia", "bonus": 5, "danni": []any{map[string]any{"dadi": "1d12", "mod": 3, "tipo": "tagliente"}}}}}})
	vm := sc.vista(sc.m)
	idA, idO := cerca(vm, "Arin")["id"].(string), cerca(vm, "Orco")["id"].(string)
	if cerca(vm, "Arin")["ira"] != true {
		t.Fatal("Arin ha l'ira dalla scheda")
	}
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idA, "valore": 30})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": idO, "valore": 1})
	sc.az(sc.m, 200, "iniz.avvia", nil)
	effetto := func(chi *cliente, codice int, comb, bersaglio string, v map[string]any, azione string) map[string]any {
		t.Helper()
		return sc.az(chi, codice, "effetto.nuovo", map[string]any{"combattente": comb, "bersagli": []string{bersaglio}, "valori": v, "azione": azione})
	}

	// Scudo della fede sull'orco (il Master, azione bonus fuori turno): CA 15 con l'originale 13; i giocatori non la vedono.
	effetto(sc.m, 200, idO, idO, map[string]any{"nome": "Scudo della fede", "tempo": "bonus", "conc": true, "round": 100, "mod": map[string]any{"caBonus": 2}}, "val-0001")
	o := cerca(sc.vista(sc.m), "Orco")
	if o["ca"].(float64) != 15 || o["caOrig"].(float64) != 13 || len(o["effettiCA"].([]any)) != 1 || o["condizioni"].([]any)[0] != "Concentrato" {
		t.Fatalf("CA dell'orco con Scudo della fede: %v", o)
	}
	if x := cerca(sc.vista(sc.luca), "Orco"); x["ca"] != nil || x["caOrig"] != nil {
		t.Fatalf("i giocatori non vedono la CA dei mostri: %v", x)
	}
	if e := sc.vista(sc.m)["effetti"].([]any)[0].(map[string]any); e["mod"].(map[string]any)["caBonus"].(float64) != 2 {
		t.Fatalf("modifiche dell'effetto nella vista: %v", e)
	}

	// Arin entra in ira (azione bonus); l'attacco 9 + 5 = 14 manca la CA 15.
	if r := sc.az(sc.luca, 200, "pg.ira", map[string]any{"combattente": idA, "azione": "val-0002"}); r["fatto"] != true {
		t.Fatalf("ira: %v", r)
	}
	if cerca(sc.vista(sc.luca), "Arin")["economia"].(map[string]any)["b"] != true {
		t.Fatal("l'ira usa l'azione bonus")
	}
	tiriDecisi(t, 9)
	if e := sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idO, "attacco": 0, "azione": "val-0003"}); e["risultato"] != "mancato" || e["totale"].(float64) != 14 {
		t.Fatalf("contro la CA 15: %v", e)
	}
	// Il Master fa attaccare ancora Arin: 15 colpisce, 6 + 3 taglienti + 2 dell'ira = 11.
	tiriDecisi(t, 15, 6)
	e := sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idO, "attacco": 0, "forza": true, "azione": "val-0004"})
	if e["totaleDanni"].(float64) != 11 || len(e["danni"].([]any)) != 2 {
		t.Fatalf("danni dell'ira: %v", e)
	}
	// L'orco colpisce Arin (CA 12): 8 + 3 = 11 taglienti, resistenza dell'ira: 5.
	tiriDecisi(t, 15, 8)
	if e := sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idO, "bersaglio": idA, "forza": true, "azione": "val-0005"}); e["totaleDanni"].(float64) != 5 {
		t.Fatalf("resistenza dell'ira: %v", e)
	}
	// Benedizione sull'orco: 4 + 5 + 1d4 (3) = 12 colpisce la CA 12; 4 + 3 = 7, metà 3. Arin da 40 a 32.
	effetto(sc.m, 200, idO, idO, map[string]any{"nome": "Benedizione", "tempo": "altro", "round": 10, "mod": map[string]any{"dadoAtt": "1d4", "dadoTS": "1d4"}}, "val-0006")
	tiriDecisi(t, 4, 3, 4)
	e = sc.az(sc.m, 200, "attacco.tira", map[string]any{"combattente": idO, "bersaglio": idA, "forza": true, "azione": "val-0007"})
	if e["risultato"] != "colpito" || e["totale"].(float64) != 12 || !slices.Contains(e["fonti"].([]any), any("+1d4 Benedizione: 3")) || e["totaleDanni"].(float64) != 3 {
		t.Fatalf("Benedizione al tiro per colpire: %v", e)
	}
	if pf := cerca(sc.vista(sc.m), "Arin")["pf"].(float64); pf != 32 {
		t.Fatalf("PF di Arin: %v", pf)
	}
	// TS su Forza di Arin in ira: vantaggio (3 e 12), 12 + 5 = 17.
	tiriDecisi(t, 3, 12)
	e = sc.az(sc.m, 200, "incantesimo.ts", map[string]any{"combattente": idO, "incantesimo": map[string]any{"nome": "Spinta", "tempo": "altro", "tipo": "ts", "cd": 15, "car": "FOR",
		"condizioni": []string{"Prono"}}, "bersagli": []string{idA}, "azione": "val-0008"})
	if rr := e["righe"].([]any)[0].(map[string]any); rr["totale"].(float64) != 17 || len(rr["d20"].([]any)) != 2 || rr["nota"] != "vantaggio per Ira" {
		t.Fatalf("TS di Arin in ira: %v", rr)
	}
	// TS dell'orco con Benedizione: 5 − 1 + 1d4 (2) = 6.
	tiriDecisi(t, 5, 2)
	e = sc.az(sc.m, 200, "incantesimo.ts", map[string]any{"combattente": idO, "incantesimo": map[string]any{"nome": "Prova", "tempo": "altro", "tipo": "ts", "cd": 10, "car": "SAG",
		"condizioni": []string{"Spaventato"}}, "bersagli": []string{idO}, "azione": "val-0009"})
	if rr := e["righe"].([]any)[0].(map[string]any); rr["totale"].(float64) != 6 || rr["bonus"].(float64) != -1 || rr["nota"] != "+1d4 Benedizione: 2" {
		t.Fatalf("TS con Benedizione: %v", rr)
	}

	// Sfocatura sull'orco; al round 2 Arin attacca con svantaggio (18 e 4): 4 + 5 = 9, mancato.
	effetto(sc.m, 200, idO, idO, map[string]any{"nome": "Sfocatura", "tempo": "altro", "round": 10, "mod": map[string]any{"svantContro": true}}, "val-0010")
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	tiriDecisi(t, 18, 4)
	e = sc.az(sc.luca, 200, "attacco.tira", map[string]any{"combattente": idA, "bersaglio": idO, "attacco": 0, "azione": "val-0011"})
	if e["modo"] != "svantaggio" || e["totale"].(float64) != 9 || !slices.Contains(e["fonti"].([]any), any("svantaggio: Sfocatura su Orco")) {
		t.Fatalf("Sfocatura: %v", e)
	}
	// Arin esce dall'ira; non può terminare gli effetti dell'orco.
	if r := sc.az(sc.luca, 200, "pg.ira", map[string]any{"combattente": idA, "azione": "val-0012"}); r["testo"] != "Arin non è più in ira" {
		t.Fatalf("fine dell'ira: %v", r)
	}
	var idSf string
	for _, x := range sc.vista(sc.m)["effetti"].([]any) {
		if x.(map[string]any)["nome"] == "Ira" {
			t.Fatal("l'ira è finita")
		}
		if x.(map[string]any)["nome"] == "Sfocatura" {
			idSf = x.(map[string]any)["id"].(string)
		}
	}
	sc.az(sc.luca, 403, "effetto.termina", map[string]any{"effetto": idSf})

	// Armatura magica: CA 13 + 2 = 15 (più alta di 12); Pelle coriacea: almeno 16. Arin termina la Pelle coriacea: torna 15.
	effetto(sc.luca, 200, idA, idA, map[string]any{"nome": "Armatura magica", "tempo": "altro", "round": 4800, "mod": map[string]any{"caBase": 13}}, "val-0013")
	if a := cerca(sc.vista(sc.luca), "Arin"); a["ca"].(float64) != 15 || a["caOrig"].(float64) != 12 {
		t.Fatalf("Armatura magica: %v", a)
	}
	// la scheda completa riceve la CA cambiata con l'originale e le modifiche in breve
	if ef, _ := sc.luca.deve(200, "GET", "/personaggi/"+sc.pg, nil)["effetti"].(map[string]any); ef == nil || ef["ca"].(float64) != 15 || ef["caOrig"].(float64) != 12 ||
		len(ef["voci"].([]any)) != 1 || ef["voci"].([]any)[0] != "Armatura magica: CA 13 + Destrezza" {
		t.Fatalf("effetti per la scheda: %v", ef)
	}
	effetto(sc.luca, 200, idA, idA, map[string]any{"nome": "Pelle coriacea", "tempo": "altro", "conc": true, "round": 600, "mod": map[string]any{"caMin": 16}}, "val-0014")
	if a := cerca(sc.vista(sc.luca), "Arin"); a["ca"].(float64) != 16 || len(a["effettiCA"].([]any)) != 2 {
		t.Fatalf("Pelle coriacea: %v", a)
	}
	var idPc string
	for _, x := range sc.vista(sc.luca)["effetti"].([]any) {
		if x.(map[string]any)["nome"] == "Pelle coriacea" {
			idPc = x.(map[string]any)["id"].(string)
		}
	}
	sc.az(sc.luca, 200, "effetto.termina", map[string]any{"effetto": idPc})
	if a := cerca(sc.vista(sc.luca), "Arin"); a["ca"].(float64) != 15 {
		t.Fatalf("dopo la Pelle coriacea: %v", a)
	}
	// Dati non validi: nessuna modifica, nome vuoto.
	effetto(sc.luca, 400, idA, idA, map[string]any{"nome": "Niente", "tempo": "altro", "mod": map[string]any{"caBonus": 99}}, "val-0015")
	effetto(sc.luca, 400, idA, idA, map[string]any{"nome": "", "tempo": "altro", "mod": map[string]any{"caBonus": 1}}, "val-0016")
}
