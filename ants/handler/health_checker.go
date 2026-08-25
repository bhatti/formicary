package handler

import (
	"context"
	"fmt"
	"sync"
	"time"

	dockerclient "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/types"
)

const (
	healthCheckInterval = 30 * time.Second
	healthCheckTimeout  = 10 * time.Second
)

// unhealthyThreshold is the number of consecutive failures before a method is marked
// unhealthy. One transient timeout does not block jobs; sustained failures do.
const unhealthyThreshold = 3

// MethodHealthChecker probes each method's backend (k8s cluster, Docker daemon, etc.)
// on a fixed interval and records results in AntRegistration.MethodHealth. Results are
// included in every heartbeat so the queen can route per-method: a KUBERNETES failure
// only blocks KUBERNETES jobs — other methods on the same ant stay available.
type MethodHealthChecker struct {
	antCfg       *ant_config.AntConfig
	registration *types.AntRegistration

	mu               sync.RWMutex
	k8sCli           kubernetes.Interface // initialized once on first k8s check
	dockerCli        *dockerclient.Client // initialized once on first docker check
	consecutiveFails map[string]int       // consecutive failure count per method

	cancel context.CancelFunc
}

// NewMethodHealthChecker creates a checker. Call Start to begin probing.
func NewMethodHealthChecker(
	antCfg *ant_config.AntConfig,
	registration *types.AntRegistration,
) *MethodHealthChecker {
	return &MethodHealthChecker{
		antCfg:           antCfg,
		registration:     registration,
		consecutiveFails: make(map[string]int),
	}
}

// Start begins the background health-check loop and runs an immediate probe.
// The loop is stopped when the provided context is cancelled or Stop() is called.
func (h *MethodHealthChecker) Start(ctx context.Context) {
	ctx, h.cancel = context.WithCancel(ctx)
	h.checkAll(ctx) // probe immediately so the first heartbeat already carries health data
	go func() {
		ticker := time.NewTicker(healthCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.checkAll(ctx)
			}
		}
	}()
	log.WithFields(log.Fields{
		"Component": "MethodHealthChecker",
		"AntID":     h.antCfg.Common.ID,
		"Methods":   h.registration.Methods,
		"Interval":  healthCheckInterval,
	}).Info("started method health checker")
}

// Stop cancels the background loop and releases any held clients.
func (h *MethodHealthChecker) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dockerCli != nil {
		_ = h.dockerCli.Close()
		h.dockerCli = nil
	}
}

// checkAll iterates over the ant's declared methods and probes each supported backend.
// A method is only marked unhealthy after unhealthyThreshold consecutive failures,
// protecting against transient API timeouts causing false-positive exclusions.
func (h *MethodHealthChecker) checkAll(ctx context.Context) {
	for _, method := range h.registration.Methods {
		var errMsg string
		switch method {
		case types.Kubernetes:
			errMsg = h.checkKubernetes(ctx)
		case types.Docker:
			errMsg = h.checkDocker(ctx)
		default:
			continue // no health probe for this method type
		}
		key := string(method)
		if errMsg != "" {
			h.consecutiveFails[key]++
			if h.consecutiveFails[key] >= unhealthyThreshold {
				h.setHealth(key, false, errMsg)
				log.WithFields(log.Fields{
					"Component":        "MethodHealthChecker",
					"AntID":            h.antCfg.Common.ID,
					"Method":           method,
					"ConsecutiveFails": h.consecutiveFails[key],
					"Error":            errMsg,
				}).Warn("method backend unhealthy — will be excluded from dispatch for this method")
			} else {
				log.WithFields(log.Fields{
					"Component":        "MethodHealthChecker",
					"AntID":            h.antCfg.Common.ID,
					"Method":           method,
					"ConsecutiveFails": h.consecutiveFails[key],
					"Threshold":        unhealthyThreshold,
					"Error":            errMsg,
				}).Debug("method probe failed (below unhealthy threshold, not yet gated)")
			}
		} else {
			if h.consecutiveFails[key] >= unhealthyThreshold {
				log.WithFields(log.Fields{
					"Component": "MethodHealthChecker",
					"AntID":     h.antCfg.Common.ID,
					"Method":    method,
				}).Info("method backend recovered — resuming dispatch")
			}
			h.consecutiveFails[key] = 0
			h.setHealth(key, true, "")
		}
	}
}

func (h *MethodHealthChecker) setHealth(method string, healthy bool, errMsg string) {
	h.registration.SetMethodHealth(method, &types.MethodHealthEntry{
		Healthy:       healthy,
		Error:         errMsg,
		LastCheckedAt: time.Now(),
	})
}

// checkKubernetes verifies the k8s API server is reachable by listing pods (limit 1).
// Returns "" when healthy, an error string when not.
func (h *MethodHealthChecker) checkKubernetes(ctx context.Context) string {
	cli, err := h.getK8sClient()
	if err != nil {
		return fmt.Sprintf("k8s client unavailable: %v", err)
	}
	tctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	if _, err := cli.CoreV1().Pods(h.antCfg.Kubernetes.Namespace).List(tctx, metav1.ListOptions{Limit: 1}); err != nil {
		return fmt.Sprintf("k8s API unreachable: %v", err)
	}
	return ""
}

// getK8sClient returns the cached k8s client, reusing the executor's client when available.
// We never call InitializeKubernetesClient here unless the executor hasn't done so yet:
// that call creates a new clientset and overwrites the shared one, breaking any in-flight
// executor requests and losing its cached TLS connections.
func (h *MethodHealthChecker) getK8sClient() (kubernetes.Interface, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.k8sCli != nil {
		return h.k8sCli, nil
	}
	// Prefer the executor's already-initialized client to avoid replacing it.
	if cli := h.antCfg.Kubernetes.GetClient(); cli != nil {
		h.k8sCli = cli
		return h.k8sCli, nil
	}
	// Executor hasn't initialized yet (rare); initialize ourselves.
	if err := h.antCfg.Kubernetes.InitializeKubernetesClient(); err != nil {
		return nil, fmt.Errorf("init k8s client: %w", err)
	}
	cli := h.antCfg.Kubernetes.GetClient()
	if cli == nil {
		return nil, fmt.Errorf("k8s client is nil after initialization")
	}
	h.k8sCli = cli
	return h.k8sCli, nil
}

// checkDocker pings the Docker daemon. Returns "" when healthy, error string otherwise.
func (h *MethodHealthChecker) checkDocker(ctx context.Context) string {
	cli, err := h.getDockerClient()
	if err != nil {
		return fmt.Sprintf("docker client unavailable: %v", err)
	}
	tctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	if _, err := cli.Ping(tctx); err != nil {
		return fmt.Sprintf("docker daemon unreachable: %v", err)
	}
	return ""
}

// getDockerClient returns the cached Docker client, creating it on the first call.
func (h *MethodHealthChecker) getDockerClient() (*dockerclient.Client, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dockerCli != nil {
		return h.dockerCli, nil
	}
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	h.dockerCli = cli
	return h.dockerCli, nil
}
