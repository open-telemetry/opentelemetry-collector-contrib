// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel2influx // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/otel2influx"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/common"
)

type metricWriter interface {
	writeMetric(ctx context.Context, resource pcommon.Resource, instrumentationScope pcommon.InstrumentationScope, metric pmetric.Metric, batch InfluxWriterBatch) error
}

type OtelMetricsToLineProtocol struct {
	iw InfluxWriter
	mw metricWriter
}

func NewOtelMetricsToLineProtocol(logger common.Logger, iw InfluxWriter, schema common.MetricsSchema) (*OtelMetricsToLineProtocol, error) {
	var mw metricWriter
	switch schema {
	case common.MetricsSchemaTelegrafPrometheusV1:
		mw = &metricWriterTelegrafPrometheusV1{
			logger: logger,
		}
	case common.MetricsSchemaTelegrafPrometheusV2:
		mw = &metricWriterTelegrafPrometheusV2{
			logger: logger,
		}
	case common.MetricsSchemaOtelV1:
		mw = &metricWriterOtelV1{
			logger: logger,
		}
	default:
		return nil, fmt.Errorf("unrecognized metrics schema %d", schema)
	}
	return &OtelMetricsToLineProtocol{
		iw: iw,
		mw: mw,
	}, nil
}

func (c *OtelMetricsToLineProtocol) WriteMetrics(ctx context.Context, md pmetric.Metrics) error {
	batch := c.iw.NewBatch()
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			ilMetrics := resourceMetrics.ScopeMetrics().At(j)
			for k := 0; k < ilMetrics.Metrics().Len(); k++ {
				metric := ilMetrics.Metrics().At(k)
				if err := c.mw.writeMetric(ctx, resourceMetrics.Resource(), ilMetrics.Scope(), metric, batch); err != nil {
					return fmt.Errorf("failed to convert OTLP metric to line protocol: %w", err)
				}
			}
		}
	}
	return batch.FlushBatch(ctx)
}
