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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/transport"
)

var _ component.MetricsReceiver = (*statsdReceiver)(nil)

// statsdReceiver implements the component.MetricsReceiver for StatsD protocol.
type statsdReceiver struct {
	settings component.ReceiverCreateSettings
	config   *Config

	server       transport.Server
	reporter     transport.Reporter
	parser       protocol.Parser
	nextConsumer consumer.Metrics
	cancel       context.CancelFunc
}

// New creates the StatsD receiver with the given parameters.
func New(
	set component.ReceiverCreateSettings,
	config Config,
	nextConsumer consumer.Metrics,
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
		settings:     set,
		config:       &config,
		nextConsumer: nextConsumer,
		server:       server,
		reporter:     newReporter(config.ID(), set),
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

	return nil, fmt.Errorf("unsupported transport %q for receiver %v", config.NetAddr.Transport, config.ID())
}

// Start starts a UDP server that can process StatsD messages.
func (r *statsdReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)
	var transferChan = make(chan string, 10)
	ticker := time.NewTicker(r.config.AggregationInterval)
	r.parser.Initialize(r.config.EnableMetricType, r.config.IsMonotonicCounter, r.config.TimerHistogramMapping)
	go func() {
		if err := r.server.ListenAndServe(r.parser, r.nextConsumer, r.reporter, transferChan); err != nil {
			host.ReportFatalError(err)
		}
	}()
	go func() {
		for {
			select {
			case <-ticker.C:
				metrics := r.parser.GetMetrics()
				if metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().Len() > 0 {
					r.Flush(ctx, metrics, r.nextConsumer)
				}
			case rawMetric := <-transferChan:
				r.parser.Aggregate(rawMetric)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// Shutdown stops the StatsD receiver.
func (r *statsdReceiver) Shutdown(context.Context) error {
	err := r.server.Close()
	r.cancel()
	return err
}

func (r *statsdReceiver) Flush(ctx context.Context, metrics pdata.Metrics, nextConsumer consumer.Metrics) error {
	error := nextConsumer.ConsumeMetrics(ctx, metrics)
	if error != nil {
		return error
	}

	return nil
}
