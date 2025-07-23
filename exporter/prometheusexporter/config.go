// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for Prometheus exporter.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`

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
	// Deprecated: Use TranslationStrategy instead when exporter.prometheusexporter.UseTranslationStrategy feature gate is enabled.
	AddMetricSuffixes bool `mapstructure:"add_metric_suffixes"`

	// TranslationStrategy controls how OTLP metric and attribute names are translated into Prometheus metric and label names.
	// Only used when exporter.prometheusexporter.UseTranslationStrategy feature gate is enabled.
	TranslationStrategy translationStrategy `mapstructure:"translation_strategy"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if translationStrategyFeatureGate.IsEnabled() {
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

var translationStrategyFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.prometheusexporter.UseTranslationStrategy",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the Prometheus exporter uses the new translation_strategy configuration instead of add_metric_suffixes"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-specification/pull/4533"),
)
