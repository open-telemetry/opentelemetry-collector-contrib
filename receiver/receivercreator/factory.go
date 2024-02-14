// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal/metadata"
)

// This file implements factory for receiver_creator. A receiver_creator can create other receivers at runtime.

var receivers = sharedcomponent.NewSharedComponents()

// NewFactory creates a factory for receiver creator.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ResourceAttributes: resourceAttributes{
			observer.PodType: map[string]string{
				conventions.AttributeK8SPodName:       "`name`",
				conventions.AttributeK8SPodUID:        "`uid`",
				conventions.AttributeK8SNamespaceName: "`namespace`",
			},
			observer.K8sServiceType: map[string]string{
				conventions.AttributeK8SNamespaceName: "`namespace`",
			},
			observer.PortType: map[string]string{
				conventions.AttributeK8SPodName:       "`pod.name`",
				conventions.AttributeK8SPodUID:        "`pod.uid`",
				conventions.AttributeK8SNamespaceName: "`pod.namespace`",
			},
			observer.ContainerType: map[string]string{
				conventions.AttributeContainerName:      "`name`",
				conventions.AttributeContainerImageName: "`image`",
			},
			observer.K8sNodeType: map[string]string{
				conventions.AttributeK8SNodeName: "`name`",
				conventions.AttributeK8SNodeUID:  "`uid`",
			},
		},
		receiverTemplates: map[string]receiverTemplate{},
	}
}

func createLogsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	var err error
	var recv receiver.Logs
	rCfg := cfg.(*Config)
	r := receivers.GetOrAdd(cfg, func() component.Component {
		recv, err = newLogsReceiverCreator(params, rCfg, consumer)
		return recv
	})
	rcvr := r.Component.(*receiverCreator)
	if rcvr.nextLogsConsumer == nil {
		rcvr.nextLogsConsumer = consumer
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	var err error
	var recv receiver.Logs
	rCfg := cfg.(*Config)
	r := receivers.GetOrAdd(cfg, func() component.Component {
		recv, err = newMetricsReceiverCreator(params, rCfg, consumer)
		return recv
	})
	rcvr := r.Component.(*receiverCreator)
	if rcvr.nextMetricsConsumer == nil {
		rcvr.nextMetricsConsumer = consumer
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func createTracesReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Traces,
) (receiver.Traces, error) {
	var err error
	var recv receiver.Logs
	rCfg := cfg.(*Config)
	r := receivers.GetOrAdd(cfg, func() component.Component {
		recv, err = newTracesReceiverCreator(params, rCfg, consumer)
		return recv
	})
	rcvr := r.Component.(*receiverCreator)
	if rcvr.nextTracesConsumer == nil {
		rcvr.nextTracesConsumer = consumer
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}
