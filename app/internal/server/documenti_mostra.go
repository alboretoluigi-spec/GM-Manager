package server

// «Mostra» un documento (richiesta dell'utente, 2026-11-09): il giocatore (o il Master) fa comparire un suo documento a
// tutto schermo sui dispositivi dei giocatori scelti, o di tutti i suoi compagni. Chi lo riceve può solo guardarlo,
// per 2 ore (il permesso è in memoria: un riavvio di Gaster Bless lo toglie); non ne riceve una copia (per quello c'è
// «Consegna»). Il documento resta privato: nella lista dei documenti di chi lo guarda non compare.

import (
	"net/http"
	"sync"
	"time"
)

const durataVisione = 2 * time.Hour

type visioniDocumenti struct {
	mu sync.Mutex
	v  map[string]time.Time // "documento|utente" → scadenza
}

func (x *visioniDocumenti) concedi(doc, utente string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.v == nil {
		x.v = map[string]time.Time{}
	}
	ora := time.Now()
	for k, t := range x.v { // pulizia delle scadute
		if ora.After(t) {
			delete(x.v, k)
		}
	}
	x.v[doc+"|"+utente] = ora.Add(durataVisione)
}

func (x *visioniDocumenti) puo(doc, utente string) bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	t, ok := x.v[doc+"|"+utente]
	return ok && time.Now().Before(t)
}

// apiMostraDocumento: {a: ["id", ...] | ["tutti"], personaggio: "id"}.
func (s *Server) apiMostraDocumento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	d, _, err := s.documento(u, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	var in struct {
		A           []string `json:"a"`
		Personaggio string   `json:"personaggio"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	ammessi := s.destinatariDi(u)
	var dest []string
	for _, a := range in.A {
		if a == "tutti" {
			dest = nil
			for _, x := range ammessi {
				if x["id"] != d.Proprietario {
					dest = append(dest, x["id"])
				}
			}
			break
		}
		ok := false
		for _, x := range ammessi {
			ok = ok || x["id"] == a
		}
		if !ok || a == d.Proprietario {
			return nil, errore(http.StatusBadRequest, "Destinatario non valido.")
		}
		dest = append(dest, a)
	}
	if len(dest) == 0 {
		return nil, errore(http.StatusBadRequest, "Scegli a chi mostrare il documento.")
	}
	chi := u.Nome
	if u.Master() {
		chi = "Il Master"
	}
	if in.Personaggio != "" {
		var nome string
		if s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ? AND proprietario_id = ? AND stato = 'attivo'`, in.Personaggio, u.ID).Scan(&nome) != nil {
			return nil, errore(http.StatusBadRequest, "Personaggio non valido.")
		}
		chi = nome
	}
	m := Msg{Tipo: "documento.mostrato", Dati: map[string]any{"id": d.ID, "nome": d.Nome, "tipo": d.Tipo, "da": chi}}
	for _, a := range dest {
		s.visioni.concedi(d.ID, a)
		s.Hub.AUtente(a, m)
	}
	return map[string]any{"mostrato": len(dest)}, nil
}
