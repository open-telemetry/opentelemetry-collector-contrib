package logtospanexporter

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	otlp "go.opentelemetry.io/collector/exporter/otlpexporter"
)

const typeStr = "logtospan" // The value of "type" key in configuration.

type logToSpanFactory struct {
	component.ExporterFactory
}

// NewFactory returns a factory of the Log to Span exporter that can be registered to the Collector.
func NewFactory() component.ExporterFactory {
	return &logToSpanFactory{
		ExporterFactory: otlp.NewFactory(),
	}
}

func (f *logToSpanFactory) Type() configmodels.Type {
	return typeStr
}

func (f *logToSpanFactory) CreateLogsExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter) (component.LogsExporter, error) {

	cfg := config.(*Config)

	if err := cfg.sanitize(); err != nil {
		return nil, err
	}

	fillUserAgent(cfg, params.ApplicationStartInfo.Version)

	traceExp, err := f.ExporterFactory.CreateTracesExporter(ctx, params, &cfg.Config)
	if err != nil {
		return nil, err
	}

	logToSpanPusher := createLogToSpanPusher(cfg, params, traceExp)

	return exporterhelper.NewLogsExporter(
		cfg,
		params.Logger,
		logToSpanPusher,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(traceExp.Start),
		exporterhelper.WithShutdown(traceExp.Shutdown),
	)
}

func (f *logToSpanFactory) CreateMetricsExporter(
	context.Context,
	component.ExporterCreateParams,
	configmodels.Exporter) (component.MetricsExporter, error) {

	return nil, configerror.ErrDataTypeIsNotSupported
}

func (f *logToSpanFactory) CreateTracesExporter(
	context.Context,
	component.ExporterCreateParams,
	configmodels.Exporter) (component.TracesExporter, error) {

	return nil, configerror.ErrDataTypeIsNotSupported
}

func (f *logToSpanFactory) CreateDefaultConfig() configmodels.Exporter {
	cfg := &Config{
		Config: *f.ExporterFactory.CreateDefaultConfig().(*otlp.Config),
	}

	cfg.TypeVal = typeStr
	cfg.NameVal = typeStr

	cfg.TraceType = W3CTraceType

	cfg.FieldMap = FieldMap{
		W3CTraceContextFields: W3CTraceContextFields{
			Traceparent: "traceparent",
			Tracestate:  "tracestate",
		},
	}

	cfg.Headers["User-Agent"] = "otel-collector-contrib {{version}}"

	return cfg
}

func fillUserAgent(cfg *Config, version string) {
	cfg.Headers["User-Agent"] = strings.ReplaceAll(cfg.Headers["User-Agent"], "{{version}}", version)
}
