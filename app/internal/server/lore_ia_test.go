package server

import (
	"fmt"
	"strings"
	"testing"
)

// Fase L4 (2026-11-12): l'assistente IA riceve le schede pertinenti del Codice della campagna con le regole d'uso.
func TestCodiceNellAssistente(t *testing.T) {
	sc := nuovoScontro(t)
	var g string
	sc.a.s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, sc.ses).Scan(&g)
	var ultima map[string]any
	var auth string
	ia := iaFinta(t, &ultima, &auth)
	defer ia.Close()
	sc.m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "altro", "indirizzo": ia.URL + "/v1", "modello": "qwen3:8b"})
	sistema := func(domanda string) string {
		t.Helper()
		sc.m.chiediIA(map[string]any{"testo": domanda, "gruppo": g})
		return leggiFinta(&ultima)["messages"].([]any)[0].(map[string]any)["content"].(string)
	}

	// Codice vuoto: nessun blocco.
	if s := sistema("Ciao"); strings.Contains(s, "CODICE DELLA CAMPAGNA") {
		t.Fatal("Codice vuoto: niente blocco")
	}
	// Codice piccolo: tutto, con le regole e lo stato del gruppo.
	md := `# PNG: Lysenne Aldaris
Alias: Madre Maren
Luogo: Orfanotrofio di Borgofosco
Collegamenti: Orfanotrofio di Borgofosco
Etichette: approvato, campagna

## Per il Master
SEGRETO-LYSENNE: è un'elfa.

## Per i giocatori
Una vecchia gentile.

# Luogo: Orfanotrofio di Borgofosco
Regione: Borgofosco
Etichette: approvato, campagna

## Per il Master
Sotto c'è il tempio.

# Segreto: Decisioni aperte – Orfanotrofio di Borgofosco
Collegamenti: Orfanotrofio di Borgofosco
Etichette: da decidere, campagna

## Per il Master
DA-DECIDERE-BAMBINO: chi è il bambino posseduto.
`
	sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": md, "applica": true})
	idL := ""
	for _, x := range sc.m.deve(200, "GET", "/lore", nil)["schede"].([]any) {
		if x.(map[string]any)["nome"] == "Lysenne Aldaris" {
			idL = x.(map[string]any)["id"].(string)
		}
	}
	sc.m.deve(200, "PUT", "/lore/"+idL+"/stato", map[string]any{"gruppo": g, "stato": "incontrato", "nota": "alla festa"})
	s := sistema("Come la presento?")
	for _, x := range []string{"CODICE DELLA CAMPAGNA", "«da decidere» = non stabilito", "SEGRETO-LYSENNE", "### [PNG] Lysenne Aldaris — etichette: approvato, campagna — per questo gruppo: incontrato o avvenuto (alla festa)",
		"Alias: Madre Maren", "DA-DECIDERE-BAMBINO"} {
		if !strings.Contains(s, x) {
			t.Fatalf("manca %q nel sistema:\n%s", x, s)
		}
	}
	if strings.Contains(s, "Mostrate") {
		t.Fatal("Codice piccolo: tutte le schede")
	}

	// Codice grande: 30 schede lunghe. Con la domanda su «Madre Maren» (alias) partono Lysenne, l'orfanotrofio collegato e
	// le sue decisioni aperte; non tutte le altre.
	var b strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "# Luogo: Riempitivo %02d\nEtichette: proposta\n\n## Per il Master\n%s\n\n", i, strings.Repeat("Testo di riempimento. ", 70))
	}
	sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": b.String(), "applica": true})
	s = sistema("Che cosa sa Madre Maren dei bambini?")
	if !strings.Contains(s, "SEGRETO-LYSENNE") || !strings.Contains(s, "### [Luogo] Orfanotrofio di Borgofosco") || !strings.Contains(s, "DA-DECIDERE-BAMBINO") ||
		!strings.Contains(s, "Mostrate ") || strings.Count(s, "### [Luogo] Riempitivo") > 6 {
		t.Fatalf("selezione per alias nel Codice grande:\n%s", s[strings.Index(s, "CODICE DELLA CAMPAGNA"):])
	}
	if i := strings.Index(s, "CODICE DELLA CAMPAGNA"); len([]rune(s[i:])) > maxLoreIA+800 {
		t.Fatalf("limite di caratteri superato: %d", len([]rune(s[i:])))
	}
	// Il gruppo a Borgofosco (645, 370): partono anche le schede del luogo, senza nominarle.
	sc.m.deve(200, "POST", "/mappa/party?sessione="+sc.ses, map[string]any{"azione": "posiziona", "x": 648, "y": 372})
	s = sistema("Idee per stasera?")
	if !strings.Contains(s, "Il segnalino del gruppo è ora a: Borgofosco.") || !strings.Contains(s, "### [Luogo] Orfanotrofio di Borgofosco") {
		t.Fatalf("schede del luogo del gruppo:\n%s", s[strings.Index(s, "CODICE DELLA CAMPAGNA"):])
	}
	// Anteprima «Cosa sa l'IA».
	if c := sc.m.deve(200, "GET", "/assistente/contesto?gruppo="+g, nil); !strings.Contains(c["codice"].(string), "Orfanotrofio di Borgofosco") {
		t.Fatalf("anteprima del Codice: %v", c["codice"])
	}
	// «Parla con l'antagonista» riceve lo stesso blocco.
	sc.m.chiediIA(map[string]any{"testo": "l'antagonista, che pensi di Madre Maren?", "gruppo": g, "modo": "antagonista"})
	if s := leggiFinta(&ultima)["messages"].([]any)[0].(map[string]any)["content"].(string); !strings.Contains(s, "SEGRETO-LYSENNE") {
		t.Fatal("l'antagonista deve ricevere il Codice")
	}
}
