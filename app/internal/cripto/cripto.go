// Package cripto raccoglie le funzioni crittografiche di Gaster Bless:
// cifratura AES-256-GCM, hash delle password con Argon2id, token casuali, HMAC.
package cripto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Chiavi contiene le chiavi derivate dalla chiave principale dei dati.
type Chiavi struct {
	cifra    cipher.AEAD
	impronta []byte // per HMAC dei telefoni (ricerca dei doppioni senza decifrare)
}

// NuoveChiavi deriva le chiavi di lavoro dalla chiave principale (32 byte).
func NuoveChiavi(principale []byte) (*Chiavi, error) {
	if len(principale) != 32 {
		return nil, errors.New("la chiave principale deve essere di 32 byte")
	}
	kc := deriva(principale, "gasterbless/cifratura/v1")
	blk, err := aes.NewCipher(kc)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	return &Chiavi{cifra: g, impronta: deriva(principale, "gasterbless/impronta/v1")}, nil
}

func deriva(k []byte, scopo string) []byte {
	m := hmac.New(sha256.New, k)
	m.Write([]byte(scopo))
	return m.Sum(nil)
}

// Cifra restituisce nonce||testo cifrato. "contesto" lega il dato al suo uso
// (per esempio "telefono:<id utente>"), così non si può spostare tra campi.
func (c *Chiavi) Cifra(dato []byte, contesto string) []byte {
	nonce := Casuali(c.cifra.NonceSize())
	return c.cifra.Seal(nonce, nonce, dato, []byte(contesto))
}

// Decifra è l'inverso di Cifra.
func (c *Chiavi) Decifra(dato []byte, contesto string) ([]byte, error) {
	n := c.cifra.NonceSize()
	if len(dato) < n {
		return nil, errors.New("dato cifrato troppo corto")
	}
	return c.cifra.Open(nil, dato[:n], dato[n:], []byte(contesto))
}

// Impronta calcola un HMAC stabile del valore (per esempio un telefono normalizzato).
func (c *Chiavi) Impronta(valore string) string {
	m := hmac.New(sha256.New, c.impronta)
	m.Write([]byte(valore))
	return hex.EncodeToString(m.Sum(nil))
}

// Casuali restituisce n byte casuali crittograficamente sicuri.
func Casuali(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("generatore casuale non disponibile: " + err.Error())
	}
	return b
}

// Token restituisce un token casuale leggibile negli URL (n byte di entropia).
func Token(n int) string { return base64.RawURLEncoding.EncodeToString(Casuali(n)) }

// ID restituisce un identificativo casuale breve (64 bit, esadecimale).
func ID() string { return hex.EncodeToString(Casuali(8)) }

// HashToken calcola lo SHA-256 di un token, da salvare al posto del token stesso.
func HashToken(t string) string {
	s := sha256.Sum256([]byte(t))
	return hex.EncodeToString(s[:])
}

// UgualiCostante confronta due stringhe in tempo costante.
func UgualiCostante(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Parametri di Argon2id (raccomandazione OWASP: 19 MiB, 2 passaggi, 1 thread).
const (
	argonMemoria  = 19 * 1024
	argonPassaggi = 2
	argonThread   = 1
	argonLung     = 32
)

// HashPassword calcola l'hash Argon2id in formato PHC.
func HashPassword(pw string) string {
	sale := Casuali(16)
	h := argon2.IDKey([]byte(pw), sale, argonPassaggi, argonMemoria, argonThread, argonLung)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemoria, argonPassaggi, argonThread,
		base64.RawStdEncoding.EncodeToString(sale), base64.RawStdEncoding.EncodeToString(h))
}

// VerificaPassword controlla una password contro un hash in formato PHC.
func VerificaPassword(pw, codifica string) bool {
	p := strings.Split(codifica, "$")
	if len(p) != 6 || p[1] != "argon2id" {
		return false
	}
	var m uint32
	var t uint32
	var th uint8
	if _, err := fmt.Sscanf(p[3], "m=%d,t=%d,p=%d", &m, &t, &th); err != nil {
		return false
	}
	sale, err1 := base64.RawStdEncoding.DecodeString(p[4])
	atteso, err2 := base64.RawStdEncoding.DecodeString(p[5])
	if err1 != nil || err2 != nil || len(atteso) == 0 {
		return false
	}
	h := argon2.IDKey([]byte(pw), sale, t, m, th, uint32(len(atteso)))
	return subtle.ConstantTimeCompare(h, atteso) == 1
}

// HashFittizio serve a rendere costante il tempo di risposta quando l'utente non esiste.
var HashFittizio = HashPassword("gasterbless-utente-inesistente")
