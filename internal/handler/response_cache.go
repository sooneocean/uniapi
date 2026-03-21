package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/sooneocean/uniapi/internal/cache"
)

type ResponseCache struct {
	cache   *cache.MemCache
	ttl     time.Duration
	enabled bool
}

func NewResponseCache(c *cache.MemCache, ttl time.Duration, enabled bool) *ResponseCache {
	return &ResponseCache{cache: c, ttl: ttl, enabled: enabled}
}

type CachedResponse struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
}

func (rc *ResponseCache) Key(model string, messages interface{}) string {
	data, _ := json.Marshal(map[string]interface{}{"model": model, "messages": messages})
	hash := sha256.Sum256(data)
	return "resp:" + hex.EncodeToString(hash[:])
}

func (rc *ResponseCache) Get(key string) (*CachedResponse, bool) {
	if !rc.enabled {
		return nil, false
	}
	val, ok := rc.cache.Get(key)
	if !ok {
		return nil, false
	}
	resp, ok := val.(*CachedResponse)
	return resp, ok
}

func (rc *ResponseCache) Set(key string, resp *CachedResponse) {
	if !rc.enabled {
		return
	}
	rc.cache.Set(key, resp, rc.ttl)
}
