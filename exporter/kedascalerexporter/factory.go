// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kedascalerexporter

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kedascalerexporter/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		CreateDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability))
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	pcfg := cfg.(*Config)

	keda, err := newKedaScalerExporter(ctx, pcfg, set)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		keda.ConsumeMetrics,
		exporterhelper.WithStart(keda.Start),
		exporterhelper.WithShutdown(keda.Shutdown),
	)
}
