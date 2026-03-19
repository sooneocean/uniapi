package router

import (
    "context"
    "fmt"
    "sync"
    "sync/atomic"
    "time"

    "github.com/user/uniapi/internal/cache"
    "github.com/user/uniapi/internal/provider"
)

type Config struct {
    Strategy         string
    MaxRetries       int
    FailoverAttempts int
}

type account struct {
    id            string
    provider      provider.Provider
    maxConcurrent int
    current       int64
}

type Router struct {
    mu       sync.RWMutex
    accounts []*account
    cache    *cache.MemCache
    config   Config
    rrIndex  uint64
}

func New(c *cache.MemCache, cfg Config) *Router {
    return &Router{cache: c, config: cfg}
}

func (r *Router) AddAccount(id string, p provider.Provider, maxConcurrent int) {
    r.mu.Lock()
    r.accounts = append(r.accounts, &account{id: id, provider: p, maxConcurrent: maxConcurrent})
    r.mu.Unlock()
}

func (r *Router) Route(ctx context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
    candidates := r.findAccounts(req.Model, req.Provider)
    if len(candidates) == 0 {
        return nil, fmt.Errorf("no provider available for model: %s", req.Model)
    }
    failovers := r.config.FailoverAttempts
    if failovers < 1 { failovers = 1 }
    var lastErr error
    tried := make(map[string]bool)
    for attempt := 0; attempt < failovers && attempt < len(candidates); attempt++ {
        acc := r.selectAccount(candidates, tried)
        if acc == nil { break }
        tried[acc.id] = true
        resp, err := r.tryAccount(ctx, acc, req)
        if err == nil { return resp, nil }
        lastErr = err
    }
    return nil, fmt.Errorf("all providers failed for model %s: %w", req.Model, lastErr)
}

func (r *Router) findAccounts(model, providerName string) []*account {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var result []*account
    for _, acc := range r.accounts {
        if providerName != "" && acc.provider.Name() != providerName { continue }
        for _, m := range acc.provider.Models() {
            if m.ID == model {
                key := fmt.Sprintf("ratelimit:%s:%s", acc.id, model)
                if _, limited := r.cache.Get(key); !limited {
                    result = append(result, acc)
                }
                break
            }
        }
    }
    return result
}

func (r *Router) selectAccount(candidates []*account, tried map[string]bool) *account {
    var available []*account
    for _, acc := range candidates {
        if !tried[acc.id] && atomic.LoadInt64(&acc.current) < int64(acc.maxConcurrent) {
            available = append(available, acc)
        }
    }
    if len(available) == 0 {
        for _, acc := range candidates {
            if !tried[acc.id] { available = append(available, acc) }
        }
    }
    if len(available) == 0 { return nil }
    switch r.config.Strategy {
    case "least_used":
        best := available[0]
        for _, acc := range available[1:] {
            if atomic.LoadInt64(&acc.current) < atomic.LoadInt64(&best.current) { best = acc }
        }
        return best
    default: // round_robin
        idx := atomic.AddUint64(&r.rrIndex, 1)
        return available[idx%uint64(len(available))]
    }
}

func (r *Router) tryAccount(ctx context.Context, acc *account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
    atomic.AddInt64(&acc.current, 1)
    defer atomic.AddInt64(&acc.current, -1)
    var lastErr error
    for retry := 0; retry <= r.config.MaxRetries; retry++ {
        if retry > 0 {
            time.Sleep(time.Duration(100<<uint(retry-1)) * time.Millisecond)
        }
        resp, err := acc.provider.ChatCompletion(ctx, req)
        if err == nil { return resp, nil }
        lastErr = err
    }
    key := fmt.Sprintf("ratelimit:%s:%s", acc.id, req.Model)
    r.cache.Set(key, true, 30*time.Second)
    return nil, lastErr
}

func (r *Router) RouteStream(ctx context.Context, req *provider.ChatRequest) (provider.Stream, error) {
    candidates := r.findAccounts(req.Model, req.Provider)
    if len(candidates) == 0 {
        return nil, fmt.Errorf("no provider available for model: %s", req.Model)
    }
    failovers := r.config.FailoverAttempts
    if failovers < 1 { failovers = 1 }
    var lastErr error
    tried := make(map[string]bool)
    for attempt := 0; attempt < failovers && attempt < len(candidates); attempt++ {
        acc := r.selectAccount(candidates, tried)
        if acc == nil { break }
        tried[acc.id] = true
        atomic.AddInt64(&acc.current, 1)
        stream, err := acc.provider.ChatCompletionStream(ctx, req)
        if err == nil {
            // Decrement when stream is closed by caller; wrap to handle decrement
            return &trackedStream{Stream: stream, dec: func() { atomic.AddInt64(&acc.current, -1) }}, nil
        }
        atomic.AddInt64(&acc.current, -1)
        lastErr = err
    }
    return nil, fmt.Errorf("all providers failed for model %s: %w", req.Model, lastErr)
}

type trackedStream struct {
    provider.Stream
    dec     func()
    closed  bool
}

func (t *trackedStream) Close() error {
    if !t.closed {
        t.closed = true
        t.dec()
    }
    return t.Stream.Close()
}

func (r *Router) AllModels() []provider.Model {
    r.mu.RLock()
    defer r.mu.RUnlock()
    seen := make(map[string]bool)
    var models []provider.Model
    for _, acc := range r.accounts {
        for _, m := range acc.provider.Models() {
            if !seen[m.ID] { seen[m.ID] = true; models = append(models, m) }
        }
    }
    return models
}
