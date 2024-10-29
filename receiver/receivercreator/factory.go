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
			observer.K8sIngressType: map[string]string{
				conventions.AttributeK8SNamespaceName: "`namespace`",
			},
			observer.PortType: map[string]string{
				conventions.AttributeK8SPodName:       "`pod.name`",
				conventions.AttributeK8SPodUID:        "`pod.uid`",
				conventions.AttributeK8SNamespaceName: "`pod.namespace`",
			},
			observer.PodContainerType: map[string]string{
				conventions.AttributeK8SPodName:         "`pod.name`",
				conventions.AttributeK8SPodUID:          "`pod.uid`",
				conventions.AttributeK8SNamespaceName:   "`pod.namespace`",
				conventions.AttributeK8SContainerName:   "`container_name`",
				conventions.AttributeContainerID:        "`container_id`",
				conventions.AttributeContainerImageName: "`container_image`",
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
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	r := receivers.GetOrAdd(cfg, func() component.Component {
		return newReceiverCreator(params, cfg.(*Config))
	})
	r.Component.(*receiverCreator).nextLogsConsumer = consumer
	return r, nil
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	r := receivers.GetOrAdd(cfg, func() component.Component {
		return newReceiverCreator(params, cfg.(*Config))
	})
	r.Component.(*receiverCreator).nextMetricsConsumer = consumer
	return r, nil
}

func createTracesReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Traces,
) (receiver.Traces, error) {
	r := receivers.GetOrAdd(cfg, func() component.Component {
		return newReceiverCreator(params, cfg.(*Config))
	})
	r.Component.(*receiverCreator).nextTracesConsumer = consumer
	return r, nil
}
