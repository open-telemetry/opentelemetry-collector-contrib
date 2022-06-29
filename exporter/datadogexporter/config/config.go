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

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/valid"
)

var (
	errUnsetAPIKey = errors.New("api.key is not set")
	errNoMetadata  = errors.New("only_metadata can't be enabled when host_metadata::enabled = false or host_metadata::hostname_source != first_resource")
)

const (
	// DefaultSite is the default site of the Datadog intake to send data to
	DefaultSite = "datadoghq.com"
)

// APIConfig defines the API configuration options
type APIConfig struct {
	// Key is the Datadog API key to associate your Agent's data with your organization.
	// Create a new API key here: https://app.datadoghq.com/account/settings
	//
	// It can also be set through the `DD_API_KEY` environment variable (Deprecated: [v0.47.0] set environment variable explicitly on configuration instead).
	Key string `mapstructure:"key"`

	// Site is the site of the Datadog intake to send data to.
	// It can also be set through the `DD_SITE` environment variable (Deprecated: [v0.47.0] set environment variable explicitly on configuration instead).
	// The default value is "datadoghq.com".
	Site string `mapstructure:"site"`

	// FailOnInvalidKey states whether to exit at startup on invalid API key.
	// The default value is false.
	FailOnInvalidKey bool `mapstructure:"fail_on_invalid_key"`
}

// MetricsConfig defines the metrics exporter specific configuration options
type MetricsConfig struct {
	// Quantiles states whether to report quantiles from summary metrics.
	// By default, the minimum, maximum and average are reported.
	// Deprecated: [v0.50.0] Use `metrics::summaries::mode` (SummaryConfig.Mode) instead.
	Quantiles bool `mapstructure:"report_quantiles"`

	// SendMonotonic states whether to report cumulative monotonic metrics as counters
	// or gauges
	// Deprecated: [v0.48.0] Use `metrics::sums::cumulative_monotonic_mode` (SumConfig.CumulativeMonotonicMode) instead.
	SendMonotonic bool `mapstructure:"send_monotonic_counter"`

	// DeltaTTL defines the time that previous points of a cumulative monotonic
	// metric are kept in memory to calculate deltas
	DeltaTTL int64 `mapstructure:"delta_ttl"`

	// TCPAddr.Endpoint is the host of the Datadog intake server to send metrics to.
	// It can also be set through the `DD_URL` environment variable (Deprecated: [v0.47.0] set environment variable explicitly on configuration instead)..
	// If unset, the value is obtained from the Site.
	confignet.TCPAddr `mapstructure:",squash"`

	ExporterConfig MetricsExporterConfig `mapstructure:",squash"`

	// HistConfig defines the export of OTLP Histograms.
	HistConfig HistogramConfig `mapstructure:"histograms"`

	// SumConfig defines the export of OTLP Sums.
	SumConfig SumConfig `mapstructure:"sums"`

	// SummaryConfig defines the export for OTLP Summaries.
	SummaryConfig SummaryConfig `mapstructure:"summaries"`
}

type HistogramMode string

const (
	// HistogramModeNoBuckets reports no bucket histogram metrics. .sum and .count metrics will still be sent
	// if `send_count_sum_metrics` is enabled.
	HistogramModeNoBuckets HistogramMode = "nobuckets"
	// HistogramModeCounters reports histograms as Datadog counts, one metric per bucket.
	HistogramModeCounters HistogramMode = "counters"
	// HistogramModeDistributions reports histograms as Datadog distributions (recommended).
	HistogramModeDistributions HistogramMode = "distributions"
)

var _ encoding.TextUnmarshaler = (*HistogramMode)(nil)

func (hm *HistogramMode) UnmarshalText(in []byte) error {
	switch mode := HistogramMode(in); mode {
	case HistogramModeCounters, HistogramModeDistributions, HistogramModeNoBuckets:
		*hm = mode
		return nil
	default:
		return fmt.Errorf("invalid histogram mode %q", mode)
	}
}

// HistogramConfig customizes export of OTLP Histograms.
type HistogramConfig struct {
	// Mode for exporting histograms. Valid values are 'distributions', 'counters' or 'nobuckets'.
	//  - 'distributions' sends histograms as Datadog distributions (recommended).
	//  - 'counters' sends histograms as Datadog counts, one metric per bucket.
	//  - 'nobuckets' sends no bucket histogram metrics. .sum and .count metrics will still be sent
	//    if `send_count_sum_metrics` is enabled.
	//
	// The current default is 'distributions'.
	Mode HistogramMode `mapstructure:"mode"`

	// SendCountSum states if the export should send .sum and .count metrics for histograms.
	// The current default is false.
	SendCountSum bool `mapstructure:"send_count_sum_metrics"`
}

func (c *HistogramConfig) validate() error {
	if c.Mode == HistogramModeNoBuckets && !c.SendCountSum {
		return fmt.Errorf("'nobuckets' mode and `send_count_sum_metrics` set to false will send no histogram metrics")
	}
	return nil
}

// CumulativeMonotonicSumMode is the export mode for OTLP Sum metrics.
type CumulativeMonotonicSumMode string

const (
	// CumulativeMonotonicSumModeToDelta calculates delta for
	// cumulative monotonic sum metrics in the client side and reports
	// them as Datadog counts.
	CumulativeMonotonicSumModeToDelta CumulativeMonotonicSumMode = "to_delta"

	// CumulativeMonotonicSumModeRawValue reports the raw value for
	// cumulative monotonic sum metrics as a Datadog gauge.
	CumulativeMonotonicSumModeRawValue CumulativeMonotonicSumMode = "raw_value"
)

var _ encoding.TextUnmarshaler = (*CumulativeMonotonicSumMode)(nil)

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (sm *CumulativeMonotonicSumMode) UnmarshalText(in []byte) error {
	switch mode := CumulativeMonotonicSumMode(in); mode {
	case CumulativeMonotonicSumModeToDelta,
		CumulativeMonotonicSumModeRawValue:
		*sm = mode
		return nil
	default:
		return fmt.Errorf("invalid cumulative monotonic sum mode %q", mode)
	}
}

// SumConfig customizes export of OTLP Sums.
type SumConfig struct {
	// CumulativeMonotonicMode is the mode for exporting OTLP Cumulative Monotonic Sums.
	// Valid values are 'to_delta' or 'raw_value'.
	//  - 'to_delta' calculates delta for cumulative monotonic sums and sends it as a Datadog count.
	//  - 'raw_value' sends the raw value of cumulative monotonic sums as Datadog gauges.
	//
	// The default is 'to_delta'.
	// See https://docs.datadoghq.com/metrics/otlp/?tab=sum#mapping for details and examples.
	CumulativeMonotonicMode CumulativeMonotonicSumMode `mapstructure:"cumulative_monotonic_mode"`
}

// SummaryMode is the export mode for OTLP Summary metrics.
type SummaryMode string

const (
	// SummaryModeNoQuantiles sends no `.quantile` metrics. `.sum` and `.count` metrics will still be sent.
	SummaryModeNoQuantiles SummaryMode = "noquantiles"
	// SummaryModeGauges sends `.quantile` metrics as gauges tagged by the quantile.
	SummaryModeGauges SummaryMode = "gauges"
)

var _ encoding.TextUnmarshaler = (*SummaryMode)(nil)

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (sm *SummaryMode) UnmarshalText(in []byte) error {
	switch mode := SummaryMode(in); mode {
	case SummaryModeNoQuantiles,
		SummaryModeGauges:
		*sm = mode
		return nil
	default:
		return fmt.Errorf("invalid summary mode %q", mode)
	}
}

// SummaryConfig customizes export of OTLP Summaries.
type SummaryConfig struct {
	// Mode is the the mode for exporting OTLP Summaries.
	// Valid values are 'noquantiles' or 'gauges'.
	//  - 'noquantiles' sends no `.quantile` metrics. `.sum` and `.count` metrics will still be sent.
	//  - 'gauges' sends `.quantile` metrics as gauges tagged by the quantile.
	//
	// The default is 'gauges'.
	// See https://docs.datadoghq.com/metrics/otlp/?tab=summary#mapping for details and examples.
	Mode SummaryMode `mapstructure:"mode"`
}

// MetricsExporterConfig provides options for a user to customize the behavior of the
// metrics exporter
type MetricsExporterConfig struct {
	// ResourceAttributesAsTags, if set to true, will use the exporterhelper feature to transform all
	// resource attributes into metric labels, which are then converted into tags
	ResourceAttributesAsTags bool `mapstructure:"resource_attributes_as_tags"`

	// Deprecated: [0.54.0] Use InstrumentationScopeMetadataAsTags instead in favor of https://github.com/open-telemetry/opentelemetry-proto/releases/tag/v0.15.0
	// Both must not be enabled at the same time.
	// InstrumentationLibraryMetadataAsTags, if set to true, adds the name and version of the
	// instrumentation library that created a metric to the metric tags
	InstrumentationLibraryMetadataAsTags bool `mapstructure:"instrumentation_library_metadata_as_tags"`

	// InstrumentationScopeMetadataAsTags, if set to true, adds the name and version of the
	// instrumentation scope that created a metric to the metric tags
	InstrumentationScopeMetadataAsTags bool `mapstructure:"instrumentation_scope_metadata_as_tags"`
}

// TracesConfig defines the traces exporter specific configuration options
type TracesConfig struct {
	// TCPAddr.Endpoint is the host of the Datadog intake server to send traces to.
	// It can also be set through the `DD_APM_URL` environment variable (Deprecated: [v0.47.0] set environment variable explicitly on configuration instead).
	// If unset, the value is obtained from the Site.
	confignet.TCPAddr `mapstructure:",squash"`

	// SampleRate is the rate at which to sample this event. Default is 1,
	// meaning no sampling. If you want to send one event out of every 250
	// times Send() is called, you would specify 250 here.
	// Deprecated: [v0.50.0] Not used anywhere.
	SampleRate uint `mapstructure:"sample_rate"`

	// ignored resources
	// A blacklist of regular expressions can be provided to disable certain traces based on their resource name
	// all entries must be surrounded by double quotes and separated by commas.
	// ignore_resources: ["(GET|POST) /healthcheck"]
	IgnoreResources []string `mapstructure:"ignore_resources"`

	// SpanNameRemappings is the map of datadog span names and preferred name to map to. This can be used to
	// automatically map Datadog Span Operation Names to an updated value. All entries should be key/value pairs.
	// span_name_remappings:
	//   io.opentelemetry.javaagent.spring.client: spring.client
	//   instrumentation:express.server: express
	//   go.opentelemetry.io_contrib_instrumentation_net_http_otelhttp.client: http.client
	SpanNameRemappings map[string]string `mapstructure:"span_name_remappings"`

	// If set to true the OpenTelemetry span name will used in the Datadog resource name.
	// If set to false the resource name will be filled with the instrumentation library name + span kind.
	// The default value is `false`.
	SpanNameAsResourceName bool `mapstructure:"span_name_as_resource_name"`
}

// TagsConfig defines the tag-related configuration
// It is embedded in the configuration
type TagsConfig struct {
	// Hostname is the host name for unified service tagging.
	// It can also be set through the `DD_HOST` environment variable (Deprecated: [v0.47.0] set environment variable explicitly on configuration instead).
	// If unset, it is determined automatically.
	// See https://docs.datadoghq.com/agent/faq/how-datadog-agent-determines-the-hostname
	// for more details.
	Hostname string `mapstructure:"hostname"`

	// Env is the environment for unified service tagging.
	// Deprecated: [v0.49.0] Set `deployment.environment` semconv instead, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9016 for details.
	// This option will be removed in v0.52.0.
	// It can also be set through the `DD_ENV` environment variable (Deprecated: [v0.47.0] set environment variable explicitly on configuration instead).
	Env string `mapstructure:"env"`

	// Service is the service for unified service tagging.
	// Deprecated: [v0.49.0] Set `service.name` semconv instead, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8781 for details.
	// This option will be removed in v0.52.0.
	// It can also be set through the `DD_SERVICE` environment variable (Deprecated: [v0.47.0] set environment variable explicitly on configuration instead).
	Service string `mapstructure:"service"`

	// Version is the version for unified service tagging.
	// Deprecated: [v0.49.0] Set `service.version` semconv instead, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8783 for details.
	// This option will be removed in v0.52.0.
	// It can also be set through the `DD_VERSION` environment variable (Deprecated: [v0.47.0] set environment variable explicitly on configuration instead).
	Version string `mapstructure:"version"`

	// EnvVarTags is the list of space-separated tags passed by the `DD_TAGS` environment variable
	// Superseded by Tags if the latter is set.
	// Should not be set in the user-provided config.
	//
	// Deprecated: [v0.47.0] Use `host_metadata::tags` HostMetadataConfig.Tags instead.
	EnvVarTags string `mapstructure:"envvartags"`

	// Tags is the list of default tags to add to every metric or trace.
	// Deprecated: [v0.49.0] Use `host_metadata::tags` (HostMetadataConfig.Tags)
	Tags []string `mapstructure:"tags"`
}

// getHostTags gets the host tags extracted from the configuration
func (t *TagsConfig) getHostTags() []string {
	tags := t.Tags

	if len(tags) == 0 {
		tags = strings.Split(t.EnvVarTags, " ")
	}

	if t.Env != "none" {
		tags = append(tags, fmt.Sprintf("env:%s", t.Env))
	}
	return tags
}

// HostnameSource is the source for the hostname of host metadata.
type HostnameSource string

const (
	// HostnameSourceFirstResource picks the host metadata hostname from the resource
	// attributes on the first OTLP payload that gets to the exporter. If it is lacking any
	// hostname-like attributes, it will fallback to 'config_or_system' behavior (see below).
	//
	// Do not use this hostname source if receiving data from multiple hosts.
	HostnameSourceFirstResource HostnameSource = "first_resource"

	// HostnameSourceConfigOrSystem picks the host metadata hostname from the 'hostname' setting,
	// and if this is empty, from available system APIs and cloud provider endpoints.
	HostnameSourceConfigOrSystem HostnameSource = "config_or_system"
)

var _ encoding.TextUnmarshaler = (*HostnameSource)(nil)

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (sm *HostnameSource) UnmarshalText(in []byte) error {
	switch mode := HostnameSource(in); mode {
	case HostnameSourceFirstResource,
		HostnameSourceConfigOrSystem:
		*sm = mode
		return nil
	default:
		return fmt.Errorf("invalid host metadata hostname source %q", mode)
	}
}

// HostMetadataConfig defines the host metadata related configuration.
// Host metadata is the information used for populating the infrastructure list,
// the host map and providing host tags functionality.
//
// The exporter will send host metadata for a single host, whose name is chosen
// according to `host_metadata::hostname_source`.
type HostMetadataConfig struct {
	// Enabled enables the host metadata functionality.
	Enabled bool `mapstructure:"enabled"`

	// HostnameSource is the source for the hostname of host metadata.
	// Valid values are 'first_resource' and 'config_or_system':
	// - 'first_resource' picks the host metadata hostname from the resource
	//    attributes on the first OTLP payload that gets to the exporter.
	//    If the first payload lacks hostname-like attributes, it will fallback to 'config_or_system'.
	//    Do not use this hostname source if receiving data from multiple hosts.
	// - 'config_or_system' picks the host metadata hostname from the 'hostname' setting,
	//    If this is empty it will use available system APIs and cloud provider endpoints.
	//
	// The current default if 'first_resource'.
	HostnameSource HostnameSource `mapstructure:"hostname_source"`

	// Tags is a list of host tags.
	// These tags will be attached to telemetry signals that have the host metadata hostname.
	// To attach tags to telemetry signals regardless of the host, use a processor instead.
	Tags []string `mapstructure:"tags"`
}

// LimitedTLSClientSetting is a subset of TLSClientSetting, see LimitedHTTPClientSettings for more details
type LimitedTLSClientSettings struct {
	// InsecureSkipVerify controls whether a client verifies the server's
	// certificate chain and host name.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

type LimitedHTTPClientSettings struct {
	TLSSetting LimitedTLSClientSettings `mapstructure:"tls,omitempty"`
}

// Config defines configuration for the Datadog exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	LimitedHTTPClientSettings `mapstructure:",squash"`

	TagsConfig `mapstructure:",squash"`

	// API defines the Datadog API configuration.
	API APIConfig `mapstructure:"api"`

	// Metrics defines the Metrics exporter specific configuration
	Metrics MetricsConfig `mapstructure:"metrics"`

	// Traces defines the Traces exporter specific configuration
	Traces TracesConfig `mapstructure:"traces"`

	// HostMetadata defines the host metadata specific configuration
	HostMetadata HostMetadataConfig `mapstructure:"host_metadata"`

	// SendMetadata defines whether to send host metadata
	// This is undocumented and only used for unit testing.
	//
	// This can't be disabled if `only_metadata` is true.
	// Deprecated: [v0.49.0] Use `host_metadata::enabled` (HostMetadata.Enabled) instead.
	SendMetadata bool `mapstructure:"send_metadata"`

	// OnlyMetadata defines whether to only send metadata
	// This is useful for agent-collector setups, so that
	// metadata about a host is sent to the backend even
	// when telemetry data is reported via a different host.
	//
	// This flag is incompatible with disabling host metadata,
	// `use_resource_metadata`, or `host_metadata::hostname_source != first_resource`
	OnlyMetadata bool `mapstructure:"only_metadata"`

	// UseResourceMetadata defines whether to use resource attributes
	// for completing host metadata (such as the hostname or host tags).
	//
	// By default this is true: the first resource attribute getting to
	// the exporter will be used for host metadata.
	// Disable this in the Collector if you are using an agent-collector setup.
	// Deprecated: [v0.49.0] Use `host_metadata::hostname_source` (HostMetadata.HostnameSource) instead.
	UseResourceMetadata bool `mapstructure:"use_resource_metadata"`

	// warnings stores non-fatal configuration errors.
	warnings []error
}

// Sanitize tries to sanitize a given configuration
// Deprecated: [v0.54.0] Will be unexported in a future minor version.
func (c *Config) Sanitize(logger *zap.Logger) error {
	for _, err := range c.warnings {
		logger.Warn(fmt.Sprintf("%v", err))
	}

	return nil
}

func (c *Config) Validate() error {
	if c.OnlyMetadata && (!c.HostMetadata.Enabled || c.HostMetadata.HostnameSource != HostnameSourceFirstResource) {
		return errNoMetadata
	}

	if err := valid.Hostname(c.Hostname); c.Hostname != "" && err != nil {
		return fmt.Errorf("hostname field is invalid: %w", err)
	}

	if c.API.Key == "" {
		return errUnsetAPIKey
	}

	if c.Traces.IgnoreResources != nil {
		for _, entry := range c.Traces.IgnoreResources {
			_, err := regexp.Compile(entry)
			if err != nil {
				return fmt.Errorf("'%s' is not valid resource filter regular expression", entry)
			}
		}
	}

	if c.Traces.SpanNameRemappings != nil {
		for key, value := range c.Traces.SpanNameRemappings {
			if value == "" {
				return fmt.Errorf("'%s' is not valid value for span name remapping", value)
			}
			if key == "" {
				return fmt.Errorf("'%s' is not valid key for span name remapping", key)
			}
		}
	}

	err := c.Metrics.HistConfig.validate()
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) Unmarshal(configMap *confmap.Conf) error {
	if err := handleRemovedSettings(configMap); err != nil {
		return err
	}

	err := configMap.UnmarshalExact(c)
	if err != nil {
		return err
	}

	c.API.Key = strings.TrimSpace(c.API.Key)

	// If an endpoint is not explicitly set, override it based on the site.
	// TODO (#8396) Remove DD_URL check.
	if !configMap.IsSet("metrics::endpoint") && os.Getenv("DD_URL") == "" {
		c.Metrics.TCPAddr.Endpoint = fmt.Sprintf("https://api.%s", c.API.Site)
	}
	// TODO (#8396) Remove DD_APM_URL check.
	if !configMap.IsSet("traces::endpoint") && os.Getenv("DD_APM_URL") == "" {
		c.Traces.TCPAddr.Endpoint = fmt.Sprintf("https://trace.agent.%s", c.API.Site)
	}

	// Add deprecation warnings for deprecated settings.
	renamingWarnings, err := handleRenamedSettings(configMap, c)
	if err != nil {
		return err
	}
	c.warnings = append(c.warnings, renamingWarnings...)

	// Add warnings about autodetected environment variables.
	c.warnings = append(c.warnings, warnUseOfEnvVars(configMap, c)...)

	deprecationTemplate := "%q has been deprecated and will be removed in %s or later. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/%d"
	if c.Service != "" {
		c.warnings = append(c.warnings, fmt.Errorf(deprecationTemplate, "service", "v0.52.0", 8781))
	}
	if c.Version != "" {
		c.warnings = append(c.warnings, fmt.Errorf(deprecationTemplate, "version", "v0.52.0", 8783))
	}
	if configMap.IsSet("env") {
		c.warnings = append(c.warnings, fmt.Errorf(deprecationTemplate, "env", "v0.52.0", 9016))
	}
	if c.Traces.SampleRate != 0 {
		c.warnings = append(c.warnings, fmt.Errorf(deprecationTemplate, "traces.sample_rate", "v0.52.0", 9771))
	}

	const settingName = "host_metadata::hostname_source"
	if !configMap.IsSet(settingName) && !featuregate.GetRegistry().IsEnabled(metadata.HostnamePreviewFeatureGate) {
		c.warnings = append(c.warnings, fmt.Errorf(
			"%q will change its default value on a future version. Use the %q feature gate to preview this and other hostname changes",
			settingName,
			metadata.HostnamePreviewFeatureGate,
		))
	}
	return nil
}
