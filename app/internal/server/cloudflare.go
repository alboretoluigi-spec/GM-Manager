package server

// Cloudflare Tunnel (richiesta dell'utente, 2026-11-08). Il programma cloudflared, sul PC, porta le richieste che
// arrivano all'indirizzo del dominio del Master (per esempio https://gasterbless.miodominio.it) alla porta pubblica di
// Gaster Bless, come fa Tailscale Funnel. Il certificato è quello vero di Cloudflare: nessun avviso nel browser e nessun
// certificato da installare, anche su iPhone e anche in casa.
//
// Chi gioca al tavolo e usa quell'indirizzo passa da Internet: per non trattarlo come un giocatore lontano (a cui
// i video proiettati arrivano con l'audio sul telefono), Gaster Bless conosce l'indirizzo Internet di casa e confronta
// quello del dispositivo (Cf-Connecting-Ip). L'indirizzo di casa si chiede ogni 10 minuti alla pagina pubblica
// «trace» di Cloudflare (1.1.1.1), che risponde solo con l'indirizzo di chi la chiede.
//
// Dal 2026-11-08 vale anche per Tailscale Funnel (indirizzo .ts.net, gratuito, certificato vero): i giocatori possono
// usare l'indirizzo .ts.net anche in casa (niente certificato da installare sugli iPhone) e restano «al tavolo».

import (
	"bufio"
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"
)

// indirizzoTraccia: pagina di Cloudflare che risponde con l'indirizzo Internet di chi la chiede («ip=…»).
var indirizzoTraccia = "https://1.1.1.1/cdn-cgi/trace"

type ipDiCasa struct{ v atomic.Pointer[netip.Addr] }

func (c *ipDiCasa) imposta(a netip.Addr) { c.v.Store(&a) }
func (c *ipDiCasa) uguale(a netip.Addr) bool {
	p := c.v.Load()
	return p != nil && p.IsValid() && *p == a.Unmap()
}

func (s *Server) conCloudflare() bool {
	return s.Cfg.PortaPubblica > 0 && s.Cfg.ServizioPubblico == "cloudflare"
}

// conPortaPubblica: l'accesso da Internet è configurato (Tailscale Funnel o Cloudflare Tunnel).
func (s *Server) conPortaPubblica() bool { return s.Cfg.PortaPubblica > 0 }

// cercaIPCasa chiede a Cloudflare l'indirizzo Internet di casa (quello con cui esce il PC).
func cercaIPCasa(ctx context.Context) (netip.Addr, error) {
	ctx, ferma := context.WithTimeout(ctx, 8*time.Second)
	defer ferma()
	req, _ := http.NewRequestWithContext(ctx, "GET", indirizzoTraccia, nil)
	req.Header.Set("User-Agent", "Gaster Bless")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return netip.Addr{}, err
	}
	defer r.Body.Close()
	sc := bufio.NewScanner(r.Body)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "ip="); ok {
			return netip.ParseAddr(strings.TrimSpace(v))
		}
	}
	return netip.Addr{}, sc.Err()
}

// seguiIPCasa aggiorna l'indirizzo di casa all'avvio e ogni 10 minuti (cambia se il router si riconnette).
func (s *Server) seguiIPCasa(ctx context.Context) {
	for {
		if a, err := cercaIPCasa(ctx); err == nil {
			s.ipCasa.imposta(a.Unmap())
		} else if ctx.Err() == nil {
			slog.Warn("indirizzo Internet di casa non trovato: chi usa l'indirizzo Internet al tavolo sembrerà lontano", "errore", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Minute):
		}
	}
}
