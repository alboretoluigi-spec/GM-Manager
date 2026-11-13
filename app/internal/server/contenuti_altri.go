package server

// Altri contenuti dei pacchetti (fase 3c, passo C3, docs/FORMATO_CONTENUTI.md): incantesimi concessi da sottoclassi,
// razze e terreni; strumenti; prezzi e catalogo dell'equipaggiamento; mostri del bestiario; oggetti magici;
// professioni. I primi arrivano alla scheda (dati.js, incantesimi_auto.js, strumenti.js), gli altri ai cataloghi del
// server (carta/mostri.json, regole/professioni.json) e all'archivio degli oggetti magici.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type incSempreFile struct {
	Modo       string                         `json:"modo"`
	Livelli    map[string][]string            `json:"livelli"`
	Trucchetti map[string][]string            `json:"trucchetti"`
	PerTerreno map[string]map[string][]string `json:"perTerreno"`
}

type incRazzaFile struct {
	Caratteristica string              `json:"caratteristica"`
	Livelli        map[string][]string `json:"livelli"`
}

type strumentoFile struct {
	ID         string   `json:"id"`
	Nome       string   `json:"nome"`
	Alias      []string `json:"alias"`
	Categoria  string   `json:"categoria"`
	Costo      string   `json:"costo"`
	Abilita    []string `json:"abilita"`
	Uso        string   `json:"uso"`
	Puo        []string `json:"puo"`
	NonPuo     []string `json:"nonPuo"`
	Operazioni []struct {
		Nome           string `json:"nome"`
		Caratteristica string `json:"caratteristica"`
		CD             *int   `json:"cd"`
		Abilita        string `json:"abilita"`
	} `json:"operazioni"`
	Fabbricazione []struct {
		Nome      string  `json:"nome"`
		Materiali float64 `json:"materiali"`
		Tempo     string  `json:"tempo"`
		Prodotto  string  `json:"prodotto"`
		Generico  bool    `json:"generico"`
	} `json:"fabbricazione"`
}

type categoriaEquipFile struct {
	Categoria string `json:"categoria"`
	Voci      []struct {
		Nome  string `json:"nome"`
		Costo string `json:"costo"`
		Nota  string `json:"nota"`
	} `json:"voci"`
}

type mostroFile struct {
	ID          string          `json:"id"`
	Nome        string          `json:"nome"`
	NomeInglese string          `json:"nomeInglese"`
	Fonte       string          `json:"fonte"`
	Note        string          `json:"note"`
	Scheda      json.RawMessage `json:"scheda"`
}

// oggettoPacchetto: un oggetto magico di un pacchetto attivo (dati già controllati, nel formato della scheda).
type oggettoPacchetto struct {
	ID, Nome, Pacchetto string
	Dati                []byte
}

// professioniPacchetto: la parte «professioni» di un pacchetto, con la stessa struttura di regole/professioni.json.
type professioniPacchetto struct {
	Professioni   map[string]json.RawMessage `json:"professioni"`
	Artigiani     []json.RawMessage          `json:"artigiani"`
	Disponibilita []json.RawMessage          `json:"disponibilita"`
	Regole        map[string]string          `json:"-"`
}

// livelliNomi: {"3": ["A", "B"]} con livelli da 1 a 20 e nomi non vuoti.
func (c *controllore) livelliNomi(campo string, m map[string][]string) map[string][]string {
	out := map[string][]string{}
	for k, v := range m {
		l, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil || l < 1 || l > 20 {
			c.err("«%s»: «%s» non è un livello da 1 a 20", campo, k)
			continue
		}
		if n := c.elencoTesti(campo, v); len(n) > 0 {
			out[strconv.Itoa(l)] = n
		}
	}
	return out
}

func (c *controllore) incantesimiSempre(nomeSub string, x *incSempreFile, d *datiScheda) {
	m := map[string]any{}
	for k, v := range c.livelliNomi("incantesimiSempre.livelli", x.Livelli) {
		m[k] = v
	}
	switch strings.ToLower(strings.TrimSpace(x.Modo)) {
	case "", "preparati":
		m["modo"] = "prep"
	case "conosciuti":
		m["modo"] = "con"
	default:
		c.err("«incantesimiSempre.modo» deve essere «preparati» o «conosciuti»")
	}
	if len(x.Trucchetti) > 0 {
		m["tr"] = c.livelliNomi("incantesimiSempre.trucchetti", x.Trucchetti)
	}
	if len(x.PerTerreno) > 0 {
		m["terra"] = true
		for t, liv := range x.PerTerreno {
			t = c.testo("incantesimiSempre.perTerreno", t, true)
			d.LandSpells[t] = c.livelliNomi("incantesimiSempre.perTerreno", liv)
		}
	}
	d.SubSpells[nomeSub] = m
}

func (c *controllore) incantesimiRazziali(x *incRazzaFile) map[string]any {
	m := map[string]any{}
	for k, v := range c.livelliNomi("incantesimiRazziali.livelli", x.Livelli) {
		m[k] = v
	}
	if x.Caratteristica != "" {
		m["ab"] = c.caratteristica("incantesimiRazziali.caratteristica", x.Caratteristica)
	}
	return m
}

func convertiStrumento(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x strumentoFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	if !reIDContenuto.MatchString(x.ID) {
		c.err("«id» deve essere fatto di lettere minuscole senza accenti, cifre, - o _ (es. «vetraio»)")
	}
	cat := strings.ToLower(strings.TrimSpace(x.Categoria))
	if cat == "" {
		cat = "artigiano"
	}
	c.codici("categoria", []string{cat}, "artigiano", "kit", "gioco", "musica", "veicolo")
	ops := []map[string]any{}
	for _, o := range x.Operazioni {
		op := map[string]any{"n": c.testo("operazioni.nome", o.Nome, true), "car": c.caratteristica("operazioni.caratteristica", o.Caratteristica), "cd": nil}
		if o.CD != nil {
			op["cd"] = c.intervallo("operazioni.cd", *o.CD, 1, 40)
		}
		if o.Abilita != "" {
			op["ab"] = c.abilita("operazioni.abilita", []string{o.Abilita})[0]
		}
		ops = append(ops, op)
	}
	fab := []map[string]any{}
	for _, f := range x.Fabbricazione {
		v := map[string]any{"n": c.testo("fabbricazione.nome", f.Nome, true), "mo": f.Materiali, "tempo": c.testo("fabbricazione.tempo", f.Tempo, false),
			"prodotto": c.testo("fabbricazione.prodotto", f.Prodotto, false)}
		if f.Generico {
			v["generico"] = true
		}
		if f.Materiali < 0 {
			c.err("«fabbricazione.materiali» non può essere negativo")
		}
		fab = append(fab, v)
	}
	d.Strumenti = append(d.Strumenti, map[string]any{"id": x.ID, "n": c.testo("nome", x.Nome, true), "alias": c.elencoTesti("alias", x.Alias), "cat": cat,
		"costo": c.testo("costo", x.Costo, false), "abil": c.abilita("abilita", x.Abilita), "uso": c.testo("uso", x.Uso, false),
		"puo": c.elencoTesti("puo", x.Puo), "nonpuo": c.elencoTesti("nonPuo", x.NonPuo), "ops": ops, "fab": fab})
	return nil
}

func convertiEquipaggiamento(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x categoriaEquipFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	nome := c.testo("categoria", x.Categoria, true)
	voci := [][]string{}
	for _, v := range x.Voci {
		voci = append(voci, []string{c.testo("voci.nome", v.Nome, true), c.testo("voci.costo", v.Costo, false), c.testo("voci.nota", v.Nota, false)})
	}
	d.EquipCat = append(d.EquipCat, categoriaEquip{ID: idDaNome(nome), N: nome, Voci: voci})
	return nil
}

type categoriaEquip struct {
	ID   string     `json:"id"`
	N    string     `json:"n"`
	Voci [][]string `json:"voci"`
}

// idDaNome: identificativo leggibile da un nome («Equipaggiamento d'avventura» → «equipaggiamento-d-avventura»).
func idDaNome(s string) string {
	var b strings.Builder
	trat := false
	for _, r := range strings.ToLower(senzaAccenti.Replace(s)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			trat = false
		} else if !trat && b.Len() > 0 {
			b.WriteByte('-')
			trat = true
		}
	}
	return strings.TrimSuffix(accorcia(b.String(), 40), "-")
}

func convertiMostro(c *controllore, raw json.RawMessage, d *datiScheda) error {
	var x mostroFile
	if err := decodifica(raw, &x); err != nil {
		return err
	}
	nome := c.testo("nome", x.Nome, true)
	id := strings.TrimSpace(x.ID)
	if id == "" {
		id = "pc-" + idDaNome(nome)
	} else if !reIDContenuto.MatchString(id) {
		c.err("«id» deve essere fatto di lettere minuscole senza accenti, cifre, - o _")
	}
	if len(x.Scheda) == 0 || string(x.Scheda) == "null" {
		c.err("manca «scheda» (i valori della creatura)")
		return nil
	}
	sc, err := validaScheda(x.Scheda)
	if err != nil {
		msg := err.Error()
		if e, ok := err.(*ErrAPI); ok {
			msg = e.Msg
		}
		var tmp schedaCreatura
		if de := decodifica(x.Scheda, &tmp); de != nil {
			msg = "«scheda»: " + de.Error()
		}
		c.err("%s", strings.TrimSuffix(msg, "."))
		return nil
	}
	d.Mostri = append(d.Mostri, map[string]any{"id": id, "nome": nome, "en": c.testo("nomeInglese", x.NomeInglese, false), "cat": "mostro",
		"fonte": c.testo("fonte", x.Fonte, false), "note": c.testo("note", x.Note, false), "pag": []int{}, "scheda": json.RawMessage(sc)})
	return nil
}

func convertiOggetto(c *controllore, raw json.RawMessage, d *datiScheda) error {
	o, b, err := validaOggettoMagico(raw)
	if err != nil {
		msg := err.Error()
		if e, ok := err.(*ErrAPI); ok {
			msg = e.Msg
		}
		c.err("%s", strings.TrimSuffix(msg, "."))
		return nil
	}
	d.Oggetti = append(d.Oggetti, oggettoPacchetto{ID: o.ID, Nome: o.Nome, Dati: b})
	return nil
}

// professioni: stessa struttura del catalogo regole/professioni.json (voci «professioni», «artigiani», «disponibilita»
// e i testi «regoleRaccolta», «regoleEstrazione», «regoleCreazione», «regoleUso»).
func (c *controllore) professioni(raw json.RawMessage, d *datiScheda) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	c.dove = "professioni: "
	defer func() { c.dove = "" }()
	var x struct {
		Professioni   map[string]json.RawMessage `json:"professioni"`
		Artigiani     []json.RawMessage          `json:"artigiani"`
		Disponibilita []json.RawMessage          `json:"disponibilita"`
		Raccolta      string                     `json:"regoleRaccolta"`
		Estrazione    string                     `json:"regoleEstrazione"`
		Creazione     string                     `json:"regoleCreazione"`
		Uso           string                     `json:"regoleUso"`
	}
	if err := decodifica(raw, &x); err != nil {
		c.err("%v", err)
		return 0
	}
	p := &professioniPacchetto{Professioni: map[string]json.RawMessage{}, Regole: map[string]string{}}
	for id, r := range x.Professioni {
		if !reIDContenuto.MatchString(id) {
			c.err("l'id della professione «%s» deve essere fatto di lettere minuscole senza accenti, cifre, - o _", id)
		}
		var pd profDef
		if err := json.Unmarshal(r, &pd); err != nil {
			c.err("professione «%s»: %v", id, erroreJSONLeggibile(err))
			continue
		}
		if strings.TrimSpace(pd.N) == "" {
			c.err("professione «%s»: manca «n» (il nome)", id)
		}
		visti := map[string]bool{}
		for _, f := range pd.Formule {
			if f.ID == "" || visti[f.ID] {
				c.err("professione «%s»: ogni formula deve avere un «id» diverso", id)
			}
			visti[f.ID] = true
		}
		for _, l := range [][]ingrDef{pd.Ingredienti, pd.Speciali} {
			for _, i := range l {
				if i.ID == "" || strings.TrimSpace(i.N) == "" {
					c.err("professione «%s»: ogni ingrediente deve avere «id» e «n»", id)
				}
			}
		}
		p.Professioni[id] = r
	}
	for _, a := range x.Artigiani {
		var v struct{ ID, N string }
		if json.Unmarshal(a, &v) != nil || !reIDContenuto.MatchString(v.ID) || strings.TrimSpace(v.N) == "" {
			c.err("ogni voce di «artigiani» deve avere «id» (minuscole senza accenti) e «n»")
		}
	}
	for _, a := range x.Disponibilita {
		var v struct {
			ID string
			N  string
			CD int
		}
		if json.Unmarshal(a, &v) != nil || v.ID == "" || v.N == "" {
			c.err("ogni voce di «disponibilita» deve avere «id», «n» e «cd»")
		}
	}
	p.Artigiani, p.Disponibilita = x.Artigiani, x.Disponibilita
	for k, v := range map[string]string{"regoleRaccolta": x.Raccolta, "regoleEstrazione": x.Estrazione, "regoleCreazione": x.Creazione, "regoleUso": x.Uso} {
		if v = c.testo(k, v, false); v != "" {
			p.Regole[k] = v
		}
	}
	d.Professioni = p
	return len(p.Professioni) + len(p.Artigiani)
}

// unisciAltri: come unisci, per i tipi del passo C3.
func (d *datiScheda) unisciAltri(o *datiScheda, sovr *[]string) {
	for k, v := range o.SubSpells {
		d.SubSpells[k] = v
	}
	for k, v := range o.LandSpells {
		d.LandSpells[k] = v
	}
	for k, v := range o.RaceSpells {
		d.RaceSpells[k] = v
	}
	for k, v := range o.RaceCantrip {
		d.RaceCantrip[k] = v
	}
	for k, v := range o.Prezzi {
		d.Prezzi[k] = v
	}
	for _, c := range o.EquipCat {
		trovata := false
		for i := range d.EquipCat {
			if d.EquipCat[i].ID == c.ID {
				trovata = true
				pos := map[string]int{}
				for j, v := range d.EquipCat[i].Voci {
					pos[strings.ToLower(v[0])] = j
				}
				for _, v := range c.Voci {
					if j, ok := pos[strings.ToLower(v[0])]; ok {
						d.EquipCat[i].Voci[j] = v
					} else {
						d.EquipCat[i].Voci = append(d.EquipCat[i].Voci, v)
					}
				}
			}
		}
		if !trovata {
			d.EquipCat = append(d.EquipCat, c)
		}
	}
	perID := func(tipo string, a *[]map[string]any, b []map[string]any) {
		pos := map[string]int{}
		for i, v := range *a {
			pos[fmt.Sprint(v["id"])] = i
		}
		for _, v := range b {
			if i, ok := pos[fmt.Sprint(v["id"])]; ok {
				(*a)[i] = v
				*sovr = append(*sovr, tipo+" «"+fmt.Sprint(v["id"])+"»")
				continue
			}
			pos[fmt.Sprint(v["id"])] = len(*a)
			*a = append(*a, v)
		}
	}
	perID("strumento", &d.Strumenti, o.Strumenti)
	perID("mostro", &d.Mostri, o.Mostri)
	pos := map[string]int{}
	for i, v := range d.Oggetti {
		pos[v.ID] = i
	}
	for _, v := range o.Oggetti {
		if i, ok := pos[v.ID]; ok {
			d.Oggetti[i] = v
			*sovr = append(*sovr, "oggetto magico «"+v.Nome+"»")
			continue
		}
		pos[v.ID] = len(d.Oggetti)
		d.Oggetti = append(d.Oggetti, v)
	}
	if o.Professioni != nil {
		if d.Professioni == nil {
			d.Professioni = &professioniPacchetto{Professioni: map[string]json.RawMessage{}, Regole: map[string]string{}}
		}
		for k, v := range o.Professioni.Professioni {
			if _, ok := d.Professioni.Professioni[k]; ok {
				*sovr = append(*sovr, "professione «"+k+"»")
			}
			d.Professioni.Professioni[k] = v
		}
		d.Professioni.Artigiani = append(d.Professioni.Artigiani, o.Professioni.Artigiani...)
		d.Professioni.Disponibilita = append(d.Professioni.Disponibilita, o.Professioni.Disponibilita...)
		for k, v := range o.Professioni.Regole {
			d.Professioni.Regole[k] = v
		}
	}
}

// nomiIncantesimiInglesi: gli incantesimi concessi si indicano con il nome (o l'altro nome) di un incantesimo; la scheda
// li cerca con il nome inglese dell'archivio (SPELLS). Si convertono dopo aver unito tutti i pacchetti attivi.
func (d *datiScheda) nomiIncantesimiInglesi() {
	en := map[string]string{}
	for _, s := range d.Incantesimi {
		en[strings.ToLower(fmt.Sprint(s[0]))] = fmt.Sprint(s[1])
		en[strings.ToLower(fmt.Sprint(s[1]))] = fmt.Sprint(s[1])
	}
	conv := func(l []string) []string {
		out := make([]string, len(l))
		for i, n := range l {
			if e, ok := en[strings.ToLower(n)]; ok {
				out[i] = e
			} else {
				out[i] = n
			}
		}
		return out
	}
	tabella := func(m map[string]any) {
		for k, v := range m {
			switch x := v.(type) {
			case []string:
				m[k] = conv(x)
			case map[string][]string:
				for kk, vv := range x {
					x[kk] = conv(vv)
				}
			}
		}
	}
	for _, m := range d.SubSpells {
		tabella(m)
	}
	for _, m := range d.RaceSpells {
		tabella(m)
	}
	for _, m := range d.LandSpells {
		for k, v := range m {
			m[k] = conv(v)
		}
	}
}

// catalogoMostri e catalogoProfessioni: i file dell'app (vuoti) con le voci dei pacchetti attivi.
func catalogoMostri(base []byte, voci []map[string]any) []byte {
	var j map[string]any
	if json.Unmarshal(base, &j) != nil || j == nil {
		j = map[string]any{"versione": 1}
	}
	tutte, _ := j["voci"].([]any)
	for _, v := range voci {
		tutte = append(tutte, v)
	}
	if tutte == nil {
		tutte = []any{}
	}
	j["voci"] = tutte
	b, _ := json.Marshal(j)
	return b
}

func catalogoProfessioniUnito(base []byte, p *professioniPacchetto) []byte {
	var j map[string]json.RawMessage
	if json.Unmarshal(base, &j) != nil || j == nil {
		j = map[string]json.RawMessage{}
	}
	if p != nil {
		prof := map[string]json.RawMessage{}
		json.Unmarshal(j["professioni"], &prof)
		for k, v := range p.Professioni {
			prof[k] = v
		}
		j["professioni"], _ = json.Marshal(prof)
		for _, k := range []string{"artigiani", "disponibilita"} {
			var l []json.RawMessage
			json.Unmarshal(j[k], &l)
			if k == "artigiani" {
				l = append(l, p.Artigiani...)
			} else {
				l = append(l, p.Disponibilita...)
			}
			j[k], _ = json.Marshal(l)
		}
		chiavi := make([]string, 0, len(p.Regole))
		for k := range p.Regole {
			chiavi = append(chiavi, k)
		}
		sort.Strings(chiavi)
		for _, k := range chiavi {
			var l []string
			if json.Unmarshal(j[k], &l) == nil { // nel file dell'app le regole sono elenchi di testi
				j[k], _ = json.Marshal(append(l, p.Regole[k]))
			} else {
				j[k], _ = json.Marshal(p.Regole[k])
			}
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.Encode(j)
	return buf.Bytes()
}
