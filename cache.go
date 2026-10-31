package main

import (
	"container/list"
	"strings"
	"sync"
	"time"
)

const (
	maxCacheKeyBytes   = 4096
	maxCacheEntryBytes = int64(512 << 10)
)

type cacheEntry struct {
	key       string
	body      []byte
	header    map[string]string
	status    int
	expiresAt time.Time
	ttl       time.Duration
	size      int64
}

type responseCache struct {
	mu            sync.Mutex
	entries       map[string]*list.Element
	lru           *list.List
	maxEntries    int
	maxBytes      int64
	maxEntryBytes int64
	ttl           time.Duration
	bytes         int64
	closed        bool
	inflight      map[string]chan struct{}
}

func newResponseCache(maxEntries int, maxBytes int64, ttl time.Duration) *responseCache {
	maxEntryBytes := maxBytes / 2
	if maxEntryBytes > maxCacheEntryBytes {
		maxEntryBytes = maxCacheEntryBytes
	}
	if maxEntryBytes <= 0 {
		maxEntryBytes = maxBytes
	}
	return &responseCache{
		entries:       make(map[string]*list.Element, maxEntries),
		lru:           list.New(),
		maxEntries:    maxEntries,
		maxBytes:      maxBytes,
		maxEntryBytes: maxEntryBytes,
		ttl:           ttl,
		inflight:      make(map[string]chan struct{}),
	}
}

func (c *responseCache) Get(key string) (cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return cacheEntry{}, false
	}
	element := c.entries[key]
	if element == nil {
		return cacheEntry{}, false
	}
	entry := element.Value.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.removeElement(element)
		return cacheEntry{}, false
	}
	c.lru.MoveToFront(element)

	return *entry, true
}

func (c *responseCache) Put(key string, status int, header map[string]string, body []byte) {
	c.PutWithTTL(key, status, header, body, c.ttl)
}

// PutWithTTL never extends the configured cache lifetime. A response's
// Cache-Control max-age may require a shorter in-process freshness period.
func (c *responseCache) PutWithTTL(key string, status int, header map[string]string, body []byte, ttl time.Duration) {
	if ttl <= 0 || ttl > c.ttl {
		return
	}
	if status != 200 || len(body) == 0 || len(key) > maxCacheKeyBytes {
		return
	}
	headerCopy := make(map[string]string, len(header))
	size := int64(len(key) + len(body))
	for name, value := range header {
		headerCopy[name] = value
		size += int64(len(name) + len(value))
	}
	if size > c.maxEntryBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	if existing := c.entries[key]; existing != nil {
		c.removeElement(existing)
	}
	entry := &cacheEntry{key: strings.Clone(key), status: status, header: headerCopy, body: append([]byte(nil), body...), expiresAt: time.Now().Add(ttl), ttl: ttl, size: size}
	element := c.lru.PushFront(entry)
	c.entries[key] = element
	c.bytes += entry.size
	if c.lru.Len() > c.maxEntries || c.bytes > c.maxBytes {
		// TTL and LRU order differ after reads. Purge expired entries first,
		// otherwise an expired MRU entry can evict a still-valid LRU entry.
		now := time.Now()
		for candidate := c.lru.Back(); candidate != nil; {
			previous := candidate.Prev()
			if !now.Before(candidate.Value.(*cacheEntry).expiresAt) {
				c.removeElement(candidate)
			}
			candidate = previous
		}
	}
	for c.lru.Len() > c.maxEntries || c.bytes > c.maxBytes {
		c.removeElement(c.lru.Back())
	}
}

func (c *responseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearLocked()
}

// Close atomically clears the cache and prevents in-flight producers from
// restoring entries after application shutdown. Clear remains reusable.
func (c *responseCache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.clearLocked()
	// Wake followers immediately on shutdown, even when a producer is still
	// running. Late producer releases must not close these channels twice.
	for key, ready := range c.inflight {
		delete(c.inflight, key)
		close(ready)
	}
}

func (c *responseCache) clearLocked() {
	c.entries = make(map[string]*list.Element, c.maxEntries)
	c.lru.Init()
	c.bytes = 0
}

func (c *responseCache) removeElement(element *list.Element) {
	if element == nil {
		return
	}
	entry := element.Value.(*cacheEntry)
	delete(c.entries, entry.key)
	c.bytes -= entry.size
	c.lru.Remove(element)
}

// beginLoad coordinates concurrent cache misses for the same representation.
// Only the leader generates a response; release always runs on leader exit.
// Waiting never holds the LRU mutex or consumes a backend execution slot.
func (c *responseCache) beginLoad(key string) (<-chan struct{}, func()) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, nil
	}
	if ready := c.inflight[key]; ready != nil {
		c.mu.Unlock()
		return ready, nil
	}
	ready := make(chan struct{})
	c.inflight[key] = ready
	c.mu.Unlock()
	return ready, func() {
		c.mu.Lock()
		if c.inflight[key] == ready {
			delete(c.inflight, key)
			close(ready)
		}
		c.mu.Unlock()
	}
}
