package types

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"testing"
	"time"
)

func Test_ShouldBuildTLSConfiguration(t *testing.T) {
	caFile, err := ioutil.TempFile(os.TempDir(), "caFile")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(caFile.Name())
	}()
	certFile, err := ioutil.TempFile(os.TempDir(), "certFile")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(certFile.Name())

	}()
	privFile, err := ioutil.TempFile(os.TempDir(), "privKey")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(privFile.Name())

	}()

	c := &KafkaConfig{
		Brokers:   []string{"a"},
		Username:  "user",
		Password:  "pass",
		Algorithm: "sha256",
		CertFile:  certFile.Name(),
		KeyFile:   privFile.Name(),
		CAFile:    caFile.Name(),
		VerifySSL: true,
		UseTLS:    true,
		UseSasl:   true,
		RetryMax:  1,
		Assignor:  "assign",
		Group:     "group",
	}
	caB, privB, err := genCert()

	err = ioutil.WriteFile(c.CertFile, caB, 0755)
	require.NoError(t, err)

	err = ioutil.WriteFile(c.CAFile, caB, 0755)
	require.NoError(t, err)

	err = ioutil.WriteFile(c.KeyFile, privB, 0755)
	require.NoError(t, err)

	_, err = c.BuildTLSConfiguration()
	require.NoError(t, err)

	c.Assignor = "roundrobin"
	_, err = c.BuildSaramaConfig(true, true)
	require.NoError(t, err)
}

func Test_ShouldValidateKafkaConfig(t *testing.T) {
	c := &KafkaConfig{
		Brokers:   []string{"a"},
		Username:  "user",
		Password:  "pass",
		Algorithm: "sha256",
		VerifySSL: true,
		UseTLS:    true,
		UseSasl:   true,
		RetryMax:  1,
		Assignor:  "assign",
		Group:     "group",
	}
	require.Error(t, c.Validate())
	require.Contains(t, c.Validate().Error(), "clientID")
	c.clientID = "123"
	require.NoError(t, c.Validate())
	c.RetryMax = 0
	c.Assignor = ""
	require.NoError(t, c.Validate())
}

func genCert() ([]byte, []byte, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"formicary"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Seattle"},
			StreetAddress: []string{""},
			PostalCode:    []string{"98101"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	caPEM := new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return nil, nil, err
	}

	caPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	if err != nil {
		return nil, nil, err
	}
	return caPEM.Bytes(), caPrivKeyPEM.Bytes(), nil
}

func newCert() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"formicary"},
			CommonName:   "Root CA",
		},
		NotBefore:             time.Now().Add(-10 * time.Second),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            2,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
}
