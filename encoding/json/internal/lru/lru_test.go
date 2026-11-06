package lru

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestGetOrCreateCoalescesAndEvicts(t *testing.T) {
	cache := New[string, *int](2)
	var calls atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); cache.GetOrCreate("a", func() *int { calls.Add(1); return new(int) }) }()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate builds", calls.Load())
	}
	a, _ := cache.Get("a")
	cache.GetOrCreate("b", func() *int { return new(int) })
	cache.Get("a")
	cache.GetOrCreate("c", func() *int { return new(int) })
	if _, ok := cache.Get("b"); ok {
		t.Fatal("LRU entry not evicted")
	}
	if value, _ := cache.Get("a"); value != a {
		t.Fatal("hot entry replaced")
	}
}
