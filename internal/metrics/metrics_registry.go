package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

// Registry keeps track of metrics
type Registry struct {
	counters map[string]*prometheus.CounterVec
	gauges   map[string]*prometheus.GaugeVec
	lock     sync.RWMutex
}

// New metrics constructor
func New() *Registry {
	return &Registry{
		counters: make(map[string]*prometheus.CounterVec),
		gauges:   make(map[string]*prometheus.GaugeVec),
	}
}

// Incr metric
func (r *Registry) Incr(id string, opts map[string]string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	counter := r.counters[id]
	if counter == nil {
		keys := make([]string, len(opts))
		i := 0
		for k := range opts {
			keys[i] = k
			i++
		}
		counter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: id,
			},
			keys,
		)
		prometheus.Register(counter)
		r.counters[id] = counter
	}
	labels := make([]string, len(opts))
	i := 0
	for _, v := range opts {
		labels[i] = v
		i++
	}
	counter.WithLabelValues(labels...).Inc()
}

// Set gauge
func (r *Registry) Set(id string, val float64, opts map[string]string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	gauge := r.gauges[id]
	if gauge == nil {
		keys := make([]string, len(opts))
		i := 0
		for k := range opts {
			keys[i] = k
			i++
		}
		gauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: id,
			},
			keys,
		)
		prometheus.MustRegister(gauge)
		r.gauges[id] = gauge
	}
	labels := make([]string, len(opts))
	i := 0
	for _, v := range opts {
		labels[i] = v
		i++
	}
	gauge.WithLabelValues(labels...).Set(val)
}
