package server

import (
	"strings"
	"testing"
)

const loreProva = `Introduzione di ChatGPT: qui sotto le schede.

# PNG: Aldric Borinovich
Luogo: Villaggio di Nebbiavalle
Collegamenti: Elara Voss; Villaggio di Nebbiavalle
Etichette: alleato, nobile

## Per il Master
Vuole portare Elara a Borgofosco.
Teme di non essere all'altezza del padre.

## Per i giocatori
Il figlio del borgomastro, detto «il Minore».

# Località: Villaggio di Nebbiavalle
**Regione**: Valle di Gaster Bless
Collegamenti: Aldric Borinovich

Un villaggio grigio e silenzioso.
## Storia
Fondato secoli fa.

# Personaggio: Elara Voss
## Giocatori
Ha due cicatrici sul collo.

# Boh: niente
testo perso

# PNG: Aldric Borinovich
Doppione.
`

// Codice della campagna (fase L1, 2026-11-12): importazione da Markdown con anteprima, stato per gruppo, esportazione.
func TestCodiceDellaCampagna(t *testing.T) {
	sc := nuovoScontro(t)
	sc.luca.deve(403, "GET", "/lore", nil)
	sc.sara.deve(403, "POST", "/lore/importa", map[string]any{"testo": loreProva, "applica": true})

	// La risposta di ChatGPT dentro un blocco di codice: i bordi «```» non entrano nei testi.
	blocco := sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": "```markdown\n# Segreto: Prova\nEtichette: da decidere\n\n## Per il Master\nNon ancora deciso.\n```"})
	if len(blocco["nuove"].([]any)) != 1 || len(blocco["errori"].([]any)) != 0 {
		t.Fatalf("blocco di codice: %v", blocco)
	}
	// Anteprima: 3 nuove, un titolo non riconosciuto, un doppione; nulla scritto.
	a := sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": loreProva})
	if len(a["nuove"].([]any)) != 3 || len(a["errori"].([]any)) != 2 || a["applicato"] != false {
		t.Fatalf("anteprima: %v", a)
	}
	if l := sc.m.deve(200, "GET", "/lore", nil)["schede"].([]any); len(l) != 0 {
		t.Fatal("l'anteprima non scrive nulla")
	}
	sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": loreProva, "applica": true})
	v := sc.m.deve(200, "GET", "/lore", nil)
	schede := v["schede"].([]any)
	trova := func(nome string) map[string]any {
		for _, x := range sc.m.deve(200, "GET", "/lore", nil)["schede"].([]any) {
			if x.(map[string]any)["nome"] == nome {
				return x.(map[string]any)
			}
		}
		return nil
	}
	if len(schede) != 3 || len(v["gruppi"].([]any)) != 2 {
		t.Fatalf("schede e gruppi: %v", v)
	}
	is := trova("Aldric Borinovich")
	if is["tipo"] != "png" || is["campi"].(map[string]any)["Luogo"] != "Villaggio di Nebbiavalle" || len(is["collegamenti"].([]any)) != 2 ||
		len(is["etichette"].([]any)) != 2 || !strings.Contains(is["testoMaster"].(string), "Teme di non essere") || !strings.HasPrefix(is["testoGiocatori"].(string), "Il figlio") {
		t.Fatalf("Aldric: %v", is)
	}
	vb := trova("Villaggio di Nebbiavalle")
	if vb["tipo"] != "luogo" || vb["campi"].(map[string]any)["Regione"] != "Valle di Gaster Bless" || !strings.Contains(vb["testoMaster"].(string), "### Storia\nFondato") {
		t.Fatalf("villaggio: %v", vb)
	}
	if ir := trova("Elara Voss"); ir["tipo"] != "png" || ir["testoGiocatori"] != "Ha due cicatrici sul collo." {
		t.Fatalf("Elara: %v", ir)
	}

	// Reimportando: tutto uguale; con un testo cambiato: «diverse», aggiornata solo se il Master lo sceglie.
	if a := sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": loreProva}); a["uguali"].(float64) != 3 || len(a["nuove"].([]any)) != 0 {
		t.Fatalf("di nuovo: %v", a)
	}
	cambiato := strings.Replace(loreProva, "Vuole portare Elara a Borgofosco.", "Vuole portare Elara a Pietralta.", 1)
	if a := sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": cambiato, "applica": true}); len(a["diverse"].([]any)) != 1 {
		t.Fatalf("diversa: %v", a)
	}
	if !strings.Contains(trova("Aldric Borinovich")["testoMaster"].(string), "Borgofosco") {
		t.Fatal("senza «aggiorna» la scheda resta com'era")
	}
	sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": cambiato, "applica": true, "aggiorna": true})
	if !strings.Contains(trova("Aldric Borinovich")["testoMaster"].(string), "Pietralta") {
		t.Fatal("con «aggiorna» la scheda cambia")
	}

	// Scheda a mano, nome doppio rifiutato, modifica, stato per gruppo, eliminazione.
	id := sc.m.deve(200, "PUT", "/lore/nuova", map[string]any{"tipo": "fazione", "nome": "Guardiani della Piuma", "testoMaster": "Wereraven.", "etichette": []string{"alleati"}})["id"].(string)
	sc.m.deve(409, "PUT", "/lore/nuova", map[string]any{"tipo": "fazione", "nome": "guardiani della piuma"})
	sc.m.deve(400, "PUT", "/lore/nuova", map[string]any{"tipo": "boh", "nome": "X"})
	sc.m.deve(200, "PUT", "/lore/"+id, map[string]any{"tipo": "fazione", "nome": "Guardiani della Piuma", "testoMaster": "Wereraven di Borgofosco."})
	gruppi := v["gruppi"].([]any)
	g1, g2 := gruppi[0].(map[string]any)["id"].(string), gruppi[1].(map[string]any)["id"].(string)
	sc.m.deve(200, "PUT", "/lore/"+id+"/stato", map[string]any{"gruppo": g1, "stato": "incontrato", "nota": "alla Piuma Blu"})
	sc.m.deve(400, "PUT", "/lore/"+id+"/stato", map[string]any{"gruppo": g2, "stato": "boh"})
	sc.m.deve(404, "PUT", "/lore/"+id+"/stato", map[string]any{"gruppo": "nessuno", "stato": "conosciuto"})
	st := trova("Guardiani della Piuma")["stati"].(map[string]any)
	if len(st) != 1 || st[g1].(map[string]any)["stato"] != "incontrato" || st[g1].(map[string]any)["nota"] != "alla Piuma Blu" {
		t.Fatalf("stato per gruppo: %v", st)
	}
	sc.m.deve(200, "PUT", "/lore/"+id+"/stato", map[string]any{"gruppo": g1, "stato": ""})
	if trova("Guardiani della Piuma")["stati"] != nil {
		t.Fatal("stato tolto")
	}

	// Esportazione nello stesso formato: reimportata dà tutto uguale.
	cod, _, b := sc.m.grezzo("GET", "/lore/esporta", nil, "")
	md := string(b)
	if cod != 200 || !strings.Contains(md, "# PNG: Aldric Borinovich\nLuogo: Villaggio di Nebbiavalle\nCollegamenti: Elara Voss; Villaggio di Nebbiavalle\nEtichette: alleato, nobile") || !strings.Contains(md, "# Fazione: Guardiani della Piuma") {
		t.Fatalf("esportazione: %d\n%s", cod, md)
	}
	if a := sc.m.deve(200, "POST", "/lore/importa", map[string]any{"testo": md}); a["uguali"].(float64) != 4 || len(a["diverse"].([]any)) != 0 || len(a["nuove"].([]any)) != 0 {
		t.Fatalf("esportazione reimportata: %v", a)
	}
	sc.m.deve(200, "DELETE", "/lore/"+id, nil)
	sc.m.deve(404, "DELETE", "/lore/"+id, nil)
	if trova("Guardiani della Piuma") != nil {
		t.Fatal("eliminata")
	}
}
