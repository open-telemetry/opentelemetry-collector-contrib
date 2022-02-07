// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const (
	typeStr         = "awsfirehose"
	defaultEncoding = encodingCWMetricStream
)

type firehoseReceiverFactory struct {
	metricsUnmarshalers map[string]MetricsUnmarshaler
}

func NewFactory() component.ReceiverFactory {
	frf := &firehoseReceiverFactory{
		metricsUnmarshalers: defaultMetricsUnmarshalers(),
	}

	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(frf.createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		Encoding:         defaultEncoding,
	}
}

func (frf *firehoseReceiverFactory) createMetricsReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	return newMetricsReceiver(cfg.(*Config), set, frf.metricsUnmarshalers, nextConsumer)
}
