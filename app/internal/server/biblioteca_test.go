package server

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// Biblioteca e due identità dell'Assistente (2026-11-13): l'Archivista del canone risponde dai testi ufficiali caricati
// dal Master, il Cronista dalla lore della campagna; ognuno riceve dell'altro solo un consulto separato e firmato.
func TestBibliotecaEIdentita(t *testing.T) {
	sc := nuovoScontro(t)
	var g string
	sc.a.s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, sc.ses).Scan(&g)
	var ultima map[string]any
	var auth string
	ia := iaFinta(t, &ultima, &auth)
	defer ia.Close()
	m := sc.m
	m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "altro", "indirizzo": ia.URL + "/v1", "modello": "qwen3:8b", "ricercaApprofondita": false})
	sistema := func() string { return leggiFinta(&ultima)["messages"].([]any)[0].(map[string]any)["content"].(string) }
	riempi := strings.Repeat("La nebbia saliva dal fiume e copriva i tetti del borgo, mentre le campane restavano mute. ", 6)
	carica := func(q url.Values, corpo string) map[string]any {
		t.Helper()
		cod, _, b := m.grezzo("POST", "/biblioteca?"+q.Encode(), []byte(corpo), "application/octet-stream")
		var out map[string]any
		json.Unmarshal(b, &out)
		if cod != 200 {
			t.Fatalf("caricamento: %d %s", cod, b)
		}
		return out
	}
	// testi inventati per la prova
	romanzo := carica(url.Values{"titolo": {"Romanzo di prova"}, "tipo": {"romanzo"}, "dominio": {"canone"}, "file": {"romanzo.txt"}},
		"Capitolo 1\n\n"+riempi+"\n\nCapitolo 2\n\nIl destriero nero dell'antagonista si chiamava Fulmine d'Ombra e dormiva nelle stalle del castello.\n\n"+riempi)
	if romanzo["parti"] != 2.0 || romanzo["dominio"] != "canone" || romanzo["attiva"] != true {
		t.Fatalf("romanzo: %v", romanzo)
	}
	carica(url.Values{"titolo": {"Diario della campagna"}, "tipo": {"appunti"}, "dominio": {"campagna"}, "file": {"diario.md"}},
		"# Sessione 5\n\nNella nostra campagna Arin ha ucciso il destriero dell'antagonista, Fulmine d'Ombra, sul ponte di Specchio.\n\n"+riempi)
	if cod, _, _ := m.grezzo("POST", "/biblioteca?file=x.pdf", []byte("%PDF"), ""); cod != 400 {
		t.Fatal("un PDF arriva solo come pagine lette nel browser")
	}
	if n := len(m.deve(200, "GET", "/biblioteca", nil)["fonti"].([]any)); n != 2 {
		t.Fatalf("fonti: %d", n)
	}
	// ricerca senza IA
	ris := m.deve(200, "GET", "/biblioteca/cerca?dominio=canone&q="+url.QueryEscape("Come si chiama il destriero dell'antagonista?"), nil)["lista"].([]any)
	if len(ris) == 0 || ris[0].(map[string]any)["dove"] != "Capitolo 2" || !strings.Contains(ris[0].(map[string]any)["testo"].(string), "Fulmine d'Ombra") {
		t.Fatalf("ricerca: %v", ris)
	}
	b := m.deve(200, "GET", "/biblioteca/"+romanzo["id"].(string)+"/brano/"+jsonNumero(ris[0].(map[string]any)["pezzo"]), nil)
	if !strings.Contains(b["testo"].(string), "Fulmine d'Ombra") || b["titolo"] != "Romanzo di prova" {
		t.Fatalf("brano: %v", b)
	}

	// Codice: una scheda del canone e una della campagna
	md := "# PNG: Fulmine d'Ombra\nEtichette: canone\n\n## Per il Master\nCANONE-CAVALLO: un destriero da incubo.\n\n" +
		"# PNG: Arin il Cacciatore\nAlias: Arin\nEtichette: approvato, campagna\n\n## Per il Master\nCAMPAGNA-ARIN: ha giurato vendetta.\n\n" +
		"# Luogo: Ponte di Specchio\nEtichette: campagna\n\n## Per il Master\nCAMPAGNA-PONTE: crollato.\n"
	m.deve(200, "POST", "/lore/importa", map[string]any{"testo": md, "applica": true})

	// Archivista del canone
	_, righe := m.chiediIA(map[string]any{"testo": "Come si chiama il destriero dell'antagonista, secondo Arin?", "gruppo": g, "modo": "canone"})
	s := sistema()
	fine := righe[len(righe)-1]
	if !strings.HasPrefix(s, "ARCHIVISTA DELLE FONTI") || !strings.Contains(s, "[C1] «Romanzo di prova» (romanzo) — Capitolo 2\nIl destriero nero dell'antagonista") ||
		!strings.Contains(s, "SCHEDE DEL CANONE NEL CODICE") || !strings.Contains(s, "CANONE-CAVALLO") || strings.Contains(s, "SITUAZIONE ATTUALE DEL GRUPPO") ||
		strings.Contains(s, "per questo gruppo") {
		t.Fatalf("archivista: %s", s)
	}
	// la campagna solo nel consulto, dopo i testi del canone
	i := strings.Index(s, "=== CONSULTO DEL CRONISTA DELLA CAMPAGNA")
	if i < 0 || strings.Index(s, "CAMPAGNA-ARIN") < i || strings.Index(s, "[H1] «Diario della campagna»") < i || strings.Contains(s, "CAMPAGNA-PONTE") && strings.Index(s, "CAMPAGNA-PONTE") < i {
		t.Fatalf("consulto del Cronista: %s", s)
	}
	fonti, _ := fine["fonti"].([]any)
	if len(fonti) < 2 || fonti[0].(map[string]any)["rif"] != "C1" || fonti[0].(map[string]any)["testo"] != nil {
		t.Fatalf("fonti nella risposta: %v", fine)
	}
	conv := m.deve(200, "GET", "/assistente/conversazioni/"+fine["conversazione"].(string), nil)
	if conv["modo"] != "canone" || conv["gruppo"] != "" || len(conv["messaggi"].([]any)[1].(map[string]any)["fonti"].([]any)) != len(fonti) {
		t.Fatalf("conversazione dell'Archivista: %v", conv)
	}

	// Cronista della campagna: i suoi documenti e il suo Codice; il canone solo nel consulto
	m.chiediIA(map[string]any{"testo": "Che fine ha fatto Fulmine d'Ombra, il destriero dell'antagonista?", "gruppo": g})
	s = sistema()
	if !strings.HasPrefix(s, "ASSISTENTE DEL MASTER") || !strings.Contains(s, "IDENTITÀ: CRONISTA DELLA CAMPAGNA") ||
		!strings.Contains(s, "[H1] «Diario della campagna» (appunti del Master) — Sessione 5") || !strings.Contains(s, "CAMPAGNA-ARIN") {
		t.Fatalf("cronista: %.300s", s)
	}
	j := strings.Index(s, "=== CONSULTO DELL'ARCHIVISTA DEL CANONE")
	k := strings.Index(s, "=== FINE DEL CONSULTO ===")
	if j < 0 || k < j || strings.Index(s, "[C1] «Romanzo di prova»") < j || strings.Index(s, "CANONE-CAVALLO") < j || strings.Index(s, "CANONE-CAVALLO") > k {
		t.Fatalf("consulto dell'Archivista: %s", s[max(0, j-200):])
	}
	if strings.Index(s, "CODICE DELLA CAMPAGNA") < k {
		t.Fatal("il Codice della campagna viene dopo il consulto")
	}
	// l'antagonista: il canone come «cronache ufficiali»
	m.chiediIA(map[string]any{"testo": "Ti ricordi di Fulmine d'Ombra?", "gruppo": g, "modo": "antagonista"})
	if s = sistema(); !strings.Contains(s, "LE FONTI DI CIÒ CHE SAI") || !strings.Contains(s, "=== CRONACHE UFFICIALI") || !strings.Contains(s, "CANONE-CAVALLO") {
		t.Fatalf("antagonista: %s", s[max(0, len(s)-1500):])
	}

	// Scambio spento: nessun consulto
	if r := m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"scambio": false}); r["scambio"] != false || r["config"].(map[string]any)["fornitore"] != "altro" {
		t.Fatalf("impostazione dello scambio: %v", r)
	}
	m.chiediIA(map[string]any{"testo": "Come si chiama il destriero dell'antagonista?", "gruppo": g})
	if s = sistema(); strings.Contains(s, "=== CONSULTO DELL'ARCHIVISTA") || strings.Contains(s, "CANONE-CAVALLO") || strings.Contains(s, "Romanzo di prova") {
		t.Fatal("senza scambio il Cronista non riceve nulla del canone")
	}
	m.chiediIA(map[string]any{"testo": "Il destriero dell'antagonista, secondo Arin?", "modo": "canone"})
	if s = sistema(); strings.Contains(s, "CAMPAGNA-ARIN") || strings.Contains(s, "Diario della campagna") {
		t.Fatal("senza scambio l'Archivista non riceve nulla della campagna")
	}
	// domanda senza risposta nei testi
	m.chiediIA(map[string]any{"testo": "Chi è il pizzaiolo di Borgofosco?", "modo": "canone"})
	if s = sistema(); !strings.Contains(s, "nessun brano sembra pertinente") || strings.Contains(s, "[C1] «") {
		t.Fatalf("niente brani: %s", s)
	}
	// testo disattivato e tolto
	m.deve(200, "PATCH", "/biblioteca/"+romanzo["id"].(string), map[string]any{"attiva": false})
	m.chiediIA(map[string]any{"testo": "Come si chiama il destriero dell'antagonista?", "modo": "canone"})
	if s = sistema(); strings.Contains(s, "[C1] «") || !strings.Contains(s, "disattivati") {
		t.Fatalf("testo disattivato: %s", s)
	}
	m.deve(200, "DELETE", "/biblioteca/"+romanzo["id"].(string), nil)
	m.deve(404, "GET", "/biblioteca/"+romanzo["id"].(string)+"/brano/0", nil)
	if n := len(m.deve(200, "GET", "/biblioteca", nil)["fonti"].([]any)); n != 1 {
		t.Fatalf("dopo l'eliminazione: %d", n)
	}
	// istruzioni dell'Archivista modificabili; solo il Master
	if p := m.deve(200, "GET", "/assistente/canone", nil); !strings.HasPrefix(p["testo"].(string), "ARCHIVISTA DELLE FONTI") {
		t.Fatal("istruzioni dell'Archivista")
	}
	sc.luca.deve(403, "GET", "/biblioteca", nil)
	sc.luca.deve(403, "GET", "/biblioteca/cerca?q=l'antagonista", nil)
}

func jsonNumero(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestDominioScheda(t *testing.T) {
	for _, c := range []struct {
		e     []string
		campi map[string]string
		dom   string
	}{
		{[]string{"canone"}, nil, "canone"},
		{[]string{"tomo interattivo", "visione"}, nil, "canone"},
		{[]string{"Romanzi"}, nil, "canone"},
		{[]string{"canone", "campagna"}, nil, "campagna"},
		{[]string{"approvato", "campagna", "variante"}, nil, "campagna"},
		{[]string{"proposta"}, nil, "campagna"},
		{nil, nil, "campagna"},
		{[]string{"canone"}, map[string]string{"Variante di": "il modulo"}, "campagna"},
	} {
		if d := dominioScheda(schedaLore{Etichette: c.e, Campi: c.campi}); d != c.dom {
			t.Fatalf("%v %v: %s invece di %s", c.e, c.campi, d, c.dom)
		}
	}
}
