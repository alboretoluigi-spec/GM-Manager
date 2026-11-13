package server

// Assistente IA del Master (richiesta dell'utente, 2026-11-09).
//
// Una chat solo per il Master che conosce la situazione di gioco del gruppo scelto: sessione, giorno di gioco, luogo
// del segnalino e ultimi viaggi, personaggi (classe, livello, PF, condizioni, tratti e background), combattimento in
// corso, ultimo riassunto. Il Master sceglie in Impostazioni dove gira l'IA:
//   - «ollama»: sul PC, con Ollama (gratis, nulla esce di casa);
//   - «gemini», «groq», «openrouter»: servizi online con piano gratuito (serve una chiave gratuita);
//   - «claude»: Claude di Anthropic (a pagamento), con la libreria ufficiale;
//   - «altro»: qualsiasi servizio con lo stesso formato di OpenAI (indirizzo e modello a mano).
//
// Privacy: all'IA arrivano solo dati di gioco. Mai nomi degli account, telefoni, password, indirizzi.
// Il Master vede con «Cosa sa l'IA» esattamente il testo che viene inviato.
// Le chiavi dei servizi online sono cifrate con la chiave di Gaster Bless e non tornano mai alle pagine.
// L'IA propone soltanto: niente cambia nelle schede, nella Mappa o nel tracker senza un clic del Master.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

// fornitoreIA: valori predefiniti di ogni servizio.
type fornitoreIA struct {
	Nome      string `json:"nome"`
	Indirizzo string `json:"indirizzo"`
	Modello   string `json:"modello"`
	Chiave    bool   `json:"chiave"` // serve una chiave
	Gratis    string `json:"gratis"` // nota per il Master
	Online    bool   `json:"online"` // i dati di gioco escono di casa
}

var fornitoriIA = map[string]fornitoreIA{
	"ollama": {Nome: "IA sul PC (Ollama)", Indirizzo: "http://127.0.0.1:11434/v1", Modello: "qwen3:8b",
		Gratis: "Gratis e privata: gira sulla scheda video del PC, nulla esce di casa.", Online: false},
	"gemini": {Nome: "Google Gemini", Indirizzo: "https://generativelanguage.googleapis.com/v1beta/openai", Modello: "gemini-2.5-flash", Chiave: true,
		Gratis: "Piano gratuito con limiti giornalieri; nel piano gratuito Google può usare i testi per migliorare i suoi modelli.", Online: true},
	"groq": {Nome: "Groq", Indirizzo: "https://api.groq.com/openai/v1", Modello: "llama-3.3-70b-versatile", Chiave: true,
		Gratis: "Piano gratuito con limiti al minuto e al giorno; molto veloce.", Online: true},
	"openrouter": {Nome: "OpenRouter", Indirizzo: "https://openrouter.ai/api/v1", Modello: "", Chiave: true,
		Gratis: "Molti modelli, alcuni gratuiti (nome che finisce con «:free»), con limiti giornalieri.", Online: true},
	"mistral": {Nome: "Mistral AI", Indirizzo: "https://api.mistral.ai/v1", Modello: "", Chiave: true,
		Gratis: "Piano gratuito con limiti; scegli il modello con «Elenca modelli». Controlla nelle condizioni del servizio se i testi del piano gratuito possono essere usati per migliorare i modelli.", Online: true},
	"cerebras": {Nome: "Cerebras", Indirizzo: "https://api.cerebras.ai/v1", Modello: "", Chiave: true,
		Gratis: "Piano gratuito con limiti, molto veloce; scegli il modello con «Elenca modelli». Controlla nelle condizioni del servizio come vengono trattati i testi.", Online: true},
	"nvidia": {Nome: "NVIDIA NIM", Indirizzo: "https://integrate.api.nvidia.com/v1", Modello: "", Chiave: true,
		Gratis: "Molti modelli gratuiti con limiti al minuto; per la chiave può servire verificare il numero di telefono. Scegli il modello con «Elenca modelli».", Online: true},
	"claude": {Nome: "Claude (Anthropic)", Indirizzo: "", Modello: "claude-opus-5-5", Chiave: true,
		Gratis: "A pagamento (si paga quello che si usa); la qualità più alta.", Online: true},
	"altro": {Nome: "Altro servizio compatibile con OpenAI", Indirizzo: "", Modello: "", Chiave: true,
		Gratis: "Indirizzo, modello e chiave a mano.", Online: true},
}

type configIA struct {
	Fornitore string `json:"fornitore"`
	Indirizzo string `json:"indirizzo"`
	Modello   string `json:"modello"`
	// Varia: risposte più varie e meno ripetitive (solo «Parla con l'antagonista», non salvato): con Ollama temperatura e
	// penalità per le ripetizioni, con i servizi compatibili solo la temperatura (accettata da tutti).
	Varia bool `json:"-"`
}

func (s *Server) rotteAssistente(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/assistente/impostazioni", api(s.apiIAImpostazioni))
	mux.Handle("PUT /api/v1/assistente/impostazioni", api(s.apiIASalvaImpostazioni))
	mux.Handle("GET /api/v1/assistente/modelli", api(s.apiIAModelli))
	mux.Handle("GET /api/v1/assistente/contesto", api(s.apiIAContesto))
	mux.Handle("GET /api/v1/assistente/conversazioni", api(s.apiIAConversazioni))
	mux.Handle("GET /api/v1/assistente/conversazioni/{id}", api(s.apiIAConversazione))
	mux.Handle("DELETE /api/v1/assistente/conversazioni/{id}", api(s.apiIAEliminaConversazione))
	mux.HandleFunc("POST /api/v1/assistente/chiedi", s.apiIAChiedi) // risposta a pezzi (NDJSON)
	// «Parla con l'antagonista»: system prompt dell'antagonista, modificabile dal Master (assistente_antagonista.go)
	mux.Handle("GET /api/v1/assistente/antagonista", api(s.apiIAAntagonista))
	mux.Handle("PUT /api/v1/assistente/antagonista", api(s.apiIAAntagonista))
	// Istruzioni dell'Assistente (Master Campaign Bible), modificabili dal Master (assistente_istruzioni.go)
	mux.Handle("GET /api/v1/assistente/istruzioni", api(s.apiTestoIA("ia_prompt_assistente", promptAssistentePredefinito)))
	mux.Handle("PUT /api/v1/assistente/istruzioni", api(s.apiTestoIA("ia_prompt_assistente", promptAssistentePredefinito)))
	// Istruzioni dell'Archivista del canone (identita_ia.go)
	mux.Handle("GET /api/v1/assistente/canone", api(s.apiTestoIA("ia_prompt_canone", promptCanonePredefinito)))
	mux.Handle("PUT /api/v1/assistente/canone", api(s.apiTestoIA("ia_prompt_canone", promptCanonePredefinito)))
	// Appunti del Master su un personaggio (fase IA-2): li vede solo il Master; l'IA li legge nel contesto.
	mux.Handle("GET /api/v1/personaggi/{id}/note-master", api(s.apiNoteMaster))
	mux.Handle("PUT /api/v1/personaggi/{id}/note-master", api(s.apiSalvaNoteMaster))
}

// ===== Impostazioni =====

func (s *Server) configIA() configIA {
	var c configIA
	v, _ := s.impostazione("ia_config")
	json.Unmarshal([]byte(v), &c)
	if _, ok := fornitoriIA[c.Fornitore]; !ok {
		c.Fornitore = ""
	}
	if c.Fornitore != "" {
		f := fornitoriIA[c.Fornitore]
		if c.Indirizzo == "" {
			c.Indirizzo = f.Indirizzo
		}
		if c.Modello == "" {
			c.Modello = f.Modello
		}
	}
	return c
}

func (s *Server) chiaveIA(fornitore string) string {
	v, _ := s.impostazione("ia_chiave_" + fornitore)
	if v == "" {
		return ""
	}
	b, err := hex.DecodeString(v)
	if err != nil {
		return ""
	}
	k, err := s.Chiavi.Decifra(b, "ia-chiave:"+fornitore)
	if err != nil {
		return ""
	}
	return string(k)
}

func (s *Server) apiIAImpostazioni(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	salvate := map[string]bool{}
	for k := range fornitoriIA {
		salvate[k] = s.chiaveIA(k) != ""
	}
	c := s.configIA()
	return map[string]any{"config": c, "fornitori": fornitoriIA, "chiaviSalvate": salvate,
		"scambio": s.scambioIA(), "ricercaApprofondita": s.ricercaApprofondita(c)}, nil
}

func (s *Server) apiIASalvaImpostazioni(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Fornitore string  `json:"fornitore"`
		Indirizzo string  `json:"indirizzo"`
		Modello   string  `json:"modello"`
		Chiave    *string `json:"chiave"` // nil: invariata; "": cancellata
		Scambio   *bool   `json:"scambio"`
		Ricerca   *bool   `json:"ricercaApprofondita"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	// le due scelte delle identità (identita_ia.go): si possono cambiare anche da sole
	siNo := func(b bool) string {
		if b {
			return "si"
		}
		return "no"
	}
	if in.Scambio != nil {
		if err := s.impostaImpostazione("ia_scambio", siNo(*in.Scambio)); err != nil {
			return nil, err
		}
	}
	if in.Ricerca != nil {
		if err := s.impostaImpostazione("ia_ricerca", siNo(*in.Ricerca)); err != nil {
			return nil, err
		}
	}
	if in.Fornitore == "" && in.Indirizzo == "" && in.Modello == "" && in.Chiave == nil && (in.Scambio != nil || in.Ricerca != nil) {
		return s.apiIAImpostazioni(w, r)
	}
	if in.Fornitore != "" {
		if _, ok := fornitoriIA[in.Fornitore]; !ok {
			return nil, errore(http.StatusBadRequest, "Servizio di IA sconosciuto.")
		}
	}
	ind, err := testo(in.Indirizzo, "Indirizzo", 0, 300)
	if err != nil {
		return nil, err
	}
	if ind != "" {
		p, e := url.Parse(ind)
		if e != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
			return nil, errore(http.StatusBadRequest, "Indirizzo non valido: deve iniziare con http:// o https://.")
		}
		ind = strings.TrimRight(ind, "/")
	}
	mod, err := testo(in.Modello, "Modello", 0, 120)
	if err != nil {
		return nil, err
	}
	if in.Fornitore != "" && ind == fornitoriIA[in.Fornitore].Indirizzo {
		ind = "" // resta il valore predefinito (si aggiorna da solo con le nuove versioni)
	}
	b, _ := json.Marshal(configIA{Fornitore: in.Fornitore, Indirizzo: ind, Modello: mod})
	if err := s.impostaImpostazione("ia_config", string(b)); err != nil {
		return nil, err
	}
	if in.Chiave != nil && in.Fornitore != "" {
		k := strings.TrimSpace(*in.Chiave)
		if len(k) > 400 || strings.ContainsAny(k, " \r\n\t") {
			return nil, errore(http.StatusBadRequest, "Chiave non valida: incollala senza spazi.")
		}
		v := ""
		if k != "" {
			v = hex.EncodeToString(s.Chiavi.Cifra([]byte(k), "ia-chiave:"+in.Fornitore))
		}
		if err := s.impostaImpostazione("ia_chiave_"+in.Fornitore, v); err != nil {
			return nil, err
		}
	}
	s.evento("ia_impostazioni", u.ID, "servizio: "+in.Fornitore)
	return s.apiIAImpostazioni(w, r)
}

// apiIAModelli elenca i modelli del servizio scelto (per Ollama: quelli già scaricati sul PC).
func (s *Server) apiIAModelli(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	c := s.configIA()
	if c.Fornitore == "" {
		return nil, errore(http.StatusConflict, "Scegli prima il servizio di IA.")
	}
	ctx, annulla := context.WithTimeout(r.Context(), 20*time.Second)
	defer annulla()
	chiave := s.chiaveIA(c.Fornitore)
	var nomi []string
	if c.Fornitore == "claude" {
		if chiave == "" {
			return nil, errore(http.StatusConflict, "Manca la chiave di Claude.")
		}
		cl := anthropic.NewClient(option.WithAPIKey(chiave))
		pag := cl.Models.ListAutoPaging(ctx, anthropic.ModelListParams{})
		for pag.Next() {
			nomi = append(nomi, pag.Current().ID)
		}
		if err := pag.Err(); err != nil {
			return nil, errore(http.StatusBadGateway, "Claude non risponde: "+erroreBreve(err))
		}
	} else {
		if c.Indirizzo == "" {
			return nil, errore(http.StatusConflict, "Manca l'indirizzo del servizio.")
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", c.Indirizzo+"/models", nil)
		if chiave != "" {
			req.Header.Set("Authorization", "Bearer "+chiave)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, errore(http.StatusBadGateway, messaggioNonRaggiungibile(c.Fornitore))
		}
		defer res.Body.Close()
		corpo, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		if res.StatusCode != 200 {
			return nil, errore(http.StatusBadGateway, "Il servizio risponde con un errore: "+erroreServizio(res.StatusCode, corpo))
		}
		var l struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		json.Unmarshal(corpo, &l)
		for _, m := range l.Data {
			nomi = append(nomi, strings.TrimPrefix(m.ID, "models/"))
		}
		if c.Fornitore == "openrouter" { // prima i gratuiti
			sort.SliceStable(nomi, func(i, j int) bool {
				return strings.HasSuffix(nomi[i], ":free") && !strings.HasSuffix(nomi[j], ":free")
			})
		}
	}
	if len(nomi) > 300 {
		nomi = nomi[:300]
	}
	return map[string]any{"modelli": nomi}, nil
}

func messaggioNonRaggiungibile(fornitore string) string {
	if fornitore == "ollama" {
		return "Ollama non risponde: controlla che sia installato e acceso sul PC (icona del lama vicino all'orologio)."
	}
	return "Il servizio non è raggiungibile: controlla la connessione a Internet e l'indirizzo."
}

// ===== Contesto di gioco =====

// gruppoAttuale: il gruppo della sessione in corso, altrimenti il primo gruppo attivo.
func (s *Server) gruppoPerIA(richiesto string) (id, nome string) {
	if richiesto != "" {
		if s.DB.QueryRow(`SELECT id, nome FROM gruppi WHERE id = ?`, richiesto).Scan(&id, &nome) == nil {
			return
		}
	}
	if s.DB.QueryRow(`SELECT g.id, g.nome FROM sessioni_gioco sg JOIN gruppi g ON g.id = sg.gruppo_id
		WHERE sg.stato = 'in_corso' ORDER BY sg.inizio_il DESC LIMIT 1`).Scan(&id, &nome) == nil {
		return
	}
	s.DB.QueryRow(`SELECT id, nome FROM gruppi WHERE archiviato = 0 ORDER BY creato_il LIMIT 1`).Scan(&id, &nome)
	return
}

// contestoGioco: il testo che l'IA riceve sulla situazione del gruppo. Solo dati di gioco.
func (s *Server) contestoGioco(gruppo, nomeGruppo string) string {
	if gruppo == "" {
		return "Nessun gruppo di gioco ancora creato."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Gruppo: %s.\n", nomeGruppo)
	var giorno int
	if s.DB.QueryRow(`SELECT giorno FROM tempo_gruppo WHERE gruppo_id = ?`, gruppo).Scan(&giorno) == nil && giorno > 0 {
		fmt.Fprintf(&b, "Giorno di gioco a Gaster Bless: %d.\n", giorno)
	}
	// Sessione: in corso, altrimenti l'ultima.
	var sesID, titolo, stato, luogo, note string
	var inizio int64
	if s.DB.QueryRow(`SELECT id, titolo, stato, luogo, note, inizio_il FROM sessioni_gioco WHERE gruppo_id = ?
		ORDER BY CASE stato WHEN 'in_corso' THEN 0 ELSE 1 END, inizio_il DESC LIMIT 1`, gruppo).
		Scan(&sesID, &titolo, &stato, &luogo, &note, &inizio) == nil {
		st := map[string]string{"in_corso": "in corso", "pianificata": "in programma", "conclusa": "conclusa"}[stato]
		fmt.Fprintf(&b, "Sessione: «%s» (%s, %s).\n", titolo, st, time.Unix(inizio, 0).Format("02/01/2006"))
		if strings.TrimSpace(note) != "" {
			fmt.Fprintf(&b, "Appunti del Master sulla sessione: %s\n", accorcia(note, 800))
		}
	}
	// Luogo: località più vicina al segnalino del gruppo e ultimi viaggi.
	var pos string
	s.DB.QueryRow(`SELECT posizione FROM stato_mappa WHERE gruppo_id = ?`, gruppo).Scan(&pos)
	if d := leggiSegnalini(pos); len(d.Segnalini) > 0 {
		for i, sg := range d.Segnalini {
			if l := s.localitaVicina(sg.X, sg.Y); l != "" {
				if i == 0 {
					fmt.Fprintf(&b, "Il gruppo si trova presso: %s.\n", l)
				} else {
					fmt.Fprintf(&b, "Una parte del gruppo (diviso) si trova presso: %s.\n", l)
				}
			}
		}
	}
	if rows, err := s.DB.Query(`SELECT arrivo, miglia, ore, giorno FROM viaggi WHERE gruppo_id = ? ORDER BY id DESC LIMIT 5`, gruppo); err == nil {
		var righe []string
		for rows.Next() {
			var arrivo string
			var miglia, ore float64
			var g int
			rows.Scan(&arrivo, &miglia, &ore, &g)
			dove := "un punto senza nome"
			if l := s.cercaLocalita(arrivo); l != nil {
				dove = l.N
			}
			righe = append(righe, fmt.Sprintf("giorno %d: %.0f miglia (%.1f ore) fino a %s", g, miglia, ore, dove))
		}
		rows.Close()
		if len(righe) > 0 {
			b.WriteString("Ultimi viaggi (dal più recente): " + strings.Join(righe, "; ") + ".\n")
		}
	}
	// Personaggi del gruppo.
	type pg struct {
		id, nome, dati, riep, bgTesto string
		pf                            *int
		cond                          string
		ispir                         int
	}
	var pgs []pg
	if rows, err := s.DB.Query(`SELECT p.id, p.nome, COALESCE(v.dati,'{}'), COALESCE(v.riepilogo,'{}'), sg.pf, COALESCE(sg.condizioni,'[]'),
		COALESCE(sg.ispirazione,0), COALESCE(bt.testo,'')
		FROM personaggi p LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		LEFT JOIN stato_in_gioco sg ON sg.personaggio_id = p.id LEFT JOIN background_testi bt ON bt.personaggio_id = p.id
		WHERE p.gruppo_id = ? AND p.stato = 'attivo' AND p.proprietario_id IS NOT NULL ORDER BY p.nome`, gruppo); err == nil {
		for rows.Next() {
			var x pg
			rows.Scan(&x.id, &x.nome, &x.dati, &x.riep, &x.pf, &x.cond, &x.ispir, &x.bgTesto)
			pgs = append(pgs, x)
		}
		rows.Close()
	}
	if len(pgs) > 0 {
		b.WriteString("\nPersonaggi dei giocatori:\n")
	}
	for _, x := range pgs {
		var rp riepilogo
		json.Unmarshal([]byte(x.riep), &rp)
		var d struct {
			Bg string         `json:"bg"`
			Al string         `json:"al"`
			F  map[string]any `json:"f"`
		}
		json.Unmarshal([]byte(x.dati), &d)
		parti := []string{}
		if rp.Razza != "" {
			parti = append(parti, rp.Razza)
		}
		cl := strings.TrimSpace(rp.Classe + " " + rp.Sottocl)
		if cl != "" {
			parti = append(parti, cl)
		}
		if rp.Livello > 0 {
			parti = append(parti, fmt.Sprintf("livello %d", rp.Livello))
		}
		if d.Bg != "" {
			parti = append(parti, "background: "+d.Bg)
		}
		if d.Al != "" {
			parti = append(parti, d.Al)
		}
		if rp.CA > 0 {
			parti = append(parti, fmt.Sprintf("CA %d", rp.CA))
		}
		if x.pf != nil && rp.PFMax > 0 {
			parti = append(parti, fmt.Sprintf("PF %d/%d", *x.pf, rp.PFMax))
		} else if rp.PFMax > 0 {
			parti = append(parti, fmt.Sprintf("PF massimi %d", rp.PFMax))
		}
		var cond []any
		json.Unmarshal([]byte(x.cond), &cond)
		if len(cond) > 0 {
			nomi := []string{}
			for _, c := range cond {
				switch v := c.(type) {
				case string:
					nomi = append(nomi, v)
				case map[string]any:
					if n, ok := v["n"].(string); ok {
						nomi = append(nomi, n)
					} else if n, ok := v["nome"].(string); ok {
						nomi = append(nomi, n)
					}
				}
			}
			if len(nomi) > 0 {
				parti = append(parti, "condizioni: "+strings.Join(nomi, ", "))
			}
		}
		if x.ispir > 0 {
			parti = append(parti, "ha l'ispirazione")
		}
		fmt.Fprintf(&b, "- %s: %s.\n", x.nome, strings.Join(parti, ", "))
		for _, k := range []struct{ chiave, nome string }{{"tratti", "Tratti"}, {"ideali", "Ideali"}, {"legami", "Legami"}, {"difetti", "Difetti"}} {
			if v, ok := d.F[k.chiave].(string); ok && strings.TrimSpace(v) != "" {
				fmt.Fprintf(&b, "  %s: %s\n", k.nome, accorcia(strings.ReplaceAll(v, "\n", " "), 300))
			}
		}
		var appunti string
		if s.DB.QueryRow(`SELECT testo FROM note_master_pg WHERE personaggio_id = ?`, x.id).Scan(&appunti) == nil && strings.TrimSpace(appunti) != "" {
			fmt.Fprintf(&b, "  Appunti del Master (segreti): %s\n", accorcia(strings.Join(strings.Fields(strings.NewReplacer("#", "", "**", "").Replace(appunti)), " "), 1200))
		}
		storia := x.bgTesto
		if storia == "" {
			if v, ok := d.F["storia"].(string); ok {
				storia = v
			}
		}
		if strings.TrimSpace(storia) != "" {
			fmt.Fprintf(&b, "  Storia: %s\n", accorcia(strings.Join(strings.Fields(strings.NewReplacer("#", "", "**", "").Replace(storia)), " "), 1500))
		}
	}
	// Combattimento in corso.
	if sesID != "" {
		var cid string
		var round int
		var turnoDi sqlNullString
		if s.DB.QueryRow(`SELECT id, round, turno_di FROM combattimenti WHERE sessione_id = ? AND attivo = 1 ORDER BY creato_il DESC LIMIT 1`, sesID).
			Scan(&cid, &round, &turnoDi) == nil {
			fmt.Fprintf(&b, "\nCombattimento in corso (round %d). In ordine di iniziativa:\n", round)
			if rows, err := s.DB.Query(`SELECT id, tipo, nome, COALESCE(iniziativa,0), nascosto, pf, pf_max, condizioni FROM combattenti
				WHERE combattimento_id = ? ORDER BY iniziativa DESC, spareggio DESC`, cid); err == nil {
				for rows.Next() {
					var id, tipo, nome, cond string
					var ini, nasc, pf, pfMax int
					rows.Scan(&id, &tipo, &nome, &ini, &nasc, &pf, &pfMax, &cond)
					riga := fmt.Sprintf("- %s (iniziativa %d", nome, ini)
					if tipo == "mostro" {
						riga += fmt.Sprintf(", mostro, PF %d/%d", pf, pfMax)
						if nasc == 1 {
							riga += ", nascosto ai giocatori"
						}
						if cond != "[]" && cond != "" {
							riga += ", condizioni " + cond
						}
					}
					riga += ")"
					if turnoDi.Valid && turnoDi.String == id {
						riga += " ← è il suo turno"
					}
					b.WriteString(riga + "\n")
				}
				rows.Close()
			}
		}
	}
	// Ultimi riassunti del gruppo (dal più vecchio al più recente).
	if rows, err := s.DB.Query(`SELECT titolo, testo FROM riassunti WHERE gruppo_id = ? ORDER BY creato_il DESC LIMIT 3`, gruppo); err == nil {
		var rias []string
		for rows.Next() {
			var rt, rtesto string
			rows.Scan(&rt, &rtesto)
			rias = append([]string{fmt.Sprintf("- «%s»: %s", rt, accorcia(strings.Join(strings.Fields(strings.NewReplacer("#", "", "**", "").Replace(rtesto)), " "), 1500))}, rias...)
		}
		rows.Close()
		if len(rias) > 0 {
			b.WriteString("\nUltimi riassunti scritti dai giocatori (dal più vecchio al più recente):\n" + strings.Join(rias, "\n") + "\n")
		}
	}
	return b.String()
}

type sqlNullString struct {
	String string
	Valid  bool
}

func (n *sqlNullString) Scan(v any) error {
	switch x := v.(type) {
	case nil:
		n.Valid = false
	case string:
		n.String, n.Valid = x, true
	case []byte:
		n.String, n.Valid = string(x), true
	}
	return nil
}

// localitaVicina: nome della località più vicina al punto (se è abbastanza vicina).
func (s *Server) localitaVicina(x, y float64) string {
	best, dist := "", math.MaxFloat64
	for _, l := range s.localita() {
		if d := math.Hypot(l.X-x, l.Y-y); d < dist {
			best, dist = l.N, d
		}
	}
	if dist > 120 {
		if best == "" {
			return ""
		}
		return "in viaggio, la località più vicina è " + best
	}
	return best
}

// Le istruzioni dell'Assistente sono in assistente_istruzioni.go (testo predefinito: prompt_assistente.md).

// sistemaPerGruppo: istruzioni + regola dei gruppi separati + situazione del solo gruppo scelto.
// Il Master conduce la stessa campagna con più gruppi, ognuno con le sue sessioni: ogni richiesta all'IA contiene i dati
// di un gruppo solo (tutte le letture di contestoGioco sono filtrate per gruppo) e l'IA non ricorda nulla tra una
// richiesta e l'altra, quindi le informazioni non possono passare da un gruppo all'altro.
func (s *Server) sistemaPerGruppo(gid, nome string) string {
	return strings.TrimSpace(s.istruzioniAssistente()) + "\n\n---\n" + regolaGruppo(nome) +
		"\n\nSITUAZIONE ATTUALE DEL GRUPPO (dati di Gaster Bless, aggiornati a ora):\n" + accorciaContesto(s.contestoGioco(gid, nome), maxContestoGruppo)
}

func regolaGruppo(nome string) string {
	return "GRUPPI SEPARATI: il Master conduce questa campagna con più gruppi di giocatori, ognuno con le sue sessioni e la sua storia. " +
		"Qui lavori SOLO sul gruppo «" + nome + "»: usa solo le informazioni di questo gruppo, non inventare collegamenti con altri gruppi " +
		"e non dare per scontato che qui sia successo quello che può essere successo altrove."
}

// ===== Conversazioni =====

type messaggioIA struct {
	Ruolo     string     `json:"ruolo"` // master, ia
	Testo     string     `json:"testo"`
	Fornitore string     `json:"fornitore,omitempty"`
	Il        int64      `json:"il"`
	Fonti     []passoBib `json:"fonti,omitempty"` // brani della biblioteca dati all'IA per questa risposta ([C…], [H…])
}

func (s *Server) apiIAConversazioni(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`SELECT c.id, c.titolo, COALESCE(c.gruppo_id,''), COALESCE(g.nome,''), c.modificato_il, c.nuova, c.modo FROM ia_conversazioni c
		LEFT JOIN gruppi g ON g.id = c.gruppo_id ORDER BY c.modificato_il DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, titolo, gid, gnome, modo string
		var mod int64
		var nuova int
		rows.Scan(&id, &titolo, &gid, &gnome, &mod, &nuova, &modo)
		out = append(out, map[string]any{"id": id, "titolo": titolo, "gruppo": gid, "nomeGruppo": gnome, "modificato": mod, "nuova": nuova == 1, "modo": modo})
	}
	return out, nil
}

func (s *Server) apiIAConversazione(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var titolo, gid, msg, modo string
	if err := s.DB.QueryRow(`SELECT titolo, COALESCE(gruppo_id,''), messaggi, modo FROM ia_conversazioni WHERE id = ?`, r.PathValue("id")).Scan(&titolo, &gid, &msg, &modo); err != nil {
		return nil, errNonTrovato
	}
	var m []messaggioIA
	json.Unmarshal([]byte(msg), &m)
	s.DB.Exec(`UPDATE ia_conversazioni SET nuova = 0 WHERE id = ?`, r.PathValue("id")) // il Master l'ha aperta
	return map[string]any{"id": r.PathValue("id"), "titolo": titolo, "gruppo": gid, "messaggi": m, "modo": modo}, nil
}

func (s *Server) apiIAEliminaConversazione(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`DELETE FROM ia_conversazioni WHERE id = ?`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errNonTrovato
	}
	return nil, nil
}

func (s *Server) apiIAContesto(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	gid, gnome := s.gruppoPerIA(r.URL.Query().Get("gruppo"))
	c := s.configIA()
	f := fornitoriIA[c.Fornitore]
	// Per i pulsanti «salva in Gaster Bless»: sessione del gruppo (per il riassunto) e personaggi (per gli appunti).
	var sesID, sesTitolo string
	s.DB.QueryRow(`SELECT id, titolo FROM sessioni_gioco WHERE gruppo_id = ? ORDER BY CASE stato WHEN 'in_corso' THEN 0 ELSE 1 END, inizio_il DESC LIMIT 1`, gid).Scan(&sesID, &sesTitolo)
	pgs := []map[string]string{}
	if rows, err := s.DB.Query(`SELECT id, nome FROM personaggi WHERE gruppo_id = ? AND stato = 'attivo' ORDER BY proprietario_id IS NULL, nome`, gid); err == nil {
		for rows.Next() {
			var id, nome string
			rows.Scan(&id, &nome)
			pgs = append(pgs, map[string]string{"id": id, "nome": nome})
		}
		rows.Close()
	}
	pAntagonista, _ := s.promptAntagonista()
	return map[string]any{"gruppo": gid, "nomeGruppo": gnome, "contesto": s.contestoGioco(gid, gnome), "istruzioni": s.istruzioniAssistente(), "istruzioniAntagonista": pAntagonista,
		"codice": s.contestoLore(gid, "", maxLoreIA), // le schede del Codice che partirebbero senza nomi nella domanda (lore_ia.go)
		// Archivista del canone (identita_ia.go): istruzioni, schede del canone e testi della biblioteca
		"istruzioniCanone": s.istruzioniCanone(),
		"codiceCanone":     s.contestoLoreDi("", "", maxLoreIA, "canone", false, "SCHEDE DEL CANONE NEL CODICE:"),
		"biblioteca":       s.fontiBib(), "scambio": s.scambioIA(),
		"fornitore": c.Fornitore, "nomeFornitore": f.Nome, "modello": c.Modello, "online": f.Online,
		"sessione": map[string]string{"id": sesID, "titolo": sesTitolo}, "personaggi": pgs}, nil
}

// ===== Appunti del Master sui personaggi =====

const maxNoteMaster = 30000

func (s *Server) apiNoteMaster(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var nome, testoNote string
	var mod int64
	if s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ? AND stato != 'archiviato'`, r.PathValue("id")).Scan(&nome) != nil {
		return nil, errNonTrovato
	}
	s.DB.QueryRow(`SELECT testo, modificato_il FROM note_master_pg WHERE personaggio_id = ?`, r.PathValue("id")).Scan(&testoNote, &mod)
	return map[string]any{"personaggio": r.PathValue("id"), "nome": nome, "testo": testoNote, "modificato": mod}, nil
}

func (s *Server) apiSalvaNoteMaster(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var n int
	if s.DB.QueryRow(`SELECT COUNT(*) FROM personaggi WHERE id = ? AND stato != 'archiviato'`, id).Scan(&n); n == 0 {
		return nil, errNonTrovato
	}
	var in struct {
		Testo string `json:"testo"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	t := strings.ReplaceAll(strings.ReplaceAll(in.Testo, "\r\n", "\n"), "\r", "\n")
	if t, err = testo(t, "Appunti", 0, maxNoteMaster); err != nil {
		return nil, err
	}
	if t == "" {
		_, err = s.DB.Exec(`DELETE FROM note_master_pg WHERE personaggio_id = ?`, id)
	} else {
		_, err = s.DB.Exec(`INSERT INTO note_master_pg(personaggio_id, testo, modificato_il) VALUES(?,?,?)
			ON CONFLICT(personaggio_id) DO UPDATE SET testo = excluded.testo, modificato_il = excluded.modificato_il`, id, t, db.Ora())
	}
	if err != nil {
		return nil, err
	}
	s.Hub.AlMaster(Msg{Tipo: "background.aggiornato", Dati: map[string]string{"personaggio": id}})
	s.evento("note_master", u.ID, id)
	return nil, nil
}

// ===== Domanda all'IA (risposta a pezzi, una riga JSON per pezzo) =====

const maxStoriaIA = 24000 // caratteri della conversazione mandati all'IA (i messaggi più vecchi restano salvati)

func (s *Server) apiIAChiedi(w http.ResponseWriter, r *http.Request) {
	u, err := richiedi(r, "master")
	if err != nil {
		api(func(http.ResponseWriter, *http.Request) (any, error) { return nil, err }).ServeHTTP(w, r)
		return
	}
	var in struct {
		Conversazione string `json:"conversazione"`
		Gruppo        string `json:"gruppo"`
		Testo         string `json:"testo"`
		Titolo        string `json:"titolo"`
		Modo          string `json:"modo"` // assistente (predefinito) o antagonista (assistente_antagonista.go)
	}
	falli := func(e error) {
		api(func(http.ResponseWriter, *http.Request) (any, error) { return nil, e }).ServeHTTP(w, r)
	}
	if err := leggiJSONMax(r, &in, 256<<10); err != nil {
		falli(err)
		return
	}
	domanda := strings.TrimSpace(in.Testo)
	if domanda == "" || utf8.RuneCountInString(domanda) > 40000 {
		falli(errore(http.StatusBadRequest, "Scrivi una domanda (al massimo 40.000 caratteri)."))
		return
	}
	c := s.configIA()
	if c.Fornitore == "" {
		falli(errore(http.StatusConflict, "Scegli prima dove gira l'IA: «⚙ Impostazioni IA» nella sezione Assistente."))
		return
	}
	chiave := s.chiaveIA(c.Fornitore)
	if fornitoriIA[c.Fornitore].Chiave && chiave == "" && c.Fornitore != "altro" {
		falli(errore(http.StatusConflict, "Manca la chiave del servizio: «⚙ Impostazioni IA» nella sezione Assistente."))
		return
	}
	if c.Modello == "" {
		falli(errore(http.StatusConflict, "Scegli il modello dell'IA: «⚙ Impostazioni IA» nella sezione Assistente."))
		return
	}
	// Conversazione: nuova o esistente.
	var storia []messaggioIA
	gid := in.Gruppo
	modo := modoValido(in.Modo)
	if in.Conversazione != "" {
		var msg, g string
		if s.DB.QueryRow(`SELECT messaggi, COALESCE(gruppo_id,''), modo FROM ia_conversazioni WHERE id = ?`, in.Conversazione).Scan(&msg, &g, &modo) != nil {
			falli(errNonTrovato)
			return
		}
		json.Unmarshal([]byte(msg), &storia)
		if g != "" {
			// Gruppi separati: una conversazione resta legata al suo gruppo, anche se la pagina ne chiede un altro,
			// così la sua storia non si mescola con la situazione di un altro gruppo.
			gid = g
		}
	}
	gid, gnome := s.gruppoPerIA(gid)
	var sistema string
	switch modo {
	case "antagonista": // «Parla con l'antagonista»: il system prompt dell'antagonista al posto delle istruzioni dell'Assistente
		sistema = s.sistemaAntagonista(gid, gnome) + "\n\n---\n" + regoleAntagonistaFonti
		c.Varia = true
	case "canone": // Archivista del canone: solo testi ufficiali, nessun gruppo (identita_ia.go)
		gid, gnome = "", ""
		sistema = s.sistemaCanone()
	default: // Cronista della campagna
		sistema = s.sistemaPerGruppo(gid, gnome) + "\n\n---\n" + regoleCronista
	}
	precedente := ""
	for i := len(storia) - 1; i >= 0; i-- {
		if storia[i].Ruolo == "master" {
			precedente = storia[i].Testo
			break
		}
	}
	// Brani della biblioteca e consulto dell'altra identità (identita_ia.go), poi il Codice della campagna (lore_ia.go)
	parole := s.paroleChiaveIA(r.Context(), c, chiave, domanda, precedente)
	conoscenza, fonti := s.conoscenzaIA(modo, gid, domanda, precedente, parole, c.Fornitore == "ollama")
	sistema += conoscenza
	if modo != "canone" {
		ricerca := domanda
		if precedente != "" {
			ricerca += "\n" + precedente
		}
		maxL := maxLoreIA
		if c.Fornitore == "ollama" {
			maxL = maxLoreOllama
		}
		if lore := s.contestoLore(gid, ricerca, maxL); lore != "" {
			sistema += "\n\n---\n" + lore
		}
	}
	storia = append(storia, messaggioIA{Ruolo: "master", Testo: domanda, Il: db.Ora()})
	// Ultimi messaggi entro il limite.
	inviati, tot := []messaggioIA{}, 0
	limite := limiteStoria(c, sistema, maxStoriaIA) // con Ollama, istruzioni lunghe = storia più corta (assistente_istruzioni.go)
	for i := len(storia) - 1; i >= 0; i-- {
		tot += len(storia[i].Testo)
		if tot > limite && len(inviati) > 0 {
			break
		}
		inviati = append([]messaggioIA{storia[i]}, inviati...)
	}
	if inviati[0].Ruolo == "ia" {
		inviati = inviati[1:]
	}

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	fl, _ := w.(http.Flusher)
	scrivi := func(v any) {
		b, _ := json.Marshal(v)
		w.Write(append(b, '\n'))
		if fl != nil {
			fl.Flush()
		}
	}
	ctx, annulla := context.WithTimeout(r.Context(), 5*time.Minute)
	defer annulla()
	var risposta strings.Builder
	pezzo := func(t string) {
		if t == "" {
			return
		}
		risposta.WriteString(t)
		scrivi(map[string]string{"t": t})
	}
	if c.Fornitore == "claude" {
		err = chiediClaude(ctx, chiave, c.Modello, sistema, inviati, pezzo)
	} else {
		err = chiediCompatibile(ctx, c, chiave, sistema, inviati, pezzo)
	}
	if err != nil && risposta.Len() == 0 {
		msg := err.Error()
		var e *ErrAPI
		if !errors.As(err, &e) {
			msg = "L'IA non ha risposto: " + erroreBreve(err)
		}
		scrivi(map[string]string{"errore": msg})
		return
	}
	testoRisposta := strings.TrimSpace(risposta.String())
	if err != nil {
		testoRisposta += "\n\n(Risposta interrotta.)"
	}
	storia = append(storia, messaggioIA{Ruolo: "ia", Testo: testoRisposta, Fornitore: c.Fornitore + " · " + c.Modello, Il: db.Ora(), Fonti: fonti})
	mb, _ := json.Marshal(storia)
	ora := db.Ora()
	id := in.Conversazione
	if id == "" {
		id = cripto.ID()
		titolo := strings.TrimSpace(in.Titolo)
		if titolo == "" {
			titolo = accorcia(strings.Join(strings.Fields(domanda), " "), 60)
		}
		_, err = s.DB.Exec(`INSERT INTO ia_conversazioni(id, gruppo_id, titolo, messaggi, creato_il, modificato_il, modo) VALUES(?,?,?,?,?,?,?)`,
			id, nullo(gid), accorcia(titolo, 80), string(mb), ora, ora, modo)
	} else {
		_, err = s.DB.Exec(`UPDATE ia_conversazioni SET messaggi = ?, gruppo_id = ?, modificato_il = ? WHERE id = ?`, string(mb), nullo(gid), ora, id)
	}
	if err != nil {
		scrivi(map[string]string{"errore": "Risposta ricevuta ma non salvata: " + erroreBreve(err)})
		return
	}
	s.evento("ia_domanda", u.ID, c.Fornitore)
	scrivi(map[string]any{"fine": true, "conversazione": id, "fornitore": c.Fornitore + " · " + c.Modello, "fonti": fonti})
}

// chiediClaude usa la libreria ufficiale di Anthropic (risposta a pezzi). Con i modelli che lo prevedono, se un
// controllo di sicurezza rifiuta la richiesta, un altro modello Claude risponde al suo posto («fallbacks: default»).
func chiediClaude(ctx context.Context, chiave, modello, sistema string, storia []messaggioIA, pezzo func(string)) error {
	cl := anthropic.NewClient(option.WithAPIKey(chiave))
	msgs := make([]anthropic.BetaMessageParam, 0, len(storia))
	for _, m := range storia {
		if m.Ruolo == "ia" {
			msgs = append(msgs, anthropic.BetaMessageParam{OfAssistant: &anthropic.BetaAssistantMessageParam{Content: []anthropic.BetaContentBlockParamUnion{anthropic.NewBetaTextBlock(m.Testo)}}})
		} else {
			msgs = append(msgs, anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(m.Testo)))
		}
	}
	p := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(modello),
		MaxTokens: 16000,
		System:    []anthropic.BetaTextBlockParam{{Text: sistema}},
		Messages:  msgs,
	}
	if conFallbackClaude(modello) {
		p.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
		p.Fallbacks = anthropic.BetaFallbacksParamOfDefault()
	}
	st := cl.Beta.Messages.NewStreaming(ctx, p)
	rifiuto := false
	for st.Next() {
		ev := st.Current()
		switch v := ev.AsAny().(type) {
		case anthropic.BetaRawContentBlockDeltaEvent:
			if d, ok := v.Delta.AsAny().(anthropic.BetaTextDelta); ok {
				pezzo(d.Text)
			}
		case anthropic.BetaRawMessageDeltaEvent:
			if v.Delta.StopReason == anthropic.BetaStopReasonRefusal {
				rifiuto = true
			}
		}
	}
	if err := st.Err(); err != nil {
		var ae *anthropic.Error
		if errors.As(err, &ae) {
			switch ae.StatusCode {
			case 401, 403:
				return errore(http.StatusBadGateway, "Claude rifiuta la chiave: controllala in «⚙ Impostazioni IA» nella sezione Assistente.")
			case 404:
				return errore(http.StatusBadGateway, "Claude non conosce il modello «"+modello+"»: scegline un altro dall'elenco.")
			case 429:
				return errore(http.StatusBadGateway, "Troppe richieste a Claude o credito esaurito: riprova tra poco.")
			}
		}
		return err
	}
	if rifiuto {
		pezzo("\n\n(Claude ha rifiutato di rispondere a questa richiesta: prova a riformularla.)")
	}
	return nil
}

// conFallbackClaude: i modelli che accettano «fallbacks: default».
func conFallbackClaude(m string) bool {
	switch m {
	case "claude-opus-5-5", "claude-opus-5", "claude-fable-5-1", "claude-sonnet-5-5":
		return true
	}
	return false
}

// contestoOllama: quanto testo Ollama legge per ogni richiesta (in token). Il valore di base di Ollama è piccolo e
// taglierebbe in silenzio storia, scheda e riassunti; 16k sta su una scheda video da 8 GB con un modello da 8 miliardi
// di parametri (se non ci sta, Ollama usa anche il processore: più lento ma funziona).
const contestoOllama = 16384

// chiediCompatibile: servizi con il formato di OpenAI (Gemini, Groq, OpenRouter...), risposta a pezzi. Per Ollama usa il
// suo collegamento nativo, che permette di scegliere quanto testo leggere (contestoOllama).
func chiediCompatibile(ctx context.Context, c configIA, chiave, sistema string, storia []messaggioIA, pezzo func(string)) error {
	if c.Indirizzo == "" {
		return errore(http.StatusConflict, "Manca l'indirizzo del servizio: «⚙ Impostazioni IA» nella sezione Assistente.")
	}
	if c.Fornitore == "ollama" {
		return chiediOllama(ctx, c, sistema, storia, pezzo)
	}
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := []msg{{"system", sistema}}
	for _, m := range storia {
		ruolo := "user"
		if m.Ruolo == "ia" {
			ruolo = "assistant"
		}
		msgs = append(msgs, msg{ruolo, m.Testo})
	}
	richiesta := map[string]any{"model": c.Modello, "messages": msgs, "stream": true}
	if c.Varia {
		richiesta["temperature"] = 0.9
	}
	corpo, _ := json.Marshal(richiesta)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.Indirizzo+"/chat/completions", bytes.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	if chiave != "" {
		req.Header.Set("Authorization", "Bearer "+chiave)
	}
	if c.Fornitore == "openrouter" {
		req.Header.Set("X-Title", "Gaster Bless")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errore(http.StatusBadGateway, messaggioNonRaggiungibile(c.Fornitore))
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		switch {
		case res.StatusCode == 401 || res.StatusCode == 403:
			return errore(http.StatusBadGateway, "Il servizio rifiuta la chiave: controllala in «⚙ Impostazioni IA» nella sezione Assistente.")
		case res.StatusCode == 404 && c.Fornitore == "ollama":
			return errore(http.StatusBadGateway, "Ollama non ha il modello «"+c.Modello+"»: scaricalo (comando nella guida) o scegline un altro.")
		case res.StatusCode == 429:
			return errore(http.StatusBadGateway, "Limite gratuito raggiunto per ora: riprova più tardi o scegli un altro servizio.")
		}
		return errore(http.StatusBadGateway, "Il servizio risponde con un errore: "+erroreServizio(res.StatusCode, b))
	}
	filtro := &filtroPensieri{}
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		riga := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(riga, "data:") {
			continue
		}
		dati := strings.TrimSpace(strings.TrimPrefix(riga, "data:"))
		if dati == "[DONE]" {
			break
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(dati), &ev) != nil {
			continue
		}
		if ev.Error != nil {
			return errore(http.StatusBadGateway, "Il servizio si è fermato: "+accorcia(ev.Error.Message, 200))
		}
		for _, ch := range ev.Choices {
			pezzo(filtro.passa(ch.Delta.Content))
		}
	}
	pezzo(filtro.fine())
	return sc.Err()
}

// chiediOllama: POST /api/chat di Ollama (righe JSON), con la lunghezza del contesto impostata.
func chiediOllama(ctx context.Context, c configIA, sistema string, storia []messaggioIA, pezzo func(string)) error {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	storia = adattaOllama(sistema, storia) // le istruzioni restano intere; si accorcia il resto (assistente_istruzioni.go)
	msgs := []msg{{"system", sistema}}
	for _, m := range storia {
		ruolo := "user"
		if m.Ruolo == "ia" {
			ruolo = "assistant"
		}
		msgs = append(msgs, msg{ruolo, m.Testo})
	}
	base := strings.TrimSuffix(strings.TrimRight(c.Indirizzo, "/"), "/v1")
	opzioni := map[string]any{"num_ctx": contestoPerOllama(sistema, storia)} // 16k, di più solo se la richiesta è lunga
	if c.Varia {                                                             // «Parla con l'antagonista»: più varietà e penalità per le parole e le frasi ripetute
		opzioni["temperature"], opzioni["top_p"], opzioni["repeat_penalty"], opzioni["repeat_last_n"] = 0.9, 0.92, 1.15, 512
	}
	corpo, _ := json.Marshal(map[string]any{"model": c.Modello, "messages": msgs, "stream": true, "options": opzioni})
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/api/chat", bytes.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errore(http.StatusBadGateway, messaggioNonRaggiungibile("ollama"))
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		if res.StatusCode == 404 {
			return errore(http.StatusBadGateway, "Ollama non ha il modello «"+c.Modello+"»: scaricalo (comando nella guida) o scegline un altro.")
		}
		return errore(http.StatusBadGateway, "Ollama risponde con un errore: "+erroreServizio(res.StatusCode, b))
	}
	filtro := &filtroPensieri{}
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		var ev struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done  bool   `json:"done"`
			Error string `json:"error"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		if ev.Error != "" {
			return errore(http.StatusBadGateway, "Ollama si è fermato: "+accorcia(ev.Error, 200))
		}
		pezzo(filtro.passa(ev.Message.Content)) // eventuali «ragionamenti» arrivano a parte (message.thinking) e restano fuori
		if ev.Done {
			break
		}
	}
	pezzo(filtro.fine())
	return sc.Err()
}

// filtroPensieri toglie i «ragionamenti» che alcuni modelli scrivono tra <think> e </think> prima della risposta.
type filtroPensieri struct {
	dentro bool
	resto  string
}

func (f *filtroPensieri) passa(t string) string {
	t = f.resto + t
	f.resto = ""
	var out strings.Builder
	for t != "" {
		tag := "<think>"
		if f.dentro {
			tag = "</think>"
		}
		i := strings.Index(t, tag)
		if i < 0 {
			// un tag potrebbe essere spezzato tra due pezzi: tieni da parte la coda che ne è l'inizio
			k := 0
			for n := len(tag) - 1; n > 0; n-- {
				if strings.HasSuffix(t, tag[:n]) {
					k = n
					break
				}
			}
			if !f.dentro {
				out.WriteString(t[:len(t)-k])
			}
			f.resto = t[len(t)-k:]
			break
		}
		if !f.dentro {
			out.WriteString(t[:i])
		}
		t = t[i+len(tag):]
		f.dentro = !f.dentro
		if !f.dentro {
			t = strings.TrimLeft(t, "\n ")
		}
	}
	return out.String()
}

func (f *filtroPensieri) fine() string {
	if f.dentro {
		return ""
	}
	r := f.resto
	f.resto = ""
	return r
}

func erroreServizio(codice int, corpo []byte) string {
	var e struct {
		Error any `json:"error"`
	}
	json.Unmarshal(corpo, &e)
	msg := ""
	switch v := e.Error.(type) {
	case string:
		msg = v
	case map[string]any:
		if m, ok := v["message"].(string); ok {
			msg = m
		}
	}
	if msg == "" {
		msg = strings.TrimSpace(string(corpo))
	}
	return fmt.Sprintf("%d %s", codice, accorcia(msg, 200))
}

func erroreBreve(err error) string {
	return accorcia(err.Error(), 200)
}
