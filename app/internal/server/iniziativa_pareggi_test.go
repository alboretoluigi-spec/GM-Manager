package server

import (
	"encoding/json"
	"strings"
	"testing"
)

// Richieste dell'utente del 2026-11-12: ogni mostro di un gruppo tira la sua iniziativa; 20 naturale primo, 1 naturale
// ultimo; pareggio PG-mostro deciso dal Master; pareggio tra PG deciso dai giocatori; scheda intera del mostro.
func TestIniziativaNaturaliEPareggi(t *testing.T) {
	sc := nuovoScontro(t) // Arin di Luca (bonus +2)
	// Mira di Sara nello stesso scontro.
	mira := sc.sara.deve(200, "POST", "/personaggi", map[string]any{"nome": "Mira"})["id"].(string)
	sc.sara.deve(200, "PATCH", "/personaggi/"+mira+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"lvl": 2},
		"riepilogo": map[string]any{"class": "Ladro", "level": 2, "ac": 14, "hpMax": 14, "initiative": 3, "dex": 16}})
	sc.az(sc.m, 200, "iniz.aggiungi_pg", map[string]any{"personaggio": mira})
	ordine := func(c *cliente) []string {
		var l []string
		for _, x := range combattenti(sc.vista(c)) {
			l = append(l, x["nome"].(string))
		}
		return l
	}

	// Tre lupi: ognuno il suo d20 (5, 12, 19) + 2. La scheda intera del lupo resta per la sua finestra.
	tiriDecisi(t, 5, 12, 19)
	scheda := map[string]any{"taglia": "Media", "tipo": "bestia", "ca": "13", "pf": "11 (2d8+2)", "tratti": []any{map[string]any{"n": "Tattiche del branco", "d": "…"}}}
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Lupo", "quanti": 3, "ca": 13, "pf": 11, "bonusIniziativa": 2, "tira": true, "scheda": scheda})
	vm := sc.vista(sc.m)
	ini := map[string]float64{}
	for _, n := range []string{"Lupo 1", "Lupo 2", "Lupo 3"} {
		ini[n] = cerca(vm, n)["iniziativa"].(float64)
	}
	if ini["Lupo 1"] != 7 || ini["Lupo 2"] != 14 || ini["Lupo 3"] != 21 {
		t.Fatalf("ogni lupo tira la sua iniziativa: %v", ini)
	}
	id := func(n string) string { return cerca(sc.vista(sc.m), n)["id"].(string) }
	// la scheda intera: al Master sì, ai giocatori no
	r := sc.m.deve(200, "GET", "/sessioni/"+sc.ses+"/combattenti/"+id("Lupo 2")+"/scheda", nil)
	if b, _ := json.Marshal(r["scheda"]); !strings.Contains(string(b), "Tattiche del branco") {
		t.Fatalf("scheda del lupo: %v", r)
	}
	sc.luca.deve(403, "GET", "/sessioni/"+sc.ses+"/combattenti/"+id("Lupo 2")+"/scheda", nil)

	// 20 naturale: Arin 20 + 2 = 22, davanti anche a un valore più alto scritto a mano (Lupo 3 a 25).
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": id("Lupo 3"), "valore": 25})
	tiriDecisi(t, 20)
	sc.az(sc.luca, 200, "iniz.iniziativa", map[string]any{"combattente": id("Arin"), "tira": true})
	if o := ordine(sc.m); o[0] != "Arin" || o[1] != "Lupo 3" {
		t.Fatalf("20 naturale primo: %v", o)
	}
	if a := cerca(sc.vista(sc.m), "Arin"); a["naturale"].(float64) != 20 {
		t.Fatalf("naturale nella vista: %v", a)
	}
	// 1 naturale: Lupo 1 tira 1 + 2 = 3, ultimo anche dietro a Mira scritta a mano a 2 (con il naturale indicato: 4).
	tiriDecisi(t, 1)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": id("Lupo 1"), "tira": true})
	sc.az(sc.sara, 200, "iniz.iniziativa", map[string]any{"combattente": id("Mira"), "valore": 2})
	if o := ordine(sc.m); o[len(o)-1] != "Lupo 1" || o[len(o)-2] != "Mira" {
		t.Fatalf("1 naturale ultimo: %v", o)
	}
	// anche scritto a mano con il naturale indicato
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": id("Lupo 1"), "valore": 30, "naturale": 1})
	if o := ordine(sc.m); o[len(o)-1] != "Lupo 1" {
		t.Fatalf("1 naturale scritto a mano: %v", o)
	}

	// Pareggio tra Mira (PG) e Lupo 2 (mostro) a 14: decide il Master; Sara lo vede ma non decide.
	sc.az(sc.sara, 200, "iniz.iniziativa", map[string]any{"combattente": id("Mira"), "valore": 14})
	p := sc.vista(sc.m)["pareggi"].([]any)
	if len(p) != 1 || p[0].(map[string]any)["chi"] != "master" || p[0].(map[string]any)["decide"] != true {
		t.Fatalf("pareggio PG-mostro per il Master: %v", p)
	}
	ps := sc.vista(sc.sara)["pareggi"].([]any)
	if len(ps) != 1 || ps[0].(map[string]any)["decide"] == true {
		t.Fatalf("Sara vede il pareggio ma non decide: %v", ps)
	}
	if sc.vista(sc.luca)["pareggi"] != nil {
		t.Fatal("Luca non è nel pareggio")
	}
	// «Inizia» non parte con un pareggio da decidere
	if r := sc.az(sc.m, 200, "iniz.avvia", nil); r["pareggi"] == nil {
		t.Fatalf("avvio con un pareggio: %v", r)
	}
	sc.az(sc.sara, 403, "iniz.ordine_pari", map[string]any{"ordine": []string{id("Mira"), id("Lupo 2")}})
	sc.az(sc.m, 200, "iniz.ordine_pari", map[string]any{"ordine": []string{id("Lupo 2"), id("Mira")}})
	o := ordine(sc.m)
	if strings.Join(o, ",") != "Arin,Lupo 3,Lupo 2,Mira,Lupo 1" {
		t.Fatalf("ordine dopo la decisione del Master: %v", o)
	}
	if sc.vista(sc.m)["pareggi"] != nil {
		t.Fatal("pareggio deciso")
	}

	// Pareggio tra Arin e Mira (solo PG) a 17: decidono i giocatori; il primo che sceglie vale per tutti.
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": id("Arin"), "valore": 17})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": id("Mira"), "valore": 17})
	pl := sc.vista(sc.luca)["pareggi"].([]any)
	if len(pl) != 1 || pl[0].(map[string]any)["chi"] != "giocatori" || pl[0].(map[string]any)["decide"] != true {
		t.Fatalf("pareggio tra PG per i giocatori: %v", pl)
	}
	if ps := sc.vista(sc.sara)["pareggi"].([]any); ps[0].(map[string]any)["decide"] != true {
		t.Fatalf("anche Sara decide: %v", ps)
	}
	sc.az(sc.luca, 200, "iniz.ordine_pari", map[string]any{"ordine": []string{id("Mira"), id("Arin")}})
	o = ordine(sc.sara)
	if strings.Index(strings.Join(o, ","), "Mira") > strings.Index(strings.Join(o, ","), "Arin") {
		t.Fatalf("Mira prima di Arin: %v", o)
	}
	// un ordine con combattenti diversi dal pareggio non vale
	sc.az(sc.luca, 409, "iniz.ordine_pari", map[string]any{"ordine": []string{id("Arin"), id("Lupo 3")}})
	// un nuovo valore di Arin: il pareggio non c'è più; di nuovo 17 → si decide di nuovo
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": id("Arin"), "valore": 17})
	if sc.vista(sc.m)["pareggi"] == nil {
		t.Fatal("con il nuovo tiro il pareggio si decide di nuovo")
	}
	// il Master può partire lo stesso (vale la Destrezza)
	if r := sc.az(sc.m, 200, "iniz.avvia", map[string]any{"forza": true}); r["pareggi"] != nil {
		t.Fatalf("avvio forzato: %v", r)
	}
	var reg []string // il registro intero (il tracker mostra le ultime 30 righe)
	rows, _ := sc.a.s.DB.Query(`SELECT testo FROM registro_combattimento ORDER BY id`)
	for rows.Next() {
		var x string
		rows.Scan(&x)
		reg = append(reg, x)
	}
	rows.Close()
	r2 := strings.Join(reg, "\n")
	for _, x := range []string{"20 naturale: agisce per primo", "1 naturale: agisce per ultimo", "pareggio a 14: Lupo 2, poi Mira (deciso da Master)", "pareggio a 17: Mira, poi Arin (deciso da Luca)"} {
		if !strings.Contains(r2, x) {
			t.Fatalf("registro senza %q:\n%s", x, r2)
		}
	}
}
