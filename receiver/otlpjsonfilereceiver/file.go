// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	rcvr "go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver/internal/metadata"
)

const (
	transport = "file"
)

// NewFactory creates a factory for file receiver
func NewFactory() rcvr.Factory {
	return rcvr.NewFactory(
		metadata.Type,
		createDefaultConfig,
		rcvr.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		rcvr.WithLogs(createLogsReceiver, metadata.LogsStability),
		rcvr.WithTraces(createTracesReceiver, metadata.TracesStability))
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

type receiver struct {
	input     *fileconsumer.Manager
	id        component.ID
	storageID *component.ID
}

func (f *receiver) Start(ctx context.Context, host component.Host) error {
	storageClient, err := adapter.GetStorageClient(ctx, host, f.storageID, f.id)
	if err != nil {
		return err
	}
	return f.input.Start(storageClient)
}

func (f *receiver) Shutdown(_ context.Context) error {
	return f.input.Stop()
}

func createLogsReceiver(_ context.Context, settings rcvr.CreateSettings, configuration component.Config, logs consumer.Logs) (rcvr.Logs, error) {
	logsUnmarshaler := &plog.JSONUnmarshaler{}
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	cfg := configuration.(*Config)
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, token []byte, _ map[string]any) {
		ctx = obsrecv.StartLogsOp(ctx)
		var l plog.Logs
		l, err = logsUnmarshaler.UnmarshalLogs(token)
		if err != nil {
			obsrecv.EndLogsOp(ctx, metadata.Type, 0, err)
		} else {
			if l.ResourceLogs().Len() != 0 {
				err = logs.ConsumeLogs(ctx, l)
			}
			obsrecv.EndLogsOp(ctx, metadata.Type, l.LogRecordCount(), err)
		}
	})
	if err != nil {
		return nil, err
	}

	return &receiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}

func createMetricsReceiver(_ context.Context, settings rcvr.CreateSettings, configuration component.Config, metrics consumer.Metrics) (rcvr.Metrics, error) {
	metricsUnmarshaler := &pmetric.JSONUnmarshaler{}
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	cfg := configuration.(*Config)
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, token []byte, _ map[string]any) {
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
	})
	if err != nil {
		return nil, err
	}

	return &receiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}

func createTracesReceiver(_ context.Context, settings rcvr.CreateSettings, configuration component.Config, traces consumer.Traces) (rcvr.Traces, error) {
	tracesUnmarshaler := &ptrace.JSONUnmarshaler{}
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	cfg := configuration.(*Config)
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, token []byte, _ map[string]any) {
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
	})
	if err != nil {
		return nil, err
	}

	return &receiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}
