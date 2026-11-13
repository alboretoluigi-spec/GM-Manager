package server

// Lettore RTF minimo per MY RULEBOOKS (docs/RULEBOOK_LIBRARY.md, passo L3): estrae il testo semplice, abbastanza per la
// ricerca e la lettura. Gestisce gruppi, paragrafi (\par, \line), tabulazioni, caratteri esadecimali (\'hh, Windows-1252),
// Unicode (\uN con \ucN), e salta le parti che non sono testo (tabelle dei caratteri e dei colori, stili, informazioni,
// immagini, destinazioni \*). Un file che non comincia con «{\rtf» o con le parentesi sbilanciate è considerato danneggiato.

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

var errRTF = errors.New("il file RTF è danneggiato o non è un RTF")

// cp1252 per i byte 0x80-0x9F (gli altri coincidono con Latin-1).
var cp1252rtf = [32]rune{'€', 0, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0, 'Ž', 0,
	0, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0, 'ž', 'Ÿ'}

func runaCP1252(b byte) rune {
	if b >= 0x80 && b < 0xA0 {
		if r := cp1252rtf[b-0x80]; r != 0 {
			return r
		}
		return utf8.RuneError
	}
	return rune(b)
}

var destinazioniSaltate = map[string]bool{"fonttbl": true, "colortbl": true, "stylesheet": true, "info": true, "pict": true,
	"header": true, "footer": true, "headerl": true, "headerr": true, "footerl": true, "footerr": true, "listtable": true,
	"listoverridetable": true, "rsidtbl": true, "generator": true, "themedata": true, "colorschememapping": true,
	"latentstyles": true, "datastore": true, "xmlnstbl": true, "object": true, "fldinst": true}

// testoDaRTF restituisce il testo del documento RTF.
func testoDaRTF(b []byte) (string, error) {
	s := string(b)
	if !strings.HasPrefix(strings.TrimLeft(s, " \r\n\t\uFEFF"), `{\rtf`) {
		return "", errRTF
	}
	type stato struct {
		salta bool
		uc    int
	}
	pila := []stato{{uc: 1}}
	var out strings.Builder
	daSaltare := 0 // caratteri di ripiego dopo \uN
	i, n := 0, len(s)
	for i < n {
		c := s[i]
		cima := &pila[len(pila)-1]
		switch c {
		case '{':
			pila = append(pila, *cima)
			i++
			if strings.HasPrefix(s[i:], `\*`) {
				pila[len(pila)-1].salta = true
			}
			continue
		case '}':
			if len(pila) <= 1 {
				return "", errRTF
			}
			pila = pila[:len(pila)-1]
			i++
			continue
		case '\r', '\n':
			i++
			continue
		case '\\':
			i++
			if i >= n {
				break
			}
			d := s[i]
			if d == '\\' || d == '{' || d == '}' {
				if !cima.salta {
					if daSaltare > 0 {
						daSaltare--
					} else {
						out.WriteByte(d)
					}
				}
				i++
				continue
			}
			if d == '\'' && i+2 < n {
				v, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
				i += 3
				if err == nil && !cima.salta {
					if daSaltare > 0 {
						daSaltare--
					} else {
						out.WriteRune(runaCP1252(byte(v)))
					}
				}
				continue
			}
			if d == '~' {
				if !cima.salta {
					out.WriteRune(' ')
				}
				i++
				continue
			}
			if !(d >= 'a' && d <= 'z' || d >= 'A' && d <= 'Z') {
				i++ // simbolo di controllo non usato (\-, \_, \:)
				continue
			}
			j := i
			for j < n && (s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z') {
				j++
			}
			parola := s[i:j]
			k := j
			if k < n && (s[k] == '-' || s[k] >= '0' && s[k] <= '9') {
				k++
				for k < n && s[k] >= '0' && s[k] <= '9' {
					k++
				}
			}
			num, haNum := 0, k > j
			if haNum {
				num, _ = strconv.Atoi(s[j:k])
			}
			if k < n && s[k] == ' ' {
				k++
			}
			i = k
			if destinazioniSaltate[parola] {
				cima.salta = true
				continue
			}
			if cima.salta {
				continue
			}
			switch parola {
			case "par", "line", "sect", "page", "row":
				out.WriteByte('\n')
			case "tab", "cell":
				out.WriteByte('\t')
			case "uc":
				cima.uc = num
			case "u":
				if haNum {
					if num < 0 {
						num += 65536
					}
					out.WriteRune(rune(num))
					daSaltare = cima.uc
				}
			case "emdash":
				out.WriteRune('—')
			case "endash":
				out.WriteRune('–')
			case "bullet":
				out.WriteRune('•')
			case "lquote":
				out.WriteRune('‘')
			case "rquote":
				out.WriteRune('’')
			case "ldblquote":
				out.WriteRune('“')
			case "rdblquote":
				out.WriteRune('”')
			}
			continue
		}
		if !cima.salta {
			if daSaltare > 0 {
				daSaltare--
			} else {
				out.WriteByte(c)
			}
		}
		i++
	}
	if len(pila) != 1 {
		return "", errRTF
	}
	return strings.TrimSpace(out.String()), nil
}
