//go:build !windows

package cripto

import "errors"

// Fuori da Windows DPAPI non esiste: si può usare solo la modalità di prova.
func dpapiCifra([]byte) ([]byte, error) {
	return nil, errors.New("DPAPI disponibile solo su Windows")
}

func dpapiDecifra([]byte) ([]byte, error) {
	return nil, errors.New("DPAPI disponibile solo su Windows")
}
