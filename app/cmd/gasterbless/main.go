// Gaster Bless: mappa, iniziativa e schede quinta edizione in un unico programma.
//
// Uso:
//
//	GasterBless.exe                         avvia il server (finestra)
//	GasterBless.exe -config percorso.conf   usa un file di configurazione diverso
//	GasterBless.exe versione                mostra la versione
//	GasterBless.exe -prova cartella         SOLO PER LE PROVE: dati in una cartella a parte, chiave senza DPAPI
//	GasterBless.exe backup                  crea subito un backup cifrato
//	GasterBless.exe ripristina FILE [-codice CODICE]   rimette un backup (a Gaster Bless fermo)
//	GasterBless.exe codice-ripristino       mostra il codice per ripristinare i backup su un altro PC
//	GasterBless.exe migra [-prova]          importa i dati della vecchia Gaster Bless (GasterBlessDati)
//	GasterBless.exe reimposta-master        nuova password del Master, letta da stdin (da amministratore;
//	                                    usare strumenti/reimposta-password-master.ps1)
//
// Gli altri comandi (servizio, backup, ripristino, migrazione) sono descritti nel README.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"gasterbless/internal/config"
	"gasterbless/internal/server"
	"gasterbless/web"
)

// Versione dell'app (impostata in fase di compilazione con -ldflags).
var Versione = "0.1.0-sviluppo"

func main() {
	percorso := flag.String("config", "", "file di configurazione (predefinito: gasterbless.conf accanto al programma)")
	prova := flag.String("prova", "", "SOLO PER LE PROVE: usa questa cartella per i dati, con la chiave non protetta da DPAPI")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Uso: GasterBless.exe [-config file] [-prova cartella] [comando]")
		fmt.Fprintln(os.Stderr, "Comandi: avvia (predefinito), versione, backup, ripristina FILE [-codice CODICE], codice-ripristino, migra [-prova], reimposta-master, ottimizza-media [-ffmpeg percorso]")
	}
	flag.Parse()
	comando := flag.Arg(0)
	if comando == "" {
		comando = "avvia"
	}
	if comando == "versione" {
		fmt.Println("Gaster Bless", Versione)
		return
	}
	if *percorso == "" {
		exe, _ := os.Executable()
		*percorso = filepath.Join(filepath.Dir(exe), "gasterbless.conf")
	}
	cfg, err := config.Leggi(*percorso)
	if err != nil {
		esci("Configurazione non valida: %v", err)
	}
	if *prova != "" {
		cfg.ModalitaProva = true
		cfg.CartellaDati = *prova
		fmt.Println("  ATTENZIONE: modalità di prova, dati in", *prova, "(non usarla per giocare).")
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if comeServizio() { // avviato da Windows come servizio (vedi strumenti/installa.ps1)
		eseguiServizio(cfg)
		return
	}
	switch comando {
	case "avvia":
		avvia(cfg)
	case "backup":
		s, err := server.Nuovo(cfg, web.File)
		if err != nil {
			esci("%v", err)
		}
		nome, err := s.CreaBackup(context.Background())
		s.Chiudi()
		if err != nil {
			esci("Backup non riuscito: %v", err)
		}
		fmt.Println("Backup creato:", filepath.Join(cfg.CartellaDati, "backup", nome))
	case "ottimizza-media":
		// Copie leggere di immagini e video della cartella Media (internal/server/media_leggeri.go).
		opz := flag.NewFlagSet("ottimizza-media", flag.ExitOnError)
		ff := opz.String("ffmpeg", "ffmpeg", "percorso di ffmpeg.exe")
		opz.Parse(flag.Args()[1:])
		fmt.Println("Copie leggere della cartella Media in", filepath.Join(cfg.CartellaMedia, "_leggere"), "(gli originali non si toccano).")
		es, err := server.OttimizzaMedia(context.Background(), cfg.CartellaMedia, *ff, func(t string) { fmt.Println(t) })
		fmt.Printf("\nFatto: %d copie leggere nuove, %d file già leggeri, %d già fatti in precedenza, %d errori.\n", es.Creati, es.Tenuti, es.Saltati, es.Errori)
		if es.Creati > 0 {
			fmt.Printf("Peso dei file alleggeriti: da %.1f MB a %.1f MB.\n", float64(es.Prima)/(1<<20), float64(es.Dopo)/(1<<20))
		}
		if err != nil {
			esci("%v", err)
		}
	case "ripristina":
		opz := flag.NewFlagSet("ripristina", flag.ExitOnError)
		codice := opz.String("codice", "", "codice di ripristino (solo per ripristinare su un altro PC)")
		if flag.NArg() < 2 {
			esci("Indica il file del backup: GasterBless.exe ripristina gasterbless-AAAAMMGG-HHMMSS.bak")
		}
		opz.Parse(flag.Args()[2:])
		if *codice == "" { // dallo script RIPRISTINA_BACKUP.bat il codice arriva così, non sulla riga di comando
			*codice = strings.TrimSpace(os.Getenv("GASTERBLESS_CODICE_RIPRISTINO"))
		}
		parte, err := server.Ripristina(cfg, flag.Arg(1), *codice)
		if err != nil {
			esci("Ripristino non riuscito: %v", err)
		}
		fmt.Println("Ripristino completato. Lo stato precedente è stato messo da parte in:", parte)
		fmt.Println("Ora puoi avviare Gaster Bless.")
	case "codice-ripristino":
		c, err := server.CodiceRipristino(cfg)
		if err != nil {
			esci("%v", err)
		}
		fmt.Print("\n  CODICE DI RIPRISTINO DI GASTER BLESS\n\n")
		fmt.Println("  " + c)
		fmt.Println("\n  Scrivilo su carta e conservalo in un posto sicuro, lontano dal PC.")
		fmt.Println("  Serve SOLO per ripristinare i backup su un altro PC (per esempio se questo si guasta).")
		fmt.Println("  Chi ha il codice e un file di backup può leggere i dati: non mandarlo per messaggio o e-mail.")
	case "migra":
		opz := flag.NewFlagSet("migra", flag.ExitOnError)
		solo := opz.Bool("prova", false, "legge e produce il rapporto senza scrivere nulla")
		opz.Parse(flag.Args()[1:])
		s, err := server.Nuovo(cfg, web.File)
		if err != nil {
			esci("%v", err)
		}
		rapporto, err := s.Migra(context.Background(), *solo)
		s.Chiudi()
		fmt.Println(rapporto)
		if err != nil {
			esci("Migrazione non riuscita: %v", err)
		}
	case "reimposta-master":
		// La password arriva dalla variabile GASTERBLESS_NUOVA_PASSWORD impostata dallo script (che la chiede
		// senza mostrarla); in alternativa da stdin. Si tolgono il carattere invisibile BOM e l'a capo che
		// PowerShell 5.1 aggiunge quando passa un testo a un programma.
		pw := os.Getenv("GASTERBLESS_NUOVA_PASSWORD")
		if pw == "" {
			riga, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			pw = riga
		}
		pw = pulisciPassword(pw)
		if pw == "" {
			esci("Nessuna password ricevuta: usa strumenti\\reimposta-password-master.ps1")
		}
		nome, err := server.ReimpostaPasswordMasterDalPC(cfg, pw)
		if err != nil {
			esci("Password non cambiata: %v", err)
		}
		fmt.Printf("Password del Master (%s) cambiata. Tutti i dispositivi del Master devono rientrare con la nuova password.\n", nome)
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func avvia(cfg config.Config) {
	sotto, err := fs.Sub(web.File, ".")
	if err != nil {
		esci("%v", err)
	}
	s, err := server.Nuovo(cfg, sotto)
	if err != nil {
		esci("Avvio non riuscito: %v", err)
	}
	defer s.Chiudi()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Printf("\n  GASTER BLESS %s\n  Lascia aperta questa finestra durante la partita: chiudendola spegni Gaster Bless.\n\n", Versione)
	if err := s.Avvia(ctx); err != nil {
		esci("Server fermato: %v", err)
	}
}

func esci(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "ERRORE: "+f+"\n", a...)
	os.Exit(1)
}

// pulisciPassword toglie il BOM iniziale e l'a capo finale che PowerShell 5.1 aggiunge a un testo
// passato a un programma; il resto della password resta com'è.
func pulisciPassword(pw string) string {
	return strings.TrimRight(strings.TrimPrefix(pw, "\ufeff"), "\r\n")
}
