package server

import "testing"

func TestNormalizzaTelefono(t *testing.T) {
	validi := map[string]string{
		"333 1234567":       "+393331234567",
		"+39 333-123.4567":  "+393331234567",
		"0039 3331234567":   "+393331234567",
		"+39 06 1234567":    "+39061234567",
		"+41 79 123 45 67":  "+41791234567",
		"+44 7700 900123":   "+447700900123",
		"+1 (555) 010-0199": "+15550100199",
	}
	for in, atteso := range validi {
		got, err := NormalizzaTelefono(in)
		if err != nil || got != atteso {
			t.Errorf("%q: atteso %s, ottenuto %s (%v)", in, atteso, got, err)
		}
	}
	for _, in := range []string{"", "123", "+39 123", "+39 33312", "abc", "+39 333 1234567 89", "+0 1234567890", "33a1234567"} {
		if _, err := NormalizzaTelefono(in); err == nil {
			t.Errorf("%q doveva essere rifiutato", in)
		}
	}
}

func TestMascheraTelefono(t *testing.T) {
	if m := MascheraTelefono("+393331234567"); m != "+39 ••• ••• 4567" {
		t.Fatalf("maschera inattesa: %s", m)
	}
}

func TestControllaPassword(t *testing.T) {
	for _, pw := range []string{"corto", "aaaaaaaaaaaa", "1234567890", "ababababab", "Luca-è-forte-99"} {
		if ControllaPassword(pw, "Luca", "+393331234567") == nil {
			t.Errorf("%q doveva essere rifiutata", pw)
		}
	}
	if err := ControllaPassword("Nebbia sul lago Varnhold", "Luca", "+393331234567"); err != nil {
		t.Errorf("password valida rifiutata: %v", err)
	}
	if ControllaPassword("x1231234567y", "Marta", "+393331234567") == nil {
		t.Error("una password con il numero di telefono doveva essere rifiutata")
	}
}
