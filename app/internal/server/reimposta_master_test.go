package server

import "testing"

func TestReimpostaPasswordMaster(t *testing.T) {
	a := nuovoAmbiente(t)
	if _, err := a.s.ReimpostaPasswordMaster("Nuova-password-1"); err == nil {
		t.Fatal("senza Master deve dare errore")
	}
	m := a.master()
	if _, err := a.s.ReimpostaPasswordMaster("corta"); err == nil {
		t.Fatal("una password troppo corta va rifiutata")
	}
	nome, err := a.s.ReimpostaPasswordMaster("La-torre-di-Argentio")
	if err != nil || nome != "Master" {
		t.Fatalf("reimpostazione: %q %v", nome, err)
	}
	// la vecchia sessione è chiusa
	if io := m.deve(200, "GET", "/io", nil); io["collegato"] != false {
		t.Fatal("la vecchia sessione del Master deve essere chiusa")
	}
	c := a.cliente("")
	c.deve(401, "POST", "/accesso", map[string]any{"nome": "Master", "password": "Il-castello-di-Corvonero"})
	c.deve(200, "POST", "/accesso", map[string]any{"nome": "Master", "password": "La-torre-di-Argentio"})
	if io := c.deve(200, "GET", "/io", nil); io["collegato"] != true {
		t.Fatal("con la nuova password il Master deve entrare")
	}
}

// Come sul PC: dal comando da amministratore, senza la chiave dei dati e con Gaster Bless accesa.
func TestReimpostaPasswordMasterDalPC(t *testing.T) {
	a := nuovoAmbiente(t)
	a.master()
	if _, err := ReimpostaPasswordMasterDalPC(a.s.Cfg, "Il-lago-di-Varnhold-9"); err != nil {
		t.Fatal(err)
	}
	c := a.cliente("")
	c.deve(200, "POST", "/accesso", map[string]any{"nome": "Master", "password": "Il-lago-di-Varnhold-9"})
}
