// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createTracesReceiver(ctx context.Context, settings receiver.Settings, cc component.Config, consumer consumer.Traces) (receiver.Traces, error) {
	return newAWSS3TraceReceiver(ctx, cc.(*Config), consumer, settings)
}

func createMetricsReceiver(ctx context.Context, settings receiver.Settings, cc component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	return newAWSS3MetricsReceiver(ctx, cc.(*Config), consumer, settings)
}

func createLogsReceiver(ctx context.Context, settings receiver.Settings, cc component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	return newAWSS3LogsReceiver(ctx, cc.(*Config), consumer, settings)
}
