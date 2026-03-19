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

func (c *MemCache) Delete(key string) {
    c.mu.Lock()
    delete(c.items, key)
    c.mu.Unlock()
}

func (c *MemCache) Stop() {
    close(c.stopCh)
}

func (c *MemCache) sweeper() {
    ticker := time.NewTicker(60 * time.Second)
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
    c.mu.Lock()
    for k, e := range c.items {
        if now.After(e.expireAt) {
            delete(c.items, k)
        }
    }
    c.mu.Unlock()
}
