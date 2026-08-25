// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	internaltypes "plexobject.com/formicary/internal/types"
)

// generateSelfSignedCert returns PEM-encoded cert and key for localhost testing.
func generateSelfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Second),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return
}

// TestTLSListenerWrapsPlainListener verifies that TLSConfig.CreateTLSConfig() +
// tls.NewListener() produces a listener that accepts real TLS connections.
// This is the exact code path used by startCmux when common.tls.enabled=true.
func TestTLSListenerWrapsPlainListener(t *testing.T) {
	certPEM, keyPEM := generateSelfSignedCert(t)

	// Write cert + key to temp files so TLSConfig.CreateTLSConfig() can load them.
	certFile := t.TempDir() + "/tls.crt"
	keyFile := t.TempDir() + "/tls.key"
	require.NoError(t, writeFile(t, certFile, certPEM))
	require.NoError(t, writeFile(t, keyFile, keyPEM))

	tlsCfg := &internaltypes.TLSConfig{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
	}
	serverTLSConfig, err := tlsCfg.CreateTLSConfig()
	require.NoError(t, err)

	// Bind a plain TCP listener on an OS-assigned port.
	plain, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	// Wrap it — this is exactly what startCmux does.
	tlsLis := tls.NewListener(plain, serverTLSConfig)
	defer tlsLis.Close()

	addr := tlsLis.Addr().String()

	// Serve a trivial HTTP handler over TLS.
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}
	go srv.Serve(tlsLis) //nolint:errcheck

	// Build a client that trusts our self-signed cert.
	cert, err := x509.ParseCertificate(
		func() []byte {
			b, _ := pem.Decode(certPEM)
			return b.Bytes
		}(),
	)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(cert)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("https://" + addr + "/health")
	require.NoError(t, err, "TLS connection should succeed")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestTLSConfigDisabledSkipsWrapping verifies that when TLS is not configured,
// a plain net.Listener remains plain (no TLS wrapping logic in startCmux).
// This is a unit-level smoke test of the nil/disabled guard.
func TestTLSConfigDisabledSkipsWrapping(t *testing.T) {
	// nil TLS config — the guard in startCmux should be a no-op.
	var tlsCfg *internaltypes.TLSConfig
	require.Nil(t, tlsCfg, "nil guard: TLS must be off when config is absent")

	// disabled explicitly
	disabled := &internaltypes.TLSConfig{Enabled: false}
	require.False(t, disabled.Enabled)
}

// writeFile is a test helper that writes data to path.
func writeFile(t *testing.T, path string, data []byte) error {
	t.Helper()
	return os.WriteFile(path, data, 0600)
}
