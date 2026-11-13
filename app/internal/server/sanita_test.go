package server

import (
	"encoding/json"
	"strings"
	"testing"
)

// preparaSanita: Luca con il PG Arin (Saggezza 16, Sanità 14, Costituzione 12, Carisma 8, livello 5).
func preparaSanita(t *testing.T) (*tavolo, string, *Cliente, *Cliente, *Cliente) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"cls": "chierico"},
		"riepilogo": map[string]any{"name": "Arin", "level": 5, "ac": 15, "hpMax": 30, "abilities": map[string]any{"FOR": 10, "DES": 12, "COS": 12, "INT": 10, "SAG": 18, "CAR": 8},
			"saveProfs": []string{"SAG", "CAR"}}})
	// Sanità mentale = 10 + mod Saggezza: con Saggezza 18 vale 14 (modificatore +2).
	c := configPredefinita()
	tv.m.deve(200, "PUT", "/sanita/config", c)
	return tv, pg, tv.clienteWS(tv.m), tv.clienteWS(tv.luca), tv.clienteWS(tv.sara)
}

func attivaTerrore(tv *tavolo) {
	c := configPredefinita()
	c.Terrore = true
	tv.m.deve(200, "PUT", "/sanita/config", c)
}

func evento(tv *tavolo, pg string, intensita int, extra map[string]any) string {
	d := map[string]any{"personaggi": []string{pg}, "fonte": "l'antagonista alla finestra", "intensita": intensita}
	for k, v := range extra {
		d[k] = v
	}
	return tv.m.deve(200, "POST", "/sanita/eventi", d)["eventi"].([]any)[0].(string)
}

func soccombi(t *testing.T, tv *tavolo, c *Cliente, ev string, seq ...int) Risoluzione {
	t.Helper()
	vero := tiraDado
	tiraDado = sequenza(seq...)
	defer func() { tiraDado = vero }()
	r, err := tv.ws(c, "sanita.soccombi", map[string]any{"evento": ev, "chiave": "k1"})
	if err != nil {
		t.Fatalf("soccombi: %v", err)
	}
	var out Risoluzione
	switch x := r.(type) {
	case *Risoluzione:
		out = *x
	case json.RawMessage:
		json.Unmarshal(x, &out)
	}
	return out
}

func TestSanitaCompatibilitaEPermessi(t *testing.T) {
	tv, pg, _, cl, _ := preparaSanita(t)
	// Personaggio esistente senza dati di Sanità: valori predefiniti sicuri.
	v := tv.luca.deve(200, "GET", "/sanita/pg/"+pg, nil)
	if v["terrore"] != 0.0 || v["competente"] != false || v["punteggio"] != 14.0 || v["mod"] != 2.0 || v["bonusTS"] != 2.0 || v["evento"] != nil {
		t.Fatalf("valori predefiniti: %v", v)
	}
	// AUTH-01: il giocatore non crea eventi, non cambia CD, Terrore, competenza o impostazioni.
	if code, _ := tv.luca.chiama("POST", "/sanita/eventi", map[string]any{"personaggi": []string{pg}, "fonte": "x", "intensita": 1}); code != 403 {
		t.Fatal("AUTH-01: il giocatore non crea eventi")
	}
	ev := evento(tv, pg, 1, nil)
	for _, r := range []struct {
		m, p string
		d    any
	}{{"PATCH", "/sanita/eventi/" + ev, map[string]any{"cd": 5}}, {"POST", "/sanita/pg/" + pg + "/terrore", map[string]any{"tipo": "manuale", "valore": 0, "motivo": "x"}},
		{"PUT", "/sanita/pg/" + pg, map[string]any{"competente": true}}, {"PUT", "/sanita/config", configPredefinita()}} {
		if code, _ := tv.luca.chiama(r.m, r.p, r.d); code != 403 {
			t.Fatalf("AUTH-01: %s %s permesso al giocatore", r.m, r.p)
		}
	}
	// Un altro giocatore non vede la Sanità di Arin e non tira per lui.
	if code, _ := tv.sara.chiama("GET", "/sanita/pg/"+pg, nil); code != 404 {
		t.Fatal("la Sanità è visibile solo al proprietario e al Master")
	}
	cs := tv.clienteWS(tv.sara)
	if _, err := tv.ws(cs, "sanita.soccombi", map[string]any{"evento": ev}); err == nil {
		t.Fatal("si tira solo per il proprio personaggio")
	}
	_ = cl
}

func TestSanitaEventoESincronizzazione(t *testing.T) {
	tv, pg, cm, cl, cs := preparaSanita(t)
	attivaTerrore(tv)
	// Senza evento attivo non si tira (nessuna CD inventata).
	if v := tv.luca.deve(200, "GET", "/sanita/pg/"+pg, nil); v["evento"] != nil {
		t.Fatal("nessun evento all'inizio")
	}
	// SYNC-01: il giocatore apre la schermata, il Master riceve l'avviso.
	ultimiMessaggi(cm, "")
	tv.wsOK(cl, "sanita.apri", map[string]any{"personaggio": pg})
	if m := ultimiMessaggi(cm, "sanita.aperta"); len(m) != 1 || !strings.Contains(m[0], pg) {
		t.Fatal("SYNC-01: il Master deve sapere che il giocatore ha aperto la schermata")
	}
	// Una bozza non è visibile al giocatore e non si può tirare.
	bozza := evento(tv, pg, 2, map[string]any{"bozza": true})
	if v := tv.luca.deve(200, "GET", "/sanita/pg/"+pg, nil); v["evento"] != nil {
		t.Fatal("la bozza non è visibile al giocatore")
	}
	if _, err := tv.ws(cl, "sanita.soccombi", map[string]any{"evento": bozza}); err == nil {
		t.Fatal("una bozza non si tira")
	}
	ultimiMessaggi(cl, "")
	tv.m.deve(200, "PATCH", "/sanita/eventi/"+bozza, map[string]any{"stato": "READY", "cd": 18})
	if len(ultimiMessaggi(cl, "sanita.evento")) != 1 {
		t.Fatal("il giocatore riceve l'avviso dell'evento attivato")
	}
	// TER-01 + SYNC-02: un solo tiro, risultato a entrambi. d20 = 5 → 5 + 2 = 7 < 18: fallito, +2 Terrore
	// dell'evento e +1 perché fallito di 5 o più (regola automatica) = 3.
	ultimiMessaggi(cm, "")
	ultimiMessaggi(cl, "")
	r := soccombi(t, tv, cl, bozza, 5)
	if r.Esito != "FALLITO" || r.TS.Totale != 7 || r.TS.CD != 18 || r.TerrorePrima != 0 || r.TerroreAgg != 3 || r.TerroreDopo != 3 || r.Livello.Nome != "Impaurito" || len(r.Effetti) != 3 || len(r.Automatiche) != 1 {
		t.Fatalf("TER-01: %+v", r)
	}
	if len(ultimiMessaggi(cm, "sanita.stato")) == 0 || len(ultimiMessaggi(cl, "sanita.stato")) == 0 || len(ultimiMessaggi(cs, "sanita.stato")) != 0 {
		t.Fatal("SYNC-02: aggiornamento a giocatore e Master, non ad altri")
	}
	// SYNC-03/04: secondo clic, nuova richiesta o pagina ricaricata → stesso risultato, nessun nuovo tiro.
	r2 := soccombi(t, tv, cl, bozza) // nessun dado disponibile: se tirasse, il test andrebbe in panico
	if r2.TS.Totale != r.TS.Totale || r2.Quando != r.Quando {
		t.Fatal("SYNC-03: il secondo clic non tira di nuovo")
	}
	v := tv.m.deve(200, "GET", "/sanita/pg/"+pg, nil)
	if v["terrore"] != 3.0 || v["ultimo"].(map[string]any)["stato"] != "RESOLVED" {
		t.Fatal("SYNC-04: lo stato salvato si rilegge dopo un ricaricamento")
	}
	var tiri int
	tv.a.s.DB.QueryRow(`SELECT COUNT(*) FROM registro_sanita WHERE azione = 'tiro_paura'`).Scan(&tiri)
	if tiri != 1 {
		t.Fatalf("un solo tiro registrato, non %d", tiri)
	}
	// TER-03: successo → niente Terrore (con competenza concessa dal Master: 12 + 2 + 3 = 17 ≥ 15).
	tv.m.deve(200, "PUT", "/sanita/pg/"+pg, map[string]any{"competente": true})
	ev := evento(tv, pg, 1, nil)
	if r := soccombi(t, tv, cl, ev, 12); r.Esito != "SUPERATO" || r.TerroreAgg != 0 || r.TS.Competenza != 3 || r.TerroreDopo != 3 {
		t.Fatalf("TER-03/SAN-05: %+v", r)
	}
	// TER-05: vantaggio impostato dal Master. (Fallito di 2: solo +1 dell'evento, Terrore 3 → 4, soglia 4 → follia
	// temporanea automatica: d20, d100, d10.)
	ev = evento(tv, pg, 0, map[string]any{"vantaggio": "svantaggio"})
	if r := soccombi(t, tv, cl, ev, 19, 3, 1, 1, 1); len(r.TS.D20) != 2 || r.TS.Scelto != 3 || r.TerroreDopo != 4 || len(r.PazzieAuto) != 1 {
		t.Fatalf("TER-05: %+v", r)
	}
	// Ripetizione esplicita e registrata del tiro.
	if code, _ := tv.m.chiama("POST", "/sanita/eventi/"+ev+"/ripeti", map[string]any{}); code != 400 {
		t.Fatal("serve il motivo della ripetizione")
	}
	nuovo := tv.m.deve(200, "POST", "/sanita/eventi/"+ev+"/ripeti", map[string]any{"motivo": "dado caduto dal tavolo", "ripristinaTerrore": true})["evento"].(string)
	if nuovo == ev {
		t.Fatal("la ripetizione crea un nuovo evento")
	}
}

func TestSanitaSoglieECondizioni(t *testing.T) {
	tv, pg, _, cl, _ := preparaSanita(t)
	attivaTerrore(tv)
	cond := func() []string {
		var c []string
		json.Unmarshal(tv.a.s.statoInGioco(pg).Condizioni, &c)
		return c
	}
	// Condizione già presente da un'altra fonte: il Terrore non deve toglierla.
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{"condizioni": []string{"Prono"}})
	// AUTH-02: il Master porta il Terrore a 6 con motivazione → Paralizzato.
	if code, _ := tv.m.chiama("POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "manuale", "valore": 6}); code != 400 {
		t.Fatal("la correzione manuale richiede il motivo")
	}
	tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "manuale", "valore": 6, "motivo": "visione del castello"})
	if c := cond(); len(c) != 2 || c[1] != "Paralizzato" {
		t.Fatalf("Terrore 6 → Paralizzato (e Prono resta): %v", c)
	}
	var motivo string
	tv.a.s.DB.QueryRow(`SELECT dati FROM registro_sanita WHERE azione = 'terrore_manuale'`).Scan(&motivo)
	if !strings.Contains(motivo, "visione del castello") {
		t.Fatal("AUTH-02: motivazione nel registro")
	}
	// TER-02: da 6, fallimento di intensità 4 (+4, +1 perché fallito di 5 o più) → 7 (non 11) e Privo di sensi.
	ev := evento(tv, pg, 4, nil)
	r := soccombi(t, tv, cl, ev, 2)
	if r.TerroreDopo != 7 || r.TerroreAgg != 5 || r.Livello.Nome != "Svenuto" {
		t.Fatalf("TER-02: %+v", r)
	}
	if c := cond(); !strings.Contains(strings.Join(c, ","), "Privo di sensi") || !strings.Contains(strings.Join(c, ","), "Paralizzato") {
		t.Fatalf("Terrore 7: Paralizzato e Privo di sensi: %v", c)
	}
	// Svenuto: TS di Costituzione CD 15 a fine turno (12 + 1 = 13 fallito, poi 15 + 1 = 16 superato → 6).
	vero := tiraDado
	tiraDado = sequenza(12, 15)
	tv.luca.deve(200, "POST", "/sanita/pg/"+pg+"/tiro", map[string]any{"tipo": "svenuto"})
	if tv.a.s.leggiSanitaPG(pg).terrore != 7 {
		t.Fatal("TS fallito: resta svenuto")
	}
	tv.luca.deve(200, "POST", "/sanita/pg/"+pg+"/tiro", map[string]any{"tipo": "svenuto"})
	tiraDado = vero
	if tv.a.s.leggiSanitaPG(pg).terrore != 6 || strings.Contains(strings.Join(cond(), ","), "Privo di sensi") {
		t.Fatal("TS superato: Terrore 6, non più Privo di sensi")
	}
	// Recuperi: riposo lungo −1 → 5, niente più Paralizzato; mai sotto 0.
	tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "riposo_lungo"})
	if c := cond(); len(c) != 1 || c[0] != "Prono" {
		t.Fatalf("Terrore 5: tolte solo le condizioni del Terrore: %v", c)
	}
	tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "manuale", "valore": 0, "motivo": "fine della paura"})
	tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "dieci_giorni"})
	if tv.a.s.leggiSanitaPG(pg).terrore != 0 {
		t.Fatal("il Terrore non scende sotto 0")
	}
	if code, _ := tv.m.chiama("POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "rimozione_spaventato"}); code != 400 {
		t.Fatal("regola opzionale disattivata")
	}
	// Disattivando il sistema del Terrore le sue condizioni spariscono.
	tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "manuale", "valore": 7, "motivo": "prova"})
	tv.m.deve(200, "PUT", "/sanita/config", configPredefinita())
	if c := cond(); len(c) != 1 {
		t.Fatalf("Terrore disattivato: niente condizioni del Terrore: %v", c)
	}
	// Immunità completa: nessun Terrore aggiunto.
	attivaTerrore(tv)
	tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "manuale", "valore": 0, "motivo": "reset"})
	tv.m.deve(200, "PUT", "/sanita/pg/"+pg, map[string]any{"immuneTerrore": true})
	if r := soccombi(t, tv, cl, evento(tv, pg, 3, nil), 1); r.TerroreAgg != 0 || r.Esito != "FALLITO" || len(r.Resistenze) != 1 {
		t.Fatalf("immunità completa: %+v", r)
	}
}

func TestSanitaPazzie(t *testing.T) {
	tv, pg, _, cl, _ := preparaSanita(t)
	attivaTerrore(tv)
	tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/terrore", map[string]any{"tipo": "manuale", "valore": 3, "motivo": "prova"})
	// MAD-01 (regole automatiche, richiesta dell'utente 2026-11-06): da 3, TS fallito con 1 sul d20 (di 12: +1) → Terrore 5,
	// soglia 4 superata e 1 naturale → UNA sola follia temporanea casuale (d20 2 = Fobia, d100 7 = voce 1, durata d10 = 4).
	r := soccombi(t, tv, cl, evento(tv, pg, 1, nil), 1, 2, 7, 4)
	if r.TerroreDopo != 5 || len(r.Pazzia) != 0 || len(r.PazzieAuto) != 1 || r.PazzieAuto[0].Tipo != "temporanea" || r.PazzieAuto[0].Categoria != "Fobia" {
		t.Fatalf("MAD-01: %+v", r)
	}
	var n int
	tv.a.s.DB.QueryRow(`SELECT COUNT(*) FROM pazzie`).Scan(&n)
	if n != 1 {
		t.Fatalf("MAD-01: una follia temporanea automatica, trovate %d", n)
	}
	// Il TS di Carisma del Master resta disponibile su un altro evento (qui superato: d20 20).
	ev := evento(tv, pg, 1, nil)
	soccombi(t, tv, cl, ev, 20)
	// Il catalogo è vuoto: la pazzia generata segnala il testo mancante (nessun testo inventato).
	tv.m.deve(200, "PUT", "/sanita/catalogo", map[string]any{"tabella": "homebrew", "categoria": "Fobia", "voci": []string{"Paura dei corvi", "", "", "", "", "", "", "", "", ""}})
	if code, _ := tv.luca.chiama("GET", "/sanita/catalogo", nil); code != 403 {
		t.Fatal("il catalogo è del Master")
	}
	// TS di Carisma chiesto dal Master: d20 = 3 → 3 − 1 + 3 = 5 < 15 → pazzia temporanea casuale (d20 2 = Fobia, d100 7 = voce 1, durata d10 = 4).
	vero := tiraDado
	tiraDado = sequenza(3, 2, 7, 4)
	out := tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/tiro", map[string]any{"tipo": "carisma_pazzia", "evento": ev})
	tiraDado = vero
	p := out["pazzia"].(map[string]any)
	if p["tipo"] != "temporanea" || p["categoria"] != "Fobia" || p["descrizione"] != "Paura dei corvi" || p["durata"] != "4 minuti" || p["cdRecupero"] != 15.0 || p["scadeIl"] == nil {
		t.Fatalf("MAD-02: pazzia temporanea di 1d10 minuti con scadenza: %v", p)
	}
	// Una sola pazzia per evento, salvo eccezione del Master; duratura solo con conferma.
	if code, _ := tv.m.chiama("POST", "/sanita/pg/"+pg+"/pazzie", map[string]any{"tipo": "duratura", "categoria": "Paranoia", "evento": ev, "conferma": true}); code != 409 {
		t.Fatal("una sola pazzia per evento")
	}
	if code, _ := tv.m.chiama("POST", "/sanita/pg/"+pg+"/pazzie", map[string]any{"tipo": "duratura", "categoria": "Paranoia"}); code != 409 {
		t.Fatal("duratura: serve la conferma")
	}
	if code, _ := tv.luca.chiama("POST", "/sanita/pg/"+pg+"/pazzie", map[string]any{"tipo": "temporanea", "categoria": "Fobia"}); code != 403 {
		t.Fatal("solo il Master assegna pazzie")
	}
	tiraDado = sequenza(5)
	d := tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/pazzie", map[string]any{"tipo": "duratura", "categoria": "Paranoia", "descrizione": "Crede che il borgomastro lo spii", "conferma": true, "cdRecupero": 12})
	tiraDado = vero
	if d["durata"] != "50 ore" {
		t.Fatalf("duratura 1d10 × 10 ore: %v", d)
	}
	// La pazzia precedente riceve +1 alla CD di recupero.
	v := tv.m.deve(200, "GET", "/sanita/pg/"+pg, nil)
	for _, x := range v["pazzie"].([]any) {
		if m := x.(map[string]any); m["descrizione"] == "Paura dei corvi" && m["cdRecupero"] != 16.0 {
			t.Fatalf("CD della pazzia precedente +1: %v", m)
		}
	}
	// Recupero (homebrew): TS Saggezza 14 + 3 + 3 = 20 ≥ 12 → CD −1d4 (4) → 8; poi 10+6 ≥ 8 → −4 → 4…
	tiraDado = sequenza(14, 4)
	rec := tv.m.deve(200, "POST", "/sanita/pazzie/"+d["id"].(string)+"/recupero", map[string]any{"dataGioco": "giorno 10"})
	tiraDado = vero
	if rec["cdPrima"] != 12.0 || rec["cdDopo"] != 8.0 || rec["guarita"] != false {
		t.Fatalf("recupero: %v", rec)
	}
	tv.m.deve(200, "PATCH", "/sanita/pazzie/"+d["id"].(string), map[string]any{"cdRecupero": 2})
	tiraDado = sequenza(14, 3)
	rec = tv.m.deve(200, "POST", "/sanita/pazzie/"+d["id"].(string)+"/recupero", nil)
	tiraDado = vero
	if rec["guarita"] != true {
		t.Fatal("CD a 0: pazzia guarita")
	}
	// Scadenza della pazzia temporanea.
	tv.a.s.DB.Exec(`UPDATE pazzie SET scade_il = 1 WHERE tipo = 'temporanea'`)
	v = tv.luca.deve(200, "GET", "/sanita/pg/"+pg, nil)
	if len(v["pazzie"].([]any)) != 0 {
		t.Fatalf("pazzia temporanea scaduta: %v", v["pazzie"])
	}
	// Rimozione da parte del Master con motivo, registrata.
	tiraDado = sequenza(3)
	tmp := tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/pazzie", map[string]any{"tipo": "temporanea", "categoria": "Allucinazioni"})
	tiraDado = vero
	tv.m.deve(200, "PATCH", "/sanita/pazzie/"+tmp["id"].(string), map[string]any{"rimuovi": true, "motivo": "incantesimo Ristorare inferiore"})
	var att int
	tv.a.s.DB.QueryRow(`SELECT attiva FROM pazzie WHERE id = ?`, tmp["id"]).Scan(&att)
	if att != 0 {
		t.Fatal("pazzia rimossa")
	}
}

func TestSanitaMetodoSaggezzaEManuale(t *testing.T) {
	tv, pg, _, _, _ := preparaSanita(t)
	// Le impostazioni salvate da versioni precedenti con il metodo DMG restano accettate, ma il valore
	// è sempre 10 + mod Saggezza: Saggezza 18 → 14 → +2.
	c := configPredefinita()
	c.Metodo = MetodoDMG
	tv.m.deve(200, "PUT", "/sanita/config", c)
	v := tv.luca.deve(200, "GET", "/sanita/pg/"+pg, nil)
	if v["punteggio"] != 14.0 || v["mod"] != 2.0 || v["metodo"] != MetodoSaggezza || v["config"].(map[string]any)["metodo"] != MetodoSaggezza {
		t.Fatalf("10 + mod Saggezza: %v", v)
	}
	tv.m.deve(200, "PUT", "/sanita/pg/"+pg, map[string]any{"punteggioManuale": 9, "bonusTemp": 1})
	v = tv.luca.deve(200, "GET", "/sanita/pg/"+pg, nil)
	if v["punteggio"] != 9.0 || v["metodo"] != MetodoManuale || v["bonusTS"] != 0.0 {
		t.Fatalf("punteggio manuale del Master (9 → −1, +1 temporaneo): %v", v)
	}
	if code, _ := tv.m.chiama("PUT", "/sanita/pg/"+pg, map[string]any{"punteggioManuale": 31}); code != 400 {
		t.Fatal("punteggio tra 1 e 30")
	}
	bad := configPredefinita()
	bad.Metodo = "INVENTATO"
	if code, _ := tv.m.chiama("PUT", "/sanita/config", bad); code != 400 {
		t.Fatal("configurazione non valida rifiutata")
	}
}

// Tiro salvezza su Sanità mentale dalla mini scheda: lo fa il giocatore (o il Master), il risultato
// arriva a entrambi e non agli altri giocatori.
func TestSanitaTiroSalvezzaLibero(t *testing.T) {
	tv, pg, cm, cl, cs := preparaSanita(t)
	vero := tiraDado
	defer func() { tiraDado = vero }()
	ultimiMessaggi(cm, "")
	ultimiMessaggi(cl, "")
	ultimiMessaggi(cs, "")
	tiraDado = sequenza(11)
	r := tv.luca.deve(200, "POST", "/sanita/pg/"+pg+"/tiro", map[string]any{"tipo": "sanita", "cd": 13})
	ts := r["ts"].(map[string]any)
	if ts["totale"] != 13.0 || ts["successo"] != true || ts["caratteristica"] != "SAN" || r["nome"] != "Arin" {
		t.Fatalf("d20 11 + 2 = 13 contro CD 13: %v", r)
	}
	if len(ultimiMessaggi(cm, "sanita.tiro")) != 1 || len(ultimiMessaggi(cl, "sanita.tiro")) != 1 || len(ultimiMessaggi(cs, "sanita.tiro")) != 0 {
		t.Fatal("il risultato lo vedono il Master e il giocatore del personaggio, non gli altri")
	}
	// Il Master tira per il personaggio, senza CD e con svantaggio: il giocatore vede il risultato.
	tiraDado = sequenza(15, 4)
	r = tv.m.deve(200, "POST", "/sanita/pg/"+pg+"/tiro", map[string]any{"tipo": "sanita", "vantaggio": "svantaggio"})
	if ts = r["ts"].(map[string]any); ts["scelto"] != 4.0 || ts["totale"] != 6.0 {
		t.Fatalf("svantaggio: tenuto il 4 → 6: %v", r)
	}
	if len(ultimiMessaggi(cl, "sanita.tiro")) != 1 {
		t.Fatal("il giocatore vede il tiro fatto dal Master")
	}
	// Un altro giocatore non può tirare per Arin; CD fuori scala rifiutata.
	if code, _ := tv.sara.chiama("POST", "/sanita/pg/"+pg+"/tiro", map[string]any{"tipo": "sanita"}); code != 404 {
		t.Fatal("solo il proprietario o il Master")
	}
	if code, _ := tv.luca.chiama("POST", "/sanita/pg/"+pg+"/tiro", map[string]any{"tipo": "sanita", "cd": 41}); code != 400 {
		t.Fatal("CD da 1 a 40")
	}
	st := tv.m.deve(200, "GET", "/sanita/pg/"+pg, nil)["storico"].([]any)
	n := 0
	for _, x := range st {
		if x.(map[string]any)["azione"] == "tiro_sanita" {
			n++
		}
	}
	if n != 2 {
		t.Fatal("i tiri restano nello storico")
	}
}

// Regola della campagna: senza impostazioni salvate la Sanità mentale è 10 + modificatore di Saggezza.
func TestSanitaRegolaCampagna(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"cls": "chierico"},
		"riepilogo": map[string]any{"name": "Arin", "level": 5, "abilities": map[string]any{"SAG": 16}, "sanity": 8, "sanityMethod": "DMG_STANDARD"}}) // valori di vecchie schede: ignorati
	v := tv.m.deve(200, "GET", "/sanita/pg/"+pg, nil)
	if v["punteggio"] != float64(13) || v["config"].(map[string]any)["metodo"] != "WISDOM_DERIVED" {
		t.Fatalf("Saggezza 16 → Sanità 10 + 3 = 13 (il valore scritto nella scheda non conta): %v / %v", v["punteggio"], v["config"])
	}
}

// Un personaggio appena creato (o salvato prima che il riepilogo avesse le caratteristiche): la scheda
// aperta manda il riepilogo senza modifiche ai campi; il server lo aggiorna senza creare una versione.
func TestRiepilogoSenzaModifiche(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	if v := tv.m.deve(200, "GET", "/sanita/pg/"+pg, nil); v["punteggio"] != nil {
		t.Fatalf("senza riepilogo la Saggezza non c'è: %v", v["punteggio"])
	}
	prima := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["personaggio"].(map[string]any)["versione"]
	cl := tv.clienteWS(tv.luca)
	ultimiMessaggi(cl, "")
	r := tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": prima, "modifiche": map[string]any{},
		"riepilogo": map[string]any{"name": "Mimmo", "level": 1, "abilities": map[string]any{"SAG": 10}}})
	if r["versione"] != prima {
		t.Fatalf("nessuna nuova versione: %v → %v", prima, r["versione"])
	}
	if v := tv.m.deve(200, "GET", "/sanita/pg/"+pg, nil); v["punteggio"] != 10.0 {
		t.Fatalf("Saggezza 10 → Sanità 10: %v", v["punteggio"])
	}
	if len(ultimiMessaggi(cl, "sanita.stato")) != 1 {
		t.Fatal("la mini scheda aperta si aggiorna")
	}
	// Un altro giocatore non può cambiare il riepilogo.
	if code, _ := tv.sara.chiama("PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": prima, "modifiche": map[string]any{},
		"riepilogo": map[string]any{"abilities": map[string]any{"SAG": 20}}}); code == 200 {
		t.Fatal("riepilogo modificato da un estraneo")
	}
}
