package server

import (
	"bytes"
	"net/http"
	"testing"
)

// Sezione «PNG e mostri»: solo il Master; PNG nuovo da un modello ufficiale (scheda copiata e modificabile),
// scheda strutturata salvata e controllata, immagine, pagina trovata nel manuale, eliminazione.
func TestSezionePNGeMostri(t *testing.T) {
	tv := nuovoTavolo(t)
	if c, _ := tv.luca.chiama("GET", "/bestiario", nil); c != 403 {
		t.Fatal("i giocatori non vedono il bestiario")
	}
	if c, _ := tv.luca.chiama("POST", "/bestiario/voci", map[string]any{"nome": "X", "cat": "png"}); c != 403 {
		t.Fatal("i giocatori non creano voci")
	}
	// PNG nuovo dal modello Nobile: la scheda del modello viene copiata.
	id := tv.m.deve(200, "POST", "/bestiario/voci", map[string]any{"nome": "Conte Dragomir", "cat": "png", "ruolo": "Nobile di Borgofosco", "base": "mm-noble"})["id"].(string)
	b := tv.m.deve(200, "GET", "/bestiario", nil)
	sh, _ := b["entries"].(map[string]any)[id].(map[string]any)["sheet"].(map[string]any)
	if sh == nil || sh["ca"] == "" || len(sh["azioni"].([]any)) == 0 {
		t.Fatalf("scheda del modello copiata: %v", sh)
	}
	ex := b["extras"].([]any)
	if len(ex) != 1 || ex[0].(map[string]any)["base"] != "mm-noble" {
		t.Fatalf("voce personale con il modello: %v", ex)
	}
	if c, _ := tv.m.chiama("POST", "/bestiario/voci", map[string]any{"nome": "Y", "cat": "png", "base": "mm-inventato"}); c != 400 {
		t.Fatal("modello inesistente")
	}
	// Scheda modificata (per esempio letta dal manuale): controlli su campi e valori.
	sc := map[string]any{"taglia": "Media", "tipo": "umanoide (umano)", "all": "legale malvagio", "ca": "15 (giaco di maglia)", "pf": "27 (5d8 + 5)", "vel": "9 m",
		"car": []int{11, 12, 11, 12, 14, 16}, "gs": "1 (200 PE)", "tratti": []map[string]string{{"n": "Arrogante", "d": "Testo del Master."}},
		"azioni": []map[string]string{{"n": "Stocco", "d": "+3 al tiro per colpire."}}, "reazioni": []any{}, "legg": []any{}}
	tv.m.deve(200, "PUT", "/bestiario/voci/"+id+"/scheda", map[string]any{"scheda": sc})
	sh = tv.m.deve(200, "GET", "/bestiario", nil)["entries"].(map[string]any)[id].(map[string]any)["sheet"].(map[string]any)
	if sh["ca"] != "15 (giaco di maglia)" || sh["tratti"].([]any)[0].(map[string]any)["n"] != "Arrogante" {
		t.Fatalf("scheda salvata: %v", sh)
	}
	sc["car"] = []int{11, 12, 11, 12, 14, 99}
	if c, _ := tv.m.chiama("PUT", "/bestiario/voci/"+id+"/scheda", map[string]any{"scheda": sc}); c != 400 {
		t.Fatal("caratteristica fuori scala")
	}
	sc["car"] = []int{10, 10, 10, 10, 10, 10}
	sc["campoStrano"] = "x"
	if c, _ := tv.m.chiama("PUT", "/bestiario/voci/"+id+"/scheda", map[string]any{"scheda": sc}); c != 400 {
		t.Fatal("campi sconosciuti rifiutati")
	}
	// Anche una voce di base può avere la sua scheda; null la toglie.
	tv.m.deve(200, "PUT", "/bestiario/voci/mm-wolf/scheda", map[string]any{"scheda": map[string]any{"ca": "13", "car": []int{12, 15, 12, 3, 12, 6}}})
	tv.m.deve(200, "PUT", "/bestiario/voci/mm-wolf/scheda", map[string]any{"scheda": nil})
	if _, ok := tv.m.deve(200, "GET", "/bestiario", nil)["entries"].(map[string]any)["mm-wolf"].(map[string]any)["sheet"]; ok {
		t.Fatal("scheda tolta")
	}
	// Nome e ruolo solo per le voci personali; note per tutte.
	tv.m.deve(200, "PATCH", "/bestiario/voci/"+id, map[string]any{"nome": "Conte Dragomir Borgofosco", "note": "Odia il barone."})
	if c, _ := tv.m.chiama("PATCH", "/bestiario/voci/mm-wolf", map[string]any{"nome": "Lupetto"}); c != 400 {
		t.Fatal("le voci di base non si rinominano")
	}
	// Immagine: solo formati immagine.
	put := func(dati []byte) int {
		req, _ := http.NewRequest("PUT", tv.a.srv.URL+"/api/v1/bestiario/voci/"+id+"/immagine", bytes.NewReader(dati))
		req.Header.Set("X-CSRF", tv.m.csrf)
		r, err := tv.m.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if put([]byte("non un'immagine")) != 415 || put([]byte("\x89PNG\r\n\x1a\n0000000000")) != 200 {
		t.Fatal("immagine del PNG")
	}
	if img := tv.m.deve(200, "GET", "/bestiario", nil)["entries"].(map[string]any)[id].(map[string]any)["img"]; img == nil {
		t.Fatal("immagine salvata")
	}
	// Pagina trovata nel manuale.
	if c, out := tv.m.caricaManuale([]byte("%PDF-1.4 prova"), "Manuale dei Mostri"); c != 200 {
		t.Fatalf("manuale: %d %v", c, out)
	}
	man := tv.m.deve(200, "GET", "/bestiario", nil)["manuals"].([]any)[0].(map[string]any)["id"].(string)
	tv.m.deve(200, "POST", "/bestiario/manuali/"+man+"/indice", map[string]any{"voce": "mm-wolf", "pagina": 341})
	if p := tv.m.deve(200, "GET", "/bestiario", nil)["manuals"].([]any)[0].(map[string]any)["index"].(map[string]any)["mm-wolf"]; p != 341.0 {
		t.Fatalf("pagina ricordata: %v", p)
	}
	// Si eliminano solo le voci personali.
	if c, _ := tv.m.chiama("DELETE", "/bestiario/voci/mm-wolf", nil); c != 400 {
		t.Fatal("voce di base non eliminabile")
	}
	tv.m.deve(200, "DELETE", "/bestiario/voci/"+id, nil)
	if len(tv.m.deve(200, "GET", "/bestiario", nil)["extras"].([]any)) != 0 {
		t.Fatal("voce eliminata")
	}
}

// «🎭 PNG in scena» in due raccolte (2026-11-13): Campagna ufficiale e CoS Reloaded. La raccolta si sceglie per i PNG del
// bestiario di Gaster Bless e per quelli nuovi; resta nel file della campagna.
func TestRaccoltaPNG(t *testing.T) {
	tv := nuovoTavolo(t)
	tv.m.deve(200, "PUT", "/bestiario/voci/png-stanimir/raccolta", map[string]any{"raccolta": "reloaded"})
	e := tv.m.deve(200, "GET", "/bestiario", nil)["entries"].(map[string]any)
	if e["png-stanimir"].(map[string]any)["raccolta"] != "reloaded" {
		t.Fatalf("raccolta di un PNG del bestiario: %v", e["png-stanimir"])
	}
	if c, _ := tv.m.chiama("PUT", "/bestiario/voci/png-stanimir/raccolta", map[string]any{"raccolta": "altro"}); c != 400 {
		t.Fatal("solo ufficiale o reloaded")
	}
	if c, _ := tv.m.chiama("PUT", "/bestiario/voci/png-inventato/raccolta", map[string]any{"raccolta": "reloaded"}); c != 404 {
		t.Fatal("voce inesistente")
	}
	if c, _ := tv.luca.chiama("PUT", "/bestiario/voci/png-stanimir/raccolta", map[string]any{"raccolta": "ufficiale"}); c != 403 {
		t.Fatal("solo il Master")
	}
	id := tv.m.deve(200, "POST", "/bestiario/voci", map[string]any{"nome": "Ezmerelda di prova", "cat": "png", "raccolta": "reloaded"})["id"].(string)
	if c, _ := tv.m.chiama("POST", "/bestiario/voci", map[string]any{"nome": "Lupo", "cat": "mostro", "raccolta": "reloaded"}); c != 400 {
		t.Fatal("la raccolta vale solo per i PNG")
	}
	e = tv.m.deve(200, "GET", "/bestiario", nil)["entries"].(map[string]any)
	if e[id].(map[string]any)["raccolta"] != "reloaded" {
		t.Fatalf("PNG nuovo nella raccolta scelta: %v", e[id])
	}
	// di nuovo nella campagna ufficiale
	tv.m.deve(200, "PUT", "/bestiario/voci/png-stanimir/raccolta", map[string]any{"raccolta": "ufficiale"})
	e = tv.m.deve(200, "GET", "/bestiario", nil)["entries"].(map[string]any)
	if e["png-stanimir"].(map[string]any)["raccolta"] != "ufficiale" {
		t.Fatal("spostato nella campagna ufficiale")
	}
}
