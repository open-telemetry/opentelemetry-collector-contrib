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

package copy // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/copy"

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("copy", func() operator.Builder { return NewCopyOperatorConfig("") })
}

// NewCopyOperatorConfig creates a new copy operator config with default values
func NewCopyOperatorConfig(operatorID string) *CopyOperatorConfig {
	return &CopyOperatorConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "copy"),
	}
}

// CopyOperatorConfig is the configuration of a copy operator
type CopyOperatorConfig struct {
	helper.TransformerConfig `mapstructure:",squash" yaml:",inline"`
	From                     entry.Field `mapstructure:"from" json:"from" yaml:"from"`
	To                       entry.Field `mapstructure:"to" json:"to" yaml:"to"`
}

// Build will build a copy operator from the supplied configuration
func (c CopyOperatorConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.From == entry.NewNilField() {
		return nil, fmt.Errorf("copy: missing from field")
	}

	if c.To == entry.NewNilField() {
		return nil, fmt.Errorf("copy: missing to field")
	}

	return &CopyOperator{
		TransformerOperator: transformerOperator,
		From:                c.From,
		To:                  c.To,
	}, nil
}

// CopyOperator copies a value from one field and creates a new field with that value
type CopyOperator struct {
	helper.TransformerOperator
	From entry.Field
	To   entry.Field
}

// Process will process an entry with a copy transformation.
func (p *CopyOperator) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the copy operation to an entry
func (p *CopyOperator) Transform(e *entry.Entry) error {
	val, exist := p.From.Get(e)
	if !exist {
		return fmt.Errorf("copy: from field does not exist in this entry: %s", p.From.String())
	}
	return p.To.Set(e, val)
}
