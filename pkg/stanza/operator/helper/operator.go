// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// NewBasicConfig creates a new basic config
func NewBasicConfig(operatorID, operatorType string) BasicConfig {
	return BasicConfig{
		Identity: operator.Identity{
			Name: operatorID,
			Type: operatorType,
		},
	}
}

// BasicConfig provides a basic implemention for an operator config.
type BasicConfig struct {
	operator.Identity `mapstructure:",squash"`
}

// Deprecated [v0.98.0] Use operator.Identity instead.
func (c BasicConfig) ID() string {
	return c.Identity.Name
}

// Deprecated [v0.98.0] Use operator.Identity instead.
func (c *BasicConfig) SetID(id string) {
	c.Identity.Name = id
}

// Deprecated [v0.98.0] Use operator.Identity instead.
func (c BasicConfig) Type() string {
	return c.Identity.Type
}

// Deprecated [v0.98.0] Use NewBasicOperator instead.
func (c BasicConfig) Build(logger *zap.SugaredLogger) (BasicOperator, error) {
	if c.Identity.Type == "" {
		return BasicOperator{}, errors.NewError(
			"missing required `type` field.",
			"ensure that all operators have a uniquely defined `type` field.",
			"operator_id", c.ID(),
		)
	}

	return NewBasicOperator(component.TelemetrySettings{Logger: logger.Desugar()}, c)
}

// BasicOperator provides a basic implementation of an operator.
func NewBasicOperator(set component.TelemetrySettings, c BasicConfig) (BasicOperator, error) {
	if c.Identity.Type == "" {
		return BasicOperator{}, errors.NewError(
			"missing required `type` field.",
			"ensure that all operators have a uniquely defined `type` field.",
			"operator_id", c.ID(),
		)
	}

	if set.Logger == nil {
		return BasicOperator{}, errors.NewError(
			"operator build context is missing a logger.",
			"this is an unexpected internal error",
			"operator_type", c.Identity.Type,
			"operator_id", c.Identity.Name,
		)
	}

	return BasicOperator{
		Identity:          c.Identity,
		TelemetrySettings: set,

		// Initialized for backwards compatibility
		OperatorType:  c.Identity.Type,
		OperatorID:    c.Identity.Name,
		SugaredLogger: set.Logger.Sugar().With("operator_id", c.Identity.ComponentID()),
	}, nil
}

// BasicOperator provides a basic implementation of an operator.
type BasicOperator struct {
	operator.Identity
	component.TelemetrySettings

	// Deprecated [v0.98.0] Use Identity.Type instead
	OperatorType string

	// Deprecated [v0.98.0] Use Identity.Name instead
	OperatorID string

	// Deprecated [v0.97.0] Use TelemetrySettings instead
	*zap.SugaredLogger
}

// Deprecated [v0.98.0] Use Identity instead.
func (p *BasicOperator) ID() string {
	if p.Identity.Name != "" {
		return p.Identity.Name
	}
	return p.Identity.Type
}

// Deprecated [v0.98.0] Use Identity instead.
func (p *BasicOperator) Type() string {
	return p.Identity.Type
}

// Deprecated [v0.97.0]
func (p *BasicOperator) Logger() *zap.SugaredLogger {
	return p.TelemetrySettings.Logger.Sugar()
}

// Start will start the operator.
func (p *BasicOperator) Start(_ operator.Persister) error {
	return nil
}

// Stop will stop the operator.
func (p *BasicOperator) Stop() error {
	return nil
}
