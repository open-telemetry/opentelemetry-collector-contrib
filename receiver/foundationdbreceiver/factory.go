// Copyright 2022 OpenTelemetry Authors
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

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
)

const (
	typeStr                 = "foundationdb"
	defaultAddress          = "localhost:8889"
	defaultMaxPacketSize    = 65_527 // max size for udp packet body (assuming ipv6)
	defaultSocketBufferSize = 0
	defaultFormat           = "opentelemetry"
)

// NewFactory creates a factory for the foundationDBReceiver.
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
        component.WithTracesReceiver(createTracesReceiver),
	)
}

func createTracesReceiver(ctx context.Context,
	settings component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Traces) (component.TracesReceiver, error) {
	c := cfg.(*Config)
	if consumer == nil {
		return nil, fmt.Errorf("nil consumer")
	}

	return NewFoundationDBReceiver(settings, c, consumer)
}

func NewFoundationDBReceiver(settings component.ReceiverCreateSettings, config *Config,
	consumer consumer.Traces) (component.TracesReceiver, error) {
	ts, err := NewUDPServer(config.Address, config.SocketBufferSize)
	if err != nil {
		return nil, err
	}
	obsrecv := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             config.ID(),
		Transport:              "udp",
		ReceiverCreateSettings: settings,
	})
	handler := createHandler(config, consumer, obsrecv)
	if handler == nil {
		return nil, fmt.Errorf("unable to create handler, tracing format %s unsupported", config.Format)
	}
	return &foundationDBReceiver{listener: ts, consumer: consumer, config: config, logger: settings.Logger, handler: handler}, nil
}

func createHandler(c *Config, consumer consumer.Traces, obsrecv *obsreport.Receiver) fdbTraceHandler {
	switch {
	case c.Format == OPENTELEMETRY:
      return &openTelemetryHandler{consumer: consumer, obsrecv: obsrecv}
	case c.Format == OPENTRACING:
      return &openTracingHandler{consumer: consumer, obsrecv: obsrecv}
	default:
		return nil
	}
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		Address:          defaultAddress,
		MaxPacketSize:    defaultMaxPacketSize,
		SocketBufferSize: defaultSocketBufferSize,
		Format:           defaultFormat,
	}
}
