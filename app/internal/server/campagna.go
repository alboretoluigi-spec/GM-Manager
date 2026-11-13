package server

// File unico della campagna (fase M4, docs/PIANO_MEDIA_MANUALI.md, 2026-11-11). Impostazioni → «📦 File della
// campagna»: il Master esporta in un file JSON leggibile gruppi e sessioni, personaggi con la scheda (versione corrente),
// bestiario (voci personali e schede/note delle voci), archivio degli oggetti magici, testi delle località e riassunti;
// lo reimporta qui o su un altro PC. Nel file non ci sono password, telefoni, dati di accesso, testo dei manuali né
// documenti. L'importazione aggiunge senza cancellare nulla: prima un'anteprima (stessa transazione, poi annullata) con
// quante voci sono nuove, uguali o diverse; le diverse si aggiornano solo se il Master lo sceglie. È diverso dal backup,
// che resta il ripristino completo e cifrato.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const (
	formatoCampagna  = "gasterbless-campagna"
	versioneCampagna = 1
	maxCampagna      = 96 << 20
	maxVociSezione   = 5000
)

var sezioniCampagna = []string{"gruppi", "personaggi", "bestiario", "oggetti", "localita", "riassunti"}

type fileCampagna struct {
	Formato         string             `json:"formato"`
	Versione        int                `json:"versione"`
	Creato          int64              `json:"creato"`
	Nota            string             `json:"nota,omitempty"`
	Gruppi          []gruppoCampagna   `json:"gruppi,omitempty"`
	Sessioni        []sessioneCampagna `json:"sessioni,omitempty"`
	Personaggi      []pgCampagna       `json:"personaggi,omitempty"`
	BestiarioVoci   []voceBestCampagna `json:"bestiarioVoci,omitempty"`
	BestiarioSchede []schedaBestCamp   `json:"bestiarioSchede,omitempty"`
	Oggetti         []oggettoCampagna  `json:"oggetti,omitempty"`
	Localita        []localitaCampagna `json:"localita,omitempty"`
	Riassunti       []riassuntoCamp    `json:"riassunti,omitempty"`
}

type gruppoCampagna struct {
	Nome       string `json:"nome"`
	Giorno     string `json:"giorno,omitempty"`
	Colore     string `json:"colore,omitempty"`
	Archiviato bool   `json:"archiviato,omitempty"`
}
type sessioneCampagna struct {
	Gruppo string `json:"gruppo"`
	Titolo string `json:"titolo"`
	Inizio int64  `json:"inizio"`
	Luogo  string `json:"luogo,omitempty"`
	Note   string `json:"note,omitempty"`
	Stato  string `json:"stato,omitempty"`
}
type pgCampagna struct {
	Nome      string          `json:"nome"`
	Stato     string          `json:"stato,omitempty"`
	Gruppo    string          `json:"gruppo,omitempty"`
	Giocatore string          `json:"giocatore,omitempty"` // nome dell'account: all'importazione si cerca un giocatore con lo stesso nome
	Scheda    string          `json:"scheda,omitempty"`    // scheda alternativa (vuoto = scheda di Gaster Bless)
	Dati      json.RawMessage `json:"dati"`
}
type voceBestCampagna struct {
	ID    string `json:"id"`
	Nome  string `json:"nome"`
	Cat   string `json:"cat"`
	Ruolo string `json:"ruolo,omitempty"`
	Base  string `json:"base,omitempty"`
}
type schedaBestCamp struct {
	Voce       string `json:"voce"`
	Immagine   string `json:"immagine,omitempty"`
	Note       string `json:"note,omitempty"`
	Scheda     string `json:"scheda,omitempty"`
	SchedaDati string `json:"schedaDati,omitempty"`
	Pagina     int    `json:"pagina,omitempty"`
	Raccolta   string `json:"raccolta,omitempty"` // PNG in scena: ufficiale | reloaded (2026-11-13)
}
type oggettoCampagna struct {
	Oggetto     json.RawMessage `json:"oggetto"`
	ManualeNome string          `json:"manualeNome,omitempty"`
	Pagina      int             `json:"pagina,omitempty"`
}
type localitaCampagna struct {
	Localita    string `json:"localita"`
	Descrizione string `json:"descrizione,omitempty"`
	NoteMaster  string `json:"noteMaster,omitempty"`
}
type riassuntoCamp struct {
	Gruppo         string `json:"gruppo"`
	SessioneTitolo string `json:"sessioneTitolo,omitempty"`
	SessioneInizio int64  `json:"sessioneInizio,omitempty"`
	Autore         string `json:"autore"`
	Titolo         string `json:"titolo,omitempty"`
	Testo          string `json:"testo"`
	Creato         int64  `json:"creato"`
	Ottimo         bool   `json:"ottimo,omitempty"`
}

func (s *Server) rotteCampagna(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/campagna/esporta", api(s.apiEsportaCampagna))
	mux.Handle("POST /api/v1/campagna/importa", api(s.apiImportaCampagna))
}

func scelteSezioni(lista string) map[string]bool {
	m := map[string]bool{}
	for _, x := range strings.Split(lista, ",") {
		for _, k := range sezioniCampagna {
			if strings.TrimSpace(x) == k {
				m[k] = true
			}
		}
	}
	if len(m) == 0 {
		for _, k := range sezioniCampagna {
			m[k] = true
		}
	}
	return m
}

// apiEsportaCampagna: ?sezioni=gruppi,personaggi,… (vuoto = tutte).
func (s *Server) apiEsportaCampagna(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := richiedi(r, "master"); err != nil {
		return nil, err
	}
	sez := scelteSezioni(r.URL.Query().Get("sezioni"))
	f := fileCampagna{Formato: formatoCampagna, Versione: versioneCampagna, Creato: time.Now().Unix(),
		Nota: "File della campagna di Gaster Bless: senza dati di accesso, senza testo dei manuali, senza documenti."}
	righe := func(q string, fn func(*sql.Rows) error) error {
		rows, err := s.DB.Query(q)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err := fn(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	if sez["gruppi"] {
		if err := righe(`SELECT nome, giorno, colore, archiviato FROM gruppi ORDER BY creato_il`, func(x *sql.Rows) error {
			var g gruppoCampagna
			var a int
			err := x.Scan(&g.Nome, &g.Giorno, &g.Colore, &a)
			g.Archiviato = a == 1
			f.Gruppi = append(f.Gruppi, g)
			return err
		}); err != nil {
			return nil, err
		}
		if err := righe(`SELECT g.nome, s.titolo, s.inizio_il, s.luogo, s.note, s.stato FROM sessioni_gioco s JOIN gruppi g ON g.id = s.gruppo_id ORDER BY s.inizio_il`, func(x *sql.Rows) error {
			var v sessioneCampagna
			err := x.Scan(&v.Gruppo, &v.Titolo, &v.Inizio, &v.Luogo, &v.Note, &v.Stato)
			f.Sessioni = append(f.Sessioni, v)
			return err
		}); err != nil {
			return nil, err
		}
	}
	if sez["personaggi"] {
		if err := righe(`SELECT p.nome, p.stato, COALESCE(g.nome,''), COALESCE(u.nome,''), p.scheda, v.dati FROM personaggi p
			JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
			LEFT JOIN gruppi g ON g.id = p.gruppo_id LEFT JOIN utenti u ON u.id = p.proprietario_id ORDER BY p.nome`, func(x *sql.Rows) error {
			var v pgCampagna
			var dati string
			err := x.Scan(&v.Nome, &v.Stato, &v.Gruppo, &v.Giocatore, &v.Scheda, &dati)
			v.Dati = json.RawMessage(dati)
			f.Personaggi = append(f.Personaggi, v)
			return err
		}); err != nil {
			return nil, err
		}
	}
	if sez["bestiario"] {
		if err := righe(`SELECT id, nome, cat, ruolo, base FROM bestiario_voci ORDER BY nome`, func(x *sql.Rows) error {
			var v voceBestCampagna
			err := x.Scan(&v.ID, &v.Nome, &v.Cat, &v.Ruolo, &v.Base)
			f.BestiarioVoci = append(f.BestiarioVoci, v)
			return err
		}); err != nil {
			return nil, err
		}
		if err := righe(`SELECT voce_id, immagine, note, scheda, scheda_dati, pagina, raccolta FROM bestiario_personalizzazioni ORDER BY voce_id`, func(x *sql.Rows) error {
			var v schedaBestCamp
			err := x.Scan(&v.Voce, &v.Immagine, &v.Note, &v.Scheda, &v.SchedaDati, &v.Pagina, &v.Raccolta)
			if v.Immagine != "" || v.Note != "" || v.Scheda != "" || v.SchedaDati != "" || v.Raccolta != "" {
				f.BestiarioSchede = append(f.BestiarioSchede, v)
			}
			return err
		}); err != nil {
			return nil, err
		}
	}
	if sez["oggetti"] {
		if err := righe(`SELECT dati, manuale_nome, pagina FROM oggetti_magici ORDER BY nome`, func(x *sql.Rows) error {
			var v oggettoCampagna
			var dati string
			err := x.Scan(&dati, &v.ManualeNome, &v.Pagina)
			v.Oggetto = json.RawMessage(dati)
			f.Oggetti = append(f.Oggetti, v)
			return err
		}); err != nil {
			return nil, err
		}
	}
	if sez["localita"] {
		if err := righe(`SELECT localita, descrizione, note_master FROM localita_contenuti WHERE descrizione <> '' OR note_master <> '' ORDER BY localita`, func(x *sql.Rows) error {
			var v localitaCampagna
			err := x.Scan(&v.Localita, &v.Descrizione, &v.NoteMaster)
			f.Localita = append(f.Localita, v)
			return err
		}); err != nil {
			return nil, err
		}
	}
	if sez["riassunti"] {
		if err := righe(`SELECT g.nome, COALESCE(s.titolo,''), COALESCE(s.inizio_il,0), r.autore_nome, r.titolo, r.testo, r.creato_il, r.ottimo FROM riassunti r
			JOIN gruppi g ON g.id = r.gruppo_id LEFT JOIN sessioni_gioco s ON s.id = r.sessione_id ORDER BY r.creato_il`, func(x *sql.Rows) error {
			var v riassuntoCamp
			var o int
			err := x.Scan(&v.Gruppo, &v.SessioneTitolo, &v.SessioneInizio, &v.Autore, &v.Titolo, &v.Testo, &v.Creato, &o)
			v.Ottimo = o == 1
			f.Riassunti = append(f.Riassunti, v)
			return err
		}); err != nil {
			return nil, err
		}
	}
	return f, nil
}

type conteggioCampagna struct {
	Nuovi       int `json:"nuovi"`
	Uguali      int `json:"uguali"`
	Diversi     int `json:"diversi"`
	Aggiornati  int `json:"aggiornati"`
	NonValidi   int `json:"nonValidi"`
	SenzaGruppo int `json:"senzaGruppo,omitempty"`
}

var errSoloAnteprima = errors.New("anteprima")

// apiImportaCampagna: {"file": {...}, "sezioni": [...], "aggiorna": bool, "prova": bool}. Con prova = true fa tutto e poi
// annulla: i numeri dell'anteprima sono esattamente quelli dell'importazione.
func (s *Server) apiImportaCampagna(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := richiedi(r, "master")
	if err != nil {
		return nil, err
	}
	var in struct {
		File     json.RawMessage `json:"file"`
		Sezioni  []string        `json:"sezioni"`
		Aggiorna bool            `json:"aggiorna"`
		Prova    bool            `json:"prova"`
	}
	if err := leggiJSONMax(r, &in, maxCampagna); err != nil {
		return nil, err
	}
	var f fileCampagna
	if json.Unmarshal(in.File, &f) != nil || f.Formato != formatoCampagna {
		return nil, errore(http.StatusBadRequest, "Questo non è un file della campagna di Gaster Bless.")
	}
	if f.Versione > versioneCampagna {
		return nil, errore(http.StatusBadRequest, "Il file viene da una versione più recente di Gaster Bless: aggiorna prima Gaster Bless.")
	}
	sez := scelteSezioni(strings.Join(in.Sezioni, ","))
	conti := map[string]*conteggioCampagna{}
	for _, k := range sezioniCampagna {
		conti[k] = &conteggioCampagna{}
	}
	type avvisoPG struct {
		id, prop string
		ver      int
		campi    []string
	}
	var avvisi []avvisoPG
	ora := db.Ora()
	err = db.Tx(r.Context(), s.DB, func(tx *sql.Tx) error {
		// --- gruppi e sessioni (anche quando si importano solo personaggi o riassunti, per ritrovare il gruppo) ---
		gruppi := map[string]string{}
		if rows, err := tx.Query(`SELECT id, nome FROM gruppi`); err == nil {
			for rows.Next() {
				var id, nome string
				rows.Scan(&id, &nome)
				gruppi[chiaveNome(nome)] = id
			}
			rows.Close()
		}
		if sez["gruppi"] {
			c := conti["gruppi"]
			for _, g := range limita2(f.Gruppi) {
				nome, err := rigaSingola(g.Nome, "Nome del gruppo", 1, 60)
				if err != nil {
					c.NonValidi++
					continue
				}
				if id, ok := gruppi[chiaveNome(nome)]; ok {
					var giorno, colore string
					tx.QueryRow(`SELECT giorno, colore FROM gruppi WHERE id = ?`, id).Scan(&giorno, &colore)
					if giorno == limitaTesto(g.Giorno, 40) && colore == limitaTesto(g.Colore, 20) {
						c.Uguali++
					} else if c.Diversi++; in.Aggiorna {
						tx.Exec(`UPDATE gruppi SET giorno = ?, colore = ? WHERE id = ?`, limitaTesto(g.Giorno, 40), limitaTesto(g.Colore, 20), id)
						c.Aggiornati++
					}
					continue
				}
				id := cripto.ID()
				if _, err := tx.Exec(`INSERT INTO gruppi(id, nome, giorno, colore, archiviato, creato_il) VALUES(?,?,?,?,?,?)`,
					id, nome, limitaTesto(g.Giorno, 40), limitaTesto(g.Colore, 20), boolInt(g.Archiviato), ora); err != nil {
					return err
				}
				gruppi[chiaveNome(nome)] = id
				c.Nuovi++
			}
			sessioni := map[string]bool{}
			for _, x := range limita2(f.Sessioni) {
				gid := gruppi[chiaveNome(x.Gruppo)]
				titolo, err := rigaSingola(x.Titolo, "Titolo", 1, 120)
				if gid == "" || err != nil || x.Inizio <= 0 {
					continue
				}
				var n int
				tx.QueryRow(`SELECT COUNT(*) FROM sessioni_gioco WHERE gruppo_id = ? AND titolo = ? AND inizio_il = ?`, gid, titolo, x.Inizio).Scan(&n)
				k := gid + "|" + titolo
				if n > 0 || sessioni[k] {
					continue
				}
				stato := x.Stato
				if stato != "pianificata" && stato != "conclusa" {
					stato = "conclusa" // una sessione «in corso» nel file non riparte da sola
				}
				if _, err := tx.Exec(`INSERT INTO sessioni_gioco(id, gruppo_id, titolo, inizio_il, luogo, note, stato, creato_il) VALUES(?,?,?,?,?,?,?,?)`,
					cripto.ID(), gid, titolo, x.Inizio, limitaTesto(x.Luogo, 200), limitaTesto(x.Note, 4000), stato, ora); err != nil {
					return err
				}
				sessioni[k] = true
			}
		}
		// --- personaggi ---
		if sez["personaggi"] {
			c := conti["personaggi"]
			for _, p := range limita2(f.Personaggi) {
				nome, err := rigaSingola(p.Nome, "Nome del personaggio", 1, 60)
				var dati map[string]json.RawMessage
				if err != nil || len(p.Dati) > maxScheda || json.Unmarshal(p.Dati, &dati) != nil || dati == nil {
					c.NonValidi++
					continue
				}
				gid := gruppi[chiaveNome(p.Gruppo)]
				if p.Gruppo != "" && gid == "" {
					c.SenzaGruppo++
				}
				piatti := appiattisci(dati)
				var id, attuale string
				tx.QueryRow(`SELECT p.id, v.dati FROM personaggi p JOIN schede_versioni v ON v.personaggio_id = p.id AND v.numero = p.versione_corrente
					WHERE lower(p.nome) = lower(?) AND COALESCE(p.gruppo_id,'') = ? LIMIT 1`, nome, gid).Scan(&id, &attuale)
				if id != "" {
					prima, _ := leggiPiatto(attuale)
					if len(differenze(prima, piatti)) == 0 {
						c.Uguali++
						continue
					}
					c.Diversi++
					if !in.Aggiorna {
						continue
					}
					ver, campi, err := s.modificaSchedaNota(tx, id, u, "importazione della campagna", func(q piatto) (bool, error) {
						for k := range q {
							if _, ok := piatti[k]; !ok {
								delete(q, k)
							}
						}
						for k, v := range piatti {
							q[k] = v
						}
						return true, nil
					})
					if err != nil {
						return err
					}
					if ver > 0 {
						var prop string
						tx.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, id).Scan(&prop)
						avvisi = append(avvisi, avvisoPG{id, prop, ver, campi})
					}
					c.Aggiornati++
					continue
				}
				// nuovo: il giocatore con lo stesso nome, se è approvato e (se c'è un gruppo) ne fa parte
				var prop string
				if p.Giocatore != "" {
					tx.QueryRow(`SELECT u.id FROM utenti u WHERE lower(u.nome) = lower(?) AND u.stato = 'approvato' AND u.ruolo <> 'master'
						AND (? = '' OR EXISTS(SELECT 1 FROM iscrizioni i WHERE i.utente_id = u.id AND i.gruppo_id = ?)) LIMIT 1`, p.Giocatore, gid, gid).Scan(&prop)
				}
				stato := p.Stato
				if !map[string]bool{"attivo": true, "ritirato": true, "morto": true, "archiviato": true, "anonimo": true}[stato] {
					stato = "attivo"
				}
				b, err := ricomponi(piatti)
				if err != nil {
					return err
				}
				id = cripto.ID()
				if _, err := tx.Exec(`INSERT INTO personaggi(id, proprietario_id, gruppo_id, nome, stato, versione_corrente, creato_il, modificato_il, scheda) VALUES(?,?,?,?,?,1,?,?,?)`,
					id, nullo(prop), nullo(gid), nome, stato, ora, ora, limitaTesto(p.Scheda, 60)); err != nil {
					return err
				}
				if _, err := tx.Exec(`INSERT INTO schede_versioni(personaggio_id, numero, dati, riepilogo, campi, autore_id, nota, creato_il) VALUES(?,1,?,'{}','[]',?,?,?)`,
					id, string(b), u.ID, "importata dal file della campagna", ora); err != nil {
					return err
				}
				if _, err := tx.Exec(`INSERT INTO stato_in_gioco(personaggio_id, modificato_il, modificato_da) VALUES(?,?,?)`, id, ora, u.ID); err != nil {
					return err
				}
				if err := applicaInGioco(tx, id, estraiInGioco(piatti), u.ID); err != nil {
					return err
				}
				avvisi = append(avvisi, avvisoPG{id, prop, 1, nil})
				c.Nuovi++
			}
		}
		// --- bestiario ---
		if sez["bestiario"] {
			c := conti["bestiario"]
			mappa := map[string]string{} // id nel file → id qui
			for _, v := range limita2(f.BestiarioVoci) {
				nome, err := rigaSingola(v.Nome, "Nome", 1, 120)
				if err != nil || (v.Cat != "png" && v.Cat != "mostro") || !idOggettoValido.MatchString(v.ID) {
					c.NonValidi++
					continue
				}
				var id, n2, ruolo string
				if tx.QueryRow(`SELECT id, nome, ruolo FROM bestiario_voci WHERE id = ?`, v.ID).Scan(&id, &n2, &ruolo) != nil {
					tx.QueryRow(`SELECT id, nome, ruolo FROM bestiario_voci WHERE lower(nome) = lower(?) AND cat = ? LIMIT 1`, nome, v.Cat).Scan(&id, &n2, &ruolo)
				}
				if id != "" {
					mappa[v.ID] = id
					if ruolo == limitaTesto(v.Ruolo, 200) && n2 == nome {
						c.Uguali++
					} else if c.Diversi++; in.Aggiorna {
						tx.Exec(`UPDATE bestiario_voci SET nome = ?, ruolo = ? WHERE id = ?`, nome, limitaTesto(v.Ruolo, 200), id)
						c.Aggiornati++
					}
					continue
				}
				if _, err := tx.Exec(`INSERT INTO bestiario_voci(id, nome, cat, ruolo, base, creato_il) VALUES(?,?,?,?,?,?)`, v.ID, nome, v.Cat, limitaTesto(v.Ruolo, 200), limitaTesto(v.Base, 60), ora); err != nil {
					return err
				}
				mappa[v.ID] = v.ID
				c.Nuovi++
			}
			for _, x := range limita2(f.BestiarioSchede) {
				voce := x.Voce
				if m, ok := mappa[voce]; ok {
					voce = m
				}
				if !idOggettoValido.MatchString(voce) {
					c.NonValidi++
					continue
				}
				sd := ""
				if x.SchedaDati != "" {
					if v, err := validaScheda(json.RawMessage(x.SchedaDati)); err == nil {
						sd = v
					}
				}
				racc := x.Raccolta
				if !raccoltaValida(racc) {
					racc = ""
				}
				nuovo := schedaBestCamp{Immagine: limitaTesto(x.Immagine, 300), Note: limitaTesto(x.Note, 20000), Scheda: limitaTesto(x.Scheda, 60000), SchedaDati: sd, Pagina: x.Pagina, Raccolta: racc}
				var prima schedaBestCamp
				if tx.QueryRow(`SELECT immagine, note, scheda, scheda_dati, pagina, raccolta FROM bestiario_personalizzazioni WHERE voce_id = ?`, voce).
					Scan(&prima.Immagine, &prima.Note, &prima.Scheda, &prima.SchedaDati, &prima.Pagina, &prima.Raccolta) == nil {
					if nuovo.Raccolta == "" { // un file di prima del 13/11 non toglie la raccolta già scelta
						nuovo.Raccolta = prima.Raccolta
					}
					if prima == nuovo {
						c.Uguali++
					} else if c.Diversi++; in.Aggiorna {
						tx.Exec(`UPDATE bestiario_personalizzazioni SET immagine = ?, note = ?, scheda = ?, scheda_dati = ?, pagina = ?, raccolta = ? WHERE voce_id = ?`,
							nuovo.Immagine, nuovo.Note, nuovo.Scheda, nuovo.SchedaDati, nuovo.Pagina, nuovo.Raccolta, voce)
						c.Aggiornati++
					}
					continue
				}
				if _, err := tx.Exec(`INSERT INTO bestiario_personalizzazioni(voce_id, immagine, note, scheda, scheda_dati, pagina, raccolta) VALUES(?,?,?,?,?,?,?)`,
					voce, nuovo.Immagine, nuovo.Note, nuovo.Scheda, nuovo.SchedaDati, nuovo.Pagina, nuovo.Raccolta); err != nil {
					return err
				}
				c.Nuovi++
			}
		}
		// --- oggetti magici ---
		if sez["oggetti"] {
			c := conti["oggetti"]
			for _, x := range limita2(f.Oggetti) {
				o, b, err := validaOggettoMagico(x.Oggetto)
				if err != nil {
					c.NonValidi++
					continue
				}
				var prima string
				if tx.QueryRow(`SELECT dati FROM oggetti_magici WHERE id = ?`, o.ID).Scan(&prima) == nil {
					var p oggettoMagico
					json.Unmarshal([]byte(prima), &p)
					if string(compatta(json.RawMessage(prima))) == string(compatta(b)) {
						c.Uguali++
					} else if c.Diversi++; in.Aggiorna && o.V >= p.V {
						tx.Exec(`UPDATE oggetti_magici SET nome = ?, chiave = ?, dati = ?, aggiornato_il = ? WHERE id = ?`, o.Nome, chiaveNome(o.Nome), string(b), ora, o.ID)
						c.Aggiornati++
					}
					continue
				}
				if _, err := tx.Exec(`INSERT INTO oggetti_magici(id, nome, chiave, dati, manuale_nome, pagina, creato_il, aggiornato_il) VALUES(?,?,?,?,?,?,?,?)`,
					o.ID, o.Nome, chiaveNome(o.Nome), string(b), limitaTesto(x.ManualeNome, 120), x.Pagina, ora, ora); err != nil {
					return err
				}
				c.Nuovi++
			}
		}
		// --- località ---
		if sez["localita"] {
			c := conti["localita"]
			for _, x := range limita2(f.Localita) {
				k := strings.TrimSpace(x.Localita)
				if k == "" || len(k) > 120 {
					c.NonValidi++
					continue
				}
				d, n := limitaTesto(x.Descrizione, 60000), limitaTesto(x.NoteMaster, 60000)
				var d0, n0 string
				if tx.QueryRow(`SELECT descrizione, note_master FROM localita_contenuti WHERE localita = ?`, k).Scan(&d0, &n0) == nil {
					if d0 == d && n0 == n {
						c.Uguali++
					} else if c.Diversi++; in.Aggiorna {
						tx.Exec(`UPDATE localita_contenuti SET descrizione = ?, note_master = ? WHERE localita = ?`, d, n, k)
						c.Aggiornati++
					}
					continue
				}
				if _, err := tx.Exec(`INSERT INTO localita_contenuti(localita, descrizione, note_master) VALUES(?,?,?)`, k, d, n); err != nil {
					return err
				}
				c.Nuovi++
			}
		}
		// --- riassunti ---
		if sez["riassunti"] {
			c := conti["riassunti"]
			for _, x := range limita2(f.Riassunti) {
				gid := gruppi[chiaveNome(x.Gruppo)]
				testo := limitaTesto(x.Testo, 100000)
				if gid == "" {
					c.SenzaGruppo++
					continue
				}
				if strings.TrimSpace(testo) == "" || x.Creato <= 0 {
					c.NonValidi++
					continue
				}
				autore := limitaTesto(x.Autore, 80)
				var id, t0 string
				tx.QueryRow(`SELECT id, testo FROM riassunti WHERE gruppo_id = ? AND autore_nome = ? AND creato_il = ? LIMIT 1`, gid, autore, x.Creato).Scan(&id, &t0)
				if id != "" {
					if t0 == testo {
						c.Uguali++
					} else if c.Diversi++; in.Aggiorna {
						tx.Exec(`UPDATE riassunti SET titolo = ?, testo = ?, modificato_il = ? WHERE id = ?`, limitaTesto(x.Titolo, 200), testo, ora, id)
						c.Aggiornati++
					}
					continue
				}
				var ses string
				if x.SessioneTitolo != "" {
					tx.QueryRow(`SELECT id FROM sessioni_gioco WHERE gruppo_id = ? AND titolo = ? AND inizio_il = ? LIMIT 1`, gid, x.SessioneTitolo, x.SessioneInizio).Scan(&ses)
				}
				if _, err := tx.Exec(`INSERT INTO riassunti(id, gruppo_id, sessione_id, autore_id, autore_nome, titolo, testo, creato_il, modificato_il, ottimo) VALUES(?,?,?,NULL,?,?,?,?,?,?)`,
					cripto.ID(), gid, nullo(ses), autore, limitaTesto(x.Titolo, 200), testo, x.Creato, x.Creato, boolInt(x.Ottimo)); err != nil {
					return err
				}
				c.Nuovi++
			}
		}
		if in.Prova {
			return errSoloAnteprima // tutto annullato: era l'anteprima
		}
		return nil
	})
	if err != nil && !errors.Is(err, errSoloAnteprima) {
		return nil, err
	}
	if !in.Prova {
		for _, a := range avvisi {
			s.avvisaSchedaDa(a.id, a.prop, a.ver, u, a.campi, true)
		}
	}
	return map[string]any{"sezioni": conti, "prova": in.Prova, "creato": f.Creato}, nil
}

func limita2[T any](l []T) []T {
	if len(l) > maxVociSezione {
		return l[:maxVociSezione]
	}
	return l
}

func limitaTesto(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
