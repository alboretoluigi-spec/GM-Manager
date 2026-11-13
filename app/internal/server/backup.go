package server

// Backup cifrati con rotazione, e ripristino.
//
// Un backup è un file .bak nella cartella backup dei dati: un archivio zip (database, background
// cifrati, certificati) cifrato con AES-256-GCM con una chiave derivata dalla chiave principale dei dati.
// - Automatico ogni giorno all'ora "backup_ora"; si tengono quelli degli ultimi "backup_giorni" giorni.
// - Il ripristino si fa a programma FERMO (GasterBless.exe ripristina <file>): prima si mette da parte lo
//   stato attuale, così non si perde mai nulla.
// - La chiave principale è protetta da DPAPI (solo questo PC e questo account). Per poter ripristinare
//   su un altro PC il Master conserva il "codice di ripristino" (GasterBless.exe codice-ripristino).

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gasterbless/internal/config"
	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

const contestoBackup = "backup-v1"

var nomeBackup = regexp.MustCompile(`^gasterbless-\d{8}-\d{6}\.bak$`)

func (s *Server) rotteBackup(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/backup", api(s.apiElencoBackup))
	mux.Handle("POST /api/v1/backup", api(s.apiCreaBackup))
	mux.Handle("POST /api/v1/backup/codice", api(s.apiCodiceRipristino))
	mux.Handle("POST /api/v1/migrazione", api(s.apiMigrazione))
}

// richiediMasterLocale: Master collegato dal PC su cui gira Gaster Bless, con la password ripetuta.
func (s *Server) richiediMasterLocale(r *http.Request, password string) (*Utente, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	if !m.locale {
		return nil, errore(http.StatusForbidden, "Questa operazione si fa solo dal PC su cui gira Gaster Bless.")
	}
	if err := s.limiteControlla("conferma:" + m.ID); err != nil {
		return nil, err
	}
	var hash string
	s.DB.QueryRow(`SELECT password_hash FROM utenti WHERE id = ?`, m.ID).Scan(&hash)
	if !cripto.VerificaPassword(password, hash) {
		s.limiteFallito("conferma:" + m.ID)
		return nil, errore(http.StatusForbidden, "Password non corretta.")
	}
	s.limiteAzzera("conferma:" + m.ID)
	return m, nil
}

// apiCodiceRipristino mostra il codice di ripristino (la chiave dei dati). Solo dal PC di Gaster Bless,
// con la password del Master: il servizio Windows è l'unico che può leggere la chiave protetta.
func (s *Server) apiCodiceRipristino(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct{ Password string }
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	m, err := s.richiediMasterLocale(r, in.Password)
	if err != nil {
		return nil, err
	}
	c, err := CodiceRipristino(s.Cfg)
	if err != nil {
		return nil, err
	}
	s.evento("sistema", m.ID, "codice di ripristino visualizzato")
	return map[string]string{"codice": c}, nil
}

// apiMigrazione importa i dati della vecchia Gaster Bless (prova o vera). Solo dal PC di Gaster Bless.
func (s *Server) apiMigrazione(w http.ResponseWriter, r *http.Request) (any, error) {
	var in struct {
		Prova    bool
		Password string
	}
	if err := leggiJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Prova {
		m, err := s.richiediMaster(r)
		if err != nil {
			return nil, err
		}
		if !m.locale {
			return nil, errore(http.StatusForbidden, "Questa operazione si fa solo dal PC su cui gira Gaster Bless.")
		}
	} else if _, err := s.richiediMasterLocale(r, in.Password); err != nil {
		return nil, err
	}
	rap, err := s.Migra(r.Context(), in.Prova)
	if err != nil {
		return nil, errore(http.StatusConflict, err.Error()+"\n\n"+rap)
	}
	if !in.Prova {
		s.avvisaTuttiMappa()
	}
	return map[string]string{"rapporto": rap}, nil
}

type voceBackup struct {
	Nome       string `json:"nome"`
	Quando     int64  `json:"quando"`
	Dimensione int64  `json:"dimensione"`
}

func (s *Server) cartellaBackup() string { return filepath.Join(s.Cfg.CartellaDati, "backup") }

// ElencoBackup: i backup presenti, dal più recente.
func (s *Server) ElencoBackup() []voceBackup {
	voci, _ := os.ReadDir(s.cartellaBackup())
	out := []voceBackup{}
	for _, v := range voci {
		if !nomeBackup.MatchString(v.Name()) {
			continue
		}
		fi, err := v.Info()
		if err != nil {
			continue
		}
		out = append(out, voceBackup{Nome: v.Name(), Quando: fi.ModTime().Unix(), Dimensione: fi.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nome > out[j].Nome })
	return out
}

func (s *Server) apiElencoBackup(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := s.richiediMaster(r); err != nil {
		return nil, err
	}
	return map[string]any{"backup": s.ElencoBackup(), "giorni": s.Cfg.BackupGiorni, "ora": s.Cfg.BackupOra, "cartella": s.cartellaBackup()}, nil
}

func (s *Server) apiCreaBackup(w http.ResponseWriter, r *http.Request) (any, error) {
	m, err := s.richiediMaster(r)
	if err != nil {
		return nil, err
	}
	nome, err := s.CreaBackup(r.Context())
	if err != nil {
		return nil, err
	}
	s.evento("sistema", m.ID, "backup creato a mano")
	return map[string]string{"nome": nome}, nil
}

// CreaBackup scrive un nuovo backup cifrato e toglie quelli troppo vecchi.
func (s *Server) CreaBackup(ctx context.Context) (string, error) {
	if err := os.MkdirAll(s.cartellaBackup(), 0o700); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(s.cartellaBackup(), "tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	// Copia coerente del database anche mentre Gaster Bless è in uso.
	copiaDB := filepath.Join(tmp, "gasterbless.db")
	if _, err := s.DB.ExecContext(ctx, `VACUUM INTO ?`, copiaDB); err != nil {
		return "", fmt.Errorf("copia del database: %w", err)
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	aggiungi := func(nomeZip, percorso string) error {
		f, err := os.Open(percorso)
		if err != nil {
			return err
		}
		defer f.Close()
		w, err := z.Create(nomeZip)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		return err
	}
	if err := aggiungi("gasterbless.db", copiaDB); err != nil {
		return "", err
	}
	for _, sub := range []string{"file", "certificati"} {
		base := filepath.Join(s.Cfg.CartellaDati, sub)
		err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(s.Cfg.CartellaDati, p)
			return aggiungi(filepath.ToSlash(rel), p)
		})
		if err != nil {
			return "", err
		}
	}
	if err := z.Close(); err != nil {
		return "", err
	}
	nome := "gasterbless-" + time.Now().Format("20060102-150405") + ".bak"
	dest := filepath.Join(s.cartellaBackup(), nome)
	if err := os.WriteFile(dest+".tmp", s.Chiavi.Cifra(buf.Bytes(), contestoBackup), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(dest+".tmp", dest); err != nil {
		return "", err
	}
	s.ruotaBackup()
	s.impostaImpostazione("ultimo_backup", fmt.Sprint(time.Now().Unix()))
	return nome, nil
}

// ruotaBackup toglie i backup più vecchi di BackupGiorni (ne resta sempre almeno uno).
func (s *Server) ruotaBackup() {
	limite := time.Now().AddDate(0, 0, -s.Cfg.BackupGiorni)
	l := s.ElencoBackup()
	for i, b := range l {
		if i > 0 && time.Unix(b.Quando, 0).Before(limite) {
			os.Remove(filepath.Join(s.cartellaBackup(), b.Nome))
		}
	}
}

// backupAutomatico: chiamato ogni minuto; crea il backup del giorno all'ora indicata.
func (s *Server) backupAutomatico(ctx context.Context) {
	ora := time.Now()
	if ora.Hour() != s.Cfg.BackupOra {
		return
	}
	ultimo, _ := s.impostazione("ultimo_backup")
	var u int64
	fmt.Sscan(ultimo, &u)
	if time.Since(time.Unix(u, 0)) < 20*time.Hour {
		return
	}
	if nome, err := s.CreaBackup(ctx); err != nil {
		slog.Error("backup automatico non riuscito", "errore", err)
	} else {
		slog.Info("backup automatico", "file", nome)
	}
}

// ===== Codice di ripristino e ripristino (a programma fermo) =====

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// CodiceRipristino restituisce la chiave principale come codice leggibile (gruppi di 4 caratteri).
func CodiceRipristino(cfg config.Config) (string, error) {
	k, err := cripto.CaricaChiavePrincipale(filepath.Join(cfg.CartellaDati, "chiave", "gasterbless.chiave"), !cfg.ModalitaProva)
	if err != nil {
		return "", err
	}
	c := b32.EncodeToString(k)
	var g []string
	for i := 0; i < len(c); i += 4 {
		g = append(g, c[i:min(i+4, len(c))])
	}
	return strings.Join(g, "-"), nil
}

func chiaveDaCodice(codice string) ([]byte, error) {
	c := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(codice))
	k, err := b32.DecodeString(c)
	if err != nil || len(k) != 32 {
		return nil, errors.New("codice di ripristino non valido")
	}
	return k, nil
}

// Ripristina rimette i dati di un backup. Gaster Bless deve essere fermo. Se codice non è vuoto, la chiave
// principale viene ricreata da lì (ripristino su un altro PC) e protetta per questo PC.
// Lo stato attuale viene prima spostato in backup/prima-del-ripristino-<data>.
func Ripristina(cfg config.Config, fileBackup, codice string) (string, error) {
	percorsoChiave := filepath.Join(cfg.CartellaDati, "chiave", "gasterbless.chiave")
	var k []byte
	var err error
	if codice != "" {
		if k, err = chiaveDaCodice(codice); err != nil {
			return "", err
		}
	} else if k, err = cripto.CaricaChiavePrincipale(percorsoChiave, !cfg.ModalitaProva); err != nil {
		return "", err
	}
	ch, err := cripto.NuoveChiavi(k)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(fileBackup) && !strings.ContainsAny(fileBackup, `/\`) {
		fileBackup = filepath.Join(cfg.CartellaDati, "backup", fileBackup)
	}
	cif, err := os.ReadFile(fileBackup)
	if err != nil {
		return "", err
	}
	dati, err := ch.Decifra(cif, contestoBackup)
	if err != nil {
		return "", errors.New("il backup non si apre con questa chiave: serve il codice di ripristino del PC su cui è stato creato")
	}
	z, err := zip.NewReader(bytes.NewReader(dati), int64(len(dati)))
	if err != nil {
		return "", fmt.Errorf("backup danneggiato: %w", err)
	}
	// Lo stato attuale si mette da parte (mai cancellato).
	parte := filepath.Join(cfg.CartellaDati, "backup", "prima-del-ripristino-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(parte, 0o700); err != nil {
		return "", err
	}
	for _, n := range []string{"gasterbless.db", "gasterbless.db-wal", "gasterbless.db-shm", "file", "certificati"} {
		p := filepath.Join(cfg.CartellaDati, n)
		if _, err := os.Stat(p); err == nil {
			if err := os.Rename(p, filepath.Join(parte, n)); err != nil {
				return "", fmt.Errorf("impossibile spostare %s (Gaster Bless è ancora acceso?): %w", n, err)
			}
		}
	}
	for _, f := range z.File {
		nome := filepath.FromSlash(f.Name)
		if strings.Contains(f.Name, "..") || filepath.IsAbs(nome) ||
			!(f.Name == "gasterbless.db" || strings.HasPrefix(f.Name, "file/") || strings.HasPrefix(f.Name, "certificati/")) {
			continue
		}
		dest := filepath.Join(cfg.CartellaDati, nome)
		os.MkdirAll(filepath.Dir(dest), 0o700)
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return "", err
		}
	}
	if codice != "" { // la chiave ricreata si protegge per questo PC
		os.MkdirAll(filepath.Dir(percorsoChiave), 0o700)
		if _, err := os.Stat(percorsoChiave); err == nil {
			os.Rename(percorsoChiave, filepath.Join(parte, "gasterbless.chiave"))
		}
		if err := cripto.SalvaChiavePrincipale(percorsoChiave, k, !cfg.ModalitaProva); err != nil {
			return "", err
		}
	}
	// Verifica: il database si apre.
	d, err := db.Apri(filepath.Join(cfg.CartellaDati, "gasterbless.db"))
	if err != nil {
		return "", fmt.Errorf("il database ripristinato non si apre: %w (lo stato precedente è in %s)", err, parte)
	}
	d.Close()
	return parte, nil
}
