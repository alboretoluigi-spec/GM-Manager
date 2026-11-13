package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// zipDi: un archivio in memoria (EPUB e DOCX sono archivi zip).
func zipDi(t *testing.T, file map[string]string, ordine ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, n := range ordine {
		w, err := z.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(file[n]))
	}
	z.Close()
	return b.Bytes()
}

// testo inventato per le prove: niente testi veri dei libri nel repository
func paragrafiProva(argomento string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "<p>Paragrafo %d su %s: la nebbia copriva la valle e le campane tacevano da giorni, mentre i corvi osservavano dal campanile.</p>\n", i+1, argomento)
	}
	return b.String()
}

func epubProva(t *testing.T) []byte {
	f := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Memorie di prova</dc:title></metadata>
<manifest><item id="c1" href="testo/cap1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="testo/cap%202.xhtml" media-type="application/xhtml+xml"/><item id="img" href="img.jpg" media-type="image/jpeg"/></manifest>
<spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`,
		"OEBPS/testo/cap1.xhtml": `<html><head><title>x</title><style>p{color:red}</style></head><body><h1>Capitolo Uno</h1><h2>Il patto</h2>` +
			paragrafiProva("il castello", 3) + `<p>Il cavallo nero si chiamava Fulmine d&#39;Ombra &amp; correva veloce.</p><script>alert(1)</script></body></html>`,
		"OEBPS/testo/cap 2.xhtml": `<html><head><title>Capitolo Due</title></head><body>` + paragrafiProva("il villaggio", 2) + `</body></html>`,
	}
	return zipDi(t, f, "mimetype", "META-INF/container.xml", "OEBPS/content.opf", "OEBPS/testo/cap1.xhtml", "OEBPS/testo/cap 2.xhtml")
}

func TestLetturaEPUB(t *testing.T) {
	parti, titolo, err := testoDaFile("libro.EPUB", epubProva(t))
	if err != nil || titolo != "Memorie di prova" {
		t.Fatalf("epub: %v %q", err, titolo)
	}
	if len(parti) != 2 || parti[0].E != "Capitolo Uno · Il patto" || parti[1].E != "Capitolo Due" {
		t.Fatalf("capitoli: %+v", parti)
	}
	if !strings.Contains(parti[0].T, "Fulmine d'Ombra & correva") || strings.Contains(parti[0].T, "alert") || strings.Contains(parti[0].T, "color") {
		t.Fatalf("testo del capitolo: %q", parti[0].T)
	}
	if strings.Count(parti[0].T, "\n\n") != 3 {
		t.Fatalf("paragrafi separati: %q", parti[0].T)
	}
	if _, _, err := testoDaFile("x.epub", []byte("non è uno zip")); err != errFormatoBib {
		t.Fatal("epub rotto")
	}
}

func TestLetturaDOCX(t *testing.T) {
	doc := `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:pPr><w:pStyle w:val="Titolo1"/></w:pPr><w:r><w:t>La famiglia Varnhold</w:t></w:r></w:p>
<w:p><w:r><w:t xml:space="preserve">Teodor era il fratello </w:t></w:r><w:r><w:t>minore.</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr><w:r><w:t>Il castello</w:t></w:r></w:p>
<w:p><w:r><w:t>Fu costruito</w:t><w:tab/><w:t>in molti anni.</w:t></w:r></w:p>
</w:body></w:document>`
	parti, _, err := testoDaFile("compendio.docx", zipDi(t, map[string]string{"word/document.xml": doc}, "word/document.xml"))
	if err != nil || len(parti) != 2 || parti[0].E != "La famiglia Varnhold" || parti[0].T != "Teodor era il fratello minore." || parti[1].T != "Fu costruito in molti anni." {
		t.Fatalf("docx: %v %+v", err, parti)
	}
}

func TestLetturaTXT(t *testing.T) {
	txt := "PROLOGO\n\nPrima riga del prologo,\nche continua qui con una pa-\nrola spezzata.\n\nSecondo paragrafo.\n\nCapitolo 1\n\nIl primo capitolo comincia.\n\n12\n\nUn capitolo con il solo numero.\n"
	// in Windows-1252: «è» = 0xE8
	parti, _, err := testoDaFile("romanzo.txt", []byte(txt))
	if err != nil || len(parti) != 3 || parti[0].E != "PROLOGO" || parti[1].E != "Capitolo 1" || parti[2].E != "cap. 12" {
		t.Fatalf("txt: %v %+v", err, parti)
	}
	if parti[0].T != "Prima riga del prologo, che continua qui con una parola spezzata.\n\nSecondo paragrafo." {
		t.Fatalf("paragrafi e sillabe: %q", parti[0].T)
	}
	p, _, _ := testoDaFile("vecchio.txt", []byte("Perch\xe8 la nebbia non se ne va? \x93Il Conte\x94 lo sa."))
	if len(p) != 1 || p[0].T != "Perchè la nebbia non se ne va? “Il Conte” lo sa." || p[0].E != "" {
		t.Fatalf("windows-1252: %+v", p)
	}
	md, _, _ := testoDaFile("note.md", []byte("# GasterBless\n\nUna valle.\n\n## Borgofosco\n\nUna cittadina."))
	if len(md) != 2 || md[1].E != "Borgofosco" {
		t.Fatalf("markdown: %+v", md)
	}
}

func TestPagineECapitoli(t *testing.T) {
	parti := testoDaPagine([]string{"Introduzione al modulo.", "", "CAPITOLO 4: CASTELLO DI CORVONERO Il castello domina la valle.", "Segue la descrizione delle sale."})
	if len(parti) != 3 || parti[0].E != "p. 1" || parti[1].E != "p. 3 · Capitolo 4: Castello di Corvonero" || parti[2].E != "p. 4 · Capitolo 4: Castello di Corvonero" || parti[2].P != 4 {
		t.Fatalf("pagine: %+v", parti)
	}
	f := fileOCRCompatto{Pagine: []paginaOCR{{P: 2, L: []rigaOCR{
		{72.0, 790.0, 280.0, 798.0, "Arabelle è una ragazza viandante che sogna un gran-"},
		{64.0, 780.0, 280.0, 788.0, "de destino lontano da qui."},
		{72.0, 760.0, 280.0, 768.0, "Nuovo paragrafo con rientro."},
		{64.0, 20.0, 80.0, 30.0, "2"}, // numero di pagina
	}}}}
	o := testoDaOCR(f)
	if len(o) != 1 || o[0].T != "Arabelle è una ragazza viandante che sogna un grande destino lontano da qui.\n\nNuovo paragrafo con rientro." || o[0].P != 2 {
		t.Fatalf("ocr: %+v", o)
	}
}

func TestBraniEParole(t *testing.T) {
	lungo := strings.Repeat("Una frase di prova che parla della valle e del castello. ", 90) // ~5.000 caratteri, un paragrafo
	pz := pezziDi([]parteTesto{{E: "cap. 1", T: lungo + "\n\nBreve."}, {E: "cap. 2", T: "Altro."}})
	if len(pz) < 4 {
		t.Fatalf("un paragrafo lungo si divide: %d brani", len(pz))
	}
	for _, p := range pz {
		if len(p.Testo) > pezzoMax+20 {
			t.Fatalf("brano troppo lungo: %d", len(p.Testo))
		}
	}
	if pz[len(pz)-1].Parte != 1 || pz[len(pz)-1].Testo != "Altro." {
		t.Fatal("un brano non passa da un capitolo all'altro")
	}
	for _, c := range [][2]string{{"fratello", "fratelli"}, {"sorella", "sorelle"}, {"vampiro", "vampiri"}, {"Città", "città"}, {"uccisero", "uccisero"}, {"nazione", "nazioni"}} {
		a, b := paroleBib(c[0], nil), paroleBib(c[1], nil)
		if len(a) != 1 || len(b) != 1 || a[0] != b[0] {
			t.Fatalf("radice di %q e %q: %v %v", c[0], c[1], a, b)
		}
	}
	if w := paroleBib("Dell'abbazia di Pietralta, nel 735 a Valdoria", nil); strings.Join(w, " ") != "abbazi pietralt 735 valdori" {
		t.Fatalf("parole: %v", w)
	}
}

func TestRicercaBM25(t *testing.T) {
	var ix indiceTesti
	ix.aggiungi(0, []pezzoBib{
		{Testo: "Il villaggio è avvolto dalla nebbia e i contadini restano chiusi in casa."},
		{Testo: "Il destriero nero dell'antagonista si chiama Fulmine d'Ombra e vive nelle stalle del castello."},
		{Testo: "l'antagonista cammina sui bastioni del castello, l'antagonista osserva, l'antagonista attende."},
	})
	ix.aggiungi(1, []pezzoBib{{Testo: "Nella campagna del Master il destriero dell'antagonista è morto in battaglia."}})
	q := domandaBib("Come si chiama il destriero dell'antagonista?", 1, nil)
	r := ix.cerca(q, 5, func(f int) bool { return f == 0 })
	if len(r) == 0 || ix.pezzi[r[0].i].pezzo != 1 {
		t.Fatalf("il brano giusto per primo: %+v", r)
	}
	for _, x := range r {
		if ix.pezzi[x.i].fonte != 0 {
			t.Fatal("solo le fonti ammesse")
		}
	}
	if r[0].copertura < 0.9 {
		t.Fatalf("copertura del brano giusto: %.2f", r[0].copertura)
	}
	r = ix.cerca(domandaBib("Chi è il pizzaiolo di Borgofosco?", 1, nil), 5, func(int) bool { return true })
	if len(r) != 0 {
		t.Fatalf("parole che non ci sono: nessun brano, non %+v", r)
	}
}
