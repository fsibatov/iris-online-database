package main

import (
	"testing"
	"time"
)

func TestResponseCacheEvictsExpiredBeforeLiveLRU(t *testing.T) {
	for _, tc := range []struct {
		name       string
		maxEntries int
		maxBytes   int64
	}{
		{name: "entry_limit", maxEntries: 2, maxBytes: 1024},
		{name: "byte_limit", maxEntries: 3, maxBytes: 18},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := newResponseCache(tc.maxEntries, tc.maxBytes, time.Minute)
			cache.Put("a", 200, nil, []byte("12345"))
			cache.Put("b", 200, nil, []byte("12345"))
			if tc.name == "byte_limit" {
				cache.Put("x", 200, nil, []byte("12345"))
			}

			// Force an expired entry to be more recently used than live entries.
			// No sleep: expiry ordering is deterministic on every test run.
			cache.mu.Lock()
			cache.entries["a"].Value.(*cacheEntry).expiresAt = time.Now().Add(-time.Second)
			cache.lru.MoveToFront(cache.entries["a"])
			cache.mu.Unlock()

			cache.Put("c", 200, nil, []byte("12345"))
			if _, ok := cache.Get("a"); ok {
				t.Fatal("expired MRU entry was retained")
			}
			if _, ok := cache.Get("b"); !ok {
				t.Fatal("live LRU entry was evicted instead of expired entry")
			}
			if _, ok := cache.Get("c"); !ok {
				t.Fatal("new entry was not cached")
			}
			cache.mu.Lock()
			defer cache.mu.Unlock()
			if cache.bytes > cache.maxBytes || cache.lru.Len() > cache.maxEntries {
				t.Fatalf("cache bounds exceeded: bytes=%d, entries=%d", cache.bytes, cache.lru.Len())
			}
		})
	}
}
