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
)

const (
	typeStr = "file"
)

// NewFactory creates a factory for file receiver
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver),
		component.WithLogsReceiver(createLogsReceiver),
		component.WithTracesReceiver(createTracesReceiver))
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
	watcher *watcher
}

func (f *receiver) Start(ctx context.Context, host component.Host) error {
	return f.watcher.start(ctx)
}

func (f *receiver) Shutdown(ctx context.Context) error {
	return f.watcher.stop()
}

var watchers = map[string]*watcher{}

func createLogsReceiver(_ context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, logs consumer.Logs) (component.LogsReceiver, error) {
	path := configuration.(*cfg).Path
	w, ok := watchers[path]
	if !ok {
		w = &watcher{path: configuration.(*cfg).Path, logger: settings.Logger}
		watchers[path] = w
	}
	w.logsConsumer = logs

	return &receiver{watcher: w}, nil
}

func createMetricsReceiver(_ context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, metrics consumer.Metrics) (component.MetricsReceiver, error) {
	path := configuration.(*cfg).Path
	w, ok := watchers[path]
	if !ok {
		w = &watcher{path: configuration.(*cfg).Path, logger: settings.Logger}
		watchers[path] = w
	}
	w.metricsConsumer = metrics

	return &receiver{watcher: w}, nil
}

func createTracesReceiver(ctx context.Context, settings component.ReceiverCreateSettings, configuration config.Receiver, traces consumer.Traces) (component.TracesReceiver, error) {
	path := configuration.(*cfg).Path
	w, ok := watchers[path]
	if !ok {
		w = &watcher{path: configuration.(*cfg).Path, logger: settings.Logger}
		watchers[path] = w
	}
	w.tracesConsumer = traces

	return &receiver{watcher: w}, nil
}
