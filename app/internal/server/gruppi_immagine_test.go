package server

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

// Immagine del gruppo: la carica solo il Master; la vedono il Master e i giocatori di quel gruppo.
func TestImmagineGruppo(t *testing.T) {
	tv := nuovoTavolo(t)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	invia := func(c *cliente, metodo, g string, dati []byte) int {
		req, _ := http.NewRequest(metodo, tv.a.srv.URL+"/api/v1/gruppi/"+g+"/immagine", bytes.NewReader(dati))
		req.Header.Set("X-CSRF", c.csrf)
		r, err := c.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	leggi := func(c *cliente, g string) (int, []byte) {
		r, err := c.c.Get(tv.a.srv.URL + "/api/v1/gruppi/" + g + "/immagine")
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, b
	}
	if invia(tv.luca, "PUT", tv.g1, png) != 403 {
		t.Fatal("solo il Master carica l'immagine")
	}
	if invia(tv.m, "PUT", tv.g1, []byte("testo, non un'immagine")) != 415 {
		t.Fatal("solo immagini")
	}
	if invia(tv.m, "PUT", "gruppo-inesistente", png) != 404 {
		t.Fatal("gruppo inesistente")
	}
	if c := invia(tv.m, "PUT", tv.g1, png); c != 200 {
		t.Fatalf("caricamento del Master: %d", c)
	}
	var img string
	for _, g := range tv.luca.deve(200, "GET", "/gruppi", nil)["lista"].([]any) {
		if gg := g.(map[string]any); gg["id"] == tv.g1 {
			img, _ = gg["immagine"].(string)
		}
	}
	if img == "" {
		t.Fatal("l'elenco dei gruppi indica l'immagine")
	}
	if c, b := leggi(tv.luca, tv.g1); c != 200 || !bytes.Equal(b, png) {
		t.Fatalf("il giocatore del gruppo vede l'immagine: %d", c)
	}
	if c, _ := leggi(tv.m, tv.g1); c != 200 {
		t.Fatal("il Master vede l'immagine")
	}
	if c, _ := leggi(tv.ugo, tv.g1); c != 404 {
		t.Fatal("chi non è nel gruppo non la vede")
	}
	// Nuovo caricamento: nome nuovo (la vecchia si cancella); poi si toglie.
	invia(tv.m, "PUT", tv.g1, append(png, 1))
	for _, g := range tv.m.deve(200, "GET", "/gruppi", nil)["lista"].([]any) {
		if gg := g.(map[string]any); gg["id"] == tv.g1 && gg["immagine"] == img {
			t.Fatal("il nome cambia a ogni caricamento")
		}
	}
	if invia(tv.m, "DELETE", tv.g1, nil) != 200 {
		t.Fatal("eliminazione")
	}
	if c, _ := leggi(tv.luca, tv.g1); c != 404 {
		t.Fatal("immagine tolta")
	}
}
