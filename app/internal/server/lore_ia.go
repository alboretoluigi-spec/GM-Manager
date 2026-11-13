package server

// Il Codice della campagna per l'assistente IA e per «Parla con l'antagonista» (fase L4, 2026-11-12). A ogni domanda Gaster Bless
// aggiunge alle istruzioni le schede più pertinenti per quel gruppo, entro un limite di caratteri (con Ollama più
// stretto): se il Codice intero ci sta, tutto; altrimenti, in ordine di punteggio:
//   - schede nominate nella domanda (nome o alias) e nell'ultimo messaggio prima;
//   - schede con un promemoria aperto per il gruppo (lore_promemoria.go);
//   - schede del luogo dove si trova il segnalino del gruppo (stesso nome, Luogo/Dove/Regione, collegamenti);
//   - schede che il gruppo conosce o ha incontrato (stato per gruppo);
//   - poi le schede collegate a quelle scelte e le «Decisioni aperte» che le riguardano.
// Con le regole: approvato / proposta / da decidere, canone / romanzi / campagna e variante, segreti del Master.
// Lo stato per gruppo è solo quello del gruppo della conversazione (gruppi separati).

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxLoreIA       = 12000 // caratteri delle schede aggiunte alla richiesta
	maxLoreOllama   = 7000  // con Ollama il contesto è più piccolo
	maxTestoSchedaI = 1500  // caratteri per ognuno dei due testi di una scheda
)

const regoleLoreIA = `CODICE DELLA CAMPAGNA (la lore scritta dal Master; qui le schede più pertinenti a questa richiesta). Come usarlo:
- Etichetta «approvato» = fatto stabilito; «proposta» = idea non ancora approvata (presentala come proposta); «da decidere» = non stabilito: non trattarlo come un fatto e non completarlo da solo; se utile, proponi opzioni dichiarandole tali.
- «canone» = dal modulo ufficiale; «romanzi» = narrativa espansa (usala solo se il Master la adotta); «campagna» e «variante» = decisioni del Master: prevalgono sempre sul canone. «Variante di:» indica il fatto del modulo che cambia.
- «Per il Master» contiene segreti: non metterli in bocca ai PNG né nei testi per i giocatori, salvo che il Master lo chieda. «Per i giocatori» è ciò che i personaggi possono sapere.
- «Per questo gruppo» dice cosa ha già conosciuto o incontrato il gruppo di questa conversazione; gli altri gruppi non contano qui.
- Se una cosa non è nel Codice né nei dati di Gaster Bless, non darla per stabilita.`

var etichetteStatoIA = map[string]string{"conosciuto": "conosciuto", "incontrato": "incontrato o avvenuto", "concluso": "concluso (morto, distrutto, risolto)"}

// analisiLore: il Codice visto da un gruppo, con il punteggio e i motivi di ogni scheda (anche per la pagina di
// preparazione della sessione, preparazione.go)
type analisiLore struct {
	l      []schedaLore
	stati  map[string]statoLore
	prom   map[string]bool
	luogo  string
	punti  map[string]float64
	motivi map[string][]string
}

func (s *Server) analizzaLore(gruppo, domanda string) *analisiLore {
	l, err := s.caricaLore("")
	if err != nil || len(l) == 0 {
		return nil
	}
	a := &analisiLore{l: l, stati: map[string]statoLore{}, prom: map[string]bool{}, punti: map[string]float64{}, motivi: map[string][]string{}}
	if rows, err := s.DB.Query(`SELECT scheda_id, stato, nota FROM lore_stato WHERE gruppo_id = ?`, gruppo); err == nil {
		for rows.Next() {
			var id string
			var x statoLore
			rows.Scan(&id, &x.Stato, &x.Nota)
			a.stati[id] = x
		}
		rows.Close()
	}
	if rows, err := s.DB.Query(`SELECT scheda_id FROM lore_promemoria WHERE gruppo_id = ? AND chiuso = 0`, gruppo); err == nil {
		for rows.Next() {
			var id string
			rows.Scan(&id)
			a.prom[id] = true
		}
		rows.Close()
	}
	a.luogo = s.luogoDelGruppo(gruppo)
	motivo := func(id, m string) {
		if !slices.Contains(a.motivi[id], m) {
			a.motivi[id] = append(a.motivi[id], m)
		}
	}

	nomi := func(x schedaLore) []string {
		out := []string{normalizza(x.Nome)}
		for k, v := range x.Campi {
			if strings.EqualFold(strings.TrimSpace(k), "alias") {
				for _, a := range strings.FieldsFunc(v, func(r rune) bool { return r == ';' || r == ',' }) {
					if a = normalizza(a); a != "" {
						out = append(out, a)
					}
				}
			}
		}
		return out
	}
	dom := normalizza(domanda)
	punti := a.punti
	perNome := map[string]string{} // nome o alias normalizzato → id
	nomeDi := map[string]string{}
	for _, x := range l {
		nomeDi[x.ID] = x.Nome
		for _, n := range nomi(x) {
			perNome[n] = x.ID
		}
	}
	for _, x := range l {
		p := 0.0
		for _, n := range nomi(x) {
			if utf8.RuneCountInString(n) >= 4 && strings.Contains(dom, n) {
				p += 100
				motivo(x.ID, "nominata")
				break
			}
		}
		if a.prom[x.ID] {
			p += 60
			motivo(x.ID, "promemoria aperto")
		}
		if a.luogo != "" {
			lu := normalizza(a.luogo)
			vicino := false
			for _, n := range nomi(x) {
				if n == lu {
					vicino = true
				}
			}
			for k, v := range x.Campi {
				if kk := strings.ToLower(strings.TrimSpace(k)); (kk == "luogo" || kk == "dove" || kk == "regione") && strings.Contains(normalizza(v), lu) {
					vicino = true
				}
			}
			for _, c := range x.Collegamenti {
				if normalizza(c) == lu {
					vicino = true
				}
			}
			if vicino {
				p += 50
				motivo(x.ID, "dove si trova il gruppo ("+a.luogo+")")
			}
		}
		if st := a.stati[x.ID].Stato; st == "conosciuto" || st == "incontrato" {
			p += 20
			motivo(x.ID, map[string]string{"conosciuto": "già nota al gruppo", "incontrato": "già vista o avvenuta per il gruppo"}[st])
		}
		punti[x.ID] = p
	}
	// collegate alle schede scelte (un terzo del punteggio) e decisioni aperte che le citano
	extra := map[string]float64{}
	da := map[string]string{}
	for _, x := range l {
		if punti[x.ID] <= 0 {
			continue
		}
		for _, c := range x.Collegamenti {
			if id, ok := perNome[normalizza(c)]; ok && punti[x.ID]/3 > extra[id] {
				extra[id] = punti[x.ID] / 3
				da[id] = x.Nome
			}
		}
	}
	for id, v := range extra {
		punti[id] += v
		motivo(id, "collegata a «"+da[id]+"»")
	}
	// le «Decisioni aperte» che citano una scheda scelta (anche scelta come collegata)
	dec := map[string]float64{}
	for _, x := range l {
		if x.Tipo == "segreto" && strings.Contains(strings.ToLower(x.Nome), "decisioni aperte") {
			for _, c := range x.Collegamenti {
				if id, ok := perNome[normalizza(c)]; ok && punti[id] > 0 && punti[id]/2 > dec[x.ID] {
					dec[x.ID] = punti[id] / 2
					da[x.ID] = nomeDi[id]
				}
			}
		}
	}
	for id, v := range dec {
		punti[id] += v
		motivo(id, "decisioni aperte su «"+da[id]+"»")
	}
	return a
}

// contestoLore: il blocco del Codice per il Cronista della campagna e per l'antagonista (solo le schede della campagna: le
// schede del canone vanno all'Archivista, identita_ia.go). "" se non ci sono schede.
func (s *Server) contestoLore(gruppo, domanda string, max int) string {
	return s.contestoLoreDi(gruppo, domanda, max, "campagna", false, regoleLoreIA)
}

// dominioScheda: «canone» per le schede che riassumono i testi ufficiali (etichette canone, romanzi, tomo interattivo,
// compendio) senza etichette della campagna; «campagna» per tutte le altre (le scrive il Master per la sua campagna).
func dominioScheda(x schedaLore) string {
	canone := false
	for _, e := range x.Etichette {
		switch normalizza(e) {
		case "campagna", "variante", "homebrew":
			return "campagna"
		case "canone", "romanzi", "romanzo", "tomo interattivo", "tomo", "compendio", "modulo":
			canone = true
		}
	}
	for k := range x.Campi {
		if strings.EqualFold(strings.TrimSpace(k), "variante di") {
			return "campagna"
		}
	}
	if canone {
		return "canone"
	}
	return "campagna"
}

// contestoLoreDi: le schede di un dominio ("" = tutte), le più pertinenti entro max caratteri, sotto l'intestazione data.
// soloPertinenti: solo le schede con un motivo (per il consulto tra le due identità, che deve restare breve).
// Senza gruppo (Archivista del canone) non si scrive lo stato per gruppo.
func (s *Server) contestoLoreDi(gruppo, domanda string, max int, dominio string, soloPertinenti bool, intestazione string) string {
	a := s.analizzaLore(gruppo, domanda)
	if a == nil {
		return ""
	}
	stati, prom, luogo, punti := a.stati, a.prom, a.luogo, a.punti
	var l []schedaLore
	for _, x := range a.l {
		if (dominio == "" || dominioScheda(x) == dominio) && (!soloPertinenti || punti[x.ID] > 0) {
			l = append(l, x)
		}
	}
	if len(l) == 0 {
		return ""
	}
	testi := make([]string, len(l))
	tot := 0
	for i, x := range l {
		testi[i] = testoSchedaIA(x, stati[x.ID], prom[x.ID], gruppo != "")
		tot += utf8.RuneCountInString(testi[i])
	}
	ordine := make([]int, len(l))
	for i := range ordine {
		ordine[i] = i
	}
	sort.SliceStable(ordine, func(a, b int) bool { return punti[l[ordine[a]].ID] > punti[l[ordine[b]].ID] })
	var b strings.Builder
	b.WriteString(intestazione)
	if luogo != "" && gruppo != "" {
		b.WriteString("\nIl segnalino del gruppo è ora a: " + luogo + ".")
	}
	usati, scelti := 0, 0
	for _, i := range ordine {
		x := l[i]
		if tot > max && punti[x.ID] <= 0 { // il Codice non ci sta tutto: solo le schede pertinenti
			break
		}
		n := utf8.RuneCountInString(testi[i])
		if usati+n > max {
			continue
		}
		b.WriteString("\n\n" + testi[i])
		usati += n
		scelti++
	}
	if scelti == 0 {
		return ""
	}
	if scelti < len(l) && !soloPertinenti {
		fmt.Fprintf(&b, "\n\n(Mostrate %d schede su %d del Codice: le altre non sembrano pertinenti a questa richiesta. Se il Master nomina una cosa che qui non c'è, chiedigli di più.)", scelti, len(l))
	}
	return b.String()
}

func testoSchedaIA(x schedaLore, st statoLore, prom, conGruppo bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### [%s] %s", titoloTipoLore[x.Tipo], x.Nome)
	if len(x.Etichette) > 0 {
		b.WriteString(" — etichette: " + strings.Join(x.Etichette, ", "))
	}
	if !conGruppo {
		// Archivista del canone: nessun gruppo
	} else if e := etichetteStatoIA[st.Stato]; e != "" {
		b.WriteString(" — per questo gruppo: " + e)
		if st.Nota != "" {
			b.WriteString(" (" + st.Nota + ")")
		}
	} else {
		b.WriteString(" — per questo gruppo: ancora sconosciuto")
	}
	if prom {
		b.WriteString(" — PROMEMORIA APERTO: l'evento è pronto a scattare per questo gruppo")
	}
	if len(x.Campi) > 0 {
		k := make([]string, 0, len(x.Campi))
		for c := range x.Campi {
			k = append(k, c)
		}
		sort.Strings(k)
		var p []string
		for _, c := range k {
			p = append(p, c+": "+x.Campi[c])
		}
		b.WriteString("\n" + strings.Join(p, "; "))
	}
	if len(x.Collegamenti) > 0 {
		b.WriteString("\nCollegamenti: " + strings.Join(x.Collegamenti, "; "))
	}
	if x.TestoMaster != "" {
		b.WriteString("\nPer il Master: " + accorciaContesto(x.TestoMaster, maxTestoSchedaI))
	}
	if x.TestoGiocatori != "" {
		b.WriteString("\nPer i giocatori: " + accorciaContesto(x.TestoGiocatori, maxTestoSchedaI))
	}
	return b.String()
}

// luogoDelGruppo: la località della mappa più vicina al segnalino principale del gruppo (entro 40 px), o "".
func (s *Server) luogoDelGruppo(gruppo string) string {
	var pos string
	s.DB.QueryRow(`SELECT posizione FROM stato_mappa WHERE gruppo_id = ?`, gruppo).Scan(&pos)
	d := leggiSegnalini(pos)
	if len(d.Segnalini) == 0 {
		return ""
	}
	p := d.Segnalini[0]
	best, bd := "", raggioArrivoLore
	for _, l := range s.localita() {
		if dd := math.Hypot(l.X-p.X, l.Y-p.Y); dd <= bd {
			best, bd = l.N, dd
		}
	}
	return best
}
