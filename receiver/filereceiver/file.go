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

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"bufio"
	"context"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/multierr"
)

const (
	typeStr = "file"
)

var (
	tracesUnmarshaler  = otlp.NewJSONTracesUnmarshaler()
	logsUmarshaler     = otlp.NewJSONLogsUnmarshaler()
	metricsUnmarshaler = otlp.NewJSONMetricsUnmarshaler()
)

// NewFactory creates a factory for file receiver
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver),
		receiverhelper.WithLogs(createLogsReceiver),
		receiverhelper.WithTraces(createTracesReceiver))
}

// ReceiverType implements file.LogReceiverType
// to create a file receiver
type ReceiverType struct{}

// Type is the receiver type
func (f ReceiverType) Type() config.Type {
	return typeStr
}

type cfg struct {
	config.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	Path                    string                   `mapstructure:"path"`
}

func createDefaultConfig() config.Receiver {
	return &cfg{
		Path:             "",
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
	}
}

type receiver struct {
	watcher watcher
}

func (f *receiver) Start(ctx context.Context, host component.Host) error {
	return f.watcher.start()
}

func (f *receiver) Shutdown(ctx context.Context) error {
	return f.watcher.stop()
}

func createLogsReceiver(_ context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, logs consumer.Logs) (component.LogsReceiver, error) {
	readLogs := func(path string) error {
		fileHandle, _ := os.Open(path)
		defer fileHandle.Close()
		fileScanner := bufio.NewScanner(fileHandle)

		var allErrors []error
		for fileScanner.Scan() {
			l, err := logsUmarshaler.UnmarshalLogs(fileScanner.Bytes())
			if err != nil {
				allErrors = append(allErrors, err)
			} else {
				err := logs.ConsumeLogs(context.Background(), l)
				allErrors = append(allErrors, err)
			}
		}

		return multierr.Combine(allErrors...)
	}
	return &receiver{watcher: watcher{path: configuration.(*cfg).Path, callback: readLogs, logger: settings.Logger}}, nil
}

func createMetricsReceiver(_ context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, metrics consumer.Metrics) (component.MetricsReceiver, error) {
	readMetrics := func(path string) error {
		fileHandle, _ := os.Open(path)
		defer fileHandle.Close()
		fileScanner := bufio.NewScanner(fileHandle)

		var allErrors []error
		for fileScanner.Scan() {
			m, err := metricsUnmarshaler.UnmarshalMetrics(fileScanner.Bytes())
			if err != nil {
				allErrors = append(allErrors, err)
			} else {
				err := metrics.ConsumeMetrics(context.Background(), m)
				allErrors = append(allErrors, err)
			}
		}

		return multierr.Combine(allErrors...)
	}
	return &receiver{watcher: watcher{path: configuration.(*cfg).Path, callback: readMetrics, logger: settings.Logger}}, nil
}

func createTracesReceiver(ctx context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, traces consumer.Traces) (component.TracesReceiver, error) {
	readTraces := func(path string) error {
		fileHandle, _ := os.Open(path)
		defer fileHandle.Close()
		fileScanner := bufio.NewScanner(fileHandle)

		var allErrors []error
		for fileScanner.Scan() {
			t, err := tracesUnmarshaler.UnmarshalTraces(fileScanner.Bytes())
			if err != nil {
				allErrors = append(allErrors, err)
			} else {
				err := traces.ConsumeTraces(context.Background(), t)
				allErrors = append(allErrors, err)
			}
		}

		return multierr.Combine(allErrors...)
	}
	return &receiver{watcher: watcher{path: configuration.(*cfg).Path, callback: readTraces, logger: settings.Logger}}, nil
}
