// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for Prometheus exporter.
type Config struct {
	ServerConfig confighttp.ServerConfig `mapstructure:",squash"`

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

	// ResourceConstantLabels controls which resource attributes are added as constant labels on Prometheus metrics.
	ResourceConstantLabels resourcetotelemetry.Settings `mapstructure:"resource_constant_labels"`

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
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	//nolint:staticcheck // check deprecated fields
	if cfg.ResourceConstantLabels.Enabled || cfg.ResourceConstantLabels.ExcludeServiceAttributes {
		return errors.New("enabled and exclude_service_attributes are not supported under resource_constant_labels; use included and excluded instead")
	}
	if metadata.ExporterPrometheusDisableResourceToTelemetryConversionFeatureGate.IsEnabled() {
		if !cfg.ResourceToTelemetrySettings.IsEmpty() {
			return errors.New("resource_to_telemetry_conversion is disabled by the exporter.prometheus.DisableResourceToTelemetryConversion feature gate; use resource_constant_labels instead")
		}
	} else if !cfg.ResourceToTelemetrySettings.IsEmpty() && !cfg.ResourceConstantLabels.IsEmpty() {
		return errors.New("cannot configure both resource_to_telemetry_conversion and resource_constant_labels; resource_to_telemetry_conversion is deprecated")
	}
	if err := cfg.ResourceToTelemetrySettings.Validate(); err != nil {
		return err
	}
	if err := cfg.ResourceConstantLabels.Validate(); err != nil {
		return err
	}
	// Validate translation strategy if set
	if cfg.TranslationStrategy != "" {
		switch cfg.TranslationStrategy {
		case underscoreEscapingWithSuffixes, underscoreEscapingWithoutSuffixes, noUTF8EscapingWithSuffixes, noTranslation:
		default:
			return fmt.Errorf("invalid translation_strategy: %s", cfg.TranslationStrategy)
		}
	}
	return nil
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
