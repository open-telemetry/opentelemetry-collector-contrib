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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr              = "googlecloudpubsub"
	stability            = component.StabilityLevelBeta
	reportTransport      = "pubsub"
	reportFormatProtobuf = "protobuf"
)

func NewFactory() receiver.Factory {
	f := &pubsubReceiverFactory{
		receivers: make(map[*Config]psReceiver),
	}
	return receiver.NewFactory(
		typeStr,
		f.CreateDefaultConfig,
		receiver.WithTraces(f.CreateTracesReceiver, stability),
		receiver.WithMetrics(f.CreateMetricsReceiver, stability),
		receiver.WithLogs(f.CreateLogsReceiver, stability),
	)
}

type pubsubReceiverFactory struct {
	receivers map[*Config]psReceiver
}

type psReceiver interface {
	receiver.Traces
	receiver.Metrics
	receiver.Logs

	setTracesConsumer(consumer.Traces)
	setMetricsConsumer(consumer.Metrics)
	setLogsConsumer(consumer.Logs)
}

func (factory *pubsubReceiverFactory) CreateDefaultConfig() component.Config {
	return &Config{}
}

func (factory *pubsubReceiverFactory) ensureReceiver(params receiver.CreateSettings, config *Config) (psReceiver, error) {
	var receiver psReceiver
	receiver = factory.receivers[config]
	if receiver != nil {
		return receiver, nil
	}

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             params.ID,
		Transport:              reportTransport,
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}

	psr := pubsubReceiver{
		logger:            params.Logger,
		obsrecv:           obsrecv,
		config:            config,
		telemetrySettings: params.TelemetrySettings,
	}
	if config.Mode == "push" {
		receiver = &pubsubPushReceiver{
			pubsubReceiver: &psr,
		}
	} else {
		receiver = &pubsubPullReceiver{
			pubsubReceiver: &psr,
			userAgent:      strings.ReplaceAll(config.UserAgent, "{{version}}", params.BuildInfo.Version),
		}
	}
	factory.receivers[config] = receiver
	return receiver, nil
}

func (factory *pubsubReceiverFactory) CreateTracesReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Traces) (receiver.Traces, error) {

	if consumer == nil {
		return nil, component.ErrNilNextConsumer
	}
	err := cfg.(*Config).validateForTrace()
	if err != nil {
		return nil, err
	}
	receiver, err := factory.ensureReceiver(params, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	receiver.setTracesConsumer(consumer)
	return receiver, nil
}

func (factory *pubsubReceiverFactory) CreateMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics) (receiver.Metrics, error) {

	if consumer == nil {
		return nil, component.ErrNilNextConsumer
	}
	config := cfg.(*Config)
	err := config.validateForMetric()
	if err != nil {
		return nil, err
	}
	receiver, err := factory.ensureReceiver(params, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	receiver.setMetricsConsumer(consumer)
	return receiver, nil

}

func (factory *pubsubReceiverFactory) CreateLogsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs) (receiver.Logs, error) {

	if consumer == nil {
		return nil, component.ErrNilNextConsumer
	}
	err := cfg.(*Config).validateForLog()
	if err != nil {
		return nil, err
	}
	receiver, err := factory.ensureReceiver(params, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	receiver.setLogsConsumer(consumer)
	return receiver, nil
}
