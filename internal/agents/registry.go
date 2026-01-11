package agents

import (
	"fmt"
	"sync"
)

// Registry manages registered agent integrations
type Registry struct {
	mu            sync.RWMutex
	hookHandlers  map[string]HookHandler
	notifyHandlers map[string]NotifyHandler
	pullAdapters  map[string]PullAdapter
}

// globalRegistry is the default registry instance
var globalRegistry = NewRegistry()

// NewRegistry creates a new agent registry
func NewRegistry() *Registry {
	return &Registry{
		hookHandlers:   make(map[string]HookHandler),
		notifyHandlers: make(map[string]NotifyHandler),
		pullAdapters:   make(map[string]PullAdapter),
	}
}

// RegisterHookHandler registers a hook-based agent handler
func (r *Registry) RegisterHookHandler(name string, handler HookHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hookHandlers[name] = handler
}

// RegisterNotifyHandler registers a notification-based agent handler
func (r *Registry) RegisterNotifyHandler(name string, handler NotifyHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifyHandlers[name] = handler
}

// RegisterPullAdapter registers a pull-based agent adapter
func (r *Registry) RegisterPullAdapter(name string, adapter PullAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pullAdapters[name] = adapter
}

// GetHookHandler returns a hook handler by name
func (r *Registry) GetHookHandler(name string) (HookHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.hookHandlers[name]
	return h, ok
}

// GetNotifyHandler returns a notify handler by name
func (r *Registry) GetNotifyHandler(name string) (NotifyHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.notifyHandlers[name]
	return h, ok
}

// GetPullAdapter returns a pull adapter by name
func (r *Registry) GetPullAdapter(name string) (PullAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.pullAdapters[name]
	return a, ok
}

// ListHookHandlers returns all registered hook handler names
func (r *Registry) ListHookHandlers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.hookHandlers))
	for name := range r.hookHandlers {
		names = append(names, name)
	}
	return names
}

// ListNotifyHandlers returns all registered notify handler names
func (r *Registry) ListNotifyHandlers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.notifyHandlers))
	for name := range r.notifyHandlers {
		names = append(names, name)
	}
	return names
}

// ListPullAdapters returns all registered pull adapter names
func (r *Registry) ListPullAdapters() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.pullAdapters))
	for name := range r.pullAdapters {
		names = append(names, name)
	}
	return names
}

// ListAll returns info about all registered agents
func (r *Registry) ListAll() []AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var infos []AgentInfo
	for _, h := range r.hookHandlers {
		infos = append(infos, h.Info())
	}
	for _, h := range r.notifyHandlers {
		infos = append(infos, h.Info())
	}
	for _, a := range r.pullAdapters {
		infos = append(infos, a.Info())
	}
	return infos
}

// Global registry functions for convenience

// Register registers a handler/adapter with the global registry
func Register(handler interface{}) error {
	switch h := handler.(type) {
	case HookHandler:
		globalRegistry.RegisterHookHandler(h.Info().Name, h)
	case NotifyHandler:
		globalRegistry.RegisterNotifyHandler(h.Info().Name, h)
	case PullAdapter:
		globalRegistry.RegisterPullAdapter(h.Info().Name, h)
	default:
		return fmt.Errorf("unknown handler type: %T", handler)
	}
	return nil
}

// GetHook returns a hook handler from the global registry
func GetHook(name string) (HookHandler, bool) {
	return globalRegistry.GetHookHandler(name)
}

// GetNotify returns a notify handler from the global registry
func GetNotify(name string) (NotifyHandler, bool) {
	return globalRegistry.GetNotifyHandler(name)
}

// GetPull returns a pull adapter from the global registry
func GetPull(name string) (PullAdapter, bool) {
	return globalRegistry.GetPullAdapter(name)
}

// List returns all registered agent infos from the global registry
func List() []AgentInfo {
	return globalRegistry.ListAll()
}
