package server

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// RPG RULEBOOK LIBRARY, passo L2: download dei soli documenti consentiti, con ripresa dopo un'interruzione, controllo
// del tipo, limite di dimensione, niente rinvii verso altri siti, niente pagine con accesso, doppioni riconosciuti,
// dati e file che restano dopo il riavvio.
func TestScaricaRegolamenti(t *testing.T) {
	pdf := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("contenuto di prova "), 20000)...) // circa 380 KB
	interrotto := 0
	editore := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.pdf", "/copia.pdf":
			http.ServeContent(w, r, "x.pdf", time.Time{}, bytes.NewReader(pdf))
		case "/interrotto.pdf":
			if r.Header.Get("Range") == "" { // prima richiesta: la connessione cade a metà
				interrotto++
				w.Header().Set("Content-Length", fmt.Sprint(len(pdf)))
				w.WriteHeader(200)
				w.Write(pdf[:len(pdf)/2])
				hj, _ := w.(http.Hijacker)
				c, _, _ := hj.Hijack()
				c.Close()
				return
			}
			http.ServeContent(w, r, "x.pdf", time.Time{}, bytes.NewReader(pdf))
		case "/pagina.pdf":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html><body>Accedi per scaricare</body></html>"))
		case "/privato.pdf":
			http.Error(w, "accesso richiesto", http.StatusForbidden)
		case "/rinvio.pdf":
			http.Redirect(w, r, "https://altro-sito.example/x.pdf", http.StatusFound)
		case "/grande.pdf":
			http.ServeContent(w, r, "x.pdf", time.Time{}, bytes.NewReader(append(pdf, pdf...)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer editore.Close()
	vecchi := []any{clientScaricoRegolamenti, attesaTraTentativi, maxScaricoRegolamenti}
	clientScaricoRegolamenti = func() *http.Client { c := *editore.Client(); return &c }
	attesaTraTentativi = 20 * time.Millisecond
	maxScaricoRegolamenti = 600 << 10
	defer func() {
		clientScaricoRegolamenti = vecchi[0].(func() *http.Client)
		attesaTraTentativi = vecchi[1].(time.Duration)
		maxScaricoRegolamenti = vecchi[2].(int64)
	}()

	a := nuovoAmbiente(t)
	m := a.master()
	voce := func(id, percorso string, diretto bool) string {
		return fmt.Sprintf(`{"id":%q,"titolo":"Documento %s","edizione":"1","tipologia":"quickstart","lingua":"it","formato":"pdf","url":%q,"licenza":"Condizioni dell'editore","condizioni":"prova","gratuito":true,"downloadDiretto":%v,"verificatoIl":"2026-11-13"}`,
			id, id, editore.URL+percorso, diretto)
	}
	docs := []string{voce("ok", "/ok.pdf", true), voce("copia", "/copia.pdf", true), voce("interrotto", "/interrotto.pdf", true),
		voce("pagina", "/pagina.pdf", true), voce("privato", "/privato.pdf", true), voce("rinvio", "/rinvio.pdf", true),
		voce("grande", "/grande.pdf", true), voce("solo-link", "/ok.pdf", false)}
	cat := `{"versione":1,"aggiornatoIl":"2026-11-13","avviso":"a","tipologie":{"quickstart":"Quickstart"},"giochi":[{"id":"g","nome":"Gioco di prova","editore":"Editore di prova","generi":["fantasy"],"edizioni":["1"],"sito":"` + editore.URL + `/","sitoVerificatoIl":"2026-11-13","documenti":[` + strings.Join(docs, ",") + `]}]}`
	dir := filepath.Join(a.s.Cfg.CartellaDati, "regolamenti")
	os.MkdirAll(dir, 0o700)
	if err := os.WriteFile(filepath.Join(dir, "catalogo.json"), []byte(cat), 0o600); err != nil {
		t.Fatal(err)
	}
	attendi := func(doc string) map[string]any {
		t.Helper()
		for i := 0; i < 200; i++ {
			st, _ := m.deve(200, "GET", "/regolamenti/scaricamenti", nil)[doc].(map[string]any)
			if st != nil && st["stato"] != "in corso" {
				return st
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("%s: download senza fine", doc)
		return nil
	}
	scarica := func(doc string) map[string]any {
		t.Helper()
		m.deve(200, "POST", "/regolamenti/scarica", map[string]any{"doc": doc})
		return attendi(doc)
	}

	// riuscito
	st := scarica("ok")
	if st["stato"] != "fatto" || st["documento"] == "" || st["ricevuti"] != float64(len(pdf)) {
		t.Fatalf("download riuscito: %v", st)
	}
	miei := m.deve(200, "GET", "/regolamenti/miei", nil)["documenti"].([]any)
	if len(miei) != 1 {
		t.Fatalf("libreria dopo il download: %v", miei)
	}
	d := miei[0].(map[string]any)
	if d["origine"] != "download" || d["catalogoId"] != "ok" || d["gioco"] != "Gioco di prova" || d["licenza"] == "" || d["fonteUrl"] != editore.URL+"/ok.pdf" {
		t.Fatalf("dati del documento scaricato: %v", d)
	}
	cod, h, corpo := m.grezzo("GET", "/regolamenti/miei/"+d["id"].(string)+"/file", nil, "")
	if cod != 200 || !bytes.Equal(corpo, pdf) || h.Get("Content-Type") != "application/pdf" {
		t.Fatalf("file del documento: %d %s %d byte", cod, h.Get("Content-Type"), len(corpo))
	}
	if n := m.deve(200, "GET", "/regolamenti/catalogo", nil)["nellaLibreria"].(map[string]any); n["ok"] != d["id"] {
		t.Fatalf("catalogo: documento nella libreria: %v", n)
	}
	// di nuovo: già presente, nessun nuovo download
	if r := m.deve(200, "POST", "/regolamenti/scarica", map[string]any{"doc": "ok"}); r["stato"] != "presente" {
		t.Fatalf("secondo download: %v", r)
	}
	// stesso contenuto da un'altra voce: doppione riconosciuto
	if st := scarica("copia"); st["stato"] != "presente" || st["documento"] != d["id"] {
		t.Fatalf("doppione: %v", st)
	}
	// interrotto a metà: ripresa con Range
	if st := scarica("interrotto"); st["stato"] != "presente" || interrotto != 1 {
		t.Fatalf("ripresa dopo l'interruzione: %v (interruzioni %d)", st, interrotto)
	}
	// errori
	casi := map[string]string{"pagina": "non è un PDF valido", "privato": "richiede un accesso", "rinvio": "altro sito", "grande": "troppo grande"}
	for doc, atteso := range casi {
		if st := scarica(doc); st["stato"] != "errore" || !strings.Contains(st["errore"].(string), atteso) {
			t.Fatalf("%s: %v", doc, st)
		}
		if _, err := os.Stat(filepath.Join(dir, "parziali", doc+".part")); err == nil {
			t.Fatalf("%s: file parziale rimasto", doc)
		}
	}
	m.deve(400, "POST", "/regolamenti/scarica", map[string]any{"doc": "solo-link"})
	m.deve(404, "POST", "/regolamenti/scarica", map[string]any{"doc": "inesistente"})
	a.giocatore("Luca", "333 1234567", "192.168.1.20").deve(403, "POST", "/regolamenti/scarica", map[string]any{"doc": "ok"})
	if n := len(m.deve(200, "GET", "/regolamenti/miei", nil)["documenti"].([]any)); n != 1 {
		t.Fatalf("documenti dopo gli errori: %d", n)
	}

	// riavvio: dati e file restano
	cfg := a.s.Cfg
	a.s.Chiudi()
	s2, err := Nuovo(cfg, fstest.MapFS{"index.html": {Data: []byte("x")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Chiudi()
	var n int
	var file string
	s2.DB.QueryRow(`SELECT COUNT(*), MAX(file) FROM regolamenti_documenti`).Scan(&n, &file)
	b, err := os.ReadFile(filepath.Join(dir, "file", file))
	if n != 1 || err != nil || !bytes.Equal(b, pdf) {
		t.Fatalf("dopo il riavvio: %d documenti, file %v", n, err)
	}
}
