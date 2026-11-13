package server

// Archivio degli oggetti magici del Master (richiesta dell'utente del 2026-11-11: «nell'app non trovo dove il master può
// inserire o creare oggetti magici»). Sezione «✨ Oggetti magici» (web/sezioni/oggetti_magici.js, pagina
// web/scheda/archivio_oggetti.html con lo stesso editor della scheda). Gli oggetti hanno il formato della scheda
// (web/regole/oggetti.js, OggettiMagici): qui si controllano solo la forma, l'id, il nome, la versione e l'immagine;
// tutto il resto lo ricontrolla la scheda quando lo carica (sanificaOggetto). «Consegna» mette un oggetto nella scheda
// di uno o più personaggi con le stesse regole dell'importazione da file: stesso id → vale la versione più alta e si
// conservano equipaggiato, sintonia, attivazione e cariche. Tutto solo per il Master.

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const (
	maxOggettoMagico = 480 << 10 // immagine (200.000 caratteri) e testi
	maxImgOggetto    = 200000    // come MAX_IMG di web/regole/oggetti.js
	maxVociMagiche   = 100       // come MAX_VOCI
)

var (
	idOggettoValido = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,40}$`)
	imgOggetto      = regexp.MustCompile(`^data:image/(jpeg|png|webp);base64,([A-Za-z0-9+/=]+)$`)
	campiOggetto    = map[string]bool{"id": true, "v": true, "nome": true, "tipo": true, "sottotipo": true, "rar": true, "sint": true, "img": true,
		"peso": true, "valore": true, "slot": true, "mani": true, "aspetto": true, "mater": true, "storia": true, "testo": true, "princ": true,
		"second": true, "speciali": true, "uso": true, "mod": true, "arma": true, "armatura": true, "scudo": true, "acc": true, "req": true,
		"condiz": true, "inter": true}
)

// oggettoMagico: i campi che servono al server (il resto resta com'è nel JSON).
type oggettoMagico struct {
	ID   string `json:"id"`
	V    int    `json:"v"`
	Nome string `json:"nome"`
	Sint bool   `json:"sint"`
	Img  string `json:"img"`
	Uso  struct {
		Cariche int `json:"cariche"`
	} `json:"uso"`
}

// validaOggettoMagico controlla un oggetto e lo restituisce ripulito (solo i campi del formato, JSON compatto).
func validaOggettoMagico(raw json.RawMessage) (oggettoMagico, []byte, error) {
	var o oggettoMagico
	if len(raw) == 0 || len(raw) > maxOggettoMagico {
		return o, nil, errore(http.StatusRequestEntityTooLarge, "Oggetto troppo grande (l'immagine deve stare sotto i 200 kB).")
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || json.Unmarshal(raw, &o) != nil {
		return o, nil, errDatiNonValidi
	}
	for k := range m {
		if !campiOggetto[k] {
			delete(m, k)
		}
	}
	o.Nome = strings.TrimSpace(o.Nome)
	if !idOggettoValido.MatchString(o.ID) || o.Nome == "" || utf8.RuneCountInString(o.Nome) > 80 || !utf8.ValidString(o.Nome) {
		return o, nil, errore(http.StatusBadRequest, "Oggetto non valido: servono un id e un nome (massimo 80 caratteri).")
	}
	if o.V < 1 {
		o.V = 1
	}
	if o.V > 9999 {
		o.V = 9999
	}
	if o.Img != "" && (len(o.Img) > maxImgOggetto || !imgOggetto.MatchString(o.Img)) {
		return o, nil, errore(http.StatusBadRequest, "Immagine dell'oggetto non valida.")
	}
	nb, _ := json.Marshal(o.Nome)
	vb, _ := json.Marshal(o.V)
	m["nome"], m["v"] = nb, vb
	b, err := json.Marshal(m)
	return o, b, err
}

func (s *Server) rotteOggettiMagici(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/oggetti-magici", api(s.apiOggettiMagici))
	mux.Handle("GET /api/v1/oggetti-magici/{id}", api(s.apiOggettoMagico))
	mux.Handle("GET /api/v1/oggetti-magici/{id}/immagine", http.HandlerFunc(s.immagineOggettoMagico))
	mux.Handle("PUT /api/v1/oggetti-magici/{id}", api(s.apiSalvaOggettoMagico))
	mux.Handle("DELETE /api/v1/oggetti-magici/{id}", api(s.apiEliminaOggettoMagico))
	mux.Handle("POST /api/v1/oggetti-magici/{id}/consegna", api(s.apiConsegnaOggettoMagico))
}

type possessore struct {
	ID   string `json:"id"`
	Nome string `json:"nome"`
	V    int    `json:"v"`
}

// possessoriOggetti: per ogni id di oggetto, i personaggi (schede di Gaster Bless) che lo hanno e con quale versione.
func (s *Server) possessoriOggetti() map[string][]possessore {
	out := map[string][]possessore{}
	rows, err := s.DB.Query(`SELECT p.id, p.nome, v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
		WHERE p.scheda = '' ORDER BY p.nome`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, nome, dati string
		if rows.Scan(&id, &nome, &dati) != nil || !strings.Contains(dati, `"mag"`) {
			continue
		}
		var sc struct {
			Mag []struct {
				O struct {
					ID string `json:"id"`
					V  int    `json:"v"`
				} `json:"o"`
			} `json:"mag"`
		}
		if json.Unmarshal([]byte(dati), &sc) != nil {
			continue
		}
		for _, v := range sc.Mag {
			if v.O.ID != "" {
				out[v.O.ID] = append(out[v.O.ID], possessore{ID: id, Nome: nome, V: v.O.V})
			}
		}
	}
	return out
}

// apiOggettiMagici: l'archivio (senza le immagini, che si scaricano a parte) e i personaggi a cui consegnare.
func (s *Server) apiOggettiMagici(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	pos := s.possessoriOggetti()
	rows, err := s.DB.Query(`SELECT id, dati, manuale_nome, pagina, aggiornato_il FROM oggetti_magici`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	oggetti := []map[string]any{}
	for rows.Next() {
		var id, dati, man string
		var pag int
		var agg int64
		if rows.Scan(&id, &dati, &man, &pag, &agg) != nil {
			continue
		}
		var m map[string]json.RawMessage
		if json.Unmarshal([]byte(dati), &m) != nil {
			continue
		}
		img := len(m["img"]) > 2
		delete(m, "img")
		p := pos[id]
		if p == nil {
			p = []possessore{}
		}
		oggetti = append(oggetti, map[string]any{"id": id, "oggetto": m, "immagine": img, "manualeNome": man, "pagina": pag, "aggiornato": agg, "possessori": p})
	}
	pgs := []map[string]any{}
	prow, err := s.DB.Query(`SELECT p.id, p.nome, p.stato, COALESCE(u.nome,'') FROM personaggi p LEFT JOIN utenti u ON u.id = p.proprietario_id
		WHERE p.scheda = '' ORDER BY p.stato <> 'attivo', u.nome, p.nome`)
	if err == nil {
		defer prow.Close()
		for prow.Next() {
			var id, nome, stato, gioc string
			if prow.Scan(&id, &nome, &stato, &gioc) == nil {
				pgs = append(pgs, map[string]any{"id": id, "nome": nome, "stato": stato, "giocatore": gioc})
			}
		}
	}
	return map[string]any{"oggetti": oggetti, "personaggi": pgs}, nil
}

func (s *Server) apiOggettoMagico(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var dati, man string
	var pag int
	if s.DB.QueryRow(`SELECT dati, manuale_nome, pagina FROM oggetti_magici WHERE id = ?`, r.PathValue("id")).Scan(&dati, &man, &pag) != nil {
		return nil, errNonTrovato
	}
	return map[string]any{"oggetto": json.RawMessage(dati), "manualeNome": man, "pagina": pag, "possessori": s.possessoriOggetti()[r.PathValue("id")]}, nil
}

// immagineOggettoMagico: l'immagine dell'oggetto come file (per le miniature dell'archivio).
func (s *Server) immagineOggettoMagico(w http.ResponseWriter, r *http.Request) {
	if _, err := richiedi(r, "master"); err != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	var dati string
	var o oggettoMagico
	if s.DB.QueryRow(`SELECT dati FROM oggetti_magici WHERE id = ?`, r.PathValue("id")).Scan(&dati) != nil || json.Unmarshal([]byte(dati), &o) != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	m := imgOggetto.FindStringSubmatch(o.Img)
	if m == nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	b, err := base64.StdEncoding.DecodeString(m[2])
	if err != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/"+m[1])
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(b)
}

func (s *Server) apiSalvaOggettoMagico(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Oggetto json.RawMessage `json:"oggetto"`
	}
	if err := leggiJSONMax(r, &in, maxOggettoMagico+4096); err != nil {
		return nil, err
	}
	o, b, err := validaOggettoMagico(in.Oggetto)
	if err != nil {
		return nil, err
	}
	if o.ID != r.PathValue("id") {
		return nil, errore(http.StatusBadRequest, "L'id dell'oggetto non corrisponde.")
	}
	var prima string
	if s.DB.QueryRow(`SELECT dati FROM oggetti_magici WHERE id = ?`, o.ID).Scan(&prima) == nil {
		var p oggettoMagico
		json.Unmarshal([]byte(prima), &p)
		if o.V < p.V {
			return nil, errore(http.StatusConflict, "Nell'archivio c'è già una versione più recente di «"+o.Nome+"» (versione "+strconv.Itoa(p.V)+").")
		}
	}
	ora := db.Ora()
	if _, err := s.DB.Exec(`INSERT INTO oggetti_magici(id, nome, chiave, dati, creato_il, aggiornato_il) VALUES(?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET nome = excluded.nome, chiave = excluded.chiave, dati = excluded.dati, aggiornato_il = excluded.aggiornato_il`,
		o.ID, o.Nome, chiaveNome(o.Nome), string(b), ora, ora); err != nil {
		return nil, err
	}
	p := s.possessoriOggetti()[o.ID]
	if p == nil {
		p = []possessore{}
	}
	return map[string]any{"id": o.ID, "v": o.V, "possessori": p}, nil
}

// apiEliminaOggettoMagico toglie l'oggetto dall'archivio; nelle schede che lo hanno resta.
func (s *Server) apiEliminaOggettoMagico(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`DELETE FROM oggetti_magici WHERE id = ?`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errNonTrovato
	}
	return nil, nil
}

// unisciVoceMagica: come OggettiMagici.unisci (web/regole/oggetti.js). Esiti: aggiunto, aggiornato, uguale, vecchio, pieno.
func unisciVoceMagica(mag json.RawMessage, o oggettoMagico, dati []byte) (json.RawMessage, string, error) {
	var voci []map[string]json.RawMessage
	if t := bytes.TrimSpace(mag); len(t) > 0 && !bytes.Equal(t, []byte("null")) {
		if json.Unmarshal(t, &voci) != nil {
			return mag, "", errore(http.StatusConflict, "L'inventario magico della scheda non è leggibile.")
		}
	}
	car := func(v map[string]json.RawMessage) (int, bool) {
		var n *int
		if json.Unmarshal(v["car"], &n) != nil || n == nil {
			return 0, false
		}
		return *n, true
	}
	esito := ""
	trovato := false
	for i, v := range voci {
		var vecchio oggettoMagico
		var stato struct {
			Sint bool `json:"sint"`
		}
		json.Unmarshal(v["o"], &vecchio)
		if vecchio.ID != o.ID {
			continue
		}
		trovato = true
		switch {
		case o.V == vecchio.V:
			esito = "uguale"
		case o.V < vecchio.V:
			esito = "vecchio"
		default:
			json.Unmarshal(v["sint"], &stato.Sint)
			c := o.Uso.Cariche
			if n, ok := car(v); ok && n < c {
				c = n
			}
			sint, _ := json.Marshal(o.Sint && stato.Sint)
			cb, _ := json.Marshal(c)
			v["o"], v["sint"], v["car"] = dati, sint, cb
			voci[i] = v
			esito = "aggiornato"
		}
		break
	}
	if !trovato {
		if len(voci) >= maxVociMagiche {
			return mag, "pieno", nil
		}
		cb, _ := json.Marshal(o.Uso.Cariche)
		voci = append(voci, map[string]json.RawMessage{"o": dati, "eq": json.RawMessage("false"), "sint": json.RawMessage("false"), "on": json.RawMessage("false"), "car": cb})
		esito = "aggiunto"
	}
	if esito != "aggiunto" && esito != "aggiornato" {
		return mag, esito, nil
	}
	b, err := json.Marshal(voci)
	return compatta(b), esito, err
}

// apiConsegnaOggettoMagico mette l'oggetto dell'archivio nelle schede scelte ({"personaggi": [id, ...]}).
func (s *Server) apiConsegnaOggettoMagico(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		Personaggi []string `json:"personaggi"`
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if len(in.Personaggi) == 0 || len(in.Personaggi) > 60 {
		return nil, errore(http.StatusBadRequest, "Scegli a chi consegnarlo.")
	}
	var dati string
	if s.DB.QueryRow(`SELECT dati FROM oggetti_magici WHERE id = ?`, r.PathValue("id")).Scan(&dati) != nil {
		return nil, errNonTrovato
	}
	o, b, err := validaOggettoMagico(json.RawMessage(dati))
	if err != nil {
		return nil, err
	}
	type risultato struct {
		Personaggio string `json:"personaggio"`
		Nome        string `json:"nome"`
		Esito       string `json:"esito"`
	}
	type avviso struct {
		pg, prop string
		ver      int
		campi    []string
	}
	ris := []risultato{}
	var avvisi []avviso
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		visti := map[string]bool{}
		for _, pg := range in.Personaggi {
			if visti[pg] {
				continue
			}
			visti[pg] = true
			var nome, prop, scheda string
			if tx.QueryRow(`SELECT nome, COALESCE(proprietario_id,''), scheda FROM personaggi WHERE id = ?`, pg).Scan(&nome, &prop, &scheda) != nil {
				ris = append(ris, risultato{pg, "", "non_trovato"})
				continue
			}
			if scheda != "" {
				ris = append(ris, risultato{pg, nome, "scheda_alternativa"})
				continue
			}
			esito := ""
			ver, campi, err := s.modificaSchedaNota(tx, pg, u, "oggetto magico: "+o.Nome, func(p piatto) (bool, error) {
				nuovo, e, err := unisciVoceMagica(p["mag"], o, b)
				esito = e
				if err != nil || (e != "aggiunto" && e != "aggiornato") {
					return false, err
				}
				p["mag"] = nuovo
				return true, nil
			})
			if err != nil {
				return err
			}
			ris = append(ris, risultato{pg, nome, esito})
			if ver > 0 {
				avvisi = append(avvisi, avviso{pg, prop, ver, campi})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, a := range avvisi {
		s.avvisaSchedaDa(a.pg, a.prop, a.ver, u, a.campi, true)
	}
	return map[string]any{"risultati": ris}, nil
}

// nuovoOggettoDaManuale: un oggetto minimo per l'archivio (la scheda completa i campi mancanti con i valori predefiniti).
func nuovoOggettoDaManuale(nome string, x valoriOggetto) []byte {
	m := map[string]any{"id": "ogm-" + cripto.ID(), "v": 1, "nome": nome, "tipo": x.Tipo, "sottotipo": x.Sottotipo, "rar": x.Rar, "sint": x.Sint, "slot": slotDiTipo[x.Tipo]}
	conBonusLetti(m, x)
	b, _ := json.Marshal(m)
	return b
}
