package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// «PNG in scena» (2026-11-12): il Master mette in scena un PNG con immagine; i giocatori del gruppo lo ricevono subito,
// possono scaricarne l'immagine (prima no), lo ritrovano aprendo Gaster Bless; togliendolo, l'immagine torna privata.
func TestPNGInScena(t *testing.T) {
	sc := nuovoScontro(t)
	g := sc.m.deve(200, "GET", "/sessioni/"+sc.ses, nil)["gruppo"].(string)
	id := sc.m.deve(200, "POST", "/bestiario/voci", map[string]any{"nome": "Madre Maren", "cat": "png"})["id"].(string)
	senza := sc.m.deve(200, "POST", "/bestiario/voci", map[string]any{"nome": "Senza volto", "cat": "png"})["id"].(string)
	req, _ := http.NewRequest("PUT", sc.a.srv.URL+"/api/v1/bestiario/voci/"+id+"/immagine", bytes.NewReader([]byte("\x89PNG\r\n\x1a\n0000000000")))
	req.Header.Set("X-CSRF", sc.m.csrf)
	if r, err := sc.m.c.Do(req); err != nil || r.StatusCode != 200 {
		t.Fatalf("immagine: %v", err)
	} else {
		r.Body.Close()
	}
	img := sc.m.deve(200, "GET", "/bestiario", nil)["entries"].(map[string]any)[id].(map[string]any)["img"].(string)
	media := func(c *cliente) int {
		r, err := c.c.Get(sc.a.srv.URL + "/media/" + img)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if media(sc.luca) != 404 {
		t.Fatal("prima della scena l'immagine è privata")
	}
	cl := sc.clienteWS(sc.luca)
	cl.canali["gruppo:"+g] = true
	ultimiMessaggi(cl, "")
	pj := sc.clienteWS(sc.m) // la finestra di proiezione: canale della mappa del gruppo
	pj.canali["mappa:"+g] = true
	ultimiMessaggi(pj, "")
	sc.luca.deve(403, "POST", "/scena", map[string]any{"gruppo": g, "voce": id})
	sc.m.deve(400, "POST", "/scena", map[string]any{"gruppo": g, "voce": senza})
	sc.m.deve(200, "POST", "/scena", map[string]any{"gruppo": g, "voce": id, "nome": "Madre Maren", "lato": "sinistra"})
	ml := ultimiMessaggi(cl, "scena.png")
	if len(ml) != 1 || !strings.Contains(ml[0], `"attiva":true`) || !strings.Contains(ml[0], `"lato":"sinistra"`) || !strings.Contains(ml[0], "/media/"+img) {
		t.Fatalf("messaggio della scena: %v", ml)
	}
	if ml := strings.Join(ultimiMessaggi(pj, "mappa"), "\n"); !strings.Contains(ml, `"t":"scena"`) || !strings.Contains(ml, "Madre Maren") {
		t.Fatalf("la scena arriva anche al proiettore: %v", ml)
	}
	if media(sc.luca) != 200 {
		t.Fatal("in scena: il giocatore del gruppo vede l'immagine")
	}
	if media(sc.ugo) != 404 {
		t.Fatal("un giocatore di un altro gruppo non la vede")
	}
	scena := func(c *cliente) string { _, _, b := c.grezzo("GET", "/scena", nil, ""); return string(b) }
	if l := scena(sc.luca); !strings.Contains(l, "Madre Maren") || strings.Contains(scena(sc.ugo), "Madre Maren") {
		t.Fatalf("PNG in scena per chi apre Gaster Bless: %s", l)
	}
	sc.m.deve(200, "DELETE", "/scena?gruppo="+g, nil)
	if ml := ultimiMessaggi(cl, "scena.png"); len(ml) != 1 || !strings.Contains(ml[0], `"attiva":false`) {
		t.Fatalf("tolto dalla scena: %v", ml)
	}
	if ml := strings.Join(ultimiMessaggi(pj, "mappa"), "\n"); !strings.Contains(ml, `"t":"scena"`) || !strings.Contains(ml, `"attiva":false`) {
		t.Fatalf("tolto anche dal proiettore: %v", ml)
	}
	if media(sc.luca) != 404 {
		t.Fatal("tolto dalla scena: l'immagine torna privata")
	}
}

// Più PNG in scena insieme (2026-11-13): con «aggiungi» si sommano (al massimo 6), lo stesso PNG cambia solo lato, senza
// «aggiungi» uno prende il posto di tutti; DELETE con la voce ne toglie uno solo e la sua immagine torna privata.
func TestPiuPNGInScena(t *testing.T) {
	sc := nuovoScontro(t)
	g := sc.m.deve(200, "GET", "/sessioni/"+sc.ses, nil)["gruppo"].(string)
	nuovo := func(nome string) (string, string) {
		id := sc.m.deve(200, "POST", "/bestiario/voci", map[string]any{"nome": nome, "cat": "png"})["id"].(string)
		req, _ := http.NewRequest("PUT", sc.a.srv.URL+"/api/v1/bestiario/voci/"+id+"/immagine", bytes.NewReader([]byte("\x89PNG\r\n\x1a\n0000000000"+nome)))
		req.Header.Set("X-CSRF", sc.m.csrf)
		r, err := sc.m.c.Do(req)
		if err != nil || r.StatusCode != 200 {
			t.Fatalf("immagine di %s: %v", nome, err)
		}
		r.Body.Close()
		return id, sc.m.deve(200, "GET", "/bestiario", nil)["entries"].(map[string]any)[id].(map[string]any)["img"].(string)
	}
	media := func(img string) int {
		r, err := sc.luca.c.Get(sc.a.srv.URL + "/media/" + img)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	a, imgA := nuovo("Aldric")
	b, imgB := nuovo("Elara")
	cl := sc.clienteWS(sc.luca)
	cl.canali["gruppo:"+g] = true
	ultimiMessaggi(cl, "")
	sc.m.deve(200, "POST", "/scena", map[string]any{"gruppo": g, "voce": a, "nome": "Aldric", "lato": "sinistra", "aggiungi": true})
	sc.m.deve(200, "POST", "/scena", map[string]any{"gruppo": g, "voce": b, "nome": "Elara", "lato": "centro", "aggiungi": true})
	ml := ultimiMessaggi(cl, "scena.png")
	if len(ml) != 2 || !strings.Contains(ml[1], "Aldric") || !strings.Contains(ml[1], "Elara") || !strings.Contains(ml[1], `"lato":"centro"`) {
		t.Fatalf("due PNG in scena: %v", ml)
	}
	if media(imgA) != 200 || media(imgB) != 200 {
		t.Fatal("i giocatori vedono le due immagini")
	}
	scena := func() []any {
		_, _, body := sc.luca.grezzo("GET", "/scena", nil, "")
		var l []any
		json.Unmarshal(body, &l)
		return l
	}
	if l := scena(); len(l) != 2 || l[0].(map[string]any)["voce"] != a || l[1].(map[string]any)["voce"] != b {
		t.Fatalf("elenco nell'ordine in cui sono stati messi: %v", l)
	}
	// lo stesso PNG di nuovo: cambia lato, resta al suo posto
	sc.m.deve(200, "POST", "/scena", map[string]any{"gruppo": g, "voce": a, "nome": "Aldric", "lato": "destra", "aggiungi": true})
	if l := scena(); len(l) != 2 || l[0].(map[string]any)["lato"] != "destra" {
		t.Fatalf("spostato a destra: %v", l)
	}
	// uno solo tolto
	sc.m.deve(200, "DELETE", "/scena?gruppo="+g+"&voce="+a, nil)
	if l := scena(); len(l) != 1 || l[0].(map[string]any)["voce"] != b || media(imgA) != 404 || media(imgB) != 200 {
		t.Fatalf("tolto solo Aldric: %v", l)
	}
	// al massimo 6 insieme
	for i := 0; i < 5; i++ {
		id, _ := nuovo(fmt.Sprintf("Comparsa %d", i))
		sc.m.deve(200, "POST", "/scena", map[string]any{"gruppo": g, "voce": id, "aggiungi": true})
	}
	sc.m.deve(409, "POST", "/scena", map[string]any{"gruppo": g, "voce": a, "aggiungi": true})
	// senza «aggiungi»: prende il posto di tutti
	sc.m.deve(200, "POST", "/scena", map[string]any{"gruppo": g, "voce": a, "lato": "sinistra"})
	if l := scena(); len(l) != 1 || l[0].(map[string]any)["voce"] != a || media(imgB) != 404 {
		t.Fatalf("una alla volta: %v", l)
	}
}
