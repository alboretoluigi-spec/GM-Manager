package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Media leggeri: con ffmpeg si creano copie leggere in Media/_leggere (originali intatti); Gaster Bless le manda al posto
// degli originali finché questi non cambiano.
func TestMediaLeggeri(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg non installato")
	}
	media := t.TempDir()
	os.MkdirAll(filepath.Join(media, "Immagini", "Borgofosco"), 0o755)
	os.MkdirAll(filepath.Join(media, "Video"), 0o755)
	foto := filepath.Join(media, "Immagini", "Borgofosco", "piazza.png")
	// immagine «fotografica» pesante (PNG senza perdita) e un video pesante
	if o, err := exec.Command(ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "mandelbrot=s=1600x1000", "-frames:v", "1", foto).CombinedOutput(); err != nil {
		t.Fatal(string(o))
	}
	video := filepath.Join(media, "Video", "arrivo.mp4")
	if o, err := exec.Command(ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=s=1280x720:r=30:d=6", "-c:v", "libx264", "-qp", "0", video).CombinedOutput(); err != nil {
		t.Fatal(string(o))
	}
	piccola := filepath.Join(media, "Immagini", "icona.png")
	exec.Command(ff, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=32x32", "-frames:v", "1", piccola).Run()
	prima, _ := os.ReadFile(foto)

	es, err := OttimizzaMedia(context.Background(), media, ff, func(string) {})
	// 3 copie: immagine leggera, video leggero 1080p e video per i telefoni (2026-11-11).
	if err != nil || es.Creati != 3 || es.Errori != 0 {
		t.Fatalf("prima passata: %+v %v", es, err)
	}
	if dopo, _ := os.ReadFile(foto); string(dopo) != string(prima) {
		t.Fatal("l'originale non va toccato")
	}
	if es.Dopo*2 > es.Prima {
		t.Fatalf("copie non abbastanza leggere: %d → %d", es.Prima, es.Dopo)
	}
	// Seconda passata: niente da rifare.
	if es2, _ := OttimizzaMedia(context.Background(), media, ff, func(string) {}); es2.Creati != 0 || es2.Saltati != 3 {
		t.Fatalf("seconda passata: %+v", es2)
	}

	// Gaster Bless manda la copia leggera (WebP) al posto del PNG.
	sc := nuovoScontro(t)
	sc.a.s.Cfg.CartellaMedia = media
	rel := "Immagini/Borgofosco/piazza.png"
	req, _ := http.NewRequest("GET", sc.a.srv.URL+"/media/"+rel, nil)
	if sc.m.ip != "" {
		req.Header.Set("X-Test-IP", sc.m.ip)
	}
	r, err := sc.m.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 || r.Header.Get("Content-Type") != "image/webp" || len(b) >= len(prima) {
		t.Fatalf("copia leggera: %d %s %d byte", r.StatusCode, r.Header.Get("Content-Type"), len(b))
	}
	// Video: ai telefoni dei giocatori la versione 720p, al Master la copia leggera 1080p.
	tel, leg := sc.a.s.versioneTelefono(video), sc.a.s.versioneLeggera(video)
	if tel == "" || leg == "" || tel == leg {
		t.Fatalf("versioni del video: telefono %q, leggera %q", tel, leg)
	}
	for _, x := range []struct {
		telefono bool
		atteso   string
	}{{true, tel}, {false, leg}} {
		rec := httptest.NewRecorder()
		sc.a.s.serviFileMedia(rec, httptest.NewRequest("GET", "/media/Video/arrivo.mp4", nil), video, x.telefono)
		if fi, _ := os.Stat(x.atteso); rec.Code != 200 || int64(rec.Body.Len()) != fi.Size() {
			t.Fatalf("telefono=%v: %d, %d byte invece di %d (%s)", x.telefono, rec.Code, rec.Body.Len(), fi.Size(), x.atteso)
		}
	}
	// Originale modificato dopo la copia: torna l'originale finché non si ripassa il comando.
	futuro := time.Now().Add(time.Hour)
	os.Chtimes(foto, futuro, futuro)
	if l := sc.a.s.versioneLeggera(foto); l != "" {
		t.Fatalf("originale più recente: si deve mandare l'originale, non %s", l)
	}
	if !strings.HasSuffix(percorsoLeggero(media, video), filepath.Join("_leggere", "Video", "arrivo.mp4.mp4")) {
		t.Fatal(percorsoLeggero(media, video))
	}
}

// trovaFfmpeg: con l'impostazione di gasterbless.conf si usa quella (e niente se il file non c'è).
func TestTrovaFfmpeg(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "ffmpeg.exe")
	os.WriteFile(f, []byte("x"), 0o755)
	if got := trovaFfmpeg(f); got != f {
		t.Fatalf("impostato: %q", got)
	}
	if got := trovaFfmpeg(filepath.Join(d, "manca.exe")); got != "" {
		t.Fatalf("impostato ma assente: %q", got)
	}
}

// Stato della preparazione: solo il Master; «Prepara ora» senza ffmpeg spiega cosa fare.
func TestPreparazioneMediaAPI(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := a.giocatore("Elara", "+393331112224", "192.168.1.30")
	g.deve(403, "GET", "/media/preparazione", nil)
	a.s.Cfg.Ffmpeg = filepath.Join(t.TempDir(), "manca.exe")
	out := m.deve(200, "GET", "/media/preparazione", nil)
	if out["ffmpeg"] != "" || out["attivo"] != false {
		t.Fatalf("senza ffmpeg: %v", out)
	}
	_, e := m.chiama("POST", "/media/preparazione", map[string]any{})
	if !strings.Contains(e["errore"].(string), "ffmpeg") {
		t.Fatalf("prepara ora senza ffmpeg: %v", e)
	}
}
