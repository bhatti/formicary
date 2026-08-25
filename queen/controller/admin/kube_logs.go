// SPDX-License-Identifier: AGPL-3.0-or-later

package admin

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// kubePodLogLine is a single parsed line from a kubernetes pod log.
type kubePodLogLine struct {
	Raw       string
	Timestamp time.Time
	Level     string // "error", "warn", "info"
}

// fetchKubeLogs fetches the last tailLines lines from the running queen pod using the
// in-cluster kubernetes API. Returns nil, nil when running outside kubernetes.
func fetchKubeLogs(ctx context.Context, tailLines int64) ([]*kubePodLogLine, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		// Not running inside a pod — return empty set silently.
		return nil, nil
	}

	cli, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	podName := os.Getenv("HOSTNAME")
	if podName == "" {
		return nil, nil
	}

	namespace := readNamespace()

	req := cli.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tailLines,
	})

	rc, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return parseKubeLogs(rc), nil
}

// parseKubeLogs converts a pod log stream into structured lines.
// Logrus text format: `time="..." level=... msg="..." ...`
// Logrus JSON format: `{"level":"...","msg":"...",...}`
// Unknown lines default to level=info.
func parseKubeLogs(r io.Reader) []*kubePodLogLine {
	var lines []*kubePodLogLine
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		raw := scanner.Text()
		if raw == "" {
			continue
		}
		level := extractLevel(raw)
		ts := extractTimestamp(raw)
		lines = append(lines, &kubePodLogLine{Raw: raw, Level: level, Timestamp: ts})
	}
	return lines
}

// filterByLevel filters log lines to only include lines at or above the given
// severity ("info" = all, "warn" = warn+error, "error" = error only).
func filterByLevel(lines []*kubePodLogLine, minLevel string) []*kubePodLogLine {
	allowed := map[string]bool{}
	switch minLevel {
	case "error":
		allowed["error"] = true
	case "warn", "warning":
		allowed["warn"] = true
		allowed["error"] = true
	default: // "info" or empty — include all
		allowed["info"] = true
		allowed["warn"] = true
		allowed["error"] = true
	}
	out := make([]*kubePodLogLine, 0, len(lines))
	for _, l := range lines {
		if allowed[l.Level] {
			out = append(out, l)
		}
	}
	return out
}

func extractLevel(raw string) string {
	r := strings.ToLower(raw)
	// Logrus text: level=error or level=warning
	for _, kv := range []string{`level=error`, `"level":"error"`, `level=err`} {
		if strings.Contains(r, kv) {
			return "error"
		}
	}
	for _, kv := range []string{`level=warn`, `"level":"warn"`} {
		if strings.Contains(r, kv) {
			return "warn"
		}
	}
	return "info"
}

func extractTimestamp(raw string) time.Time {
	// Logrus text: time="2006-01-02T15:04:05Z07:00"
	const prefix = `time="`
	if idx := strings.Index(raw, prefix); idx != -1 {
		tail := raw[idx+len(prefix):]
		if end := strings.Index(tail, `"`); end > 0 {
			if t, err := time.Parse(time.RFC3339, tail[:end]); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// readNamespace reads the pod's namespace from the service account projection.
// Falls back to "default" when not running inside a pod.
func readNamespace() string {
	const nsFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	if b, err := os.ReadFile(nsFile); err == nil {
		return strings.TrimSpace(string(b))
	}
	return "default"
}
