// Copyright 2020, OpenTelemetry Authors
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

package statsdreceiver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/transport"
)

var _ component.MetricsReceiver = (*statsdReceiver)(nil)

// statsdReceiver implements the component.MetricsReceiver for StatsD protocol.
type statsdReceiver struct {
	sync.Mutex
	logger *zap.Logger
	config *Config

	server       transport.Server
	reporter     transport.Reporter
	parser       protocol.Parser
	nextConsumer consumer.MetricsConsumer

	startOnce sync.Once
	stopOnce  sync.Once
}

// New creates the StatsD receiver with the given parameters.
func New(
	logger *zap.Logger,
	config Config,
	nextConsumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	if config.NetAddr.Endpoint == "" {
		config.NetAddr.Endpoint = "localhost:8125"
	}

	server, err := buildTransportServer(config)
	if err != nil {
		return nil, err
	}

	r := &statsdReceiver{
		logger:       logger,
		config:       &config,
		nextConsumer: nextConsumer,
		server:       server,
		reporter:     newReporter(config.Name(), logger),
		parser:       &protocol.StatsDParser{},
	}
	return r, nil
}

func buildTransportServer(config Config) (transport.Server, error) {
	// TODO: Add TCP/unix socket transport implementations
	switch strings.ToLower(config.NetAddr.Transport) {
	case "", "udp":
		return transport.NewUDPServer(config.NetAddr.Endpoint)
	}

	return nil, fmt.Errorf("unsupported transport %q for receiver %q", config.NetAddr.Transport, config.Name())
}

// StartMetricsReception starts a UDP server that can process StatsD messages.
func (r *statsdReceiver) Start(_ context.Context, host component.Host) error {
	r.Lock()
	defer r.Unlock()

	err := componenterror.ErrAlreadyStarted
	r.startOnce.Do(func() {
		err = nil
		go func() {
			err = r.server.ListenAndServe(r.parser, r.nextConsumer, r.reporter)
			if err != nil {
				host.ReportFatalError(err)
			}
		}()
	})

	return err
}

// StopMetricsReception stops the StatsD receiver.
func (r *statsdReceiver) Shutdown(context.Context) error {
	r.Lock()
	defer r.Unlock()

	var err = componenterror.ErrAlreadyStopped
	r.stopOnce.Do(func() {
		err = r.server.Close()
	})
	return err
}
