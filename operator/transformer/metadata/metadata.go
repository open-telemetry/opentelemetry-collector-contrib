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

package metadata

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/errors"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("metadata", func() operator.Builder { return NewMetadataOperatorConfig("") })
}

// NewMetadataOperatorConfig creates a new metadata config with default values
func NewMetadataOperatorConfig(operatorID string) *MetadataOperatorConfig {
	return &MetadataOperatorConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "metadata"),
		AttributerConfig:  helper.NewAttributerConfig(),
		IdentifierConfig:  helper.NewIdentifierConfig(),
	}
}

// MetadataOperatorConfig is the configuration of a metadata operator
type MetadataOperatorConfig struct {
	helper.TransformerConfig `mapstructure:",squash"  yaml:",inline"`
	helper.AttributerConfig  `mapstructure:",squash"  yaml:",inline"`
	helper.IdentifierConfig  `mapstructure:",squash"  yaml:",inline"`
}

// Build will build a metadata operator from the supplied configuration
func (c MetadataOperatorConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build transformer")
	}

	attributer, err := c.AttributerConfig.Build()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build attributer")
	}

	identifier, err := c.IdentifierConfig.Build()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build identifier")
	}

	return &MetadataOperator{
		TransformerOperator: transformerOperator,
		Attributer:          attributer,
		Identifier:          identifier,
	}, nil
}

// MetadataOperator is an operator that can add metadata to incoming entries
type MetadataOperator struct {
	helper.TransformerOperator
	helper.Attributer
	helper.Identifier
}

// Process will process an incoming entry using the metadata transform.
func (p *MetadataOperator) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will transform an entry using the attributer and tagger.
func (p *MetadataOperator) Transform(entry *entry.Entry) error {
	if err := p.Attribute(entry); err != nil {
		return errors.Wrap(err, "failed to add attributes to entry")
	}

	if err := p.Identify(entry); err != nil {
		return errors.Wrap(err, "failed to add resource keys to entry")
	}

	return nil
}
