package rete

import (
	"context"
	"crypto/sha256"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
)

func sha256Sum(b []byte) [32]byte { return sha256.Sum256(b) }

// Filtro decide quali indirizzi possono collegarsi.
// Modalità "casa": solo il PC stesso e le reti private (192.168.x, 10.x, 172.16-31.x).
// Modalità "vpn": anche le reti della VPN indicate in configurazione (per Tailscale 100.64.0.0/10).
type Filtro struct {
	esterno  atomic.Bool
	pubblico atomic.Bool // accesso da Internet tramite Tailscale Funnel (porta pubblica)
	vpn      []netip.Prefix
	// Casa restituisce l'indirizzo di Gaster Bless sulla rete di casa, per la pagina «accesso chiuso» (può essere nil).
	Casa func() string
	// IntestazioneIP: intestazione con l'indirizzo del dispositivo messa dal servizio che porta Internet alla porta
	// pubblica ("" = X-Forwarded-For, come Tailscale Funnel; "Cf-Connecting-Ip" per Cloudflare Tunnel). Sulla porta
	// pubblica arrivano solo richieste dal PC stesso (il programma del tunnel), quindi l'intestazione è affidabile.
	IntestazioneIP string
}

// ImpostaPubblico apre o chiude l'accesso da Internet (porta pubblica di Tailscale Funnel).
func (f *Filtro) ImpostaPubblico(on bool) { f.pubblico.Store(on) }

// Pubblico dice se l'accesso da Internet è aperto.
func (f *Filtro) Pubblico() bool { return f.pubblico.Load() }

type chiavePubblica struct{}

// Pubblica dice se la richiesta è arrivata dalla porta pubblica (cioè da Internet, tramite Funnel).
func Pubblica(r *http.Request) bool { v, _ := r.Context().Value(chiavePubblica{}).(bool); return v }

// IndirizzoSconosciuto: si usa per le richieste da Internet senza un indirizzo affidabile del dispositivo
// (blocco dei tentativi comune a tutte). È un indirizzo "di documentazione": né locale né privato né VPN.
var IndirizzoSconosciuto = netip.MustParseAddr("192.0.2.1")

// PortaPubblica prepara le richieste che Tailscale Funnel inoltra dalla rete Internet: non sono mai del PC
// né della rete di casa (l'indirizzo del dispositivo viene da X-Forwarded-For, e se manca o è locale/privato
// si usa IndirizzoSconosciuto); l'host è il nome pubblico, così i controlli sull'origine funzionano.
func (f *Filtro) PortaPubblica(next http.Handler, nome string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IPRichiesta(r).IsLoopback() { // la porta ascolta solo su 127.0.0.1: per sicurezza si ricontrolla
			http.Error(w, "Accesso non consentito.", http.StatusForbidden)
			return
		}
		ip := IndirizzoSconosciuto
		xff := r.Header.Get("X-Forwarded-For")
		if f.IntestazioneIP != "" && r.Header.Get(f.IntestazioneIP) != "" {
			xff = r.Header.Get(f.IntestazioneIP)
		}
		if xff != "" {
			parti := strings.Split(xff, ",")
			if a, err := netip.ParseAddr(strings.TrimSpace(parti[len(parti)-1])); err == nil {
				a = a.Unmap()
				if a.IsGlobalUnicast() && !a.IsPrivate() && !f.EsternaVPN(a) {
					ip = a
				}
			}
		}
		r2 := r.Clone(context.WithValue(r.Context(), chiavePubblica{}, true))
		r2.RemoteAddr = net.JoinHostPort(ip.String(), "0")
		r2.Host = nome
		if !f.pubblico.Load() {
			// Chi apre Gaster Bless dal browser o dall'icona sulla schermata Home vede una pagina chiara; l'API e le
			// app esterne (come una scheda esterna) ricevono il JSON con il codice.
			if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") && !strings.HasPrefix(r.URL.Path, "/api/") {
				casa := ""
				if f.Casa != nil {
					casa = f.Casa()
				}
				paginaChiusa(w, casa)
				return
			}
			// JSON con un codice, come gli errori dell'API (le app esterne, come una scheda esterna, lo riconoscono)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"errore":"Gaster Bless in questo momento non è raggiungibile da Internet: chiedi al Master di aprire l'accesso.","codice":"accesso_chiuso"}` + "\n"))
			return
		}
		next.ServeHTTP(w, r2)
	})
}

// NuovoFiltro crea il filtro. vpn sono le reti accettate quando l'accesso esterno è attivo.
func NuovoFiltro(vpn []string, esternoAttivo bool) *Filtro {
	f := &Filtro{}
	for _, s := range vpn {
		if p, err := netip.ParsePrefix(strings.TrimSpace(s)); err == nil {
			f.vpn = append(f.vpn, p)
		}
	}
	f.esterno.Store(esternoAttivo)
	return f
}

// ImpostaEsterno attiva o disattiva l'accesso dalla VPN.
func (f *Filtro) ImpostaEsterno(on bool) { f.esterno.Store(on) }

// Esterno dice se l'accesso dalla VPN è attivo.
func (f *Filtro) Esterno() bool { return f.esterno.Load() }

// IPRichiesta estrae l'indirizzo del dispositivo (senza fidarsi di intestazioni come X-Forwarded-For).
func IPRichiesta(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

// Locale dice se la richiesta arriva dal PC stesso.
func Locale(r *http.Request) bool { return IPRichiesta(r).IsLoopback() }

// EsternaVPN dice se l'indirizzo appartiene a una rete VPN configurata.
func (f *Filtro) EsternaVPN(a netip.Addr) bool {
	for _, p := range f.vpn {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Ammesso applica le regole della modalità corrente.
func (f *Filtro) Ammesso(a netip.Addr) bool {
	if !a.IsValid() {
		return false
	}
	if a.IsLoopback() {
		return true
	}
	if f.EsternaVPN(a) {
		return f.esterno.Load()
	}
	return a.IsPrivate() || (a.Is6() && a.IsLinkLocalUnicast())
}

// Middleware rifiuta le richieste da indirizzi non ammessi.
func (f *Filtro) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if Pubblica(r) { // già controllata da PortaPubblica
			if !f.pubblico.Load() {
				http.Error(w, "Accesso non consentito da questa rete.", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if !f.Ammesso(IPRichiesta(r)) {
			http.Error(w, "Accesso non consentito da questa rete.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CSP: nessuno script in linea, nessuna risorsa esterna. "blob:" serve per i PDF e le
// anteprime delle immagini; "data:" per le immagini generate (QR, ritratti ridotti).
const CSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; " +
	"media-src 'self' blob:; font-src 'self'; connect-src 'self'; worker-src 'self' blob:; " +
	"frame-src 'self' blob:; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'"

// CSPScheda vale solo per la scheda 5e (/scheda/) e per la mappa (/carta/): ammette gli stili scritti
// nell'HTML, che le pagine originali usano ovunque. Gli script restano ammessi solo da file del server.
const CSPScheda = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; " +
	"font-src 'self'; connect-src 'self'; worker-src 'self' blob:; frame-src 'none'; object-src 'none'; " +
	"base-uri 'none'; form-action 'none'; frame-ancestors 'self'"

// CSPRemoto vale solo per la pagina del gioco da remoto (/remoto/): come CSP, ma può collegarsi
// anche ai server MQTT indicati in configurazione.
func CSPRemoto(broker []string) string {
	c := "'self'"
	for _, b := range broker {
		if u, err := url.Parse(b); err == nil && (u.Scheme == "wss" || u.Scheme == "ws") && u.Host != "" {
			c += " " + u.Scheme + "://" + u.Host
		}
	}
	return strings.Replace(CSP, "connect-src 'self'", "connect-src "+c, 1)
}

// Intestazioni aggiunge le intestazioni di sicurezza a ogni risposta HTTPS. broker sono i server
// MQTT del gioco da remoto (ammessi solo dalla pagina /remoto/).
func Intestazioni(next http.Handler, broker []string) http.Handler {
	cspRemoto := CSPRemoto(broker)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if strings.HasPrefix(r.URL.Path, "/scheda/") || strings.HasPrefix(r.URL.Path, "/carta/") {
			h.Set("Content-Security-Policy", CSPScheda)
		} else if strings.HasPrefix(r.URL.Path, "/remoto/") {
			h.Set("Content-Security-Policy", cspRemoto)
		} else {
			h.Set("Content-Security-Policy", CSP)
		}
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		h.Set("Strict-Transport-Security", "max-age=31536000")
		next.ServeHTTP(w, r)
	})
}
