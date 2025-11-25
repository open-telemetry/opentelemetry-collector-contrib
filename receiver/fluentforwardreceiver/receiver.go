// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import (
	"context"
	"net"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/internal/metadata"
)

// Give the event channel a bit of buffer to help reduce backpressure on
// FluentBit and increase throughput.
const eventChannelLength = 100

type fluentReceiver struct {
	collector *collector
	listener  net.Listener
	conf      *Config
	logger    *zap.Logger
	server    *server
	udpSocket net.PacketConn
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

	eventCh := make(chan event, eventChannelLength)
	collector := newCollector(eventCh, next, set.Logger, obsrecv, telemetryBuilder)

	server := newServer(eventCh, set.Logger, telemetryBuilder)

	return &fluentReceiver{
		collector: collector,
		server:    server,
		conf:      conf,
		logger:    set.Logger,
	}, nil
}

func (r *fluentReceiver) Start(_ context.Context, _ component.Host) error {
	r.collector.Start()

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

	r.listener = listener

	r.server.Start(listener)

	if udpListener != nil {
		r.udpSocket = udpListener
		go respondToHeartbeats(udpListener, r.logger)
	}

	return nil
}

func (r *fluentReceiver) Shutdown(context.Context) error {
	if r.listener != nil {
		_ = r.listener.Close() // stop TCP traffic
	}
	if r.udpSocket != nil {
		_ = r.udpSocket.Close()
	}
	r.server.Shutdown() // stop accepting data
	r.collector.Shutdown()
	return nil
}
