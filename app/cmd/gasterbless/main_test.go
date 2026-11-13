package main

import "testing"

func TestPulisciPassword(t *testing.T) {
	for in, atteso := range map[string]string{
		"\ufeffNove-car9\r\n": "Nove-car9",
		"Nove-car9\n":         "Nove-car9",
		" spazi  ":            " spazi  ",
	} {
		if got := pulisciPassword(in); got != atteso {
			t.Errorf("pulisciPassword(%q) = %q, atteso %q", in, got, atteso)
		}
	}
}
