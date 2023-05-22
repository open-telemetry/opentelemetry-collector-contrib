// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver/internal/metadata"
)

// This file implements factory for SignalFx receiver.

const (

	// Default endpoints to bind to.
	defaultEndpoint = ":9943"
)

// NewFactory creates a factory for SignalFx receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: defaultEndpoint,
		},
	}
}

// extract the port number from string in "address:port" format. If the
// port number cannot be extracted returns an error.
func extractPortFromEndpoint(endpoint string) (int, error) {
	_, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return 0, fmt.Errorf("endpoint is not formatted correctly: %w", err)
	}
	port, err := strconv.ParseInt(portStr, 10, 0)
	if err != nil {
		return 0, fmt.Errorf("endpoint port is not a number: %w", err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port number must be between 1 and 65535")
	}
	return int(port), nil
}

// createMetricsReceiver creates a metrics receiver based on provided config.
func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	rCfg := cfg.(*Config)

	receiverLock.Lock()
	r := receivers[rCfg]
	if r == nil {
		var err error
		r, err = newReceiver(params, *rCfg)
		if err != nil {
			return nil, err
		}
		receivers[rCfg] = r
	}
	receiverLock.Unlock()

	r.RegisterMetricsConsumer(consumer)

	return r, nil
}

// createLogsReceiver creates a logs receiver based on provided config.
func createLogsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	rCfg := cfg.(*Config)

	receiverLock.Lock()
	r := receivers[rCfg]
	if r == nil {
		var err error
		r, err = newReceiver(params, *rCfg)
		if err != nil {
			return nil, err
		}
		receivers[rCfg] = r
	}
	receiverLock.Unlock()

	r.RegisterLogsConsumer(consumer)

	return r, nil
}

var receiverLock sync.Mutex
var receivers = map[*Config]*sfxReceiver{}
