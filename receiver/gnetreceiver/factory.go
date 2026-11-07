// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnetreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnetreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	defaultEndpoint      = "0.0.0.0:4320"
	defaultNumEventLoops = 0    // 0 = one per CPU core
	defaultReadBufferCap = 4096 // bytes
)

// NewFactory creates a factory for the gnet receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType("gnet"),
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, component.StabilityLevelDevelopment),
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelDevelopment),
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint:      defaultEndpoint,
		NumEventLoops: defaultNumEventLoops,
		ReadBufferCap: defaultReadBufferCap,
	}
}

func createTracesReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	next consumer.Traces,
) (receiver.Traces, error) {
	r, err := newReceiver(params, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	r.nextTraces = next
	return r, nil
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	r, err := newReceiver(params, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	r.nextMetrics = next
	return r, nil
}

func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	r, err := newReceiver(params, cfg.(*Config))
	if err != nil {
		return nil, err
	}
	r.nextLogs = next
	return r, nil
}
