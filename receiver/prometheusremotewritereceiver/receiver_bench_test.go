package prometheusremotewritereceiver

import (
	"fmt"
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// makeLabels builds a labels.Labels with the given total number of labels.
// It always includes "job", "instance", and the metric name label, plus (total-3) extra labels.
func makeLabels(total int) labels.Labels {
	if total < 3 {
		total = 3
	}
	m := make(map[string]string, total)
	m["job"] = "job"
	m["instance"] = "instance"
	m[labels.MetricName] = "metric"
	for i := 0; i < total-3; i++ {
		m[fmt.Sprintf("k%d", i)] = fmt.Sprintf("v%d", i)
	}
	return labels.FromMap(m)
}

// extractAttributesNoEnsure is a copy of extractAttributes without the EnsureCapacity call.
// It intentionally avoids pre-sizing the returned pcommon.Map so we can compare allocations.
func extractAttributesNoEnsure(ls labels.Labels) pcommon.Map {
	attrs := pcommon.NewMap()
	labelMap := ls.Map()
	for labelName, labelValue := range labelMap {
		if labelName == "instance" || labelName == "job" ||
			labelName == labels.MetricName ||
			labelName == "otel_scope_name" || labelName == "otel_scope_version" {
			continue
		}
		attrs.PutStr(labelName, labelValue)
	}
	return attrs
}

func BenchmarkExtractAttributes(b *testing.B) {
	sizes := []int{5, 20, 100, 500}
	for _, sz := range sizes {
		ls := makeLabels(sz)
		b.Run(fmt.Sprintf("%d_withEnsure", sz), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m := extractAttributes(ls)
				_ = m.Len()
			}
		})

		b.Run(fmt.Sprintf("%d_withoutEnsure", sz), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m := extractAttributesNoEnsure(ls)
				_ = m.Len()
			}
		})
	}
}
