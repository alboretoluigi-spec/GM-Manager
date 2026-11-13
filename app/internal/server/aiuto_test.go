package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"testing/fstest"

	"gasterbless/internal/config"
)

// ambiente di prova: server HTTPS vero (httptest), database temporaneo, chiave senza DPAPI.
type ambiente struct {
	t   *testing.T
	s   *Server
	srv *httptest.Server
}

func nuovoAmbiente(t *testing.T) *ambiente {
	t.Helper()
	cfg := config.Predefinita()
	cfg.CartellaDati = t.TempDir()
	cfg.CartellaMedia = t.TempDir()
	cfg.ModalitaProva = true
	cfg.ApprovazioneSchede = false // i test delle altre funzioni salvano le schede come prima; TestModificaManuale la accende
	locs, err := os.ReadFile("../../web/carta/locs.json")
	if err != nil {
		t.Fatal(err)
	}
	best, err := os.ReadFile("../../web/carta/bestiario.json")
	if err != nil {
		t.Fatal(err)
	}
	prof, err := os.ReadFile("../../web/regole/professioni.json")
	if err != nil {
		t.Fatal(err)
	}
	web := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>Gaster Bless</title>")}, "carta/locs.json": {Data: locs}, "carta/bestiario.json": {Data: best},
		"regole/professioni.json":      {Data: prof},
		"carta/img/lm/borgofosco.webp": {Data: []byte("RIFF....WEBPVP8 ")}}
	s, err := Nuovo(cfg, web)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	// Solo nei test: l'intestazione X-Test-IP simula un dispositivo con un altro indirizzo.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := r.Header.Get("X-Test-IP"); ip != "" {
			r.RemoteAddr = ip + ":40000"
		}
		h.ServeHTTP(w, r)
	}))
	srv.StartTLS()
	t.Cleanup(func() { srv.Close(); s.Chiudi() })
	return &ambiente{t: t, s: s, srv: srv}
}

// cliente simula un dispositivo: ha i suoi cookie, il suo token CSRF e il suo indirizzo IP.
type cliente struct {
	a    *ambiente
	c    *http.Client
	csrf string
	ip   string
}

func (a *ambiente) cliente(ip string) *cliente {
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, Jar: jar}
	return &cliente{a: a, c: hc, ip: ip}
}

// chiama esegue una richiesta e restituisce codice HTTP e JSON decodificato.
func (c *cliente) chiama(metodo, percorso string, corpo any) (int, map[string]any) {
	c.a.t.Helper()
	var r io.Reader
	if corpo != nil {
		b, _ := json.Marshal(corpo)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(metodo, c.a.srv.URL+"/api/v1"+percorso, r)
	if corpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF", c.csrf)
	}
	if c.ip != "" {
		req.Header.Set("X-Test-IP", c.ip)
	}
	resp, err := c.c.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	b, _ := io.ReadAll(resp.Body)
	if len(b) > 0 && b[0] == '[' {
		var lista []any
		json.Unmarshal(b, &lista)
		out = map[string]any{"lista": lista}
	} else {
		json.Unmarshal(b, &out)
	}
	if v, ok := out["csrf"].(string); ok && v != "" {
		c.csrf = v
	}
	return resp.StatusCode, out
}

func (c *cliente) deve(codice int, metodo, percorso string, corpo any) map[string]any {
	c.a.t.Helper()
	got, out := c.chiama(metodo, percorso, corpo)
	if got != codice {
		c.a.t.Fatalf("%s %s: atteso %d, ottenuto %d (%v)", metodo, percorso, codice, got, out)
	}
	return out
}

// master crea l'account del Master dal PC locale e restituisce il suo cliente.
func (a *ambiente) master() *cliente {
	m := a.cliente("")
	m.deve(200, "POST", "/primo-avvio", map[string]any{"nome": "Master", "password": "Il-castello-di-Corvonero"})
	m.deve(200, "GET", "/io", nil)
	return m
}

// giocatore registra un giocatore da un dispositivo della rete di casa.
func (a *ambiente) giocatore(nome, tel, ip string) *cliente {
	g := a.cliente(ip)
	g.deve(200, "POST", "/registrazione", map[string]any{"nome": nome, "telefono": tel, "password": "Nebbia sul lago Varnhold",
		"conferma": "Nebbia sul lago Varnhold", "consenso": true, "avvisi": true})
	g.deve(200, "GET", "/io", nil)
	return g
}

func (a *ambiente) idUtente(c *cliente) string {
	_, io := c.chiama("GET", "/io", nil)
	return io["utente"].(map[string]any)["id"].(string)
}

func jsonString(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}
