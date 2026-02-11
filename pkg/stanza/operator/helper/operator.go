// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/stanzaerrors"
)

// NewBasicConfig creates a new basic config
func NewBasicConfig(operatorID, operatorType string) BasicConfig {
	return BasicConfig{
		OperatorID:   operatorID,
		OperatorType: operatorType,
	}
}

// BasicConfig provides a basic implemention for an operator config.
type BasicConfig struct {
	OperatorID   string `mapstructure:"id"`
	OperatorType string `mapstructure:"type"`
}

// ID will return the operator id.
func (c BasicConfig) ID() string {
	if c.OperatorID == "" {
		return c.OperatorType
	}
	return c.OperatorID
}

// SetID will Update the operator id.
func (c *BasicConfig) SetID(id string) {
	c.OperatorID = id
}

// Type will return the operator type.
func (c BasicConfig) Type() string {
	return c.OperatorType
}

// Build will build a basic operator.
func (c BasicConfig) Build(set component.TelemetrySettings) (BasicOperator, error) {
	if c.OperatorType == "" {
		return BasicOperator{}, stanzaerrors.NewError(
			"missing required `type` field.",
			"ensure that all operators have a uniquely defined `type` field.",
			"operator_id", c.ID(),
		)
	}

	if set.Logger == nil {
		return BasicOperator{}, stanzaerrors.NewError(
			"operator build context is missing a logger.",
			"this is an unexpected internal error",
			"operator_id", c.ID(),
			"operator_type", c.Type(),
		)
	}

	set.Logger = set.Logger.With(zap.String("operator_id", c.ID()), zap.String("operator_type", c.Type()))
	operator := BasicOperator{
		OperatorID:   c.ID(),
		OperatorType: c.Type(),
		set:          set,
	}

	return operator, nil
}

// BasicOperator provides a basic implementation of an operator.
type BasicOperator struct {
	OperatorID   string
	OperatorType string
	set          component.TelemetrySettings
	// prevent unkeyed literal initialization
	_ struct{}
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

// Logger returns the operator's scoped logger.
func (p *BasicOperator) Logger() *zap.Logger {
	return p.set.Logger
}

// Start will start the operator.
func (*BasicOperator) Start(operator.Persister) error {
	return nil
}

// Stop will stop the operator.
func (*BasicOperator) Stop() error {
	return nil
}
