package server

// Media più leggeri (richiesta dell'utente, 2026-11-08: «ridurre drasticamente il peso di immagini e video mantenendo
// alta la qualità»). Il comando «GasterBless.exe ottimizza-media» (strumenti\ottimizza-media.ps1) usa ffmpeg per creare,
// nella cartella Media\_leggere, una copia leggera di ogni immagine e video pesante della cartella Media:
//   - immagini (jpg, png, webp) oltre 150 KB → WebP qualità 85, lato lungo al massimo 2560 px (trasparenza conservata);
//   - video (mp4, webm, mov, m4v) oltre 2 MB → MP4 H.264 (CRF 22, preset slow), larghezza al massimo 1920 px, audio AAC
//     128 kbps, pronto per partire subito (faststart).
// La copia si tiene solo se pesa almeno il 20% in meno. Gli originali non vengono MAI toccati: quando un dispositivo
// chiede un file, Gaster Bless manda la copia leggera se esiste ed è più recente dell'originale, altrimenti l'originale
// (sostituire o modificare un originale lo rende di nuovo valido finché non si ripassa il comando).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const cartellaLeggeri = "_leggere"

var (
	immaginiDaAlleggerire = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	videoDaAlleggerire    = map[string]bool{".mp4": true, ".webm": true, ".mov": true, ".m4v": true}
)

const (
	sogliaImmagine = 150 << 10
	sogliaVideo    = 2 << 20
	sogliaTelefono = 1 << 20 // video per i telefoni: da 1 MB in su (2026-11-11)
)

// percorsoLeggero: dove sta (o starebbe) la copia leggera di un file della cartella Media ("" se non previsto).
func percorsoLeggero(media, originale string) string {
	rel, err := filepath.Rel(media, originale)
	if err != nil || strings.HasPrefix(rel, "..") || strings.HasPrefix(filepath.ToSlash(rel), cartellaLeggeri+"/") {
		return ""
	}
	est := strings.ToLower(filepath.Ext(rel))
	switch {
	case immaginiDaAlleggerire[est]:
		return filepath.Join(media, cartellaLeggeri, rel+".webp")
	case videoDaAlleggerire[est]:
		return filepath.Join(media, cartellaLeggeri, rel+".mp4")
	}
	return ""
}

// percorsoTelefono: dove sta (o starebbe) la versione per i telefoni di un video della cartella Media.
func percorsoTelefono(media, originale string) string {
	if !videoDaAlleggerire[strings.ToLower(filepath.Ext(originale))] {
		return ""
	}
	l := percorsoLeggero(media, originale)
	if l == "" {
		return ""
	}
	return strings.TrimSuffix(l, ".mp4") + ".telefono.mp4"
}

// versioneLeggera: la copia leggera di un file della cartella Media, se esiste ed è più recente dell'originale.
func (s *Server) versioneLeggera(originale string) string {
	if s.Cfg.CartellaMedia == "" {
		return ""
	}
	return copiaValida(originale, percorsoLeggero(s.Cfg.CartellaMedia, originale))
}

// versioneTelefono: la versione per i telefoni di un video, se esiste ed è più recente dell'originale.
func (s *Server) versioneTelefono(originale string) string {
	if s.Cfg.CartellaMedia == "" {
		return ""
	}
	return copiaValida(originale, percorsoTelefono(s.Cfg.CartellaMedia, originale))
}

// copiaValida: l se esiste, non è vuota e non è più vecchia dell'originale.
func copiaValida(originale, l string) string {
	if l == "" {
		return ""
	}
	fl, err := os.Stat(l)
	if err != nil || fl.Size() == 0 {
		return ""
	}
	fo, err := os.Stat(originale)
	if err != nil || fo.ModTime().After(fl.ModTime()) {
		return ""
	}
	return l
}

// EsitoOttimizza riassume un passaggio di «ottimizza-media» o della preparazione automatica.
type EsitoOttimizza struct {
	Creati, Tenuti, Saltati, Errori int
	Prima, Dopo                     int64
}

type voceRegistro struct {
	Mod   int64  `json:"mod"`
	Esito string `json:"esito"`
}

type registroLeggeri map[string]voceRegistro

// lavoroMedia: una copia da creare (o già fatta) per un file della cartella Media.
type lavoroMedia struct {
	originale, chiave, out string
	telefono, img          bool
	mod, dim               int64
}

// argomentiFfmpeg: i parametri di ffmpeg per ogni tipo di copia.
func argomentiFfmpeg(l lavoroMedia, tmp string) []string {
	base := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", l.originale}
	switch {
	case l.img:
		return append(base, "-frames:v", "1",
			"-vf", "scale='min(2560,iw)':'min(2560,ih)':force_original_aspect_ratio=decrease",
			"-c:v", "libwebp", "-quality", "85", "-compression_level", "6", "-map_metadata", "-1", tmp)
	case l.telefono:
		// Telefoni al tavolo (2026-11-11): 720p, circa 2 Mbit/s con tetto, profilo «main» letto da ogni telefono,
		// un fotogramma chiave ogni 2 secondi (si salta avanti senza aspettare), avvio immediato (faststart).
		return append(base, "-map", "0:v:0", "-map", "0:a:0?",
			"-vf", "scale='min(1280,iw)':-2", "-c:v", "libx264", "-preset", "medium", "-crf", "25",
			"-maxrate", "2500k", "-bufsize", "5000k", "-profile:v", "main", "-pix_fmt", "yuv420p", "-g", "60",
			"-c:a", "aac", "-b:a", "96k", "-ac", "2", "-movflags", "+faststart", "-map_metadata", "-1", tmp)
	default:
		return append(base, "-map", "0:v:0", "-map", "0:a:0?",
			"-vf", "scale='min(1920,iw)':-2", "-c:v", "libx264", "-preset", "slow", "-crf", "22", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", "-map_metadata", "-1", tmp)
	}
}

// lavoriMedia elenca le copie da fare e conta quelle già fatte e ancora valide.
func lavoriMedia(media string, reg registroLeggeri) (daFare []lavoroMedia, fatti int) {
	dest := filepath.Join(media, cartellaLeggeri)
	filepath.WalkDir(media, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p == dest || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		est := strings.ToLower(filepath.Ext(p))
		img, vid := immaginiDaAlleggerire[est], videoDaAlleggerire[est]
		if !img && !vid {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(media, p)
		chiave := filepath.ToSlash(rel)
		var cand []lavoroMedia
		if (img && info.Size() >= sogliaImmagine) || (vid && info.Size() >= sogliaVideo) {
			cand = append(cand, lavoroMedia{originale: p, chiave: chiave, out: percorsoLeggero(media, p), img: img})
		}
		if vid && info.Size() >= sogliaTelefono {
			cand = append(cand, lavoroMedia{originale: p, chiave: chiave + "#telefono", out: percorsoTelefono(media, p), telefono: true})
		}
		for _, l := range cand {
			l.mod, l.dim = info.ModTime().Unix(), info.Size()
			if r, ok := reg[l.chiave]; ok && r.Mod == l.mod && (r.Esito != "creato" || fileEsiste(l.out)) {
				fatti++ // già fatto e originale non cambiato (anche «errore»: non si riprova finché il file non cambia)
				continue
			}
			daFare = append(daFare, l)
		}
		return nil
	})
	return daFare, fatti
}

func leggiRegistro(media string) registroLeggeri {
	reg := registroLeggeri{}
	if b, err := os.ReadFile(filepath.Join(media, cartellaLeggeri, "registro.json")); err == nil {
		json.Unmarshal(b, &reg)
	}
	return reg
}

func salvaRegistro(media string, reg registroLeggeri) {
	if b, err := json.MarshalIndent(reg, "", " "); err == nil {
		os.WriteFile(filepath.Join(media, cartellaLeggeri, "registro.json"), b, 0o644)
	}
}

// eseguiLavoro crea una copia con ffmpeg. La tiene solo se pesa abbastanza meno dell'originale.
func eseguiLavoro(ctx context.Context, ffmpeg string, l lavoroMedia) (esito string, dopo int64, err error) {
	os.MkdirAll(filepath.Dir(l.out), 0o755)
	tmp := l.out + ".tmp" + filepath.Ext(l.out)
	cmd := exec.CommandContext(ctx, ffmpeg, argomentiFfmpeg(l, tmp)...)
	prioritaBassa(cmd)
	if o, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		if ctx.Err() != nil {
			return "", 0, ctx.Err()
		}
		return "errore", 0, errors.New(strings.TrimSpace(string(o)))
	}
	ft, err := os.Stat(tmp)
	limite := l.dim * 8 / 10
	if l.telefono {
		limite = l.dim * 9 / 10 // per i telefoni basta un 10% in meno: conta soprattutto la risoluzione
	}
	if err == nil && ft.Size() > 0 && ft.Size() <= limite {
		if err := os.Rename(tmp, l.out); err == nil {
			return "creato", ft.Size(), nil
		}
	}
	os.Remove(tmp)
	os.Remove(l.out)
	return "tenuto", 0, nil
}

// OttimizzaMedia crea le copie leggere (comando «GasterBless.exe ottimizza-media»). ffmpeg è il percorso del programma;
// scrivi riceve i messaggi di avanzamento.
func OttimizzaMedia(ctx context.Context, media, ffmpeg string, scrivi func(string)) (EsitoOttimizza, error) {
	return ottimizza(ctx, media, ffmpeg, func(_ string, msg string) { scrivi(msg) })
}

// ottimizza: avanza(fase, testo) con fase "inizio" (testo = file), "fine", "errore" o "" (solo messaggio).
func ottimizza(ctx context.Context, media, ffmpeg string, avanza func(fase, testo string)) (EsitoOttimizza, error) {
	var es EsitoOttimizza
	if media == "" {
		return es, errors.New("cartella Media non impostata in gasterbless.conf")
	}
	if _, err := exec.LookPath(ffmpeg); err != nil {
		return es, fmt.Errorf("ffmpeg non trovato (%s)", ffmpeg)
	}
	if err := os.MkdirAll(filepath.Join(media, cartellaLeggeri), 0o755); err != nil {
		return es, err
	}
	reg := leggiRegistro(media)
	defer func() { salvaRegistro(media, reg) }()
	daFare, fatti := lavoriMedia(media, reg)
	es.Saltati = fatti
	for i, l := range daFare {
		if ctx.Err() != nil {
			return es, ctx.Err()
		}
		tipo := ""
		if l.telefono {
			tipo = " (telefoni)"
		}
		avanza("inizio", l.chiave)
		avanza("", fmt.Sprintf("… %s%s", strings.TrimSuffix(l.chiave, "#telefono"), tipo))
		inizio := time.Now()
		esito, dopo, err := eseguiLavoro(ctx, ffmpeg, l)
		if ctx.Err() != nil {
			return es, ctx.Err()
		}
		switch esito {
		case "errore":
			es.Errori++
			avanza("errore", strings.TrimSuffix(l.chiave, "#telefono")+": "+err.Error())
			avanza("", "  ✗ errore di ffmpeg: "+err.Error())
		case "creato":
			es.Creati++
			es.Prima += l.dim
			es.Dopo += dopo
			avanza("", fmt.Sprintf("  ✓ %s → %s (%s)", mb(l.dim), mb(dopo), time.Since(inizio).Round(time.Second)))
		default:
			es.Tenuti++
			avanza("", "  = già leggero: resta l'originale")
		}
		reg[l.chiave] = voceRegistro{l.mod, esito}
		avanza("fine", l.chiave)
		if i%5 == 4 {
			salvaRegistro(media, reg)
		}
	}
	return es, nil
}

// serviFileMedia manda la versione più adatta di un file della cartella Media: ai telefoni dei giocatori (telefono = true)
// la versione per i telefoni, se c'è; poi la copia leggera; altrimenti l'originale. Richieste parziali comprese.
func (s *Server) serviFileMedia(w http.ResponseWriter, r *http.Request, p string, telefono bool) {
	if t := s.versioneTelefono(p); telefono && t != "" {
		p = t
	} else if l := s.versioneLeggera(p); l != "" {
		p = l
	}
	http.ServeFile(w, r, p)
}

func fileEsiste(p string) bool { _, err := os.Stat(p); return err == nil }

func mb(n int64) string {
	if n < 1<<20 {
		return fmt.Sprintf("%d KB", n>>10)
	}
	return strings.Replace(fmt.Sprintf("%.1f MB", float64(n)/(1<<20)), ".", ",", 1)
}
