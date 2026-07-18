// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Azure Monitor
type Config struct {
	QueueSettings          configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	ConnectionString       configopaque.String                                      `mapstructure:"connection_string"`
	InstrumentationKey     configopaque.String                                      `mapstructure:"instrumentation_key"`
	MaxBatchSize           int                                                      `mapstructure:"maxbatchsize"`
	MaxBatchInterval       time.Duration                                            `mapstructure:"maxbatchinterval"`
	SpanEventsEnabled      bool                                                     `mapstructure:"spaneventsenabled"`
	ShutdownTimeout        time.Duration                                            `mapstructure:"shutdown_timeout"`
	CustomEventsEnabled    bool                                                     `mapstructure:"custom_events_enabled"`
	ExceptionEventsEnabled bool                                                     `mapstructure:"exception_events_enabled"`
	TagMappings            TagMappingsConfig                                        `mapstructure:"tag_mappings"`
	ClientConfig           confighttp.ClientConfig                                  `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
}

// TagMappingsConfig overrides the precedence used to populate selected
// Application Insights envelope tags from OpenTelemetry resource attributes.
// Each field is an ordered list of sources consulted left-to-right; the first
// source that resolves to a non-empty string wins.
//
// A source entry is interpreted as a resource attribute key when it contains a
// "." (e.g. "service.instance.id"); otherwise it is treated as a string
// literal terminal default (e.g. "unknown-instance").
//
// When a field is left unset, the factory default — which preserves the
// historical hardcoded behavior — is used. This feature is alpha and the
// schema may evolve.
type TagMappingsConfig struct {
	// CloudRoleInstance controls how the ai.cloud.roleInstance envelope tag is
	// populated. Default: [service.instance.id].
	CloudRoleInstance []string `mapstructure:"cloud_role_instance"`

	// ApplicationVersion controls how the ai.application.ver envelope tag is
	// populated. Default: [service.version].
	ApplicationVersion []string `mapstructure:"application_version"`
}

// Validate enforces invariants on the configured tag mappings.
func (m TagMappingsConfig) Validate() error {
	if m.CloudRoleInstance != nil && len(m.CloudRoleInstance) == 0 {
		return errors.New("tag_mappings.cloud_role_instance must contain at least one source when set")
	}
	if m.ApplicationVersion != nil && len(m.ApplicationVersion) == 0 {
		return errors.New("tag_mappings.application_version must contain at least one source when set")
	}
	return nil
}

// Validate forwards to nested config validators.
func (c *Config) Validate() error {
	return c.TagMappings.Validate()
}
