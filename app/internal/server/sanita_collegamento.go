package server

// Collegamento tra Sanità mentale e Terrore (sanita.go e sanita_regole.go, scritti dall'altro account sul
// ramo main-ke8sy4) e questa versione: nomi del riepilogo e dello stato in gioco usati da quei file.

import (
	"bytes"
	"encoding/json"
)

// Riepilogo: nel ramo il riepilogo della scheda si chiamava così (qui è riepilogo, in personaggi.go).
type Riepilogo = riepilogo

// StatoInGioco: la parte dello stato in gioco che serve alla Sanità (le condizioni).
type StatoInGioco struct {
	Condizioni json.RawMessage `json:"condizioni"`
}

func (s *Server) statoInGioco(pg string) StatoInGioco {
	var cond string
	s.DB.QueryRow(`SELECT condizioni FROM stato_in_gioco WHERE personaggio_id = ?`, pg).Scan(&cond)
	if cond == "" {
		cond = "[]"
	}
	return StatoInGioco{Condizioni: json.RawMessage(cond)}
}

// applicaCondizioniSanita scrive le condizioni (messe o tolte dal Terrore) e avvisa scheda e tracker.
// Nel ramo era applicaInGioco, che qui esiste già con un altro compito.
func (s *Server) applicaCondizioniSanita(pg string, cond []string) {
	c, err := condizioniValide(cond)
	if err != nil {
		return
	}
	b, _ := json.Marshal(c)
	s.DB.Exec(`UPDATE stato_in_gioco SET condizioni = ? WHERE personaggio_id = ?`, string(b), pg)
	var prop string
	s.DB.QueryRow(`SELECT COALESCE(proprietario_id,'') FROM personaggi WHERE id = ?`, pg).Scan(&prop)
	s.avvisaStatoInGioco(pg, prop)
}

// tipoMedia riconosce immagini e video dal contenuto (non dal nome del file). Dal ramo main-ke8sy4 (mappa.go).
func tipoMedia(b []byte) (tipo, est string) {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "img", "jpg"
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "img", "png"
	case bytes.HasPrefix(b, []byte("GIF8")):
		return "img", "gif"
	case len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "img", "webp"
	case len(b) >= 12 && string(b[4:8]) == "ftyp":
		return "vid", "mp4" // MP4 e MOV
	case bytes.HasPrefix(b, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "vid", "webm"
	}
	return "", ""
}

var tipiContenuto = map[string]string{"jpg": "image/jpeg", "png": "image/png", "gif": "image/gif", "webp": "image/webp", "mp4": "video/mp4", "webm": "video/webm"}
