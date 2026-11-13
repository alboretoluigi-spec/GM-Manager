package server

import (
	"database/sql"
	"strings"
	"testing"
)

func schedaProva(ca, pf string) map[string]any {
	return map[string]any{"taglia": "Media", "tipo": "bestia", "all": "senza allineamento", "ca": ca, "pf": pf, "vel": "12 m",
		"car": []int{12, 15, 12, 3, 12, 6}, "ts": "", "abil": "Percezione +3", "vuln": "", "res": "", "imm": "", "immc": "",
		"sensi": "Saggezza (Percezione) passiva 13", "ling": "", "gs": "1/4 (50 PE)",
		"tratti":   []map[string]string{{"n": "Olfatto acuto", "d": "Testo letto dal manuale."}},
		"azioni":   []map[string]string{{"n": "Morso", "d": "Attacco con arma da mischia: +4 al tiro per colpire."}},
		"reazioni": []any{}, "legg": []any{}}
}

func statiRiepilogo(out map[string]any) []string {
	var st []string
	for _, v := range out["voci"].([]any) {
		m := v.(map[string]any)
		st = append(st, m["nome"].(string)+":"+m["stato"].(string))
	}
	return st
}

// «Importa dai manuali» (M3.1): confronto con il bestiario, voci modificate a mano protette, applicazione delle voci
// scelte, provenienza ricordata anche dopo l'eliminazione del manuale, sostituzione del PDF.
func TestImportazioneManuali(t *testing.T) {
	tv := nuovoTavolo(t)
	_, out := tv.m.caricaManuale(pdfDiProva(2), "Manuale dei Mostri")
	man := out["id"].(string)
	tv.m.deve(200, "PATCH", "/manuali/"+man, map[string]any{"condivisoTutti": true})

	// Il Bandito l'aveva già compilato a mano il Master.
	tv.m.deve(200, "PUT", "/bestiario/voci/mm-bandit/scheda", map[string]any{"scheda": schedaProva("12", "11 (2d8 + 2)")})

	cand := []map[string]any{
		{"tipo": "mostro", "nome": "LUPO", "manuale": man, "pagina": 341, "dati": schedaProva("14 (armatura naturale)", "11 (2d8 + 2)"), "testo": "Lupo. Bestia Media..."},
		{"tipo": "mostro", "nome": "Strano essere della nebbia", "manuale": man, "pagina": 12, "dati": schedaProva("14", "30 (4d8 + 12)")},
		{"tipo": "mostro", "nome": "Bandito", "manuale": man, "pagina": 343, "dati": schedaProva("12 (armatura di cuoio)", "11 (2d8 + 2)")},
		{"tipo": "mostro", "nome": "Rotto", "manuale": man, "pagina": 5, "dati": map[string]any{"campo_sconosciuto": 1}},
		{"tipo": "mostro", "nome": "Lupo", "manuale": man, "pagina": 400, "dati": schedaProva("13", "11")},
	}
	for _, g := range []*cliente{tv.luca} {
		g.deve(403, "POST", "/importazione/confronta", map[string]any{"candidati": cand})
		g.deve(403, "GET", "/importazione", nil)
		g.deve(403, "GET", "/elementi-manuali", nil)
	}
	out = tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand})
	if got := strings.Join(statiRiepilogo(out), " "); got != "LUPO:cambiato Strano essere della nebbia:nuovo Bandito:a_mano Rotto:non_valido Lupo:cambiato" {
		t.Fatalf("stati: %s", got)
	}
	voci := out["voci"].([]any)
	lupo := voci[0].(map[string]any)
	if lupo["voce"] != "mm-wolf" || lupo["manualeNome"] != "Manuale dei Mostri" || !strings.Contains(strings.Join(anyStr(lupo["campi"]), ","), "CA") {
		t.Fatalf("lupo: %v", lupo)
	}
	if voci[4].(map[string]any)["doppio"] != "Manuale dei Mostri, p. 341" {
		t.Fatalf("doppio: %v", voci[4])
	}
	// Il riepilogo resta consultabile.
	if len(tv.m.deve(200, "GET", "/importazione", nil)["voci"].([]any)) != 5 {
		t.Fatal("riepilogo salvato")
	}

	// Il Master conferma il Lupo e la creatura nuova (e prova a confermare quella non valida: si salta).
	res := tv.m.deve(200, "POST", "/importazione/applica", map[string]any{"indici": []int{0, 1, 3}})
	if res["applicate"] != float64(2) || res["saltate"] != float64(1) {
		t.Fatalf("applica: %v", res)
	}
	var sc string
	var manID sql.NullString
	var pag int
	tv.a.s.DB.QueryRow(`SELECT scheda_dati, manuale_id, pagina FROM bestiario_personalizzazioni WHERE voce_id = 'mm-wolf'`).Scan(&sc, &manID, &pag)
	if !strings.Contains(sc, "14 (armatura naturale)") || manID.String != man || pag != 341 {
		t.Fatalf("lupo nel bestiario: %s %v %d", sc, manID, pag)
	}
	// Provenienza nella scheda, pagina del libro (calibrazione 1 = 1) e pagina del PDF nell'indice del manuale.
	if !strings.Contains(sc, "Importata da «Manuale dei Mostri», pagina 341 del PDF") {
		t.Fatalf("nota di provenienza: %s", sc)
	}
	var ind string
	tv.a.s.DB.QueryRow(`SELECT indice FROM manuali WHERE id = ?`, man).Scan(&ind)
	if !strings.Contains(ind, `"mm-wolf":341`) {
		t.Fatalf("indice del manuale: %s", ind)
	}
	var nuova string
	tv.a.s.DB.QueryRow(`SELECT id FROM bestiario_voci WHERE nome = 'Strano essere della nebbia'`).Scan(&nuova)
	if nuova == "" {
		t.Fatal("creatura nuova non aggiunta al bestiario")
	}
	// Il Bandito modificato a mano non è stato toccato.
	tv.a.s.DB.QueryRow(`SELECT scheda_dati FROM bestiario_personalizzazioni WHERE voce_id = 'mm-bandit'`).Scan(&sc)
	if strings.Contains(sc, "armatura di cuoio") {
		t.Fatal("la scheda scritta a mano dal Master non va toccata senza conferma")
	}
	// Applicare due volte la stessa voce non la rifà.
	if res := tv.m.deve(200, "POST", "/importazione/applica", map[string]any{"indici": []int{0}}); res["applicate"] != float64(0) {
		t.Fatalf("doppia applicazione: %v", res)
	}

	// Seconda importazione: il Lupo ora è uguale; se poi il Master lo corregge a mano diventa «a_mano».
	// Confermata prima della nota di provenienza: stessi valori, ma si propone di aggiungerla.
	tv.a.s.DB.Exec(`UPDATE bestiario_personalizzazioni SET scheda_dati = json_remove(scheda_dati, '$.nota') WHERE voce_id = 'mm-wolf'`)
	var senza string
	tv.a.s.DB.QueryRow(`SELECT scheda_dati FROM bestiario_personalizzazioni WHERE voce_id = 'mm-wolf'`).Scan(&senza)
	tv.a.s.DB.Exec(`UPDATE elementi_manuali SET impronta = ? WHERE voce = 'mm-wolf'`, improntaDati(senza))
	if out := tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand[:1]}); statiRiepilogo(out)[0] != "LUPO:cambiato" ||
		strings.Join(anyStr(out["voci"].([]any)[0].(map[string]any)["campi"]), ",") != "Provenienza" {
		t.Fatalf("provenienza mancante: %v", out["voci"])
	}
	tv.m.deve(200, "POST", "/importazione/applica", map[string]any{"indici": []int{0}})
	if st := statiRiepilogo(tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand[:1]})); st[0] != "LUPO:uguale" {
		t.Fatalf("dopo l'importazione: %v", st)
	}
	tv.m.deve(200, "PUT", "/bestiario/voci/mm-wolf/scheda", map[string]any{"scheda": schedaProva("15", "20")})
	if st := statiRiepilogo(tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand[:1]})); st[0] != "LUPO:a_mano" {
		t.Fatalf("corretto a mano: %v", st)
	}

	// Elementi importati con la provenienza; il testo solo chiedendolo.
	el := tv.m.deve(200, "GET", "/elementi-manuali?tipo=mostro", nil)["lista"].([]any)
	if len(el) != 2 {
		t.Fatalf("elementi: %v", el)
	}
	var idLupo string
	for _, e := range el {
		m := e.(map[string]any)
		if m["nome"] == "LUPO" {
			idLupo = m["id"].(string)
		}
	}
	if d := tv.m.deve(200, "GET", "/elementi-manuali?id="+idLupo, nil); d["testo"] != "Lupo. Bestia Media..." || d["pagina"] != float64(341) {
		t.Fatalf("testo dell'elemento: %v", d)
	}

	// «🔁 Sostituisci il PDF»: stesso manuale (condivisione compresa), file nuovo.
	var fileVecchio string
	tv.a.s.DB.QueryRow(`SELECT file FROM manuali WHERE id = ?`, man).Scan(&fileVecchio)
	if code, _, b := tv.luca.grezzo("PUT", "/manuali/"+man+"/file", pdfDiProva(3), "application/pdf"); code != 403 {
		t.Fatalf("un giocatore sostituisce il PDF: %d %s", code, b)
	}
	if code, _, _ := tv.m.grezzo("PUT", "/manuali/"+man+"/file", []byte("non pdf"), "application/pdf"); code != 415 {
		t.Fatalf("non PDF: %d", code)
	}
	if code, _, b := tv.m.grezzo("PUT", "/manuali/"+man+"/file?file=MM2.pdf", pdfDiProva(3), "application/pdf"); code != 200 {
		t.Fatalf("sostituzione: %d %s", code, b)
	}
	var fileNuovo string
	var tutti int
	tv.a.s.DB.QueryRow(`SELECT file, condiviso_tutti FROM manuali WHERE id = ?`, man).Scan(&fileNuovo, &tutti)
	if fileNuovo == fileVecchio || tutti != 1 {
		t.Fatalf("dopo la sostituzione: %s %d", fileNuovo, tutti)
	}
	if code, b := tv.luca.scaricaParte("/manuali/"+man+"/file", ""); code != 200 || len(b) == 0 {
		t.Fatalf("il giocatore legge il PDF nuovo: %d", code)
	}

	// Eliminando il manuale gli elementi restano, con il suo nome.
	tv.m.deve(200, "DELETE", "/manuali/"+man, nil)
	el = tv.m.deve(200, "GET", "/elementi-manuali", nil)["lista"].([]any)
	if len(el) != 2 || el[0].(map[string]any)["eliminato"] != true || el[0].(map[string]any)["manualeNome"] != "Manuale dei Mostri" {
		t.Fatalf("dopo l'eliminazione del manuale: %v", el)
	}
}

func anyStr(v any) []string {
	var out []string
	l, _ := v.([]any)
	for _, x := range l {
		out = append(out, x.(string))
	}
	return out
}

func TestDistanzaNomi(t *testing.T) {
	for _, c := range []struct {
		a, b   string
		max, d int
	}{{"lupo", "lupo", 1, 0}, {"drago dj rgento antico", "drago d argento antico", 3, 2}, {"lupo", "lupi", 1, 1}, {"lupo", "orso bruno", 2, 3}} {
		if got := distanzaNomi(c.a, c.b, c.max); got != c.d {
			t.Errorf("%q → %q: %d invece di %d", c.a, c.b, got, c.d)
		}
	}
}

// Nome preso dall'intestazione della pagina («nome indovinato»): se coincide con una creatura del bestiario non è un dubbio.
func TestImportazioneNomeDallIntestazione(t *testing.T) {
	tv := nuovoTavolo(t)
	cand := []map[string]any{
		{"tipo": "mostro", "nome": "Lupo mannaro", "pagina": 1, "dati": schedaProva("12", "58 (9d8 + 18)"), "incerto": true, "motivi": []string{"nome indovinato"}},
		{"tipo": "mostro", "nome": "Lupo manaro", "pagina": 2, "dati": schedaProva("12", "58 (9d8 + 18)"), "incerto": true, "motivi": []string{"nome indovinato"}},
		{"tipo": "mostro", "nome": "Lupo mannaro", "pagina": 3, "dati": schedaProva("12", "58"), "incerto": true, "motivi": []string{"nome indovinato", "caratteristiche non lette"}},
	}
	voci := tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand})["voci"].([]any)
	attesi := []bool{false, true, true} // esatto: non dubbio; simile (OCR): resta da controllare; altri motivi: resta
	for i, v := range voci {
		m := v.(map[string]any)
		if inc, _ := m["incerto"].(bool); inc != attesi[i] || m["voce"] != "mm-werewolf" {
			t.Fatalf("voce %d: incerto %v, voce %v", i, m["incerto"], m["voce"])
		}
	}
}

// Pagina del libro dalla pagina del PDF, con la calibrazione del manuale (anche con due pagine del libro per pagina).
func TestPaginaLibro(t *testing.T) {
	tv := nuovoTavolo(t)
	_, out := tv.m.caricaManuale(pdfDiProva(1), "Calibrato")
	id := out["id"].(string)
	for _, c := range []struct{ refLibro, refPdf, doppia, pdf, libro int }{{1, 1, 0, 341, 341}, {5, 9, 0, 20, 16}, {10, 6, 1, 8, 14}, {50, 60, 0, 3, 3}} {
		tv.a.s.DB.Exec(`UPDATE manuali SET ref_libro = ?, ref_pdf = ?, doppia = ? WHERE id = ?`, c.refLibro, c.refPdf, c.doppia, id)
		tx, _ := tv.a.s.DB.Begin()
		got := paginaLibro(tx, id, c.pdf)
		tx.Rollback()
		if got != c.libro {
			t.Errorf("%+v: pagina del libro %d", c, got)
		}
	}
}

// Incantesimi (M3.2): confronto con l'archivio della scheda, applicazione, valori a tutti ma testo solo al Master.
func TestImportazioneIncantesimi(t *testing.T) {
	tv := nuovoTavolo(t)
	_, out := tv.m.caricaManuale(pdfDiProva(1), "Manuale del Giocatore")
	man := out["id"].(string)
	base := map[string]any{"voce": "Fireball", "l": 3, "sc": "V", "t": "a", "r": "45 m", "cp": "VSM", "d": "I", "cl": "sm"}
	pf := map[string]any{"l": 3, "sc": "V", "t": "a", "r": "45 m", "cp": "VSM", "d": "I", "cl": "sm", "hi": "SEGRETO-SUPERIORI", "mat": "SEGRETO-MATERIALI"}
	cand := []map[string]any{
		{"tipo": "incantesimo", "nome": "Palla di Fuoco", "manuale": man, "pagina": 241, "dati": pf, "testo": "SEGRETO-TESTO della palla di fuoco", "base": base},
		{"tipo": "incantesimo", "nome": "Dardo Incantato", "manuale": man, "pagina": 233, "dati": map[string]any{"l": 1, "sc": "V", "t": "a", "r": "40 m", "cp": "VS", "d": "I", "cl": "smx"},
			"testo": "Tre dardi.", "base": map[string]any{"voce": "Magic Missile", "l": 1, "sc": "V", "t": "a", "r": "36 m", "cp": "VS", "d": "I", "cl": "sm"}},
		{"tipo": "incantesimo", "nome": "Rito della Nebbia", "manuale": man, "pagina": 300, "dati": map[string]any{"l": 2, "sc": "C", "t": "*10m", "r": "Sé", "cp": "V", "d": "1h", "cl": "dw"}, "testo": "Nebbia."},
		{"tipo": "incantesimo", "nome": "Sbagliato", "manuale": man, "pagina": 1, "dati": map[string]any{"l": 3, "sc": "X"}},
	}
	tv.luca.deve(403, "POST", "/importazione/confronta", map[string]any{"candidati": cand})
	out = tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand})
	if got := strings.Join(statiRiepilogo(out), " "); got != "Palla di Fuoco:cambiato Dardo Incantato:cambiato Rito della Nebbia:nuovo Sbagliato:non_valido" {
		t.Fatalf("stati: %s", got)
	}
	voci := out["voci"].([]any)
	if c := strings.Join(anyStr(voci[0].(map[string]any)["campi"]), ","); c != "Testo del manuale" {
		t.Fatalf("palla di fuoco: %s", c)
	}
	if c := strings.Join(anyStr(voci[1].(map[string]any)["campi"]), ","); c != "Gittata,Testo del manuale" {
		t.Fatalf("dardo incantato (la «x» non è una classe): %s", c)
	}
	if res := tv.m.deve(200, "POST", "/importazione/applica", map[string]any{"indici": []int{0, 1, 2, 3}}); res["applicate"] != float64(3) {
		t.Fatalf("applica: %v", res)
	}

	// Valori a tutti i giocatori, senza testo del manuale.
	code, _, body := tv.luca.grezzo("GET", "/regole/incantesimi", nil, "")
	if code != 200 || strings.Contains(string(body), "SEGRETO") {
		t.Fatalf("valori ai giocatori: %d %s", code, body)
	}
	lista := tv.luca.deve(200, "GET", "/regole/incantesimi", nil)["lista"].([]any)
	per := map[string]map[string]any{}
	for _, x := range lista {
		m := x.(map[string]any)
		per[m["voce"].(string)] = m
	}
	if per["Magic Missile"]["r"] != "40 m" || per["Fireball"]["fonte"] != "Manuale del Giocatore" || per["man:rito della nebbia"]["nuovo"] != true || per["man:rito della nebbia"]["t"] != "*10m" {
		t.Fatalf("valori: %v", per)
	}
	// Il testo solo al Master.
	tv.luca.deve(403, "GET", "/elementi-manuali/testo?tipo=incantesimo&voce=Fireball", nil)
	d := tv.m.deve(200, "GET", "/elementi-manuali/testo?tipo=incantesimo&voce=Fireball", nil)
	if d["testo"] != "SEGRETO-TESTO della palla di fuoco" || d["superiori"] != "SEGRETO-SUPERIORI" || d["pagina"] != float64(241) {
		t.Fatalf("testo al Master: %v", d)
	}
	if d := tv.m.deve(200, "GET", "/elementi-manuali/testo?tipo=incantesimo&nome=Rito%20della%20Nebbia", nil); d["testo"] != "Nebbia." {
		t.Fatalf("testo per nome: %v", d)
	}
	// Seconda importazione uguale: niente da fare.
	if st := statiRiepilogo(tv.m.deve(200, "POST", "/importazione/confronta", map[string]any{"candidati": cand[:1]})); st[0] != "Palla di Fuoco:uguale" {
		t.Fatalf("seconda importazione: %v", st)
	}
}
