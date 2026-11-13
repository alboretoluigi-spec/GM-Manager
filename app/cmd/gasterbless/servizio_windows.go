//go:build windows

package main

// Avvio come servizio di Windows ("Gaster Bless", account virtuale NT SERVICE\GasterBless, creato da
// strumenti/installa.ps1). Il servizio si avvia da solo con Windows e si ferma con "Ferma Gaster Bless".

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"

	"gasterbless/internal/config"
	"gasterbless/internal/server"
	"gasterbless/web"
)

func comeServizio() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

type servizio struct{ cfg config.Config }

func (x *servizio) Execute(_ []string, richieste <-chan svc.ChangeRequest, stato chan<- svc.Status) (bool, uint32) {
	stato <- svc.Status{State: svc.StartPending}
	s, err := server.Nuovo(x.cfg, web.File)
	if err != nil {
		slog.Error("avvio del servizio non riuscito", "errore", err)
		return true, 1
	}
	ctx, ferma := context.WithCancel(context.Background())
	fine := make(chan error, 1)
	go func() { fine <- s.Avvia(ctx) }()
	stato <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case c := <-richieste:
			switch c.Cmd {
			case svc.Interrogate:
				stato <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				stato <- svc.Status{State: svc.StopPending}
				ferma()
				select {
				case <-fine:
				case <-time.After(10 * time.Second):
				}
				s.Chiudi()
				return false, 0
			}
		case err := <-fine:
			slog.Error("il server si è fermato", "errore", err)
			s.Chiudi()
			ferma()
			return true, 2
		}
	}
}

// eseguiServizio scrive il registro in <cartella dati>\registro\gasterbless.log e passa il controllo a Windows.
func eseguiServizio(cfg config.Config) {
	dir := filepath.Join(cfg.CartellaDati, "registro")
	os.MkdirAll(dir, 0o700)
	if f, err := os.OpenFile(filepath.Join(dir, "gasterbless.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})))
	}
	if err := svc.Run("Gaster Bless", &servizio{cfg: cfg}); err != nil {
		fmt.Fprintln(os.Stderr, "Servizio:", err)
		os.Exit(1)
	}
}
