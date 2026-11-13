package server

import (
	"testing"
	"time"
)

func versioneDi(c *cliente, pg string) int {
	return int(c.deve(200, "GET", "/personaggi/"+pg, nil)["personaggio"].(map[string]any)["versione"].(float64))
}

// aspettaControlli: i controlli partono in background; si aspetta che il numero di avvisi aperti sia quello atteso.
func aspettaControlli(t *testing.T, m *cliente, aperti int) []any {
	t.Helper()
	var l []any
	for i := 0; i < 100; i++ {
		l = nil
		for _, x := range m.deve(200, "GET", "/controlli-schede", nil)["lista"].([]any) {
			if x.(map[string]any)["stato"] == "aperto" {
				l = append(l, x)
			}
		}
		if len(l) == aperti {
			time.Sleep(30 * time.Millisecond) // nessun altro controllo in arrivo
			return l
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("avvisi aperti: %d, attesi %d", len(l), aperti)
	return nil
}

// Controllo della scheda all'accesso in sessione: avviso al Master solo per le modifiche fuori sessione o offline
// dei campi importanti; «Va bene» e «Ripristina».
func TestControlloSchedeInSessione(t *testing.T) {
	tv := nuovoTavolo(t)
	pg := tv.nuovoPG(tv.luca, tv.g1)
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"lvl": 3, "base.FOR": 14}})
	ses := tv.m.deve(200, "POST", "/gruppi/"+tv.g1+"/sessioni", map[string]any{"titolo": "La Tana del Lupo", "inizio": 1893456000})["id"].(string)
	stato := func(s string) { tv.m.deve(200, "PATCH", "/sessioni/"+ses, map[string]any{"stato": s}) }

	// Prima sessione: personaggio mai controllato, la scheda di oggi diventa il punto di partenza (nessun avviso).
	stato("in_corso")
	tv.luca.deve(200, "POST", "/sessioni/"+ses+"/conferma", map[string]any{"confermato": true, "personaggio": pg})
	aspettaControlli(t, tv.m, 0)

	// Durante la sessione il giocatore cambia il livello: nessun avviso.
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"lvl": 4}})
	tv.luca.deve(200, "POST", "/sessioni/"+ses+"/conferma", map[string]any{"confermato": true, "personaggio": pg})
	aspettaControlli(t, tv.m, 0)

	// Fuori sessione: Forza 18 e una nota personale (che non conta). Nessun avviso finché la sessione non riparte.
	stato("conclusa")
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"base.FOR": 18, "f.storia": "Nato a Borgofosco"}})
	aspettaControlli(t, tv.m, 0)

	cm := tv.clienteWS(tv.m)
	ultimiMessaggi(cm, "")
	stato("in_corso")
	l := aspettaControlli(t, tv.m, 1)
	c := l[0].(map[string]any)
	diff := c["differenze"].([]any)
	if len(diff) != 1 || diff[0].(map[string]any)["campo"] != "base.FOR" || diff[0].(map[string]any)["prima"] != 14.0 || diff[0].(map[string]any)["dopo"] != 18.0 {
		t.Fatalf("differenze attese solo per la Forza 14 → 18: %v", diff)
	}
	if c["offline"] != false || c["giocatore"] != "Luca" || c["sessione"] != "La Tana del Lupo" {
		t.Fatalf("avviso: %v", c)
	}
	if len(ultimiMessaggi(cm, "scheda.controllo")) != 1 {
		t.Fatal("il Master riceve l'avviso in tempo reale")
	}
	// Il controllo è fatto: ricollegarsi non crea un altro avviso.
	tv.luca.deve(200, "POST", "/sessioni/"+ses+"/conferma", map[string]any{"confermato": true, "personaggio": pg})
	aspettaControlli(t, tv.m, 1)

	// Modifiche fatte offline e inviate durante la sessione: avviso aggiornato (quello vecchio è superato).
	vecchio := c["id"]
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "offline": true, "modifiche": map[string]any{"eq": "Spada +3"}})
	for i := 0; i < 100; i++ { // si aspetta il nuovo avviso (quello vecchio diventa «superato»)
		if c = aspettaControlli(t, tv.m, 1)[0].(map[string]any); c["id"] != vecchio {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	campi := map[string]bool{}
	for _, d := range c["differenze"].([]any) {
		campi[d.(map[string]any)["campo"].(string)] = d.(map[string]any)["offline"].(bool)
	}
	if len(campi) != 2 || campi["base.FOR"] || !campi["eq"] || c["offline"] != true {
		t.Fatalf("avviso unito con la Forza (fuori sessione) e l'equipaggiamento (offline): %v", c)
	}

	// Solo il Master vede e decide.
	tv.luca.deve(403, "GET", "/controlli-schede", nil)
	tv.luca.deve(403, "POST", "/controlli-schede/"+c["id"].(string), map[string]any{"azione": "accetta"})

	// Ripristina: Forza 14 e niente equipaggiamento; il livello cambiato in sessione resta 4.
	tv.m.deve(200, "POST", "/controlli-schede/"+c["id"].(string), map[string]any{"azione": "ripristina"})
	d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
	if d["base"].(map[string]any)["FOR"] != 14.0 || d["eq"] != nil && d["eq"] != "" || d["lvl"] != 4.0 || d["f"].(map[string]any)["storia"] != "Nato a Borgofosco" {
		t.Fatalf("ripristino: %v", d)
	}
	tv.m.deve(409, "POST", "/controlli-schede/"+c["id"].(string), map[string]any{"azione": "accetta"})
	aspettaControlli(t, tv.m, 0)

	// Altra modifica fuori sessione, poi «Va bene»: la scheda resta com'è.
	stato("conclusa")
	tv.luca.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(tv.luca, pg), "modifiche": map[string]any{"cls": "mago"}})
	stato("in_corso")
	c = aspettaControlli(t, tv.m, 1)[0].(map[string]any)
	tv.m.deve(200, "POST", "/controlli-schede/"+c["id"].(string), map[string]any{"azione": "accetta"})
	if d := tv.luca.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any); d["cls"] != "mago" {
		t.Fatalf("accettata: %v", d["cls"])
	}
	aspettaControlli(t, tv.m, 0)
}
