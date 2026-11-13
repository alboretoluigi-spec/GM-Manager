package cripto

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP secondo RFC 6238 (SHA-1, 6 cifre, periodo 30 s): compatibile con le app
// di autenticazione più diffuse (Google Authenticator, Microsoft Authenticator, Aegis...).

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NuovoSegretoTOTP genera un segreto di 20 byte codificato in base32.
func NuovoSegretoTOTP() string { return b32.EncodeToString(Casuali(20)) }

// URITOTP costruisce l'indirizzo otpauth:// da mostrare come codice QR.
func URITOTP(segreto, account string) string {
	return "otpauth://totp/" + url.PathEscape("Gaster Bless:"+account) +
		"?secret=" + segreto + "&issuer=Gaster Bless&digits=6&period=30"
}

// CodiceTOTP calcola il codice per un istante.
func CodiceTOTP(segreto string, t time.Time) (string, error) {
	k, err := b32.DecodeString(strings.ToUpper(strings.TrimSpace(segreto)))
	if err != nil {
		return "", err
	}
	return hotp(k, uint64(t.Unix()/30), 6), nil
}

func hotp(k []byte, contatore uint64, cifre int) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], contatore)
	m := hmac.New(sha1.New, k)
	m.Write(msg[:])
	s := m.Sum(nil)
	off := s[len(s)-1] & 0x0f
	v := binary.BigEndian.Uint32(s[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < cifre; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", cifre, v%mod)
}

// VerificaTOTP accetta il codice dell'intervallo corrente e di quelli adiacenti
// (tolleranza di ±30 s per orologi leggermente sfasati). Restituisce l'intervallo
// usato, così il chiamante può impedire il riuso dello stesso codice.
func VerificaTOTP(segreto, codice string, ora time.Time) (int64, bool) {
	codice = strings.ReplaceAll(strings.TrimSpace(codice), " ", "")
	k, err := b32.DecodeString(strings.ToUpper(strings.TrimSpace(segreto)))
	if err != nil || len(codice) != 6 {
		return 0, false
	}
	base := ora.Unix() / 30
	for _, d := range []int64{0, -1, 1} {
		if UgualiCostante(hotp(k, uint64(base+d), 6), codice) {
			return base + d, true
		}
	}
	return 0, false
}
