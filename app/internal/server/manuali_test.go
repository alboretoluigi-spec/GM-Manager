package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
)

// pdfDiProva crea un PDF valido con n pagine, ognuna con la scritta "Pagina N".
func pdfDiProva(n int) []byte {
	t := make([][2]string, n)
	for i := range t {
		t[i][0] = fmt.Sprintf("Pagina %d", i+1)
	}
	return pdfConTesti(t)
}

// pdfConTesti: una pagina per elemento, con un titolo grande e una riga di testo piccola.
func pdfConTesti(pagine [][2]string) []byte {
	n := len(pagine)
	var b bytes.Buffer
	off := []int{}
	obj := func(s string) { off = append(off, b.Len()); fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(off), s) }
	b.WriteString("%PDF-1.4\n")
	kids := ""
	for i := 0; i < n; i++ {
		kids += fmt.Sprintf("%d 0 R ", 4+2*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids, n))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for i := 0; i < n; i++ {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 400] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+2*i))
		testo := fmt.Sprintf("BT /F1 24 Tf 40 300 Td (%s) Tj ET", pagine[i][0])
		if pagine[i][1] != "" {
			testo += fmt.Sprintf(" BT /F1 9 Tf 40 260 Td (%s) Tj ET BT /F1 9 Tf 40 245 Td (%s) Tj ET", pagine[i][1], pagine[i][1])
		}
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(testo), testo))
	}
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(off)+1)
	for _, o := range off {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(off)+1, x)
	return b.Bytes()
}

// caricaManuale invia un PDF come farebbe la pagina (corpo binario, nome nei parametri).
func (c *cliente) caricaManuale(corpo []byte, nome string) (int, map[string]any) {
	c.a.t.Helper()
	req, _ := http.NewRequest("POST", c.a.srv.URL+"/api/v1/manuali?tipo=altro&file=manuale.pdf&nome="+url.QueryEscape(nome), bytes.NewReader(corpo))
	req.Header.Set("Content-Type", "application/pdf")
	req.Header.Set("X-CSRF", c.csrf)
	if c.ip != "" {
		req.Header.Set("X-Test-IP", c.ip)
	}
	resp, err := c.c.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	b, _ := io.ReadAll(resp.Body)
	json.Unmarshal(b, &out)
	return resp.StatusCode, out
}

// scaricaParte legge il file del manuale, eventualmente solo un pezzo (Range).
func (c *cliente) scaricaParte(percorso, rng string) (int, []byte) {
	req, _ := http.NewRequest("GET", c.a.srv.URL+"/api/v1"+percorso, nil)
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	if c.ip != "" {
		req.Header.Set("X-Test-IP", c.ip)
	}
	resp, err := c.c.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestManuali(t *testing.T) {
	tv := nuovoTavolo(t)
	pdf := pdfDiProva(3)
	if code, _ := tv.luca.caricaManuale(pdf, "Furto"); code != 403 {
		t.Fatalf("solo il Master carica i manuali (%d)", code)
	}
	if code, _ := tv.m.caricaManuale([]byte("non sono un pdf"), "Falso"); code != 415 {
		t.Fatalf("i file che non sono PDF vanno rifiutati (%d)", code)
	}
	code, out := tv.m.caricaManuale(pdf, "Manuale del Giocatore")
	if code != 200 {
		t.Fatalf("caricamento: %d %v", code, out)
	}
	id := out["id"].(string)
	f := "/manuali/" + id + "/file"
	if l := tv.m.deve(200, "GET", "/manuali", nil)["lista"].([]any); len(l) != 1 {
		t.Fatal("il Master vede il manuale")
	}
	if code, b := tv.m.scaricaParte(f, ""); code != 200 || !bytes.Equal(b, pdf) {
		t.Fatal("il Master legge il PDF intero")
	}
	if code, b := tv.m.scaricaParte(f, "bytes=0-7"); code != 206 || string(b) != "%PDF-1.4" {
		t.Fatalf("lettura a pezzi per i manuali grandi: %d %q", code, b)
	}
	// Non condiviso: i giocatori non lo vedono e non possono scaricarlo.
	for _, g := range []*cliente{tv.luca, tv.sara} {
		if l := g.deve(200, "GET", "/manuali", nil)["lista"].([]any); len(l) != 0 {
			t.Fatal("un manuale non condiviso non compare ai giocatori")
		}
		if code, _ := g.scaricaParte(f, ""); code != 404 {
			t.Fatal("un manuale non condiviso non si scarica")
		}
	}
	// Condiviso solo con Luca.
	if code, _ := tv.luca.chiama("PATCH", "/manuali/"+id, map[string]any{"condivisoTutti": true}); code != 403 {
		t.Fatal("solo il Master cambia la condivisione")
	}
	tv.m.deve(200, "PATCH", "/manuali/"+id, map[string]any{"utenti": []string{tv.a.idUtente(tv.luca)}})
	if code, _ := tv.luca.scaricaParte(f, ""); code != 200 {
		t.Fatal("Luca ora può leggerlo")
	}
	if code, _ := tv.sara.scaricaParte(f, ""); code != 404 {
		t.Fatal("Sara no")
	}
	if l := tv.m.deve(200, "GET", "/manuali", nil)["lista"].([]any); len(l[0].(map[string]any)["utenti"].([]any)) != 1 {
		t.Fatal("il Master vede con chi è condiviso")
	}
	if code, _ := tv.m.chiama("PATCH", "/manuali/"+id, map[string]any{"utenti": []string{tv.a.idUtente(tv.m)}}); code != 400 {
		t.Fatal("si condivide solo con giocatori approvati")
	}
	// Con tutti, poi revoca.
	tv.m.deve(200, "PATCH", "/manuali/"+id, map[string]any{"condivisoTutti": true, "utenti": []string{}})
	if code, _ := tv.sara.scaricaParte(f, ""); code != 200 {
		t.Fatal("condiviso con tutti: anche Sara")
	}
	tv.m.deve(200, "PATCH", "/manuali/"+id, map[string]any{"condivisoTutti": false})
	for _, g := range []*cliente{tv.luca, tv.sara} {
		if code, _ := g.scaricaParte(f, ""); code != 404 {
			t.Fatal("revocata la condivisione, il file non si legge più")
		}
	}
	// Eliminazione: sparisce anche il file su disco.
	var file string
	tv.a.s.DB.QueryRow(`SELECT file FROM manuali WHERE id = ?`, id).Scan(&file)
	p, _ := tv.a.s.percorsoFileDati(file)
	tv.m.deve(200, "DELETE", "/manuali/"+id, nil)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("il file del manuale eliminato deve sparire dal disco")
	}
}
