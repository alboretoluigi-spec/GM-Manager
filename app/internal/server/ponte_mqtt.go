package server

// Gioco da remoto tramite server MQTT pubblici (opzione spenta all'inizio, per singola sessione).
//
// Come funziona:
//   - Quando il Master attiva l'opzione, il server crea un nome di canale casuale. Ogni giocatore
//     remoto riceve un link personale con un alias casuale e una chiave casuale a 256 bit, scritti
//     nella parte del link dopo "#", che il browser non invia mai a nessun server.
//   - È il server di Gaster Bless a collegarsi ai server MQTT. Per ogni giocatore pubblica, cifrata con la
//     sua chiave, solo la vista dell'iniziativa che vedrebbe un giocatore (niente mostri nascosti,
//     niente PF esatti dei mostri, niente messaggi, schede o telefoni) su gasterbless/<canale>/s/<alias>,
//     come messaggio "retained" (chi si collega dopo lo riceve subito).
//   - Il giocatore invia le sue azioni su gasterbless/<canale>/a, cifrate e autenticate con la sua
//     chiave. Il server le controlla (alias valido, ora vicina, nessuna ripetizione) e le esegue come
//     se arrivassero dall'app, con gli stessi permessi. L'esito torna su gasterbless/<canale>/e/<alias>.
//   - Revocare un giocatore cancella la sua chiave e il suo stato dal server MQTT. Spegnere l'opzione
//     o concludere la sessione cancella tutto e butta le chiavi.
//
// Le chiavi restano nel database cifrate con la chiave dei dati (impostazione "remoto:<sessione>").

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/mqtt"
)

// Azioni che un giocatore remoto può chiedere (le stesse del suo PG nel tracker dell'app).
// Dal 2026-11-12 (decisione dell'utente) anche gli attacchi automatici con la scelta del critico, le azioni del turno,
// l'ira del barbaro e la fine dei propri effetti: le stesse regole dell'app (turno, economia, permessi) le controlla
// azioneIniziativa.
var azioniRemote = map[string]bool{
	"iniz.iniziativa": true, "pg.danno": true, "pg.cura": true, "pg.temp": true,
	"pg.condizioni": true, "pg.ts_morte": true,
	"attacco.tira": true, "attacco.critico": true, "iniz.economia": true, "pg.ira": true, "effetto.termina": true,
	"incantesimo.ts": true, "incantesimo.cura": true, // incantesimi automatici dalla vista (incantesimi_remoto.go, fase 9b)
}

type giocatoreRemoto struct {
	Utente string `json:"utente"`
	Chiave []byte `json:"chiave"`
	Creato int64  `json:"creato"`
}

type datiRemoto struct {
	Canale    string                      `json:"canale"`
	Giocatori map[string]*giocatoreRemoto `json:"giocatori"` // alias → giocatore
}

type ponteMQTT struct {
	s      *Server
	ctx    context.Context
	ferma  context.CancelFunc
	mu     sync.Mutex
	attivi map[string]*canaleRemoto // per sessione
}

type canaleRemoto struct {
	p        *ponteMQTT
	sessione string
	ferma    context.CancelFunc
	sveglia  chan struct{} // c'è un nuovo stato da pubblicare

	mu      sync.Mutex
	d       datiRemoto
	clienti map[*mqtt.Client]bool
	ultimo  map[string][]byte // alias → ultimo stato cifrato (per i nuovi collegamenti)
	visti   map[string]int64  // azioni già eseguite (contro le ripetizioni) → scadenza
}

func nuovoPonte(s *Server) *ponteMQTT {
	ctx, ferma := context.WithCancel(context.Background())
	return &ponteMQTT{s: s, ctx: ctx, ferma: ferma, attivi: map[string]*canaleRemoto{}}
}

// riattiva, all'avvio del server, il gioco da remoto delle sessioni che lo avevano acceso.
func (p *ponteMQTT) riattiva() {
	rows, err := p.s.DB.Query(`SELECT id FROM sessioni_gioco WHERE mqtt_remoto = 1 AND stato <> 'conclusa'`)
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		p.aggiorna(id, true)
	}
}

// chiudi scollega tutto senza cancellare nulla (spegnimento del server: il gioco riprende al riavvio).
func (p *ponteMQTT) chiudi() {
	p.ferma()
	p.mu.Lock()
	for k, c := range p.attivi {
		c.ferma()
		delete(p.attivi, k)
	}
	p.mu.Unlock()
}

// aggiorna accende o spegne il gioco da remoto di una sessione.
func (p *ponteMQTT) aggiorna(sessione string, attivo bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	c := p.attivi[sessione]
	if attivo && c == nil && p.ctx.Err() == nil {
		d, err := p.leggi(sessione)
		if err != nil {
			slog.Warn("gioco da remoto: dati illeggibili, ne creo di nuovi", "errore", err)
		}
		if d == nil || d.Canale == "" {
			d = &datiRemoto{Canale: casuale(16), Giocatori: map[string]*giocatoreRemoto{}}
			p.salva(sessione, d)
		}
		ctx, ferma := context.WithCancel(p.ctx)
		c = &canaleRemoto{p: p, sessione: sessione, ferma: ferma, sveglia: make(chan struct{}, 1), d: *d,
			clienti: map[*mqtt.Client]bool{}, ultimo: map[string][]byte{}, visti: map[string]int64{}}
		p.attivi[sessione] = c
		for _, b := range p.s.Cfg.BrokerMQTT {
			go c.collega(ctx, b)
		}
		go c.pubblicatore(ctx)
		c.segnala()
	}
	if !attivo {
		delete(p.attivi, sessione)
	}
	p.mu.Unlock()
	if !attivo {
		if c != nil {
			c.cancellaTutto() // svuota i messaggi conservati sui server MQTT, poi scollega
			c.ferma()
		}
		p.s.DB.Exec(`DELETE FROM impostazioni WHERE chiave = ?`, "remoto:"+sessione)
	}
}

func (p *ponteMQTT) canale(sessione string) *canaleRemoto {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.attivi[sessione]
}

// pubblica chiede di inviare il nuovo stato del combattimento (se la sessione è in remoto).
func (p *ponteMQTT) pubblica(sessione string) {
	if c := p.canale(sessione); c != nil {
		c.segnala()
	}
}

func (p *ponteMQTT) leggi(sessione string) (*datiRemoto, error) {
	v, err := p.s.impostazione("remoto:" + sessione)
	if err != nil || v == "" {
		return nil, err
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, err
	}
	j, err := p.s.Chiavi.Decifra(b, "remoto:"+sessione)
	if err != nil {
		return nil, err
	}
	var d datiRemoto
	if err := json.Unmarshal(j, &d); err != nil {
		return nil, err
	}
	if d.Giocatori == nil {
		d.Giocatori = map[string]*giocatoreRemoto{}
	}
	return &d, nil
}

func (p *ponteMQTT) salva(sessione string, d *datiRemoto) error {
	j, _ := json.Marshal(d)
	return p.s.impostaImpostazione("remoto:"+sessione, base64.StdEncoding.EncodeToString(p.s.Chiavi.Cifra(j, "remoto:"+sessione)))
}

// ===== Giocatori =====

// nuovoGiocatore crea (o rinnova) il link di un giocatore: un nuovo alias e una nuova chiave.
// Il vecchio link smette di funzionare.
func (c *canaleRemoto) nuovoGiocatore(utente string) (alias string, chiave []byte, err error) {
	c.mu.Lock()
	var vecchi []string
	for a, g := range c.d.Giocatori {
		if g.Utente == utente {
			vecchi = append(vecchi, a)
			delete(c.d.Giocatori, a)
		}
	}
	alias, chiave = casuale(8), cripto.Casuali(32)
	c.d.Giocatori[alias] = &giocatoreRemoto{Utente: utente, Chiave: chiave, Creato: time.Now().Unix()}
	d := c.d
	c.mu.Unlock()
	for _, a := range vecchi {
		c.cancella(a)
	}
	err = c.p.salva(c.sessione, &d)
	c.segnala()
	return
}

// revoca toglie il link di un giocatore e cancella il suo stato dai server MQTT.
func (c *canaleRemoto) revoca(utente string) bool {
	c.mu.Lock()
	var tolti []string
	for a, g := range c.d.Giocatori {
		if g.Utente == utente {
			tolti = append(tolti, a)
			delete(c.d.Giocatori, a)
		}
	}
	d := c.d
	c.mu.Unlock()
	for _, a := range tolti {
		c.cancella(a)
	}
	c.p.salva(c.sessione, &d)
	return len(tolti) > 0
}

// giocatori restituisce gli utenti con un link attivo (utente → data di creazione).
func (c *canaleRemoto) giocatori() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]int64{}
	for _, g := range c.d.Giocatori {
		out[g.Utente] = g.Creato
	}
	return out
}

func (c *canaleRemoto) argomento(parti ...string) string {
	t := "gasterbless/" + c.d.Canale
	for _, p := range parti {
		t += "/" + p
	}
	return t
}

// ===== Collegamento ai server MQTT =====

func (c *canaleRemoto) collega(ctx context.Context, url string) {
	attesa := 2 * time.Second
	for ctx.Err() == nil {
		cl, err := mqtt.Dial(ctx, url, "gasterbless-"+casuale(6), c.ricevi)
		if err == nil {
			err = cl.Subscribe(c.argomento("a"))
		}
		if err == nil {
			attesa = 2 * time.Second
			c.mu.Lock()
			c.clienti[cl] = true
			ultimo := map[string][]byte{}
			for a, b := range c.ultimo {
				ultimo[a] = b
			}
			c.mu.Unlock()
			for a, b := range ultimo { // un server appena raggiunto riceve subito l'ultimo stato
				cl.Publish(c.argomento("s", a), b, true)
			}
			select {
			case <-cl.Fine():
				err = cl.Err()
			case <-ctx.Done():
			}
			c.mu.Lock()
			delete(c.clienti, cl)
			c.mu.Unlock()
			cl.Close()
			if ctx.Err() != nil {
				return
			}
		} else if cl != nil {
			cl.Close()
		}
		slog.Info("gioco da remoto: server MQTT non raggiungibile, riprovo", "server", url, "errore", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(attesa):
		}
		attesa = min(attesa*2, time.Minute)
	}
}

// invia pubblica su tutti i server collegati.
func (c *canaleRemoto) invia(argomento string, dati []byte, conserva bool) {
	c.mu.Lock()
	var l []*mqtt.Client
	for cl := range c.clienti {
		l = append(l, cl)
	}
	c.mu.Unlock()
	for _, cl := range l {
		if err := cl.Publish(argomento, dati, conserva); err != nil {
			slog.Info("gioco da remoto: invio non riuscito", "errore", err)
		}
	}
}

func (c *canaleRemoto) segnala() {
	select {
	case c.sveglia <- struct{}{}:
	default:
	}
}

// pubblicatore invia lo stato in un'unica goroutine: le richieste dell'app non aspettano mai i
// server MQTT, e più cambiamenti ravvicinati diventano un solo invio.
func (c *canaleRemoto) pubblicatore(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.sveglia:
		}
		c.pubblicaStato()
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (c *canaleRemoto) pubblicaStato() {
	s := c.p.s
	var titolo string
	s.DB.QueryRow(`SELECT titolo FROM sessioni_gioco WHERE id = ?`, c.sessione).Scan(&titolo)
	c.mu.Lock()
	gioc := map[string]*giocatoreRemoto{}
	for a, g := range c.d.Giocatori {
		gioc[a] = g
	}
	c.mu.Unlock()
	for alias, g := range gioc {
		// Chi non partecipa più alla sessione non riceve più nulla.
		if _, ok := s.partecipa(s.utentePerID(g.Utente), c.sessione); !ok {
			c.cancella(alias)
			continue
		}
		// la vista del giocatore (con gli attacchi e le azioni del turno del suo PG), senza proprietari né ritratti
		v := *s.vistaPerRuolo(c.sessione, "remoto", g.Utente)
		v.Ruolo = "giocatore"
		for _, cc := range v.Combattenti {
			cc.Proprietario, cc.HaRitratto = "", false
		}
		j, _ := json.Marshal(map[string]any{"titolo": titolo, "ts": time.Now().UnixMilli(), "vista": v})
		b := sigilla(g.Chiave, j, "stato|"+c.d.Canale+"|"+alias)
		c.mu.Lock()
		if c.d.Giocatori[alias] == nil { // revocato nel frattempo
			c.mu.Unlock()
			continue
		}
		c.ultimo[alias] = b
		c.mu.Unlock()
		c.invia(c.argomento("s", alias), b, true)
	}
}

// cancella svuota lo stato conservato di un alias sui server MQTT.
func (c *canaleRemoto) cancella(alias string) {
	c.mu.Lock()
	delete(c.ultimo, alias)
	c.mu.Unlock()
	c.invia(c.argomento("s", alias), nil, true)
}

func (c *canaleRemoto) cancellaTutto() {
	c.mu.Lock()
	var l []string
	for a := range c.d.Giocatori {
		l = append(l, a)
	}
	for a := range c.ultimo {
		l = append(l, a)
	}
	c.mu.Unlock()
	for _, a := range l {
		c.cancella(a)
	}
}

// ===== Azioni dei giocatori =====

const tolleranzaAzioni = 2 * time.Minute

type azioneRemota struct {
	Tipo string          `json:"tipo"`
	Dati json.RawMessage `json:"dati"`
	TS   int64           `json:"ts"` // millisecondi
	N    string          `json:"n"`  // identificativo casuale dell'azione
}

// ricevi gestisce un messaggio arrivato su gasterbless/<canale>/a (può arrivare da più server).
func (c *canaleRemoto) ricevi(argomento string, dati []byte) {
	if argomento != c.argomento("a") || len(dati) > 8192 {
		return
	}
	var busta struct {
		U string `json:"u"`
		C string `json:"c"`
	}
	if json.Unmarshal(dati, &busta) != nil {
		return
	}
	c.mu.Lock()
	g := c.d.Giocatori[busta.U]
	c.mu.Unlock()
	if g == nil {
		return // alias sconosciuto o revocato: si ignora in silenzio
	}
	cif, err := base64.StdEncoding.DecodeString(busta.C)
	if err != nil {
		return
	}
	chiaro, err := apri(g.Chiave, cif, "azione|"+c.d.Canale+"|"+busta.U)
	if err != nil {
		return // non firmata con la chiave del giocatore
	}
	var a azioneRemota
	if json.Unmarshal(chiaro, &a) != nil || a.N == "" || len(a.N) > 64 {
		return
	}
	ora := time.Now()
	if d := ora.Sub(time.UnixMilli(a.TS)); d > tolleranzaAzioni || d < -tolleranzaAzioni {
		c.esito(busta.U, g, a.N, nil, errore(400, "L'orologio del telefono non è giusto: controlla data e ora."))
		return
	}
	c.mu.Lock()
	for k, scade := range c.visti {
		if scade < ora.Unix() {
			delete(c.visti, k)
		}
	}
	gia := c.visti[busta.U+"|"+a.N] != 0
	c.visti[busta.U+"|"+a.N] = ora.Add(2 * tolleranzaAzioni).Unix()
	c.mu.Unlock()
	if gia {
		return // stessa azione arrivata da un altro server, o ripetuta da qualcuno
	}
	if !azioniRemote[a.Tipo] {
		c.esito(busta.U, g, a.N, nil, errVietato)
		return
	}
	u := c.p.s.utentePerID(g.Utente)
	if u == nil {
		c.esito(busta.U, g, a.N, nil, errVietato)
		return
	}
	r, err := c.p.s.azioneIniziativa(u, a.Tipo, c.sessione, a.Dati)
	c.esito(busta.U, g, a.N, r, err)
}

func (c *canaleRemoto) esito(alias string, g *giocatoreRemoto, n string, r any, err error) {
	out := map[string]any{"n": n, "ok": err == nil, "risultato": r}
	if err != nil {
		var e *ErrAPI
		if errors.As(err, &e) {
			out["errore"] = e.Msg
		} else {
			out["errore"] = "Operazione non riuscita."
		}
	}
	j, _ := json.Marshal(out)
	c.invia(c.argomento("e", alias), sigilla(g.Chiave, j, "esito|"+c.d.Canale+"|"+alias), false)
}

// evento manda a ogni giocatore remoto della sessione un esito dello scontro (attacco.esito, incantesimo.esito) come lo
// vedrebbe nell'app: «per» lo calcola per l'utente (nil: niente). Viaggia sull'argomento dell'esito, non conservato.
func (p *ponteMQTT) evento(sessione, tipo string, per func(u *Utente) any) {
	if p == nil {
		return
	}
	p.mu.Lock()
	c := p.attivi[sessione]
	p.mu.Unlock()
	if c == nil {
		return
	}
	c.mu.Lock()
	gioc := map[string]*giocatoreRemoto{}
	for a, g := range c.d.Giocatori {
		gioc[a] = g
	}
	canale := c.d.Canale
	c.mu.Unlock()
	for alias, g := range gioc {
		u := p.s.utentePerID(g.Utente)
		if u == nil {
			continue
		}
		x := per(u)
		if x == nil {
			continue
		}
		j, _ := json.Marshal(map[string]any{"evento": tipo, "dati": x})
		go c.invia(c.argomento("e", alias), sigilla(g.Chiave, j, "esito|"+canale+"|"+alias), false)
	}
}

// utentePerID carica un utente (per le azioni che non arrivano da una sessione dell'app).
func (s *Server) utentePerID(id string) *Utente {
	u := &Utente{}
	if err := s.DB.QueryRow(`SELECT id, nome, ruolo, stato FROM utenti WHERE id = ?`, id).Scan(&u.ID, &u.Nome, &u.Ruolo, &u.Stato); err != nil {
		return nil
	}
	return u
}

// ===== Cifratura (AES-256-GCM, formato nonce||testo cifrato, compatibile con WebCrypto) =====

func sigilla(chiave, dati []byte, contesto string) []byte {
	g := gcm(chiave)
	nonce := cripto.Casuali(g.NonceSize())
	return g.Seal(nonce, nonce, dati, []byte(contesto))
}

func apri(chiave, dati []byte, contesto string) ([]byte, error) {
	g := gcm(chiave)
	if len(dati) < g.NonceSize()+g.Overhead() {
		return nil, errors.New("messaggio troppo corto")
	}
	return g.Open(nil, dati[:g.NonceSize()], dati[g.NonceSize():], []byte(contesto))
}

func gcm(chiave []byte) cipher.AEAD {
	b, err := aes.NewCipher(chiave)
	if err != nil {
		panic(err) // chiavi sempre di 32 byte, create dal server
	}
	g, _ := cipher.NewGCM(b)
	return g
}

// casuale restituisce n byte casuali in esadecimale.
func casuale(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// collegati dice a quanti server MQTT il canale è collegato in questo momento.
func (c *canaleRemoto) collegati() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.clienti)
}
