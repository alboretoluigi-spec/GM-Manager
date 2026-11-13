package server

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Villaggio di Nebbiavalle (1249, 622) e Borgofosco: due località note di locs.json.
const villaggio, borgofosco = "villaggio-di-nebbiavalle", "borgofosco"

func (sc *scontro) mappa(c *cliente, codice int, metodo, p string, dati any) map[string]any {
	sc.a.t.Helper()
	sep := "?"
	if bytes.ContainsRune([]byte(p), '?') {
		sep = "&"
	}
	return c.deve(codice, metodo, "/mappa/"+p+sep+"sessione="+sc.ses, dati)
}

// nebbiaSvelata scopre la cella che contiene (x, y).
func nebbiaSvelata(x, y int) string {
	b := nebbiaVuota()
	i := (y/cellaNebbia)*colonneNebbia + x/cellaNebbia
	b[i>>3] |= 1 << (i & 7)
	return base64.StdEncoding.EncodeToString(b)
}

func (sc *scontro) scarica(c *cliente, rel string) int {
	req, _ := http.NewRequest("GET", sc.a.srv.URL+"/media/"+rel, nil)
	if c.ip != "" {
		req.Header.Set("X-Test-IP", c.ip)
	}
	r, err := c.c.Do(req)
	if err != nil {
		sc.a.t.Fatal(err)
	}
	io.Copy(io.Discard, r.Body)
	r.Body.Close()
	return r.StatusCode
}

func TestMappaVisibilitaPerGiocatore(t *testing.T) {
	sc := nuovoScontro(t)
	// Un file messo a mano nella cartella Immagini\Borgofosco e uno nella cartella del villaggio.
	media := sc.a.s.Cfg.CartellaMedia
	for _, d := range []string{"Immagini/Borgofosco", "Immagini/Villaggio di Nebbiavalle"} {
		os.MkdirAll(filepath.Join(media, filepath.FromSlash(d)), 0o755)
	}
	png := []byte("\x89PNG\r\n\x1a\nfinto")
	os.WriteFile(filepath.Join(media, "Immagini", "Borgofosco", "piazza.png"), png, 0o644)
	os.WriteFile(filepath.Join(media, "Immagini", "Villaggio di Nebbiavalle", "chiesa.png"), png, 0o644)
	os.WriteFile(filepath.Join(media, "Immagini", "sciolto.png"), png, 0o644)
	os.WriteFile(filepath.Join(sc.a.s.Cfg.CartellaDati, "segreto.png"), png, 0o644)

	// "Cerca nelle cartelle": le sottocartelle con il nome di una località si collegano da sole.
	r := sc.mappa(sc.m, 200, "POST", "folders", map[string]any{})
	if r["linked"].(float64) != 2 || len(r["loose"].([]any)) != 1 {
		t.Fatalf("collegamento delle cartelle: %v", r)
	}
	sc.mappa(sc.m, 200, "POST", "loc", map[string]any{"k": villaggio, "desc": "Case vuote e nebbia.", "notes": "Aldric è alla taverna."})
	sc.mappa(sc.m, 200, "POST", "loc", map[string]any{"k": borgofosco, "desc": "La città del Barone.", "notes": "Segreto"})

	// All'inizio tutto è coperto: il giocatore non riceve nulla.
	st := sc.mappa(sc.luca, 200, "GET", "state", nil)
	if len(st["locs"].(map[string]any)) != 0 {
		t.Fatalf("con la nebbia il giocatore non deve ricevere località: %v", st["locs"])
	}
	// Il Master svela il nome di Borgofosco ma la nebbia la copre ancora: niente.
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"kind": "name", "key": borgofosco, "val": true})
	if len(sc.mappa(sc.luca, 200, "GET", "state", nil)["locs"].(map[string]any)) != 0 {
		t.Fatal("un nome svelato sotto la nebbia non deve arrivare al giocatore")
	}
	// Scopre la nebbia sul villaggio e ne svela il nome: arriva nome e descrizione, non le note né i media.
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"fog": nebbiaSvelata(1249, 622)})
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"kind": "name", "key": villaggio, "val": true})
	l := sc.mappa(sc.luca, 200, "GET", "state", nil)["locs"].(map[string]any)
	v, ok := l[villaggio].(map[string]any)
	if !ok || len(l) != 1 || v["desc"] != "Case vuote e nebbia." || v["notes"] != nil || len(v["items"].([]any)) != 0 || v["builtin"] != true {
		t.Fatalf("vista del giocatore sul villaggio: %v", l)
	}
	// Il file della chiesa non è svelato: il giocatore non può scaricarlo, il Master sì.
	rel := "Immagini/Villaggio di Nebbiavalle/chiesa.png"
	if c := sc.scarica(sc.luca, rel); c != 404 {
		t.Fatalf("file non svelato: atteso 404, ottenuto %d", c)
	}
	if c := sc.scarica(sc.m, rel); c != 200 {
		t.Fatalf("il Master deve poter aprire il file: %d", c)
	}
	// Svelato il media: ora sì.
	var id string
	for _, it := range sc.mappa(sc.m, 200, "GET", "state", nil)["content"].(map[string]any)[villaggio].(map[string]any)["items"].([]any) {
		id = it.(map[string]any)["id"].(string)
	}
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"kind": "item", "key": id, "val": true})
	if c := sc.scarica(sc.luca, rel); c != 200 {
		t.Fatalf("file svelato: atteso 200, ottenuto %d", c)
	}
	// Ugo, di un altro gruppo, non vede né la mappa né il file.
	sc.mappa(sc.ugo, 404, "GET", "state", nil)
	if c := sc.scarica(sc.ugo, rel); c != 404 {
		t.Fatalf("un giocatore di un altro gruppo non deve aprire il file: %d", c)
	}
	// Percorsi fuori dalla cartella dei media: rifiutati.
	for _, p := range []string{"../segreto.png", "..%2Fsegreto.png", "Immagini/../../segreto.png"} {
		if c := sc.scarica(sc.m, p); c == 200 {
			t.Fatalf("percorso %q fuori dalla cartella dei media non deve essere servito", p)
		}
	}
	// Comandi del Master vietati ai giocatori.
	sc.mappa(sc.luca, 403, "POST", "group", map[string]any{"fog": nebbiaSvelata(0, 0)})
	sc.mappa(sc.luca, 403, "POST", "loc", map[string]any{"k": villaggio, "desc": "x"})
	sc.mappa(sc.luca, 403, "POST", "proj", map[string]any{"kind": "map"})
	sc.mappa(sc.m, 400, "POST", "item", map[string]any{"k": villaggio, "file": "../segreto.png"})
}

func TestMappaProiezioneSvelaAiGiocatori(t *testing.T) {
	sc := nuovoScontro(t)
	media := sc.a.s.Cfg.CartellaMedia
	os.MkdirAll(filepath.Join(media, "Immagini"), 0o755)
	os.WriteFile(filepath.Join(media, "Immagini", "antagonista.png"), []byte("\x89PNG\r\n\x1a\nfinto"), 0o644)
	sc.mappa(sc.m, 200, "POST", "item", map[string]any{"k": villaggio, "file": "Immagini/antagonista.png"})
	var id string
	for _, it := range sc.mappa(sc.m, 200, "GET", "state", nil)["content"].(map[string]any)[villaggio].(map[string]any)["items"].([]any) {
		id = it.(map[string]any)["id"].(string)
	}
	if c := sc.scarica(sc.luca, "Immagini/antagonista.png"); c != 404 {
		t.Fatalf("prima della proiezione: %d", c)
	}
	sc.mappa(sc.m, 200, "POST", "proj", map[string]any{"kind": "item", "id": id})
	if c := sc.scarica(sc.luca, "Immagini/antagonista.png"); c != 200 {
		t.Fatalf("ciò che si proietta diventa visibile ai giocatori: %d", c)
	}
	if p := sc.mappa(sc.m, 200, "GET", "proj", nil); p["id"] != id {
		t.Fatalf("proiezione non salvata: %v", p)
	}
	// Caricamento: solo immagini e video veri.
	carica := func(dati []byte) int {
		req, _ := http.NewRequest("POST", sc.a.srv.URL+"/api/v1/mappa/upload?sessione="+sc.ses+"&k="+villaggio+"&cap=prova", bytes.NewReader(dati))
		req.Header.Set("X-CSRF", sc.m.csrf)
		r, err := sc.m.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if c := carica([]byte("<script>alert(1)</script>")); c != 415 {
		t.Fatalf("file non multimediale: atteso 415, ottenuto %d", c)
	}
	if c := carica([]byte("\x89PNG\r\n\x1a\n0000000000")); c != 200 {
		t.Fatalf("immagine: atteso 200, ottenuto %d", c)
	}
	if _, err := os.Stat(filepath.Join(media, "Immagini", "Villaggio di Nebbiavalle")); err != nil {
		t.Fatal("l'immagine caricata va nella cartella con il nome della località")
	}
}

func segnalini(p map[string]any) []map[string]any {
	var out []map[string]any
	for _, x := range p["segnalini"].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

func nomiPG(x map[string]any) string {
	var n []string
	for _, p := range x["pg"].([]any) {
		n = append(n, p.(map[string]any)["nome"].(string))
	}
	return strings.Join(n, ",")
}

func TestMappaSegnalinoDelGruppo(t *testing.T) {
	sc := nuovoScontro(t) // Luca partecipa con Arin; Sara (Segretario) partecipa con Bryn
	bryn := sc.sara.deve(200, "POST", "/personaggi", map[string]any{"nome": "Bryn"})["id"].(string)
	sc.sara.deve(200, "POST", "/sessioni/"+sc.ses+"/conferma", map[string]any{"confermato": true, "personaggio": bryn})
	stato := func(c *cliente) map[string]any {
		p, _ := sc.mappa(c, 200, "GET", "state", nil)["party"].(map[string]any)
		return p
	}
	if p := stato(sc.luca); p != nil {
		t.Fatalf("all'inizio il gruppo non è posizionato: %v", p)
	}
	// Anche i giocatori della sessione posizionano e spostano; chi non partecipa no.
	sc.ugo.deve(404, "POST", "/mappa/party?sessione="+sc.ses, map[string]any{"azione": "posiziona", "x": 100, "y": 100})
	sc.mappa(sc.luca, 400, "POST", "party", map[string]any{"azione": "posiziona", "x": 5000, "y": 100})
	sc.mappa(sc.luca, 200, "POST", "party", map[string]any{"azione": "posiziona", "x": 1249, "y": 622})
	l := segnalini(stato(sc.luca))
	if len(l) != 1 || l[0]["x"].(float64) != 1249 || nomiPG(l[0]) != "Arin,Bryn" || l[0]["principale"] != true {
		t.Fatalf("segnalino del gruppo: %v", l)
	}
	sc.mappa(sc.sara, 200, "POST", "party", map[string]any{"azione": "muovi", "id": "g", "x": 800, "y": 500})
	// Divisione: Arin si separa. Non si può separare tutto il gruppo né qualcuno che non c'è.
	sc.mappa(sc.luca, 400, "POST", "party", map[string]any{"azione": "dividi", "id": "g", "pg": []string{sc.pg, bryn}, "x": 900, "y": 500})
	sc.mappa(sc.luca, 400, "POST", "party", map[string]any{"azione": "dividi", "id": "g", "pg": []string{"inventato"}, "x": 900, "y": 500})
	sc.mappa(sc.luca, 200, "POST", "party", map[string]any{"azione": "dividi", "id": "g", "pg": []string{sc.pg}, "x": 900, "y": 500})
	l = segnalini(stato(sc.luca))
	if len(l) != 2 || nomiPG(l[0]) != "Bryn" || nomiPG(l[1]) != "Arin" || l[1]["x"].(float64) != 900 {
		t.Fatalf("dopo la divisione: %v", l)
	}
	altro := l[1]["id"].(string)
	sc.mappa(sc.sara, 200, "POST", "party", map[string]any{"azione": "muovi", "id": altro, "x": 300, "y": 300})
	// Il principale non si "riunisce"; il secondario sì.
	sc.mappa(sc.luca, 404, "POST", "party", map[string]any{"azione": "riunisci", "id": "g"})
	// Aspetto e rimozione: solo il Master.
	sc.mappa(sc.luca, 403, "POST", "party", map[string]any{"azione": "aspetto", "nome": "Lupi", "colore": "#336699", "simbolo": "🐺"})
	sc.mappa(sc.m, 400, "POST", "party", map[string]any{"azione": "aspetto", "nome": "Lupi", "colore": "blu", "simbolo": "🐺"})
	sc.mappa(sc.m, 400, "POST", "party", map[string]any{"azione": "aspetto", "nome": "Lupi", "colore": "#336699", "simbolo": "<script>"})
	sc.mappa(sc.m, 200, "POST", "party", map[string]any{"azione": "aspetto", "nome": "Persi nella nebbia", "colore": "#336699", "simbolo": "🐺"})
	p := stato(sc.luca)
	if p["nome"] != "Persi nella nebbia" || p["colore"] != "#336699" || p["simbolo"] != "🐺" {
		t.Fatalf("aspetto: %v", p)
	}
	sc.mappa(sc.luca, 200, "POST", "party", map[string]any{"azione": "riunisci", "id": altro})
	if l = segnalini(stato(sc.luca)); len(l) != 1 || nomiPG(l[0]) != "Arin,Bryn" {
		t.Fatalf("dopo la riunione: %v", l)
	}
	sc.mappa(sc.luca, 403, "POST", "party", map[string]any{"azione": "togli"})
	sc.mappa(sc.m, 200, "POST", "party", map[string]any{"azione": "togli"})
	if p := sc.mappa(sc.m, 200, "GET", "state", nil)["g"].(map[string]any)["party"].(map[string]any); len(segnalini(p)) != 0 || p["nome"] != "Persi nella nebbia" {
		t.Fatalf("tolto il segnalino resta l'aspetto: %v", p)
	}
	// Dati salvati dalla versione precedente ({"x":..,"y":..}): un segnalino principale.
	var g string
	sc.a.s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, sc.ses).Scan(&g)
	sc.a.s.DB.Exec(`UPDATE stato_mappa SET posizione = '{"x":10,"y":20}' WHERE gruppo_id = ?`, g)
	if l = segnalini(stato(sc.luca)); len(l) != 1 || l[0]["y"].(float64) != 20 {
		t.Fatalf("formato precedente: %v", l)
	}
}

func TestMappeLocaliSoloSeSvelate(t *testing.T) {
	sc := nuovoScontro(t)
	get := func(c *cliente) int { return sc.scaricaPercorso(c, "/carta/img/lm/borgofosco.webp") }
	if c := get(sc.luca); c != 404 {
		t.Fatalf("mappa locale non svelata: %d", c)
	}
	if c := get(sc.m); c != 200 {
		t.Fatalf("il Master la vede: %d", c)
	}
	// Borgofosco (circa 700, 330): si scopre la nebbia e si svela il nome.
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"fog": base64.StdEncoding.EncodeToString(func() []byte {
		b := nebbiaVuota()
		for i := range b {
			b[i] = 0xFF
		}
		return b
	}())})
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"kind": "name", "key": "borgofosco", "val": true})
	if c := get(sc.luca); c != 200 {
		t.Fatalf("mappa locale svelata: %d", c)
	}
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"kind": "builtin", "key": "borgofosco", "val": false}) // nascosta dal Master
	if c := get(sc.luca); c != 404 {
		t.Fatalf("mappa locale nascosta: %d", c)
	}
}

// Il Master prepara la mappa di un gruppo anche senza una sessione aperta ("g:<gruppo>"); i giocatori del gruppo la
// consultano soltanto (dal 2026-11-08: prima non potevano aprirla), senza modificare nulla.
func TestMappaPreparazioneSenzaSessione(t *testing.T) {
	sc := nuovoScontro(t)
	var g string
	sc.a.s.DB.QueryRow(`SELECT gruppo_id FROM sessioni_gioco WHERE id = ?`, sc.ses).Scan(&g)
	me := sc.m.deve(200, "GET", "/mappa/me?sessione=g:"+g, nil)
	if me["hosting"].(map[string]any)["id"] != g {
		t.Fatalf("il Master deve aprire la mappa del gruppo: %v", me)
	}
	sc.m.deve(200, "GET", "/mappa/state?sessione=g:"+g, nil)
	sc.luca.deve(200, "GET", "/mappa/me?sessione=g:"+g, nil)
	sc.luca.deve(200, "GET", "/mappa/state?sessione=g:"+g, nil)
	sc.luca.deve(403, "POST", "/mappa/party?sessione=g:"+g, map[string]any{"azione": "posiziona", "x": 500, "y": 500})
	sc.m.deve(404, "GET", "/mappa/me?sessione=g:inesistente", nil)
}

// B7: due dispositivi del Master scoprono la nebbia in punti diversi partendo dalla stessa base:
// il secondo salvataggio non deve cancellare il primo.
func TestNebbiaDueDispositivi(t *testing.T) {
	sc := nuovoScontro(t)
	cella := func(x, y int) int { return (y/cellaNebbia)*colonneNebbia + x/cellaNebbia }
	con := func(celle ...int) string {
		b := nebbiaVuota()
		for _, i := range celle {
			b[i>>3] |= 1 << (i & 7)
		}
		return base64.StdEncoding.EncodeToString(b)
	}
	a, b, c := cella(100, 100), cella(700, 330), cella(1200, 600)
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"fog": con(a)}) // stato iniziale: solo a
	base := con(a)
	// PC: scopre b. Tablet (partito dalla stessa base): scopre c e copre a.
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"fog": con(a, b), "base": base})
	out := sc.mappa(sc.m, 200, "POST", "group", map[string]any{"fog": con(c), "base": base})
	if out["fog"] != con(b, c) {
		t.Fatal("la nebbia unita deve avere b (dal PC) e c (dal tablet), e a coperta dal tablet")
	}
	// Senza base (pagine vecchie) si sostituisce tutto, come prima.
	out = sc.mappa(sc.m, 200, "POST", "group", map[string]any{"fog": con(a)})
	if out["fog"] != con(a) {
		t.Fatal("senza base la nebbia si sostituisce")
	}
	sc.mappa(sc.m, 400, "POST", "group", map[string]any{"fog": con(a), "base": "%%%"})
}

// P1: i percorsi si calcolano sul server; i giocatori solo fuori dalla nebbia.
func TestStradeSulServer(t *testing.T) {
	sc := nuovoScontro(t)
	sc.a.s.Web = os.DirFS("testdata/web")
	ag := func(c *cliente, codice, x, y int) int {
		out := sc.mappa(c, codice, "POST", "strada", map[string]any{"op": "aggancia", "x": x, "y": y, "r": 42})
		if codice != 200 {
			return -2
		}
		return int(out["i"].(float64))
	}
	a, b := ag(sc.m, 200, 645, 370), ag(sc.m, 200, 1249, 622) // Borgofosco e il villaggio di Nebbiavalle
	if a < 0 || b < 0 {
		t.Fatalf("il Master deve agganciarsi alle strade: %d %d", a, b)
	}
	out := sc.mappa(sc.m, 200, "POST", "strada", map[string]any{"op": "percorso", "a": a, "b": b})
	pts, _ := out["pts"].([]any)
	if out["nessuno"] != nil || len(pts) < 3 || out["px"].(float64) < 700 {
		t.Fatalf("percorso del Master inatteso: %d punti, %v px", len(pts), out["px"])
	}
	// Giocatore con tutta la nebbia: nessuna strada visibile.
	if i := ag(sc.luca, 200, 645, 370); i != -1 {
		t.Fatalf("sotto la nebbia il giocatore non deve trovare strade: %d", i)
	}
	if o := sc.mappa(sc.luca, 200, "POST", "strada", map[string]any{"op": "percorso", "a": a, "b": b}); o["nessuno"] != true {
		t.Fatal("sotto la nebbia il giocatore non deve avere percorsi")
	}
	// Nebbia tutta scoperta: il giocatore ottiene lo stesso percorso del Master.
	tutto := nebbiaVuota()
	for i := range tutto {
		tutto[i] = 0xFF
	}
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"fog": base64.StdEncoding.EncodeToString(tutto)})
	o := sc.mappa(sc.luca, 200, "POST", "strada", map[string]any{"op": "percorso", "a": a, "b": b})
	if o["px"] != out["px"] {
		t.Fatalf("con la nebbia scoperta il percorso deve essere uguale: %v e %v", o["px"], out["px"])
	}
	sc.mappa(sc.m, 400, "POST", "strada", map[string]any{"op": "percorso", "a": -1, "b": b})
	sc.mappa(sc.m, 400, "POST", "strada", map[string]any{"op": "boh"})
}

// Immagine della mappa coperta dalla nebbia (dal ramo main-ke8sy4): niente spoiler dagli indirizzi dei file.
func TestMappaImmagineConNebbia(t *testing.T) {
	sc := nuovoScontro(t)
	sc.a.s.Web = os.DirFS("testdata/web")
	// (il Master li riceve dal gestore dei file statici: qui l'ambiente di prova non li contiene)
	for _, p := range []string{"/carta/img/gasterbless.webp", "/carta/img/mask.png"} {
		if c := sc.scaricaPercorso(sc.luca, p); c != 404 {
			t.Fatalf("%s: il giocatore non deve scaricare il file completo (%d)", p, c)
		}
	}
	// Si scopre solo la cella di Borgofosco (circa 645, 370).
	sc.mappa(sc.m, 200, "POST", "group", map[string]any{"fog": nebbiaSvelata(645, 370)})
	scarica := func(c *cliente) image.Image {
		req, _ := http.NewRequest("GET", sc.a.srv.URL+"/api/v1/mappa/immagine?sessione="+sc.ses, nil)
		if c.ip != "" {
			req.Header.Set("X-Test-IP", c.ip)
		}
		r, err := c.c.Do(req)
		if err != nil || r.StatusCode != 200 {
			t.Fatalf("immagine: %v %v", err, r)
		}
		defer r.Body.Close()
		im, err := jpeg.Decode(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		return im
	}
	vicino := func(a, b color.Color) bool {
		r1, g1, b1, _ := a.RGBA()
		r2, g2, b2, _ := b.RGBA()
		d := func(x, y uint32) int { return int(x>>8) - int(y>>8) }
		ab := func(n int) int { return max(n, -n) }
		return ab(d(r1, r2)) < 12 && ab(d(g1, g2)) < 12 && ab(d(b1, b2)) < 12
	}
	gioc, master := scarica(sc.luca), scarica(sc.m)
	if !vicino(gioc.At(100, 900), coloreNebbia) {
		t.Fatal("sotto la nebbia il giocatore vede solo il colore della nebbia")
	}
	if vicino(master.At(100, 900), coloreNebbia) && vicino(master.At(1500, 100), coloreNebbia) {
		t.Fatal("il Master vede la mappa completa")
	}
	cx, cy := 645/cellaNebbia*cellaNebbia+cellaNebbia/2, 370/cellaNebbia*cellaNebbia+cellaNebbia/2
	if !vicino(gioc.At(cx, cy), master.At(cx, cy)) {
		t.Fatal("nella zona svelata il giocatore vede la mappa vera")
	}
	if c := sc.scaricaPercorso(sc.ugo, "/api/v1/mappa/immagine?sessione="+sc.ses); c != 404 {
		t.Fatalf("chi non partecipa non la scarica: %d", c)
	}
}
