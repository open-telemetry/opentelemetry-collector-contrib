// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"fmt"
	"strings"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/core/log"
	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/comp/forwarder/orchestrator/orchestratorinterface"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter"
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	pkgconfigsetup "github.com/DataDog/datadog-agent/pkg/config/setup"
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

func newConfigComponent(set component.TelemetrySettings, cfg *Config) (coreconfig.Component, error) {
	pkgconfig := pkgconfigmodel.NewConfig("DD", "DD", strings.NewReplacer(".", "_"))
	pkgconfigsetup.InitConfig(pkgconfig)

	// Set the API Key
	pkgconfig.Set("api_key", string(cfg.API.Key), pkgconfigmodel.SourceFile)
	pkgconfig.Set("site", cfg.API.Site, pkgconfigmodel.SourceFile)
	pkgconfig.Set("logs_enabled", true, pkgconfigmodel.SourceFile)
	pkgconfig.Set("log_level", set.Logger.Level().String(), pkgconfigmodel.SourceFile)
	pkgconfig.Set("forwarder_timeout", 10, pkgconfigmodel.SourceDefault)
	pkgconfig.Set("apm_config.enabled", true, pkgconfigmodel.SourceFile)
	pkgconfig.Set("apm_config.apm_non_local_traffic", true, pkgconfigmodel.SourceFile)

	return pkgconfig, nil
}

func newSerializer(set component.TelemetrySettings, cfg *Config) (*serializer.Serializer, error) {
	var f defaultforwarder.Component
	var c coreconfig.Component
	var s *serializer.Serializer
	app := fx.New(
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		fx.Supply(set.Logger),
		fx.Supply(set),
		fx.Supply(cfg),
		fx.Provide(newLogComponent),
		fx.Provide(newConfigComponent),

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
	exporterConfig.TimeoutSettings = cfg.TimeoutSettings
	exporterConfig.QueueSettings = cfg.QueueSettings

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
