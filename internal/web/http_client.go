package web

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"plexobject.com/formicary/internal/types"

	log "github.com/sirupsen/logrus"
)

// HTTPClient defines methods for http get and post methods
type HTTPClient interface {
	Get(
		ctx context.Context,
		url string,
		headers map[string]string,
		params map[string]string,
	) ([]byte, int, error)
	PostForm(
		ctx context.Context,
		url string,
		headers map[string]string, params map[string]string) ([]byte, int, error)
	PostJSON(
		ctx context.Context,
		url string,
		headers map[string]string, params map[string]string, body []byte) ([]byte, int, error)
	PutJSON(
		ctx context.Context,
		url string,
		headers map[string]string, params map[string]string, body []byte) ([]byte, int, error)
	Delete(
		ctx context.Context,
		url string,
		headers map[string]string, body []byte) ([]byte, int, error)
}

// DefaultHTTPClient implements HTTPClient
type DefaultHTTPClient struct {
	config *types.CommonConfig
}

// New creates structure for HTTPClient
func New(config *types.CommonConfig) HTTPClient {
	return &DefaultHTTPClient{config: config}
}

// PostForm makes HTTP POST request
func (w *DefaultHTTPClient) PostForm(
	ctx context.Context,
	u string,
	headers map[string]string,
	params map[string]string,
) ([]byte, int, error) {
	log.WithFields(log.Fields{
		"Component": "DefaultHTTPClient",
		"URL":       u,
	}).Info("POST FORM BEGIN")

	data := url.Values{}
	for k, v := range params {
		data[k] = []string{v}
	}
	started := time.Now()

	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, 0, err
	}
	respBody, statusCode, err := w.execute(req, headers, nil)

	elapsed := time.Since(started).String()
	log.WithFields(log.Fields{
		"Component":  "DefaultHTTPClient",
		"URL":        u,
		"StatusCode": statusCode,
		"Elapsed":    elapsed,
		"Error":      err}).Info("POST FORM END")
	return respBody, statusCode, err

}

// PostJSON makes HTTP POST request with JSON payload
func (w *DefaultHTTPClient) PostJSON(
	ctx context.Context,
	u string,
	headers map[string]string,
	params map[string]string,
	data []byte) ([]byte, int, error) {
	log.WithFields(log.Fields{
		"Component": "DefaultHTTPClient",
		"URL":       u,
	}).Info("POST JSON BEGIN")

	started := time.Now()
	if data == nil {
		data = make([]byte, 0)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewBuffer(data))
	if err != nil {
		return nil, 0, err
	}
	respBody, statusCode, err := w.execute(req, headers, params)

	elapsed := time.Since(started).String()
	log.WithFields(log.Fields{
		"Component":  "DefaultHTTPClient",
		"URL":        u,
		"StatusCode": statusCode,
		"Elapsed":    elapsed,
		"Error":      err}).Info("POST JSON END")
	return respBody, statusCode, err

}

// PutJSON makes HTTP PUT request
func (w *DefaultHTTPClient) PutJSON(
	ctx context.Context,
	u string,
	headers map[string]string,
	params map[string]string,
	data []byte) ([]byte, int, error) {
	log.WithFields(log.Fields{
		"Component": "DefaultHTTPClient",
		"URL":       u,
	}).Info("PUT BEGIN")

	started := time.Now()
	if data == nil {
		data = make([]byte, 0)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", u, bytes.NewBuffer(data))
	if err != nil {
		return nil, 0, err
	}
	respBody, statusCode, err := w.execute(req, headers, params)

	elapsed := time.Since(started).String()
	log.WithFields(log.Fields{
		"Component":  "DefaultHTTPClient",
		"URL":        u,
		"StatusCode": statusCode,
		"Elapsed":    elapsed,
		"Error":      err}).Info("PUT END")
	return respBody, statusCode, err

}

// Delete makes HTTP DELETE request
func (w *DefaultHTTPClient) Delete(
	ctx context.Context,
	u string,
	headers map[string]string,
	body []byte) ([]byte, int, error) {
	log.WithFields(log.Fields{
		"Component": "DefaultHTTPClient",
		"URL":       u,
	}).Info("DELETE BEGIN")
	started := time.Now()

	var buf *bytes.Buffer
	if body != nil {
		buf = bytes.NewBuffer(body)
	} else {
		buf = bytes.NewBuffer(make([]byte, 0))
	}
	req, err := http.NewRequestWithContext(ctx, "DELETE", u, buf)
	if err != nil {
		return nil, 0, err
	}
	respBody, statusCode, err := w.execute(req, headers, nil)

	elapsed := time.Since(started).String()

	log.WithFields(log.Fields{
		"Component":  "DefaultHTTPClient",
		"URL":        u,
		"StatusCode": statusCode,
		"Elapsed":    elapsed,
		"Error":      err}).Info("DELETE END")
	return respBody, statusCode, err
}

// Get makes HTTP GET request
func (w *DefaultHTTPClient) Get(
	ctx context.Context,
	u string,
	headers map[string]string,
	params map[string]string,
) ([]byte, int, error) {
	log.WithFields(log.Fields{
		"Component": "DefaultHTTPClient",
		"URL":       u,
	}).Info("GET BEGIN")
	started := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, 0, err
	}
	respBody, statusCode, err := w.execute(req, headers, params)

	elapsed := time.Since(started).String()

	log.WithFields(log.Fields{
		"Component":  "DefaultHTTPClient",
		"URL":        u,
		"StatusCode": statusCode,
		"Elapsed":    elapsed,
		"Error":      err}).Info("GET END")

	return respBody, statusCode, err
}

//////////////////////////////////// PRIVATE METHODS ///////////////////////////////////////////
func (w *DefaultHTTPClient) execute(
	req *http.Request,
	headers map[string]string,
	params map[string]string,
) ([]byte, int, error) {
	if req == nil {
		return nil, 0, fmt.Errorf("request not specified")
	}
	if len(params) > 0 {
		paramVals := url.Values{}
		for k, v := range params {
			paramVals.Add(k, v)
		}
		req.URL.RawQuery = paramVals.Encode()
	}
	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	if w.config.UserAgent != "" {
		req.Header.Set("User-Agent", w.config.UserAgent)
	}

	client := httpClient(w.config)
	resp, err := client.Do(req)
	statusCode := 0
	var respBody []byte

	if resp != nil {
		defer func() {
			_ = resp.Body.Close()
		}()
		statusCode = resp.StatusCode
		respBody, _ = ioutil.ReadAll(resp.Body)
	}

	if err == nil && (statusCode < 200 || statusCode >= 300) {
		err = fmt.Errorf("request for %v failed with status %d", req.URL, statusCode)
	}

	if respBody == nil {
		respBody = make([]byte, 0)
	}
	return respBody, statusCode, err
}

func getLocalIPAddresses() []string {
	ips := make([]string, 0)
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	// handle err
	for _, i := range interfaces {
		addresses, err := i.Addrs()
		if err != nil {
			return ips
		}
		for _, addr := range addresses {
			switch v := addr.(type) {
			case *net.IPNet:
				ips = append(ips, v.IP.String())
			case *net.IPAddr:
				ips = append(ips, v.IP.String())
			}
		}
	}
	return ips
}

func getRemoteIPAddressFromURL(targetURL string) string {
	hostIP := ""
	u, err := url.Parse(targetURL)
	if err == nil {
		addr, err := net.LookupIP(u.Host)
		if err == nil {
			hostIP = ""
			for i, a := range addr {
				if i > 0 {
					hostIP = hostIP + " "
				}
				hostIP = hostIP + a.String()
			}
		}
	}
	return hostIP
}

func getProxyEnv() map[string]string {
	proxies := make(map[string]string)
	proxies["HTTP_PROXY"] = os.Getenv("HTTP_PROXY")
	proxies["HTTPS_PROXY"] = os.Getenv("HTTPS_PROXY")
	proxies["NO_PROXY"] = os.Getenv("NO_PROXY")
	return proxies
}

func httpClient(config *types.CommonConfig) *http.Client {
	if config.ProxyURL == "" {
		return &http.Client{}
	}
	proxyURL, err := url.Parse(config.ProxyURL)
	if err != nil {
		log.WithFields(log.Fields{
			"Component": "DefaultHTTPClient",
			"IP":        getRemoteIPAddressFromURL(config.ProxyURL),
			"Error":     err,
			"Proxy":     config.ProxyURL}).Warn("Failed to parse proxy header")
		return &http.Client{}
	}

	headers := make(http.Header, 0)
	headers.Set("User-Agent", config.UserAgent)

	//adding the proxy settings to the Transport object
	transport := &http.Transport{
		Proxy:              http.ProxyURL(proxyURL),
		ProxyConnectHeader: headers,
	}

	log.WithFields(log.Fields{
		"Component": "DefaultHTTPClient",
		"LocalIP":   getLocalIPAddresses(),
		"EnvProxy":  getProxyEnv(),
		"Proxy":     proxyURL}).Info("Http client using proxy")
	return &http.Client{
		Transport: transport,
	}
}
