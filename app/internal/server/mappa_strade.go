package server

// Percorsi sulla mappa calcolati sul PC (P1 dell'analisi). Prima ogni dispositivo, telefoni compresi,
// scaricava la rete stradale e teneva in memoria circa 35 MB di tabelle per cercare il percorso; ora lo
// fa il server con lo stesso algoritmo (A* pixel per pixel, stessi costi, stessa semplificazione) e i
// dispositivi ricevono solo il tracciato.
//
// POST /api/v1/mappa/strada {op:"aggancia", x, y, r}  -> {i}   punto di strada più vicino (-1 se nessuno)
// POST /api/v1/mappa/strada {op:"percorso", a, b}     -> {pts, px} oppure {nessuno:true}
// Per i giocatori valgono solo le zone fuori dalla nebbia del loro gruppo, come prima nella pagina.

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"math"
	"net/http"
	"sync"
)

const nPixel = mappaL * mappaA

type reteStrade struct {
	once sync.Once
	m    []byte // 2 = strada o sentiero, 1 = margine, 0 = non percorribile
	err  error

	mu     sync.Mutex // una ricerca alla volta: le tabelle sono condivise
	g      []float64
	pr     []int32
	st, cl []uint32
	gen    uint32
	hk     []float64
	hv     []int32
}

func (s *Server) strade() (*reteStrade, error) {
	r := &s.reteStrade
	r.once.Do(func() {
		b, err := fs.ReadFile(s.Web, "carta/img/mask.png")
		if err != nil {
			r.err = fmt.Errorf("rete stradale non trovata: %w", err)
			return
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			r.err = fmt.Errorf("rete stradale illeggibile: %w", err)
			return
		}
		if img.Bounds() != image.Rect(0, 0, mappaL, mappaA) {
			r.err = fmt.Errorf("rete stradale di dimensioni inattese: %v", img.Bounds())
			return
		}
		m := make([]byte, nPixel)
		for y := 0; y < mappaA; y++ {
			for x := 0; x < mappaL; x++ {
				rr, _, _, _ := img.At(x, y).RGBA()
				v := rr >> 8
				switch {
				case v > 191:
					m[y*mappaL+x] = 2
				case v > 64:
					m[y*mappaL+x] = 1
				}
			}
		}
		r.m = m
	})
	return r, r.err
}

// aggancia: il primo punto di strada (valore 2) percorribile su quadrati concentrici fino a raggio R.
func (r *reteStrade) aggancia(x, y, raggio int, passa func(int) bool) int {
	for d := 0; d <= raggio; d++ {
		for dy := -d; dy <= d; dy++ {
			ny := y + dy
			if ny < 0 || ny >= mappaA {
				continue
			}
			prova := func(nx int) int {
				if nx < 0 || nx >= mappaL {
					return -1
				}
				n := ny*mappaL + nx
				if r.m[n] == 2 && passa(n) {
					return n
				}
				return -1
			}
			if dy != d && dy != -d {
				for _, nx := range []int{x - d, x + d} {
					if n := prova(nx); n >= 0 {
						return n
					}
				}
			} else {
				for nx := x - d; nx <= x+d; nx++ {
					if n := prova(nx); n >= 0 {
						return n
					}
				}
			}
		}
	}
	return -1
}

func (r *reteStrade) push(k float64, v int32) {
	r.hk, r.hv = append(r.hk, 0), append(r.hv, 0)
	i := len(r.hk) - 1
	for i > 0 {
		p := (i - 1) >> 1
		if r.hk[p] <= k {
			break
		}
		r.hk[i], r.hv[i] = r.hk[p], r.hv[p]
		i = p
	}
	r.hk[i], r.hv[i] = k, v
}

func (r *reteStrade) pop() int32 {
	top := r.hv[0]
	n := len(r.hk) - 1
	k, v := r.hk[n], r.hv[n]
	r.hk, r.hv = r.hk[:n], r.hv[:n]
	if n > 0 {
		i := 0
		for {
			c := 2*i + 1
			if c >= n {
				break
			}
			if c+1 < n && r.hk[c+1] < r.hk[c] {
				c++
			}
			if r.hk[c] >= k {
				break
			}
			r.hk[i], r.hv[i] = r.hk[c], r.hv[c]
			i = c
		}
		r.hk[i], r.hv[i] = k, v
	}
	return top
}

// percorso: A* da a a b (indici di pixel); nil se non collegati.
func (r *reteStrade) percorso(a, b int, passa func(int) bool) ([][2]int, float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.g == nil {
		r.g, r.pr = make([]float64, nPixel), make([]int32, nPixel)
		r.st, r.cl = make([]uint32, nPixel), make([]uint32, nPixel)
	}
	r.gen++
	gen := r.gen
	r.hk, r.hv = r.hk[:0], r.hv[:0]
	bx, by := b%mappaL, b/mappaL
	h := func(v int) float64 {
		dx, dy := math.Abs(float64(v%mappaL-bx)), math.Abs(float64(v/mappaL-by))
		if dx > dy {
			return dx + .41421*dy
		}
		return dy + .41421*dx
	}
	r.st[a], r.g[a] = gen, 0
	r.push(h(a), int32(a))
	for len(r.hk) > 0 {
		v := int(r.pop())
		if r.cl[v] == gen {
			continue
		}
		r.cl[v] = gen
		if v == b {
			break
		}
		vx, vy, gv := v%mappaL, v/mappaL, r.g[v]
		for dy := -1; dy <= 1; dy++ {
			ny := vy + dy
			if ny < 0 || ny >= mappaA {
				continue
			}
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx := vx + dx
				if nx < 0 || nx >= mappaL {
					continue
				}
				n := ny*mappaL + nx
				if r.m[n] == 0 || r.cl[n] == gen || !passa(n) {
					continue
				}
				passo := 1.0
				if dx != 0 && dy != 0 {
					passo = 1.41421
				}
				if r.m[n] != 2 {
					passo *= 3
				}
				nd := gv + passo
				if r.st[n] != gen || nd < r.g[n] {
					r.st[n], r.g[n], r.pr[n] = gen, nd, int32(v)
					r.push(nd+h(n), int32(n))
				}
			}
		}
	}
	if r.cl[b] != gen {
		return nil, 0
	}
	var p [][2]int
	for v := b; ; v = int(r.pr[v]) {
		p = append(p, [2]int{v % mappaL, v / mappaL})
		if v == a {
			break
		}
	}
	for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
	p = semplifica(p, 1.5)
	lung := 0.0
	for i := 1; i < len(p); i++ {
		lung += math.Hypot(float64(p[i][0]-p[i-1][0]), float64(p[i][1]-p[i-1][1]))
	}
	return p, lung
}

// semplifica: Douglas–Peucker, toglie la "scalettatura" dei pixel che gonfierebbe la lunghezza.
func semplifica(p [][2]int, eps float64) [][2]int {
	if len(p) < 3 {
		return append([][2]int(nil), p...)
	}
	tieni := make([]bool, len(p))
	tieni[0], tieni[len(p)-1] = true, true
	pila := [][2]int{{0, len(p) - 1}}
	for len(pila) > 0 {
		ab := pila[len(pila)-1]
		pila = pila[:len(pila)-1]
		a, b := ab[0], ab[1]
		ax, ay := float64(p[a][0]), float64(p[a][1])
		dx, dy := float64(p[b][0])-ax, float64(p[b][1])-ay
		l := math.Hypot(dx, dy)
		if l == 0 {
			l = 1
		}
		md, mi := 0.0, -1
		for i := a + 1; i < b; i++ {
			d := math.Abs(dy*(float64(p[i][0])-ax)-dx*(float64(p[i][1])-ay)) / l
			if d > md {
				md, mi = d, i
			}
		}
		if md > eps {
			tieni[mi] = true
			pila = append(pila, [2]int{a, mi}, [2]int{mi, b})
		}
	}
	var out [][2]int
	for i, ok := range tieni {
		if ok {
			out = append(out, p[i])
		}
	}
	return out
}

// mappaStrada: POST /api/v1/mappa/strada.
func (s *Server) mappaStrada(w http.ResponseWriter, r *http.Request) (any, error) {
	u, _, gruppo, err := s.contestoMappa(r, "")
	if err != nil {
		return nil, err
	}
	var in struct { // JSON: {op, x, y, r} oppure {op, a, b}
		Op      string
		X, Y, R float64
		A, B    int
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	rete, err := s.strade()
	if err != nil {
		return nil, err
	}
	passa := func(int) bool { return true }
	if !u.Master() {
		st, err := s.leggiStatoMappa(gruppo)
		if err != nil {
			return nil, err
		}
		passa = func(n int) bool { return st.svelata(float64(n%mappaL), float64(n/mappaL)) }
	}
	switch in.Op {
	case "aggancia":
		x, y := int(math.Round(in.X)), int(math.Round(in.Y))
		if in.R < 0 || in.R > 600 || !dentroMappa(in.X, in.Y) {
			return nil, errDatiNonValidi
		}
		return map[string]any{"i": rete.aggancia(x, y, int(in.R), passa)}, nil
	case "percorso":
		if in.A < 0 || in.A >= nPixel || in.B < 0 || in.B >= nPixel {
			return nil, errDatiNonValidi
		}
		p, lung := rete.percorso(in.A, in.B, passa)
		if p == nil {
			return map[string]any{"nessuno": true}, nil
		}
		return map[string]any{"pts": p, "px": lung}, nil
	}
	return nil, errDatiNonValidi
}
