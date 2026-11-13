package cripto

import (
	"bytes"
	"encoding/base32"
	"testing"
	"time"
)

func TestPassword(t *testing.T) {
	h := HashPassword("Corvo-di-Gaster Bless-42")
	if !VerificaPassword("Corvo-di-Gaster Bless-42", h) {
		t.Fatal("la password corretta deve essere accettata")
	}
	if VerificaPassword("corvo-di-gasterbless-42", h) {
		t.Fatal("la password sbagliata deve essere rifiutata")
	}
	if h == HashPassword("Corvo-di-Gaster Bless-42") {
		t.Fatal("due hash della stessa password devono avere sali diversi")
	}
	if VerificaPassword("x", "non-un-hash") {
		t.Fatal("un hash malformato non deve mai essere accettato")
	}
}

func TestCifraturaConContesto(t *testing.T) {
	k, err := NuoveChiavi(Casuali(32))
	if err != nil {
		t.Fatal(err)
	}
	c := k.Cifra([]byte("+393331234567"), "telefono:a")
	if bytes.Contains(c, []byte("3331234567")) {
		t.Fatal("il testo cifrato non deve contenere il numero in chiaro")
	}
	p, err := k.Decifra(c, "telefono:a")
	if err != nil || string(p) != "+393331234567" {
		t.Fatal("decifratura non riuscita")
	}
	if _, err := k.Decifra(c, "telefono:b"); err == nil {
		t.Fatal("un dato non deve decifrarsi con un contesto diverso")
	}
	if uno, due := k.Impronta("+393331234567"), k.Impronta("+393331234567"); uno != due {
		t.Fatal("l'impronta deve essere stabile")
	}
}

// Vettori di prova ufficiali dell'RFC 6238 (SHA-1, chiave "12345678901234567890", 8 cifre).
func TestTOTPVettoriRFC(t *testing.T) {
	k := []byte("12345678901234567890")
	casi := map[int64]string{59: "94287082", 1111111109: "07081804", 1111111111: "14050471", 1234567890: "89005924", 2000000000: "69279037", 20000000000: "65353130"}
	for sec, atteso := range casi {
		if g := hotp(k, uint64(sec/30), 8); g != atteso {
			t.Errorf("t=%d: atteso %s, ottenuto %s", sec, atteso, g)
		}
	}
	seg := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(k)
	ora := time.Unix(1234567890, 0)
	cod, _ := CodiceTOTP(seg, ora)
	if _, ok := VerificaTOTP(seg, cod, ora); !ok {
		t.Fatal("il codice corrente deve essere accettato")
	}
	if _, ok := VerificaTOTP(seg, cod, ora.Add(5*time.Minute)); ok {
		t.Fatal("un codice vecchio di 5 minuti deve essere rifiutato")
	}
}

func TestChiaveInChiaroModalitaProva(t *testing.T) {
	p := t.TempDir() + "/k"
	a, err := CaricaChiavePrincipale(p, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CaricaChiavePrincipale(p, false)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("la chiave deve essere riletta uguale")
	}
}
