// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

// Action specifies how to handle the lookup result.
type Action string

const (
	ActionInsert Action = "insert"
	ActionUpdate Action = "update"
	ActionUpsert Action = "upsert"
)

func (a *Action) UnmarshalText(text []byte) error {
	str := Action(strings.ToLower(string(text)))
	switch str {
	case ActionInsert, ActionUpdate, ActionUpsert:
		*a = str
		return nil
	default:
		return fmt.Errorf("invalid action %q, must be one of: insert, update, upsert", str)
	}
}

// ContextID specifies where to apply the lookup.
// Matches the semantic used by geoipprocessor.
type ContextID string

const (
	ContextRecord   ContextID = "record"
	ContextResource ContextID = "resource"
)

func (c *ContextID) UnmarshalText(text []byte) error {
	str := ContextID(strings.ToLower(string(text)))
	switch str {
	case ContextRecord, ContextResource:
		*c = str
		return nil
	default:
		return fmt.Errorf("invalid context %q, must be one of: record, resource", str)
	}
}

type Config struct {
	Source SourceConfig `mapstructure:"source"`

	// Attributes defines the attribute enrichment rules.
	// Each rule specifies how to look up a value and where to store the result.
	Attributes []AttributeConfig `mapstructure:"attributes"`
}

type SourceConfig struct {
	// Type is the source type identifier (e.g., "noop", "yaml").
	Type string `mapstructure:"type"`

	// Config holds the source-specific configuration as raw map.
	// This is passed to the source factory for parsing.
	Config map[string]any `mapstructure:",remain"`

	// ParsedConfig holds the parsed source-specific configuration.
	// This is populated during config unmarshaling based on the Type.
	ParsedConfig lookupsource.SourceConfig `mapstructure:"-"`
}

// AttributeConfig defines a single attribute enrichment rule.
type AttributeConfig struct {
	// Key is the name of the attribute to set with the lookup result.
	// Required.
	Key string `mapstructure:"key"`

	// FromAttribute is the name of the attribute containing the lookup key.
	// The value of this attribute will be used to query the source.
	// Required.
	FromAttribute string `mapstructure:"from_attribute"`

	// Default is the value to use when the lookup returns no result.
	// If not set and the lookup fails, no attribute is added.
	// Optional.
	Default string `mapstructure:"default"`

	// Action specifies how to handle the lookup result.
	// Valid values: "insert", "update", "upsert".
	// Default: "upsert"
	Action Action `mapstructure:"action"`

	// Context specifies where to read the source attribute and write the result.
	// Valid values: "record" (log record attributes), "resource" (resource attributes).
	// Default: "record"
	Context ContextID `mapstructure:"context"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if len(cfg.Attributes) == 0 {
		return errors.New("at least one attribute mapping must be configured")
	}

	for i, attr := range cfg.Attributes {
		if attr.Key == "" {
			return fmt.Errorf("attributes[%d]: key is required", i)
		}
		if attr.FromAttribute == "" {
			return fmt.Errorf("attributes[%d]: from_attribute is required", i)
		}
	}

	return nil
}

func (a *AttributeConfig) GetAction() Action {
	if a.Action == "" {
		return ActionUpsert
	}
	return a.Action
}

func (a *AttributeConfig) GetContext() ContextID {
	if a.Context == "" {
		return ContextRecord
	}
	return a.Context
}
