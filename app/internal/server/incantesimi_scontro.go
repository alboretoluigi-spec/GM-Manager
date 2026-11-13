package server

// Incantesimi nello scontro (Parte 2 del piano di combattimento, fasi F2–F3; decisioni dell'utente del 2026-11-11 in
// docs/PIANO_COMBATTIMENTO.md). Lo stesso motore degli attacchi (attacchi.go):
//   - attacco con incantesimo: «attacco.tira» con il campo «incantesimo» (bonus d'attacco con incantesimo, danni al livello
//     dello slot, raggi). Usa l'azione, l'azione bonus o la reazione del tempo di lancio una volta per lancio (i raggi dello
//     stesso lancio no); critico, difese e PF come per le armi.
//   - tiro salvezza («incantesimo.ts»): il server tira i TS di tutti i bersagli (con il loro bonus: PG dal riepilogo della
//     scheda, mostri dalla loro scheda; Resistenza alla magia = vantaggio) e i danni una volta sola per tutti. Il riepilogo va
//     SOLO al Master, che può cambiare ogni esito (per esempio con la Resistenza leggendaria) e poi preme «Applica»
//     («incantesimo.applica»): metà o niente se riesce, difese per tipo, condizioni a chi fallisce (tranne a chi ne è
//     immune), registro, esito a tutti.
//   - cure e PF temporanei («incantesimo.cura»): tirati una volta, applicati ai bersagli (mai oltre il massimo; i PF
//     temporanei non si sommano: resta il valore più alto).
//   - Concentrazione: l'incantatore riceve la condizione «Concentrato»; chi è concentrato e subisce danni riceve
//     nell'esito la CD del TS di Costituzione (attacchi.go, danneggia).
// I valori dell'incantesimo (bonus, CD, dadi al livello dello slot) li calcola la scheda (web/scheda/incantesimi_ui.js,
// effettoScontro) come gli attacchi delle armi; qui si ricontrollano.

import (
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"gasterbless/internal/cripto"
)

type incantesimoScontro struct {
	Nome       string            `json:"nome"`
	Livello    int               `json:"livello"`
	Slot       int               `json:"slot"`
	Tempo      string            `json:"tempo"` // azione, bonus, reazione, altro
	Conc       bool              `json:"conc"`
	Tipo       string            `json:"tipo"` // attacco, ts, cura, pftemp
	Bonus      int               `json:"bonus"`
	CD         int               `json:"cd"`
	Car        string            `json:"car"`      // FOR, DES, COS, INT, SAG, CAR
	SeRiesce   string            `json:"seRiesce"` // meta, nulla
	Danni      []componenteDanno `json:"danni"`
	Condizioni []string          `json:"condizioni"`
	Cura       *componenteDanno  `json:"cura"`
	Raggi      int               `json:"raggi"`
	Lancio     string            `json:"lancio"`
	Mischia    bool              `json:"mischia"` // attacco con incantesimo in mischia (altrimenti a distanza)
	Round      int               `json:"round"`   // durata in round (1 minuto = 10; 0 = istantaneo): effetti in corso
	Ripeti     bool              `json:"ripeti"`  // il bersaglio ripete il TS alla fine di ogni suo turno
}

var caratteristicheTS = map[string]string{"FOR": "Forza", "DES": "Destrezza", "COS": "Costituzione", "INT": "Intelligenza", "SAG": "Saggezza", "CAR": "Carisma"}

func validaIncantesimoScontro(x *incantesimoScontro) (incantesimoScontro, error) {
	if x == nil {
		return incantesimoScontro{}, errDatiNonValidi
	}
	i := *x
	i.Nome = limita(i.Nome, 60)
	if i.Nome == "" || i.Livello < 0 || i.Livello > 9 || i.Slot < 0 || i.Slot > 9 {
		return i, errDatiNonValidi
	}
	switch i.Tempo {
	case "azione", "bonus", "reazione", "altro":
	default:
		i.Tempo = "azione"
	}
	if p := validaProfili([]profiloAttacco{{Nome: i.Nome, Danni: i.Danni}}); len(p) == 1 {
		i.Danni = p[0].Danni
	} else {
		i.Danni = nil
	}
	switch i.Tipo {
	case "attacco":
		if i.Bonus < -5 || i.Bonus > 20 {
			return i, errDatiNonValidi
		}
	case "ts":
		if i.CD < 5 || i.CD > 30 || caratteristicheTS[i.Car] == "" {
			return i, errDatiNonValidi
		}
		if i.SeRiesce != "meta" {
			i.SeRiesce = "nulla"
		}
	case "cura", "pftemp":
		if i.Cura == nil {
			return i, errDatiNonValidi
		}
		p := validaProfili([]profiloAttacco{{Nome: i.Nome, Danni: []componenteDanno{{Dadi: i.Cura.Dadi, Mod: i.Cura.Mod}}}})
		if len(p) != 1 || len(p[0].Danni) != 1 || i.Cura.Mod < -5 || i.Cura.Mod > 30 {
			return i, errDatiNonValidi
		}
		c := p[0].Danni[0]
		c.Tipo = ""
		i.Cura = &c
	default:
		return i, errDatiNonValidi
	}
	cond := []string{}
	for _, n := range i.Condizioni {
		if condizioniAmmesse[n] && n != "Concentrato" && !slices.Contains(cond, n) && len(cond) < 6 {
			cond = append(cond, n)
		}
	}
	i.Condizioni = cond
	if i.Raggi < 1 {
		i.Raggi = 1
	}
	if i.Round < 0 || i.Round > 14400 {
		i.Round = 0
	}
	if i.Raggi > 10 || i.Lancio != "" && !reAzione.MatchString(i.Lancio) {
		return i, errDatiNonValidi
	}
	return i, nil
}

// lancioInCorso: incantesimo d'attacco con più raggi (Raggio rovente, Deflagrazione occulta…).
type lancioInCorso struct {
	incantatore  string
	raggi, usati int
	quando       time.Time
}

// usaEconomia controlla e segna l'azione, l'azione bonus o la reazione del tempo di lancio. Al Master restituisce il testo
// di una richiesta di conferma quando è già usata; al giocatore un errore.
func (s *Server) usaEconomia(a *vistaCombattente, tempo, turnoDi string, master, forza bool) (string, error) {
	if tempo == "altro" {
		return "", nil
	}
	if !master && turnoDi != a.ID && tempo != "reazione" {
		return "", errore(http.StatusConflict, "Non è il turno di "+a.Nome+".")
	}
	eco := economiaTurno{}
	if a.Economia != nil {
		eco = *a.Economia
	}
	nome := map[string]string{"azione": "l'azione", "bonus": "l'azione bonus", "reazione": "la reazione"}[tempo]
	usata := tempo == "azione" && eco.A || tempo == "bonus" && eco.B || tempo == "reazione" && eco.R
	if usata && !(master && forza) {
		if master {
			return a.Nome + " ha già usato " + nome + " in questo turno. Lanciare lo stesso?", nil
		}
		return "", errore(http.StatusConflict, "Hai già usato "+nome+" in questo turno.")
	}
	switch tempo {
	case "azione":
		eco.A = true
	case "bonus":
		eco.B = true
	case "reazione":
		eco.R = true
	}
	b, _ := json.Marshal(eco)
	s.DB.Exec(`UPDATE combattenti SET economia = ? WHERE id = ?`, string(b), a.ID)
	a.Economia = &eco
	return "", nil
}

// aggiungiCondizioni aggiunge condizioni a un combattente (PG: nello stato in gioco, come dal tracker).
func (s *Server) aggiungiCondizioni(u *Utente, c *vistaCombattente, nuove []string) []string {
	l := append([]string{}, c.Condizioni...)
	aggiunte := []string{}
	for _, n := range nuove {
		if !slices.Contains(l, n) {
			l = append(l, n)
			aggiunte = append(aggiunte, n)
		}
	}
	if len(aggiunte) == 0 {
		return nil
	}
	l, err := condizioniValide(l)
	if err != nil {
		return nil
	}
	if c.Tipo == "mostro" {
		b, _ := json.Marshal(l)
		s.DB.Exec(`UPDATE combattenti SET condizioni = ? WHERE id = ?`, string(b), c.ID)
	} else if s.modificaPG(u, c, "pg.condizioni", datiAzione{Condizioni: l}) != nil {
		return nil
	} else {
		s.avvisaStatoInGioco(c.Personaggio, c.Proprietario)
	}
	c.Condizioni = l
	return aggiunte
}

// lancioConRaggi: economia e concentrazione al primo raggio di un incantesimo d'attacco; i raggi successivi dello stesso
// lancio (stesso id «lancio») non usano di nuovo l'azione. Chi chiama tiene attacchiInCorso.serie.
func (s *Server) lancioConRaggi(u *Utente, comb string, a *vistaCombattente, inc incantesimoScontro, turnoDi string, master, forza, usaSlot bool) (int, any, error) {
	m := attacchiInCorso
	if l := m.lanci[inc.Lancio]; inc.Lancio != "" && l != nil && l.incantatore == a.ID {
		if l.usati >= l.raggi {
			return 0, nil, errore(http.StatusConflict, "Hai già tirato tutti i raggi di questo incantesimo.")
		}
		l.usati++
		return l.raggi - l.usati, nil, nil
	}
	if usaSlot {
		if err := s.controllaSlot(a, inc); err != nil {
			return 0, nil, err
		}
	}
	if t, err := s.usaEconomia(a, inc.Tempo, turnoDi, master, forza); err != nil || t != "" {
		if err != nil {
			return 0, nil, err
		}
		return 0, map[string]any{"serveConferma": t}, nil
	}
	if usaSlot {
		s.usaSlot(u, a, inc)
	}
	if inc.Conc {
		s.finisceConcentrazione(u, comb, a.ID, a.Nome+" si concentra su "+inc.Nome)
		s.aggiungiCondizioni(u, a, []string{"Concentrato"})
	}
	if inc.Lancio != "" {
		m.lanci[inc.Lancio] = &lancioInCorso{incantatore: a.ID, raggi: inc.Raggi, usati: 1, quando: time.Now()}
	}
	return inc.Raggi - 1, nil, nil
}

// ===== tiri salvezza =====

type rigaTS struct {
	Bersaglio   string   `json:"bersaglio"`
	Nome        string   `json:"nome"`
	Tipo        string   `json:"tipo"`
	D20         []int    `json:"d20"`
	Bonus       int      `json:"bonus"`
	Totale      int      `json:"totale"`
	Riuscito    bool     `json:"riuscito"`
	ResMagia    bool     `json:"resMagia,omitempty"`
	Leggendaria int      `json:"leggendaria,omitempty"` // Resistenza leggendaria (usi al giorno, dalla scheda del mostro)
	Immune      []string `json:"immuneA,omitempty"`     // condizioni dell'incantesimo a cui è immune
	Nota        string   `json:"nota,omitempty"`        // fallimento automatico, vantaggio o svantaggio e perché
	// dopo «Applica»
	Danni     []parteDanno `json:"danni,omitempty"`
	TotDanni  int          `json:"totaleDanni"`
	PFPrima   *int64       `json:"pfPrima,omitempty"`
	PFDopo    *int64       `json:"pfDopo,omitempty"`
	Sconfitto bool         `json:"sconfitto,omitempty"`
	Cond      []string     `json:"condizioni,omitempty"`
	Conc      int          `json:"concentrazione,omitempty"`
	Cura      int          `json:"cura,omitempty"`
	nascosto  bool
	prop      string // proprietario del PG (esito anonimo di un incantatore nascosto)
}

type esitoIncantesimo struct {
	ID          string       `json:"id"` // per scartare i doppioni (canale della sessione e messaggio a chi ha lanciato)
	Sessione    string       `json:"sessione"`
	Azione      string       `json:"azione,omitempty"`
	Autore      string       `json:"autore"`            // chi ha lanciato (utente): a lui l'esito arriva con i dettagli
	Gettone     string       `json:"gettone,omitempty"` // TS in attesa della conferma del Master
	Incantatore string       `json:"incantatore"`
	NomeInc     string       `json:"nomeIncantatore"`
	Incantesimo string       `json:"incantesimo"`
	Tipo        string       `json:"tipo"` // ts, cura, pftemp
	CD          int          `json:"cd,omitempty"`
	Car         string       `json:"car,omitempty"`
	SeRiesce    string       `json:"seRiesce,omitempty"`
	Tiro        []parteDanno `json:"tiro,omitempty"`       // danni (o cura) tirati una volta per tutti
	Condizioni  []string     `json:"condizioni,omitempty"` // dell'incantesimo, a chi fallisce il TS
	Righe       []rigaTS     `json:"righe"`
	Applicato   bool         `json:"applicato"`
	Anonimo     bool         `json:"anonimo,omitempty"` // incantatore nascosto: senza il suo nome e l'incantesimo
	Colore      string       `json:"colore,omitempty"`  // dadi 3D visti da tutti: set e classe di chi lancia (dadi_aspetto.go)
	Classe      string       `json:"classe,omitempty"`
	incNascosto bool         // l'incantatore è un mostro nascosto: solo il Master (e, senza nome, i PG colpiti)
}

type tsInAttesa struct {
	comb, sessione, autore string
	inc                    incantesimoScontro
	esito                  esitoIncantesimo
	scade                  time.Time
}

// bonusTS: bonus al tiro salvezza di un combattente. PG: il valore calcolato dalla scheda (competenza, Aura di protezione,
// oggetti magici; riepilogo «tsTot»), altrimenti modificatore dal punteggio + competenza; vantaggio contro gli incantesimi
// sulle caratteristiche di «resMagia» (yuan-ti, gnomo, abiuratore), immunità alle condizioni e vantaggi contro le
// condizioni della scheda (difese del 2026-11-12). Mostro: dalla sua scheda (profilo).
func (s *Server) bonusTS(c *vistaCombattente, car string) (bonus int, resMagia bool, legg int, immCond, vantCond []string) {
	if c.Tipo == "mostro" {
		var p profiloMostro
		var raw string
		s.DB.QueryRow(`SELECT profilo FROM combattenti WHERE id = ?`, c.ID).Scan(&raw)
		json.Unmarshal([]byte(raw), &p)
		return p.TS[car], p.ResMagia, p.Leggendaria, p.ImmCond, nil
	}
	var r riepilogo
	var raw string
	s.DB.QueryRow(`SELECT COALESCE(v.riepilogo,'{}') FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		WHERE p.id = ?`, c.Personaggio).Scan(&raw)
	json.Unmarshal([]byte(raw), &r)
	if v, ok := r.TSTot[car]; ok {
		return v, slices.Contains(r.ResMagia, car), 0, r.ImmCond, r.VantTS
	}
	if v, ok := r.Car[car]; ok {
		bonus = int(math.Floor(float64(v-10) / 2))
	}
	if slices.Contains(r.TS, car) {
		bonus += 2 + (max(1, r.Livello)-1)/4
	}
	return bonus, slices.Contains(r.ResMagia, car), 0, r.ImmCond, r.VantTS
}

// tiraTS: tiro salvezza di un combattente contro una CD, con le regole delle condizioni (paralizzato, stordito, privo di
// sensi, pietrificato falliscono da soli i TS su Forza e Destrezza; indebolimento 3+ svantaggio, trattenuto svantaggio su
// Destrezza) e la Resistenza alla magia (vantaggio). Restituisce anche le immunità alle condizioni.
// cond: condizioni che l'incantesimo o l'effetto mette a chi fallisce (vantaggio di chi ce l'ha contro quelle condizioni).
func (s *Server) tiraTS(b *vistaCombattente, car string, cd int, cond []string) (rigaTS, []string) {
	bonus, resMagia, legg, immCond, vantCond := s.bonusTS(b, car)
	if b.mod != nil { // effetti in corso (effetti_valori.go): bonus fisso, immunità alle condizioni
		bonus += b.mod.bonusTS
		immCond = append(slices.Clone(immCond), b.mod.immCond...)
	}
	r := rigaTS{Bersaglio: b.ID, Nome: b.Nome, Tipo: b.Tipo, Bonus: bonus, ResMagia: resMagia, Leggendaria: legg, nascosto: b.Nascosto, prop: propPG(b)}
	if c := nonPuoAgire(b); c != "" && c != "incapacitato" && (car == "FOR" || car == "DES") {
		r.Nota = "fallisce da solo: è " + c
		return r, immCond
	}
	adv, dis := resMagia, indebolimentoDi(b) >= 3 || car == "DES" && slices.Contains(b.Condizioni, "Trattenuto")
	contro := ""
	for _, c := range cond {
		if slices.Contains(vantCond, c) {
			adv, contro = true, strings.ToLower(c)
			break
		}
	}
	if b.mod != nil && b.mod.vantTS[car] != "" && contro == "" {
		adv, contro = true, "per "+b.mod.vantTS[car]
	} else if b.mod != nil && b.mod.vantTS[car] != "" {
		adv = true
	}
	t1 := dado(20)
	r.D20 = []int{t1}
	nat := t1
	if adv != dis {
		t2 := dado(20)
		r.D20 = append(r.D20, t2)
		if adv {
			nat = max(t1, t2)
		} else {
			nat = min(t1, t2)
			r.Nota = "svantaggio (indebolimento o trattenuto)"
		}
		if adv && contro != "" {
			if strings.HasPrefix(contro, "per ") {
				r.Nota = "vantaggio " + contro
			} else {
				r.Nota = "vantaggio contro " + contro
			}
		}
	} else if adv && dis {
		r.Nota = "vantaggio e svantaggio si annullano"
	}
	r.Totale = nat + bonus
	if b.mod != nil && len(b.mod.dadiTS) > 0 { // Benedizione, Anatema…
		x, testi := tiraDadiEffetto(b.mod.dadiTS)
		r.Totale += x
		if r.Nota != "" {
			r.Nota += "; "
		}
		r.Nota += strings.Join(testi, "; ")
	}
	r.Riuscito = r.Totale >= cd
	return r, immCond
}

// incantesimoScontro esegue «incantesimo.ts», «incantesimo.applica» e «incantesimo.cura» (da azioneIniziativa).
func (s *Server) incantesimoScontro(u *Utente, tipo, sessione, comb, ruolo string, d datiAzione, registro func(string)) (any, error) {
	master := ruolo == "master"
	if d.Azione != "" && !reAzione.MatchString(d.Azione) {
		return nil, errDatiNonValidi
	}
	m := attacchiInCorso
	m.serie.Lock()
	defer m.serie.Unlock()
	m.pulisci()
	if r, ok := m.risolti[d.Azione]; ok && d.Azione != "" {
		return r, nil
	}
	_, round, turni, turnoDi, err := s.combattimentoAttivo(s.DB, sessione)
	if err != nil {
		return nil, errNessunCombattimento
	}
	tutti, err := s.caricaCombattenti(comb)
	if err != nil {
		return nil, err
	}
	trova := func(id string) *vistaCombattente {
		for _, c := range tutti {
			if c.ID == id {
				return c
			}
		}
		return nil
	}
	ricorda := func(r any) (any, error) {
		if d.Azione != "" {
			m.risolti[d.Azione], m.quando[d.Azione] = r, time.Now()
		}
		return r, nil
	}

	if tipo == "incantesimo.applica" { // solo il Master: applica i TS confermati
		if !master {
			return nil, errVietato
		}
		m.tsMu.Lock()
		manca := m.ts[d.Gettone] == nil
		m.tsMu.Unlock()
		if manca { // dopo un riavvio di Gaster Bless: dal database (attese_scontro.go)
			s.ripristinaAttese(comb, true)
		}
		m.tsMu.Lock()
		p := m.ts[d.Gettone]
		if p != nil && p.comb == comb {
			delete(m.ts, d.Gettone)
			s.togliAttesa(d.Gettone)
		}
		m.tsMu.Unlock()
		if p == nil || p.comb != comb {
			return nil, errore(http.StatusConflict, "Questo incantesimo è già stato applicato o è scaduto.")
		}
		e := p.esito
		e.ID, e.Gettone, e.Applicato, e.Azione = cripto.ID(), "", true, d.Azione
		var conEffetto []bersaglioEffetto // bersagli con le condizioni messe: diventano un effetto in corso
		for i := range e.Righe {
			x := &e.Righe[i]
			if v, ok := d.Riusciti[x.Bersaglio]; ok {
				x.Riuscito = v
			}
			b := trova(x.Bersaglio)
			if b == nil {
				continue
			}
			esitoTS := "fallisce"
			if x.Riuscito {
				esitoTS = "riesce"
			}
			t := b.Nome + " " + esitoTS + " il TS su " + caratteristicheTS[p.inc.Car] + " (" + itoa(x.Totale) + " contro CD " + itoa(p.inc.CD) + ")"
			if len(e.Tiro) > 0 && !(x.Riuscito && p.inc.SeRiesce != "meta") {
				parti := make([]parteDanno, len(e.Tiro))
				copy(parti, e.Tiro)
				for k := range parti {
					parti[k].Dadi = nil
					if x.Riuscito {
						parti[k].Tirato /= 2
					}
				}
				r, err := s.danneggia(u, comb, b, parti, colpoArma{magica: true, incantesimo: true})
				if err != nil {
					return nil, err
				}
				x.Danni, x.TotDanni, x.PFPrima, x.Sconfitto, x.Conc = r.parti, r.tot, r.prima, r.sconfitto, r.cdConc
				dopo := r.dopo
				x.PFDopo = &dopo
				t += ": " + r.testo() + r.coda(b)
				if r.sconfitto && b.Tipo == "mostro" {
					if z := s.ricompensaMostro(comb, b.ID, u); z != "" {
						defer registro(z)
					}
				}
			}
			if !x.Riuscito && len(p.inc.Condizioni) > 0 && d.ConCondizioni {
				da := []string{}
				for _, c := range p.inc.Condizioni {
					if !slices.Contains(x.Immune, c) {
						da = append(da, c)
					}
				}
				if agg := s.aggiungiCondizioni(u, b, da); len(agg) > 0 {
					x.Cond = agg
					t += " · " + strings.Join(agg, ", ")
					conEffetto = append(conEffetto, bersaglioEffetto{ID: b.ID, Nome: b.Nome, Condizioni: agg})
				}
			}
			registro(t)
		}
		if a := trova(e.Incantatore); a != nil {
			s.nuovoEffetto(comb, a, p.inc, conEffetto, round)
		}
		s.pubblicaEsitoIncantesimo(sessione, e, p.autore)
		s.pubblicaCombattimento(sessione)
		return ricorda(e)
	}

	// incantesimo.ts e incantesimo.cura: incantatore, bersagli, economia
	inc, err := validaIncantesimoScontro(d.Incantesimo)
	if err != nil || tipo == "incantesimo.ts" && inc.Tipo != "ts" || tipo == "incantesimo.cura" && inc.Tipo != "cura" && inc.Tipo != "pftemp" {
		return nil, errDatiNonValidi
	}
	a := trova(d.Combattente)
	if a == nil {
		return nil, errNonTrovato
	}
	if a.Tipo == "pg" && a.Proprietario != u.ID && !master || a.Tipo == "mostro" && !master {
		return nil, errVietato
	}
	if turni == 0 || turnoDi == "" {
		return nil, errore(http.StatusConflict, "Il combattimento non è ancora iniziato.")
	}
	if c := nonPuoAgire(a); c != "" && !(master && d.Forza) {
		if master {
			return map[string]any{"serveConferma": a.Nome + " è " + c + ": non può lanciare incantesimi. Lanciare lo stesso?"}, nil
		}
		return nil, errore(http.StatusConflict, a.Nome+" è "+c+": non può lanciare incantesimi.")
	}
	if len(d.Bersagli) == 0 || len(d.Bersagli) > 20 {
		return nil, errore(http.StatusBadRequest, "Scegli da 1 a 20 bersagli.")
	}
	var bers []*vistaCombattente
	for _, id := range d.Bersagli {
		b := trova(id)
		if b == nil || b.Nascosto && !master {
			return nil, errNonTrovato
		}
		if !slices.Contains(bers, b) {
			bers = append(bers, b)
		}
	}
	if d.UsaSlot {
		if err := s.controllaSlot(a, inc); err != nil {
			return nil, err
		}
	}
	if t, err := s.usaEconomia(a, inc.Tempo, turnoDi, master, d.Forza); err != nil || t != "" {
		if err != nil {
			return nil, err
		}
		return map[string]any{"serveConferma": t}, nil
	}
	if d.UsaSlot {
		s.usaSlot(u, a, inc)
	}
	if inc.Conc { // una sola concentrazione alla volta: finisce quella di prima
		s.finisceConcentrazione(u, comb, a.ID, a.Nome+" si concentra su "+inc.Nome)
		s.aggiungiCondizioni(u, a, []string{"Concentrato"})
	}
	e := esitoIncantesimo{ID: cripto.ID(), Sessione: sessione, Azione: d.Azione, Autore: u.ID, Incantatore: a.ID, NomeInc: a.Nome, Incantesimo: inc.Nome, Tipo: inc.Tipo,
		Righe: []rigaTS{}, incNascosto: a.Nascosto}
	e.Colore, e.Classe = s.aspettoTiratore(a, u)
	livello := ""
	if inc.Slot > inc.Livello {
		livello = " (slot di " + itoa(inc.Slot) + "° livello)"
	}

	if inc.Tipo == "cura" || inc.Tipo == "pftemp" { // tirata una volta, applicata a tutti
		x, dadi := tiraDadi(inc.Cura.Dadi, 1)
		v := max(0, x+inc.Cura.Mod)
		espr := inc.Cura.Dadi
		if inc.Cura.Mod != 0 || espr == "" {
			if espr == "" {
				espr = itoa(inc.Cura.Mod)
			} else {
				espr += conSegno(inc.Cura.Mod)
			}
		}
		e.Tiro = []parteDanno{{Espr: espr, Dadi: dadi, Tirato: v, Finale: v}}
		e.Applicato = true
		nomi := []string{}
		for _, b := range bers {
			r := rigaTS{Bersaglio: b.ID, Nome: b.Nome, Tipo: b.Tipo, Cura: v, nascosto: b.Nascosto, prop: propPG(b)}
			if b.PF != nil {
				p := *b.PF
				r.PFPrima = &p
			}
			var dopo int64
			if inc.Tipo == "cura" {
				if b.Tipo == "mostro" {
					s.DB.Exec(`UPDATE combattenti SET pf = MIN(pf_max, pf + ?) WHERE id = ?`, v, b.ID)
					s.DB.QueryRow(`SELECT pf FROM combattenti WHERE id = ?`, b.ID).Scan(&dopo)
				} else {
					if err := s.modificaPG(u, b, "pg.cura", datiAzione{Valore: &v}); err != nil {
						return nil, err
					}
					s.DB.QueryRow(`SELECT COALESCE(pf, 0) FROM stato_in_gioco WHERE personaggio_id = ?`, b.Personaggio).Scan(&dopo)
					s.avvisaStatoInGioco(b.Personaggio, b.Proprietario)
				}
				r.PFDopo = &dopo
			} else if b.Tipo == "pg" && v > b.PFTemp { // i PF temporanei non si sommano: resta il valore più alto
				if err := s.modificaPG(u, b, "pg.temp", datiAzione{Valore: &v}); err != nil {
					return nil, err
				}
				s.avvisaStatoInGioco(b.Personaggio, b.Proprietario)
			}
			e.Righe = append(e.Righe, r)
			nomi = append(nomi, b.Nome)
		}
		cosa := " PF recuperati"
		if inc.Tipo == "pftemp" {
			cosa = " PF temporanei"
		}
		registro("effetto di " + inc.Nome + livello + " lanciato da " + a.Nome + ": " + itoa(v) + cosa + " (" + espr + ") a " + strings.Join(nomi, ", "))
		s.pubblicaEsitoIncantesimo(sessione, e, u.ID)
		s.pubblicaCombattimento(sessione)
		return ricorda(e)
	}

	// tiro salvezza: TS di ogni bersaglio e danni una volta per tutti; riepilogo riservato al Master
	e.CD, e.Car, e.SeRiesce, e.Condizioni = inc.CD, inc.Car, inc.SeRiesce, inc.Condizioni
	for _, c := range inc.Danni {
		x, dadi := tiraDadi(c.Dadi, 1)
		espr := c.Dadi
		if c.Mod != 0 || espr == "" {
			if espr == "" {
				espr = itoa(c.Mod)
			} else {
				espr += conSegno(c.Mod)
			}
		}
		e.Tiro = append(e.Tiro, parteDanno{Tipo: c.Tipo, Espr: espr, Dadi: dadi, Tirato: max(0, x+c.Mod)})
	}
	for _, b := range bers {
		r, immCond := s.tiraTS(b, inc.Car, inc.CD, inc.Condizioni)
		for _, c := range inc.Condizioni {
			if slices.Contains(immCond, c) {
				r.Immune = append(r.Immune, c)
			}
		}
		e.Righe = append(e.Righe, r)
	}
	g := cripto.ID()
	e.Gettone = g
	m.tsMu.Lock()
	m.ts[g] = &tsInAttesa{comb: comb, sessione: sessione, autore: u.ID, inc: inc, esito: e, scade: time.Now().Add(30 * time.Minute)}
	s.salvaTS(g, m.ts[g])
	m.tsMu.Unlock()
	nomi := []string{}
	for _, r := range e.Righe {
		nomi = append(nomi, r.Nome)
	}
	insolito := ""
	if a.Tipo == "pg" {
		insolito = valoriInsoliti(0, inc.CD, nil)
	}
	registro("effetto di " + inc.Nome + livello + " lanciato da " + a.Nome + " (TS su " + caratteristicheTS[inc.Car] + " CD " + itoa(inc.CD) + ") su " + strings.Join(nomi, ", ") + ": in attesa della conferma del Master" + insolito)
	// il riepilogo con i tiri va solo al Master (anche se ha lanciato un giocatore)
	for _, id := range []string{s.masterID()} {
		s.Hub.AUtente(id, Msg{Tipo: "incantesimo.conferma", Dati: e})
	}
	s.pubblicaCombattimento(sessione)
	if master {
		return ricorda(e)
	}
	return ricorda(map[string]any{"inAttesa": true, "incantesimo": inc.Nome, "messaggio": "Tiri salvezza fatti: il Master li controlla e applica l'effetto."})
}

// incantesimiInAttesa: i TS di questo scontro che aspettano la conferma del Master (vista del Master, iniziativa.go).
func (s *Server) incantesimiInAttesa(comb string) []esitoIncantesimo {
	s.ripristinaAttese(comb, false) // dopo un riavvio di Gaster Bless (attese_scontro.go)
	m := attacchiInCorso
	m.tsMu.Lock()
	defer m.tsMu.Unlock()
	var l []esitoIncantesimo
	for _, p := range m.ts {
		if p.comb == comb && time.Now().Before(p.scade) {
			l = append(l, p.esito)
		}
	}
	slices.SortFunc(l, func(a, b esitoIncantesimo) int { return strings.Compare(a.Gettone, b.Gettone) })
	return l
}

// pubblicaEsitoIncantesimo: l'esito applicato a tutti i presenti (ai giocatori senza PF esatti dei mostri e senza i mostri
// nascosti) e a chi ha lanciato, anche se non ha il tracker aperto.
func (s *Server) pubblicaEsitoIncantesimo(sessione string, e esitoIncantesimo, autore string) {
	if e.ID == "" {
		e.ID = cripto.ID()
	}
	perRuolo := func(ruolo, uid string) *esitoIncantesimo {
		if ruolo == "" {
			return nil
		}
		if e.incNascosto && ruolo != "master" {
			// decisione dell'utente del 2026-11-12: il giocatore riceve solo la riga del suo PG, senza chi e che cosa
			x := e
			x.Gettone, x.Autore, x.Incantatore, x.NomeInc, x.Incantesimo, x.CD, x.Anonimo = "", "", "", "Qualcosa", "un incantesimo", 0, true
			x.Tiro = nil // dei danni restano solo il tipo e il valore
			for _, t := range e.Tiro {
				x.Tiro = append(x.Tiro, parteDanno{Tipo: t.Tipo, Tirato: t.Tirato, Finale: t.Finale})
			}
			x.Righe = []rigaTS{}
			for _, r := range e.Righe {
				if r.prop != "" && r.prop == uid {
					r.D20, r.Bonus, r.Totale, r.Leggendaria = nil, 0, 0, 0
					var d []parteDanno
					for _, x := range r.Danni {
						d = append(d, parteDanno{Tipo: x.Tipo, Tirato: x.Tirato, Finale: x.Finale, Nota: x.Nota})
					}
					r.Danni = d
					x.Righe = append(x.Righe, r)
				}
			}
			if len(x.Righe) == 0 {
				return nil
			}
			return &x
		}
		x := e
		x.Gettone = ""
		x.Righe = []rigaTS{}
		for _, r := range e.Righe {
			if r.nascosto && ruolo != "master" {
				continue
			}
			if ruolo == "giocatore" && r.Tipo == "mostro" {
				r.PFPrima, r.PFDopo, r.D20, r.Bonus, r.Totale = nil, nil, nil, 0, 0
			}
			x.Righe = append(x.Righe, r)
		}
		if len(x.Righe) == 0 && ruolo != "master" {
			return nil
		}
		return &x
	}
	s.Hub.PubblicaPer("sessione:"+sessione, func(c *Cliente) *Msg {
		if c.U == nil {
			return nil
		}
		ruolo, _ := s.ruoloIn(c.U, sessione)
		x := perRuolo(ruolo, c.U.ID)
		if x == nil {
			return nil
		}
		return &Msg{Tipo: "incantesimo.esito", Dati: x}
	})
	// i giocatori del gioco da remoto (ponte_mqtt.go)
	s.remoto.evento(sessione, "incantesimo.esito", func(u *Utente) any {
		ruolo, _ := s.ruoloIn(u, sessione)
		if x := perRuolo(ruolo, u.ID); x != nil {
			return *x
		}
		return nil
	})
	// chi ha lanciato dalla scheda (senza il tracker aperto) lo vede lo stesso; la pagina scarta i doppioni per «azione»
	if u := s.utentePerID(autore); u != nil {
		ruolo, _ := s.ruoloIn(u, sessione)
		if x := perRuolo(ruolo, u.ID); x != nil {
			s.Hub.AUtente(autore, Msg{Tipo: "incantesimo.esito", Dati: x})
		}
	}
}
