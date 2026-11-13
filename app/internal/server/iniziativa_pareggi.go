package server

// Tiri d'iniziativa (richieste dell'utente, 2026-11-12; migrazione 031):
//   - 20 naturale sul d20: il combattente agisce per primo (davanti a ogni totale); 1 naturale: per ultimo;
//   - pareggio (stesso totale) tra un PG e un mostro o PNG: decide il Master l'ordine;
//   - pareggio tra soli PG: decidono i loro giocatori (uno qualsiasi di loro, o il Master);
//   - pareggio tra soli mostri e PNG: decide la Destrezza, come prima.
// L'ordine deciso sta in combattenti.ordine_pari (più alto prima); quando cambia l'iniziativa di qualcuno il suo valore si
// azzera e il pareggio torna da decidere. «Inizia» non parte finché ci sono pareggi da decidere (il Master può forzare:
// vale la Destrezza).

import (
	"encoding/json"
	"net/http"
	"slices"
	"sort"
	"strings"
)

// prioritaNaturale: 1 con il 20 naturale, -1 con l'1 naturale, 0 altrimenti.
func prioritaNaturale(c *vistaCombattente) int {
	if c.Naturale == nil {
		return 0
	}
	switch *c.Naturale {
	case 20:
		return 1
	case 1:
		return -1
	}
	return 0
}

type membroPari struct {
	ID   string `json:"id"`
	Nome string `json:"nome"`
	Tipo string `json:"tipo"`
	Mio  bool   `json:"mio,omitempty"`
}

// gruppoPari: un pareggio da decidere. Chi: «master» (c'è almeno un mostro o PNG) o «giocatori» (solo PG).
type gruppoPari struct {
	Valore int64        `json:"valore"`
	Chi    string       `json:"chi"`
	Membri []membroPari `json:"membri"`
	Decide bool         `json:"decide,omitempty"` // chi guarda la vista può decidere
}

// pareggi: i gruppi di combattenti con lo stesso totale (e la stessa priorità del naturale) con almeno un PG, non ancora
// decisi. Nell'ordine del tracker.
func pareggi(tutti []*vistaCombattente) [][]*vistaCombattente {
	gruppi := map[[2]int64][]*vistaCombattente{}
	var chiavi [][2]int64
	for _, c := range tutti {
		if c.Iniziativa == nil {
			continue
		}
		k := [2]int64{int64(prioritaNaturale(c)), *c.Iniziativa}
		if _, ok := gruppi[k]; !ok {
			chiavi = append(chiavi, k)
		}
		gruppi[k] = append(gruppi[k], c)
	}
	var out [][]*vistaCombattente
	for _, k := range chiavi {
		g := gruppi[k]
		if len(g) < 2 || !slices.ContainsFunc(g, func(c *vistaCombattente) bool { return c.Tipo == "pg" }) {
			continue
		}
		visti, deciso := map[int]bool{}, true
		for _, c := range g {
			if c.ordinePari <= 0 || visti[c.ordinePari] {
				deciso = false
			}
			visti[c.ordinePari] = true
		}
		if !deciso {
			out = append(out, g)
		}
	}
	return out
}

func chiDecide(g []*vistaCombattente) string {
	if slices.ContainsFunc(g, func(c *vistaCombattente) bool { return c.Tipo != "pg" }) {
		return "master"
	}
	return "giocatori"
}

// pareggiPerRuolo: il Master li vede tutti; i giocatori quelli in cui c'è un loro PG (senza i mostri nascosti).
func pareggiPerRuolo(tutti []*vistaCombattente, ruolo, uid string) []gruppoPari {
	var out []gruppoPari
	for _, g := range pareggi(tutti) {
		chi := chiDecide(g)
		suo := slices.ContainsFunc(g, func(c *vistaCombattente) bool { return c.Proprietario != "" && c.Proprietario == uid })
		if ruolo != "master" && !suo {
			continue
		}
		gp := gruppoPari{Valore: *g[0].Iniziativa, Chi: chi, Decide: ruolo == "master" || chi == "giocatori" && suo}
		for _, c := range g {
			if c.Nascosto && ruolo != "master" {
				continue
			}
			gp.Membri = append(gp.Membri, membroPari{ID: c.ID, Nome: c.Nome, Tipo: c.Tipo, Mio: c.Proprietario != "" && c.Proprietario == uid})
		}
		out = append(out, gp)
	}
	return out
}

// ordinePari («iniz.ordine_pari»): l'ordine di un pareggio, dal primo all'ultimo.
func (s *Server) ordinePari(u *Utente, sessione, comb string, master bool, tutti []*vistaCombattente, ordine []string, registro func(string)) (any, error) {
	if len(ordine) < 2 || len(ordine) > 20 {
		return nil, errDatiNonValidi
	}
	for _, g := range pareggi(tutti) {
		ids := []string{}
		for _, c := range g {
			ids = append(ids, c.ID)
		}
		a, b := slices.Clone(ids), slices.Clone(ordine)
		sort.Strings(a)
		sort.Strings(b)
		if !slices.Equal(a, b) {
			continue
		}
		suo := slices.ContainsFunc(g, func(c *vistaCombattente) bool { return c.Proprietario == u.ID })
		if !master && !(chiDecide(g) == "giocatori" && suo) {
			return nil, errore(http.StatusForbidden, "Questo pareggio lo decide il Master.")
		}
		nomi := []string{}
		for i, id := range ordine {
			s.DB.Exec(`UPDATE combattenti SET ordine_pari = ? WHERE id = ? AND combattimento_id = ?`, len(ordine)-i, id, comb)
			for _, c := range g {
				if c.ID == id {
					nomi = append(nomi, c.Nome)
				}
			}
		}
		registro("pareggio a " + itoa(int(*g[0].Iniziativa)) + ": " + strings.Join(nomi, ", poi ") + " (deciso da " + u.Nome + ")")
		s.pubblicaCombattimento(sessione)
		return map[string]any{"fatto": true}, nil
	}
	return nil, errore(http.StatusConflict, "Il pareggio è cambiato o è già deciso: riapri il tracker.")
}

// testoNaturale: nota per il registro.
func testoNaturale(d20 int) string {
	switch d20 {
	case 20:
		return " (20 naturale: agisce per primo)"
	case 1:
		return " (1 naturale: agisce per ultimo)"
	}
	return ""
}

// validaSchedaMostro: la scheda della creatura dal bestiario (oggetto JSON, al massimo 64 KB).
func validaSchedaMostro(raw json.RawMessage) string {
	if len(raw) == 0 || len(raw) > 64<<10 {
		return ""
	}
	var o map[string]any
	if json.Unmarshal(raw, &o) != nil || o == nil {
		return ""
	}
	return string(raw)
}

// apiSchedaCombattente (solo il Master): la scheda intera di un mostro dello scontro.
func (s *Server) apiSchedaCombattente(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var sc string
	if s.DB.QueryRow(`SELECT c.scheda_mostro FROM combattenti c JOIN combattimenti k ON k.id = c.combattimento_id
		WHERE c.id = ? AND k.sessione_id = ?`, r.PathValue("c"), r.PathValue("id")).Scan(&sc) != nil {
		return nil, errNonTrovato
	}
	if sc == "" {
		return map[string]any{"scheda": nil}, nil
	}
	return map[string]any{"scheda": json.RawMessage(sc)}, nil
}
