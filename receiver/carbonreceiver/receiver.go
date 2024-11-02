// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/transport"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

var (
	errEmptyEndpoint = errors.New("empty endpoint")
)

// carbonreceiver implements a receiver.Metrics for Carbon plaintext, aka "line", protocol.
// see https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol.
type carbonReceiver struct {
	settings receiver.Settings
	config   *Config

	server       transport.Server
	reporter     transport.Reporter
	parser       protocol.Parser
	nextConsumer consumer.Metrics
}

var _ receiver.Metrics = (*carbonReceiver)(nil)

// newMetricsReceiver creates the Carbon receiver with the given configuration.
func newMetricsReceiver(
	set receiver.Settings,
	config Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
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

	rep, err := newReporter(set)
	if err != nil {
		return nil, err
	}

	r := carbonReceiver{
		settings:     set,
		config:       &config,
		nextConsumer: nextConsumer,
		reporter:     rep,
		parser:       parser,
	}

	return &r, nil
}

func buildTransportServer(config Config) (transport.Server, error) {
	switch strings.ToLower(string(config.Transport)) {
	case "", "tcp":
		return transport.NewTCPServer(config.Endpoint, config.TCPIdleTimeout)
	case "udp":
		return transport.NewUDPServer(config.Endpoint)
	}

	return nil, fmt.Errorf("unsupported transport %q", string(config.Transport))
}

// Start tells the receiver to start its processing.
// By convention the consumer of the received data is set when the receiver
// instance is created.
func (r *carbonReceiver) Start(_ context.Context, host component.Host) error {
	server, err := buildTransportServer(*r.config)
	if err != nil {
		return err
	}
	r.server = server
	go func() {
		if err := r.server.ListenAndServe(r.parser, r.nextConsumer, r.reporter); err != nil && !errors.Is(err, net.ErrClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up.
func (r *carbonReceiver) Shutdown(context.Context) error {
	if r.server == nil {
		return nil
	}
	return r.server.Close()
}
