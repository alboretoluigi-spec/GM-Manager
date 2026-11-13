package server

// Biblioteca dell'assistente IA (2026-11-13): lettura dei testi e ricerca. Il Master carica le sue copie dei testi
// (manuale dell'avventura, romanzi, compendi, Tomo interattivo, documenti della sua campagna); Gaster Bless li divide in
// brani e, a ogni domanda, trova quelli pertinenti da dare all'IA insieme alla domanda (biblioteca.go). Qui solo la
// parte che non dipende dal server: lettura dei formati (EPUB, DOCX, TXT/MD, pagine di un PDF, testo OCR), divisione
// in parti (capitoli o pagine) e brani, parole normalizzate per l'italiano e ricerca BM25 con un premio per le parole
// vicine. I testi non stanno mai nel codice né nel repository: restano nella cartella dei dati del PC del Master.

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"math"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// parteTesto: un capitolo (libri) o una pagina (PDF). E dice dove si trova («cap. 3 · Il ritorno», «p. 45 · Capitolo 4»).
type parteTesto struct {
	E string `json:"e"`
	T string `json:"t"`
	P int    `json:"p,omitempty"` // pagina del PDF, per aprire il manuale nel punto giusto
}

const (
	pezzoMin = 700  // caratteri: sotto, il brano si unisce al paragrafo seguente
	pezzoMax = 1500 // sopra, il paragrafo si divide tra una frase e l'altra
)

var errFormatoBib = errors.New("formato non riconosciuto")

// ===== Lettura dei formati =====

// testoDaFile sceglie il lettore dall'estensione del nome del file.
func testoDaFile(nome string, b []byte) (parti []parteTesto, titolo string, err error) {
	switch strings.ToLower(path.Ext(nome)) {
	case ".epub":
		return testoDaEPUB(b)
	case ".docx":
		p, e := testoDaDOCX(b)
		return p, "", e
	case ".txt", ".md", ".text", ".markdown":
		return partiDaRighe(strings.Split(decodificaTesto(b), "\n")), "", nil
	case ".htm", ".html", ".xhtml":
		return partiDaBlocchi(blocchiHTML(decodificaTesto(b))), "", nil
	}
	return nil, "", errFormatoBib
}

// decodificaTesto: UTF-8 (senza BOM) o, se non lo è, Windows-1252 (i vecchi file di testo italiani).
func decodificaTesto(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	var s string
	if utf8.Valid(b) {
		s = string(b)
	} else {
		var sb strings.Builder
		for _, c := range b {
			if c >= 0x80 && c <= 0x9F {
				sb.WriteRune(cp1252[c-0x80])
			} else {
				sb.WriteRune(rune(c))
			}
		}
		s = sb.String()
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

var cp1252 = [32]rune{'€', ' ', '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', ' ', 'Ž', ' ',
	' ', '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', ' ', 'ž', 'Ÿ'}

// blocco: un paragrafo o un titolo.
type blocco struct {
	titolo bool
	testo  string
}

var (
	reTagHTML   = regexp.MustCompile(`(?s)<(/?)([a-zA-Z][a-zA-Z0-9:]*)[^>]*>|<!--.*?-->`)
	reSpaziBib  = regexp.MustCompile(`[ \t\x{00A0}\x{2009}\x{202F}]+`)
	reTitoloEP  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reBloccoTag = map[string]bool{"p": true, "div": true, "br": true, "li": true, "tr": true, "blockquote": true, "section": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "dd": true, "dt": true, "pre": true, "table": true}
)

// blocchiHTML trasforma un capitolo XHTML in paragrafi e titoli (h1–h3), senza immagini, stili e script.
func blocchiHTML(s string) []blocco {
	if i := strings.Index(strings.ToLower(s), "<body"); i >= 0 {
		s = s[i:]
	}
	var out []blocco
	var cur strings.Builder
	titolo, salta := false, 0
	chiudi := func() {
		t := strings.TrimSpace(reSpaziBib.ReplaceAllString(html.UnescapeString(cur.String()), " "))
		cur.Reset()
		if t != "" {
			out = append(out, blocco{titolo: titolo, testo: t})
		}
	}
	pos := 0
	for _, m := range reTagHTML.FindAllStringSubmatchIndex(s, -1) {
		if salta == 0 {
			cur.WriteString(strings.ReplaceAll(s[pos:m[0]], "\n", " "))
		}
		pos = m[1]
		if m[4] < 0 { // commento
			continue
		}
		fine := m[3] > m[2]
		tag := strings.ToLower(s[m[4]:m[5]])
		if i := strings.IndexByte(tag, ':'); i >= 0 {
			tag = tag[i+1:]
		}
		switch tag {
		case "script", "style", "head", "svg":
			if fine {
				salta = max(0, salta-1)
			} else if !strings.HasSuffix(s[m[0]:m[1]], "/>") {
				salta++
			}
			continue
		}
		if salta > 0 || !reBloccoTag[tag] {
			continue
		}
		chiudi()
		if tag == "h1" || tag == "h2" || tag == "h3" {
			titolo = !fine
		} else if fine {
			titolo = false
		}
	}
	if salta == 0 && pos < len(s) {
		cur.WriteString(strings.ReplaceAll(s[pos:], "\n", " "))
	}
	chiudi()
	return out
}

// testoDaEPUB: i capitoli nell'ordine di lettura (spine), ognuno diviso ai suoi titoli.
func testoDaEPUB(b []byte) ([]parteTesto, string, error) {
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, "", errFormatoBib
	}
	leggi := func(nome string) ([]byte, bool) {
		for _, f := range z.File {
			if f.Name == nome || strings.EqualFold(f.Name, nome) {
				r, err := f.Open()
				if err != nil {
					return nil, false
				}
				defer r.Close()
				d, err := io.ReadAll(io.LimitReader(r, 32<<20))
				return d, err == nil
			}
		}
		return nil, false
	}
	cont, ok := leggi("META-INF/container.xml")
	if !ok {
		return nil, "", errFormatoBib
	}
	var c struct {
		Rootfiles []struct {
			Path string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if xml.Unmarshal(cont, &c) != nil || len(c.Rootfiles) == 0 {
		return nil, "", errFormatoBib
	}
	opfNome := c.Rootfiles[0].Path
	opf, ok := leggi(opfNome)
	if !ok {
		return nil, "", errFormatoBib
	}
	var o struct {
		Titolo   string `xml:"metadata>title"`
		Manifest []struct {
			ID   string `xml:"id,attr"`
			Href string `xml:"href,attr"`
			Tipo string `xml:"media-type,attr"`
		} `xml:"manifest>item"`
		Spine []struct {
			Rif string `xml:"idref,attr"`
		} `xml:"spine>itemref"`
	}
	if xml.Unmarshal(opf, &o) != nil {
		return nil, "", errFormatoBib
	}
	base := path.Dir(opfNome)
	href := map[string]string{}
	for _, it := range o.Manifest {
		if strings.Contains(it.Tipo, "html") {
			h := it.Href
			if i := strings.IndexAny(h, "#?"); i >= 0 {
				h = h[:i]
			}
			if p, err := urlDecodifica(h); err == nil {
				h = p
			}
			if base != "." {
				h = path.Join(base, h)
			}
			href[it.ID] = h
		}
	}
	var blocchi []blocco
	for _, sp := range o.Spine {
		h, ok := href[sp.Rif]
		if !ok {
			continue
		}
		d, ok := leggi(h)
		if !ok {
			continue
		}
		bl := blocchiHTML(decodificaTesto(d))
		if len(bl) > 0 && !bl[0].titolo {
			// capitolo senza titolo h1–h3: si usa il <title> del file, se è diverso dal titolo del libro
			if m := reTitoloEP.FindSubmatch(d); m != nil {
				t := strings.TrimSpace(html.UnescapeString(string(m[1])))
				if t != "" && !strings.EqualFold(t, strings.TrimSpace(o.Titolo)) && utf8.RuneCountInString(t) <= 80 {
					bl = append([]blocco{{titolo: true, testo: t}}, bl...)
				}
			}
		}
		blocchi = append(blocchi, bl...)
	}
	parti := partiDaBlocchi(blocchi)
	if len(parti) == 0 {
		return nil, "", errors.New("nel libro non c'è testo leggibile (forse è protetto da DRM)")
	}
	return parti, strings.TrimSpace(o.Titolo), nil
}

func urlDecodifica(s string) (string, error) {
	if !strings.Contains(s, "%") {
		return s, nil
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			n, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
			if err != nil {
				return s, err
			}
			b.WriteByte(byte(n))
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String(), nil
}

// testoDaDOCX: i paragrafi di word/document.xml; gli stili «Titolo 1–3» / «Heading 1–3» diventano capitoli.
func testoDaDOCX(b []byte) ([]parteTesto, error) {
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, errFormatoBib
	}
	var doc []byte
	for _, f := range z.File {
		if f.Name == "word/document.xml" {
			r, err := f.Open()
			if err != nil {
				return nil, errFormatoBib
			}
			doc, err = io.ReadAll(io.LimitReader(r, 64<<20))
			r.Close()
			if err != nil {
				return nil, errFormatoBib
			}
		}
	}
	if doc == nil {
		return nil, errFormatoBib
	}
	d := xml.NewDecoder(bytes.NewReader(doc))
	var blocchi []blocco
	var cur strings.Builder
	titolo, inTesto := false, false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errFormatoBib
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				cur.Reset()
				titolo = false
			case "pStyle":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						v := strings.ToLower(a.Value)
						if strings.HasPrefix(v, "heading") || strings.HasPrefix(v, "titolo") || v == "title" {
							titolo = !strings.HasSuffix(v, "4") && !strings.HasSuffix(v, "5") && !strings.HasSuffix(v, "6")
						}
					}
				}
			case "t":
				inTesto = true
			case "tab":
				cur.WriteByte(' ')
			case "br":
				cur.WriteByte(' ')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inTesto = false
			case "p":
				if s := strings.TrimSpace(reSpaziBib.ReplaceAllString(cur.String(), " ")); s != "" {
					blocchi = append(blocchi, blocco{titolo: titolo, testo: s})
				}
				cur.Reset()
			}
		case xml.CharData:
			if inTesto {
				cur.Write(t)
			}
		}
	}
	parti := partiDaBlocchi(blocchi)
	if len(parti) == 0 {
		return nil, errors.New("nel documento non c'è testo")
	}
	return parti, nil
}

// ===== Capitoli =====

var (
	reCapitolo  = regexp.MustCompile(`(?i)^\s*(capitolo|cap\.|parte|libro|prologo|epilogo|interludio|appendice|chapter|part|book|prologue|epilogue|introduzione|prefazione|postfazione)\b`)
	reNumeroCap = regexp.MustCompile(`^\s*(?:[IVXLC]{1,7}|\d{1,3})\s*[.)]?\s*$`)
)

// pareTitolo: una riga di testo semplice che sembra il titolo di un capitolo.
func pareTitolo(r string) bool {
	r = strings.TrimSpace(r)
	n := utf8.RuneCountInString(r)
	if n == 0 || n > 70 {
		return false
	}
	if reCapitolo.MatchString(r) || reNumeroCap.MatchString(r) {
		return true
	}
	// tutta maiuscola, con almeno 4 lettere e senza punto finale
	lettere, maiusc := 0, 0
	for _, c := range r {
		if unicode.IsLetter(c) {
			lettere++
			if unicode.IsUpper(c) {
				maiusc++
			}
		}
	}
	return lettere >= 4 && maiusc == lettere && !strings.HasSuffix(r, ".")
}

// nomeCapitolo: come si cita il capitolo («cap. 12» per un titolo fatto solo di un numero).
func nomeCapitolo(t string) string {
	t = strings.Trim(strings.TrimSpace(t), "#*_ ")
	if reNumeroCap.MatchString(t) {
		return "cap. " + strings.TrimRight(strings.TrimSpace(t), ".)")
	}
	if utf8.RuneCountInString(t) > 80 {
		t = string([]rune(t)[:80]) + "…"
	}
	return t
}

// partiDaRighe: testo semplice; i paragrafi sono separati da righe vuote (o, se non ce ne sono, da ogni a capo).
// I titoli dei capitoli aprono una parte nuova; «# Titolo» (markdown) vale sempre come titolo.
func partiDaRighe(righe []string) []parteTesto {
	vuote := 0
	for _, r := range righe {
		if strings.TrimSpace(r) == "" {
			vuote++
		}
	}
	perRiga := vuote < len(righe)/20 // quasi nessuna riga vuota: ogni riga è un paragrafo
	var blocchi []blocco
	var cur []string
	chiudi := func() {
		if len(cur) > 0 {
			blocchi = append(blocchi, blocco{testo: strings.Join(cur, " ")})
			cur = nil
		}
	}
	for i, r := range righe {
		t := strings.TrimSpace(r)
		if t == "" {
			chiudi()
			continue
		}
		if strings.HasPrefix(t, "#") {
			chiudi()
			blocchi = append(blocchi, blocco{titolo: true, testo: strings.TrimLeft(t, "# ")})
			continue
		}
		// un titolo è una riga breve isolata (vuota prima e dopo, o in modalità per riga)
		isolata := perRiga || ((i == 0 || strings.TrimSpace(righe[i-1]) == "") && (i == len(righe)-1 || strings.TrimSpace(righe[i+1]) == ""))
		if isolata && pareTitolo(t) {
			chiudi()
			blocchi = append(blocchi, blocco{titolo: true, testo: t})
			continue
		}
		cur = append(cur, t) // le parole spezzate a fine riga si riuniscono dopo (uniscriSillabe)
		if perRiga {
			chiudi()
		}
	}
	chiudi()
	for i := range blocchi {
		blocchi[i].testo = uniscriSillabe(blocchi[i].testo)
	}
	return partiDaBlocchi(blocchi)
}

var reSillaba = regexp.MustCompile(`(\p{Ll})- (\p{Ll})`)

func uniscriSillabe(s string) string { return reSillaba.ReplaceAllString(s, "$1$2") }

// partiDaBlocchi: ogni titolo apre una parte; titoli consecutivi si uniscono («Parte prima · Capitolo 1»).
// Senza nessun titolo il libro diventa un'unica parte e i brani si citano per posizione (pezziDi).
func partiDaBlocchi(bl []blocco) []parteTesto {
	var out []parteTesto
	var cur strings.Builder
	etichetta, ultimoTitolo := "", false
	chiudi := func() {
		if t := strings.TrimSpace(cur.String()); t != "" {
			out = append(out, parteTesto{E: etichetta, T: t})
		}
		cur.Reset()
	}
	for _, b := range bl {
		if b.titolo {
			n := nomeCapitolo(b.testo)
			if n == "" {
				continue
			}
			if ultimoTitolo && cur.Len() == 0 {
				if utf8.RuneCountInString(etichetta)+utf8.RuneCountInString(n) < 110 {
					etichetta += " · " + n
				}
				continue
			}
			chiudi()
			etichetta, ultimoTitolo = n, true
			continue
		}
		ultimoTitolo = false
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(b.testo)
	}
	chiudi()
	return out
}

// testoDaPagine: le pagine di un PDF (testo estratto nel browser). Le righe «Capitolo N…» danno il nome al capitolo,
// che resta nelle pagine seguenti («p. 45 · Capitolo 4: Castello di Corvonero»).
func testoDaPagine(pagine []string) []parteTesto {
	var out []parteTesto
	cap := ""
	for i, p := range pagine {
		t := strings.TrimSpace(reSpaziBib.ReplaceAllString(strings.ReplaceAll(p, "\r", ""), " "))
		if t == "" {
			continue
		}
		if c := capitoloInPagina(t); c != "" {
			cap = c
		}
		out = append(out, parteTesto{E: etichettaPagina(i+1, cap), T: uniscriSillabe(t), P: i + 1})
	}
	return out
}

var reCapPagina = regexp.MustCompile(`(?i)\b(capitolo|appendice)\s+([0-9]{1,2}|[A-Z])\s*[:.–-]\s*([^\n.]{3,50})`)

func capitoloInPagina(t string) string {
	if len(t) > 400 {
		t = t[:400] // il titolo del capitolo sta in cima alla pagina
	}
	m := reCapPagina.FindStringSubmatch(t)
	if m == nil {
		return ""
	}
	nome := strings.TrimSpace(m[3])
	// titolo in maiuscolo seguito dal testo sulla stessa riga (pdf.js unisce le righe): solo le parole maiuscole
	if p := strings.Fields(nome); len(p) > 0 && strings.ToUpper(p[0]) == p[0] {
		k := 0
		for k < len(p) && strings.ToUpper(p[k]) == p[k] && strings.ToLower(p[k]) != p[k] {
			k++
		}
		if k >= 2 || k == len(p) {
			nome = strings.Join(p[:k], " ")
		}
	}
	// «CAPITOLO 4: CASTELLO DI CORVONERO» → «Capitolo 4: Castello di Corvonero»
	if strings.ToUpper(nome) == nome {
		nome = maiuscolaIniziali(strings.ToLower(nome))
	}
	return maiuscolaIniziali(strings.ToLower(m[1])) + " " + strings.ToUpper(m[2]) + ": " + nome
}

func maiuscolaIniziali(s string) string {
	piccole := map[string]bool{"di": true, "del": true, "della": true, "dei": true, "delle": true, "e": true, "il": true, "la": true, "lo": true, "le": true, "i": true, "gli": true, "a": true, "in": true, "da": true}
	p := strings.Fields(s)
	for i, w := range p {
		if i > 0 && piccole[w] {
			continue
		}
		r, n := utf8.DecodeRuneInString(w)
		p[i] = string(unicode.ToUpper(r)) + w[n:]
	}
	return strings.Join(p, " ")
}

func etichettaPagina(n int, cap string) string {
	if cap != "" {
		return "p. " + strconv.Itoa(n) + " · " + cap
	}
	return "p. " + strconv.Itoa(n)
}

// testoDaOCR: le righe del testo OCR di un manuale (già nell'ordine di lettura, colonna per colonna). Un rientro o una
// riga tutta maiuscola aprono un paragrafo nuovo; le parole spezzate a fine riga si riuniscono.
func testoDaOCR(f fileOCRCompatto) []parteTesto {
	pagine := map[int]string{}
	maxP := 0
	for _, p := range f.Pagine {
		var b strings.Builder
		x0prec, y0prec := -1.0, -1.0
		for _, l := range p.L {
			x0, _ := l[0].(float64)
			y0, _ := l[1].(float64)
			y1, _ := l[3].(float64)
			t, _ := l[4].(string)
			t = strings.TrimSpace(t)
			if t == "" || y1 < 45 || y0 > 805 { // numeri di pagina e testatine
				continue
			}
			nuovo := b.Len() == 0 || x0 > x0prec+5 || (y0prec >= 0 && y0prec-y0 > 18) || pareTitolo(t)
			switch {
			case b.Len() == 0:
			case nuovo:
				b.WriteString("\n\n")
			default:
				b.WriteByte(' ')
			}
			b.WriteString(t)
			x0prec, y0prec = x0, y0
		}
		pagine[p.P] = strings.ReplaceAll(b.String(), "- ", "-\u0000")
		maxP = max(maxP, p.P)
	}
	var out []parteTesto
	cap := ""
	for n := 1; n <= maxP; n++ {
		t := strings.TrimSpace(pagine[n])
		if t == "" {
			continue
		}
		t = reSillabaOCR.ReplaceAllString(t, "$1$2")
		t = strings.ReplaceAll(t, "-\u0000", "- ")
		if c := capitoloInPagina(t); c != "" {
			cap = c
		}
		out = append(out, parteTesto{E: etichettaPagina(n, cap), T: t, P: n})
	}
	return out
}

var reSillabaOCR = regexp.MustCompile(`(\p{Ll})-\x00(\p{Ll})`)

// ===== Brani =====

// pezzoBib: un brano da 700–1500 caratteri dentro una parte. Inizio: posizione in caratteri dall'inizio del libro
// (per citare «circa al 35%» i libri senza capitoli).
type pezzoBib struct {
	Parte  int
	Testo  string
	Inizio int
}

var reFineFrase = regexp.MustCompile(`[.!?…»”"]\s+`)

// pezziDi divide le parti in brani: paragrafi interi finché il brano è sotto pezzoMin; i paragrafi molto lunghi si
// dividono tra una frase e l'altra. Un brano non passa mai da una parte all'altra (la citazione resta giusta).
func pezziDi(parti []parteTesto) []pezzoBib {
	var out []pezzoBib
	pos := 0
	for i, p := range parti {
		var cur strings.Builder
		inizio := pos
		chiudi := func() {
			if t := strings.TrimSpace(cur.String()); t != "" {
				out = append(out, pezzoBib{Parte: i, Testo: t, Inizio: inizio})
			}
			cur.Reset()
		}
		for _, par := range strings.Split(p.T, "\n") {
			par = strings.TrimSpace(par)
			if par == "" {
				continue
			}
			for _, fr := range dividiLungo(par) {
				if cur.Len() > 0 && cur.Len()+len(fr) > pezzoMax {
					chiudi()
					inizio = pos
				}
				if cur.Len() > 0 {
					cur.WriteString("\n")
				}
				cur.WriteString(fr)
				pos += len(fr) + 1
				if cur.Len() >= pezzoMin && cur.Len() >= pezzoMax*2/3 {
					chiudi()
					inizio = pos
				}
			}
		}
		chiudi()
	}
	return out
}

// dividiLungo: un paragrafo più lungo di pezzoMax diventa più pezzi, tagliati alla fine di una frase.
func dividiLungo(par string) []string {
	if len(par) <= pezzoMax {
		return []string{par}
	}
	var out []string
	for len(par) > pezzoMax {
		taglio := -1
		for _, m := range reFineFrase.FindAllStringIndex(par[:pezzoMax], -1) {
			if m[1] >= pezzoMin/2 {
				taglio = m[1]
			}
		}
		if taglio < 0 {
			taglio = strings.LastIndexByte(par[:pezzoMax], ' ')
			if taglio <= 0 {
				taglio = pezzoMax
			}
			for taglio < len(par) && !utf8.RuneStart(par[taglio]) {
				taglio++
			}
		}
		out = append(out, strings.TrimSpace(par[:taglio]))
		par = strings.TrimSpace(par[taglio:])
	}
	if par != "" {
		out = append(out, par)
	}
	return out
}

// ===== Parole =====

var paroleVuote = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`a ad al allo ai agli all agl alla alle con col coi da dal dallo dai dagli dall dagl dalla dalle di del dello
	dei degli dell degl della delle in nel nello nei negli nell negl nella nelle su sul sullo sui sugli sull sugl sulla sulle per tra fra
	il lo la i gli le l un uno una un' e ed o od ma se che chi cui non piu più quale quali quanto quanta quanti quante quando come dove
	perche perché anche ancora gia già poi cosa cos mi ti si ci vi ne me te se sé lui lei loro noi voi io tu egli ella esso essa essi esse
	mio mia miei mie tuo tua tuoi tue suo sua suoi sue nostro nostra nostri nostre vostro vostra vostri vostre questo questa questi queste
	quello quella quelli quelle quel quei quegli c sono sei e è era erano essere stato stata stati state ho hai ha abbiamo avete hanno
	aveva avevano avere fu furono sia siano molto poco tutto tutti tutta tutte ogni altro altra altri altre sempre mai solo cosi così
	allora qui qua li là lì dunque quindi però pero mentre dopo prima sopra sotto verso senza oltre fino d s n the of and to in a is was
	he she it his her they that this with for as on at by from be or an are not but what who which`) {
		m[senzaAccenti.Replace(w)] = true
	}
	return m
}()

// paroleBib: le parole di un testo, minuscole, senza accenti né parole vuote, ridotte alla radice (radiceIt).
// Gli apostrofi separano («dell'abbazia» → «abbazia»).
func paroleBib(s string, out []string) []string {
	s = senzaAccenti.Replace(strings.ToLower(s))
	inizio := -1
	for i, c := range s + " " {
		alfa := unicode.IsLetter(c) || unicode.IsDigit(c)
		if alfa && inizio < 0 {
			inizio = i
		} else if !alfa && inizio >= 0 {
			w := s[inizio:i]
			inizio = -1
			if paroleVuote[w] || (utf8.RuneCountInString(w) < 2 && !unicode.IsDigit(rune(w[0]))) {
				continue
			}
			out = append(out, radiceIt(w))
		}
	}
	return out
}

// radiceIt: una radice «leggera» per l'italiano: plurale e femminile («fratello», «fratelli» → «fratell»), alcuni
// suffissi comuni (-mente, -zione, -issimo) e le desinenze verbali più frequenti. Non serve che sia una parola vera:
// serve che domanda e testo diano la stessa radice.
func radiceIt(w string) string {
	n := len(w)
	if n <= 4 || w[0] >= '0' && w[0] <= '9' {
		return w
	}
	for _, s := range [...]struct{ suf, con string }{
		{"issimi", ""}, {"issime", ""}, {"issimo", ""}, {"issima", ""}, {"amente", ""}, {"mente", ""},
		{"zioni", "zion"}, {"zione", "zion"}, {"sioni", "sion"}, {"sione", "sion"},
		{"arono", ""}, {"erono", ""}, {"irono", ""}, {"avano", ""}, {"evano", ""}, {"ivano", ""},
		{"ando", ""}, {"endo", ""}, {"ava", ""}, {"eva", ""}, {"iva", ""},
		{"are", ""}, {"ere", ""}, {"ire", ""}, {"ato", ""}, {"ata", ""}, {"ati", ""}, {"ate", ""},
		{"uto", ""}, {"uta", ""}, {"uti", ""}, {"ute", ""}, {"ito", ""}, {"ita", ""}, {"iti", ""}, {"ite", ""},
		{"che", "c"}, {"chi", "c"}, {"ghe", "g"}, {"ghi", "g"},
	} {
		if strings.HasSuffix(w, s.suf) && n-len(s.suf)+len(s.con) >= 4 {
			return w[:n-len(s.suf)] + s.con
		}
	}
	switch w[n-1] {
	case 'a', 'e', 'i', 'o':
		return w[:n-1]
	}
	return w
}

// ===== Ricerca =====

type postBib struct {
	p  int32
	tf uint16
}

// indiceTesti: BM25 su tutti i brani. lung: parole di ogni brano; media: lunghezza media.
type indiceTesti struct {
	pezzi []pezzoIndice
	post  map[string][]postBib
	media float64
}

type pezzoIndice struct {
	fonte int // posizione nella lista delle fonti
	pezzo int // numero del brano dentro la fonte
	testo string
	lung  int
}

// aggiungi mette nell'indice i brani di una fonte.
func (ix *indiceTesti) aggiungi(fonte int, pezzi []pezzoBib) {
	if ix.post == nil {
		ix.post = map[string][]postBib{}
	}
	tot := ix.media * float64(len(ix.pezzi))
	buf := make([]string, 0, 256)
	conta := map[string]int{}
	for n, p := range pezzi {
		buf = paroleBib(p.Testo, buf[:0])
		clear(conta)
		for _, w := range buf {
			conta[w]++
		}
		id := int32(len(ix.pezzi))
		for w, c := range conta {
			ix.post[w] = append(ix.post[w], postBib{id, uint16(min(c, 65535))})
		}
		ix.pezzi = append(ix.pezzi, pezzoIndice{fonte: fonte, pezzo: n, testo: p.Testo, lung: len(buf)})
		tot += float64(len(buf))
	}
	if len(ix.pezzi) > 0 {
		ix.media = tot / float64(len(ix.pezzi))
	}
}

// termineQ: una parola della domanda con il suo peso (1 = domanda, meno = messaggio precedente o nome alternativo).
// opz: parola aggiunta (nome alternativo, parola chiave dell'IA): alza il punteggio dei brani che la contengono ma non
// conta nella copertura, così un soprannome che nei testi non c'è non fa sembrare la domanda senza risposta.
type termineQ struct {
	w    string
	peso float64
	opz  bool
}

// domandaBib: le parole da cercare. Le parole con l'iniziale maiuscola a metà frase (nomi propri) pesano di più.
func domandaBib(testo string, peso float64, out []termineQ) []termineQ {
	return paroleDomanda(testo, peso, false, out)
}

// aggiunteBib: parole facoltative (vedi termineQ.opz).
func aggiunteBib(testo string, peso float64, out []termineQ) []termineQ {
	return paroleDomanda(testo, peso, true, out)
}

func paroleDomanda(testo string, peso float64, opz bool, out []termineQ) []termineQ {
	visti := map[string]int{}
	for i, t := range out {
		visti[t.w] = i
	}
	inizioFrase := true
	for _, campo := range strings.FieldsFunc(testo, func(r rune) bool { return unicode.IsSpace(r) || r == '\'' || r == '’' }) {
		r, _ := utf8.DecodeRuneInString(campo)
		nome := !inizioFrase && unicode.IsUpper(r)
		inizioFrase = strings.ContainsAny(campo[len(campo)-1:], ".?!:")
		for _, w := range paroleBib(campo, nil) {
			p := peso
			if nome {
				p *= 1.6
			}
			if i, ok := visti[w]; ok {
				out[i].peso = math.Max(out[i].peso, p)
				continue
			}
			visti[w] = len(out)
			out = append(out, termineQ{w, p, opz})
		}
	}
	return out
}

type risultatoBib struct {
	i         int // posizione in ix.pezzi
	punti     float64
	copertura float64 // quanta parte della domanda (pesata per rarità) c'è nel brano, da 0 a 1
}

// cerca: i brani migliori (al massimo n) tra quelli per cui ammessa(fonte) è vera. Punteggio BM25 pesato, più un premio
// quando due parole consecutive della domanda sono vicine anche nel brano e quando nel brano compaiono quasi tutte.
func (ix *indiceTesti) cerca(q []termineQ, n int, ammessa func(fonte int) bool) []risultatoBib {
	if len(ix.pezzi) == 0 || len(q) == 0 {
		return nil
	}
	const k1, b = 1.2, 0.75
	N := float64(len(ix.pezzi))
	punti := map[int32]float64{}
	quante := map[int32]int{}
	cop := map[int32]float64{}
	copTot := 0.0
	for _, t := range q {
		ps := ix.post[t.w]
		// una parola che non c'è in nessun testo conta come la più rara: abbassa la copertura di tutti i brani
		idf := math.Log(1 + (N-float64(len(ps))+0.5)/(float64(len(ps))+0.5))
		if !t.opz {
			copTot += t.peso * idf
		}
		for _, p := range ps {
			pz := ix.pezzi[p.p]
			if !ammessa(pz.fonte) {
				continue
			}
			tf := float64(p.tf)
			punti[p.p] += t.peso * idf * tf * (k1 + 1) / (tf + k1*(1-b+b*float64(pz.lung)/ix.media))
			if !t.opz {
				quante[p.p]++
				cop[p.p] += t.peso * idf
			}
		}
	}
	if len(punti) == 0 {
		return nil
	}
	ris := make([]risultatoBib, 0, len(punti))
	presenti := 0
	for _, t := range q {
		if len(ix.post[t.w]) > 0 && !t.opz {
			presenti++
		}
	}
	for p, v := range punti {
		if presenti > 2 { // quasi tutte le parole della domanda nello stesso brano
			v *= 1 + 0.5*float64(quante[p])/float64(presenti)
		}
		ris = append(ris, risultatoBib{int(p), v, cop[p] / math.Max(copTot, 1e-9)})
	}
	sort.Slice(ris, func(a, b int) bool { return ris[a].punti > ris[b].punti })
	// premio per la vicinanza: sui primi 300
	if len(q) > 1 {
		lim := min(len(ris), 300)
		for i := 0; i < lim; i++ {
			parole := paroleBib(ix.pezzi[ris[i].i].testo, nil)
			pos := map[string][]int{}
			for j, w := range parole {
				pos[w] = append(pos[w], j)
			}
			premio := 0.0
			for j := 0; j+1 < len(q); j++ {
				if vicine(pos[q[j].w], pos[q[j+1].w], 3) {
					premio += 0.6 * math.Min(q[j].peso, q[j+1].peso)
				}
			}
			ris[i].punti *= 1 + 0.25*premio
		}
		sort.Slice(ris[:lim], func(a, b int) bool { return ris[a].punti > ris[b].punti })
	}
	if len(ris) > n {
		ris = ris[:n]
	}
	return ris
}

// vicine: c'è una posizione di a e una di b distanti al massimo d parole?
func vicine(a, b []int, d int) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if x := a[i] - b[j]; x >= -d && x <= d {
			return true
		} else if x < 0 {
			i++
		} else {
			j++
		}
	}
	return false
}
