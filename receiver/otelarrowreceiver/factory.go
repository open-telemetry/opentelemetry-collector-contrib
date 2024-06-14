// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metadata"
)

const (
	defaultGRPCEndpoint = "0.0.0.0:4317"

	defaultMemoryLimitMiB    = 128
	defaultAdmissionLimitMiB = defaultMemoryLimitMiB / 2
	defaultWaiterLimit       = 1000
)

type receiverFactory struct {
	receivers *sharedcomponent.SharedComponents
}

// NewFactory creates a new OTel-Arrow receiver factory.
func NewFactory() receiver.Factory {
	f := &receiverFactory{
		receivers: sharedcomponent.NewSharedComponents(),
	}
	return receiver.NewFactory(
		metadata.Type,
		f.createDefaultConfig,
		receiver.WithTraces(f.createTraces, metadata.TracesStability),
		receiver.WithMetrics(f.createMetrics, metadata.MetricsStability),
		receiver.WithLogs(f.createLog, metadata.LogsStability))
}

// createDefaultConfig creates the default configuration for receiver.
func (f *receiverFactory) createDefaultConfig() component.Config {
	return &Config{
		Protocols: Protocols{
			GRPC: configgrpc.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Endpoint:  defaultGRPCEndpoint,
					Transport: confignet.TransportTypeTCP,
				},
				// We almost write 0 bytes, so no need to tune WriteBufferSize.
				ReadBufferSize: 512 * 1024,
			},
			Arrow: ArrowConfig{
				MemoryLimitMiB:    defaultMemoryLimitMiB,
				AdmissionLimitMiB: defaultAdmissionLimitMiB,
				WaiterLimit:       defaultWaiterLimit,
			},
		},
	}
}

// createTraces creates a trace receiver based on provided config.
func (f *receiverFactory) createTraces(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	oCfg := cfg.(*Config)
	var compErr error
	r := f.receivers.GetOrAdd(oCfg, func() component.Component {
		var comp component.Component
		comp, compErr = newOTelArrowReceiver(oCfg, set)
		return comp
	})
	if compErr != nil {
		return nil, compErr
	}

	r.Unwrap().(*otelArrowReceiver).registerTraceConsumer(nextConsumer)
	return r, nil
}

// createMetrics creates a metrics receiver based on provided config.
func (f *receiverFactory) createMetrics(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	oCfg := cfg.(*Config)
	var compErr error
	r := f.receivers.GetOrAdd(oCfg, func() component.Component {
		var comp component.Component
		comp, compErr = newOTelArrowReceiver(oCfg, set)
		return comp
	})
	if compErr != nil {
		return nil, compErr
	}

	r.Unwrap().(*otelArrowReceiver).registerMetricsConsumer(consumer)
	return r, nil
}

// createLog creates a log receiver based on provided config.
func (f *receiverFactory) createLog(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	oCfg := cfg.(*Config)
	var compErr error
	r := f.receivers.GetOrAdd(oCfg, func() component.Component {
		var comp component.Component
		comp, compErr = newOTelArrowReceiver(oCfg, set)
		return comp
	})
	if compErr != nil {
		return nil, compErr
	}

	r.Unwrap().(*otelArrowReceiver).registerLogsConsumer(consumer)
	return r, nil
}

// This is the map of already created OTel-Arrow receivers for particular configurations.
// We maintain this map because the Factory is asked trace and metric receivers separately
// when it gets CreateTracesReceiver() and CreateMetricsReceiver() but they must not
// create separate objects, they must use one otelArrowReceiver object per configuration.
// When the receiver is shutdown it should be removed from this map so the same configuration
// can be recreated successfully.
var receivers = sharedcomponent.NewSharedComponents()
