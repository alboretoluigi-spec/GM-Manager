package server

import "testing"

// scontro prepara: Master; gruppo con Luca (giocatore, PG "Arin") e Sara (Segretario); Ugo in un altro
// gruppo; una sessione del primo gruppo con un combattimento già creato.
type scontro struct {
	a                  *ambiente
	m, luca, sara, ugo *cliente
	ses, pg, idLuca    string
}

func nuovoScontro(t *testing.T) *scontro {
	a := nuovoAmbiente(t)
	sc := &scontro{a: a, m: a.master()}
	g1 := sc.m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	g2 := sc.m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Venerdì"})["id"].(string)
	sc.luca = a.giocatore("Luca", "333 1234567", "192.168.1.20")
	sc.sara = a.giocatore("Sara", "347 1112223", "192.168.1.21")
	sc.ugo = a.giocatore("Ugo", "320 9998887", "192.168.1.22")
	sc.idLuca = a.idUtente(sc.luca)
	sc.m.deve(200, "POST", "/utenti/"+sc.idLuca+"/approva", map[string]any{"gruppi": []string{g1}})
	idSara := a.idUtente(sc.sara)
	sc.m.deve(200, "POST", "/utenti/"+idSara+"/approva", map[string]any{"gruppi": []string{g1}})
	sc.m.deve(200, "PUT", "/utenti/"+idSara+"/gruppi", map[string]any{"gruppi": []string{g1}, "segretario": []string{g1}})
	sc.m.deve(200, "POST", "/utenti/"+a.idUtente(sc.ugo)+"/approva", map[string]any{"gruppi": []string{g2}})
	// Luca crea Arin; la scheda invia il riepilogo (CA 16, PF max 20, iniziativa +2, Des 14).
	sc.pg = sc.luca.deve(200, "POST", "/personaggi", map[string]any{"nome": "Arin"})["id"].(string)
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"lvl": 3},
		"riepilogo": map[string]any{"class": "Chierico", "level": 3, "ac": 16, "hpMax": 20, "initiative": 2, "dex": 14}})
	sc.ses = sc.m.deve(200, "POST", "/gruppi/"+g1+"/sessioni", map[string]any{"titolo": "La Tana del Lupo", "inizio": 1893456000})["id"].(string)
	sc.luca.deve(200, "POST", "/sessioni/"+sc.ses+"/conferma", map[string]any{"confermato": true, "personaggio": sc.pg})
	sc.az(sc.m, 200, "iniz.nuovo", nil)
	return sc
}

func (sc *scontro) az(c *cliente, codice int, tipo string, dati map[string]any) map[string]any {
	sc.a.t.Helper()
	return c.deve(codice, "POST", "/sessioni/"+sc.ses+"/iniziativa", map[string]any{"tipo": tipo, "dati": dati})
}

func (sc *scontro) vista(c *cliente) map[string]any {
	sc.a.t.Helper()
	return c.deve(200, "GET", "/sessioni/"+sc.ses+"/combattimento", nil)
}

func combattenti(v map[string]any) []map[string]any {
	var out []map[string]any
	for _, x := range v["combattenti"].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

func cerca(v map[string]any, nome string) map[string]any {
	for _, c := range combattenti(v) {
		if c["nome"] == nome {
			return c
		}
	}
	return nil
}

func TestIniziativaVisibilitaEPermessi(t *testing.T) {
	sc := nuovoScontro(t)
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Lupo", "quanti": 2, "ca": 13, "pf": 11, "bonusIniziativa": 2})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "l'antagonista", "quanti": 1, "ca": 16, "pf": 144, "nascosto": true})

	// Il PG è entrato da solo con i dati della scheda.
	vm := sc.vista(sc.m)
	arin := cerca(vm, "Arin")
	if arin == nil || arin["ca"].(float64) != 16 || arin["pf"].(float64) != 20 || len(combattenti(vm)) != 4 {
		t.Fatalf("vista del Master sbagliata: %v", vm)
	}
	// Il giocatore: niente mostro nascosto, niente PF esatti dei mostri.
	vl := sc.vista(sc.luca)
	if cerca(vl, "l'antagonista") != nil || len(combattenti(vl)) != 3 {
		t.Fatal("il giocatore non deve vedere il mostro nascosto")
	}
	if l := cerca(vl, "Lupo 1"); l["pf"] != nil || l["salute"] != "illeso" {
		t.Fatalf("il giocatore deve vedere solo lo stato del mostro: %v", l)
	}
	if !cerca(vl, "Arin")["mio"].(bool) {
		t.Fatal("Arin deve risultare di Luca")
	}
	// Il Segretario vede i PF dei mostri visibili.
	if l := cerca(sc.vista(sc.sara), "Lupo 1"); l["pf"].(float64) != 11 {
		t.Fatalf("il Segretario deve vedere i PF: %v", l)
	}
	// Un giocatore di un altro gruppo non vede nulla.
	sc.ugo.deve(404, "GET", "/sessioni/"+sc.ses+"/combattimento", nil)
	sc.az(sc.ugo, 404, "iniz.prossimo_turno", nil)

	// Iniziativa: Luca la imposta per il proprio PG, non per un mostro.
	sc.az(sc.luca, 200, "iniz.iniziativa", map[string]any{"personaggio": sc.pg, "valore": 15})
	lupo := cerca(vm, "Lupo 1")["id"].(string)
	antagonista := cerca(vm, "l'antagonista")["id"].(string)
	sc.az(sc.luca, 403, "iniz.iniziativa", map[string]any{"combattente": lupo, "valore": 30})
	// Turni: prima dell'avvio non si avanza; poi il Segretario sì, il giocatore no.
	sc.az(sc.sara, 409, "iniz.prossimo_turno", nil)
	sc.az(sc.sara, 403, "iniz.avvia", nil)
	// «forza»: i mostri tirano l'iniziativa a caso e un pareggio con Arin (15) fermerebbe l'avvio (i pareggi hanno la loro
	// prova, TestIniziativaNaturaliEPareggi)
	ra := sc.az(sc.m, 200, "iniz.avvia", map[string]any{"forza": true})
	v := sc.vista(sc.m)
	if v["combattimento"].(map[string]any)["round"].(float64) != 1 {
		t.Fatalf("round atteso 1: %v (avvio: %v)", v["combattimento"], ra)
	}
	for _, c := range combattenti(v) {
		if c["iniziativa"] == nil {
			t.Fatalf("all'avvio tutti devono avere un'iniziativa: %v", c)
		}
	}
	sc.az(sc.sara, 200, "iniz.prossimo_turno", nil)
	sc.az(sc.luca, 403, "iniz.prossimo_turno", nil)
	// Quattro turni dopo l'inizio si torna al primo: round 2.
	for i := 0; i < 3; i++ {
		sc.az(sc.m, 200, "iniz.prossimo_turno", nil)
	}
	if r := sc.vista(sc.m)["combattimento"].(map[string]any)["round"].(float64); r != 2 {
		t.Fatalf("round atteso 2, ottenuto %v", r)
	}

	// Mostri: il Segretario colpisce quelli visibili, non quello nascosto; il giocatore nessuno.
	sc.az(sc.sara, 200, "mostro.danno", map[string]any{"combattente": lupo, "valore": 7})
	sc.az(sc.sara, 403, "mostro.danno", map[string]any{"combattente": antagonista, "valore": 7})
	sc.az(sc.luca, 403, "mostro.danno", map[string]any{"combattente": lupo, "valore": 7})
	if l := cerca(sc.vista(sc.luca), "Lupo 1"); l["salute"] != "gravemente ferito" {
		t.Fatalf("4 PF su 11: atteso gravemente ferito, %v", l["salute"])
	}
	sc.az(sc.m, 400, "mostro.condizioni", map[string]any{"combattente": lupo, "condizioni": []string{"Arrabbiato"}})
	sc.az(sc.m, 200, "mostro.condizioni", map[string]any{"combattente": lupo, "condizioni": []string{"Prono"}})

	// Il Master svela l'antagonista: ora i giocatori lo vedono.
	sc.az(sc.m, 200, "iniz.nascondi", map[string]any{"combattente": antagonista, "nascosto": false})
	if cerca(sc.vista(sc.luca), "l'antagonista") == nil {
		t.Fatal("l'antagonista svelato deve essere visibile")
	}
	// Fine del combattimento.
	sc.az(sc.luca, 403, "iniz.fine", nil)
	sc.az(sc.m, 200, "iniz.fine", nil)
	if sc.vista(sc.luca)["combattimento"] != nil {
		t.Fatal("dopo la fine non c'è un combattimento attivo")
	}
	sc.az(sc.m, 409, "iniz.prossimo_turno", nil)
}

func TestIniziativaPFCondivisiConLaScheda(t *testing.T) {
	sc := nuovoScontro(t)
	// 3 PF temporanei, poi 5 danni: i temporanei si consumano per primi → 18 PF, 0 temporanei.
	sc.az(sc.luca, 200, "pg.temp", map[string]any{"personaggio": sc.pg, "valore": 3})
	sc.az(sc.sara, 200, "pg.danno", map[string]any{"personaggio": sc.pg, "valore": 5})
	a := cerca(sc.vista(sc.luca), "Arin")
	if a["pf"].(float64) != 18 || a["pfTemp"] != nil {
		t.Fatalf("attesi 18 PF e 0 temporanei: %v", a)
	}
	// La scheda vede gli stessi PF.
	if d := sc.luca.deve(200, "GET", "/personaggi/"+sc.pg, nil)["dati"].(map[string]any); d["hpCur"] != "18" {
		t.Fatalf("la scheda deve avere 18 PF: %v", d["hpCur"])
	}
	// A 0 PF, tiri contro la morte; una cura li azzera e non supera il massimo.
	sc.az(sc.m, 200, "pg.danno", map[string]any{"personaggio": sc.pg, "valore": 50})
	sc.az(sc.luca, 200, "pg.ts_morte", map[string]any{"personaggio": sc.pg, "s": 1, "f": 2})
	sc.az(sc.luca, 400, "pg.ts_morte", map[string]any{"personaggio": sc.pg, "s": 4, "f": 0})
	sc.az(sc.sara, 200, "pg.cura", map[string]any{"personaggio": sc.pg, "valore": 100})
	a = cerca(sc.vista(sc.m), "Arin")
	ts := a["tsMorte"].(map[string]any)
	if a["pf"].(float64) != 20 || ts["s"].(float64) != 0 || ts["f"].(float64) != 0 {
		t.Fatalf("cura: attesi 20 PF e tiri azzerati: %v", a)
	}
	// Condizioni ufficiali soltanto; il riposo lungo lo decidono giocatore o Master, non il Segretario.
	sc.az(sc.luca, 400, "pg.condizioni", map[string]any{"personaggio": sc.pg, "condizioni": []string{"<b>Stanco</b>"}})
	sc.az(sc.luca, 200, "pg.condizioni", map[string]any{"personaggio": sc.pg, "condizioni": []string{"Avvelenato", "Concentrato"}})
	sc.az(sc.sara, 403, "pg.riposo_lungo", map[string]any{"personaggio": sc.pg})
	sc.az(sc.m, 200, "pg.danno", map[string]any{"personaggio": sc.pg, "valore": 4})
	sc.az(sc.luca, 200, "pg.riposo_lungo", map[string]any{"personaggio": sc.pg})
	if a = cerca(sc.vista(sc.m), "Arin"); a["pf"].(float64) != 20 {
		t.Fatalf("dopo il riposo lungo PF al massimo: %v", a["pf"])
	}
	// Un altro giocatore del gruppo (senza ruolo di Segretario) non tocca Arin: qui è Ugo, fuori gruppo.
	sc.az(sc.ugo, 404, "pg.danno", map[string]any{"personaggio": sc.pg, "valore": 1})
}

func TestMessaggiPrivati(t *testing.T) {
	sc := nuovoScontro(t)
	sc.az(sc.luca, 200, "messaggio.invia", map[string]any{"testo": "Provo a borseggiare il mercante."})
	// Sara (stessa sessione) non vede il messaggio di Luca al Master.
	if l := sc.sara.deve(200, "GET", "/sessioni/"+sc.ses+"/messaggi", nil)["lista"].([]any); len(l) != 0 {
		t.Fatalf("Sara non deve vedere messaggi altrui: %v", l)
	}
	if l := sc.m.deve(200, "GET", "/sessioni/"+sc.ses+"/messaggi", nil)["lista"].([]any); len(l) != 1 {
		t.Fatalf("il Master deve ricevere il messaggio: %v", l)
	}
	sc.az(sc.m, 200, "messaggio.invia", map[string]any{"a": sc.idLuca, "testo": "Tira Rapidità di Mano."})
	sc.az(sc.m, 400, "messaggio.invia", map[string]any{"a": sc.a.idUtente(sc.ugo), "testo": "Ugo non partecipa."})
	sc.az(sc.luca, 400, "messaggio.invia", map[string]any{"testo": ""})
	if l := sc.luca.deve(200, "GET", "/sessioni/"+sc.ses+"/messaggi", nil)["lista"].([]any); len(l) != 2 {
		t.Fatalf("Luca deve vedere la sua conversazione con il Master: %v", l)
	}
}
