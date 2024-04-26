// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

const (
	defaultAddress          = "localhost:8889"
	defaultMaxPacketSize    = 65_527 // max size for udp packet body (assuming ipv6)
	defaultSocketBufferSize = 0
	defaultFormat           = OPENTELEMETRY
)

var (
	Type = component.MustNewType("foundationdb")
)

// NewFactory creates a factory for the foundationDBReceiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		Type,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, component.StabilityLevelBeta),
	)
}

func createTracesReceiver(ctx context.Context,
	settings receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Traces) (receiver.Traces, error) {
	c := cfg.(*Config)
	if consumer == nil {
		return nil, fmt.Errorf("nil consumer")
	}

	return NewFoundationDBReceiver(settings, c, consumer)
}

func NewFoundationDBReceiver(settings receiver.CreateSettings, config *Config,
	consumer consumer.Traces) (receiver.Traces, error) {
	ts, err := NewUDPServer(config.Address, config.SocketBufferSize)
	if err != nil {
		return nil, err
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "udp",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	handler := createHandler(config, consumer, obsrecv, settings.Logger)
	if handler == nil {
		return nil, fmt.Errorf("unable to create handler, tracing format %s unsupported", config.Format)
	}
	return &foundationDBReceiver{
		listener:               ts,
		consumer:               consumer,
		config:                 config,
		logger:                 settings.Logger,
		receiverCreateSettings: settings,
		handler:                handler,
	}, nil
}

func createHandler(c *Config, consumer consumer.Traces, obsrecv *receiverhelper.ObsReport, logger *zap.Logger) fdbTraceHandler {
	switch {
	case c.Format == OPENTELEMETRY:
		return &openTelemetryHandler{consumer: consumer, obsrecv: obsrecv, logger: logger}
	case c.Format == OPENTRACING:
		return &openTracingHandler{consumer: consumer, obsrecv: obsrecv, logger: logger}
	default:
		return nil
	}
}

func createDefaultConfig() component.Config {

	return &Config{
		Address:          defaultAddress,
		MaxPacketSize:    defaultMaxPacketSize,
		SocketBufferSize: defaultSocketBufferSize,
		Format:           defaultFormat,
	}
}
