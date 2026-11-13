package server

// Migrazione dei dati della vecchia Gaster Bless (cartella GasterBlessDati) nel database della nuova app.
//
// - Legge SOLO: GasterBlessDati non viene mai modificata (i media restano dove sono e la nuova app li usa lì).
// - "GasterBless.exe migra -prova" legge tutto e scrive un rapporto, senza toccare nulla.
// - La migrazione vera crea prima un backup della nuova app, poi scrive tutto in un'unica transazione.
// - Password e link d'invito dei gruppi NON si migrano: ora ci sono gli account.
//
// Il formato dei file vecchi si ricava dalla pagina della mappa (il server originale è andato perso):
// si accettano le varianti più probabili dei nomi dei campi e il rapporto elenca tutto ciò che non è
// stato riconosciuto, così si può correggere prima della migrazione vera.

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

type rapportoMigrazione struct {
	righe    []string
	avvisi   []string
	conteggi map[string]int
}

func (r *rapportoMigrazione) ok(f string, a ...any) {
	r.righe = append(r.righe, "  ✓ "+fmt.Sprintf(f, a...))
}
func (r *rapportoMigrazione) avviso(f string, a ...any) {
	r.avvisi = append(r.avvisi, "  ⚠ "+fmt.Sprintf(f, a...))
}

func (r *rapportoMigrazione) testo(prova bool, cartella string) string {
	var b strings.Builder
	if prova {
		b.WriteString("MIGRAZIONE DI PROVA (nessun dato è stato scritto)\n")
	} else {
		b.WriteString("MIGRAZIONE COMPLETATA\n")
	}
	b.WriteString("Cartella letta: " + cartella + "\n\n")
	b.WriteString(strings.Join(r.righe, "\n") + "\n")
	if len(r.avvisi) > 0 {
		b.WriteString("\nDa controllare:\n" + strings.Join(r.avvisi, "\n") + "\n")
	}
	return b.String()
}

// leggiJSONFile legge un file JSON in una mappa generica. ok=false se il file non c'è.
func leggiJSONFile(p string) (any, bool, error) {
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	b = []byte(strings.TrimPrefix(string(b), "\uFEFF"))
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, true, fmt.Errorf("%s non è un JSON valido: %w", filepath.Base(p), err)
	}
	return v, true, nil
}

// campo restituisce il primo campo presente tra i nomi indicati.
func campo(m map[string]any, nomi ...string) any {
	for _, n := range nomi {
		if v, ok := m[n]; ok && v != nil {
			return v
		}
	}
	return nil
}

func stringa(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprint(t)
	}
	return ""
}

func mappaBool(v any) map[string]bool {
	out := map[string]bool{}
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if b, ok := x.(bool); ok && b || x == float64(1) {
				out[k] = true
			}
		}
	case []any:
		for _, x := range t {
			if s := stringa(x); s != "" {
				out[s] = true
			}
		}
	}
	return out
}

func noti(m map[string]any, attesi ...string) []string {
	s := map[string]bool{}
	for _, a := range attesi {
		s[a] = true
	}
	var out []string
	for k := range m {
		if !s[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

var reData = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})|(\d{1,2})/(\d{1,2})/(\d{4})`)

// Migra importa i dati. Con prova=true legge soltanto e restituisce il rapporto.
func (s *Server) Migra(ctx context.Context, prova bool) (string, error) {
	cartella := s.Cfg.CartellaMedia
	r := &rapportoMigrazione{conteggi: map[string]int{}}
	if fi, err := os.Stat(cartella); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("cartella dei dati della vecchia Gaster Bless non trovata: %s (impostala con cartella_media nel file gasterbless.conf)", cartella)
	}
	if v, _ := s.impostazione("migrazione_fatta"); v != "" && !prova {
		return "", fmt.Errorf("la migrazione è già stata fatta il %s; per rifarla ripristina prima un backup precedente", v)
	}
	if !prova {
		nome, err := s.CreaBackup(ctx)
		if err != nil {
			return "", fmt.Errorf("backup preliminare non riuscito: %w", err)
		}
		r.ok("backup della nuova app prima della migrazione: %s", nome)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	ora := db.Ora()

	// ----- Gruppi e stato della mappa -----
	vg, c, err := leggiJSONFile(filepath.Join(cartella, "gruppi.json"))
	if err != nil {
		return "", err
	}
	if !c {
		r.avviso("gruppi.json non trovato: nessun gruppo da migrare")
	}
	var lista []any
	switch t := vg.(type) {
	case []any:
		lista = t
	case map[string]any:
		if l, ok := campo(t, "list", "gruppi", "groups").([]any); ok {
			lista = l
		} else {
			for k, x := range t { // forma {id: gruppo}
				if g, ok := x.(map[string]any); ok {
					if g["id"] == nil {
						g["id"] = k
					}
					lista = append(lista, g)
				}
			}
		}
	}
	for _, x := range lista {
		g, ok := x.(map[string]any)
		if !ok {
			continue
		}
		nome := strings.TrimSpace(stringa(campo(g, "name", "nome")))
		if nome == "" {
			r.avviso("un gruppo senza nome è stato saltato")
			continue
		}
		giorno := stringa(campo(g, "day", "giorno"))
		if !giorniValidi[giorno] {
			giorno = ""
		}
		var id string
		if tx.QueryRow(`SELECT id FROM gruppi WHERE nome = ? AND archiviato = 0`, nome).Scan(&id) != nil {
			id = cripto.ID()
			if _, err := tx.Exec(`INSERT INTO gruppi(id, nome, giorno, colore, creato_il) VALUES(?,?,?,'',?)`, id, nome, giorno, ora); err != nil {
				return "", err
			}
			r.ok("gruppo «%s» creato", nome)
		} else {
			r.ok("gruppo «%s» già presente: si aggiorna", nome)
		}
		r.conteggi["gruppi"]++
		if strings.Contains(strings.ToLower(strings.Join(noti(g), " ")), "pw") || g["pw"] != nil || g["password"] != nil {
			r.ok("  password e link d'invito di «%s» non migrati (ora i giocatori hanno un account)", nome)
		}
		// Cartella della sessione: dal campo del gruppo, altrimenti per nome.
		sub := stringa(campo(g, "folder", "cartella", "dir", "id"))
		dirG := filepath.Join(cartella, "Sessioni", sub)
		if _, err := os.Stat(filepath.Join(dirG, "stato.json")); err != nil {
			dirG = filepath.Join(cartella, "Sessioni", nome)
		}
		vs, c, err := leggiJSONFile(filepath.Join(dirG, "stato.json"))
		if err != nil {
			r.avviso("%v", err)
		} else if !c {
			r.avviso("stato.json del gruppo «%s» non trovato (cercato in Sessioni\\%s): la mappa del gruppo parte coperta", nome, sub)
		} else if st, ok := vs.(map[string]any); ok {
			if err := migraStatoMappa(tx, id, st, r, nome); err != nil {
				return "", err
			}
		}
		// Registro delle sessioni giocate: una sessione conclusa per ogni data.
		if b, err := os.ReadFile(filepath.Join(dirG, "registro.txt")); err == nil {
			n := 0
			for _, riga := range strings.Split(string(b), "\n") {
				m := reData.FindStringSubmatch(riga)
				if m == nil {
					continue
				}
				var t time.Time
				if m[1] != "" {
					t, _ = time.ParseInLocation("2006-01-02", m[1]+"-"+m[2]+"-"+m[3], time.Local)
				} else {
					t, _ = time.ParseInLocation("2/1/2006", m[4]+"/"+m[5]+"/"+m[6], time.Local)
				}
				if t.IsZero() {
					continue
				}
				t = t.Add(21 * time.Hour)
				titolo := "Sessione del " + t.Format("02/01/2006")
				var esiste int
				tx.QueryRow(`SELECT COUNT(*) FROM sessioni_gioco WHERE gruppo_id = ? AND titolo = ?`, id, titolo).Scan(&esiste)
				if esiste > 0 {
					continue
				}
				tx.Exec(`INSERT INTO sessioni_gioco(id, gruppo_id, titolo, inizio_il, luogo, note, stato, creato_il) VALUES(?,?,?,?,'','importata dalla vecchia Gaster Bless','conclusa',?)`,
					cripto.ID(), id, titolo, t.Unix(), ora)
				n++
			}
			r.ok("  %d sessioni giocate importate da registro.txt", n)
		}
	}

	// ----- Contenuti delle località -----
	vc, c, err := leggiJSONFile(filepath.Join(cartella, "contenuti.json"))
	if err != nil {
		r.avviso("%v", err)
	} else if c {
		cont, _ := vc.(map[string]any)
		if inner, ok := campo(cont, "locs", "content", "contenuti").(map[string]any); ok { // forma {"locs": {località: ...}}
			cont = inner
		}
		nLoc, nMedia, sconosciute := 0, 0, []string{}
		campiLoc, campiMedia := map[string]bool{}, map[string]bool{} // campi non riconosciuti (per il rapporto)
		for k, x := range cont {
			l, ok := x.(map[string]any)
			if !ok {
				continue
			}
			for _, c := range noti(l, "desc", "descrizione", "notes", "note", "items", "media") {
				campiLoc[c] = true
			}
			if s.cercaLocalita(k) == nil {
				sconosciute = append(sconosciute, k)
				continue
			}
			desc, note := stringa(campo(l, "desc", "descrizione")), stringa(campo(l, "notes", "note"))
			if desc != "" || note != "" {
				tx.Exec(`INSERT INTO localita_contenuti(localita, descrizione, note_master) VALUES(?,?,?)
					ON CONFLICT(localita) DO UPDATE SET descrizione = excluded.descrizione, note_master = excluded.note_master`, k, desc, note)
				nLoc++
			}
			items, _ := campo(l, "items", "media").([]any)
			for i, y := range items {
				it, ok := y.(map[string]any)
				if !ok {
					continue
				}
				for _, c := range noti(it, "id", "k", "tipo", "file", "url", "cap", "didascalia") {
					campiMedia[c] = true
				}
				idm := stringa(it["id"])
				if idm == "" {
					idm = cripto.ID()
				}
				tipo := stringa(campo(it, "k", "tipo"))
				file := strings.ReplaceAll(stringa(it["file"]), "\\", "/")
				url := stringa(it["url"])
				if tipo != "img" && tipo != "vid" && tipo != "link" {
					r.avviso("elemento %s di %s con tipo sconosciuto «%s»: saltato", idm, k, tipo)
					continue
				}
				if file != "" {
					if trovato := cercaFileMedia(cartella, file); trovato != "" {
						file = trovato
					} else {
						r.avviso("file mancante per %s: %s (l'elemento viene comunque importato)", k, file)
					}
				}
				tx.Exec(`INSERT OR IGNORE INTO media(id, localita, tipo, file, url, didascalia, ordine, creato_il) VALUES(?,?,?,?,?,?,?,?)`,
					idm, k, tipo, file, url, stringa(campo(it, "cap", "didascalia")), i+1, ora)
				nMedia++
			}
		}
		r.ok("contenuti: %d località con testi, %d elementi multimediali (i file restano nella cartella)", nLoc, nMedia)
		if len(sconosciute) > 0 {
			sort.Strings(sconosciute)
			r.avviso("località non riconosciute in contenuti.json: %s", strings.Join(sconosciute, ", "))
		}
		for titolo, m := range map[string]map[string]bool{"delle località": campiLoc, "degli elementi multimediali": campiMedia} {
			if len(m) > 0 {
				l := make([]string, 0, len(m))
				for c := range m {
					l = append(l, c)
				}
				sort.Strings(l)
				r.avviso("campi %s non riconosciuti in contenuti.json (ignorati): %s", titolo, strings.Join(l, ", "))
			}
		}
	} else {
		r.avviso("contenuti.json non trovato")
	}

	// ----- Bestiario e manuali -----
	vb, c, err := leggiJSONFile(filepath.Join(cartella, "bestiario.json"))
	if err != nil {
		r.avviso("%v", err)
	} else if c {
		b, _ := vb.(map[string]any)
		nE, nX, nM := 0, 0, 0
		if ex, ok := campo(b, "extras", "voci").([]any); ok {
			for _, x := range ex {
				e, _ := x.(map[string]any)
				id, nome := stringa(e["id"]), stringa(e["nome"])
				cat := stringa(e["cat"])
				if id == "" || nome == "" {
					continue
				}
				if cat != "png" {
					cat = "mostro"
				}
				tx.Exec(`INSERT OR IGNORE INTO bestiario_voci(id, nome, cat, ruolo, creato_il) VALUES(?,?,?,?,?)`, id, nome, cat, stringa(e["ruolo"]), ora)
				nX++
			}
		}
		if ms, ok := campo(b, "manuals", "manuali").([]any); ok {
			for _, x := range ms {
				m, _ := x.(map[string]any)
				id, file := stringa(m["id"]), filepath.Base(strings.ReplaceAll(stringa(m["file"]), "\\", "/"))
				if id == "" || file == "" || file == "." {
					continue
				}
				if _, err := os.Stat(filepath.Join(cartella, "Manuali", file)); err != nil {
					r.avviso("manuale «%s»: file Manuali\\%s non trovato", stringa(m["nome"]), file)
				}
				tipo := stringa(m["tipo"])
				if tipo != "mm" && tipo != "avventura" {
					tipo = "altro"
				}
				idx, _ := json.Marshal(m["index"])
				if string(idx) == "null" {
					idx = []byte("{}")
				}
				rb, rp := numero(m["refBook"], 1), numero(m["refPdf"], 1)
				tx.Exec(`INSERT OR IGNORE INTO manuali(id, nome, tipo, file, ref_libro, ref_pdf, doppia, indice, indicizzato, senza_testo, creato_il) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
					id, stringa(m["nome"]), tipo, file, rb, rp, boolInt(m["spread"] == true), string(idx), boolInt(m["indexed"] == true), boolInt(m["noText"] == true), ora)
				nM++
			}
		}
		if es, ok := campo(b, "entries", "voci_personalizzate").(map[string]any); ok {
			for id, x := range es {
				e, _ := x.(map[string]any)
				img := strings.ReplaceAll(stringa(e["img"]), "\\", "/")
				man := stringa(e["man"])
				tx.Exec(`INSERT INTO bestiario_personalizzazioni(voce_id, immagine, note, scheda, manuale_id, pagina) VALUES(?,?,?,?,?,?)
					ON CONFLICT(voce_id) DO UPDATE SET immagine = excluded.immagine, note = excluded.note, scheda = excluded.scheda,
					manuale_id = excluded.manuale_id, pagina = excluded.pagina`, id, img, stringa(e["notes"]), stringa(e["custom"]), nullo(man), numero(e["page"], 0))
				nE++
			}
		}
		r.ok("bestiario: %d personalizzazioni, %d voci personali, %d manuali", nE, nX, nM)
	}

	// ----- Musica -----
	vm, c, err := leggiJSONFile(filepath.Join(cartella, "musica.json"))
	if err != nil {
		r.avviso("%v", err)
	} else if c {
		m, _ := vm.(map[string]any)
		cfgM, _ := campo(m, "cfg").(map[string]any)
		if cfgM == nil {
			cfgM = m
		}
		mi := impostazioniMusica{Volume: 0.6, Mode: "playlist", Out: "master", Fight: []string{}, Locs: map[string]string{}, Lock: map[string]bool{}}
		if v, ok := cfgM["volume"].(float64); ok && v >= 0 && v <= 1 {
			mi.Volume = v
		}
		if v := stringa(cfgM["mode"]); v == "shuffle" || v == "repeat" || v == "playlist" {
			mi.Mode = v
		}
		if v := stringa(cfgM["out"]); v == "proj" || v == "master" {
			mi.Out = v
		}
		if l, ok := campo(cfgM, "fight").([]any); ok {
			for _, x := range l {
				mi.Fight = append(mi.Fight, strings.ReplaceAll(stringa(x), "\\", "/"))
			}
		}
		if l, ok := campo(m, "locs").(map[string]any); ok {
			for k, x := range l {
				mi.Locs[k] = strings.ReplaceAll(stringa(x), "\\", "/")
			}
		}
		mi.Lock = mappaBool(campo(m, "songLock", "lock"))
		b, _ := json.Marshal(mi)
		tx.Exec(`INSERT INTO impostazioni(chiave, valore) VALUES('musica', ?) ON CONFLICT(chiave) DO UPDATE SET valore = excluded.valore`, string(b))
		r.ok("musica: volume, ordine, %d brani di combattimento, %d musiche di località", len(mi.Fight), len(mi.Locs))
	}

	// ----- File multimediali (restano dove sono) -----
	for _, sub := range []string{"Video", "Musica", "Manuali", "Immagini"} {
		n, dim := 0, int64(0)
		filepath.Walk(filepath.Join(cartella, sub), func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				n++
				dim += fi.Size()
			}
			return nil
		})
		r.ok("%s: %d file (%d MB) — restano nella cartella, la nuova app li usa da lì", sub, n, dim>>20)
	}

	if prova {
		return r.testo(true, cartella), nil
	}
	tx.Exec(`INSERT INTO impostazioni(chiave, valore) VALUES('migrazione_fatta', ?) ON CONFLICT(chiave) DO UPDATE SET valore = excluded.valore`, time.Now().Format("02/01/2006 15:04"))
	if err := tx.Commit(); err != nil {
		return r.testo(false, cartella), err
	}
	m := s.mus() // le impostazioni della musica si rileggono dal database
	m.mu.Lock()
	m.letta = false
	m.mu.Unlock()
	return r.testo(false, cartella), nil
}

// migraStatoMappa: nebbia, scala, nomi/media svelati, mappe locali nascoste e proiezione del gruppo.
func migraStatoMappa(tx *sql.Tx, gruppo string, st map[string]any, r *rapportoMigrazione, nome string) error {
	neb := nebbiaVuota()
	nebbiaOK := false
	if f := stringa(campo(st, "fog", "nebbia")); f != "" {
		if b, err := base64.StdEncoding.DecodeString(f); err == nil && len(b) <= byteNebbia && len(b) >= byteNebbia-1 {
			copy(neb, b)
			nebbiaOK = true
		}
	}
	if !nebbiaOK {
		r.avviso("nebbia del gruppo «%s» in un formato non riconosciuto: la mappa parte coperta", nome)
	}
	scala := 0.25
	if v, ok := campo(st, "miHex", "scala").(float64); ok && v >= 0.05 && v <= 10 {
		scala = v
	}
	js := func(v any) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	proj := map[string]any{"kind": "none"}
	if p, ok := campo(st, "proj", "proiezione").(map[string]any); ok && stringa(p["kind"]) != "" {
		proj = p
	}
	nomi, media, nasc := mappaBool(campo(st, "pubNames")), mappaBool(campo(st, "pubItems")), mappaBool(campo(st, "hideBuiltin"))
	_, err := tx.Exec(`INSERT INTO stato_mappa(gruppo_id, nebbia, scala, nomi_svelati, media_svelati, mappe_nascoste, proiezione, modificato_il) VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(gruppo_id) DO UPDATE SET nebbia = excluded.nebbia, scala = excluded.scala, nomi_svelati = excluded.nomi_svelati,
		media_svelati = excluded.media_svelati, mappe_nascoste = excluded.mappe_nascoste, proiezione = excluded.proiezione, modificato_il = excluded.modificato_il`,
		gruppo, neb, scala, js(nomi), js(media), js(nasc), js(proj), db.Ora())
	if err != nil {
		return err
	}
	r.ok("  mappa di «%s»: nebbia %s, scala %.2f mi/esagono, %d nomi e %d media svelati", nome, map[bool]string{true: "importata", false: "coperta"}[nebbiaOK], scala, len(nomi), len(media))
	// "shown": immagini del bestiario mostrate ai giocatori in quel momento (stato temporaneo, non serve).
	if x := noti(st, "fog", "nebbia", "miHex", "scala", "pubNames", "pubItems", "hideBuiltin", "proj", "proiezione", "shown"); len(x) > 0 {
		r.avviso("campi non riconosciuti nello stato di «%s» (ignorati): %s", nome, strings.Join(x, ", "))
	}
	return nil
}

func numero(v any, predef int) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return predef
}

// cercaFileMedia trova un file della vecchia Gaster Bless dentro la cartella dei media e restituisce il
// percorso relativo ("Video/Borgofosco/abc.mp4"). La vecchia app a volte salvava solo il nome del file:
// in quel caso lo si cerca nelle cartelle Video e Immagini (e nelle loro sottocartelle).
func cercaFileMedia(cartella, file string) string {
	if fi, err := os.Stat(filepath.Join(cartella, filepath.FromSlash(file))); err == nil && !fi.IsDir() && strings.Contains(file, "/") {
		return file
	}
	nome := path.Base(file)
	trovato := ""
	for _, base := range []string{"Video", "Immagini"} {
		filepath.WalkDir(filepath.Join(cartella, base), func(p string, d fs.DirEntry, err error) error {
			if err != nil || trovato != "" {
				return nil
			}
			if !d.IsDir() && strings.EqualFold(d.Name(), nome) {
				if rel, err := filepath.Rel(cartella, p); err == nil {
					trovato = filepath.ToSlash(rel)
				}
				return filepath.SkipAll
			}
			return nil
		})
		if trovato != "" {
			break
		}
	}
	return trovato
}
