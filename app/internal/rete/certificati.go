// Package rete gestisce HTTPS (autorità di certificazione locale), il filtro degli
// indirizzi (rete di casa, VPN) e le intestazioni di sicurezza.
package rete

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// Certificati gestisce l'autorità locale "Gaster Bless" e il certificato del server.
// L'autorità dura 10 anni e non cambia: i dispositivi la accettano una volta sola.
// Il certificato del server viene rifatto quando cambiano gli indirizzi del PC.
type Certificati struct {
	cartella string
	mu       sync.RWMutex
	ca       *x509.Certificate
	caKey    *ecdsa.PrivateKey
	caPEM    []byte
	server   *tls.Certificate
	nomi     string // elenco ordinato di indirizzi/nomi coperti dal certificato attuale
	extra    []string
}

// ImpostaNomiExtra aggiunge al certificato del server nomi in più (per esempio il nome Tailscale del PC).
// Vale dal prossimo Aggiorna.
func (c *Certificati) ImpostaNomiExtra(nomi []string) {
	c.mu.Lock()
	c.extra = append([]string(nil), nomi...)
	c.mu.Unlock()
}

// CaricaCertificati legge o crea l'autorità locale nella cartella indicata.
func CaricaCertificati(cartella string) (*Certificati, error) {
	if err := os.MkdirAll(cartella, 0o700); err != nil {
		return nil, err
	}
	c := &Certificati{cartella: cartella}
	caCrt := filepath.Join(cartella, "gasterbless-ca.crt")
	caKey := filepath.Join(cartella, "gasterbless-ca.key")
	if crtPEM, err := os.ReadFile(caCrt); err == nil {
		keyPEM, err := os.ReadFile(caKey)
		if err != nil {
			return nil, err
		}
		if err := c.leggiCA(crtPEM, keyPEM); err != nil {
			return nil, err
		}
	} else {
		if err := c.creaCA(caCrt, caKey); err != nil {
			return nil, err
		}
	}
	return c, c.Aggiorna()
}

func (c *Certificati) leggiCA(crtPEM, keyPEM []byte) error {
	b, _ := pem.Decode(crtPEM)
	k, _ := pem.Decode(keyPEM)
	if b == nil || k == nil {
		return errors.New("file dell'autorità locale danneggiati")
	}
	crt, err := x509.ParseCertificate(b.Bytes)
	if err != nil {
		return err
	}
	key, err := x509.ParseECPrivateKey(k.Bytes)
	if err != nil {
		return err
	}
	c.ca, c.caKey, c.caPEM = crt, key, crtPEM
	return nil
}

func (c *Certificati) creaCA(crtPath, keyPath string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          seriale(),
		Subject:               pkix.Name{CommonName: "Gaster Bless - autorità locale", Organization: []string{"Gaster Bless"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	kder, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kder})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(crtPath, crtPEM, 0o644); err != nil {
		return err
	}
	return c.leggiCA(crtPEM, keyPEM)
}

func seriale() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	return n
}

// IndirizziLocali elenca gli IPv4 del PC su interfacce attive (esclusa la loopback).
func IndirizziLocali() []net.IP {
	var out []net.IP
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLinkLocalUnicast() {
				out = append(out, n.IP.To4())
			}
		}
	}
	return out
}

// Aggiorna rifà il certificato del server se gli indirizzi del PC sono cambiati.
func (c *Certificati) Aggiorna() error {
	ips := append([]net.IP{net.IPv4(127, 0, 0, 1)}, IndirizziLocali()...)
	host, _ := os.Hostname()
	dns := []string{"localhost"}
	if host != "" {
		dns = append(dns, strings.ToLower(host), strings.ToLower(host)+".local")
	}
	c.mu.RLock()
	for _, n := range c.extra {
		if n != "" && !slices.Contains(dns, n) {
			dns = append(dns, n)
		}
	}
	c.mu.RUnlock()
	var chiavi []string
	for _, ip := range ips {
		chiavi = append(chiavi, ip.String())
	}
	chiavi = append(chiavi, dns...)
	sort.Strings(chiavi)
	firma := strings.Join(chiavi, ",")

	c.mu.RLock()
	uguale := c.server != nil && c.nomi == firma
	c.mu.RUnlock()
	if uguale {
		return nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber: seriale(),
		Subject:      pkix.Name{CommonName: "Gaster Bless", Organization: []string{"Gaster Bless"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(0, 0, 397), // limite accettato dai browser
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  ips,
		DNSNames:     dns,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.ca, &key.PublicKey, c.caKey)
	if err != nil {
		return err
	}
	cert := &tls.Certificate{Certificate: [][]byte{der, c.ca.Raw}, PrivateKey: key}
	c.mu.Lock()
	c.server, c.nomi = cert, firma
	c.mu.Unlock()
	return nil
}

// ConfigTLS restituisce la configurazione TLS del server.
func (c *Certificati) ConfigTLS() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			c.mu.RLock()
			defer c.mu.RUnlock()
			return c.server, nil
		},
	}
}

// PEMAutorita restituisce il certificato dell'autorità da installare sui dispositivi.
func (c *Certificati) PEMAutorita() []byte { return c.caPEM }

// DERAutorita restituisce il certificato dell'autorità in formato binario
// (alcuni telefoni preferiscono il file .cer/.der).
func (c *Certificati) DERAutorita() []byte { return c.ca.Raw }

// PoolAutorita serve ai test per fidarsi del certificato del server.
func (c *Certificati) PoolAutorita() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(c.ca)
	return p
}

// ImprontaAutorita mostra l'impronta SHA-256 dell'autorità, per un controllo a vista.
func (c *Certificati) ImprontaAutorita() string {
	s := fmt.Sprintf("%X", sha256Sum(c.ca.Raw))
	var b strings.Builder
	for i := 0; i < len(s); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(s[i : i+2])
	}
	return b.String()
}
