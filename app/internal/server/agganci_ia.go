package server

// Agganci per l'antagonista scritti dall'assistente IA (richiesta dell'utente, 2026-11-09).
//
// Sostituiscono l'analisi automatica a temi fissi della «Scheda per il Master» nella sezione Background, che
// proponeva sempre gli stessi punti. Per ogni personaggio con un background l'IA scelta in «⚙ Impostazioni IA» legge
// la storia, la scheda (classe, razza, tratti, ideali, legami, difetti) e i riassunti delle ultime sessioni del gruppo,
// e scrive: sintesi, agganci per l'antagonista (con il dettaglio da cui nascono) e opportunità nate dalle ultime sessioni.
//
// Gli agganci si rigenerano da soli quando cambiano i dati usati (impronta): background salvato, riassunto
// pubblicato o modificato, scheda cambiata (controllata quando il Master apre il Background). Una richiesta alla
// volta, in sottofondo: le pagine si aggiornano quando l'IA ha finito. Il Master può chiedere «🔄 Rigenera».
// Se l'IA non risponde (per esempio Ollama spento) restano gli ultimi agganci scritti e l'errore si vede nella scheda.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

type codaAgganci struct {
	mu      sync.Mutex
	attesa  map[string]bool // personaggio → rigenerazione forzata
	inCorso string
	errori  map[string]erroreAgganci
	segnale chan struct{}
	avvio   sync.Once
}

type erroreAgganci struct {
	msg, impronta string
	il            time.Time
}

func (s *Server) rotteAgganciIA(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/personaggi/{id}/agganci-ia", api(s.apiRigeneraAgganci))
}

const maxRiassuntiAgganci = 6

// datiAgganci prepara la richiesta per l'IA e l'impronta dei dati usati. ok = false se il personaggio non ha un
// background.
func (s *Server) datiAgganci(pg string) (domanda, impronta string, ok bool) {
	var nome, gruppo, bg, dati, riep, nomeGruppo string
	if s.DB.QueryRow(`SELECT p.nome, COALESCE(p.gruppo_id,''), b.testo, COALESCE(v.dati,'{}'), COALESCE(v.riepilogo,'{}'), COALESCE(g.nome,'')
		FROM personaggi p JOIN background_testi b ON b.personaggio_id = p.id
		LEFT JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		LEFT JOIN gruppi g ON g.id = p.gruppo_id
		WHERE p.id = ? AND p.stato != 'archiviato'`, pg).Scan(&nome, &gruppo, &bg, &dati, &riep, &nomeGruppo) != nil {
		return "", "", false
	}
	var rp riepilogo
	json.Unmarshal([]byte(riep), &rp)
	var d struct {
		Bg string         `json:"bg"`
		Al string         `json:"al"`
		F  map[string]any `json:"f"`
	}
	json.Unmarshal([]byte(dati), &d)
	var b strings.Builder
	fmt.Fprintf(&b, "PERSONAGGIO: %s", nome)
	parti := []string{}
	for _, x := range []string{rp.Razza, strings.TrimSpace(rp.Classe + " " + rp.Sottocl)} {
		if x != "" {
			parti = append(parti, x)
		}
	}
	if rp.Livello > 0 {
		parti = append(parti, fmt.Sprintf("livello %d", rp.Livello))
	}
	if d.Bg != "" {
		parti = append(parti, "background: "+d.Bg)
	}
	if d.Al != "" {
		parti = append(parti, d.Al)
	}
	if len(parti) > 0 {
		b.WriteString(" (" + strings.Join(parti, ", ") + ")")
	}
	b.WriteString(".\n")
	for _, k := range []struct{ chiave, nome string }{{"tratti", "Tratti caratteriali"}, {"ideali", "Ideali"}, {"legami", "Legami"}, {"difetti", "Difetti"}} {
		if v, ok := d.F[k.chiave].(string); ok && strings.TrimSpace(v) != "" {
			fmt.Fprintf(&b, "%s: %s\n", k.nome, accorcia(strings.Join(strings.Fields(v), " "), 500))
		}
	}
	fmt.Fprintf(&b, "\nSTORIA SCRITTA DAL GIOCATORE:\n%s\n", accorcia(bg, 12000))
	// Riassunti delle ultime sessioni del gruppo (dal più vecchio al più recente).
	var rias []string
	if gruppo != "" {
		if rows, err := s.DB.Query(`SELECT titolo, testo FROM riassunti WHERE gruppo_id = ? ORDER BY creato_il DESC LIMIT ?`, gruppo, maxRiassuntiAgganci); err == nil {
			for rows.Next() {
				var t, x string
				rows.Scan(&t, &x)
				rias = append([]string{fmt.Sprintf("- «%s»: %s", t, accorcia(strings.Join(strings.Fields(strings.NewReplacer("#", "", "**", "").Replace(x)), " "), 2000))}, rias...)
			}
			rows.Close()
		}
	}
	if len(rias) > 0 {
		fmt.Fprintf(&b, "\nRIASSUNTI DELLE ULTIME SESSIONI DEL GRUPPO %s (dal più vecchio al più recente):\n%s\n", nomeGruppo, strings.Join(rias, "\n"))
	} else {
		b.WriteString("\nRIASSUNTI DELLE ULTIME SESSIONI: nessuno ancora.\n")
	}
	h := sha256.Sum256([]byte(b.String()))
	impronta = hex.EncodeToString(h[:12])
	// Memoria: gli agganci già proposti (non entrano nell'impronta, altrimenti si rigenererebbero all'infinito).
	var prima string
	s.DB.QueryRow(`SELECT testo FROM ia_agganci_bg WHERE personaggio_id = ?`, pg).Scan(&prima)
	if strings.TrimSpace(prima) != "" {
		fmt.Fprintf(&b, "\nLA SCHEDA CHE AVEVI GIÀ SCRITTO PER QUESTO PERSONAGGIO (riparti da qui):\n%s\n", accorcia(prima, 6000))
	}
	domanda = `Scrivi la scheda per il Master di questo personaggio, per la campagna «l'avventura». Usa esattamente queste tre sezioni, con questi titoli:

## Sintesi
Da 3 a 5 punti brevi: chi è, cosa vuole, cosa teme, cosa ha lasciato indietro.

## Agganci per l'antagonista
Da 4 a 6 punti. Ogni punto: **titolo breve**: come l'antagonista (o i suoi servitori, o la valle stessa) può usare questo dettaglio, in modo concreto e giocabile. Termina ogni punto con «Nasce da: …» indicando il dettaglio della storia o della scheda. Cerca idee originali e specifiche di questo personaggio, non quelle ovvie valide per chiunque.

## Opportunità dalle ultime sessioni
Da 2 a 4 punti che collegano quello che è successo nei riassunti con la storia del personaggio: mosse che l'antagonista può fare adesso, PNG già incontrati che può usare, conseguenze delle scelte del gruppo. Se non ci sono riassunti, scrivi una sola riga che lo dice.

Se qui sotto trovi la scheda che avevi già scritto, ragiona come in una campagna che prosegue: tieni gli agganci ancora validi (aggiornandoli con quello che è successo nelle sessioni), togli quelli superati dagli eventi e aggiungi quelli nuovi, segnandoli all'inizio con «🆕».

Non contraddire la storia del giocatore e non inventare fatti certi sul suo passato: le idee nuove presentale come possibilità. Scrivi in italiano, in modo sintetico.

` + b.String()
	domanda = regolaGruppo(nomeGruppo) + "\n\n" + domanda
	return domanda, impronta, true
}

// programmaAgganci mette il personaggio in coda (se un'IA è scelta). forza: rigenera anche se i dati non sono cambiati.
func (s *Server) programmaAgganci(pg string, forza bool) {
	if s.configIA().Fornitore == "" {
		return
	}
	q := &s.agganci
	q.mu.Lock()
	if q.attesa == nil {
		q.attesa = map[string]bool{}
		q.errori = map[string]erroreAgganci{}
		q.segnale = make(chan struct{}, 1)
	}
	q.attesa[pg] = q.attesa[pg] || forza
	q.mu.Unlock()
	q.avvio.Do(func() { go s.lavoraAgganci() })
	select {
	case q.segnale <- struct{}{}:
	default:
	}
}

// programmaAgganciGruppo: dopo un riassunto nuovo o cambiato, tutti i personaggi del gruppo con un background.
func (s *Server) programmaAgganciGruppo(gruppo string) {
	if s.configIA().Fornitore == "" {
		return
	}
	rows, err := s.DB.Query(`SELECT p.id FROM personaggi p JOIN background_testi b ON b.personaggio_id = p.id
		WHERE p.gruppo_id = ? AND p.stato != 'archiviato'`, gruppo)
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		s.programmaAgganci(id, false)
	}
}

func (s *Server) lavoraAgganci() {
	q := &s.agganci
	ctx, annulla := context.WithCancel(context.Background())
	go func() { <-s.chiudi; annulla() }()
	for {
		q.mu.Lock()
		pg, forza := "", false
		for k, v := range q.attesa {
			pg, forza = k, v
			break
		}
		if pg != "" {
			delete(q.attesa, pg)
			q.inCorso = pg
		}
		q.mu.Unlock()
		if pg == "" {
			select {
			case <-q.segnale:
				continue
			case <-ctx.Done():
				return
			}
		}
		if strings.HasPrefix(pg, "r:") {
			s.generaProposta(ctx, strings.TrimPrefix(pg, "r:"))
		} else {
			s.generaAgganci(ctx, pg, forza)
		}
		q.mu.Lock()
		q.inCorso = ""
		q.mu.Unlock()
		if ctx.Err() != nil {
			return
		}
	}
}

func (s *Server) generaAgganci(ctx context.Context, pg string, forza bool) {
	domanda, imp, ok := s.datiAgganci(pg)
	if !ok {
		return
	}
	var salvata string
	s.DB.QueryRow(`SELECT impronta FROM ia_agganci_bg WHERE personaggio_id = ?`, pg).Scan(&salvata)
	if salvata == imp && !forza {
		return
	}
	q := &s.agganci
	if !forza && q.fallitoDaPoco(pg, imp) {
		return // ha appena fallito con gli stessi dati: non insistere a ogni apertura della pagina
	}
	c := s.configIA()
	s.Hub.AlMaster(Msg{Tipo: "background.aggiornato", Dati: map[string]string{"personaggio": pg}}) // «l'IA sta scrivendo…»
	ctxR, annulla := context.WithTimeout(ctx, 6*time.Minute)
	defer annulla()
	var b strings.Builder
	pezzo := func(t string) { b.WriteString(t) }
	storia := []messaggioIA{{Ruolo: "master", Testo: domanda}}
	var err error
	if c.Fornitore == "claude" {
		err = chiediClaude(ctxR, s.chiaveIA(c.Fornitore), c.Modello, s.istruzioniAssistente(), storia, pezzo)
	} else {
		err = chiediCompatibile(ctxR, c, s.chiaveIA(c.Fornitore), s.istruzioniAssistente(), storia, pezzo)
	}
	testoA := strings.TrimSpace(b.String())
	if err == nil && testoA == "" {
		err = errore(http.StatusBadGateway, "L'IA non ha scritto nulla.")
	}
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		q.mu.Lock()
		q.errori[pg] = erroreAgganci{msg: accorcia(err.Error(), 200), impronta: imp, il: time.Now()}
		q.mu.Unlock()
		s.Hub.AlMaster(Msg{Tipo: "background.aggiornato", Dati: map[string]string{"personaggio": pg}})
		return
	}
	if _, err := s.DB.Exec(`INSERT INTO ia_agganci_bg(personaggio_id, testo, fornitore, impronta, generato_il) VALUES(?,?,?,?,?)
		ON CONFLICT(personaggio_id) DO UPDATE SET testo = excluded.testo, fornitore = excluded.fornitore, impronta = excluded.impronta, generato_il = excluded.generato_il`,
		pg, testoA, c.Fornitore+" · "+c.Modello, imp, db.Ora()); err != nil {
		return
	}
	q.mu.Lock()
	delete(q.errori, pg)
	q.mu.Unlock()
	s.Hub.AlMaster(Msg{Tipo: "background.aggiornato", Dati: map[string]string{"personaggio": pg}})
}

// fallitoDaPoco: l'IA ha fallito con questi stessi dati meno di 10 minuti fa.
func (q *codaAgganci) fallitoDaPoco(pg, imp string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	e, c := q.errori[pg]
	return c && e.impronta == imp && time.Since(e.il) < 10*time.Minute
}

// statoAgganci: quello che la scheda per il Master mostra (solo al Master). Se i dati sono cambiati, rimette in coda.
func (s *Server) statoAgganci(pg string) map[string]any {
	out := map[string]any{"iaScelta": s.configIA().Fornitore != ""}
	var testoA, forn, salvata string
	var il int64
	if s.DB.QueryRow(`SELECT testo, fornitore, impronta, generato_il FROM ia_agganci_bg WHERE personaggio_id = ?`, pg).Scan(&testoA, &forn, &salvata, &il) == nil {
		out["testo"], out["fornitore"], out["generato"] = testoA, forn, il
	}
	_, imp, ok := s.datiAgganci(pg)
	aggiornati := ok && imp == salvata
	out["aggiornati"] = aggiornati
	if !aggiornati && ok && !s.agganci.fallitoDaPoco(pg, imp) {
		s.programmaAgganci(pg, false)
	}
	q := &s.agganci
	q.mu.Lock()
	_, inAttesa := q.attesa[pg]
	out["inCorso"] = q.inCorso == pg || inAttesa
	if e, c := q.errori[pg]; c {
		out["errore"] = e.msg
	}
	q.mu.Unlock()
	return out
}

func (s *Server) apiRigeneraAgganci(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	if _, _, ok := s.datiAgganci(r.PathValue("id")); !ok {
		return nil, errore(http.StatusNotFound, "Questo personaggio non ha ancora un background.")
	}
	if s.configIA().Fornitore == "" {
		return nil, errore(http.StatusConflict, "Scegli prima l'IA: Assistente IA → «⚙ Impostazioni IA».")
	}
	s.programmaAgganci(r.PathValue("id"), true)
	return nil, nil
}

// ===== Proposte al Master dopo ogni riassunto nuovo =====
// L'IA legge il riassunto appena pubblicato con tutta la situazione del gruppo (personaggi, appunti del Master, ultimi
// riassunti) e gli agganci già scritti, e apre per il Master una conversazione nell'Assistente: idee per ogni
// personaggio, mosse per la trama, domande. Il Master può rispondere e ragionarci insieme all'IA.

func (s *Server) programmaProposta(riassunto string) {
	s.programmaAgganci("r:"+riassunto, false)
}

func (s *Server) generaProposta(ctx context.Context, riassunto string) {
	var gruppo, titolo, testoR, nomeGruppo string
	if s.DB.QueryRow(`SELECT r.gruppo_id, r.titolo, r.testo, COALESCE(g.nome,'') FROM riassunti r LEFT JOIN gruppi g ON g.id = r.gruppo_id
		WHERE r.id = ?`, riassunto).Scan(&gruppo, &titolo, &testoR, &nomeGruppo) != nil {
		return
	}
	if strings.TrimSpace(titolo) == "" {
		titolo = "riassunto senza titolo"
	}
	var ag strings.Builder
	if rows, err := s.DB.Query(`SELECT p.nome, a.testo FROM ia_agganci_bg a JOIN personaggi p ON p.id = a.personaggio_id
		WHERE p.gruppo_id = ? AND p.stato = 'attivo' ORDER BY p.nome`, gruppo); err == nil {
		for rows.Next() {
			var n, t string
			rows.Scan(&n, &t)
			fmt.Fprintf(&ag, "\n### %s\n%s\n", n, accorcia(t, 1500))
		}
		rows.Close()
	}
	agganci := ag.String()
	if agganci == "" {
		agganci = "\n(nessuna scheda di agganci ancora scritta)\n"
	}
	domanda := fmt.Sprintf(`I giocatori del gruppo %s hanno appena pubblicato un nuovo riassunto, «%s». Leggilo e ragiona con me come un co-Master che conosce tutta la campagna. Usa queste sezioni:

## Cosa cambia
2-3 punti: cosa è successo di importante per la trama e per l'antagonista.

## Idee per ogni personaggio
Per ogni personaggio del gruppo, 1-2 nuovi agganci o sviluppi nati da questa sessione e dalla sua storia (segna con «🆕» quelli che non avevi mai proposto).

## Mosse per la trama
2-4 mosse concrete per la prossima sessione: cosa fanno l'antagonista, i suoi servitori o i PNG già incontrati.

## Domande per te
1-2 domande al Master per capire dove vuole portare la storia.

RIASSUNTO NUOVO «%s»:
%s

AGGANCI CHE HAI GIÀ PROPOSTO PER I PERSONAGGI:%s`, nomeGruppo, titolo, titolo, accorcia(testoR, 12000), agganci)
	c := s.configIA()
	if c.Fornitore == "" {
		return
	}
	sistema := s.sistemaPerGruppo(gruppo, nomeGruppo)
	ctxR, annulla := context.WithTimeout(ctx, 6*time.Minute)
	defer annulla()
	var b strings.Builder
	storia := []messaggioIA{{Ruolo: "master", Testo: domanda, Il: db.Ora()}}
	var err error
	if c.Fornitore == "claude" {
		err = chiediClaude(ctxR, s.chiaveIA(c.Fornitore), c.Modello, sistema, storia, func(t string) { b.WriteString(t) })
	} else {
		err = chiediCompatibile(ctxR, c, s.chiaveIA(c.Fornitore), sistema, storia, func(t string) { b.WriteString(t) })
	}
	risposta := strings.TrimSpace(b.String())
	if err != nil || risposta == "" {
		s.evento("ia_proposta_fallita", "", accorcia(fmt.Sprint(err), 150))
		return
	}
	storia = append(storia, messaggioIA{Ruolo: "ia", Testo: risposta, Fornitore: c.Fornitore + " · " + c.Modello, Il: db.Ora()})
	mb, _ := json.Marshal(storia)
	id, ora := cripto.ID(), db.Ora()
	tit := accorcia("💡 Dopo «"+titolo+"»: idee per l'antagonista", 80) // 💡: diversa da 🧛 «Parla con l'antagonista»
	if _, err := s.DB.Exec(`INSERT INTO ia_conversazioni(id, gruppo_id, titolo, messaggi, creato_il, modificato_il, nuova) VALUES(?,?,?,?,?,?,1)`,
		id, nullo(gruppo), tit, string(mb), ora, ora); err != nil {
		return
	}
	s.Hub.AlMaster(Msg{Tipo: "assistente.nuova", Dati: map[string]string{"conversazione": id, "titolo": tit, "gruppo": nomeGruppo}})
}
