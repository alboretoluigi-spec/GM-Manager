package server

// Le due identità dell'Assistente IA (decisione dell'utente del 2026-11-13: «due identità distinte e separate, che non
// confondono mai le cose ma possono scambiarsi informazioni»):
//   - 📜 Archivista del canone (modo «canone»): solo i testi ufficiali della biblioteca (manuale dell'avventura, romanzi,
//     compendi, Tomo interattivo) e le schede del Codice segnate canone / romanzi / tomo interattivo. Nessun gruppo.
//   - 🏰 Cronista della campagna (modo «assistente», quello di sempre) e 🧛 l'antagonista: la lore homebrew del Master (Campaign
//     Bible, schede del Codice della campagna, documenti della campagna nella biblioteca, dati di Gaster Bless sui gruppi).
// Lo scambio: ognuna riceve, solo quando serve alla domanda, un breve «CONSULTO» dell'altra in un riquadro separato e
// firmato, con regole precise su come usarlo (mai come fatto proprio). Nelle pagine, «↔ Chiedi all'altra identità»
// manda la stessa domanda all'altra identità e ne riporta la risposta. Riferimenti: [C…] canone, [H…] campagna.

import (
	"context"
	_ "embed"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed prompt_canone.md
var promptCanonePredefinito string

func (s *Server) istruzioniCanone() string {
	t, _ := s.testoIA("ia_prompt_canone", promptCanonePredefinito)
	return t
}

// sistemaCanone: istruzioni dell'Archivista (il resto, brani e schede, lo aggiunge conoscenzaIA).
func (s *Server) sistemaCanone() string { return strings.TrimSpace(s.istruzioniCanone()) }

// scambioIA: consulto automatico tra le due identità (impostazione «ia_scambio»; predefinito sì).
func (s *Server) scambioIA() bool {
	v, _ := s.impostazione("ia_scambio")
	return v != "no"
}

// ricercaApprofondita: prima di cercare nei testi, l'IA scrive le parole chiave (nomi, sinonimi). Predefinito: sì con i
// servizi online (un attimo), no con Ollama (sul PC ogni richiesta in più costa secondi). Impostazione «ia_ricerca».
func (s *Server) ricercaApprofondita(c configIA) bool {
	v, _ := s.impostazione("ia_ricerca")
	switch v {
	case "si":
		return true
	case "no":
		return false
	}
	return c.Fornitore != "ollama"
}

const regoleCronista = `IDENTITÀ: CRONISTA DELLA CAMPAGNA
In questa conversazione sei il Cronista della campagna homebrew del Master. La tua verità è la sua lore: queste istruzioni, il Codice della campagna (schede con etichetta campagna o variante, e quelle senza etichetta di fonte), i documenti della campagna (riferimenti [H1], [H2]…) e i dati di Gaster Bless sui gruppi.
Esiste un'altra identità dell'Assistente, l'Archivista del canone, che conosce i testi ufficiali (manuale dell'avventura, romanzi, compendi, Tomo interattivo). Regole:
- Ciò che dice la campagna vale sempre, anche quando il canone dice altro.
- Le informazioni del canone ti arrivano solo dentro il riquadro «CONSULTO DELL'ARCHIVISTA DEL CANONE». Usale solo se servono alla domanda, sempre dichiarate («Secondo il canone [C2]…») e separate da ciò che vale nella campagna.
- Se la campagna non stabilisce una cosa e il canone sì, dillo così: «Nella tua campagna non è stabilito; nel canone…», e chiedi al Master se vuole adottarla.
- Mai presentare un fatto del canone come fatto della campagna, né il contrario. Se nel dubbio, dichiara la fonte.
- Quando usi un documento della campagna, metti il suo riferimento ([H1]).`

const regoleAntagonistaFonti = `LE FONTI DI CIÒ CHE SAI
La tua storia, in questa campagna, è quella scritta dal Master: le sue istruzioni, il Codice della campagna e i dati di Gaster Bless valgono sempre. Il riquadro «CRONACHE UFFICIALI» riporta ciò che i testi ufficiali (modulo, romanzi, compendi, Tomo interattivo) raccontano di te: sono ricordi validi solo dove la campagna del Master non dice altro. Quando parli in personaggio non mettere riferimenti tra parentesi quadre; se il Master ti chiede da dove viene un'informazione, distingui ciò che è canone da ciò che è la sua campagna.`

// budget di caratteri per ogni parte (servizi online / Ollama, che ha un contesto più piccolo)
type budgetIA struct{ propri, schede, consultoTesti, consultoSchede int }

func budgetPer(modo string, ollama bool) budgetIA {
	switch {
	case modo == "canone" && ollama:
		return budgetIA{8000, 3500, 1500, 1500}
	case modo == "canone":
		return budgetIA{20000, 8000, 3000, 3000}
	case modo == "antagonista" && ollama:
		return budgetIA{2500, maxLoreOllama, 2500, 1500}
	case modo == "antagonista":
		return budgetIA{6000, maxLoreIA, 6000, 4000}
	case ollama:
		return budgetIA{3000, maxLoreOllama, 2000, 1200}
	}
	return budgetIA{8000, maxLoreIA, 6000, 3000}
}

// Copertura minima (quanta parte della domanda contiene il brano migliore) per i brani propri e per il consulto:
// il consulto parte solo quando l'altra identità ha qualcosa di davvero pertinente.
const (
	sogliaPropri   = 0.12
	sogliaConsulto = 0.5
)

// conoscenzaIA: il testo da aggiungere alle istruzioni per l'identità della conversazione, e i brani citabili (per le
// fonti sotto la risposta). ricerca: domanda + messaggio precedente del Master (per il Codice); parole: le parole chiave
// scritte dall'IA (ricerca approfondita), se ci sono.
func (s *Server) conoscenzaIA(modo, gid, domanda, precedente string, parole []string, ollama bool) (string, []passoBib) {
	bu := budgetPer(modo, ollama)
	ricerca := domanda
	if precedente != "" {
		ricerca += "\n" + precedente
	}
	alias := append(s.aliasNominati(ricerca), parole...)
	q := terminiBib(domanda, precedente, alias)
	scambio := s.scambioIA()
	var b strings.Builder
	var fonti []passoBib
	scaffale := func(dominio string) (bool, bool) { // c'è almeno una fonte attiva? / in tutto?
		attive, tutte := false, false
		for _, f := range s.fontiBib() {
			if f.Dominio == dominio {
				tutte = true
				attive = attive || f.Attiva
			}
		}
		return attive, tutte
	}
	if modo == "canone" {
		attive, tutte := scaffale("canone")
		passi := s.cercaBib(q, "canone", bu.propri, sogliaPropri)
		switch {
		case len(passi) > 0:
			t, p := testoPassi(passi, "C", 1)
			b.WriteString("\n\n---\nTESTI DEL CANONE PERTINENTI ALLA DOMANDA (brani scelti da Gaster Bless nella biblioteca; citali con il loro riferimento):" + t)
			fonti = append(fonti, p...)
		case attive:
			b.WriteString("\n\n---\nTESTI DEL CANONE: nella biblioteca nessun brano sembra pertinente a questa domanda. Se la risposta non è nemmeno nelle schede qui sotto, dillo e suggerisci al Master come chiedere (un nome, un luogo, un capitolo).")
		case tutte:
			b.WriteString("\n\n---\nTESTI DEL CANONE: i testi della biblioteca sono tutti disattivati. Rispondi solo dalle schede del canone e dillo al Master.")
		default:
			b.WriteString("\n\n---\nTESTI DEL CANONE: la biblioteca è ancora vuota (il Master non ha caricato testi in «📚 Biblioteca»). Rispondi solo dalle schede del canone qui sotto e avvisa il Master che le risposte saranno più precise quando caricherà i testi.")
		}
		if sc := s.contestoLoreDi("", ricerca, bu.schede, "canone", false, "SCHEDE DEL CANONE NEL CODICE (riassunti del Master dai testi ufficiali; etichette canone, romanzi, tomo interattivo):"); sc != "" {
			b.WriteString("\n\n---\n" + sc)
		}
		if scambio {
			var c strings.Builder
			if sc := s.contestoLoreDi("", ricerca, bu.consultoSchede, "campagna", true, "Schede della campagna che riguardano la domanda:"); sc != "" {
				c.WriteString("\n" + sc)
			}
			if passi := s.cercaBib(q, "campagna", bu.consultoTesti, sogliaConsulto); len(passi) > 0 {
				t, p := testoPassi(passi, "H", 1)
				c.WriteString("\n\nDocumenti della campagna:" + t)
				fonti = append(fonti, p...)
			}
			if c.Len() > 0 {
				b.WriteString("\n\n=== CONSULTO DEL CRONISTA DELLA CAMPAGNA (decisioni homebrew del Master: NON sono canone) ===\n" +
					"Usalo solo per aggiungere, separato e in fondo alla risposta, «Nella campagna del Master invece…». Non mescolarlo con il canone." +
					c.String() + "\n=== FINE DEL CONSULTO ===")
			}
		}
		return b.String(), fonti
	}
	// Cronista della campagna e l'antagonista
	if passi := s.cercaBib(q, "campagna", bu.propri, sogliaPropri); len(passi) > 0 {
		t, p := testoPassi(passi, "H", 1)
		b.WriteString("\n\n---\nDOCUMENTI DELLA CAMPAGNA PERTINENTI ALLA DOMANDA (testi del Master nella biblioteca; citali con il loro riferimento):" + t)
		fonti = append(fonti, p...)
	}
	if scambio {
		var c strings.Builder
		if passi := s.cercaBib(q, "canone", bu.consultoTesti, sogliaConsulto); len(passi) > 0 {
			t, p := testoPassi(passi, "C", 1)
			c.WriteString("\nBrani dei testi ufficiali:" + t)
			fonti = append(fonti, p...)
		}
		if sc := s.contestoLoreDi("", ricerca, bu.consultoSchede, "canone", true, "Schede del canone nel Codice:"); sc != "" {
			c.WriteString("\n\n" + sc)
		}
		if c.Len() > 0 {
			if modo == "antagonista" {
				b.WriteString("\n\n=== CRONACHE UFFICIALI (ciò che i testi ufficiali raccontano: valgono solo dove la campagna del Master non dice altro) ===" +
					c.String() + "\n=== FINE DELLE CRONACHE ===")
			} else {
				b.WriteString("\n\n=== CONSULTO DELL'ARCHIVISTA DEL CANONE (testi ufficiali: NON è la campagna del Master) ===\n" +
					"Usalo solo se serve, sempre dichiarato («Secondo il canone…») e separato da ciò che vale nella campagna." +
					c.String() + "\n=== FINE DEL CONSULTO ===")
			}
		}
	}
	return b.String(), fonti
}

// aliasNominati: per ogni scheda del Codice nominata nella domanda (nome o alias), gli altri suoi nomi: così la
// ricerca nei testi trova anche «Mirela» quando si chiede di «Elara» (se la scheda li collega).
func (s *Server) aliasNominati(testo string) []string {
	l, err := s.caricaLore("")
	if err != nil || len(l) == 0 {
		return nil
	}
	dom := normalizza(testo)
	var out []string
	for _, x := range l {
		nomi := []string{x.Nome}
		for k, v := range x.Campi {
			if strings.EqualFold(strings.TrimSpace(k), "alias") {
				nomi = append(nomi, strings.FieldsFunc(v, func(r rune) bool { return r == ';' || r == ',' })...)
			}
		}
		if len(nomi) < 2 {
			continue
		}
		trovata := false
		for _, n := range nomi {
			if n = normalizza(n); utf8.RuneCountInString(n) >= 4 && strings.Contains(dom, n) {
				trovata = true
				break
			}
		}
		if trovata {
			out = append(out, strings.Join(nomi, " "))
		}
		if len(out) >= 6 {
			break
		}
	}
	return out
}

const promptParoleChiave = `Aiuti a cercare in una biblioteca di testi su l'avventura (manuale dell'avventura in italiano, romanzi, compendi, Tomo interattivo, documenti di una campagna) con una ricerca per parole, non per significato.
Data la domanda del Master, scrivi da 2 a 4 righe di parole chiave in italiano: i nomi propri (con le varianti e i soprannomi noti), i luoghi, gli oggetti e i sinonimi delle parole importanti (per esempio «cavallo destriero»). Se conosci il nome di ciò che la domanda descrive senza nominarlo, aggiungilo.
Una riga per ricerca, solo parole separate da spazi: niente frasi, spiegazioni, numeri d'elenco o virgolette.`

// paroleChiaveIA: ricerca approfondita. Un'unica richiesta breve allo stesso servizio dell'Assistente; se non risponde
// entro 25 secondi si cerca con le sole parole della domanda.
func (s *Server) paroleChiaveIA(ctx context.Context, c configIA, chiave, domanda, precedente string) []string {
	if !s.ricercaApprofondita(c) || len(s.indiceBiblioteca().fonti) == 0 {
		return nil
	}
	ctx, annulla := context.WithTimeout(ctx, 25*time.Second)
	defer annulla()
	msg := "Domanda: " + accorcia(domanda, 2000)
	if precedente != "" {
		msg = "Messaggio precedente del Master: " + accorcia(precedente, 600) + "\n\n" + msg
	}
	var b strings.Builder
	pezzo := func(t string) {
		if b.Len() < 3000 {
			b.WriteString(t)
		}
	}
	storia := []messaggioIA{{Ruolo: "master", Testo: msg}}
	c.Varia = false
	var err error
	if c.Fornitore == "claude" {
		err = chiediClaude(ctx, chiave, c.Modello, promptParoleChiave, storia, pezzo)
	} else {
		err = chiediCompatibile(ctx, c, chiave, promptParoleChiave, storia, pezzo)
	}
	if err != nil && b.Len() == 0 {
		return nil
	}
	var out []string
	for _, r := range strings.Split(b.String(), "\n") {
		r = strings.TrimSpace(strings.TrimLeft(r, "-*•0123456789.) \t"))
		r = strings.Trim(r, "\"«»“”")
		if r == "" || utf8.RuneCountInString(r) > 200 {
			continue
		}
		out = append(out, r)
		if len(out) == 4 {
			break
		}
	}
	return out
}
