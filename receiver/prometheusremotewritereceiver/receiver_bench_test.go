// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver

import (
	"fmt"
	"testing"

	"github.com/prometheus/prometheus/model/labels"
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
	m["__name__"] = "metric"
	for i := 0; i < total-3; i++ {
		m[fmt.Sprintf("k%d", i)] = fmt.Sprintf("v%d", i)
	}
	return labels.FromMap(m)
}

func BenchmarkExtractAttributes(b *testing.B) {
	sizes := []int{5, 20, 100, 500, 1000, 2000}
	for _, sz := range sizes {
		ls := makeLabels(sz)
		b.Run(fmt.Sprintf("%d", sz), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m := extractAttributes(ls)
				_ = m.Len()
			}
		})
	}
}
