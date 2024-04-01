// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"go.opentelemetry.io/collector/component"
)

type Factory interface {
	Type() component.Type
	NewDefaultConfig(operatorID string) component.Config
	CreateOperator(component.TelemetrySettings, component.Config) (Operator, error)
}

func NewFactory(
	t component.Type,
	newDefaultConfig func(string) component.Config,
	createOperator func(component.TelemetrySettings, component.Config) (Operator, error),
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
	createOperator   func(component.TelemetrySettings, component.Config) (Operator, error)
}

func (f *factory) Type() component.Type {
	return f.t
}

func (f *factory) NewDefaultConfig(operatorID string) component.Config {
	return f.newDefaultConfig(operatorID)
}

func (f *factory) CreateOperator(set component.TelemetrySettings, cfg component.Config) (Operator, error) {
	return f.createOperator(set, cfg)
}
