// Package server contiene il server HTTPS di Gaster Bless: API, canale in tempo reale,
// controllo dei permessi e servizio delle pagine.
package server

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gasterbless/internal/config"
	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
	"gasterbless/internal/rete"
)

// Server raccoglie tutto lo stato condiviso dell'applicazione.
type Server struct {
	visioni    visioniDocumenti        // documenti mostrati ad altri giocatori (documenti_mostra.go)
	scene      scenePNG                // PNG in scena per gruppo (scena_png.go)
	scarichi   scaricamentiRegolamenti // download della RPG RULEBOOK LIBRARY (regolamenti_scarica.go)
	ipCasa     ipDiCasa                // indirizzo Internet di casa, con Cloudflare Tunnel (cloudflare.go)
	avviatoIl  time.Time               // ora di avvio: fa parte della versione vista dalle pagine
	fileApp    cacheStatiche           // impronte e versioni compresse dei file dell'app (statiche_veloci.go)
	reteStrade reteStrade              // percorsi della mappa (mappa_strade.go)
	indeb      livelliIndebolimento
	// ultimo livello di indebolimento noto per personaggio (indebolimento.go)
	Cfg    config.Config
	DB     *sql.DB
	Chiavi *cripto.Chiavi
	Cert   *rete.Certificati
	Filtro *rete.Filtro
	Hub    *Hub
	Web    fs.FS // pagine e risorse statiche (incorporate nell'eseguibile)
	// DopoInGioco viene chiamata quando cambiano PF o condizioni di un PG (aggiorna il tracker).
	DopoInGioco func(personaggio string)

	muLimite sync.Mutex // serializza gli aggiornamenti dei tentativi di accesso
	muMusica sync.Once  // crea lo stato della musica del tavolo
	musica   *musica
	remoto   *ponteMQTT // gioco da remoto tramite server MQTT pubblici
	chiudi   chan struct{}
	agganci  codaAgganci // agganci per l'antagonista scritti dall'IA, in sottofondo (agganci_ia.go)
	// opPianificate: operazioni pianificate di Windows per il tasto «Aggiorna» (aggiornamento.go); nil = quelle vere.
	opPianificate  operazioniPianificate
	prepMedia      statoPrepMedia // preparazione automatica delle versioni leggere dei media (media_preparazione.go)
	bib            biblioteca     // testi dell'assistente IA e indice di ricerca (biblioteca.go)
	pacchettiCache cacheContenuti // pacchetti di contenuti attivi uniti ai dati della scheda (contenuti.go)
}

// Nuovo prepara cartelle, chiave dei dati, database e certificati.
func Nuovo(cfg config.Config, web fs.FS) (*Server, error) {
	if err := os.MkdirAll(cfg.CartellaDati, 0o700); err != nil {
		return nil, fmt.Errorf("cartella dati %s: %w", cfg.CartellaDati, err)
	}
	for _, sub := range []string{"backup", "file", "certificati", "chiave"} {
		if err := os.MkdirAll(filepath.Join(cfg.CartellaDati, sub), 0o700); err != nil {
			return nil, err
		}
	}
	k, err := cripto.CaricaChiavePrincipale(filepath.Join(cfg.CartellaDati, "chiave", "gasterbless.chiave"), !cfg.ModalitaProva)
	if err != nil {
		return nil, err
	}
	ch, err := cripto.NuoveChiavi(k)
	if err != nil {
		return nil, err
	}
	d, err := db.Apri(filepath.Join(cfg.CartellaDati, "gasterbless.db"))
	if err != nil {
		return nil, err
	}
	cert, err := rete.CaricaCertificati(filepath.Join(cfg.CartellaDati, "certificati"))
	if err != nil {
		d.Close()
		return nil, err
	}
	if len(cfg.NomiCertificato) > 0 { // per esempio il nome Tailscale del PC
		cert.ImpostaNomiExtra(cfg.NomiCertificato)
		if err := cert.Aggiorna(); err != nil {
			d.Close()
			return nil, err
		}
	}
	s := &Server{avviatoIl: time.Now(),
		Cfg: cfg, DB: d, Chiavi: ch, Cert: cert, Web: web,
		Filtro: rete.NuovoFiltro(cfg.RetiVPN, false),
		chiudi: make(chan struct{}),
	}
	s.Hub = NuovoHub(s)
	s.Filtro.Casa = s.indirizzoCasa // per la pagina «accesso da Internet chiuso»
	s.remoto = nuovoPonte(s)
	// L'accesso esterno parte sempre come nel file di configurazione, salvo che il Master
	// lo abbia chiuso dal pannello: in quel caso resta chiuso.
	esterno := cfg.AccessoEsterno == "vpn"
	if v, _ := s.impostazione("accesso_esterno"); v == "chiuso" {
		esterno = false
	}
	s.Filtro.ImpostaEsterno(esterno)
	// L'accesso da Internet (Tailscale Funnel) parte chiuso; resta aperto dopo un riavvio solo se il Master
	// l'aveva aperto e la porta pubblica è configurata.
	if cfg.ServizioPubblico == "cloudflare" {
		s.Filtro.IntestazioneIP = "Cf-Connecting-Ip" // indirizzo del dispositivo messo da Cloudflare (cloudflare.go)
	}
	if v, _ := s.impostazione("accesso_pubblico"); v == "aperto" && cfg.PortaPubblica > 0 {
		s.Filtro.ImpostaPubblico(true)
	}
	s.caricaIndebolimento()
	return s, nil
}

// Chiudi libera le risorse.
func (s *Server) Chiudi() error {
	select {
	case <-s.chiudi:
	default:
		close(s.chiudi)
		s.remoto.chiudi()
	}
	return s.DB.Close()
}

// Handler costruisce l'albero delle rotte HTTPS completo di middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.rotteAccesso(mux)
	s.rotteUtenti(mux)
	s.rotteGruppi(mux)
	s.rotteSistema(mux)
	s.rottePersonaggi(mux)
	s.rotteIniziativa(mux)
	s.rotteMappa(mux)
	s.rotteBackup(mux)
	s.rotteAvvisi(mux)
	s.rotteRemoto(mux)
	s.rotteRiassunti(mux) // portato dal ramo main-ke8sy4
	s.rotteSchedePlugin(mux)
	s.rotteManuali(mux)            // portato dal ramo main-ke8sy4
	s.rotteSanita(mux)             // portato dal ramo main-ke8sy4
	s.rotteBackgroundTesto(mux)    // background in testo con l'analisi per il Master (2026-11-09)
	s.rotteAssistente(mux)         // assistente IA del Master (2026-11-09)
	s.rotteEffettiPersistenti(mux) // effetti che durano dopo gli scontri (2026-11-12)
	s.rotteAgganciIA(mux)          // agganci per l'antagonista scritti dall'IA nel Background (2026-11-09)
	s.rotteViaggi(mux)             // viaggi del segnalino del gruppo (2026-11-08)
	s.rotteDocumenti(mux)          // documenti dei giocatori e del Master (2026-11-07)
	s.rotteControlloSchede(mux)    // controllo delle schede all'accesso in sessione (2026-11-07)
	s.rotteFinanze(mux)            // monete: richieste, acquisti, trasferimenti, ricompense (2026-11-07)
	s.rotteModificheManuali(mux)   // «Modifica manuale» della scheda con approvazione del Master (2026-11-11)
	s.rotteImportazione(mux)       // «📥 Importa dai manuali» (2026-11-11)
	s.rotteOggettiMagici(mux)      // archivio degli oggetti magici del Master (2026-11-11)
	s.rotteCampagna(mux)           // file unico della campagna (2026-11-11)
	s.rottePagineManuali(mux)      // pagine dei manuali pronte per i telefoni (2026-11-11)
	s.rottePreparazioneMedia(mux)  // video per i telefoni preparati in sottofondo con ffmpeg (2026-11-11)
	s.rotteAggiornamento(mux)      // tasto «Aggiorna» del Master: aggiornamento da GitHub con le operazioni pianificate (2026-11-11)
	s.rotteRiposi(mux)             // riposo lungo e breve dalla barra in alto, con approvazione del Master (2026-11-11)
	s.rotteProfessioni(mux)        // raccolta e fabbricazione (2026-11-07)
	s.rotteGuida(mux)              // «Scarica Guida» nella barra in alto (2026-11-12)
	s.rotteScenaPNG(mux)           // PNG in scena sugli schermi (2026-11-12)
	s.rotteAspettoDadi(mux)        // colore dei dadi 3D (2026-11-12)
	s.rottePreparazione(mux)       // preparazione della sessione (fase L5, 2026-11-12)
	s.rotteBilanciamento(mux)      // bilanciamento delle avventure (2026-11-12)
	s.rotteLore(mux)               // Codice della campagna: la lore del Master (2026-11-12)
	s.rotteAnalisiRegole(mux)      // analisi dei regolamenti: proposte da approvare (L5)
	s.rotteCampagne(mux)           // campagne: sistema, regolamento, documenti, gruppi (L4)
	s.rotteScaricaRegolamenti(mux) // download dei documenti consentiti (L2)
	s.rotteMieiRegolamenti(mux)    // MY RULEBOOKS
	s.rotteRegolamenti(mux)        // RPG RULEBOOK LIBRARY e MY RULEBOOKS (docs/RULEBOOK_LIBRARY.md, 2026-11-13)
	s.rotteGioco(mux)              // interfaccia adattiva: campagna e gioco attivi (fase 3c, C4a)
	s.rotteContenuti(mux)          // pacchetti di contenuti importati dall'utente (fase 3c, C1)
	s.rotteBiblioteca(mux)         // biblioteca dei testi per le due identità dell'assistente IA (2026-11-13)
	mux.Handle("GET /ws", http.HandlerFunc(s.Hub.Gestisci))
	mux.Handle("/api/", api(func(http.ResponseWriter, *http.Request) (any, error) { return nil, errNonTrovato }))
	mux.Handle("/", s.statiche())

	var h http.Handler = mux
	h = s.conUtente(h)                         // legge la sessione (se c'è) e la mette nel contesto
	h = s.controlloOrigine(h)                  // anti-CSRF per le richieste che modificano dati
	h = rete.Intestazioni(h, s.Cfg.BrokerMQTT) // CSP, HSTS, nosniff...
	h = s.Filtro.Middleware(h)                 // rete di casa / VPN
	return recupero(h)
}

// recupero evita che un errore imprevisto in un gestore fermi il server.
func recupero(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("errore grave in una richiesta", "percorso", r.URL.Path, "errore", v)
				http.Error(w, "Errore interno.", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statiche serve le pagine dell'app. Le risorse con versione restano in cache a lungo.
func (s *Server) statiche() http.Handler {
	fsrv := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.FileServerFS(s.Web).ServeHTTP(w, r) })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Metodo non consentito.", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		// Il bestiario contiene nomi e ruoli dei PNG dell'avventura (spoiler): solo per il Master.
		if (p == "/carta/bestiario.json" || p == "/carta/mostri.json") && !utenteDa(r.Context()).Master() {
			http.Error(w, "Non trovato.", http.StatusNotFound)
			return
		}
		// Mappa completa e rete stradale: solo al Master. I giocatori ricevono l'immagine coperta dalla nebbia
		// del loro gruppo (/api/v1/mappa/immagine, mappa_immagine.go) e i percorsi dal server.
		if p == "/carta/img/gasterbless.webp" || p == "/carta/img/mask.png" {
			if u := utenteDa(r.Context()); u == nil || !u.Master() {
				http.Error(w, "Non trovato.", http.StatusNotFound)
				return
			}
		}
		// Mappe locali (spoiler, punto S10 della Fase 1): a un giocatore solo se svelate al suo gruppo.
		if strings.HasPrefix(p, "/carta/img/lm/") && !s.mappaLocaleVisibile(utenteDa(r.Context()), strings.TrimSuffix(strings.TrimPrefix(p, "/carta/img/lm/"), ".webp")) {
			http.Error(w, "Non trovato.", http.StatusNotFound)
			return
		}
		// Pagine, script e stili si ricontrollano sempre (dopo un aggiornamento si vede subito la versione
		// nuova); solo i file pesanti che non cambiano restano in memoria nel browser.
		if strings.HasPrefix(p, "/vendor/") || strings.HasPrefix(p, "/font/") || strings.HasPrefix(p, "/img/") ||
			strings.HasPrefix(p, "/suoni/") || strings.HasPrefix(p, "/carta/img/") && !strings.HasPrefix(p, "/carta/img/lm/") {
			// «private»: solo il browser tiene la copia. Con «public» un servizio come Cloudflare potrebbe conservarla e
			// darla a chiunque, anche file solo per il Master come la mappa completa (2026-11-08).
			w.Header().Set("Cache-Control", "private, max-age=3600")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		// Le sezioni dell'app (es. /mappa, /schede) sono gestite dalla pagina principale.
		if _, err := fs.Stat(s.Web, trimSlash(p)); err != nil && p != "/" {
			if filepath.Ext(p) == "" {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				s.serviStatico(w, r2, fsrv, ".")
				return
			}
		}
		if s.serviContenuti(w, r, trimSlash(p)) { // dati della scheda con i pacchetti di contenuti attivi (contenuti.go)
			return
		}
		s.serviStatico(w, r, fsrv, trimSlash(p)) // impronta (304) e compressione: statiche_veloci.go
	})
}

func trimSlash(p string) string {
	for len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	for len(p) > 0 && p[len(p)-1] == '/' { // "scheda/" → "scheda" (i nomi di fs.FS non finiscono con /)
		p = p[:len(p)-1]
	}
	if p == "" {
		return "."
	}
	return p
}

// Avvia mette in ascolto il server HTTPS e la pagina di benvenuto HTTP. Si ferma quando ctx termina.
//
// Standby del PC (segnalato dall'utente il 2026-11-08): al risveglio Windows può chiudere le porte in ascolto e il server
// si fermava, senza che Windows lo riavviasse. Ora ogni porta, se si interrompe, viene riaperta da sola (dopo 1 s, poi
// fino a 30 s tra un tentativo e l'altro); solo la prima apertura della porta dell'app, se fallisce, ferma l'avvio
// (per esempio porta già occupata da un altro programma).
func (s *Server) Avvia(ctx context.Context) error {
	addr := net.JoinHostPort(s.Cfg.Indirizzo, strconv.Itoa(s.Cfg.PortaHTTPS))
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	var mu sync.Mutex
	attivi := map[string]*http.Server{}
	var gruppo sync.WaitGroup
	servi := func(nome string, primo net.Listener, crea func() *http.Server, ascolta func() (net.Listener, error), conTLS bool, avvisoPrimo func(error)) {
		gruppo.Add(1)
		go func() {
			defer gruppo.Done()
			serviConRipresa(ctx, nome, primo, func() *http.Server {
				srv := crea()
				mu.Lock()
				attivi[nome] = srv
				mu.Unlock()
				return srv
			}, ascolta, conTLS, avvisoPrimo, time.Second)
		}()
	}
	log := slog.NewLogLogger(slog.Default().Handler(), slog.LevelDebug)
	// I gestori si costruiscono una volta sola, qui: ricostruirli a ogni riapertura (e in parallelo per più porte)
	// registrerebbe due volte i messaggi in tempo reale.
	gestoreApp := s.Handler()
	var gestorePubblico http.Handler
	if s.Cfg.PortaPubblica > 0 {
		gestorePubblico = s.Filtro.PortaPubblica(s.Handler(), s.Cfg.NomePubblico)
	}
	slog.Info("Gaster Bless in ascolto", "indirizzo", "https://"+addr)
	servi("app", l, func() *http.Server {
		return &http.Server{Addr: addr, Handler: gestoreApp, TLSConfig: s.Cert.ConfigTLS(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second, ErrorLog: log}
	}, func() (net.Listener, error) { return net.Listen("tcp", addr) }, true, nil)
	if s.Cfg.PortaBenvenuto > 0 {
		// Se la porta è occupata da un altro programma si perde solo la pagina del certificato:
		// l'app resta accesa e lo dice nel registro.
		ab := net.JoinHostPort(s.Cfg.Indirizzo, strconv.Itoa(s.Cfg.PortaBenvenuto))
		servi("benvenuto", nil, func() *http.Server {
			return &http.Server{Addr: ab, Handler: recupero(s.Filtro.Middleware(s.benvenuto())), ReadHeaderTimeout: 10 * time.Second}
		}, func() (net.Listener, error) { return net.Listen("tcp", ab) }, false, func(err error) {
			slog.Warn("pagina del certificato non disponibile: porta occupata da un altro programma (cambia porta_benvenuto in gasterbless.conf)",
				"porta", s.Cfg.PortaBenvenuto, "errore", err)
		})
	}
	if s.Cfg.PortaPubblica > 0 {
		// Porta per Tailscale Funnel: solo sul PC stesso (127.0.0.1), HTTP semplice perché la cifratura
		// verso Internet la fa Tailscale. Tutto ciò che arriva qui è trattato come Internet.
		ap := net.JoinHostPort("127.0.0.1", strconv.Itoa(s.Cfg.PortaPubblica))
		slog.Info("porta pubblica per l'accesso da Internet", "servizio", s.Cfg.ServizioPubblico, "indirizzo", "http://"+ap, "nome", "https://"+s.Cfg.NomePubblico)
		servi("pubblica", nil, func() *http.Server {
			return &http.Server{Addr: ap, Handler: gestorePubblico, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second, ErrorLog: log}
		}, func() (net.Listener, error) { return net.Listen("tcp", ap) }, false, func(err error) {
			slog.Warn("porta pubblica non disponibile (cambia porta_pubblica in gasterbless.conf)", "errore", err)
		})
	}
	go s.lavoriPeriodici(ctx)
	go s.preparazioneMedia(ctx) // versioni leggere dei media in sottofondo, se c'è ffmpeg (media_preparazione.go)
	if s.conPortaPubblica() {
		go s.seguiIPCasa(ctx) // chi usa l'indirizzo Internet al tavolo resta «al tavolo» (cloudflare.go)
	}
	s.remoto.riattiva()

	<-ctx.Done()
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Hub.ChiudiTutti()
	mu.Lock()
	for _, srv := range attivi {
		srv.Shutdown(sctx)
	}
	mu.Unlock()
	fatto := make(chan struct{})
	go func() { gruppo.Wait(); close(fatto) }()
	select {
	case <-fatto:
	case <-sctx.Done():
	}
	return nil
}

// serviConRipresa serve una porta e, se si interrompe (per esempio al risveglio dallo standby), la riapre:
// dopo «attesa», poi raddoppiando fino a 30 secondi. Torna quando ctx termina o il server viene chiuso.
func serviConRipresa(ctx context.Context, nome string, primo net.Listener, crea func() *http.Server, ascolta func() (net.Listener, error),
	conTLS bool, avvisoPrimo func(error), attesa time.Duration) {
	iniziale := attesa
	for ctx.Err() == nil {
		li := primo
		primo = nil
		if li == nil {
			var err error
			if li, err = ascolta(); err != nil {
				if avvisoPrimo != nil {
					avvisoPrimo(err) // porta occupata: lo si dice una volta sola e si continua a riprovare
					avvisoPrimo = nil
				}
				li = nil
			}
		}
		if li != nil {
			srv := crea()
			if conTLS {
				li = tls.NewListener(li, srv.TLSConfig)
			}
			inizio := time.Now()
			err := srv.Serve(li)
			if ctx.Err() != nil || errors.Is(err, http.ErrServerClosed) {
				return
			}
			slog.Warn("porta interrotta (per esempio dopo lo standby del PC): la riapro", "porta", nome, "errore", err)
			if time.Since(inizio) > time.Minute {
				attesa = iniziale
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(attesa):
		}
		attesa = min(attesa*2, 30*time.Second)
	}
}

// lavoriPeriodici: rinnovo del certificato se cambiano gli indirizzi, pulizia delle sessioni scadute.
func (s *Server) lavoriPeriodici(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.lavoriDelMinuto(ctx)
		}
	}
}

// lavoriDelMinuto: un errore imprevisto (panic) in uno dei lavori finisce nel registro e non spegne Gaster Bless.
func (s *Server) lavoriDelMinuto(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("errore imprevisto nei lavori periodici", "errore", r)
		}
	}()
	if err := s.Cert.Aggiorna(); err != nil {
		slog.Warn("rinnovo del certificato non riuscito", "errore", err)
	}
	s.DB.Exec(`DELETE FROM sessioni_accesso WHERE scade_il < ?`, db.Ora())
	s.DB.Exec(`DELETE FROM tentativi_accesso WHERE bloccato_fino < ? AND aggiornato_il < ?`, db.Ora(), db.Ora()-86400)
	s.backupAutomatico(ctx)
}

// impostazione legge un valore dalla tabella impostazioni.
func (s *Server) impostazione(k string) (string, error) {
	var v string
	err := s.DB.QueryRow(`SELECT valore FROM impostazioni WHERE chiave = ?`, k).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Server) impostaImpostazione(k, v string) error {
	_, err := s.DB.Exec(`INSERT INTO impostazioni(chiave, valore) VALUES(?, ?) ON CONFLICT(chiave) DO UPDATE SET valore = excluded.valore`, k, v)
	return err
}

// evento scrive nel registro degli eventi di sistema (mai dati personali).
func (s *Server) evento(tipo, utente, dettaglio string) {
	if _, err := s.DB.Exec(`INSERT INTO registro_eventi(tipo, utente_id, dettaglio, creato_il) VALUES(?,?,?,?)`,
		tipo, nullo(utente), dettaglio, db.Ora()); err != nil {
		slog.Warn("registro eventi", "errore", err)
	}
}

func nullo(s string) any {
	if s == "" {
		return nil
	}
	return s
}
