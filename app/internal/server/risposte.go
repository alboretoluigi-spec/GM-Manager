package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"
)

// ErrAPI è un errore con codice HTTP e messaggio in italiano da mostrare all'utente.
type ErrAPI struct {
	Codice int
	Msg    string
	Tipo   string // facoltativo: codice leggibile dalle pagine (per esempio "monete_insufficienti")
}

func (e *ErrAPI) Error() string { return e.Msg }

func errore(codice int, msg string) *ErrAPI { return &ErrAPI{Codice: codice, Msg: msg} }

// erroreMonete: il personaggio non ha abbastanza monete (la pagina mostra l'immagine «povero»).
func erroreMonete(costoRame int) *ErrAPI {
	return &ErrAPI{Codice: http.StatusConflict, Msg: "Monete insufficienti: servono " + testoMo(costoRame) + ".", Tipo: "monete_insufficienti"}
}

var (
	errNonAutenticato = errore(http.StatusUnauthorized, "Devi accedere per continuare.")
	errVietato        = errore(http.StatusForbidden, "Non hai il permesso per questa operazione.")
	errNonTrovato     = errore(http.StatusNotFound, "Elemento non trovato.")
	errDatiNonValidi  = errore(http.StatusBadRequest, "Dati non validi.")
	// errGiaRisposto: il gestore ha già scritto la risposta (per esempio un 409 con dettagli).
	errGiaRisposto = errors.New("risposta già inviata")
)

// gestore è la firma dei gestori delle API: restituiscono un valore da inviare in JSON o un errore.
type gestore func(w http.ResponseWriter, r *http.Request) (any, error)

// api adatta un gestore a http.Handler, con la gestione uniforme degli errori.
// I dettagli degli errori interni vanno solo nel log (mai telefoni o password).
func api(h gestore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, err := h(w, r)
		if errors.Is(err, errGiaRisposto) {
			return
		}
		if err != nil {
			var e *ErrAPI
			if errors.As(err, &e) {
				out := map[string]string{"errore": e.Msg}
				if e.Tipo != "" {
					out["codice"] = e.Tipo
				}
				scriviJSON(w, e.Codice, out)
				return
			}
			slog.Error("errore interno", "percorso", r.URL.Path, "errore", err)
			scriviJSON(w, http.StatusInternalServerError, map[string]string{"errore": "Errore interno del server. Il dettaglio è nel registro di Gaster Bless."})
			return
		}
		if v == nil {
			v = map[string]bool{"ok": true}
		}
		scriviJSON(w, http.StatusOK, v)
	})
}

func scriviJSON(w http.ResponseWriter, codice int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(codice)
	json.NewEncoder(w).Encode(v)
}

// leggiJSON decodifica il corpo della richiesta (massimo 1 MB, campi sconosciuti rifiutati).
func leggiJSON(r *http.Request, v any) error {
	return leggiJSONMax(r, v, 1<<20)
}

func leggiJSONMax(r *http.Request, v any, max int64) error {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		return errore(http.StatusUnsupportedMediaType, "Formato della richiesta non valido.")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, max))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errore(http.StatusBadRequest, "Dati della richiesta non leggibili.")
	}
	return nil
}

// testo pulisce e controlla un campo di testo: spazi ai lati, lunghezza, niente caratteri di controllo.
func testo(s string, campo string, min, max int) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) {
		return "", errore(http.StatusBadRequest, campo+": testo non valido.")
	}
	for _, r := range s {
		if r < 0x20 && r != '\n' && r != '\t' {
			return "", errore(http.StatusBadRequest, campo+": contiene caratteri non ammessi.")
		}
	}
	n := utf8.RuneCountInString(s)
	if n < min {
		if min == 1 {
			return "", errore(http.StatusBadRequest, campo+": campo obbligatorio.")
		}
		return "", errore(http.StatusBadRequest, campo+": troppo corto.")
	}
	if n > max {
		return "", errore(http.StatusBadRequest, campo+": troppo lungo (massimo "+itoa(max)+" caratteri).")
	}
	return s, nil
}

// rigaSingola come testo, ma senza a capo.
func rigaSingola(s, campo string, min, max int) (string, error) {
	s, err := testo(s, campo, min, max)
	if err == nil && strings.ContainsAny(s, "\n\t") {
		return "", errore(http.StatusBadRequest, campo+": deve stare su una riga.")
	}
	return s, err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
