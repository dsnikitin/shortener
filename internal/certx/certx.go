package certx

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

var (
	ErrCertFileNotFound = errors.New("certificate file not found")
	ErrKeyFileNotFound  = errors.New("key file not found")
	ErrCertExpired      = errors.New("certificate expired")
	ErrCertNotYetValid  = errors.New("certificate not yet valid")
)

// CheckCert проверяет сертификат.
func CheckCert(certFilePath, keyFilePath string) error {
	certBytes, err := os.ReadFile(certFilePath) // #nosec G304
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrCertFileNotFound
		}
		return errors.Wrap(err, "read certificate file")
	}

	keyBytes, err := os.ReadFile(keyFilePath) // #nosec G304
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrKeyFileNotFound
		}
		return errors.Wrap(err, "read key file")
	}

	pair, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return errors.Wrap(err, "parse X509 key pair")
	}

	now := time.Now()
	if now.Before(pair.Leaf.NotBefore) {
		return ErrCertNotYetValid
	}

	if now.After(pair.Leaf.NotAfter) {
		return ErrCertExpired
	}

	return nil
}

// GenerateSelfSignedCert генерирует самоподписанный сертификат и новый приватный ключ, если он не существует.
func GenerateSelfSignedCert(certFilePath, privateKeyFilePath string) error {
	privateKey, isNewPrivateKey, err := getOrCreatePrivateKey(privateKeyFilePath)
	if err != nil {
		return errors.Wrap(err, "get or create private key")
	}

	// создаём шаблон сертификата
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Country: []string{"RU"},
		},
		// разрешаем использование сертификата для 127.0.0.1 и ::1
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		// устанавливаем использование ключа для цифровой подписи, а также клиентской и серверной авторизации
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
	if err != nil {
		return errors.Wrap(err, "create certificate")
	}

	// кодируем сертификат и ключ в формате PEM, который используется для хранения и обмена криптографическими ключами
	var certPEM bytes.Buffer
	err = pem.Encode(&certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	if err != nil {
		return errors.Wrap(err, "encode certificate")
	}

	if err = os.MkdirAll(filepath.Dir(certFilePath), 0750); err != nil {
		return errors.Wrap(err, "make cert file dirs")
	}

	if err = os.WriteFile(certFilePath, certPEM.Bytes(), 0600); err != nil {
		return errors.Wrap(err, "write certificate file")
	}

	if isNewPrivateKey {
		var keyPEM bytes.Buffer
		err = pem.Encode(&keyPEM, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		})
		if err != nil {
			return errors.Wrap(err, "encode private key")
		}

		if err = os.MkdirAll(filepath.Dir(privateKeyFilePath), 0750); err != nil {
			return errors.Wrap(err, "make private key file dirs")
		}

		if err = os.WriteFile(privateKeyFilePath, keyPEM.Bytes(), 0600); err != nil {
			return errors.Wrap(err, "write private key file")
		}
	}

	return nil
}

// getOrCreatePrivateKey Возвращает существующий или создает новый приватный ключ.
func getOrCreatePrivateKey(privateKeyFilePath string) (*rsa.PrivateKey, bool, error) {
	keyBytes, err := os.ReadFile(privateKeyFilePath) // #nosec G304
	switch {
	case err == nil:
		keyPemBlock, _ := pem.Decode(keyBytes)
		if keyPemBlock == nil {
			return nil, false, errors.Errorf("key not found in file %s", privateKeyFilePath)
		}

		key, err := x509.ParsePKCS1PrivateKey(keyPemBlock.Bytes)
		if err != nil {
			return nil, false, errors.Wrap(err, "parse key")
		}

		return key, false, nil
	case errors.Is(err, os.ErrNotExist):
		key, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, false, errors.Wrap(err, "generate key")
		}

		return key, true, nil
	default:
		return nil, false, errors.Wrap(err, "read key file")
	}
}
