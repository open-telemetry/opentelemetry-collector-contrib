// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import (
	"context"
	"net"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
	"golang.org/x/net/netutil"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/internal/metadata"
)

// Give the event channel a bit of buffer to help reduce backpressure on
// FluentBit and increase throughput.
const eventChannelLength = 100

const (
	defaultACKWaitTimeout = 30 * time.Second
	// defaultMaxConnections bounds the number of simultaneously accepted
	// connections. Each connection acknowledges its chunks serially (it blocks
	// until the pipeline responds before reading the next event), so limiting
	// connections also bounds the number of in-flight chunk acknowledgments and
	// keeps memory use bounded when clients delay behind pipeline backpressure.
	defaultMaxConnections = 100
)

type fluentReceiver struct {
	collector      *collector
	listener       net.Listener
	conf           *Config
	logger         *zap.Logger
	server         *server
	cancel         context.CancelFunc
	maxConnections int
}

func newFluentReceiver(set receiver.Settings, conf *Config, next consumer.Logs) (receiver.Logs, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "http",
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	eventCh := make(chan eventWithACK, eventChannelLength)
	collector := newCollector(eventCh, next, defaultACKWaitTimeout, set.Logger, obsrecv, telemetryBuilder)

	server := newServer(eventCh, defaultACKWaitTimeout, set.Logger, telemetryBuilder)

	return &fluentReceiver{
		collector:      collector,
		server:         server,
		conf:           conf,
		logger:         set.Logger,
		maxConnections: defaultMaxConnections,
	}, nil
}

func (r *fluentReceiver) Start(ctx context.Context, _ component.Host) error {
	receiverCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	r.collector.Start(receiverCtx)

	listenAddr := r.conf.ListenAddress

	var listener net.Listener
	var udpListener net.PacketConn
	var err error
	if strings.HasPrefix(listenAddr, "/") || strings.HasPrefix(listenAddr, "unix://") {
		listener, err = net.Listen("unix", strings.TrimPrefix(listenAddr, "unix://"))
	} else {
		listener, err = net.Listen("tcp", listenAddr)
		if err == nil {
			udpListener, err = net.ListenPacket("udp", listenAddr)
		}
	}

	if err != nil {
		return err
	}

	// Bound the number of simultaneously accepted connections. Excess
	// connections wait in the accept backlog until a slot frees, which keeps
	// the number of in-flight chunk acknowledgments bounded under load.
	if r.maxConnections > 0 {
		listener = netutil.LimitListener(listener, r.maxConnections)
	}

	r.listener = listener

	r.server.Start(receiverCtx, listener)

	if udpListener != nil {
		go respondToHeartbeats(receiverCtx, udpListener, r.logger)
	}

	return nil
}

func (r *fluentReceiver) Shutdown(context.Context) error {
	if r.listener == nil {
		return nil
	}
	// Close the listener to stop accepting new connections
	_ = r.listener.Close()
	r.server.closeAllConns()
	r.cancel()
	return nil
}
