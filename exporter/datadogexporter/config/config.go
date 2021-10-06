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

package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/valid"
)

var (
	errUnsetAPIKey = errors.New("api.key is not set")
	errNoMetadata  = errors.New("only_metadata can't be enabled when send_metadata or use_resource_metadata is disabled")
	errBuckets     = errors.New("can't use 'metrics::report_buckets' and 'metrics::histograms::mode' at the same time")
)

// TODO: Import these from translator when we eliminate cyclic dependency.
const (
	histogramModeNoBuckets     = "nobuckets"
	histogramModeCounters      = "counters"
	histogramModeDistributions = "distributions"
)

const (
	// DefaultSite is the default site of the Datadog intake to send data to
	DefaultSite = "datadoghq.com"
)

// APIConfig defines the API configuration options
type APIConfig struct {
	// Key is the Datadog API key to associate your Agent's data with your organization.
	// Create a new API key here: https://app.datadoghq.com/account/settings
	// It can also be set through the `DD_API_KEY` environment variable.
	Key string `mapstructure:"key"`

	// Site is the site of the Datadog intake to send data to.
	// It can also be set through the `DD_SITE` environment variable.
	// The default value is "datadoghq.com".
	Site string `mapstructure:"site"`
}

// GetCensoredKey returns the API key censored for logging purposes
func (api *APIConfig) GetCensoredKey() string {
	if len(api.Key) <= 5 {
		return api.Key
	}
	return strings.Repeat("*", len(api.Key)-5) + api.Key[len(api.Key)-5:]
}

// MetricsConfig defines the metrics exporter specific configuration options
type MetricsConfig struct {
	// Buckets states whether to report buckets from histogram metrics.
	// Deprecated: use `metrics::histograms::mode`.
	Buckets bool `mapstructure:"report_buckets"`

	// Quantiles states whether to report quantiles from summary metrics.
	// By default, the minimum, maximum and average are reported.
	Quantiles bool `mapstructure:"report_quantiles"`

	// SendMonotonic states whether to report cumulative monotonic metrics as counters
	// or gauges
	SendMonotonic bool `mapstructure:"send_monotonic_counter"`

	// DeltaTTL defines the time that previous points of a cumulative monotonic
	// metric are kept in memory to calculate deltas
	DeltaTTL int64 `mapstructure:"delta_ttl"`

	// TCPAddr.Endpoint is the host of the Datadog intake server to send metrics to.
	// It can also be set through the `DD_URL` environment variable.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddr `mapstructure:",squash"`

	ExporterConfig MetricsExporterConfig `mapstructure:",squash"`

	// HistConfig defines the export of OTLP Histograms.
	HistConfig HistogramConfig `mapstructure:"histograms"`
}

// HistogramConfig customizes export of OTLP Histograms.
type HistogramConfig struct {
	// Mode for exporting histograms. Valid values are 'counters' or 'nobuckets'.
	//  - 'counters' sends histograms as Datadog counts, one metric per bucket.
	//  - 'nobuckets' sends no bucket histogram metrics. .sum and .count metrics will still be sent
	//    if `send_count_sum_metrics` is enabled.
	//  - 'distributions' sends histograms as Datadog distributions (recommended).
	//
	// The current default is 'nobuckets'.
	Mode string `mapstructure:"mode"`

	// SendCountSum states if the export should send .sum and .count metrics for histograms.
	// The current default is true.
	SendCountSum bool `mapstructure:"send_count_sum_metrics"`
}

func (c *HistogramConfig) validate() error {
	if c.Mode == histogramModeNoBuckets && !c.SendCountSum {
		return fmt.Errorf("'nobuckets' mode and `send_count_sum_metrics` set to false will send no histogram metrics")
	}
	return nil
}

// MetricsExporterConfig provides options for a user to customize the behavior of the
// metrics exporter
type MetricsExporterConfig struct {
	// ResourceAttributesAsTags, if set to true, will use the exporterhelper feature to transform all
	// resource attributes into metric labels, which are then converted into tags
	ResourceAttributesAsTags bool `mapstructure:"resource_attributes_as_tags"`

	// InstrumentationLibraryMetadataAsTags, if set to true, adds the name and version of the
	// instrumentation library that created a metric to the metric tags
	InstrumentationLibraryMetadataAsTags bool `mapstructure:"instrumentation_library_metadata_as_tags"`
}

// TracesConfig defines the traces exporter specific configuration options
type TracesConfig struct {
	// TCPAddr.Endpoint is the host of the Datadog intake server to send traces to.
	// It can also be set through the `DD_APM_URL` environment variable.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddr `mapstructure:",squash"`

	// SampleRate is the rate at which to sample this event. Default is 1,
	// meaning no sampling. If you want to send one event out of every 250
	// times Send() is called, you would specify 250 here.
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
	//   instrumentation::express.server: express
	SpanNameRemappings map[string]string `mapstructure:"span_name_remappings"`
}

// TagsConfig defines the tag-related configuration
// It is embedded in the configuration
type TagsConfig struct {
	// Hostname is the host name for unified service tagging.
	// It can also be set through the `DD_HOST` environment variable.
	// If unset, it is determined automatically.
	// See https://docs.datadoghq.com/agent/faq/how-datadog-agent-determines-the-hostname
	// for more details.
	Hostname string `mapstructure:"hostname"`

	// Env is the environment for unified service tagging.
	// It can also be set through the `DD_ENV` environment variable.
	Env string `mapstructure:"env"`

	// Service is the service for unified service tagging.
	// It can also be set through the `DD_SERVICE` environment variable.
	Service string `mapstructure:"service"`

	// Version is the version for unified service tagging.
	// It can also be set through the `DD_VERSION` environment variable.
	Version string `mapstructure:"version"`

	// EnvVarTags is the list of space-separated tags passed by the `DD_TAGS` environment variable
	// Superseded by Tags if the latter is set.
	// Should not be set in the user-provided config.
	EnvVarTags string `mapstructure:"envvartags"`

	// Tags is the list of default tags to add to every metric or trace.
	Tags []string `mapstructure:"tags"`
}

// GetHostTags gets the host tags extracted from the configuration
func (t *TagsConfig) GetHostTags() []string {
	tags := t.Tags

	if len(tags) == 0 {
		tags = strings.Split(t.EnvVarTags, " ")
	}

	if t.Env != "none" {
		tags = append(tags, fmt.Sprintf("env:%s", t.Env))
	}
	return tags
}

// Config defines configuration for the Datadog exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	TagsConfig `mapstructure:",squash"`

	// API defines the Datadog API configuration.
	API APIConfig `mapstructure:"api"`

	// Metrics defines the Metrics exporter specific configuration
	Metrics MetricsConfig `mapstructure:"metrics"`

	// Traces defines the Traces exporter specific configuration
	Traces TracesConfig `mapstructure:"traces"`

	// SendMetadata defines whether to send host metadata
	// This is undocumented and only used for unit testing.
	//
	// This can't be disabled if `only_metadata` is true.
	SendMetadata bool `mapstructure:"send_metadata"`

	// OnlyMetadata defines whether to only send metadata
	// This is useful for agent-collector setups, so that
	// metadata about a host is sent to the backend even
	// when telemetry data is reported via a different host.
	//
	// This flag is incompatible with disabling `send_metadata`
	// or `use_resource_metadata`.
	OnlyMetadata bool `mapstructure:"only_metadata"`

	// UseResourceMetadata defines whether to use resource attributes
	// for completing host metadata (such as the hostname or host tags).
	//
	// By default this is true: the first resource attribute getting to
	// the exporter will be used for host metadata.
	// Disable this in the Collector if you are using an agent-collector setup.
	UseResourceMetadata bool `mapstructure:"use_resource_metadata"`

	// onceMetadata ensures only one exporter (metrics/traces) sends host metadata
	onceMetadata sync.Once

	// warnings stores non-fatal configuration errors.
	warnings []error
}

func (c *Config) OnceMetadata() *sync.Once {
	return &c.onceMetadata
}

// Sanitize tries to sanitize a given configuration
func (c *Config) Sanitize(logger *zap.Logger) error {
	if c.TagsConfig.Env == "" {
		c.TagsConfig.Env = "none"
	}

	if c.OnlyMetadata && (!c.SendMetadata || !c.UseResourceMetadata) {
		return errNoMetadata
	}

	if err := valid.Hostname(c.Hostname); c.Hostname != "" && err != nil {
		return fmt.Errorf("hostname field is invalid: %s", err)
	}

	if c.API.Key == "" {
		return errUnsetAPIKey
	}

	c.API.Key = strings.TrimSpace(c.API.Key)

	// Set default site
	if c.API.Site == "" {
		c.API.Site = "datadoghq.com"
	}

	// Set the endpoint based on the Site
	if c.Metrics.TCPAddr.Endpoint == "" {
		c.Metrics.TCPAddr.Endpoint = fmt.Sprintf("https://api.%s", c.API.Site)
	}

	if c.Traces.TCPAddr.Endpoint == "" {
		c.Traces.TCPAddr.Endpoint = fmt.Sprintf("https://trace.agent.%s", c.API.Site)
	}

	for _, err := range c.warnings {
		logger.Warn("deprecation warning", zap.Error(err))
	}

	return nil
}

func (c *Config) Validate() error {
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

func (c *Config) Unmarshal(configMap *config.Map) error {
	err := configMap.UnmarshalExact(c)
	if err != nil {
		return err
	}

	if configMap.IsSet("metrics::report_buckets") && configMap.IsSet("metrics::histograms::mode") {
		return errBuckets
	}

	// Deprecation of `report_buckets`: add warning and set `HistConfig.Mode` instead.
	if configMap.IsSet("metrics::report_buckets") {
		if c.Metrics.Buckets {
			c.warnings = append(c.warnings, fmt.Errorf("'metrics::report_buckets' is deprecated. Set 'metrics::histograms::mode' to 'counters' instead"))
			c.Metrics.HistConfig.Mode = histogramModeCounters
		} else {
			c.warnings = append(c.warnings, fmt.Errorf("'metrics::report_buckets' is deprecated. Set 'metrics::histograms::mode' to 'nobuckets' instead"))
			c.Metrics.HistConfig.Mode = histogramModeNoBuckets
		}
	}

	switch c.Metrics.HistConfig.Mode {
	case histogramModeCounters, histogramModeNoBuckets, histogramModeDistributions:
		// Do nothing
	default:
		return fmt.Errorf("invalid `mode` %s", c.Metrics.HistConfig.Mode)
	}

	return nil
}
