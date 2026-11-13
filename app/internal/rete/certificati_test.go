package rete

import (
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"testing"
)

// Il nome Tailscale del PC (nomi_certificato) entra nel certificato, firmato dalla stessa autorità.
func TestNomiExtraNelCertificato(t *testing.T) {
	c, err := CaricaCertificati(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	foglia := func() *x509.Certificate {
		cert, err := c.ConfigTLS().GetCertificate(nil)
		if err != nil {
			t.Fatal(err)
		}
		x, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			t.Fatal(err)
		}
		return x
	}
	if slices.Contains(foglia().DNSNames, "pc-master.tail1a2b.ts.net") {
		t.Fatal("all'inizio il nome non c'è")
	}
	c.ImpostaNomiExtra([]string{"pc-master.tail1a2b.ts.net", "localhost"})
	if err := c.Aggiorna(); err != nil {
		t.Fatal(err)
	}
	f := foglia()
	if !slices.Contains(f.DNSNames, "pc-master.tail1a2b.ts.net") {
		t.Fatalf("nome Tailscale nel certificato: %v", f.DNSNames)
	}
	n := 0
	for _, x := range f.DNSNames {
		if x == "localhost" {
			n++
		}
	}
	if n != 1 {
		t.Fatal("nessun doppione")
	}
	if _, err := f.Verify(x509.VerifyOptions{DNSName: "pc-master.tail1a2b.ts.net", Roots: c.PoolAutorita()}); err != nil {
		t.Fatalf("il certificato vale per il nome Tailscale con l'autorità di Gaster Bless: %v", err)
	}
}

// Porta pubblica (Tailscale Funnel): chi arriva da Internet non è mai "il PC", né la rete di casa, né la VPN.
func TestPortaPubblica(t *testing.T) {
	f := NuovoFiltro([]string{"100.64.0.0/10"}, false)
	var visto struct {
		ip            netip.Addr
		locale, pubbl bool
		host          string
	}
	h := f.PortaPubblica(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visto.ip, visto.locale, visto.pubbl, visto.host = IPRichiesta(r), Locale(r), Pubblica(r), r.Host
	}), "pc-master.tail1a2b.ts.net")
	prova := func(remoto, xff string) int {
		r := httptest.NewRequest("GET", "http://127.0.0.1:7602/", nil)
		r.RemoteAddr = remoto
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		visto.ip = netip.Addr{}
		h.ServeHTTP(w, r)
		return w.Code
	}
	if prova("127.0.0.1:5000", "203.0.113.7") != 403 {
		t.Fatal("accesso da Internet chiuso all'inizio")
	}
	f.ImpostaPubblico(true)
	if prova("192.168.1.20:5000", "203.0.113.7") != 403 {
		t.Fatal("la porta pubblica accetta solo il PC stesso (Tailscale)")
	}
	casi := map[string]string{"": "192.0.2.1", "203.0.113.7": "203.0.113.7", "1.2.3.4, 203.0.113.9": "203.0.113.9",
		"192.168.1.5": "192.0.2.1", "203.0.113.7, 127.0.0.1": "192.0.2.1", "100.100.1.1": "192.0.2.1", "10.0.0.8": "192.0.2.1", "spazzatura": "192.0.2.1"}
	for xff, atteso := range casi {
		if prova("127.0.0.1:5000", xff) != 200 || visto.ip.String() != atteso || visto.locale || !visto.pubbl || visto.host != "pc-master.tail1a2b.ts.net" {
			t.Fatalf("X-Forwarded-For %q: ip %v locale %v pubblica %v host %q", xff, visto.ip, visto.locale, visto.pubbl, visto.host)
		}
		if f.Ammesso(visto.ip) {
			t.Fatalf("%v non deve passare dal filtro della rete di casa", visto.ip)
		}
	}
	// Il filtro generale lascia passare le richieste pubbliche solo con l'accesso da Internet aperto.
	ok := 0
	m := f.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ok++ }))
	hp := f.PortaPubblica(m, "pc-master.tail1a2b.ts.net")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1"
	hp.ServeHTTP(httptest.NewRecorder(), r)
	f.ImpostaPubblico(false)
	w := httptest.NewRecorder()
	hp.ServeHTTP(w, r)
	if ok != 1 || w.Code != 403 {
		t.Fatalf("filtro: passate %d, codice %d", ok, w.Code)
	}
}

// Accesso da Internet chiuso: chi apre Gaster Bless dal browser o dall'icona vede una pagina chiara (con il
// collegamento alla Gaster Bless di casa); l'API e le app esterne ricevono il JSON con il codice.
func TestPaginaAccessoChiuso(t *testing.T) {
	f := NuovoFiltro(nil, false)
	f.Casa = func() string { return "https://192.168.1.202:7600" }
	h := f.PortaPubblica(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), "pc-master.tail1a2b.ts.net")
	chiedi := func(percorso, accept string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://127.0.0.1:7602"+percorso, nil)
		r.RemoteAddr = "127.0.0.1:5000"
		r.Header.Set("Accept", accept)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := chiedi("/", "text/html,application/xhtml+xml")
	if w.Code != 503 || !strings.Contains(w.Body.String(), "La porta di Gaster Bless su Internet ora è chiusa") ||
		!strings.Contains(w.Body.String(), `href="https://192.168.1.202:7600/"`) || strings.Contains(w.Body.String(), "<script") {
		t.Fatalf("pagina per il browser: %d %s", w.Code, w.Body.String())
	}
	if w := chiedi("/api/v1/io", "text/html"); w.Code != 403 || !strings.Contains(w.Body.String(), `"codice":"accesso_chiuso"`) {
		t.Fatalf("l'API riceve il JSON: %d", w.Code)
	}
	if w := chiedi("/", "application/json"); w.Code != 403 || !strings.Contains(w.Body.String(), "accesso_chiuso") {
		t.Fatalf("le app esterne ricevono il JSON: %d", w.Code)
	}
}
