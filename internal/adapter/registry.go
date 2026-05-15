package adapter

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	adapters = map[string]Adapter{}
)

// Register adds an adapter under its Name(). Panics on duplicate, since
// registration happens at init() time and a clash is a programming error.
func Register(a Adapter) {
	mu.Lock()
	defer mu.Unlock()
	name := a.Name()
	if _, exists := adapters[name]; exists {
		panic(fmt.Sprintf("adapter %q already registered", name))
	}
	adapters[name] = a
}

func Get(name string) (Adapter, bool) {
	mu.RLock()
	defer mu.RUnlock()
	a, ok := adapters[name]
	return a, ok
}

func All() []Adapter {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Adapter, 0, len(adapters))
	for _, a := range adapters {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}
