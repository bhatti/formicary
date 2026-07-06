package artifacts

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	logrus "github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
)

// LocalServer manages the embedded SeaweedFS subprocess lifecycle.
type LocalServer struct {
	cmd      *exec.Cmd
	logFile  *os.File
	Endpoint string        // "127.0.0.1:<port>" — the S3 endpoint to connect to
	ready    chan struct{}  // closed when S3 port is accepting connections
	readyErr error         // populated before ready is closed, if startup failed
}

// StartLocalServer starts the weed binary as a subprocess (or reuses a surviving one).
// Port selection:
//   - If conf.LocalS3Port > 0: use that fixed port. If weed is already listening there
//     (orphaned from a previous crashed run), reattach instead of spawning a new instance.
//   - Otherwise: pick a random free port and start fresh.
//
// It returns immediately; readiness is checked lazily via WaitReady.
// Weed output is written to <LocalDataDir>/weed.log instead of stderr.
func StartLocalServer(conf *types.S3Config) (*LocalServer, error) {
	weedBin := conf.LocalWeedBin
	if weedBin == "" {
		weedBin = "weed"
	}

	if err := os.MkdirAll(conf.LocalDataDir, 0755); err != nil {
		return nil, fmt.Errorf("seaweedfs: could not create data dir %s: %w", conf.LocalDataDir, err)
	}

	// Fixed port: probe first; reuse if already listening (survives server restart).
	if conf.LocalS3Port > 0 {
		endpoint := fmt.Sprintf("127.0.0.1:%d", conf.LocalS3Port)
		conn, err := net.DialTimeout("tcp", endpoint, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			logrus.WithFields(logrus.Fields{
				"Component": "LocalServer",
				"Endpoint":  endpoint,
			}).Info("seaweedfs: reusing existing weed process on fixed port")
			srv := &LocalServer{Endpoint: endpoint, ready: make(chan struct{})}
			close(srv.ready)
			return srv, nil
		}
		// Not listening yet — fall through to start weed on that port.
		return startWeedProcess(conf, weedBin, conf.LocalS3Port)
	}

	// Dynamic port.
	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("seaweedfs: could not find free port: %w", err)
	}
	return startWeedProcess(conf, weedBin, port)
}

func startWeedProcess(conf *types.S3Config, weedBin string, port int) (*LocalServer, error) {
	logPath := conf.LocalDataDir + "/weed.log"
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("seaweedfs: could not open log file %s: %w", logPath, err)
	}

	cmd := exec.Command(weedBin,
		"server",
		"-s3",
		fmt.Sprintf("-s3.port=%d", port),
		fmt.Sprintf("-dir=%s", conf.LocalDataDir),
		"-ip=0.0.0.0",
		"-ip.bind=0.0.0.0",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	logrus.WithFields(logrus.Fields{
		"Component": "LocalServer",
		"WeedBin":   weedBin,
		"Port":      port,
		"DataDir":   conf.LocalDataDir,
	}).Info("seaweedfs: starting weed subprocess")

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("seaweedfs: failed to start weed binary (%s): %w", weedBin, err)
	}

	endpoint := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &LocalServer{
		cmd:      cmd,
		logFile:  logFile,
		Endpoint: endpoint,
		ready:    make(chan struct{}),
	}

	go func() {
		srv.readyErr = waitForPort(endpoint, 90*time.Second)
		if srv.readyErr != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "LocalServer",
				"Endpoint":  endpoint,
				"Error":     srv.readyErr,
			}).Error("seaweedfs: weed process failed to become ready")
		} else {
			logrus.WithFields(logrus.Fields{
				"Component": "LocalServer",
				"Endpoint":  endpoint,
			}).Info("seaweedfs: weed process is ready")
		}
		close(srv.ready)
	}()

	return srv, nil
}

// WaitReady blocks until the S3 port is accepting connections or ctx is cancelled.
func (s *LocalServer) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return s.readyErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close sends SIGTERM to the weed process, waits for it to exit, and closes the log file.
func (s *LocalServer) Close() error {
	defer func() {
		if s.logFile != nil {
			_ = s.logFile.Close()
		}
	}()
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	return s.cmd.Wait()
}

// freePort returns a free TCP port in the range 10000–55535.
// The upper bound ensures that port+10000 (SeaweedFS gRPC offset) stays <= 65535.
func freePort() (int, error) {
	for attempts := 0; attempts < 20; attempts++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		if port >= 10000 && port <= 55535 {
			return port, nil
		}
	}
	// Fallback: pick a fixed port in the safe range and let the OS tell us if it's taken.
	for port := 19000; port <= 55000; port += 97 {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return port, nil
	}
	return 0, fmt.Errorf("could not find a free port in the safe range 10000-55535")
}

func waitForPort(endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", endpoint, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timed out after %s", timeout)
}
