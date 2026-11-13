package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// iaFinta imita un servizio con il formato di OpenAI (come Ollama): elenco dei modelli e risposta a pezzi.
// Registra l'ultima richiesta ricevuta per controllare cosa Gaster Bless manda all'IA.
func iaFinta(t *testing.T, ultima *map[string]any, chiave *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		muIAFinta.Lock()
		*chiave = r.Header.Get("Authorization")
		muIAFinta.Unlock()
		switch r.URL.Path {
		case "/api/chat": // Ollama nativo
			b, _ := io.ReadAll(r.Body)
			var m map[string]any
			json.Unmarshal(b, &m)
			muIAFinta.Lock()
			*ultima = m
			muIAFinta.Unlock()
			for _, p := range []string{"Risposta ", "da Ollama."} {
				d, _ := json.Marshal(map[string]any{"message": map[string]any{"role": "assistant", "content": p, "thinking": "penso"}, "done": false})
				fmt.Fprintf(w, "%s\n", d)
				w.(http.Flusher).Flush()
			}
			fmt.Fprint(w, `{"done":true}`+"\n")
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"qwen3:8b"},{"id":"gemma3:12b"}]}`)
		case "/v1/chat/completions":
			b, _ := io.ReadAll(r.Body)
			var m map[string]any
			json.Unmarshal(b, &m)
			muIAFinta.Lock()
			*ultima = m
			muIAFinta.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			for _, p := range []string{"<thi", "nk>ragiono…</think>\n", "Il PNG è ", "**Vasilka**", ", una vecchia ", "viandante."} {
				d, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": p}}}})
				fmt.Fprintf(w, "data: %s\n\n", d)
				w.(http.Flusher).Flush()
			}
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			http.NotFound(w, r)
		}
	}))
}

// muIAFinta protegge le richieste registrate dall'IA finta (gli agganci si generano in sottofondo).
var muIAFinta sync.Mutex

func leggiFinta(m *map[string]any) map[string]any {
	muIAFinta.Lock()
	defer muIAFinta.Unlock()
	return *m
}

func (c *cliente) chiediIA(corpo any) (int, []map[string]any) {
	c.a.t.Helper()
	b, _ := json.Marshal(corpo)
	req, _ := http.NewRequest("POST", c.a.srv.URL+"/api/v1/assistente/chiedi", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF", c.csrf)
	req.Header.Set("X-Test-IP", c.ip)
	res, err := c.c.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer res.Body.Close()
	tutto, _ := io.ReadAll(res.Body)
	var righe []map[string]any
	for _, r := range strings.Split(strings.TrimSpace(string(tutto)), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(r), &m) == nil {
			righe = append(righe, m)
		}
	}
	return res.StatusCode, righe
}

func TestAssistenteIA(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	l := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	m.deve(200, "POST", "/utenti/"+a.idUtente(l)+"/approva", map[string]any{"gruppi": []string{g}})
	pg := l.deve(200, "POST", "/personaggi", map[string]any{"gruppo": g, "nome": "Arin"})["id"].(string)
	l.deve(200, "PATCH", "/personaggi/"+pg+"/scheda", map[string]any{"base": versioneDi(l, pg),
		"modifiche": map[string]any{"bg": "criminale", "f.legami": "Mia sorella è scomparsa a Borgofosco."},
		"riepilogo": map[string]any{"class": "Ladro", "level": 3, "race": "Halfling", "hpMax": 20}})
	l.deve(200, "PUT", "/personaggi/"+pg+"/background-testo", map[string]any{"testo": "Arin è cresciuto nei vicoli di Baldur's Gate."})

	// Solo il Master.
	l.deve(403, "GET", "/assistente/impostazioni", nil)
	l.deve(403, "GET", "/assistente/contesto", nil)
	if c, _ := l.chiediIA(map[string]any{"testo": "ciao"}); c != 403 {
		t.Fatalf("giocatore: atteso 403, ottenuto %d", c)
	}
	// Senza servizio scelto: messaggio chiaro.
	if c, r := m.chiediIA(map[string]any{"testo": "ciao"}); c != 409 || !strings.Contains(fmt.Sprint(r), "Impostazioni") {
		t.Fatalf("senza servizio: %d %v", c, r)
	}

	var ultima map[string]any
	var auth string
	ia := iaFinta(t, &ultima, &auth)
	defer ia.Close()
	m.deve(400, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "boh"})
	m.deve(400, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "ollama", "indirizzo": "ftp://x"})
	chiave := "segreta-123"
	imp := m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "altro", "indirizzo": ia.URL + "/v1", "modello": "qwen3:8b", "chiave": chiave})
	if b, _ := json.Marshal(imp); strings.Contains(string(b), chiave) {
		t.Fatal("la chiave non deve tornare alle pagine")
	}
	if imp["chiaviSalvate"].(map[string]any)["altro"] != true {
		t.Fatalf("chiave salvata: %v", imp)
	}
	var grezza string
	a.s.DB.QueryRow(`SELECT valore FROM impostazioni WHERE chiave = 'ia_chiave_altro'`).Scan(&grezza)
	if grezza == "" || strings.Contains(grezza, chiave) {
		t.Fatal("la chiave deve essere salvata cifrata")
	}
	if mod := m.deve(200, "GET", "/assistente/modelli", nil)["modelli"].([]any); len(mod) != 2 || mod[1] != "gemma3:12b" {
		t.Fatalf("modelli: %v", mod)
	}

	// Contesto: dati di gioco, nessun dato personale.
	ctx := m.deve(200, "GET", "/assistente/contesto?gruppo="+g, nil)["contesto"].(string)
	for _, x := range []string{"Martedì", "Arin", "Ladro", "livello 3", "criminale", "sorella", "Baldur"} {
		if !strings.Contains(ctx, x) {
			t.Fatalf("nel contesto manca %q:\n%s", x, ctx)
		}
	}
	for _, x := range []string{"Luca", "333", "1234567"} {
		if strings.Contains(ctx, x) {
			t.Fatalf("il contesto contiene un dato personale %q", x)
		}
	}

	// Domanda: risposta a pezzi, senza i «ragionamenti», salvata nella conversazione.
	c, righe := m.chiediIA(map[string]any{"testo": "Inventa un PNG per la taverna", "gruppo": g})
	if c != 200 || len(righe) < 2 {
		t.Fatalf("risposta: %d %v", c, righe)
	}
	testo := ""
	for _, r := range righe[:len(righe)-1] {
		testo += r["t"].(string)
	}
	if testo != "Il PNG è **Vasilka**, una vecchia viandante." {
		t.Fatalf("testo: %q", testo)
	}
	fine := righe[len(righe)-1]
	if fine["fine"] != true {
		t.Fatalf("fine: %v", fine)
	}
	conv := fine["conversazione"].(string)
	if auth != "Bearer "+chiave {
		t.Fatalf("chiave inviata al servizio: %q", auth)
	}
	msgs := ultima["messages"].([]any)
	sis := msgs[0].(map[string]any)["content"].(string)
	if !strings.Contains(sis, "Arin") || strings.Contains(sis, "Luca") || ultima["stream"] != true || ultima["model"] != "qwen3:8b" {
		t.Fatalf("richiesta all'IA: %v", ultima)
	}
	// Seconda domanda nella stessa conversazione: l'IA riceve anche la risposta precedente.
	m.chiediIA(map[string]any{"testo": "E il suo segreto?", "conversazione": conv})
	msgs = ultima["messages"].([]any)
	if len(msgs) != 4 || msgs[2].(map[string]any)["role"] != "assistant" {
		t.Fatalf("storia della conversazione: %v", msgs)
	}
	lista := m.deve(200, "GET", "/assistente/conversazioni", nil)["lista"].([]any)
	if len(lista) != 1 || lista[0].(map[string]any)["titolo"] != "Inventa un PNG per la taverna" {
		t.Fatalf("conversazioni: %v", lista)
	}
	if mm := m.deve(200, "GET", "/assistente/conversazioni/"+conv, nil)["messaggi"].([]any); len(mm) != 4 {
		t.Fatalf("messaggi salvati: %v", mm)
	}
	m.deve(200, "DELETE", "/assistente/conversazioni/"+conv, nil)
	m.deve(404, "GET", "/assistente/conversazioni/"+conv, nil)

	// Fase IA-2: appunti del Master sui personaggi (solo Master), letti dall'IA e mostrati nel Background.
	l.deve(403, "PUT", "/personaggi/"+pg+"/note-master", map[string]any{"testo": "x"})
	l.deve(403, "GET", "/personaggi/"+pg+"/note-master", nil)
	m.deve(404, "PUT", "/personaggi/nessuno/note-master", map[string]any{"testo": "x"})
	m.deve(200, "PUT", "/personaggi/"+pg+"/note-master", map[string]any{"testo": "## Agganci\n\n- l'antagonista tiene prigioniera la sorella."})
	if n := m.deve(200, "GET", "/personaggi/"+pg+"/note-master", nil); !strings.Contains(n["testo"].(string), "prigioniera") || n["nome"] != "Arin" {
		t.Fatalf("appunti: %v", n)
	}
	cx := m.deve(200, "GET", "/assistente/contesto?gruppo="+g, nil)
	if !strings.Contains(cx["contesto"].(string), "Appunti del Master (segreti): Agganci - l'antagonista tiene prigioniera") {
		t.Fatalf("appunti nel contesto: %s", cx["contesto"])
	}
	if p := cx["personaggi"].([]any); len(p) != 1 || p[0].(map[string]any)["nome"] != "Arin" {
		t.Fatalf("personaggi del contesto: %v", p)
	}
	bgM := m.deve(200, "GET", "/background-testi", nil)
	if b := bgM["lista"].([]any)[0].(map[string]any); !strings.Contains(fmt.Sprint(b["noteMaster"]), "prigioniera") {
		t.Fatalf("appunti nel background del Master: %v", b)
	}
	bgL := l.deve(200, "GET", "/background-testi", nil)
	if b, _ := json.Marshal(bgL); strings.Contains(string(b), "prigioniera") {
		t.Fatal("il giocatore non deve vedere gli appunti del Master")
	}
	// Personaggio senza background: gli appunti compaiono a parte.
	pg2 := l.deve(200, "POST", "/personaggi", map[string]any{"gruppo": g, "nome": "Mira"})["id"].(string)
	m.deve(200, "PUT", "/personaggi/"+pg2+"/note-master", map[string]any{"testo": "Ha paura dei lupi."})
	if ns := m.deve(200, "GET", "/background-testi", nil)["noteSenzaBackground"].([]any); len(ns) != 1 || ns[0].(map[string]any)["nome"] != "Mira" {
		t.Fatalf("appunti senza background: %v", ns)
	}
	m.deve(200, "PUT", "/personaggi/"+pg2+"/note-master", map[string]any{"testo": ""})
	if ns := m.deve(200, "GET", "/background-testi", nil)["noteSenzaBackground"].([]any); len(ns) != 0 {
		t.Fatalf("appunti cancellati: %v", ns)
	}

	// Ollama: collegamento nativo con il contesto lungo (da 16k; con la Campaign Bible, lunga, di più fino a 32k),
	// senza i «ragionamenti».
	m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "ollama", "indirizzo": ia.URL + "/v1", "modello": "qwen3:8b"})
	_, righe = m.chiediIA(map[string]any{"testo": "ciao"})
	nctx, _ := leggiFinta(&ultima)["options"].(map[string]any)["num_ctx"].(float64)
	if righe[0]["t"] != "Risposta " || righe[len(righe)-1]["fine"] != true || nctx < 16384 || nctx > 32768 || int(nctx)%4096 != 0 {
		t.Fatalf("Ollama nativo: %v %v", righe, leggiFinta(&ultima))
	}

	// Servizio spento: messaggio chiaro, nessun crash.
	ia.Close()
	m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "ollama", "indirizzo": ia.URL + "/v1", "modello": "qwen3:8b"})
	_, righe = m.chiediIA(map[string]any{"testo": "ciao"})
	if len(righe) != 1 || !strings.Contains(fmt.Sprint(righe[0]["errore"]), "Ollama non risponde") {
		t.Fatalf("servizio spento: %v", righe)
	}
}

func TestFiltroPensieri(t *testing.T) {
	f := &filtroPensieri{}
	out := ""
	for _, p := range []string{"Ciao <", "think>no", "n mostrare</th", "ink>\n", "risposta <b>", "</b> fine"} {
		out += f.passa(p)
	}
	out += f.fine()
	if out != "Ciao risposta <b></b> fine" {
		t.Fatalf("%q", out)
	}
}

// Agganci per l'antagonista scritti dall'IA nel Background: si generano da soli, rileggono i riassunti, il giocatore non li vede.
func TestAgganciIA(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	l := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	m.deve(200, "POST", "/utenti/"+a.idUtente(l)+"/approva", map[string]any{"gruppi": []string{g}})
	pg := l.deve(200, "POST", "/personaggi", map[string]any{"gruppo": g, "nome": "Arin"})["id"].(string)
	var ultima map[string]any
	var auth string
	ia := iaFinta(t, &ultima, &auth)
	defer ia.Close()
	m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "altro", "indirizzo": ia.URL + "/v1", "modello": "qwen3:8b"})
	l.deve(200, "PUT", "/personaggi/"+pg+"/background-testo", map[string]any{"testo": "Arin ha giurato di ritrovare il fratello Teodor."})
	scheda := func() map[string]any {
		return m.deve(200, "GET", "/background-testi", nil)["lista"].([]any)[0].(map[string]any)["agganciIA"].(map[string]any)
	}
	aspetta := func(cond func(map[string]any) bool, cosa string) map[string]any {
		t.Helper()
		for i := 0; i < 100; i++ {
			if ag := scheda(); cond(ag) {
				return ag
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("%s: %v", cosa, scheda())
		return nil
	}
	ag := aspetta(func(ag map[string]any) bool { return ag["aggiornati"] == true && ag["testo"] != nil }, "agganci generati da soli")
	if !strings.Contains(ag["testo"].(string), "Vasilka") || strings.Contains(ag["testo"].(string), "ragiono") || ag["fornitore"] != "altro · qwen3:8b" {
		t.Fatalf("agganci: %v", ag)
	}
	dom := leggiFinta(&ultima)["messages"].([]any)[1].(map[string]any)["content"].(string)
	if !strings.Contains(dom, "Teodor") || !strings.Contains(dom, "## Agganci per l'antagonista") || !strings.Contains(dom, "nessuno ancora") || strings.Contains(dom, "Luca") {
		t.Fatalf("richiesta all'IA: %s", dom)
	}
	// Un riassunto nuovo: l'IA rilegge e riscrive.
	l.deve(200, "POST", "/riassunti", map[string]any{"gruppo": g, "titolo": "La locanda dell'Acqua Blu", "testo": "Abbiamo conosciuto Urwin Martikov."})
	aspetta(func(ag map[string]any) bool { return ag["aggiornati"] == true }, "agganci aggiornati dopo il riassunto")
	dom = leggiFinta(&ultima)["messages"].([]any)[1].(map[string]any)["content"].(string)
	_ = dom
	// Dopo il riassunto nuovo l'IA apre per il Master una conversazione con le sue idee, segnata come nuova.
	var conv map[string]any
	for i := 0; i < 100 && conv == nil; i++ {
		for _, c := range m.deve(200, "GET", "/assistente/conversazioni", nil)["lista"].([]any) {
			if strings.Contains(c.(map[string]any)["titolo"].(string), "Dopo «La locanda dell'Acqua Blu»") {
				conv = c.(map[string]any)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if conv == nil || conv["nuova"] != true {
		t.Fatalf("proposta dopo il riassunto: %v", conv)
	}
	msgs := m.deve(200, "GET", "/assistente/conversazioni/"+conv["id"].(string), nil)["messaggi"].([]any)
	if len(msgs) != 2 || !strings.Contains(msgs[0].(map[string]any)["testo"].(string), "Urwin Martikov") || !strings.Contains(msgs[0].(map[string]any)["testo"].(string), "### Arin") {
		t.Fatalf("la proposta deve leggere il riassunto e gli agganci già scritti: %v", msgs)
	}
	for _, c := range m.deve(200, "GET", "/assistente/conversazioni", nil)["lista"].([]any) {
		if c.(map[string]any)["id"] == conv["id"] && c.(map[string]any)["nuova"] != false {
			t.Fatal("aperta: non è più nuova")
		}
	}
	// Memoria: la scheda si riscrive partendo da quella già scritta, e il riassunto nuovo è tra i dati.
	if dom, _, _ := a.s.datiAgganci(pg); !strings.Contains(dom, "AVEVI GIÀ SCRITTO") || !strings.Contains(dom, "Vasilka") || !strings.Contains(dom, "Urwin Martikov") {
		t.Fatalf("richiesta degli agganci senza memoria o senza riassunto: %s", dom)
	}

	// Il giocatore non vede la scheda del Master; «Rigenera» solo per il Master.
	if b, _ := json.Marshal(l.deve(200, "GET", "/background-testi", nil)); strings.Contains(string(b), "Vasilka") {
		t.Fatal("il giocatore non deve vedere gli agganci")
	}
	l.deve(403, "POST", "/personaggi/"+pg+"/agganci-ia", nil)
	muIAFinta.Lock()
	ultima = nil
	muIAFinta.Unlock()
	m.deve(200, "POST", "/personaggi/"+pg+"/agganci-ia", nil)
	for i := 0; i < 100 && leggiFinta(&ultima) == nil; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if leggiFinta(&ultima) == nil {
		t.Fatal("«Rigenera» non ha richiamato l'IA")
	}
	// IA spenta: restano gli ultimi agganci e si vede l'errore.
	ia.Close()
	m.deve(200, "PUT", "/personaggi/"+pg+"/background-testo", map[string]any{"testo": "Arin cerca il fratello Teodor, sparito nella nebbia."})
	ag = aspetta(func(ag map[string]any) bool { return ag["errore"] != nil && ag["inCorso"] == false }, "errore con l'IA spenta")
	if !strings.Contains(ag["testo"].(string), "Vasilka") || ag["aggiornati"] != false {
		t.Fatalf("con l'IA spenta restano gli ultimi agganci: %v", ag)
	}
}

// Gruppi separati (richiesta dell'utente): l'IA riceve solo i dati del gruppo su cui lavora, in chat, negli agganci e
// nelle proposte dopo i riassunti; una conversazione resta legata al suo gruppo.
func TestIAGruppiSeparati(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	gA := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Martedì"})["id"].(string)
	gB := m.deve(200, "POST", "/gruppi", map[string]any{"nome": "Venerdì"})["id"].(string)
	l := a.giocatore("Luca", "333 1234567", "192.168.1.20")
	sa := a.giocatore("Sara", "333 7654321", "192.168.1.21")
	m.deve(200, "POST", "/utenti/"+a.idUtente(l)+"/approva", map[string]any{"gruppi": []string{gA}})
	m.deve(200, "POST", "/utenti/"+a.idUtente(sa)+"/approva", map[string]any{"gruppi": []string{gB}})
	pgA := l.deve(200, "POST", "/personaggi", map[string]any{"gruppo": gA, "nome": "Arin"})["id"].(string)
	pgB := sa.deve(200, "POST", "/personaggi", map[string]any{"gruppo": gB, "nome": "Brunilde"})["id"].(string)
	m.deve(200, "PUT", "/personaggi/"+pgA+"/note-master", map[string]any{"testo": "SEGRETO-A: Arin è figlio di un Vallakovich."})
	m.deve(200, "PUT", "/personaggi/"+pgB+"/note-master", map[string]any{"testo": "SEGRETO-B: Brunilde ha venduto l'anima."})
	l.deve(200, "PUT", "/personaggi/"+pgA+"/background-testo", map[string]any{"testo": "STORIA-A: Arin viene da Daggerford."})
	sa.deve(200, "PUT", "/personaggi/"+pgB+"/background-testo", map[string]any{"testo": "STORIA-B: Brunilde viene da Neverwinter."})
	l.deve(200, "POST", "/riassunti", map[string]any{"gruppo": gA, "titolo": "Sessione A", "testo": "EVENTO-A: abbiamo ucciso il drago di Argentio."})
	sa.deve(200, "POST", "/riassunti", map[string]any{"gruppo": gB, "titolo": "Sessione B", "testo": "EVENTO-B: siamo fuggiti da Borgofosco."})

	soloA := func(cosa, testo string) {
		t.Helper()
		for _, x := range []string{"SEGRETO-B", "STORIA-B", "EVENTO-B", "Brunilde", "Venerdì"} {
			if strings.Contains(testo, x) {
				t.Fatalf("%s del gruppo A contiene %q del gruppo B:\n%s", cosa, x, testo)
			}
		}
	}
	ctxA := m.deve(200, "GET", "/assistente/contesto?gruppo="+gA, nil)["contesto"].(string)
	if !strings.Contains(ctxA, "SEGRETO-A") || !strings.Contains(ctxA, "EVENTO-A") || !strings.Contains(ctxA, "STORIA-A") {
		t.Fatalf("contesto A incompleto: %s", ctxA)
	}
	soloA("contesto", ctxA)
	domA, _, _ := a.s.datiAgganci(pgA)
	if !strings.Contains(domA, "EVENTO-A") || !strings.Contains(domA, "SOLO sul gruppo «Martedì»") {
		t.Fatalf("agganci A: %s", domA)
	}
	soloA("richiesta degli agganci", domA)

	// Con l'IA: la chat di una conversazione del gruppo A resta sul gruppo A anche se la pagina chiede il gruppo B.
	var ultima map[string]any
	var auth string
	ia := iaFinta(t, &ultima, &auth)
	defer ia.Close()
	m.deve(200, "PUT", "/assistente/impostazioni", map[string]any{"fornitore": "altro", "indirizzo": ia.URL + "/v1", "modello": "qwen3:8b"})
	_, righe := m.chiediIA(map[string]any{"testo": "Idee?", "gruppo": gA})
	conv := righe[len(righe)-1]["conversazione"].(string)
	m.chiediIA(map[string]any{"testo": "E ora?", "gruppo": gB, "conversazione": conv})
	sis := leggiFinta(&ultima)["messages"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(sis, "EVENTO-A") {
		t.Fatalf("la conversazione del gruppo A deve restare sul gruppo A: %s", sis)
	}
	soloA("chat", sis)
	// Proposta dopo un riassunto nuovo del gruppo B: nessun dato del gruppo A.
	sa.deve(200, "POST", "/riassunti", map[string]any{"gruppo": gB, "titolo": "Sessione B2", "testo": "EVENTO-B2: Elara è sparita."})
	var prop string
	for i := 0; i < 100 && prop == ""; i++ {
		for _, c := range m.deve(200, "GET", "/assistente/conversazioni", nil)["lista"].([]any) {
			if strings.Contains(c.(map[string]any)["titolo"].(string), "Sessione B2") {
				prop = c.(map[string]any)["id"].(string)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if prop == "" {
		t.Fatal("proposta per il gruppo B non creata")
	}
	pm := m.deve(200, "GET", "/assistente/conversazioni/"+prop, nil)
	if pm["gruppo"] != gB {
		t.Fatalf("la proposta deve essere del gruppo B: %v", pm["gruppo"])
	}
	testoP := pm["messaggi"].([]any)[0].(map[string]any)["testo"].(string)
	for _, x := range []string{"SEGRETO-A", "STORIA-A", "EVENTO-A", "Arin", "Martedì"} {
		if strings.Contains(testoP, x) {
			t.Fatalf("la proposta del gruppo B contiene %q del gruppo A: %s", x, testoP)
		}
	}
}
