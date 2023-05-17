// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr   = "file"
	stability = component.StabilityLevelDevelopment
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
		receiver.WithTraces(createTracesReceiver, stability),
		receiver.WithLogs(createLogsReceiver, stability),
	)
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := cc.(*Config)
	return &fileReceiver{
		consumer: consumerType{
			metricsConsumer: consumer,
		},
		path:        cfg.Path,
		logger:      settings.Logger,
		throttle:    cfg.Throttle,
		format:      cfg.FormatType,
		compression: cfg.Compression,
	}, nil
}

func createTracesReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Traces,
) (receiver.Traces, error) {
	cfg := cc.(*Config)
	return &fileReceiver{
		consumer: consumerType{
			tracesConsumer: consumer,
		},
		path:        cfg.Path,
		logger:      settings.Logger,
		throttle:    cfg.Throttle,
		format:      cfg.FormatType,
		compression: cfg.Compression,
	}, nil
}

func createLogsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := cc.(*Config)
	return &fileReceiver{
		consumer: consumerType{
			logsConsumer: consumer,
		},
		path:        cfg.Path,
		logger:      settings.Logger,
		throttle:    cfg.Throttle,
		format:      cfg.FormatType,
		compression: cfg.Compression,
	}, nil
}
