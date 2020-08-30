package logzioexporter

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/logzio/jaeger-logzio/store"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/trace/jaeger"
	"go.uber.org/zap/zapcore"
	"io"
)

const (
	loggerName 			  = "logzio-exporter"
)

var _ io.Writer = logWriter{}

// logWriter wraps a zap.Logger into an io.Writer.
type logWriter struct {
	logf func(string, ...zapcore.Field)
}

// Write implements io.Writer
func (w logWriter) Write(p []byte) (n int, err error) {
	w.logf(string(p))
	return len(p), nil
}

// exporter exporters OpenTelemetry Collector data to New Relic.
type logzioExporter struct {
	accountToken string
	writer       *store.LogzioSpanWriter
	logger		 hclog.Logger
}

func newLogzioExporter(config *Config, params component.ExporterCreateParams) (*logzioExporter, error) {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Name:       loggerName,
		JSONFormat: true,
	})

	writerConfig := store.LogzioConfig{
		Region:			config.Region,
		AccountToken:	config.Token,
	}

	spanWriter, err := store.NewLogzioSpanWriter(writerConfig, logger)
	if err != nil {
		return nil, err
	}

	return &logzioExporter{
		writer:			spanWriter,
		accountToken:	config.Token,
		logger:			logger,
	}, nil
}

func newLogzioTraceExporter(config *Config, params component.ExporterCreateParams) (component.TraceExporter, error) {
	exporter, err := newLogzioExporter(config, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTraceExporter(
		config,
		exporter.pushTraceData,
		exporterhelper.WithShutdown(exporter.Shutdown))
}

func (exporter *logzioExporter) pushTraceData(ctx context.Context, traces pdata.Traces) (droppedSpansCount int, err error) {
	droppedSpans := 0
	batches, err := jaeger.InternalTracesToJaegerProto(traces)
	if err != nil {
		return traces.SpanCount(), err
	}
	for _, batch := range batches {
		for _, span := range batch.Spans {
			span.Process = batch.Process
			if span.Process == nil {
				span.Process = &model.Process{}
			}
			if err := exporter.writer.WriteSpan(span); err != nil {
				exporter.logger.Debug(fmt.Sprintf("dropped bad span: %s", span.String()))
				droppedSpans++
			}
		}
	}
	return droppedSpans, nil
}

func (exporter *logzioExporter) Shutdown(ctx context.Context) error {
	exporter.writer.Close()
	return nil
}
