package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

// Cloudflare Tunnel (2026-11-08): indirizzo del dispositivo da Cf-Connecting-Ip, chi arriva dall'indirizzo Internet di
// casa è «al tavolo» (niente audio dei video sul telefono), niente avviso del certificato per iPhone, e i file
// dell'app non sono mai «public» (Cloudflare non deve tenerne copie da dare ad altri).
func TestCloudflareTunnel(t *testing.T) {
	// L'indirizzo di casa si chiede a Cloudflare (qui una finta pagina «trace»).
	traccia := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "fl=1\nh=1.1.1.1\nip=198.51.100.20\nts=1\n")
	}))
	defer traccia.Close()
	vecchio := indirizzoTraccia
	indirizzoTraccia = traccia.URL
	defer func() { indirizzoTraccia = vecchio }()
	if a, err := cercaIPCasa(context.Background()); err != nil || a != netip.MustParseAddr("198.51.100.20") {
		t.Fatalf("indirizzo di casa: %v %v", a, err)
	}

	a := nuovoAmbiente(t)
	s := a.s
	s.Cfg.PortaPubblica, s.Cfg.NomePubblico, s.Cfg.ServizioPubblico = 7602, "gasterbless.esempio.it", "cloudflare"
	s.Filtro.IntestazioneIP = "Cf-Connecting-Ip"
	s.ipCasa.imposta(netip.MustParseAddr("198.51.100.20"))
	s.Filtro.ImpostaPubblico(true)
	pub := httptest.NewServer(s.Filtro.PortaPubblica(s.Handler(), s.Cfg.NomePubblico))
	defer pub.Close()
	m := a.master()
	g := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	gr := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	m.deve(200, "POST", "/utenti/"+a.idUtente(g)+"/approva", map[string]any{"gruppi": []string{gr}})

	entra := func(ipDispositivo string) map[string]any {
		jar, _ := cookiejar.New(nil)
		hc := &http.Client{Jar: jar}
		chiama := func(metodo, percorso string, corpo any) map[string]any {
			var b io.Reader
			if corpo != nil {
				j, _ := json.Marshal(corpo)
				b = bytes.NewReader(j)
			}
			req, _ := http.NewRequest(metodo, pub.URL+"/api/v1"+percorso, b)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "https://gasterbless.esempio.it")
			req.Header.Set("Cf-Connecting-Ip", ipDispositivo)
			req.Header.Set("X-Forwarded-For", "203.0.113.99") // non conta: con Cloudflare vale Cf-Connecting-Ip
			r, err := hc.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Body.Close()
			var out map[string]any
			json.NewDecoder(r.Body).Decode(&out)
			if r.StatusCode != 200 {
				t.Fatalf("%s %s: %d %v", metodo, percorso, r.StatusCode, out)
			}
			return out
		}
		chiama("POST", "/accesso", map[string]any{"telefono": "333 1234567", "password": "Nebbia sul lago Varnhold"})
		return chiama("GET", "/io", nil)
	}
	if io := entra("198.51.100.20"); io["daFuori"] != false || io["pubblica"] != true || io["locale"] == true {
		t.Fatalf("al tavolo, dall'indirizzo di casa: %v", io)
	}
	if io := entra("203.0.113.7"); io["daFuori"] != true {
		t.Fatalf("da casa sua il giocatore è lontano: %v", io)
	}
	// Nessun file dell'app con Cache-Control «public» (Cloudflare lo conserverebbe e lo darebbe a chiunque).
	req, _ := http.NewRequest("GET", pub.URL+"/img/povero.webp", nil)
	req.Header.Set("Cf-Connecting-Ip", "203.0.113.7")
	if r, err := http.DefaultClient.Do(req); err == nil {
		r.Body.Close()
		if cc := r.Header.Get("Cache-Control"); cc != "" && !strings.Contains(cc, "private") && !strings.Contains(cc, "no-") {
			t.Fatalf("Cache-Control: %q", cc)
		}
	}
}

// Tailscale Funnel usato anche in casa (2026-11-08): dall'indirizzo Internet di casa si è al tavolo; la pagina di
// benvenuto aperta da iPhone propone l'indirizzo .ts.net (nessun certificato da installare) quando è aperto.
func TestTailscaleAncheInCasa(t *testing.T) {
	a := nuovoAmbiente(t)
	s := a.s
	s.Cfg.PortaPubblica, s.Cfg.NomePubblico = 7602, "pc-master.tail1a2b.ts.net"
	s.ipCasa.imposta(netip.MustParseAddr("198.51.100.20"))
	pub := httptest.NewServer(s.Filtro.PortaPubblica(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.daFuori(r) {
			io.WriteString(w, "fuori")
		} else {
			io.WriteString(w, "casa")
		}
	}), s.Cfg.NomePubblico))
	defer pub.Close()
	s.Filtro.ImpostaPubblico(true)
	prova := func(xff string) string {
		req, _ := http.NewRequest("GET", pub.URL+"/", nil)
		req.Header.Set("X-Forwarded-For", xff)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return string(b)
	}
	if v := prova("198.51.100.20"); v != "casa" {
		t.Fatalf("dal Wi-Fi di casa con l'indirizzo .ts.net: %s", v)
	}
	if v := prova("203.0.113.7"); v != "fuori" {
		t.Fatalf("da casa sua: %s", v)
	}
	// Pagina di benvenuto da iPhone: alternativa senza certificato.
	r := httptest.NewRequest("GET", "http://192.168.1.10:7701/?invito=abc", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X)")
	w := httptest.NewRecorder()
	s.benvenuto().ServeHTTP(w, r)
	if c := w.Body.String(); !strings.Contains(c, `href="https://pc-master.tail1a2b.ts.net/?invito=abc"`) {
		t.Fatalf("alternativa .ts.net nella pagina di benvenuto:\n%s", c)
	}
	s.Filtro.ImpostaPubblico(false)
	w = httptest.NewRecorder()
	s.benvenuto().ServeHTTP(w, r)
	if strings.Contains(w.Body.String(), "ts.net") {
		t.Fatal("con l'accesso da Internet chiuso l'alternativa non compare")
	}
}
