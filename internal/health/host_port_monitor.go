package health

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

// HostPortMonitor structure
type HostPortMonitor struct {
	name string
	host string
	port int
}

// NewHostPortMonitor constructor
func NewHostPortMonitor(name string, url string) (Monitorable, error) {
	host, port, err := parseURL(url)
	if err != nil {
		return nil, err
	}
	return &HostPortMonitor{
		name: name,
		host: host,
		port: port,
	}, nil
}

// Name of service
func (m *HostPortMonitor) Name() string {
	return m.name
}

// PerformHealthCheck runs health check
func (m *HostPortMonitor) PerformHealthCheck(_ context.Context) error {
	return IsNetworkHostPortAlive(net.JoinHostPort(m.host, strconv.Itoa(m.port)), m.name)
}

// IsNetworkHostPortAlive checks if host/port is reachable
func IsNetworkHostPortAlive(hostPort string, name string) error {
	conn, err := net.DialTimeout("tcp", hostPort, 1*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s for %s due to '%v'",
			hostPort, name, err)
	}
	if conn == nil {
		return fmt.Errorf("failed to connect to %s for %s",
			hostPort, name)
	}
	_ = conn.Close()
	return nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func parseURL(strURL string) (host string, port int, err error) {
	r := regexp.MustCompile(`([A-Za-z0-9\.\-]+):(\d+)`)
	tokens := r.FindStringSubmatch(strURL)
	if len(tokens) != 3 {
		var u *url.URL
		if u, err = url.Parse(strURL); err != nil {
			return "", 0, fmt.Errorf("failed to parse %s due to %v", strURL, err)
		}
		port := 80
		if u.Port() != "" {
			port, _ = strconv.Atoi(u.Port())
		} else if u.Scheme == "http" {
			port = 80
		} else if u.Scheme == "https" {
			port = 443
		}
		return u.Host, port, nil
	}
	host = tokens[1]
	port, _ = strconv.Atoi(tokens[2])
	if host == "" || port <= 0 {
		return "", 0, fmt.Errorf("failed to find valid host/port %v in strURL %s", tokens, strURL)
	}
	return
}
