package server

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tavolo: Master, due giocatori dello stesso gruppo e uno di un altro gruppo.
type tavolo struct {
	a                  *ambiente
	m, luca, sara, ugo *cliente
	g1, g2             string
}

func nuovoTavolo(t *testing.T) *tavolo {
	a := nuovoAmbiente(t)
	tv := &tavolo{a: a, m: a.master()}
	tv.g1 = tv.m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	tv.g2 = tv.m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Venerdì"})["id"].(string)
	tv.luca = a.giocatore("Luca", "333 1234567", "192.168.1.20")
	tv.sara = a.giocatore("Sara", "347 1112223", "192.168.1.21")
	tv.ugo = a.giocatore("Ugo", "320 9998887", "192.168.1.22")
	for c, g := range map[*cliente]string{tv.luca: tv.g1, tv.sara: tv.g1, tv.ugo: tv.g2} {
		tv.m.deve(200, "POST", "/utenti/"+a.idUtente(c)+"/approva", map[string]any{"gruppi": []string{g}})
	}
	return tv
}

func (tv *tavolo) nuovoPG(c *cliente, gruppo string) string {
	return c.deve(200, "POST", "/personaggi", map[string]any{"gruppo": gruppo, "nome": "Arin"})["id"].(string)
}

func TestSchedePermessi(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	tv.luca.deve(400, "POST", "/personaggi", map[string]any{"gruppo": tv.g2, "nome": "X"}) // gruppo non suo
	tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)
	tv.m.deve(200, "GET", "/personaggi/"+pg, nil)
	// Né un compagno di gruppo né un giocatore di un altro gruppo possono leggere o modificare.
	for _, altro := range []*cliente{tv.sara, tv.ugo} {
		altro.deve(404, "GET", "/personaggi/"+pg, nil)
		altro.deve(404, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"cls": "mago"}})
		altro.deve(404, "GET", "/personaggi/"+pg+"/versioni", nil)
		altro.deve(404, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{"pf": 1})
		if l := altro.deve(200, "GET", "/personaggi", nil)["lista"].([]any); len(l) != 0 {
			t.Fatal("l'elenco di un altro giocatore non deve contenere schede altrui")
		}
	}
	if l := tv.m.deve(200, "GET", "/personaggi", nil)["lista"].([]any); len(l) != 1 {
		t.Fatal("il Master deve vedere tutte le schede")
	}
}

func TestSchedeUnioneEConflitti(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	riep := map[string]any{"name": "Arin", "ac": 15, "hpMax": 12, "initiative": 2, "dex": 14, "level": 1, "passivePerception": 12}
	r := tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1,
		"modifiche": map[string]any{"cls": "guerriero", "base.FOR": 16, "base.DES": 14}, "riepilogo": riep})
	v := int(r["versione"].(float64))

	// Il Master e il giocatore partono dalla stessa versione e cambiano campi diversi: nessun conflitto.
	tv.m.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": v, "modifiche": map[string]any{"base.FOR": 17}})
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": v, "modifiche": map[string]any{"base.DES": 15}})
	d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)
	dati := d["dati"].(map[string]any)["base"].(map[string]any)
	if dati["FOR"] != float64(17) || dati["DES"] != float64(15) {
		t.Fatalf("le modifiche dei due devono essere unite: %v", dati)
	}
	ultima := int(d["personaggio"].(map[string]any)["versione"].(float64))

	// Ora entrambi cambiano lo stesso campo partendo dalla stessa versione: il secondo riceve un conflitto.
	tv.m.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": ultima, "modifiche": map[string]any{"cls": "paladino"}})
	code, out := tv.luca.chiama("PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": ultima, "modifiche": map[string]any{"cls": "ranger"}})
	if code != 409 || len(out["conflitti"].([]any)) != 1 {
		t.Fatalf("atteso conflitto, ottenuto %d %v", code, out)
	}
	c := out["conflitti"].([]any)[0].(map[string]any)
	if c["attuale"] != "paladino" || c["tuo"] != "ranger" || c["autore"] != "Master" {
		t.Fatalf("dettagli del conflitto: %v", c)
	}
	// Il giocatore sceglie di tenere la sua versione.
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": ultima, "modifiche": map[string]any{"cls": "ranger"}, "forza": []string{"cls"}})
	vers := tv.luca.deve(200, "GET", "/personaggi/"+pg+"/versioni", nil)["lista"].([]any)
	autori := map[string]bool{}
	for _, x := range vers {
		autori[x.(map[string]any)["autore"].(string)] = true
	}
	if !autori["Master"] || !autori["Luca"] {
		t.Fatalf("lo storico deve riportare entrambi gli autori: %v", vers)
	}
	// Nessun dato perso: la versione del Master è ancora nello storico e si può ripristinare.
	tv.luca.deve(200, "POST", "/personaggi/"+pg+"/ripristina/"+itoa(int(vers[1].(map[string]any)["numero"].(float64))), map[string]any{})
	d = tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)
	if d["dati"].(map[string]any)["cls"] != "paladino" {
		t.Fatalf("ripristino non riuscito: %v", d["dati"])
	}
}

func TestSchedeCampiNonAmmessi(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	tv.luca.deve(400, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"a.b.c": 1}})
	// I PF inviati con la scheda vanno nello stato in gioco e non creano versioni.
	if v := tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"hpCur": 3}}); v["versione"].(float64) != 1 {
		t.Fatalf("i PF non devono creare versioni: %v", v)
	}
	tv.luca.deve(400, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{"condizioni": []string{"<script>"}})
	tv.luca.deve(400, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{"tsMorte": map[string]int{"s": 5, "f": 0}})
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/stato-in-gioco", map[string]any{"pf": 7, "pfTemp": 3, "condizioni": []string{"Prono"}})
}

func TestPersonaggioRitiratoSolaLettura(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	tv.luca.deve(200, "POST", "/personaggi/"+pg+"/stato", map[string]any{"stato": "morto"})
	tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)
	tv.luca.deve(403, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"cls": "mago"}})
	tv.luca.deve(403, "POST", "/personaggi/"+pg+"/stato", map[string]any{"stato": "attivo"}) // solo il Master riattiva
	tv.m.deve(200, "POST", "/personaggi/"+pg+"/stato", map[string]any{"stato": "attivo"})
}

// carica invia un file binario come fa la pagina.
func (c *cliente) carica(percorso string, corpo []byte, nome string) (int, string) {
	req, _ := http.NewRequest("PUT", c.a.srv.URL+"/api/v1"+percorso, bytes.NewReader(corpo))
	req.Header.Set("X-CSRF", c.csrf)
	req.Header.Set("X-Nome-File", nome)
	req.Header.Set("X-Test-IP", c.ip)
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.c.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (c *cliente) scarica(percorso string) (int, []byte) {
	req, _ := http.NewRequest("GET", c.a.srv.URL+"/api/v1"+percorso, nil)
	req.Header.Set("X-Test-IP", c.ip)
	resp, err := c.c.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func docxFinto() []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, n := range []string{"[Content_Types].xml", "word/document.xml"} {
		w, _ := z.Create(n)
		w.Write([]byte("<x/>"))
	}
	z.Close()
	return b.Bytes()
}

func TestBackgroundPrivato(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	pdf := []byte("%PDF-1.4\n% Il passato di Arin a Pietralta\n%%EOF")

	// File falsi, troppo grandi o di altri tipi: rifiutati.
	if c, _ := tv.luca.carica("/personaggi/"+pg+"/background", []byte("MZ eseguibile travestito"), "storia.pdf"); c != 415 {
		t.Fatalf("un file falso deve essere rifiutato, codice %d", c)
	}
	if c, _ := tv.luca.carica("/personaggi/"+pg+"/background", append([]byte("%PDF-"), make([]byte, maxBackground+10)...), "grande.pdf"); c != 413 {
		t.Fatalf("un file troppo grande deve essere rifiutato, codice %d", c)
	}
	if c, _ := tv.luca.carica("/personaggi/"+pg+"/background", []byte("PK\x03\x04non-un-docx"), "storia.docx"); c != 415 {
		t.Fatalf("uno zip qualsiasi non è un docx, codice %d", c)
	}
	// Nome con percorso manipolato: salvato con un nome generato dal server.
	if c, b := tv.luca.carica("/personaggi/"+pg+"/background", pdf, "..%2F..%2Fwindows%2Fsystem32%2Fstoria.pdf"); c != 200 || !strings.Contains(b, `"nome":"storia.pdf"`) {
		t.Fatalf("caricamento valido non riuscito: %d %s", c, b)
	}
	// Su disco è cifrato.
	var file string
	tv.a.s.DB.QueryRow(`SELECT file FROM background WHERE personaggio_id = ?`, pg).Scan(&file)
	grezzo, _ := os.ReadFile(filepath.Join(tv.a.s.Cfg.CartellaDati, "file", file))
	if bytes.Contains(grezzo, []byte("Pietralta")) {
		t.Fatal("il background deve essere cifrato su disco")
	}
	// Il proprietario e il Master lo leggono; un compagno di gruppo e un altro giocatore ricevono un rifiuto.
	if c, b := tv.luca.scarica("/personaggi/" + pg + "/background"); c != 200 || !bytes.Equal(b, pdf) {
		t.Fatalf("il proprietario deve poter leggere: %d", c)
	}
	if c, b := tv.m.scarica("/personaggi/" + pg + "/background"); c != 200 || !bytes.Equal(b, pdf) {
		t.Fatalf("il Master deve poter leggere: %d", c)
	}
	for _, altro := range []*cliente{tv.sara, tv.ugo} {
		if c, _ := altro.scarica("/personaggi/" + pg + "/background"); c != 404 {
			t.Fatalf("un altro giocatore deve ricevere un rifiuto, codice %d", c)
		}
		if c, _ := altro.carica("/personaggi/"+pg+"/background", pdf, "x.pdf"); c != 404 {
			t.Fatalf("un altro giocatore non deve poter sostituire il background, codice %d", c)
		}
	}
	tv.sara.deve(403, "GET", "/background", nil)
	if l := tv.m.deve(200, "GET", "/background", nil)["lista"].([]any); len(l) != 1 {
		t.Fatal("il Master deve vedere l'elenco dei background")
	}
	// Un docx valido sostituisce il PDF.
	if c, _ := tv.luca.carica("/personaggi/"+pg+"/background", docxFinto(), "storia.docx"); c != 200 {
		t.Fatalf("docx valido rifiutato: %d", c)
	}
}
