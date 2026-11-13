package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
	"gasterbless/internal/rete"
)

const (
	nomeCookie          = "gasterbless_sessione"
	durataSessione      = 30 * 24 * time.Hour
	VersioneInformativa = "2026-11-04"
)

// Utente è l'utente collegato, messo nel contesto della richiesta.
type Utente struct {
	ID     string `json:"id"`
	Nome   string `json:"nome"`
	Ruolo  string `json:"ruolo"`
	Stato  string `json:"stato"`
	csrf   string
	token  string
	totpOK bool
	haTOTP bool
	locale bool
}

// Master dice se l'utente è il Master.
func (u *Utente) Master() bool { return u != nil && u.Ruolo == "master" }

// Approvato dice se l'utente può usare le funzioni di gioco.
func (u *Utente) Approvato() bool { return u != nil && u.Stato == "approvato" }

type chiaveCtx struct{}

func utenteDa(ctx context.Context) *Utente {
	u, _ := ctx.Value(chiaveCtx{}).(*Utente)
	return u
}

// conUtente legge il cookie di sessione e, se valido, mette l'utente nel contesto.
func (s *Server) conUtente(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(nomeCookie); err == nil && c.Value != "" {
			if u := s.caricaSessione(r, c.Value); u != nil {
				r = r.WithContext(context.WithValue(r.Context(), chiaveCtx{}, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) caricaSessione(r *http.Request, token string) *Utente {
	h := cripto.HashToken(token)
	u := &Utente{token: token, locale: rete.Locale(r)}
	var scade, ultimo int64
	var totpOK int
	var totp []byte
	err := s.DB.QueryRow(`SELECT u.id, u.nome, u.ruolo, u.stato, s.csrf, s.scade_il, s.ultimo_uso, s.totp_ok, u.totp_cifrato
		FROM sessioni_accesso s JOIN utenti u ON u.id = s.utente_id WHERE s.token_hash = ?`, h).
		Scan(&u.ID, &u.Nome, &u.Ruolo, &u.Stato, &u.csrf, &scade, &ultimo, &totpOK, &totp)
	if err != nil {
		return nil
	}
	ora := db.Ora()
	if scade < ora {
		s.DB.Exec(`DELETE FROM sessioni_accesso WHERE token_hash = ?`, h)
		return nil
	}
	u.totpOK, u.haTOTP = totpOK == 1, len(totp) > 0
	// Una sessione "provvisoria" (in attesa del secondo fattore) non dà accesso a nulla.
	if !u.totpOK {
		return nil
	}
	// Il Master che arriva da fuori casa deve avere il secondo fattore attivo.
	if u.Master() && !u.locale && s.daFuori(r) && !u.haTOTP {
		return nil
	}
	if ora-ultimo > 300 { // aggiorna l'ultimo uso e rinnova la scadenza, al massimo ogni 5 minuti
		s.DB.Exec(`UPDATE sessioni_accesso SET ultimo_uso = ?, scade_il = ? WHERE token_hash = ?`,
			ora, ora+int64(durataSessione/time.Second), h)
	}
	return u
}

// controlloOrigine: le richieste che modificano dati devono venire da una pagina di Gaster Bless
// (intestazione Origin uguale all'host) e, se c'è una sessione, portare il token anti-CSRF.
func (s *Server) controlloOrigine(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			if u, err := url.Parse(o); err != nil || u.Host != r.Host {
				scriviJSON(w, http.StatusForbidden, map[string]string{"errore": "Richiesta da un'origine non consentita."})
				return
			}
		} else if sf := r.Header.Get("Sec-Fetch-Site"); sf != "" && sf != "same-origin" {
			scriviJSON(w, http.StatusForbidden, map[string]string{"errore": "Richiesta da un'origine non consentita."})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// richiedi controlla accesso, CSRF e stato dell'account. Da usare in ogni gestore protetto.
// livello: "qualsiasi" (anche in attesa), "approvato", "master".
func richiedi(r *http.Request, livello string) (*Utente, error) {
	u := utenteDa(r.Context())
	if u == nil {
		return nil, errNonAutenticato
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && !cripto.UgualiCostante(r.Header.Get("X-CSRF"), u.csrf) {
		return nil, errore(http.StatusForbidden, "Sessione non valida: ricarica la pagina.")
	}
	switch livello {
	case "qualsiasi":
		if u.Stato == "bloccato" || u.Stato == "rifiutato" {
			return nil, errVietato
		}
		return u, nil
	case "approvato":
		if !u.Approvato() {
			return nil, errore(http.StatusForbidden, "Il tuo account non è ancora approvato dal Master.")
		}
		return u, nil
	case "master":
		if !u.Master() {
			return nil, errVietato
		}
		return u, nil
	}
	return nil, errVietato
}

func (s *Server) rotteAccesso(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/io", api(s.apiIo))
	mux.Handle("POST /api/v1/primo-avvio", api(s.apiPrimoAvvio))
	mux.Handle("POST /api/v1/registrazione", api(s.apiRegistrazione))
	mux.Handle("POST /api/v1/accesso", api(s.apiAccesso))
	mux.Handle("POST /api/v1/accesso/totp", api(s.apiAccessoTOTP))
	mux.Handle("POST /api/v1/uscita", api(s.apiUscita))
	mux.Handle("POST /api/v1/password/reimposta", api(s.apiReimposta))
	mux.Handle("POST /api/v1/password/cambia", api(s.apiCambiaPassword))
	mux.Handle("GET /api/v1/privacy", api(s.apiPrivacy))
	mux.Handle("POST /api/v1/totp/prepara", api(s.apiTOTPPrepara))
	mux.Handle("POST /api/v1/totp/attiva", api(s.apiTOTPAttiva))
	mux.Handle("GET /api/v1/invito/{codice}", api(s.apiInvitoInfo))
}

// apiIo descrive chi è collegato. Senza sessione dice solo se serve il primo avvio.
func (s *Server) apiIo(w http.ResponseWriter, r *http.Request) (any, error) {
	u := utenteDa(r.Context())
	if u == nil {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM utenti WHERE ruolo = 'master'`).Scan(&n)
		return map[string]any{"collegato": false, "primoAvvio": n == 0 && rete.Locale(r), "serveMaster": n == 0, "versione": s.versioneApp()}, nil
	}
	out := map[string]any{"collegato": true, "utente": u, "csrf": u.csrf, "locale": u.locale,
		"accessoEsterno": s.Filtro.Esterno(), "totp": u.haTOTP,
		// collegato da fuori casa (VPN o Internet): la mappa alza l'audio dei video proiettati, che al tavolo esce dal PC del Master
		"daFuori":  s.daFuori(r),
		"versione": s.versioneApp(), // le pagine si ricaricano da sole quando cambia (web/ui/aggiorna.js)
		// iPhone senza certificato: l'app manda alla pagina del certificato (HTTP, porta di benvenuto); 0 = spenta
		"portaCertificato": s.Cfg.PortaBenvenuto,
		"pubblica":         rete.Pubblica(r),    // arrivato dall'indirizzo Internet (Funnel o Cloudflare): certificato vero
		"accessoPubblico":  s.Filtro.Pubblico(), // l'indirizzo Internet è aperto adesso (iPhone: alternativa al certificato)
		// indirizzi di Gaster Bless: l'icona sulla schermata Home ricorda quello da cui è stata creata
		"indirizzi":  map[string]string{"casa": s.indirizzoCasa(), "pubblico": s.indirizzoPubblico()},
		"coloreDadi": s.coloreDadi(u.ID)} // set dei dadi 3D (dadi_aspetto.go)
	if u.Master() {
		var n int
		s.DB.QueryRow(`SELECT COUNT(*) FROM utenti WHERE stato = 'in_attesa'`).Scan(&n)
		out["iscrizioniInAttesa"] = n
	} else if u.Approvato() {
		out["professioni"] = s.contaProfessioni(u) // la voce «Professioni» compare solo se ce n'è almeno una
	}
	return out, nil
}

// daFuori: la richiesta arriva dalla VPN o da Internet (Tailscale Funnel), non dalla rete di casa.
// Chi usa l'indirizzo Internet (Tailscale Funnel o Cloudflare Tunnel) anche in casa è «in casa» se arriva
// dall'indirizzo Internet di casa (cloudflare.go).
func (s *Server) daFuori(r *http.Request) bool {
	if rete.Pubblica(r) {
		return !s.ipCasa.uguale(rete.IPRichiesta(r))
	}
	return s.Filtro.EsternaVPN(rete.IPRichiesta(r))
}

// apiPrimoAvvio crea l'account del Master. Possibile una sola volta e solo dal PC locale.
func (s *Server) apiPrimoAvvio(w http.ResponseWriter, r *http.Request) (any, error) {
	if !rete.Locale(r) {
		return nil, errore(http.StatusForbidden, "L'account del Master si crea solo dal PC su cui gira Gaster Bless.")
	}
	var in struct{ Nome, Password string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	nome, err := rigaSingola(in.Nome, "Nome", 1, 60)
	if err != nil {
		return nil, err
	}
	if err := ControllaPassword(in.Password, nome, ""); err != nil {
		return nil, err
	}
	id := cripto.ID()
	ora := db.Ora()
	_, err = s.DB.Exec(`INSERT INTO utenti(id, nome, password_hash, ruolo, stato, avvisi_whatsapp, creato_il, modificato_il)
		SELECT ?, ?, ?, 'master', 'approvato', 0, ?, ? WHERE NOT EXISTS (SELECT 1 FROM utenti WHERE ruolo = 'master')`,
		id, nome, cripto.HashPassword(in.Password), ora, ora)
	if err != nil {
		return nil, err
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM utenti WHERE id = ?`, id).Scan(&n)
	if n == 0 {
		return nil, errore(http.StatusConflict, "L'account del Master esiste già.")
	}
	s.evento("sistema", id, "creato l'account del Master")
	return s.apriSessione(w, r, id, false)
}

type datiRegistrazione struct {
	Nome, Telefono, Password, Conferma, Invito string
	Consenso                                   bool
	Avvisi                                     bool
}

// apiRegistrazione crea un account giocatore "in attesa" di approvazione.
func (s *Server) apiRegistrazione(w http.ResponseWriter, r *http.Request) (any, error) {
	var in datiRegistrazione
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if err := s.limiteControlla("ip:" + rete.IPRichiesta(r).String()); err != nil {
		return nil, err
	}
	nome, err := rigaSingola(in.Nome, "Nome", 2, 40)
	if err != nil {
		return nil, err
	}
	tel, err := NormalizzaTelefono(in.Telefono)
	if err != nil {
		return nil, err
	}
	if in.Password != in.Conferma {
		return nil, errore(http.StatusBadRequest, "Le due password non coincidono.")
	}
	if err := ControllaPassword(in.Password, nome, tel); err != nil {
		return nil, err
	}
	if !in.Consenso {
		return nil, errore(http.StatusBadRequest, "Per registrarti devi leggere e accettare l'informativa sulla privacy.")
	}
	gruppoInvito := ""
	if in.Invito != "" {
		g, err := s.usaInvito(in.Invito, false)
		if err != nil {
			return nil, err
		}
		gruppoInvito = g
	} else if rete.Pubblica(r) {
		return nil, errore(http.StatusForbidden, "Da Internet ci si registra solo con il link d'invito del Master.")
	} else if v, _ := s.impostazione("registrazione_senza_invito"); v == "no" {
		return nil, errore(http.StatusForbidden, "Per registrarti serve il link d'invito del Master.")
	}
	impronta := s.Chiavi.Impronta(tel)
	id := cripto.ID()
	ora := db.Ora()
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		var n int
		tx.QueryRow(`SELECT COUNT(*) FROM utenti WHERE telefono_impronta = ?`, impronta).Scan(&n)
		if n > 0 {
			return errore(http.StatusConflict, "Questo numero di telefono è già registrato. Se hai dimenticato la password, chiedi al Master un codice di reimpostazione.")
		}
		avvisi := 0
		if in.Avvisi {
			avvisi = 1
		}
		if _, err := tx.Exec(`INSERT INTO utenti(id, nome, telefono_cifrato, telefono_impronta, telefono_finale, password_hash, ruolo, stato, avvisi_whatsapp, creato_il, modificato_il)
			VALUES(?,?,?,?,?,?,'giocatore','in_attesa',?,?,?)`,
			id, nome, s.Chiavi.Cifra([]byte(tel), "telefono:"+id), impronta, MascheraTelefono(tel),
			cripto.HashPassword(in.Password), avvisi, ora, ora); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO consensi(utente_id, versione, accettato_il) VALUES(?,?,?)`, id, VersioneInformativa, ora); err != nil {
			return err
		}
		if gruppoInvito != "" {
			if _, err := tx.Exec(`INSERT INTO impostazioni(chiave, valore) VALUES(?, ?)`, "gruppo_proposto:"+id, gruppoInvito); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if in.Invito != "" {
		s.usaInvito(in.Invito, true)
	}
	s.evento("registrazione", id, "nuova richiesta di iscrizione")
	s.Hub.AlMaster(Msg{Tipo: "iscrizioni.nuova", Dati: map[string]any{"nome": nome}})
	return s.apriSessione(w, r, id, false)
}

// apiAccesso: telefono e password (giocatori) oppure nome e password (Master).
func (s *Server) apiAccesso(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Telefono, Nome, Password string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	ip := "ip:" + rete.IPRichiesta(r).String()
	if err := s.limiteControlla(ip); err != nil {
		return nil, err
	}
	var id, hash, stato, ruolo string
	var totp []byte
	chiaveTel := ""
	var err error
	if strings.TrimSpace(in.Telefono) != "" {
		tel, e := NormalizzaTelefono(in.Telefono)
		if e == nil {
			chiaveTel = "tel:" + s.Chiavi.Impronta(tel)
			if err := s.limiteControlla(chiaveTel); err != nil {
				return nil, err
			}
			err = s.DB.QueryRow(`SELECT id, password_hash, stato, ruolo, totp_cifrato FROM utenti WHERE telefono_impronta = ?`,
				s.Chiavi.Impronta(tel)).Scan(&id, &hash, &stato, &ruolo, &totp)
		} else {
			err = sql.ErrNoRows
		}
	} else {
		// Il Master entra con il nome (non ha bisogno di un telefono).
		err = s.DB.QueryRow(`SELECT id, password_hash, stato, ruolo, totp_cifrato FROM utenti WHERE ruolo = 'master' AND nome = ?`,
			strings.TrimSpace(in.Nome)).Scan(&id, &hash, &stato, &ruolo, &totp)
	}
	if errors.Is(err, sql.ErrNoRows) {
		cripto.VerificaPassword(in.Password, cripto.HashFittizio) // tempo di risposta costante
		s.limiteFallito(ip)
		if chiaveTel != "" {
			s.limiteFallito(chiaveTel)
		}
		return nil, errore(http.StatusUnauthorized, "Credenziali non corrette.")
	}
	if err != nil {
		return nil, err
	}
	if !cripto.VerificaPassword(in.Password, hash) {
		s.limiteFallito(ip)
		if chiaveTel != "" {
			s.limiteFallito(chiaveTel)
		}
		s.evento("accesso_fallito", id, "password errata")
		return nil, errore(http.StatusUnauthorized, "Credenziali non corrette.")
	}
	switch stato {
	case "bloccato", "rifiutato":
		return nil, errore(http.StatusForbidden, "Questo account non è abilitato. Rivolgiti al Master.")
	}
	s.limiteAzzera(ip)
	if chiaveTel != "" {
		s.limiteAzzera(chiaveTel)
	}
	if ruolo == "master" && !rete.Locale(r) {
		esterno := s.daFuori(r)
		if len(totp) == 0 && esterno {
			return nil, errore(http.StatusForbidden, "Da fuori casa il Master deve avere attivato il secondo fattore (TOTP) dal PC locale.")
		}
		if len(totp) > 0 {
			// Sessione provvisoria: vale solo per completare il secondo fattore.
			return s.apriSessione(w, r, id, true)
		}
	}
	return s.apriSessione(w, r, id, false)
}

// apriSessione crea la sessione, imposta il cookie e registra eventuali dispositivi nuovi.
func (s *Server) apriSessione(w http.ResponseWriter, r *http.Request, id string, inAttesaTOTP bool) (any, error) {
	token := cripto.Token(32)
	csrf := cripto.Token(24)
	ora := db.Ora()
	scade := ora + int64(durataSessione/time.Second)
	if inAttesaTOTP {
		scade = ora + 300
	}
	disp := descriviDispositivo(r.UserAgent())
	totpOK := 0
	if !inAttesaTOTP {
		totpOK = 1
	}
	if _, err := s.DB.Exec(`INSERT INTO sessioni_accesso(token_hash, utente_id, csrf, creato_il, scade_il, ultimo_uso, ip, dispositivo, totp_ok)
		VALUES(?,?,?,?,?,?,?,?,?)`, cripto.HashToken(token), id, csrf, ora, scade, ora, rete.IPRichiesta(r).String(), disp, totpOK); err != nil {
		return nil, err
	}
	http.SetCookie(w, &http.Cookie{Name: nomeCookie, Value: token, Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteStrictMode, MaxAge: int(durataSessione / time.Second)})
	if inAttesaTOTP {
		return map[string]any{"serveTOTP": true}, nil
	}
	s.DB.Exec(`UPDATE utenti SET ultimo_accesso = ? WHERE id = ?`, ora, id)
	s.registraDispositivo(id, r.UserAgent(), disp)
	s.evento("accesso", id, disp)
	return map[string]any{"ok": true, "csrf": csrf}, nil
}

func impronta(ua string) string {
	h := sha256.Sum256([]byte(ua))
	return hex.EncodeToString(h[:8])
}

func (s *Server) registraDispositivo(id, ua, disp string) {
	imp := impronta(ua)
	ora := db.Ora()
	res, _ := s.DB.Exec(`UPDATE dispositivi_noti SET ultimo_il = ? WHERE utente_id = ? AND impronta = ?`, ora, id, imp)
	if n, _ := res.RowsAffected(); n > 0 {
		return
	}
	var visti int
	s.DB.QueryRow(`SELECT COUNT(*) FROM dispositivi_noti WHERE utente_id = ?`, id).Scan(&visti)
	s.DB.Exec(`INSERT OR IGNORE INTO dispositivi_noti(utente_id, impronta, descrizione, primo_il, ultimo_il) VALUES(?,?,?,?,?)`, id, imp, disp, ora, ora)
	if visti > 0 { // il primo dispositivo non è "nuovo"
		var nome string
		s.DB.QueryRow(`SELECT nome FROM utenti WHERE id = ?`, id).Scan(&nome)
		s.evento("nuovo_dispositivo", id, disp)
		s.Hub.AlMaster(Msg{Tipo: "accesso.nuovo_dispositivo", Dati: map[string]any{"nome": nome, "dispositivo": disp}})
	}
}

// descriviDispositivo produce una descrizione leggibile e generica (niente identificativi).
func descriviDispositivo(ua string) string {
	so := "dispositivo"
	switch {
	case strings.Contains(ua, "iPhone"):
		so = "iPhone"
	case strings.Contains(ua, "iPad"):
		so = "iPad"
	case strings.Contains(ua, "Android"):
		so = "Android"
	case strings.Contains(ua, "Windows"):
		so = "Windows"
	case strings.Contains(ua, "Mac OS"):
		so = "Mac"
	case strings.Contains(ua, "Linux"):
		so = "Linux"
	}
	br := "browser"
	switch {
	case strings.Contains(ua, "Edg/"):
		br = "Edge"
	case strings.Contains(ua, "Firefox/"):
		br = "Firefox"
	case strings.Contains(ua, "Chrome/"):
		br = "Chrome"
	case strings.Contains(ua, "Safari/"):
		br = "Safari"
	}
	return br + " su " + so
}

// apiAccessoTOTP completa l'accesso del Master con il codice dell'app di autenticazione.
func (s *Server) apiAccessoTOTP(w http.ResponseWriter, r *http.Request) (any, error) {
	c, err := r.Cookie(nomeCookie)
	if err != nil {
		return nil, errNonAutenticato
	}
	var in struct{ Codice string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	ip := "ip:" + rete.IPRichiesta(r).String()
	if err := s.limiteControlla(ip); err != nil {
		return nil, err
	}
	h := cripto.HashToken(c.Value)
	var id string
	var cif []byte
	var ultimo int64
	err = s.DB.QueryRow(`SELECT u.id, u.totp_cifrato, u.totp_ultimo FROM sessioni_accesso s JOIN utenti u ON u.id = s.utente_id
		WHERE s.token_hash = ? AND s.totp_ok = 0 AND s.scade_il > ?`, h, db.Ora()).Scan(&id, &cif, &ultimo)
	if err != nil {
		return nil, errNonAutenticato
	}
	seg, err := s.Chiavi.Decifra(cif, "totp:"+id)
	if err != nil {
		return nil, err
	}
	passo, ok := cripto.VerificaTOTP(string(seg), in.Codice, time.Now())
	if !ok || passo <= ultimo {
		s.limiteFallito(ip)
		return nil, errore(http.StatusUnauthorized, "Codice non valido.")
	}
	s.limiteAzzera(ip)
	s.DB.Exec(`UPDATE utenti SET totp_ultimo = ? WHERE id = ?`, passo, id)
	s.DB.Exec(`DELETE FROM sessioni_accesso WHERE token_hash = ?`, h)
	return s.apriSessione(w, r, id, false)
}

func (s *Server) apiUscita(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "qualsiasi")
	if err != nil && !errors.Is(err, errVietato) {
		return nil, err
	}
	if u != nil {
		s.DB.Exec(`DELETE FROM sessioni_accesso WHERE token_hash = ?`, cripto.HashToken(u.token))
	}
	http.SetCookie(w, &http.Cookie{Name: nomeCookie, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	return nil, nil
}

// apiReimposta: il giocatore usa il codice monouso generato dal Master per scegliere una nuova password.
func (s *Server) apiReimposta(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Telefono, Codice, Password, Conferma string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	ip := "ip:" + rete.IPRichiesta(r).String()
	if err := s.limiteControlla(ip); err != nil {
		return nil, err
	}
	tel, err := NormalizzaTelefono(in.Telefono)
	if err != nil {
		return nil, err
	}
	if in.Password != in.Conferma {
		return nil, errore(http.StatusBadRequest, "Le due password non coincidono.")
	}
	codice := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(in.Codice), "-", ""))
	var id, nome, cid string
	err = s.DB.QueryRow(`SELECT u.id, u.nome, c.id FROM codici_reimpostazione c JOIN utenti u ON u.id = c.utente_id
		WHERE c.codice_hash = ? AND u.telefono_impronta = ? AND c.usato_il IS NULL AND c.scade_il > ?`,
		cripto.HashToken(codice), s.Chiavi.Impronta(tel), db.Ora()).Scan(&id, &nome, &cid)
	if err != nil {
		s.limiteFallito(ip)
		return nil, errore(http.StatusBadRequest, "Codice non valido, già usato o scaduto. Chiedine uno nuovo al Master.")
	}
	if err := ControllaPassword(in.Password, nome, tel); err != nil {
		return nil, err
	}
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE codici_reimpostazione SET usato_il = ? WHERE id = ? AND usato_il IS NULL`, db.Ora(), cid)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return errore(http.StatusBadRequest, "Codice già usato.")
		}
		if _, err := tx.Exec(`UPDATE utenti SET password_hash = ?, modificato_il = ? WHERE id = ?`, cripto.HashPassword(in.Password), db.Ora(), id); err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM sessioni_accesso WHERE utente_id = ?`, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.limiteAzzera(ip)
	s.evento("password_reimpostata", id, "")
	return nil, nil
}

func (s *Server) apiCambiaPassword(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "qualsiasi")
	if err != nil {
		return nil, err
	}
	var in struct{ Attuale, Nuova, Conferma string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	var hash string
	s.DB.QueryRow(`SELECT password_hash FROM utenti WHERE id = ?`, u.ID).Scan(&hash)
	if !cripto.VerificaPassword(in.Attuale, hash) {
		return nil, errore(http.StatusBadRequest, "La password attuale non è corretta.")
	}
	if in.Nuova != in.Conferma {
		return nil, errore(http.StatusBadRequest, "Le due password non coincidono.")
	}
	if err := ControllaPassword(in.Nuova, u.Nome, ""); err != nil {
		return nil, err
	}
	s.DB.Exec(`UPDATE utenti SET password_hash = ?, modificato_il = ? WHERE id = ?`, cripto.HashPassword(in.Nuova), db.Ora(), u.ID)
	// Chiude le altre sessioni, tiene quella corrente.
	s.DB.Exec(`DELETE FROM sessioni_accesso WHERE utente_id = ? AND token_hash <> ?`, u.ID, cripto.HashToken(u.token))
	return nil, nil
}

// apiTOTPPrepara genera un nuovo segreto (non ancora attivo) per il Master. Solo dal PC locale.
func (s *Server) apiTOTPPrepara(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	if !u.locale {
		return nil, errore(http.StatusForbidden, "Il secondo fattore si configura solo dal PC su cui gira Gaster Bless.")
	}
	seg := cripto.NuovoSegretoTOTP()
	s.impostaImpostazione("totp_in_preparazione", hex.EncodeToString(s.Chiavi.Cifra([]byte(seg), "totp-prep:"+u.ID)))
	return map[string]any{"segreto": seg, "uri": cripto.URITOTP(seg, u.Nome)}, nil
}

func (s *Server) apiTOTPAttiva(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	if !u.locale {
		return nil, errore(http.StatusForbidden, "Il secondo fattore si configura solo dal PC su cui gira Gaster Bless.")
	}
	var in struct{ Codice string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	v, _ := s.impostazione("totp_in_preparazione")
	cif, err := hex.DecodeString(v)
	if err != nil || len(cif) == 0 {
		return nil, errore(http.StatusBadRequest, "Prepara prima il codice QR.")
	}
	seg, err := s.Chiavi.Decifra(cif, "totp-prep:"+u.ID)
	if err != nil {
		return nil, err
	}
	passo, ok := cripto.VerificaTOTP(string(seg), in.Codice, time.Now())
	if !ok {
		return nil, errore(http.StatusBadRequest, "Codice non valido: controlla l'ora del telefono e riprova.")
	}
	s.DB.Exec(`UPDATE utenti SET totp_cifrato = ?, totp_ultimo = ? WHERE id = ?`, s.Chiavi.Cifra(seg, "totp:"+u.ID), passo, u.ID)
	s.DB.Exec(`DELETE FROM impostazioni WHERE chiave = 'totp_in_preparazione'`)
	s.evento("sistema", u.ID, "secondo fattore attivato")
	return nil, nil
}

// apiPrivacy restituisce versione e testo dell'informativa (pagina sempre raggiungibile).
func (s *Server) apiPrivacy(w http.ResponseWriter, r *http.Request) (any, error) {
	mesi, _ := s.impostazione("inattivi_mesi")
	if mesi == "" {
		mesi = itoa(s.Cfg.InattiviMesi)
	}
	return map[string]any{"versione": VersioneInformativa, "conservazioneMesi": mesi, "backupGiorni": s.Cfg.BackupGiorni}, nil
}

// ===== Limite ai tentativi =====

// limiteControlla rifiuta se la chiave (IP o telefono) è temporaneamente bloccata.
func (s *Server) limiteControlla(chiave string) error {
	var fino int64
	s.DB.QueryRow(`SELECT bloccato_fino FROM tentativi_accesso WHERE chiave = ?`, chiave).Scan(&fino)
	if rimasti := fino - db.Ora(); rimasti > 0 {
		min := (rimasti + 59) / 60
		return errore(http.StatusTooManyRequests, "Troppi tentativi falliti. Riprova tra "+itoa(int(min))+" minut"+map[bool]string{true: "o", false: "i"}[min == 1]+".")
	}
	return nil
}

// Blocco crescente: 5 errori → 1 minuto, poi 5, poi 15 minuti.
var durateBlocco = []int64{60, 300, 900}

func (s *Server) limiteFallito(chiave string) {
	s.muLimite.Lock()
	defer s.muLimite.Unlock()
	ora := db.Ora()
	var falliti, blocchi int
	s.DB.QueryRow(`SELECT falliti, blocchi FROM tentativi_accesso WHERE chiave = ?`, chiave).Scan(&falliti, &blocchi)
	falliti++
	var fino int64
	if falliti >= 5 {
		i := blocchi
		if i >= len(durateBlocco) {
			i = len(durateBlocco) - 1
		}
		fino = ora + durateBlocco[i]
		blocchi++
		falliti = 0
	}
	s.DB.Exec(`INSERT INTO tentativi_accesso(chiave, falliti, blocchi, bloccato_fino, aggiornato_il) VALUES(?,?,?,?,?)
		ON CONFLICT(chiave) DO UPDATE SET falliti = excluded.falliti, blocchi = excluded.blocchi,
		bloccato_fino = MAX(tentativi_accesso.bloccato_fino, excluded.bloccato_fino), aggiornato_il = excluded.aggiornato_il`,
		chiave, falliti, blocchi, fino, ora)
}

func (s *Server) limiteAzzera(chiave string) {
	s.DB.Exec(`DELETE FROM tentativi_accesso WHERE chiave = ?`, chiave)
}
