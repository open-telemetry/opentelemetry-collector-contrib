// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type kineticaMetricsExporter struct {
	logger *zap.Logger
}

func newMetricsExporter(_ *zap.Logger, cfg *Config) (*kineticaMetricsExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	kineticaLogger := cfg.createLogger()
	metricsExp := &kineticaMetricsExporter{
		logger: kineticaLogger,
	}
	return metricsExp, nil
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
func (e *kineticaMetricsExporter) pushMetricsData(_ context.Context, md pmetric.Metrics) error {
	var metricType pmetric.MetricType
	var errs []error

	e.logger.Debug("Resource metrics ", zap.Int("count = ", md.ResourceMetrics().Len()))

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		metrics := md.ResourceMetrics().At(i)

		e.logger.Debug("Scope metrics ", zap.Int("count = ", metrics.ScopeMetrics().Len()))

		for j := 0; j < metrics.ScopeMetrics().Len(); j++ {
			metricSlice := metrics.ScopeMetrics().At(j).Metrics()

			e.logger.Debug("metrics ", zap.Int("count = ", metricSlice.Len()))

			for k := 0; k < metricSlice.Len(); k++ {

				metric := metricSlice.At(k)
				metricType = metric.Type()
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					e.logger.Debug("Added gauge")
				case pmetric.MetricTypeSum:
					e.logger.Debug("Added sum")
				case pmetric.MetricTypeHistogram:
					e.logger.Debug("Added histogram")
				case pmetric.MetricTypeExponentialHistogram:
					e.logger.Debug("Added exp histogram")
				case pmetric.MetricTypeSummary:
					e.logger.Debug("Added summary")
				default:
					return fmt.Errorf("Unsupported metrics type")
				}

				e.logger.Debug("Metric ", zap.String("count = ", metricType.String()))

				if len(errs) > 0 {
					e.logger.Error(multierr.Combine(errs...).Error())
					return multierr.Combine(errs...)
				}
			}
		}
	}

	return multierr.Combine(errs...)
}
