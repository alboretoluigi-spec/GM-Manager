package server

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RPG RULEBOOK LIBRARY, passo L1: il catalogo incorporato è valido, l'API filtra per gioco, genere, editore, lingua,
// tipologia, edizione e gratuità, solo il Master lo vede, e una copia locale non valida non rompe nulla.
func TestCatalogoRegolamenti(t *testing.T) {
	c, err := leggiCatalogo(catalogoIncorporato)
	if err != nil {
		t.Fatalf("catalogo incorporato: %v", err)
	}
	if len(c.Giochi) != 21 {
		t.Fatalf("giochi nel catalogo: %d", len(c.Giochi))
	}
	for _, g := range c.Giochi {
		for _, d := range g.Documenti {
			if d.DownloadDiretto && (!strings.HasSuffix(strings.ToLower(d.URL), ".pdf") || d.VerificatoIl == "") {
				t.Errorf("%s: download diretto solo per PDF verificati", d.ID)
			}
		}
	}

	a := nuovoAmbiente(t)
	m := a.master()
	cat := func(q url.Values) map[string]any {
		t.Helper()
		return m.deve(200, "GET", "/regolamenti/catalogo?"+q.Encode(), nil)
	}
	nomi := func(r map[string]any) []string {
		var l []string
		for _, g := range r["giochi"].([]any) {
			l = append(l, g.(map[string]any)["nome"].(string))
		}
		return l
	}
	tutto := cat(nil)
	if len(tutto["giochi"].([]any)) != 21 || tutto["avviso"] == "" || tutto["totale"] != 21.0 {
		t.Fatalf("catalogo completo: %d giochi, avviso %q", len(tutto["giochi"].([]any)), tutto["avviso"])
	}
	f := tutto["filtri"].(map[string]any)
	for _, k := range []string{"generi", "editori", "lingue", "tipologie", "edizioni"} {
		if len(f[k].([]any)) == 0 {
			t.Fatalf("filtro %s vuoto", k)
		}
	}
	if n := nomi(cat(url.Values{"gioco": {"cthulhu"}})); len(n) != 1 || n[0] != "Call of Cthulhu" {
		t.Fatalf("ricerca per nome: %v", n)
	}
	if n := nomi(cat(url.Values{"editore": {"Chaosium"}})); strings.Join(n, ",") != "Basic Roleplaying,Call of Cthulhu,RuneQuest" {
		t.Fatalf("filtro editore: %v", n)
	}
	if n := nomi(cat(url.Values{"genere": {"cyberpunk"}})); strings.Join(n, ",") != "Cyberpunk RED,Shadowrun" {
		t.Fatalf("filtro genere: %v", n)
	}
	srd := cat(url.Values{"tipologia": {"srd"}})
	for _, g := range srd["giochi"].([]any) {
		for _, d := range g.(map[string]any)["documenti"].([]any) {
			if d.(map[string]any)["tipologia"] != "srd" {
				t.Fatalf("filtro tipologia: %v", d)
			}
		}
	}
	if len(srd["giochi"].([]any)) < 3 {
		t.Fatalf("giochi con SRD: %v", nomi(srd))
	}
	gratis := nomi(cat(url.Values{"gratuito": {"1"}}))
	for _, n := range gratis {
		if n == "The One Ring" || n == "Alien RPG" { // nessun documento gratuito verificato
			t.Fatalf("gioco senza documenti gratuiti nel filtro «gratuito»: %v", gratis)
		}
	}
	if n := nomi(cat(url.Values{"edizione": {"7e"}})); len(n) != 1 || n[0] != "Call of Cthulhu" {
		t.Fatalf("filtro edizione: %v", n)
	}
	if n := nomi(cat(url.Values{"lingua": {"xx"}})); len(n) != 0 {
		t.Fatalf("lingua inesistente: %v", n)
	}

	// solo il Master
	a.giocatore("Luca", "333 1234567", "192.168.1.20").deve(403, "GET", "/regolamenti/catalogo", nil)

	// copia locale: se valida si usa, se non valida si ignora
	dir := filepath.Join(a.s.Cfg.CartellaDati, "regolamenti")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "catalogo.json"), []byte(`{"versione":1,"giochi":[{"id":"x","nome":"X","sito":"http://non-sicuro"}]}`), 0o600)
	if len(cat(nil)["giochi"].([]any)) != 21 {
		t.Fatal("catalogo locale non valido: deve restare quello incorporato")
	}
	valido := `{"versione":1,"aggiornatoIl":"2026-12-01","avviso":"a","tipologie":{"srd":"SRD"},"giochi":[{"id":"x","nome":"Gioco X","editore":"E","generi":["g"],"edizioni":["1"],"sito":"https://esempio.it","documenti":[{"id":"x1","titolo":"T","tipologia":"srd","lingua":"it","formato":"pdf","url":"https://esempio.it/t.pdf","licenza":"CC-BY-4.0","gratuito":true,"downloadDiretto":true,"verificatoIl":"2026-12-01"}]}]}`
	os.WriteFile(filepath.Join(dir, "catalogo.json"), []byte(valido), 0o600)
	if n := nomi(cat(nil)); len(n) != 1 || n[0] != "Gioco X" {
		t.Fatalf("catalogo locale valido: %v", n)
	}
	for _, sbagliato := range []string{
		`{"versione":1,"tipologie":{"srd":"SRD"},"giochi":[{"id":"x","nome":"X","sito":"https://a.it","documenti":[{"id":"d","titolo":"T","tipologia":"boh","lingua":"it","formato":"pdf","url":"https://a.it/x.pdf","licenza":"L"}]}]}`,
		`{"versione":1,"tipologie":{"srd":"SRD"},"giochi":[{"id":"x","nome":"X","sito":"https://a.it","documenti":[{"id":"d","titolo":"T","tipologia":"srd","lingua":"it","formato":"pdf","url":"https://a.it/x.pdf","licenza":"L","gratuito":false,"downloadDiretto":true,"verificatoIl":"2026-01-01"}]}]}`,
		`{"versione":1,"tipologie":{"srd":"SRD"},"giochi":[{"id":"x","nome":"X","sito":"https://a.it"},{"id":"x","nome":"Y","sito":"https://a.it"}]}`,
	} {
		if _, err := leggiCatalogo([]byte(sbagliato)); err == nil {
			t.Fatalf("catalogo sbagliato accettato: %s", sbagliato)
		}
	}
}
