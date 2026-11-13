package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"gasterbless/internal/config"
)

func TestBackupERipristino(t *testing.T) {
	sc := nuovoScontro(t) // Master, gruppi, giocatori, PG "Arin", sessione
	s := sc.a.s
	cfg := s.Cfg
	sc.luca.deve(200, "PATCH", "/personaggi/"+sc.pg+"/scheda", map[string]any{"base": 2, "modifiche": map[string]any{"al": "Caotico buono"}})
	// Background (file cifrato nella cartella dati).
	if c := sc.invia(sc.luca, "PUT", "../personaggi/"+sc.pg+"/background", []byte("%PDF-1.4 storia")); c != 200 {
		t.Fatalf("background: %d", c)
	}
	sc.luca.deve(403, "GET", "/backup", nil)
	nome := sc.m.deve(200, "POST", "/backup", map[string]any{})["nome"].(string)
	if l := sc.m.deve(200, "GET", "/backup", nil)["backup"].([]any); len(l) != 1 {
		t.Fatalf("elenco dei backup: %v", l)
	}
	// Il file del backup è cifrato: non contiene testo leggibile.
	b, _ := os.ReadFile(filepath.Join(cfg.CartellaDati, "backup", nome))
	if len(b) == 0 || bytesContains(b, "Arin") || bytesContains(b, "SQLite format") {
		t.Fatal("il backup deve essere cifrato")
	}
	// Dopo il backup qualcosa cambia (per errore): il personaggio viene eliminato.
	sc.m.deve(200, "DELETE", "/utenti/"+sc.idLuca+"?schede=elimina", nil)
	codice, err := CodiceRipristino(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sc.a.srv.Close()
	s.Chiudi()

	// 1) Ripristino sullo stesso PC.
	parte, err := Ripristina(cfg, nome, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(parte, "gasterbless.db")); err != nil {
		t.Fatal("lo stato precedente va messo da parte, non cancellato")
	}
	s2, err := Nuovo(cfg, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	var nomePG, al string
	s2.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ?`, sc.pg).Scan(&nomePG)
	if nomePG != "Arin" {
		t.Fatal("dopo il ripristino il personaggio deve esserci di nuovo")
	}
	var vc int
	s2.DB.QueryRow(`SELECT versione_corrente FROM personaggi WHERE id = ?`, sc.pg).Scan(&vc)
	d, _ := s2.schedaCompleta(sc.pg, vc)
	if !bytesContains(d, "Caotico buono") {
		_ = al
		t.Fatalf("scheda ripristinata: %s", d)
	}
	var file string
	s2.DB.QueryRow(`SELECT file FROM background WHERE personaggio_id = ?`, sc.pg).Scan(&file)
	if dati, err := s2.leggiFileCifrato(file, "background:"+sc.pg); err != nil || string(dati) != "%PDF-1.4 storia" {
		t.Fatalf("background ripristinato: %v %q", err, dati)
	}
	if _, err := s2.CreaBackup(context.Background()); err != nil {
		t.Fatal(err)
	}
	s2.Chiudi()

	// 2) Ripristino su un "PC nuovo": cartella vuota, solo il file del backup e il codice.
	nuovo := config.Predefinita()
	nuovo.CartellaDati, nuovo.CartellaMedia, nuovo.ModalitaProva = t.TempDir(), t.TempDir(), true
	os.MkdirAll(filepath.Join(nuovo.CartellaDati, "backup"), 0o700)
	b, _ = os.ReadFile(filepath.Join(cfg.CartellaDati, "backup", nome))
	os.WriteFile(filepath.Join(nuovo.CartellaDati, "backup", nome), b, 0o600)
	if _, err := Ripristina(nuovo, nome, "AAAA-BBBB"); err == nil {
		t.Fatal("un codice sbagliato deve essere rifiutato")
	}
	if _, err := Ripristina(nuovo, nome, codice); err != nil {
		t.Fatalf("ripristino con il codice: %v", err)
	}
	s3, err := Nuovo(nuovo, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Chiudi()
	var tel string
	s3.DB.QueryRow(`SELECT COUNT(*) FROM personaggi WHERE id = ?`, sc.pg).Scan(&tel)
	if v, err := s3.telefonoDi(sc.idLuca); err != nil || v == "" {
		t.Fatalf("i dati cifrati (telefono) devono leggersi sul PC nuovo: %v", err)
	}
}

func bytesContains[T ~[]byte | ~string](b T, s string) bool {
	return len(b) > 0 && (string(b) != "" && containsStr(string(b), s))
}

func containsStr(a, b string) bool {
	for i := 0; i+len(b) <= len(a); i++ {
		if a[i:i+len(b)] == b {
			return true
		}
	}
	return false
}
