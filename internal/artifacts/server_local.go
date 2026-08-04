package artifacts

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	logrus "github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
)

// LocalServer manages the embedded SeaweedFS subprocess lifecycle.
// It automatically restarts the weed subprocess if it crashes.
type LocalServer struct {
	conf     *types.S3Config
	weedBin  string
	port     int

	mu       sync.Mutex
	cmd      *exec.Cmd
	logFile  *os.File
	closed   bool          // set by Close(); watchdog exits when true
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

// removeStaleLocks removes any LevelDB LOCK files left by a crashed weed process.
// LevelDB uses a LOCK file to enforce single-writer access; if weed exits uncleanly
// (e.g. pod force-restart) the lock is not released and the next startup fails with
// "resource temporarily unavailable". Removing it before launch is safe because we
// have already confirmed no weed process is listening on the target port.
func removeStaleLocks(dataDir string) {
	_ = filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Name() == "LOCK" {
			if rmErr := os.Remove(path); rmErr == nil {
				logrus.WithFields(logrus.Fields{
					"Component": "LocalServer",
					"Path":      path,
				}).Warn("seaweedfs: removed stale LevelDB LOCK file from previous run")
			}
		}
		return nil
	})
}

func startWeedProcess(conf *types.S3Config, weedBin string, port int) (*LocalServer, error) {
	// Remove any stale LevelDB lock files left by a previously crashed weed process.
	removeStaleLocks(conf.LocalDataDir)

	endpoint := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &LocalServer{
		conf:     conf,
		weedBin:  weedBin,
		port:     port,
		Endpoint: endpoint,
		ready:    make(chan struct{}),
	}
	if err := srv.launchProcess(); err != nil {
		return nil, err
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
		// Start watchdog after initial readiness
		if srv.readyErr == nil {
			go srv.watchdog()
		}
	}()
	return srv, nil
}

// launchProcess starts the weed subprocess and updates s.cmd / s.logFile under mu.
func (s *LocalServer) launchProcess() error {
	logPath := s.conf.LocalDataDir + "/weed.log"
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("seaweedfs: could not open log file %s: %w", logPath, err)
	}

	cmd := exec.Command(s.weedBin,
		"server",
		"-s3",
		fmt.Sprintf("-s3.port=%d", s.port),
		fmt.Sprintf("-dir=%s", s.conf.LocalDataDir),
		"-ip=0.0.0.0",
		"-ip.bind=0.0.0.0",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	logrus.WithFields(logrus.Fields{
		"Component": "LocalServer",
		"WeedBin":   s.weedBin,
		"Port":      s.port,
		"DataDir":   s.conf.LocalDataDir,
	}).Info("seaweedfs: starting weed subprocess")

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("seaweedfs: failed to start weed binary (%s): %w", s.weedBin, err)
	}

	s.mu.Lock()
	oldLog := s.logFile
	s.cmd = cmd
	s.logFile = logFile
	s.mu.Unlock()
	// Close the previous log file now that it has been replaced.
	if oldLog != nil {
		_ = oldLog.Close()
	}
	return nil
}

// watchdog monitors the weed subprocess and restarts it if it exits unexpectedly.
// It exits cleanly when Close() is called first (s.closed == true).
func (s *LocalServer) watchdog() {
	for {
		s.mu.Lock()
		cmd := s.cmd
		s.mu.Unlock()

		if cmd == nil || cmd.Process == nil {
			return
		}
		exitErr := cmd.Wait()

		// Close() sets s.closed before sending SIGTERM so we can distinguish
		// an intentional shutdown from a crash.
		s.mu.Lock()
		isClosed := s.closed
		s.mu.Unlock()
		if isClosed {
			return
		}

		if exitErr != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "LocalServer",
				"Endpoint":  s.Endpoint,
				"Error":     exitErr,
			}).Error("seaweedfs: weed process exited unexpectedly — restarting")
		} else {
			logrus.WithFields(logrus.Fields{
				"Component": "LocalServer",
				"Endpoint":  s.Endpoint,
			}).Warn("seaweedfs: weed process exited cleanly — restarting")
		}

		time.Sleep(2 * time.Second)
		removeStaleLocks(s.conf.LocalDataDir)

		if err := s.launchProcess(); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "LocalServer",
				"Error":     err,
			}).Error("seaweedfs: failed to restart weed process")
			return
		}

		if err := waitForPort(s.Endpoint, 90*time.Second); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "LocalServer",
				"Endpoint":  s.Endpoint,
				"Error":     err,
			}).Error("seaweedfs: restarted weed process did not become ready")
			return
		}
		logrus.WithFields(logrus.Fields{
			"Component": "LocalServer",
			"Endpoint":  s.Endpoint,
		}).Info("seaweedfs: restarted weed process is ready")
	}
}

// WaitReady blocks until the S3 port is accepting connections or ctx is cancelled.
// After the initial startup the ready channel is already closed, so this also
// probes the endpoint directly to handle the restart case.
func (s *LocalServer) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		if s.readyErr != nil {
			return s.readyErr
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	// Verify the endpoint is actually accepting connections (handles post-restart).
	conn, err := net.DialTimeout("tcp", s.Endpoint, 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return nil
	}
	// Not immediately ready — wait up to the context deadline.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			conn, err := net.DialTimeout("tcp", s.Endpoint, 500*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}

// Close signals the weed subprocess to exit and stops the watchdog.
// Safe to call multiple times; no-ops after the first call.
func (s *LocalServer) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true  // watchdog checks this before restarting
	cmd := s.cmd
	logFile := s.logFile
	s.cmd = nil
	s.logFile = nil
	s.mu.Unlock()

	defer func() {
		if logFile != nil {
			_ = logFile.Close()
		}
	}()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	return cmd.Wait()
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
