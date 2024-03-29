// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"go.opentelemetry.io/collector/component"
)

type Factory interface {
	Type() component.Type
	NewDefaultConfig(operatorID string) component.Config
	CreateOperator(component.Config, component.TelemetrySettings) (Operator, error)
}

func NewFactory(
	t component.Type,
	newDefaultConfig func(string) component.Config,
	createOperator func(component.Config, component.TelemetrySettings) (Operator, error),
) Factory {
	return &factory{
		t:                t,
		newDefaultConfig: newDefaultConfig,
		createOperator:   createOperator,
	}
}

type factory struct {
	t                component.Type
	newDefaultConfig func(string) component.Config
	createOperator   func(component.Config, component.TelemetrySettings) (Operator, error)
}

func (f *factory) Type() component.Type {
	return f.t
}

func (f *factory) NewDefaultConfig(operatorID string) component.Config {
	return f.newDefaultConfig(operatorID)
}

func (f *factory) CreateOperator(config component.Config, set component.TelemetrySettings) (Operator, error) {
	return f.createOperator(config, set)
}

// GlobalFactoryRegistry is a global registry of operator types and their factories.
var GlobalFactoryRegistry = NewFactories()

// Factories is used to track and retrieve known operator types
type Factories struct {
	factories map[component.Type]Factory
}

// NewFactories creates a new registry
func NewFactories() *Factories {
	return &Factories{
		factories: make(map[component.Type]Factory),
	}
}

// RegisterFactory will register a function to an operator type.
// This function will return a builder for the supplied type.
func (r *Factories) RegisterFactory(f Factory) {
	r.factories[f.Type()] = f
}

// LookupFactory looks up a given operator type. Its second return value will
// be false if no builder is registered for that type.
func (r *Factories) LookupFactory(t component.Type) (Factory, bool) {
	b, ok := r.factories[t]
	if ok {
		return b, ok
	}
	return nil, false
}

// RegisterFactory will register an operator in the default registry
func RegisterFactory(f Factory) {
	GlobalFactoryRegistry.RegisterFactory(f)
}

// LookupFactory looks up a given operator type.Its second return value will
// be false if no builder is registered for that type.
func LookupFactory(t component.Type) (Factory, bool) {
	return GlobalFactoryRegistry.LookupFactory(t)
}
