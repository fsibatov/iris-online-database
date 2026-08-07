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
	}
}

func (c *responseCache) Get(key string) (cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	// Entries are immutable after insertion. Returning the stored byte slice avoids
	// allocating a full response copy on every cache hit; eviction only removes the
	// cache's reference and does not invalidate a value already returned to a caller.
	return *entry, true
}

func (c *responseCache) Put(key string, status int, header map[string]string, body []byte) {
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
	if existing := c.entries[key]; existing != nil {
		c.removeElement(existing)
	}
	entry := &cacheEntry{key: strings.Clone(key), status: status, header: headerCopy, body: append([]byte(nil), body...), expiresAt: time.Now().Add(c.ttl), size: size}
	element := c.lru.PushFront(entry)
	c.entries[key] = element
	c.bytes += entry.size
	for c.lru.Len() > c.maxEntries || c.bytes > c.maxBytes {
		c.removeElement(c.lru.Back())
	}
}

func (c *responseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
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
