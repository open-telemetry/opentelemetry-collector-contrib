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
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/parser"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"
)

var _ receiver.Metrics = (*statsdReceiver)(nil)

// statsdReceiver implements the receiver.Metrics for StatsD protocol.
type statsdReceiver struct {
	settings receiver.Settings
	config   *Config

	server       transport.Server
	reporter     *reporter
	obsrecv      *receiverhelper.ObsReport
	parser       parser.Parser
	nextConsumer consumer.Metrics
	cancel       context.CancelFunc
}

// newReceiver creates the StatsD receiver with the given parameters.
func newReceiver(
	set receiver.Settings,
	config Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	trans := transport.NewTransport(strings.ToLower(string(config.NetAddr.Transport)))

	if config.NetAddr.Endpoint == "" {
		if trans == transport.UDS {
			config.NetAddr.Endpoint = "/var/run/statsd-receiver.sock"
		} else {
			config.NetAddr.Endpoint = "localhost:8125"
		}
	}

	rep, err := newReporter(set)
	if err != nil {
		return nil, err
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		LongLivedCtx:           true,
		ReceiverID:             set.ID,
		ReceiverCreateSettings: set,
		Transport:              trans.String(),
	})
	if err != nil {
		return nil, err
	}

	r := &statsdReceiver{
		settings:     set,
		config:       &config,
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
		reporter:     rep,
		parser: &parser.StatsDParser{
			BuildInfo: set.BuildInfo,
		},
	}
	return r, nil
}

func buildTransportServer(config Config) (transport.Server, error) {
	trans := transport.NewTransport(strings.ToLower(string(config.NetAddr.Transport)))
	switch trans {
	case transport.UDP, transport.UDP4, transport.UDP6:
		return transport.NewUDPServer(trans, config.NetAddr.Endpoint)
	case transport.TCP, transport.TCP4, transport.TCP6:
		return transport.NewTCPServer(trans, config.NetAddr.Endpoint)
	case transport.UDS:
		return transport.NewUDSServer(trans, config.NetAddr.Endpoint, config.SocketPermissions)
	}

	return nil, fmt.Errorf("unsupported transport %q", string(config.NetAddr.Transport))
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
		r.config.EnableSimpleTags,
		r.config.IsMonotonicCounter,
		r.config.EnableIPOnlyAggregation,
		r.config.TimerHistogramMapping,
	)
	if err != nil {
		return err
	}
	go func() {
		if err := r.server.ListenAndServe(r.nextConsumer, r.reporter, transferChan); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			}
		}
	}()
	go func() {
		var failCnt, successCnt int64
		for {
			select {
			case <-ticker.C:
				batchMetrics := r.parser.GetMetrics()
				for _, batch := range batchMetrics {
					batchCtx := client.NewContext(ctx, batch.Info)
					numPoints := batch.Metrics.DataPointCount()
					flushCtx := r.obsrecv.StartMetricsOp(batchCtx)
					err := r.Flush(flushCtx, batch.Metrics, r.nextConsumer)
					if err != nil {
						r.reporter.OnDebugf("Error flushing metrics", zap.Error(err))
					}
					r.obsrecv.EndMetricsOp(flushCtx, metadata.Type.String(), numPoints, err)
				}
			case metric := <-transferChan:
				err := r.parser.Aggregate(metric.Raw, metric.Addr)
				if err != nil {
					failCnt++
					if failCnt%100 == 0 {
						r.reporter.RecordParseFailure()
						failCnt = 0
					}
					r.reporter.OnDebugf("Error aggregating pmetric", zap.Error(err))
				} else {
					successCnt++
					// Record every 100 to reduce overhead
					if successCnt%100 == 0 {
						r.reporter.RecordParseSuccess(successCnt)
						successCnt = 0
					}
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
