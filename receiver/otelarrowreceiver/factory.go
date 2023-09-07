// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	defaultGRPCEndpoint = "0.0.0.0:4317"
	defaultHTTPEndpoint = "0.0.0.0:4318"

	defaultTracesURLPath  = "/v1/traces"
	defaultMetricsURLPath = "/v1/metrics"
	defaultLogsURLPath    = "/v1/logs"

	defaultMemoryLimit = 128 << 20 // 128MB
)

// NewFactory creates a new OTLP receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createTraces, metadata.TracesStability),
		receiver.WithMetrics(createMetrics, metadata.MetricsStability),
		receiver.WithLogs(createLogs, metadata.LogsStability),
	)
}

// createDefaultConfig creates the default configuration for receiver.
func createDefaultConfig() component.Config {
	return &Config{
		Protocols: Protocols{
			GRPC: &configgrpc.GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint:  defaultGRPCEndpoint,
					Transport: "tcp",
				},
				// We almost write 0 bytes, so no need to tune WriteBufferSize.
				ReadBufferSize: 512 * 1024,
			},
			HTTP: &httpServerSettings{
				HTTPServerSettings: &confighttp.HTTPServerSettings{
					Endpoint: defaultHTTPEndpoint,
				},
				TracesURLPath:  defaultTracesURLPath,
				MetricsURLPath: defaultMetricsURLPath,
				LogsURLPath:    defaultLogsURLPath,
			},
			Arrow: &ArrowSettings{
				MemoryLimit: defaultMemoryLimit,
			},
		},
	}
}

// createTraces creates a trace receiver based on provided config.
func createTraces(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	return nil, fmt.Errorf("unimplemented")
}

// createMetrics creates a metrics receiver based on provided config.
func createMetrics(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	return nil, fmt.Errorf("unimplemented")
}

// createLog creates a log receiver based on provided config.
func createLogs(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	return nil, fmt.Errorf("unimplemented")
}
