package server

import (
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizzaTelefono porta un numero al formato internazionale E.164 (+39...).
// Accetta spazi, trattini, punti e parentesi; "00" iniziale diventa "+";
// senza prefisso si assume l'Italia (+39).
func NormalizzaTelefono(s string) (string, error) {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')' || r == '/':
		default:
			return "", errore(http.StatusBadRequest, "Numero di telefono: contiene caratteri non ammessi.")
		}
	}
	n := b.String()
	switch {
	case strings.HasPrefix(n, "+"):
	case strings.HasPrefix(n, "00"):
		n = "+" + n[2:]
	default:
		n = "+39" + n
	}
	cifre := n[1:]
	if len(cifre) < 8 || len(cifre) > 15 || cifre[0] == '0' {
		return "", errore(http.StatusBadRequest, "Numero di telefono non valido: usa il formato internazionale, per esempio +39 333 1234567.")
	}
	if strings.HasPrefix(n, "+39") {
		naz := n[3:]
		// Italia: cellulari 3xx (9-10 cifre), fissi 0x (6-11 cifre).
		ok := (naz[0] == '3' && len(naz) >= 9 && len(naz) <= 10) || (naz[0] == '0' && len(naz) >= 6 && len(naz) <= 11)
		if !ok {
			return "", errore(http.StatusBadRequest, "Numero italiano non valido: controlla le cifre (per esempio +39 333 1234567).")
		}
	}
	return n, nil
}

// MascheraTelefono restituisce la forma da mostrare nell'interfaccia: "+39 ••• ••• 4567".
func MascheraTelefono(e164 string) string {
	if len(e164) < 6 {
		return "•••"
	}
	pref := "+" + prefissoPaese(e164[1:])
	return pref + " ••• ••• " + e164[len(e164)-4:]
}

// prefissoPaese riconosce i prefissi internazionali più comuni (1-3 cifre).
func prefissoPaese(c string) string {
	for _, p := range []string{"39", "41", "33", "34", "44", "49", "43", "32", "31", "351", "356", "378", "1"} {
		if strings.HasPrefix(c, p) {
			return p
		}
	}
	if len(c) > 2 {
		return c[:2]
	}
	return c
}

// Password troppo comuni (estratto): rifiutate anche se lunghe abbastanza.
var passwordComuni = map[string]bool{
	"1234567890": true, "0123456789": true, "password12": true, "password123": true, "qwertyuiop": true,
	"1234567891": true, "1111111111": true, "123456789a": true, "iloveyou12": true, "abcdefghij": true,
	"passwordpassword": true, "gasterbless123": true, "antagonista1234": true, "dungeons123": true, "dragons123": true,
	"qwerty1234": true, "asdfghjkl1": true, "1q2w3e4r5t": true, "zaq12wsxcde": true, "forzainter": true,
	"forzamilan": true, "forzajuve1": true, "forzaroma1": true, "napoli1926": true, "juventus1897": true,
}

// ControllaPassword applica i requisiti minimi e restituisce un messaggio chiaro se non vanno bene.
func ControllaPassword(pw, nome, telefono string) error {
	n := utf8.RuneCountInString(pw)
	if n < 10 {
		return errore(http.StatusBadRequest, "La password deve avere almeno 10 caratteri.")
	}
	if n > 128 {
		return errore(http.StatusBadRequest, "La password è troppo lunga (massimo 128 caratteri).")
	}
	diversi := map[rune]bool{}
	for _, r := range pw {
		diversi[unicode.ToLower(r)] = true
	}
	if len(diversi) < 4 {
		return errore(http.StatusBadRequest, "La password è troppo semplice: usa almeno 4 caratteri diversi.")
	}
	low := strings.ToLower(pw)
	if passwordComuni[low] {
		return errore(http.StatusBadRequest, "Questa password è tra le più usate: scegline un'altra.")
	}
	if nome = strings.ToLower(strings.TrimSpace(nome)); len(nome) >= 4 && strings.Contains(low, nome) {
		return errore(http.StatusBadRequest, "La password non deve contenere il tuo nome.")
	}
	if len(telefono) > 6 && strings.Contains(pw, telefono[len(telefono)-6:]) {
		return errore(http.StatusBadRequest, "La password non deve contenere il tuo numero di telefono.")
	}
	return nil
}
