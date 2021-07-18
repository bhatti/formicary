package utils

import (
	"sync"
)

// SafeMap map with lock
type SafeMap struct {
	mu      sync.RWMutex
	objects map[string]interface{} // map of objects
}

// NewSafeMap returns instance of SafeMap
func NewSafeMap() *SafeMap {
	return &SafeMap{
		objects: make(map[string]interface{}),
	}
}

// Len returns size of objects
func (m *SafeMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.objects)
}

// GetObject returns the object for the given key
func (m *SafeMap) GetObject(key string) interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.objects[key]
}

// SetObject sets the given object for the given key
func (m *SafeMap) SetObject(key string, obj interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = obj
}

// DeleteObject removes the object for the given key
func (m *SafeMap) DeleteObject(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
}

// Values returns all the objects in the underlying map in a slice
func (m *SafeMap) Values() []interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]interface{}, len(m.objects))
	i := 0
	for _, v := range m.objects {
		values[i] = v
		i++
	}
	return values
}

// Keys returns all keys for the map in a slice
func (m *SafeMap) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, len(m.objects))
	i := 0
	for k := range m.objects {
		keys[i] = k
		i++
	}
	return keys
}
