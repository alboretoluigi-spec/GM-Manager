package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// opFinte simula le operazioni pianificate di Windows.
type opFinte struct {
	presenti map[string]bool
	avviate  []string
	errore   error
}

func (o *opFinte) Presente(nome string) bool { return o.presenti[nome] }
func (o *opFinte) Avvia(nome string) error {
	if o.errore != nil {
		return o.errore
	}
	o.avviate = append(o.avviate, nome)
	return nil
}

func TestAggiornamento(t *testing.T) {
	a := nuovoAmbiente(t)
	m := a.master()
	g := a.giocatore("Elara", "+393331112223", "192.168.1.20")
	op := &opFinte{presenti: map[string]bool{}}
	a.s.opPianificate = op

	// Solo il Master, e solo dal PC di Gaster Bless.
	g.deve(403, "GET", "/aggiornamento", nil)
	m.ip = "192.168.1.50"
	m.deve(403, "GET", "/aggiornamento", nil)
	m.deve(403, "POST", "/aggiornamento/avvia", map[string]any{})
	m.ip = ""

	// Senza preparazione una tantum: si dice cosa fare e non si avvia nulla.
	out := m.deve(200, "GET", "/aggiornamento", nil)
	if out["pronto"] != false || !strings.Contains(out["messaggio"].(string), "aggiorna.ps1") {
		t.Fatalf("senza operazione: %v", out)
	}
	m.deve(409, "POST", "/aggiornamento/avvia", map[string]any{})
	if len(op.avviate) != 0 {
		t.Fatalf("avviato senza preparazione: %v", op.avviate)
	}

	// Con le operazioni: controllo e aggiornamento partono.
	op.presenti["Aggiorna"], op.presenti["Controlla"] = true, true
	out = m.deve(200, "GET", "/aggiornamento", nil)
	if out["pronto"] != true || out["controlloPronto"] != true || out["inCorso"] != false {
		t.Fatalf("con operazione: %v", out)
	}
	m.deve(200, "POST", "/aggiornamento/controlla", map[string]any{})
	m.deve(200, "POST", "/aggiornamento/avvia", map[string]any{})
	if strings.Join(op.avviate, ",") != "Controlla,Aggiorna" {
		t.Fatalf("operazioni avviate: %v", op.avviate)
	}

	// Esiti scritti dallo script (con BOM, come può fare PowerShell) e registro.
	dir := filepath.Join(a.s.Cfg.CartellaDati, "aggiornamento")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "stato.json"), []byte("\ufeff"+`{"esito":"riuscito","novita":["abc1234  Riposi"],"registro":"C:\\ProgramData\\GasterBless\\aggiornamento\\registro-20261111-074329.txt"}`), 0o600)
	os.WriteFile(filepath.Join(dir, "controllo.json"), []byte(`{"esito":"controllo","novita":["def5678  Chat"],"registro":"..\\..\\chiave\\gasterbless.chiave"}`), 0o600)
	os.WriteFile(filepath.Join(dir, "registro-20261111-074329.txt"), []byte("\ufeffAggiornamento riuscito"), 0o600)
	out = m.deve(200, "GET", "/aggiornamento", nil)
	if out["ultimoAggiornamento"].(map[string]any)["esito"] != "riuscito" || out["ultimoControllo"].(map[string]any)["esito"] != "controllo" {
		t.Fatalf("esiti: %v", out)
	}
	reg := m.deve(200, "GET", "/aggiornamento/registro", nil)
	if reg["testo"] != "Aggiornamento riuscito" {
		t.Fatalf("registro: %v", reg)
	}
	// Un percorso fuori dalla cartella dell'aggiornamento non si legge.
	m.deve(404, "GET", "/aggiornamento/registro?di=controllo", nil)

	// Aggiornamento in corso: niente doppioni.
	os.WriteFile(filepath.Join(dir, "in_corso.lock"), []byte("123"), 0o600)
	if out = m.deve(200, "GET", "/aggiornamento", nil); out["inCorso"] != true {
		t.Fatalf("in corso: %v", out)
	}
	m.deve(409, "POST", "/aggiornamento/avvia", map[string]any{})
	os.Remove(filepath.Join(dir, "in_corso.lock"))

	// Windows rifiuta (per esempio il Master non è collegato): messaggio chiaro.
	op.errore = errors.New("ERRORE: accesso negato.")
	_, e := m.chiama("POST", "/aggiornamento/avvia", map[string]any{})
	if !strings.Contains(e["errore"].(string), "accesso negato") {
		t.Fatalf("errore di Windows: %v", e)
	}
}

func TestTestoRegistro(t *testing.T) {
	utf16 := []byte{0xFF, 0xFE, 'O', 0, 'k', 0, 0xE0, 0}
	if got := testoRegistro(utf16); got != "Okà" {
		t.Fatalf("UTF-16: %q", got)
	}
	if got := testoRegistro([]byte("\ufeffciao")); got != "ciao" {
		t.Fatalf("UTF-8 con BOM: %q", got)
	}
}
