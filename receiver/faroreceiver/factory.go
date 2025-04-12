// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver/internal/metadata"
)

const (
	defaultFaroEndpoint = "localhost:8080"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createFaroReceiverTraces, metadata.TracesStability),
		receiver.WithLogs(createFaroReceiverLogs, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: defaultFaroEndpoint,
		},
	}
}

func createFaroReceiverTraces(
	_ context.Context,
	_ receiver.Settings,
	conf component.Config,
	consumer consumer.Traces,
) (receiver.Traces, error) {
	cfg := conf.(*Config)
	return newTraceReceiverTraces(cfg, consumer), nil
}

func createFaroReceiverLogs(
	_ context.Context,
	_ receiver.Settings,
	conf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := conf.(*Config)
	return newTraceReceiverLogs(cfg, consumer), nil
}
