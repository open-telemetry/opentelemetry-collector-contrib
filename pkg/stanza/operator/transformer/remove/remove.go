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

package remove // import "github.com/open-telemetry/opentelemetry-log-collection/operator/transformer/remove"

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("remove", func() operator.Builder { return NewRemoveOperatorConfig("") })
}

// NewRemoveOperatorConfig creates a new remove operator config with default values
func NewRemoveOperatorConfig(operatorID string) *RemoveOperatorConfig {
	return &RemoveOperatorConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "remove"),
	}
}

// RemoveOperatorConfig is the configuration of a remove operator
type RemoveOperatorConfig struct {
	helper.TransformerConfig `mapstructure:",squash" yaml:",inline"`

	Field rootableField `mapstructure:"field"  json:"field" yaml:"field"`
}

// Build will build a Remove operator from the supplied configuration
func (c RemoveOperatorConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.Field.Field == entry.NewNilField() {
		return nil, fmt.Errorf("remove: field is empty")
	}

	return &RemoveOperator{
		TransformerOperator: transformerOperator,
		Field:               c.Field,
	}, nil
}

// RemoveOperator is an operator that deletes a field
type RemoveOperator struct {
	helper.TransformerOperator
	Field rootableField
}

// Process will process an entry with a remove transformation.
func (p *RemoveOperator) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the remove operation to an entry
func (p *RemoveOperator) Transform(entry *entry.Entry) error {
	if p.Field.allAttributes {
		entry.Attributes = nil
		return nil
	}

	if p.Field.allResource {
		entry.Resource = nil
		return nil
	}

	_, exist := entry.Delete(p.Field.Field)
	if !exist {
		return fmt.Errorf("remove: field does not exist")
	}
	return nil
}
