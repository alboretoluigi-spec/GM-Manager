package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func contoCampagna(t *testing.T, out map[string]any, sez, k string) int {
	t.Helper()
	c, ok := out["sezioni"].(map[string]any)[sez].(map[string]any)
	if !ok {
		t.Fatalf("sezione %s mancante: %v", sez, out)
	}
	return int(c[k].(float64))
}

// File unico della campagna (M4): esporta senza dati di accesso, anteprima che non cambia nulla, aggiunta senza cancellare,
// aggiornamento delle voci diverse solo se richiesto, importazione su un altro Gaster Bless con il giocatore ritrovato per nome.
func TestCampagna(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 1, "modifiche": map[string]any{"cls": "guerriero", "base.FOR": 16}})
	tv.m.deve(200, "POST", "/gruppi/"+tv.g1+"/sessioni", map[string]any{"titolo": "La Tana del Lupo", "inizio": 1893456000})
	tv.m.deve(200, "PUT", "/oggetti-magici/ogm-c1", map[string]any{"oggetto": map[string]any{"id": "ogm-c1", "nome": "Anello della nebbia", "tipo": "anello", "rar": "raro"}})
	db := tv.a.s.DB
	for _, q := range []string{
		`INSERT INTO localita_contenuti(localita, descrizione, note_master) VALUES('borgofosco', 'Città dei festival', 'Il borgomastro')`,
		`INSERT INTO bestiario_voci(id, nome, cat, ruolo, base, creato_il) VALUES('pv-rictavio', 'Rictavio', 'png', 'bardo', '', 1)`,
		`INSERT INTO bestiario_personalizzazioni(voce_id, note) VALUES('mm-wolf', 'Lupi della foresta di Selvascura')`,
		`INSERT INTO riassunti(id, gruppo_id, autore_nome, titolo, testo, creato_il, modificato_il) VALUES('r1', '` + tv.g1 + `', 'Luca', 'Arrivo', 'Siamo arrivati nella nebbia.', 1893460000, 1893460000)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	tv.luca.deve(403, "GET", "/campagna/esporta", nil)
	tv.luca.deve(403, "POST", "/campagna/importa", map[string]any{"file": map[string]any{}, "prova": true})

	file := tv.m.deve(200, "GET", "/campagna/esporta", nil)
	b, _ := json.Marshal(file)
	testo := string(b)
	for _, vietato := range []string{"password", "telefono", "333 1234567", "csrf"} {
		if strings.Contains(strings.ToLower(testo), vietato) {
			t.Fatalf("il file della campagna non deve contenere %q", vietato)
		}
	}
	for k, n := range map[string]int{"gruppi": 2, "sessioni": 1, "personaggi": 1, "bestiarioVoci": 1, "bestiarioSchede": 1, "oggetti": 1, "localita": 1, "riassunti": 1} {
		if l, _ := file[k].([]any); len(l) != n {
			t.Fatalf("%s: attesi %d, trovati %d (%v)", k, n, len(l), file[k])
		}
	}
	if file["formato"] != "gasterbless-campagna" || file["personaggi"].([]any)[0].(map[string]any)["giocatore"] != "Luca" {
		t.Fatalf("file: %v", file)
	}
	solo := tv.m.deve(200, "GET", "/campagna/esporta?sezioni=oggetti", nil)
	if solo["personaggi"] != nil || len(solo["oggetti"].([]any)) != 1 {
		t.Fatalf("solo oggetti: %v", solo)
	}

	// stesso Gaster Bless: tutto uguale
	out := tv.m.deve(200, "POST", "/campagna/importa", map[string]any{"file": file, "prova": true})
	for _, s := range sezioniCampagna {
		if contoCampagna(t, out, s, "nuovi") != 0 || contoCampagna(t, out, s, "diversi") != 0 {
			t.Fatalf("reimportazione identica, %s: %v", s, out["sezioni"])
		}
	}
	// cambia la scheda e sparisce l'oggetto: l'anteprima lo dice e non cambia nulla
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"base.FOR": 8}})
	tv.m.deve(200, "DELETE", "/oggetti-magici/ogm-c1", nil)
	out = tv.m.deve(200, "POST", "/campagna/importa", map[string]any{"file": file, "prova": true})
	if contoCampagna(t, out, "personaggi", "diversi") != 1 || contoCampagna(t, out, "oggetti", "nuovi") != 1 {
		t.Fatalf("anteprima: %v", out["sezioni"])
	}
	if len(tv.m.deve(200, "GET", "/oggetti-magici", nil)["oggetti"].([]any)) != 0 {
		t.Fatal("l'anteprima non deve importare nulla")
	}
	// importazione senza «aggiorna»: l'oggetto torna, la scheda resta com'è
	out = tv.m.deve(200, "POST", "/campagna/importa", map[string]any{"file": file})
	if contoCampagna(t, out, "personaggi", "aggiornati") != 0 || len(tv.m.deve(200, "GET", "/oggetti-magici", nil)["oggetti"].([]any)) != 1 {
		t.Fatalf("importazione: %v", out["sezioni"])
	}
	forza := func() any {
		return tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)["base"].(map[string]any)["FOR"]
	}
	if forza() != float64(8) {
		t.Fatal("senza «aggiorna» la scheda non deve cambiare")
	}
	// con «aggiorna» la scheda torna quella del file (versione nuova, nulla cancellato)
	out = tv.m.deve(200, "POST", "/campagna/importa", map[string]any{"file": file, "aggiorna": true, "sezioni": []string{"personaggi"}})
	if contoCampagna(t, out, "personaggi", "aggiornati") != 1 || forza() != float64(16) {
		t.Fatalf("aggiornamento: %v", out["sezioni"])
	}
	tv.m.deve(400, "POST", "/campagna/importa", map[string]any{"file": map[string]any{"formato": "altro"}, "prova": true})

	// un altro Gaster Bless: gruppi con lo stesso nome ritrovati, personaggio nuovo assegnato a «Luca»
	tv2 := nuovoTavolo(t)
	out = tv2.m.deve(200, "POST", "/campagna/importa", map[string]any{"file": file})
	if contoCampagna(t, out, "gruppi", "uguali") != 2 || contoCampagna(t, out, "personaggi", "nuovi") != 1 || contoCampagna(t, out, "riassunti", "nuovi") != 1 ||
		contoCampagna(t, out, "bestiario", "nuovi") != 2 || contoCampagna(t, out, "localita", "nuovi") != 1 || contoCampagna(t, out, "oggetti", "nuovi") != 1 {
		t.Fatalf("altro Gaster Bless: %v", out["sezioni"])
	}
	l := tv2.m.deve(200, "GET", "/personaggi", nil)["lista"].([]any)
	if len(l) != 1 || l[0].(map[string]any)["nomeGiocatore"] != "Luca" || l[0].(map[string]any)["gruppo"] != tv2.g1 {
		t.Fatalf("personaggio importato: %v", l)
	}
	if n := len(tv2.luca.deve(200, "GET", "/personaggi", nil)["lista"].([]any)); n != 1 {
		t.Fatalf("Luca deve vedere il suo personaggio: %d", n)
	}
	var ses int
	tv2.a.s.DB.QueryRow(`SELECT COUNT(*) FROM sessioni_gioco WHERE titolo = 'La Tana del Lupo'`).Scan(&ses)
	if ses != 1 {
		t.Fatalf("sessione importata: %d", ses)
	}
}
