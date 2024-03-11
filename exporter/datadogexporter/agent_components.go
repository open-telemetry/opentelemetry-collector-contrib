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
	"github.com/DataDog/datadog-agent/comp/forwarder/orchestrator/orchestratorinterface"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter"
	"github.com/DataDog/datadog-agent/pkg/serializer"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	otlpmetrics "github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

func newLogComponent(set component.TelemetrySettings) (log.Component, error) {
	zlog := &zaplogger{
		logger: set.Logger,
	}
	return zlog, nil
}

func newSerializer(set component.TelemetrySettings, cfg *Config) (*serializer.Serializer, error) {
	var f defaultforwarder.Component
	var c coreconfig.Component
	var s *serializer.Serializer
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
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		fx.Supply(set.Logger),
		coreconfig.Module(),
		fx.Provide(func() coreconfig.Params {
			return coreconfig.NewAgentParams(tempFilePath)
		}),

		fx.Supply(cfg),
		fx.Supply(set),
		fx.Provide(newLogComponent),

		fx.Provide(func(c coreconfig.Component, l log.Component) (defaultforwarder.Params, error) {
			return defaultforwarder.NewParams(c, l), nil
		}),
		fx.Provide(func(c defaultforwarder.Component) (defaultforwarder.Forwarder, error) {
			return defaultforwarder.Forwarder(c), nil
		}),
		fx.Provide(func() string {
			return ""
		}),
		fx.Provide(NewOrchestratorinterfaceimpl),
		fx.Provide(serializer.NewSerializer),
		defaultforwarder.Module(),
		fx.Populate(&f),
		fx.Populate(&c),
		fx.Populate(&s),
	)
	fmt.Printf("### done with app\n")
	if err := app.Err(); err != nil {
		return nil, err
	}
	go func() {
		err := f.Start()
		if err != nil {
			fmt.Printf("### error starting forwarder: %s\n", err)
		}
	}()
	return s, nil
}

type orchestratorinterfaceimpl struct {
	f defaultforwarder.Forwarder
}

func NewOrchestratorinterfaceimpl(f defaultforwarder.Forwarder) orchestratorinterface.Component {
	return &orchestratorinterfaceimpl{
		f: f,
	}
}

func (o *orchestratorinterfaceimpl) Get() (defaultforwarder.Forwarder, bool) {
	return o.f, true
}

func (o *orchestratorinterfaceimpl) Reset() {
	o.f = nil
}

type tagEnricher struct{}

func (t *tagEnricher) SetCardinality(cardinality string) error {
	return nil
}

func (t *tagEnricher) Enrich(ctx context.Context, extraTags []string, dimensions *otlpmetrics.Dimensions) []string {
	return nil
}

func newSerializerExporter(ctx context.Context, set exporter.CreateSettings, cfg *Config) (*serializerexporter.Exporter, error) {
	s, err := newSerializer(set.TelemetrySettings, cfg)
	if err != nil {
		return nil, err
	}
	hostGetter := func(_ context.Context) (string, error) {
		return "", nil
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
	fmt.Printf("### sending iterable series\n")
	exporterConfig.TimeoutSettings = cfg.TimeoutSettings
	exporterConfig.QueueSettings = cfg.QueueSettings
	fmt.Printf("### createMetricsExporter: %#v\n", exporterConfig)
	fmt.Printf("######## creating metric exporter\n")

	// TODO: Ideally the attributes translator would be created once and reused
	// across all signals. This would need unifying the logsagent and serializer
	// exporters into a single exporter.
	attributesTranslator, err := attributes.NewTranslator(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	fmt.Printf("### created attributes translator\n")
	newExp, err := serializerexporter.NewExporter(set.TelemetrySettings, attributesTranslator, s, exporterConfig, &tagEnricher{}, hostGetter)
	if err != nil {
		return nil, err
	}

	return newExp, nil
}
