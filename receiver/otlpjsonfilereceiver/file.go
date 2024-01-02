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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver/internal/metadata"
)

const (
	transport = "file"
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

func createLogsReceiver(_ context.Context, settings receiver.CreateSettings, configuration component.Config, logs consumer.Logs) (receiver.Logs, error) {
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
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, token []byte, _ map[string]any) error {
		ctx = obsrecv.StartLogsOp(ctx)
		var l plog.Logs
		l, err = logsUnmarshaler.UnmarshalLogs(token)
		if err != nil {
			obsrecv.EndLogsOp(ctx, metadata.Type, 0, err)
		} else {
			logRecordCount := l.LogRecordCount()
			if logRecordCount != 0 {
				err = logs.ConsumeLogs(ctx, l)
			}
			obsrecv.EndLogsOp(ctx, metadata.Type, logRecordCount, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &otlpjsonfilereceiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}

func createMetricsReceiver(_ context.Context, settings receiver.CreateSettings, configuration component.Config, metrics consumer.Metrics) (receiver.Metrics, error) {
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
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, token []byte, _ map[string]any) error {
		ctx = obsrecv.StartMetricsOp(ctx)
		var m pmetric.Metrics
		m, err = metricsUnmarshaler.UnmarshalMetrics(token)
		if err != nil {
			obsrecv.EndMetricsOp(ctx, metadata.Type, 0, err)
		} else {
			if m.ResourceMetrics().Len() != 0 {
				err = metrics.ConsumeMetrics(ctx, m)
			}
			obsrecv.EndMetricsOp(ctx, metadata.Type, m.MetricCount(), err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &otlpjsonfilereceiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}

func createTracesReceiver(_ context.Context, settings receiver.CreateSettings, configuration component.Config, traces consumer.Traces) (receiver.Traces, error) {
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
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, token []byte, _ map[string]any) error {
		ctx = obsrecv.StartTracesOp(ctx)
		var t ptrace.Traces
		t, err = tracesUnmarshaler.UnmarshalTraces(token)
		if err != nil {
			obsrecv.EndTracesOp(ctx, metadata.Type, 0, err)
		} else {
			if t.ResourceSpans().Len() != 0 {
				err = traces.ConsumeTraces(ctx, t)
			}
			obsrecv.EndTracesOp(ctx, metadata.Type, t.SpanCount(), err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &otlpjsonfilereceiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}
