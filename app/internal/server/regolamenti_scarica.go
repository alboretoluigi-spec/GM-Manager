package server

// RPG RULEBOOK LIBRARY, passo L2 (docs/RULEBOOK_LIBRARY.md): download nella libreria personale (MY RULEBOOKS) dei soli
// documenti del catalogo segnati «downloadDiretto» (gratuiti, verificati, ospitati dall'editore senza accesso). Parte
// solo quando il Master lo chiede. Regole: solo https, solo verso il dominio della voce (anche dopo un rinvio), dimensione
// massima, tempo massimo, ripresa dal punto raggiunto dopo un'interruzione (intestazione Range) con al massimo tre
// tentativi, tipo del file controllato dal contenuto, doppioni riconosciuti dall'impronta SHA-256. Il file parziale sta
// in Dati\regolamenti\parziali e diventa definitivo solo alla fine. Nessun accesso, pagamento o protezione aggirati: se
// il server dell'editore chiede altro, il download fallisce e resta il collegamento alla fonte ufficiale.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gasterbless/internal/cripto"
)

var (
	maxScaricoRegolamenti    int64 = 200 << 20 // 200 MB
	tempoMaxScarico                = 10 * time.Minute
	tentativiScarico               = 3
	attesaTraTentativi             = 2 * time.Second
	clientScaricoRegolamenti       = func() *http.Client { return &http.Client{} } // le prove lo sostituiscono
)

type statoScarico struct {
	Doc       string `json:"doc"`
	Stato     string `json:"stato"` // in corso, fatto, presente, errore
	Ricevuti  int64  `json:"ricevuti"`
	Totale    int64  `json:"totale"`
	Errore    string `json:"errore,omitempty"`
	Documento string `json:"documento,omitempty"` // id in MY RULEBOOKS
}

type scaricamentiRegolamenti struct {
	mu    sync.Mutex
	stato map[string]*statoScarico
}

func (s *Server) rotteScaricaRegolamenti(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/regolamenti/scarica", api(s.apiScaricaRegolamento))
	mux.Handle("GET /api/v1/regolamenti/scaricamenti", api(s.apiScaricamentiRegolamenti))
}

func (s *Server) cartellaRegolamenti(sotto string) string {
	return filepath.Join(s.Cfg.CartellaDati, "regolamenti", sotto)
}

// cercaDocCatalogo: voce del catalogo e gioco a cui appartiene.
func (s *Server) cercaDocCatalogo(id string) (*docCatalogo, *giocoCatalogo) {
	c := s.catalogoRegolamenti()
	for gi := range c.Giochi {
		for di := range c.Giochi[gi].Documenti {
			if c.Giochi[gi].Documenti[di].ID == id {
				return &c.Giochi[gi].Documenti[di], &c.Giochi[gi]
			}
		}
	}
	return nil, nil
}

func (s *Server) aggiornaScarico(st statoScarico) {
	s.scarichi.mu.Lock()
	if s.scarichi.stato == nil {
		s.scarichi.stato = map[string]*statoScarico{}
	}
	s.scarichi.stato[st.Doc] = &st
	s.scarichi.mu.Unlock()
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.scarico", Dati: st})
}

func (s *Server) apiScaricamentiRegolamenti(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	s.scarichi.mu.Lock()
	defer s.scarichi.mu.Unlock()
	out := map[string]statoScarico{}
	for k, v := range s.scarichi.stato {
		out[k] = *v
	}
	return out, nil
}

func (s *Server) apiScaricaRegolamento(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	var in struct {
		Doc string `json:"doc"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
		return nil, errore(http.StatusBadRequest, "Richiesta non valida.")
	}
	d, g := s.cercaDocCatalogo(in.Doc)
	if d == nil {
		return nil, errore(http.StatusNotFound, "Documento non presente nel catalogo.")
	}
	if !d.DownloadDiretto {
		return nil, errore(http.StatusBadRequest, "Per questo documento il download diretto non è consentito: apri la fonte ufficiale.")
	}
	var id string
	if err := s.DB.QueryRow(`SELECT id FROM regolamenti_documenti WHERE catalogo_id = ? ORDER BY creato_il LIMIT 1`, d.ID).Scan(&id); err == nil {
		st := statoScarico{Doc: d.ID, Stato: "presente", Documento: id}
		return st, nil
	}
	s.scarichi.mu.Lock()
	if v := s.scarichi.stato[d.ID]; v != nil && v.Stato == "in corso" {
		st := *v
		s.scarichi.mu.Unlock()
		return st, nil
	}
	s.scarichi.mu.Unlock()
	st := statoScarico{Doc: d.ID, Stato: "in corso"}
	s.aggiornaScarico(st)
	go s.scaricaRegolamento(*d, *g)
	return st, nil
}

// stessoDominio: l'host di arrivo appartiene allo stesso dominio di quello della voce (es. downloads.editore.com).
func stessoDominio(origine, arrivo string) bool {
	base := func(h string) string {
		p := strings.Split(strings.ToLower(h), ".")
		if len(p) < 2 {
			return strings.ToLower(h)
		}
		return strings.Join(p[len(p)-2:], ".")
	}
	return base(origine) == base(arrivo)
}

// erroreDefinitivo: errori per cui un nuovo tentativo non serve.
type erroreDefinitivo struct{ msg string }

func (e erroreDefinitivo) Error() string { return e.msg }

func (s *Server) scaricaRegolamento(d docCatalogo, g giocoCatalogo) {
	st := statoScarico{Doc: d.ID, Stato: "in corso"}
	fallisci := func(msg string) {
		st.Stato, st.Errore = "errore", msg
		s.aggiornaScarico(st)
		slog.Warn("download di un regolamento non riuscito", "documento", d.ID, "errore", msg)
	}
	u, err := url.Parse(d.URL)
	if err != nil || u.Scheme != "https" {
		fallisci("Indirizzo non valido: sono ammessi solo indirizzi https.")
		return
	}
	if err := os.MkdirAll(s.cartellaRegolamenti("parziali"), 0o700); err != nil {
		fallisci("Cartella dei download non disponibile.")
		return
	}
	parziale := filepath.Join(s.cartellaRegolamenti("parziali"), d.ID+".part")
	scadenza := time.Now().Add(tempoMaxScarico)
	var ultimo error
	for t := 0; t < tentativiScarico && time.Now().Before(scadenza); t++ {
		if t > 0 {
			time.Sleep(attesaTraTentativi)
		}
		ultimo = s.tentativoScarico(u, parziale, scadenza, &st)
		if ultimo == nil {
			break
		}
		var def erroreDefinitivo
		if errors.As(ultimo, &def) {
			os.Remove(parziale)
			fallisci(def.msg)
			return
		}
	}
	if ultimo != nil {
		fallisci("Download interrotto dopo " + fmt.Sprint(tentativiScarico) + " tentativi: " + ultimo.Error() + ". Riprova più tardi: si riparte dal punto raggiunto.")
		return
	}
	id, presente, err := s.salvaScaricato(parziale, d, g)
	os.Remove(parziale)
	if err != nil {
		fallisci(err.Error())
		return
	}
	st.Stato, st.Documento = "fatto", id
	if presente {
		st.Stato = "presente"
	}
	s.aggiornaScarico(st)
}

// tentativoScarico scarica (o riprende) il file in «parziale».
func (s *Server) tentativoScarico(u *url.URL, parziale string, scadenza time.Time, st *statoScarico) error {
	cl := clientScaricoRegolamenti()
	cl.Timeout = time.Until(scadenza)
	cl.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return erroreDefinitivo{"Troppi rinvii dalla fonte."}
		}
		if req.URL.Scheme != "https" || !stessoDominio(u.Hostname(), req.URL.Hostname()) {
			return erroreDefinitivo{"La fonte rinvia a un altro sito (" + req.URL.Hostname() + "): download annullato, apri la fonte ufficiale."}
		}
		return nil
	}
	var gia int64
	if fi, err := os.Stat(parziale); err == nil {
		gia = fi.Size()
	}
	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("User-Agent", "GasterBless/1 (download di documenti gratuiti)")
	if gia > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", gia))
	}
	resp, err := cl.Do(req)
	if err != nil {
		var def erroreDefinitivo
		if errors.As(err, &def) {
			return def
		}
		return err
	}
	defer resp.Body.Close()
	flag := os.O_CREATE | os.O_WRONLY
	switch {
	case resp.StatusCode == http.StatusPartialContent && gia > 0:
		flag |= os.O_APPEND
	case resp.StatusCode == http.StatusOK:
		flag |= os.O_TRUNC
		gia = 0
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusPaymentRequired:
		return erroreDefinitivo{"La fonte richiede un accesso: il download diretto non è possibile, apri la fonte ufficiale."}
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return erroreDefinitivo{"Il documento non è più a questo indirizzo: il catalogo va aggiornato."}
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		os.Remove(parziale)
		return fmt.Errorf("ripresa non accettata dalla fonte")
	default:
		return fmt.Errorf("risposta %d dalla fonte", resp.StatusCode)
	}
	if resp.ContentLength > 0 && gia+resp.ContentLength > maxScaricoRegolamenti {
		return erroreDefinitivo{fmt.Sprintf("Il file è troppo grande (oltre %d MB).", maxScaricoRegolamenti>>20)}
	}
	if resp.ContentLength > 0 {
		st.Totale = gia + resp.ContentLength
	}
	f, err := os.OpenFile(parziale, flag, 0o600)
	if err != nil {
		return erroreDefinitivo{"Impossibile scrivere il file sul disco."}
	}
	defer f.Close()
	buf := make([]byte, 64<<10)
	ricevuti, ultimoAvviso := gia, time.Now()
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if ricevuti+int64(n) > maxScaricoRegolamenti {
				return erroreDefinitivo{fmt.Sprintf("Il file è troppo grande (oltre %d MB).", maxScaricoRegolamenti>>20)}
			}
			if _, e := f.Write(buf[:n]); e != nil {
				return erroreDefinitivo{"Impossibile scrivere il file sul disco."}
			}
			ricevuti += int64(n)
			if time.Since(ultimoAvviso) > 500*time.Millisecond {
				st.Ricevuti = ricevuti
				s.aggiornaScarico(*st)
				ultimoAvviso = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err // si riprende da qui al prossimo tentativo
		}
	}
	if st.Totale > 0 && ricevuti < st.Totale {
		return fmt.Errorf("file incompleto (%d di %d byte)", ricevuti, st.Totale)
	}
	st.Ricevuti = ricevuti
	return nil
}

// firmaFormato: il contenuto corrisponde al formato dichiarato? (PDF, ZIP/EPUB/DOCX)
func firmaFormato(formato string, inizio []byte) bool {
	switch formato {
	case "pdf":
		return len(inizio) >= 5 && string(inizio[:5]) == "%PDF-"
	case "zip", "epub", "docx":
		return len(inizio) >= 4 && string(inizio[:4]) == "PK\x03\x04"
	}
	return false
}

// salvaScaricato controlla il file e lo registra in MY RULEBOOKS. presente = c'era già un documento identico.
func (s *Server) salvaScaricato(parziale string, d docCatalogo, g giocoCatalogo) (string, bool, error) {
	f, err := os.Open(parziale)
	if err != nil {
		return "", false, errors.New("file scaricato non leggibile")
	}
	defer f.Close()
	inizio := make([]byte, 8)
	n, _ := io.ReadFull(f, inizio)
	if !firmaFormato(d.Formato, inizio[:n]) {
		return "", false, errors.New("Il file ricevuto non è un " + strings.ToUpper(d.Formato) + " valido (la fonte potrebbe aver cambiato pagina): non è stato salvato.")
	}
	f.Seek(0, io.SeekStart)
	h := sha256.New()
	dim, _ := io.Copy(h, f)
	impronta := hex.EncodeToString(h.Sum(nil))
	var esiste string
	if err := s.DB.QueryRow(`SELECT id FROM regolamenti_documenti WHERE impronta = ?`, impronta).Scan(&esiste); err == nil {
		return esiste, true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", false, errors.New("database non disponibile")
	}
	id := cripto.ID()
	nome := id + "." + d.Formato
	if err := os.MkdirAll(s.cartellaRegolamenti("file"), 0o700); err != nil {
		return "", false, errors.New("cartella della libreria non disponibile")
	}
	f.Close()
	dest := filepath.Join(s.cartellaRegolamenti("file"), nome)
	if err := os.Rename(parziale, dest); err != nil {
		return "", false, errors.New("impossibile spostare il file nella libreria")
	}
	editore := d.Editore
	if editore == "" {
		editore = g.Editore
	}
	ora := time.Now().Unix()
	_, err = s.DB.Exec(`INSERT INTO regolamenti_documenti (id, titolo, gioco, edizione, tipologia, lingua, editore, formato, file,
		dimensione, impronta, origine, catalogo_id, fonte_url, licenza, creato_il, aggiornato_il)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'download', ?, ?, ?, ?, ?)`,
		id, d.Titolo, g.Nome, d.Edizione, d.Tipologia, d.Lingua, editore, d.Formato, nome, dim, impronta, d.ID, d.URL, d.Licenza, ora, ora)
	if err != nil {
		os.Remove(dest)
		return "", false, errors.New("registrazione nella libreria non riuscita")
	}
	s.Hub.AlMaster(Msg{Tipo: "regolamenti.miei", Dati: map[string]any{}})
	return id, false, nil
}
