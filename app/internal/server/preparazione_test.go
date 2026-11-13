package server

import (
	"strings"
	"testing"
	"time"
)

// Fase L5 (2026-11-12): la preparazione della sessione mette insieme presenti, luogo, riassunto, promemoria, schede
// pertinenti del Codice con il motivo, decisioni aperte e bilanciamenti del gruppo.
func TestPreparazioneSessione(t *testing.T) {
	sc := nuovoScontro(t)
	g := sc.m.deve(200, "GET", "/sessioni/"+sc.ses, nil)["gruppo"].(string)
	sc.luca.deve(200, "POST", "/riassunti", map[string]any{"gruppo": g, "titolo": "Arrivo a Borgofosco", "testo": "Siamo entrati in città al tramonto."})
	md := `# Luogo: Borgofosco
## Per il Master
Città del borgomastro.

# PNG: Madre Maren
Luogo: Borgofosco
Collegamenti: Orfanotrofio di Borgofosco

## Per il Master
Dirige l'orfanotrofio.

# Luogo: Orfanotrofio di Borgofosco
Regione: Borgofosco

# Evento: Il festival
Innesco: quando il gruppo arriva a Borgofosco

# Segreto: Decisioni aperte – Orfanotrofio
Collegamenti: Orfanotrofio di Borgofosco

## Per il Master
- Nome dell'orfanotrofio.

# Luogo: Pietralta
## Per il Master
Lontano.
`
	sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": md, "applica": true})
	// il gruppo arriva a Borgofosco: il promemoria del festival scatta
	sc.m.deve(200, "POST", "/mappa/party?sessione="+sc.ses, map[string]any{"azione": "posiziona", "x": 648, "y": 372})

	sc.luca.deve(403, "GET", "/preparazione?gruppo="+g, nil)
	sc.m.deve(404, "GET", "/preparazione?gruppo=nessuno", nil)
	p := sc.m.deve(200, "GET", "/preparazione?gruppo="+g, nil)
	if p["sessione"].(map[string]any)["id"] != sc.ses || p["luogo"] != "Borgofosco" {
		t.Fatalf("sessione o luogo: %v %v", p["sessione"], p["luogo"])
	}
	pres := p["presenti"].([]any)
	if len(pres) != 1 || pres[0].(map[string]any)["nome"] != "Arin" || pres[0].(map[string]any)["confermato"] != true || pres[0].(map[string]any)["livello"].(float64) != 3 {
		t.Fatalf("presenti: %v", pres)
	}
	if r := p["riassunto"].(map[string]any); r["titolo"] != "Arrivo a Borgofosco" {
		t.Fatalf("riassunto: %v", r)
	}
	// il promemoria nasce in parallelo allo spostamento del segnalino (lore_promemoria.go): si aspetta fino a 2 secondi
	for fine := time.Now().Add(2 * time.Second); len(p["promemoria"].([]any)) == 0 && time.Now().Before(fine); time.Sleep(30 * time.Millisecond) {
		p = sc.m.deve(200, "GET", "/preparazione?gruppo="+g, nil)
	}
	prom := p["promemoria"].([]any)
	if len(prom) != 1 || prom[0].(map[string]any)["nome"] != "Il festival" {
		t.Fatalf("promemoria: %v", prom)
	}
	motivi := map[string]string{}
	for _, x := range p["schede"].([]any) {
		x := x.(map[string]any)
		var m []string
		for _, v := range x["motivi"].([]any) {
			m = append(m, v.(string))
		}
		motivi[x["nome"].(string)] = strings.Join(m, " · ")
	}
	if !strings.Contains(motivi["Madre Maren"], "dove si trova il gruppo (Borgofosco)") || !strings.Contains(motivi["Il festival"], "promemoria aperto") ||
		!strings.Contains(motivi["Orfanotrofio di Borgofosco"], "dove si trova il gruppo") {
		t.Fatalf("motivi: %v", motivi)
	}
	if _, ok := motivi["Pietralta"]; ok {
		t.Fatal("Pietralta non c'entra con il gruppo")
	}
	if _, ok := motivi["Decisioni aperte – Orfanotrofio"]; ok {
		t.Fatal("le decisioni aperte vanno nel loro riquadro")
	}
	dec := p["decisioni"].([]any)
	if len(dec) != 1 || !strings.Contains(dec[0].(map[string]any)["testo"].(string), "Nome dell'orfanotrofio") {
		t.Fatalf("decisioni aperte: %v", dec)
	}
	if d := p["domandaIA"].(string); !strings.Contains(d, "si trova a Borgofosco") || !strings.Contains(d, "«Il festival»") || !strings.Contains(d, "Martedì") {
		t.Fatalf("domanda per l'IA: %s", d)
	}
	// nessun modello di scenario incluso nell'app (i modelli arriveranno dall'utente): nessun bilanciamento salvato
	altro := sc.m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Domenica"})["id"].(string)
	p = sc.m.deve(200, "GET", "/preparazione?gruppo="+g, nil)
	if b, _ := p["bilanciamenti"].([]any); len(b) != 0 {
		t.Fatalf("bilanciamenti: %v", b)
	}
	if b := sc.m.deve(200, "GET", "/preparazione?gruppo="+altro, nil)["bilanciamenti"].([]any); len(b) != 0 {
		t.Fatalf("il bilanciamento di Martedì è finito nel gruppo Domenica: %v", b)
	}
	// senza sessione: i personaggi attivi del gruppo, nessuna sessione
	p = sc.m.deve(200, "GET", "/preparazione?gruppo="+g+"&sessione=-", nil)
	if p["sessione"] != nil || len(p["presenti"].([]any)) != 1 {
		t.Fatalf("senza sessione: %v %v", p["sessione"], p["presenti"])
	}
	// promemoria segnato avvenuto: sparisce dalla preparazione
	sc.m.deve(200, "POST", "/lore/"+schedaPerNome(t, sc.m, "Il festival")+"/promemoria", map[string]any{"gruppo": g, "azione": "avvenuto"})
	if prom := sc.m.deve(200, "GET", "/preparazione?gruppo="+g, nil)["promemoria"].([]any); len(prom) != 0 {
		t.Fatalf("il promemoria resta: %v", prom)
	}
}

func schedaPerNome(t *testing.T, c *cliente, nome string) string {
	t.Helper()
	for _, x := range c.deve(200, "GET", "/lore", nil)["schede"].([]any) {
		if x.(map[string]any)["nome"] == nome {
			return x.(map[string]any)["id"].(string)
		}
	}
	t.Fatalf("scheda %q non trovata", nome)
	return ""
}
