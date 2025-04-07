// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/DataDog/datadog-agent/pkg/util/hostname/validate"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

var (
	// ErrUnsetAPIKey is returned when the API key is not set.
	ErrUnsetAPIKey = errors.New("api.key is not set")
	// ErrNoMetadata is returned when only_metadata is enabled but host metadata is disabled or hostname_source is not first_resource.
	ErrNoMetadata = errors.New("only_metadata can't be enabled when host_metadata::enabled = false or host_metadata::hostname_source != first_resource")
	// ErrInvalidHostname is returned when the hostname is invalid.
	ErrEmptyEndpoint = errors.New("endpoint cannot be empty")
	// ErrAPIKeyFormat is returned if API key contains invalid characters
	ErrAPIKeyFormat = errors.New("api::key contains invalid characters")
	// NonHexRegex is a regex of characters that are always invalid in a Datadog API key
	NonHexRegex = regexp.MustCompile(NonHexChars)
)

const (
	// DefaultSite is the default site of the Datadog intake to send data to
	DefaultSite = "datadoghq.com"
	// NonHexChars is a regex of characters that are always invalid in a Datadog API key
	NonHexChars = "[^0-9a-fA-F]"
)

// APIConfig defines the API configuration options
type APIConfig struct {
	// Key is the Datadog API key to associate your Agent's data with your organization.
	// Create a new API key here: https://app.datadoghq.com/account/settings
	Key configopaque.String `mapstructure:"key"`

	// Site is the site of the Datadog intake to send data to.
	// The default value is "datadoghq.com".
	Site string `mapstructure:"site"`

	// FailOnInvalidKey states whether to exit at startup on invalid API key.
	// The default value is false.
	FailOnInvalidKey bool `mapstructure:"fail_on_invalid_key"`
}

// TagsConfig defines the tag-related configuration
// It is embedded in the configuration
type TagsConfig struct {
	// Hostname is the fallback hostname used for payloads without hostname-identifying attributes.
	// This option will NOT change the hostname applied to your metrics, traces and logs if they already have hostname-identifying attributes.
	// If unset, the hostname will be determined automatically. See https://docs.datadoghq.com/opentelemetry/schema_semantics/hostname/?tab=datadogexporter#fallback-hostname-logic for details.
	//
	// Prefer using the `datadog.host.name` resource attribute over using this setting.
	// See https://docs.datadoghq.com/opentelemetry/schema_semantics/hostname/?tab=datadogexporter#general-hostname-semantic-conventions for details.
	Hostname string `mapstructure:"hostname"`
}

// Config defines configuration for the Datadog exporter.
type Config struct {
	confighttp.ClientConfig   `mapstructure:",squash"`        // squash ensures fields are correctly decoded in embedded struct.
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	TagsConfig `mapstructure:",squash"`

	// API defines the Datadog API configuration.
	API APIConfig `mapstructure:"api"`

	// Metrics defines the Metrics exporter specific configuration
	Metrics MetricsConfig `mapstructure:"metrics"`

	// Traces defines the Traces exporter specific configuration
	Traces TracesExporterConfig `mapstructure:"traces"`

	// Logs defines the Logs exporter specific configuration
	Logs LogsConfig `mapstructure:"logs"`

	// HostMetadata defines the host metadata specific configuration
	HostMetadata HostMetadataConfig `mapstructure:"host_metadata"`

	// OnlyMetadata defines whether to only send metadata
	// This is useful for agent-collector setups, so that
	// metadata about a host is sent to the backend even
	// when telemetry data is reported via a different host.
	//
	// This flag is incompatible with disabling host metadata,
	// `use_resource_metadata`, or `host_metadata::hostname_source != first_resource`
	OnlyMetadata bool `mapstructure:"only_metadata"`

	// Non-fatal warnings found during configuration loading.
	warnings []error
}

// LogWarnings logs warning messages that were generated on unmarshaling.
func (c *Config) LogWarnings(logger *zap.Logger) {
	for _, err := range c.warnings {
		logger.Warn(fmt.Sprintf("%v", err))
	}
}

var _ component.Config = (*Config)(nil)

// Validate the configuration for errors. This is required by component.Config.
func (c *Config) Validate() error {
	if err := validateClientConfig(c.ClientConfig); err != nil {
		return err
	}

	if c.OnlyMetadata && (!c.HostMetadata.Enabled || c.HostMetadata.HostnameSource != HostnameSourceFirstResource) {
		return ErrNoMetadata
	}

	if err := validate.ValidHostname(c.Hostname); c.Hostname != "" && err != nil {
		return fmt.Errorf("hostname field is invalid: %w", err)
	}

	if err := StaticAPIKeyCheck(string(c.API.Key)); err != nil {
		return err
	}

	if err := c.Traces.Validate(); err != nil {
		return err
	}

	err := c.Metrics.HistConfig.validate()
	if err != nil {
		return err
	}

	if c.HostMetadata.ReporterPeriod < 5*time.Minute {
		return errors.New("reporter_period must be 5 minutes or higher")
	}

	return nil
}

// StaticAPIKey Check checks if api::key is either empty or contains invalid (non-hex) characters
// It does not validate online; this is handled on startup.
func StaticAPIKeyCheck(key string) error {
	if key == "" {
		return ErrUnsetAPIKey
	}
	invalidAPIKeyChars := NonHexRegex.FindAllString(key, -1)
	if len(invalidAPIKeyChars) > 0 {
		return fmt.Errorf("%w: invalid characters: %s", ErrAPIKeyFormat, strings.Join(invalidAPIKeyChars, ", "))
	}
	return nil
}

func validateClientConfig(cfg confighttp.ClientConfig) error {
	var unsupported []string
	if cfg.Auth != nil {
		unsupported = append(unsupported, "auth")
	}
	if cfg.Endpoint != "" {
		unsupported = append(unsupported, "endpoint")
	}
	if cfg.Compression != "" {
		unsupported = append(unsupported, "compression")
	}
	if len(cfg.Headers) > 0 {
		unsupported = append(unsupported, "headers")
	}
	if cfg.HTTP2ReadIdleTimeout != 0 {
		unsupported = append(unsupported, "http2_read_idle_timeout")
	}
	if cfg.HTTP2PingTimeout != 0 {
		unsupported = append(unsupported, "http2_ping_timeout")
	}

	if len(unsupported) > 0 {
		return fmt.Errorf("these confighttp client configs are currently not respected by Datadog exporter: %s", strings.Join(unsupported, ", "))
	}
	return nil
}

var _ error = (*renameError)(nil)

// renameError is an error related to a renamed setting.
type renameError struct {
	// oldName of the configuration option.
	oldName string
	// newName of the configuration option.
	newName string
	// issueNumber on opentelemetry-collector-contrib for tracking
	issueNumber uint
}

// List of settings that have been removed, but for which we keep a custom error.
var removedSettings = []renameError{
	{
		oldName:     "metrics::send_monotonic_counter",
		newName:     "metrics::sums::cumulative_monotonic_mode",
		issueNumber: 8489,
	},
	{
		oldName:     "tags",
		newName:     "host_metadata::tags",
		issueNumber: 9099,
	},
	{
		oldName:     "send_metadata",
		newName:     "host_metadata::enabled",
		issueNumber: 9099,
	},
	{
		oldName:     "use_resource_metadata",
		newName:     "host_metadata::hostname_source",
		issueNumber: 9099,
	},
	{
		oldName:     "metrics::report_quantiles",
		newName:     "metrics::summaries::mode",
		issueNumber: 8845,
	},
	{
		oldName:     "metrics::instrumentation_library_metadata_as_tags",
		newName:     "metrics::instrumentation_scope_as_tags",
		issueNumber: 11135,
	},
}

// Error implements the error interface.
func (e renameError) Error() string {
	return fmt.Sprintf(
		"%q was removed in favor of %q. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/%d",
		e.oldName,
		e.newName,
		e.issueNumber,
	)
}

func handleRemovedSettings(configMap *confmap.Conf) error {
	var errs []error
	for _, removedErr := range removedSettings {
		if configMap.IsSet(removedErr.oldName) {
			errs = append(errs, removedErr)
		}
	}

	return errors.Join(errs...)
}

var _ confmap.Unmarshaler = (*Config)(nil)

// Unmarshal a configuration map into the configuration struct.
func (c *Config) Unmarshal(configMap *confmap.Conf) error {
	if err := handleRemovedSettings(configMap); err != nil {
		return err
	}

	err := configMap.Unmarshal(c)
	if err != nil {
		return err
	}

	// Add deprecation warnings for deprecated settings.
	renamingWarnings, err := handleRenamedSettings(configMap, c)
	if err != nil {
		return err
	}
	c.warnings = append(c.warnings, renamingWarnings...)

	if c.HostMetadata.HostnameSource == HostnameSourceFirstResource {
		c.warnings = append(c.warnings, errors.New("first_resource is deprecated, opt in to https://docs.datadoghq.com/opentelemetry/mapping/host_metadata/ instead"))
	}

	c.API.Key = configopaque.String(strings.TrimSpace(string(c.API.Key)))

	// If an endpoint is not explicitly set, override it based on the site.
	if !configMap.IsSet("metrics::endpoint") {
		c.Metrics.TCPAddrConfig.Endpoint = fmt.Sprintf("https://api.%s", c.API.Site)
	}
	if !configMap.IsSet("traces::endpoint") {
		c.Traces.TCPAddrConfig.Endpoint = fmt.Sprintf("https://trace.agent.%s", c.API.Site)
	}
	if !configMap.IsSet("logs::endpoint") {
		c.Logs.TCPAddrConfig.Endpoint = fmt.Sprintf("https://http-intake.logs.%s", c.API.Site)
	}

	// Return an error if an endpoint is explicitly set to ""
	if c.Metrics.TCPAddrConfig.Endpoint == "" || c.Traces.TCPAddrConfig.Endpoint == "" || c.Logs.TCPAddrConfig.Endpoint == "" {
		return ErrEmptyEndpoint
	}

	const (
		initialValueSetting = "metrics::sums::initial_cumulative_monotonic_value"
		cumulMonoMode       = "metrics::sums::cumulative_monotonic_mode"
	)
	if configMap.IsSet(initialValueSetting) && c.Metrics.SumConfig.CumulativeMonotonicMode != CumulativeMonotonicSumModeToDelta {
		return fmt.Errorf("%q can only be configured when %q is set to %q",
			initialValueSetting, cumulMonoMode, CumulativeMonotonicSumModeToDelta)
	}

	return nil
}

func defaultClientConfig() confighttp.ClientConfig {
	client := confighttp.NewDefaultClientConfig()
	client.Timeout = 15 * time.Second
	return client
}

// CreateDefaultConfig creates the default exporter configuration
func CreateDefaultConfig() component.Config {
	return &Config{
		ClientConfig:  defaultClientConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),

		API: APIConfig{
			Site: "datadoghq.com",
		},

		Metrics: MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://api.datadoghq.com",
			},
			DeltaTTL: 3600,
			ExporterConfig: MetricsExporterConfig{
				ResourceAttributesAsTags:           false,
				InstrumentationScopeMetadataAsTags: false,
			},
			HistConfig: HistogramConfig{
				Mode:             "distributions",
				SendAggregations: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode:        CumulativeMonotonicSumModeToDelta,
				InitialCumulativeMonotonicMode: InitialValueModeAuto,
			},
			SummaryConfig: SummaryConfig{
				Mode: SummaryModeGauges,
			},
		},

		Traces: TracesExporterConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			TracesConfig: TracesConfig{
				IgnoreResources:        []string{},
				PeerServiceAggregation: true,
				PeerTagsAggregation:    true,
				ComputeStatsBySpanKind: true,
			},
		},

		Logs: LogsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://http-intake.logs.datadoghq.com",
			},
			UseCompression:   true,
			CompressionLevel: 6,
			BatchWait:        5,
		},

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceConfigOrSystem,
			ReporterPeriod: 30 * time.Minute,
		},
	}
}
