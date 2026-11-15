package transport

import (
	"fmt"
	"sort"
	"sync"
)

// Meta describes a registered transport for UIs and validation.
type Meta struct {
	// Label is the human-readable display name (defaults to the registry name).
	Label string
	// AddressPlaceholder is an example address shown in the printer form.
	AddressPlaceholder string
	// Order controls display order (ascending); ties are broken by name.
	Order int
	// Available is false for stubs that only return a "not supported in this
	// build" error when used.
	Available bool
}

// Info is a registered transport's name together with its metadata.
type Info struct {
	Name string
	Meta
}

type entry struct {
	factory func() Transport
	meta    Meta
}

var (
	registryMu sync.RWMutex
	registry   = map[string]entry{}
)

// Register adds a transport factory under name. It is intended to be called
// from init() in each implementation file. Register panics if the name is
// already taken, so that conflicting registrations fail loudly at startup.
func Register(name string, factory func() Transport) {
	RegisterWithMeta(name, factory, Meta{Available: true})
}

// RegisterWithMeta is like Register but also records display metadata. An
// empty Label defaults to name. Like Register it panics on a duplicate name.
func RegisterWithMeta(name string, factory func() Transport, meta Meta) {
	if meta.Label == "" {
		meta.Label = name
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("transport: duplicate registration for %q", name))
	}
	registry[name] = entry{factory: factory, meta: meta}
}

// New returns a fresh transport registered under name, or nil if unknown.
func New(name string) Transport {
	registryMu.RLock()
	e, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil
	}
	return e.factory()
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

// Lookup returns the metadata of the transport registered under name.
func Lookup(name string) (Info, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	e, ok := registry[name]
	return Info{Name: name, Meta: e.meta}, ok
}

// List returns all registered transports ordered by Meta.Order, then name.
func List() []Info {
	registryMu.RLock()
	infos := make([]Info, 0, len(registry))
	for n, e := range registry {
		infos = append(infos, Info{Name: n, Meta: e.meta})
	}
	registryMu.RUnlock()
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Order != infos[j].Order {
			return infos[i].Order < infos[j].Order
		}
		return infos[i].Name < infos[j].Name
	})
	return infos
}
