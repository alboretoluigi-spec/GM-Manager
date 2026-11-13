package server

// Regole automatiche della Sanità mentale (richiesta dell'utente, 2026-11-06). Valgono per i tiri salvezza
// contro la paura (eventi) e per il TS su Sanità mentale con una CD:
//   - TS fallito di 5 o più: Terrore +1;
//   - tre TS falliti di seguito: Terrore +2 (poi il conteggio riparte; un successo lo azzera);
//   - TS fallito con 1 sul d20: follia temporanea casuale;
//   - il Terrore arriva a 4: follia temporanea casuale; arriva a 6: follia permanente (indeterminata) casuale.
// Le regole sul Terrore valgono con la scala del Terrore attiva, quelle sulle follie con le follie attive.
// Le soglie di 4 e 6 valgono per qualunque aumento del Terrore (eventi, Master, regole opzionali).
// Chi ha l'immunità completa al Terrore non riceve né Terrore né la follia del 1 sul d20.

import "strconv"

// RegoleTSSanita calcola il Terrore in più e il nuovo conteggio dei fallimenti consecutivi dopo un TS.
func RegoleTSSanita(ts EsitoTS, fallimentiPrima int) (extra, fallimenti int, motivi []string) {
	if ts.Successo {
		return 0, 0, nil
	}
	fallimenti = fallimentiPrima + 1
	if ts.CD-ts.Totale >= 5 {
		extra++
		motivi = append(motivi, "TS fallito di "+strconv.Itoa(ts.CD-ts.Totale)+" (5 o più): Terrore +1")
	}
	if fallimenti >= 3 {
		extra += 2
		fallimenti = 0
		motivi = append(motivi, "terzo TS fallito di seguito: Terrore +2")
	}
	return
}

// UnoNaturale: TS fallito con 1 sul d20 tenuto.
func UnoNaturale(ts EsitoTS) bool { return !ts.Successo && ts.Scelto == 1 }

func (s *Server) fallimentiConsecutivi(pg string) int {
	var n int
	s.DB.QueryRow(`SELECT fallimenti_consecutivi FROM sanita_pg WHERE personaggio_id = ?`, pg).Scan(&n)
	return n
}

// pazzieAutomatiche assegna le follie casuali dovute a un 1 naturale e alle soglie 4 e 6 del Terrore.
// Una sola follia temporanea per tiro (1 naturale e soglia 4 insieme non ne danno due).
func (s *Server) pazzieAutomatiche(pg string, u *Utente, evento string, prima, dopo int, unoNaturale bool) (assegnate []*vistaPazzia, motivi []string) {
	cfg := s.configSanita()
	if !cfg.Pazzia {
		return nil, nil
	}
	crea := func(tipo, motivo string) {
		p, err := s.creaPazzia(pg, u, nuovaPazzia{Tipo: tipo, Casuale: true, Evento: evento, Conferma: true, Override: true})
		if err != nil {
			motivi = append(motivi, motivo+": follia non assegnata ("+err.Error()+")")
			return
		}
		assegnate = append(assegnate, p)
		testo := motivo + ": follia " + map[string]string{"temporanea": "temporanea", "indeterminata": "permanente"}[tipo]
		if p.Categoria != "" {
			testo += " «" + p.Categoria + "»"
		}
		motivi = append(motivi, testo)
	}
	soglia4 := cfg.Terrore && prima < 4 && dopo >= 4
	soglia6 := cfg.Terrore && prima < 6 && dopo >= 6
	switch {
	case unoNaturale:
		crea("temporanea", "1 sul d20")
	case soglia4:
		crea("temporanea", "Terrore arrivato a "+strconv.Itoa(dopo)+" (soglia 4)")
	}
	if soglia6 {
		crea("indeterminata", "Terrore arrivato a "+strconv.Itoa(dopo)+" (soglia 6)")
	}
	if len(assegnate) > 0 {
		s.registraSanita(pg, u, "pazzie_automatiche", map[string]any{"motivi": motivi, "evento": evento})
	}
	return
}
