package server

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"testing"
)

// giocatoreApprovato prepara Master e giocatore approvato nel gruppo A, più un personaggio del giocatore.
func giocatoreApprovato(t *testing.T) (a *ambiente, master, gioc *cliente, pg string) {
	a, master, gioc, gA, _, idGioc := preparaPartita(t)
	master.deve(200, "POST", "/utenti/"+idGioc+"/approva", map[string]any{"gruppi": []string{gA}})
	pg = gioc.deve(200, "POST", "/personaggi", map[string]any{"nome": "Arin"})["id"].(string)
	return
}

func versione(m map[string]any) int { return int(m["versione"].(float64)) }

func TestSchedaVersioniEUnione(t *testing.T) {
	_, master, gioc, pg := giocatoreApprovato(t)

	// Il giocatore vede il personaggio, già nel suo unico gruppo, con il nome dentro la scheda.
	m := gioc.deve(200, "GET", "/personaggi/"+pg, nil)
	if m["personaggio"].(map[string]any)["gruppo"] == "" {
		t.Fatal("il personaggio dovrebbe essere assegnato all'unico gruppo del giocatore")
	}
	if m["dati"].(map[string]any)["f"].(map[string]any)["nome"] != "Arin" {
		t.Fatalf("nome non salvato nella scheda: %v", m["dati"])
	}

	// Il giocatore cambia livello e classe partendo dalla versione 1.
	v2 := versione(gioc.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1,
		"modifiche": map[string]any{"lvl": 3, "cls": "Chierico", "f.giocatore": "Sara"},
		"riepilogo": map[string]any{"class": "Chierico", "level": 3, "ac": 16, "hpMax": 24, "initiative": 1}}))
	if v2 != 2 {
		t.Fatalf("attesa versione 2, ricevuta %d", v2)
	}
	// Il Master, partito anche lui dalla versione 1, cambia un campo DIVERSO: si unisce.
	v3 := versione(master.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1,
		"modifiche": map[string]any{"al": "Neutrale buono"}}))
	if v3 != 3 {
		t.Fatalf("attesa versione 3, ricevuta %d", v3)
	}
	d := master.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
	if d["lvl"].(float64) != 3 || d["al"] != "Neutrale buono" || d["cls"] != "Chierico" {
		t.Fatalf("unione non riuscita: %v", d)
	}

	// Il Master (ancora sulla versione 1) cambia LO STESSO campo con un altro valore: conflitto.
	cod, risp := master.chiama("PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"lvl": 4}})
	if cod != 409 {
		t.Fatalf("atteso conflitto 409, ricevuto %d %v", cod, risp)
	}
	c := risp["conflitti"].([]any)[0].(map[string]any)
	if c["campo"] != "lvl" || c["attuale"].(float64) != 3 || c["tuo"].(float64) != 4 {
		t.Fatalf("conflitto descritto male: %v", c)
	}
	// Sceglie di tenere il proprio valore.
	v4 := versione(master.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"lvl": 4}, "forza": []string{"lvl"}}))
	if v4 != 4 {
		t.Fatalf("attesa versione 4, ricevuta %d", v4)
	}

	// Lo storico si ripristina creando una nuova versione.
	vers := elenco(gioc.deve(200, "GET", "/personaggi/"+pg+"/versioni", nil))
	if len(vers) < 3 {
		t.Fatalf("storico troppo corto: %v", vers)
	}
	v5 := versione(gioc.deve(200, "POST", "/personaggi/"+pg+"/ripristina/2", map[string]any{}))
	d = gioc.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
	if v5 != 5 || d["lvl"].(float64) != 3 || d["al"] != nil {
		t.Fatalf("ripristino non riuscito (v%d): %v", v5, d)
	}

	// L'elenco mostra il riepilogo.
	l := elenco(gioc.deve(200, "GET", "/personaggi", nil))
	if len(l) != 1 || l[0]["riepilogo"].(map[string]any)["classe"] != "Chierico" {
		t.Fatalf("riepilogo mancante: %v", l)
	}
}

func TestSchedaStatoInGioco(t *testing.T) {
	_, master, gioc, pg := giocatoreApprovato(t)
	// I PF dalla scheda non creano versioni.
	v := versione(gioc.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"hpCur": "17", "ds.s": 1}}))
	if v != 1 {
		t.Fatalf("i PF non devono creare versioni: versione %d", v)
	}
	// Il Master (tracker) toglie PF: la scheda li vede.
	master.deve(200, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{"pf": 9, "pfTemp": 3, "condizioni": []string{"Prono"}})
	d := gioc.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
	if d["hpCur"] != "9" || d["hpTmp"] != "3" || d["ds"].(map[string]any)["s"].(float64) != 1 {
		t.Fatalf("stato in gioco non riportato nella scheda: %v", d)
	}
	master.deve(400, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{"tsMorte": map[string]int{"s": 5, "f": 0}})
}

func TestSchedaPermessi(t *testing.T) {
	a, master, gioc, pg := giocatoreApprovato(t)
	// Un secondo giocatore, approvato nello stesso gruppo, non vede né modifica la scheda di Sara.
	gruppi := elenco(master.deve(200, "GET", "/gruppi", nil))
	altro := a.cliente("")
	altro.deve(200, "POST", "/registrazione", map[string]any{"nome": "Marco", "telefono": "+39 333 7654321",
		"password": pwGiocatore, "conferma": pwGiocatore, "consenso": true})
	var idAltro string
	for _, u := range elenco(master.deve(200, "GET", "/utenti", nil)) {
		if u["nome"] == "Marco" {
			idAltro = u["id"].(string)
		}
	}
	master.deve(200, "POST", "/utenti/"+idAltro+"/approva", map[string]any{"gruppi": []string{gruppi[0]["id"].(string)}})
	altro.deve(404, "GET", "/personaggi/"+pg, nil)
	altro.deve(404, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"lvl": 20}})
	altro.deve(404, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{"pf": 0})
	altro.deve(404, "GET", "/personaggi/"+pg+"/versioni", nil)
	if l := elenco(altro.deve(200, "GET", "/personaggi", nil)); len(l) != 0 {
		t.Fatalf("Marco non deve vedere personaggi altrui: %v", l)
	}
	// Il Master li vede tutti.
	if l := elenco(master.deve(200, "GET", "/personaggi", nil)); len(l) != 1 {
		t.Fatalf("il Master deve vedere il personaggio: %v", l)
	}
	// Campi con percorsi strani rifiutati.
	gioc.deve(400, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"a.b.c": 1}})
	// Personaggio ritirato: solo consultazione; il Master lo può archiviare, il giocatore no.
	gioc.deve(200, "POST", "/personaggi/"+pg+"/stato", map[string]any{"stato": "ritirato"})
	gioc.deve(403, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"lvl": 2}})
	gioc.deve(403, "POST", "/personaggi/"+pg+"/stato", map[string]any{"stato": "attivo"}) // solo il Master riattiva
	gioc.deve(403, "POST", "/personaggi/"+pg+"/stato", map[string]any{"stato": "archiviato"})
	master.deve(200, "POST", "/personaggi/"+pg+"/stato", map[string]any{"stato": "archiviato"})
}

func TestBackground(t *testing.T) {
	a, master, gioc, pg := giocatoreApprovato(t)
	carica := func(c *cliente, dati []byte) int {
		req, _ := http.NewRequest("PUT", a.srv.URL+"/api/v1/personaggi/"+pg+"/background", bytes.NewReader(dati))
		req.Header.Set("Origin", a.srv.URL)
		req.Header.Set("X-CSRF", c.csrf)
		req.Header.Set("X-Nome-File", "storia%20di%20Arin.pdf")
		r, err := c.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	// Un file che finge di essere un PDF viene rifiutato.
	if c := carica(gioc, []byte("<html><script>alert(1)</script>")); c != 415 {
		t.Fatalf("file non valido: atteso 415, ricevuto %d", c)
	}
	// Il Master non carica al posto del giocatore.
	pdf := []byte("%PDF-1.4\n% prova\n")
	if c := carica(master, pdf); c != 403 {
		t.Fatalf("il Master non deve caricare: ricevuto %d", c)
	}
	if c := carica(gioc, pdf); c != 200 {
		t.Fatalf("caricamento PDF: ricevuto %d", c)
	}
	// Anche un .docx vero (zip con word/document.xml) è accettato.
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	f, _ := z.Create("word/document.xml")
	f.Write([]byte("<w:document/>"))
	z.Close()
	if c := carica(gioc, buf.Bytes()); c != 200 {
		t.Fatalf("caricamento docx: ricevuto %d", c)
	}
	info := gioc.deve(200, "GET", "/personaggi/"+pg+"/background/info", nil)["background"].(map[string]any)
	if info["tipo"] != "docx" || info["nome"] != "storia di Arin.pdf" {
		t.Fatalf("informazioni sbagliate: %v", info)
	}
	// Il Master lo legge (e resta nel registro), l'elenco è solo suo.
	r, err := master.c.Get(a.srv.URL + "/api/v1/personaggi/" + pg + "/background")
	if err != nil {
		t.Fatal(err)
	}
	letto, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if !bytes.Equal(letto, buf.Bytes()) {
		t.Fatal("il file letto non corrisponde a quello caricato")
	}
	if l := elenco(master.deve(200, "GET", "/background", nil)); len(l) != 1 {
		t.Fatalf("elenco del Master: %v", l)
	}
	gioc.deve(403, "GET", "/background", nil)
	gioc.deve(200, "DELETE", "/personaggi/"+pg+"/background", nil)
	if m := gioc.deve(200, "GET", "/personaggi/"+pg+"/background/info", nil); m["background"] != nil {
		t.Fatalf("background non eliminato: %v", m)
	}
}
