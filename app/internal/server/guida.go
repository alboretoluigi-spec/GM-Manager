package server

// «Scarica Guida» nella barra in alto (richiesta dell'utente del 2026-11-12): il Master scarica la guida del Master, ogni
// altro utente approvato (giocatori e Segretari) la guida per i giocatori. Le guide sono incorporate nell'eseguibile
// (package guide): dopo ogni «⟳ Aggiorna» sono quelle più recenti del repository.

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"

	"gasterbless/guide"
)

func (s *Server) rotteGuida(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/guida", api(s.apiGuida))
}

func (s *Server) apiGuida(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	dati, nome := guide.Giocatori, "GasterBless_Guida_Giocatori.pdf"
	if u.Master() {
		dati, nome = guide.Master, "GasterBless_Guida_Master.pdf"
	}
	h := sha256.Sum256(dati)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`"; filename*=UTF-8''`+url.PathEscape(nome))
	w.Header().Set("Cache-Control", "no-cache") // sempre la versione del server (cambia con «Aggiorna»)
	w.Header().Set("ETag", `"`+hex.EncodeToString(h[:8])+`"`)
	w.Write(dati)
	return nil, errGiaRisposto
}
