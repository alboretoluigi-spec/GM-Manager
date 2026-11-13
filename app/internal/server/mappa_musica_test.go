package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestMusicaECanti(t *testing.T) {
	sc := nuovoScontro(t)
	dir := filepath.Join(sc.a.s.Cfg.CartellaMedia, "Musica")
	for _, f := range []string{"a-nebbia.mp3", "b-taverna.mp3", "Combattimento/scontro.mp3", "Canti/Viandante/ballata.mp3"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("ID3 finto"), 0o644)
	}
	mu := sc.mappa(sc.m, 200, "GET", "music", nil)
	if len(mu["files"].([]any)) != 3 || len(mu["fight"].([]any)) != 1 {
		t.Fatalf("elenco dei brani (i canti esclusi) e brani di combattimento: %v", mu)
	}
	cmd := func(c *cliente, codice int, d map[string]any) map[string]any {
		return sc.mappa(c, codice, "POST", "music/cmd", d)
	}
	cmd(sc.luca, 403, map[string]any{"op": "next"})
	st := cmd(sc.m, 200, map[string]any{"op": "play", "file": "a-nebbia.mp3"})["state"].(map[string]any)
	if st["file"] != "a-nebbia.mp3" || st["playing"] != true {
		t.Fatalf("play: %v", st)
	}
	// Combattimento: parte il brano degli scontri; alla fine si torna a quello di prima.
	st = cmd(sc.m, 200, map[string]any{"op": "fight", "on": true})["state"].(map[string]any)
	if st["file"] != "Combattimento/scontro.mp3" || st["fight"] != true {
		t.Fatalf("combattimento: %v", st)
	}
	st = cmd(sc.m, 200, map[string]any{"op": "fight", "on": false})["state"].(map[string]any)
	if st["file"] != "a-nebbia.mp3" {
		t.Fatalf("fine combattimento: si torna al brano di prima: %v", st)
	}
	st = cmd(sc.m, 200, map[string]any{"op": "next"})["state"].(map[string]any)
	if st["file"] != "b-taverna.mp3" {
		t.Fatalf("brano successivo: %v", st)
	}
	cmd(sc.m, 404, map[string]any{"op": "play", "file": "../segreto.mp3"})
	cmd(sc.m, 200, map[string]any{"op": "setloc", "k": "borgofosco", "file": "b-taverna.mp3"})
	cmd(sc.m, 200, map[string]any{"op": "volume", "value": 0.3})
	if c := sc.mappa(sc.m, 200, "GET", "music", nil)["cfg"].(map[string]any)["volume"].(float64); c != 0.3 {
		t.Fatalf("volume salvato: %v", c)
	}
	// I file audio li scarica solo il Master.
	if c := sc.scaricaPercorso(sc.luca, "/musica/a-nebbia.mp3"); c != 404 {
		t.Fatalf("il giocatore non scarica la musica: %d", c)
	}
	if c := sc.scaricaPercorso(sc.m, "/musica/a-nebbia.mp3"); c != 200 {
		t.Fatalf("il Master sì: %d", c)
	}
	// Canti: il giocatore avvia il Viandante; un secondo canto insieme no; il Master blocca il bardico.
	sc.mappa(sc.luca, 200, "POST", "song", map[string]any{"kind": "viandante"})
	sc.mappa(sc.sara, 409, "POST", "song", map[string]any{"kind": "viandante"})
	if s := sc.mappa(sc.sara, 200, "GET", "song", nil); s["song"] != "viandante" {
		t.Fatalf("canto in corso: %v", s)
	}
	sc.mappa(sc.luca, 403, "POST", "song", map[string]any{"op": "stop"})
	sc.mappa(sc.m, 200, "POST", "song", map[string]any{"op": "end"})
	sc.mappa(sc.m, 200, "POST", "song", map[string]any{"op": "lock", "kind": "viandante", "on": true})
	sc.mappa(sc.luca, 403, "POST", "song", map[string]any{"kind": "viandante"})
	sc.mappa(sc.luca, 409, "POST", "song", map[string]any{"kind": "bardo"}) // cartella del bardo vuota
	sc.mappa(sc.ugo, 404, "POST", "song", map[string]any{"kind": "bardo"})  // non partecipa
}

func (sc *scontro) scaricaPercorso(c *cliente, p string) int {
	req, _ := http.NewRequest("GET", sc.a.srv.URL+p, nil)
	if c.ip != "" {
		req.Header.Set("X-Test-IP", c.ip)
	}
	r, err := c.c.Do(req)
	if err != nil {
		sc.a.t.Fatal(err)
	}
	r.Body.Close()
	return r.StatusCode
}

// Video nei canti (dal ramo main-ke8sy4): solo il Master li avvia; il file si scarica solo durante il canto.
func TestCantiVideo(t *testing.T) {
	sc := nuovoScontro(t)
	dir := filepath.Join(sc.a.s.Cfg.CartellaMedia, "Musica", "Canti", "Viandante")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "danza.mp4"), []byte("\x00\x00\x00\x18ftypmp42 video di prova"), 0o644)
	os.WriteFile(filepath.Join(dir, "canto.mp3"), []byte("ID3 audio"), 0o644)
	st := sc.mappa(sc.m, 200, "GET", "song", nil)
	v := st["videos"].(map[string]any)["viandante"].([]any)
	if len(v) != 1 || v[0] != "Canti/Viandante/danza.mp4" {
		t.Fatalf("il Master vede i video della cartella del canto: %v", v)
	}
	if _, ok := sc.mappa(sc.luca, 200, "GET", "song", nil)["videos"]; ok {
		t.Fatal("ai giocatori l'elenco dei video non serve")
	}
	video := "/api/v1/mappa/songvideo?sessione=" + sc.ses
	if c := sc.scaricaPercorso(sc.luca, video); c != 404 {
		t.Fatalf("senza canto in corso il video non si scarica: %d", c)
	}
	sc.mappa(sc.luca, 403, "POST", "song", map[string]any{"kind": "viandante", "video": "Canti/Viandante/danza.mp4"})
	sc.mappa(sc.m, 400, "POST", "song", map[string]any{"kind": "viandante", "video": "../../segreto.mp4"})
	sc.mappa(sc.m, 400, "POST", "song", map[string]any{"kind": "bardo", "video": "Canti/Viandante/danza.mp4"})
	sc.mappa(sc.m, 200, "POST", "song", map[string]any{"kind": "viandante", "video": "Canti/Viandante/danza.mp4"})
	if s := sc.mappa(sc.luca, 200, "GET", "song", nil); s["song"] != "viandante" || s["video"] != true {
		t.Fatalf("canto con video in corso: %v", s)
	}
	if c := sc.scaricaPercorso(sc.luca, video); c != 200 {
		t.Fatalf("durante il canto il giocatore scarica il video: %d", c)
	}
	if c := sc.scaricaPercorso(sc.ugo, "/api/v1/mappa/songvideo?sessione="+sc.ses); c != 404 {
		t.Fatalf("chi non partecipa non lo scarica: %d", c)
	}
	sc.mappa(sc.m, 200, "POST", "song", map[string]any{"op": "stop"})
	if c := sc.scaricaPercorso(sc.luca, video); c != 404 {
		t.Fatalf("finito il canto il video non si scarica più: %d", c)
	}
}

// All'inizio dello scontro dal tracker la musica passa da sola al combattimento; alla fine torna il brano di prima.
func TestMusicaDelloScontro(t *testing.T) {
	sc := nuovoScontro(t)
	dir := filepath.Join(sc.a.s.Cfg.CartellaMedia, "Musica")
	for _, f := range []string{"a-nebbia.mp3", "Combattimento/scontro.mp3"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("ID3 finto"), 0o644)
	}
	sc.mappa(sc.m, 200, "POST", "music/cmd", map[string]any{"op": "play", "file": "a-nebbia.mp3"})
	sc.az(sc.m, 200, "iniz.aggiungi_mostri", map[string]any{"nome": "Lupo", "quanti": 1, "ca": 13, "pf": 11})
	// iniziative fissate: tirate a caso potrebbero uscire pari, e con i pareggi lo scontro aspetta la scelta del Master
	vm := sc.vista(sc.m)
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": cerca(vm, "Arin")["id"], "valore": 15})
	sc.az(sc.m, 200, "iniz.iniziativa", map[string]any{"combattente": cerca(vm, "Lupo")["id"], "valore": 10})
	r := sc.az(sc.m, 200, "iniz.avvia", nil)
	mu, _ := r["musica"].(map[string]any)
	if mu == nil || mu["fight"] != true || mu["file"] != "Combattimento/scontro.mp3" || mu["playing"] != true {
		t.Fatalf("all'inizio dello scontro: musica di combattimento: %v", r)
	}
	if st := sc.mappa(sc.m, 200, "GET", "music", nil)["state"].(map[string]any); st["fight"] != true {
		t.Fatalf("stato della musica: %v", st)
	}
	sc.az(sc.m, 200, "iniz.fine", nil)
	if st := sc.mappa(sc.m, 200, "GET", "music", nil)["state"].(map[string]any); st["fight"] != false || st["file"] != "a-nebbia.mp3" {
		t.Fatalf("alla fine dello scontro torna il brano di prima: %v", st)
	}
}
