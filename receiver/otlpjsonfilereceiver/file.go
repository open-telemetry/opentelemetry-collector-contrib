// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver/internal/metadata"
)

const (
	transport = "file"
)

// NewFactory creates a factory for file receiver
func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		xreceiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		xreceiver.WithTraces(createTracesReceiver, metadata.TracesStability),
		xreceiver.WithProfiles(createProfilesReceiver, metadata.ProfilesStability))
}

type Config struct {
	fileconsumer.Config `mapstructure:",squash"`
	StorageID           *component.ID `mapstructure:"storage"`
	ReplayFile          bool          `mapstructure:"replay_file"`
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
	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token emit.Token) error {
		ctx = obsrecv.StartLogsOp(ctx)
		var l plog.Logs
		l, err = logsUnmarshaler.UnmarshalLogs(token.Body)
		// Appends token.Attributes
		for i := range l.ResourceLogs().Len() {
			resourceLog := l.ResourceLogs().At(i)
			for j := range resourceLog.ScopeLogs().Len() {
				scopeLog := resourceLog.ScopeLogs().At(j)
				for k := range scopeLog.LogRecords().Len() {
					LogRecords := scopeLog.LogRecords().At(k)
					appendToMap(token, LogRecords.Attributes())
				}
			}
		}
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
	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token emit.Token) error {
		ctx = obsrecv.StartMetricsOp(ctx)
		var m pmetric.Metrics
		m, err = metricsUnmarshaler.UnmarshalMetrics(token.Body)
		// Appends token.Attributes
		for i := range m.ResourceMetrics().Len() {
			resourceMetric := m.ResourceMetrics().At(i)
			for j := range resourceMetric.ScopeMetrics().Len() {
				ScopeMetric := resourceMetric.ScopeMetrics().At(j)
				for k := range ScopeMetric.Metrics().Len() {
					metric := ScopeMetric.Metrics().At(k)
					appendToMap(token, metric.Metadata())
				}
			}
		}
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
	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token emit.Token) error {
		ctx = obsrecv.StartTracesOp(ctx)
		var t ptrace.Traces
		t, err = tracesUnmarshaler.UnmarshalTraces(token.Body)
		// Appends token.Attributes
		for i := range t.ResourceSpans().Len() {
			resourceSpan := t.ResourceSpans().At(i)
			for j := range resourceSpan.ScopeSpans().Len() {
				scopeSpan := resourceSpan.ScopeSpans().At(j)
				for k := range scopeSpan.Spans().Len() {
					spans := scopeSpan.Spans().At(k)
					appendToMap(token, spans.Attributes())
				}
			}
		}
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

func createProfilesReceiver(_ context.Context, settings receiver.Settings, configuration component.Config, profiles xconsumer.Profiles) (xreceiver.Profiles, error) {
	profilesUnmarshaler := &pprofile.JSONUnmarshaler{}
	cfg := configuration.(*Config)
	opts := make([]fileconsumer.Option, 0)
	if cfg.ReplayFile {
		opts = append(opts, fileconsumer.WithNoTracking())
	}
	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token emit.Token) error {
		p, _ := profilesUnmarshaler.UnmarshalProfiles(token.Body)
		// TODO Append token.Attributes
		if p.ResourceProfiles().Len() != 0 {
			_ = profiles.ConsumeProfiles(ctx, p)
		}
		return nil
	}, opts...)
	if err != nil {
		return nil, err
	}

	return &otlpjsonfilereceiver{input: input, id: settings.ID, storageID: cfg.StorageID}, nil
}

func appendToMap(token emit.Token, attr pcommon.Map) {
	for key, value := range token.Attributes {
		switch v := value.(type) {
		case string:
			attr.PutStr(key, v)
		case int:
			attr.PutInt(key, int64(v))
		case float64:
			attr.PutDouble(key, float64(v))
		case bool:
			attr.PutBool(key, v)
		}
	}
}
