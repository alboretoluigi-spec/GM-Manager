package server

// Effetti che cambiano i valori (Parte 2 del piano di combattimento, F2; 2026-11-12; migrazione 029).
// Un effetto in corso (tabella effetti_scontro) può dare ai suoi bersagli delle modifiche:
//   - CA: bonus (Scudo della fede +2), base + Destrezza se è più alta (Armatura magica 13), minimo (Pelle coriacea 16);
//   - resistenze e immunità ai danni (Pelle di pietra, Protezione dall'energia, ira), immunità alle condizioni (Eroismo);
//   - TS: vantaggio su una caratteristica (Velocità: Destrezza; ira: Forza), bonus fisso (Legame protettivo), dadi in più o
//     in meno (Benedizione +1d4);
//   - tiri per colpire: dadi in più (Benedizione); chi attacca il bersaglio ha svantaggio (Sfocatura);
//   - danni in mischia in più (ira).
// Le modifiche si applicano quando si caricano i combattenti (caricaCombattenti → applicaEffettiValori): nel database
// restano i valori originali (CA della scheda o del mostro), che il tracker mostra accanto a quelli modificati. Gli effetti
// con lo stesso nome non si sommano sullo stesso bersaglio (vale il primo). Durata, concentrazione e fine: come gli altri
// effetti (effetti_scontro.go).

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

type modificheEffetto struct {
	CABonus      int      `json:"caBonus,omitempty"`      // -5..+10
	CABase       int      `json:"caBase,omitempty"`       // 10..20: CA base + modificatore di Destrezza, se è più alta
	CAMin        int      `json:"caMin,omitempty"`        // 10..25: la CA non scende sotto
	Res          []string `json:"res,omitempty"`          // tipi di danno, anche «contundente (non magici)»
	Imm          []string `json:"imm,omitempty"`          // tipi di danno
	ImmCond      []string `json:"immCond,omitempty"`      // condizioni
	VantTS       []string `json:"vantTS,omitempty"`       // caratteristiche con vantaggio ai TS
	BonusTS      int      `json:"bonusTS,omitempty"`      // -5..+5
	DadoTS       string   `json:"dadoTS,omitempty"`       // «1d4», «-1d4»
	DadoAtt      string   `json:"dadoAtt,omitempty"`      // «1d4», «-1d4»
	SvantContro  bool     `json:"svantContro,omitempty"`  // chi attacca il bersaglio ha svantaggio
	DanniMischia int      `json:"danniMischia,omitempty"` // 0..10 ai danni degli attacchi in mischia (non incantesimi)
	// Fase 2 (2026-11-12): Eroismo, PF temporanei all'inizio di ogni turno del bersaglio (PG); Scudo di fuoco, danni a chi
	// colpisce il bersaglio con un attacco in mischia.
	PFTempTurno int         `json:"pfTempTurno,omitempty"` // 1..10
	Ritorsione  *ritorsione `json:"ritorsione,omitempty"`
}

type ritorsione struct {
	Dadi string `json:"dadi"` // «2d8»
	Tipo string `json:"tipo"` // tipo di danno
}

// effettoValori: i dati di «effetto.nuovo».
type effettoValori struct {
	Nome       string           `json:"nome"`
	Tempo      string           `json:"tempo"` // azione, bonus, reazione, altro
	Conc       bool             `json:"conc"`
	Round      int              `json:"round"` // 0: finché non si termina
	Mod        modificheEffetto `json:"mod"`
	Condizioni []string         `json:"condizioni"` // messe ai bersagli finché dura (Invisibilità superiore)
	// Anatema (2026-11-12): i bersagli fanno un tiro salvezza; chi riesce non subisce l'effetto.
	TSCar    string `json:"tsCar,omitempty"`
	TSCD     int    `json:"tsCd,omitempty"`
	fineConc bool   // l'effetto interrompe la concentrazione di chi lo usa (ira)
}

type dadoEffetto struct {
	segno int
	dadi  string
	nome  string
}

// modAttive: le modifiche degli effetti in corso su un combattente che non stanno già in CA e difese.
type modAttive struct {
	vantTS      map[string]string // caratteristica → effetto
	bonusTS     int
	dadiTS      []dadoEffetto
	dadiAtt     []dadoEffetto
	svantContro string // effetto
	mischia     int
	nomeMischia string
	immCond     []string
	pfTurno     int // Eroismo: PF temporanei a inizio turno
	nomePFTurno string
	ritorsioni  []ritorsioneAttiva // Scudo di fuoco
}

type ritorsioneAttiva struct {
	ritorsione
	nome string
}

var reDadoEffetto = regexp.MustCompile(`^(-?)([1-9]d(?:4|6|8|10|12))$`)

const maxRoundEffetto = 14400 // 24 ore

// voceDifesa: «fuoco» o «contundente (non magici)» → difesa con il tipo normalizzato.
func voceDifesa(t string) (difesa, bool) {
	d := leggiVoceDifesa(t)
	d.Tipo = tipoDanno(d.Tipo)
	return d, d.Tipo != ""
}

func condizioneDiEffetto(c string) bool {
	return condizioniAmmesse[c] && c != "Concentrato" && !strings.HasPrefix(c, "Indebolimento")
}

// validaModifiche tiene solo i valori ammessi.
func validaModifiche(m modificheEffetto) modificheEffetto {
	out := modificheEffetto{SvantContro: m.SvantContro}
	if m.CABonus >= -5 && m.CABonus <= 10 {
		out.CABonus = m.CABonus
	}
	if m.CABase >= 10 && m.CABase <= 20 {
		out.CABase = m.CABase
	}
	if m.CAMin >= 10 && m.CAMin <= 25 {
		out.CAMin = m.CAMin
	}
	difese := func(l []string) []string {
		var r []string
		for _, t := range l {
			if d, ok := voceDifesa(t); ok && len(r) < 13 {
				v := d.Tipo
				if d.NonMagici {
					v += " (non magici)"
				}
				if !slices.Contains(r, v) {
					r = append(r, v)
				}
			}
		}
		return r
	}
	out.Res, out.Imm = difese(m.Res), difese(m.Imm)
	for _, c := range m.ImmCond {
		if condizioneDiEffetto(c) && !slices.Contains(out.ImmCond, c) {
			out.ImmCond = append(out.ImmCond, c)
		}
	}
	for _, c := range m.VantTS {
		if caratteristicheTS[c] != "" && !slices.Contains(out.VantTS, c) {
			out.VantTS = append(out.VantTS, c)
		}
	}
	if m.BonusTS >= -5 && m.BonusTS <= 5 {
		out.BonusTS = m.BonusTS
	}
	if reDadoEffetto.MatchString(m.DadoTS) {
		out.DadoTS = m.DadoTS
	}
	if reDadoEffetto.MatchString(m.DadoAtt) {
		out.DadoAtt = m.DadoAtt
	}
	if m.DanniMischia >= 0 && m.DanniMischia <= 10 {
		out.DanniMischia = m.DanniMischia
	}
	if m.PFTempTurno >= 1 && m.PFTempTurno <= 10 {
		out.PFTempTurno = m.PFTempTurno
	}
	if m.Ritorsione != nil && reDadiRitorsione.MatchString(m.Ritorsione.Dadi) {
		if t := tipoDanno(m.Ritorsione.Tipo); t != "" {
			out.Ritorsione = &ritorsione{Dadi: m.Ritorsione.Dadi, Tipo: t}
		}
	}
	return out
}

var reDadiRitorsione = regexp.MustCompile(`^[1-4]d(?:4|6|8|10|12)$`)

func (m modificheEffetto) vuota() bool {
	return m.CABonus == 0 && m.CABase == 0 && m.CAMin == 0 && len(m.Res)+len(m.Imm)+len(m.ImmCond)+len(m.VantTS) == 0 &&
		m.BonusTS == 0 && m.DadoTS == "" && m.DadoAtt == "" && !m.SvantContro && m.DanniMischia == 0 && m.PFTempTurno == 0 && m.Ritorsione == nil
}

// testoModifiche: le modifiche in breve, per il registro dello scontro.
func testoModifiche(m modificheEffetto) string {
	var p []string
	if m.CABase > 0 {
		p = append(p, "CA "+itoa(m.CABase)+" + Destrezza")
	}
	if m.CABonus != 0 {
		p = append(p, "CA "+conSegno(m.CABonus))
	}
	if m.CAMin > 0 {
		p = append(p, "CA almeno "+itoa(m.CAMin))
	}
	if len(m.Res) > 0 {
		p = append(p, "resistenza: "+strings.Join(m.Res, ", "))
	}
	if len(m.Imm) > 0 {
		p = append(p, "immunità: "+strings.Join(m.Imm, ", "))
	}
	if len(m.ImmCond) > 0 {
		p = append(p, "immune a: "+strings.ToLower(strings.Join(m.ImmCond, ", ")))
	}
	for _, c := range m.VantTS {
		p = append(p, "vantaggio ai TS su "+caratteristicheTS[c])
	}
	if m.BonusTS != 0 {
		p = append(p, "TS "+conSegno(m.BonusTS))
	}
	if m.DadoTS != "" {
		p = append(p, segnoDado(m.DadoTS)+" ai TS")
	}
	if m.DadoAtt != "" {
		p = append(p, segnoDado(m.DadoAtt)+" ai tiri per colpire")
	}
	if m.SvantContro {
		p = append(p, "chi lo attacca ha svantaggio")
	}
	if m.DanniMischia > 0 {
		p = append(p, "+"+itoa(m.DanniMischia)+" ai danni in mischia")
	}
	if m.PFTempTurno > 0 {
		p = append(p, itoa(m.PFTempTurno)+" PF temporanei a inizio turno")
	}
	if m.Ritorsione != nil {
		p = append(p, m.Ritorsione.Dadi+" "+m.Ritorsione.Tipo+" a chi lo colpisce in mischia")
	}
	return strings.Join(p, ", ")
}

func segnoDado(d string) string {
	if strings.HasPrefix(d, "-") {
		return "−" + d[1:]
	}
	return "+" + d
}

func dadoDi(espr, nome string) *dadoEffetto {
	m := reDadoEffetto.FindStringSubmatch(espr)
	if m == nil {
		return nil
	}
	d := dadoEffetto{segno: 1, dadi: m[2], nome: nome}
	if m[1] == "-" {
		d.segno = -1
	}
	return &d
}

// tiraDadiEffetto tira i dadi in più (o in meno) degli effetti: totale e spiegazione («+1d4 Benedizione: 3»).
func tiraDadiEffetto(l []dadoEffetto) (int, []string) {
	tot, testi := 0, []string{}
	for _, d := range l {
		x, _ := tiraDadi(d.dadi, 1)
		tot += d.segno * x
		s := "+"
		if d.segno < 0 {
			s = "−"
		}
		testi = append(testi, s+d.dadi+" "+d.nome+": "+itoa(x))
	}
	return tot, testi
}

// unisciDifese aggiunge resistenze e immunità di un effetto: se una delle fonti vale contro tutti gli attacchi, il tipo
// vale contro tutti (non resta il limite «non magici»).
func unisciDifese(d difeseCombattente, res, imm []difesa) difeseCombattente {
	aggiungi := func(l, nuove []difesa) []difesa {
		out := slices.Clone(l)
		for _, n := range nuove {
			if i := slices.IndexFunc(out, func(x difesa) bool { return x.Tipo == n.Tipo }); i >= 0 {
				out[i].NonMagici = out[i].NonMagici && n.NonMagici
			} else {
				out = append(out, n)
			}
		}
		return out
	}
	d.Res, d.Imm = aggiungi(d.Res, res), aggiungi(d.Imm, imm)
	return d
}

// applicaEffettiValori somma ai combattenti le modifiche degli effetti in corso (da caricaCombattenti).
func (s *Server) applicaEffettiValori(comb string, out []*vistaCombattente) {
	rows, err := s.DB.Query(`SELECT nome, bersagli, modifiche FROM effetti_scontro WHERE combattimento_id = ? AND modifiche <> ''
		ORDER BY creato_il, id`, comb)
	if err != nil {
		return
	}
	type voce struct {
		nome string
		ids  []string
		m    modificheEffetto
	}
	var l []voce
	for rows.Next() {
		var nome, b, mod string
		if rows.Scan(&nome, &b, &mod) != nil {
			continue
		}
		var bers []bersaglioEffetto
		var m modificheEffetto
		if json.Unmarshal([]byte(b), &bers) != nil || json.Unmarshal([]byte(mod), &m) != nil {
			continue
		}
		v := voce{nome: nome, m: m}
		for _, x := range bers {
			v.ids = append(v.ids, x.ID)
		}
		l = append(l, v)
	}
	rows.Close()
	for _, c := range out { // effetti che durano da scontri precedenti o da fuori dello scontro (effetti_persistenti.go)
		if c.Tipo == "pg" && c.Personaggio != "" {
			for _, d := range s.modificheDurevoli(c.Personaggio) {
				l = append(l, voce{nome: d.nome, ids: []string{c.ID}, m: d.m})
			}
		}
	}
	if len(l) == 0 {
		return
	}
	for _, c := range out {
		bonus, base, minimo := 0, 0, 0
		var res, imm []difesa
		var nomiCA []string
		visti := map[string]bool{}
		for _, e := range l {
			if !slices.Contains(e.ids, c.ID) || visti[e.nome] {
				continue
			}
			visti[e.nome] = true
			m := e.m
			if m.CABonus != 0 || m.CABase > 0 || m.CAMin > 0 {
				nomiCA = append(nomiCA, e.nome)
			}
			bonus += m.CABonus
			base, minimo = max(base, m.CABase), max(minimo, m.CAMin)
			for _, t := range m.Res {
				if d, ok := voceDifesa(t); ok {
					res = append(res, d)
				}
			}
			for _, t := range m.Imm {
				if d, ok := voceDifesa(t); ok {
					imm = append(imm, d)
				}
			}
			if c.mod == nil {
				c.mod = &modAttive{vantTS: map[string]string{}}
			}
			a := c.mod
			for _, k := range m.VantTS {
				if a.vantTS[k] == "" {
					a.vantTS[k] = e.nome
				}
			}
			a.bonusTS += m.BonusTS
			if d := dadoDi(m.DadoTS, e.nome); d != nil {
				a.dadiTS = append(a.dadiTS, *d)
			}
			if d := dadoDi(m.DadoAtt, e.nome); d != nil {
				a.dadiAtt = append(a.dadiAtt, *d)
			}
			if m.SvantContro && a.svantContro == "" {
				a.svantContro = e.nome
			}
			if m.DanniMischia > a.mischia {
				a.mischia, a.nomeMischia = m.DanniMischia, e.nome
			}
			for _, x := range m.ImmCond {
				if !slices.Contains(a.immCond, x) {
					a.immCond = append(a.immCond, x)
				}
			}
			if m.PFTempTurno > a.pfTurno {
				a.pfTurno, a.nomePFTurno = m.PFTempTurno, e.nome
			}
			if m.Ritorsione != nil {
				a.ritorsioni = append(a.ritorsioni, ritorsioneAttiva{*m.Ritorsione, e.nome})
			}
		}
		if c.CA != nil && len(nomiCA) > 0 {
			orig := *c.CA
			ca := orig
			if base > 0 && c.desMod != nil {
				ca = max(ca, base+*c.desMod)
			}
			ca += bonus
			if minimo > 0 {
				ca = max(ca, minimo)
			}
			if ca != orig {
				c.CA, c.CAOrig, c.EffettiCA = &ca, &orig, nomiCA
			}
		}
		if len(res)+len(imm) > 0 {
			c.difese = unisciDifese(c.difese, res, imm)
		}
	}
}

// effettiPG: per la scheda completa, la CA cambiata dagli effetti in corso (con l'originale) e le modifiche in breve.
type effettiPG struct {
	CA     *int     `json:"ca,omitempty"`
	CAOrig *int     `json:"caOrig,omitempty"`
	Voci   []string `json:"voci,omitempty"` // «Scudo della fede: CA +2»
}

// effettiDelPG: gli effetti con modifiche sul PG: nello scontro attivo e quelli che durano (nil se non ne ha).
func (s *Server) effettiDelPG(id string) *effettiPG {
	durevoli := s.durevoliDi([]string{id})
	var comb, chi string
	if s.DB.QueryRow(`SELECT k.id, x.id FROM combattenti x JOIN combattimenti k ON k.id = x.combattimento_id
		WHERE x.personaggio_id = ? AND k.attivo = 1 ORDER BY k.creato_il DESC LIMIT 1`, id).Scan(&comb, &chi) != nil {
		return s.effettiDurevoliFuori(id, durevoli) // fuori dallo scontro
	}
	var n int
	if s.DB.QueryRow(`SELECT COUNT(*) FROM effetti_scontro WHERE combattimento_id = ? AND modifiche <> ''`, comb).Scan(&n) != nil || n+len(durevoli) == 0 {
		return nil
	}
	out := &effettiPG{}
	for _, d := range durevoli {
		if d.Mod != nil {
			out.Voci = append(out.Voci, d.Nome+": "+d.Testo)
		}
	}
	if tutti, err := s.caricaCombattenti(comb); err == nil {
		for _, c := range tutti {
			if c.ID == chi && c.CAOrig != nil {
				out.CA, out.CAOrig = c.CA, c.CAOrig
			}
		}
	}
	for _, e := range s.caricaEffetti(comb) {
		if e.Mod != nil && slices.ContainsFunc(e.Bersagli, func(b bersaglioEffetto) bool { return b.ID == chi }) {
			out.Voci = append(out.Voci, e.Nome+": "+testoModifiche(*e.Mod))
		}
	}
	if out.CA == nil && len(out.Voci) == 0 {
		return nil
	}
	return out
}

// nuovoEffettoValori («effetto.nuovo»): un effetto che cambia i valori dei bersagli. Incantatore: PG (proprietario,
// Segretario, Master) o mostro (Master): permessi controllati da chi chiama. Usa l'azione, l'azione bonus o la reazione del
// tempo indicato (a scontro iniziato); a concentrazione termina quella di prima; lo stesso effetto dello stesso incantatore
// sostituisce quello di prima.
func (s *Server) nuovoEffettoValori(u *Utente, sessione, comb string, round int, turnoDi string, master bool, a *vistaCombattente,
	tutti []*vistaCombattente, d datiAzione, registro func(string)) (any, error) {
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
	r, err := s.creaEffettoValori(u, sessione, comb, round, turnoDi, master, a, tutti, d, registro)
	if err == nil && d.Azione != "" && r != nil {
		if mm, ok := r.(map[string]any); ok && mm["fatto"] == true {
			m.risolti[d.Azione], m.quando[d.Azione] = r, time.Now()
		}
	}
	return r, err
}

// creaEffettoValori: il lavoro di nuovoEffettoValori (chi chiama tiene attacchiInCorso.serie).
func (s *Server) creaEffettoValori(u *Utente, sessione, comb string, round int, turnoDi string, master bool, a *vistaCombattente,
	tutti []*vistaCombattente, d datiAzione, registro func(string)) (any, error) {
	if d.Valori == nil {
		return nil, errDatiNonValidi
	}
	e := *d.Valori
	e.Nome = limita(e.Nome, 60)
	e.Mod = validaModifiche(e.Mod)
	cond := []string{}
	for _, c := range e.Condizioni {
		if condizioneDiEffetto(c) && !slices.Contains(cond, c) {
			cond = append(cond, c)
		}
	}
	if e.Nome == "" || e.Round < 0 || e.Round > maxRoundEffetto || e.Mod.vuota() && len(cond) == 0 {
		return nil, errDatiNonValidi
	}
	switch e.Tempo {
	case "azione", "bonus", "reazione", "altro":
	default:
		e.Tempo = "altro"
	}
	if len(d.Bersagli) == 0 || len(d.Bersagli) > 20 {
		return nil, errore(http.StatusBadRequest, "Scegli da 1 a 20 bersagli.")
	}
	var bers []*vistaCombattente
	for _, id := range d.Bersagli {
		i := slices.IndexFunc(tutti, func(c *vistaCombattente) bool { return c.ID == id })
		if i < 0 || tutti[i].Nascosto && !master {
			return nil, errNonTrovato
		}
		if !slices.Contains(bers, tutti[i]) {
			bers = append(bers, tutti[i])
		}
	}
	if c := nonPuoAgire(a); c != "" && e.Tempo != "altro" && !(master && d.Forza) {
		if master {
			return map[string]any{"serveConferma": a.Nome + " è " + c + ": non può agire. Applicare lo stesso?"}, nil
		}
		return nil, errore(http.StatusConflict, a.Nome+" è "+c+": non può agire.")
	}
	if round > 0 { // prima dell'inizio dello scontro non c'è un turno da consumare
		if t, err := s.usaEconomia(a, e.Tempo, turnoDi, master, d.Forza); err != nil || t != "" {
			if err != nil {
				return nil, err
			}
			return map[string]any{"serveConferma": t}, nil
		}
	}
	esitiTS := []string{}
	if caratteristicheTS[e.TSCar] != "" && e.TSCD >= 1 && e.TSCD <= 30 { // Anatema: chi riesce il TS non subisce l'effetto
		restano := []*vistaCombattente{}
		for _, b := range bers {
			r, _ := s.tiraTS(b, e.TSCar, e.TSCD, nil)
			if r.Riuscito {
				esitiTS = append(esitiTS, b.Nome+" riesce")
			} else {
				esitiTS = append(esitiTS, b.Nome+" fallisce")
				restano = append(restano, b)
			}
		}
		if len(restano) == 0 {
			t := a.Nome + ": " + e.Nome + " (TS su " + caratteristicheTS[e.TSCar] + " CD " + itoa(e.TSCD) + ": " + strings.Join(esitiTS, ", ") + "): nessuno è colpito"
			registro(t)
			s.pubblicaCombattimento(sessione)
			return map[string]any{"fatto": true, "testo": t}, nil
		}
		bers = restano
	}
	if e.fineConc && slices.Contains(a.Condizioni, "Concentrato") {
		s.rimuoviCondizioni(u, a, []string{"Concentrato"})
		s.finisceConcentrazione(u, comb, a.ID, a.Nome+": "+e.Nome)
	}
	if e.Conc {
		s.finisceConcentrazione(u, comb, a.ID, a.Nome+" si concentra su "+e.Nome)
		s.aggiungiCondizioni(u, a, []string{"Concentrato"})
	}
	for _, x := range s.caricaEffetti(comb) {
		if x.Nome == e.Nome && x.Incantatore == a.ID {
			s.terminaEffetto(u, comb, x, "di nuovo")
		}
	}
	lista, nomi := []bersaglioEffetto{}, []string{}
	for _, b := range bers {
		messe := []string{}
		if len(cond) > 0 {
			if x := s.aggiungiCondizioni(u, b, cond); x != nil {
				messe = x
			}
		}
		lista = append(lista, bersaglioEffetto{ID: b.ID, Nome: b.Nome, Condizioni: messe})
		nomi = append(nomi, b.Nome)
	}
	fine := 0
	if e.Round > 0 {
		fine = round + e.Round
	}
	mod := ""
	if !e.Mod.vuota() {
		b, _ := json.Marshal(e.Mod)
		mod = string(b)
	}
	bj, _ := json.Marshal(lista)
	conc := 0
	if e.Conc {
		conc = 1
	}
	s.DB.Exec(`INSERT INTO effetti_scontro(id, combattimento_id, incantatore_id, nome, bersagli, conc, inizio_round, fine_round, ts, creato_il, modifiche)
		VALUES(?,?,?,?,?,?,?,?,'',?,?)`, cripto.ID(), comb, a.ID, e.Nome, string(bj), conc, round, fine, db.Ora(), mod)
	if mod != "" { // le schede aperte dei PG bersaglio mostrano la CA cambiata
		for _, b := range bers {
			if b.Tipo == "pg" && b.Personaggio != "" {
				s.avvisaStatoInGioco(b.Personaggio, b.Proprietario)
			}
		}
	}
	t := a.Nome + ": " + e.Nome
	if len(bers) > 1 || bers[0].ID != a.ID {
		t += " su " + strings.Join(nomi, ", ")
	}
	dett := []string{}
	if x := testoModifiche(e.Mod); x != "" {
		dett = append(dett, x)
	}
	if len(cond) > 0 {
		dett = append(dett, strings.ToLower(strings.Join(cond, ", ")))
	}
	if e.Conc {
		dett = append(dett, "concentrazione")
	}
	if e.Round > 0 {
		dett = append(dett, itoa(e.Round)+" round")
	}
	if len(esitiTS) > 0 {
		dett = append(dett, "TS su "+caratteristicheTS[e.TSCar]+" CD "+itoa(e.TSCD)+": "+strings.Join(esitiTS, ", "))
	}
	t += " (" + strings.Join(dett, "; ") + ")"
	registro(t)
	s.pubblicaCombattimento(sessione)
	return map[string]any{"fatto": true, "testo": t}, nil
}

// iraPG («pg.ira»): il barbaro entra in ira (azione bonus, 10 round) con le modifiche calcolate dalla scheda (riepilogo
// «ira»: resistenze, vantaggio ai TS su Forza, danni in mischia; Totem dell'orso, Berserker). L'ira interrompe la
// concentrazione. Di nuovo «pg.ira» mentre è in ira: l'ira termina.
func (s *Server) iraPG(u *Utente, sessione, comb string, round int, turnoDi string, master bool, a *vistaCombattente,
	tutti []*vistaCombattente, d datiAzione, registro func(string)) (any, error) {
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
	ricorda := func(r map[string]any) (any, error) {
		if d.Azione != "" {
			m.risolti[d.Azione], m.quando[d.Azione] = r, time.Now()
		}
		return r, nil
	}
	for _, e := range s.caricaEffetti(comb) {
		if e.Nome == "Ira" && e.Incantatore == a.ID {
			s.terminaEffetto(u, comb, e, a.Nome+" smette")
			s.pubblicaCombattimento(sessione)
			return ricorda(map[string]any{"fatto": true, "testo": a.Nome + " non è più in ira"})
		}
	}
	var raw string
	s.DB.QueryRow(`SELECT COALESCE(v.riepilogo,'{}') FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		WHERE p.id = ?`, a.Personaggio).Scan(&raw)
	var r riepilogo
	json.Unmarshal([]byte(raw), &r)
	if r.Ira == nil {
		return nil, errore(http.StatusConflict, "La scheda di "+a.Nome+" non ha l'ira del barbaro: se l'hai appena aggiornata, aprila una volta.")
	}
	d.Valori = &effettoValori{Nome: "Ira", Tempo: "bonus", Round: 10, Mod: *r.Ira, fineConc: true}
	d.Bersagli = []string{a.ID}
	x, err := s.creaEffettoValori(u, sessione, comb, round, turnoDi, master, a, tutti, d, registro)
	if mm, ok := x.(map[string]any); ok && err == nil && mm["fatto"] == true {
		return ricorda(mm)
	}
	return x, err
}

// effettiDurevoliFuori: fuori dallo scontro, la CA del PG cambiata dagli effetti che durano (con l'originale).
func (s *Server) effettiDurevoliFuori(id string, durevoli []effettoDurevole) *effettiPG {
	if len(durevoli) == 0 {
		return nil
	}
	var riep string
	s.DB.QueryRow(`SELECT COALESCE(v.riepilogo,'{}') FROM personaggi p LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		WHERE p.id = ?`, id).Scan(&riep)
	var r riepilogo
	json.Unmarshal([]byte(riep), &r)
	c := &vistaCombattente{ID: "pg:" + id, Tipo: "pg", Personaggio: id}
	if r.CA > 0 {
		c.CA = intP(r.CA)
	}
	if r.Des > 0 {
		c.desMod = intP(Modificatore(r.Des))
	}
	s.applicaEffettiValori("", []*vistaCombattente{c})
	out := &effettiPG{}
	if c.CAOrig != nil {
		out.CA, out.CAOrig = c.CA, c.CAOrig
	}
	for _, d := range durevoli {
		if d.Mod != nil {
			out.Voci = append(out.Voci, d.Nome+": "+d.Testo)
		}
	}
	if out.CA == nil && len(out.Voci) == 0 {
		return nil
	}
	return out
}
