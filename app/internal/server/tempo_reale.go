package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"gasterbless/internal/rete"
)

// Msg è il formato di tutti i messaggi in tempo reale, in entrambe le direzioni.
type Msg struct {
	Tipo string `json:"tipo"`
	Dati any    `json:"dati,omitempty"`
	ID   string `json:"id,omitempty"` // risposta a una richiesta del client
}

type msgIn struct {
	Tipo string          `json:"tipo"`
	Dati json.RawMessage `json:"dati"`
	ID   string          `json:"id"`
}

// Cliente è una connessione WebSocket aperta.
type Cliente struct {
	U      *Utente
	ip     netip.Addr
	invio  chan []byte
	mu     sync.Mutex
	canali map[string]bool
	chiudi context.CancelFunc
}

// Iscritto dice se il cliente riceve un canale.
func (c *Cliente) Iscritto(canale string) bool {
	// Il Master vede tutti i gruppi, anche quelli creati dopo che si è collegato.
	if c.U.Master() && strings.HasPrefix(canale, "gruppo:") {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.canali[canale]
}

// GestoreWS gestisce un tipo di messaggio in arrivo. Restituisce l'eventuale risposta.
type GestoreWS func(c *Cliente, dati json.RawMessage) (any, error)

// Hub tiene le connessioni e instrada i messaggi.
type Hub struct {
	s       *Server
	mu      sync.RWMutex
	clienti map[*Cliente]bool
	gestori map[string]GestoreWS
}

// NuovoHub crea l'hub e registra i gestori di base.
func NuovoHub(s *Server) *Hub {
	h := &Hub{s: s, clienti: map[*Cliente]bool{}, gestori: map[string]GestoreWS{}}
	h.Registra("ping", func(*Cliente, json.RawMessage) (any, error) { return "pong", nil })
	return h
}

// Registra associa un gestore a un tipo di messaggio in arrivo.
func (h *Hub) Registra(tipo string, g GestoreWS) { h.gestori[tipo] = g }

// Gestisci apre la connessione WebSocket per un utente approvato.
func (h *Hub) Gestisci(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	if u == nil || !u.Approvato() {
		http.Error(w, "Accesso richiesto.", http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover})
	if err != nil {
		return
	}
	conn.SetReadLimit(64 << 10)
	ctx, cancel := context.WithCancel(context.Background())
	c := &Cliente{U: u, ip: rete.IPRichiesta(r), invio: make(chan []byte, 128), canali: map[string]bool{}, chiudi: cancel}
	c.canali["utente:"+u.ID] = true
	if u.Master() {
		c.canali["master"] = true
	}
	for _, g := range h.s.gruppiDi(u) {
		c.canali["gruppo:"+g] = true
	}
	h.mu.Lock()
	h.clienti[c] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clienti, c)
		h.mu.Unlock()
		cancel()
		conn.CloseNow()
	}()

	// Scrittura: un solo goroutine scrive sulla connessione.
	go func() {
		ping := time.NewTicker(25 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				conn.Close(websocket.StatusNormalClosure, "")
				return
			case b := <-c.invio:
				wctx, wc := context.WithTimeout(ctx, 10*time.Second)
				err := conn.Write(wctx, websocket.MessageText, b)
				wc()
				if err != nil {
					cancel()
					return
				}
			case <-ping.C:
				pctx, pc := context.WithTimeout(ctx, 10*time.Second)
				err := conn.Ping(pctx)
				pc()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	h.invia(c, Msg{Tipo: "benvenuto", Dati: map[string]any{"utente": u}})
	go h.s.controlloAllAccesso(u) // il giocatore si collega: controllo della scheda se una sua sessione è in corso

	// Lettura: ogni messaggio viene validato e passato al suo gestore.
	limite := newSecchiello(30, time.Second) // al massimo ~30 messaggi al secondo per connessione
	for {
		typ, b, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText || !limite.consenti() {
			continue
		}
		var m msgIn
		if json.Unmarshal(b, &m) != nil || m.Tipo == "" || len(m.ID) > 40 {
			h.invia(c, Msg{Tipo: "errore", Dati: "Messaggio non valido."})
			continue
		}
		g := h.gestori[m.Tipo]
		if g == nil {
			h.invia(c, Msg{Tipo: "errore", ID: m.ID, Dati: "Tipo di messaggio sconosciuto."})
			continue
		}
		// Lo stato dell'account potrebbe essere cambiato: si ricontrolla prima di ogni azione.
		if !h.s.utenteAncoraValido(u.ID) {
			return
		}
		ris, err := g(c, m.Dati)
		if err != nil {
			var e *ErrAPI
			msg := "Errore interno."
			if errors.As(err, &e) {
				msg = e.Msg
			} else {
				slog.Error("tempo reale", "tipo", m.Tipo, "errore", err)
			}
			h.invia(c, Msg{Tipo: "errore", ID: m.ID, Dati: msg})
			continue
		}
		if m.ID != "" {
			h.invia(c, Msg{Tipo: "risposta", ID: m.ID, Dati: ris})
		}
	}
}

func (h *Hub) invia(c *Cliente, m Msg) {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	select {
	case c.invio <- b:
	default:
		// Il dispositivo non riesce a stare al passo: si chiude la connessione, la pagina si ricollegherà.
		c.chiudi()
	}
}

// Pubblica invia lo stesso messaggio a tutti gli iscritti a un canale.
func (h *Hub) Pubblica(canale string, m Msg) {
	h.PubblicaPer(canale, func(*Cliente) *Msg { return &m })
}

// PubblicaPer costruisce un messaggio diverso per ogni destinatario (o nessuno, se f restituisce nil).
// È il punto in cui si applica il filtro "ognuno vede solo ciò che può vedere".
func (h *Hub) PubblicaPer(canale string, f func(*Cliente) *Msg) {
	h.mu.RLock()
	var dest []*Cliente
	for c := range h.clienti {
		if c.Iscritto(canale) {
			dest = append(dest, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range dest {
		if m := f(c); m != nil {
			h.invia(c, *m)
		}
	}
}

// AlMaster invia un avviso a tutte le connessioni del Master.
func (h *Hub) AlMaster(m Msg) { h.Pubblica("master", m) }

// AUtente invia un messaggio a tutte le connessioni di un utente.
func (h *Hub) AUtente(id string, m Msg) { h.Pubblica("utente:"+id, m) }

// Iscrivi aggiunge un canale a un cliente (dopo che il chiamante ha verificato il permesso).
func (h *Hub) Iscrivi(c *Cliente, canale string) {
	c.mu.Lock()
	c.canali[canale] = true
	c.mu.Unlock()
}

// Disiscrivi toglie i canali con un certo prefisso (per esempio quando si cambia sessione).
func (h *Hub) Disiscrivi(c *Cliente, prefisso string) {
	c.mu.Lock()
	for k := range c.canali {
		if len(k) >= len(prefisso) && k[:len(prefisso)] == prefisso {
			delete(c.canali, k)
		}
	}
	c.mu.Unlock()
}

// ChiudiUtente chiude tutte le connessioni di un utente (blocco, sospensione, cambio di gruppi).
func (h *Hub) ChiudiUtente(id string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clienti {
		if c.U.ID == id {
			c.chiudi()
		}
	}
}

// ChiudiEsterni chiude le connessioni arrivate dalla VPN (pulsante "Chiudi subito l'accesso esterno").
func (h *Hub) ChiudiEsterni(f *rete.Filtro) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clienti {
		if !f.Ammesso(c.ip) {
			c.chiudi()
		}
	}
}

// ChiudiTutti chiude ogni connessione (arresto del server).
func (h *Hub) ChiudiTutti() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clienti {
		c.chiudi()
	}
}

// Collegati restituisce gli utenti con almeno una connessione aperta.
func (h *Hub) Collegati() map[string]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := map[string]bool{}
	for c := range h.clienti {
		out[c.U.ID] = true
	}
	return out
}

// secchiello limita la frequenza dei messaggi in arrivo.
type secchiello struct {
	cap, gettoni float64
	ricarica     float64
	ultimo       time.Time
}

func newSecchiello(n int, per time.Duration) *secchiello {
	return &secchiello{cap: float64(n), gettoni: float64(n), ricarica: float64(n) / per.Seconds(), ultimo: time.Now()}
}

func (b *secchiello) consenti() bool {
	ora := time.Now()
	b.gettoni = min(b.cap, b.gettoni+ora.Sub(b.ultimo).Seconds()*b.ricarica)
	b.ultimo = ora
	if b.gettoni < 1 {
		return false
	}
	b.gettoni--
	return true
}
