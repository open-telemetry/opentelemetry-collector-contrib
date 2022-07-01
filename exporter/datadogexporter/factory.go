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

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/service/featuregate"

	ddconfig "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/source"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

const (
	// typeStr is the type of the exporter
	typeStr = "datadog"
)

type factory struct {
	onceMetadata sync.Once

	onceProvider   sync.Once
	sourceProvider source.Provider
	providerErr    error

	registry *featuregate.Registry
}

func (f *factory) SourceProvider(set component.TelemetrySettings, configHostname string) (source.Provider, error) {
	f.onceProvider.Do(func() {
		f.sourceProvider, f.providerErr = metadata.GetSourceProvider(set, configHostname)
	})
	return f.sourceProvider, f.providerErr
}

func newFactoryWithRegistry(registry *featuregate.Registry) component.ExporterFactory {
	f := &factory{registry: registry}
	return component.NewExporterFactory(
		typeStr,
		f.createDefaultConfig,
		component.WithMetricsExporter(f.createMetricsExporter),
		component.WithTracesExporter(f.createTracesExporter),
	)
}

// NewFactory creates a Datadog exporter factory
func NewFactory() component.ExporterFactory {
	return newFactoryWithRegistry(featuregate.GetRegistry())
}

func defaulttimeoutSettings() exporterhelper.TimeoutSettings {
	return exporterhelper.TimeoutSettings{
		Timeout: 15 * time.Second,
	}
}

// createDefaultConfig creates the default exporter configuration
// TODO (#8396): Remove `os.Getenv` everywhere.
func (f *factory) createDefaultConfig() config.Exporter {
	env := os.Getenv("DD_ENV")
	if env == "" {
		env = "none"
	}

	site := os.Getenv("DD_SITE")
	if site == "" {
		site = "datadoghq.com"
	}

	metricsEndpoint := os.Getenv("DD_URL")
	if metricsEndpoint == "" {
		metricsEndpoint = fmt.Sprintf("https://api.%s", site)
	}

	tracesEndpoint := os.Getenv("DD_APM_URL")
	if tracesEndpoint == "" {
		tracesEndpoint = fmt.Sprintf("https://trace.agent.%s", site)
	}

	hostnameSource := ddconfig.HostnameSourceFirstResource
	if f.registry.IsEnabled(metadata.HostnamePreviewFeatureGate) {
		hostnameSource = ddconfig.HostnameSourceConfigOrSystem
	}

	return &ddconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),

		API: ddconfig.APIConfig{
			Key:  os.Getenv("DD_API_KEY"), // Must be set if using API
			Site: site,
		},

		TagsConfig: ddconfig.TagsConfig{
			Hostname:   os.Getenv("DD_HOST"),
			Env:        env,
			Service:    os.Getenv("DD_SERVICE"),
			Version:    os.Getenv("DD_VERSION"),
			EnvVarTags: os.Getenv("DD_TAGS"), // Only taken into account if Tags is not set
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: metricsEndpoint,
			},
			SendMonotonic: true,
			DeltaTTL:      3600,
			Quantiles:     true,
			ExporterConfig: ddconfig.MetricsExporterConfig{
				ResourceAttributesAsTags:             false,
				InstrumentationLibraryMetadataAsTags: false,
				InstrumentationScopeMetadataAsTags:   false,
			},
			HistConfig: ddconfig.HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: ddconfig.SumConfig{
				CumulativeMonotonicMode: ddconfig.CumulativeMonotonicSumModeToDelta,
			},
			SummaryConfig: ddconfig.SummaryConfig{
				Mode: ddconfig.SummaryModeGauges,
			},
		},

		Traces: ddconfig.TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: tracesEndpoint,
			},
			IgnoreResources: []string{},
		},

		HostMetadata: ddconfig.HostMetadataConfig{
			Enabled:        true,
			HostnameSource: hostnameSource,
		},

		SendMetadata:        true,
		UseResourceMetadata: true,
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func (f *factory) createMetricsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	c config.Exporter,
) (component.MetricsExporter, error) {

	cfg := c.(*ddconfig.Config)

	if err := cfg.Sanitize(set.Logger); err != nil {
		return nil, err
	}

	hostProvider, err := f.SourceProvider(set.TelemetrySettings, cfg.Hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to build hostname provider: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	var pushMetricsFn consumer.ConsumeMetricsFunc

	if cfg.OnlyMetadata {
		pushMetricsFn = func(_ context.Context, md pmetric.Metrics) error {
			// only sending metadata use only metrics
			f.onceMetadata.Do(func() {
				attrs := pcommon.NewMap()
				if md.ResourceMetrics().Len() > 0 {
					attrs = md.ResourceMetrics().At(0).Resource().Attributes()
				}
				go metadata.Pusher(ctx, set, newMetadataConfigfromConfig(cfg), hostProvider, attrs)
			})

			return nil
		}
	} else {
		exp, metricsErr := newMetricsExporter(ctx, set, cfg, &f.onceMetadata, hostProvider)
		if metricsErr != nil {
			cancel()
			return nil, metricsErr
		}
		pushMetricsFn = exp.PushMetricsDataScrubbed
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		cfg,
		set,
		pushMetricsFn,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0 * time.Second}),
		// We use our own custom mechanism for retries, since we hit several endpoints.
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
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
func (f *factory) createTracesExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	c config.Exporter,
) (component.TracesExporter, error) {
	cfg, ok := c.(*ddconfig.Config)
	if !ok {
		return nil, errors.New("programming error: config structure is not of type *ddconfig.Config")
	}
	if err := cfg.Sanitize(set.Logger); err != nil {
		return nil, err
	}
	var (
		pusher consumer.ConsumeTracesFunc
		stop   component.ShutdownFunc
	)
	hostProvider, err := f.SourceProvider(set.TelemetrySettings, cfg.Hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to build hostname provider: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx) // nolint:govet
	// cancel() runs on shutdown
	if cfg.OnlyMetadata {
		// only host metadata needs to be sent, once.
		pusher = func(_ context.Context, td ptrace.Traces) error {
			f.onceMetadata.Do(func() {
				attrs := pcommon.NewMap()
				if td.ResourceSpans().Len() > 0 {
					attrs = td.ResourceSpans().At(0).Resource().Attributes()
				}
				go metadata.Pusher(ctx, set, newMetadataConfigfromConfig(cfg), hostProvider, attrs)
			})
			return nil
		}
		stop = func(context.Context) error {
			cancel()
			return nil
		}
	} else {
		tracex, err2 := newTracesExporter(ctx, set, cfg, &f.onceMetadata, hostProvider)
		if err2 != nil {
			cancel()
			return nil, err2
		}
		pusher = tracex.consumeTraces
		stop = func(context.Context) error {
			cancel()              // first cancel context
			tracex.waitShutdown() // then wait for shutdown
			return nil
		}
	}

	return exporterhelper.NewTracesExporter(
		cfg,
		set,
		pusher,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0 * time.Second}),
		// We don't do retries on traces because of deduping concerns on APM Events.
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithShutdown(stop),
	)
}
