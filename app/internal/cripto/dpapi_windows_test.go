//go:build windows

package cripto

import (
	"bytes"
	"os"
	"testing"
)

func TestChiaveProtettaDPAPI(t *testing.T) {
	p := t.TempDir() + "/k"
	a, err := CaricaChiavePrincipale(p, true)
	if err != nil {
		t.Fatal(err)
	}
	grezzo, _ := os.ReadFile(p)
	if bytes.Contains(grezzo, a) {
		t.Fatal("il file deve contenere la chiave protetta, non in chiaro")
	}
	b, err := CaricaChiavePrincipale(p, true)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("la chiave protetta deve essere riletta uguale")
	}
}
