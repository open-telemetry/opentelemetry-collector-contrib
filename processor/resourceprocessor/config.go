// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
)

// Config defines configuration for Resource processor.
type Config struct {
	// AttributesActions specifies the list of actions to be applied on resource attributes.
	// The set of actions are {INSERT, UPDATE, UPSERT, DELETE, HASH, EXTRACT}.
	AttributesActions []attraction.ActionKeyValue `mapstructure:"attributes"`

	// prevent unkeyed literal initialization
	_ struct{}
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.AttributesActions) == 0 {
		return errors.New("missing required field \"attributes\"")
	}
	return nil
}
