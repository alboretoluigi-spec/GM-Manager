package server

import (
	"bytes"
	"strings"
	"testing"

	"gasterbless/guide"
)

// «Scarica Guida» (2026-11-12): il Master riceve la guida del Master, i giocatori (anche i Segretari) quella per i giocatori.
func TestScaricaGuida(t *testing.T) {
	sc := nuovoScontro(t)
	cod, h, b := sc.m.grezzo("GET", "/guida", nil, "")
	if cod != 200 || !bytes.Equal(b, guide.Master) || h.Get("Content-Type") != "application/pdf" ||
		!strings.Contains(h.Get("Content-Disposition"), "GasterBless_Guida_Master.pdf") {
		t.Fatalf("guida del Master: %d %v", cod, h)
	}
	for _, c := range []*cliente{sc.luca, sc.sara} {
		cod, h, b := c.grezzo("GET", "/guida", nil, "")
		if cod != 200 || !bytes.Equal(b, guide.Giocatori) || !strings.Contains(h.Get("Content-Disposition"), "GasterBless_Guida_Giocatori.pdf") {
			t.Fatalf("guida per i giocatori: %d %v", cod, h)
		}
	}
	// senza accesso: niente guida
	if cod, _, _ := sc.a.cliente("192.168.1.99").grezzo("GET", "/guida", nil, ""); cod != 401 {
		t.Fatalf("senza accesso: %d", cod)
	}
	if !bytes.HasPrefix(guide.Master, []byte("%PDF")) || !bytes.HasPrefix(guide.Giocatori, []byte("%PDF")) {
		t.Fatal("le guide incorporate devono essere PDF")
	}
}
