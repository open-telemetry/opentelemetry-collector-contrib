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

package datadogexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumerhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"

	ddconfig "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

const (
	// typeStr is the type of the exporter
	typeStr = "datadog"
)

// NewFactory creates a Datadog exporter factory
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithMetrics(createMetricsExporter),
		exporterhelper.WithTraces(createTracesExporter),
	)
}

// createDefaultConfig creates the default exporter configuration
func createDefaultConfig() config.Exporter {
	return &ddconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  exporterhelper.DefaultTimeoutSettings(),
		RetrySettings:    exporterhelper.DefaultRetrySettings(),
		QueueSettings:    exporterhelper.DefaultQueueSettings(),

		API: ddconfig.APIConfig{
			Key:  "$DD_API_KEY", // Must be set if using API
			Site: "$DD_SITE",    // If not provided, set during config sanitization
		},

		TagsConfig: ddconfig.TagsConfig{
			Hostname:   "$DD_HOST",
			Env:        "$DD_ENV",
			Service:    "$DD_SERVICE",
			Version:    "$DD_VERSION",
			EnvVarTags: "$DD_TAGS", // Only taken into account if Tags is not set
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "$DD_URL", // If not provided, set during config sanitization
			},
			SendMonotonic: true,
			DeltaTTL:      3600,
			Quantiles:     true,
			ExporterConfig: ddconfig.MetricsExporterConfig{
				ResourceAttributesAsTags:             false,
				InstrumentationLibraryMetadataAsTags: false,
			},
			HistConfig: ddconfig.HistogramConfig{
				Mode:         "nobuckets",
				SendCountSum: true,
			},
		},

		Traces: ddconfig.TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "$DD_APM_URL", // If not provided, set during config sanitization
			},
			IgnoreResources: []string{},
		},

		SendMetadata:        true,
		UseResourceMetadata: true,
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	c config.Exporter,
) (component.MetricsExporter, error) {

	cfg := c.(*ddconfig.Config)

	set.Logger.Info("sanitizing Datadog metrics exporter configuration")
	if err := cfg.Sanitize(set.Logger); err != nil {
		return nil, err
	}

	// TODO: Remove after two releases
	if cfg.Metrics.HistConfig.Mode == "counters" {
		set.Logger.Warn("Histogram bucket metrics now end with .bucket instead of .count_per_bucket")
	}

	// TODO: Remove after changing the default mode.
	set.Logger.Warn("Default histograms configuration will change to mode 'distributions' and no .count and .sum metrics in a future release.")

	ctx, cancel := context.WithCancel(ctx)
	var pushMetricsFn consumerhelper.ConsumeMetricsFunc

	if cfg.OnlyMetadata {
		pushMetricsFn = func(_ context.Context, md pdata.Metrics) error {
			// only sending metadata use only metrics
			once := cfg.OnceMetadata()
			once.Do(func() {
				attrs := pdata.NewAttributeMap()
				if md.ResourceMetrics().Len() > 0 {
					attrs = md.ResourceMetrics().At(0).Resource().Attributes()
				}
				go metadata.Pusher(ctx, set, cfg, attrs)
			})
			return nil
		}
	} else {
		exp, err := newMetricsExporter(ctx, set, cfg)
		if err != nil {
			cancel()
			return nil, err
		}
		pushMetricsFn = exp.PushMetricsDataScrubbed
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		cfg,
		set,
		pushMetricsFn,
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithShutdown(func(context.Context) error {
			cancel()
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	return resourcetotelemetry.WrapMetricsExporter(
		resourcetotelemetry.Settings{Enabled: cfg.Metrics.ExporterConfig.ResourceAttributesAsTags}, exporter), nil
}

// createTracesExporter creates a trace exporter based on this config.
func createTracesExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	c config.Exporter,
) (component.TracesExporter, error) {

	cfg := c.(*ddconfig.Config)

	set.Logger.Info("sanitizing Datadog metrics exporter configuration")
	if err := cfg.Sanitize(set.Logger); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	var pushTracesFn consumerhelper.ConsumeTracesFunc

	if cfg.OnlyMetadata {
		pushTracesFn = func(_ context.Context, td pdata.Traces) error {
			// only sending metadata, use only attributes
			once := cfg.OnceMetadata()
			once.Do(func() {
				attrs := pdata.NewAttributeMap()
				if td.ResourceSpans().Len() > 0 {
					attrs = td.ResourceSpans().At(0).Resource().Attributes()
				}
				go metadata.Pusher(ctx, set, cfg, attrs)
			})
			return nil
		}
	} else {
		pushTracesFn = newTracesExporter(ctx, set, cfg).pushTraceDataScrubbed
	}

	return exporterhelper.NewTracesExporter(
		cfg,
		set,
		pushTracesFn,
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithShutdown(func(context.Context) error {
			cancel()
			return nil
		}),
	)
}
