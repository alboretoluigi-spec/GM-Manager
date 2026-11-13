package server

// Istruzioni (system prompt) dell'Assistente IA del Master: dal 2026-11-09 il testo predefinito è la «Master Campaign
// Bible» data dall'utente (prompt_assistente.md: ruolo dell'assistente, classificazione delle informazioni, canone della
// campagna, trame dei personaggi, questioni aperte). Il Master la modifica in «⚙ Impostazioni IA» → «📜 Istruzioni
// dell'Assistente…» (impostazione «ia_prompt_assistente»; vuota = testo predefinito). Valgono per la chat
// dell'Assistente, la «Scheda per il Master» del Background e le idee dopo i riassunti; «Parla con l'antagonista» ha le sue.

import (
	_ "embed"
	"net/http"
	"strings"
	"unicode/utf8"
)

//go:embed prompt_assistente.md
var promptAssistentePredefinito string

func (s *Server) istruzioniAssistente() string {
	t, _ := s.testoIA("ia_prompt_assistente", promptAssistentePredefinito)
	return t
}

// testoIA: testo salvato dal Master o, se vuoto, quello predefinito.
func (s *Server) testoIA(chiave, predefinito string) (testo string, personalizzato bool) {
	v, _ := s.impostazione(chiave)
	if strings.TrimSpace(v) != "" {
		return v, true
	}
	return predefinito, false
}

// apiTestoIA: GET restituisce il testo attuale; PUT lo salva (vuoto o uguale al predefinito = torna al predefinito).
func (s *Server) apiTestoIA(chiave, predefinito string) gestore {
	return func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := richiedi(r, "master"); err != nil {
			return nil, err
		}
		if r.Method == http.MethodPut {
			var in struct {
				Testo string `json:"testo"`
			}
			if err := leggiJSONMax(r, &in, 256<<10); err != nil {
				return nil, err
			}
			t := strings.TrimSpace(in.Testo)
			if utf8.RuneCountInString(t) > maxPromptAntagonista {
				return nil, errore(http.StatusBadRequest, "Il testo è troppo lungo (al massimo 60.000 caratteri).")
			}
			if t == strings.TrimSpace(predefinito) {
				t = "" // uguale al predefinito: si segue il predefinito anche se in futuro cambia
			}
			if err := s.impostaImpostazione(chiave, t); err != nil {
				return nil, err
			}
		}
		t, pers := s.testoIA(chiave, predefinito)
		return map[string]any{"testo": t, "personalizzato": pers, "predefinito": predefinito}, nil
	}
}

// Spazio per Ollama. Il contesto (quanto testo il modello legge per volta) parte da 16.384 «token» e, solo per le
// richieste lunghe (Bible, l'antagonista), sale fino a 32.768 (contestoPerOllama): più memoria della scheda video, quindi può
// essere più lento. Con il massimo stanno circa 89.000 caratteri in ingresso (circa 3 caratteri a token, con margine,
// più il posto per la risposta). Le istruzioni lunghe lasciano meno spazio alla storia della conversazione, che si
// accorcia (i messaggi restano tutti salvati); almeno 6.000 caratteri di storia restano sempre.
const (
	contestoOllamaMax = 32768
	riservaRisposta   = 3000 // token lasciati alla risposta
	caratteriPerToken = 3
	caratteriOllama   = (contestoOllamaMax - riservaRisposta) * caratteriPerToken
	minStoriaOllama   = 6000
	maxContestoGruppo = 14000 // situazione del gruppo nella chat dell'Assistente
)

// contestoPerOllama: il contesto da chiedere a Ollama per questa richiesta (16.384, poi a passi di 4.096 fino a 32.768).
func contestoPerOllama(sistema string, storia []messaggioIA) int {
	n := utf8.RuneCountInString(sistema)
	for _, m := range storia {
		n += utf8.RuneCountInString(m.Testo)
	}
	serve := n/caratteriPerToken + riservaRisposta
	ctx := contestoOllama
	for ctx < serve && ctx < contestoOllamaMax {
		ctx += 4096
	}
	return min(ctx, contestoOllamaMax)
}

func limiteStoria(c configIA, sistema string, limite int) int {
	if c.Fornitore != "ollama" {
		return limite
	}
	return min(limite, max(minStoriaOllama, caratteriOllama-utf8.RuneCountInString(sistema)))
}

func accorciaContesto(ctx string, max int) string {
	if utf8.RuneCountInString(ctx) > max {
		return string([]rune(ctx)[:max]) + "\n[…]"
	}
	return ctx
}

// adattaOllama fa stare istruzioni e messaggi nel contesto di Ollama senza toccare le istruzioni (Ollama, oltre il
// limite, taglierebbe da solo e senza avvisare): si tolgono i messaggi più vecchi e, se l'ultimo da solo è troppo
// lungo (per esempio i dati della «Scheda per il Master»), se ne accorcia la parte finale.
func adattaOllama(sistema string, storia []messaggioIA) []messaggioIA {
	spazio := max(minStoriaOllama, caratteriOllama-utf8.RuneCountInString(sistema))
	out, tot := []messaggioIA{}, 0
	for i := len(storia) - 1; i >= 0; i-- {
		n := utf8.RuneCountInString(storia[i].Testo)
		if tot+n > spazio {
			if len(out) == 0 { // l'ultimo messaggio da solo non ci sta: se ne tiene l'inizio
				m := storia[i]
				m.Testo = string([]rune(m.Testo)[:spazio]) + "\n[… testo accorciato da Gaster Bless per stare nel limite del modello]"
				out = append(out, m)
			}
			break
		}
		out = append([]messaggioIA{storia[i]}, out...)
		tot += n
	}
	if len(out) > 0 && out[0].Ruolo == "ia" { // la conversazione mandata comincia sempre con una domanda
		out = out[1:]
	}
	return out
}
