package cache

import (
    "sync"
    "time"
)

type entry struct {
    value    interface{}
    expireAt time.Time
}

type MemCache struct {
    mu     sync.RWMutex
    items  map[string]entry
    stopCh chan struct{}
}

func New() *MemCache {
    c := &MemCache{
        items:  make(map[string]entry),
        stopCh: make(chan struct{}),
    }
    go c.sweeper()
    return c
}

func (c *MemCache) Set(key string, value interface{}, ttl time.Duration) {
    c.mu.Lock()
    c.items[key] = entry{value: value, expireAt: time.Now().Add(ttl)}
    c.mu.Unlock()
}

func (c *MemCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    e, ok := c.items[key]
    c.mu.RUnlock()
    if !ok {
        return nil, false
    }
    if time.Now().After(e.expireAt) {
        c.Delete(key)
        return nil, false
    }
    return e.value, true
}

// Increment atomically increments a counter without resetting its TTL.
// Returns the new count. If key doesn't exist, returns 0.
func (c *MemCache) Increment(key string) int {
    c.mu.RLock()
    e, ok := c.items[key]
    c.mu.RUnlock()

    if !ok || time.Now().After(e.expireAt) {
        return 0
    }

    c.mu.Lock()
    // Re-check under write lock (double-check pattern)
    e, ok = c.items[key]
    if !ok || time.Now().After(e.expireAt) {
        c.mu.Unlock()
        return 0
    }
    count, _ := e.value.(int)
    count++
    c.items[key] = entry{value: count, expireAt: e.expireAt}
    c.mu.Unlock()
    return count
}

func (c *MemCache) Delete(key string) {
    c.mu.Lock()
    delete(c.items, key)
    c.mu.Unlock()
}

func (c *MemCache) Stop() {
    close(c.stopCh)
}

func (c *MemCache) sweeper() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            c.evictExpired()
        case <-c.stopCh:
            return
        }
    }
}

func (c *MemCache) evictExpired() {
    now := time.Now()

    // Phase 1: collect expired keys under read lock
    c.mu.RLock()
    var expired []string
    for k, e := range c.items {
        if now.After(e.expireAt) {
            expired = append(expired, k)
        }
    }
    c.mu.RUnlock()

    if len(expired) == 0 {
        return
    }

    // Phase 2: delete under write lock (shorter hold time)
    c.mu.Lock()
    for _, k := range expired {
        // Re-check in case it was refreshed
        if e, ok := c.items[k]; ok && now.After(e.expireAt) {
            delete(c.items, k)
        }
    }
    c.mu.Unlock()
}
