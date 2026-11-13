package server

// API del gioco da remoto (solo Master): stato, link personali con QR, revoca.
// Il ponte vero e proprio è in ponte_mqtt.go.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"net/http"
	"strconv"

	"rsc.io/qr"

	"gasterbless/internal/rete"
)

func (s *Server) rotteRemoto(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/sessioni/{id}/remoto", api(s.apiRemoto))
	mux.Handle("POST /api/v1/sessioni/{id}/remoto/giocatori", api(s.apiLinkRemoto))
	mux.Handle("DELETE /api/v1/sessioni/{id}/remoto/giocatori/{utente}", api(s.apiRevocaRemoto))
}

type vistaGiocatoreRemoto struct {
	Utente string `json:"utente"`
	Nome   string `json:"nome"`
	Link   bool   `json:"link"`             // ha un link attivo
	Creato int64  `json:"creato,omitempty"` // quando è stato creato
}

func (s *Server) apiRemoto(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var mq int
	var stato string
	if s.DB.QueryRow(`SELECT mqtt_remoto, stato FROM sessioni_gioco WHERE id = ?`, id).Scan(&mq, &stato) != nil {
		return nil, errNonTrovato
	}
	attivi := map[string]int64{}
	c := s.remoto.canale(id)
	if c != nil {
		attivi = c.giocatori()
	}
	out := []vistaGiocatoreRemoto{}
	rows, err := s.DB.Query(`SELECT u.id, u.nome FROM partecipanti p JOIN utenti u ON u.id = p.utente_id
		WHERE p.sessione_id = ? AND u.stato = 'approvato' ORDER BY u.nome`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v vistaGiocatoreRemoto
		rows.Scan(&v.Utente, &v.Nome)
		v.Creato, v.Link = attivi[v.Utente], attivi[v.Utente] != 0
		out = append(out, v)
	}
	return map[string]any{
		"attivo":    mq == 1 && stato != "conclusa",
		"collegato": c != nil && c.collegati() > 0,
		"server":    s.Cfg.BrokerMQTT,
		"giocatori": out,
	}, nil
}

// apiLinkRemoto crea il link personale di un giocatore (il precedente smette di funzionare).
// Il link e il QR vengono mostrati una volta sola: il server non li conserva in chiaro altrove.
func (s *Server) apiLinkRemoto(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var in struct{ Utente string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	c := s.remoto.canale(id)
	if c == nil {
		return nil, errore(http.StatusConflict, "Prima attiva il gioco da remoto per questa sessione.")
	}
	u := s.utentePerID(in.Utente)
	if u == nil || u.Master() {
		return nil, errNonTrovato
	}
	if _, ok := s.partecipa(u, id); !ok {
		return nil, errore(http.StatusBadRequest, "Questo giocatore non partecipa alla sessione.")
	}
	alias, chiave, err := c.nuovoGiocatore(u.ID)
	if err != nil {
		return nil, err
	}
	var titolo string
	s.DB.QueryRow(`SELECT titolo FROM sessioni_gioco WHERE id = ?`, id).Scan(&titolo)
	conf := map[string]any{
		"b": s.Cfg.BrokerMQTT, "t": c.d.Canale, "u": alias,
		"k": base64.RawURLEncoding.EncodeToString(chiave), "n": u.Nome, "s": titolo,
	}
	j, _ := json.Marshal(conf)
	link := s.indirizzoPerLink(r) + "/remoto/#" + base64.RawURLEncoding.EncodeToString(j)
	out := map[string]any{"link": link, "configurazione": conf}
	if q, err := qr.Encode(link, qr.L); err == nil {
		var b bytes.Buffer
		png.Encode(&b, immagineQR(q, 4))
		out["qr"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
	}
	s.evento("remoto", u.ID, "link remoto creato per la sessione "+id)
	return out, nil
}

func (s *Server) apiRevocaRemoto(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	c := s.remoto.canale(r.PathValue("id"))
	if c == nil || !c.revoca(r.PathValue("utente")) {
		return nil, errNonTrovato
	}
	s.evento("remoto", r.PathValue("utente"), "link remoto revocato")
	return nil, nil
}

// indirizzoPerLink: l'indirizzo del PC nella rete di casa (il giocatore apre il link la prima volta
// in casa, così la pagina resta sul telefono), altrimenti quello usato dal Master.
func (s *Server) indirizzoPerLink(r *http.Request) string {
	if l := rete.IndirizziLocali(); len(l) > 0 && rete.Locale(r) {
		return "https://" + l[0].String() + ":" + strconv.Itoa(s.Cfg.PortaHTTPS)
	}
	return "https://" + r.Host
}
