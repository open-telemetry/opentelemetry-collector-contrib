// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver/internal/metadata"
)

const (
	transport = "file"
)

var (
	logsFinder = jsonFinder{
		matcher:    regexp.MustCompile(`(\{"resourceLogs|\{"resource_logs)|(\{"resource":)`),
		fillPrefix: `{"resourceLogs":[`,
	}
	metricsFinder = jsonFinder{
		matcher:    regexp.MustCompile(`(\{"resourceMetrics|\{"resource_metrics)|(\{"resource":)`),
		fillPrefix: `{"resourceMetrics":[`,
	}
	tracesFinder = jsonFinder{
		matcher:    regexp.MustCompile(`(\{"resourceSpans|\{"resource_spans)|(\{"resource":)`),
		fillPrefix: `{"resourceSpans":[`,
	}
)

// NewFactory creates a factory for file receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability))
}

type Config struct {
	fileconsumer.Config `mapstructure:",squash"`
	StorageID           *component.ID `mapstructure:"storage"`
	ReplayFile          bool          `mapstructure:"replay_file"`
}

type jsonFinder struct {
	matcher    *regexp.Regexp
	fillPrefix string
}

func createDefaultConfig() component.Config {
	return &Config{
		Config: *fileconsumer.NewConfig(),
	}
}

type otlpjsonfilereceiver struct {
	input     *fileconsumer.Manager
	id        component.ID
	storageID *component.ID
}

func (f *otlpjsonfilereceiver) Start(ctx context.Context, host component.Host) error {
	storageClient, err := adapter.GetStorageClient(ctx, host, f.storageID, f.id)
	if err != nil {
		return err
	}
	return f.input.Start(storageClient)
}

func (f *otlpjsonfilereceiver) Shutdown(_ context.Context) error {
	return f.input.Stop()
}

func createLogsReceiver(_ context.Context, settings receiver.Settings, configuration component.Config, logs consumer.Logs) (receiver.Logs, error) {
	logsUnmarshaler := &plog.JSONUnmarshaler{}
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	cfg := configuration.(*Config)
	opts := make([]fileconsumer.Option, 0)
	if cfg.ReplayFile {
		opts = append(opts, fileconsumer.WithNoTracking())
	}
	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token []byte, _ map[string]any) error {
		ctx = obsrecv.StartLogsOp(ctx)
		var l plog.Logs
		token = logsFinder.findJson(token)
		if token == nil {
			return nil
		}
		l, err = logsUnmarshaler.UnmarshalLogs(token)
		if err != nil {
			obsrecv.EndLogsOp(ctx, metadata.Type.String(), 0, err)
		} else {
			logRecordCount := l.LogRecordCount()
			if logRecordCount != 0 {
				err = logs.ConsumeLogs(ctx, l)
			}
			obsrecv.EndLogsOp(ctx, metadata.Type.String(), logRecordCount, err)
		}
		return nil
	}, opts...)
	if err != nil {
		return nil, err
	}

	return &otlpjsonfilereceiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}

func createMetricsReceiver(_ context.Context, settings receiver.Settings, configuration component.Config, metrics consumer.Metrics) (receiver.Metrics, error) {
	metricsUnmarshaler := &pmetric.JSONUnmarshaler{}
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	cfg := configuration.(*Config)
	opts := make([]fileconsumer.Option, 0)
	if cfg.ReplayFile {
		opts = append(opts, fileconsumer.WithNoTracking())
	}
	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token []byte, _ map[string]any) error {
		ctx = obsrecv.StartMetricsOp(ctx)
		var m pmetric.Metrics
		token = metricsFinder.findJson(token)
		if token == nil {
			return nil
		}
		m, err = metricsUnmarshaler.UnmarshalMetrics(token)
		if err != nil {
			obsrecv.EndMetricsOp(ctx, metadata.Type.String(), 0, err)
		} else {
			if m.ResourceMetrics().Len() != 0 {
				err = metrics.ConsumeMetrics(ctx, m)
			}
			obsrecv.EndMetricsOp(ctx, metadata.Type.String(), m.MetricCount(), err)
		}
		return nil
	}, opts...)
	if err != nil {
		return nil, err
	}

	return &otlpjsonfilereceiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}

func createTracesReceiver(_ context.Context, settings receiver.Settings, configuration component.Config, traces consumer.Traces) (receiver.Traces, error) {
	tracesUnmarshaler := &ptrace.JSONUnmarshaler{}
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	cfg := configuration.(*Config)
	opts := make([]fileconsumer.Option, 0)
	if cfg.ReplayFile {
		opts = append(opts, fileconsumer.WithNoTracking())
	}
	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token []byte, _ map[string]any) error {
		ctx = obsrecv.StartTracesOp(ctx)
		var t ptrace.Traces
		token = tracesFinder.findJson(token)
		if token == nil {
			return nil
		}
		t, err = tracesUnmarshaler.UnmarshalTraces(token)
		if err != nil {
			obsrecv.EndTracesOp(ctx, metadata.Type.String(), 0, err)
		} else {
			if t.ResourceSpans().Len() != 0 {
				err = traces.ConsumeTraces(ctx, t)
			}
			obsrecv.EndTracesOp(ctx, metadata.Type.String(), t.SpanCount(), err)
		}
		return nil
	}, opts...)
	if err != nil {
		return nil, err
	}

	return &otlpjsonfilereceiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}

func (f jsonFinder) findJson(token []byte) []byte {
	submatchIndexes := f.matcher.FindSubmatchIndex(token)
	arrayStart := submatchIndexes[2]
	recordStart := submatchIndexes[4]
	if arrayStart >= 0 {
		token = token[arrayStart:]
	} else if recordStart >= 0 {
		token = token[recordStart:]
		token = append([]byte(f.fillPrefix), token...)
		token = append(token, []byte(`]}`)...)
	} else {
		return nil
	}

	return token
}
