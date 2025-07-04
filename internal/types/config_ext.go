package types

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

func configFile(dir string, filename string) (string, error) {
	if st, err := os.Stat(filename); err == nil {
		if st.IsDir() || !st.Mode().IsRegular() {
			return "", NewValidationError(fmt.Sprintf("%s is directory", filename))
		}
		return filename, nil
	}
	f := filepath.Join(dir, filename)
	if st, err := os.Stat(f); err == nil {
		if st.IsDir() || !st.Mode().IsRegular() {
			return "", NewValidationError(fmt.Sprintf("%s is directory", f))
		}
		return f, nil
	} else {
		cwd, _ := os.Getwd()
		debug.PrintStack()
		return "", NewValidationError(fmt.Sprintf(
			"failed to find '%s' config file [cwd %s] in %s due to %s", f, cwd, dir, err))
	}
}

func (c *TLSConfig) CreateTLSConfig() (tlsConfig *tls.Config, err error) {
	tlsConfig = &tls.Config{}

	if c.CertFile != "" && c.KeyFile != "" {
		tlsConfig.Certificates = make([]tls.Certificate, 1)
		tlsConfig.Certificates[0], err = tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}
	}

	if c.CaFile != "" {
		b, err := os.ReadFile(c.CaFile)
		if err != nil {
			return nil, err
		}

		ca := x509.NewCertPool()
		if !ca.AppendCertsFromPEM(b) {
			return nil, NewValidationError(
				fmt.Sprintf("failed to parse root certificate %q", c.CaFile))
		}

		if c.Server {
			tlsConfig.ClientCAs = ca
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		} else {
			tlsConfig.RootCAs = ca
		}

		tlsConfig.ServerName = c.ServerAddress
	}

	return
}

func (c *TLSConfig) BuildCaFile(dir string) (string, error) {
	return configFile(dir, c.CaFile) // ca.pem
}

func (c *TLSConfig) BuildCertFile(dir string) (string, error) {
	return configFile(dir, c.CertFile) // "server.pem"
}

func (c *TLSConfig) BuildKeyFile(dir string) (string, error) {
	return configFile(dir, c.KeyFile) // "server-key.pem")
}

// BuildTLSConfiguration builds TLS config
func (c *QueueConfig) BuildTLSConfiguration() (t *tls.Config, err error) {
	t = &tls.Config{
		InsecureSkipVerify: c.Tls.VerifySsl,
	}
	if c.Tls.CertFile != "" && c.Tls.KeyFile != "" && c.Tls.CaFile != "" {
		cert, err := tls.LoadX509KeyPair(c.Tls.CertFile, c.Tls.KeyFile)
		if err != nil {
			return nil, err
		}

		caCert, err := os.ReadFile(c.Tls.CaFile)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		t.Certificates = []tls.Certificate{cert}
		t.RootCAs = caCertPool
	}
	return t, nil
}

func isURLOpen(urlStr string, timeout time.Duration) bool {
	u, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return false
	}

	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
