// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/core/log"
	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter"
	"github.com/DataDog/datadog-agent/pkg/serializer"
	otlpmetrics "github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

func newConfigComponent(set component.TelemetrySettings, cfg *Config) (coreconfig.Component, error) {
	var c coreconfig.Component
	yamldata := fmt.Sprintf(`logs_enabled: true
log_level: %s
site: %s
api_key: %s
apm_config:
  enabled: true
  apm_non_local_traffic: true
forwarder_timeout: 10`, set.Logger.Level().String(), cfg.API.Site, cfg.API.Key)

	tempDir, err := os.MkdirTemp("", "conf")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir) // Clean up

	// Create a temporary file within tempDir
	tempFilePath := filepath.Join(tempDir, "datadog.yaml")
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()
	// Write data to the temp file
	if _, err := io.WriteString(tempFile, yamldata); err != nil {
		return nil, err
	}
	app := fx.New(
		coreconfig.Module(),
		fx.Provide(func() coreconfig.Params {
			return coreconfig.NewAgentParams(tempFilePath)
		}),
		fx.Populate(&c),
	)
	if err := app.Err(); err != nil {
		return nil, err
	}

	return c, nil
}

func newLogComponent(set component.TelemetrySettings) (log.Component, error) {
	return &zaplogger{
		logger: set.Logger,
	}, nil
}

func newAgentForwarder(set component.TelemetrySettings, cfg *Config) (defaultforwarder.Forwarder, coreconfig.Component, error) {
	var f defaultforwarder.Component
	var c coreconfig.Component
	app := fx.New(
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		defaultforwarder.Module(),
		fx.Supply(set.Logger),
		fx.Provide(newConfigComponent),
		fx.Supply(cfg),
		fx.Supply(set),
		fx.Provide(newLogComponent),

		fx.Provide(func(c coreconfig.Component, l log.Component) (defaultforwarder.Params, error) {
			return defaultforwarder.NewParams(c, l), nil
		}),
		fx.Populate(&f),
		fx.Populate(&c),
	)
	if err := app.Err(); err != nil {
		return nil, nil, err
	}

	return f, c, nil
}

type orchestratorinterfaceimpl struct {
	f defaultforwarder.Forwarder
}

func (o *orchestratorinterfaceimpl) Get() (defaultforwarder.Forwarder, bool) {
	return o.f, true
}

func (o *orchestratorinterfaceimpl) Reset() {
	o.f = nil
}

func newSerializer(ctx context.Context, set component.TelemetrySettings, cfg *Config) (*serializer.Serializer, error) {
	f, c, err := newAgentForwarder(set, cfg)
	if err != nil {
		return nil, err
	}

	go f.Start()
	return serializer.NewSerializer(f, &orchestratorinterfaceimpl{
		f: f,
	}, c, ""), nil
}

type tagEnricher struct{}

func (t *tagEnricher) SetCardinality(cardinality string) error {
	return nil
}

func (t *tagEnricher) Enrich(ctx context.Context, extraTags []string, dimensions *otlpmetrics.Dimensions) []string {
	return nil
}

func newSerializerExporter(ctx context.Context, set exporter.CreateSettings, cfg *Config) (exporter.Metrics, error) {
	s, err := newSerializer(ctx, set.TelemetrySettings, cfg)
	if err != nil {
		return nil, err
	}
	factory := serializerexporter.NewFactory(s, &tagEnricher{}, func(_ context.Context) (string, error) {
		return "", nil
	})
	if err != nil {
		return nil, err
	}
	exporterConfig := &serializerexporter.ExporterConfig{
		Metrics: serializerexporter.MetricsConfig{
			Enabled:  true,
			DeltaTTL: cfg.Metrics.DeltaTTL,
			ExporterConfig: serializerexporter.MetricsExporterConfig{
				ResourceAttributesAsTags:           cfg.Metrics.ExporterConfig.ResourceAttributesAsTags,
				InstrumentationScopeMetadataAsTags: cfg.Metrics.ExporterConfig.InstrumentationScopeMetadataAsTags,
			},
			HistConfig: serializerexporter.HistogramConfig{
				Mode: string(cfg.Metrics.HistConfig.Mode),
			},
		},
	}
	exporterConfig.TimeoutSettings = cfg.TimeoutSettings
	exporterConfig.QueueSettings = cfg.QueueSettings
	fmt.Printf("### createMetricsExporter: %#v\n", exporterConfig)
	exp, err := factory.CreateMetricsExporter(ctx, set, exporterConfig)
	if err != nil {
		return nil, fmt.Errorf("creating exporter %v", err)
	}
	fmt.Printf("### createMetricsExporter: %v\n", exp)
	return exp, nil
}
