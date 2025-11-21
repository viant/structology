package visitor

import "sync"

// SyncMap is a thread-safe map backed by sync.Map
type SyncMap[K comparable, V any] struct {
	m sync.Map
}

// Get returns a value from the map
func (m *SyncMap[K, V]) Get(k K) (V, bool) {
	if v, ok := m.m.Load(k); ok {
		return v.(V), true
	}
	var zero V
	return zero, false
}

// Put adds a value to the map
func (m *SyncMap[K, V]) Put(k K, v V) {
	m.m.Store(k, v)
}

func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{}
}
