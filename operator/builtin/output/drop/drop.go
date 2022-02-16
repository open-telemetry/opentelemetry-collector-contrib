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

package drop

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("drop_output", func() operator.Builder { return NewDropOutputConfig("") })
}

// NewDropOutputConfig creates a new drop output config with default values
func NewDropOutputConfig(operatorID string) *DropOutputConfig {
	return &DropOutputConfig{
		OutputConfig: helper.NewOutputConfig(operatorID, "drop_output"),
	}
}

// DropOutputConfig is the configuration of a drop output operator.
type DropOutputConfig struct {
	helper.OutputConfig `yaml:",inline"`
}

// Build will build a drop output operator.
func (c DropOutputConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	outputOperator, err := c.OutputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &DropOutput{
		OutputOperator: outputOperator,
	}, nil
}

// DropOutput is an operator that consumes and ignores incoming entries.
type DropOutput struct {
	helper.OutputOperator
}

// Process will drop the incoming entry.
func (p *DropOutput) Process(ctx context.Context, entry *entry.Entry) error {
	return nil
}
