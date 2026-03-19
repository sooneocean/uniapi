package provider

import (
	"fmt"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	factories map[string]ProviderFactory
	instances map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ProviderFactory),
		instances: make(map[string]Provider),
	}
}

func (r *Registry) RegisterFactory(typeName string, factory ProviderFactory) {
	r.mu.Lock()
	r.factories[typeName] = factory
	r.mu.Unlock()
}

func (r *Registry) Create(typeName string, cfg ProviderConfig) (Provider, error) {
	r.mu.RLock()
	factory, ok := r.factories[typeName]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", typeName)
	}
	p, err := factory(cfg)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.instances[cfg.Name] = p
	r.mu.Unlock()
	return p, nil
}

func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	p, ok := r.instances[name]
	r.mu.RUnlock()
	return p, ok
}

func (r *Registry) AllModels() []Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	var models []Model
	for _, p := range r.instances {
		for _, m := range p.Models() {
			if !seen[m.ID] {
				seen[m.ID] = true
				models = append(models, m)
			}
		}
	}
	return models
}

func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var providers []Provider
	for _, p := range r.instances {
		providers = append(providers, p)
	}
	return providers
}
