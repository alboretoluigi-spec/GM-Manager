package mqtt

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPubblicaERicevi(t *testing.T) {
	b := NuovoBrokerDiProva()
	srv := httptest.NewServer(b)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx := context.Background()
	ricevuto := make(chan string, 4)
	a, err := Dial(ctx, url, "a", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	// Un messaggio "retained" grande (più di 127 byte: lunghezza su due byte).
	grande := strings.Repeat("x", 300)
	if err := a.Publish("gasterbless/c/stato", []byte(grande), true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	c, err := Dial(ctx, url, "c", func(topic string, p []byte) { ricevuto <- topic + ":" + string(p) })
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Subscribe("gasterbless/c/stato")
	select {
	case m := <-ricevuto:
		if m != "gasterbless/c/stato:"+grande {
			t.Fatalf("messaggio conservato sbagliato: %d byte", len(m))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("il messaggio conservato non arriva a chi si iscrive dopo")
	}
	a.Publish("gasterbless/c/stato", []byte("nuovo"), true)
	select {
	case m := <-ricevuto:
		if m != "gasterbless/c/stato:nuovo" {
			t.Fatalf("messaggio: %q", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("il messaggio non arriva")
	}
	a.Publish("gasterbless/c/stato", nil, true) // cancella il conservato
	time.Sleep(100 * time.Millisecond)
	if b.Conservato("gasterbless/c/stato") != nil {
		t.Fatal("il messaggio conservato va cancellato")
	}
}
