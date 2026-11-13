package mqtt

// BrokerDiProva è un server MQTT minimo in memoria, SOLO per i test automatici (nessun uso nell'app):
// CONNECT/CONNACK, SUBSCRIBE (argomento esatto), PUBLISH con "retain", PINGREQ/PINGRESP.

import (
	"context"
	"encoding/binary"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

type BrokerDiProva struct {
	mu         sync.Mutex
	iscritti   map[string]map[*websocket.Conn]bool
	conservati map[string][]byte
	Ricevuti   []string // argomenti dei PUBLISH ricevuti (per controlli)
}

func NuovoBrokerDiProva() *BrokerDiProva {
	return &BrokerDiProva{iscritti: map[string]map[*websocket.Conn]bool{}, conservati: map[string][]byte{}}
}

// Conservato restituisce il messaggio "retained" di un argomento (nil se assente o cancellato).
func (b *BrokerDiProva) Conservato(topic string) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.conservati[topic]
}

func (b *BrokerDiProva) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"mqtt"}, InsecureSkipVerify: true})
	if err != nil {
		return
	}
	ctx := context.Background()
	var mu sync.Mutex
	invia := func(p []byte) {
		mu.Lock()
		conn.Write(ctx, websocket.MessageBinary, p)
		mu.Unlock()
	}
	defer func() {
		b.mu.Lock()
		for _, s := range b.iscritti {
			delete(s, conn)
		}
		b.mu.Unlock()
		conn.CloseNow()
	}()
	var buf []byte
	for {
		_, m, err := conn.Read(ctx)
		if err != nil {
			return
		}
		buf = append(buf, m...)
		for {
			tipo, flag, corpo, resto, ok := Pacchetto(buf)
			if !ok {
				break
			}
			buf = resto
			switch tipo {
			case tConnect:
				invia(Codifica(tConnack<<4, []byte{0, 0}))
			case tPingreq:
				invia(Codifica(tPingresp<<4, nil))
			case tSubscribe:
				id := corpo[:2]
				n := int(binary.BigEndian.Uint16(corpo[2:]))
				topic := string(corpo[4 : 4+n])
				b.mu.Lock()
				if b.iscritti[topic] == nil {
					b.iscritti[topic] = map[*websocket.Conn]bool{}
				}
				b.iscritti[topic][conn] = true
				ret := b.conservati[topic]
				b.mu.Unlock()
				invia(Codifica(tSuback<<4, append(append([]byte{}, id...), 0)))
				if ret != nil {
					invia(Codifica(tPublish<<4|1, append(Stringa(topic), ret...)))
				}
			case tPublish:
				n := int(binary.BigEndian.Uint16(corpo))
				topic := string(corpo[2 : 2+n])
				dati := append([]byte(nil), corpo[2+n:]...)
				b.mu.Lock()
				b.Ricevuti = append(b.Ricevuti, topic)
				if flag&1 == 1 {
					if len(dati) == 0 {
						delete(b.conservati, topic)
					} else {
						b.conservati[topic] = dati
					}
				}
				var dest []*websocket.Conn
				for c := range b.iscritti[topic] {
					dest = append(dest, c)
				}
				b.mu.Unlock()
				for _, c := range dest {
					c.Write(ctx, websocket.MessageBinary, Codifica(tPublish<<4, append(Stringa(topic), dati...)))
				}
			}
		}
	}
}
