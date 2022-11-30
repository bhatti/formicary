package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

// StubHTTPResponse defines stub response
type StubHTTPResponse struct {
	Filename      string
	Bytes         []byte
	Status        int
	Error         error
	sleepDuration time.Duration
}

// NewStubHTTPResponseError creates stubbed response with error
func NewStubHTTPResponseError(status int, sleep time.Duration, err error) *StubHTTPResponse {
	return &StubHTTPResponse{Status: status, sleepDuration: sleep, Error: err}
}

// NewStubHTTPResponse creates stubbed response
func NewStubHTTPResponse(status int, unk interface{}) *StubHTTPResponse {
	if unk == nil {
		return &StubHTTPResponse{Status: status}
	}
	switch unk.(type) {
	case string:
		if _, err := os.Stat(unk.(string)); err == nil {
			return &StubHTTPResponse{Status: status, Filename: unk.(string)}
		}
		return &StubHTTPResponse{Status: status, Bytes: []byte(unk.(string))}
	default:
		b, err := json.Marshal(unk)
		if err != nil {
			panic(fmt.Errorf("failed to serialize %v due to error %w", unk, err))
		}
		if _, err := os.Stat(string(b)); err == nil {
			return &StubHTTPResponse{Status: status, Filename: string(b)}
		}
		return &StubHTTPResponse{Status: status, Bytes: b}
	}
}

// StubHTTPClient implements HTTPClient for stubbed response
type StubHTTPClient struct {
	PostMapping   map[string]*StubHTTPResponse
	GetMapping    map[string]*StubHTTPResponse
	PutMapping    map[string]*StubHTTPResponse
	DeleteMapping map[string]*StubHTTPResponse
}

// NewStubHTTPClient - creates structure for HTTPClient
func NewStubHTTPClient() *StubHTTPClient {
	return &StubHTTPClient{
		PostMapping:   make(map[string]*StubHTTPResponse),
		GetMapping:    make(map[string]*StubHTTPResponse),
		PutMapping:    make(map[string]*StubHTTPResponse),
		DeleteMapping: make(map[string]*StubHTTPResponse),
	}
}

// PostJSON makes HTTP POST JSON request
func (w *StubHTTPClient) PostJSON(
	_ context.Context,
	url string,
	_ map[string]string,
	_ map[string]string,
	_ []byte) ([]byte, int, error) {
	return w.handle("POST", url, w.PostMapping)
}

// PutJSON makes HTTP POST JSON request
func (w *StubHTTPClient) PutJSON(
	_ context.Context,
	url string,
	_ map[string]string,
	_ map[string]string,
	_ []byte) ([]byte, int, error) {
	return w.handle("PUT", url, w.PutMapping)
}

// PostForm makes HTTP POST Form request
func (w *StubHTTPClient) PostForm(
	_ context.Context,
	url string,
	_ map[string]string,
	_ map[string]string) ([]byte, int, error) {
	return w.handle("POST", url, w.PostMapping)
}

// Get makes HTTP GET request
func (w *StubHTTPClient) Get(
	_ context.Context,
	url string,
	_ map[string]string,
	_ map[string]string,
) ([]byte, int, error) {
	return w.handle("GET", url, w.GetMapping)
}

// Delete makes HTTP DELETE request
func (w *StubHTTPClient) Delete(
	_ context.Context,
	url string,
	_ map[string]string,
	_ []byte) ([]byte, int, error) {
	return w.handle("DELETE", url, w.DeleteMapping)
}

func (w *StubHTTPClient) handle(
	method string,
	url string,
	mapping map[string]*StubHTTPResponse) ([]byte, int, error) {
	log.WithFields(log.Fields{"component": "stub-web", "url": url, "method": method}).Info("BEGIN")
	resp := mapping[url]
	if resp == nil {
		return nil, 404, errors.New("URL '" + url + "' not set for " + method + " mapping")
	}
	if resp.sleepDuration > 0 {
		time.Sleep(resp.sleepDuration)
	}
	if len(resp.Bytes) > 0 {
		//log.Trace("Method ", method, " url=", url, " Bytes=", string(resp.Bytes))
		return resp.Bytes, resp.Status, resp.Error
	}
	if resp.Error != nil {
		return nil, resp.Status, resp.Error
	}
	b, err := ioutil.ReadFile(resp.Filename)
	if err != nil {
		return nil, 404, fmt.Errorf("error reading file=%v for url %v due to %w", resp.Filename, url, err)
	}
	return b, resp.Status, resp.Error
}
