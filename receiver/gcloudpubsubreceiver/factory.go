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

package gcloudpubsubreceiver

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
)

const (
	typeStr = "gcloudpubsub"
)

func NewFactory() component.ReceiverFactory {
	return &PubsubReceiverFactory{
		receivers: make(map[string]*pubsubReceiver),
	}
}

type PubsubReceiverFactory struct {
	receivers map[string]*pubsubReceiver
}

func (factory *PubsubReceiverFactory) Type() configmodels.Type {
	return typeStr
}

func (factory *PubsubReceiverFactory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
	}
}

func (factory *PubsubReceiverFactory) ensureReceiver(params component.ReceiverCreateParams, config configmodels.Receiver) *pubsubReceiver {
	receiver := factory.receivers[config.Name()]
	if receiver != nil {
		return receiver
	}
	rconfig := config.(*Config)
	receiver = &pubsubReceiver{
		instanceName: config.Name(),
		logger:       params.Logger,
		userAgent:    strings.ReplaceAll(rconfig.UserAgent, "{{version}}", params.ApplicationStartInfo.Version),
		config:       rconfig,
	}
	factory.receivers[config.Name()] = receiver
	return receiver
}

func (factory *PubsubReceiverFactory) CreateTracesReceiver(
	_ context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.TracesConsumer) (component.TracesReceiver, error) {
	if consumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}
	receiver := factory.ensureReceiver(params, cfg)
	receiver.tracesConsumer = consumer
	return receiver, nil
}

func (factory *PubsubReceiverFactory) CreateMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumer) (component.MetricsReceiver, error) {
	if consumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}
	receiver := factory.ensureReceiver(params, cfg)
	receiver.metricsConsumer = consumer
	return receiver, nil
}

func (factory *PubsubReceiverFactory) CreateLogsReceiver(
	_ context.Context,
	params component.ReceiverCreateParams,
	cfg configmodels.Receiver,
	consumer consumer.LogsConsumer) (component.LogsReceiver, error) {
	if consumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}
	receiver := factory.ensureReceiver(params, cfg)
	receiver.logsConsumer = consumer
	return receiver, nil
}
