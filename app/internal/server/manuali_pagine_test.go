package server

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// grezzo: richiesta con corpo binario; restituisce codice, intestazioni e corpo.
func (c *cliente) grezzo(metodo, percorso string, corpo []byte, tipo string) (int, http.Header, []byte) {
	c.a.t.Helper()
	req, _ := http.NewRequest(metodo, c.a.srv.URL+"/api/v1"+percorso, bytes.NewReader(corpo))
	if tipo != "" {
		req.Header.Set("Content-Type", tipo)
	}
	req.Header.Set("X-CSRF", c.csrf)
	if c.ip != "" {
		req.Header.Set("X-Test-IP", c.ip)
	}
	resp, err := c.c.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b
}

func webpFinto(n int) []byte {
	return append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{byte(n)}, 50)...)
}

// Pagine pronte dei manuali (M2): solo il Master le prepara, i giocatori le leggono solo se il manuale è condiviso con
// loro, restano nella memoria del telefono, si azzerano se il PDF cambia e spariscono con il manuale.
func TestPagineManuali(t *testing.T) {
	tv := nuovoTavolo(t)
	_, out := tv.m.caricaManuale(pdfDiProva(3), "Guida del Master")
	id := out["id"].(string)
	base := "/manuali/" + id
	q := "?totale=3&larghezza=1400"

	if code, _, _ := tv.luca.grezzo("PUT", base+"/pagine/1"+q, webpFinto(1), "image/webp"); code != 403 {
		t.Fatalf("un giocatore non prepara pagine: %d", code)
	}
	if code, _, _ := tv.m.grezzo("PUT", base+"/pagine/1"+q, []byte("\x89PNG nient'altro"), "image/png"); code != 415 {
		t.Fatalf("solo WebP: %d", code)
	}
	if code, _, _ := tv.m.grezzo("PUT", base+"/pagine/4"+q, webpFinto(4), "image/webp"); code != 400 {
		t.Fatalf("pagina oltre il totale: %d", code)
	}
	for _, n := range []string{"1", "3"} {
		if code, _, b := tv.m.grezzo("PUT", base+"/pagine/"+n+q, webpFinto(int(n[0])), "image/webp"); code != 200 {
			t.Fatalf("pagina %s: %d %s", n, code, b)
		}
	}
	st := tv.m.deve(200, "GET", base+"/pagine", nil)
	if st["totale"] != float64(3) || st["pronte"] != float64(2) || st["testo"] != false {
		t.Fatalf("stato: %v", st)
	}
	if m := st["mancanti"].([]any); len(m) != 1 || m[0] != float64(2) {
		t.Fatalf("il Master sa quali mancano: %v", st)
	}
	versione := st["versione"].(string)

	// Non condiviso: niente pagine per i giocatori.
	if code, _, _ := tv.luca.grezzo("GET", base+"/pagine/1", nil, ""); code != 404 {
		t.Fatalf("manuale non condiviso: %d", code)
	}
	tv.m.deve(200, "PATCH", base, map[string]any{"utenti": []string{tv.a.idUtente(tv.luca)}})
	code, h, b := tv.luca.grezzo("GET", base+"/pagine/1?v="+versione, nil, "")
	if code != 200 || !bytes.Equal(b, webpFinto('1')) || h.Get("Content-Type") != "image/webp" || !strings.Contains(h.Get("Cache-Control"), "immutable") {
		t.Fatalf("pagina al giocatore: %d %s %s", code, h.Get("Content-Type"), h.Get("Cache-Control"))
	}
	if code, h, _ := tv.luca.grezzo("GET", base+"/pagine/1", nil, ""); code != 200 || strings.Contains(h.Get("Cache-Control"), "immutable") {
		t.Fatalf("senza versione si ricontrolla: %d %s", code, h.Get("Cache-Control"))
	}
	if code, _, _ := tv.luca.grezzo("GET", base+"/pagine/2", nil, ""); code != 404 {
		t.Fatalf("pagina non pronta: %d", code)
	}
	if code, _, _ := tv.sara.grezzo("GET", base+"/pagine/1", nil, ""); code != 404 {
		t.Fatalf("Sara non ha il manuale: %d", code)
	}
	// Elenco: i giocatori vedono quante pagine sono pronte (non quali mancano).
	l := tv.luca.deve(200, "GET", "/manuali", nil)["lista"].([]any)
	if p := l[0].(map[string]any)["pagine"].(map[string]any); p["pronte"] != float64(2) || p["mancanti"] != nil {
		t.Fatalf("elenco del giocatore: %v", p)
	}

	// Testo per la ricerca: solo dopo le pagine, una voce per pagina.
	if code, _ := tv.m.chiama("PUT", base+"/testo", map[string]any{"pagine": []string{"uno"}}); code != 400 {
		t.Fatalf("testo con pagine sbagliate: %d", code)
	}
	tv.m.deve(200, "PUT", base+"/testo", map[string]any{"pagine": []string{"Pagina 1", "Pagina 2", "Vampiri e nebbia"}})
	if code, _, b := tv.luca.grezzo("GET", base+"/testo", nil, ""); code != 200 || !strings.Contains(string(b), "Vampiri e nebbia") {
		t.Fatalf("testo al giocatore: %d %s", code, b)
	}

	// Il PDF cambia (manuale sostituito): le pagine vecchie non valgono più e la preparazione riparte.
	tv.a.s.DB.Exec(`UPDATE manuali SET file = 'nuovofile0000000.pdf' WHERE id = ?`, id)
	if st := tv.m.deve(200, "GET", base+"/pagine", nil); st["pronte"] != float64(0) {
		t.Fatalf("PDF cambiato: %v", st)
	}
	if code, _, _ := tv.luca.grezzo("GET", base+"/pagine/1", nil, ""); code != 404 {
		t.Fatalf("pagina di un PDF vecchio: %d", code)
	}
	if code, _, _ := tv.m.grezzo("PUT", base+"/pagine/1"+q+"&file="+versione+".pdf", webpFinto(1), "image/webp"); code != 409 {
		t.Fatalf("pagina letta dal PDF vecchio: %d", code)
	}
	tv.m.grezzo("PUT", base+"/pagine/2"+q, webpFinto(2), "image/webp")
	if st := tv.m.deve(200, "GET", base+"/pagine", nil); st["pronte"] != float64(1) {
		t.Fatalf("ripartita: %v", st)
	}

	// Eliminando il manuale spariscono anche le pagine.
	tv.m.deve(200, "DELETE", base, nil)
	if _, err := os.Stat(tv.a.s.cartellaPagine(id)); !os.IsNotExist(err) {
		t.Fatalf("cartella delle pagine rimasta: %v", err)
	}
}
