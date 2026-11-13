package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scrivi(t *testing.T, p string, v any) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	var b []byte
	if s, ok := v.(string); ok {
		b = []byte(s)
	} else {
		b, _ = json.Marshal(v)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrazioneDaGasterBlessDati(t *testing.T) {
	a := nuovoAmbiente(t)
	a.master()
	s := a.s
	d := s.Cfg.CartellaMedia
	neb := nebbiaVuota()
	neb[0] = 0xFF
	scrivi(t, filepath.Join(d, "gruppi.json"), map[string]any{"active": "g1", "list": []any{
		map[string]any{"id": "g1", "name": "Martedì", "day": "Martedì", "pw": "corvo", "invite": "abc", "folder": "g1"}}})
	scrivi(t, filepath.Join(d, "Sessioni", "g1", "stato.json"), map[string]any{"fog": base64.StdEncoding.EncodeToString(neb), "miHex": 0.5,
		"pubNames": map[string]bool{"borgofosco": true}, "pubItems": map[string]bool{"it1": true}, "hideBuiltin": map[string]bool{},
		"proj": map[string]any{"kind": "map"}, "shown": []string{"best/acolyte.webp"}, "campoStrano": 1})
	scrivi(t, filepath.Join(d, "Sessioni", "g1", "registro.txt"), "2026-10-01 sessione 1\r\n15/10/2026 sessione 2\r\n")
	scrivi(t, filepath.Join(d, "contenuti.json"), "\uFEFF"+`{"locs":{"borgofosco":{"desc":"La città del Barone","notes":"Segreto","items":[{"id":"it1","k":"vid","file":"Video/Borgofosco/arrivo.mp4","cap":"Arrivo"},{"id":"it2","k":"vid","file":"786a50431b4a68b4.mp4"}]},"inventata":{"desc":"x"}}}`)
	scrivi(t, filepath.Join(d, "Video", "Borgofosco", "arrivo.mp4"), "....ftypmp42")
	scrivi(t, filepath.Join(d, "Video", "Borgofosco", "786a50431b4a68b4.mp4"), "....ftypmp42") // salvato solo con il nome
	scrivi(t, filepath.Join(d, "bestiario.json"), map[string]any{"entries": map[string]any{"mm-acolyte": map[string]any{"notes": "Donavich", "man": "m1", "page": 46}},
		"extras": []any{}, "manuals": []any{map[string]any{"id": "m1", "nome": "Manuale dei Mostri", "tipo": "mm", "file": "mm.pdf", "refBook": 10, "refPdf": 12, "spread": false}}})
	scrivi(t, filepath.Join(d, "Manuali", "mm.pdf"), "%PDF-1.4")
	scrivi(t, filepath.Join(d, "musica.json"), map[string]any{"volume": 0.4, "mode": "shuffle", "out": "master", "fight": []string{"Combattimento/a.mp3"}, "locs": map[string]string{"borgofosco": "b.mp3"}})
	prima, _ := os.ReadFile(filepath.Join(d, "contenuti.json"))

	// Prova a secco: rapporto, nessuna scrittura.
	rap, err := s.Migra(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM gruppi`).Scan(&n)
	if n != 0 || !strings.Contains(rap, "PROVA") || !strings.Contains(rap, "inventata") || !strings.Contains(rap, "campoStrano") || strings.Contains(rap, "shown") {
		t.Fatalf("la prova non deve scrivere e deve segnalare ciò che non riconosce (gruppi: %d):\n%s", n, rap)
	}
	// Migrazione vera.
	rap, err = s.Migra(context.Background(), false)
	if err != nil {
		t.Fatalf("%v\n%s", err, rap)
	}
	var g string
	s.DB.QueryRow(`SELECT id FROM gruppi WHERE nome = 'Martedì'`).Scan(&g)
	st, _ := s.leggiStatoMappa(g)
	if g == "" || !st.Nomi["borgofosco"] || !st.Media["it1"] || st.Scala != 0.5 || st.Nebbia[0] != 0xFF || st.Proiez["kind"] != "map" {
		t.Fatalf("stato della mappa non migrato: %+v", st)
	}
	var desc, file string
	s.DB.QueryRow(`SELECT descrizione FROM localita_contenuti WHERE localita = 'borgofosco'`).Scan(&desc)
	s.DB.QueryRow(`SELECT file FROM media WHERE id = 'it1'`).Scan(&file)
	if desc != "La città del Barone" || file != "Video/Borgofosco/arrivo.mp4" {
		t.Fatalf("contenuti: %q %q", desc, file)
	}
	s.DB.QueryRow(`SELECT file FROM media WHERE id = 'it2'`).Scan(&file)
	if file != "Video/Borgofosco/786a50431b4a68b4.mp4" {
		t.Fatalf("il file salvato solo con il nome va cercato nelle cartelle: %q", file)
	}
	s.DB.QueryRow(`SELECT COUNT(*) FROM sessioni_gioco WHERE gruppo_id = ? AND stato = 'conclusa'`, g).Scan(&n)
	if n != 2 {
		t.Fatalf("sessioni dal registro: %d", n)
	}
	var pag int
	s.DB.QueryRow(`SELECT pagina FROM bestiario_personalizzazioni WHERE voce_id = 'mm-acolyte' AND manuale_id = 'm1'`).Scan(&pag)
	s.DB.QueryRow(`SELECT COUNT(*) FROM manuali WHERE id = 'm1' AND ref_pdf = 12`).Scan(&n)
	if pag != 46 || n != 1 {
		t.Fatalf("bestiario e manuali: pagina %d, manuali %d", pag, n)
	}
	if m := s.mus(); m.cfg.Volume != 0.4 || m.cfg.Mode != "shuffle" || m.cfg.Locs["borgofosco"] != "b.mp3" {
		t.Fatalf("musica: %+v", m.cfg)
	}
	if l := s.ElencoBackup(); len(l) != 1 {
		t.Fatalf("prima della migrazione serve un backup: %v", l)
	}
	// GasterBlessDati non viene toccata; una seconda migrazione viene rifiutata.
	dopo, _ := os.ReadFile(filepath.Join(d, "contenuti.json"))
	if string(prima) != string(dopo) {
		t.Fatal("la cartella della vecchia Gaster Bless non deve essere modificata")
	}
	if _, err := s.Migra(context.Background(), false); err == nil {
		t.Fatal("una seconda migrazione deve essere rifiutata")
	}
}
