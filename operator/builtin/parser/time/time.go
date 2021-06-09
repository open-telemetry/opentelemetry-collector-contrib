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

package time

import (
	"context"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("time_parser", func() operator.Builder { return NewTimeParserConfig("") })
}

// NewTimeParserConfig creates a new time parser config with default values
func NewTimeParserConfig(operatorID string) *TimeParserConfig {
	return &TimeParserConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "time_parser"),
		TimeParser:        helper.NewTimeParser(),
	}
}

// TimeParserConfig is the configuration of a time parser operator.
type TimeParserConfig struct {
	helper.TransformerConfig `mapstructure:",squash" yaml:",inline"`
	helper.TimeParser        `mapstructure:",omitempty,squash" yaml:",omitempty,inline"`
}

// Build will build a time parser operator.
func (c TimeParserConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(context)
	if err != nil {
		return nil, err
	}

	if err := c.TimeParser.Validate(context); err != nil {
		return nil, err
	}

	timeParser := &TimeParserOperator{
		TransformerOperator: transformerOperator,
		TimeParser:          c.TimeParser,
	}

	return []operator.Operator{timeParser}, nil
}

// TimeParserOperator is an operator that parses time from a field to an entry.
type TimeParserOperator struct {
	helper.TransformerOperator
	helper.TimeParser
}

// CanOutput will always return true for a parser operator.
func (t *TimeParserOperator) CanOutput() bool {
	return true
}

// Process will parse time from an entry.
func (t *TimeParserOperator) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.TimeParser.Parse)
}
