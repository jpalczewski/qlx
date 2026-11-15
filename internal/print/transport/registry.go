package transport

import (
	"fmt"
	"sort"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]func() Transport{}
)

// Register adds a transport factory under name. It is intended to be called
// from init() in each implementation file. Register panics if the name is
// already taken, so that conflicting registrations fail loudly at startup.
func Register(name string, factory func() Transport) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("transport: duplicate registration for %q", name))
	}
	registry[name] = factory
}

// New returns a fresh transport registered under name, or nil if unknown.
func New(name string) Transport {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil
	}
	return factory()
}

// Names returns the registered transport names in sorted order.
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
