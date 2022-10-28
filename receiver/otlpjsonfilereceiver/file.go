// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
)

const (
	typeStr   = "otlpjsonfile"
	stability = component.StabilityLevelAlpha
	transport = "file"
)

// NewFactory creates a factory for file receiver
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver, stability),
		component.WithLogsReceiver(createLogsReceiver, stability),
		component.WithTracesReceiver(createTracesReceiver, stability))
}

type Config struct {
	config.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	fileconsumer.Config     `mapstructure:",squash"`
	StorageID               *config.ComponentID `mapstructure:"storage"`
}

func createDefaultConfig() config.Receiver {
	return &Config{
		Config:           *fileconsumer.NewConfig(),
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
	}
}

type receiver struct {
	input     *fileconsumer.Manager
	id        config.ComponentID
	storageID *config.ComponentID
}

func (f *receiver) Start(ctx context.Context, host component.Host) error {
	storageClient, err := adapter.GetStorageClient(ctx, host, f.storageID, f.id)
	if err != nil {
		return err
	}
	return f.input.Start(storageClient)
}

func (f *receiver) Shutdown(ctx context.Context) error {
	return f.input.Stop()
}

func createLogsReceiver(_ context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, logs consumer.Logs) (component.LogsReceiver, error) {
	logsUnmarshaler := &plog.JSONUnmarshaler{}
	obsrecv := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             configuration.ID(),
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	cfg := configuration.(*Config)
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, attrs *fileconsumer.FileAttributes, token []byte) {
		ctx = obsrecv.StartLogsOp(ctx)
		l, err := logsUnmarshaler.UnmarshalLogs(token)
		if err != nil {
			obsrecv.EndLogsOp(ctx, typeStr, 0, err)
		} else {
			err = logs.ConsumeLogs(ctx, l)
			obsrecv.EndLogsOp(ctx, typeStr, l.LogRecordCount(), err)
		}
	})
	if err != nil {
		return nil, err
	}

	return &receiver{input: input, id: cfg.ID(), storageID: cfg.StorageID}, nil
}

func createMetricsReceiver(_ context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, metrics consumer.Metrics) (component.MetricsReceiver, error) {
	metricsUnmarshaler := &pmetric.JSONUnmarshaler{}
	obsrecv := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             configuration.ID(),
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	cfg := configuration.(*Config)
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, attrs *fileconsumer.FileAttributes, token []byte) {
		ctx = obsrecv.StartMetricsOp(ctx)
		m, err := metricsUnmarshaler.UnmarshalMetrics(token)
		if err != nil {
			obsrecv.EndMetricsOp(ctx, typeStr, 0, err)
		} else {
			err = metrics.ConsumeMetrics(ctx, m)
			obsrecv.EndMetricsOp(ctx, typeStr, m.MetricCount(), err)
		}
	})
	if err != nil {
		return nil, err
	}

	return &receiver{input: input, id: cfg.ID(), storageID: cfg.StorageID}, nil
}

func createTracesReceiver(ctx context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, traces consumer.Traces) (component.TracesReceiver, error) {
	tracesUnmarshaler := &ptrace.JSONUnmarshaler{}
	obsrecv := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             configuration.ID(),
		Transport:              transport,
		ReceiverCreateSettings: settings,
	})
	cfg := configuration.(*Config)
	input, err := cfg.Config.Build(settings.Logger.Sugar(), func(ctx context.Context, attrs *fileconsumer.FileAttributes, token []byte) {
		ctx = obsrecv.StartTracesOp(ctx)
		t, err := tracesUnmarshaler.UnmarshalTraces(token)
		if err != nil {
			obsrecv.EndTracesOp(ctx, typeStr, 0, err)
		} else {
			err = traces.ConsumeTraces(ctx, t)
			obsrecv.EndTracesOp(ctx, typeStr, t.SpanCount(), err)
		}
	})
	if err != nil {
		return nil, err
	}

	return &receiver{input: input, id: cfg.ID(), storageID: cfg.StorageID}, nil
}
