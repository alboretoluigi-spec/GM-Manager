package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// statico chiede un file dell'app (fuori da /api/v1) con i cookie del cliente.
func (c *cliente) statico(percorso string) (int, string) {
	c.a.t.Helper()
	req, _ := http.NewRequest("GET", c.a.srv.URL+percorso, nil)
	if c.ip != "" {
		req.Header.Set("X-Test-IP", c.ip)
	}
	resp, err := c.c.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// Pacchetti di contenuti (fase 3c, C1): il modello si importa e arriva alla scheda (dati.js e talenti.js) per il
// Master e i giocatori, non a chi non è collegato; doppioni ed errori del file sono spiegati con il punto preciso;
// disattivare, sovrascrivere, esportare ed eliminare; solo il Master gestisce i pacchetti.
func TestPacchettiContenuti(t *testing.T) {
	a := nuovoAmbiente(t)
	web := a.s.Web.(fstest.MapFS)
	for _, f := range []string{"regole/dati.js", "regole/talenti.js"} {
		b, err := os.ReadFile("../../web/" + f)
		if err != nil {
			t.Fatal(err)
		}
		web[f] = &fstest.MapFile{Data: b}
	}
	modello, err := os.ReadFile("../../web/modelli/modello_pacchetto.json")
	if err != nil {
		t.Fatal(err)
	}
	m := a.master()
	g := a.giocatore("Lia", "3330000001", "192.168.1.20")
	importa := func(c *cliente, corpo []byte, atteso int) map[string]any {
		t.Helper()
		cod, _, b := c.grezzo("POST", "/contenuti", corpo, "application/json")
		if cod != atteso {
			t.Fatalf("importazione: atteso %d, ottenuto %d (%s)", atteso, cod, b)
		}
		out := map[string]any{}
		json.Unmarshal(b, &out)
		return out
	}
	verifica := func(corpo string) string {
		t.Helper()
		cod, _, b := m.grezzo("POST", "/contenuti/verifica", []byte(corpo), "application/json")
		if cod != 400 {
			t.Fatalf("verifica di un file sbagliato: atteso 400, ottenuto %d (%s)", cod, b)
		}
		out := map[string]any{}
		json.Unmarshal(b, &out)
		return out["errore"].(string)
	}

	// Prima di qualsiasi pacchetto la scheda non ha contenuti.
	if _, js := m.statico("/regole/dati.js"); strings.Contains(js, "Custode") {
		t.Fatal("contenuti presenti senza pacchetti")
	}
	// Anteprima del modello senza salvarlo.
	cod, _, b := m.grezzo("POST", "/contenuti/verifica", modello, "application/json")
	if cod != 200 || !strings.Contains(string(b), `"classi":1`) || !strings.Contains(string(b), `"incantesimi":2`) {
		t.Fatalf("verifica del modello: %d %s", cod, b)
	}
	if n := len(m.deve(200, "GET", "/contenuti", nil)["pacchetti"].([]any)); n != 0 {
		t.Fatalf("la verifica non deve salvare: %d pacchetti", n)
	}
	// Importazione (con il segno BOM che aggiunge il Blocco note di Windows) e doppione.
	out := importa(m, append([]byte("\xef\xbb\xbf"), modello...), 200)
	if av, _ := out["avvisi"].([]any); len(av) != 0 {
		t.Fatalf("il modello non deve dare avvisi: %v", av)
	}
	id := out["id"].(string)
	if out := importa(m, modello, 409); !strings.Contains(out["errore"].(string), "già stato importato") {
		t.Fatalf("doppione: %v", out)
	}
	importa(g, modello, 403)

	// Il Master e i giocatori ricevono i contenuti nella scheda; chi non è collegato riceve solo il file dell'app.
	for _, c := range []*cliente{m, g} {
		_, js := c.statico("/regole/dati.js")
		for _, atteso := range []string{`"custode":{`, `"n":"Custode delle Lanterne"`, `"f":"1:Vigilanza|Luce Guida;2:Incantesimi;3:Ordine della Lanterna;5:Fiamma Costante"`,
			`["Ordine del Faro","MOD","3:Segnale Lontano;7:Sguardo del Faro",{"skFix":["Percezione"]}]`, `"asi":[["COS",2]]`,
			`"valle":{`, `["Scintilla del Custode","Keeper's Spark",0,"C","a","18 m","V, S","I","l","MOD"`,
			`["Velo di Nebbia Chiara","Bright Mist Veil",1,"I","*a","Sé stesso","V, S, M","C10m","l"`,
			`["Bastone ferrato","s","m","1d6","contundente","Versatile (1d8)","v"]`, `["Giubba imbottita di lana","l",11,0,0]`,
			`"codici":{"custode":"l"}`, `"nomiListe":{"l":"Custode"}`, `"custode|Luce Guida":`, `"Occhio per le Strade":`, "Object.assign(CLASSES,P.classi)"} {
			if !strings.Contains(js, atteso) {
				t.Fatalf("dati.js: manca %s\n%s", atteso, js[strings.Index(js, "===== Pacchetti"):])
			}
		}
		if _, tal := c.statico("/regole/talenti.js"); !strings.Contains(tal, `["Passo Leggero","Light Step","MOD",["DES"],{"speed":1.5}`) {
			t.Fatalf("talenti.js senza il talento del pacchetto: %s", tal)
		}
	}
	anon := a.cliente("")
	if cod, js := anon.statico("/regole/dati.js"); cod != 200 || strings.Contains(js, "Custode") {
		t.Fatalf("senza accesso: %d, contenuti presenti=%v", cod, strings.Contains(js, "Custode"))
	}

	// Errori del file, con il punto preciso.
	casi := map[string]string{
		`{`: "vuoto o incompleto",
		`{"formato":"altro","versioneFormato":1}`:                                                                                                                                            "non è un pacchetto di contenuti di Gaster Bless",
		`{"formato":"gasterbless-contenuti","versioneFormato":9,"sistema":"d20-5e","nome":"x"}`:                                                                                              "Versione del formato 9",
		`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d100","nome":"x"}`:                                                                                                "non ancora supportato",
		`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d20-5e","nome":"x"}`:                                                                                              "non contiene nessuna voce",
		`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d20-5e","nome":"x","classi":[{"id":"a","nome":"A","dadi":8}]}`:                                                    `classi n. 1 («A»): campo sconosciuto "dadi"`,
		`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d20-5e","nome":"x","classi":[{"id":"Spadaccino","nome":"S","dadoVita":30,"tiriSalvezza":["FORZA"]}]}`:             "dadoVita» deve essere tra 4 e 20",
		`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d20-5e","nome":"x","razze":[{"id":"r","nome":"R","caratteristiche":{"FOR":"due"}}]}`:                              "razze n. 1 («R»): «caratteristiche.FOR» deve essere un numero intero",
		`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d20-5e","nome":"x","incantesimi":[{"nome":"Lampo","livello":12,"scuola":"fuoco","tempo":"azione","classi":"q"}]}`: "la lista «q» non esiste",
		`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d20-5e","nome":"x","armi":[{"nome":"Mazza","categoria":"pesante","tipo":"mischia","danno":"tanto"}]}`:             "«categoria» deve essere «semplice» o «guerra»",
		`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d20-5e","nome":"x","talenti":[{"nome":"T","effetti":{"volo":1}}]}`:                                                "«effetti»: «volo» sconosciuto",
	}
	for corpo, atteso := range casi {
		if e := verifica(corpo); !strings.Contains(e, atteso) {
			t.Errorf("file %s: atteso «%s», ottenuto «%s»", corpo, atteso, e)
		}
	}
	if e := verifica(`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d20-5e","nome":"x","classi":[{"id":"Spadaccino","nome":"S","dadoVita":30,"tiriSalvezza":["FORZA"]}]}`); !strings.Contains(e, "«id» deve essere") || !strings.Contains(e, "«FORZA» non è una caratteristica") {
		t.Errorf("tutti gli errori insieme: %s", e)
	}
	if e := verifica(`{"formato": "gasterbless-contenuti", "nome": "x",}`); !strings.Contains(e, "non è un JSON valido") {
		t.Errorf("JSON sbagliato: %s", e)
	}
	if cod, _, _ := m.grezzo("POST", "/contenuti", nil, "application/json"); cod != 400 {
		t.Fatalf("file vuoto: %d", cod)
	}
	// Un'abilità sconosciuta è solo un avviso.
	out = importa(m, []byte(`{"formato":"gasterbless-contenuti","versioneFormato":1,"sistema":"d20-5e","nome":"Seconda versione","classi":[{"id":"custode","nome":"Custode rivisto","dadoVita":10,"abilita":["Cucina"]}]}`), 200)
	if av := out["avvisi"].([]any); len(av) != 1 || !strings.Contains(av[0].(string), "Cucina") {
		t.Fatalf("avviso sull'abilità sconosciuta: %v", out["avvisi"])
	}
	id2 := out["id"].(string)
	// Il pacchetto importato dopo vince e viene segnalato.
	el := m.deve(200, "GET", "/contenuti", nil)
	if sv := el["sovrascritti"].([]any); len(sv) != 1 || sv[0] != "classe «custode»" {
		t.Fatalf("sovrascritti: %v", sv)
	}
	if _, js := g.statico("/regole/dati.js"); !strings.Contains(js, "Custode rivisto") || strings.Contains(js, `"n":"Custode delle Lanterne"`) {
		t.Fatal("il secondo pacchetto deve sostituire la classe")
	}
	// Disattivato: la scheda torna alla classe del primo.
	m.deve(200, "PATCH", "/contenuti/"+id2, map[string]any{"attivo": false})
	if _, js := g.statico("/regole/dati.js"); strings.Contains(js, "Custode rivisto") || !strings.Contains(js, "Custode delle Lanterne") {
		t.Fatal("pacchetto disattivato ancora nella scheda")
	}
	m.deve(400, "PATCH", "/contenuti/"+id2, map[string]any{})
	m.deve(404, "PATCH", "/contenuti/nessuno", map[string]any{"attivo": true})
	g.deve(403, "PATCH", "/contenuti/"+id2, map[string]any{"attivo": true})
	// Esportazione: il file originale.
	cod, h, file := m.grezzo("GET", "/contenuti/"+id+"/file", nil, "")
	if cod != 200 || string(file) != string(modello) || !strings.Contains(h.Get("Content-Disposition"), "attachment") {
		t.Fatalf("esportazione: %d %s", cod, h)
	}
	if cod, _, _ := g.grezzo("GET", "/contenuti/"+id+"/file", nil, ""); cod != 403 {
		t.Fatalf("esportazione da un giocatore: %d", cod)
	}
	g.deve(403, "GET", "/contenuti", nil)
	// Eliminazione.
	g.deve(403, "DELETE", "/contenuti/"+id, nil)
	m.deve(200, "DELETE", "/contenuti/"+id, nil)
	m.deve(404, "DELETE", "/contenuti/"+id, nil)
	if _, js := g.statico("/regole/dati.js"); strings.Contains(js, "Custode delle Lanterne") {
		t.Fatal("pacchetto eliminato ancora nella scheda")
	}
	if p := m.deve(200, "GET", "/contenuti", nil)["pacchetti"].([]any); len(p) != 1 || p[0].(map[string]any)["attivo"] != false {
		t.Fatalf("resta il secondo, disattivato: %v", p)
	}
}
