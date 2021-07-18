package math

import (
	"sync"
	"sync/atomic"
)

// RollingAverage rolling average
type RollingAverage struct {
	samples []int64
	total   int64
	index   int
	length  int
	lock    sync.RWMutex
}

// NewRollingAverage constructor
func NewRollingAverage(capacity int) *RollingAverage {
	return &RollingAverage{
		samples: make([]int64, capacity),
	}
}

// Add latency
func (r *RollingAverage) Add(x int64) {
	if x <= 0 {
		return
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.length < len(r.samples) {
		r.length++
	}
	atomic.AddInt64(&r.total, -r.samples[r.index])
	r.samples[r.index] = x
	atomic.AddInt64(&r.total, x)
	r.index++
	if r.index >= len(r.samples) {
		r.index = 0 // cheaper than modulus
	}
}

// Average latency
func (r *RollingAverage) Average() float64 {
	size := Max(1, Min(len(r.samples), r.length))
	return float64(r.total) / float64(size)
}
