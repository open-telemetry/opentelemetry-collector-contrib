// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricsProcessor struct {
	*ecsCore
	next consumer.Metrics
}

func newMetricsProcessor(logger *zap.Logger, cfg *Config, next consumer.Metrics, endpoints endpointsFn) *metricsProcessor {
	return &metricsProcessor{ecsCore: newCore(logger, cfg, endpoints), next: next}
}

func (p *metricsProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	rms := md.ResourceMetrics()
	for i := range rms.Len() {
		p.enrichResource(ctx, rms.At(i).Resource())
	}
	return p.next.ConsumeMetrics(ctx, md)
}
