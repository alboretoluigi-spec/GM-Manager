package server

import (
	"strings"
	"testing"
)

// Archivio degli oggetti magici del Master (2026-11-11): solo il Master, oggetto ripulito, consegna nelle schede con le
// regole delle versioni, eliminazione che non tocca le schede.
func TestArchivioOggettiMagici(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	anello := map[string]any{"id": "ogm-prova", "v": 1, "nome": "  Anello della nebbia ", "tipo": "anello", "rar": "raro", "sint": true,
		"uso": map[string]any{"cariche": 3}, "princ": "Avvolge chi lo porta nella nebbia.", "campoStrano": 1}
	tv.luca.deve(403, "GET", "/oggetti-magici", nil)
	tv.luca.deve(403, "PUT", "/oggetti-magici/ogm-prova", map[string]any{"oggetto": anello})
	tv.luca.deve(403, "POST", "/oggetti-magici/ogm-prova/consegna", map[string]any{"personaggi": []string{pg}})
	tv.m.deve(400, "PUT", "/oggetti-magici/altro-id", map[string]any{"oggetto": anello})
	tv.m.deve(400, "PUT", "/oggetti-magici/ogm-x", map[string]any{"oggetto": map[string]any{"id": "ogm-x", "nome": ""}})
	tv.m.deve(400, "PUT", "/oggetti-magici/ogm-x", map[string]any{"oggetto": map[string]any{"id": "ogm-x", "nome": "X", "img": "javascript:alert(1)"}})
	tv.m.deve(200, "PUT", "/oggetti-magici/ogm-prova", map[string]any{"oggetto": anello})

	l := tv.m.deve(200, "GET", "/oggetti-magici", nil)
	ogg := l["oggetti"].([]any)
	if len(ogg) != 1 {
		t.Fatalf("archivio: %v", l)
	}
	o := ogg[0].(map[string]any)["oggetto"].(map[string]any)
	if o["nome"] != "Anello della nebbia" || o["campoStrano"] != nil || o["princ"] != "Avvolge chi lo porta nella nebbia." {
		t.Fatalf("oggetto ripulito: %v", o)
	}
	if pgs := l["personaggi"].([]any); len(pgs) != 1 || pgs[0].(map[string]any)["id"] != pg || pgs[0].(map[string]any)["giocatore"] != "Luca" {
		t.Fatalf("personaggi a cui consegnare: %v", pgs)
	}

	esito := func() string {
		r := tv.m.deve(200, "POST", "/oggetti-magici/ogm-prova/consegna", map[string]any{"personaggi": []string{pg, pg}})["risultati"].([]any)
		if len(r) != 1 {
			t.Fatalf("risultati: %v", r)
		}
		return r[0].(map[string]any)["esito"].(string)
	}
	voce := func() map[string]any {
		mag, _ := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)["mag"].([]any)
		if len(mag) != 1 {
			t.Fatalf("inventario magico: %v", mag)
		}
		return mag[0].(map[string]any)
	}
	if e := esito(); e != "aggiunto" {
		t.Fatalf("prima consegna: %s", e)
	}
	v := voce()
	if v["o"].(map[string]any)["nome"] != "Anello della nebbia" || v["car"] != float64(3) || v["eq"] != false {
		t.Fatalf("voce consegnata: %v", v)
	}
	if e := esito(); e != "uguale" {
		t.Fatalf("stessa versione: %s", e)
	}
	// nuova versione con meno cariche: si aggiorna, le cariche non superano il nuovo massimo
	anello["v"], anello["uso"] = 2, map[string]any{"cariche": 2}
	r := tv.m.deve(200, "PUT", "/oggetti-magici/ogm-prova", map[string]any{"oggetto": anello})
	if p := r["possessori"].([]any); len(p) != 1 || p[0].(map[string]any)["v"] != float64(1) {
		t.Fatalf("chi lo ha (versione vecchia): %v", r)
	}
	if e := esito(); e != "aggiornato" {
		t.Fatalf("versione nuova: %s", e)
	}
	if v := voce(); v["o"].(map[string]any)["v"] != float64(2) || v["car"] != float64(2) {
		t.Fatalf("voce aggiornata: %v", v)
	}
	anello["v"] = 1
	tv.m.deve(409, "PUT", "/oggetti-magici/ogm-prova", map[string]any{"oggetto": anello})
	// eliminato dall'archivio: nella scheda resta
	tv.m.deve(200, "DELETE", "/oggetti-magici/ogm-prova", nil)
	tv.m.deve(404, "POST", "/oggetti-magici/ogm-prova/consegna", map[string]any{"personaggi": []string{pg}})
	if len(tv.m.deve(200, "GET", "/oggetti-magici", nil)["oggetti"].([]any)) != 0 {
		t.Fatal("l'archivio deve essere vuoto")
	}
	voce()
}

// «Importa dai manuali», oggetti magici (M3.3): nuovo → nell'archivio con manuale e pagina; poi cambiato; se il Master
// lo modifica nell'archivio → a_mano; il testo resta al Master.
func TestImportazioneOggetti(t *testing.T) {
	tv := nuovoTavolo(t)
	_, out := tv.m.caricaManuale(pdfDiProva(2), "Guida del Dungeon Master")
	man := out["id"].(string)
	cand := func(rar string, testo string) []map[string]any {
		return []map[string]any{
			{"tipo": "oggetto", "nome": "Mantello della protezione", "manuale": man, "pagina": 180, "testo": testo,
				"dati": map[string]any{"tipo": "mantello", "rar": rar, "sint": true, "sintDa": ""}},
			{"tipo": "oggetto", "nome": "Rotto", "manuale": man, "pagina": 2, "dati": map[string]any{"tipo": "astronave", "rar": "raro"}},
		}
	}
	out = tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand("non_comune", "Testo del manuale.")})
	if got := strings.Join(statiRiepilogo(out), " "); got != "Mantello della protezione:nuovo Rotto:non_valido" {
		t.Fatalf("stati: %s", got)
	}
	tv.m.deve(200, "POST", "/importazione/applica", map[string]any{"indici": []int{0, 1}})
	l := tv.m.deve(200, "GET", "/oggetti-magici", nil)["oggetti"].([]any)
	if len(l) != 1 {
		t.Fatalf("archivio dopo l'importazione: %v", l)
	}
	x := l[0].(map[string]any)
	o := x["oggetto"].(map[string]any)
	if o["nome"] != "Mantello della protezione" || o["rar"] != "non_comune" || o["sint"] != true || o["slot"] != "mantello" ||
		x["manualeNome"] != "Guida del Dungeon Master" || x["pagina"] != float64(180) {
		t.Fatalf("oggetto importato: %v", x)
	}
	id := x["id"].(string)
	if tt := tv.m.deve(200, "GET", "/elementi-manuali/testo?tipo=oggetto&voce="+id, nil); tt["testo"] != "Testo del manuale." {
		t.Fatalf("testo del manuale: %v", tt)
	}
	// rilettura con la rarità diversa: cambiato; applicata, la versione sale
	out = tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand("raro", "Testo del manuale.")})
	voce := out["voci"].([]any)[0].(map[string]any)
	if voce["stato"] != "cambiato" || voce["voce"] != id || strings.Join(anyStr(voce["campi"]), ",") != "Rarità" {
		t.Fatalf("rilettura: %v", voce)
	}
	tv.m.deve(200, "POST", "/importazione/applica", map[string]any{"indici": []int{0}})
	o = tv.m.deve(200, "GET", "/oggetti-magici/"+id, nil)["oggetto"].(map[string]any)
	if o["rar"] != "raro" || o["v"] != float64(2) {
		t.Fatalf("oggetto aggiornato: %v", o)
	}
	// il Master lo cambia a mano: la prossima lettura non lo spunta («a_mano»)
	o["rar"], o["v"] = "molto_raro", 3
	tv.m.deve(200, "PUT", "/oggetti-magici/"+id, map[string]any{"oggetto": o})
	out = tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand("raro", "Testo del manuale.")})
	if s := out["voci"].([]any)[0].(map[string]any)["stato"]; s != "a_mano" {
		t.Fatalf("modificato a mano: %v", s)
	}
	// un oggetto creato a mano con gli stessi valori: si collega il testo del manuale
	tv.m.deve(200, "PUT", "/oggetti-magici/ogm-a-mano", map[string]any{"oggetto": map[string]any{"id": "ogm-a-mano", "nome": "Stivali elfici", "tipo": "stivali", "rar": "non_comune"}})
	out = tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": []map[string]any{
		{"tipo": "oggetto", "nome": "STIVALI ELFICI", "manuale": man, "pagina": 200, "dati": map[string]any{"tipo": "stivali", "rar": "non_comune"}}}})
	if v := out["voci"].([]any)[0].(map[string]any); v["stato"] != "cambiato" || v["voce"] != "ogm-a-mano" || strings.Join(anyStr(v["campi"]), ",") != "Testo del manuale" {
		t.Fatalf("oggetto creato a mano: %v", v)
	}
	tv.luca.deve(403, "GET", "/elementi-manuali/testo?tipo=oggetto&voce="+id, nil)
}

// «Importa dai manuali», regole (M3.4): solo testo per il Master, una voce per nome; nuovo → uguale → cambiato.
func TestImportazioneRegole(t *testing.T) {
	tv := nuovoTavolo(t)
	_, out := tv.m.caricaManuale(pdfDiProva(2), "Manuale del Giocatore")
	man := out["id"].(string)
	cand := func(testo string) []map[string]any {
		return []map[string]any{
			{"tipo": "regola", "sottotipo": "talento", "nome": "Allerta", "manuale": man, "pagina": 165, "testo": testo, "dati": map[string]any{}},
			{"tipo": "regola", "sottotipo": "condizione", "nome": "Accecato", "manuale": man, "pagina": 290, "testo": "", "dati": map[string]any{}},
			{"tipo": "regola", "sottotipo": "incantesimo", "nome": "Strano", "manuale": man, "pagina": 3, "testo": "x", "dati": map[string]any{}},
		}
	}
	out = tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand("Sempre all'erta.")})
	if got := strings.Join(statiRiepilogo(out), " "); got != "Allerta:nuovo Accecato:non_valido Strano:non_valido" {
		t.Fatalf("stati: %s", got)
	}
	tv.m.deve(200, "POST", "/importazione/applica", map[string]any{"indici": []int{0, 1, 2}})
	if tt := tv.m.deve(200, "GET", "/elementi-manuali/testo?tipo=regola&nome=ALLERTA", nil); tt["testo"] != "Sempre all'erta." || tt["manualeNome"] != "Manuale del Giocatore" {
		t.Fatalf("testo della regola: %v", tt)
	}
	tv.luca.deve(403, "GET", "/elementi-manuali/testo?tipo=regola&nome=Allerta", nil)
	l := tv.m.deve(200, "GET", "/elementi-manuali?tipo=regola", nil)["lista"].([]any)
	if len(l) != 1 || l[0].(map[string]any)["sottotipo"] != "talento" {
		t.Fatalf("elenco delle regole: %v", l)
	}
	if got := strings.Join(statiRiepilogo(tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand("Sempre all'erta.")})), " "); !strings.HasPrefix(got, "Allerta:uguale") {
		t.Fatalf("rilettura uguale: %s", got)
	}
	if got := strings.Join(statiRiepilogo(tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand("Testo nuovo.")})), " "); !strings.HasPrefix(got, "Allerta:cambiato") {
		t.Fatalf("rilettura cambiata: %s", got)
	}
}

// Bonus letti dal testo (2026-11-11, «la scheda non calcola il bonus»): l'oggetto nuovo li riceve; uno già nell'archivio
// senza modificatori risulta «cambiato: Bonus calcolati» e li riceve senza perdere i suoi dati.
func TestImportazioneOggettiConBonus(t *testing.T) {
	tv := nuovoTavolo(t)
	_, out := tv.m.caricaManuale(pdfDiProva(2), "Guida del Dungeon Master")
	man := out["id"].(string)
	mod := []map[string]any{{"par": "CA", "op": "agg", "val": 1, "cond": "sint"}, {"par": "TS", "op": "agg", "val": 1, "cond": "sint"}}
	tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": []map[string]any{
		{"tipo": "oggetto", "nome": "Anello di protezione", "manuale": man, "pagina": 191, "testo": "x", "dati": map[string]any{"tipo": "anello", "rar": "raro", "sint": true, "mod": mod}},
		{"tipo": "oggetto", "nome": "Arma +2", "manuale": man, "pagina": 213, "testo": "y", "dati": map[string]any{"tipo": "arma", "rar": "raro", "bonus": 2}},
	}})
	tv.m.deve(200, "POST", "/importazione/applica", map[string]any{"indici": []int{0, 1}})
	per := map[string]map[string]any{}
	for _, x := range tv.m.deve(200, "GET", "/oggetti-magici", nil)["oggetti"].([]any) {
		o := x.(map[string]any)["oggetto"].(map[string]any)
		per[o["nome"].(string)] = o
	}
	if l, _ := per["Anello di protezione"]["mod"].([]any); len(l) != 2 || l[0].(map[string]any)["par"] != "CA" {
		t.Fatalf("modificatori dell'anello: %v", per["Anello di protezione"])
	}
	if a, _ := per["Arma +2"]["arma"].(map[string]any); a == nil || a["bAtt"] != float64(2) || a["bDanno"] != float64(2) {
		t.Fatalf("bonus dell'arma: %v", per["Arma +2"])
	}
	// un oggetto creato a mano senza modificatori: la lettura li propone
	tv.m.deve(200, "PUT", "/oggetti-magici/ogm-amuleto", map[string]any{"oggetto": map[string]any{"id": "ogm-amuleto", "nome": "Amuleto della salute", "tipo": "amuleto", "rar": "raro", "sint": true, "princ": "Scritto dal Master"}})
	out = tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": []map[string]any{
		{"tipo": "oggetto", "nome": "Amuleto della salute", "manuale": man, "pagina": 150, "testo": "z", "dati": map[string]any{"tipo": "amuleto", "rar": "raro", "sint": true,
			"mod": []map[string]any{{"par": "COS", "op": "sost", "val": 19, "cond": "sint"}}}}}})
	v := out["voci"].([]any)[0].(map[string]any)
	if v["stato"] != "cambiato" || strings.Join(anyStr(v["campi"]), ",") != "Bonus calcolati,Testo del manuale" {
		t.Fatalf("oggetto senza bonus: %v", v)
	}
	tv.m.deve(200, "POST", "/importazione/applica", map[string]any{"indici": []int{0}})
	o := tv.m.deve(200, "GET", "/oggetti-magici/ogm-amuleto", nil)["oggetto"].(map[string]any)
	if l, _ := o["mod"].([]any); len(l) != 1 || o["princ"] != "Scritto dal Master" || o["v"] != float64(2) {
		t.Fatalf("amuleto aggiornato: %v", o)
	}
}
