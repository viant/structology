package visitor

import "sync"

// SyncMap is a thread-safe map
type SyncMap[K comparable, V any] struct {
	m   map[K]V
	mux sync.RWMutex
}

// Get returns a value from the map
func (m *SyncMap[K, V]) Get(k K) (V, bool) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	v, ok := m.m[k]
	return v, ok
}

// Put adds a value to the map
func (m *SyncMap[K, V]) Put(k K, v V) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.m[k] = v
}

func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{m: make(map[K]V)}
}
