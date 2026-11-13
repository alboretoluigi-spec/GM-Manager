package server

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func elenco(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, x := range m["lista"].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

const (
	pwMaster    = "l'antagonista-Borgofosco-1"
	pwGiocatore = "Elara-Voss"
)

// preparaPartita crea Master, due gruppi e un giocatore approvato nel solo gruppo A.
func preparaPartita(t *testing.T) (a *ambiente, master, gioc *cliente, gA, gB, idGioc string) {
	a = nuovoAmbiente(t)
	master = a.cliente("")
	master.deve(200, "POST", "/primo-avvio", map[string]any{"nome": "Luigi", "password": pwMaster})
	gA = master.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì sera", "giorno": "Martedì"})["id"].(string)
	gB = master.deve(200, "POST", "/gruppi", map[string]any{"nome": "Venerdì", "giorno": "Venerdì"})["id"].(string)

	gioc = a.cliente("")
	gioc.deve(200, "POST", "/registrazione", map[string]any{"nome": "Sara", "telefono": "+39 333 1234567",
		"password": pwGiocatore, "conferma": pwGiocatore, "consenso": true})
	ut := elenco(master.deve(200, "GET", "/utenti", nil))
	if len(ut) != 1 || ut[0]["stato"] != "in_attesa" {
		t.Fatalf("iscrizione non in attesa: %v", ut)
	}
	idGioc = ut[0]["id"].(string)
	return
}

func TestPrimoAvvioUnaVolta(t *testing.T) {
	a := nuovoAmbiente(t)
	c := a.cliente("")
	if m := c.deve(200, "GET", "/io", nil); m["primoAvvio"] != true {
		t.Fatalf("atteso primo avvio: %v", m)
	}
	c.deve(400, "POST", "/primo-avvio", map[string]any{"nome": "Luigi", "password": "corta"})
	c.deve(200, "POST", "/primo-avvio", map[string]any{"nome": "Luigi", "password": pwMaster})
	altro := a.cliente("")
	altro.deve(409, "POST", "/primo-avvio", map[string]any{"nome": "Intruso", "password": pwMaster})
}

func TestGiocatoreInAttesaNonVedeNulla(t *testing.T) {
	_, _, gioc, _, _, _ := preparaPartita(t)
	gioc.deve(403, "GET", "/gruppi", nil)
	if m := gioc.deve(200, "GET", "/io", nil); m["utente"].(map[string]any)["stato"] != "in_attesa" {
		t.Fatalf("stato atteso in_attesa: %v", m)
	}
}

func TestPermessiGruppiESessioni(t *testing.T) {
	_, master, gioc, gA, gB, idGioc := preparaPartita(t)
	master.deve(200, "POST", "/utenti/"+idGioc+"/approva", map[string]any{"gruppi": []string{gA}})

	inizio := time.Now().Add(48 * time.Hour).Unix()
	sA := master.deve(200, "POST", "/gruppi/"+gA+"/sessioni", map[string]any{"titolo": "La Tana del Lupo", "inizio": inizio})["id"].(string)
	sB := master.deve(200, "POST", "/gruppi/"+gB+"/sessioni", map[string]any{"titolo": "Rocca d'Argento", "inizio": inizio})["id"].(string)

	// Il giocatore vede solo il proprio gruppo.
	gr := elenco(gioc.deve(200, "GET", "/gruppi", nil))
	if len(gr) != 1 || gr[0]["id"] != gA {
		t.Fatalf("il giocatore dovrebbe vedere solo il gruppo A: %v", gr)
	}
	// Le sessioni dell'altro gruppo non esistono, per lui.
	gioc.deve(404, "GET", "/gruppi/"+gB+"/sessioni", nil)
	gioc.deve(404, "GET", "/sessioni/"+sB, nil)
	gioc.deve(404, "POST", "/sessioni/"+sB+"/conferma", map[string]any{"confermato": true})

	// Nella propria sessione è già partecipante e può confermare.
	v := gioc.deve(200, "GET", "/sessioni/"+sA, nil)
	if v["partecipo"] != true {
		t.Fatalf("il giocatore dovrebbe essere partecipante: %v", v)
	}
	gioc.deve(200, "POST", "/sessioni/"+sA+"/conferma", map[string]any{"confermato": true})
	p := master.deve(200, "GET", "/sessioni/"+sA, nil)["partecipanti"].([]any)
	if len(p) != 1 || p[0].(map[string]any)["confermato"] != true {
		t.Fatalf("conferma non registrata: %v", p)
	}

	// Azioni riservate al Master.
	gioc.deve(403, "POST", "/gruppi", map[string]any{"nome": "Pirati"})
	gioc.deve(403, "PATCH", "/sessioni/"+sA, map[string]any{"stato": "conclusa"})
	gioc.deve(403, "DELETE", "/sessioni/"+sA, nil)
	gioc.deve(403, "PUT", "/sessioni/"+sA+"/partecipanti", []any{})
	gioc.deve(403, "GET", "/utenti", nil)
	gioc.deve(403, "GET", "/impostazioni", nil)

	// Date assurde rifiutate anche in modifica.
	master.deve(400, "PATCH", "/sessioni/"+sA, map[string]any{"inizio": 0})

	// Eliminazione della sessione.
	master.deve(200, "DELETE", "/sessioni/"+sA, nil)
	master.deve(404, "DELETE", "/sessioni/"+sA, nil)
}

func TestProtezioneCSRFeOrigine(t *testing.T) {
	a, master, _, _, _, _ := preparaPartita(t)
	// Senza il token anti-CSRF.
	senza := *master
	senza.csrf = "sbagliato"
	senza.deve(403, "POST", "/gruppi", map[string]any{"nome": "X"})
	// Da un'altra origine (per esempio un sito malevolo aperto nello stesso browser).
	b, _ := json.Marshal(map[string]any{"nome": "X"})
	req, _ := http.NewRequest("POST", a.srv.URL+"/api/v1/gruppi", bytes.NewReader(b))
	req.Header.Set("Origin", "https://sito-malevolo.example")
	req.Header.Set("X-CSRF", master.csrf)
	r, err := master.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatalf("origine estranea: atteso 403, ricevuto %d", r.StatusCode)
	}
}

func TestQRLeggibile(t *testing.T) {
	a, master, gioc, _, _, _ := preparaPartita(t)
	get := func(c *cliente) *http.Response {
		r, err := c.c.Get(a.srv.URL + "/api/v1/qr.png?t=https%3A%2F%2F192.168.1.10%3A7700%2F%3Finvito%3Dabc")
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := get(gioc); r.StatusCode != 403 {
		t.Fatalf("il QR è solo per il Master: ricevuto %d", r.StatusCode)
	}
	r := get(master)
	defer r.Body.Close()
	img, err := png.Decode(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	lato := img.Bounds().Dx()
	if lato < 200 {
		t.Fatalf("QR troppo piccolo: %d px", lato)
	}
	// Margine bianco di 4 moduli, poi l'angolo nero del primo quadrato di posizionamento.
	nero := func(x, y int) bool { g, _, _, _ := img.At(x, y).RGBA(); return g < 0x8000 }
	if nero(2, 2) || !nero(4*8+2, 4*8+2) {
		t.Fatal("il QR non è disegnato correttamente (margine o ingrandimento sbagliati)")
	}
}

// Pagine e script si ricontrollano sempre: dopo un aggiornamento il browser non usa versioni vecchie.
func TestCacheDellePagine(t *testing.T) {
	a := nuovoAmbiente(t)
	for p, atteso := range map[string]string{"/": "no-cache", "/carta/locs.json": "no-cache", "/mappa": "no-cache"} {
		r, err := a.cliente("").c.Get(a.srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.Header.Get("Cache-Control") != atteso {
			t.Fatalf("%s: Cache-Control %q", p, r.Header.Get("Cache-Control"))
		}
	}
}

// Pagina di benvenuto: si può entrare subito (accettando l'avviso del browser) senza installare il
// certificato; l'invito passa alla pagina dell'app.
func TestPaginaBenvenuto(t *testing.T) {
	a := nuovoAmbiente(t)
	w := httptest.NewRecorder()
	a.s.benvenuto().ServeHTTP(w, httptest.NewRequest("GET", "http://192.168.1.10:7701/?invito=abc", nil))
	corpo := w.Body.String()
	if !strings.Contains(corpo, `href="https://192.168.1.10:7700/?invito=abc"`) || !strings.Contains(corpo, "Entra in Gaster Bless") || !strings.Contains(corpo, "Avanzate") {
		t.Fatalf("pagina di benvenuto:\n%s", corpo)
	} // iPhone: percorso guidato del certificato (obbligatorio su Safari) con controllo prima di entrare.
	r := httptest.NewRequest("GET", "http://192.168.1.10:7701/", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1")
	w = httptest.NewRecorder()
	a.s.benvenuto().ServeHTTP(w, r)
	corpo = w.Body.String()
	if !strings.Contains(corpo, `<body class="ios">`) || !strings.Contains(corpo, "Profilo scaricato") || !strings.Contains(corpo, `"https://192.168.1.10:7700/manifest.webmanifest"`) ||
		!strings.Contains(w.Header().Get("Content-Security-Policy"), "connect-src https://192.168.1.10:7700") {
		t.Fatalf("pagina di benvenuto per iPhone:\n%s", corpo)
	}
}

// Un giocatore approvato in un gruppo partecipa subito alle sessioni non concluse di quel gruppo.
func TestApprovatoPartecipaAlleSessioniAperte(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Persi nella nebbia"})["id"].(string)
	aperta := m.deve(200, "POST", "/gruppi/"+g+"/sessioni", map[string]any{"titolo": "Prossima", "inizio": 1893456000})["id"].(string)
	chiusa := m.deve(200, "POST", "/gruppi/"+g+"/sessioni", map[string]any{"titolo": "Vecchia", "inizio": 1893456000})["id"].(string)
	m.deve(200, "PATCH", "/sessioni/"+chiusa, map[string]any{"stato": "conclusa"})
	l := a.giocatore("Lock", "333 1112223", "192.168.1.30")
	id := a.idUtente(l)
	m.deve(200, "POST", "/utenti/"+id+"/approva", map[string]any{"gruppi": []string{g}})
	var n int
	a.s.DB.QueryRow(`SELECT COUNT(*) FROM partecipanti WHERE utente_id = ? AND sessione_id = ?`, id, aperta).Scan(&n)
	if n != 1 {
		t.Fatal("il giocatore approvato deve partecipare alla sessione aperta")
	}
	a.s.DB.QueryRow(`SELECT COUNT(*) FROM partecipanti WHERE utente_id = ? AND sessione_id = ?`, id, chiusa).Scan(&n)
	if n != 0 {
		t.Fatal("non deve essere aggiunto alle sessioni concluse")
	}
}
