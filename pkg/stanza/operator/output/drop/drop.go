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

package drop // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/drop"

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("drop_output", func() operator.Builder { return NewOutputConfig("") })
}

// NewOutputConfig creates a new drop output config with default values
func NewOutputConfig(operatorID string) *OutputConfig {
	return &OutputConfig{
		OutputConfig: helper.NewOutputConfig(operatorID, "drop_output"),
	}
}

// OutputConfig is the configuration of a drop output operator.
type OutputConfig struct {
	helper.OutputConfig `yaml:",inline"`
}

// Build will build a drop output operator.
func (c OutputConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	outputOperator, err := c.OutputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &Output{
		OutputOperator: outputOperator,
	}, nil
}

// Output is an operator that consumes and ignores incoming entries.
type Output struct {
	helper.OutputOperator
}

// Process will drop the incoming entry.
func (p *Output) Process(ctx context.Context, entry *entry.Entry) error {
	return nil
}
