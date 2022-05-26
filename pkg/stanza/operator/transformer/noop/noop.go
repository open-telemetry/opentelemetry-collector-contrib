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

package noop // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/noop"

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("noop", func() operator.Builder { return NewNoopOperatorConfig("") })
}

// NewNoopOperatorConfig creates a new noop operator config with default values
func NewNoopOperatorConfig(operatorID string) *NoopOperatorConfig {
	return &NoopOperatorConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "noop"),
	}
}

// NoopOperatorConfig is the configuration of a noop operator.
type NoopOperatorConfig struct {
	helper.TransformerConfig `yaml:",inline"`
}

// Build will build a noop operator.
func (c NoopOperatorConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &NoopOperator{
		TransformerOperator: transformerOperator,
	}, nil
}

// NoopOperator is an operator that performs no operations on an entry.
type NoopOperator struct {
	helper.TransformerOperator
}

// Process will forward the entry to the next output without any alterations.
func (p *NoopOperator) Process(ctx context.Context, entry *entry.Entry) error {
	p.Write(ctx, entry)
	return nil
}
