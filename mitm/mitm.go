package mitm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mhrv-go/config"
)

type CertManager struct {
	mu        sync.Mutex
	caCert    *x509.Certificate
	caKey     *rsa.PrivateKey
	certCache map[string]*tls.Certificate
}

func NewCertManager() *CertManager {
	cm := &CertManager{certCache: make(map[string]*tls.Certificate)}
	cm.loadOrCreateCA()
	return cm
}

func caDir() string {
	return filepath.Join(config.GetDocsDir(), "ca")
}

func keyFile() string {
	return filepath.Join(caDir(), "ca.key")
}

func certFile() string {
	return filepath.Join(caDir(), "ca.crt")
}

func (cm *CertManager) loadOrCreateCA() {
	os.MkdirAll(caDir(), 0700)

	if _, err := os.Stat(keyFile()); err == nil {
		keyPEM, _ := os.ReadFile(keyFile())
		block, _ := pem.Decode(keyPEM)
		key, _ := x509.ParsePKCS1PrivateKey(block.Bytes)
		cm.caKey = key

		certPEM, _ := os.ReadFile(certFile())
		block, _ = pem.Decode(certPEM)
		cert, _ := x509.ParseCertificate(block.Bytes)
		cm.caCert = cert
		return
	}

	// Generate new CA
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	cm.caKey = key

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "MihaniRelay MITM CA",
			Organization: []string{"MihaniRelay"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}

	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	cm.caCert, _ = x509.ParseCertificate(certDER)

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	os.WriteFile(keyFile(), keyPEM, 0600)
	os.WriteFile(certFile(), certPEM, 0644)
}

func (cm *CertManager) GetTLSConfig(host string) *tls.Config {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cert, ok := cm.certCache[host]; ok {
		return &tls.Config{Certificates: []tls.Certificate{*cert}}
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(1, 0, 0),
		DNSNames:  []string{host},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, cm.caCert, &cm.caKey.PublicKey, cm.caKey)
	if err != nil {
		return &tls.Config{}
	}

	cert := &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  cm.caKey,
	}
	cm.certCache[host] = cert

	return &tls.Config{Certificates: []tls.Certificate{*cert}}
}