// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal/metadata"
)

const (
	reportTransport      = "pubsub"
	reportFormatProtobuf = "protobuf"
)

func NewFactory() receiver.Factory {
	f := &pubsubReceiverFactory{
		receivers: make(map[*Config]*pubsubReceiver),
	}
	return receiver.NewFactory(
		metadata.Type,
		f.CreateDefaultConfig,
		receiver.WithTraces(f.CreateTracesReceiver, metadata.TracesStability),
		receiver.WithMetrics(f.CreateMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(f.CreateLogsReceiver, metadata.LogsStability),
	)
}

type pubsubReceiverFactory struct {
	receivers map[*Config]*pubsubReceiver
}

func (factory *pubsubReceiverFactory) CreateDefaultConfig() component.Config {
	return &Config{}
}

func (factory *pubsubReceiverFactory) ensureReceiver(params receiver.CreateSettings, config component.Config) (*pubsubReceiver, error) {
	receiver := factory.receivers[config.(*Config)]
	if receiver != nil {
		return receiver, nil
	}
	rconfig := config.(*Config)
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             params.ID,
		Transport:              reportTransport,
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}
	receiver = &pubsubReceiver{
		logger:    params.Logger,
		obsrecv:   obsrecv,
		userAgent: strings.ReplaceAll(rconfig.UserAgent, "{{version}}", params.BuildInfo.Version),
		config:    rconfig,
	}
	factory.receivers[config.(*Config)] = receiver
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
	receiver, err := factory.ensureReceiver(params, cfg)
	if err != nil {
		return nil, err
	}
	receiver.tracesConsumer = consumer
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
	err := cfg.(*Config).validateForMetric()
	if err != nil {
		return nil, err
	}
	receiver, err := factory.ensureReceiver(params, cfg)
	if err != nil {
		return nil, err
	}
	receiver.metricsConsumer = consumer
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
	receiver, err := factory.ensureReceiver(params, cfg)
	if err != nil {
		return nil, err
	}
	receiver.logsConsumer = consumer
	return receiver, nil
}
