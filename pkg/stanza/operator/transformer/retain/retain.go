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

package retain // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/retain"

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("retain", func() operator.Builder { return NewOperatorConfig("") })
}

// NewOperatorConfig creates a new retain operator config with default values
func NewOperatorConfig(operatorID string) *OperatorConfig {
	return &OperatorConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "retain"),
	}
}

// OperatorConfig is the configuration of a retain operator
type OperatorConfig struct {
	helper.TransformerConfig `mapstructure:",squash" yaml:",inline"`
	Fields                   []entry.Field `mapstructure:"fields" json:"fields" yaml:"fields"`
}

// Build will build a retain operator from the supplied configuration
func (c OperatorConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}
	if c.Fields == nil || len(c.Fields) == 0 {
		return nil, fmt.Errorf("retain: 'fields' is empty")
	}

	retainOp := &Operator{
		TransformerOperator: transformerOperator,
		Fields:              c.Fields,
	}

	for _, field := range c.Fields {
		typeCheck := field.String()
		if strings.HasPrefix(typeCheck, "resource") {
			retainOp.AllResourceFields = true
			continue
		}
		if strings.HasPrefix(typeCheck, "attributes") {
			retainOp.AllAttributeFields = true
			continue
		}
		retainOp.AllBodyFields = true
	}
	return retainOp, nil
}

// Operator keeps the given fields and deletes the rest.
type Operator struct {
	helper.TransformerOperator
	Fields             []entry.Field
	AllBodyFields      bool
	AllAttributeFields bool
	AllResourceFields  bool
}

// Process will process an entry with a retain transformation.
func (p *Operator) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the retain operation to an entry
func (p *Operator) Transform(e *entry.Entry) error {
	newEntry := entry.New()
	newEntry.ObservedTimestamp = e.ObservedTimestamp
	newEntry.Timestamp = e.Timestamp

	if !p.AllResourceFields {
		newEntry.Resource = e.Resource
	}
	if !p.AllAttributeFields {
		newEntry.Attributes = e.Attributes
	}
	if !p.AllBodyFields {
		newEntry.Body = e.Body
	}

	for _, field := range p.Fields {
		val, ok := e.Get(field)
		if !ok {
			continue
		}
		err := newEntry.Set(field, val)
		if err != nil {
			return err
		}
	}

	*e = *newEntry
	return nil
}
