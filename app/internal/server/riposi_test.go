package server

import (
	"encoding/json"
	"testing"
)

func TestRiposiRegole(t *testing.T) {
	for in, atteso := range map[string]int{`"2"`: 2, `"3d10"`: 3, `""`: 0, `"usati"`: 0, `4`: 4, `"-2"`: 0, `" 5 "`: 5} {
		if n := dadiUsatiDa(json.RawMessage(in)); n != atteso {
			t.Fatalf("dadiUsatiDa(%s) = %d, attesi %d", in, n, atteso)
		}
	}
	if totaleDadi([]dadoVita{{10, 3}, {6, 2}}) != 5 {
		t.Fatal("totale dei Dadi Vita")
	}
	if durataOre(8) != "8 ore" || durataOre(1) != "1 ora" || durataOre(0.5) != "30 minuti" {
		t.Fatalf("durate: %s, %s, %s", durataOre(8), durataOre(1), durataOre(0.5))
	}
}

// Percorso completo: richiesta del giocatore con un compagno, approvazione del Master, effetti, tempo, Dadi Vita.
func TestRiposi(t *testing.T) {
	tv := nuovoTavolo(t)
	arin := tv.nuovoPG(tv.luca, tv.g1)
	bea := tv.sara.deve(200, "POST", "/personaggi", map[string]any{"gruppo": tv.g1, "nome": "Bea"})["id"].(string)
	riep := map[string]any{"hpMax": 30, "hitDice": []map[string]int{{"f": 10, "n": 4}}, "conMod": 2}
	for _, x := range []struct {
		c  *cliente
		pg string
	}{{tv.luca, arin}, {tv.sara, bea}} {
		x.c.deve(200, "PATCH", "/personaggi/"+x.pg+"/scheda", map[string]any{"base": versioneDi(x.c, x.pg),
			"modifiche": map[string]any{"lvl": 4, "f.hdUsati": "3", "hpCur": "5", "slotU.1": 2, "exh": 2}, "riepilogo": riep})
	}
	l := tv.luca.deve(200, "GET", "/riposi", nil)
	if g := l["gruppi"].([]any); len(g) != 1 || len(g[0].(map[string]any)["personaggi"].([]any)) != 2 {
		t.Fatalf("Luca vede il suo gruppo con 2 personaggi: %v", g)
	}
	// Luca chiede un riposo lungo per Arin e Bea; Ugo (altro gruppo) non può includerli.
	tv.ugo.deve(404, "POST", "/riposi", map[string]any{"tipo": "lungo", "gruppo": tv.g1, "personaggi": []string{arin}})
	tv.luca.deve(400, "POST", "/riposi", map[string]any{"tipo": "lungo", "gruppo": tv.g1, "personaggi": []string{bea}}) // nessun suo personaggio
	cm := tv.clienteWS(tv.m)
	cs := tv.clienteWS(tv.sara)
	ultimiMessaggi(cm, "")
	ultimiMessaggi(cs, "")
	r := tv.luca.deve(200, "POST", "/riposi", map[string]any{"tipo": "lungo", "gruppo": tv.g1, "personaggi": []string{arin, bea}})
	id := r["riposo"].(map[string]any)["id"].(string)
	if len(ultimiMessaggi(cm, "riposo.richiesta")) != 1 || len(ultimiMessaggi(cs, "riposo.info")) != 1 {
		t.Fatal("il Master riceve la richiesta, Sara l'avviso")
	}
	tv.sara.deve(409, "POST", "/riposi", map[string]any{"tipo": "breve", "gruppo": tv.g1, "personaggi": []string{bea}}) // una alla volta
	tv.luca.deve(403, "POST", "/riposi/"+id, map[string]any{"azione": "approva"})
	// Finché il Master non decide non cambia nulla.
	if d := tv.luca.deve(200, "GET", "/personaggi/"+arin, nil)["dati"].(map[string]any); d["hpCur"] != "5" {
		t.Fatalf("in attesa: PF invariati, %v", d["hpCur"])
	}
	tv.m.deve(200, "POST", "/riposi/"+id, map[string]any{"azione": "approva"})
	for _, pg := range []string{arin, bea} {
		d := tv.m.deve(200, "GET", "/personaggi/"+pg, nil)["dati"].(map[string]any)
		if d["hpCur"] != "30" && d["hpCur"] != 30.0 {
			t.Fatalf("PF al massimo: %v", d["hpCur"])
		}
		if d["f"].(map[string]any)["hdUsati"] != "1" { // 3 usati − metà di 4 = 1
			t.Fatalf("Dadi Vita usati: %v", d["f"].(map[string]any)["hdUsati"])
		}
		if s, _ := d["slotU"].(map[string]any); len(s) != 0 || d["exh"] != 1.0 {
			t.Fatalf("slot recuperati e Indebolimento 2 → 1: slot %v exh %v", d["slotU"], d["exh"])
		}
	}
	if g := tv.luca.deve(200, "GET", "/riposi", nil)["gruppi"].([]any)[0].(map[string]any); g["ora"] != 8.0 {
		t.Fatalf("passano 8 ore di gioco: %v", g["ora"])
	}
	// Secondo riposo lungo subito: il Master deve forzare.
	id2 := tv.luca.deve(200, "POST", "/riposi", map[string]any{"tipo": "lungo", "gruppo": tv.g1, "personaggi": []string{arin}})["riposo"].(map[string]any)["id"].(string)
	tv.m.deve(409, "POST", "/riposi/"+id2, map[string]any{"azione": "approva"})
	tv.m.deve(200, "POST", "/riposi/"+id2, map[string]any{"azione": "rifiuta"})
	// Riposo breve fatto dal Master per Arin: poi Luca spende 2 Dadi Vita (d10: 5 + 2 a dado).
	tv.luca.deve(200, "PATCH", "/personaggi/"+arin+"/scheda", map[string]any{"base": versioneDi(tv.luca, arin), "modifiche": map[string]any{"hpCur": "10"}})
	b := tv.m.deve(200, "POST", "/riposi", map[string]any{"tipo": "breve", "gruppo": tv.g1, "personaggi": []string{arin}})
	idb := b["riposo"].(map[string]any)["id"].(string)
	vecchio := tiraDado
	tiraDado = func(int) int { return 5 }
	defer func() { tiraDado = vecchio }()
	tv.sara.deve(404, "POST", "/riposi/"+idb+"/dadi-vita", map[string]any{"personaggio": arin, "dadi": map[string]int{"10": 1}})
	tv.luca.deve(400, "POST", "/riposi/"+idb+"/dadi-vita", map[string]any{"personaggio": arin, "dadi": map[string]int{"10": 4}}) // ne ha 3 liberi
	x := tv.luca.deve(200, "POST", "/riposi/"+idb+"/dadi-vita", map[string]any{"personaggio": arin, "dadi": map[string]int{"10": 2}})
	if x["curati"] != 14.0 || x["pf"] != 24.0 {
		t.Fatalf("2 × (5 + 2) = 14 PF, da 10 a 24: %v", x)
	}
	// i dadi tirati, per i dadi 3D della pagina (dadi D2)
	if g, _ := x["dadi"].([]any); len(g) != 1 || g[0].(map[string]any)["facce"] != 10.0 || g[0].(map[string]any)["n"] != 2.0 ||
		len(g[0].(map[string]any)["valori"].([]any)) != 2 || g[0].(map[string]any)["valori"].([]any)[0] != 5.0 || x["modCos"] != 2.0 || x["tirato"] != 14.0 {
		t.Fatalf("dadi vita per i dadi 3D: %v", x)
	}
	tv.luca.deve(409, "POST", "/riposi/"+idb+"/dadi-vita", map[string]any{"personaggio": arin, "dadi": map[string]int{"10": 1}})
	if d := tv.m.deve(200, "GET", "/personaggi/"+arin, nil)["dati"].(map[string]any); d["f"].(map[string]any)["hdUsati"] != "3" {
		t.Fatalf("Dadi Vita usati 1 + 2 = 3: %v", d["f"].(map[string]any)["hdUsati"])
	}
	if g := tv.m.deve(200, "GET", "/riposi", nil)["gruppi"].([]any); g[0].(map[string]any)["ora"] != 9.0 && g[1].(map[string]any)["ora"] != 9.0 {
		t.Fatalf("riposo breve: 1 ora in più: %v", g)
	}
}
