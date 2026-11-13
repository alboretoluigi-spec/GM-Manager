package server

// «[Creatura] prende di mira [Bersaglio]» (richiesta dell'utente, 2026-11-12). Nel combattimento, quando il Master sceglie
// il bersaglio di un attacco o di un incantesimo di un mostro o di un PNG (finestre web/ui/attacco.js e
// web/ui/incantesimo_scontro.js), tutti i presenti alla sessione vedono il messaggio sullo schermo (web/ui/mira.js).
// Un mostro nascosto non si rivela: niente messaggio. I bersagli nascosti non si nominano a chi non è il Master.

import "slices"

type miraMsg struct {
	Sessione  string   `json:"sessione"`
	Chi       string   `json:"chi"`
	Bersagli  []string `json:"bersagli"`  // nomi
	Proprieta []string `json:"proprieta"` // utenti dei PG presi di mira (per evidenziarlo a loro)
}

func (s *Server) prendeDiMira(sessione string, cb *vistaCombattente, tutti []*vistaCombattente, d datiAzione) (any, error) {
	ids := d.Bersagli
	if d.Bersaglio != "" {
		ids = append([]string{d.Bersaglio}, ids...)
	}
	if len(ids) == 0 || len(ids) > 20 {
		return nil, errDatiNonValidi
	}
	var pub, mast miraMsg
	pub.Sessione, pub.Chi = sessione, cb.Nome
	mast = pub
	for _, x := range tutti {
		if !slices.Contains(ids, x.ID) || x.ID == cb.ID {
			continue
		}
		mast.Bersagli = append(mast.Bersagli, x.Nome)
		if !x.Nascosto {
			pub.Bersagli = append(pub.Bersagli, x.Nome)
		}
		if x.Tipo == "pg" && x.Proprietario != "" {
			pub.Proprieta = append(pub.Proprieta, x.Proprietario)
		}
	}
	if len(mast.Bersagli) == 0 {
		return nil, errDatiNonValidi
	}
	if cb.Nascosto { // il mostro nascosto resta nascosto
		return map[string]any{"inviato": false}, nil
	}
	for _, id := range s.destinatariSessione(sessione) {
		var u Utente
		s.DB.QueryRow(`SELECT ruolo FROM utenti WHERE id = ?`, id).Scan(&u.Ruolo)
		m := pub
		if u.Master() {
			m = mast
			m.Proprieta = pub.Proprieta
		} else if len(m.Bersagli) == 0 {
			continue
		}
		s.Hub.AUtente(id, Msg{Tipo: "iniz.mira", Dati: m})
	}
	return map[string]any{"inviato": true}, nil
}
