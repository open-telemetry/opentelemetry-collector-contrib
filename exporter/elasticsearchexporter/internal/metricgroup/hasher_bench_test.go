// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricgroup

import (
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
)

func BenchmarkDataPointHasher(b *testing.B) {
	cases := []struct {
		name           string
		mode           string
		resourceAttrs  int
		dataPointAttrs int
	}{
		{name: "ecs_r10_dp5", mode: "ecs", resourceAttrs: 10, dataPointAttrs: 5},
		{name: "ecs_r30_dp10", mode: "ecs", resourceAttrs: 30, dataPointAttrs: 10},
		{name: "ecs_r50_dp20", mode: "ecs", resourceAttrs: 50, dataPointAttrs: 20},
		{name: "otel_r30_dp10", mode: "otel", resourceAttrs: 30, dataPointAttrs: 10},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			resource, dp := newBenchDataPoint(tc.resourceAttrs, tc.dataPointAttrs)
			var hasher DataPointHasher
			if tc.mode == "ecs" {
				hasher = &ECSDataPointHasher{}
			} else {
				hasher = &OTelDataPointHasher{}
			}
			hasher.UpdateResource(resource)
			hasher.UpdateScope(pcommon.NewInstrumentationScope())
			hasher.UpdateDataPoint(dp)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				// Match pushMetricsData: UpdateDataPoint + HashKey per DP.
				hasher.UpdateDataPoint(dp)
				_ = hasher.HashKey()
			}
		})
	}
}

// BenchmarkECSHashKeyOnly isolates HashKey after resource/dp are already set.
func BenchmarkECSHashKeyOnly(b *testing.B) {
	for _, resourceAttrs := range []int{10, 30, 50} {
		b.Run(fmt.Sprintf("r%d", resourceAttrs), func(b *testing.B) {
			resource, dp := newBenchDataPoint(resourceAttrs, 10)
			h := &ECSDataPointHasher{}
			h.UpdateResource(resource)
			h.UpdateDataPoint(dp)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = h.HashKey()
			}
		})
	}
}

func newBenchDataPoint(resourceAttrs, dataPointAttrs int) (pcommon.Resource, datapoints.DataPoint) {
	resource := pcommon.NewResource()
	for i := 0; i < resourceAttrs; i++ {
		resource.Attributes().PutStr(fmt.Sprintf("resource.attr.%02d", i), fmt.Sprintf("value-%d", i))
	}
	// Overlap a few keys so ECS merge overwrite path is exercised.
	resource.Attributes().PutStr("service.name", "bench-service")
	resource.Attributes().PutStr("host.name", "bench-host")

	metric := pmetric.NewMetric()
	metric.SetName("http.server.request.duration")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1_700_000_000, 0)))
	dp.SetDoubleValue(1.23)
	for i := range dataPointAttrs {
		dp.Attributes().PutStr(fmt.Sprintf("dp.attr.%02d", i), fmt.Sprintf("dp-%d", i))
	}
	dp.Attributes().PutStr("service.name", "overwritten-service")

	return resource, datapoints.NewNumber(metric, dp)
}
