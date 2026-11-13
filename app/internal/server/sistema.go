package server

import (
	"bytes"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"rsc.io/qr"

	"gasterbless/internal/rete"
)

func (s *Server) rotteSistema(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/impostazioni", api(s.apiImpostazioni))
	mux.Handle("PATCH /api/v1/impostazioni", api(s.apiModificaImpostazioni))
	mux.Handle("POST /api/v1/accesso-esterno/chiudi", api(s.apiChiudiEsterno))
	mux.Handle("POST /api/v1/accesso-esterno/apri", api(s.apiApriEsterno))
	mux.Handle("POST /api/v1/accesso-pubblico/apri", api(s.apiApriPubblico))
	mux.Handle("GET /api/v1/qr.png", http.HandlerFunc(s.qrPNG))
	mux.Handle("GET /api/v1/indirizzi", api(s.apiIndirizzi))
}

func ipStringa(r *http.Request) string { return rete.IPRichiesta(r).String() }

func (s *Server) apiImpostazioni(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	senza, _ := s.impostazione("registrazione_senza_invito")
	return map[string]any{
		"accessoEsterno":           s.Filtro.Esterno(),
		"accessoEsternoConfig":     s.Cfg.AccessoEsterno,
		"accessoPubblico":          s.Filtro.Pubblico(),
		"pubblicoConfigurato":      s.Cfg.PortaPubblica > 0,
		"nomePubblico":             s.Cfg.NomePubblico,
		"servizioPubblico":         s.Cfg.ServizioPubblico, // tailscale o cloudflare
		"pannelloSoloPC":           s.Cfg.PannelloSoloPC,
		"registrazioneSenzaInvito": senza != "no",
		"backupGiorni":             s.Cfg.BackupGiorni,
		"inattiviMesi":             s.Cfg.InattiviMesi,
		"totpAttivo":               u.haTOTP,
		"impronta":                 s.Cert.ImprontaAutorita(),
		"locale":                   u.locale,
		"approvazioneSchede":       s.approvazioneSchede(),
	}, nil
}

func (s *Server) apiModificaImpostazioni(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	var in struct {
		RegistrazioneSenzaInvito *bool
		ApprovazioneSchede       *bool // «Modifica manuale» delle schede con approvazione del Master (2026-11-11)
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.RegistrazioneSenzaInvito != nil {
		v := "si"
		if !*in.RegistrazioneSenzaInvito {
			v = "no"
		}
		s.impostaImpostazione("registrazione_senza_invito", v)
	}
	if in.ApprovazioneSchede != nil {
		v := "si"
		if !*in.ApprovazioneSchede {
			v = "no"
		}
		s.impostaImpostazione("approvazione_schede", v)
	}
	return s.apiImpostazioni(w, r)
}

// apiChiudiEsterno chiude subito l'accesso dalla VPN: rifiuta le nuove richieste e
// chiude le connessioni in tempo reale aperte da indirizzi esterni.
func (s *Server) apiChiudiEsterno(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	// Chiude tutto ciò che viene da fuori: VPN e Internet (Tailscale Funnel).
	s.Filtro.ImpostaEsterno(false)
	s.Filtro.ImpostaPubblico(false)
	s.impostaImpostazione("accesso_esterno", "chiuso")
	s.impostaImpostazione("accesso_pubblico", "chiuso")
	s.DB.Exec(`DELETE FROM sessioni_accesso WHERE ip NOT LIKE '192.168.%' AND ip NOT LIKE '10.%' AND ip NOT LIKE '127.%' AND ip NOT LIKE '172.%' AND ip <> '::1'`)
	s.Hub.ChiudiEsterni(s.Filtro)
	s.evento("sistema", m.ID, "accesso esterno chiuso")
	return map[string]bool{"accessoEsterno": false}, nil
}

func (s *Server) apiApriEsterno(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	if !m.locale {
		return nil, errore(http.StatusForbidden, "L'accesso esterno si riapre solo dal PC su cui gira Gaster Bless.")
	}
	if s.Cfg.AccessoEsterno != "vpn" {
		return nil, errore(http.StatusBadRequest, "L'accesso esterno non è configurato: segui la guida della VPN e imposta accesso_esterno = vpn.")
	}
	if !m.haTOTP {
		return nil, errore(http.StatusBadRequest, "Prima attiva il secondo fattore (TOTP) per l'account del Master.")
	}
	s.Filtro.ImpostaEsterno(true)
	s.impostaImpostazione("accesso_esterno", "aperto")
	s.evento("sistema", m.ID, "accesso esterno aperto")
	return map[string]bool{"accessoEsterno": true}, nil
}

// apiApriPubblico apre l'accesso da Internet (Tailscale Funnel): solo dal PC di Gaster Bless, con il secondo fattore.
func (s *Server) apiApriPubblico(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	if !m.locale {
		return nil, errore(http.StatusForbidden, "L'accesso da Internet si apre solo dal PC su cui gira Gaster Bless.")
	}
	if s.Cfg.PortaPubblica == 0 {
		return nil, errore(http.StatusBadRequest, "L'accesso da Internet non è configurato: segui la guida (guide\\internet.md) e imposta porta_pubblica e nome_pubblico.")
	}
	if !m.haTOTP {
		return nil, errore(http.StatusBadRequest, "Prima attiva il secondo fattore (TOTP) per l'account del Master.")
	}
	s.Filtro.ImpostaPubblico(true)
	s.impostaImpostazione("accesso_pubblico", "aperto")
	s.evento("sistema", m.ID, "accesso da Internet aperto")
	return map[string]bool{"accessoPubblico": true}, nil
}

// qrPNG disegna un codice QR. Solo per il Master e solo per indirizzi di Gaster Bless o otpauth://,
// così non diventa un generatore di QR per contenuti arbitrari.
func (s *Server) qrPNG(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	if !u.Master() {
		http.Error(w, "Non consentito.", http.StatusForbidden)
		return
	}
	testo := r.URL.Query().Get("t")
	if len(testo) > 600 || !(regexp.MustCompile(`^(https?://[^\s]+|otpauth://totp/[^\s]+)$`).MatchString(testo)) {
		http.Error(w, "Testo non valido.", http.StatusBadRequest)
		return
	}
	c, err := qr.Encode(testo, qr.M)
	if err != nil {
		http.Error(w, "Testo troppo lungo.", http.StatusBadRequest)
		return
	}
	img := immagineQR(c, 8)
	var b bytes.Buffer
	png.Encode(&b, img)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b.Bytes())
}

// immagineQR disegna il codice ingrandito, con il margine bianco di 4 moduli richiesto dallo standard.
// (Code.Image di rsc.io/qr ignora l'ingrandimento e disegna il codice minuscolo in un angolo.)
func immagineQR(c *qr.Code, scala int) *image.Gray {
	lato := (c.Size + 8) * scala
	img := image.NewGray(image.Rect(0, 0, lato, lato))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	for y := 0; y < c.Size; y++ {
		for x := 0; x < c.Size; x++ {
			if !c.Black(x, y) {
				continue
			}
			for dy := 0; dy < scala; dy++ {
				riga := (4+y)*scala + dy
				for dx := 0; dx < scala; dx++ {
					img.Pix[riga*img.Stride+(4+x)*scala+dx] = 0
				}
			}
		}
	}
	return img
}

// apiIndirizzi restituisce gli indirizzi del PC da usare nei link d'invito.
// indirizzoCasa: il primo indirizzo HTTPS di Gaster Bless sulla rete di casa (non quello della VPN), o "".
func (s *Server) indirizzoCasa() string {
	for _, ip := range rete.IndirizziLocali() {
		if a, ok := netip.AddrFromSlice(ip.To4()); ok && !s.Filtro.EsternaVPN(a) {
			return "https://" + ip.String() + ":" + strconv.Itoa(s.Cfg.PortaHTTPS)
		}
	}
	return ""
}

// indirizzoPubblico: l'indirizzo Internet (Tailscale Funnel), se configurato.
func (s *Server) indirizzoPubblico() string {
	if s.Cfg.PortaPubblica > 0 && s.Cfg.NomePubblico != "" {
		return "https://" + s.Cfg.NomePubblico
	}
	return ""
}

func (s *Server) apiIndirizzi(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	// Gli indirizzi della VPN (per Tailscale 100.x) si mostrano a parte, con i nomi del certificato.
	out, vpn := []string{}, []string{}
	for _, ip := range rete.IndirizziLocali() {
		u := "https://" + ip.String() + ":" + strconv.Itoa(s.Cfg.PortaHTTPS)
		if a, ok := netip.AddrFromSlice(ip.To4()); ok && s.Filtro.EsternaVPN(a) {
			vpn = append(vpn, u)
		} else {
			out = append(out, u)
		}
	}
	for _, n := range s.Cfg.NomiCertificato {
		vpn = append(vpn, "https://"+n+":"+strconv.Itoa(s.Cfg.PortaHTTPS))
	}
	benv := ""
	if len(out) > 0 && s.Cfg.PortaBenvenuto > 0 {
		benv = "http://" + strings.TrimSuffix(strings.TrimPrefix(out[0], "https://"), ":"+strconv.Itoa(s.Cfg.PortaHTTPS)) + ":" + strconv.Itoa(s.Cfg.PortaBenvenuto)
	}
	pub := ""
	if s.Cfg.PortaPubblica > 0 {
		pub = "https://" + s.Cfg.NomePubblico
	}
	return map[string]any{"https": out, "vpn": vpn, "pubblico": pub, "benvenuto": benv}, nil
}

// ===== File nella cartella dati =====

var nomeFileSicuro = regexp.MustCompile(`^[a-z0-9]{16}\.[a-z0-9]{2,5}$`)

// percorsoFileDati restituisce il percorso di un file generato dal server (nome controllato:
// nessun percorso di cartella manipolabile).
func (s *Server) percorsoFileDati(nome string) (string, error) {
	if !nomeFileSicuro.MatchString(nome) {
		return "", fmt.Errorf("nome di file non valido")
	}
	return filepath.Join(s.Cfg.CartellaDati, "file", nome), nil
}

func (s *Server) eliminaFileDati(nome string) {
	if p, err := s.percorsoFileDati(nome); err == nil {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			slog.Warn("eliminazione file", "errore", err)
		}
	}
}

// ===== Pagina di benvenuto (HTTP, porta 7601): solo il certificato e le istruzioni =====

var paginaBenvenuto = template.Must(template.New("b").Parse(`<!doctype html>
<html lang="it"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Benvenuto in Gaster Bless</title>
<style>body{font:17px/1.5 Georgia,serif;background:#15120e;color:#e8dfcc;margin:0;padding:20px;max-width:640px;margin:auto}
h1{color:#d9a441;letter-spacing:4px}a.b{display:block;background:#8a1c1c;color:#fff;text-align:center;padding:14px;border-radius:8px;margin:12px 0;text-decoration:none;font-size:19px}
details{background:#221c15;border:1px solid #4a3d2b;border-radius:8px;padding:10px 14px;margin:10px 0}summary{cursor:pointer;color:#d9a441}
code{background:#000;padding:2px 6px;border-radius:4px;word-break:break-all}small{opacity:.8}
.sez-ios{display:none}body.ios .sez-ios{display:block}body.ios .non-ios{display:none}
ol.passi{padding-left:22px}ol.passi li{margin:10px 0}#esito{font-weight:bold;min-height:1.5em}#esito.ok{color:#8fd18f}#esito.no{color:#ff9b8a}</style></head><body{{if .IOS}} class="ios"{{end}}>
<h1>GASTER BLESS</h1>
<div class="sez-ios">
<h2>iPhone e iPad: prima il certificato</h2>
<p>Su iPhone e iPad Gaster Bless funziona solo dopo aver installato una volta il «certificato di casa». Senza, Safari apre la pagina ma non riceve gli avvisi, il combattimento e la Mappa in tempo reale. Bastano 2 minuti.</p>
<ol class="passi">
<li><b>Usa Safari</b> (non Chrome o altre app) e tocca <a class="b" href="/gasterbless-ca.crt">1. Scarica il certificato</a> Al messaggio «Questo sito web sta cercando di scaricare un profilo di configurazione» tocca <b>Consenti</b>, poi <b>Chiudi</b>.</li>
<li>Apri <b>Impostazioni</b>: in alto compare <b>Profilo scaricato</b> (se non c'è: <i>Generali → VPN e gestione dispositivi</i>). Tocca <b>Gaster Bless - autorità locale → Installa</b>, inserisci il codice del telefono, ancora <b>Installa</b>, poi <b>Fine</b>.</li>
<li>Sempre in Impostazioni: <i>Generali → Info</i>, scorri fino in fondo e apri la voce sull'<b>attendibilità dei certificati</b>. Attiva l'interruttore di <b>Gaster Bless - autorità locale</b> e conferma con <b>Continua</b>.</li>
<li>Torna qui su Safari e tocca <a class="b" id="controlla" href="{{.URL}}">2. Controlla ed entra in Gaster Bless</a></li>
</ol>
<p id="esito" role="status"></p>
{{if .Pubblico}}<p><b>Senza certificato:</b> il Master ha aperto l'indirizzo Internet di Gaster Bless, che funziona anche qui in casa e non chiede nulla da installare:</p>
<a class="b" href="{{.Pubblico}}">Entra da {{.PubblicoNome}}</a>{{end}}
<p><small>Poi, per avere l'icona: in Gaster Bless tocca Condividi → «Aggiungi alla schermata Home». Fuori casa, con l'indirizzo Internet di Gaster Bless, il certificato non serve.</small></p>
</div>
<div class="non-ios">
<a class="b" href="{{.URL}}">Entra in Gaster Bless</a>
<p>Il browser mostrerà l'avviso <b>«La connessione non è privata»</b>: tocca <b>Avanzate</b> e poi <b>Procedi</b> (o <i>Continua</i>). È normale: la connessione è comunque cifrata, solo che il telefono non conosce ancora il «certificato di casa» di Gaster Bless. Ogni tanto l'avviso ricompare: basta ripetere.</p>
<details><summary>Facoltativo: non vedere più l'avviso (installa il certificato una volta)</summary>
<p>Con il certificato installato l'avviso sparisce e Gaster Bless si può installare come app.</p>
<a class="b" href="/gasterbless-ca.crt">Scarica il certificato</a>
<p><b>Android</b>: apri il file scaricato oppure vai in <i>Impostazioni → Sicurezza → Crittografia e credenziali → Installa un certificato → Certificato CA</i>, scegli <i>gasterbless-ca.crt</i> e conferma.</p>
<p><b>iPhone e iPad</b>: apri questa pagina con Safari e consenti il download del profilo; <i>Impostazioni → Profilo scaricato → Installa</i>; poi <i>Impostazioni → Generali → Info → Impostazioni certificati</i>: attiva la fiducia completa per «Gaster Bless - autorità locale».</p>
<p><b>Windows</b>: apri il file, <i>Installa certificato → Utente corrente → Colloca tutti i certificati nel seguente archivio → Autorità di certificazione radice attendibili</i>.</p>
<p><b>Mac</b>: apri il file con Accesso Portachiavi, poi doppio clic su «Gaster Bless - autorità locale» → <i>Fidati → Sempre</i>.</p>
<p><small>Controllo a vista: l'impronta del certificato è<br><code>{{.Impronta}}</code><br>e deve coincidere con quella mostrata al Master nelle impostazioni.</small></p>
</details>
</div>
<script>
(function(){var u=navigator.userAgent,ios=/iPhone|iPad|iPod/.test(u)||(/Macintosh/.test(u)&&navigator.maxTouchPoints>1);
if(ios)document.body.classList.add("ios");
var b=document.getElementById("controlla"),e=document.getElementById("esito"),url={{.URL}},prova={{.Prova}};
b.addEventListener("click",function(ev){ev.preventDefault();e.className="";e.textContent="Controllo in corso…";
fetch(prova,{mode:"no-cors",cache:"no-store"}).then(function(){e.className="ok";e.textContent="✔ Certificato attivo: entro in Gaster Bless…";setTimeout(function(){location.href=url},700)},
function(){e.className="no";e.textContent="✗ Il certificato non è ancora attivo. Controlla i passi 2 e 3 (soprattutto l'interruttore in Generali → Info) e riprova."})})})();
</script>
</body></html>`))

func (s *Server) benvenuto() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /gasterbless-ca.crt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Header().Set("Content-Disposition", `attachment; filename="gasterbless-ca.crt"`)
		w.Write(s.Cert.DERAutorita())
	})
	mux.HandleFunc("GET /gasterbless-ca.pem", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Write(s.Cert.PEMAutorita())
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		h := r.Host
		if i := bytes.LastIndexByte([]byte(h), ':'); i > 0 {
			h = h[:i]
		}
		url := "https://" + h + ":" + strconv.Itoa(s.Cfg.PortaHTTPS) + "/"
		if q := r.URL.RawQuery; q != "" {
			url += "?" + q
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		origine := "https://" + h + ":" + strconv.Itoa(s.Cfg.PortaHTTPS)
		// Lo script controlla, da iPhone, che il certificato sia attivo (una richiesta all'app in HTTPS).
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src "+origine)
		ua := r.UserAgent()
		ios := strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") || strings.Contains(ua, "iPod")
		pubblico, pubNome := "", ""
		if s.Filtro.Pubblico() && s.indirizzoPubblico() != "" {
			pubblico, pubNome = s.indirizzoPubblico()+"/", s.Cfg.NomePubblico
			if q := r.URL.RawQuery; q != "" {
				pubblico += "?" + q
			}
		}
		paginaBenvenuto.Execute(w, map[string]any{"URL": url, "Prova": origine + "/manifest.webmanifest", "Impronta": s.Cert.ImprontaAutorita(), "IOS": ios,
			"Pubblico": pubblico, "PubblicoNome": pubNome})
	})
	return mux
}
