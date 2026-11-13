package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func docxConTesto(t string) []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, _ := z.Create("[Content_Types].xml")
	w.Write([]byte("<Types/>"))
	w, _ = z.Create("word/document.xml")
	w.Write([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>` + t + `</w:t></w:r></w:p></w:body></w:document>`))
	z.Close()
	return b.Bytes()
}

// MY RULEBOOKS, passo L3: importazione di tutti i formati con i controlli (danneggiati, doppioni, non supportati),
// testo per la ricerca (anche dei PDF, mandato dal browser), modifica, categorie, versioni, sostituzione, lettura,
// eliminazione, solo il Master, dati che restano dopo il riavvio.
func TestMieiRegolamenti(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	carica := func(nome string, corpo []byte, extra url.Values) (int, map[string]any) {
		t.Helper()
		q := url.Values{"file": {nome}}
		for k, v := range extra {
			q[k] = v
		}
		cod, _, b := m.grezzo("POST", "/regolamenti/miei?"+q.Encode(), corpo, "application/octet-stream")
		var out map[string]any
		json.Unmarshal(b, &out)
		return cod, out
	}
	deve := func(nome string, corpo []byte, extra url.Values) map[string]any {
		t.Helper()
		cod, out := carica(nome, corpo, extra)
		if cod != 200 {
			t.Fatalf("%s: %d %v", nome, cod, out)
		}
		return out
	}
	pdf := []byte("%PDF-1.7\n1 0 obj << >> endobj\ntrailer << >>\n%%EOF\n")
	rtf := []byte(`{\rtf1\ansi{\fonttbl{\f0 Arial;}}{\*\generator Prova;}\f0 Perch\'e8 il drago dorme?\par Seconda riga \u8364? fine.}`)
	txt := deve("Manuale base.txt", []byte("Capitolo 1\n\nIl tiro di iniziativa si fa all'inizio dello scontro.\n"), url.Values{"gioco": {"Gioco A"}, "edizione": {"1"}})
	if txt["titolo"] != "Manuale base" || txt["formato"] != "txt" || txt["origine"] != "importato" || txt["tipologia"] != "personale" || txt["testo"] != true || txt["gioco"] != "Gioco A" {
		t.Fatalf("txt: %v", txt)
	}
	deve("note.md", []byte("# Regole della casa\n\nIl riposo breve dura un'ora."), nil)
	deve("pagina.html", []byte("<html><body><h1>Armi</h1><p>La spada lunga fa danni taglienti.</p></body></html>"), nil)
	r := deve("antico.rtf", rtf, url.Values{"titolo": {"Documento RTF"}})
	if r["testo"] != true {
		t.Fatalf("rtf: %v", r)
	}
	docx := deve("regole.docx", docxConTesto("Regola del vantaggio: tira due dadi e tieni il migliore."), url.Values{"tipologia": {"regolamento"}})
	deve("memorie.epub", epubProva(t), nil)
	p := deve("manuale.pdf", pdf, url.Values{"gioco": {"Gioco A"}, "tipologia": {"srd"}})
	if p["testo"] != false {
		t.Fatalf("pdf: il testo arriva dopo dal browser: %v", p)
	}
	m.deve(200, "POST", "/regolamenti/miei/"+p["id"].(string)+"/testo", map[string]any{"pagine": []string{"Pagina uno: creazione del personaggio.", "Pagina due: la magia rituale."}})
	m.deve(400, "POST", "/regolamenti/miei/"+txt["id"].(string)+"/testo", map[string]any{"pagine": []string{"x"}})

	// file sbagliati
	casi := []struct {
		nome   string
		corpo  []byte
		codice int
		testo  string
	}{
		{"rotto.pdf", []byte("%PDF-1.7\nmanca la fine"), 400, "danneggiato"},
		{"finto.pdf", []byte("<html>non sono un pdf</html>"), 400, "danneggiato"},
		{"senza.docx", docxFinto()[:30], 400, "danneggiato"},
		{"vuoto.docx", func() []byte { var b bytes.Buffer; z := zip.NewWriter(&b); w, _ := z.Create("altro.xml"); w.Write([]byte("x")); z.Close(); return b.Bytes() }(), 400, "danneggiato"},
		{"rotto.rtf", []byte(`{\rtf1 testo senza chiusura`), 400, "danneggiato"},
		{"falso.epub", []byte("non è uno zip"), 400, "danneggiato"},
		{"binario.txt", []byte("testo\x00\x01\x02 binario"), 400, "non è un testo"},
		{"programma.exe", []byte("MZ"), 415, "non supportato"},
		{"vuoto.txt", []byte{}, 400, "vuoto"},
		{"Manuale base copia.txt", []byte("Capitolo 1\n\nIl tiro di iniziativa si fa all'inizio dello scontro.\n"), 409, "già nella libreria"},
	}
	for _, c := range casi {
		cod, out := carica(c.nome, c.corpo, nil)
		if cod != c.codice || !strings.Contains(out["errore"].(string), c.testo) {
			t.Fatalf("%s: %d %v", c.nome, cod, out)
		}
	}
	elenco := func() []any { return m.deve(200, "GET", "/regolamenti/miei", nil)["documenti"].([]any) }
	if n := len(elenco()); n != 7 {
		t.Fatalf("documenti nella libreria: %d", n)
	}
	if fs, _ := os.ReadDir(filepath.Join(a.s.Cfg.CartellaDati, "regolamenti", "file")); len(fs) != 7 {
		t.Fatalf("file sul disco: %d (i file rifiutati non devono restare)", len(fs))
	}

	// ricerca nel titolo e nel contenuto, senza accenti e maiuscole
	cerca := func(q string) []any {
		t.Helper()
		return m.deve(200, "GET", "/regolamenti/miei/cerca?q="+url.QueryEscape(q), nil)["risultati"].([]any)
	}
	if r := cerca("VANTAGGIO"); len(r) != 1 || r[0].(map[string]any)["doc"] != docx["id"] || !strings.Contains(r[0].(map[string]any)["estratto"].(string), "vantaggio") {
		t.Fatalf("ricerca nel DOCX: %v", r)
	}
	if r := cerca("perche drago"); len(r) != 1 || r[0].(map[string]any)["doc"] != r[0].(map[string]any)["doc"] || !strings.Contains(r[0].(map[string]any)["estratto"].(string), "Perchè il drago") {
		t.Fatalf("ricerca nell'RTF senza accenti: %v", r)
	}
	if r := cerca("magia rituale"); len(r) != 1 || r[0].(map[string]any)["pagina"] != 2.0 {
		t.Fatalf("ricerca nel testo del PDF con la pagina: %v", r)
	}
	if r := cerca("manuale base"); len(r) == 0 || r[0].(map[string]any)["nelTitolo"] != true {
		t.Fatalf("ricerca nel titolo: %v", r)
	}
	m.deve(400, "GET", "/regolamenti/miei/cerca?q=", nil)

	// categorie
	cat := m.deve(200, "POST", "/regolamenti/categorie", map[string]any{"nome": "Regole base"})
	m.deve(409, "POST", "/regolamenti/categorie", map[string]any{"nome": "Regole base"})
	m.deve(400, "POST", "/regolamenti/categorie", map[string]any{"nome": " "})
	m.deve(200, "PATCH", "/regolamenti/miei/"+txt["id"].(string), map[string]any{"categoria": cat["id"], "lingua": "it", "note": "copia del Master"})
	m.deve(400, "PATCH", "/regolamenti/miei/"+txt["id"].(string), map[string]any{"categoria": "inesistente"})
	m.deve(400, "PATCH", "/regolamenti/miei/"+txt["id"].(string), map[string]any{"tipologia": "boh"})
	m.deve(400, "PATCH", "/regolamenti/miei/"+txt["id"].(string), map[string]any{"titolo": ""})
	cs := m.deve(200, "GET", "/regolamenti/categorie", nil)["categorie"].([]any)
	if len(cs) != 1 || cs[0].(map[string]any)["documenti"] != 1.0 {
		t.Fatalf("categorie: %v", cs)
	}
	m.deve(200, "PATCH", "/regolamenti/categorie/"+cat["id"].(string), map[string]any{"nome": "Regole principali"})

	// versioni dello stesso regolamento (un livello)
	b := m.deve(200, "PATCH", "/regolamenti/miei/"+p["id"].(string), map[string]any{"versioneDi": txt["id"], "versione": "2ª stampa"})
	if b["versioneDi"] != txt["id"] || b["versione"] != "2ª stampa" || b["gioco"] != "Gioco A" {
		t.Fatalf("versione: %v", b)
	}
	m.deve(400, "PATCH", "/regolamenti/miei/"+txt["id"].(string), map[string]any{"versioneDi": docx["id"]})  // ha già versioni
	m.deve(400, "PATCH", "/regolamenti/miei/"+docx["id"].(string), map[string]any{"versioneDi": p["id"]})  // p è una versione
	m.deve(400, "PATCH", "/regolamenti/miei/"+docx["id"].(string), map[string]any{"versioneDi": docx["id"]}) // sé stesso

	// sostituzione del file: nuovi contenuti e nuovo testo, stessi dati
	cod, _, sb := m.grezzo("PUT", "/regolamenti/miei/"+txt["id"].(string)+"/file?file=nuovo.txt", []byte("Edizione rivista: il tiro di sorpresa cambia."), "application/octet-stream")
	if cod != 200 || !strings.Contains(string(sb), "Manuale base") {
		t.Fatalf("sostituzione: %d %s", cod, sb)
	}
	if len(cerca("sorpresa")) != 1 || len(cerca("iniziativa")) != 0 {
		t.Fatal("dopo la sostituzione la ricerca deve usare il testo nuovo")
	}
	cod, _, _ = m.grezzo("PUT", "/regolamenti/miei/"+txt["id"].(string)+"/file?file=x.rtf", rtf, "application/octet-stream")
	if cod != 409 {
		t.Fatalf("sostituzione con un file già presente: %d", cod)
	}
	// lettura a blocchi
	tx := m.deve(200, "GET", "/regolamenti/miei/"+docx["id"].(string)+"/testo?da=0&n=5", nil)
	if tx["totale"].(float64) < 1 || len(tx["parti"].([]any)) < 1 {
		t.Fatalf("testo: %v", tx)
	}
	// eliminazione: le versioni restano, senza principale; il file sparisce
	m.deve(200, "DELETE", "/regolamenti/miei/"+txt["id"].(string), nil)
	m.deve(404, "DELETE", "/regolamenti/miei/"+txt["id"].(string), nil)
	for _, d := range elenco() {
		if d.(map[string]any)["id"] == p["id"] && d.(map[string]any)["versioneDi"] != nil {
			t.Fatalf("versione rimasta collegata a un documento eliminato: %v", d)
		}
	}
	if fs, _ := os.ReadDir(filepath.Join(a.s.Cfg.CartellaDati, "regolamenti", "file")); len(fs) != 6 {
		t.Fatalf("file dopo l'eliminazione: %d", len(fs))
	}
	m.deve(200, "DELETE", "/regolamenti/categorie/"+cat["id"].(string), nil)

	g := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	g.deve(403, "GET", "/regolamenti/miei", nil)
	g.deve(403, "GET", "/regolamenti/miei/cerca?q=drago", nil)
	if c, _, _ := g.grezzo("POST", "/regolamenti/miei?file=x.txt", []byte("ciao"), "application/octet-stream"); c != 403 {
		t.Fatalf("giocatore: importazione %d", c)
	}

	// riavvio
	cfg := a.s.Cfg
	a.s.Chiudi()
	s2, err := Nuovo(cfg, fstest.MapFS{"index.html": {Data: []byte("x")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Chiudi()
	var n int
	s2.DB.QueryRow(`SELECT COUNT(*) FROM regolamenti_documenti`).Scan(&n)
	if n != 6 {
		t.Fatalf("dopo il riavvio: %d documenti", n)
	}
}

func TestLettoreRTF(t *testing.T) {
	s, err := testoDaRTF([]byte(`{\rtf1\ansi\uc1{\fonttbl{\f0\fswiss Arial;}}{\colortbl;\red0\green0\blue0;}{\info{\title Titolo nascosto}}\pard Prima riga\par Citt\'e0 \u8220?bella\u8221? \{graffe\}\tab fine{\*\ignorami testo}}`))
	if err != nil || s != "Prima riga\nCittà “bella” {graffe}\tfine" {
		t.Fatalf("rtf: %q %v", s, err)
	}
	for _, sbagliato := range []string{"testo semplice", `{\rtf1 aperto`, `{\rtf1 chiuso}}`} {
		if _, err := testoDaRTF([]byte(sbagliato)); err == nil {
			t.Fatalf("rtf sbagliato accettato: %q", sbagliato)
		}
	}
}
