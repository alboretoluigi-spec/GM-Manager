package server

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"gasterbless/internal/config"
	"gasterbless/web"
)

// Con nomi_certificato il server presenta un certificato valido anche per il nome Tailscale del PC,
// e il filtro accetta la rete Tailscale solo con l'accesso esterno aperto (con accesso_esterno = vpn parte
// aperto, salvo che il Master l'abbia chiuso dal pannello), mai il resto di Internet.
func TestNomeTailscaleEFiltro(t *testing.T) {
	cfg := config.Predefinita()
	cfg.CartellaDati, cfg.CartellaMedia, cfg.ModalitaProva = t.TempDir(), t.TempDir(), true
	cfg.AccessoEsterno, cfg.NomiCertificato = "vpn", []string{"pc-master.tail1a2b.ts.net"}
	s, err := Nuovo(cfg, web.File)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Chiudi()
	c, err := s.Cert.ConfigTLS().GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := x509.ParseCertificate(c.Certificate[0])
	if _, err := f.Verify(x509.VerifyOptions{DNSName: "pc-master.tail1a2b.ts.net", Roots: s.Cert.PoolAutorita()}); err != nil {
		t.Fatalf("certificato per il nome Tailscale: %v", err)
	}
	remoto := netip.MustParseAddr("100.101.102.103")
	if !s.Filtro.Ammesso(remoto) || s.Filtro.Ammesso(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("aperto: solo la rete della VPN, mai Internet")
	}
	s.Filtro.ImpostaEsterno(false) // "Chiudi subito l'accesso esterno"
	if s.Filtro.Ammesso(remoto) || !s.Filtro.Ammesso(netip.MustParseAddr("192.168.1.20")) {
		t.Fatal("chiuso: fuori la VPN, dentro il Wi-Fi di casa")
	}
}

// Da Internet (porta pubblica di Tailscale Funnel): niente privilegi del PC, invito obbligatorio,
// secondo fattore obbligatorio per il Master, tutto chiuso con "Chiudi subito".
func TestAccessoDaInternet(t *testing.T) {
	a := nuovoAmbiente(t)
	s := a.s
	s.Cfg.PortaPubblica, s.Cfg.NomePubblico = 7602, "pc-master.tail1a2b.ts.net"
	pub := httptest.NewServer(s.Filtro.PortaPubblica(s.Handler(), s.Cfg.NomePubblico))
	defer pub.Close()
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Jar: jar}
	chiama := func(metodo, percorso string, corpo any) (int, map[string]any) {
		var b io.Reader
		if corpo != nil {
			j, _ := json.Marshal(corpo)
			b = bytes.NewReader(j)
		}
		req, _ := http.NewRequest(metodo, pub.URL+"/api/v1"+percorso, b)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://pc-master.tail1a2b.ts.net")
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		resp, err := hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	if c, out := chiama("GET", "/io", nil); c != 403 || out["codice"] != "accesso_chiuso" {
		t.Fatalf("chiuso finché il Master non lo apre, con un errore JSON riconoscibile: %d %v", c, out)
	}
	s.Filtro.ImpostaPubblico(true)
	if c, io := chiama("GET", "/io", nil); c != 200 || io["primoAvvio"] == true {
		t.Fatalf("da Internet non si crea l'account del Master: %d %v", c, io)
	}
	if c, _ := chiama("POST", "/primo-avvio", map[string]any{"nome": "Intruso", "password": "Il-castello-di-Corvonero"}); c != 403 {
		t.Fatal("primo avvio solo dal PC")
	}
	m := a.master() // dal PC
	if c, _ := chiama("POST", "/registrazione", map[string]any{"nome": "Ospite", "telefono": "333 7654321", "password": "Nebbia sul lago Varnhold",
		"conferma": "Nebbia sul lago Varnhold", "consenso": true}); c != 403 {
		t.Fatal("da Internet ci si registra solo con l'invito")
	}
	if c, out := chiama("POST", "/accesso", map[string]any{"nome": "Master", "password": "Il-castello-di-Corvonero"}); c != 403 {
		t.Fatalf("il Master senza secondo fattore non entra da Internet: %d %v", c, out)
	}
	// Un giocatore approvato entra da Internet; "Chiudi subito" lo butta fuori.
	g := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	gr := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	m.deve(200, "POST", "/utenti/"+a.idUtente(g)+"/approva", map[string]any{"gruppi": []string{gr}})
	if c, out := chiama("POST", "/accesso", map[string]any{"telefono": "333 1234567", "password": "Nebbia sul lago Varnhold"}); c != 200 {
		t.Fatalf("giocatore da Internet: %d %v", c, out)
	}
	if c, io := chiama("GET", "/io", nil); c != 200 || io["collegato"] != true || io["locale"] == true {
		t.Fatalf("collegato, ma mai come il PC: %v", io)
	}
	if _, io := chiama("GET", "/io", nil); io["daFuori"] != true {
		t.Fatalf("da Internet il giocatore risulta lontano (audio dei video proiettati al massimo): %v", io)
	}
	if io := g.deve(200, "GET", "/io", nil); io["daFuori"] != false {
		t.Fatalf("dal Wi-Fi di casa il giocatore è al tavolo: %v", io)
	}
	if c, _ := chiama("GET", "/impostazioni", nil); c != 403 {
		t.Fatal("il giocatore non vede le impostazioni")
	}
	// Tempo reale attraverso la porta pubblica (l'origine è il nome .ts.net).
	ctx, annulla := context.WithTimeout(context.Background(), 5*time.Second)
	defer annulla()
	ws, _, err := websocket.Dial(ctx, strings.Replace(pub.URL, "http://", "ws://", 1)+"/ws", &websocket.DialOptions{HTTPClient: hc,
		HTTPHeader: http.Header{"Origin": {"https://pc-master.tail1a2b.ts.net"}, "X-Forwarded-For": {"203.0.113.7"}}})
	if err != nil {
		t.Fatalf("tempo reale da Internet: %v", err)
	}
	if _, msg, err := ws.Read(ctx); err != nil || !strings.Contains(string(msg), "benvenuto") {
		t.Fatalf("primo messaggio: %v %s", err, msg)
	}
	if _, _, err := websocket.Dial(ctx, strings.Replace(pub.URL, "http://", "ws://", 1)+"/ws", &websocket.DialOptions{HTTPClient: hc,
		HTTPHeader: http.Header{"Origin": {"https://sito-cattivo.example"}}}); err == nil {
		t.Fatal("tempo reale da un'altra origine rifiutato")
	}
	m.deve(400, "POST", "/accesso-pubblico/apri", map[string]any{}) // dal PC ma senza secondo fattore
	m.deve(200, "POST", "/accesso-esterno/chiudi", map[string]any{})
	ctx2, annulla2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer annulla2()
	for { // eventuali messaggi già in viaggio, poi la chiusura (non la scadenza dell'attesa)
		if _, _, err := ws.Read(ctx2); err != nil {
			if ctx2.Err() != nil {
				t.Fatal("chiudendo, la connessione in tempo reale da Internet si interrompe")
			}
			break
		}
	}
	if s.Filtro.Pubblico() {
		t.Fatal("Chiudi subito chiude anche Internet")
	}
	if c, _ := chiama("GET", "/io", nil); c != 403 {
		t.Fatal("chiuso: nessuna richiesta da Internet")
	}
	s.Filtro.ImpostaPubblico(true)
	if _, io := chiama("GET", "/io", nil); io["collegato"] == true {
		t.Fatal("le sessioni aperte da Internet sono state cancellate")
	}
	if v, _ := s.impostazione("accesso_pubblico"); v != "chiuso" {
		t.Fatal("la chiusura resta dopo un riavvio")
	}
}
