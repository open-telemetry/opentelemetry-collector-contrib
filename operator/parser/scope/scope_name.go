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

package scope

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("scope_name_parser", func() operator.Builder { return NewScopeNameParserConfig("") })
}

// NewScopeNameParserConfig creates a new logger name parser config with default values
func NewScopeNameParserConfig(operatorID string) *ScopeNameParserConfig {
	return &ScopeNameParserConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "scope_name_parser"),
		ScopeNameParser:   helper.NewScopeNameParser(),
	}
}

// ScopeNameParserConfig is the configuration of a logger name parser operator.
type ScopeNameParserConfig struct {
	helper.TransformerConfig `mapstructure:",squash"           yaml:",inline"`
	helper.ScopeNameParser   `mapstructure:",omitempty,squash" yaml:",omitempty,inline"`
}

// Build will build a logger name parser operator.
func (c ScopeNameParserConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &ScopeNameParserOperator{
		TransformerOperator: transformerOperator,
		ScopeNameParser:     c.ScopeNameParser,
	}, nil
}

// ScopeNameParserOperator is an operator that parses logger name from a field to an entry.
type ScopeNameParserOperator struct {
	helper.TransformerOperator
	helper.ScopeNameParser
}

// Process will parse logger name from an entry.
func (p *ScopeNameParserOperator) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Parse)
}
