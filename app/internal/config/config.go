// Package config legge il file di configurazione di Gaster Bless.
//
// Il formato è volutamente semplice (una riga "chiave = valore", commenti con #),
// così il Master può modificarlo con il Blocco note senza librerie esterne.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config raccoglie tutte le impostazioni lette dal file.
type Config struct {
	CartellaDati   string // database, chiavi, backup, certificati
	CartellaMedia  string // Video, Musica, Manuali, Immagini (copiati dalla vecchia GasterBlessDati)
	PortaHTTPS     int    // porta dell'app
	PortaBenvenuto int    // porta HTTP solo per scaricare il certificato
	Indirizzo      string // interfaccia di ascolto ("" = tutte)
	AccessoEsterno string // "casa" (predefinito) oppure "vpn"
	RetiVPN        []string
	// NomiCertificato: nomi in più coperti dal certificato, per esempio il nome Tailscale del PC
	// (nome-pc.rete.ts.net), con cui i giocatori remoti raggiungono Gaster Bless.
	NomiCertificato []string
	// Accesso da Internet con Tailscale Funnel (spento se PortaPubblica = 0): Funnel inoltra le richieste a
	// http://127.0.0.1:PortaPubblica; NomePubblico è il nome del PC (nome-pc.rete.ts.net) che aprono i giocatori.
	PortaPubblica int
	NomePubblico  string
	// ServizioPubblico: chi porta Internet alla porta pubblica: "tailscale" (Funnel, predefinito) oppure "cloudflare"
	// (Cloudflare Tunnel, 2026-11-08: indirizzo del proprio dominio, va bene anche in casa senza certificato).
	ServizioPubblico string
	// OriginiEsterne: siti da cui La scheda esterna può mandare dati a /api/v1/esterne/arcana (CORS).
	OriginiEsterne []string
	PannelloSoloPC bool     // pannello del Master raggiungibile solo dal PC locale
	BackupGiorni   int      // giorni di conservazione dei backup
	BackupOra      int      // ora del backup automatico (0-23)
	InattiviMesi   int      // mesi dopo cui un account inattivo viene segnalato
	BrokerMQTT     []string // server MQTT pubblici per il gioco da remoto (indirizzi wss://)
	ModalitaProva  bool     // solo per i test automatici: dati temporanei, nessun DPAPI
	// ApprovazioneSchede (2026-11-11): i giocatori cambiano da soli solo i campi di gioco della scheda; il resto passa
	// da «Modifica manuale» e vale dopo l'approvazione del Master. Valore di partenza: il Master lo cambia nelle
	// Impostazioni (che hanno la precedenza). Le prove automatiche lo spengono (approvazione_schede = no).
	ApprovazioneSchede bool
	// Ffmpeg (2026-11-11): percorso di ffmpeg.exe per preparare le versioni leggere dei video e delle immagini.
	// Vuoto: Gaster Bless lo cerca in C:\Progetti\Strumenti\ffmpeg, in Program Files e nel PATH.
	Ffmpeg string
}

// Predefinita restituisce la configurazione usata se il file non specifica un valore.
func Predefinita() Config {
	prog := os.Getenv("ProgramData")
	if prog == "" {
		prog = os.TempDir()
	}
	return Config{
		CartellaDati:       filepath.Join(prog, "Gaster Bless"),
		CartellaMedia:      filepath.Join(prog, "Gaster Bless", "Media"),
		PortaHTTPS:         7700, // 7700 e 7701: non si scontrano con altre app del Master sulla 7600 e 7601
		PortaBenvenuto:     7701, // non 7680: è usata da Windows Update (Ottimizzazione recapito)
		AccessoEsterno:     "casa",
		ServizioPubblico:   "tailscale",
		RetiVPN:            []string{"100.64.0.0/10"},
		PannelloSoloPC:     false,
		BackupGiorni:       30,
		BackupOra:          4,
		InattiviMesi:       12,
		BrokerMQTT:         []string{"wss://broker.hivemq.com:8884/mqtt", "wss://broker.emqx.io:8084/mqtt"},
		OriginiEsterne:     nil,
		ApprovazioneSchede: true,
	}
}

// Leggi carica il file indicato. Se il file non esiste usa i valori predefiniti.
func Leggi(percorso string) (Config, error) {
	c := Predefinita()
	f, err := os.Open(percorso)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		riga := strings.TrimSpace(sc.Text())
		if riga == "" || strings.HasPrefix(riga, "#") {
			continue
		}
		k, v, ok := strings.Cut(riga, "=")
		if !ok {
			return c, fmt.Errorf("%s, riga %d: manca il segno =", percorso, n)
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		if i := strings.Index(v, " #"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		v = strings.Trim(v, `"`)
		if err := c.imposta(k, v); err != nil {
			return c, fmt.Errorf("%s, riga %d: %w", percorso, n, err)
		}
	}
	if err := sc.Err(); err != nil {
		return c, err
	}
	if c.PortaPubblica > 0 && c.NomePubblico == "" {
		return c, fmt.Errorf("%s: con porta_pubblica serve anche nome_pubblico (il nome .ts.net del PC o l'indirizzo del tunnel Cloudflare)", percorso)
	}
	if c.PortaPubblica > 0 && (c.PortaPubblica == c.PortaHTTPS || c.PortaPubblica == c.PortaBenvenuto) {
		return c, fmt.Errorf("%s: porta_pubblica deve essere diversa dalle altre porte di Gaster Bless", percorso)
	}
	return c, nil
}

func (c *Config) imposta(k, v string) error {
	intero := func(min, max int) (int, error) {
		n, err := strconv.Atoi(v)
		if err != nil || n < min || n > max {
			return 0, fmt.Errorf("%s deve essere un numero tra %d e %d", k, min, max)
		}
		return n, nil
	}
	booleano := func() (bool, error) {
		switch strings.ToLower(v) {
		case "si", "sì", "true", "1", "vero":
			return true, nil
		case "no", "false", "0", "falso":
			return false, nil
		}
		return false, fmt.Errorf("%s deve essere sì oppure no", k)
	}
	var err error
	switch k {
	case "cartella_dati":
		c.CartellaDati = os.ExpandEnv(v)
	case "cartella_media":
		c.CartellaMedia = os.ExpandEnv(v)
	case "ffmpeg":
		c.Ffmpeg = os.ExpandEnv(v)
	case "porta_https":
		c.PortaHTTPS, err = intero(1, 65535)
	case "porta_benvenuto":
		c.PortaBenvenuto, err = intero(0, 65535)
	case "indirizzo":
		c.Indirizzo = v
	case "accesso_esterno":
		if v != "casa" && v != "vpn" {
			return fmt.Errorf("accesso_esterno deve essere casa oppure vpn")
		}
		c.AccessoEsterno = v
	case "reti_vpn":
		c.RetiVPN = nil
		for _, r := range strings.Split(v, ",") {
			if r = strings.TrimSpace(r); r != "" {
				c.RetiVPN = append(c.RetiVPN, r)
			}
		}
	case "nomi_certificato":
		c.NomiCertificato = nil
		for _, n := range strings.Split(v, ",") {
			n = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(n), ".")))
			if n == "" {
				continue
			}
			if !nomeHostValido(n) {
				return fmt.Errorf("nomi_certificato: %q non è un nome valido (esempio: pc-master.tail1234.ts.net)", n)
			}
			c.NomiCertificato = append(c.NomiCertificato, n)
		}
		if len(c.NomiCertificato) > 10 {
			return fmt.Errorf("nomi_certificato: al massimo 10 nomi")
		}
	case "porta_pubblica":
		c.PortaPubblica, err = intero(0, 65535)
	case "servizio_pubblico":
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "tailscale" && v != "cloudflare" {
			return fmt.Errorf("servizio_pubblico: %q non valido (tailscale oppure cloudflare)", v)
		}
		c.ServizioPubblico = v
	case "nome_pubblico":
		v = strings.ToLower(strings.TrimSuffix(v, "."))
		if v != "" && !nomeHostValido(v) {
			return fmt.Errorf("nome_pubblico: %q non è un nome valido (esempio: pc-master.tail1234.ts.net oppure gasterbless.miodominio.it)", v)
		}
		c.NomePubblico = v
	case "origini_arcana":
		c.OriginiEsterne = nil
		for _, o := range strings.Split(v, ",") {
			o = strings.TrimSuffix(strings.TrimSpace(o), "/")
			if o == "" {
				continue
			}
			h, ok := strings.CutPrefix(o, "https://")
			if !ok || !nomeHostValido(strings.Split(h, ":")[0]) || strings.ContainsAny(h, "/?#") {
				return fmt.Errorf("origini_arcana: %q non è un sito valido (esempio: https://esempio.it)", o)
			}
			c.OriginiEsterne = append(c.OriginiEsterne, strings.ToLower(o))
		}
	case "pannello_solo_pc":
		c.PannelloSoloPC, err = booleano()
	case "backup_giorni":
		c.BackupGiorni, err = intero(1, 3650)
	case "backup_ora":
		c.BackupOra, err = intero(0, 23)
	case "inattivi_mesi":
		c.InattiviMesi, err = intero(1, 120)
	case "approvazione_schede":
		c.ApprovazioneSchede, err = booleano()
	case "broker_mqtt":
		c.BrokerMQTT = nil
		for _, b := range strings.Split(v, ",") {
			if b = strings.TrimSpace(b); b != "" {
				// ws:// (senza cifratura) solo verso il PC stesso: serve alle prove automatiche.
				locale := strings.HasPrefix(b, "ws://127.0.0.1:") || strings.HasPrefix(b, "ws://localhost:")
				if !(strings.HasPrefix(b, "wss://") || locale) || strings.ContainsAny(b, " ;'\"") {
					return fmt.Errorf("broker_mqtt: ogni indirizzo deve iniziare con wss://")
				}
				c.BrokerMQTT = append(c.BrokerMQTT, b)
			}
		}
	default:
		return fmt.Errorf("chiave sconosciuta: %s", k)
	}
	return err
}

// nomeHostValido: nome DNS fatto di etichette con lettere, cifre e trattini (niente indirizzi IP né asterischi).
func nomeHostValido(n string) bool {
	if len(n) > 253 || !strings.Contains(n, ".") {
		return false
	}
	soloCifre := true
	for _, e := range strings.Split(n, ".") {
		if e == "" || len(e) > 63 || strings.HasPrefix(e, "-") || strings.HasSuffix(e, "-") {
			return false
		}
		for _, r := range e {
			switch {
			case r >= 'a' && r <= 'z', r == '-':
				soloCifre = false
			case r >= '0' && r <= '9':
			default:
				return false
			}
		}
	}
	return !soloCifre
}
