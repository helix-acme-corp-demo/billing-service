package provider

import "fmt"

// ConstructorFunc is a factory function that creates a PaymentProvider from a
// configuration map. Each provider implementation registers one of these.
type ConstructorFunc func(cfg map[string]string) (PaymentProvider, error)

// registry holds the global set of known provider constructors keyed by name.
var registry = map[string]ConstructorFunc{}

// Register adds a named provider constructor to the global registry. It is
// typically called from an init() function in each provider implementation
// file.
func Register(name string, fn ConstructorFunc) {
	registry[name] = fn
}

// NewFromConfig looks up the provider by name in the registry and calls its
// constructor with the supplied configuration. Returns an error if the name is
// unknown.
func NewFromConfig(name string, cfg map[string]string) (PaymentProvider, error) {
	fn, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown payment provider %q; registered providers: %v", name, registeredNames())
	}
	return fn(cfg)
}

// registeredNames returns a sorted slice of all registered provider names (used
// for error messages).
func registeredNames() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}
