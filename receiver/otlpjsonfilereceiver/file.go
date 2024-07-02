// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"context"
	"errors"
	"regexp"

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

var submatchMismatch = errors.New("regex must have exactly one submatch")

// NewFactory creates a factory for file receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability))
}

type SignalConfig struct {
	RegEx   string `mapstructure:"regex"`
	Enabled *bool  `mapstructure:"enabled"`
}

type Config struct {
	fileconsumer.Config `mapstructure:",squash"`
	StorageID           *component.ID `mapstructure:"storage"`
	ReplayFile          bool          `mapstructure:"replay_file"`
	Logs                SignalConfig  `mapstructure:"logs"`
	Metrics             SignalConfig  `mapstructure:"metrics"`
	Traces              SignalConfig  `mapstructure:"traces"`
}

type signalMatcher struct {
	enabled bool
	re      *regexp.Regexp
}

func (c SignalConfig) regEx() (signalMatcher, error) {
	if c.Enabled != nil && !*c.Enabled {
		return signalMatcher{}, nil
	}
	if c.RegEx == "" {
		return signalMatcher{enabled: true}, nil
	}
	r, err := regexp.Compile(c.RegEx)
	if err != nil {
		return signalMatcher{}, err
	}
	return signalMatcher{enabled: true, re: r}, nil
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

	re, err := cfg.Logs.regEx()
	if err != nil {
		return nil, err
	}

	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token []byte, _ map[string]any) error {
		ctx = obsrecv.StartLogsOp(ctx)
		var l plog.Logs
		token = re.applyRegex(ctx, obsrecv.EndLogsOp, token)
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

	re, err := cfg.Metrics.regEx()
	if err != nil {
		return nil, err
	}

	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token []byte, _ map[string]any) error {
		ctx = obsrecv.StartMetricsOp(ctx)
		var m pmetric.Metrics
		token = re.applyRegex(ctx, obsrecv.EndMetricsOp, token)
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

	re, err := cfg.Traces.regEx()
	if err != nil {
		return nil, err
	}

	input, err := cfg.Config.Build(settings.TelemetrySettings, func(ctx context.Context, token []byte, _ map[string]any) error {
		ctx = obsrecv.StartTracesOp(ctx)
		var t ptrace.Traces
		token = re.applyRegex(ctx, obsrecv.EndTracesOp, token)
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

func (m signalMatcher) applyRegex(ctx context.Context, end func(receiverCtx context.Context, format string, numReceivedLogRecords int, err error), token []byte) []byte {
	if !m.enabled {
		return nil
	}
	if m.re != nil {
		submatch := m.re.FindSubmatch(token)
		if submatch == nil {
			end(ctx, metadata.Type.String(), 0, nil)
			return nil
		}
		if len(submatch) != 2 {
			end(ctx, metadata.Type.String(), 0, submatchMismatch)
			return nil
		}
		token = submatch[1]
	}
	return token
}
