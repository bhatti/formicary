package cache

import (
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/types"
)

// RedisRepository cache repository
type RedisRepository struct {
	pool       *redis.Pool
	ttlMinutes time.Duration
}

// New constructor
func New(config *types.RedisConfig) (Repository, error) {
	pool, err := config.GetPool()
	if err != nil {
		return nil, err
	}
	logrus.WithFields(
		logrus.Fields{
			"Component": "RedisRepository",
			"Host":      config.Host,
			"Port":      config.Port,
		}).Info("connected to Redis")
	return &RedisRepository{pool: pool, ttlMinutes: config.TTLMinutes}, nil
}

// Get reads cached data
func (r *RedisRepository) Get(group string, ids ...string) (res map[string][]byte, err error) {
	conn := r.pool.Get()
	defer func() {
		_ = conn.Close()
	}()

	res = make(map[string][]byte)
	arr, err := toArray(conn.Do("HMGET", insert(group, ids)...))
	if err != nil {
		return nil, err
	}
	for i, a := range arr {
		res[ids[i]], err = redis.Bytes(a, err)
		if err != nil {
			return nil, err
		}
	}
	return
}

// GetAll reads all properties for the group
func (r *RedisRepository) GetAll(group string) (res map[string][]byte, err error) {
	res = make(map[string][]byte)
	conn := r.pool.Get()
	defer func() {
		_ = conn.Close()
	}()
	arr, err := toArray(conn.Do("HGETALL", group))
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(arr); i += 2 {
		name, err := redis.String(arr[i], nil)
		if err != nil {
			return nil, err
		}
		value, err := redis.Bytes(arr[i+1], nil)
		if err != nil {
			return nil, err
		}
		res[name] = value
	}
	return
}

// Save updates cache entry
func (r *RedisRepository) Save(group string, id string, value []byte) (err error) {
	conn := r.pool.Get()
	defer func() {
		_ = conn.Close()
	}()

	_, err = conn.Do("HSET", group, id, value)
	if err == nil {
		_, err = conn.Do("EXPIRE", group, uint64(r.ttlMinutes*60))
	}

	return
}

// Delete removes cache entry
func (r *RedisRepository) Delete(group string, id string) (err error) {
	conn := r.pool.Get()
	defer func() {
		_ = conn.Close()
	}()

	_, err = conn.Do("HDEL", group, id)
	return
}

func toArray(i interface{}, err error) ([]interface{}, error) {
	if err != nil {
		return nil, err
	}
	switch arr := i.(type) {
	case []interface{}:
		return arr, nil
	}
	return nil, fmt.Errorf("unexpected type for %v", i)
}

func insert(group string, ids []string) (res []interface{}) {
	res = append(res, group)
	for _, id := range ids {
		res = append(res, id)
	}
	return
}
