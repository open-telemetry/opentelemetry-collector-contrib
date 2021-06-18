// Copyright 2019, OpenTelemetry Authors
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

package carbonreceiver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport"
)

var (
	errEmptyEndpoint = errors.New("empty endpoint")
)

// carbonreceiver implements a component.MetricsReceiver for Carbon plaintext, aka "line", protocol.
// see https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol.
type carbonReceiver struct {
	logger *zap.Logger
	config *Config

	server       transport.Server
	reporter     transport.Reporter
	parser       protocol.Parser
	nextConsumer consumer.Metrics
}

var _ component.MetricsReceiver = (*carbonReceiver)(nil)

// New creates the Carbon receiver with the given configuration.
func New(
	logger *zap.Logger,
	config Config,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {

	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	if config.Endpoint == "" {
		return nil, errEmptyEndpoint
	}

	if config.Parser == nil {
		// Set the defaults
		config.Parser = &protocol.Config{
			Type:   "plaintext",
			Config: &protocol.PlaintextConfig{},
		}
	}

	parser, err := config.Parser.Config.BuildParser()
	if err != nil {
		return nil, err
	}

	// This should be the last one built, or if any other error is raised after
	// it, the server should be closed.
	server, err := buildTransportServer(config)
	if err != nil {
		return nil, err
	}

	r := carbonReceiver{
		logger:       logger,
		config:       &config,
		nextConsumer: nextConsumer,
		server:       server,
		reporter:     newReporter(config.ID(), logger),
		parser:       parser,
	}

	return &r, nil
}

func buildTransportServer(config Config) (transport.Server, error) {
	switch strings.ToLower(config.Transport) {
	case "", "tcp":
		return transport.NewTCPServer(config.Endpoint, config.TCPIdleTimeout)
	case "udp":
		return transport.NewUDPServer(config.Endpoint)
	}

	return nil, fmt.Errorf("unsupported transport %q for receiver %v", config.Transport, config.ID())
}

// Start tells the receiver to start its processing.
// By convention the consumer of the received data is set when the receiver
// instance is created.
func (r *carbonReceiver) Start(_ context.Context, host component.Host) error {
	go func() {
		if err := r.server.ListenAndServe(r.parser, r.nextConsumer, r.reporter); err != nil {
			host.ReportFatalError(err)
		}
	}()
	return nil
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up.
func (r *carbonReceiver) Shutdown(context.Context) error {
	return r.server.Close()
}
