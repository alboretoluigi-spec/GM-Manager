package server

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func (sc *scontro) invia(c *cliente, metodo, p string, dati []byte) int {
	sc.a.t.Helper()
	req, _ := http.NewRequest(metodo, sc.a.srv.URL+"/api/v1/mappa/"+p, bytes.NewReader(dati))
	req.Header.Set("X-CSRF", c.csrf)
	if c.ip != "" {
		req.Header.Set("X-Test-IP", c.ip)
	}
	r, err := c.c.Do(req)
	if err != nil {
		sc.a.t.Fatal(err)
	}
	r.Body.Close()
	return r.StatusCode
}

func TestBestiarioImmaginiMostrateAiGiocatori(t *testing.T) {
	sc := nuovoScontro(t)
	png := []byte("\x89PNG\r\n\x1a\n0000000000")
	// Solo il Master legge il bestiario e carica immagini.
	sc.mappa(sc.luca, 403, "GET", "best", nil)
	if c := sc.invia(sc.luca, "POST", "best/img?sessione="+sc.ses+"&id=mm-acolyte", png); c != 403 {
		t.Fatalf("il giocatore non carica immagini: %d", c)
	}
	if c := sc.invia(sc.m, "POST", "best/img?sessione="+sc.ses+"&id=voce-inesistente", png); c != 404 {
		t.Fatalf("voce inesistente: %d", c)
	}
	if c := sc.invia(sc.m, "POST", "best/img?sessione="+sc.ses+"&id=mm-acolyte", []byte("non un'immagine")); c != 415 {
		t.Fatalf("file non immagine: %d", c)
	}
	if c := sc.invia(sc.m, "POST", "best/img?sessione="+sc.ses+"&id=mm-acolyte", png); c != 200 {
		t.Fatalf("caricamento immagine: %d", c)
	}
	img := sc.mappa(sc.m, 200, "GET", "best", nil)["entries"].(map[string]any)["mm-acolyte"].(map[string]any)["img"].(string)
	// Finché non è mostrata, il giocatore non la apre.
	if c := sc.scarica(sc.luca, img); c != 404 {
		t.Fatalf("immagine non mostrata: %d", c)
	}
	sc.mappa(sc.m, 200, "POST", "best/show", map[string]any{"items": []map[string]string{{"id": "mm-acolyte", "name": "Donavich"}}})
	if c := sc.scarica(sc.luca, img); c != 200 {
		t.Fatalf("immagine mostrata: %d", c)
	}
	if c := sc.scarica(sc.ugo, img); c != 404 {
		t.Fatalf("un altro gruppo non la vede: %d", c)
	}
	sc.mappa(sc.m, 200, "POST", "best/show", map[string]any{"hide": true})
	if c := sc.scarica(sc.luca, img); c != 404 {
		t.Fatalf("immagine nascosta: %d", c)
	}
	// Proiettata: di nuovo visibile.
	sc.mappa(sc.m, 200, "POST", "proj", map[string]any{"kind": "beast", "file": img, "cap": "Donavich"})
	if c := sc.scarica(sc.luca, img); c != 200 {
		t.Fatalf("immagine proiettata: %d", c)
	}
	sc.mappa(sc.m, 404, "POST", "proj", map[string]any{"kind": "beast", "file": "Immagini/qualsiasi.png"})
	// Voce personale, scheda trascritta e note.
	id := sc.mappa(sc.m, 200, "POST", "best", map[string]any{"new": map[string]string{"nome": "Lupo albino", "cat": "mostro", "ruolo": "capobranco"}})["id"].(string)
	sc.mappa(sc.m, 200, "POST", "best", map[string]any{"id": id, "custom": "CA 14, PF 37", "notes": "Fugge a metà PF"})
	b := sc.mappa(sc.m, 200, "GET", "best", nil)
	if b["entries"].(map[string]any)[id].(map[string]any)["custom"] != "CA 14, PF 37" || len(b["extras"].([]any)) != 1 {
		t.Fatalf("voce personale non salvata: %v", b)
	}
	sc.mappa(sc.m, 400, "DELETE", "best?id=mm-acolyte", nil) // le voci di base non si eliminano
	sc.mappa(sc.m, 200, "DELETE", "best?id="+id, nil)
}

func TestManualiSoloMaster(t *testing.T) {
	sc := nuovoScontro(t)
	if c := sc.invia(sc.m, "PUT", "manual?sessione="+sc.ses+"&nome=Prova&tipo=mm", []byte("<html>")); c != 415 {
		t.Fatalf("un file non PDF va rifiutato: %d", c)
	}
	if c := sc.invia(sc.m, "PUT", "manual?sessione="+sc.ses+"&nome=Manuale%20dei%20Mostri&tipo=mm", []byte("%PDF-1.4\n%%EOF")); c != 200 {
		t.Fatalf("caricamento del manuale: %d", c)
	}
	man := sc.mappa(sc.m, 200, "GET", "best", nil)["manuals"].([]any)[0].(map[string]any)
	id, file := man["id"].(string), man["file"].(string)
	sc.mappa(sc.m, 200, "POST", "manual", map[string]any{"id": id, "refBook": 10, "refPdf": 12, "spread": false})
	sc.mappa(sc.m, 200, "POST", "manual", map[string]any{"id": id, "index": map[string]int{"mm-acolyte": 40, "inventata": 3}, "noText": false})
	man = sc.mappa(sc.m, 200, "GET", "best", nil)["manuals"].([]any)[0].(map[string]any)
	if man["refPdf"].(float64) != 12 || len(man["index"].(map[string]any)) != 1 || man["indexed"] != true {
		t.Fatalf("calibrazione o indice non salvati: %v", man)
	}
	sc.mappa(sc.m, 200, "POST", "best", map[string]any{"id": "mm-acolyte", "man": id, "page": 46})
	// Il PDF si apre solo dal Master.
	get := func(c *cliente) int {
		req, _ := http.NewRequest("GET", sc.a.srv.URL+"/manuali/"+file, nil)
		if c.ip != "" {
			req.Header.Set("X-Test-IP", c.ip)
		}
		r, err := c.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if c := get(sc.luca); c != 404 {
		t.Fatalf("il giocatore non deve aprire i manuali: %d", c)
	}
	if c := get(sc.m); c != 200 {
		t.Fatalf("il Master apre il manuale: %d", c)
	}
	sc.mappa(sc.m, 200, "DELETE", "manual?id="+id, nil)
	if _, err := os.Stat(filepath.Join(sc.a.s.Cfg.CartellaMedia, "Manuali", file)); !os.IsNotExist(err) {
		t.Fatal("il PDF caricato dall'app va cancellato con il manuale")
	}
}

func TestBestiarioJSONSoloMaster(t *testing.T) {
	a := nuovoAmbiente(t)
	a.s.Web = os.DirFS("../../web")
	srv := a.srv
	m := a.master()
	g := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	get := func(c *cliente) int {
		req, _ := http.NewRequest("GET", srv.URL+"/carta/bestiario.json", nil)
		if c.ip != "" {
			req.Header.Set("X-Test-IP", c.ip)
		}
		r, err := c.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if c := get(g); c != 404 {
		t.Fatalf("il giocatore non deve scaricare il bestiario: %d", c)
	}
	if c := get(m); c != 200 {
		t.Fatalf("il Master sì: %d", c)
	}
}
