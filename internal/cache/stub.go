package cache

import (
	"fmt"
	"sync"
)

// Stub defines mocked cache
type Stub struct {
	cache map[string]map[string][]byte
	lock             sync.RWMutex
}

// NewStub constructor
func NewStub() (Repository, error) {
	return &Stub{cache: make(map[string]map[string][]byte)}, nil
}

// Get finds entry in cache
func (s *Stub) Get(group string, ids ...string) (map[string][]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	m := s.cache[group]
	if m == nil {
		return nil, fmt.Errorf("group %s not found", group)
	}
	res := make(map[string][]byte)
	for k, v := range m {
		matched := false
		for _, id := range ids {
			if id == k {
				matched = true
				break
			}
		}
		if matched {
			res[k] = v
		}
	}
	return res, nil
}

// GetAll finds all cache entries for the group
func (s *Stub) GetAll(group string) (map[string][]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	m := s.cache[group]
	if m == nil {
		return nil, fmt.Errorf("group %s not found", group)
	}
	return m, nil
}

// Save adds entry to cache
func (s *Stub) Save(group string, id string, value []byte) (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	m := s.cache[group]
	if m == nil {
		m = make(map[string][]byte)
	}
	m[id] = value
	s.cache[group] = m
	return
}

// Delete removes cache entry
func (s *Stub) Delete(group string, id string) (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	m := s.cache[group]
	if m == nil {
		return fmt.Errorf("group %s not found", group)
	}
	delete(m, id)
	s.cache[group] = m
	return
}
