// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type kineticaMetricsExporter struct {
	logger *zap.Logger
}

func newMetricsExporter(_ *zap.Logger, _ *Config) *kineticaMetricsExporter {
	metricsExp := &kineticaMetricsExporter{
		logger: nil,
	}
	return metricsExp
}

func (e *kineticaMetricsExporter) start(_ context.Context, _ component.Host) error {

	return nil
}

// shutdown will shut down the exporter.
func (e *kineticaMetricsExporter) shutdown(_ context.Context) error {
	return nil
}

// pushMetricsData - this method is called by the collector to feed the metrics data to the exporter
//
//	@receiver e
//	@param _ ctx unused
//	@param md
//	@return error
func (e *kineticaMetricsExporter) pushMetricsData(_ context.Context, _ pmetric.Metrics) error {

	return nil
}
