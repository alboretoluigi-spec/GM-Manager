// Package db apre il database SQLite di Gaster Bless e applica le migrazioni dello schema.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema/*.sql
var schemi embed.FS

// Apri apre (o crea) il database e lo porta all'ultima versione dello schema.
func Apri(percorso string) (*sql.DB, error) {
	dsn := "file:" + percorso + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite gestisce bene un solo scrittore: limitiamo le connessioni per evitare blocchi.
	d.SetMaxOpenConns(4)
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, err
	}
	if err := migra(d); err != nil {
		d.Close()
		return nil, fmt.Errorf("aggiornamento dello schema: %w", err)
	}
	return d, nil
}

func migra(d *sql.DB) error {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS versione_schema (versione INTEGER PRIMARY KEY, applicata_il INTEGER NOT NULL)`); err != nil {
		return err
	}
	var attuale int
	if err := d.QueryRow(`SELECT COALESCE(MAX(versione),0) FROM versione_schema`).Scan(&attuale); err != nil {
		return err
	}
	file, err := fs.Glob(schemi, "schema/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(file)
	for _, f := range file {
		nome := strings.TrimPrefix(f, "schema/")
		n, err := strconv.Atoi(strings.SplitN(nome, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("nome di migrazione non valido: %s", nome)
		}
		if n <= attuale {
			continue
		}
		testo, err := schemi.ReadFile(f)
		if err != nil {
			return err
		}
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(testo)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", nome, err)
		}
		if _, err := tx.Exec(`INSERT INTO versione_schema VALUES (?, ?)`, n, time.Now().Unix()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Tx esegue fn in una transazione: commit se fn restituisce nil, altrimenti rollback.
func Tx(ctx context.Context, d *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Ora restituisce l'istante corrente in secondi Unix (sostituibile nei test).
var Ora = func() int64 { return time.Now().Unix() }
