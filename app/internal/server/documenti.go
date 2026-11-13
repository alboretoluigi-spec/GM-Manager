package server

// Documenti (richiesta dell'utente, 2026-11-07).
// Master e giocatori caricano PDF, documenti Word e immagini. Ogni giocatore vede solo i propri documenti; il Master vede
// le cartelle di tutti e può rinominare, sostituire, eliminare e condividere (consegnare una copia ai giocatori).
// Un giocatore può consegnare un suo documento a un altro giocatore: il destinatario riceve una copia con la notifica
// «[personaggio] ti ha consegnato un documento»; il documento del mittente passa nella sua sottocartella «Backup».
// I file sono cifrati nella cartella dati, con un nome scelto dal server (come il background privato).

import (
	"bytes"
	"database/sql"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const maxDocumento = 25 << 20

// Formati ammessi: tipo → Content-Type con cui il file viene mostrato o scaricato.
var mimeDocumento = map[string]string{
	"pdf": "application/pdf", "docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "doc": "application/msword",
	"png": "image/png", "jpg": "image/jpeg", "gif": "image/gif", "webp": "image/webp",
}

// tipoDocumento riconosce il formato dal contenuto, non dal nome.
func tipoDocumento(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("%PDF-")):
		return "pdf"
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case bytes.HasPrefix(b, []byte("\xff\xd8\xff")):
		return "jpg"
	case bytes.HasPrefix(b, []byte("GIF87a")), bytes.HasPrefix(b, []byte("GIF89a")):
		return "gif"
	case len(b) > 12 && bytes.HasPrefix(b, []byte("RIFF")) && string(b[8:12]) == "WEBP":
		return "webp"
	case bytes.HasPrefix(b, []byte("\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1")):
		return "doc" // Word 97-2003
	}
	if tipoBackground(b) == "docx" {
		return "docx"
	}
	return ""
}

func (s *Server) rotteDocumenti(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/documenti", api(s.apiDocumenti))
	mux.Handle("PUT /api/v1/documenti", api(s.apiCaricaDocumento))
	mux.Handle("GET /api/v1/documenti/destinatari", api(s.apiDestinatariDocumento))
	mux.Handle("GET /api/v1/documenti/{id}/file", http.HandlerFunc(s.scaricaDocumento))
	mux.Handle("PUT /api/v1/documenti/{id}/file", api(s.apiSostituisciDocumento))
	mux.Handle("PATCH /api/v1/documenti/{id}", api(s.apiRinominaDocumento))
	mux.Handle("DELETE /api/v1/documenti/{id}", api(s.apiEliminaDocumento))
	mux.Handle("POST /api/v1/documenti/{id}/consegna", api(s.apiConsegnaDocumento))
	mux.Handle("POST /api/v1/documenti/{id}/visto", api(s.apiDocumentoVisto))
	mux.Handle("POST /api/v1/documenti/{id}/mostra", api(s.apiMostraDocumento)) // a tutto schermo ad altri (documenti_mostra.go)
}

type vistaDocumento struct {
	ID            string `json:"id"`
	Proprietario  string `json:"proprietario"`
	NomeUtente    string `json:"nomeUtente"`
	Cartella      string `json:"cartella"`
	Nome          string `json:"nome"`
	Tipo          string `json:"tipo"`
	Dimensione    int64  `json:"dimensione"`
	DaPersonaggio string `json:"daPersonaggio,omitempty"`
	DaUtente      string `json:"daUtente,omitempty"`
	AUtente       string `json:"aUtente,omitempty"`
	Nuovo         bool   `json:"nuovo"`
	Creato        int64  `json:"creato"`
	Modificato    int64  `json:"modificato"`
}

const selDocumento = `SELECT d.id, d.proprietario_id, COALESCE(u.nome,''), d.cartella, d.nome, d.tipo, d.dimensione, d.da_personaggio,
	COALESCE(du.nome,''), COALESCE(au.nome,''), d.visto, d.creato_il, d.modificato_il
	FROM documenti d LEFT JOIN utenti u ON u.id = d.proprietario_id LEFT JOIN utenti du ON du.id = d.da_utente_id LEFT JOIN utenti au ON au.id = d.a_utente_id`

func scanDocumento(sc interface{ Scan(...any) error }) (*vistaDocumento, error) {
	d := &vistaDocumento{}
	var visto int
	if err := sc.Scan(&d.ID, &d.Proprietario, &d.NomeUtente, &d.Cartella, &d.Nome, &d.Tipo, &d.Dimensione, &d.DaPersonaggio, &d.DaUtente, &d.AUtente,
		&visto, &d.Creato, &d.Modificato); err != nil {
		return nil, err
	}
	d.Nuovo = visto == 0
	return d, nil
}

// documento: il documento se l'utente può gestirlo (proprietario o Master).
func (s *Server) documento(u *Utente, id string) (*vistaDocumento, string, error) {
	var file string
	if s.DB.QueryRow(`SELECT file FROM documenti WHERE id = ?`, id).Scan(&file) != nil {
		return nil, "", errNonTrovato
	}
	d, err := scanDocumento(s.DB.QueryRow(selDocumento+` WHERE d.id = ?`, id))
	if err != nil || !u.Master() && d.Proprietario != u.ID {
		return nil, "", errNonTrovato
	}
	return d, file, nil
}

func (s *Server) apiDocumenti(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	q := selDocumento + ` WHERE d.proprietario_id = ? ORDER BY d.cartella, d.creato_il DESC`
	args := []any{u.ID}
	if u.Master() {
		q = selDocumento + ` ORDER BY u.ruolo = 'master' DESC, u.nome COLLATE NOCASE, d.cartella, d.creato_il DESC`
		args = nil
	}
	righe, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer righe.Close()
	out := []*vistaDocumento{}
	for righe.Next() {
		if d, err := scanDocumento(righe); err == nil {
			out = append(out, d)
		}
	}
	risp := map[string]any{"lista": out}
	if u.Master() { // anche le cartelle vuote dei giocatori approvati
		cartelle := []map[string]string{}
		ru, err := s.DB.Query(`SELECT id, nome FROM utenti WHERE stato = 'approvato' AND ruolo <> 'master' ORDER BY nome COLLATE NOCASE`)
		if err == nil {
			for ru.Next() {
				var id, nome string
				ru.Scan(&id, &nome)
				cartelle = append(cartelle, map[string]string{"id": id, "nome": nome})
			}
			ru.Close()
		}
		risp["giocatori"] = cartelle
	}
	return risp, nil
}

// leggiCaricamento: corpo del file con il limite di dimensione e il formato riconosciuto.
func leggiCaricamento(r *http.Request) ([]byte, string, string, error) {
	nome, _ := url.QueryUnescape(r.Header.Get("X-Nome-File"))
	dati, err := io.ReadAll(io.LimitReader(r.Body, maxDocumento+1))
	if err != nil {
		return nil, "", "", errore(http.StatusBadRequest, "Caricamento interrotto.")
	}
	if len(dati) > maxDocumento {
		return nil, "", "", errore(http.StatusRequestEntityTooLarge, "File troppo grande: massimo 25 MB.")
	}
	tipo := tipoDocumento(dati)
	if tipo == "" {
		return nil, "", "", errore(http.StatusUnsupportedMediaType, "Formato non supportato: carica un PDF, un documento Word (.docx o .doc) o un'immagine (JPG, PNG, GIF, WebP).")
	}
	return dati, tipo, nomeOriginaleSicuro(nome, tipo), nil
}

func contestoDocumento(file string) string { return "documento:" + file }

// salvaFileDocumento cifra e scrive il file; restituisce il nome scelto dal server.
func (s *Server) salvaFileDocumento(dati []byte, tipo string) (string, error) {
	file := cripto.ID() + ".bin"
	return file, s.scriviFileCifrato(file, dati, contestoDocumento(file))
}

func (s *Server) apiCaricaDocumento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	prop := u.ID
	if per := r.URL.Query().Get("per"); per != "" && u.Master() { // il Master può mettere un file nella cartella di un giocatore
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM utenti WHERE id = ? AND stato = 'approvato'`, per).Scan(&n)
		if n == 0 {
			return nil, errNonTrovato
		}
		prop = per
	}
	dati, tipo, nome, err := leggiCaricamento(r)
	if err != nil {
		return nil, err
	}
	file, err := s.salvaFileDocumento(dati, tipo)
	if err != nil {
		return nil, err
	}
	id, ora := cripto.ID(), db.Ora()
	if _, err := s.DB.Exec(`INSERT INTO documenti(id, proprietario_id, nome, tipo, dimensione, file, creato_il, modificato_il) VALUES(?,?,?,?,?,?,?,?)`,
		id, prop, nome, tipo, len(dati), file, ora, ora); err != nil {
		s.eliminaFileDati(file)
		return nil, err
	}
	s.avvisaDocumenti(prop)
	d, _, err := s.documento(u, id)
	return d, err
}

// avvisaDocumenti: le pagine Documenti aperte si aggiornano (proprietario e Master).
func (s *Server) avvisaDocumenti(utenti ...string) {
	m := Msg{Tipo: "documenti.aggiornati", Dati: map[string]any{}}
	for _, x := range utenti {
		if x != "" {
			s.Hub.AUtente(x, m)
		}
	}
	s.Hub.AlMaster(m)
}

func (s *Server) scaricaDocumento(w http.ResponseWriter, r *http.Request) {
	u := utenteDa(r.Context())
	if !u.Approvato() {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	d, file, err := s.documento(u, r.PathValue("id"))
	if err != nil && s.visioni.puo(r.PathValue("id"), u.ID) { // documento che qualcuno gli sta mostrando
		var e2 error
		d, e2 = scanDocumento(s.DB.QueryRow(selDocumento+` WHERE d.id = ?`, r.PathValue("id")))
		if e2 == nil && s.DB.QueryRow(`SELECT file FROM documenti WHERE id = ?`, r.PathValue("id")).Scan(&file) == nil {
			err = nil
		}
	}
	if err != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	dati, err := s.leggiFileCifrato(file, contestoDocumento(file))
	if err != nil {
		http.Error(w, "File non leggibile.", http.StatusInternalServerError)
		return
	}
	if u.Master() && d.Proprietario != u.ID {
		s.registroDati(u.ID, "documento_letto", d.Proprietario)
	}
	nome := d.Nome
	if !strings.HasSuffix(strings.ToLower(nome), "."+d.Tipo) {
		nome += "." + d.Tipo
	}
	disp := "inline" // PDF e immagini si aprono nel browser; i documenti Word si scaricano
	if d.Tipo == "doc" || d.Tipo == "docx" || r.URL.Query().Get("scarica") == "1" {
		disp = "attachment"
	}
	w.Header().Set("Content-Type", mimeDocumento[d.Tipo])
	w.Header().Set("Content-Disposition", disp+`; filename="documento.`+d.Tipo+`"; filename*=UTF-8''`+url.PathEscape(nome))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if d.Tipo != "pdf" { // (il visore PDF del browser non funziona in un documento «sandbox»; il PDF non esegue script della pagina)
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Write(dati)
}

func (s *Server) apiSostituisciDocumento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	d, vecchio, err := s.documento(u, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	dati, tipo, _, err := leggiCaricamento(r)
	if err != nil {
		return nil, err
	}
	file, err := s.salvaFileDocumento(dati, tipo)
	if err != nil {
		return nil, err
	}
	if _, err := s.DB.Exec(`UPDATE documenti SET file = ?, tipo = ?, dimensione = ?, modificato_il = ? WHERE id = ?`, file, tipo, len(dati), db.Ora(), d.ID); err != nil {
		s.eliminaFileDati(file)
		return nil, err
	}
	s.eliminaFileDati(vecchio)
	s.avvisaDocumenti(d.Proprietario)
	d, _, err = s.documento(u, d.ID)
	return d, err
}

func (s *Server) apiRinominaDocumento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	d, _, err := s.documento(u, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	var in struct{ Nome string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	nome, err := rigaSingola(in.Nome, "Nome", 1, 100)
	if err != nil {
		return nil, err
	}
	nome = nomeOriginaleSicuro(nome, d.Tipo)
	s.DB.Exec(`UPDATE documenti SET nome = ?, modificato_il = ? WHERE id = ?`, nome, db.Ora(), d.ID)
	s.avvisaDocumenti(d.Proprietario)
	d, _, err = s.documento(u, d.ID)
	return d, err
}

func (s *Server) apiEliminaDocumento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	d, file, err := s.documento(u, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if _, err := s.DB.Exec(`DELETE FROM documenti WHERE id = ?`, d.ID); err != nil {
		return nil, err
	}
	s.eliminaFileDati(file)
	s.avvisaDocumenti(d.Proprietario)
	return nil, nil
}

func (s *Server) apiDocumentoVisto(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	s.DB.Exec(`UPDATE documenti SET visto = 1 WHERE id = ? AND proprietario_id = ?`, r.PathValue("id"), u.ID)
	return nil, nil
}

// destinatariDi: i giocatori approvati che condividono almeno un gruppo con u (per il Master: tutti i giocatori).
func (s *Server) destinatariDi(u *Utente) []map[string]string {
	q := `SELECT DISTINCT x.id, x.nome FROM utenti x JOIN iscrizioni ix ON ix.utente_id = x.id JOIN iscrizioni iu ON iu.gruppo_id = ix.gruppo_id
		WHERE iu.utente_id = ? AND x.id <> ? AND x.stato = 'approvato' AND x.ruolo <> 'master' ORDER BY x.nome COLLATE NOCASE`
	args := []any{u.ID, u.ID}
	if u.Master() {
		q = `SELECT id, nome FROM utenti WHERE stato = 'approvato' AND ruolo <> 'master' ORDER BY nome COLLATE NOCASE`
		args = nil
	}
	out := []map[string]string{}
	righe, err := s.DB.Query(q, args...)
	if err != nil {
		return out
	}
	defer righe.Close()
	for righe.Next() {
		var id, nome string
		righe.Scan(&id, &nome)
		out = append(out, map[string]string{"id": id, "nome": nome})
	}
	return out
}

func (s *Server) apiDestinatariDocumento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	pg := []map[string]string{}
	righe, err := s.DB.Query(`SELECT id, nome FROM personaggi WHERE proprietario_id = ? AND stato = 'attivo' ORDER BY nome COLLATE NOCASE`, u.ID)
	if err == nil {
		for righe.Next() {
			var id, nome string
			righe.Scan(&id, &nome)
			pg = append(pg, map[string]string{"id": id, "nome": nome})
		}
		righe.Close()
	}
	return map[string]any{"giocatori": s.destinatariDi(u), "personaggi": pg}, nil
}

// apiConsegnaDocumento: il giocatore consegna un suo documento a un altro giocatore (copia al destinatario, il suo passa
// nel Backup); il Master condivide un documento con uno o più giocatori (copia a ciascuno, il suo resta dov'è).
func (s *Server) apiConsegnaDocumento(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "approvato")
	if err != nil {
		return nil, err
	}
	d, file, err := s.documento(u, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	var in struct {
		A           []string
		Personaggio string
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	ammessi := map[string]bool{}
	for _, x := range s.destinatariDi(u) {
		ammessi[x["id"]] = true
	}
	var dest []string
	visti := map[string]bool{}
	for _, a := range in.A {
		if !ammessi[a] || visti[a] || a == d.Proprietario {
			return nil, errore(http.StatusBadRequest, "Destinatario non valido.")
		}
		visti[a] = true
		dest = append(dest, a)
	}
	if len(dest) == 0 || !u.Master() && len(dest) > 1 {
		return nil, errore(http.StatusBadRequest, "Scegli a chi consegnare il documento.")
	}
	if !u.Master() && d.Proprietario != u.ID {
		return nil, errNonTrovato
	}
	if !u.Master() && d.Cartella == "backup" {
		return nil, errore(http.StatusBadRequest, "Questo documento è già stato consegnato: nel Backup ne resta solo la copia.")
	}
	// Chi consegna: il personaggio scelto (del giocatore), altrimenti il nome del giocatore o «Il Master».
	mittente := u.Nome
	if u.Master() {
		mittente = "Il Master"
	}
	if in.Personaggio != "" {
		var nome string
		if s.DB.QueryRow(`SELECT nome FROM personaggi WHERE id = ? AND proprietario_id = ? AND stato = 'attivo'`, in.Personaggio, u.ID).Scan(&nome) != nil {
			return nil, errore(http.StatusBadRequest, "Personaggio non valido.")
		}
		mittente = nome
	} else if !u.Master() {
		s.DB.QueryRow(`SELECT nome FROM personaggi WHERE proprietario_id = ? AND stato = 'attivo' ORDER BY modificato_il DESC LIMIT 1`, u.ID).Scan(&mittente)
	}
	dati, err := s.leggiFileCifrato(file, contestoDocumento(file))
	if err != nil {
		return nil, err
	}
	var nuovi []string
	var ids []string
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		ora := db.Ora()
		for _, a := range dest {
			f, err := s.salvaFileDocumento(dati, d.Tipo)
			if err != nil {
				return err
			}
			nuovi = append(nuovi, f)
			id := cripto.ID()
			ids = append(ids, id)
			if _, err := tx.Exec(`INSERT INTO documenti(id, proprietario_id, nome, tipo, dimensione, file, da_utente_id, da_personaggio, visto, creato_il, modificato_il)
				VALUES(?,?,?,?,?,?,?,?,0,?,?)`, id, a, d.Nome, d.Tipo, d.Dimensione, f, u.ID, mittente, ora, ora); err != nil {
				return err
			}
		}
		if !u.Master() { // il documento del mittente diventa la copia nel suo Backup
			_, err := tx.Exec(`UPDATE documenti SET cartella = 'backup', a_utente_id = ?, modificato_il = ? WHERE id = ?`, dest[0], ora, d.ID)
			return err
		}
		return nil
	})
	if err != nil {
		for _, f := range nuovi {
			s.eliminaFileDati(f)
		}
		return nil, err
	}
	testo := mittente + " ti ha consegnato un documento"
	for i, a := range dest {
		s.Hub.AUtente(a, Msg{Tipo: "documento.consegnato", Dati: map[string]any{"id": ids[i], "nome": d.Nome, "tipo": d.Tipo, "da": mittente, "testo": testo}})
	}
	s.avvisaDocumenti(append(dest, d.Proprietario)...)
	return map[string]any{"consegnati": len(dest), "da": mittente}, nil
}
