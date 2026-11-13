// Package mqtt è un client MQTT 3.1.1 minimo sopra WebSocket, scritto in proprio per non aggiungere
// librerie al progetto. Serve solo al ponte per il gioco da remoto (internal/server/ponte_mqtt.go):
// QoS 0, messaggi "retained", una sottoscrizione per argomento esatto, ping periodico.
package mqtt

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Tipi di pacchetto (MQTT 3.1.1, sezione 2.2.1).
const (
	tConnect   = 1
	tConnack   = 2
	tPublish   = 3
	tSubscribe = 8
	tSuback    = 9
	tPingreq   = 12
	tPingresp  = 13
	tDisconn   = 14
)

// Client è una connessione a un server MQTT.
type Client struct {
	conn  *websocket.Conn
	mu    sync.Mutex // serializza le scritture
	onMsg func(topic string, payload []byte)
	ctx   context.Context
	stop  context.CancelFunc
	fine  chan struct{}
	err   error
}

// Dial si collega (URL wss://...), invia CONNECT e attende CONNACK.
func Dial(ctx context.Context, url, clientID string, onMsg func(topic string, payload []byte)) (*Client, error) {
	dctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dctx, url, &websocket.DialOptions{Subprotocols: []string{"mqtt"}})
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(1 << 20)
	cctx, stop := context.WithCancel(context.Background())
	c := &Client{conn: conn, onMsg: onMsg, ctx: cctx, stop: stop, fine: make(chan struct{})}
	// CONNECT: protocollo "MQTT" livello 4, sessione pulita, keep alive 60 s.
	var v []byte
	v = append(v, stringa("MQTT")...)
	v = append(v, 4, 0x02, 0, 60)
	v = append(v, stringa(clientID)...)
	if err := c.invia(tConnect<<4, v); err != nil {
		conn.CloseNow()
		return nil, err
	}
	letti := make(chan error, 1)
	go c.leggi(letti)
	select {
	case err := <-letti:
		if err != nil {
			c.Close()
			return nil, err
		}
	case <-dctx.Done():
		c.Close()
		return nil, errors.New("il server MQTT non risponde")
	}
	go c.ping()
	return c, nil
}

// Publish invia un messaggio con QoS 0 (retain = il server lo conserva per chi si collega dopo).
func (c *Client) Publish(topic string, payload []byte, retain bool) error {
	h := byte(tPublish << 4)
	if retain {
		h |= 1
	}
	return c.invia(h, append(stringa(topic), payload...))
}

// Subscribe si iscrive a un argomento con QoS 0.
func (c *Client) Subscribe(topic string) error {
	v := []byte{0, 1}
	v = append(v, stringa(topic)...)
	v = append(v, 0)
	return c.invia(tSubscribe<<4|2, v)
}

// Fine si chiude quando la connessione cade; Err dice perché.
func (c *Client) Fine() <-chan struct{} { return c.fine }
func (c *Client) Err() error            { return c.err }

// Close chiude la connessione in modo ordinato.
func (c *Client) Close() {
	c.invia(tDisconn<<4, nil)
	c.stop()
	c.conn.Close(websocket.StatusNormalClosure, "")
}

func (c *Client) invia(intestazione byte, variabile []byte) error {
	b := []byte{intestazione}
	b = append(b, lunghezza(len(variabile))...)
	b = append(b, variabile...)
	c.mu.Lock()
	defer c.mu.Unlock()
	wctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()
	return c.conn.Write(wctx, websocket.MessageBinary, b)
}

func (c *Client) ping() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-t.C:
			if c.invia(tPingreq<<4, nil) != nil {
				return
			}
		}
	}
}

// leggi riceve i pacchetti (anche spezzati o uniti in più messaggi WebSocket). Il primo CONNACK
// viene segnalato su connesso.
func (c *Client) leggi(connesso chan<- error) {
	defer close(c.fine)
	defer c.stop()
	var buf []byte
	primo := true
	for {
		_, b, err := c.conn.Read(c.ctx)
		if err != nil {
			c.err = err
			if primo {
				connesso <- err
			}
			return
		}
		buf = append(buf, b...)
		for {
			tipo, flag, corpo, resto, ok := Pacchetto(buf)
			if !ok {
				break
			}
			buf = resto
			switch tipo {
			case tConnack:
				if primo {
					primo = false
					if len(corpo) < 2 || corpo[1] != 0 {
						connesso <- fmt.Errorf("connessione MQTT rifiutata (codice %d)", corpo[min(1, len(corpo)-1)])
						return
					}
					connesso <- nil
				}
			case tPublish:
				if len(corpo) < 2 {
					continue
				}
				n := int(binary.BigEndian.Uint16(corpo))
				if len(corpo) < 2+n {
					continue
				}
				topic := string(corpo[2 : 2+n])
				dati := corpo[2+n:]
				if (flag>>1)&3 > 0 && len(dati) >= 2 { // QoS 1/2: c'è l'identificativo (non usato)
					dati = dati[2:]
				}
				if c.onMsg != nil {
					c.onMsg(topic, append([]byte(nil), dati...))
				}
			}
		}
	}
}

// ===== Codifica =====

func stringa(s string) []byte {
	b := make([]byte, 2, 2+len(s))
	binary.BigEndian.PutUint16(b, uint16(len(s)))
	return append(b, s...)
}

func lunghezza(n int) []byte {
	var b []byte
	for {
		d := byte(n % 128)
		n /= 128
		if n > 0 {
			d |= 0x80
		}
		b = append(b, d)
		if n == 0 {
			return b
		}
	}
}

// Pacchetto estrae il primo pacchetto completo da buf (tipo, flag, corpo) e restituisce il resto.
func Pacchetto(buf []byte) (tipo, flag byte, corpo, resto []byte, ok bool) {
	if len(buf) < 2 {
		return
	}
	n, molt, i := 0, 1, 1
	for {
		if i >= len(buf) || i > 4 {
			return
		}
		d := buf[i]
		n += int(d&0x7f) * molt
		molt *= 128
		i++
		if d&0x80 == 0 {
			break
		}
	}
	if len(buf) < i+n {
		return
	}
	return buf[0] >> 4, buf[0] & 0x0f, buf[i : i+n], buf[i+n:], true
}

// Codifica costruisce un pacchetto (usato dal server di prova).
func Codifica(intestazione byte, variabile []byte) []byte {
	b := []byte{intestazione}
	b = append(b, lunghezza(len(variabile))...)
	return append(b, variabile...)
}

// Stringa codifica una stringa MQTT (2 byte di lunghezza + testo).
func Stringa(s string) []byte { return stringa(s) }
