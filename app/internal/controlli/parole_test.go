// Package controlli contiene prove che riguardano tutto il prodotto.
package controlli

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestParoleVietate fa fallire le prove se nel prodotto (cartella app/) compare un nome di prodotti di terzi o della
// campagna da cui l'app è nata. L'elenco è in docs/parole_vietate.txt; le librerie di terzi (web/vendor) non si
// controllano, e nemmeno il catalogo della RPG RULEBOOK LIBRARY (catalogo_regolamenti.json), dove i nomi ufficiali
// dei giochi compaiono come riferimento con l'avviso sui marchi (decisione del Master del 13/11/2026).
// Regole: docs/LICENZE_E_CONTENUTI.md.
func TestParoleVietate(t *testing.T) {
	radice, _ := filepath.Abs(filepath.Join("..", ".."))
	f, err := os.Open(filepath.Join(radice, "..", "docs", "parole_vietate.txt"))
	if err != nil {
		t.Fatalf("elenco delle parole vietate: %v", err)
	}
	defer f.Close()
	var parole []*regexp.Regexp
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		r := strings.TrimSpace(sc.Text())
		if r == "" || strings.HasPrefix(r, "#") {
			continue
		}
		parole = append(parole, regexp.MustCompile("(?i)"+r))
	}
	if len(parole) < 10 {
		t.Fatalf("elenco delle parole vietate troppo corto: %d", len(parole))
	}
	trovate := 0
	filepath.WalkDir(radice, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(radice, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "web/vendor" || strings.HasPrefix(d.Name(), ".") && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == fileCatalogo {
			return nil
		}
		for _, r := range parole {
			if r.MatchString(rel) {
				t.Errorf("%s: il nome del file contiene %q", rel, r.String())
				trovate++
			}
		}
		b, err := os.ReadFile(p)
		if err != nil || !testuale(b) {
			return nil
		}
		for i, riga := range strings.Split(string(b), "\n") {
			for _, r := range parole {
				if m := r.FindString(riga); m != "" {
					if trovate < 50 {
						t.Errorf("%s:%d: %q", rel, i+1, m)
					}
					trovate++
				}
			}
		}
		return nil
	})
	if trovate > 0 {
		t.Errorf("in tutto %d parole vietate", trovate)
	}
}

// fileCatalogo: il catalogo dei regolamenti, escluso dal controllo (vedi sopra).
const fileCatalogo = "catalogo_regolamenti.json"

// testuale dice se un file è di testo (niente byte nulli nei primi 8 KB).
func testuale(b []byte) bool {
	if len(b) > 8192 {
		b = b[:8192]
	}
	for _, c := range b {
		if c == 0 {
			return false
		}
	}
	return true
}
