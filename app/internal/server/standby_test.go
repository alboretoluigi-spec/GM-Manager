package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ascoltoCheCade: come dopo lo standby di Windows, Accept restituisce un errore definitivo.
type ascoltoCheCade struct{ net.Listener }

func (a ascoltoCheCade) Accept() (net.Conn, error) {
	return nil, errors.New("rete interrotta (standby)")
}

// Standby del PC (2026-11-08): se la porta in ascolto si interrompe, il server la riapre da solo invece di fermarsi.
func TestPortaRiapertaDopoStandby(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	ctx, ferma := context.WithCancel(context.Background())
	var riaperture atomic.Int32
	finito := make(chan struct{})
	var ultimo atomic.Pointer[http.Server]
	go func() {
		serviConRipresa(ctx, "prova", ascoltoCheCade{l}, func() *http.Server {
			srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })}
			ultimo.Store(srv)
			return srv
		}, func() (net.Listener, error) { riaperture.Add(1); return net.Listen("tcp", addr) }, false, nil, 10*time.Millisecond)
		close(finito)
	}()
	var corpo []byte
	for i := 0; i < 100; i++ {
		if r, err := http.Get("http://" + addr + "/"); err == nil {
			corpo, _ = io.ReadAll(r.Body)
			r.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if string(corpo) != "ok" || riaperture.Load() < 1 {
		t.Fatalf("dopo l'interruzione la porta non è stata riaperta: %q, riaperture %d", corpo, riaperture.Load())
	}
	ferma()
	ultimo.Load().Close()
	select {
	case <-finito:
	case <-time.After(2 * time.Second):
		t.Fatal("alla chiusura il servizio non si ferma")
	}
}

// File dell'app: impronta (304 se non cambiato) e compressione gzip per script e dati.
func TestStaticheVeloci(t *testing.T) {
	a := nuovoAmbiente(t)
	a.s.Web = os.DirFS("../../web")
	cl := &http.Client{Transport: &http.Transport{DisableCompression: true, TLSClientConfig: a.srv.Client().Transport.(*http.Transport).TLSClientConfig}}
	chiedi := func(p string, h map[string]string) *http.Response {
		req, _ := http.NewRequest("GET", a.srv.URL+p, nil)
		for k, v := range h {
			req.Header.Set(k, v)
		}
		r, err := cl.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
		return r
	}
	r := chiedi("/ui/app.js", map[string]string{"Accept-Encoding": "gzip"})
	if r.StatusCode != 200 || r.Header.Get("Content-Encoding") != "gzip" || r.Header.Get("ETag") == "" {
		t.Fatalf("app.js: %d %v", r.StatusCode, r.Header)
	}
	if r2 := chiedi("/ui/app.js", map[string]string{"Accept-Encoding": "gzip", "If-None-Match": r.Header.Get("ETag")}); r2.StatusCode != 304 {
		t.Fatalf("app.js non cambiato: %d", r2.StatusCode)
	}
	if r3 := chiedi("/ui/app.js", nil); r3.StatusCode != 200 || r3.Header.Get("Content-Encoding") != "" {
		t.Fatalf("senza gzip: %d %v", r3.StatusCode, r3.Header)
	}
	if r4 := chiedi("/mappa", map[string]string{"Accept-Encoding": "gzip"}); r4.StatusCode != 200 || !strings.HasPrefix(r4.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("pagina principale: %d %v", r4.StatusCode, r4.Header)
	}
	im := chiedi("/img/povero.webp", nil)
	if im.StatusCode != 200 || im.Header.Get("ETag") == "" || chiedi("/img/povero.webp", map[string]string{"If-None-Match": im.Header.Get("ETag")}).StatusCode != 304 {
		t.Fatalf("immagine: %v", im.Header)
	}
}
