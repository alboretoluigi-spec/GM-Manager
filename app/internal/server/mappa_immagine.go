package server

// Immagine della mappa coperta dalla nebbia (dal ramo main-ke8sy4, mappa_immagine.go dell'altro account).
// Il Master riceve la mappa completa; i giocatori una copia in cui tutto ciò che sta sotto la nebbia del loro
// gruppo è sostituito dal colore della nebbia: aprendo l'indirizzo dell'immagine non si scopre nulla di
// nascosto. I file completi della pagina (carta/img/gasterbless.webp e la rete stradale carta/img/mask.png) sono
// serviti solo al Master (server.go); i percorsi dei giocatori li calcola il server (mappa_strade.go).

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"sync"
)

//go:embed dati/gasterbless.jpg
var datiMappa embed.FS

// Colore della nebbia (lo stesso della texture disegnata dalla pagina).
var coloreNebbia = color.RGBA{0x9e, 0xa8, 0xb0, 0xff}

var (
	mappaBase = sync.OnceValue(func() *image.RGBA {
		b, _ := datiMappa.ReadFile("dati/gasterbless.jpg")
		im, err := jpeg.Decode(bytes.NewReader(b))
		if err != nil {
			panic(err)
		}
		out := image.NewRGBA(im.Bounds())
		for y := 0; y < mappaA; y++ {
			for x := 0; x < mappaL; x++ {
				out.Set(x, y, im.At(x, y))
			}
		}
		return out
	})
	muCacheMappa sync.Mutex
	cacheMappa   = map[string][]byte{}
)

// versioneNebbia identifica una nebbia (per l'indirizzo dell'immagine e la cache).
func versioneNebbia(n []byte) string {
	h := sha256.Sum256(n)
	return hex.EncodeToString(h[:8])
}

// pixelVisibili: un pixel è visibile se è vicino al centro di una cella svelata. Il raggio è quello
// della sfumatura disegnata dalla pagina (0,95 celle), così il bordo coincide con la nebbia sullo schermo.
func pixelVisibili(nebbia []byte) []bool {
	vis := make([]bool, mappaL*mappaA)
	R := float64(cellaNebbia) * 0.95
	r2 := R * R
	for j := 0; j < righeNebbia; j++ {
		for i := 0; i < colonneNebbia; i++ {
			k := j*colonneNebbia + i
			if k/8 >= len(nebbia) || nebbia[k/8]&(1<<(k%8)) == 0 {
				continue
			}
			cx, cy := (float64(i)+0.5)*cellaNebbia, (float64(j)+0.5)*cellaNebbia
			for y := max(0, int(cy-R)); y <= min(mappaA-1, int(cy+R)); y++ {
				dy := float64(y) + 0.5 - cy
				for x := max(0, int(cx-R)); x <= min(mappaL-1, int(cx+R)); x++ {
					dx := float64(x) + 0.5 - cx
					if dx*dx+dy*dy <= r2 {
						vis[y*mappaL+x] = true
					}
				}
			}
		}
	}
	return vis
}

func immagineConNebbia(nebbia []byte) []byte {
	base, vis := mappaBase(), pixelVisibili(nebbia)
	out := image.NewRGBA(base.Bounds())
	copy(out.Pix, base.Pix)
	for p, ok := range vis {
		if !ok {
			o := p * 4
			out.Pix[o], out.Pix[o+1], out.Pix[o+2], out.Pix[o+3] = coloreNebbia.R, coloreNebbia.G, coloreNebbia.B, 0xff
		}
	}
	var b bytes.Buffer
	jpeg.Encode(&b, out, &jpeg.Options{Quality: 88})
	return b.Bytes()
}

// inCache prepara un'immagine una volta sola per ogni nebbia (fino a 40 versioni in memoria).
func inCache(chiave string, f func() []byte) []byte {
	muCacheMappa.Lock()
	b, ok := cacheMappa[chiave]
	muCacheMappa.Unlock()
	if ok {
		return b
	}
	b = f()
	muCacheMappa.Lock()
	if len(cacheMappa) >= 40 {
		cacheMappa = map[string][]byte{}
	}
	cacheMappa[chiave] = b
	muCacheMappa.Unlock()
	return b
}

// immagineMappa: GET /api/v1/mappa/immagine?sessione=…&v=<versione della nebbia>.
func (s *Server) immagineMappa(w http.ResponseWriter, r *http.Request) {
	u, _, gruppo, err := s.contestoMappa(r, "")
	if err != nil {
		http.Error(w, "Non trovata.", http.StatusNotFound)
		return
	}
	var b []byte
	if u.Master() && r.URL.Query().Get("vista") != "giocatori" {
		b, _ = datiMappa.ReadFile("dati/gasterbless.jpg")
	} else {
		st, err := s.leggiStatoMappa(gruppo)
		if err != nil {
			http.Error(w, "Errore.", http.StatusInternalServerError)
			return
		}
		n := st.Nebbia
		b = inCache(versioneNebbia(n), func() []byte { return immagineConNebbia(n) })
	}
	w.Header().Set("Content-Type", "image/jpeg")
	// L'indirizzo contiene la versione della nebbia: quando cambia, la pagina chiede un'immagine nuova.
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Write(b)
}
