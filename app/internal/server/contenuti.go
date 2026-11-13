package server

// Pacchetti di contenuti (fase 3c, passo C1, docs/FORMATO_CONTENUTI.md). L'app non include nessun contenuto di gioco:
// il Master importa file nel formato aperto «gasterbless-contenuti» (classi, razze, background, incantesimi, armi,
// armature, talenti, stili), li attiva o li disattiva, li esporta di nuovo e li elimina. I pacchetti attivi, nell'ordine
// di importazione, si aggiungono ai dati che la scheda legge: /regole/dati.js e /regole/talenti.js sono i file
// dell'app (vuoti) più una coda generata qui. La coda va solo agli utenti collegati. Migrazione 041.

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gasterbless/internal/cripto"
)

const maxFileContenuti = 20 << 20 // 20 MB

// cacheContenuti tiene i due file generati finché un pacchetto non cambia.
type cacheContenuti struct {
	mu     sync.Mutex
	pronti bool
	file   map[string]*fileGenerato // "regole/dati.js", "regole/talenti.js"
	sovr   []string                 // voci sovrascritte da un pacchetto successivo
	liste  map[string]string        // liste di incantesimi dichiarate dai pacchetti attivi (per l'inserimento a mano)
}

type fileGenerato struct {
	dati, gz []byte
	etag     string
}

func (c *cacheContenuti) svuota() {
	c.mu.Lock()
	c.pronti, c.file, c.sovr = false, nil, nil
	c.mu.Unlock()
}

type vistaPacchetto struct {
	ID          string         `json:"id"`
	Nome        string         `json:"nome"`
	Sistema     string         `json:"sistema"`
	Autore      string         `json:"autore"`
	Versione    string         `json:"versione"`
	Licenza     string         `json:"licenza"`
	Descrizione string         `json:"descrizione"`
	Conteggi    map[string]int `json:"conteggi"`
	Attivo      bool           `json:"attivo"`
	Personale   bool           `json:"personale"`
	CreatoIl    int64          `json:"creatoIl"`
}

func (s *Server) rotteContenuti(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/contenuti", api(s.apiContenuti))
	mux.Handle("POST /api/v1/contenuti", api(s.apiImportaContenuti))
	mux.Handle("POST /api/v1/contenuti/verifica", api(s.apiVerificaContenuti))
	mux.Handle("POST /api/v1/contenuti/personale", api(s.apiNuovoPersonale))
	mux.Handle("PUT /api/v1/contenuti/{id}", api(s.apiSalvaContenuti))
	mux.Handle("PATCH /api/v1/contenuti/{id}", api(s.apiModificaContenuti))
	mux.Handle("DELETE /api/v1/contenuti/{id}", api(s.apiEliminaContenuti))
	mux.Handle("GET /api/v1/contenuti/{id}/file", http.HandlerFunc(s.apiFileContenuti))
}

func (s *Server) pacchetti(soloAttivi bool) ([]vistaPacchetto, []string, error) {
	q := `SELECT id, nome, sistema, autore, versione, licenza, descrizione, conteggi, attivo, personale, creato_il, file FROM contenuti_pacchetti`
	if soloAttivi {
		q += ` WHERE attivo = 1`
	}
	rows, err := s.DB.Query(q + ` ORDER BY creato_il, rowid`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out, file := []vistaPacchetto{}, []string{}
	for rows.Next() {
		var p vistaPacchetto
		var cont, f string
		var att, pers int
		if err := rows.Scan(&p.ID, &p.Nome, &p.Sistema, &p.Autore, &p.Versione, &p.Licenza, &p.Descrizione, &cont, &att, &pers, &p.CreatoIl, &f); err != nil {
			return nil, nil, err
		}
		p.Attivo, p.Personale = att == 1, pers == 1
		json.Unmarshal([]byte(cont), &p.Conteggi)
		out = append(out, p)
		file = append(file, f)
	}
	return out, file, rows.Err()
}

func (s *Server) apiContenuti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	p, _, err := s.pacchetti(false)
	if err != nil {
		return nil, err
	}
	sovr, liste := []string{}, map[string]string{}
	for k, v := range listeIncantesimiBase {
		liste[k] = v
	}
	if _, err := s.contenutiGenerati(); err == nil {
		s.pacchettiCache.mu.Lock()
		sovr = append(sovr, s.pacchettiCache.sovr...)
		for k, v := range s.pacchettiCache.liste {
			liste[k] = v
		}
		s.pacchettiCache.mu.Unlock()
	}
	return map[string]any{"pacchetti": p, "sovrascritti": sovr, "liste": liste, "formato": formatoContenuti, "versioneFormato": versioneFormatoContenuti,
		"sistemi": []string{sistemaQuintaEdizione}}, nil
}

func (s *Server) leggiFileContenuti(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.ContentLength > maxFileContenuti {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande: un pacchetto di contenuti può essere al massimo di 20 MB.")
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxFileContenuti))
	if err != nil {
		return nil, errore(http.StatusRequestEntityTooLarge, "File troppo grande o caricamento interrotto: massimo 20 MB.")
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")) // file salvati dal Blocco note di Windows
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, errore(http.StatusBadRequest, "Il file è vuoto.")
	}
	return b, nil
}

func riepilogoLetto(p *pacchettoLetto) map[string]any {
	return map[string]any{"nome": p.Nome, "sistema": p.Sistema, "autore": p.Autore, "versione": p.Versione, "licenza": p.Licenza,
		"conteggi": p.Conteggi, "avvisi": p.Avvisi}
}

// apiVerificaContenuti controlla un file senza salvarlo (anteprima prima di importare).
func (s *Server) apiVerificaContenuti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	b, err := s.leggiFileContenuti(w, r)
	if err != nil {
		return nil, err
	}
	p, err := leggiPacchetto(b)
	if err != nil {
		return nil, &ErrAPI{Codice: http.StatusBadRequest, Msg: err.Error(), Tipo: "formato"}
	}
	out := riepilogoLetto(p)
	var id, nome string
	if s.DB.QueryRow(`SELECT id, nome FROM contenuti_pacchetti WHERE impronta = ?`, improntaContenuti(b)).Scan(&id, &nome) == nil {
		out["doppione"] = nome
	}
	return out, nil
}

func improntaContenuti(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (s *Server) apiImportaContenuti(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	b, err := s.leggiFileContenuti(w, r)
	if err != nil {
		return nil, err
	}
	p, err := leggiPacchetto(b)
	if err != nil {
		return nil, &ErrAPI{Codice: http.StatusBadRequest, Msg: err.Error(), Tipo: "formato"}
	}
	hash := improntaContenuti(b)
	var nome string
	if s.DB.QueryRow(`SELECT nome FROM contenuti_pacchetti WHERE impronta = ?`, hash).Scan(&nome) == nil {
		return nil, &ErrAPI{Codice: http.StatusConflict, Msg: "Questo pacchetto è già stato importato: «" + nome + "».", Tipo: "doppione"}
	}
	cont, _ := json.Marshal(p.Conteggi)
	id := cripto.ID()
	_, err = s.DB.Exec(`INSERT INTO contenuti_pacchetti (id, nome, sistema, autore, versione, licenza, descrizione, file, impronta, conteggi, attivo, creato_il)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`, id, p.Nome, p.Sistema, p.Autore, p.Versione, p.Licenza, p.Descrizione, string(b), hash, string(cont), time.Now().UnixNano())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, &ErrAPI{Codice: http.StatusConflict, Msg: "Questo pacchetto è già stato importato.", Tipo: "doppione"}
		}
		return nil, err
	}
	s.pacchettiCache.svuota()
	s.evento("contenuti_importa", u.ID, p.Nome)
	s.Hub.AlMaster(Msg{Tipo: "contenuti", Dati: map[string]any{}})
	out := riepilogoLetto(p)
	out["id"] = id
	return out, nil
}

// apiNuovoPersonale crea un pacchetto personale vuoto, da riempire a mano nella pagina «Contenuti di gioco».
func (s *Server) apiNuovoPersonale(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var d struct {
		Nome string `json:"nome"`
	}
	json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&d)
	d.Nome = strings.TrimSpace(d.Nome)
	if d.Nome == "" || utf8.RuneCountInString(d.Nome) > 120 {
		return nil, errore(http.StatusBadRequest, "Scrivi il nome del pacchetto (al massimo 120 caratteri).")
	}
	file, _ := json.MarshalIndent(map[string]any{"formato": formatoContenuti, "versioneFormato": versioneFormatoContenuti,
		"sistema": sistemaQuintaEdizione, "nome": d.Nome, "autore": "", "versione": "1", "licenza": "", "descrizione": ""}, "", "  ")
	p, err := leggiPacchettoOpz(file, true)
	if err != nil {
		return nil, &ErrAPI{Codice: http.StatusBadRequest, Msg: err.Error(), Tipo: "formato"}
	}
	id, ora := cripto.ID(), time.Now()
	cont, _ := json.Marshal(p.Conteggi)
	if _, err := s.DB.Exec(`INSERT INTO contenuti_pacchetti (id, nome, sistema, autore, versione, licenza, descrizione, file, impronta, conteggi, attivo, personale, creato_il, aggiornato_il)
		VALUES (?, ?, ?, '', '1', '', '', ?, ?, ?, 1, 1, ?, ?)`, id, p.Nome, p.Sistema, string(file), improntaContenuti(file)+":"+id, string(cont), ora.UnixNano(), ora.Unix()); err != nil {
		return nil, err
	}
	s.pacchettiCache.svuota()
	s.evento("contenuti_personale", u.ID, p.Nome)
	s.Hub.AlMaster(Msg{Tipo: "contenuti", Dati: map[string]any{}})
	return map[string]any{"id": id}, nil
}

// apiSalvaContenuti sostituisce il file di un pacchetto con la versione modificata nell'app (inserimento a mano). Il
// file passa lo stesso controllo dell'importazione; solo un pacchetto personale può restare vuoto.
func (s *Server) apiSalvaContenuti(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	var pers int
	if s.DB.QueryRow(`SELECT personale FROM contenuti_pacchetti WHERE id = ?`, id).Scan(&pers) != nil {
		return nil, errNonTrovato
	}
	b, err := s.leggiFileContenuti(w, r)
	if err != nil {
		return nil, err
	}
	p, err := leggiPacchettoOpz(b, pers == 1)
	if err != nil {
		return nil, &ErrAPI{Codice: http.StatusBadRequest, Msg: err.Error(), Tipo: "formato"}
	}
	hash := improntaContenuti(b)
	if pers == 1 {
		hash += ":" + id
	} else {
		var altro string
		if s.DB.QueryRow(`SELECT nome FROM contenuti_pacchetti WHERE impronta = ? AND id != ?`, hash, id).Scan(&altro) == nil {
			return nil, &ErrAPI{Codice: http.StatusConflict, Msg: "Un altro pacchetto ha già esattamente questo contenuto: «" + altro + "».", Tipo: "doppione"}
		}
	}
	cont, _ := json.Marshal(p.Conteggi)
	if _, err := s.DB.Exec(`UPDATE contenuti_pacchetti SET nome = ?, autore = ?, versione = ?, licenza = ?, descrizione = ?, file = ?, impronta = ?,
		conteggi = ?, aggiornato_il = ? WHERE id = ?`, p.Nome, p.Autore, p.Versione, p.Licenza, p.Descrizione, string(b), hash, string(cont),
		time.Now().Unix(), id); err != nil {
		return nil, err
	}
	s.pacchettiCache.svuota()
	s.evento("contenuti_modifica", u.ID, p.Nome)
	s.Hub.AlMaster(Msg{Tipo: "contenuti", Dati: map[string]any{}})
	out := riepilogoLetto(p)
	out["id"] = id
	return out, nil
}

func (s *Server) apiModificaContenuti(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var d struct {
		Attivo *bool `json:"attivo"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&d); err != nil || d.Attivo == nil {
		return nil, errore(http.StatusBadRequest, "Indica se il pacchetto è attivo.")
	}
	att := 0
	if *d.Attivo {
		att = 1
	}
	res, err := s.DB.Exec(`UPDATE contenuti_pacchetti SET attivo = ? WHERE id = ?`, att, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errNonTrovato
	}
	s.pacchettiCache.svuota()
	s.evento("contenuti_attivo", u.ID, r.PathValue("id"))
	s.Hub.AlMaster(Msg{Tipo: "contenuti", Dati: map[string]any{}})
	return map[string]any{"ok": true}, nil
}

func (s *Server) apiEliminaContenuti(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var nome string
	if s.DB.QueryRow(`SELECT nome FROM contenuti_pacchetti WHERE id = ?`, r.PathValue("id")).Scan(&nome) != nil {
		return nil, errNonTrovato
	}
	if _, err := s.DB.Exec(`DELETE FROM contenuti_pacchetti WHERE id = ?`, r.PathValue("id")); err != nil {
		return nil, err
	}
	s.pacchettiCache.svuota()
	s.evento("contenuti_elimina", u.ID, nome)
	s.Hub.AlMaster(Msg{Tipo: "contenuti", Dati: map[string]any{}})
	return map[string]any{"ok": true}, nil
}

// apiFileContenuti restituisce il file originale del pacchetto (per salvarlo o passarlo a un altro PC).
func (s *Server) apiFileContenuti(w http.ResponseWriter, r *http.Request) {
	if _, err := richiedi(r, "master"); err != nil {
		http.Error(w, "Non autorizzato.", http.StatusForbidden)
		return
	}
	var nome, file string
	if s.DB.QueryRow(`SELECT nome, file FROM contenuti_pacchetti WHERE id = ?`, r.PathValue("id")).Scan(&nome, &file) != nil {
		http.Error(w, "Non trovato.", http.StatusNotFound)
		return
	}
	nf := strings.Map(func(c rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, c) || c < 32 {
			return '_'
		}
		return c
	}, nome)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+urlPercorso(nf+".json"))
	w.Header().Set("Cache-Control", "no-store")
	io.WriteString(w, file)
}

func urlPercorso(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-._~", c) >= 0 {
			b.WriteByte(c)
		} else {
			b.WriteString("%" + strings.ToUpper(hex.EncodeToString([]byte{c})))
		}
	}
	return b.String()
}

// contenutiGenerati prepara (una volta, finché nulla cambia) i due file della scheda con i pacchetti attivi.
func (s *Server) contenutiGenerati() (map[string]*fileGenerato, error) {
	c := &s.pacchettiCache
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pronti {
		return c.file, nil
	}
	_, file, err := s.pacchetti(true)
	if err != nil {
		return nil, err
	}
	tot := nuoviDatiScheda()
	sovr := []string{}
	for _, f := range file {
		p, err := leggiPacchettoOpz([]byte(f), true)
		if err != nil { // non dovrebbe accadere: il file è stato controllato all'importazione
			continue
		}
		sovr = append(sovr, tot.unisci(p.Dati)...)
	}
	out := map[string]*fileGenerato{}
	for nome, coda := range codeContenuti(tot, len(file)) {
		base, err := fs.ReadFile(s.Web, nome)
		if err != nil {
			return nil, err
		}
		dati := append(append(append([]byte{}, base...), '\n'), coda...)
		h := sha256.Sum256(dati)
		g := &fileGenerato{dati: dati, etag: `"c-` + hex.EncodeToString(h[:8]) + `"`}
		var zb bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&zb, gzip.BestCompression)
		zw.Write(dati)
		zw.Close()
		g.gz = zb.Bytes()
		out[nome] = g
	}
	c.file, c.sovr, c.liste, c.pronti = out, sovr, tot.NomiListe, true
	return out, nil
}

// codeContenuti: il codice aggiunto in fondo ai due file. Lavora sulle costanti già dichiarate nel file (oggetti ed
// elenchi vuoti) e funziona sia come script della pagina sia nel caricamento con new Function delle altre sezioni.
func codeContenuti(d *datiScheda, n int) map[string][]byte {
	j, _ := json.Marshal(d)
	js := strings.ReplaceAll(string(j), "</", `<\/`)
	intest := "/* ===== Pacchetti di contenuti attivi: " + itoa(n) + " (aggiunti da Gaster Bless, docs/FORMATO_CONTENUTI.md) ===== */\n"
	dati := intest + `(function(P){
 Object.assign(CLASSES,P.classi);Object.assign(RACES,P.razze);Object.assign(BGS,P.bg);
 SPELLS.push.apply(SPELLS,P.incantesimi);WEAPONS.push.apply(WEAPONS,P.armi);ARMORS.push.apply(ARMORS,P.armature);
 STYLES.push.apply(STYLES,P.stili);STYLES_T.push.apply(STYLES_T,P.stiliT);
 Object.assign(KNOWN,P.conosciuti);Object.assign(CANTRIPS,P.trucchetti);
 Object.assign(CODICI_LISTE,P.codici);Object.assign(NOMI_LISTE,P.nomiListe);Object.assign(FONTI,P.fonti);
 Object.assign(FD_GEN,P.fdGen);Object.assign(FD_CLS,P.fdCls);Object.assign(FD_SUB,P.fdSub);Object.assign(FD_RACE,P.fdRace);
 Object.assign(FD_BG,P.fdBg);Object.assign(FD_RES,P.fdRes);
})(` + js + `);
`
	tj, _ := json.Marshal(d.Talenti)
	tal := intest + `(function(T){FEATS.push.apply(FEATS,T);T.forEach(function(f){FEAT_IDX[_fnorm(f[0])]=f;FEAT_IDX[_fnorm(f[1])]=f;});})(` +
		strings.ReplaceAll(string(tj), "</", `<\/`) + ");\n"
	return map[string][]byte{"regole/dati.js": []byte(dati), "regole/talenti.js": []byte(tal)}
}

// serviContenuti risponde per /regole/dati.js e /regole/talenti.js quando c'è un utente collegato; true se ha risposto.
func (s *Server) serviContenuti(w http.ResponseWriter, r *http.Request, nome string) bool {
	if nome != "regole/dati.js" && nome != "regole/talenti.js" || utenteDa(r.Context()) == nil {
		return false
	}
	file, err := s.contenutiGenerati()
	if err != nil || file[nome] == nil {
		return false
	}
	f := file[nome]
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("ETag", f.etag)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Add("Vary", "Accept-Encoding, Cookie")
	if inm := r.Header.Get("If-None-Match"); inm != "" && strings.Contains(inm, f.etag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	corpo := f.dati
	if accettaGzip(r) {
		w.Header().Set("Content-Encoding", "gzip")
		corpo = f.gz
	}
	w.Header().Set("Content-Length", itoa(len(corpo)))
	if r.Method != http.MethodHead {
		w.Write(corpo)
	}
	return true
}
