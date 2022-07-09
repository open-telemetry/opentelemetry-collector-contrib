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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

const (
	typeStr   = "file"
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for file receiver
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiverAndStabilityLevel(createMetricsReceiver, stability),
		component.WithLogsReceiverAndStabilityLevel(createLogsReceiver, stability),
		component.WithTracesReceiverAndStabilityLevel(createTracesReceiver, stability))
}

type cfg struct {
	config.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	fileconsumer.Config     `mapstructure:",squash"`
}

func createDefaultConfig() config.Receiver {
	return &cfg{
		Config:           *fileconsumer.NewConfig(),
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
	}
}

type receiver struct {
	input *fileconsumer.Input
	id    config.ComponentID
}

func (f *receiver) Start(ctx context.Context, host component.Host) error {
	storageClient, err := adapter.GetStorageClient(ctx, f.id, component.KindReceiver, host)
	if err != nil {
		return err
	}
	return f.input.Start(storageClient)
}

func (f *receiver) Shutdown(ctx context.Context) error {
	return f.input.Stop()
}

func createLogsReceiver(_ context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, logs consumer.Logs) (component.LogsReceiver, error) {
	logsUnmarshaler := plog.NewJSONUnmarshaler()
	input, err := configuration.(*cfg).Config.Build(settings.Logger.Sugar(), func(ctx context.Context, attrs *fileconsumer.FileAttributes, token []byte) {
		l, err := logsUnmarshaler.UnmarshalLogs(token)
		if err != nil {
			settings.Logger.Warn("Error unmarshaling line", zap.Error(err))
		} else {
			err := logs.ConsumeLogs(ctx, l)
			if err != nil {
				settings.Logger.Error("Error consuming line", zap.Error(err))
			}
		}
	})
	if err != nil {
		return nil, err
	}

	return &receiver{input: input, id: configuration.ID()}, nil
}

func createMetricsReceiver(_ context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, metrics consumer.Metrics) (component.MetricsReceiver, error) {
	metricsUnmarshaler := pmetric.NewJSONUnmarshaler()
	input, err := configuration.(*cfg).Config.Build(settings.Logger.Sugar(), func(ctx context.Context, attrs *fileconsumer.FileAttributes, token []byte) {
		l, err := metricsUnmarshaler.UnmarshalMetrics(token)
		if err != nil {
			settings.Logger.Warn("Error unmarshaling line", zap.Error(err))
		} else {
			err := metrics.ConsumeMetrics(ctx, l)
			if err != nil {
				settings.Logger.Error("Error consuming line", zap.Error(err))
			}
		}
	})
	if err != nil {
		return nil, err
	}

	return &receiver{input: input, id: configuration.ID()}, nil
}

func createTracesReceiver(ctx context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, traces consumer.Traces) (component.TracesReceiver, error) {
	tracesUnmarshaler := ptrace.NewJSONUnmarshaler()
	input, err := configuration.(*cfg).Config.Build(settings.Logger.Sugar(), func(ctx context.Context, attrs *fileconsumer.FileAttributes, token []byte) {
		l, err := tracesUnmarshaler.UnmarshalTraces(token)
		if err != nil {
			settings.Logger.Warn("Error unmarshaling line", zap.Error(err))
		} else {
			err := traces.ConsumeTraces(ctx, l)
			if err != nil {
				settings.Logger.Error("Error consuming line", zap.Error(err))
			}
		}
	})
	if err != nil {
		return nil, err
	}

	return &receiver{input: input, id: configuration.ID()}, nil
}
