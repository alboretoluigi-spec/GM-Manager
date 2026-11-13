package server

// Tasto «Aggiorna» del Master (docs/PIANO_AGGIORNA.md, fase U2).
//
// Il servizio Gaster Bless gira con un account senza privilegi: non può usare le credenziali GitHub del Master né
// sostituire e riavviare sé stesso. Per questo non aggiorna nulla da solo: chiede a Windows di avviare due
// operazioni pianificate, create una volta sola da installa.ps1 con l'utente del Master:
//   \GasterBless\Controlla  -> strumenti\aggiorna_auto.ps1 -SoloControllo (elenco delle novità su GitHub)
//   \GasterBless\Aggiorna   -> strumenti\aggiorna_auto.ps1               (aggiornamento completo)
// Lo script scrive l'esito in <cartella dati>\aggiornamento\controllo.json e stato.json, che qui si leggono.
// Tutto solo per il Master e solo dal PC di Gaster Bless.

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

// operazioniPianificate è sostituibile nei test (su Linux e nelle prove non esiste il Task Scheduler).
type operazioniPianificate interface {
	Presente(nome string) bool
	Avvia(nome string) error
}

// operazioniWindows usa schtasks.exe, presente in ogni Windows.
type operazioniWindows struct{}

func (operazioniWindows) Presente(nome string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return exec.Command("schtasks.exe", "/query", "/tn", `\GasterBless\`+nome).Run() == nil
}

func (operazioniWindows) Avvia(nome string) error {
	if runtime.GOOS != "windows" {
		return errors.New("non disponibile su questo sistema")
	}
	out, err := exec.Command("schtasks.exe", "/run", "/tn", `\GasterBless\`+nome).CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

// operazioni restituisce quelle in uso (Windows, oppure quelle finte impostate dai test).
func (s *Server) operazioni() operazioniPianificate {
	if s.opPianificate != nil {
		return s.opPianificate
	}
	return operazioniWindows{}
}

func (s *Server) cartellaAggiornamento() string {
	return filepath.Join(s.Cfg.CartellaDati, "aggiornamento")
}

func (s *Server) rotteAggiornamento(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/aggiornamento", api(s.apiAggiornamento))
	mux.Handle("POST /api/v1/aggiornamento/controlla", api(s.apiAggiornamentoControlla))
	mux.Handle("POST /api/v1/aggiornamento/avvia", api(s.apiAggiornamentoAvvia))
	mux.Handle("GET /api/v1/aggiornamento/registro", api(s.apiAggiornamentoRegistro))
}

// soloMasterLocale: il Master, dal PC di Gaster Bless (scelta dell'utente del 2026-11-11).
func soloMasterLocale(r *http.Request) error {
	u, err := richiedi(r, "master")
	if err != nil {
		return err
	}
	if !u.locale {
		return errore(http.StatusForbidden, "L'aggiornamento si avvia solo dal PC di Gaster Bless.")
	}
	return nil
}

// leggiEsito legge uno dei file JSON scritti dallo script (nil se non c'è o non è leggibile).
func (s *Server) leggiEsito(nome string) map[string]any {
	b, err := os.ReadFile(filepath.Join(s.cartellaAggiornamento(), nome))
	if err != nil {
		return nil
	}
	b = []byte(strings.TrimPrefix(string(b), "\ufeff")) // PowerShell potrebbe aggiungere il BOM
	var out map[string]any
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

// aggiornamentoInCorso: lo script tiene un file di blocco mentre lavora (ignorato se più vecchio di 30 minuti).
func (s *Server) aggiornamentoInCorso() bool {
	st, err := os.Stat(filepath.Join(s.cartellaAggiornamento(), "in_corso.lock"))
	return err == nil && time.Since(st.ModTime()) < 30*time.Minute
}

// apiAggiornamento: operazioni pronte?, ultimo controllo, ultimo aggiornamento, se ce n'è uno in corso.
func (s *Server) apiAggiornamento(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := soloMasterLocale(r); err != nil {
		return nil, err
	}
	op := s.operazioni()
	pronto := op.Presente("Aggiorna")
	out := map[string]any{
		"pronto":              pronto,
		"controlloPronto":     pronto && op.Presente("Controlla"),
		"inCorso":             s.aggiornamentoInCorso(),
		"versione":            s.versioneApp(),
		"avviatoIl":           s.avviatoIl.Unix(),
		"ultimoControllo":     s.leggiEsito("controllo.json"),
		"ultimoAggiornamento": s.leggiEsito("stato.json"),
	}
	if !pronto {
		out["messaggio"] = "Manca la preparazione una tantum: lancia una volta aggiorna.ps1 da amministratore (vedi la guida del Master)."
	}
	return out, nil
}

// apiAggiornamentoControlla avvia il controllo delle novità; la pagina poi rilegge GET /aggiornamento.
func (s *Server) apiAggiornamentoControlla(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := soloMasterLocale(r); err != nil {
		return nil, err
	}
	if s.aggiornamentoInCorso() {
		return nil, errore(http.StatusConflict, "C'è già un aggiornamento in corso.")
	}
	if err := s.operazioni().Avvia("Controlla"); err != nil {
		return nil, errore(http.StatusConflict, "Non riesco ad avviare il controllo: "+err.Error()+
			" Controlla di essere collegato a Windows sul PC di Gaster Bless e di aver fatto la preparazione una tantum.")
	}
	return map[string]any{"avviato": true}, nil
}

// apiAggiornamentoAvvia avvisa tutti i collegati e avvia l'aggiornamento completo. Gaster Bless si fermerà per circa
// un minuto: la pagina del Master aspetta e si ricollega da sola.
func (s *Server) apiAggiornamentoAvvia(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := soloMasterLocale(r); err != nil {
		return nil, err
	}
	if s.aggiornamentoInCorso() {
		return nil, errore(http.StatusConflict, "C'è già un aggiornamento in corso.")
	}
	if !s.operazioni().Presente("Aggiorna") {
		return nil, errore(http.StatusConflict, "Manca la preparazione una tantum: lancia una volta aggiorna.ps1 da amministratore.")
	}
	if err := s.operazioni().Avvia("Aggiorna"); err != nil {
		return nil, errore(http.StatusConflict, "Non riesco ad avviare l'aggiornamento: "+err.Error()+
			" Controlla di essere collegato a Windows sul PC di Gaster Bless.")
	}
	s.Hub.ATutti(Msg{Tipo: "sistema.aggiornamento", Dati: map[string]any{
		"testo": "Gaster Bless si sta aggiornando: tra qualche minuto si ricollega da sola."}})
	return map[string]any{"avviato": true}, nil
}

// apiAggiornamentoRegistro: il testo dell'ultimo registro (al massimo gli ultimi 200 KB).
func (s *Server) apiAggiornamentoRegistro(w http.ResponseWriter, r *http.Request) (any, error) {
	if err := soloMasterLocale(r); err != nil {
		return nil, err
	}
	quale := "stato.json"
	if r.URL.Query().Get("di") == "controllo" {
		quale = "controllo.json"
	}
	es := s.leggiEsito(quale)
	nome, _ := es["registro"].(string)
	// Il percorso viene da un file su disco: se ne tiene solo il nome, dentro la cartella dell'aggiornamento.
	nome = filepath.Base(strings.ReplaceAll(nome, `\`, "/"))
	if nome == "" || nome == "." || nome == "/" || !strings.HasPrefix(nome, "registro-") || !strings.HasSuffix(nome, ".txt") {
		return nil, errNonTrovato
	}
	b, err := os.ReadFile(filepath.Join(s.cartellaAggiornamento(), nome))
	if err != nil {
		return nil, errNonTrovato
	}
	testo := testoRegistro(b)
	if len(testo) > 200<<10 {
		testo = strings.ToValidUTF8(testo[len(testo)-200<<10:], "")
	}
	return map[string]any{"nome": nome, "testo": testo}, nil
}

// testoRegistro: il registro di PowerShell può essere in UTF-8 (con o senza BOM) o in UTF-16.
func testoRegistro(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		u := make([]uint16, 0, len(b)/2)
		for i := 2; i+1 < len(b); i += 2 {
			u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
		}
		return string(utf16.Decode(u))
	}
	return strings.TrimPrefix(string(b), "\ufeff")
}

// ATutti invia un messaggio a ogni dispositivo collegato (Master e giocatori di tutti i gruppi).
func (h *Hub) ATutti(m Msg) {
	h.mu.RLock()
	dest := make([]*Cliente, 0, len(h.clienti))
	for c := range h.clienti {
		dest = append(dest, c)
	}
	h.mu.RUnlock()
	for _, c := range dest {
		h.invia(c, m)
	}
}
