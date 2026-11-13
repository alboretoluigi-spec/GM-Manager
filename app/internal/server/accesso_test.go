package server

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrimoAvvioSoloLocale(t *testing.T) {
	a := nuovoAmbiente(t)
	da_rete := a.cliente("192.168.1.50")
	da_rete.deve(403, "POST", "/primo-avvio", map[string]any{"nome": "Intruso", "password": "Il-castello-di-Corvonero"})
	a.master()
	a.cliente("").deve(409, "POST", "/primo-avvio", map[string]any{"nome": "Secondo", "password": "Il-castello-di-Corvonero"})
}

func TestRetiNonAmmesse(t *testing.T) {
	a := nuovoAmbiente(t)
	a.cliente("8.8.8.8").deve(403, "GET", "/io", nil)
	// Rete VPN rifiutata finché l'accesso esterno è chiuso (modalità "solo rete di casa").
	a.cliente("100.100.1.2").deve(403, "GET", "/io", nil)
	a.cliente("192.168.1.20").deve(200, "GET", "/io", nil)
}

func TestRegistrazioneAttesaApprovazione(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	gr := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì", "giorno": "Martedì"})["id"].(string)
	g := a.giocatore("Luca", "333 1234567", "192.168.1.20")

	// In attesa: nessuna funzione di gioco, nemmeno chiamando le API direttamente.
	g.deve(403, "GET", "/gruppi", nil)
	g.deve(403, "GET", "/gruppi/"+gr+"/sessioni", nil)
	g.deve(403, "GET", "/utenti", nil)
	g.deve(200, "GET", "/profilo", nil) // il profilo resta accessibile

	// Numeri duplicati (anche scritti in modo diverso) e password deboli sono rifiutati.
	a.cliente("192.168.1.21").deve(409, "POST", "/registrazione", map[string]any{"nome": "Altro", "telefono": "+39 333-123-4567",
		"password": "Nebbia sul lago Varnhold", "conferma": "Nebbia sul lago Varnhold", "consenso": true})
	a.cliente("192.168.1.21").deve(400, "POST", "/registrazione", map[string]any{"nome": "Altro", "telefono": "+39 333 7654321",
		"password": "1234567890", "conferma": "1234567890", "consenso": true})
	a.cliente("192.168.1.21").deve(400, "POST", "/registrazione", map[string]any{"nome": "Altro", "telefono": "+39 12",
		"password": "Nebbia sul lago Varnhold", "conferma": "Nebbia sul lago Varnhold", "consenso": true})
	a.cliente("192.168.1.21").deve(400, "POST", "/registrazione", map[string]any{"nome": "Altro", "telefono": "+39 333 7654321",
		"password": "Nebbia sul lago Varnhold", "conferma": "Nebbia sul lago Varnhold", "consenso": false})

	// Il Master vede la richiesta con il telefono mascherato.
	lista := m.deve(200, "GET", "/utenti", nil)["lista"].([]any)
	u := lista[0].(map[string]any)
	if u["stato"] != "in_attesa" || u["telefono"] != "+39 ••• ••• 4567" {
		t.Fatalf("richiesta inattesa: %v", u)
	}
	m.deve(200, "POST", "/utenti/"+u["id"].(string)+"/approva", map[string]any{"gruppi": []string{gr}})
	g.deve(200, "GET", "/io", nil)
	gruppi := g.deve(200, "GET", "/gruppi", nil)["lista"].([]any)
	if len(gruppi) != 1 {
		t.Fatalf("il giocatore approvato deve vedere il suo gruppo: %v", gruppi)
	}

	// Rifiutato o bloccato: niente accesso.
	m.deve(200, "POST", "/utenti/"+u["id"].(string)+"/stato", map[string]any{"stato": "bloccato"})
	g.deve(401, "GET", "/gruppi", nil) // la sessione è stata chiusa
	g2 := a.cliente("192.168.1.20")
	g2.deve(403, "POST", "/accesso", map[string]any{"telefono": "3331234567", "password": "Nebbia sul lago Varnhold"})
}

func TestTelefonoCifratoNelDatabase(t *testing.T) {
	a := nuovoAmbiente(t)
	a.master()
	a.giocatore("Luca", "333 1234567", "192.168.1.20")
	var cif []byte
	var imp, mask string
	a.s.DB.QueryRow(`SELECT telefono_cifrato, telefono_impronta, telefono_finale FROM utenti WHERE nome = 'Luca'`).Scan(&cif, &imp, &mask)
	if len(cif) == 0 || bytes.Contains(cif, []byte("3331234567")) || strings.Contains(imp, "3331234567") {
		t.Fatal("il numero non deve comparire in chiaro nel database")
	}
	if mask != "+39 ••• ••• 4567" {
		t.Fatalf("maschera: %s", mask)
	}
}

func TestCSRFeOrigine(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	vero := m.csrf
	m.csrf = "sbagliato"
	m.deve(403, "POST", "/gruppi", map[string]any{"nome": "X"})
	m.csrf = vero
	m.deve(200, "POST", "/gruppi", map[string]any{"nome": "X"})
}

func TestLimiteTentativi(t *testing.T) {
	a := nuovoAmbiente(t)
	a.master()
	a.giocatore("Luca", "333 1234567", "192.168.1.20")
	c := a.cliente("192.168.1.30")
	for i := 0; i < 5; i++ {
		c.deve(401, "POST", "/accesso", map[string]any{"telefono": "3331234567", "password": "sbagliata-" + string(rune('a'+i))})
	}
	// Dopo 5 errori: bloccato anche con la password giusta.
	c.deve(429, "POST", "/accesso", map[string]any{"telefono": "3331234567", "password": "Nebbia sul lago Varnhold"})
	// Il blocco vale anche per lo stesso numero da un altro dispositivo.
	a.cliente("192.168.1.31").deve(429, "POST", "/accesso", map[string]any{"telefono": "3331234567", "password": "Nebbia sul lago Varnhold"})
}

func TestCodiceReimpostazioneMonousoEScadenza(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	id := a.idUtente(g)
	cod := m.deve(200, "POST", "/utenti/"+id+"/codice-reimpostazione", map[string]any{})["codice"].(string)
	nuova := map[string]any{"telefono": "3331234567", "codice": cod, "password": "Le campane di Borgofosco", "conferma": "Le campane di Borgofosco"}
	c := a.cliente("192.168.1.20")
	c.deve(200, "POST", "/password/reimposta", nuova)
	c.deve(400, "POST", "/password/reimposta", nuova) // una sola volta
	c.deve(200, "POST", "/accesso", map[string]any{"telefono": "3331234567", "password": "Le campane di Borgofosco"})

	// Codice scaduto
	cod2 := m.deve(200, "POST", "/utenti/"+id+"/codice-reimpostazione", map[string]any{})["codice"].(string)
	a.s.DB.Exec(`UPDATE codici_reimpostazione SET scade_il = 1 WHERE usato_il IS NULL`)
	c.deve(400, "POST", "/password/reimposta", map[string]any{"telefono": "3331234567", "codice": cod2, "password": "Il Mago dei Vini 77", "conferma": "Il Mago dei Vini 77"})
}

func TestInvitoProponeGruppoEScade(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	gr := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Venerdì"})["id"].(string)
	inv := m.deve(200, "POST", "/inviti", map[string]any{"gruppo": gr, "usiMax": 1})["codice"].(string)
	c := a.cliente("192.168.1.40")
	c.deve(200, "GET", "/invito/"+inv, nil)
	c.deve(200, "POST", "/registrazione", map[string]any{"nome": "Sara", "telefono": "+39 347 1112223", "password": "Nebbia sul lago Varnhold",
		"conferma": "Nebbia sul lago Varnhold", "consenso": true, "invito": inv})
	lista := m.deve(200, "GET", "/utenti", nil)["lista"].([]any)
	if lista[0].(map[string]any)["gruppoProposto"] != gr {
		t.Fatal("l'invito deve proporre il gruppo al Master")
	}
	// Usi esauriti
	a.cliente("192.168.1.41").deve(400, "GET", "/invito/"+inv, nil)
}

func TestEsportazioneSenzaDatiAltrui(t *testing.T) {
	a := nuovoAmbiente(t)
	a.master()
	g := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	a.giocatore("Sara", "347 1112223", "192.168.1.21")
	_, out := g.chiama("GET", "/profilo/esporta", nil)
	acc := out["account"].(map[string]any)
	if acc["telefono"] != "+393331234567" || acc["nome"] != "Luca" {
		t.Fatalf("esportazione incompleta: %v", acc)
	}
	b, _ := jsonString(out)
	if strings.Contains(b, "Sara") || strings.Contains(b, "1112223") || strings.Contains(b, "password") {
		t.Fatal("l'esportazione non deve contenere dati di altri né la password")
	}
}

func TestMasterDaVPNRichiedeTOTP(t *testing.T) {
	a := nuovoAmbiente(t)
	a.master()
	a.s.Cfg.AccessoEsterno = "vpn"
	a.s.Filtro.ImpostaEsterno(true)
	c := a.cliente("100.100.1.2")
	c.deve(403, "POST", "/accesso", map[string]any{"nome": "Master", "password": "Il-castello-di-Corvonero"})
}
