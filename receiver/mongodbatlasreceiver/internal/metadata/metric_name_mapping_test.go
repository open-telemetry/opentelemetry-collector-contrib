package metadata

import (
	"fmt"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/model/pdata"
)

func TestKnownMetricsMapped(t *testing.T) {
	missingMetrics := make([]string, 0)
	wrongNames := make([]string, 0)
	// Test all at once so we get one failure with all
	// missing or unmatching metrics.
	for mongodbName, metricData := range metricNameMapping {
		m := pdata.NewMetric()
		metricf, _ := mappedMetricByName(mongodbName)
		if metricf == nil {
			missingMetrics = append(missingMetrics, mongodbName)
		} else {
			metricf.Init(m)
			if metricData.metricName != m.Name() {
				wrongNames = append(wrongNames, fmt.Sprintf("found: %s, expected: %s", m.Name(), metricData.metricName))
			}
		}
	}

	if len(missingMetrics) > 0 {
		t.Errorf("Missing metrics with MongoDB names: %s", strings.Join(missingMetrics, ", "))
	}

	if len(wrongNames) > 0 {
		t.Errorf("Mismatching names found: %s", strings.Join(wrongNames, ","))
	}
}
