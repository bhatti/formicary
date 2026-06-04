package stats

import (
	"encoding/json"

	"plexobject.com/formicary/internal/cache"
)

const statsRedisGroup = "formicary:job_stats"

// statsBackend persists JobStats keyed by job-type key.
// Implementations are either a Redis-backed store or a no-op (in-memory only).
type statsBackend interface {
	save(key string, s *JobStats)
	loadAll() map[string]*JobStats
}

// redisStatsBackend uses the existing cache.Repository (Redis HASH) for persistence.
type redisStatsBackend struct {
	repo cache.Repository
}

func (b *redisStatsBackend) save(key string, s *JobStats) {
	// Marshal under the caller's lock to get a consistent snapshot, then
	// dispatch the network write asynchronously so the lock is not held
	// during the Redis round-trip.
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	go func() {
		_ = b.repo.Save(statsRedisGroup, key, data)
	}()
}

func (b *redisStatsBackend) loadAll() map[string]*JobStats {
	result := make(map[string]*JobStats)
	all, err := b.repo.GetAll(statsRedisGroup)
	if err != nil {
		return result
	}
	for key, data := range all {
		var s JobStats
		if json.Unmarshal(data, &s) == nil {
			result[key] = &s
		}
	}
	return result
}

// memoryStatsBackend is a no-op used when Redis is not configured.
type memoryStatsBackend struct{}

func (b *memoryStatsBackend) save(_ string, _ *JobStats)  {}
func (b *memoryStatsBackend) loadAll() map[string]*JobStats { return make(map[string]*JobStats) }
