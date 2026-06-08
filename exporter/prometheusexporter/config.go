// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for Prometheus exporter.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// QueueBatchConfig defines the queue configuration.
	QueueBatchConfig configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	// Namespace if set, exports metrics under the provided value.
	Namespace string `mapstructure:"namespace"`

	// ConstLabels are values that are applied for every exported metric.
	ConstLabels prometheus.Labels `mapstructure:"const_labels"`

	// SendTimestamps will send the underlying scrape timestamp with the export
	SendTimestamps bool `mapstructure:"send_timestamps"`

	// MetricExpiration defines how long metrics are kept without updates
	MetricExpiration time.Duration `mapstructure:"metric_expiration"`

	// ResourceToTelemetrySettings defines configuration for converting resource attributes to metric labels.
	//
	// Deprecated: Use ResourceConstantLabels instead.
	ResourceToTelemetrySettings resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`

	// ResourceConstantLabels defines resource attributes to add to metric labels.
	ResourceConstantLabels configoptional.Optional[ResourceConstantLabels] `mapstructure:"resource_constant_labels"`

	// EnableOpenMetrics enables the use of the OpenMetrics encoding option for the prometheus exporter.
	EnableOpenMetrics bool `mapstructure:"enable_open_metrics"`

	// WithoutScopeInfo controls the addition of labels for the instrumentation scope.
	WithoutScopeInfo bool `mapstructure:"without_scope_info"`

	// AddMetricSuffixes controls whether suffixes are added to metric names. Defaults to true.
	//
	// Deprecated: Use TranslationStrategy instead. This setting is ignored when TranslationStrategy is explicitly set.
	AddMetricSuffixes bool `mapstructure:"add_metric_suffixes"`

	// TranslationStrategy controls how OTLP metric and attribute names are translated into Prometheus metric and label names.
	// When set, this takes precedence over AddMetricSuffixes.
	TranslationStrategy translationStrategy `mapstructure:"translation_strategy"`

	resourceToTelemetrySettingsConfigured bool
}

var _ component.Config = (*Config)(nil)

// ResourceConstantLabels defines which resource attributes are copied to metric labels.
type ResourceConstantLabels struct {
	// Included resource attributes are copied to metric labels.
	// If empty, all resource attributes except excluded attributes are copied.
	Included []string `mapstructure:"included"`

	// Excluded resource attributes are not copied to metric labels.
	Excluded []string `mapstructure:"excluded"`
}

func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	if err := conf.Unmarshal(cfg); err != nil {
		return err
	}
	cfg.resourceToTelemetrySettingsConfigured = conf.IsSet("resource_to_telemetry_conversion")
	return nil
}

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	// Validate translation strategy if set
	if cfg.TranslationStrategy != "" {
		switch cfg.TranslationStrategy {
		case underscoreEscapingWithSuffixes, underscoreEscapingWithoutSuffixes, noUTF8EscapingWithSuffixes, noTranslation:
		default:
			return fmt.Errorf("invalid translation_strategy: %s", cfg.TranslationStrategy)
		}
	}
	if cfg.ResourceConstantLabels.HasValue() && cfg.resourceToTelemetryConfigured() {
		return fmt.Errorf("resource_constant_labels and resource_to_telemetry_conversion cannot be configured at the same time")
	}
	if metadata.ExporterPrometheusexporterResourceConstantLabelsFeatureGate.IsEnabled() {
		if cfg.resourceToTelemetryConfigured() {
			return fmt.Errorf("resource_to_telemetry_conversion is disabled by feature gate %q; use resource_constant_labels instead", "exporter.prometheusexporter.ResourceConstantLabels")
		}
	} else if cfg.ResourceConstantLabels.HasValue() {
		return fmt.Errorf("resource_constant_labels requires feature gate %q", "exporter.prometheusexporter.ResourceConstantLabels")
	}
	return nil
}

func (cfg *Config) resourceToTelemetryConfigured() bool {
	return cfg.resourceToTelemetrySettingsConfigured || cfg.ResourceToTelemetrySettings.Enabled || cfg.ResourceToTelemetrySettings.ExcludeServiceAttributes
}

type translationStrategy string

const (
	// underscoreEscapingWithSuffixes fully escapes metric names for classic Prometheus metric name compatibility,
	// and includes appending type and unit suffixes
	underscoreEscapingWithSuffixes translationStrategy = "UnderscoreEscapingWithSuffixes"

	// underscoreEscapingWithoutSuffixes escapes special characters to '_', but suffixes won't be attached
	underscoreEscapingWithoutSuffixes translationStrategy = "UnderscoreEscapingWithoutSuffixes"

	// noUTF8EscapingWithSuffixes disables changing special characters to '_'. Special suffixes like units and '_total' for counters will be attached
	noUTF8EscapingWithSuffixes translationStrategy = "NoUTF8EscapingWithSuffixes"

	// noTranslation bypasses all metric and label name translation, passing them through unaltered
	noTranslation translationStrategy = "NoTranslation"
)
