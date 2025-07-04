package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// Metric collection constants
const (
	metricsCollectionInterval = 30 * time.Second
)

type QueueMetrics struct {
	Topic              string         `json:"topic,omitempty"`
	MessagesPublished  int64          `json:"messages_published,omitempty"`   // Number of messages published
	MessagesConsumed   int64          `json:"messages_consumed,omitempty"`    // Number of messages consumed
	MessagesRetried    int64          `json:"messages_retried,omitempty"`     // Number of messages retried
	MessagesFailed     int64          `json:"messages_failed,omitempty"`      // Number of messages failed
	ProcessingLatency  *time.Duration `json:"processing_latency,omitempty"`   // Average processing latency
	LastProcessingTime *time.Time     `json:"last_processing_time,omitempty"` // time of oldest message
	QueueDepth         int64          `json:"queue_depth,omitempty"`          // Current queue depth
}

type MetricsCollector struct {
	metrics          map[string]*QueueMetrics
	closed           bool
	metricsTopicLock sync.RWMutex
	validTopics      map[string]bool
}

func newMetricsCollector(ctx context.Context) *MetricsCollector {
	collector := &MetricsCollector{
		metrics:     make(map[string]*QueueMetrics),
		validTopics: make(map[string]bool),
	}
	// Register metrics collector
	go collector.collectMetrics(ctx)
	return collector
}

// GetMetrics implements queue.Client interface
func (m *MetricsCollector) GetMetrics(_ context.Context, topic string) (*QueueMetrics, error) {
	m.metricsTopicLock.RLock()
	defer m.metricsTopicLock.RUnlock()

	metrics := &QueueMetrics{
		Topic: topic,
	}

	if counters, exists := m.metrics[topic]; exists {
		metrics.MessagesPublished = counters.MessagesPublished
		metrics.MessagesConsumed = counters.MessagesConsumed
		metrics.MessagesRetried = counters.MessagesRetried
		metrics.MessagesFailed = counters.MessagesFailed

		if counters.LastProcessingTime != nil {
			metrics.LastProcessingTime = counters.LastProcessingTime
		}
	}

	return metrics, nil
}

// collectMetrics periodically collects metrics for all topics
func (m *MetricsCollector) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(metricsCollectionInterval)
	defer ticker.Stop()

	for {
		if m.closed {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.gatherMetrics(ctx); err != nil {
				logrus.WithError(err).Error("Failed to gather metrics")
			}
		}
	}
}

// gatherMetrics collects metrics for all topics
func (m *MetricsCollector) hasTopic(topic string) bool {
	m.metricsTopicLock.RLock()
	defer m.metricsTopicLock.RUnlock()
	return m.validTopics[topic]
}

// gatherMetrics collects metrics for all topics
func (m *MetricsCollector) setTopic(topic string, valid bool) {
	m.metricsTopicLock.Lock()
	defer m.metricsTopicLock.Unlock()
	m.validTopics[topic] = valid
}

// gatherMetrics collects metrics for all topics
func (m *MetricsCollector) gatherMetrics(ctx context.Context) error {
	m.metricsTopicLock.RLock()
	defer m.metricsTopicLock.RUnlock()

	// Collect metrics for each topic
	for topic := range m.validTopics {
		metrics, err := m.getTopicMetrics(ctx, topic)
		if err != nil {
			logrus.WithError(err).WithField("Topic", topic).Error("Failed to get topic metrics")
			continue
		}
		m.metrics[topic] = metrics
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		m.logMetrics()
	}

	return nil
}

// getTopicMetrics collects metrics for a specific topic
func (m *MetricsCollector) getTopicMetrics(_ context.Context,
	topic string) (res *QueueMetrics, err error) {
	res = &QueueMetrics{
		Topic: topic,
	}

	// GetArtifact res from our internal counters
	if counters, exists := m.metrics[topic]; exists {
		res.MessagesConsumed = counters.MessagesConsumed
		res.MessagesRetried = counters.MessagesRetried
		res.MessagesFailed = counters.MessagesFailed
		res.MessagesPublished = counters.MessagesPublished
		res.QueueDepth = counters.QueueDepth

		// If we have recent processing activity, calculate processing latency
		if counters.LastProcessingTime != nil {
			latency := time.Since(*counters.LastProcessingTime)
			res.ProcessingLatency = &latency
			res.LastProcessingTime = counters.LastProcessingTime
		}
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logFields := logrus.Fields{
			"Component": "ClientPulsar",
			"Topic":     topic,
			"Published": res.MessagesPublished,
			"Consumed":  res.MessagesConsumed,
			"Retried":   res.MessagesRetried,
			"Failed":    res.MessagesFailed,
		}

		if res.ProcessingLatency != nil {
			logFields["ProcessingLatency"] = res.ProcessingLatency
		}
		if res.LastProcessingTime != nil {
			logFields["LastProcessingTime"] = res.LastProcessingTime
		}

		logrus.WithFields(logFields).Debug("Collected res")
	}

	return res, nil
}

// updateMetrics updates metrics for a topic
func (m *MetricsCollector) updateMetrics(topic string, published, consumed, retried, failed, queueDepth int64) {
	m.metricsTopicLock.Lock()
	defer m.metricsTopicLock.Unlock()

	metrics, exists := m.metrics[topic]
	if !exists {
		metrics = &QueueMetrics{
			Topic: topic,
		}
		m.metrics[topic] = metrics
	}

	if published > 0 {
		atomic.AddInt64(&metrics.MessagesPublished, published)
	}
	if consumed > 0 {
		atomic.AddInt64(&metrics.MessagesConsumed, consumed)
	}
	if retried > 0 {
		atomic.AddInt64(&metrics.MessagesRetried, retried)
	}
	if failed > 0 {
		atomic.AddInt64(&metrics.MessagesFailed, failed)
	}
	if queueDepth >= 0 {
		atomic.StoreInt64(&metrics.QueueDepth, queueDepth)
	}

	// Update last processing time for any message activity
	if consumed > 0 || retried > 0 || published > 0 {
		now := time.Now()
		metrics.LastProcessingTime = &now
		if metrics.ProcessingLatency == nil {
			var duration time.Duration = 0
			metrics.ProcessingLatency = &duration
		}
		currentLatency := metrics.ProcessingLatency
		if metrics.MessagesConsumed > 0 {
			newLatency := (*currentLatency*time.Duration(metrics.MessagesConsumed-1) +
				time.Since(now)) / time.Duration(metrics.MessagesConsumed)
			metrics.ProcessingLatency = &newLatency
		}
	}
	// Log metrics update if debug is enabled
	if logrus.IsLevelEnabled(logrus.TraceLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":          "ClientPulsar",
			"Topic":              topic,
			"Published":          metrics.MessagesPublished,
			"Consumed":           metrics.MessagesConsumed,
			"Retried":            metrics.MessagesRetried,
			"Failed":             metrics.MessagesFailed,
			"LastProcessingTime": metrics.LastProcessingTime,
		}).Debug("updated metrics")
	}
}

// logMetrics logs current metrics for debugging
func (m *MetricsCollector) logMetrics() {
	for topic, metrics := range m.metrics {
		logrus.WithFields(logrus.Fields{
			"Component":          "ClientKafka",
			"Topic":              topic,
			"Published":          metrics.MessagesPublished,
			"Consumed":           metrics.MessagesConsumed,
			"Failed":             metrics.MessagesFailed,
			"QueueDepth":         metrics.QueueDepth,
			"ProcessingLatency":  metrics.ProcessingLatency,
			"LastProcessingTime": metrics.LastProcessingTime,
		}).Debug("Kafka metrics")
	}
}

// Helper method to calculate exponential moving average
func calculateEMA(current, new float64, weight float64) float64 {
	return current*weight + new*(1-weight)
}

// Helper to calculate differences between metric snapshots
func calculateMetricsDelta(current, previous *QueueMetrics) *QueueMetrics {
	if previous == nil {
		return current
	}

	delta := &QueueMetrics{
		Topic:             current.Topic,
		MessagesPublished: current.MessagesPublished - previous.MessagesPublished,
		MessagesConsumed:  current.MessagesConsumed - previous.MessagesConsumed,
		MessagesRetried:   current.MessagesRetried - previous.MessagesRetried,
		MessagesFailed:    current.MessagesFailed - previous.MessagesFailed,
		QueueDepth:        current.QueueDepth, // Gauge value, not a delta
	}

	// Process latency is averaged, not subtracted
	if current.ProcessingLatency != nil {
		delta.ProcessingLatency = current.ProcessingLatency
	}

	// Oldest message age is taken directly, not subtracted
	if current.LastProcessingTime != nil {
		delta.LastProcessingTime = current.LastProcessingTime
	}

	return delta
}
