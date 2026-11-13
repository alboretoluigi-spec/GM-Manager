package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installaPlugin copia la scheda d'esempio del kit nella cartella dei plugin, con il nome indicato.
func installaPlugin(t *testing.T, a *ambiente, id string) {
	t.Helper()
	dest := filepath.Join(a.s.Cfg.CartellaMedia, "SchedePlugin", id)
	os.MkdirAll(dest, 0o755)
	for _, f := range []string{"manifest.json", "index.html", "scheda.js", "stile.css"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "kit-scheda", "esempio", f))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dest, f), b, 0o644)
	}
}

func (c *cliente) prendi(percorso string) (int, http.Header, string) {
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
	return resp.StatusCode, resp.Header, string(b)
}

func TestSchedePlugin(t *testing.T) {
	tv := nuovoTavolo(t)
	a := tv.a
	installaPlugin(t, a, "esempio")
	// Una cartella con un manifest sbagliato e una con un nome non valido.
	rotta := filepath.Join(a.s.Cfg.CartellaMedia, "SchedePlugin", "rotta")
	os.MkdirAll(rotta, 0o755)
	os.WriteFile(filepath.Join(rotta, "manifest.json"), []byte(`{"nome":"Rotta","protocollo":2}`), 0o644)
	os.MkdirAll(filepath.Join(a.s.Cfg.CartellaMedia, "SchedePlugin", "Nome Sbagliato"), 0o755)

	l := tv.m.deve(200, "GET", "/schede-plugin", nil)["lista"].([]any)
	if len(l) != 3 {
		t.Fatalf("il Master vede tutte le cartelle: %v", l)
	}
	errori := map[string]string{}
	for _, x := range l {
		p := x.(map[string]any)
		e, _ := p["errore"].(string)
		errori[p["id"].(string)] = e
	}
	if errori["esempio"] != "" || !strings.Contains(errori["rotta"], "protocollo") || !strings.Contains(errori["Nome Sbagliato"], "nome della cartella") {
		t.Fatalf("controllo dei manifest: %v", errori)
	}
	if l := tv.luca.deve(200, "GET", "/schede-plugin", nil)["lista"].([]any); len(l) != 0 {
		t.Fatal("i giocatori vedono solo le schede abilitate")
	}
	// Non abilitata: non si serve e non si sceglie.
	if c, _, _ := tv.luca.prendi("/plugin-schede/esempio/index.html"); c != 404 {
		t.Fatal("scheda non abilitata: file non serviti")
	}
	tv.luca.deve(400, "POST", "/personaggi", map[string]any{"nome": "Arin", "scheda": "esempio"})
	tv.luca.deve(403, "PUT", "/schede-plugin/esempio", map[string]any{"abilitato": true})
	tv.m.deve(400, "PUT", "/schede-plugin/rotta", map[string]any{"abilitato": true})
	tv.m.deve(200, "PUT", "/schede-plugin/esempio", map[string]any{"abilitato": true})

	l = tv.luca.deve(200, "GET", "/schede-plugin", nil)["lista"].([]any)
	if len(l) != 1 || l[0].(map[string]any)["url"] != "/plugin-schede/esempio/index.html" || l[0].(map[string]any)["nome"] != "Scheda d'esempio" {
		t.Fatalf("scheda abilitata visibile ai giocatori: %v", l)
	}
	// File serviti in una sandbox, senza rete.
	c, h, corpo := tv.luca.prendi("/plugin-schede/esempio/index.html")
	csp := h.Get("Content-Security-Policy")
	if c != 200 || !strings.Contains(corpo, "scheda.js") || !strings.HasPrefix(csp, "sandbox allow-scripts") || strings.Contains(csp, "allow-same-origin") || !strings.Contains(csp, "connect-src https://") || !strings.Contains(csp, "/plugin-schede/esempio/;") {
		t.Fatalf("pagina del plugin isolata: %d %q", c, csp)
	}
	if c, h, _ := tv.luca.prendi("/plugin-schede/esempio/scheda.js"); c != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/javascript") {
		t.Fatal("script del plugin")
	}
	for _, p := range []string{"/plugin-schede/esempio/../../gasterbless.db", "/plugin-schede/esempio/%2e%2e/rotta/manifest.json", "/plugin-schede/esempio/.nascosto.js",
		"/plugin-schede/esempio/manifest.exe", "/plugin-schede/rotta/manifest.json", "/plugin-schede/esempio/manca.js"} {
		if c, _, _ := tv.luca.prendi(p); c != 404 && c != 400 {
			t.Fatalf("%s: %d", p, c)
		}
	}
	// Personaggio con la scheda alternativa.
	id := tv.luca.deve(200, "POST", "/personaggi", map[string]any{"nome": "Arin", "scheda": "esempio"})["id"].(string)
	if p := tv.luca.deve(200, "GET", "/personaggi/"+id, nil)["personaggio"].(map[string]any); p["scheda"] != "esempio" {
		t.Fatalf("personaggio con la scheda alternativa: %v", p)
	}
	// Lo stato del plugin si salva come quello della scheda di Gaster Bless; PF condivisi con il tracker.
	tv.luca.deve(200, "PATCH", "/personaggi/"+id+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"classe": "Chierico", "car.DES": 14, "hpCur": 20},
		"riepilogo": map[string]any{"class": "Chierico", "level": 3, "ac": 16, "hpMax": 24, "initiative": 2, "dex": 14}})
	p := tv.luca.deve(200, "GET", "/personaggi/"+id, nil)
	if r := p["personaggio"].(map[string]any)["riepilogo"].(map[string]any); r["ca"] != 16.0 || r["pfMax"] != 24.0 || r["iniziativa"] != 2.0 {
		t.Fatalf("riepilogo per il tracker: %v", r)
	}
	if d := p["dati"].(map[string]any); d["classe"] != "Chierico" || fmt.Sprint(d["car"].(map[string]any)["DES"]) != "14" || fmt.Sprint(d["hpCur"]) != "20" {
		t.Fatalf("dati della scheda alternativa: %v", d)
	}
	// Cambio della scheda: solo il Master, solo verso schede abilitate.
	tv.luca.deve(403, "PUT", "/personaggi/"+id+"/tipo-scheda", map[string]any{"scheda": ""})
	tv.m.deve(400, "PUT", "/personaggi/"+id+"/tipo-scheda", map[string]any{"scheda": "rotta"})
	tv.m.deve(200, "PUT", "/personaggi/"+id+"/tipo-scheda", map[string]any{"scheda": ""})
	tv.m.deve(200, "PUT", "/personaggi/"+id+"/tipo-scheda", map[string]any{"scheda": "esempio"})
	for _, x := range tv.m.deve(200, "GET", "/schede-plugin", nil)["lista"].([]any) {
		if p := x.(map[string]any); p["id"] == "esempio" && fmt.Sprint(p["personaggi"]) != "1" {
			t.Fatalf("il Master vede quanti personaggi la usano: %v", p["personaggi"])
		}
	}
	// Disabilitata: file non più serviti; il personaggio resta con la sua scelta.
	tv.m.deve(200, "PUT", "/schede-plugin/esempio", map[string]any{"abilitato": false})
	if c, _, _ := tv.luca.prendi("/plugin-schede/esempio/index.html"); c != 404 {
		t.Fatal("disabilitata: non si serve più")
	}
	if p := tv.luca.deve(200, "GET", "/personaggi/"+id, nil)["personaggio"].(map[string]any); p["scheda"] != "esempio" {
		t.Fatal("la scelta del personaggio resta")
	}
}
