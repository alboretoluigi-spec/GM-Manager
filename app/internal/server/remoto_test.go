package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gasterbless/internal/mqtt"
)

// giocatoreLontano simula la pagina /remoto/ di un giocatore: si collega al server MQTT di prova,
// legge il proprio stato cifrato e invia azioni firmate con la sua chiave.
type giocatoreLontano struct {
	t      *testing.T
	cl     *mqtt.Client
	canale string
	alias  string
	chiave []byte
	mu     sync.Mutex
	stato  map[string]any
	esiti  map[string]map[string]any
	eventi []map[string]any // esiti degli scontri inoltrati (attacco.esito, incantesimo.esito)
}

func collegaLontano(t *testing.T, url string, conf map[string]any) *giocatoreLontano {
	t.Helper()
	k, err := base64.RawURLEncoding.DecodeString(conf["k"].(string))
	if err != nil {
		t.Fatal(err)
	}
	g := &giocatoreLontano{t: t, canale: conf["t"].(string), alias: conf["u"].(string), chiave: k, esiti: map[string]map[string]any{}}
	g.cl, err = mqtt.Dial(context.Background(), url, "prova-"+g.alias, func(topic string, dati []byte) {
		if len(dati) == 0 {
			g.mu.Lock()
			g.stato = nil
			g.mu.Unlock()
			return
		}
		tipo := "stato"
		if strings.Contains(topic, "/e/") {
			tipo = "esito"
		}
		chiaro, err := apri(g.chiave, dati, tipo+"|"+g.canale+"|"+g.alias)
		if err != nil {
			t.Errorf("messaggio non decifrabile su %s", topic)
			return
		}
		var m map[string]any
		json.Unmarshal(chiaro, &m)
		g.mu.Lock()
		if tipo == "stato" {
			g.stato = m
		} else if m["evento"] != nil {
			g.eventi = append(g.eventi, m)
		} else {
			g.esiti[m["n"].(string)] = m
		}
		g.mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.cl.Close)
	g.cl.Subscribe("gasterbless/" + g.canale + "/s/" + g.alias)
	g.cl.Subscribe("gasterbless/" + g.canale + "/e/" + g.alias)
	return g
}

// aspetta finché la condizione sullo stato ricevuto è vera.
func (g *giocatoreLontano) aspetta(cosa string, ok func(stato map[string]any) bool) map[string]any {
	g.t.Helper()
	for fine := time.Now().Add(5 * time.Second); time.Now().Before(fine); time.Sleep(20 * time.Millisecond) {
		g.mu.Lock()
		s := g.stato
		g.mu.Unlock()
		if ok(s) {
			return s
		}
	}
	g.t.Fatalf("il giocatore remoto non ha ricevuto: %s (ultimo stato %v)", cosa, g.stato)
	return nil
}

func (g *giocatoreLontano) azione(n, tipo string, dati map[string]any, quando time.Time) {
	j, _ := json.Marshal(map[string]any{"tipo": tipo, "dati": dati, "ts": quando.UnixMilli(), "n": n})
	c := sigilla(g.chiave, j, "azione|"+g.canale+"|"+g.alias)
	b, _ := json.Marshal(map[string]string{"u": g.alias, "c": base64.StdEncoding.EncodeToString(c)})
	if err := g.cl.Publish("gasterbless/"+g.canale+"/a", b, false); err != nil {
		g.t.Fatal(err)
	}
}

func (g *giocatoreLontano) esito(n string) map[string]any {
	g.t.Helper()
	for fine := time.Now().Add(5 * time.Second); time.Now().Before(fine); time.Sleep(20 * time.Millisecond) {
		g.mu.Lock()
		e := g.esiti[n]
		g.mu.Unlock()
		if e != nil {
			return e
		}
	}
	g.t.Fatalf("nessun esito per l'azione %s", n)
	return nil
}

func pfDi(stato map[string]any, nome string) float64 {
	if stato == nil {
		return -1
	}
	for _, x := range stato["vista"].(map[string]any)["combattenti"].([]any) {
		c := x.(map[string]any)
		if c["nome"] == nome && c["pf"] != nil {
			return c["pf"].(float64)
		}
	}
	return -1
}

func TestGiocoDaRemoto(t *testing.T) {
	broker := mqtt.NuovoBrokerDiProva()
	hs := httptest.NewServer(broker)
	defer hs.Close()
	url := "ws" + strings.TrimPrefix(hs.URL, "http")

	sc := nuovoScontro(t)
	sc.a.s.Cfg.BrokerMQTT = []string{url}
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "l'antagonista", "quanti": 1, "ca": 16, "pf": 144, "nascosto": true})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Lupo", "quanti": 1, "ca": 13, "pf": 11})

	// Spento: niente link, solo il Master vede lo stato.
	sc.luca.deve(403, "GET", "/sessioni/"+sc.ses+"/remoto", nil)
	sc.m.deve(409, "POST", "/sessioni/"+sc.ses+"/remoto/giocatori", map[string]any{"utente": sc.idLuca})

	sc.m.deve(200, "PATCH", "/sessioni/"+sc.ses, map[string]any{"mqttRemoto": true})
	r := sc.m.deve(200, "GET", "/sessioni/"+sc.ses+"/remoto", nil)
	if r["attivo"] != true {
		t.Fatalf("il gioco da remoto deve risultare attivo: %v", r)
	}
	// Ugo non partecipa: niente link.
	sc.m.deve(404, "POST", "/sessioni/"+sc.ses+"/remoto/giocatori", map[string]any{"utente": "inesistente"})
	l := sc.m.deve(200, "POST", "/sessioni/"+sc.ses+"/remoto/giocatori", map[string]any{"utente": sc.idLuca})
	link := l["link"].(string)
	if !strings.Contains(link, "/remoto/#") || !strings.HasPrefix(l["qr"].(string), "data:image/png;base64,") {
		t.Fatalf("link o QR mancanti: %v", l)
	}
	// La chiave è solo nella parte dopo "#" (mai inviata ai server).
	conf := l["configurazione"].(map[string]any)
	if strings.Contains(strings.Split(link, "#")[0], conf["k"].(string)) {
		t.Fatal("la chiave non deve stare nella parte del link inviata al server")
	}

	g := collegaLontano(t, url, conf)
	// Riceve lo stato (messaggio conservato): niente mostro nascosto, niente PF esatti dei mostri.
	s := g.aspetta("lo stato iniziale", func(s map[string]any) bool { return pfDi(s, "Arin") == 20 })
	if s["titolo"] != "La Tana del Lupo" {
		t.Fatalf("titolo sbagliato: %v", s["titolo"])
	}
	j, _ := json.Marshal(s)
	if strings.Contains(string(j), "l'antagonista") || strings.Contains(string(j), sc.idLuca) || strings.Contains(string(j), "1234567") {
		t.Fatalf("lo stato remoto contiene dati che non deve: %s", j)
	}
	if pfDi(s, "Lupo") != -1 {
		t.Fatal("i PF esatti dei mostri non devono uscire")
	}
	for _, x := range s["vista"].(map[string]any)["combattenti"].([]any) {
		if c := x.(map[string]any); c["nome"] == "Arin" && c["mio"] != true {
			t.Fatal("Arin deve risultare del giocatore remoto")
		}
	}
	// Sul server MQTT resta solo byte cifrati.
	if b := broker.Conservato("gasterbless/" + conf["t"].(string) + "/s/" + conf["u"].(string)); b == nil || strings.Contains(string(b), "Arin") {
		t.Fatal("lo stato conservato deve esserci ed essere cifrato")
	}

	// Azione valida: danno al proprio PG; lo stato si aggiorna anche per il Master.
	g.azione("a1", "pg.danno", map[string]any{"personaggio": sc.pg, "valore": 5}, time.Now())
	if e := g.esito("a1"); e["ok"] != true {
		t.Fatalf("l'azione doveva riuscire: %v", e)
	}
	g.aspetta("i PF dopo il danno", func(s map[string]any) bool { return pfDi(s, "Arin") == 15 })
	if pf := cerca(sc.vista(sc.m), "Arin")["pf"].(float64); pf != 15 {
		t.Fatalf("il Master deve vedere 15 PF, vede %v", pf)
	}
	// La stessa azione ripetuta viene ignorata.
	g.azione("a1", "pg.danno", map[string]any{"personaggio": sc.pg, "valore": 5}, time.Now())
	// Azione vecchia (orologio sbagliato o messaggio registrato e rinviato): rifiutata.
	g.azione("a2", "pg.danno", map[string]any{"personaggio": sc.pg, "valore": 5}, time.Now().Add(-10*time.Minute))
	if e := g.esito("a2"); e["ok"] != false {
		t.Fatalf("l'azione vecchia doveva essere rifiutata: %v", e)
	}
	// Azioni non ammesse da remoto o su ciò che non è suo: rifiutate con gli stessi permessi dell'app.
	g.azione("a3", "iniz.fine", nil, time.Now())
	if e := g.esito("a3"); e["ok"] != false {
		t.Fatalf("iniz.fine non è ammessa da remoto: %v", e)
	}
	lupo := cerca(sc.vista(sc.m), "Lupo")["id"].(string)
	g.azione("a4", "iniz.iniziativa", map[string]any{"combattente": lupo, "valore": 30}, time.Now())
	if e := g.esito("a4"); e["ok"] != false {
		t.Fatalf("non può cambiare l'iniziativa di un mostro: %v", e)
	}
	// Un messaggio con una chiave sbagliata non produce nulla.
	falso := &giocatoreLontano{t: t, cl: g.cl, canale: g.canale, alias: g.alias, chiave: make([]byte, 32)}
	falso.azione("a5", "pg.danno", map[string]any{"personaggio": sc.pg, "valore": 5}, time.Now())
	time.Sleep(300 * time.Millisecond)
	if pf := cerca(sc.vista(sc.m), "Arin")["pf"].(float64); pf != 15 {
		t.Fatalf("un'azione falsa o ripetuta ha cambiato i PF: %v", pf)
	}

	// Revoca: lo stato conservato sparisce e le azioni vengono ignorate.
	sc.m.deve(200, "DELETE", "/sessioni/"+sc.ses+"/remoto/giocatori/"+sc.idLuca, nil)
	g.aspetta("la cancellazione dello stato", func(s map[string]any) bool { return s == nil })
	g.azione("a6", "pg.danno", map[string]any{"personaggio": sc.pg, "valore": 5}, time.Now())
	time.Sleep(300 * time.Millisecond)
	if pf := cerca(sc.vista(sc.m), "Arin")["pf"].(float64); pf != 15 {
		t.Fatal("dopo la revoca le azioni devono essere ignorate")
	}

	// Nuovo link, poi la sessione si conclude: tutto cancellato dal server MQTT e chiavi buttate.
	l = sc.m.deve(200, "POST", "/sessioni/"+sc.ses+"/remoto/giocatori", map[string]any{"utente": sc.idLuca})
	conf = l["configurazione"].(map[string]any)
	argomento := "gasterbless/" + conf["t"].(string) + "/s/" + conf["u"].(string)
	for fine := time.Now().Add(5 * time.Second); broker.Conservato(argomento) == nil && time.Now().Before(fine); {
		time.Sleep(20 * time.Millisecond)
	}
	if broker.Conservato(argomento) == nil {
		t.Fatal("con il nuovo link lo stato deve essere pubblicato")
	}
	sc.m.deve(200, "PATCH", "/sessioni/"+sc.ses, map[string]any{"stato": "conclusa"})
	for fine := time.Now().Add(5 * time.Second); broker.Conservato(argomento) != nil && time.Now().Before(fine); {
		time.Sleep(20 * time.Millisecond)
	}
	if broker.Conservato(argomento) != nil {
		t.Fatal("alla fine della sessione lo stato deve essere cancellato dal server MQTT")
	}
	if v, _ := sc.a.s.impostazione("remoto:" + sc.ses); v != "" {
		t.Fatal("alla fine della sessione le chiavi devono essere cancellate")
	}
	if r := sc.m.deve(200, "GET", "/sessioni/"+sc.ses+"/remoto", nil); r["attivo"] != false {
		t.Fatal("una sessione conclusa non è più in remoto")
	}
}
