// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

// Deprecated: [v0.97.0] Use GlobalFactoryRegistry instead.
var DefaultRegistry = NewRegistry()

// Deprecated: [v0.97.0] Use Factories instead.
type Registry struct {
	operators map[string]func() Builder
}

// Deprecated: [v0.97.0] Use NewFactories instead.
func NewRegistry() *Registry {
	return &Registry{
		operators: make(map[string]func() Builder),
	}
}

func (r *Registry) Register(operatorType string, newBuilder func() Builder) {
	r.operators[operatorType] = newBuilder
}

func (r *Registry) Lookup(configType string) (func() Builder, bool) {
	b, ok := r.operators[configType]
	if ok {
		return b, ok
	}
	return nil, false
}

// Deprecated: [v0.97.0] Use RegisterFactory instead.
func Register(operatorType string, newBuilder func() Builder) {
	DefaultRegistry.Register(operatorType, newBuilder)
}

// Deprecated: [v0.97.0] Use LookupFactory instead.
func Lookup(configType string) (func() Builder, bool) {
	return DefaultRegistry.Lookup(configType)
}
