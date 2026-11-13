package server

import (
	"database/sql"
	"errors"
	"path/filepath"

	"gasterbless/internal/config"
	"gasterbless/internal/cripto"
	"gasterbless/internal/db"
)

// ReimpostaPasswordMasterDalPC apre direttamente il database, senza la chiave dei dati: la chiave è
// protetta da DPAPI per l'account del servizio e un amministratore non la può leggere, ma per la
// password basta il database (l'hash non usa la chiave). Gaster Bless può restare accesa (SQLite in WAL).
func ReimpostaPasswordMasterDalPC(cfg config.Config, nuova string) (string, error) {
	d, err := db.Apri(filepath.Join(cfg.CartellaDati, "gasterbless.db"))
	if err != nil {
		return "", err
	}
	defer d.Close()
	return (&Server{DB: d}).ReimpostaPasswordMaster(nuova)
}

// ReimpostaPasswordMaster dà al Master una nuova password senza conoscere quella vecchia. Si usa solo
// dal comando "GasterBless.exe reimposta-master", che richiede di essere amministratori del PC (i dati di
// Gaster Bless sono leggibili solo da amministratori e servizio). Chiude tutte le sessioni del Master; il
// secondo fattore, se attivo, resta com'è. Restituisce il nome del Master.
func (s *Server) ReimpostaPasswordMaster(nuova string) (string, error) {
	var id, nome string
	err := s.DB.QueryRow(`SELECT id, nome FROM utenti WHERE ruolo = 'master' ORDER BY creato_il LIMIT 1`).Scan(&id, &nome)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("non c'è ancora un account del Master: crealo aprendo Gaster Bless dal PC")
	}
	if err != nil {
		return "", err
	}
	if err := ControllaPassword(nuova, nome, ""); err != nil {
		return "", err
	}
	if _, err := s.DB.Exec(`UPDATE utenti SET password_hash = ?, modificato_il = ? WHERE id = ?`, cripto.HashPassword(nuova), db.Ora(), id); err != nil {
		return "", err
	}
	s.DB.Exec(`DELETE FROM sessioni_accesso WHERE utente_id = ?`, id)
	s.evento("sistema", id, "password del Master reimpostata dal PC (comando da amministratore)")
	return nome, nil
}
