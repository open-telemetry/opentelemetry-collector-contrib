// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func BenchmarkCollectorCollect(b *testing.B) {
	for _, tc := range []struct {
		name        string
		numMetrics  int
		numLabels   int
		constLabels prometheus.Labels
	}{
		{name: "1000metrics_5labels", numMetrics: 1000, numLabels: 5},
		{name: "1000metrics_20labels", numMetrics: 1000, numLabels: 20},
		{name: "1000metrics_5labels_constLabels", numMetrics: 1000, numLabels: 5, constLabels: prometheus.Labels{"env": "prod", "region": "us-east1"}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c := newCollector(&Config{
				MetricExpiration: time.Hour,
				ConstLabels:      tc.constLabels,
			}, zap.NewNop())

			rm := pmetric.NewResourceMetrics()
			resAttrs := rm.Resource().Attributes()
			resAttrs.PutStr("service.name", "test-service")
			resAttrs.PutStr("service.instance.id", "instance-123")
			resAttrs.PutStr("k8s.cluster.name", "prod-cluster")
			resAttrs.PutStr("k8s.namespace.name", "default")
			resAttrs.PutStr("k8s.pod.name", "app-0")

			sm := rm.ScopeMetrics().AppendEmpty()
			sm.Scope().SetName("otelcol/prometheusreceiver")
			sm.Scope().SetVersion("0.130.0")
			sm.SetSchemaUrl("https://opentelemetry.io/schemas/1.26.0")

			ts := pcommon.NewTimestampFromTime(time.Now())
			for i := range tc.numMetrics {
				m := sm.Metrics().AppendEmpty()
				m.SetName(fmt.Sprintf("http_request_duration_%d", i%10))
				m.SetDescription("Test metric")
				dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)
				dp.SetDoubleValue(float64(i))
				attrs := dp.Attributes()
				attrs.PutStr("series_id", fmt.Sprintf("series_%d", i))
				for j := 1; j < tc.numLabels; j++ {
					attrs.PutStr(fmt.Sprintf("http.label.%d", j), fmt.Sprintf("val_%d", j))
				}
			}
			c.accumulator.Accumulate(rm)

			b.ReportAllocs()
			for b.Loop() {
				ch := make(chan prometheus.Metric, tc.numMetrics+1)
				c.Collect(ch)
				close(ch)
			}
		})
	}
}
