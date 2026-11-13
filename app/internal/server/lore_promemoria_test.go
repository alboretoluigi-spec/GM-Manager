package server

import (
	"strings"
	"testing"
	"time"
)

// Fase L3 (2026-11-12): eventi del Codice con «Innesco:» → promemoria per gruppo (arrivo, sessione, giorno).
func TestPromemoriaDelCodice(t *testing.T) {
	sc := nuovoScontro(t)
	cm := sc.clienteWS(sc.m)
	ultimiMessaggi(cm, "")
	md := `# Evento: Il funerale di Borin
Innesco: quando il gruppo arriva al Villaggio di Nebbiavalle

## Per il Master
Il funerale.

# Evento: Visita dell'antagonista
Innesco: alla prossima sessione

# Evento: La nebbia si alza
Innesco: dal giorno 3

# Evento: Temporale
Innesco: quando piove
`
	sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": md, "applica": true})
	v := sc.m.deve(200, "GET", "/lore", nil)
	inn := map[string]any{}
	id := map[string]string{}
	for _, x := range v["schede"].([]any) {
		m := x.(map[string]any)
		inn[m["nome"].(string)] = m["innesco"]
		id[m["nome"].(string)] = m["id"].(string)
	}
	if a := inn["Il funerale di Borin"].(map[string]any); a["tipo"] != "arrivo" || a["luogo"] != "Villaggio di Nebbiavalle" {
		t.Fatalf("innesco d'arrivo: %v", a)
	}
	if inn["Visita dell'antagonista"].(map[string]any)["tipo"] != "sessione" || inn["La nebbia si alza"].(map[string]any)["giorno"].(float64) != 3 || inn["Temporale"] != nil {
		t.Fatalf("inneschi: %v", inn)
	}
	g := v["gruppi"].([]any)[0].(map[string]any)["id"].(string)
	var gSes string
	sc.a.s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, sc.ses).Scan(&gSes)
	if gSes != g {
		for _, x := range v["gruppi"].([]any) {
			if x.(map[string]any)["id"] == gSes {
				g = gSes
			}
		}
	}
	aperti := func() []string {
		var l []string
		for fine := time.Now().Add(2 * time.Second); time.Now().Before(fine); time.Sleep(30 * time.Millisecond) {
			l = nil
			for _, x := range sc.m.deve(200, "GET", "/lore", nil)["promemoria"].([]any) {
				p := x.(map[string]any)
				if p["gruppo"] == g {
					l = append(l, p["nome"].(string))
				}
			}
			if len(l) > 0 {
				break
			}
		}
		return l
	}
	if l := aperti(); len(l) != 0 {
		t.Fatalf("nessun promemoria prima: %v", l)
	}
	// Il gruppo arriva al Villaggio di Nebbiavalle (1249, 622).
	sc.m.deve(200, "POST", "/mappa/party?sessione="+sc.ses, map[string]any{"azione": "posiziona", "x": 1255, "y": 618})
	if l := aperti(); len(l) != 1 || l[0] != "Il funerale di Borin" {
		t.Fatalf("promemoria d'arrivo: %v", l)
	}
	if m := ultimiMessaggi(cm, "lore.promemoria"); len(m) != 1 || !strings.Contains(m[0], "Il funerale di Borin") {
		t.Fatalf("avviso al Master: %v", m)
	}
	// Spostandosi e tornando non si ripete; segnato avvenuto, la scheda diventa «incontrato» per il gruppo.
	sc.m.deve(200, "POST", "/mappa/party?sessione="+sc.ses, map[string]any{"azione": "posiziona", "x": 500, "y": 500})
	sc.m.deve(200, "POST", "/mappa/party?sessione="+sc.ses, map[string]any{"azione": "posiziona", "x": 1250, "y": 622})
	time.Sleep(200 * time.Millisecond)
	if m := ultimiMessaggi(cm, "lore.promemoria"); len(m) != 0 {
		t.Fatalf("un solo promemoria per scheda e gruppo: %v", m)
	}
	sc.luca.deve(403, "POST", "/lore/"+id["Il funerale di Borin"]+"/promemoria", map[string]any{"gruppo": g, "azione": "avvenuto"})
	sc.m.deve(200, "POST", "/lore/"+id["Il funerale di Borin"]+"/promemoria", map[string]any{"gruppo": g, "azione": "avvenuto"})
	var stato string
	sc.a.s.DB.QueryRow(`SELECT stato FROM lore_stato WHERE scheda_id = ? AND gruppo_id = ?`, id["Il funerale di Borin"], g).Scan(&stato)
	if stato != "incontrato" || len(sc.m.deve(200, "GET", "/lore", nil)["promemoria"].([]any)) != 0 {
		t.Fatalf("avvenuto: stato %q", stato)
	}
	// Inizio della sessione.
	sc.m.deve(200, "PATCH", "/sessioni/"+sc.ses, map[string]any{"stato": "in_corso"})
	if l := aperti(); len(l) != 1 || l[0] != "Visita dell'antagonista" {
		t.Fatalf("promemoria di sessione: %v", l)
	}
	sc.m.deve(200, "POST", "/lore/"+id["Visita dell'antagonista"]+"/promemoria", map[string]any{"gruppo": g, "azione": "ignora"})
	// Giorni di gioco: al giorno 2 niente, al giorno 3 sì.
	sc.m.deve(200, "POST", "/mappa/viaggi/giorno?sessione="+sc.ses, map[string]any{})
	if l := aperti(); len(l) != 0 {
		t.Fatalf("giorno 2: %v", l)
	}
	sc.m.deve(200, "POST", "/mappa/viaggi/giorno?sessione="+sc.ses, map[string]any{})
	if l := aperti(); len(l) != 1 || l[0] != "La nebbia si alza" {
		t.Fatalf("giorno 3: %v", l)
	}
	sc.m.deve(400, "POST", "/lore/"+id["La nebbia si alza"]+"/promemoria", map[string]any{"gruppo": g, "azione": "boh"})
}
