package server

import (
	"strings"
	"testing"
)

// «Parla con l'antagonista»: la conversazione con modo antagonista manda all'IA il system prompt dell'antagonista (poi la situazione del
// gruppo come informazione del Master); quella dell'Assistente resta con le sue istruzioni. Il prompt si modifica e
// si ripristina.
func TestParlaConAntagonista(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	var ultima map[string]any
	var auth string
	ia := iaFinta(t, &ultima, &auth)
	defer ia.Close()
	m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "altro", "indirizzo": ia.URL + "/v1", "modello": "qwen3:8b"})
	sistema := func() string { return leggiFinta(&ultima)["messages"].([]any)[0].(map[string]any)["content"].(string) }

	// Assistente: istruzioni di sempre
	_, righe := m.chiediIA(map[string]any{"testo": "Un PNG per Borgofosco?", "gruppo": g})
	if leggiFinta(&ultima)["temperature"] != nil {
		t.Fatal("l'Assistente resta con le impostazioni del servizio")
	}
	if s := sistema(); !strings.HasPrefix(s, "ASSISTENTE DEL MASTER") || strings.Contains(s, "il Conte Varn, Conte di Gaster Bless") {
		t.Fatalf("assistente: istruzioni sbagliate: %.200s", s)
	}
	convA := righe[len(righe)-1]["conversazione"].(string)

	// l'antagonista: il suo prompt, poi la situazione del gruppo fuori dal personaggio
	_, righe = m.chiediIA(map[string]any{"testo": "Chi ha osato svegliarti?", "gruppo": g, "modo": "antagonista"})
	s := sistema()
	if !strings.HasPrefix(s, "PARLA CON L'ANTAGONISTA") || !strings.Contains(s, "Rispondi sempre in italiano.") ||
		!strings.Contains(s, "IL TUO RUOLO CON IL MASTER") || !strings.Contains(s, "CIÒ CHE GASTER BLESS TI RIFERISCE SULLA CAMPAGNA") ||
		!strings.Contains(s, "SOLO sul gruppo «Martedì»") || strings.Contains(s, "ASSISTENTE DEL MASTER") {
		t.Fatalf("antagonista: prompt sbagliato: %.300s … %.300s", s, s[len(s)-300:])
	}
	convS := righe[len(righe)-1]["conversazione"].(string)
	if leggiFinta(&ultima)["temperature"] != 0.9 {
		t.Fatal("con l'antagonista risposte più varie (temperatura 0.9)")
	}
	// la conversazione resta dell'antagonista anche senza ripetere il modo
	m.chiediIA(map[string]any{"testo": "E Aldric?", "conversazione": convS})
	if !strings.HasPrefix(sistema(), "PARLA CON L'ANTAGONISTA") {
		t.Fatal("la conversazione con l'antagonista resta con l'antagonista")
	}
	m.chiediIA(map[string]any{"testo": "Altro PNG?", "conversazione": convA, "modo": "antagonista"}) // modo ignorato: la conversazione è dell'Assistente
	if !strings.HasPrefix(sistema(), "ASSISTENTE DEL MASTER") {
		t.Fatal("una conversazione dell'Assistente non diventa dell'antagonista")
	}
	modi := map[string]string{}
	for _, c := range m.deve(200, "GET", "/assistente/conversazioni", nil)["lista"].([]any) {
		c := c.(map[string]any)
		modi[c["id"].(string)] = c["modo"].(string)
	}
	if modi[convS] != "antagonista" || modi[convA] != "assistente" {
		t.Fatalf("elenco: %v", modi)
	}
	if m.deve(200, "GET", "/assistente/conversazioni/"+convS, nil)["modo"] != "antagonista" {
		t.Fatal("conversazione: modo antagonista")
	}

	// prompt modificabile e ripristinabile; solo il Master
	if p := m.deve(200, "GET", "/assistente/antagonista", nil); p["personalizzato"] != false || !strings.HasPrefix(p["testo"].(string), "PARLA CON L'ANTAGONISTA") {
		t.Fatalf("prompt predefinito: %v", p["personalizzato"])
	}
	m.deve(200, "PUT", "/assistente/antagonista", map[string]any{"testo": "Sei l'antagonista. In questa campagna Elara è viva."})
	m.chiediIA(map[string]any{"testo": "Parla.", "conversazione": convS})
	if !strings.HasPrefix(sistema(), "Sei l'antagonista. In questa campagna Elara è viva.") {
		t.Fatal("prompt modificato dal Master")
	}
	if p := m.deve(200, "PUT", "/assistente/antagonista", map[string]any{"testo": ""}); p["personalizzato"] != false {
		t.Fatal("vuoto: torna al testo predefinito")
	}
	g2 := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	g2.deve(403, "GET", "/assistente/antagonista", nil)
}

// Istruzioni dell'Assistente: la Master Campaign Bible è il testo predefinito, modificabile e ripristinabile; con Ollama
// le istruzioni lunghe accorciano la storia mandata (mai sotto il minimo), con i servizi online no.
func TestIstruzioniAssistente(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	var ultima map[string]any
	var auth string
	ia := iaFinta(t, &ultima, &auth)
	defer ia.Close()
	m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "altro", "indirizzo": ia.URL + "/v1", "modello": "qwen3:8b"})
	sistema := func() string { return leggiFinta(&ultima)["messages"].([]any)[0].(map[string]any)["content"].(string) }
	m.chiediIA(map[string]any{"testo": "Prepara la festa di Borgofosco.", "gruppo": g})
	if s := sistema(); !strings.HasPrefix(s, "ASSISTENTE DEL MASTER") || !strings.Contains(s, "SOLO sul gruppo «Martedì»") || !strings.Contains(s, "SITUAZIONE ATTUALE DEL GRUPPO") {
		t.Fatalf("istruzioni dell'assistente: %.200s", s)
	}
	if c := m.deve(200, "GET", "/assistente/contesto?gruppo="+g, nil); !strings.HasPrefix(c["istruzioni"].(string), "ASSISTENTE DEL MASTER") {
		t.Fatal("«Cosa sa l'IA» mostra la Bible")
	}
	m.deve(200, "PUT", "/assistente/istruzioni", map[string]any{"testo": "BIBLE 2.0: Nikka è una nipote del cacciatore."})
	m.chiediIA(map[string]any{"testo": "E Nikka?", "gruppo": g})
	if !strings.HasPrefix(sistema(), "BIBLE 2.0: Nikka è una nipote del cacciatore.") {
		t.Fatal("istruzioni modificate dal Master")
	}
	if p := m.deve(200, "PUT", "/assistente/istruzioni", map[string]any{"testo": ""}); p["personalizzato"] != false || !strings.HasPrefix(p["testo"].(string), "ASSISTENTE DEL MASTER") {
		t.Fatal("vuoto: torna alla Bible")
	}
	a.giocatore("Luca", "333 1234567", "192.168.1.20").deve(403, "GET", "/assistente/istruzioni", nil)

	lungo := strings.Repeat("x", caratteriOllama-8000)
	if n := limiteStoria(configIA{Fornitore: "ollama"}, lungo, maxStoriaIA); n != 8000 {
		t.Fatalf("Ollama, istruzioni lunghe: storia di 8.000, non %d", n)
	}
	if n := limiteStoria(configIA{Fornitore: "ollama"}, strings.Repeat("x", caratteriOllama-1000), maxStoriaIA); n != minStoriaOllama {
		t.Fatalf("Ollama: mai sotto il minimo (%d)", n)
	}
	if n := limiteStoria(configIA{Fornitore: "gemini"}, lungo, maxStoriaIA); n != maxStoriaIA {
		t.Fatal("servizi online: storia intera")
	}
}

func TestAdattaOllama(t *testing.T) {
	sist := strings.Repeat("s", caratteriOllama-16000) // spazio per i messaggi: 16.000
	st := []messaggioIA{{Ruolo: "master", Testo: strings.Repeat("a", 9000)}, {Ruolo: "ia", Testo: strings.Repeat("b", 5000)}, {Ruolo: "master", Testo: strings.Repeat("c", 8000)}}
	out := adattaOllama(sist, st)
	if len(out) != 1 || out[0].Testo != strings.Repeat("c", 8000) {
		t.Fatalf("restano solo i messaggi che stanno, cominciando da una domanda: %d", len(out))
	}
	out = adattaOllama(sist, []messaggioIA{{Ruolo: "master", Testo: strings.Repeat("d", 40000)}})
	// contesto di Ollama: 16.384 per le richieste brevi, di più (fino a 32.768) per quelle lunghe
	if c := contestoPerOllama("breve", []messaggioIA{{Testo: "ciao"}}); c != 16384 {
		t.Fatalf("richiesta breve: 16384, non %d", c)
	}
	if c := contestoPerOllama(strings.Repeat("x", 50000), []messaggioIA{{Testo: strings.Repeat("y", 10000)}}); c != 24576 {
		t.Fatalf("60.000 caratteri ≈ 23.000 token: 24576, non %d", c)
	}
	if c := contestoPerOllama(strings.Repeat("x", 200000), nil); c != 32768 {
		t.Fatalf("mai oltre 32768: %d", c)
	}
	_ = out
	out = adattaOllama(sist, []messaggioIA{{Ruolo: "master", Testo: strings.Repeat("d", 40000)}})
	if len(out) != 1 || !strings.HasPrefix(out[0].Testo, strings.Repeat("d", 16000)+"\n[…") {
		t.Fatal("un messaggio troppo lungo si accorcia in fondo")
	}
}
