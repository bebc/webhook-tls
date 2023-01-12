package pkg

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

var (
	serviceName    = "monitoring-operator-service"
	CAName         = "monitoring-operator-ca"
	CAOrganization = "monitoring-operator"
)

const (
	certName               = "tls.crt"
	keyName                = "tls.key"
	caCertName             = "ca.crt"
	caKeyName              = "ca.key"
	rotationCheckFrequency = 12 * time.Hour
	certValidityDuration   = 10 * 365 * 24 * time.Hour
	lookaheadInterval      = 90 * 24 * time.Hour
)

type WebhookTls struct {
	Namespace string
	CertDir   string
	ClientSet *kubernetes.Clientset
}

type KeyPairArtifacts struct {
	Cert    *x509.Certificate
	Key     *rsa.PrivateKey
	CertPEM []byte
	KeyPEM  []byte
}

func NewWebHookTls(namespace string, client *kubernetes.Clientset, dir string) *WebhookTls {
	return &WebhookTls{
		Namespace: namespace,
		CertDir:   dir,
		ClientSet: client,
	}
}

//创建根证书
func (w *WebhookTls) createCACert() (*KeyPairArtifacts, error) {
	begin := time.Now().Add(-1 * time.Hour)
	end := begin.Add(certValidityDuration)

	templ := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName:   CAName,
			Organization: []string{CAOrganization},
		},
		DNSNames: []string{
			CAName,
		},
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	//生成根证书私钥
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.Wrap(err, "generating key")
	}
	//生成根证书
	der, err := x509.CreateCertificate(rand.Reader, templ, templ, key.Public(), key)
	if err != nil {
		return nil, errors.Wrap(err, "creating certificate")
	}
	//certPEM, keyPEM, err := pemEncode(der, key)
	//if err != nil {
	//	return nil, errors.Wrap(err, "encoding PEM")
	//}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, errors.Wrap(err, "parsing certificate")
	}

	return &KeyPairArtifacts{Cert: cert, Key: key}, nil
}

//创建服务器证书
func (w *WebhookTls) createCertPEM() (*KeyPairArtifacts, error) {
	ca, err := w.createCACert()

	begin := time.Now().Add(-1 * time.Hour)
	end := begin.Add(certValidityDuration)
	DNSName := fmt.Sprintf("%s.%s.svc", serviceName, w.Namespace)

	templ := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: DNSName,
		},
		DNSNames: []string{
			DNSName,
		},
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	//创建webhook服务器私钥
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.Wrap(err, "generating key")
	}
	//创建webhook服务器证书
	der, err := x509.CreateCertificate(rand.Reader, templ, ca.Cert, key.Public(), ca.Key)
	if err != nil {
		return nil, errors.Wrap(err, "creating certificate")
	}
	certPEM, keyPEM, err := pemEncode(der, key)
	if err != nil {
		return nil, errors.Wrap(err, "encoding PEM")
	}
	return &KeyPairArtifacts{CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

func pemEncode(certificateDER []byte, key *rsa.PrivateKey) ([]byte, []byte, error) {
	certBuf := &bytes.Buffer{}
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}); err != nil {
		return nil, nil, errors.Wrap(err, "encoding cert")
	}
	keyBuf := &bytes.Buffer{}
	if err := pem.Encode(keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return nil, nil, errors.Wrap(err, "encoding key")
	}
	return certBuf.Bytes(), keyBuf.Bytes(), nil
}
