package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAvvisiWhatsApp(t *testing.T) {
	sc := nuovoScontro(t) // Luca e Sara partecipano (Luca con telefono +39 333 1234567)
	sc.mappa(sc.luca, 403, "GET", "../modelli-avviso", nil)
	mod := sc.m.deve(200, "GET", "/modelli-avviso", nil)["lista"].([]any)
	testoMod := mod[0].(map[string]any)["testo"].(string)
	sc.m.deve(200, "PATCH", "/sessioni/"+sc.ses, map[string]any{"luogo": "Casa del Master", "note": ""})
	ant := sc.m.deve(200, "POST", "/sessioni/"+sc.ses+"/avvisi/anteprima", map[string]any{"testo": testoMod})
	dd := ant["destinatari"].([]any)
	if len(dd) != 2 {
		t.Fatalf("destinatari: %v", dd)
	}
	l := dd[0].(map[string]any)
	if l["link"] != nil || !strings.Contains(l["testo"].(string), "Ciao Luca! «La Tana del Lupo» sarà ") || strings.Contains(l["telefono"].(string), "1234567") {
		t.Fatalf("l'anteprima non deve contenere numeri in chiaro né link: %v", l)
	}
	// Preparazione dell'invio: link wa.me con il numero e il testo; resta nel registro.
	cr := sc.m.deve(200, "POST", "/sessioni/"+sc.ses+"/avvisi", map[string]any{"testo": "Ciao {nome}, ci vediamo {data}!"})
	link := cr["destinatari"].([]any)[0].(map[string]any)["link"].(string)
	if !strings.HasPrefix(link, "https://wa.me/393331234567?text=Ciao+Luca") {
		t.Fatalf("link WhatsApp: %s", link)
	}
	id := cr["id"].(string)
	sc.m.deve(200, "PATCH", "/avvisi/"+id+"/destinatari/"+sc.idLuca, map[string]any{"stato": "inviato"})
	st := sc.m.deve(200, "GET", "/sessioni/"+sc.ses+"/avvisi", nil)["lista"].([]any)[0].(map[string]any)["destinatari"].([]any)
	for _, x := range st {
		d := x.(map[string]any)
		if d["nome"] == "Luca" && (d["stato"] != "inviato" || d["link"] != nil) {
			t.Fatalf("stato dell'avviso di Luca: %v", d)
		}
	}
	sc.luca.deve(403, "POST", "/sessioni/"+sc.ses+"/avvisi", map[string]any{"testo": "x"})
	if r := sc.m.deve(200, "GET", "/registro-dati", nil)["lista"].([]any); len(r) == 0 {
		t.Fatal("l'uso del numero per l'avviso va registrato")
	}
	// Calendario: per i membri del gruppo, non per gli altri.
	get := func(c *cliente) (int, string) {
		req, _ := http.NewRequest("GET", sc.a.srv.URL+"/api/v1/sessioni/"+sc.ses+"/calendario.ics", nil)
		if c.ip != "" {
			req.Header.Set("X-Test-IP", c.ip)
		}
		r, err := c.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		return r.StatusCode, string(b)
	}
	if c, b := get(sc.luca); c != 200 || !strings.Contains(b, "SUMMARY:La Tana del Lupo") || !strings.Contains(b, "LOCATION:Casa del Master") {
		t.Fatalf("calendario: %d %s", c, b)
	}
	if c, _ := get(sc.ugo); c != 404 {
		t.Fatalf("un altro gruppo non scarica il calendario: %d", c)
	}
}
