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
	parser       protocol.Parser
	nextConsumer consumer.MetricsConsumerOld

	startOnce sync.Once
	stopOnce  sync.Once
}

// New creates the StatsD receiver with the given parameters.
func New(
	logger *zap.Logger,
	config Config,
	nextConsumer consumer.MetricsConsumerOld) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	if config.Endpoint == "" {
		config.Endpoint = "localhost:8125"
	}

	server, err := transport.NewUDPServer(config.Endpoint)
	if err != nil {
		return nil, err
	}

	r := &statsdReceiver{
		logger:       logger,
		config:       &config,
		parser:       &protocol.StatsDParser{},
		nextConsumer: nextConsumer,
		server:       server,
	}
	return r, nil
}

// StartMetricsReception starts a UDP server that can process StatsD messages.
func (ddr *statsdReceiver) Start(_ context.Context, host component.Host) error {
	ddr.Lock()
	defer ddr.Unlock()

	err := componenterror.ErrAlreadyStarted
	ddr.startOnce.Do(func() {
		err = nil
		go func() {
			err = ddr.server.ListenAndServe(ddr.parser, ddr.nextConsumer)
			if err != nil {
				host.ReportFatalError(err)
			}
		}()
	})

	return err
}

// StopMetricsReception stops the StatsD receiver.
func (ddr *statsdReceiver) Shutdown(context.Context) error {
	ddr.Lock()
	defer ddr.Unlock()

	var err = componenterror.ErrAlreadyStopped
	ddr.stopOnce.Do(func() {
		err = ddr.server.Close()
	})
	return err
}
