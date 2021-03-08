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

package hostmetadata

import (
	"context"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/errors"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("host_metadata", func() operator.Builder { return NewHostMetadataConfig("") })
}

// NewHostMetadataConfig returns a HostMetadataConfig with default values
func NewHostMetadataConfig(operatorID string) *HostMetadataConfig {
	return &HostMetadataConfig{
		TransformerConfig:    helper.NewTransformerConfig(operatorID, "host_decorator"),
		HostIdentifierConfig: helper.NewHostIdentifierConfig(),
	}
}

// HostMetadataConfig is the configuration of a host metadata operator
type HostMetadataConfig struct {
	helper.TransformerConfig    `yaml:",inline"`
	helper.HostIdentifierConfig `yaml:",inline"`
}

// Build will build an operator from the supplied configuration
func (c HostMetadataConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(context)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build transformer")
	}

	hostIdentifier, err := c.HostIdentifierConfig.Build()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build host attributer")
	}

	op := &HostMetadata{
		TransformerOperator: transformerOperator,
		HostIdentifier:      hostIdentifier,
	}

	return []operator.Operator{op}, nil
}

// HostMetadata is an operator that can add host metadata to incoming entries
type HostMetadata struct {
	helper.TransformerOperator
	helper.HostIdentifier
}

// Process will process an incoming entry using the metadata transform.
func (h *HostMetadata) Process(ctx context.Context, entry *entry.Entry) error {
	return h.ProcessWith(ctx, entry, h.Transform)
}

// Transform will transform an entry, adding the configured host metadata.
func (h *HostMetadata) Transform(entry *entry.Entry) error {
	h.HostIdentifier.Identify(entry)
	return nil
}
