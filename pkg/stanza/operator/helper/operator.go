// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
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
func (c BasicConfig) Build(logger *zap.SugaredLogger) (BasicOperator, error) {
	if c.OperatorType == "" {
		return BasicOperator{}, errors.NewError(
			"missing required `type` field.",
			"ensure that all operators have a uniquely defined `type` field.",
			"operator_id", c.ID(),
		)
	}

	if logger == nil {
		return BasicOperator{}, errors.NewError(
			"operator build context is missing a logger.",
			"this is an unexpected internal error",
			"operator_id", c.ID(),
			"operator_type", c.Type(),
		)
	}

	operator := BasicOperator{
		OperatorID:    c.ID(),
		OperatorType:  c.Type(),
		SugaredLogger: logger.With("operator_id", c.ID(), "operator_type", c.Type()),
	}

	return operator, nil
}

// BasicOperator provides a basic implementation of an operator.
type BasicOperator struct {
	OperatorID   string
	OperatorType string
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

// Logger returns the operator's scoped logger.
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
