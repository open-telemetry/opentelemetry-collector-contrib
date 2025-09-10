// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for Prometheus exporter.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// QueueBatchConfig defines the queue configuration.
	QueueBatchConfig exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	// Namespace if set, exports metrics under the provided value.
	Namespace string `mapstructure:"namespace"`

	// ConstLabels are values that are applied for every exported metric.
	ConstLabels prometheus.Labels `mapstructure:"const_labels"`

	// SendTimestamps will send the underlying scrape timestamp with the export
	SendTimestamps bool `mapstructure:"send_timestamps"`

	// MetricExpiration defines how long metrics are kept without updates
	MetricExpiration time.Duration `mapstructure:"metric_expiration"`

	// ResourceToTelemetrySettings defines configuration for converting resource attributes to metric labels.
	ResourceToTelemetrySettings resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`

	// EnableOpenMetrics enables the use of the OpenMetrics encoding option for the prometheus exporter.
	EnableOpenMetrics bool `mapstructure:"enable_open_metrics"`

	// AddMetricSuffixes controls whether suffixes are added to metric names. Defaults to true.
	// Deprecated: Use TranslationStrategy instead. This setting is ignored when TranslationStrategy is explicitly set.
	AddMetricSuffixes bool `mapstructure:"add_metric_suffixes"`

	// TranslationStrategy controls how OTLP metric and attribute names are translated into Prometheus metric and label names.
	// When set, this takes precedence over AddMetricSuffixes.
	TranslationStrategy translationStrategy `mapstructure:"translation_strategy"`
}

var _ component.Config = (*Config)(nil)

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

var disableAddMetricSuffixesFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.prometheusexporter.DisableAddMetricSuffixes",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the deprecated add_metric_suffixes configuration option is ignored and translation_strategy is always used"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-specification/pull/4533"),
)
