package kafkaexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	metricViews := MetricViews()
	viewNames := []string{
		"kafka_exporter_messages",
	}
	for i, viewName := range viewNames {
		assert.Equal(t, viewName, metricViews[i].Name)
	}
}
