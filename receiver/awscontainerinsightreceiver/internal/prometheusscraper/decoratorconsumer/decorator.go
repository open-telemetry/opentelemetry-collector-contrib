// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package decoratorconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper/decoratorconsumer"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

// Decorator acts as an interceptor of metrics before the scraper sends them to the next designated consumer
type DecorateConsumer struct {
	ContainerOrchestrator string
	NextConsumer          consumer.Metrics
	K8sDecorator          Decorator
	MetricType            string
	MetricToUnitMap       map[string]string
	Logger                *zap.Logger
}

func (dc *DecorateConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (dc *DecorateConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
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
				converted := ci.ConvertToFieldsAndTags(m, dc.Logger)
				var rcis []*stores.CIMetricImpl
				for _, pair := range converted {
					rcis = append(rcis, stores.NewCIMetricWithData(dc.MetricType, pair.Fields, pair.Tags, dc.Logger))
				}

				decorated := dc.decorateMetrics(rcis)
				dc.updateAttributes(m, decorated)
				if unit, ok := dc.MetricToUnitMap[m.Name()]; ok {
					m.SetUnit(unit)
				}
			}
		}
	}
	return dc.NextConsumer.ConsumeMetrics(ctx, md)
}

type Decorator interface {
	Decorate(stores.CIMetric) stores.CIMetric
	Shutdown() error
}

func (dc *DecorateConsumer) decorateMetrics(rcis []*stores.CIMetricImpl) []*stores.CIMetricImpl {
	var result []*stores.CIMetricImpl
	if dc.ContainerOrchestrator != ci.EKS {
		return result
	}
	for _, rci := range rcis {
		// add tags for EKS
		out := dc.K8sDecorator.Decorate(rci)
		if out != nil {
			result = append(result, out.(*stores.CIMetricImpl))
		}
	}
	return result
}

func (dc *DecorateConsumer) updateAttributes(m pmetric.Metric, rcis []*stores.CIMetricImpl) {
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
		dc.Logger.Warn("Unsupported metric type", zap.String("metric", m.Name()), zap.String("type", m.Type().String()))
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

func (dc *DecorateConsumer) Shutdown() error {
	if dc.K8sDecorator != nil {
		return dc.K8sDecorator.Shutdown()
	}
	return nil
}
