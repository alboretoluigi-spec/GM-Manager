package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Il file d'esempio distribuito con Gaster Bless deve essere sempre valido.
func TestFileEsempioValido(t *testing.T) {
	c, err := Leggi("../../gasterbless.conf.esempio")
	if err != nil {
		t.Fatal(err)
	}
	if c.PortaHTTPS != 7600 || c.AccessoEsterno != "casa" || c.BackupGiorni != 30 || c.CartellaDati != `C:\ProgramData\GasterBless` || c.CartellaMedia != `C:\ProgramData\GasterBless\Media` || len(c.BrokerMQTT) != 2 {
		t.Fatalf("valori letti: %+v", c)
	}
}

func TestErroriChiari(t *testing.T) {
	for _, riga := range []string{"porta_https = 99999", "accesso_esterno = internet", "chiave_inventata = 1",
		"broker_mqtt = ws://broker.esempio.it/mqtt", "broker_mqtt = https://broker.esempio.it",
		"nomi_certificato = *.ts.net", "nomi_certificato = 100.101.102.103", "nomi_certificato = pc master.ts.net",
		"nomi_certificato = localhost", "nomi_certificato = https://pc.tail1.ts.net", "nomi_certificato = -pc.tail1.ts.net"} {
		c := Predefinita()
		k, v, _ := strings.Cut(riga, "=")
		if err := c.imposta(strings.TrimSpace(k), strings.TrimSpace(v)); err == nil {
			t.Fatalf("%q deve dare errore", riga)
		}
	}
}

func TestPortaPubblica(t *testing.T) {
	scrivi := func(testo string) string {
		p := filepath.Join(t.TempDir(), "b.conf")
		os.WriteFile(p, []byte(testo), 0o600)
		return p
	}
	c, err := Leggi(scrivi("porta_pubblica = 7602\nnome_pubblico = PC-Master.tail1a2b.ts.net\n"))
	if err != nil || c.PortaPubblica != 7602 || c.NomePubblico != "pc-master.tail1a2b.ts.net" {
		t.Fatalf("%v %+v", err, c)
	}
	for _, testo := range []string{"porta_pubblica = 7602\n", "porta_pubblica = 7600\nnome_pubblico = pc.tail1.ts.net\n", "nome_pubblico = 1.2.3.4\n"} {
		if _, err := Leggi(scrivi(testo)); err == nil {
			t.Fatalf("%q deve dare errore", testo)
		}
	}
	if c, _ := Leggi(scrivi("")); c.PortaPubblica != 0 {
		t.Fatal("spenta se non indicata")
	}
}

func TestNomiCertificato(t *testing.T) {
	c := Predefinita()
	if err := c.imposta("nomi_certificato", " PC-Master.Tail1a2b.ts.net. , gasterbless.casa "); err != nil {
		t.Fatal(err)
	}
	if strings.Join(c.NomiCertificato, ",") != "pc-master.tail1a2b.ts.net,gasterbless.casa" {
		t.Fatalf("nomi: %v", c.NomiCertificato)
	}
	if err := c.imposta("nomi_certificato", ""); err != nil || len(c.NomiCertificato) != 0 {
		t.Fatal("vuoto = nessun nome in più")
	}
}

func TestApprovazioneSchede(t *testing.T) {
	c := Predefinita()
	if !c.ApprovazioneSchede {
		t.Fatal("di serie l'approvazione delle schede è attiva")
	}
	if err := c.imposta("approvazione_schede", "no"); err != nil || c.ApprovazioneSchede {
		t.Fatalf("approvazione_schede = no: %v %v", err, c.ApprovazioneSchede)
	}
	if err := c.imposta("approvazione_schede", "forse"); err == nil {
		t.Fatal("valore non valido: errore")
	}
}
