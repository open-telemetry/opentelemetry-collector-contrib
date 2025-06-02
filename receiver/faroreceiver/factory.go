// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver/internal/metadata"
)

const (
	defaultFaroEndpoint = "localhost:8080"
)

// This is the map of already created Faro receivers for particular configurations.
// We maintain this map because the receiver.Factory is asked trace and log receivers separately
// when it gets createFaroReceiverTraces() and createFaroReceiverLogs() but they must not
// create separate objects, they must use one faroReceiver object per configuration.
// When the receiver is shutdown it should be removed from this map so the same configuration
// can be recreated successfully.
var receivers = sharedcomponent.NewSharedComponents()

func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: defaultFaroEndpoint,
		},
	}
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createFaroReceiverTraces, metadata.TracesStability),
		receiver.WithLogs(createFaroReceiverLogs, metadata.LogsStability))
}

func newFaroReceiverFactory(fCfg *Config, set *receiver.Settings, err *error) func() component.Component {
	return func() component.Component {
		var rcv component.Component
		rcv, *err = newFaroReceiver(fCfg, set)
		return rcv
	}
}

func createFaroReceiverTraces(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextTraces consumer.Traces,
) (receiver.Traces, error) {
	fCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid configuration: %T", cfg)
	}
	var err error
	receiver := receivers.GetOrAdd(fCfg, newFaroReceiverFactory(fCfg, &set, &err))
	if err != nil {
		return nil, err
	}

	receiver.Unwrap().(*faroReceiver).RegisterTracesConsumer(nextTraces)

	return receiver, nil
}

func createFaroReceiverLogs(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextLogs consumer.Logs,
) (receiver.Logs, error) {
	fCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid configuration: %T", cfg)
	}
	var err error
	receiver := receivers.GetOrAdd(fCfg, newFaroReceiverFactory(fCfg, &set, &err))
	if err != nil {
		return nil, err
	}

	receiver.Unwrap().(*faroReceiver).RegisterLogsConsumer(nextLogs)

	return receiver, nil
}
