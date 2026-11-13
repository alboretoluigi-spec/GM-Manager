package server

import "testing"

// Dadi 3D (2026-11-12): ogni utente sceglie il colore del suo set; ogni tiro porta il colore di chi tira e la classe
// del suo personaggio (per l'effetto del d20); il Master che tira per i mostri non ha classe.
func TestColoreEClasseDeiDadi(t *testing.T) {
	sc := nuovoScontro(t) // Arin, chierico di 3° livello di Luca, nella sessione
	sc.luca.deve(400, "PUT", "/io/dadi", map[string]any{"colore": "rosso"})
	sc.luca.deve(200, "PUT", "/io/dadi", map[string]any{"colore": "#1F4FA0/#FFFFFF"})
	if c := sc.luca.deve(200, "GET", "/io", nil)["coloreDadi"]; c != "#1f4fa0/#ffffff" {
		t.Fatalf("colore salvato: %v", c)
	}
	cl, cm := sc.clienteWS(sc.luca), sc.clienteWS(sc.m)
	r := sc.wsOK(cl, "dadi.tira", map[string]any{"sessione": sc.ses, "dadi": []map[string]any{{"n": 1, "facce": 20}}}).(tiroDadi)
	if r.Colore != "#1f4fa0/#ffffff" || r.Classe != "chierico" {
		t.Fatalf("tiro del giocatore: colore %q, classe %q", r.Colore, r.Classe)
	}
	// dalla scheda di un personaggio: la sua classe
	r = sc.wsOK(cm, "dadi.tira", map[string]any{"dadi": []map[string]any{{"n": 1, "facce": 20}}, "scheda": true, "personaggio": sc.pg}).(tiroDadi)
	if r.Classe != "chierico" {
		t.Fatalf("tiro del Master dalla scheda di Arin: classe %q", r.Classe)
	}
	// il Master per conto suo: il suo colore (vuoto = predefinito), nessuna classe
	sc.m.deve(200, "PUT", "/io/dadi", map[string]any{"colore": "#1d1d22/#e8c25a"})
	r = sc.wsOK(cm, "dadi.tira", map[string]any{"sessione": sc.ses, "dadi": []map[string]any{{"n": 2, "facce": 6}}}).(tiroDadi)
	if r.Colore != "#1d1d22/#e8c25a" || r.Classe != "" {
		t.Fatalf("tiro del Master: colore %q, classe %q", r.Colore, r.Classe)
	}
	sc.luca.deve(200, "PUT", "/io/dadi", map[string]any{"colore": ""}) // torna al set predefinito
	if c := sc.luca.deve(200, "GET", "/io", nil)["coloreDadi"]; c != "" {
		t.Fatalf("colore azzerato: %v", c)
	}
}

// 2026-11-13: l'esito di un attacco porta il colore e la classe di chi tira, così anche gli altri presenti vedono i dadi
// 3D con il suo set (un mostro: il set del Master, senza classe).
func TestDadiVistiDaTutti(t *testing.T) {
	s := nuovaScena(t, nil, bandito("Bandito", nil))
	s.luca.deve(200, "PUT", "/io/dadi", map[string]any{"colore": "#1d6b45/#f6f0d8"})
	s.m.deve(200, "PUT", "/io/dadi", map[string]any{"colore": "#1d1d22/#e8c25a"})
	e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], map[string]any{"attacco": 0}, 15, 4)
	if e["colore"] != "#1d6b45/#f6f0d8" || e["classe"] != "guerriero" {
		t.Fatalf("attacco di Arin: colore %v, classe %v", e["colore"], e["classe"])
	}
	s.az(s.m, 200, "iniz.prossimo_turno", nil)
	e = s.attacca(s.m, 200, s.id["Bandito"], s.idA, map[string]any{"attacco": 0, "forza": true}, 18, 5)
	if e["colore"] != "#1d1d22/#e8c25a" || e["classe"] != nil {
		t.Fatalf("attacco del bandito: colore %v, classe %v", e["colore"], e["classe"])
	}
}
