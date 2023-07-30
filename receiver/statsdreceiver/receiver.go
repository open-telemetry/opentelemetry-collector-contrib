// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/transport"
)

var _ receiver.Metrics = (*statsdReceiver)(nil)

// statsdReceiver implements the receiver.Metrics for StatsD protocol.
type statsdReceiver struct {
	settings receiver.CreateSettings
	config   *Config

	server       transport.Server
	reporter     transport.Reporter
	parser       protocol.Parser
	nextConsumer consumer.Metrics
	cancel       context.CancelFunc
}

// New creates the StatsD receiver with the given parameters.
func New(
	set receiver.CreateSettings,
	config Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	if config.NetAddr.Endpoint == "" {
		config.NetAddr.Endpoint = "localhost:8125"
	}

	rep, err := newReporter(set)
	if err != nil {
		return nil, err
	}

	r := &statsdReceiver{
		settings:     set,
		config:       &config,
		nextConsumer: nextConsumer,
		reporter:     rep,
		parser: &protocol.StatsDParser{
			BuildInfo: set.BuildInfo,
		},
	}
	return r, nil
}

func buildTransportServer(config Config) (transport.Server, error) {
	// TODO: Add TCP/unix socket transport implementations
	switch strings.ToLower(config.NetAddr.Transport) {
	case "", "udp":
		return transport.NewUDPServer(config.NetAddr.Endpoint)
	}

	return nil, fmt.Errorf("unsupported transport %q", config.NetAddr.Transport)
}

// Start starts a UDP server that can process StatsD messages.
func (r *statsdReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)
	server, err := buildTransportServer(*r.config)
	if err != nil {
		return err
	}
	r.server = server
	transferChan := make(chan transport.Metric, 10)
	ticker := time.NewTicker(r.config.AggregationInterval)
	err = r.parser.Initialize(
		r.config.EnableMetricType,
		r.config.IsMonotonicCounter,
		r.config.TimerHistogramMapping,
	)
	if err != nil {
		return err
	}
	go func() {
		if err := r.server.ListenAndServe(r.parser, r.nextConsumer, r.reporter, transferChan); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				host.ReportFatalError(err)
			}
		}
	}()
	go func() {
		for {
			select {
			case <-ticker.C:
				batchMetrics := r.parser.GetMetrics()
				for _, batch := range batchMetrics {
					batchCtx := client.NewContext(ctx, batch.Info)

					if err := r.Flush(batchCtx, batch.Metrics, r.nextConsumer); err != nil {
						r.reporter.OnDebugf("Error flushing metrics", zap.Error(err))
					}
				}
			case metric := <-transferChan:
				if err := r.parser.Aggregate(metric.Raw, metric.Addr); err != nil {
					r.reporter.OnDebugf("Error aggregating metric", zap.Error(err))
				}
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
	if r.cancel == nil || r.server == nil {
		return nil
	}
	err := r.server.Close()
	r.cancel()
	return err
}

func (r *statsdReceiver) Flush(ctx context.Context, metrics pmetric.Metrics, nextConsumer consumer.Metrics) error {
	return nextConsumer.ConsumeMetrics(ctx, metrics)
}
