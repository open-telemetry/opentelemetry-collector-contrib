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

package googlecloudpubsubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
)

const (
	typeStr              = "googlecloudpubsub"
	reportTransport      = "pubsub"
	reportFormatProtobuf = "protobuf"
)

func NewFactory() component.ReceiverFactory {
	f := &pubsubReceiverFactory{
		receivers: make(map[*Config]*pubsubReceiver),
	}
	return component.NewReceiverFactory(
		typeStr,
		f.CreateDefaultConfig,
		component.WithTracesReceiver(f.CreateTracesReceiver),
		component.WithMetricsReceiver(f.CreateMetricsReceiver),
		component.WithLogsReceiver(f.CreateLogsReceiver),
	)
}

type pubsubReceiverFactory struct {
	receivers map[*Config]*pubsubReceiver
}

func (factory *pubsubReceiverFactory) CreateDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
	}
}

func (factory *pubsubReceiverFactory) ensureReceiver(params component.ReceiverCreateSettings, config config.Receiver) *pubsubReceiver {
	receiver := factory.receivers[config.(*Config)]
	if receiver != nil {
		return receiver
	}
	rconfig := config.(*Config)
	receiver = &pubsubReceiver{
		logger: params.Logger,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              reportTransport,
			ReceiverCreateSettings: params,
		}),
		userAgent: strings.ReplaceAll(rconfig.UserAgent, "{{version}}", params.BuildInfo.Version),
		config:    rconfig,
	}
	factory.receivers[config.(*Config)] = receiver
	return receiver
}

func (factory *pubsubReceiverFactory) CreateTracesReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Traces) (component.TracesReceiver, error) {

	if consumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}
	err := cfg.(*Config).validateForTrace()
	if err != nil {
		return nil, err
	}
	receiver := factory.ensureReceiver(params, cfg)
	receiver.tracesConsumer = consumer
	return receiver, nil
}

func (factory *pubsubReceiverFactory) CreateMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Metrics) (component.MetricsReceiver, error) {

	if consumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}
	err := cfg.(*Config).validateForMetric()
	if err != nil {
		return nil, err
	}
	receiver := factory.ensureReceiver(params, cfg)
	receiver.metricsConsumer = consumer
	return receiver, nil
}

func (factory *pubsubReceiverFactory) CreateLogsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Logs) (component.LogsReceiver, error) {

	if consumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}
	err := cfg.(*Config).validateForLog()
	if err != nil {
		return nil, err
	}
	receiver := factory.ensureReceiver(params, cfg)
	receiver.logsConsumer = consumer
	return receiver, nil
}
