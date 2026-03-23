package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hill/orion/internal/model"
)

const (
	CertStatusEtlSynced     = "etl_synced"
	CertStatusCertIssued    = "cert_issued"
	CertStatusMQTTPending   = "mqtt_pending"
	CertStatusMQTTConnected = "mqtt_connected"

	caValidityYears     = 10
	clientValidityYears = 5
)

// PKIService manages the root CA and issues / revokes gateway client certificates.
type PKIService struct {
	db *gorm.DB
}

// NewPKIService creates a new PKIService backed by the given DB.
func NewPKIService(db *gorm.DB) *PKIService {
	return &PKIService{db: db}
}

// GetOrCreateCA returns the active CA, creating a new self-signed one on first call.
//
// Race safety: the pki_ca table carries a UNIQUE index on the singleton column
// (always TRUE). On concurrent first-time calls from multiple replicas each
// generates a candidate CA locally, then attempts INSERT ... ON CONFLICT DO NOTHING.
// PostgreSQL guarantees exactly one INSERT succeeds; losing replicas observe
// RowsAffected == 0 and re-read the winner's row, so all callers always return
// the same single CA regardless of concurrency.
func (s *PKIService) GetOrCreateCA(ctx context.Context) (*model.PKICA, error) {
	// Fast path: CA already exists (common case after first boot).
	var ca model.PKICA
	err := s.db.WithContext(ctx).Order("created_at DESC").First(&ca).Error
	if err == nil {
		return &ca, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("query ca: %w", err)
	}

	// No CA yet — generate a candidate locally, then race to insert it.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ca key: %w", err)
	}

	serial, err := randSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Orion Platform CA",
			Organization: []string{"Orion"},
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(caValidityYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create ca cert: %w", err)
	}

	certPEM, keyPEM, err := encodeCertAndKey(certDER, caKey)
	if err != nil {
		return nil, err
	}

	ca = model.PKICA{
		ID:        uuid.New(),
		CertPEM:   certPEM,
		KeyPEM:    keyPEM,
		ExpiresAt: tmpl.NotAfter,
		CreatedAt: now,
		Singleton: true,
	}

	// INSERT ... ON CONFLICT DO NOTHING — only one replica wins.
	result := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&ca)
	if result.Error != nil {
		return nil, fmt.Errorf("store ca: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		// Another replica inserted first — fetch the canonical CA.
		if err := s.db.WithContext(ctx).Order("created_at DESC").First(&ca).Error; err != nil {
			return nil, fmt.Errorf("re-read ca after conflict: %w", err)
		}
	}

	return &ca, nil
}

// RecordRevocation writes an audit entry for a revoked client certificate.
// See model.RevokedCertSerial for the CRL enforcement TODO.
func (s *PKIService) RecordRevocation(ctx context.Context, gatewayID uuid.UUID, certSerial, reason string) error {
	rec := model.RevokedCertSerial{
		ID:         uuid.New(),
		GatewayID:  gatewayID,
		CertSerial: certSerial,
		RevokedAt:  time.Now().UTC(),
		Reason:     reason,
	}
	if err := s.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return fmt.Errorf("record revocation: %w", err)
	}
	return nil
}

// IssueClientCert generates a new client certificate for the gateway and
// advances its cert_status to cert_issued.
func (s *PKIService) IssueClientCert(ctx context.Context, gw *model.Gateway) error {
	ca, err := s.GetOrCreateCA(ctx)
	if err != nil {
		return err
	}

	caCert, caKey, err := parseCAPEM(ca.CertPEM, ca.KeyPEM)
	if err != nil {
		return err
	}

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate client key: %w", err)
	}

	serial, err := randSerial()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   gw.SerialNo,
			Organization: []string{"Orion"},
		},
		NotBefore: now,
		NotAfter:  now.AddDate(clientValidityYears, 0, 0),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("sign client cert: %w", err)
	}

	certPEM, keyPEM, err := encodeCertAndKey(certDER, clientKey)
	if err != nil {
		return err
	}

	notAfter := tmpl.NotAfter
	gw.CertStatus = CertStatusCertIssued
	gw.CertIssuedAt = &now
	gw.CertExpiresAt = &notAfter
	gw.CertSerial = serial.Text(16)
	gw.ClientCertPEM = certPEM
	gw.ClientKeyPEM = keyPEM

	return s.db.WithContext(ctx).Model(gw).Updates(map[string]any{
		"cert_status":     gw.CertStatus,
		"cert_issued_at":  gw.CertIssuedAt,
		"cert_expires_at": gw.CertExpiresAt,
		"cert_serial":     gw.CertSerial,
		"client_cert_pem": gw.ClientCertPEM,
		"client_key_pem":  gw.ClientKeyPEM,
	}).Error
}

// BuildCertZip returns the bytes of a zip archive containing:
//   - ca.crt       – the CA certificate (PEM)
//   - client.crt   – the gateway's client certificate (PEM)
//   - client.key   – the gateway's client private key (PEM)
func (s *PKIService) BuildCertZip(ctx context.Context, gw *model.Gateway) ([]byte, error) {
	if gw.ClientCertPEM == "" || gw.ClientKeyPEM == "" {
		return nil, fmt.Errorf("no certificate issued for gateway %s", gw.ID)
	}

	ca, err := s.GetOrCreateCA(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	files := []struct {
		name    string
		content string
	}{
		{"ca.crt", ca.CertPEM},
		{"client.crt", gw.ClientCertPEM},
		{"client.key", gw.ClientKeyPEM},
	}
	for _, f := range files {
		fw, err := w.Create(f.name)
		if err != nil {
			return nil, fmt.Errorf("zip create %s: %w", f.name, err)
		}
		if _, err := fw.Write([]byte(f.content)); err != nil {
			return nil, fmt.Errorf("zip write %s: %w", f.name, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("zip close: %w", err)
	}
	return buf.Bytes(), nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func randSerial() (*big.Int, error) {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, max)
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}
	return serial, nil
}

func encodeCertAndKey(certDER []byte, key *ecdsa.PrivateKey) (certPEM, keyPEM string, err error) {
	certBuf := &bytes.Buffer{}
	if err = pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return "", "", fmt.Errorf("encode cert pem: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", fmt.Errorf("marshal key: %w", err)
	}
	keyBuf := &bytes.Buffer{}
	if err = pem.Encode(keyBuf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return "", "", fmt.Errorf("encode key pem: %w", err)
	}

	return certBuf.String(), keyBuf.String(), nil
}

func parseCAPEM(certPEM, keyPEM string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, nil, fmt.Errorf("decode ca cert pem")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca cert: %w", err)
	}

	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("decode ca key pem")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca key: %w", err)
	}

	return cert, key, nil
}
