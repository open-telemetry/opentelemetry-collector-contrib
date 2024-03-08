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
			ID:   operatorID,
			Type: operatorType,
		},
	}
}

// BasicConfig provides a basic implemention for an operator config.
type BasicConfig struct {
	operator.Identity `mapstructure:",squash"`
}

// ID will return the operator id.
func (c BasicConfig) ID() string {
	return c.Identity.ID
}

// SetID will Update the operator id.
func (c *BasicConfig) SetID(id string) {
	c.Identity.ID = id
}

// Type will return the operator type.
func (c BasicConfig) Type() string {
	return c.Identity.Type
}

// Deprecated [v0.97.0] Use NewBasicOperator instead.
func (c BasicConfig) Build(logger *zap.SugaredLogger) (BasicOperator, error) {
	if c.Identity.Type == "" {
		return BasicOperator{}, errors.NewError(
			"missing required `type` field.",
			"ensure that all operators have a uniquely defined `type` field.",
			"operator_id", c.ID(),
		)
	}

	return NewBasicOperator(c, component.TelemetrySettings{Logger: logger.Desugar()})
}

// BasicOperator provides a basic implementation of an operator.
func NewBasicOperator(c BasicConfig, set component.TelemetrySettings) (BasicOperator, error) {
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
			"operator_id", c.Identity.ID,
		)
	}

	return BasicOperator{
		OperatorID:        c.Identity.ID,
		OperatorType:      c.Identity.Type,
		SugaredLogger:     set.Logger.Sugar().With("operator_type", c.Identity.Type, "operator_id", c.Identity.ID),
		TelemetrySettings: set,
	}, nil
}

// BasicOperator provides a basic implementation of an operator.
type BasicOperator struct {
	OperatorID   string
	OperatorType string
	component.TelemetrySettings

	// Deprecated [v0.97.0] Use TelemetrySettings instead
	*zap.SugaredLogger
}

// ID will return the operator id.
func (p *BasicOperator) ID() string {
	if p.OperatorID == "" {
		return p.OperatorType
	}
	return p.OperatorID
}

// Type will return the operator type.
func (p *BasicOperator) Type() string {
	return p.OperatorType
}

// Deprecated [v0.97.0]
func (p *BasicOperator) Logger() *zap.SugaredLogger {
	return p.SugaredLogger
}

// Start will start the operator.
func (p *BasicOperator) Start(_ operator.Persister) error {
	return nil
}

// Stop will stop the operator.
func (p *BasicOperator) Stop() error {
	return nil
}
