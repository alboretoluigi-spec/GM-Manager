package server

// Scenari di prova delle due specifiche dell'11 novembre (fase 5 delle richieste in sospeso, 2026-11-12).
// Il testo originale delle specifiche non è nel repository: gli scenari sono RICOSTRUITI dai requisiti riassunti in
// docs/PIANO_COMBATTIMENTO.md (Parte 1: combattimento, 20 scenari; Parte 2: incantesimi, 28 scenari), con l'accordo
// dell'utente. Elenco e corrispondenze in docs/SCENARI_COMBATTIMENTO.md.
// Ogni scenario parte da uno scontro nuovo e usa solo i comandi che usano le pagine (stessi permessi); i dadi sono decisi
// con tiriDecisi (ordine: d20 dell'attacco, poi i dadi dei danni; negli incantesimi con TS prima i danni, poi i d20 dei
// bersagli nell'ordine dato). Si eseguono con: go test ./internal/server -run Scenari -v

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

type scena struct {
	*scontro
	t   *testing.T
	idA string            // Arin, il guerriero di Luca
	id  map[string]string // nome del mostro → id
}

// nuovaScena: Arin (Guerriero 3°, CA 16, 40 PF, Destrezza 14) con quattro attacchi — 0 Spada lunga (+5, 1d8+3 tagliente),
// 1 Spada di fuoco +1 (magica, +6, 1d8+4 tagliente + 1d6 fuoco), 2 Mazza d'argento (+5, 1d6+3 contundente, argentata),
// 3 Arco lungo (+5, a distanza, 1d8+3 perforante) — più i mostri indicati, con l'iniziativa già decisa (Arin per primo,
// poi i mostri nell'ordine) e lo scontro avviato. «extra» cambia o aggiunge voci del riepilogo della scheda.
func nuovaScena(t *testing.T, extra map[string]any, mostri ...map[string]any) *scena {
	t.Helper()
	sc := nuovoScontro(t)
	danni := func(dadi string, mod int, tipo string) map[string]any {
		return map[string]any{"dadi": dadi, "mod": mod, "tipo": tipo}
	}
	rp := map[string]any{"class": "Guerriero", "level": 3, "ac": 16, "hpMax": 40, "initiative": 2, "dex": 14,
		"abilities": map[string]any{"FOR": 16, "DES": 14, "COS": 14, "SAG": 10}, "saveProfs": []string{"FOR", "COS"}, "attacksPerAction": 1,
		"attacks": []any{
			map[string]any{"nome": "Spada lunga", "bonus": 5, "portata": "1,5 m", "danni": []any{danni("1d8", 3, "tagliente")}},
			map[string]any{"nome": "Spada di fuoco +1", "bonus": 6, "portata": "1,5 m", "magica": true, "danni": []any{danni("1d8", 4, "tagliente"), danni("1d6", 0, "fuoco")}},
			map[string]any{"nome": "Mazza d'argento", "bonus": 5, "portata": "1,5 m", "argentata": true, "danni": []any{danni("1d6", 3, "contundente")}},
			map[string]any{"nome": "Arco lungo", "bonus": 5, "portata": "45/180 m", "danni": []any{danni("1d8", 3, "perforante")}}}}
	maps.Copy(rp, extra)
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"lvl": 3}, "riepilogo": rp})
	s := &scena{scontro: sc, t: t, id: map[string]string{}}
	for _, m := range mostri {
		d := map[string]any{"quanti": 1}
		maps.Copy(d, m)
		sc.az(sc.m, 200, "iniz.aggiungi_mostri", d)
	}
	vm := sc.vista(sc.m)
	s.idA = cerca(vm, "Arin")["id"].(string)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": s.idA, "valore": 30})
	for i, m := range mostri {
		n := m["nome"].(string)
		s.id[n] = cerca(vm, n)["id"].(string)
		sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": s.id[n], "valore": 20 - i})
	}
	sc.az(sc.m, 200, "iniz.avvia", nil)
	return s
}

// bandito: il mostro di base (CA 12, 30 PF, Scimitarra +3 1d6+1 tagliente, Maglio +5 4d6+8 contundente).
func bandito(nome string, profilo map[string]any) map[string]any {
	p := map[string]any{"attacchi": []any{
		map[string]any{"nome": "Scimitarra", "bonus": 3, "danni": []any{map[string]any{"dadi": "1d6", "mod": 1, "tipo": "tagliente"}}},
		map[string]any{"nome": "Maglio", "bonus": 5, "danni": []any{map[string]any{"dadi": "4d6", "mod": 8, "tipo": "contundente"}}}}}
	maps.Copy(p, profilo)
	return map[string]any{"nome": nome, "ca": 12, "pf": 30, "profilo": p}
}

// azione: identificativo unico in tutto il pacchetto (il server ricorda le azioni già eseguite: doppio clic).
var azioniScenari atomic.Int64

func (s *scena) azione() string { return fmt.Sprintf("scen-%06d", azioniScenari.Add(1)) }

// attacca: «chi» fa attaccare il combattente «da» (id) contro «a» (id); dati in più (attacco, modo, forza, incantesimo).
func (s *scena) attacca(chi *cliente, codice int, da, a string, dati map[string]any, dadi ...int) map[string]any {
	s.t.Helper()
	if dadi != nil {
		tiriDecisi(s.t, dadi...)
	}
	d := map[string]any{"combattente": da, "bersaglio": a, "azione": s.azione()}
	maps.Copy(d, dati)
	return s.az(chi, codice, "attacco.tira", d)
}

func (s *scena) critico(chi *cliente, codice int, gettone, modo string, dadi ...int) map[string]any {
	s.t.Helper()
	tiriDecisi(s.t, dadi...)
	return s.az(chi, codice, "attacco.critico", map[string]any{"gettone": gettone, "modo": modo, "azione": s.azione()})
}

// ts: un incantesimo con tiro salvezza; restituisce la risposta (per il Master righe e gettone).
func (s *scena) ts(chi *cliente, codice int, da string, inc map[string]any, bersagli []string, dadi ...int) map[string]any {
	s.t.Helper()
	tiriDecisi(s.t, dadi...)
	return s.az(chi, codice, "incantesimo.ts", map[string]any{"combattente": da, "incantesimo": inc, "bersagli": bersagli, "forza": chi == s.m, "azione": s.azione()})
}

// inAttesa: il primo incantesimo che aspetta la conferma del Master.
func (s *scena) inAttesa() map[string]any {
	s.t.Helper()
	l, _ := s.vista(s.m)["incantesimiInAttesa"].([]any)
	if len(l) == 0 {
		s.t.Fatal("nessun incantesimo in attesa")
	}
	return l[0].(map[string]any)
}

func (s *scena) applica(chi *cliente, codice int, gettone string, dati map[string]any) map[string]any {
	s.t.Helper()
	d := map[string]any{"gettone": gettone, "applicaCondizioni": true, "azione": s.azione()}
	maps.Copy(d, dati)
	return s.az(chi, codice, "incantesimo.applica", d)
}

func (s *scena) pf(nome string) float64 { return cerca(s.vista(s.m), nome)["pf"].(float64) }
func (s *scena) cond(nome string) []any {
	c, _ := cerca(s.vista(s.m), nome)["condizioni"].([]any)
	return c
}
func (s *scena) effetti() []any { x, _ := s.vista(s.m)["effetti"].([]any); return x }
func (s *scena) economia(c *cliente, nome string) map[string]any {
	return cerca(s.vista(c), nome)["economia"].(map[string]any)
}
func (s *scena) registro() string {
	var l []string
	for _, x := range s.vista(s.m)["registro"].([]any) {
		l = append(l, x.(string))
	}
	return strings.Join(l, "\n")
}
func riga0(e map[string]any) map[string]any { return e["righe"].([]any)[0].(map[string]any) }
func (s *scena) attende(ok bool, formato string, args ...any) {
	s.t.Helper()
	if !ok {
		s.t.Fatalf(formato, args...)
	}
}

// ===== Parte 1: combattimento (20 scenari) =====

func TestScenariCombattimento(t *testing.T) {
	t.Run("C01 attacco che colpisce: danni del tipo dell'arma, PF e registro", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], map[string]any{"attacco": 0}, 10, 5) // 10+5=15 contro 12; 5+3=8
		s.attende(e["risultato"] == "colpito" && e["totale"].(float64) == 15 && e["totaleDanni"].(float64) == 8, "esito: %v", e)
		s.attende(e["danni"].([]any)[0].(map[string]any)["tipo"] == "tagliente", "tipo di danno: %v", e["danni"])
		s.attende(s.pf("Bandito") == 22, "PF del bandito: %v", s.pf("Bandito"))
		s.attende(strings.Contains(s.registro(), "Bandito subisce 8 danni"), "registro: %s", s.registro())
	})
	t.Run("C02 attacco sotto la CA: mancato, nessun danno", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 3) // 8 contro 12
		s.attende(e["risultato"] == "mancato" && e["totaleDanni"].(float64) == 0 && s.pf("Bandito") == 30, "mancato: %v", e)
	})
	t.Run("C03 totale pari alla CA: colpisce", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 7, 2) // 12 contro 12
		s.attende(e["risultato"] == "colpito" && s.pf("Bandito") == 25, "pari alla CA: %v", e)
	})
	t.Run("C04 1 naturale: manca sempre, anche contro una CA bassissima", func(t *testing.T) {
		m := bandito("Melma", nil)
		m["ca"] = 5
		s := nuovaScena(t, nil, m)
		e := s.attacca(s.luca, 200, s.idA, s.id["Melma"], nil, 1) // 1+5=6 supererebbe la CA 5
		s.attende(e["risultato"] == "mancato" && s.pf("Melma") == 30, "1 naturale: %v", e)
	})
	t.Run("C05 20 naturale: critico in due passi, dadi tirati due volte", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 20)
		g, _ := e["gettone"].(string)
		s.attende(e["risultato"] == "critico" && g != "" && e["danni"] == nil && s.pf("Bandito") == 30, "prima della scelta: %v", e)
		e = s.critico(s.luca, 200, g, "dadi", 4, 6) // 4+6+3 = 13
		s.attende(e["totaleDanni"].(float64) == 13 && e["modoCritico"] != nil && s.pf("Bandito") == 17, "critico con i dadi due volte: %v", e)
		s.attende(strings.Contains(s.registro(), "COLPO CRITICO"), "registro del critico")
	})
	t.Run("C06 critico con la variante «raddoppia il risultato»", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		g := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 20)["gettone"].(string)
		e := s.critico(s.luca, 200, g, "raddoppia", 6) // 6×2+3 = 15 (con i dadi due volte sarebbe 6+8+3 = 17)
		s.attende(e["totaleDanni"].(float64) == 15 && s.pf("Bandito") == 15, "raddoppia: %v", e)
	})
	t.Run("C07 doppio clic e gettone monouso: i danni si applicano una volta", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		d := map[string]any{"combattente": s.idA, "bersaglio": s.id["Bandito"], "attacco": 0, "azione": s.azione()}
		tiriDecisi(t, 15, 4)
		s.az(s.luca, 200, "attacco.tira", d)
		s.az(s.luca, 200, "attacco.tira", d)
		s.attende(s.pf("Bandito") == 23, "doppio clic: %v", s.pf("Bandito"))
		g := s.attacca(s.m, 200, s.idA, s.id["Bandito"], map[string]any{"forza": true}, 20)["gettone"].(string)
		s.critico(s.m, 200, g, "dadi", 1, 1)
		s.critico(s.m, 409, g, "dadi", 8, 8)
		s.attende(s.pf("Bandito") == 18, "gettone usato una volta: %v", s.pf("Bandito"))
	})
	t.Run("C08 resistenza: metà dei danni, per difetto", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Scheletro", map[string]any{"difese": map[string]any{"res": []any{map[string]any{"tipo": "tagliente"}}}}))
		e := s.attacca(s.luca, 200, s.idA, s.id["Scheletro"], nil, 15, 6) // 9 → 4
		s.attende(e["totaleDanni"].(float64) == 4 && strings.Contains(fmt.Sprint(e["danni"]), "resistenza"), "resistenza: %v", e)
	})
	t.Run("C09 vulnerabilità: danni doppi", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Scheletro", map[string]any{"difese": map[string]any{"vuln": []any{map[string]any{"tipo": "contundente"}}}}))
		e := s.attacca(s.luca, 200, s.idA, s.id["Scheletro"], map[string]any{"attacco": 2}, 15, 4) // 7 → 14
		s.attende(e["totaleDanni"].(float64) == 14 && s.pf("Scheletro") == 16, "vulnerabilità: %v", e)
	})
	t.Run("C10 immunità: nessun danno", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Golem", map[string]any{"difese": map[string]any{"imm": []any{map[string]any{"tipo": "tagliente"}}}}))
		e := s.attacca(s.luca, 200, s.idA, s.id["Golem"], nil, 15, 8)
		s.attende(e["risultato"] == "colpito" && e["totaleDanni"].(float64) == 0 && s.pf("Golem") == 30, "immunità: %v", e)
	})
	t.Run("C11 danni di più tipi: ognuno con la sua difesa", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Salamandra", map[string]any{"difese": map[string]any{"res": []any{map[string]any{"tipo": "fuoco"}}}}))
		e := s.attacca(s.luca, 200, s.idA, s.id["Salamandra"], map[string]any{"attacco": 1}, 15, 5, 5) // 9 tagliente + 5 fuoco → 2
		s.attende(e["totaleDanni"].(float64) == 11 && len(e["danni"].([]any)) == 2, "più tipi: %v", e)
	})
	t.Run("C12 armi magiche e argentate contro le difese «non magici (non argentati)»", func(t *testing.T) {
		fisici := []any{}
		for _, x := range []string{"contundente", "perforante", "tagliente"} {
			fisici = append(fisici, map[string]any{"tipo": x, "nonMagici": true, "nonArgentate": true})
		}
		s := nuovaScena(t, nil, bandito("Lupo mannaro", map[string]any{"difese": map[string]any{"imm": fisici}}))
		lm := s.id["Lupo mannaro"]
		e := s.attacca(s.m, 200, s.idA, lm, map[string]any{"attacco": 0, "forza": true}, 15, 5)
		s.attende(e["totaleDanni"].(float64) == 0, "arma comune: %v", e)
		e = s.attacca(s.m, 200, s.idA, lm, map[string]any{"attacco": 2, "forza": true}, 15, 4)
		s.attende(e["totaleDanni"].(float64) == 7, "arma argentata: %v", e)
		e = s.attacca(s.m, 200, s.idA, lm, map[string]any{"attacco": 1, "forza": true}, 15, 5, 3)
		s.attende(e["totaleDanni"].(float64) == 12 && s.pf("Lupo mannaro") == 11, "arma magica: %v", e)
	})
	t.Run("C13 vantaggio e svantaggio: due d20, vale il più alto o il più basso", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], map[string]any{"modo": "vantaggio"}, 4, 17, 1)
		s.attende(len(e["d20"].([]any)) == 2 && e["naturale"].(float64) == 17, "vantaggio: %v", e)
		e = s.attacca(s.m, 200, s.idA, s.id["Bandito"], map[string]any{"modo": "svantaggio", "forza": true}, 17, 4)
		s.attende(len(e["d20"].([]any)) == 2 && e["naturale"].(float64) == 4 && e["risultato"] == "mancato", "svantaggio: %v", e)
	})
	t.Run("C14 economia del turno: l'attacco usa l'azione, un solo attacco senza Attacco Extra", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 3)
		s.attende(s.economia(s.luca, "Arin")["a"] == true, "azione usata")
		s.attacca(s.luca, 409, s.idA, s.id["Bandito"], nil)
	})
	t.Run("C15 Attacco Extra: due attacchi con la stessa azione", func(t *testing.T) {
		s := nuovaScena(t, map[string]any{"attacksPerAction": 2}, bandito("Bandito", nil))
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 3)
		s.attende(e["attacchiRimasti"].(float64) == 1, "ne resta uno: %v", e)
		e = s.attacca(s.luca, 200, s.idA, s.id["Bandito"], map[string]any{"attacco": 3}, 3)
		s.attende(e["attacchiRimasti"].(float64) == 0, "finiti: %v", e)
		s.attacca(s.luca, 409, s.idA, s.id["Bandito"], nil)
	})
	t.Run("C16 Multiattacco del mostro: la sequenza (un becco e due artigli) e la conferma oltre", func(t *testing.T) {
		danni := func(d, tipo string) []any { return []any{map[string]any{"dadi": d, "mod": 0, "tipo": tipo}} }
		s := nuovaScena(t, nil, map[string]any{"nome": "Orso gufo", "ca": 13, "pf": 59, "profilo": map[string]any{"multi": 3, "attacchi": []any{
			map[string]any{"nome": "Becco", "bonus": 7, "danni": danni("1d10", "perforante")}, map[string]any{"nome": "Artigli", "bonus": 7, "danni": danni("2d8", "tagliente")}},
			"sequenza": []any{map[string]any{"attacco": 0, "volte": 1}, map[string]any{"attacco": 1, "volte": 2}}}})
		s.az(s.m, 200, "iniz.prossimo_turno", nil) // turno dell'orso
		o := s.id["Orso gufo"]
		s.attacca(s.m, 200, o, s.idA, map[string]any{"attacco": 0}, 2)
		e := s.attacca(s.m, 200, o, s.idA, map[string]any{"attacco": 0}, 2)
		s.attende(e["serveConferma"] != nil && strings.Contains(e["serveConferma"].(string), "1 Becco e 2 Artigli"), "secondo becco: %v", e)
		s.attacca(s.m, 200, o, s.idA, map[string]any{"attacco": 1}, 2)
		e = s.attacca(s.m, 200, o, s.idA, map[string]any{"attacco": 1}, 2)
		s.attende(e["serveConferma"] == nil && e["attacchiRimasti"].(float64) == 0, "sequenza completa: %v", e)
	})
	t.Run("C17 attacco fuori turno: usa la reazione (opportunità); il secondo chiede conferma", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		b := s.id["Bandito"]
		s.attacca(s.m, 200, b, s.idA, nil, 2)
		eco := s.economia(s.m, "Bandito")
		s.attende(eco["r"] == true && eco["a"] == false, "reazione usata: %v", eco)
		s.attende(s.attacca(s.m, 200, b, s.idA, nil)["serveConferma"] != nil, "seconda reazione: conferma")
		s.attende(s.attacca(s.m, 200, b, s.idA, map[string]any{"forza": true}, 2)["risultato"] == "mancato", "forzato dal Master")
	})
	t.Run("C18 permessi: il giocatore attacca solo con il suo PG e nel suo turno", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		b := s.id["Bandito"]
		s.attacca(s.luca, 403, b, s.idA, nil)      // non muove i mostri
		s.attacca(s.sara, 403, s.idA, b, nil)      // la Segretaria non attacca con Arin
		s.attacca(s.ugo, 404, s.idA, b, nil)       // Ugo non è nella sessione
		s.az(s.m, 200, "iniz.prossimo_turno", nil) // turno del bandito
		s.attacca(s.luca, 409, s.idA, b, nil)      // fuori dal suo turno
	})
	t.Run("C19 visibilità: ai giocatori niente CA e PF dei mostri", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 15, 4)
		s.attende(e["ca"] == nil && e["pfDopo"] == nil && e["totaleDanni"].(float64) == 7, "esito per il giocatore: %v", e)
		bl := cerca(s.vista(s.luca), "Bandito")
		s.attende(bl["pf"] == nil && bl["ca"] == nil && bl["salute"] == "ferito", "vista del giocatore: %v", bl)
		e = s.attacca(s.m, 200, s.id["Bandito"], s.idA, nil, 15, 2)
		s.attende(e["ca"].(float64) == 16 && e["pfDopo"].(float64) == 37, "esito per il Master: %v", e)
	})
	t.Run("C20 a 0 PF: mostro sconfitto, PG a terra con i PF condivisi con la scheda", func(t *testing.T) {
		m := bandito("Bandito", nil)
		m["pf"] = 5
		s := nuovaScena(t, nil, m)
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 15, 6)
		s.attende(e["sconfitto"] == true && s.pf("Bandito") == 0 && cerca(s.vista(s.luca), "Bandito")["salute"] == "a terra", "mostro a terra: %v", e)
		s.az(s.m, 200, "pg.danno", map[string]any{"combattente": s.idA, "valore": 35})
		e = s.attacca(s.m, 200, s.id["Bandito"], s.idA, map[string]any{"forza": true}, 15, 6) // 7 danni su 5 PF
		s.attende(e["sconfitto"] == true && cerca(s.vista(s.luca), "Arin")["pf"].(float64) == 0, "PG a terra: %v", e)
	})
}

// ===== Parte 2: incantesimi (28 scenari) =====

func TestScenariIncantesimi(t *testing.T) {
	raggioGelo := func(lancio string) map[string]any {
		return map[string]any{"nome": "Raggio di gelo", "livello": 0, "tempo": "azione", "tipo": "attacco", "bonus": 5, "lancio": lancio,
			"danni": []any{map[string]any{"dadi": "1d8", "tipo": "freddo"}}}
	}
	palla := map[string]any{"nome": "Palla di fuoco", "livello": 3, "slot": 3, "tempo": "azione", "tipo": "ts", "cd": 13, "car": "DES", "seRiesce": "meta",
		"danni": []any{map[string]any{"dadi": "8d6", "tipo": "fuoco"}}}
	blocca := map[string]any{"nome": "Blocca persone", "livello": 2, "slot": 2, "tempo": "azione", "conc": true, "round": 10, "tipo": "ts", "cd": 13, "car": "SAG",
		"condizioni": []string{"Paralizzato"}}
	tsBandito := map[string]any{"ts": map[string]any{"DES": 1, "SAG": 0, "COS": 1}}

	t.Run("I01 attacco con incantesimo: colpisce, danni del suo tipo, usa l'azione", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], map[string]any{"incantesimo": raggioGelo("lancio-i01")}, 12, 6)
		s.attende(e["risultato"] == "colpito" && e["totaleDanni"].(float64) == 6 && e["danni"].([]any)[0].(map[string]any)["tipo"] == "freddo", "raggio di gelo: %v", e)
		s.attende(s.economia(s.luca, "Arin")["a"] == true && s.pf("Bandito") == 24, "azione e PF")
	})
	t.Run("I02 attacco con incantesimo: 1 naturale manca", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], map[string]any{"incantesimo": raggioGelo("lancio-i02")}, 1)
		s.attende(e["risultato"] == "mancato" && s.pf("Bandito") == 30, "1 naturale: %v", e)
	})
	t.Run("I03 attacco con incantesimo: critico con i dadi due volte", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		g := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], map[string]any{"incantesimo": raggioGelo("lancio-i03")}, 20)["gettone"].(string)
		e := s.critico(s.luca, 200, g, "dadi", 5, 6)
		s.attende(e["totaleDanni"].(float64) == 11 && s.pf("Bandito") == 19, "critico dell'incantesimo: %v", e)
	})
	t.Run("I04 più raggi: un attacco per raggio, l'azione una volta sola", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		rr := map[string]any{"nome": "Raggio rovente", "livello": 2, "slot": 2, "tempo": "azione", "tipo": "attacco", "bonus": 5, "raggi": 3, "lancio": "lancio-i04",
			"danni": []any{map[string]any{"dadi": "2d6", "tipo": "fuoco"}}}
		for i, atteso := range []float64{2, 1, 0} {
			e := s.attacca(s.luca, 200, s.idA, s.id["Bandito"], map[string]any{"incantesimo": rr}, 15, 1, 1)
			s.attende(e["attacchiRimasti"].(float64) == atteso && e["totaleDanni"].(float64) == 2, "raggio %d: %v", i+1, e)
		}
		s.attacca(s.luca, 409, s.idA, s.id["Bandito"], map[string]any{"incantesimo": rr})
		s.attende(s.pf("Bandito") == 24, "tre raggi da 2: %v", s.pf("Bandito"))
	})
	t.Run("I05 TS su più bersagli: esiti riservati al Master, PF fermi fino alla conferma", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito A", tsBandito), bandito("Bandito B", tsBandito))
		r := s.ts(s.luca, 200, s.idA, palla, []string{s.id["Bandito A"], s.id["Bandito B"]}, 3, 3, 3, 3, 3, 3, 3, 3, 5, 15)
		s.attende(r["inAttesa"] == true && r["righe"] == nil, "il giocatore aspetta: %v", r)
		s.attende(s.vista(s.luca)["incantesimiInAttesa"] == nil, "i giocatori non vedono i tiri dei mostri")
		p := s.inAttesa()
		s.attende(len(p["righe"].([]any)) == 2 && s.pf("Bandito A") == 30 && s.pf("Bandito B") == 30, "in attesa: %v", p)
	})
	t.Run("I06 «metà se riesce»: danni tirati una volta, metà per difetto a chi riesce", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito A", tsBandito), bandito("Bandito B", tsBandito))
		s.ts(s.luca, 200, s.idA, palla, []string{s.id["Bandito A"], s.id["Bandito B"]}, 3, 3, 3, 3, 3, 3, 3, 4, 5, 15) // 25; A 6 fallisce, B 16 riesce
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		s.attende(s.pf("Bandito A") == 5 && s.pf("Bandito B") == 18, "metà: %v %v", s.pf("Bandito A"), s.pf("Bandito B"))
	})
	t.Run("I07 «nessun effetto se riesce»", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito A", tsBandito), bandito("Bandito B", tsBandito))
		inc := map[string]any{"nome": "Raggio di malattia", "livello": 1, "slot": 1, "tempo": "azione", "tipo": "ts", "cd": 13, "car": "COS",
			"danni": []any{map[string]any{"dadi": "2d8", "tipo": "veleno"}}, "condizioni": []string{"Avvelenato"}}
		s.ts(s.luca, 200, s.idA, inc, []string{s.id["Bandito A"], s.id["Bandito B"]}, 5, 5, 2, 15)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		s.attende(s.pf("Bandito A") == 20 && s.pf("Bandito B") == 30, "PF: %v %v", s.pf("Bandito A"), s.pf("Bandito B"))
		s.attende(slices.Contains(s.cond("Bandito A"), any("Avvelenato")) && len(s.cond("Bandito B")) == 0, "condizioni: %v %v", s.cond("Bandito A"), s.cond("Bandito B"))
	})
	t.Run("I08 il Master cambia un esito prima di applicare", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		s.ts(s.luca, 200, s.idA, palla, []string{s.id["Bandito"]}, 3, 3, 3, 3, 3, 3, 3, 3, 2)
		p := s.inAttesa()
		s.attende(riga0(p)["riuscito"] == false, "fallito: %v", p)
		s.applica(s.m, 200, p["gettone"].(string), map[string]any{"riusciti": map[string]bool{s.id["Bandito"]: true}})
		s.attende(s.pf("Bandito") == 18, "con il TS riuscito per decisione del Master: %v", s.pf("Bandito"))
	})
	t.Run("I09 la conferma è solo del Master e vale una volta", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		s.ts(s.luca, 200, s.idA, palla, []string{s.id["Bandito"]}, 3, 3, 3, 3, 3, 3, 3, 3, 2)
		g := s.inAttesa()["gettone"].(string)
		s.applica(s.luca, 403, g, nil)
		s.applica(s.m, 200, g, nil)
		s.applica(s.m, 409, g, nil)
		s.attende(s.pf("Bandito") == 6, "applicato una volta: %v", s.pf("Bandito"))
	})
	t.Run("I10 Resistenza alla magia: vantaggio ai TS contro gli incantesimi", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Lupo", map[string]any{"ts": map[string]any{"DES": 2}, "resMagia": true}))
		s.ts(s.luca, 200, s.idA, palla, []string{s.id["Lupo"]}, 3, 3, 3, 3, 3, 3, 3, 3, 4, 12)
		r := riga0(s.inAttesa())
		s.attende(len(r["d20"].([]any)) == 2 && r["totale"].(float64) == 14 && r["riuscito"] == true && r["resMagia"] == true, "resistenza alla magia: %v", r)
	})
	t.Run("I11 Resistenza leggendaria: il Master la vede e trasforma il fallimento in successo", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Vampiro", map[string]any{"ts": map[string]any{"SAG": 0}, "leggendaria": 3}))
		s.ts(s.luca, 200, s.idA, blocca, []string{s.id["Vampiro"]}, 2)
		p := s.inAttesa()
		r := riga0(p)
		s.attende(r["riuscito"] == false && r["leggendaria"].(float64) == 3, "riga del vampiro: %v", r)
		s.applica(s.m, 200, p["gettone"].(string), map[string]any{"riusciti": map[string]bool{s.id["Vampiro"]: true}})
		s.attende(len(s.cond("Vampiro")) == 0, "nessuna paralisi: %v", s.cond("Vampiro"))
	})
	t.Run("I12 immunità alle condizioni: la condizione non si applica", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Zombi", map[string]any{"ts": map[string]any{"COS": 0}, "immCond": []string{"Avvelenato"}}))
		inc := map[string]any{"nome": "Nube velenosa", "tempo": "azione", "tipo": "ts", "cd": 13, "car": "COS", "condizioni": []string{"Avvelenato", "Prono"}}
		s.ts(s.luca, 200, s.idA, inc, []string{s.id["Zombi"]}, 2)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		c := s.cond("Zombi")
		s.attende(len(c) == 1 && c[0] == "Prono", "solo prono: %v", c)
	})
	t.Run("I13 condizioni a chi fallisce, nessuna a chi riesce", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito A", tsBandito), bandito("Bandito B", tsBandito))
		s.ts(s.luca, 200, s.idA, blocca, []string{s.id["Bandito A"], s.id["Bandito B"]}, 2, 18)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		s.attende(slices.Contains(s.cond("Bandito A"), any("Paralizzato")) && len(s.cond("Bandito B")) == 0, "paralisi: %v %v", s.cond("Bandito A"), s.cond("Bandito B"))
	})
	t.Run("I14 concentrazione: l'incantatore è «Concentrato», l'effetto è in corso", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		s.ts(s.luca, 200, s.idA, blocca, []string{s.id["Bandito"]}, 2)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		ef := s.effetti()
		s.attende(slices.Contains(s.cond("Arin"), any("Concentrato")) && len(ef) == 1 && ef[0].(map[string]any)["conc"] == true, "concentrazione: %v %v", s.cond("Arin"), ef)
	})
	t.Run("I15 un nuovo incantesimo a concentrazione termina il precedente", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito A", tsBandito), bandito("Bandito B", tsBandito))
		s.ts(s.luca, 200, s.idA, blocca, []string{s.id["Bandito A"]}, 2)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		s.az(s.m, 200, "iniz.prossimo_turno", nil)
		s.az(s.m, 200, "iniz.prossimo_turno", nil)
		s.az(s.m, 200, "iniz.prossimo_turno", nil) // round 2, di nuovo Arin (il TS ripetuto non c'è: resta paralizzato)
		s.ts(s.luca, 200, s.idA, blocca, []string{s.id["Bandito B"]}, 2)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		s.attende(len(s.cond("Bandito A")) == 0 && slices.Contains(s.cond("Bandito B"), any("Paralizzato")) && len(s.effetti()) == 1,
			"il primo finisce: %v %v %v", s.cond("Bandito A"), s.cond("Bandito B"), s.effetti())
	})
	t.Run("I16 TS di concentrazione dopo i danni: CD 10 o metà dei danni", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		s.ts(s.luca, 200, s.idA, blocca, []string{s.id["Bandito"]}, 18) // il bandito riesce: nessun effetto; Arin concentrato a mano
		s.az(s.m, 200, "pg.condizioni", map[string]any{"combattente": s.idA, "condizioni": []string{"Concentrato"}})
		e := s.attacca(s.m, 200, s.id["Bandito"], s.idA, map[string]any{"attacco": 0, "forza": true}, 15, 3)
		s.attende(e["concentrazione"].(float64) == 10, "CD 10 con 4 danni: %v", e)
		e = s.attacca(s.m, 200, s.id["Bandito"], s.idA, map[string]any{"attacco": 1, "forza": true}, 15, 6, 6, 6, 6) // 32 danni
		s.attende(e["concentrazione"].(float64) == 16, "CD 16 con 32 danni: %v", e)
	})
	t.Run("I17 concentrazione persa: cadono le condizioni dell'effetto", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		s.ts(s.luca, 200, s.idA, blocca, []string{s.id["Bandito"]}, 2)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		s.az(s.luca, 200, "pg.condizioni", map[string]any{"combattente": s.idA, "condizioni": []string{}})
		s.attende(len(s.cond("Bandito")) == 0 && len(s.effetti()) == 0, "liberato: %v", s.cond("Bandito"))
	})
	t.Run("I18 a 0 PF l'incantatore perde la concentrazione", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito), bandito("Bruto", nil))
		s.ts(s.luca, 200, s.idA, blocca, []string{s.id["Bandito"]}, 2)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		s.az(s.m, 200, "pg.danno", map[string]any{"combattente": s.idA, "valore": 35})
		s.attacca(s.m, 200, s.id["Bruto"], s.idA, map[string]any{"forza": true}, 15, 6)
		s.attende(len(s.cond("Bandito")) == 0 && len(s.effetti()) == 0 && len(s.cond("Arin")) == 0, "a 0 PF: %v %v", s.cond("Bandito"), s.cond("Arin"))
	})
	t.Run("I19 TS ripetuto alla fine del turno: il bersaglio si libera", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		inc := maps.Clone(blocca)
		inc["ripeti"] = true
		s.ts(s.luca, 200, s.idA, inc, []string{s.id["Bandito"]}, 2)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		s.az(s.m, 200, "iniz.prossimo_turno", nil)
		tiriDecisi(t, 17)
		s.az(s.m, 200, "iniz.prossimo_turno", nil) // fine del turno del bandito: 17 riesce
		s.attende(len(s.cond("Bandito")) == 0 && len(s.effetti()) == 0, "liberato: %v", s.cond("Bandito"))
	})
	t.Run("I20 durata in round: l'effetto finisce da solo", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		inc := map[string]any{"nome": "Ragnatela", "tempo": "azione", "round": 1, "tipo": "ts", "cd": 13, "car": "DES", "condizioni": []string{"Trattenuto"}}
		s.ts(s.luca, 200, s.idA, inc, []string{s.id["Bandito"]}, 2)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		s.az(s.m, 200, "iniz.prossimo_turno", nil)
		s.attende(len(s.cond("Bandito")) == 1, "ancora trattenuto nel suo turno")
		s.az(s.m, 200, "iniz.prossimo_turno", nil) // round 2, turno di Arin
		s.attende(len(s.cond("Bandito")) == 0 && len(s.effetti()) == 0, "finito: %v", s.cond("Bandito"))
	})
	t.Run("I21 il Master termina un effetto, il giocatore no", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		s.ts(s.luca, 200, s.idA, blocca, []string{s.id["Bandito"]}, 2)
		s.applica(s.m, 200, s.inAttesa()["gettone"].(string), nil)
		id := s.effetti()[0].(map[string]any)["id"].(string)
		s.az(s.luca, 403, "effetto.termina", map[string]any{"effetto": id})
		s.az(s.m, 200, "effetto.termina", map[string]any{"effetto": id})
		s.attende(len(s.cond("Bandito")) == 0 && len(s.effetti()) == 0, "terminato dal Master")
	})
	t.Run("I22 cura: dadi più modificatore, mai oltre i PF massimi", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		cura := map[string]any{"nome": "Cura ferite", "livello": 1, "slot": 1, "tempo": "azione", "tipo": "cura", "cura": map[string]any{"dadi": "1d8", "mod": 3}}
		s.az(s.m, 200, "pg.danno", map[string]any{"combattente": s.idA, "valore": 5})
		tiriDecisi(t, 8)
		e := s.az(s.luca, 200, "incantesimo.cura", map[string]any{"combattente": s.idA, "incantesimo": cura, "bersagli": []string{s.idA}, "azione": s.azione()})
		s.attende(s.pf("Arin") == 40, "PF al massimo, non 46: %v %v", s.pf("Arin"), e)
	})
	t.Run("I23 PF temporanei: non si sommano, resta il valore più alto", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		vf := func(d int) {
			tiriDecisi(t, d)
			s.az(s.m, 200, "incantesimo.cura", map[string]any{"combattente": s.idA, "incantesimo": map[string]any{"nome": "Vita falsa", "livello": 1, "slot": 1, "tempo": "altro",
				"tipo": "pftemp", "cura": map[string]any{"dadi": "1d4", "mod": 4}}, "bersagli": []string{s.idA}, "azione": s.azione()})
		}
		vf(3)
		s.attende(cerca(s.vista(s.m), "Arin")["pfTemp"].(float64) == 7, "7 temporanei")
		vf(1)
		s.attende(cerca(s.vista(s.m), "Arin")["pfTemp"].(float64) == 7, "5 non sostituisce 7")
		e := s.attacca(s.m, 200, s.id["Bandito"], s.idA, map[string]any{"forza": true}, 15, 5) // 6 danni: prima i temporanei
		s.attende(cerca(s.vista(s.m), "Arin")["pfTemp"].(float64) == 1 && s.pf("Arin") == 40, "i danni tolgono prima i temporanei: %v", e)
	})
	t.Run("I24 valore modificato da un effetto: CA nuova con l'originale visibile, poi torna", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", nil))
		s.az(s.luca, 200, "effetto.nuovo", map[string]any{"combattente": s.idA, "bersagli": []string{s.idA}, "azione": s.azione(),
			"valori": map[string]any{"nome": "Scudo della fede", "tempo": "bonus", "conc": true, "round": 100, "mod": map[string]any{"caBonus": 2}}})
		a := cerca(s.vista(s.luca), "Arin")
		s.attende(a["ca"].(float64) == 18 && a["caOrig"].(float64) == 16, "CA con lo scudo: %v", a)
		e := s.attacca(s.m, 200, s.id["Bandito"], s.idA, map[string]any{"forza": true}, 14) // 17 contro 18: mancato
		s.attende(e["risultato"] == "mancato" && e["ca"].(float64) == 18, "l'attacco usa la CA cambiata: %v", e)
		s.az(s.luca, 200, "pg.condizioni", map[string]any{"combattente": s.idA, "condizioni": []string{}}) // concentrazione persa
		s.attende(cerca(s.vista(s.luca), "Arin")["ca"].(float64) == 16, "CA di nuovo 16")
	})
	t.Run("I25 Armatura magica: CA base 13 + Destrezza, non sommata all'armatura", func(t *testing.T) {
		s := nuovaScena(t, map[string]any{"ac": 12}, bandito("Bandito", nil))
		s.az(s.luca, 200, "effetto.nuovo", map[string]any{"combattente": s.idA, "bersagli": []string{s.idA}, "azione": s.azione(),
			"valori": map[string]any{"nome": "Armatura magica", "tempo": "altro", "round": 4800, "mod": map[string]any{"caBase": 13}}})
		a := cerca(s.vista(s.luca), "Arin")
		s.attende(a["ca"].(float64) == 15 && a["caOrig"].(float64) == 12, "Armatura magica: %v", a)
	})
	t.Run("I26 condizioni nel motore: paralizzato (vantaggio, critico in mischia, TS di Destrezza falliti), stordito non agisce", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		b := s.id["Bandito"]
		s.az(s.m, 200, "mostro.condizioni", map[string]any{"combattente": b, "condizioni": []string{"Paralizzato"}})
		e := s.attacca(s.luca, 200, s.idA, b, nil, 3, 9)
		s.attende(e["modo"] == "vantaggio" && e["risultato"] == "critico", "mischia contro un paralizzato: %v", e)
		e = s.ts(s.m, 200, s.idA, map[string]any{"nome": "Onda tonante", "tempo": "altro", "tipo": "ts", "cd": 13, "car": "DES",
			"danni": []any{map[string]any{"dadi": "2d8", "tipo": "tuono"}}}, []string{b}, 4, 4)
		s.attende(riga0(e)["riuscito"] == false && riga0(e)["d20"] == nil, "TS di Destrezza fallito da solo: %v", riga0(e))
		s.az(s.m, 200, "pg.condizioni", map[string]any{"combattente": s.idA, "condizioni": []string{"Stordito"}})
		s.az(s.m, 200, "iniz.prossimo_turno", nil)
		s.az(s.m, 200, "iniz.prossimo_turno", nil) // round 2, di nuovo Arin
		s.attacca(s.luca, 409, s.idA, b, nil)
	})
	t.Run("I27 tempo di lancio: azione già usata rifiutata, azione bonus permessa", func(t *testing.T) {
		s := nuovaScena(t, nil, bandito("Bandito", tsBandito))
		s.attacca(s.luca, 200, s.idA, s.id["Bandito"], nil, 3)
		s.ts(s.luca, 409, s.idA, blocca, []string{s.id["Bandito"]})
		inc := map[string]any{"nome": "Parola guaritrice", "livello": 1, "slot": 1, "tempo": "bonus", "tipo": "cura", "cura": map[string]any{"dadi": "1d4", "mod": 3}}
		tiriDecisi(t, 2)
		s.az(s.luca, 200, "incantesimo.cura", map[string]any{"combattente": s.idA, "incantesimo": inc, "bersagli": []string{s.idA}, "azione": s.azione()})
		s.attende(s.economia(s.luca, "Arin")["b"] == true, "azione bonus usata")
	})
	t.Run("I28 incantesimi dei mostri: slot e usi al giorno, conferma a slot finiti, solo il Master", func(t *testing.T) {
		s := nuovaScena(t, nil, map[string]any{"nome": "Mago", "ca": 12, "pf": 40, "profilo": map[string]any{"incantesimi": map[string]any{"cd": 15, "att": 7, "mod": 3, "liv": 9,
			"gruppi": []any{map[string]any{"etichetta": "Trucchetti", "livello": 0, "usi": 0, "nomi": []string{"raggio di gelo"}},
				map[string]any{"etichetta": "3°", "livello": 3, "usi": 1, "nomi": []string{"palla di fuoco"}}}}}})
		id := s.id["Mago"]
		usa := func(c *cliente, codice, g int, forza bool) map[string]any {
			return s.az(c, codice, "mostro.incantesimo", map[string]any{"combattente": id, "gruppo": g, "forza": forza})
		}
		s.attende(usa(s.m, 200, 0, false)["rimasti"].(float64) == -1, "trucchetto a volontà")
		s.attende(usa(s.m, 200, 1, false)["rimasti"].(float64) == 0, "ultimo slot di 3°")
		s.attende(usa(s.m, 200, 1, false)["serveConferma"] != nil, "slot finiti: conferma")
		usa(s.luca, 403, 1, false)
		s.attende(cerca(s.vista(s.luca), "Mago")["incantesimi"] == nil, "i giocatori non vedono gli incantesimi del mostro")
	})
}
