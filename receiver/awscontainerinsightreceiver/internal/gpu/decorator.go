// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

const (
	gpuUtil        = "DCGM_FI_DEV_GPU_UTIL"
	gpuMemUtil     = "DCGM_FI_DEV_FB_USED_PERCENT"
	gpuMemUsed     = "DCGM_FI_DEV_FB_USED"
	gpuMemTotal    = "DCGM_FI_DEV_FB_TOTAL"
	gpuTemperature = "DCGM_FI_DEV_GPU_TEMP"
	gpuPowerDraw   = "DCGM_FI_DEV_POWER_USAGE"
)

var metricToUnit = map[string]string{
	gpuUtil:        "Percent",
	gpuMemUtil:     "Percent",
	gpuMemUsed:     "Bytes",
	gpuMemTotal:    "Bytes",
	gpuTemperature: "None",
	gpuPowerDraw:   "None",
}

// GPU decorator acts as an interceptor of metrics before the scraper sends them to the next designated consumer
type decorateConsumer struct {
	containerOrchestrator string
	nextConsumer          consumer.Metrics
	k8sDecorator          Decorator
	logger                *zap.Logger
}

func (dc *decorateConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (dc *decorateConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	resourceTags := make(map[string]string)
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		// get resource attributes
		ras := rms.At(i).Resource().Attributes()
		ras.Range(func(k string, v pcommon.Value) bool {
			resourceTags[k] = v.AsString()
			return true
		})
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ms := ilms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				converted := ci.ConvertToFieldsAndTags(m, dc.logger)
				var rcis []*stores.RawContainerInsightsMetric
				for _, pair := range converted {
					rcis = append(rcis, stores.NewRawContainerInsightsMetricWithData(ci.TypeGpuContainer, pair.Fields, pair.Tags, dc.logger))
				}

				decorated := dc.decorateMetrics(rcis)
				dc.updateAttributes(m, decorated)
				if unit, ok := metricToUnit[m.Name()]; ok {
					m.SetUnit(unit)
				}
			}
		}
	}
	return dc.nextConsumer.ConsumeMetrics(ctx, md)
}

type Decorator interface {
	Decorate(stores.CIMetric) stores.CIMetric
	Shutdown() error
}

func (dc *decorateConsumer) decorateMetrics(rcis []*stores.RawContainerInsightsMetric) []*stores.RawContainerInsightsMetric {
	var result []*stores.RawContainerInsightsMetric
	if dc.containerOrchestrator != ci.EKS {
		return result
	}
	for _, rci := range rcis {
		// add tags for EKS
		out := dc.k8sDecorator.Decorate(rci)
		if out != nil {
			result = append(result, out.(*stores.RawContainerInsightsMetric))
		}
	}
	return result
}

func (dc *decorateConsumer) updateAttributes(m pmetric.Metric, rcis []*stores.RawContainerInsightsMetric) {
	if len(rcis) == 0 {
		return
	}
	var dps pmetric.NumberDataPointSlice
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps = m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = m.Sum().DataPoints()
	default:
		dc.logger.Warn("Unsupported metric type", zap.String("metric", m.Name()), zap.String("type", m.Type().String()))
	}
	if dps.Len() == 0 {
		return
	}
	for i := 0; i < dps.Len(); i++ {
		if i >= len(rcis) {
			// this shouldn't be the case, but it helps to avoid panic
			continue
		}
		attrs := dps.At(i).Attributes()
		tags := rcis[i].Tags
		for tk, tv := range tags {
			// type gets set with metrictransformer while duplicating metrics at different resource levels
			if tk == ci.MetricType {
				continue
			}
			attrs.PutStr(tk, tv)
		}
	}
}

func (dc *decorateConsumer) Shutdown() error {
	if dc.k8sDecorator != nil {
		return dc.k8sDecorator.Shutdown()
	}
	return nil
}
