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

package flatten // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/flatten"

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "flatten"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new flatten operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new flatten operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a flatten operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Field                    entry.BodyField `mapstructure:"field"`
}

// Build will build a Flatten operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if strings.Contains(c.Field.String(), "attributes") || strings.Contains(c.Field.String(), "resource") {
		return nil, fmt.Errorf("flatten: field cannot be a resource or attribute")
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
		Field:               c.Field,
	}, nil
}

// Transformer flattens an object in the body field
type Transformer struct {
	helper.TransformerOperator
	Field entry.BodyField
}

// Process will process an entry with a flatten transformation.
func (p *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the flatten operation to an entry
func (p *Transformer) Transform(entry *entry.Entry) error {
	parent := p.Field.Parent()
	val, ok := entry.Delete(p.Field)
	if !ok {
		// The field doesn't exist, so ignore it
		return fmt.Errorf("apply flatten: field %s does not exist on body", p.Field)
	}

	valMap, ok := val.(map[string]interface{})
	if !ok {
		// The field we were asked to flatten was not a map, so put it back
		err := entry.Set(p.Field, val)
		if err != nil {
			return errors.Wrap(err, "reset non-map field")
		}
		return fmt.Errorf("apply flatten: field %s is not a map", p.Field)
	}

	for k, v := range valMap {
		err := entry.Set(parent.Child(k), v)
		if err != nil {
			return err
		}
	}
	return nil
}
