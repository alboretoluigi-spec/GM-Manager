package cripto

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CaricaChiavePrincipale legge la chiave principale dei dati dal file indicato,
// creandola se non esiste. Su Windows il file è protetto con DPAPI: solo l'account
// che l'ha creata (l'account del servizio Gaster Bless) può decifrarlo, e solo su questo PC.
// Se proteggi è false (modalità di prova) la chiave è salvata in chiaro.
func CaricaChiavePrincipale(percorso string, proteggi bool) ([]byte, error) {
	if b, err := os.ReadFile(percorso); err == nil {
		if !proteggi {
			if len(b) != 32 {
				return nil, errors.New("file della chiave non valido")
			}
			return b, nil
		}
		k, err := dpapiDecifra(b)
		if err != nil {
			return nil, fmt.Errorf("impossibile leggere la chiave dei dati (%s): è stata creata da un altro account o su un altro PC: %w", percorso, err)
		}
		if len(k) != 32 {
			return nil, errors.New("chiave dei dati non valida")
		}
		return k, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(percorso), 0o700); err != nil {
		return nil, err
	}
	k := Casuali(32)
	return k, SalvaChiavePrincipale(percorso, k, proteggi)
}

// SalvaChiavePrincipale scrive la chiave (protetta con DPAPI se proteggi è vero). Usata alla creazione e
// nel ripristino su un altro PC con il codice di ripristino.
func SalvaChiavePrincipale(percorso string, k []byte, proteggi bool) error {
	if len(k) != 32 {
		return errors.New("chiave non valida")
	}
	dato := k
	if proteggi {
		var err error
		if dato, err = dpapiCifra(k); err != nil {
			return fmt.Errorf("protezione DPAPI non riuscita: %w", err)
		}
	}
	tmp := percorso + ".nuova"
	if err := os.WriteFile(tmp, dato, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, percorso)
}
