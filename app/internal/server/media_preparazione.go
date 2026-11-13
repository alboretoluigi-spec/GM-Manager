package server

// Preparazione automatica dei media (fase M1, docs/PIANO_MEDIA_MANUALI.md, 2026-11-11). Al tavolo i video andavano a
// scatti: 5-6 telefoni scaricavano insieme, sullo stesso Wi-Fi, i video originali. Ora, se sul PC c'è ffmpeg, Gaster Bless
// prepara da sola in sottofondo (un file alla volta, priorità bassa) la versione per i telefoni (720p, ~2 Mbit/s) e le
// copie leggere già previste da media_leggeri.go. Gli originali non si toccano. Riparte dopo un caricamento, dopo
// «Cerca nelle cartelle», da «Prepara ora» nelle Impostazioni e comunque ogni 30 minuti (file copiati a mano).

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type statoPrepMedia struct {
	mu        sync.Mutex
	ffmpeg    string
	attivo    bool
	corrente  string
	totale    int // copie da fare nel giro in corso
	fatte     int // fatte nel giro in corso
	pronte    int // già pronte e valide all'inizio del giro
	errori    []string
	ultimo    time.Time
	sveglia   chan struct{}
	unaVolta  sync.Once
	createTot int // create da quando Gaster Bless è partita
}

func (s *Server) prep() *statoPrepMedia {
	s.prepMedia.unaVolta.Do(func() { s.prepMedia.sveglia = make(chan struct{}, 1) })
	return &s.prepMedia
}

// svegliaMedia chiede un nuovo giro di preparazione (non blocca mai).
func (s *Server) svegliaMedia() {
	select {
	case s.prep().sveglia <- struct{}{}:
	default:
	}
}

// trovaFfmpeg: l'impostazione «ffmpeg» di gasterbless.conf, poi le cartelle solite, poi il PATH. "" se non c'è.
// Non si usano le installazioni di winget: stanno nel profilo dell'utente e il servizio non le può aprire.
func trovaFfmpeg(conf string) string {
	nome := "ffmpeg"
	if runtime.GOOS == "windows" {
		nome = "ffmpeg.exe"
	}
	if conf != "" {
		if fileEsiste(conf) {
			return conf
		}
		return ""
	}
	var dove []string
	if runtime.GOOS == "windows" {
		dove = []string{`C:\Progetti\Strumenti\ffmpeg`, filepath.Join(os.Getenv("ProgramFiles"), "ffmpeg")}
	}
	for _, d := range dove {
		trovato := ""
		filepath.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
			if err != nil || trovato != "" {
				return filepath.SkipDir
			}
			if rel, _ := filepath.Rel(d, p); e.IsDir() && strings.Count(rel, string(filepath.Separator)) >= 3 {
				return filepath.SkipDir // al massimo qualche livello: lo zip crea ffmpeg-x.y-essentials_build\bin
			}
			if !e.IsDir() && strings.EqualFold(e.Name(), nome) {
				trovato = p
			}
			return nil
		})
		if trovato != "" {
			return trovato
		}
	}
	if p, err := exec.LookPath(nome); err == nil {
		return p
	}
	return ""
}

// preparazioneMedia gira finché Gaster Bless è accesa.
func (s *Server) preparazioneMedia(ctx context.Context) {
	st := s.prep()
	attesa := 2 * time.Minute // all'avvio si lascia respirare Gaster Bless
	for {
		select {
		case <-ctx.Done():
			return
		case <-st.sveglia:
		case <-time.After(attesa):
		}
		ff := trovaFfmpeg(s.Cfg.Ffmpeg)
		st.mu.Lock()
		st.ffmpeg = ff
		st.mu.Unlock()
		if ff == "" || s.Cfg.CartellaMedia == "" {
			attesa = 10 * time.Minute // forse ffmpeg verrà installato
			continue
		}
		s.giroPreparazione(ctx, ff)
		attesa = 30 * time.Minute
	}
}

func (s *Server) giroPreparazione(ctx context.Context, ff string) {
	st := s.prep()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("errore imprevisto nella preparazione dei media", "errore", r)
		}
		st.mu.Lock()
		st.attivo, st.corrente, st.ultimo = false, "", time.Now()
		st.mu.Unlock()
	}()
	daFare, pronte := lavoriMedia(s.Cfg.CartellaMedia, leggiRegistro(s.Cfg.CartellaMedia))
	st.mu.Lock()
	st.attivo, st.totale, st.fatte, st.pronte, st.errori = len(daFare) > 0, len(daFare), 0, pronte, nil
	st.mu.Unlock()
	if len(daFare) == 0 {
		return
	}
	slog.Info("preparazione dei media", "da_fare", len(daFare), "ffmpeg", ff)
	es, err := ottimizza(ctx, s.Cfg.CartellaMedia, ff, func(fase, testo string) {
		st.mu.Lock()
		defer st.mu.Unlock()
		switch fase {
		case "inizio":
			st.corrente = strings.TrimSuffix(testo, "#telefono")
		case "fine":
			st.fatte++
		case "errore":
			if len(st.errori) < 20 {
				st.errori = append(st.errori, testo)
			}
		}
	})
	st.mu.Lock()
	st.createTot += es.Creati
	st.mu.Unlock()
	if err != nil && ctx.Err() == nil {
		slog.Warn("preparazione dei media interrotta", "errore", err)
	}
	slog.Info("preparazione dei media finita", "create", es.Creati, "gia_leggere", es.Tenuti, "errori", es.Errori)
}

func (s *Server) rottePreparazioneMedia(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/media/preparazione", api(s.apiPreparazioneMedia))
	mux.Handle("POST /api/v1/media/preparazione", api(s.apiPreparaOra))
}

// apiPreparazioneMedia: stato per le Impostazioni (solo Master).
func (s *Server) apiPreparazioneMedia(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	st := s.prep()
	ff := ""
	st.mu.Lock()
	ff = st.ffmpeg
	st.mu.Unlock()
	if ff == "" {
		ff = trovaFfmpeg(s.Cfg.Ffmpeg) // prima del primo giro (o subito dopo l'installazione)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out := map[string]any{"ffmpeg": ff, "attivo": st.attivo, "corrente": st.corrente, "totale": st.totale, "fatte": st.fatte,
		"pronte": st.pronte, "errori": st.errori, "createTot": st.createTot}
	if !st.ultimo.IsZero() {
		out["ultimo"] = st.ultimo.Unix()
	}
	if st.errori == nil {
		out["errori"] = []string{}
	}
	return out, nil
}

// apiPreparaOra: «Prepara ora» (anche subito dopo aver installato ffmpeg).
func (s *Server) apiPreparaOra(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	if trovaFfmpeg(s.Cfg.Ffmpeg) == "" {
		return nil, errore(http.StatusConflict, "ffmpeg non trovato: installalo in C:\\Progetti\\Strumenti\\ffmpeg (vedi la guida del Master) e riprova.")
	}
	s.svegliaMedia()
	return map[string]any{"avviato": true}, nil
}
