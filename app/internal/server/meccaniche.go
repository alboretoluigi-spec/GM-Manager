package server

// Motore dei tiri per famiglia di meccaniche (fase 3c, passo C4c, docs/MECCANICHE.md). Ogni famiglia del registro
// web/sistemi/moduli.json ha qui il suo tiro: i dadi li tira il server (si vedono anche in 3D come gli altri tiri),
// l'esito è scritto in parole e arriva a chi deve vederlo con lo stesso percorso dei tiri normali (inviaTiro). Sono
// meccaniche generiche scritte da noi: nessun testo dei manuali, nomi descrittivi.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// richiestaMeccanica: i dati di un tiro. Non tutte le meccaniche usano tutti i campi (docs/MECCANICHE.md).
type richiestaMeccanica struct {
	Meccanica  string         `json:"meccanica"`
	Valore     int            `json:"valore"`     // valore dell'abilità o della caratteristica (d100, 3d6, d20 sotto, d10 + somma, Fate)
	Dadi       int            `json:"dadi"`       // dadi della riserva, oppure facce del dado del tratto (passi esplosivi)
	Mod        int            `json:"mod"`        // modificatore
	Difficolta *int           `json:"difficolta"` // successi richiesti, numero da raggiungere o da superare
	Soglia     int            `json:"soglia"`     // riserve: risultato minimo di un dado per un successo
	Extra      int            `json:"extra"`      // d100: dadi bonus (positivo) o penalità (negativo)
	Modo       string         `json:"modo"`       // riserva d6: «successi» o «massimo»; d20 sotto: vantaggio/svantaggio
	Opzioni    map[string]any `json:"opzioni"`    // coppie di 10, dado jolly…
	Narrativi  map[string]int `json:"narrativi"`  // dadi narrativi per tipo
}

func (r richiestaMeccanica) opzione(k string) bool { v, _ := r.Opzioni[k].(bool); return v }

// esitoMeccanica: ciò che si aggiunge al tiro normale.
type esitoMeccanica struct {
	Esito    string `json:"esito"`              // chiave: critico, successo, parziale, fallimento, disastro…
	Testo    string `json:"testo"`              // in parole, per chi guarda
	Successi *int   `json:"successi,omitempty"` // riserve e dadi narrativi
}

var nomiMeccaniche = map[string]string{"d100": "d100 sotto il valore", "riserva-d10": "riserva di d10", "riserva-d6": "riserva di d6",
	"2d6-fasce": "2d6 a fasce", "d10-somma": "d10 + valore", "passi-esplosivi": "dado esplosivo", "fate": "dadi Fate", "3d6-sotto": "3d6 sotto il valore",
	"d20-sotto": "d20 sotto il valore", "narrativi": "dadi narrativi"}

func lancia(n, facce int) []int {
	v := make([]int, n)
	for i := range v {
		v[i] = tiraDado(facce)
	}
	return v
}

func puntatore(n int) *int { return &n }

// tiraMeccanica esegue il tiro di una famiglia: gruppi di dadi (per la grafica), totale, formula ed esito.
func tiraMeccanica(in richiestaMeccanica) (t tiroDadi, e esitoMeccanica, err error) {
	if in.Mod < -50 || in.Mod > 50 {
		return t, e, fmt.Errorf("il modificatore va da −50 a +50")
	}
	diff := func(pred int) int {
		if in.Difficolta != nil {
			return *in.Difficolta
		}
		return pred
	}
	switch in.Meccanica {
	case "d100":
		v := in.Valore + in.Mod
		if in.Valore < 1 || in.Valore > 200 {
			return t, e, fmt.Errorf("il valore va da 1 a 200")
		}
		if in.Extra < -2 || in.Extra > 2 {
			return t, e, fmt.Errorf("al massimo 2 dadi bonus o penalità")
		}
		unita := tiraDado(10) - 1
		decine := lancia(1+abs(in.Extra), 10)
		risultati := []int{}
		for _, d := range decine {
			x := (d-1)*10 + unita
			if x == 0 {
				x = 100
			}
			risultati = append(risultati, x)
		}
		sort.Ints(risultati)
		r := risultati[0] // con i dadi bonus si tiene il più basso
		if in.Extra < 0 {
			r = risultati[len(risultati)-1]
		}
		t.Dadi = []gruppoDadi{{N: 1, Facce: 100, Valori: []int{r}}}
		t.Totale = r
		t.Formula = "d100 ≤ " + strconv.Itoa(v)
		if in.Extra != 0 {
			t.Formula += fmt.Sprintf(" (%d dadi %s: %v)", abs(in.Extra), map[bool]string{true: "bonus", false: "penalità"}[in.Extra > 0], risultati)
		}
		disastro := r == 100 || v < 50 && r >= 96
		switch {
		case r == 1:
			e = esitoMeccanica{"critico", "Successo critico", nil}
		case disastro:
			e = esitoMeccanica{"disastro", "Disastro", nil}
		case r <= v/5:
			e = esitoMeccanica{"estremo", "Successo estremo", nil}
		case r <= v/2:
			e = esitoMeccanica{"difficile", "Successo difficile", nil}
		case r <= v:
			e = esitoMeccanica{"successo", "Successo", nil}
		default:
			e = esitoMeccanica{"fallimento", "Fallimento", nil}
		}

	case "riserva-d10", "riserva-d6":
		facce := 10
		if in.Meccanica == "riserva-d6" {
			facce = 6
		}
		n := in.Dadi + in.Mod
		if in.Dadi < 0 || in.Dadi > 40 {
			return t, e, fmt.Errorf("da 0 a 40 dadi nella riserva")
		}
		if in.Meccanica == "riserva-d6" && in.Modo == "massimo" {
			// il dado più alto: 6 successo pieno (due 6 critico), 4-5 parziale, 1-3 fallimento; senza dadi se ne
			// tirano 2 e si tiene il più basso.
			zero := n <= 0
			if zero {
				n = 2
			}
			v := lancia(n, 6)
			sort.Sort(sort.Reverse(sort.IntSlice(v)))
			alto, sei := v[0], 0
			if zero {
				alto = v[len(v)-1]
			}
			for _, x := range v {
				if x == 6 {
					sei++
				}
			}
			t.Dadi = []gruppoDadi{{N: n, Facce: 6, Valori: v}}
			t.Totale = alto
			t.Formula = strconv.Itoa(n) + "d6, il più " + map[bool]string{true: "basso", false: "alto"}[zero]
			switch {
			case !zero && sei >= 2:
				e = esitoMeccanica{"critico", "Critico", nil}
			case alto == 6:
				e = esitoMeccanica{"successo", "Successo pieno", nil}
			case alto >= 4:
				e = esitoMeccanica{"parziale", "Successo parziale", nil}
			default:
				e = esitoMeccanica{"fallimento", "Fallimento", nil}
			}
			break
		}
		if n < 1 {
			return t, e, fmt.Errorf("serve almeno un dado nella riserva")
		}
		if n > 40 {
			n = 40
		}
		soglia := in.Soglia
		if soglia == 0 {
			soglia = 6
		}
		if soglia < 2 || soglia > facce {
			return t, e, fmt.Errorf("la soglia va da 2 a %d", facce)
		}
		v := lancia(n, facce)
		succ, dieci, uno := 0, 0, 0
		for _, x := range v {
			if x >= soglia {
				succ++
			}
			if x == facce {
				dieci++
			}
			if x == 1 {
				uno++
			}
		}
		critico := false
		if facce == 10 && in.opzione("coppieDi10") && dieci >= 2 {
			succ += (dieci / 2) * 2 // ogni coppia di 10 vale due successi in più
			critico = true
		}
		for i := 0; i < len(v); i += 20 {
			j := min(i+20, len(v))
			t.Dadi = append(t.Dadi, gruppoDadi{N: j - i, Facce: facce, Valori: v[i:j]})
		}
		d := diff(1)
		t.Totale = succ
		t.Formula = fmt.Sprintf("%dd%d, successo da %d in su, ne servono %d", n, facce, soglia, d)
		e.Successi = puntatore(succ)
		switch {
		case succ >= d && critico:
			e.Esito, e.Testo = "critico", fmt.Sprintf("Successo critico (%d successi)", succ)
		case succ >= d:
			e.Esito, e.Testo = "successo", fmt.Sprintf("Successo (%d successi)", succ)
		case succ == 0 && in.opzione("unoDisastro") && uno > 0:
			e.Esito, e.Testo = "disastro", "Disastro (nessun successo e almeno un 1)"
		default:
			e.Esito, e.Testo = "fallimento", fmt.Sprintf("Fallimento (%d successi su %d)", succ, d)
		}

	case "2d6-fasce":
		v := lancia(2, 6)
		tot := v[0] + v[1] + in.Mod
		t.Dadi = []gruppoDadi{{N: 2, Facce: 6, Valori: v}}
		t.Totale = tot
		t.Formula = "2d6 " + conSegno(in.Mod)
		switch {
		case tot >= 10:
			e = esitoMeccanica{"successo", "Successo pieno (10 o più)", nil}
		case tot >= 7:
			e = esitoMeccanica{"parziale", "Successo parziale (7-9)", nil}
		default:
			e = esitoMeccanica{"fallimento", "Fallimento (6 o meno)", nil}
		}

	case "d10-somma":
		// d10 + valore + modificatore: con 10 si tira un altro d10 e si somma, con 1 se ne tira un altro e si sottrae.
		d := tiraDado(10)
		valori, extra := []int{d}, 0
		nota := ""
		if d == 10 {
			extra = tiraDado(10)
			valori = append(valori, extra)
			nota = " (10: +" + strconv.Itoa(extra) + ")"
		} else if d == 1 {
			extra = -tiraDado(10)
			valori = append(valori, -extra)
			nota = " (1: −" + strconv.Itoa(-extra) + ")"
		}
		tot := d + extra + in.Valore + in.Mod
		t.Dadi = []gruppoDadi{{N: len(valori), Facce: 10, Valori: valori}}
		t.Totale = tot
		dv := diff(15)
		t.Formula = fmt.Sprintf("d10 + %d %s%s contro %d", in.Valore, conSegno(in.Mod), nota, dv)
		switch {
		case d == 10 && tot >= dv:
			e = esitoMeccanica{"critico", "Successo critico", nil}
		case d == 1 && tot < dv:
			e = esitoMeccanica{"disastro", "Fallimento critico", nil}
		case tot >= dv:
			e = esitoMeccanica{"successo", "Successo", nil}
		default:
			e = esitoMeccanica{"fallimento", "Fallimento", nil}
		}

	case "passi-esplosivi":
		facce := in.Dadi
		if facce == 0 {
			facce = 6
		}
		if facce != 4 && facce != 6 && facce != 8 && facce != 10 && facce != 12 {
			return t, e, fmt.Errorf("il dado del tratto è d4, d6, d8, d10 o d12")
		}
		esplodi := func(f int) ([]int, int) {
			v, s := []int{}, 0
			for i := 0; i < 10; i++ { // al massimo 10 esplosioni
				x := tiraDado(f)
				v = append(v, x)
				s += x
				if x != f {
					break
				}
			}
			return v, s
		}
		v, s := esplodi(facce)
		t.Dadi = []gruppoDadi{{N: len(v), Facce: facce, Valori: v}}
		tot := s + in.Mod
		t.Formula = fmt.Sprintf("d%d esplosivo %s", facce, conSegno(in.Mod))
		uni := v[0] == 1
		if in.opzione("jolly") {
			vj, sj := esplodi(6)
			t.Dadi = append(t.Dadi, gruppoDadi{N: len(vj), Facce: 6, Valori: vj})
			t.Formula += ", con il dado jolly"
			uni = uni && vj[0] == 1
			if sj+in.Mod > tot {
				tot = sj + in.Mod
			}
		} else {
			uni = false
		}
		t.Totale = tot
		dv := diff(4)
		switch {
		case uni:
			e = esitoMeccanica{"disastro", "Disastro (1 su entrambi i dadi)", nil}
		case tot >= dv+4:
			inc := (tot - dv) / 4
			e = esitoMeccanica{"critico", fmt.Sprintf("Successo con %d %s", inc, map[bool]string{true: "incremento", false: "incrementi"}[inc == 1]), nil}
		case tot >= dv:
			e = esitoMeccanica{"successo", "Successo", nil}
		default:
			e = esitoMeccanica{"fallimento", "Fallimento", nil}
		}

	case "fate":
		// 4 dadi con −, vuoto e +: qui un d6 (1-2 = −, 3-4 = vuoto, 5-6 = +).
		v := lancia(4, 6)
		somma, segni := 0, []string{}
		for _, x := range v {
			switch {
			case x <= 2:
				somma--
				segni = append(segni, "−")
			case x <= 4:
				segni = append(segni, "0")
			default:
				somma++
				segni = append(segni, "+")
			}
		}
		tot := somma + in.Valore + in.Mod
		t.Dadi = []gruppoDadi{{N: 4, Facce: 6, Valori: v}}
		t.Totale = tot
		dv := diff(0)
		scarto := tot - dv
		t.Formula = fmt.Sprintf("4dF [%s] + %d %s contro %d", strings.Join(segni, " "), in.Valore, conSegno(in.Mod), dv)
		switch {
		case scarto >= 3:
			e = esitoMeccanica{"critico", fmt.Sprintf("Successo con stile (+%d)", scarto), nil}
		case scarto >= 1:
			e = esitoMeccanica{"successo", fmt.Sprintf("Successo (+%d)", scarto), nil}
		case scarto == 0:
			e = esitoMeccanica{"parziale", "Pareggio", nil}
		default:
			e = esitoMeccanica{"fallimento", fmt.Sprintf("Fallimento (%d)", scarto), nil}
		}

	case "3d6-sotto":
		v := lancia(3, 6)
		tot := v[0] + v[1] + v[2]
		bersaglio := in.Valore + in.Mod
		if in.Valore < 1 || in.Valore > 40 {
			return t, e, fmt.Errorf("il valore va da 1 a 40")
		}
		t.Dadi = []gruppoDadi{{N: 3, Facce: 6, Valori: v}}
		t.Totale = tot
		t.Formula = "3d6 ≤ " + strconv.Itoa(bersaglio)
		margine := bersaglio - tot
		switch {
		case tot <= 4 || tot == 5 && bersaglio >= 15 || tot == 6 && bersaglio >= 16:
			e = esitoMeccanica{"critico", "Successo critico", nil}
		case tot == 18 || tot == 17 && bersaglio <= 15 || margine <= -10:
			e = esitoMeccanica{"disastro", "Fallimento critico", nil}
		case margine >= 0:
			e = esitoMeccanica{"successo", fmt.Sprintf("Successo (margine %d)", margine), nil}
		default:
			e = esitoMeccanica{"fallimento", fmt.Sprintf("Fallimento (margine %d)", margine), nil}
		}

	case "d20-sotto":
		bersaglio := in.Valore + in.Mod
		if in.Valore < 1 || in.Valore > 30 {
			return t, e, fmt.Errorf("il valore va da 1 a 30")
		}
		v := lancia(1, 20)
		g := gruppoDadi{N: 1, Facce: 20, Valori: v}
		if in.Modo == "vantaggio" || in.Modo == "svantaggio" { // si tirano due d20: con il vantaggio si tiene il più basso
			w := tiraDado(20)
			if (in.Modo == "vantaggio") == (w < v[0]) {
				v[0], w = w, v[0]
			}
			g.Scarto = []int{w}
			t.Modo = in.Modo
		}
		t.Dadi = []gruppoDadi{g}
		r := v[0]
		t.Totale = r
		t.Formula = "d20 ≤ " + strconv.Itoa(bersaglio)
		switch {
		case r == 1:
			e = esitoMeccanica{"critico", "Successo critico (1)", nil}
		case r == 20:
			e = esitoMeccanica{"disastro", "Fallimento critico (20)", nil}
		case r <= bersaglio:
			e = esitoMeccanica{"successo", "Successo", nil}
		default:
			e = esitoMeccanica{"fallimento", "Fallimento", nil}
		}

	case "narrativi":
		return tiraNarrativi(in.Narrativi)

	default:
		return t, e, fmt.Errorf("meccanica «%s» sconosciuta", in.Meccanica)
	}
	t.Modo = map[bool]string{true: t.Modo, false: "normale"}[t.Modo != ""]
	t.Critico = e.Esito == "critico"
	t.Fallimento = e.Esito == "disastro"
	return t, e, nil
}

// Dadi narrativi: sei tipi di dado con simboli. s = successo, f = fallimento, v = vantaggio, m = minaccia,
// T = trionfo (vale anche un successo), D = disperazione (vale anche un fallimento).
var facceNarrativi = map[string]struct {
	facce int
	nome  string
	val   []string
}{
	"abilita":    {8, "abilità", []string{"", "s", "s", "ss", "v", "v", "sv", "vv"}},
	"competenza": {12, "competenza", []string{"", "s", "s", "ss", "ss", "v", "sv", "sv", "sv", "vv", "vv", "T"}},
	"aiuto":      {6, "aiuto", []string{"", "", "s", "sv", "vv", "v"}},
	"difficolta": {8, "difficoltà", []string{"", "f", "ff", "m", "m", "m", "mm", "fm"}},
	"sfida":      {12, "sfida", []string{"", "f", "f", "ff", "ff", "m", "m", "fm", "fm", "mm", "mm", "D"}},
	"ostacolo":   {6, "ostacolo", []string{"", "", "f", "f", "m", "m"}},
}

func tiraNarrativi(n map[string]int) (t tiroDadi, e esitoMeccanica, err error) {
	tot := 0
	ordine := []string{"abilita", "competenza", "aiuto", "difficolta", "sfida", "ostacolo"}
	for k, q := range n {
		if _, ok := facceNarrativi[k]; !ok || q < 0 || q > 10 {
			return t, e, fmt.Errorf("dadi narrativi: «%s» non valido (da 0 a 10 per tipo)", k)
		}
		tot += q
	}
	if tot == 0 {
		return t, e, fmt.Errorf("scegli almeno un dado")
	}
	s, f, v, m, tr, di := 0, 0, 0, 0, 0, 0
	parti := []string{}
	for _, k := range ordine {
		q := n[k]
		if q == 0 {
			continue
		}
		d := facceNarrativi[k]
		val := lancia(q, d.facce)
		t.Dadi = append(t.Dadi, gruppoDadi{N: q, Facce: d.facce, Valori: val})
		parti = append(parti, strconv.Itoa(q)+" "+d.nome)
		for _, x := range val {
			for _, c := range d.val[x-1] {
				switch c {
				case 's':
					s++
				case 'f':
					f++
				case 'v':
					v++
				case 'm':
					m++
				case 'T':
					tr++
					s++
				case 'D':
					di++
					f++
				}
			}
		}
	}
	netti := s - f
	t.Totale = netti
	t.Formula = strings.Join(parti, ", ")
	e.Successi = puntatore(netti)
	testo := []string{}
	if netti > 0 {
		e.Esito = "successo"
		testo = append(testo, fmt.Sprintf("Successo (%d netti)", netti))
	} else {
		e.Esito = "fallimento"
		testo = append(testo, "Fallimento")
	}
	if v > m {
		testo = append(testo, fmt.Sprintf("%d vantaggi", v-m))
	} else if m > v {
		testo = append(testo, fmt.Sprintf("%d minacce", m-v))
	}
	if tr > 0 {
		testo = append(testo, fmt.Sprintf("%d trionfo", tr))
		e.Esito = map[bool]string{true: "critico", false: e.Esito}[netti > 0]
	}
	if di > 0 {
		testo = append(testo, fmt.Sprintf("%d disperazione", di))
		if netti <= 0 {
			e.Esito = "disastro"
		}
	}
	e.Testo = strings.Join(testo, " · ")
	t.Modo = "normale"
	t.Critico = e.Esito == "critico"
	t.Fallimento = e.Esito == "disastro"
	return t, e, nil
}

// wsTiraMeccanica: «dadi.meccanica», come «dadi.tira» ma con la meccanica di una famiglia.
func (s *Server) wsTiraMeccanica(c *Cliente, d json.RawMessage) (any, error) {
	var in struct {
		richiestaMeccanica
		Sessione    string `json:"sessione"`
		Etichetta   string `json:"etichetta"`
		Segreto     bool   `json:"segreto"`
		Scheda      bool   `json:"scheda"`
		Personaggio string `json:"personaggio"`
	}
	if len(d) > 4<<10 || json.Unmarshal(d, &in) != nil {
		return nil, errDatiNonValidi
	}
	etich, err := rigaSingola(in.Etichetta, "Descrizione", 0, 40)
	if err != nil {
		return nil, err
	}
	t, e, err := tiraMeccanica(in.richiestaMeccanica)
	if err != nil {
		return nil, errore(http.StatusBadRequest, "Tiro non valido: "+err.Error()+".")
	}
	t.Chi, t.ChiID, t.Etichetta, t.Mod, t.Scheda = c.U.Nome, c.U.ID, etich, in.Mod, in.Scheda
	t.Segreto = in.Segreto && c.U.Master()
	t.Meccanica, t.Esito, t.EsitoTesto, t.Successi = in.Meccanica, e.Esito, e.Testo, e.Successi
	return s.inviaTiro(c.U, t, in.Sessione, in.Personaggio), nil
}
