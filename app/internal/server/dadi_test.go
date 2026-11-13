package server

// Test di dadi, tabella della sfortuna e dadi reali: presi dal ramo main-ke8sy4 (iniziativa_test.go,
// scritti dall'altro account) e adattati all'ambiente di prova di questa versione.

import (
	"encoding/json"
	"strings"
	"testing"
)

// clienteWS simula una connessione in tempo reale di un utente del tavolo di prova.
func (sc *scontro) clienteWS(c *cliente) *Cliente {
	sc.a.t.Helper()
	id := sc.a.idUtente(c)
	u := &Utente{}
	if err := sc.a.s.DB.QueryRow(`SELECT id, nome, ruolo, stato FROM utenti WHERE id = ?`, id).Scan(&u.ID, &u.Nome, &u.Ruolo, &u.Stato); err != nil {
		sc.a.t.Fatal(err)
	}
	cl := &Cliente{U: u, invio: make(chan []byte, 512), canali: map[string]bool{"utente:" + u.ID: true}, chiudi: func() {}}
	if u.Master() { // come la connessione vera (Hub.Gestisci)
		cl.canali["master"] = true
	}
	h := sc.a.s.Hub
	h.mu.Lock()
	h.clienti[cl] = true
	h.mu.Unlock()
	return cl
}

// ws chiama un gestore come farebbe un messaggio in arrivo.
func (sc *scontro) ws(c *Cliente, tipo string, dati map[string]any) (any, error) {
	b, _ := json.Marshal(dati)
	g := sc.a.s.Hub.gestori[tipo]
	if g == nil {
		sc.a.t.Fatalf("gestore %s mancante", tipo)
	}
	return g(c, b)
}

func (sc *scontro) wsOK(c *Cliente, tipo string, dati map[string]any) any {
	sc.a.t.Helper()
	r, err := sc.ws(c, tipo, dati)
	if err != nil {
		sc.a.t.Fatalf("%s: %v", tipo, err)
	}
	return r
}

// ultimiMessaggi svuota la coda di un cliente e restituisce i messaggi del tipo indicato.
func ultimiMessaggi(c *Cliente, tipo string) []string {
	var out []string
	for {
		select {
		case b := <-c.invio:
			if strings.Contains(string(b), `"tipo":"`+tipo+`"`) {
				out = append(out, string(b))
			}
		default:
			return out
		}
	}
}

func TestDadi(t *testing.T) {
	// In questa versione lo scontro nasce con "Nuovo combattimento": si parte senza (iniz.fine) e lo si
	// riapre con iniz.nuovo dove l'originale usava iniz.inizia. Ugo (altro gruppo) non partecipa.
	tv := nuovoScontro(t)
	tv.az(tv.m, 200, "iniz.fine", nil)
	pgL, sess := tv.pg, tv.ses
	_, _ = pgL, sess
	cm, cl, cs := tv.clienteWS(tv.m), tv.clienteWS(tv.luca), tv.clienteWS(tv.ugo)
	// Dadi truccati per la prova: restituiscono i valori in ordine.
	var seq []int
	vero := tiraDado
	tiraDado = func(f int) int { v := seq[0]; seq = seq[1:]; return v }
	defer func() { tiraDado = vero }()
	tira := func(c *Cliente, dati map[string]any, valori ...int) tiroDadi {
		t.Helper()
		seq = valori
		r, err := tv.ws(c, "dadi.tira", dati)
		if err != nil {
			t.Fatalf("dadi.tira: %v", err)
		}
		return r.(tiroDadi)
	}
	for _, sbagliato := range []map[string]any{{"dadi": []map[string]any{{"n": 1, "facce": 7}}}, {"dadi": []map[string]any{{"n": 60, "facce": 6}}}, {"dadi": []map[string]any{}},
		{"dadi": []map[string]any{{"n": 1, "facce": 20}}, "mod": 99}} {
		if _, err := tv.ws(cl, "dadi.tira", sbagliato); err == nil {
			t.Fatalf("tiro non valido accettato: %v", sbagliato)
		}
	}
	// Fuori dallo scontro: 2d6+3, il risultato arriva a chi tira e al Master (2026-11-12), non agli altri giocatori.
	ultimiMessaggi(cl, "")
	ultimiMessaggi(cm, "")
	r := tira(cl, map[string]any{"dadi": []map[string]any{{"n": 2, "facce": 6}}, "mod": 3}, 4, 5)
	if r.Totale != 12 || r.Scontro || r.Formula != "2d6 +3" {
		t.Fatalf("2d6+3: %+v", r)
	}
	if len(ultimiMessaggi(cl, "dadi.risultato")) != 1 || len(ultimiMessaggi(cm, "dadi.risultato")) != 1 || len(ultimiMessaggi(cs, "dadi.risultato")) != 0 {
		t.Fatal("fuori dallo scontro il tiro lo vedono chi tira e il Master, non gli altri")
	}
	// Vantaggio: si tiene il d20 più alto; svantaggio il più basso.
	if r := tira(cl, map[string]any{"dadi": []map[string]any{{"n": 1, "facce": 20}}, "modo": "vantaggio"}, 7, 15); r.Totale != 15 || r.Dadi[0].Scarto[0] != 7 {
		t.Fatalf("vantaggio: %+v", r)
	}
	if r := tira(cl, map[string]any{"dadi": []map[string]any{{"n": 1, "facce": 20}}, "modo": "svantaggio"}, 7, 15); r.Totale != 7 {
		t.Fatalf("svantaggio: %+v", r)
	}
	// Durante lo scontro: arriva a Master e partecipanti (non a Sara, che non partecipa), con l'effetto.
	tv.az(tv.m, 200, "iniz.nuovo", nil)
	ultimiMessaggi(cm, "")
	ultimiMessaggi(cl, "")
	ultimiMessaggi(cs, "")
	r = tira(cl, map[string]any{"dadi": []map[string]any{{"n": 1, "facce": 20}}, "mod": 5}, 20)
	if !r.Critico || !r.Scontro || r.Sessione != sess || r.Totale != 25 {
		t.Fatalf("20 naturale in scontro: %+v", r)
	}
	if len(ultimiMessaggi(cm, "dadi.risultato")) != 1 || len(ultimiMessaggi(cl, "dadi.risultato")) != 1 || len(ultimiMessaggi(cs, "dadi.risultato")) != 0 {
		t.Fatal("in scontro il tiro arriva al Master e ai partecipanti, non agli altri")
	}
	if r := tira(cm, map[string]any{"dadi": []map[string]any{{"n": 1, "facce": 20}}}, 1); !r.Fallimento || r.Critico {
		t.Fatalf("1 naturale: %+v", r)
	}
	// Il 1 sul d20 con svantaggio conta solo se è il dado tenuto.
	if r := tira(cl, map[string]any{"dadi": []map[string]any{{"n": 1, "facce": 20}}, "modo": "vantaggio"}, 1, 12); r.Fallimento {
		t.Fatal("con vantaggio un 1 scartato non è un fallimento")
	}
	// Tiro segreto del Master: lo vede solo lui; un giocatore non può tirare in segreto.
	ultimiMessaggi(cl, "")
	tira(cm, map[string]any{"dadi": []map[string]any{{"n": 1, "facce": 20}}, "segreto": true}, 20)
	if len(ultimiMessaggi(cl, "dadi.risultato")) != 0 {
		t.Fatal("il tiro segreto del Master non arriva ai giocatori")
	}
	ultimiMessaggi(cm, "")
	if r := tira(cl, map[string]any{"dadi": []map[string]any{{"n": 1, "facce": 20}}, "segreto": true}, 3); r.Segreto || len(ultimiMessaggi(cm, "dadi.risultato")) != 1 {
		t.Fatal("i giocatori non tirano in segreto")
	}
	v := tv.a.s.vistaCombattimento(sess, cm.U)
	if !strings.Contains(strings.Join(v.Registro, "|"), "CRITICO") {
		t.Fatal("il registro dello scontro riporta i tiri")
	}
}

func TestSfortuna(t *testing.T) {
	if len(sfortunaPredefinita) != 100 {
		t.Fatalf("la tabella predefinita deve avere 100 effetti, ne ha %d", len(sfortunaPredefinita))
	}
	// In questa versione lo scontro nasce con "Nuovo combattimento": si parte senza (iniz.fine) e lo si
	// riapre con iniz.nuovo dove l'originale usava iniz.inizia. Ugo (altro gruppo) non partecipa.
	tv := nuovoScontro(t)
	tv.az(tv.m, 200, "iniz.fine", nil)
	pgL, sess := tv.pg, tv.ses
	_, _ = pgL, sess
	cm, cl, cs := tv.clienteWS(tv.m), tv.clienteWS(tv.luca), tv.clienteWS(tv.ugo)
	var seq []int
	vero := tiraDado
	tiraDado = func(f int) int { v := seq[0]; seq = seq[1:]; return v }
	defer func() { tiraDado = vero }()
	d20 := map[string]any{"dadi": []map[string]any{{"n": 1, "facce": 20}}}
	// Fuori dallo scontro un 1 non dà il pulsante.
	seq = []int{1}
	if r := tv.wsOK(cl, "dadi.tira", d20).(tiroDadi); r.Sfortuna {
		t.Fatal("fuori dallo scontro niente tabella della sfortuna")
	}
	tv.az(tv.m, 200, "iniz.nuovo", nil)
	seq = []int{1}
	if r := tv.wsOK(cm, "dadi.tira", d20).(tiroDadi); !r.Sfortuna {
		t.Fatal("anche il Master riceve il tasto sui suoi 1")
	}
	seq = []int{1}
	r := tv.wsOK(cl, "dadi.tira", d20).(tiroDadi)
	if !r.Sfortuna || r.ID == "" || !r.Fallimento {
		t.Fatalf("1 naturale di un giocatore in scontro: %+v", r)
	}
	if _, err := tv.ws(cs, "dadi.sfortuna", map[string]any{"tiro": r.ID}); err == nil {
		t.Fatal("solo chi ha fatto 1 può premere il pulsante")
	}
	ultimiMessaggi(cm, "")
	seq = []int{1}
	e := tv.wsOK(cl, "dadi.sfortuna", map[string]any{"tiro": r.ID}).(map[string]any)
	if e["effetto"] != sfortunaPredefinita[0] {
		t.Fatalf("d100 = 1 → primo effetto: %v", e["effetto"])
	}
	if _, ok := e["d100"]; ok {
		t.Fatal("il numero del d100 non deve essere mandato")
	}
	if m := ultimiMessaggi(cm, "dadi.sfortuna"); len(m) != 1 || strings.Contains(m[0], sfortunaPredefinita[1]) {
		t.Fatal("l'effetto arriva al Master, la tabella no")
	}
	if _, err := tv.ws(cl, "dadi.sfortuna", map[string]any{"tiro": r.ID}); err == nil {
		t.Fatal("la sfortuna si tira una volta sola per ogni 1")
	}
	// Tabella del Master: i giocatori non possono leggerla; con 2 effetti, d100 = 60 → secondo effetto.
	if code, _ := tv.luca.chiama("GET", "/sfortuna", nil); code != 403 {
		t.Fatal("la tabella è visibile solo al Master")
	}
	tv.m.deve(200, "PUT", "/sfortuna", map[string]any{"effetti": []string{"Primo guaio", "  ", "Secondo guaio"}})
	if l := tv.m.deve(200, "GET", "/sfortuna", nil)["effetti"].([]any); len(l) != 2 {
		t.Fatal("righe vuote ignorate")
	}
	seq = []int{1}
	r = tv.wsOK(cl, "dadi.tira", d20).(tiroDadi)
	seq = []int{60}
	if e := tv.wsOK(cl, "dadi.sfortuna", map[string]any{"tiro": r.ID}).(map[string]any); e["effetto"] != "Secondo guaio" {
		t.Fatalf("tabella personalizzata: %v", e["effetto"])
	}
	tv.m.deve(200, "PUT", "/sfortuna", map[string]any{"effetti": []string{}})
	if p := tv.m.deve(200, "GET", "/sfortuna", nil)["personalizzata"]; p != false {
		t.Fatal("elenco vuoto: torna la tabella predefinita")
	}
}

// Dadi reali: il Master segna un 20 o un 1 tirato al tavolo da un giocatore.
func TestDadoReale(t *testing.T) {
	// In questa versione lo scontro nasce con "Nuovo combattimento": si parte senza (iniz.fine) e lo si
	// riapre con iniz.nuovo dove l'originale usava iniz.inizia. Ugo (altro gruppo) non partecipa.
	tv := nuovoScontro(t)
	tv.az(tv.m, 200, "iniz.fine", nil)
	pgL, sess := tv.pg, tv.ses
	_, _ = pgL, sess
	cm, cl, cs := tv.clienteWS(tv.m), tv.clienteWS(tv.luca), tv.clienteWS(tv.ugo)
	segna := map[string]any{"sessione": sess, "personaggio": pgL, "nome": "critico"}
	if _, err := tv.ws(cm, "dadi.reale", segna); err == nil {
		t.Fatal("senza scontro in corso non si segna nulla")
	}
	tv.az(tv.m, 200, "iniz.nuovo", nil)
	if _, err := tv.ws(cl, "dadi.reale", segna); err == nil {
		t.Fatal("solo il Master segna i dadi reali")
	}
	ultimiMessaggi(cl, "")
	ultimiMessaggi(cs, "")
	r := tv.wsOK(cm, "dadi.reale", segna).(tiroDadi)
	if !r.Critico || !r.Reale || r.Chi != "Arin" || r.ChiID != tv.a.idUtente(tv.luca) {
		t.Fatalf("critico reale: %+v", r)
	}
	if len(ultimiMessaggi(cl, "dadi.risultato")) != 1 || len(ultimiMessaggi(cs, "dadi.risultato")) != 0 {
		t.Fatal("il CRITICO arriva ai partecipanti, non agli altri")
	}
	segna["nome"] = "fallimento"
	r = tv.wsOK(cm, "dadi.reale", segna).(tiroDadi)
	if !r.Fallimento || !r.Sfortuna || r.ID == "" {
		t.Fatalf("OOOPS reale: %+v", r)
	}
	if _, err := tv.ws(cm, "dadi.sfortuna", map[string]any{"tiro": r.ID}); err == nil {
		t.Fatal("il tasto è del giocatore, non del Master")
	}
	if e := tv.wsOK(cl, "dadi.sfortuna", map[string]any{"tiro": r.ID}).(map[string]any); e["chi"] != "Arin" {
		t.Fatalf("effetto per Arin: %v", e)
	}
	segna["nome"] = "boh"
	if _, err := tv.ws(cm, "dadi.reale", segna); err == nil {
		t.Fatal("tipo non valido")
	}
}

// Tiri dalla scheda: fuori dallo scontro li vedono chi tira e il Master (e il giocatore, se tira il Master
// dalla sua scheda); non gli altri giocatori.
func TestDadiDaScheda(t *testing.T) {
	tv := nuovoScontro(t)
	tv.az(tv.m, 200, "iniz.fine", nil)
	cm, cl, cs := tv.clienteWS(tv.m), tv.clienteWS(tv.luca), tv.clienteWS(tv.sara)
	d20 := []map[string]any{{"n": 1, "facce": 20}}
	ultimiMessaggi(cm, "")
	ultimiMessaggi(cl, "")
	ultimiMessaggi(cs, "")
	r := tv.wsOK(cl, "dadi.tira", map[string]any{"dadi": d20, "mod": 3, "etichetta": "Arin · TS Forza", "scheda": true, "personaggio": tv.pg}).(tiroDadi)
	if !r.Scheda || r.Scontro || r.Totale < 4 || r.Totale > 23 {
		t.Fatalf("tiro dalla scheda: %+v", r)
	}
	if len(ultimiMessaggi(cl, "dadi.risultato")) != 1 || len(ultimiMessaggi(cm, "dadi.risultato")) != 1 || len(ultimiMessaggi(cs, "dadi.risultato")) != 0 {
		t.Fatal("il tiro dalla scheda lo vedono il giocatore e il Master, non gli altri")
	}
	tv.wsOK(cm, "dadi.tira", map[string]any{"dadi": d20, "mod": 1, "etichetta": "Arin · Furtività", "scheda": true, "personaggio": tv.pg})
	if len(ultimiMessaggi(cm, "dadi.risultato")) != 1 || len(ultimiMessaggi(cl, "dadi.risultato")) != 1 || len(ultimiMessaggi(cs, "dadi.risultato")) != 0 {
		t.Fatal("il Master tira dalla scheda di Arin: lo vede anche Luca")
	}
	// un tiro normale fuori dallo scontro: lo vede anche il Master (2026-11-12), non gli altri giocatori
	tv.wsOK(cl, "dadi.tira", map[string]any{"dadi": d20})
	if len(ultimiMessaggi(cm, "dadi.risultato")) != 1 || len(ultimiMessaggi(cs, "dadi.risultato")) != 0 {
		t.Fatal("i tiri dei giocatori fuori dallo scontro li vede anche il Master, non gli altri")
	}
}

// Tiro manuale dalla scheda (dado reale): il giocatore scrive il d20, l'app aggiunge il modificatore; lo vedono
// giocatore e Master; con 20 CRITICO; valori fuori da 1–20 rifiutati.
func TestTiroManuale(t *testing.T) {
	sc := nuovoScontro(t)
	cl, cm := sc.clienteWS(sc.luca), sc.clienteWS(sc.m)
	ultimiMessaggi(cl, "")
	ultimiMessaggi(cm, "")
	r := sc.wsOK(cl, "dadi.manuale", map[string]any{"d20": 20, "mod": 5, "modo": "vantaggio", "etichetta": "Arin · TS Saggezza", "personaggio": sc.pg}).(tiroDadi)
	if r.Totale != 25 || !r.Critico || !r.Reale || !r.Scheda || !strings.Contains(r.Formula, "dado reale") || r.Modo != "vantaggio" {
		t.Fatalf("tiro manuale: %+v", r)
	}
	if len(ultimiMessaggi(cl, "dadi.risultato")) != 1 || len(ultimiMessaggi(cm, "dadi.risultato")) != 1 {
		t.Fatal("il tiro manuale lo vedono giocatore e Master")
	}
	if r := sc.wsOK(cl, "dadi.manuale", map[string]any{"d20": 1, "mod": -1}).(tiroDadi); r.Totale != 0 || !r.Fallimento {
		t.Fatalf("1 naturale: %+v", r)
	}
	for _, v := range []int{0, 21} {
		if _, err := sc.ws(cl, "dadi.manuale", map[string]any{"d20": v}); err == nil {
			t.Fatalf("d20 = %d: va rifiutato", v)
		}
	}
}
